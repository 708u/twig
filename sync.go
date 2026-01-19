package twig

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"
)

// LogCategorySync is the log category for sync operations.
const LogCategorySync = "sync"

// SyncCommand syncs symlinks and submodules from source worktree to target worktrees.
type SyncCommand struct {
	FS  FileSystem
	Git *GitRunner
	Log *slog.Logger
}

// SyncOptions configures the sync operation.
type SyncOptions struct {
	Check         bool   // Show what would be synced (dry-run)
	Force         bool   // Replace existing symlinks
	All           bool   // Sync all worktrees (except main)
	Source        string // Source branch (empty = use DefaultSource)
	DefaultSource string // Fallback when Source is empty (from config)
	Verbose       bool   // Verbose output
}

// SyncTargetResult holds the result of syncing a single worktree.
type SyncTargetResult struct {
	Branch        string
	WorktreePath  string
	Symlinks      []SymlinkResult
	SubmoduleInit SubmoduleInitResult
	Skipped       bool
	SkipReason    string
	Err           error
}

// SyncResult aggregates results from sync operations.
type SyncResult struct {
	Targets       []SyncTargetResult
	SourceBranch  string
	Check         bool // --check mode
	NothingToSync bool // No symlinks or submodules configured
}

// NewSyncCommand creates a SyncCommand with explicit dependencies.
func NewSyncCommand(fs FileSystem, git *GitRunner, log *slog.Logger) *SyncCommand {
	if log == nil {
		log = NewNopLogger()
	}
	return &SyncCommand{
		FS:  fs,
		Git: git,
		Log: log,
	}
}

// NewDefaultSyncCommand creates a SyncCommand with production defaults.
func NewDefaultSyncCommand(gitDir string, log *slog.Logger) *SyncCommand {
	return NewSyncCommand(osFS{}, NewGitRunner(gitDir, WithLogger(log)), log)
}

// SyncFormatOptions configures sync output formatting.
type SyncFormatOptions struct {
	Verbose bool
	Quiet   bool
}

// Format formats the SyncResult for display.
func (r SyncResult) Format(opts SyncFormatOptions) FormatResult {
	if opts.Quiet {
		return r.formatQuiet()
	}
	return r.formatDefault(opts)
}

// formatQuiet outputs minimal information.
func (r SyncResult) formatQuiet() FormatResult {
	var stdout strings.Builder
	for i := range r.Targets {
		t := &r.Targets[i]
		if t.Err == nil && !t.Skipped {
			fmt.Fprintln(&stdout, t.WorktreePath)
		}
	}
	return FormatResult{Stdout: stdout.String()}
}

// formatDefault outputs the default or verbose format.
func (r SyncResult) formatDefault(opts SyncFormatOptions) FormatResult {
	var stdout, stderr strings.Builder

	// Handle nothing to sync
	if r.NothingToSync {
		fmt.Fprintln(&stdout, "nothing to sync (no symlinks or submodules configured)")
		return FormatResult{Stdout: stdout.String()}
	}

	// Check mode header
	if r.Check && len(r.Targets) > 0 {
		fmt.Fprintf(&stdout, "Would sync from %s:\n\n", r.SourceBranch)
	}

	for i := range r.Targets {
		t := &r.Targets[i]
		if t.Err != nil {
			fmt.Fprintf(&stderr, "error: %s: %v\n", t.Branch, t.Err)
			continue
		}

		if r.Check {
			r.formatCheckTarget(&stdout, *t, opts)
		} else {
			r.formatTarget(&stdout, &stderr, *t, opts)
		}
	}

	return FormatResult{Stdout: stdout.String(), Stderr: stderr.String()}
}

