package twig

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
)

// CleanCommand removes merged worktrees that are no longer needed.
type CleanCommand struct {
	FS     FileSystem
	Git    *GitRunner
	Config *Config
	Log    *slog.Logger
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
func NewCleanCommand(fs FileSystem, git *GitRunner, cfg *Config, log *slog.Logger) *CleanCommand {
	if log == nil {
		log = NewNopLogger()
	}
	return &CleanCommand{
		FS:     fs,
		Git:    git,
		Config: cfg,
		Log:    log,
	}
}

// NewDefaultCleanCommand creates a new CleanCommand with production dependencies.
func NewDefaultCleanCommand(cfg *Config, log *slog.Logger) *CleanCommand {
	return NewCleanCommand(osFS{}, NewGitRunner(cfg.WorktreeSourceDir, WithLogger(log)), cfg, log)
}

// CleanCandidate represents a worktree that can be cleaned.
type CleanCandidate struct {
	Branch       string
	WorktreePath string
	Prunable     bool
	Skipped      bool
	SkipReason   SkipReason
	CleanReason  CleanReason
	ChangedFiles []FileStatus
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
		for i := range r.Removed {
			if r.Removed[i].Err != nil {
				fmt.Fprintf(&stderr, "error: %s: %v\n", r.Removed[i].Branch, r.Removed[i].Err)
				continue
			}
			if opts.Verbose {
				fmt.Fprintf(&stdout, "Removed worktree and branch: %s\n", r.Removed[i].Branch)
			}
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
				fmt.Fprintf(&stdout, "  %s\n", c.Branch)
				if c.CleanReason != "" {
					fmt.Fprintf(&stdout, "    ✓ %s\n", c.CleanReason)
				}
				fmt.Fprintf(&stdout, "    ✗ %s\n", c.SkipReason.Format(r.TargetBranch))
				if (c.SkipReason == SkipHasChanges || c.SkipReason == SkipDirtySubmodule) &&
					len(c.ChangedFiles) > 0 {
					for _, f := range c.ChangedFiles {
						fmt.Fprintf(&stdout, "      %s %s\n", f.Status, f.Path)
					}
				}
			}
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
			fmt.Fprintf(&stdout, "  %s\n", c.Branch)
			if c.CleanReason != "" {
				fmt.Fprintf(&stdout, "    ✓ %s\n", c.CleanReason)
			}
			fmt.Fprintf(&stdout, "    ✗ %s\n", c.SkipReason.Format(r.TargetBranch))
			if (c.SkipReason == SkipHasChanges || c.SkipReason == SkipDirtySubmodule) &&
				len(c.ChangedFiles) > 0 {
				for _, f := range c.ChangedFiles {
					fmt.Fprintf(&stdout, "      %s %s\n", f.Status, f.Path)
				}
			}
		}
	}

	return FormatResult{Stdout: stdout.String(), Stderr: stderr.String()}
}

