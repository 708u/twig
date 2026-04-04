package twig

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
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
	Dirty   bool   // Include uncommitted changes from source worktree
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
	DirtyFiles    int
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
		"force", opts.Force,
		"dirty", opts.Dirty)

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

	// Source == target check (relaxed when --dirty is set)
	sameCommit := sourceCommit == targetCommit
	if sameCommit && !opts.Dirty {
		return result, fmt.Errorf("source and target are at the same commit")
	}

	if !sameCommit {
		allChangedOut, err := targetGit.Run(ctx, GitCmdDiff, "--name-only", "HEAD", sourceCommit)
		if err != nil {
			return result, fmt.Errorf("failed to diff: %w", err)
		}
		allChanged := splitNonEmpty(string(allChangedOut))
		result.ModifiedFiles = len(allChanged)

		deletedOut, err := targetGit.Run(ctx, GitCmdDiff, "--name-only", "--diff-filter=D", "HEAD", sourceCommit)
		if err != nil {
			return result, fmt.Errorf("failed to diff for deleted files: %w", err)
		}
		result.DeletedFiles = splitNonEmpty(string(deletedOut))

		addedOut, err := targetGit.Run(ctx, GitCmdDiff, "--name-only", "--diff-filter=A", "HEAD", sourceCommit)
		if err != nil {
			return result, fmt.Errorf("failed to diff for added files: %w", err)
		}
		result.AddedFiles = splitNonEmpty(string(addedOut))
	}

	// Resolve dirty files from source worktree (before check mode early return)
	var dirtyFiles []FileStatus
	var sourceWtPath string
	if opts.Dirty {
		sourceWt, err := c.Git.WorktreeFindByBranch(ctx, sourceBranch)
		if err != nil {
			return result, fmt.Errorf("--dirty requires source branch %q to have a worktree: %w", sourceBranch, err)
		}
		sourceWtPath = sourceWt.Path
		sourceGit := c.Git.InDir(sourceWtPath)
		dirtyFiles, err = sourceGit.ChangedFilesAll(ctx)
		if err != nil {
			return result, fmt.Errorf("failed to get dirty files from source: %w", err)
		}
		result.DirtyFiles = len(dirtyFiles)

		if sameCommit && len(dirtyFiles) == 0 {
			return result, fmt.Errorf("source and target are at the same commit and source has no uncommitted changes")
		}
	}

	if opts.Check {
		c.Log.DebugContext(ctx, "apply check completed",
			LogAttrKeyCategory.String(), LogCategoryOverlay,
			"modifiedFiles", result.ModifiedFiles,
			"deletedFiles", len(result.DeletedFiles),
			"addedFiles", len(result.AddedFiles),
			"dirtyFiles", result.DirtyFiles)
		return result, nil
	}

	if !sameCommit {
		if _, err := targetGit.Run(ctx, GitCmdCheckout, sourceBranch, "--", "."); err != nil {
			return result, fmt.Errorf("failed to checkout source files: %w", err)
		}

		for _, f := range result.DeletedFiles {
			path := filepath.Join(targetPath, f)
			if err := c.FS.Remove(path); err != nil && !c.FS.IsNotExist(err) {
				c.Log.DebugContext(ctx, "failed to remove file",
					LogAttrKeyCategory.String(), LogCategoryOverlay,
					"file", f, "error", err)
			}
		}

		if _, err := targetGit.Run(ctx, GitCmdReset, "HEAD"); err != nil {
			return result, fmt.Errorf("failed to unstage changes: %w", err)
		}
	}

	if opts.Dirty && len(dirtyFiles) > 0 {
		dirtyAdded, err := c.copyDirtyFiles(ctx, dirtyFiles, sourceWtPath, targetPath)
		if err != nil {
			return result, err
		}
		result.AddedFiles = mergeUnique(result.AddedFiles, dirtyAdded)
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
		"modifiedFiles", result.ModifiedFiles,
		"dirtyFiles", result.DirtyFiles)

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
	for _, f := range state.AddedFiles {
		path := filepath.Join(targetPath, f)
		if err := c.FS.Remove(path); err != nil && !c.FS.IsNotExist(err) {
			c.Log.DebugContext(ctx, "failed to remove file",
				LogAttrKeyCategory.String(), LogCategoryOverlay,
				"file", f, "error", err)
		}
	}

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
			if r.DirtyFiles > 0 {
				fmt.Fprintf(&stdout, "  %d dirty file(s) would be applied\n", r.DirtyFiles)
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
		if r.DirtyFiles > 0 {
			fmt.Fprintf(&stdout, ", %d dirty", r.DirtyFiles)
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

// copyDirtyFiles copies uncommitted changes from the source worktree to the target.
// Returns the list of newly-added file paths for restore tracking.
func (c *OverlayCommand) copyDirtyFiles(ctx context.Context, dirtyFiles []FileStatus, sourceWtPath, targetPath string) ([]string, error) {
	c.Log.DebugContext(ctx, "applying dirty files",
		LogAttrKeyCategory.String(), LogCategoryOverlay,
		"count", len(dirtyFiles),
		"source_worktree", sourceWtPath)

	var addedFiles []string
	for _, f := range dirtyFiles {
		srcPath := filepath.Join(sourceWtPath, f.Path)
		dstPath := filepath.Join(targetPath, f.Path)

		// ReadFile directly instead of Stat-then-Read to avoid TOCTOU.
		// This also handles edge cases like "DM" (staged delete then
		// re-created) correctly by checking actual file existence.
		data, readErr := c.FS.ReadFile(srcPath)
		if readErr != nil && c.FS.IsNotExist(readErr) {
			if err := c.FS.Remove(dstPath); err != nil && !c.FS.IsNotExist(err) {
				c.Log.DebugContext(ctx, "failed to remove dirty-deleted file",
					LogAttrKeyCategory.String(), LogCategoryOverlay,
					"file", f.Path, "error", err)
			}
			continue
		}
		if readErr != nil {
			return nil, fmt.Errorf("failed to read dirty file %s: %w", f.Path, readErr)
		}

		if err := c.FS.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory for %s: %w", f.Path, err)
		}

		// Preserve source file permissions (e.g., executable bit).
		perm := fs.FileMode(0644)
		if info, err := c.FS.Stat(srcPath); err == nil {
			perm = info.Mode().Perm()
		}

		if err := c.FS.WriteFile(dstPath, data, perm); err != nil {
			return nil, fmt.Errorf("failed to write dirty file %s: %w", f.Path, err)
		}

		// Track untracked and staged-new files for restore cleanup
		if f.Status == "??" || strings.HasPrefix(f.Status, "A") {
			addedFiles = append(addedFiles, f.Path)
		}
	}

	return addedFiles, nil
}

// mergeUnique merges b into a, skipping duplicates.
func mergeUnique(a, b []string) []string {
	seen := make(map[string]bool, len(a))
	for _, s := range a {
		seen[s] = true
	}
	for _, s := range b {
		if !seen[s] {
			a = append(a, s)
			seen[s] = true
		}
	}
	return a
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