// formatCheckTarget formats a single target in check mode.
func (r SyncResult) formatCheckTarget(stdout *strings.Builder, t SyncTargetResult, opts SyncFormatOptions) {
	if t.Skipped {
		if opts.Verbose {
			fmt.Fprintf(stdout, "%s:\n  (skipped: %s)\n\n", t.Branch, t.SkipReason)
		}
		return
	}

	fmt.Fprintf(stdout, "%s:\n", t.Branch)
	for _, s := range t.Symlinks {
		if !s.Skipped {
			fmt.Fprintf(stdout, "  Would create symlink: %s\n", s.Dst)
		} else if opts.Verbose {
			fmt.Fprintf(stdout, "  Would skip: %s (%s)\n", s.Dst, s.Reason)
		}
	}
	if t.SubmoduleInit.Attempted {
		fmt.Fprintln(stdout, "  Would initialize submodules")
	}
	fmt.Fprintln(stdout)
}

// formatTarget formats a single target in normal mode.
func (r SyncResult) formatTarget(stdout, stderr *strings.Builder, t SyncTargetResult, opts SyncFormatOptions) {
	// Count created symlinks
	var createdCount int
	for _, s := range t.Symlinks {
		if s.Skipped {
			fmt.Fprintf(stderr, "warning: %s\n", s.Reason)
		} else {
			createdCount++
		}
	}

	// Output submodule init warning
	if t.SubmoduleInit.Skipped {
		fmt.Fprintf(stderr, "warning: %s\n", t.SubmoduleInit.Reason)
	}

	if opts.Verbose {
		fmt.Fprintf(stdout, "Syncing from %s to %s\n", r.SourceBranch, t.Branch)
		for _, s := range t.Symlinks {
			if !s.Skipped {
				fmt.Fprintf(stdout, "Created symlink: %s -> %s\n", s.Dst, s.Src)
			}
		}
		if t.SubmoduleInit.Attempted && t.SubmoduleInit.Count > 0 {
			fmt.Fprintf(stdout, "Initialized %d submodule(s)\n", t.SubmoduleInit.Count)
		}
	}

	if t.Skipped {
		fmt.Fprintf(stdout, "twig sync: %s (skipped: %s)\n", t.Branch, t.SkipReason)
		return
	}

	var submoduleInfo string
	if t.SubmoduleInit.Attempted && t.SubmoduleInit.Count > 0 {
		submoduleInfo = fmt.Sprintf(", %d submodule(s)", t.SubmoduleInit.Count)
	}
	fmt.Fprintf(stdout, "twig sync: %s (%d symlinks%s)\n", t.Branch, createdCount, submoduleInfo)
}

// Run syncs symlinks and submodules from source to target worktrees.
func (c *SyncCommand) Run(ctx context.Context, targets []string, cwd string, opts SyncOptions) (SyncResult, error) {
	c.Log.DebugContext(ctx, "run started",
		LogAttrKeyCategory.String(), LogCategorySync,
		"targets", targets,
		"all", opts.All,
		"check", opts.Check,
		"force", opts.Force)

	var result SyncResult
	result.Check = opts.Check

	// Resolve source branch
	sourceBranch, err := resolveSource(opts.Source, opts.DefaultSource)
	if err != nil {
		return result, err
	}
	result.SourceBranch = sourceBranch

	c.Log.DebugContext(ctx, "source resolved",
		LogAttrKeyCategory.String(), LogCategorySync,
		"source", sourceBranch)

	// Find source worktree
	sourceWT, err := c.Git.WorktreeFindByBranch(ctx, sourceBranch)
	if err != nil {
		return result, fmt.Errorf("failed to find source worktree for branch %q: %w", sourceBranch, err)
	}

	// Load config from source worktree
	configResult, err := LoadConfig(sourceWT.Path)
	if err != nil {
		return result, fmt.Errorf("failed to load config from source worktree: %w", err)
	}
	sourceCfg := configResult.Config

	c.Log.DebugContext(ctx, "config loaded from source",
		LogAttrKeyCategory.String(), LogCategorySync,
		"path", sourceWT.Path,
		"symlinksCount", len(sourceCfg.Symlinks),
		"initSubmodules", sourceCfg.InitSubmodules)

	// Check if there's anything to sync
	if len(sourceCfg.Symlinks) == 0 && !sourceCfg.ShouldInitSubmodules() {
		result.NothingToSync = true
		c.Log.DebugContext(ctx, "nothing to sync",
			LogAttrKeyCategory.String(), LogCategorySync)
		return result, nil
	}

	// Resolve target worktrees
	targetWTs, err := c.resolveTargets(ctx, targets, sourceBranch, cwd, opts.All)
	if err != nil {
		return result, err
	}

	c.Log.DebugContext(ctx, "targets resolved",
		LogAttrKeyCategory.String(), LogCategorySync,
		"count", len(targetWTs))

	// Sync each target
	for _, wt := range targetWTs {
		c.Log.DebugContext(ctx, "syncing target",
			LogAttrKeyCategory.String(), LogCategorySync,
			"branch", wt.Branch,
			"path", wt.Path)

		targetResult := c.syncTarget(ctx, sourceWT.Path, wt, sourceCfg, opts)
		result.Targets = append(result.Targets, targetResult)

		c.Log.DebugContext(ctx, "target synced",
			LogAttrKeyCategory.String(), LogCategorySync,
			"branch", wt.Branch,
			"skipped", targetResult.Skipped,
			"error", targetResult.Err)
	}

	c.Log.DebugContext(ctx, "run completed",
		LogAttrKeyCategory.String(), LogCategorySync,
		"targetCount", len(result.Targets))

	return result, nil
}

