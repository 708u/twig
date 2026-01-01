package gwt

import (
	"fmt"
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
}

// NewAddCommand creates a new AddCommand with the given config and git runner.
func NewAddCommand(cfg *Config, git *GitRunner) *AddCommand {
	return &AddCommand{
		FS:     osFS{},
		Git:    git,
		Config: cfg,
	}
}

// Run creates a new worktree for the given branch name.
func (c *AddCommand) Run(name string) error {
	cwd, err := c.FS.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	dirName := strings.ReplaceAll(name, "/", "-")
	worktreePath := filepath.Join(cwd, "..", dirName)

	if err := c.createWorktree(name, worktreePath); err != nil {
		return err
	}

	if err := c.createSymlinks(cwd, worktreePath, c.Config.Include); err != nil {
		return err
	}

	fmt.Printf("Created worktree at %s\n", worktreePath)
	return nil
}

func (c *AddCommand) createWorktree(branch, path string) error {
	if _, err := c.FS.Stat(path); err == nil {
		return fmt.Errorf("directory already exists: %s", path)
	}

	if c.Git.BranchExists(branch) {
		branches, err := c.Git.WorktreeListBranches()
		if err != nil {
			return fmt.Errorf("failed to list worktree branches: %w", err)
		}
		if slices.Contains(branches, branch) {
			return fmt.Errorf("branch %s is already checked out in another worktree", branch)
		}
		if err := c.Git.WorktreeAdd(path, branch, false); err != nil {
			return fmt.Errorf("failed to create worktree: %w", err)
		}
	} else {
		if err := c.Git.WorktreeAdd(path, branch, true); err != nil {
			return fmt.Errorf("failed to create worktree: %w", err)
		}
	}

	return nil
}

func (c *AddCommand) createSymlinks(srcDir, dstDir string, targets []string) error {
	for _, target := range targets {
		srcPath := filepath.Join(srcDir, target)
		dstPath := filepath.Join(dstDir, target)

		if _, err := c.FS.Stat(srcPath); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "warning: %s does not exist, skipping\n", target)
			continue
		}

		if err := c.FS.Symlink(srcPath, dstPath); err != nil {
			return fmt.Errorf("failed to create symlink for %s: %w", target, err)
		}

		fmt.Printf("Created symlink: %s -> %s\n", dstPath, srcPath)
	}

	return nil
}
