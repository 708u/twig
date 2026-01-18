package twig

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
)

// SkipReason describes why a worktree was skipped.
type SkipReason string

const (
	SkipNotMerged      SkipReason = "not merged"
	SkipHasChanges     SkipReason = "has uncommitted changes"
	SkipLocked         SkipReason = "locked"
	SkipCurrentDir     SkipReason = "current directory"
	SkipDetached       SkipReason = "detached HEAD"
	SkipDirtySubmodule SkipReason = "submodule has uncommitted changes"
)

// SkipError represents an error when a worktree cannot be removed due to a skip condition.
type SkipError struct {
	Reason SkipReason
}

func (e *SkipError) Error() string {
	return fmt.Sprintf("cannot remove: %s", e.Reason)
}

// CleanReason describes why a branch is cleanable.
type CleanReason string

const (
	CleanMerged       CleanReason = "merged"
	CleanUpstreamGone CleanReason = "upstream gone"
)

// CheckResult holds the result of checking whether a worktree can be removed.
type CheckResult struct {
	CanRemove    bool         // Whether the worktree can be removed
	SkipReason   SkipReason   // Reason if cannot be removed
	CleanReason  CleanReason  // Reason if can be removed (for clean command display)
	Prunable     bool         // Whether worktree is prunable (directory was deleted externally)
	WorktreePath string       // Path to the worktree
	Branch       string       // Branch name
	ChangedFiles []FileStatus // Uncommitted changes (for verbose output)
}

// CheckOptions configures the check operation.
type CheckOptions struct {
	Force          WorktreeForceLevel // Force level to apply
	Target         string             // Target branch for merged check (empty = skip merged check)
	Cwd            string             // Current directory for cwd check
	WorktreeInfo   *Worktree          // Pre-fetched worktree info (skips WorktreeFindByBranch if set)
	MergedBranches map[string]bool    // Pre-fetched merged branches (skips IsBranchMerged if set)
}

// RemoveCommand removes git worktrees with their associated branches.
type RemoveCommand struct {
	FS     FileSystem
	Git    *GitRunner
	Config *Config
	Log    *slog.Logger
}

// RemoveOptions configures the remove operation.
type RemoveOptions struct {
	// Force specifies the force level.
	// Matches git worktree behavior: -f for unclean, -f -f for locked.
	Force WorktreeForceLevel
	Check bool // Show what would be removed without making changes
}

// NewRemoveCommand creates a RemoveCommand with explicit dependencies.
func NewRemoveCommand(fs FileSystem, git *GitRunner, cfg *Config, log *slog.Logger) *RemoveCommand {
	if log == nil {
		log = NewNopLogger()
	}
	return &RemoveCommand{
		FS:     fs,
		Git:    git,
		Config: cfg,
		Log:    log,
	}
}

// NewDefaultRemoveCommand creates a RemoveCommand with production defaults.
func NewDefaultRemoveCommand(cfg *Config, log *slog.Logger) *RemoveCommand {
	return NewRemoveCommand(osFS{}, NewGitRunnerWithLogger(cfg.WorktreeSourceDir, log), cfg, log)
}

// RemovedWorktree holds the result of a single worktree removal.
type RemovedWorktree struct {
	Branch       string
	WorktreePath string
	CleanedDirs  []string     // Empty parent directories that were removed
	Pruned       bool         // Stale worktree record was pruned (directory was already deleted)
	Check        bool         // --check mode: show what would be removed
	CanRemove    bool         // Whether the worktree can be removed (from Check)
	SkipReason   SkipReason   // Reason if cannot be removed (from Check)
	ChangedFiles []FileStatus // Uncommitted changes (for verbose output)
	GitOutput    []byte
	Err          error // nil if success
}

// RemoveResult aggregates results from remove operations.
type RemoveResult struct {
	Removed []RemovedWorktree
}

// HasErrors returns true if any errors occurred.
func (r RemoveResult) HasErrors() bool {
	for i := range r.Removed {
		if r.Removed[i].Err != nil {
			return true
		}
	}
	return false
}

// ErrorCount returns the number of failed removals.
func (r RemoveResult) ErrorCount() int {
	count := 0
	for i := range r.Removed {
		if r.Removed[i].Err != nil {
			count++
		}
	}
	return count
}

