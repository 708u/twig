package twig

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// SkipReason describes why a worktree was skipped.
type SkipReason string

const (
	SkipNotMerged  SkipReason = "not merged"
	SkipHasChanges SkipReason = "has uncommitted changes"
	SkipLocked     SkipReason = "locked"
	SkipCurrentDir SkipReason = "current directory"
	SkipDetached   SkipReason = "detached HEAD"
)

// CleanReason describes why a branch is cleanable.
type CleanReason string

const (
	CleanMerged       CleanReason = "merged"
	CleanUpstreamGone CleanReason = "upstream gone"
)

// CheckResult holds the result of checking whether a worktree can be removed.
type CheckResult struct {
	CanRemove    bool        // Whether the worktree can be removed
	SkipReason   SkipReason  // Reason if cannot be removed
	CleanReason  CleanReason // Reason if can be removed (for clean command display)
	Prunable     bool        // Whether worktree is prunable (directory was deleted externally)
	WorktreePath string      // Path to the worktree
	Branch       string      // Branch name
}

// CheckOptions configures the check operation.
type CheckOptions struct {
	Force  WorktreeForceLevel // Force level to apply
	Target string             // Target branch for merged check (empty = skip merged check)
	Cwd    string             // Current directory for cwd check
}

// RemoveCommand removes git worktrees with their associated branches.
type RemoveCommand struct {
	FS     FileSystem
	Git    *GitRunner
	Config *Config
}

// RemoveOptions configures the remove operation.
type RemoveOptions struct {
	// Force specifies the force level.
	// Matches git worktree behavior: -f for unclean, -f -f for locked.
	Force WorktreeForceLevel
	Check bool // Show what would be removed without making changes
}

// NewRemoveCommand creates a RemoveCommand with explicit dependencies.
func NewRemoveCommand(fs FileSystem, git *GitRunner, cfg *Config) *RemoveCommand {
	return &RemoveCommand{
		FS:     fs,
		Git:    git,
		Config: cfg,
	}
}

// NewDefaultRemoveCommand creates a RemoveCommand with production defaults.
func NewDefaultRemoveCommand(cfg *Config) *RemoveCommand {
	return NewRemoveCommand(osFS{}, NewGitRunner(cfg.WorktreeSourceDir), cfg)
}

// RemovedWorktree holds the result of a single worktree removal.
type RemovedWorktree struct {
	Branch       string
	WorktreePath string
	CleanedDirs  []string // Empty parent directories that were removed
	Pruned       bool     // Stale worktree record was pruned (directory was already deleted)
	Check        bool     // --check mode: show what would be removed
	GitOutput    []byte
	Err          error // nil if success
}

// RemoveResult aggregates results from remove operations.
type RemoveResult struct {
	Removed []RemovedWorktree
}

// HasErrors returns true if any errors occurred.
func (r RemoveResult) HasErrors() bool {
	for _, wt := range r.Removed {
		if wt.Err != nil {
			return true
		}
	}
	return false
}

// ErrorCount returns the number of failed removals.
func (r RemoveResult) ErrorCount() int {
	count := 0
	for _, wt := range r.Removed {
		if wt.Err != nil {
			count++
		}
	}
	return count
}