// Run analyzes worktrees and optionally removes them.
// cwd is the current working directory (absolute path) passed from CLI layer.
func (c *CleanCommand) Run(ctx context.Context, cwd string, opts CleanOptions) (CleanResult, error) {
	c.Log.DebugContext(ctx, "run started",
		LogAttrKeyCategory.String(), LogCategoryClean,
		"check", opts.Check,
		"force", opts.Force,
		"target", opts.Target)

	var result CleanResult
	result.Check = opts.Check

	// Resolve target branch
	target, err := c.resolveTarget(ctx, opts.Target)
	if err != nil {
		return result, err
	}
	result.TargetBranch = target

	c.Log.DebugContext(ctx, "target resolved",
		LogAttrKeyCategory.String(), LogCategoryClean,
		"target", target)

	// Get all worktrees
	worktrees, err := c.Git.WorktreeList(ctx)
	if err != nil {
		return result, fmt.Errorf("failed to list worktrees: %w", err)
	}

	c.Log.DebugContext(ctx, "worktrees listed",
		LogAttrKeyCategory.String(), LogCategoryClean,
		"count", len(worktrees))

	// Pre-fetch branch merge status to avoid redundant git branch --merged calls
	mergeStatus, err := c.Git.ClassifyBranchMergeStatus(ctx, target)
	if err != nil {
		c.Log.DebugContext(ctx, "failed to classify branch merge status",
			LogAttrKeyCategory.String(), LogCategoryClean,
			"error", err.Error())
		// Continue without cache - Check() will fall back to individual calls
		mergeStatus = BranchMergeStatus{}
	} else {
		c.Log.DebugContext(ctx, "branch merge status classified",
			LogAttrKeyCategory.String(), LogCategoryClean,
			"mergedCount", len(mergeStatus.Merged),
			"sameCommitCount", len(mergeStatus.SameCommit))
	}

	// RemoveCommand is used for both Check and Run
	removeCmd := &RemoveCommand{
		FS:     c.FS,
		Git:    c.Git,
		Config: c.Config,
		Log:    c.Log,
	}

	// Analyze each worktree using RemoveCommand.Check (parallel execution)
	type indexedCandidate struct {
		index     int
		candidate CleanCandidate
	}

	var (
		wg         sync.WaitGroup
		mu         sync.Mutex
		candidates []indexedCandidate
	)

	candidateIndex := 0
	for i, wt := range worktrees {
		// Skip main worktree (first non-bare worktree)
		if i == 0 || wt.Bare {
			continue
		}

		// Handle detached HEAD worktrees directly (they have no branch name)
		if wt.Detached || wt.Branch == "" {
			c.Log.DebugContext(ctx, "skipping detached worktree",
				LogAttrKeyCategory.String(), LogCategoryClean,
				"path", wt.Path)
			candidates = append(candidates, indexedCandidate{
				index: candidateIndex,
				candidate: CleanCandidate{
					Branch:       wt.Branch,
					WorktreePath: wt.Path,
					Skipped:      true,
					SkipReason:   SkipDetached,
				},
			})
			candidateIndex++
			continue
		}

		// Launch parallel check.
		// Each Check() runs git status which is slow for large repos.
		// Parallelizing gives ~3x speedup.
		wg.Add(1)
		go func(idx int, wt Worktree) {
			defer wg.Done()

			c.Log.DebugContext(ctx, "checking worktree",
				LogAttrKeyCategory.String(), LogCategoryClean,
				"branch", wt.Branch)

			checkResult, err := removeCmd.Check(ctx, wt.Branch, CheckOptions{
				Force:        opts.Force,
				Target:       target,
				Cwd:          cwd,
				WorktreeInfo: &wt,
				MergeStatus:  mergeStatus,
			})
			if err != nil {
				c.Log.DebugContext(ctx, "check failed",
					LogAttrKeyCategory.String(), LogCategoryClean,
					"branch", wt.Branch,
					"error", err.Error())
				// Skip worktrees that fail to check (e.g., not in any worktree)
				return
			}

			candidate := CleanCandidate{
				Branch:       wt.Branch,
				WorktreePath: checkResult.WorktreePath,
				Prunable:     checkResult.Prunable,
				Skipped:      !checkResult.CanRemove,
				SkipReason:   checkResult.SkipReason,
				CleanReason:  checkResult.CleanReason,
				ChangedFiles: checkResult.ChangedFiles,
			}

			c.Log.DebugContext(ctx, "check completed",
				LogAttrKeyCategory.String(), LogCategoryClean,
				"branch", wt.Branch,
				"canRemove", checkResult.CanRemove,
				"reason", string(checkResult.CleanReason),
				"skipReason", string(checkResult.SkipReason))

			mu.Lock()
			candidates = append(candidates, indexedCandidate{index: idx, candidate: candidate})
			mu.Unlock()
		}(candidateIndex, wt)
		candidateIndex++
	}

	wg.Wait()

	// Sort candidates by original index to maintain consistent ordering
	slices.SortFunc(candidates, func(a, b indexedCandidate) int {
		return a.index - b.index
	})

	// Extract candidates in order
	for _, ic := range candidates {
		result.Candidates = append(result.Candidates, ic.candidate)
	}

	// If check mode, just return candidates (no execution)
	if result.Check {
		c.Log.DebugContext(ctx, "run completed (check mode)",
			LogAttrKeyCategory.String(), LogCategoryClean,
			"candidates", len(result.Candidates))
		return result, nil
	}

	// Execute removal for cleanable candidates (parallel execution)
	// Pass the same force level since RemoveCommand.Check already validated conditions
	type indexedRemoved struct {
		index int
		wt    RemovedWorktree
	}

	var (
		removeWg      sync.WaitGroup
		removeMu      sync.Mutex
		removedResult []indexedRemoved
	)

	removeIndex := 0
	for _, candidate := range result.Candidates {
		if candidate.Skipped {
			continue
		}

		removeWg.Add(1)
		go func(idx int, candidate CleanCandidate) {
			defer removeWg.Done()

			c.Log.DebugContext(ctx, "removing worktree",
				LogAttrKeyCategory.String(), LogCategoryClean,
				"branch", candidate.Branch)

			wt, err := removeCmd.Run(ctx, candidate.Branch, cwd, RemoveOptions{
				Force: opts.Force,
				Check: false,
			})
			if err != nil {
				c.Log.DebugContext(ctx, "removal failed",
					LogAttrKeyCategory.String(), LogCategoryClean,
					"branch", candidate.Branch,
					"error", err.Error())
				wt.Branch = candidate.Branch
				wt.Err = err
			}

			removeMu.Lock()
			removedResult = append(removedResult, indexedRemoved{index: idx, wt: wt})
			removeMu.Unlock()
		}(removeIndex, candidate)
		removeIndex++
	}

	removeWg.Wait()

	// Sort by original index to maintain consistent ordering
	slices.SortFunc(removedResult, func(a, b indexedRemoved) int {
		return a.index - b.index
	})

	// Extract results in order and track prunable branches
	for i := range removedResult {
		result.Removed = append(result.Removed, removedResult[i].wt)
		if removedResult[i].wt.Pruned {
			result.Pruned = true
		}
	}

	c.Log.DebugContext(ctx, "run completed",
		LogAttrKeyCategory.String(), LogCategoryClean,
		"removed", len(result.Removed))

	return result, nil
}

// resolveTarget resolves the target branch for merge checking.
// If target is specified, use it. Otherwise, auto-detect from first non-bare worktree.
func (c *CleanCommand) resolveTarget(ctx context.Context, target string) (string, error) {
	if target != "" {
		return target, nil
	}

	// Find first non-bare worktree (usually main)
	worktrees, err := c.Git.WorktreeList(ctx)
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
