package twig

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"
)

// OverlayCommand overlays file contents from a source branch onto a target worktree.
type OverlayCommand struct {
	FS  FileSystem
	Git *GitRunner
	Log *slog.Logger
}

// OverlayOptions configures the overlay operation.
type OverlayOptions struct {
	Restore bool   // Restore the target worktree to its original state
	Check   bool   // Dry-run mode
	Force   bool   // Proceed even if target is dirty or HEAD has moved
	Target  string // Target branch (empty = current worktree)
}

// OverlayResult holds the result of an overlay operation.
type OverlayResult struct {
	SourceBranch  string
	SourceCommit  string
	TargetBranch  string
	TargetPath    string
	Restored      bool
	Check         bool
	ModifiedFiles int
	DeletedFiles  []string
	AddedFiles    []string
}

// OverlayFormatOptions configures overlay output formatting.
type OverlayFormatOptions struct {
	Verbose bool
	Quiet   bool
}

// overlayState is persisted in the git directory to track active overlays.
type overlayState struct {
	SourceBranch string   `json:"source_branch"`
	SourceCommit string   `json:"source_commit"`
	TargetBranch string   `json:"target_branch"`
	TargetCommit string   `json:"target_commit"`
	AddedFiles   []string `json:"added_files,omitempty"`
	CreatedAt    string   `json:"created_at"`
}

const overlayStateFile = "twig-overlay"

// NewOverlayCommand creates an OverlayCommand with explicit dependencies.
func NewOverlayCommand(fs FileSystem, git *GitRunner, log *slog.Logger) *OverlayCommand {
	if log == nil {
		log = NewNopLogger()
	}
	return &OverlayCommand{
		FS:  fs,
		Git: git,
		Log: log,
	}
}

// NewDefaultOverlayCommand creates an OverlayCommand with production defaults.
func NewDefaultOverlayCommand(gitDir string, log *slog.Logger) *OverlayCommand {
	return NewOverlayCommand(osFS{}, NewGitRunner(gitDir, WithLogger(log)), log)
}

// Run executes the overlay operation.
func (c *OverlayCommand) Run(ctx context.Context, sourceBranch string, cwd string, opts OverlayOptions) (OverlayResult, error) {
	if opts.Restore {
		return c.restore(ctx, cwd, opts)
	}
	return c.apply(ctx, sourceBranch, cwd, opts)
}