// Format formats the RemoveResult for display.
func (r RemoveResult) Format(opts FormatOptions) FormatResult {
	var stdout, stderr strings.Builder

	for _, wt := range r.Removed {
		if wt.Err != nil {
			formatRemoveError(&stderr, wt.Branch, wt.Err, opts.Verbose)
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
func formatRemoveError(w *strings.Builder, branch string, err error, verbose bool) {
	var gitErr *GitError
	if errors.As(err, &gitErr) {
		fmt.Fprintf(w, "error: %s: failed to %s\n", branch, gitErr.Op)
		if verbose && gitErr.Stderr != "" {
			fmt.Fprintf(w, "       git: %s\n", gitErr.Stderr)
		}
		if hint := gitErr.Hint(); hint != "" {
			fmt.Fprintf(w, "hint: %s\n", hint)
		}
	} else {
		// Fallback for non-GitError
		fmt.Fprintf(w, "error: %s: %v\n", branch, err)
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

	fmt.Fprintf(&stdout, "twig remove: %s\n", r.Branch)

	return FormatResult{Stdout: stdout.String()}
}

// Run removes the worktree and branch for the given branch name.
// cwd is used to prevent removal when inside the target worktree.
func (c *RemoveCommand) Run(branch string, cwd string, opts RemoveOptions) (RemovedWorktree, error) {
	var result RemovedWorktree
	result.Branch = branch
	result.Check = opts.Check

	if branch == "" {
		return result, fmt.Errorf("branch name is required")
	}
	if c.Config.WorktreeSourceDir == "" {
		return result, fmt.Errorf("worktree source directory is not configured")
	}

	wtInfo, err := c.Git.WorktreeFindByBranch(branch)
	if err != nil {
		return result, err
	}
	result.WorktreePath = wtInfo.Path
	result.Pruned = wtInfo.Prunable

	// Handle prunable worktree (directory already deleted externally)
	if wtInfo.Prunable {
		return c.removePrunable(branch, opts, result)
	}

	// Normal worktree removal
	if strings.HasPrefix(cwd, wtInfo.Path) {
		return result, fmt.Errorf("cannot remove: current directory is inside worktree %s", wtInfo.Path)
	}

	if opts.Check {
		result.CleanedDirs = c.predictEmptyParentDirs(wtInfo.Path)
		return result, nil
	}

	var gitOutput []byte
	var wtOpts []WorktreeRemoveOption
	if opts.Force > WorktreeForceLevelNone {
		wtOpts = append(wtOpts, WithForceRemove(opts.Force))
	}
	wtOut, err := c.Git.WorktreeRemove(wtInfo.Path, wtOpts...)
	if err != nil {
		return result, err
	}
	gitOutput = append(gitOutput, wtOut...)

	result.CleanedDirs = c.cleanupEmptyParentDirs(wtInfo.Path)

	var branchOpts []BranchDeleteOption
	if opts.Force > WorktreeForceLevelNone {
		branchOpts = append(branchOpts, WithForceDelete())
	}
	brOut, err := c.Git.BranchDelete(branch, branchOpts...)
	if err != nil {
		return result, err
	}
	gitOutput = append(gitOutput, brOut...)

	result.GitOutput = gitOutput
	return result, nil
}

// removePrunable handles removal of a prunable worktree (directory already deleted).
// It prunes the stale worktree record and deletes the branch.
func (c *RemoveCommand) removePrunable(branch string, opts RemoveOptions, result RemovedWorktree) (RemovedWorktree, error) {
	if opts.Check {
		return result, nil
	}

	// Prune stale worktree records
	if _, err := c.Git.WorktreePrune(); err != nil {
		return result, fmt.Errorf("failed to prune worktrees: %w", err)
	}

	// Delete the branch
	var branchOpts []BranchDeleteOption
	if opts.Force > WorktreeForceLevelNone {
		branchOpts = append(branchOpts, WithForceDelete())
	}
	brOut, err := c.Git.BranchDelete(branch, branchOpts...)
	if err != nil {
		result.Err = err
		return result, err
	}
	result.GitOutput = brOut

	return result, nil
}

// cleanupEmptyParentDirs removes empty parent directories up to WorktreeDestBaseDir.
// Returns the list of directories that were removed. Errors are ignored since
// cleanup failures should not fail the overall remove operation.
func (c *RemoveCommand) cleanupEmptyParentDirs(wtPath string) []string {
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
func (c *RemoveCommand) Check(branch string, opts CheckOptions) (CheckResult, error) {
	var result CheckResult
	result.Branch = branch

	if branch == "" {
		return result, fmt.Errorf("branch name is required")
	}
	if c.Config.WorktreeSourceDir == "" {
		return result, fmt.Errorf("worktree source directory is not configured")
	}

	wtInfo, err := c.Git.WorktreeFindByBranch(branch)
	if err != nil {
		return result, err
	}
	result.WorktreePath = wtInfo.Path
	result.Prunable = wtInfo.Prunable

	if wtInfo.Prunable {
		// Prunable branch: worktree directory was deleted externally
		if reason := c.checkPrunableSkipReason(branch, opts.Target, opts.Force); reason != "" {
			result.CanRemove = false
			result.SkipReason = reason
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
		if reason := c.checkSkipReason(wt, opts.Cwd, opts.Target, opts.Force); reason != "" {
			result.CanRemove = false
			result.SkipReason = reason
			return result, nil
		}
	}

	result.CanRemove = true
	// CleanReason requires a target branch to determine merge status
	if opts.Target != "" {
		result.CleanReason = c.getCleanReason(branch, opts.Target)
	}
	return result, nil
}

// checkSkipReason checks if worktree should be skipped and returns the reason.
// force level controls which conditions can be bypassed (matches git worktree behavior).
func (c *RemoveCommand) checkSkipReason(wt Worktree, cwd, target string, force WorktreeForceLevel) SkipReason {
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

	// Check uncommitted changes
	if force < WorktreeForceLevelUnclean {
		hasChanges, err := c.Git.InDir(wt.Path).HasChanges()
		if err != nil || hasChanges {
			return SkipHasChanges
		}
	}

	// Check merged (only when target is specified)
	if target != "" && force < WorktreeForceLevelUnclean {
		merged, err := c.Git.IsBranchMerged(wt.Branch, target)
		if err != nil || !merged {
			return SkipNotMerged
		}
	}

	return ""
}

// checkPrunableSkipReason checks if a prunable branch should be skipped.
// Only checks merged status since worktree-specific conditions don't apply.
func (c *RemoveCommand) checkPrunableSkipReason(branch, target string, force WorktreeForceLevel) SkipReason {
	// Check merged (only when target is specified)
	if target != "" && force < WorktreeForceLevelUnclean {
		merged, err := c.Git.IsBranchMerged(branch, target)
		if err != nil || !merged {
			return SkipNotMerged
		}
	}
	return ""
}

// getCleanReason determines why a branch is cleanable.
func (c *RemoveCommand) getCleanReason(branch, target string) CleanReason {
	// Check if branch is merged via traditional merge
	out, err := c.Git.Run(GitCmdBranch, "--merged", target, "--format=%(refname:short)")
	if err == nil {
		for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
			if line == branch {
				return CleanMerged
			}
		}
	}

	// Check if upstream is gone (squash/rebase merge)
	gone, err := c.Git.IsBranchUpstreamGone(branch)
	if err == nil && gone {
		return CleanUpstreamGone
	}

	return ""
}
