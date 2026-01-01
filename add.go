package gwt

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
)

// AddCommand creates git worktrees with symlinks.
type AddCommand struct {
	FS     FileSystem
	Git    *GitRunner
	Config *Config
	Sync   bool
}

// AddOptions holds options for the add command.
type AddOptions struct {
	Sync bool
}

// NewAddCommand creates a new AddCommand with the given config.
func NewAddCommand(cfg *Config, opts AddOptions) *AddCommand {
	return &AddCommand{
		FS:     osFS{},
		Git:    NewGitRunner(cfg.WorktreeSourceDir),
		Config: cfg,
		Sync:   opts.Sync,
	}
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
	Branch        string
	WorktreePath  string
	Symlinks      []SymlinkResult
	GitOutput     []byte
	ChangesSynced bool
}

// Format formats the AddResult for display.
func (r AddResult) Format(opts FormatOptions) FormatResult {
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
	}

	var syncInfo string
	if r.ChangesSynced {
		syncInfo = ", synced"
	}
	stdout.WriteString(fmt.Sprintf("gwt add: %s (%d symlinks%s)\n", r.Branch, createdCount, syncInfo))

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

	// Check for changes and stash if sync is enabled
	var shouldSync bool
	if c.Sync {
		hasChanges, err := c.Git.HasChanges()
		if err != nil {
			return result, fmt.Errorf("failed to check for changes: %w", err)
		}
		shouldSync = hasChanges
		if shouldSync {
			if _, err := c.Git.StashPush("gwt sync"); err != nil {
				return result, fmt.Errorf("failed to stash changes: %w", err)
			}
		}
	}

	gitOutput, err := c.createWorktree(name, wtPath)
	if err != nil {
		if shouldSync {
			// Restore stash on worktree creation failure
			_, _ = c.Git.StashPop()
		}
		return result, err
	}
	result.GitOutput = gitOutput

	// Apply stash to new worktree if sync is enabled
	if shouldSync {
		if _, err := c.Git.InDir(wtPath).StashApply(); err != nil {
			// Rollback: remove worktree and restore stash
			_, _ = c.Git.WorktreeRemove(wtPath, WithForceRemove())
			_, _ = c.Git.StashPop()
			return result, fmt.Errorf("failed to apply changes to new worktree: %w", err)
		}
		// Restore stash in source
		if _, err := c.Git.StashPop(); err != nil {
			return result, fmt.Errorf("failed to restore changes in source: %w", err)
		}
		result.ChangesSynced = true
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