// Format formats the RemoveResult for display.
func (r RemoveResult) Format(opts FormatOptions) FormatResult {
	var stdout, stderr strings.Builder

	for i := range r.Removed {
		wt := &r.Removed[i]
		if wt.Err != nil {
			formatRemoveError(&stderr, wt.Branch, wt.Err, opts.Verbose, wt.ChangedFiles)
			continue
		}
		formatted := wt.Format(opts)
		stdout.WriteString(formatted.Stdout)
		stderr.WriteString(formatted.Stderr)
	}

	return FormatResult{Stdout: stdout.String(), Stderr: stderr.String()}
}

// formatRemoveError formats an error from the remove operation.
// It shows a short error message, and optionally the detailed git error.
func formatRemoveError(w *strings.Builder, branch string, err error, verbose bool, changedFiles []FileStatus) {
	var skipErr *SkipError
	var gitErr *GitError

	// Format error message
	switch {
	case errors.As(err, &gitErr):
		fmt.Fprintf(w, "error: %s: failed to %s\n", branch, gitErr.Op)
		if verbose && gitErr.Stderr != "" {
			fmt.Fprintf(w, "       git: %s\n", gitErr.Stderr)
		}
	default:
		fmt.Fprintf(w, "error: %s: %v\n", branch, err)
	}

	// Show changed files in verbose mode for SkipHasChanges
	if verbose && len(changedFiles) > 0 {
		if errors.As(err, &skipErr) && skipErr.Reason == SkipHasChanges {
			fmt.Fprintf(w, "Uncommitted changes:\n")
			for _, f := range changedFiles {
				fmt.Fprintf(w, "  %s %s\n", f.Status, f.Path)
			}
		}
	}

	// Format hint based on error type
	var hint string
	switch {
	case errors.As(err, &skipErr):
		switch skipErr.Reason {
		case SkipHasChanges, SkipNotMerged, SkipDirtySubmodule:
			hint = "use 'twig remove --force' to force removal"
		case SkipLocked:
			hint = "run 'git worktree unlock <path>' first, or use 'twig remove -f -f'"
		}
	case errors.As(err, &gitErr):
		switch {
		case strings.Contains(gitErr.Stderr, "modified or untracked files"):
			hint = "use 'twig remove --force' to force removal"
		case strings.Contains(gitErr.Stderr, "locked working tree"):
			hint = "run 'git worktree unlock <path>' first, or use 'twig remove -f -f'"
		}
	}
	if hint != "" {
		fmt.Fprintf(w, "hint: %s\n", hint)
	}
}

// Format formats the RemovedWorktree for display.
func (r RemovedWorktree) Format(opts FormatOptions) FormatResult {
	var stdout strings.Builder

	if r.Check {
		if r.Pruned {
			fmt.Fprintf(&stdout, "Would prune stale worktree record\n")
		} else if r.WorktreePath != "" {
			fmt.Fprintf(&stdout, "Would remove worktree: %s\n", r.WorktreePath)
		}
		// Show changed files in verbose check mode
		if opts.Verbose && len(r.ChangedFiles) > 0 {
			fmt.Fprintf(&stdout, "Uncommitted changes:\n")
			for _, f := range r.ChangedFiles {
				fmt.Fprintf(&stdout, "  %s %s\n", f.Status, f.Path)
			}
		}
		fmt.Fprintf(&stdout, "Would delete branch: %s\n", r.Branch)
		for _, dir := range r.CleanedDirs {
			fmt.Fprintf(&stdout, "Would remove empty directory: %s\n", dir)
		}
		return FormatResult{Stdout: stdout.String()}
	}

	if opts.Verbose {
		if len(r.GitOutput) > 0 {
			stdout.Write(r.GitOutput)
		}
		if r.Pruned {
			fmt.Fprintf(&stdout, "Pruned stale worktree and deleted branch: %s\n", r.Branch)
		} else {
			fmt.Fprintf(&stdout, "Removed worktree and branch: %s\n", r.Branch)
		}
		for _, dir := range r.CleanedDirs {
			fmt.Fprintf(&stdout, "Removed empty directory: %s\n", dir)
		}
	}

	return FormatResult{Stdout: stdout.String()}
}

