package twig

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
)

// AddCommand creates git worktrees with symlinks.
type AddCommand struct {
	FS         FileSystem
	Git        *GitRunner
	Config     *Config
	Sync       bool
	CarryFrom  string
	CarryFiles []string
	Lock       bool
	LockReason string
}

// AddOptions holds options for the add command.
type AddOptions struct {
	Sync       bool
	CarryFrom  string   // empty: no carry, non-empty: resolved path to carry from
	CarryFiles []string // file patterns to carry (empty means all files)
	Lock       bool
	LockReason string
}

// NewAddCommand creates an AddCommand with explicit dependencies (for testing).
func NewAddCommand(fs FileSystem, git *GitRunner, cfg *Config, opts AddOptions) *AddCommand {
	return &AddCommand{
		FS:         fs,
		Git:        git,
		Config:     cfg,
		Sync:       opts.Sync,
		CarryFrom:  opts.CarryFrom,
		CarryFiles: opts.CarryFiles,
		Lock:       opts.Lock,
		LockReason: opts.LockReason,
	}
}

// NewDefaultAddCommand creates an AddCommand with production defaults.
func NewDefaultAddCommand(cfg *Config, opts AddOptions) *AddCommand {
	return NewAddCommand(osFS{}, NewGitRunner(cfg.WorktreeSourceDir), cfg, opts)
}

// SymlinkResult holds information about a symlink operation.
type SymlinkResult struct {
	Src     string
	Dst     string
	Skipped bool
	Reason  string
}

// AddResult holds the result of an add operation.
type AddResult struct {
	Branch         string
	WorktreePath   string
	Symlinks       []SymlinkResult
	GitOutput      []byte
	ChangesSynced  bool
	ChangesCarried bool
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
			stderr.WriteString(fmt.Sprintf("warning: %s\n", s.Reason))
		} else {
			createdCount++
		}
	}

	if opts.Verbose {
		if len(r.GitOutput) > 0 {
			stdout.Write(r.GitOutput)
		}
		stdout.WriteString(fmt.Sprintf("Created worktree at %s\n", r.WorktreePath))
		for _, s := range r.Symlinks {
			if !s.Skipped {
				stdout.WriteString(fmt.Sprintf("Created symlink: %s -> %s\n", s.Dst, s.Src))
			}
		}
		if r.ChangesSynced {
			stdout.WriteString("Synced uncommitted changes\n")
		}
		if r.ChangesCarried {
			stdout.WriteString("Carried uncommitted changes (source is now clean)\n")
		}
	}

	var syncInfo string
	if r.ChangesSynced {
		syncInfo = ", synced"
	} else if r.ChangesCarried {
		syncInfo = ", carried"
	}
	stdout.WriteString(fmt.Sprintf("twig add: %s (%d symlinks%s)\n", r.Branch, createdCount, syncInfo))

	return FormatResult{Stdout: stdout.String(), Stderr: stderr.String()}
}

// Run creates a new worktree for the given branch name.
func (c *AddCommand) Run(name string) (AddResult, error) {
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
		hasChanges, err := stashSourceGit.HasChanges()
		if err != nil {
			return result, fmt.Errorf("failed to check for changes: %w", err)
		}
		if hasChanges {
			// CarryFiles only applies to carry mode, not sync
			var pathspecs []string
			if isCarry && len(c.CarryFiles) > 0 {
				// Expand glob patterns to actual file paths using doublestar
				seen := make(map[string]bool)
				for _, pattern := range c.CarryFiles {
					matches, err := c.FS.Glob(c.CarryFrom, pattern)
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
			hash, err := stashSourceGit.StashPush(stashMsg, pathspecs...)
			if err != nil {
				return result, fmt.Errorf("failed to stash changes: %w", err)
			}
			stashHash = hash
		}
	}

	gitOutput, err := c.createWorktree(name, wtPath)
	if err != nil {
		if stashHash != "" {
			_, _ = stashSourceGit.StashPopByHash(stashHash)
		}
		return result, err
	}
	result.GitOutput = gitOutput

	// Apply stashed changes to new worktree
	if stashHash != "" {
		if _, err := c.Git.InDir(wtPath).StashApplyByHash(stashHash); err != nil {
			_, _ = c.Git.WorktreeRemove(wtPath, WithForceRemove(WorktreeForceLevelUnclean))
			_, _ = stashSourceGit.StashPopByHash(stashHash)
			return result, fmt.Errorf("failed to apply changes to new worktree: %w", err)
		}
		if isCarry {
			// Carry: drop stash (source becomes clean)
			_, _ = stashSourceGit.StashDropByHash(stashHash)
			result.ChangesCarried = true
		} else {
			// Sync: restore stash in source (both have changes)
			if _, err := stashSourceGit.StashPopByHash(stashHash); err != nil {
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

func (c *AddCommand) createWorktree(branch, path string) ([]byte, error) {
	if _, err := c.FS.Stat(path); err == nil {
		return nil, fmt.Errorf("directory already exists: %s", path)
	}

	var opts []WorktreeAddOption
	if c.Git.BranchExists(branch) {
		branches, err := c.Git.WorktreeListBranches()
		if err != nil {
			return nil, fmt.Errorf("failed to list worktree branches: %w", err)
		}
		if slices.Contains(branches, branch) {
			return nil, fmt.Errorf("branch %s is already checked out in another worktree", branch)
		}
	} else {
		opts = append(opts, WithCreateBranch())
	}

	if c.Lock {
		opts = append(opts, WithLock())
		if c.LockReason != "" {
			opts = append(opts, WithLockReason(c.LockReason))
		}
	}

	output, err := c.Git.WorktreeAdd(path, branch, opts...)
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
