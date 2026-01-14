package twig

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

// CleanReason describes why a branch is cleanable.
type CleanReason string

const (
	CleanMerged       CleanReason = "merged"
	CleanUpstreamGone CleanReason = "upstream gone"
)

// CleanCandidate represents a worktree that can be cleaned.
type CleanCandidate struct {
	Branch       string
	WorktreePath string
	Prunable     bool
	Skipped      bool
	SkipReason   SkipReason
	CleanReason  CleanReason
}

// OrphanBranch represents a local branch without an associated worktree.
type OrphanBranch struct {
	Name string
}

// LockedWorktreeInfo represents a locked worktree with its lock reason.
type LockedWorktreeInfo struct {
	Branch     string
	Path       string
	LockReason string
}

// DetachedWorktreeInfo represents a worktree in detached HEAD state.
type DetachedWorktreeInfo struct {
	Path string
	HEAD string
}

// CleanResult aggregates results from clean operations.
type CleanResult struct {
	Candidates   []CleanCandidate
	Removed      []RemovedWorktree
	TargetBranch string
	Pruned       bool
	Check        bool // --check mode (show candidates only, no prompt)

	// Integrity info (populated in --check mode)
	OrphanBranches    []OrphanBranch
	LockedWorktrees   []LockedWorktreeInfo
	DetachedWorktrees []DetachedWorktreeInfo
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
			fmt.Fprintf(&stdout, "twig clean: %s\n", wt.Branch)
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
		if r.formatIntegrityInfo(&stdout, opts) {
			fmt.Fprintln(&stdout)
		}
		fmt.Fprintln(&stdout, "No worktrees to clean")
		return FormatResult{Stdout: stdout.String(), Stderr: stderr.String()}
	}

	// Output cleanable candidates with group header and reasons
	fmt.Fprintln(&stdout, "clean:")
	for _, c := range cleanable {
		reason := string(c.CleanReason)
		if c.Prunable {
			reason = "prunable, " + reason
		}
		fmt.Fprintf(&stdout, "  %s (%s)\n", c.Branch, reason)
	}

	// Output skipped candidates with group header (verbose only)
	if opts.Verbose && len(skipped) > 0 {
		fmt.Fprintln(&stdout)
		fmt.Fprintln(&stdout, "skip:")
		for _, c := range skipped {
			fmt.Fprintf(&stdout, "  %s (%s)\n", c.Branch, c.SkipReason)
		}
	}

	// Output integrity info
	r.formatIntegrityInfo(&stdout, opts)

	return FormatResult{Stdout: stdout.String(), Stderr: stderr.String()}
}