// Run removes the worktree and branch for the given branch name.
// cwd is used to prevent removal when inside the target worktree.
func (c *RemoveCommand) Run(ctx context.Context, branch string, cwd string, opts RemoveOptions) (RemovedWorktree, error) {
	c.Log.DebugContext(ctx, "run started",
		"category", LogCategoryRemove,
		"branch", branch,
		"check", opts.Check,
		"force", opts.Force)

	var result RemovedWorktree
	result.Branch = branch
	result.Check = opts.Check

	// Check removal eligibility first
	checkResult, err := c.Check(ctx, branch, CheckOptions{
		Force: opts.Force,
		Cwd:   cwd,
	})
	if err != nil {
		return result, err
	}

	// Copy check results
	result.WorktreePath = checkResult.WorktreePath
	result.Pruned = checkResult.Prunable
	result.CanRemove = checkResult.CanRemove
	result.SkipReason = checkResult.SkipReason
	result.ChangedFiles = checkResult.ChangedFiles

	c.Log.DebugContext(ctx, "check completed",
		"category", LogCategoryRemove,
		"canRemove", checkResult.CanRemove,
		"branch", branch)

	if !checkResult.CanRemove {
		return result, &SkipError{Reason: checkResult.SkipReason}
	}

	// Handle prunable worktree (directory already deleted externally)
	if checkResult.Prunable {
		c.Log.DebugContext(ctx, "handling prunable worktree",
			"category", LogCategoryRemove,
			"branch", branch)
		return c.removePrunable(ctx, branch, opts, result)
	}

	// Check submodule status to determine effective force level.
	// Clean submodules require auto-force for git worktree remove,
	// but this is safe since Check() already verified no dirty submodules.
	effectiveForce := opts.Force
	smStatus, smErr := c.Git.InDir(checkResult.WorktreePath).CheckSubmoduleCleanStatus(ctx)
	if smErr == nil && smStatus == SubmoduleCleanStatusClean {
		if effectiveForce < WorktreeForceLevelUnclean {
			effectiveForce = WorktreeForceLevelUnclean
		}
	}
	c.Log.DebugContext(ctx, "submodule check",
		"category", LogCategoryRemove,
		"status", smStatus,
		"effectiveForce", effectiveForce,
		"branch", branch)

	if opts.Check {
		result.CleanedDirs = c.predictEmptyParentDirs(checkResult.WorktreePath)
		return result, nil
	}

	var gitOutput []byte
	var wtOpts []WorktreeRemoveOption
	if effectiveForce > WorktreeForceLevelNone {
		wtOpts = append(wtOpts, WithForceRemove(effectiveForce))
	}
	wtOut, err := c.Git.WorktreeRemove(ctx, checkResult.WorktreePath, wtOpts...)
	if err != nil {
		return result, err
	}
	gitOutput = append(gitOutput, wtOut...)

	result.CleanedDirs = c.cleanupEmptyParentDirs(ctx, checkResult.WorktreePath)
	if len(result.CleanedDirs) > 0 {
		c.Log.DebugContext(ctx, "cleaned empty dirs",
			"category", LogCategoryRemove,
			"count", len(result.CleanedDirs),
			"branch", branch)
	}

	var branchOpts []BranchDeleteOption
	if opts.Force > WorktreeForceLevelNone {
		branchOpts = append(branchOpts, WithForceDelete())
	} else {
		// upstream gone (squash/rebase merge) requires -D since commits differ
		// Run() reaches here only after Check() verified no uncommitted changes
		if gone, goneErr := c.Git.IsBranchUpstreamGone(ctx, branch); goneErr == nil && gone {
			c.Log.DebugContext(ctx, "upstream gone, using force delete",
				"category", LogCategoryRemove,
				"branch", branch)
			branchOpts = append(branchOpts, WithForceDelete())
		}
	}
	brOut, err := c.Git.BranchDelete(ctx, branch, branchOpts...)
	if err != nil {
		return result, err
	}
	gitOutput = append(gitOutput, brOut...)

	result.GitOutput = gitOutput

	c.Log.DebugContext(ctx, "run completed",
		"category", LogCategoryRemove,
		"branch", branch)

	return result, nil
}

// removePrunable handles removal of a prunable worktree (directory already deleted).
// It prunes the stale worktree record and deletes the branch.
func (c *RemoveCommand) removePrunable(ctx context.Context, branch string, opts RemoveOptions, result RemovedWorktree) (RemovedWorktree, error) {
	if opts.Check {
		return result, nil
	}

	c.Log.DebugContext(ctx, "pruning stale worktree record",
		"category", LogCategoryRemove,
		"branch", branch)

	// Prune stale worktree records
	if _, err := c.Git.WorktreePrune(ctx); err != nil {
		return result, fmt.Errorf("failed to prune worktrees: %w", err)
	}

	// Delete the branch
	var branchOpts []BranchDeleteOption
	if opts.Force > WorktreeForceLevelNone {
		branchOpts = append(branchOpts, WithForceDelete())
	} else {
		// upstream gone (squash/rebase merge) requires -D since commits differ
		if gone, err := c.Git.IsBranchUpstreamGone(ctx, branch); err == nil && gone {
			c.Log.DebugContext(ctx, "prunable: upstream gone, using force delete",
				"category", LogCategoryRemove,
				"branch", branch)
			branchOpts = append(branchOpts, WithForceDelete())
		}
	}
	brOut, err := c.Git.BranchDelete(ctx, branch, branchOpts...)
	if err != nil {
		result.Err = err
		return result, err
	}
	result.GitOutput = brOut

	c.Log.DebugContext(ctx, "run completed",
		"category", LogCategoryRemove,
		"branch", branch,
		"prunable", true)

	return result, nil
}