// apply overlays the source branch content onto the target worktree.
func (c *OverlayCommand) apply(ctx context.Context, sourceBranch string, cwd string, opts OverlayOptions) (OverlayResult, error) {
	c.Log.DebugContext(ctx, "apply started",
		LogAttrKeyCategory.String(), LogCategoryOverlay,
		"source", sourceBranch,
		"target", opts.Target,
		"check", opts.Check,
		"force", opts.Force)

	var result OverlayResult
	result.SourceBranch = sourceBranch
	result.Check = opts.Check

	// Resolve target worktree
	targetPath, targetBranch, err := c.resolveTarget(ctx, cwd, opts.Target)
	if err != nil {
		return result, err
	}
	result.TargetBranch = targetBranch
	result.TargetPath = targetPath

	targetGit := c.Git.InDir(targetPath)

	// Get target git directory (for state file)
	gitDir, err := targetGit.GitDir(ctx)
	if err != nil {
		return result, fmt.Errorf("failed to get git directory: %w", err)
	}

	// Check for existing overlay
	statePath := filepath.Join(gitDir, overlayStateFile)
	if _, err := c.FS.Stat(statePath); err == nil {
		return result, fmt.Errorf("overlay already active on %s\nhint: use 'twig overlay --restore' to restore first", targetBranch)
	}

	// Check for uncommitted changes
	if !opts.Force {
		hasChanges, err := targetGit.HasChanges(ctx)
		if err != nil {
			return result, fmt.Errorf("failed to check uncommitted changes: %w", err)
		}
		if hasChanges {
			return result, fmt.Errorf("target worktree has uncommitted changes\nhint: use --force to proceed (uncommitted changes will be lost)")
		}
	}

	// Resolve source commit
	sourceCommitOut, err := targetGit.Run(ctx, GitCmdRevParse, sourceBranch)
	if err != nil {
		return result, fmt.Errorf("branch %q not found", sourceBranch)
	}
	sourceCommit := strings.TrimSpace(string(sourceCommitOut))
	result.SourceCommit = sourceCommit

	// Get target HEAD commit
	targetCommitOut, err := targetGit.Run(ctx, GitCmdRevParse, "HEAD")
	if err != nil {
		return result, fmt.Errorf("failed to get target HEAD: %w", err)
	}
	targetCommit := strings.TrimSpace(string(targetCommitOut))

	// Source == target check
	if sourceCommit == targetCommit {
		return result, fmt.Errorf("source and target are at the same commit")
	}

	// Get all changed files in a single diff, then filter by type.
	// This avoids running three separate git diff commands.
	allChangedOut, err := targetGit.Run(ctx, GitCmdDiff, "--name-only", "HEAD", sourceCommit)
	if err != nil {
		return result, fmt.Errorf("failed to diff: %w", err)
	}
	allChanged := splitNonEmpty(string(allChangedOut))
	result.ModifiedFiles = len(allChanged)

	// Identify deleted files (in HEAD but not in source)
	deletedOut, err := targetGit.Run(ctx, GitCmdDiff, "--name-only", "--diff-filter=D", "HEAD", sourceCommit)
	if err != nil {
		return result, fmt.Errorf("failed to diff for deleted files: %w", err)
	}
	result.DeletedFiles = splitNonEmpty(string(deletedOut))

	// Identify added files (in source but not in HEAD)
	addedOut, err := targetGit.Run(ctx, GitCmdDiff, "--name-only", "--diff-filter=A", "HEAD", sourceCommit)
	if err != nil {
		return result, fmt.Errorf("failed to diff for added files: %w", err)
	}
	result.AddedFiles = splitNonEmpty(string(addedOut))

	if opts.Check {
		c.Log.DebugContext(ctx, "apply check completed",
			LogAttrKeyCategory.String(), LogCategoryOverlay,
			"modifiedFiles", result.ModifiedFiles,
			"deletedFiles", len(result.DeletedFiles),
			"addedFiles", len(result.AddedFiles))
		return result, nil
	}

	// Checkout source branch files onto target
	if _, err := targetGit.Run(ctx, GitCmdCheckout, sourceBranch, "--", "."); err != nil {
		return result, fmt.Errorf("failed to checkout source files: %w", err)
	}

	// Delete files that exist in target HEAD but not in source
	c.removeFiles(ctx, targetPath, result.DeletedFiles)

	// Unstage all changes
	if _, err := targetGit.Run(ctx, GitCmdReset, "HEAD"); err != nil {
		return result, fmt.Errorf("failed to unstage changes: %w", err)
	}

	// Write state file
	state := overlayState{
		SourceBranch: sourceBranch,
		SourceCommit: sourceCommit,
		TargetBranch: targetBranch,
		TargetCommit: targetCommit,
		AddedFiles:   result.AddedFiles,
		CreatedAt:    time.Now().UTC().Format(time.RFC3339),
	}
	stateData, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return result, fmt.Errorf("failed to marshal state: %w", err)
	}
	if err := c.FS.WriteFile(statePath, stateData, 0644); err != nil {
		return result, fmt.Errorf("failed to write state file: %w", err)
	}

	c.Log.DebugContext(ctx, "apply completed",
		LogAttrKeyCategory.String(), LogCategoryOverlay,
		"source", sourceBranch,
		"target", targetBranch,
		"modifiedFiles", result.ModifiedFiles)

	return result, nil
}

