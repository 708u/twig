package twig

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"slices"
	"strings"
)

// AddCommand creates git worktrees with symlinks.
type AddCommand struct {
	FS             FileSystem
	Git            *GitRunner
	Config         *Config
	Log            *slog.Logger
	Sync           bool
	CarryFrom      string
	FilePatterns   []string
	Lock           bool
	LockReason     string
	InitSubmodules bool
}

// AddOptions holds options for the add command.
type AddOptions struct {
	Sync           bool
	CarryFrom      string   // empty: no carry, non-empty: resolved path to carry from
	FilePatterns   []string // file patterns to carry (empty means all files)
	Lock           bool
	LockReason     string
	InitSubmodules bool
}

// NewAddCommand creates an AddCommand with explicit dependencies (for testing).
func NewAddCommand(fs FileSystem, git *GitRunner, cfg *Config, log *slog.Logger, opts AddOptions) *AddCommand {
	if log == nil {
		log = NewNopLogger()
	}
	return &AddCommand{
		FS:             fs,
		Git:            git,
		Config:         cfg,
		Log:            log,
		Sync:           opts.Sync,
		CarryFrom:      opts.CarryFrom,
		FilePatterns:   opts.FilePatterns,
		Lock:           opts.Lock,
		LockReason:     opts.LockReason,
		InitSubmodules: opts.InitSubmodules,
	}
}

// NewDefaultAddCommand creates an AddCommand with production defaults.
func NewDefaultAddCommand(cfg *Config, log *slog.Logger, opts AddOptions) *AddCommand {
	return NewAddCommand(osFS{}, NewGitRunner(cfg.WorktreeSourceDir, log), cfg, log, opts)
}

// SymlinkResult holds information about a symlink operation.
type SymlinkResult struct {
	Src     string
	Dst     string
	Skipped bool
	Reason  string
}

// SubmoduleInitResult holds information about submodule initialization.
type SubmoduleInitResult struct {
	Attempted bool   // true if initialization was attempted
	Count     int    // number of initialized submodules
	Skipped   bool   // true if initialization failed
	Reason    string // reason for failure (warning message)
}

// AddResult holds the result of an add operation.
type AddResult struct {
	Branch         string
	WorktreePath   string
	Symlinks       []SymlinkResult
	GitOutput      []byte
	ChangesSynced  bool
	ChangesCarried bool
	SubmoduleInit  SubmoduleInitResult
}

// AddFormatOptions configures add output formatting.
type AddFormatOptions struct {
	Verbose bool
	Quiet   bool
}

// Format formats the AddResult for display.
func (r AddResult) Format(opts AddFormatOptions) FormatResult {
	if opts.Quiet {
		return r.formatQuiet()
	}
	return r.formatDefault(opts)
}

// formatQuiet outputs only the worktree path.
func (r AddResult) formatQuiet() FormatResult {
	return FormatResult{Stdout: r.WorktreePath + "\n"}
}

// formatDefault outputs the default or verbose format.
func (r AddResult) formatDefault(opts AddFormatOptions) FormatResult {
	var stdout, stderr strings.Builder

	var createdCount int
	for _, s := range r.Symlinks {
		if s.Skipped {
			fmt.Fprintf(&stderr, "warning: %s\n", s.Reason)
		} else {
			createdCount++
		}
	}

	// Output submodule init warning
	if r.SubmoduleInit.Skipped {
		fmt.Fprintf(&stderr, "warning: %s\n", r.SubmoduleInit.Reason)
	}

	if opts.Verbose {
		if len(r.GitOutput) > 0 {
			stdout.Write(r.GitOutput)
		}
		fmt.Fprintf(&stdout, "Created worktree at %s\n", r.WorktreePath)
		for _, s := range r.Symlinks {
			if !s.Skipped {
				fmt.Fprintf(&stdout, "Created symlink: %s -> %s\n", s.Dst, s.Src)
			}
		}
		if r.ChangesSynced {
			stdout.WriteString("Synced uncommitted changes\n")
		}
		if r.ChangesCarried {
			stdout.WriteString("Carried uncommitted changes (source is now clean)\n")
		}
		if r.SubmoduleInit.Attempted && r.SubmoduleInit.Count > 0 {
			fmt.Fprintf(&stdout, "Initialized %d submodule(s)\n", r.SubmoduleInit.Count)
		}
	}

	var syncInfo string
	if r.ChangesSynced {
		syncInfo = ", synced"
	} else if r.ChangesCarried {
		syncInfo = ", carried"
	}

	var submoduleInfo string
	if r.SubmoduleInit.Attempted && r.SubmoduleInit.Count > 0 {
		submoduleInfo = fmt.Sprintf(", %d submodules", r.SubmoduleInit.Count)
	}
	fmt.Fprintf(&stdout, "twig add: %s (%d symlinks%s%s)\n", r.Branch, createdCount, syncInfo, submoduleInfo)

	return FormatResult{Stdout: stdout.String(), Stderr: stderr.String()}
}