// resolveSource resolves the source branch.
func resolveSource(source, defaultSource string) (string, error) {
	if source != "" {
		return source, nil
	}

	if defaultSource != "" {
		return defaultSource, nil
	}

	return "", fmt.Errorf("source branch not specified and no default_source configured\nhint: use --source <branch> or set default_source in .twig/settings.toml")
}

// resolveTargets resolves the list of target worktrees.
func (c *SyncCommand) resolveTargets(ctx context.Context, targets []string, sourceBranch, cwd string, all bool) ([]Worktree, error) {
	// Get all worktrees
	allWTs, err := c.Git.WorktreeList(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w", err)
	}

	// If --all, return all worktrees except main (first) and source
	if all {
		var result []Worktree
		for i, wt := range allWTs {
			// Skip main worktree (first one), bare, and source
			if i == 0 || wt.Bare || wt.Branch == sourceBranch {
				continue
			}
			result = append(result, wt)
		}
		return result, nil
	}

	// If no targets specified, use current worktree
	if len(targets) == 0 {
		// Find worktree containing cwd
		for _, wt := range allWTs {
			if strings.HasPrefix(cwd, wt.Path) {
				if wt.Branch == sourceBranch {
					return nil, fmt.Errorf("cannot sync source worktree to itself")
				}
				return []Worktree{wt}, nil
			}
		}
		return nil, fmt.Errorf("current directory is not in any worktree")
	}

	// Resolve specified targets
	var result []Worktree
	for _, target := range targets {
		wt, err := c.Git.WorktreeFindByBranch(ctx, target)
		if err != nil {
			return nil, fmt.Errorf("failed to find worktree for branch %q: %w", target, err)
		}
		if wt.Branch == sourceBranch {
			return nil, fmt.Errorf("cannot sync source worktree to itself: %s", target)
		}
		result = append(result, *wt)
	}
	return result, nil
}