// formatIntegrityInfo appends integrity info sections to the output.
// Returns true if any info was written.
// All integrity info requires verbose mode.
func (r CleanResult) formatIntegrityInfo(stdout *strings.Builder, opts FormatOptions) bool {
	if !opts.Verbose {
		return false
	}

	wrote := false

	// Detached HEAD worktrees
	if len(r.DetachedWorktrees) > 0 {
		fmt.Fprintln(stdout)
		fmt.Fprintln(stdout, "detached:")
		for _, d := range r.DetachedWorktrees {
			fmt.Fprintf(stdout, "  %s (HEAD at %s)\n", d.Path, d.HEAD)
		}
		wrote = true
	}

	// Locked worktrees
	if len(r.LockedWorktrees) > 0 {
		fmt.Fprintln(stdout)
		fmt.Fprintln(stdout, "locked:")
		for _, l := range r.LockedWorktrees {
			if l.LockReason != "" {
				fmt.Fprintf(stdout, "  %s (reason: %s)\n", l.Branch, l.LockReason)
			} else {
				fmt.Fprintf(stdout, "  %s (no reason)\n", l.Branch)
			}
		}
		wrote = true
	}

	// Orphan branches
	if len(r.OrphanBranches) > 0 {
		fmt.Fprintln(stdout)
		fmt.Fprintln(stdout, "orphan branches:")
		for _, o := range r.OrphanBranches {
			fmt.Fprintf(stdout, "  %s (no worktree)\n", o.Name)
		}
		wrote = true
	}

	return wrote
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

	// Track worktree branches for orphan detection
	worktreeBranches := make(map[string]bool)

	// Analyze each worktree
	for i, wt := range worktrees {
		// Track branch for orphan detection
		if wt.Branch != "" {
			worktreeBranches[wt.Branch] = true
		}

		// Skip main worktree (first non-bare worktree)
		if i == 0 || wt.Bare {
			continue
		}

		// Collect integrity info
		if wt.Detached {
			result.DetachedWorktrees = append(result.DetachedWorktrees, DetachedWorktreeInfo{
				Path: wt.Path,
				HEAD: wt.ShortHEAD(),
			})
		}
		if wt.Locked {
			result.LockedWorktrees = append(result.LockedWorktrees, LockedWorktreeInfo{
				Branch:     wt.Branch,
				Path:       wt.Path,
				LockReason: wt.LockReason,
			})
		}

		var candidate CleanCandidate

		if wt.Prunable {
			// Prunable branch: worktree directory was deleted
			candidate = CleanCandidate{
				Branch:   wt.Branch,
				Prunable: true,
			}
			if reason := c.checkPrunableSkipReason(wt.Branch, target, opts.Force); reason != "" {
				candidate.Skipped = true
				candidate.SkipReason = reason
			}
		} else {
			// Normal worktree
			candidate = CleanCandidate{
				Branch:       wt.Branch,
				WorktreePath: wt.Path,
			}
			if reason := c.checkSkipReason(wt, cwd, target, opts.Force); reason != "" {
				candidate.Skipped = true
				candidate.SkipReason = reason
			}
		}

		// Set clean reason for non-skipped candidates
		if !candidate.Skipped {
			candidate.CleanReason = c.getCleanReason(wt.Branch, target)
		}

		result.Candidates = append(result.Candidates, candidate)
	}

	// Collect orphan branches
	result.OrphanBranches = c.findOrphanBranches(worktreeBranches)

	// If check mode, just return candidates (no execution)
	if result.Check {
		return result, nil
	}

	// Execute removal for cleanable candidates
	// RemoveCommand handles both normal and prunable worktrees
	removeCmd := &RemoveCommand{
		FS:     c.FS,
		Git:    c.Git,
		Config: c.Config,
	}

	// Determine force level for RemoveCommand.
	// At minimum use Unclean since clean already validated conditions.
	// Use higher level if clean was invoked with -ff to handle locked worktrees.
	removeForce := max(opts.Force, WorktreeForceLevelUnclean)

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

		// Track if any prunable branches were processed
		if wt.Pruned {
			result.Pruned = true
		}
	}

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
// force level controls which conditions can be bypassed (matches git worktree behavior).
func (c *CleanCommand) checkSkipReason(wt Worktree, cwd, target string, force WorktreeForceLevel) SkipReason {
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

	// Check merged
	if force < WorktreeForceLevelUnclean {
		merged, err := c.Git.IsBranchMerged(wt.Branch, target)
		if err != nil || !merged {
			return SkipNotMerged
		}
	}

	return ""
}

// checkPrunableSkipReason checks if a prunable branch should be skipped.
// Only checks merged status since worktree-specific conditions don't apply.
func (c *CleanCommand) checkPrunableSkipReason(branch, target string, force WorktreeForceLevel) SkipReason {
	if force < WorktreeForceLevelUnclean {
		merged, err := c.Git.IsBranchMerged(branch, target)
		if err != nil || !merged {
			return SkipNotMerged
		}
	}
	return ""
}

// getCleanReason determines why a branch is cleanable.
func (c *CleanCommand) getCleanReason(branch, target string) CleanReason {
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

// findOrphanBranches finds local branches that don't have associated worktrees.
func (c *CleanCommand) findOrphanBranches(worktreeBranches map[string]bool) []OrphanBranch {
	branches, err := c.Git.BranchList()
	if err != nil {
		return nil
	}

	var orphans []OrphanBranch
	for _, branch := range branches {
		if !worktreeBranches[branch] {
			orphans = append(orphans, OrphanBranch{Name: branch})
		}
	}
	return orphans
}
