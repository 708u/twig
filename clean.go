package gwt

import (
	"fmt"
	"strings"
)

// CleanCommand removes merged worktrees that are no longer needed.
type CleanCommand struct {
	FS     FileSystem
	Git    *GitRunner
	Config *Config
}

// CleanOptions configures the clean operation.
type CleanOptions struct {
	Yes     bool               // Execute without confirmation
	Check   bool               // Show candidates only (no prompt)
	Target  string             // Target branch for merge check (auto-detect if empty)
	Verbose bool               // Show skip reasons
	Force   WorktreeForceLevel // Force level: -f for unclean, -ff for locked
}

// NewCleanCommand creates a new CleanCommand with explicit dependencies.
// Use this for testing or when custom dependencies are needed.
func NewCleanCommand(fs FileSystem, git *GitRunner, cfg *Config) *CleanCommand {
	return &CleanCommand{
		FS:     fs,
		Git:    git,
		Config: cfg,
	}
}

// NewDefaultCleanCommand creates a new CleanCommand with production dependencies.
func NewDefaultCleanCommand(cfg *Config) *CleanCommand {
	return NewCleanCommand(osFS{}, NewGitRunner(cfg.WorktreeSourceDir), cfg)
}

// SkipReason describes why a worktree was skipped.
type SkipReason string

const (
	SkipNotMerged  SkipReason = "not merged"
	SkipHasChanges SkipReason = "has uncommitted changes"
	SkipLocked     SkipReason = "locked"
	SkipCurrentDir SkipReason = "current directory"
	SkipDetached   SkipReason = "detached HEAD"
)

// CleanCandidate represents a worktree that can be cleaned.
type CleanCandidate struct {
	Branch       string
	WorktreePath string
	Skipped      bool
	SkipReason   SkipReason
}

// CleanResult aggregates results from clean operations.
type CleanResult struct {
	Candidates   []CleanCandidate
	Removed      []RemovedWorktree
	TargetBranch string
	Pruned       bool
	Check        bool // --check mode (show candidates only, no prompt)
}

// CleanableCount returns the number of worktrees that can be cleaned.
func (r CleanResult) CleanableCount() int {
	count := 0
	for _, c := range r.Candidates {
		if !c.Skipped {
			count++
		}
	}
	return count
}

// Format formats the CleanResult for display.
func (r CleanResult) Format(opts FormatOptions) FormatResult {
	var stdout, stderr strings.Builder

	// Show removal results (execution completed)
	if !r.Check && len(r.Removed) > 0 {
		for _, wt := range r.Removed {
			if wt.Err != nil {
				fmt.Fprintf(&stderr, "error: %s: %v\n", wt.Branch, wt.Err)
				continue
			}
			fmt.Fprintf(&stdout, "gwt clean: %s\n", wt.Branch)
		}
		return FormatResult{Stdout: stdout.String(), Stderr: stderr.String()}
	}

	// Show candidates (check mode or before execution)
	var cleanable, skipped []CleanCandidate
	for _, c := range r.Candidates {
		if c.Skipped {
			skipped = append(skipped, c)
		} else {
			cleanable = append(cleanable, c)
		}
	}

	// No cleanable candidates
	if len(cleanable) == 0 {
		if opts.Verbose && len(skipped) > 0 {
			fmt.Fprintln(&stdout, "skip:")
			for _, c := range skipped {
				fmt.Fprintf(&stdout, "  %s (%s)\n", c.Branch, c.SkipReason)
			}
			fmt.Fprintln(&stdout)
		}
		fmt.Fprintln(&stdout, "No worktrees to clean")
		return FormatResult{Stdout: stdout.String(), Stderr: stderr.String()}
	}

	// Output cleanable candidates with group header
	fmt.Fprintln(&stdout, "clean:")
	for _, c := range cleanable {
		fmt.Fprintf(&stdout, "  %s\n", c.Branch)
	}

	// Output skipped candidates with group header (verbose only)
	if opts.Verbose && len(skipped) > 0 {
		fmt.Fprintln(&stdout)
		fmt.Fprintln(&stdout, "skip:")
		for _, c := range skipped {
			fmt.Fprintf(&stdout, "  %s (%s)\n", c.Branch, c.SkipReason)
		}
	}

	return FormatResult{Stdout: stdout.String(), Stderr: stderr.String()}
}