// cleanupEmptyParentDirs removes empty parent directories up to WorktreeDestBaseDir.
// Returns the list of directories that were removed. Errors are ignored since
// cleanup failures should not fail the overall remove operation.
func (c *RemoveCommand) cleanupEmptyParentDirs(ctx context.Context, wtPath string) []string {
	var cleaned []string
	baseDir := c.Config.WorktreeDestBaseDir
	if baseDir == "" {
		return cleaned
	}

	current := filepath.Dir(wtPath)
	for current != baseDir && strings.HasPrefix(current, baseDir) {
		entries, err := c.FS.ReadDir(current)
		if err != nil {
			break
		}
		if len(entries) > 0 {
			break
		}
		if err := c.FS.Remove(current); err != nil {
			break
		}
		c.Log.DebugContext(ctx, "removed empty dir",
			"category", LogCategoryRemove,
			"dir", current)
		cleaned = append(cleaned, current)
		current = filepath.Dir(current)
	}

	return cleaned
}

// predictEmptyParentDirs predicts which parent directories would become empty
// if wtPath were removed. Used for dry-run mode.
func (c *RemoveCommand) predictEmptyParentDirs(wtPath string) []string {
	var wouldClean []string
	baseDir := c.Config.WorktreeDestBaseDir
	if baseDir == "" {
		return wouldClean
	}

	// Track the path being "removed" in simulation
	removedPath := wtPath
	current := filepath.Dir(wtPath)

	for current != baseDir && strings.HasPrefix(current, baseDir) {
		entries, err := c.FS.ReadDir(current)
		if err != nil {
			break
		}
		// Check if directory would be empty after removing the simulated path
		remaining := 0
		for _, entry := range entries {
			entryPath := filepath.Join(current, entry.Name())
			if entryPath != removedPath {
				remaining++
			}
		}
		if remaining > 0 {
			break
		}
		wouldClean = append(wouldClean, current)
		removedPath = current
		current = filepath.Dir(current)
	}

	return wouldClean
}

// Check checks whether a worktree can be removed based on the given options.
// This method does not modify any state.
func (c *RemoveCommand) Check(ctx context.Context, branch string, opts CheckOptions) (CheckResult, error) {
	var result CheckResult
	result.Branch = branch

	if branch == "" {
		return result, fmt.Errorf("branch name is required")
	}
	if c.Config.WorktreeSourceDir == "" {
		return result, fmt.Errorf("worktree source directory is not configured")
	}

	// Use pre-fetched worktree info if available to avoid redundant WorktreeList calls
	var wtInfo *Worktree
	if opts.WorktreeInfo != nil {
		wtInfo = opts.WorktreeInfo
	} else {
		var err error
		wtInfo, err = c.Git.WorktreeFindByBranch(ctx, branch)
		if err != nil {
			return result, err
		}
	}
	result.WorktreePath = wtInfo.Path
	result.Prunable = wtInfo.Prunable

	c.Log.DebugContext(ctx, "checking",
		"category", LogCategoryRemove,
		"branch", branch,
		"path", wtInfo.Path,
		"prunable", wtInfo.Prunable)

	if wtInfo.Prunable {
		// Prunable branch: worktree directory was deleted externally
		if reason := c.checkPrunableSkipReason(ctx, branch, opts.Target, opts.Force, opts.MergedBranches); reason != "" {
			result.CanRemove = false
			result.SkipReason = reason
			c.Log.DebugContext(ctx, "skip",
				"category", LogCategoryRemove,
				"reason", reason,
				"branch", branch)
			return result, nil
		}
	} else {
		// Normal worktree
		wt := Worktree{
			Path:     wtInfo.Path,
			Branch:   wtInfo.Branch,
			Locked:   wtInfo.Locked,
			Detached: wtInfo.Detached,
		}
		// Get changed files for verbose output (low cost, useful for all cases)
		if changedFiles, err := c.Git.InDir(wtInfo.Path).ChangedFiles(ctx); err == nil {
			result.ChangedFiles = changedFiles
		}
		if reason := c.checkSkipReason(ctx, wt, opts.Cwd, opts.Target, opts.Force, result.ChangedFiles, opts.MergedBranches); reason != "" {
			result.CanRemove = false
			result.SkipReason = reason
			c.Log.DebugContext(ctx, "skip",
				"category", LogCategoryRemove,
				"reason", reason,
				"branch", branch)
			return result, nil
		}
	}

	result.CanRemove = true
	// CleanReason requires a target branch to determine merge status
	if opts.Target != "" {
		result.CleanReason = c.getCleanReason(ctx, branch, opts.Target)
		if result.CleanReason != "" {
			c.Log.DebugContext(ctx, "clean reason",
				"category", LogCategoryRemove,
				"reason", result.CleanReason,
				"branch", branch)
		}
	}
	return result, nil
}