// Run creates a new worktree for the given branch name.
func (c *AddCommand) Run(ctx context.Context, name string) (AddResult, error) {
	var result AddResult
	result.Branch = name

	if name == "" {
		return result, fmt.Errorf("branch name is required")
	}

	if c.Config.WorktreeSourceDir == "" {
		return result, fmt.Errorf("worktree source directory is not configured")
	}
	if c.Config.WorktreeDestBaseDir == "" {
		return result, fmt.Errorf("worktree destination base directory is not configured")
	}

	wtPath := filepath.Join(c.Config.WorktreeDestBaseDir, name)
	result.WorktreePath = wtPath

	// Determine stash mode and source
	var stashMsg string
	var isCarry bool
	var stashSourceGit *GitRunner
	if c.Sync {
		stashMsg = "twig sync"
		stashSourceGit = c.Git
	}
	if c.CarryFrom != "" {
		stashMsg = "twig carry"
		isCarry = true
		stashSourceGit = c.Git.InDir(c.CarryFrom)
	}

	// Stash changes if sync or carry is enabled
	var stashHash string
	if stashMsg != "" {
		hasChanges, err := stashSourceGit.HasChanges(ctx)
		if err != nil {
			return result, fmt.Errorf("failed to check for changes: %w", err)
		}
		if hasChanges {
			var pathspecs []string
			if len(c.FilePatterns) > 0 {
				// Expand glob patterns to actual file paths using doublestar
				globDir := c.Config.WorktreeSourceDir
				if isCarry {
					globDir = c.CarryFrom
				}
				seen := make(map[string]bool)
				for _, pattern := range c.FilePatterns {
					matches, err := c.FS.Glob(globDir, pattern)
					if err != nil {
						return result, fmt.Errorf("invalid glob pattern %q: %w", pattern, err)
					}
					for _, match := range matches {
						if !seen[match] {
							seen[match] = true
							pathspecs = append(pathspecs, match)
						}
					}
				}
			}
			hash, err := stashSourceGit.StashPush(ctx, stashMsg, pathspecs...)
			if err != nil {
				return result, fmt.Errorf("failed to stash changes: %w", err)
			}
			stashHash = hash
		}
	}

	gitOutput, err := c.createWorktree(ctx, name, wtPath)
	if err != nil {
		if stashHash != "" {
			_, _ = stashSourceGit.StashPopByHash(ctx, stashHash)
		}
		return result, err
	}
	result.GitOutput = gitOutput

	// Initialize submodules in new worktree (CLI flag forces enable)
	if c.InitSubmodules || c.Config.ShouldInitSubmodules() {
		wtGit := c.Git.InDir(wtPath)
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

	// Apply stashed changes to new worktree
	if stashHash != "" {
		_, err = c.Git.InDir(wtPath).StashApplyByHash(ctx, stashHash)
		if err != nil {
			_, _ = c.Git.WorktreeRemove(ctx, wtPath, WithForceRemove(WorktreeForceLevelUnclean))
			_, _ = stashSourceGit.StashPopByHash(ctx, stashHash)
			return result, fmt.Errorf("failed to apply changes to new worktree: %w", err)
		}
		if isCarry {
			// Carry: drop stash (source becomes clean)
			_, _ = stashSourceGit.StashDropByHash(ctx, stashHash)
			result.ChangesCarried = true
		} else {
			// Sync: restore stash in source (both have changes)
			_, err = stashSourceGit.StashPopByHash(ctx, stashHash)
			if err != nil {
				return result, fmt.Errorf("failed to restore changes in source: %w", err)
			}
			result.ChangesSynced = true
		}
	}

	symlinks, err := c.createSymlinks(
		c.Config.WorktreeSourceDir, wtPath, c.Config.Symlinks)
	if err != nil {
		return result, err
	}
	result.Symlinks = symlinks

	return result, nil
}

func (c *AddCommand) createWorktree(ctx context.Context, branch, path string) ([]byte, error) {
	if _, err := c.FS.Stat(path); err == nil {
		return nil, fmt.Errorf("directory already exists: %s", path)
	}

	var opts []WorktreeAddOption
	exists, err := c.Git.LocalBranchExists(ctx, branch)
	if err != nil {
		return nil, fmt.Errorf("failed to check branch existence: %w", err)
	}
	if exists {
		var branches []string
		branches, err = c.Git.WorktreeListBranches(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list worktree branches: %w", err)
		}
		if slices.Contains(branches, branch) {
			return nil, fmt.Errorf("branch %s is already checked out in another worktree", branch)
		}
	} else {
		var remote string
		remote, err = c.Git.FindRemoteForBranch(ctx, branch)
		if err != nil {
			return nil, err
		}

		if remote != "" {
			// Remote branch found, fetch it
			err = c.Git.Fetch(ctx, remote, branch)
			if err != nil {
				return nil, fmt.Errorf("failed to fetch %s from %s: %w", branch, remote, err)
			}
			// After fetch, git worktree add will auto-track the remote branch
		} else {
			// No remote branch found, create new local branch
			opts = append(opts, WithCreateBranch())
		}
	}

	if c.Lock {
		opts = append(opts, WithLock())
		if c.LockReason != "" {
			opts = append(opts, WithLockReason(c.LockReason))
		}
	}

	output, err := c.Git.WorktreeAdd(ctx, path, branch, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create worktree: %w", err)
	}

	return output, nil
}

func (c *AddCommand) createSymlinks(
	srcDir, dstDir string, patterns []string) ([]SymlinkResult, error) {
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
			src := filepath.Join(srcDir, match)
			dst := filepath.Join(dstDir, match)

			// Skip if destination already exists (e.g., git-tracked file checked out by worktree).
			if _, err := c.FS.Stat(dst); err == nil {
				results = append(results, SymlinkResult{
					Src:     src,
					Dst:     dst,
					Skipped: true,
					Reason:  fmt.Sprintf("skipping symlink for %s (already exists)", match),
				})
				continue
			}

			if dir := filepath.Dir(dst); dir != dstDir {
				if err := c.FS.MkdirAll(dir, 0755); err != nil {
					return nil, fmt.Errorf("failed to create directory for %s: %w", match, err)
				}
			}

			if err := c.FS.Symlink(src, dst); err != nil {
				return nil, fmt.Errorf("failed to create symlink for %s: %w", match, err)
			}

			results = append(results, SymlinkResult{Src: src, Dst: dst})
		}
	}

	return results, nil
}
