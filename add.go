package gwt

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func Add(name string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	config, err := LoadConfig(cwd)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	dirName := strings.ReplaceAll(name, "/", "-")
	worktreePath := filepath.Join(cwd, "..", dirName)

	if err := createWorktree(name, worktreePath); err != nil {
		return err
	}

	if err := createSymlinks(cwd, worktreePath, config.Include); err != nil {
		return err
	}

	fmt.Printf("Created worktree at %s\n", worktreePath)
	return nil
}

func createWorktree(branch, path string) error {
	cmd := exec.Command("git", "worktree", "add", "-b", branch, path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create worktree: %w", err)
	}

	return nil
}

func createSymlinks(srcDir, dstDir string, targets []string) error {
	for _, target := range targets {
		srcPath := filepath.Join(srcDir, target)
		dstPath := filepath.Join(dstDir, target)

		if _, err := os.Stat(srcPath); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "warning: %s does not exist, skipping\n", target)
			continue
		}

		if err := os.Symlink(srcPath, dstPath); err != nil {
			return fmt.Errorf("failed to create symlink for %s: %w", target, err)
		}

		fmt.Printf("Created symlink: %s -> %s\n", dstPath, srcPath)
	}

	return nil
}