// Run analyzes worktrees and optionally removes them.
// cwd is the current working directory (absolute path) passed from CLI layer.
func (c *CleanCommand) Run(cwd string, opts CleanOptions) (CleanResult, error) {
	var result CleanResult
	result.Check = opts.Check

	// Resolve target branch
	target, err := c.resolveTarget(opts.Target)
	if err != nil {
		return result, err
	}
	result.TargetBranch = target

	// Get all worktrees
	worktrees, err := c.Git.WorktreeList()
	if err != nil {
		return result, fmt.Errorf("failed to list worktrees: %w", err)
	}

	// Analyze each worktree
	for i, wt := range worktrees {
		// Skip main worktree (first non-bare worktree)
		if i == 0 || wt.Bare {
			continue
		}

		candidate := CleanCandidate{
			Branch:       wt.Branch,
			WorktreePath: wt.Path,
		}

		// Check skip conditions
		if reason := c.checkSkipReason(wt, cwd, target, opts.Force); reason != "" {
			candidate.Skipped = true
			candidate.SkipReason = reason
		}

		result.Candidates = append(result.Candidates, candidate)
	}

	// If check mode, just return candidates (no execution)
	if result.Check {
		return result, nil
	}

	// Execute removal for cleanable candidates
	removeCmd := &RemoveCommand{
		FS:     c.FS,
		Git:    c.Git,
		Config: c.Config,
	}

	// Determine force level for RemoveCommand.
	// At minimum use Unclean since clean already validated conditions.
	// Use higher level if clean was invoked with -ff to handle locked worktrees.
	removeForce := opts.Force
	if removeForce < WorktreeForceLevelUnclean {
		removeForce = WorktreeForceLevelUnclean
	}

	for _, candidate := range result.Candidates {
		if candidate.Skipped {
			continue
		}

		wt, err := removeCmd.Run(candidate.Branch, cwd, RemoveOptions{
			Force:  removeForce,
			DryRun: false,
		})
		if err != nil {
			wt.Branch = candidate.Branch
			wt.Err = err
		}
		result.Removed = append(result.Removed, wt)
	}

	// Prune worktrees
	if _, err := c.Git.WorktreePrune(); err != nil {
		return result, fmt.Errorf("failed to prune worktrees: %w", err)
	}
	result.Pruned = true

	return result, nil
}

// resolveTarget resolves the target branch for merge checking.
// If target is specified, use it. Otherwise, auto-detect from first non-bare worktree.
func (c *CleanCommand) resolveTarget(target string) (string, error) {
	if target != "" {
		return target, nil
	}

	// Find first non-bare worktree (usually main)
	worktrees, err := c.Git.WorktreeList()
	if err != nil {
		return "", fmt.Errorf("failed to list worktrees: %w", err)
	}

	for _, wt := range worktrees {
		if !wt.Bare && wt.Branch != "" {
			return wt.Branch, nil
		}
	}

	return "", fmt.Errorf("no target branch found")
}

// checkSkipReason checks if worktree should be skipped and returns the reason.
// force level controls which conditions can be bypassed:
//   - WorktreeForceLevelNone: all conditions apply
//   - WorktreeForceLevelUnclean (-f): bypass HasChanges, NotMerged
//   - WorktreeForceLevelLocked (-ff): also bypass Locked
func (c *CleanCommand) checkSkipReason(wt WorktreeInfo, cwd, target string, force WorktreeForceLevel) SkipReason {
	// Check detached HEAD (never bypassed - RemoveCommand requires branch name)
	if wt.Detached {
		return SkipDetached
	}

	// Check current directory (never bypassed - dangerous to remove cwd)
	if strings.HasPrefix(cwd, wt.Path) {
		return SkipCurrentDir
	}

	// Check locked (bypassed with -ff)
	if wt.Locked && force < WorktreeForceLevelLocked {
		return SkipLocked
	}

	// Check uncommitted changes (bypassed with -f)
	if force < WorktreeForceLevelUnclean {
		gitInDir := c.Git.InDir(wt.Path)
		hasChanges, err := gitInDir.HasChanges()
		if err != nil || hasChanges {
			return SkipHasChanges
		}
	}

	// Check merged (bypassed with -f)
	if force < WorktreeForceLevelUnclean {
		merged, err := c.Git.IsBranchMerged(wt.Branch, target)
		if err != nil || !merged {
			return SkipNotMerged
		}
	}

	return ""
}
