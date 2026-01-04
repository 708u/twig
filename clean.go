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
	Yes     bool   // Execute without confirmation
	DryRun  bool   // Show candidates only
	Target  string // Target branch for merge check
	Verbose bool   // Show skip reasons
}

// NewCleanCommand creates a new CleanCommand with the given config.
func NewCleanCommand(cfg *Config) *CleanCommand {
	return &CleanCommand{
		FS:     osFS{},
		Git:    NewGitRunner(cfg.WorktreeSourceDir),
		Config: cfg,
	}
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
	DryRun       bool
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

	// Show candidates
	if r.DryRun || len(r.Removed) == 0 {
		for _, c := range r.Candidates {
			if c.Skipped {
				if opts.Verbose {
					fmt.Fprintf(&stdout, "skip: %s (%s)\n", c.Branch, c.SkipReason)
				}
			} else {
				fmt.Fprintf(&stdout, "clean: %s\n", c.Branch)
			}
		}
		if r.CleanableCount() == 0 {
			fmt.Fprintln(&stdout, "No worktrees to clean")
		}
		return FormatResult{Stdout: stdout.String(), Stderr: stderr.String()}
	}

	// Show removal results
	for _, wt := range r.Removed {
		if wt.Err != nil {
			fmt.Fprintf(&stderr, "error: %s: %v\n", wt.Branch, wt.Err)
			continue
		}
		fmt.Fprintf(&stdout, "gwt clean: %s\n", wt.Branch)
	}

	return FormatResult{Stdout: stdout.String(), Stderr: stderr.String()}
}

// Run analyzes worktrees and optionally removes them.
// cwd is the current working directory (absolute path) passed from CLI layer.
func (c *CleanCommand) Run(cwd string, opts CleanOptions) (CleanResult, error) {
	var result CleanResult
	result.DryRun = opts.DryRun || !opts.Yes

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
		if reason := c.checkSkipReason(wt, cwd, target); reason != "" {
			candidate.Skipped = true
			candidate.SkipReason = reason
		}

		result.Candidates = append(result.Candidates, candidate)
	}

	// If dry-run or no --yes, just return candidates
	if result.DryRun {
		return result, nil
	}

	// Execute removal for cleanable candidates
	removeCmd := &RemoveCommand{
		FS:     c.FS,
		Git:    c.Git,
		Config: c.Config,
	}

	for _, candidate := range result.Candidates {
		if candidate.Skipped {
			continue
		}

		wt, err := removeCmd.Run(candidate.Branch, cwd, RemoveOptions{
			Force:  true, // Force because we already checked conditions
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
// Priority: 1. opts.Target, 2. config.DefaultSource, 3. first non-bare worktree
func (c *CleanCommand) resolveTarget(target string) (string, error) {
	if target != "" {
		return target, nil
	}

	if c.Config.DefaultSource != "" {
		return c.Config.DefaultSource, nil
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
func (c *CleanCommand) checkSkipReason(wt WorktreeInfo, cwd, target string) SkipReason {
	// Check detached HEAD
	if wt.Detached {
		return SkipDetached
	}

	// Check locked
	if wt.Locked {
		return SkipLocked
	}

	// Check current directory
	if strings.HasPrefix(cwd, wt.Path) {
		return SkipCurrentDir
	}

	// Check uncommitted changes
	gitInDir := c.Git.InDir(wt.Path)
	hasChanges, err := gitInDir.HasChanges()
	if err != nil || hasChanges {
		return SkipHasChanges
	}

	// Check merged
	merged, err := c.Git.IsBranchMerged(wt.Branch, target)
	if err != nil || !merged {
		return SkipNotMerged
	}

	return ""
}