// restore removes the overlay and returns the target worktree to its original state.
func (c *OverlayCommand) restore(ctx context.Context, cwd string, opts OverlayOptions) (OverlayResult, error) {
	c.Log.DebugContext(ctx, "restore started",
		LogAttrKeyCategory.String(), LogCategoryOverlay,
		"target", opts.Target,
		"check", opts.Check,
		"force", opts.Force)

	var result OverlayResult
	result.Restored = true
	result.Check = opts.Check

	// Resolve target worktree
	targetPath, targetBranch, err := c.resolveTarget(ctx, cwd, opts.Target)
	if err != nil {
		return result, err
	}
	result.TargetBranch = targetBranch
	result.TargetPath = targetPath

	targetGit := c.Git.InDir(targetPath)

	// Get git directory
	gitDir, err := targetGit.GitDir(ctx)
	if err != nil {
		return result, fmt.Errorf("failed to get git directory: %w", err)
	}

	// Read state file
	statePath := filepath.Join(gitDir, overlayStateFile)
	stateData, err := c.FS.ReadFile(statePath)
	if err != nil {
		return result, fmt.Errorf("no overlay active on %s", targetBranch)
	}

	var state overlayState
	if err := json.Unmarshal(stateData, &state); err != nil {
		return result, fmt.Errorf("failed to parse state file: %w", err)
	}
	result.SourceBranch = state.SourceBranch
	result.SourceCommit = state.SourceCommit
	result.AddedFiles = state.AddedFiles

	// Check if HEAD has moved since overlay
	currentHEAD, err := targetGit.Run(ctx, GitCmdRevParse, "HEAD")
	if err != nil {
		return result, fmt.Errorf("failed to get current HEAD: %w", err)
	}
	if strings.TrimSpace(string(currentHEAD)) != state.TargetCommit {
		if !opts.Force {
			return result, fmt.Errorf(
				"HEAD has moved since overlay was applied\n"+
					"hint: commits were made on the overlaid worktree\n"+
					"hint: use 'git log --oneline %s..HEAD' to review\n"+
					"hint: use 'twig overlay --restore --force' to restore anyway",
				state.TargetCommit)
		}
		c.Log.DebugContext(ctx, "HEAD moved but --force specified",
			LogAttrKeyCategory.String(), LogCategoryOverlay)
	}

	if opts.Check {
		c.Log.DebugContext(ctx, "restore check completed",
			LogAttrKeyCategory.String(), LogCategoryOverlay,
			"source", state.SourceBranch)
		return result, nil
	}

	// Restore tracked files from HEAD
	if _, err := targetGit.Run(ctx, GitCmdCheckout, "HEAD", "--", "."); err != nil {
		return result, fmt.Errorf("failed to restore tracked files: %w", err)
	}

	// Remove files that were added by the overlay
	c.removeFiles(ctx, targetPath, state.AddedFiles)

	// Remove state file
	if err := c.FS.Remove(statePath); err != nil {
		return result, fmt.Errorf("failed to remove state file: %w", err)
	}

	c.Log.DebugContext(ctx, "restore completed",
		LogAttrKeyCategory.String(), LogCategoryOverlay,
		"source", state.SourceBranch,
		"target", targetBranch)

	return result, nil
}

// removeFiles removes a list of files from the target directory.
// Errors are logged but do not stop the operation.
func (c *OverlayCommand) removeFiles(ctx context.Context, targetPath string, files []string) {
	for _, f := range files {
		path := filepath.Join(targetPath, f)
		if err := c.FS.Remove(path); err != nil && !c.FS.IsNotExist(err) {
			c.Log.DebugContext(ctx, "failed to remove file",
				LogAttrKeyCategory.String(), LogCategoryOverlay,
				"file", f, "error", err)
		}
	}
}

// resolveTarget resolves the target worktree path and branch.
func (c *OverlayCommand) resolveTarget(ctx context.Context, cwd, target string) (string, string, error) {
	if target != "" {
		wt, err := c.Git.WorktreeFindByBranch(ctx, target)
		if err != nil {
			return "", "", fmt.Errorf("target worktree not found: %w", err)
		}
		return wt.Path, wt.Branch, nil
	}

	// Use current worktree
	root, err := c.Git.InDir(cwd).WorktreeRoot(ctx)
	if err != nil {
		return "", "", fmt.Errorf("current directory is not in any worktree: %w", err)
	}

	worktrees, err := c.Git.WorktreeList(ctx)
	if err != nil {
		return "", "", fmt.Errorf("failed to list worktrees: %w", err)
	}
	for _, wt := range worktrees {
		if wt.Path == root {
			branch := wt.Branch
			if branch == "" {
				branch = "HEAD"
			}
			return wt.Path, branch, nil
		}
	}
	return "", "", fmt.Errorf("current directory is not in any worktree")
}

