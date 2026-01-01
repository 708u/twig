package gwt

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
)

// AddCommand creates git worktrees with symlinks.
type AddCommand struct {
	FS     FileSystem
	Git    *GitRunner
	Config *Config
	Stdout io.Writer
	Stderr io.Writer
}

// NewAddCommand creates a new AddCommand with the given config.
func NewAddCommand(cfg *Config) *AddCommand {
	return &AddCommand{
		FS:     osFS{},
		Git:    NewGitRunner(cfg.WorktreeSourceDir),
		Config: cfg,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
}

// Run creates a new worktree for the given branch name.
func (c *AddCommand) Run(name string) error {
	if name == "" {
		return fmt.Errorf("branch name is required")
	}

	srcDir := c.Config.WorktreeSourceDir
	if srcDir == "" {
		return fmt.Errorf("worktree source directory is not configured")
	}

	destBaseDir := c.Config.WorktreeDestBaseDir
	if destBaseDir == "" {
		repoName := filepath.Base(srcDir)
		destBaseDir = filepath.Join(srcDir, "..", repoName+"-worktree")
	}
	wtPath := filepath.Join(destBaseDir, name)

	if err := c.createWorktree(name, wtPath); err != nil {
		return err
	}

	if err := c.createSymlinks(srcDir, wtPath, c.Config.Symlinks); err != nil {
		return err
	}

	fmt.Fprintf(c.Stdout, "Created worktree at %s\n", wtPath)
	return nil
}

func (c *AddCommand) createWorktree(branch, path string) error {
	if _, err := c.FS.Stat(path); err == nil {
		return fmt.Errorf("directory already exists: %s", path)
	}

	var opts []WorktreeAddOption
	if c.Git.BranchExists(branch) {
		branches, err := c.Git.WorktreeListBranches()
		if err != nil {
			return fmt.Errorf("failed to list worktree branches: %w", err)
		}
		if slices.Contains(branches, branch) {
			return fmt.Errorf("branch %s is already checked out in another worktree", branch)
		}
	} else {
		opts = append(opts, WithCreateBranch())
	}

	if err := c.Git.WorktreeAdd(path, branch, opts...); err != nil {
		return fmt.Errorf("failed to create worktree: %w", err)
	}

	return nil
}

func (c *AddCommand) createSymlinks(srcDir, dstDir string, patterns []string) error {
	for _, pattern := range patterns {
		matches, err := c.FS.Glob(srcDir, pattern)
		if err != nil {
			return fmt.Errorf("invalid glob pattern %s: %w", pattern, err)
		}
		if len(matches) == 0 {
			fmt.Fprintf(c.Stderr, "warning: %s does not match any files, skipping\n", pattern)
			continue
		}

		for _, match := range matches {
			src := filepath.Join(srcDir, match)
			dst := filepath.Join(dstDir, match)

			if dir := filepath.Dir(dst); dir != dstDir {
				if err := c.FS.MkdirAll(dir, 0755); err != nil {
					return fmt.Errorf("failed to create directory for %s: %w", match, err)
				}
			}

			if err := c.FS.Symlink(src, dst); err != nil {
				return fmt.Errorf("failed to create symlink for %s: %w", match, err)
			}

			fmt.Fprintf(c.Stdout, "Created symlink: %s -> %s\n", dst, src)
		}
	}

	return nil
}