// syncTarget syncs a single target worktree.
func (c *SyncCommand) syncTarget(ctx context.Context, sourcePath string, target Worktree, cfg *Config, opts SyncOptions) SyncTargetResult {
	result := SyncTargetResult{
		Branch:       target.Branch,
		WorktreePath: target.Path,
	}

	// Check if target is same as source
	if target.Path == sourcePath {
		result.Skipped = true
		result.SkipReason = "same as source"
		return result
	}

	// Sync symlinks
	if len(cfg.Symlinks) > 0 {
		if opts.Check {
			// In check mode, predict what would be created
			symlinks, err := c.predictSymlinks(sourcePath, target.Path, cfg.Symlinks, opts.Force)
			if err != nil {
				result.Err = err
				return result
			}
			result.Symlinks = symlinks
		} else {
			symlinks, err := createSymlinks(c.FS, sourcePath, target.Path, cfg.Symlinks, CreateSymlinksOptions{Force: opts.Force})
			if err != nil {
				result.Err = err
				return result
			}
			result.Symlinks = symlinks
		}
	}

	// Sync submodules
	if cfg.ShouldInitSubmodules() {
		if opts.Check {
			// In check mode, indicate submodules would be initialized
			result.SubmoduleInit.Attempted = true
		} else {
			wtGit := c.Git.InDir(target.Path)
			count, initErr := wtGit.SubmoduleUpdate(ctx)
			if initErr != nil {
				result.SubmoduleInit.Attempted = true
				result.SubmoduleInit.Skipped = true
				result.SubmoduleInit.Reason = initErr.Error()
			} else if count > 0 {
				result.SubmoduleInit.Attempted = true
				result.SubmoduleInit.Count = count
			}
		}
	}

	// Check if anything was synced
	createdSymlinks := 0
	for _, s := range result.Symlinks {
		if !s.Skipped {
			createdSymlinks++
		}
	}
	if createdSymlinks == 0 && !result.SubmoduleInit.Attempted {
		result.Skipped = true
		result.SkipReason = "up to date"
	}

	return result
}

// predictSymlinks predicts what symlinks would be created without actually creating them.
func (c *SyncCommand) predictSymlinks(srcDir, dstDir string, patterns []string, force bool) ([]SymlinkResult, error) {
	var results []SymlinkResult

	for _, pattern := range patterns {
		matches, err := c.FS.Glob(srcDir, pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid glob pattern %s: %w", pattern, err)
		}
		if len(matches) == 0 {
			results = append(results, SymlinkResult{
				Skipped: true,
				Reason:  fmt.Sprintf("%s does not match any files, skipping", pattern),
			})
			continue
		}

		for _, match := range matches {
			src := srcDir + "/" + match
			dst := dstDir + "/" + match

			// Check if destination already exists
			if info, err := c.FS.Lstat(dst); err == nil {
				isSymlink := info.Mode()&0o120000 != 0
				if force && isSymlink {
					// Would replace
					results = append(results, SymlinkResult{Src: src, Dst: dst})
				} else {
					// Would skip
					results = append(results, SymlinkResult{
						Src:     src,
						Dst:     dst,
						Skipped: true,
						Reason:  fmt.Sprintf("skipping symlink for %s (already exists)", match),
					})
				}
			} else {
				// Would create
				results = append(results, SymlinkResult{Src: src, Dst: dst})
			}
		}
	}

	return results, nil
}

// HasErrors returns true if any errors occurred.
func (r SyncResult) HasErrors() bool {
	for i := range r.Targets {
		if r.Targets[i].Err != nil {
			return true
		}
	}
	return false
}

// ErrorCount returns the number of failed targets.
func (r SyncResult) ErrorCount() int {
	count := 0
	for i := range r.Targets {
		if r.Targets[i].Err != nil {
			count++
		}
	}
	return count
}

// SuccessCount returns the number of successfully synced targets.
func (r SyncResult) SuccessCount() int {
	count := 0
	for i := range r.Targets {
		if r.Targets[i].Err == nil && !r.Targets[i].Skipped {
			count++
		}
	}
	return count
}

// SkippedCount returns the number of skipped targets.
func (r SyncResult) SkippedCount() int {
	count := 0
	for i := range r.Targets {
		if r.Targets[i].Skipped {
			count++
		}
	}
	return count
}

// SyncedBranches returns the list of successfully synced branch names.
func (r SyncResult) SyncedBranches() []string {
	var branches []string
	for i := range r.Targets {
		if r.Targets[i].Err == nil && !r.Targets[i].Skipped {
			branches = append(branches, r.Targets[i].Branch)
		}
	}
	slices.Sort(branches)
	return branches
}
