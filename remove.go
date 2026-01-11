package twig

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

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
	Force  WorktreeForceLevel
	DryRun bool
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
	DryRun       bool
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

	if r.DryRun {
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
	if branch == "" {
		return RemovedWorktree{Branch: branch, DryRun: opts.DryRun}, fmt.Errorf("branch name is required")
	}
	if c.Config.WorktreeSourceDir == "" {
		return RemovedWorktree{Branch: branch, DryRun: opts.DryRun}, fmt.Errorf("worktree source directory is not configured")
	}

	worktrees, err := c.Git.WorktreeList()
	if err != nil {
		return RemovedWorktree{Branch: branch, DryRun: opts.DryRun}, err
	}

	return c.runWithWorktrees(branch, cwd, opts, worktrees)
}

// RunMultiple removes multiple worktrees efficiently by caching the worktree list.
// This avoids repeated WorktreeList() calls when processing multiple branches.
func (c *RemoveCommand) RunMultiple(branches []string, cwd string, opts RemoveOptions) RemoveResult {
	var result RemoveResult

	if c.Config.WorktreeSourceDir == "" {
		err := fmt.Errorf("worktree source directory is not configured")
		for _, branch := range branches {
			result.Removed = append(result.Removed, RemovedWorktree{
				Branch: branch,
				DryRun: opts.DryRun,
				Err:    err,
			})
		}
		return result
	}

	worktrees, err := c.Git.WorktreeList()
	if err != nil {
		for _, branch := range branches {
			result.Removed = append(result.Removed, RemovedWorktree{
				Branch: branch,
				DryRun: opts.DryRun,
				Err:    err,
			})
		}
		return result
	}

	for _, branch := range branches {
		wt, err := c.runWithWorktrees(branch, cwd, opts, worktrees)
		if err != nil {
			wt.Branch = branch
			wt.Err = err
		}
		result.Removed = append(result.Removed, wt)
	}

	return result
}

// runWithWorktrees performs the actual removal using a pre-fetched worktree list.
func (c *RemoveCommand) runWithWorktrees(branch string, cwd string, opts RemoveOptions, worktrees []Worktree) (RemovedWorktree, error) {
	var result RemovedWorktree
	result.Branch = branch
	result.DryRun = opts.DryRun

	if branch == "" {
		return result, fmt.Errorf("branch name is required")
	}

	wtInfo, err := c.Git.WorktreeFindByBranchFromList(branch, worktrees)
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

	if opts.DryRun {
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
	if opts.DryRun {
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