// Format formats the OverlayResult for display.
func (r OverlayResult) Format(opts OverlayFormatOptions) FormatResult {
	if opts.Quiet {
		return FormatResult{}
	}

	var stdout, stderr strings.Builder

	if r.Check {
		if r.Restored {
			fmt.Fprintf(&stdout, "Would restore %s (remove overlay from %s)\n", r.TargetBranch, r.SourceBranch)
			if len(r.AddedFiles) > 0 {
				fmt.Fprintf(&stdout, "  %d overlay-added file(s) would be removed\n", len(r.AddedFiles))
			}
		} else {
			fmt.Fprintf(&stdout, "Would overlay %s with %s:\n", r.TargetBranch, r.SourceBranch)
			fmt.Fprintf(&stdout, "  %d file(s) would change\n", r.ModifiedFiles)
			if len(r.DeletedFiles) > 0 {
				fmt.Fprintf(&stdout, "  %d file(s) would be deleted\n", len(r.DeletedFiles))
			}
			if len(r.AddedFiles) > 0 {
				fmt.Fprintf(&stdout, "  %d file(s) would be added\n", len(r.AddedFiles))
			}
		}

		if opts.Verbose {
			if len(r.DeletedFiles) > 0 {
				fmt.Fprintln(&stdout, "Deleted files:")
				for _, f := range r.DeletedFiles {
					fmt.Fprintf(&stdout, "  %s\n", f)
				}
			}
			if len(r.AddedFiles) > 0 {
				fmt.Fprintln(&stdout, "Added files:")
				for _, f := range r.AddedFiles {
					fmt.Fprintf(&stdout, "  %s\n", f)
				}
			}
		}
		return FormatResult{Stdout: stdout.String(), Stderr: stderr.String()}
	}

	if r.Restored {
		fmt.Fprintf(&stdout, "Restored %s (removed overlay from %s)\n", r.TargetBranch, r.SourceBranch)
	} else {
		fmt.Fprintf(&stdout, "Overlaid %s with %s", r.TargetBranch, r.SourceBranch)
		fmt.Fprintf(&stdout, " (%d files changed", r.ModifiedFiles)
		if len(r.DeletedFiles) > 0 {
			fmt.Fprintf(&stdout, ", %d deleted", len(r.DeletedFiles))
		}
		if len(r.AddedFiles) > 0 {
			fmt.Fprintf(&stdout, ", %d added", len(r.AddedFiles))
		}
		fmt.Fprintln(&stdout, ")")

		// Warning about not committing
		fmt.Fprintln(&stderr, "warning: do not commit in the overlaid worktree.")
		fmt.Fprintln(&stderr, "         Use 'twig overlay --restore' when done.")
	}

	if opts.Verbose {
		if len(r.DeletedFiles) > 0 {
			fmt.Fprintln(&stdout, "Deleted files:")
			for _, f := range r.DeletedFiles {
				fmt.Fprintf(&stdout, "  %s\n", f)
			}
		}
		if len(r.AddedFiles) > 0 {
			fmt.Fprintln(&stdout, "Added files:")
			for _, f := range r.AddedFiles {
				fmt.Fprintf(&stdout, "  %s\n", f)
			}
		}
	}

	return FormatResult{Stdout: stdout.String(), Stderr: stderr.String()}
}

// splitNonEmpty splits a newline-separated string and removes empty entries.
func splitNonEmpty(s string) []string {
	var result []string
	for _, line := range strings.Split(strings.TrimSpace(s), "\n") {
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}
