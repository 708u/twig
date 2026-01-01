package gwt

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
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
		Git:    NewGitRunner(),
		Config: cfg,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
}

// Run creates a new worktree for the given branch name.
func (c *AddCommand) Run(name string) error {
	cwd, err := c.FS.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// TODO: pathについて考える
	dirName := strings.ReplaceAll(name, "/", "-")
	// TODO: configから取るようにするか考える
	worktreePath := filepath.Join(cwd, "..", dirName)

	if err := c.createWorktree(name, worktreePath); err != nil {
		return err
	}

	if err := c.createSymlinks(cwd, worktreePath, c.Config.Include); err != nil {
		return err
	}

	fmt.Fprintf(c.Stdout, "Created worktree at %s\n", worktreePath)
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

func (c *AddCommand) createSymlinks(srcDir, dstDir string, targets []string) error {
	for _, target := range targets {
		srcPath := filepath.Join(srcDir, target)
		dstPath := filepath.Join(dstDir, target)

		if _, err := c.FS.Stat(srcPath); c.FS.IsNotExist(err) {
			fmt.Fprintf(c.Stderr, "warning: %s does not exist, skipping\n", target)
			continue
		}

		if err := c.FS.Symlink(srcPath, dstPath); err != nil {
			return fmt.Errorf("failed to create symlink for %s: %w", target, err)
		}

		fmt.Fprintf(c.Stdout, "Created symlink: %s -> %s\n", dstPath, srcPath)
	}

	return nil
}