// checkSkipReason checks if worktree should be skipped and returns the reason.
// force level controls which conditions can be bypassed (matches git worktree behavior).
// changedFiles is pre-fetched to avoid redundant git status calls.
// mergedBranches is pre-fetched to avoid redundant git branch --merged calls.
func (c *RemoveCommand) checkSkipReason(ctx context.Context, wt Worktree, cwd, target string, force WorktreeForceLevel, changedFiles []FileStatus, mergedBranches map[string]bool) SkipReason {
	// Check detached HEAD (never bypassed)
	if wt.Detached {
		return SkipDetached
	}

	// Check current directory (never bypassed)
	if strings.HasPrefix(cwd, wt.Path) {
		return SkipCurrentDir
	}

	// Check locked
	if wt.Locked && force < WorktreeForceLevelLocked {
		return SkipLocked
	}

	// Check dirty submodule and uncommitted changes
	if force < WorktreeForceLevelUnclean {
		smStatus, err := c.Git.InDir(wt.Path).CheckSubmoduleCleanStatus(ctx)
		if err == nil && smStatus == SubmoduleCleanStatusDirty {
			return SkipDirtySubmodule
		}

		// Use pre-fetched changedFiles instead of calling HasChanges() again
		if changedFiles != nil {
			if len(changedFiles) > 0 {
				return SkipHasChanges
			}
		} else {
			// Fallback: changedFiles was not fetched (error occurred)
			hasChanges, err := c.Git.InDir(wt.Path).HasChanges(ctx)
			if err != nil || hasChanges {
				return SkipHasChanges
			}
		}
	}

	// Check merged (only when target is specified)
	if target != "" && force < WorktreeForceLevelUnclean {
		merged := c.isBranchMergedCached(ctx, wt.Branch, target, mergedBranches)
		if !merged {
			return SkipNotMerged
		}
	}

	return ""
}

// checkPrunableSkipReason checks if a prunable branch should be skipped.
// Only checks merged status since worktree-specific conditions don't apply.
// mergedBranches is pre-fetched to avoid redundant git branch --merged calls.
func (c *RemoveCommand) checkPrunableSkipReason(ctx context.Context, branch, target string, force WorktreeForceLevel, mergedBranches map[string]bool) SkipReason {
	// Check merged (only when target is specified)
	if target != "" && force < WorktreeForceLevelUnclean {
		merged := c.isBranchMergedCached(ctx, branch, target, mergedBranches)
		if !merged {
			return SkipNotMerged
		}
	}
	return ""
}

// isBranchMergedCached checks if a branch is merged using pre-fetched cache or fallback to IsBranchMerged.
func (c *RemoveCommand) isBranchMergedCached(ctx context.Context, branch, target string, mergedBranches map[string]bool) bool {
	if mergedBranches != nil {
		return mergedBranches[branch]
	}
	// Fallback: no cache available
	merged, err := c.Git.IsBranchMerged(ctx, branch, target)
	return err == nil && merged
}

// getCleanReason determines why a branch is cleanable.
func (c *RemoveCommand) getCleanReason(ctx context.Context, branch, target string) CleanReason {
	// Check if branch is merged via traditional merge
	out, err := c.Git.Run(ctx, GitCmdBranch, "--merged", target, "--format=%(refname:short)")
	if err == nil {
		for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
			if line == branch {
				return CleanMerged
			}
		}
	}

	// Check if upstream is gone (squash/rebase merge)
	gone, err := c.Git.IsBranchUpstreamGone(ctx, branch)
	if err == nil && gone {
		return CleanUpstreamGone
	}

	return ""
}
