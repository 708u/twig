package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func setupRepo(outputDir string, numFiles, numCommits, numWorktrees int, merged bool) error {
	if err := os.RemoveAll(outputDir); err != nil {
		return fmt.Errorf("failed to clean output directory: %w", err)
	}

	mainDir := filepath.Join(outputDir, "main")
	if err := os.MkdirAll(mainDir, 0o755); err != nil {
		return fmt.Errorf("failed to create main directory: %w", err)
	}

	fmt.Printf("Setting up benchmark repository in %s\n", outputDir)
	fmt.Printf("  Files: %d, Commits: %d, Worktrees: %d, Merged: %v\n", numFiles, numCommits, numWorktrees, merged)

	if err := git(mainDir, "init"); err != nil {
		return fmt.Errorf("git init failed: %w", err)
	}

	if err := git(mainDir, "config", "user.email", "bench@example.com"); err != nil {
		return fmt.Errorf("git config email failed: %w", err)
	}

	if err := git(mainDir, "config", "user.name", "Benchmark"); err != nil {
		return fmt.Errorf("git config name failed: %w", err)
	}

	fmt.Printf("Generating %d files...\n", numFiles)
	if err := generateFiles(mainDir, numFiles); err != nil {
		return fmt.Errorf("failed to generate files: %w", err)
	}

	if err := git(mainDir, "add", "-A"); err != nil {
		return fmt.Errorf("git add failed: %w", err)
	}

	if err := git(mainDir, "commit", "-m", "Initial commit with files"); err != nil {
		return fmt.Errorf("git commit failed: %w", err)
	}

	if err := gitQuiet(mainDir, "branch", "-M", "main"); err != nil {
		return fmt.Errorf("git branch rename failed: %w", err)
	}

	fmt.Printf("Creating %d commits...\n", numCommits)
	if err := createCommits(mainDir, numCommits); err != nil {
		return fmt.Errorf("failed to create commits: %w", err)
	}

	if err := createTwigConfig(mainDir, outputDir); err != nil {
		return fmt.Errorf("failed to create twig config: %w", err)
	}

	if err := git(mainDir, "add", "-A"); err != nil {
		return fmt.Errorf("git add twig config failed: %w", err)
	}

	if err := git(mainDir, "commit", "-m", "Add twig configuration"); err != nil {
		return fmt.Errorf("git commit twig config failed: %w", err)
	}

	fmt.Printf("Creating %d worktrees...\n", numWorktrees)
	if err := createWorktrees(mainDir, outputDir, numWorktrees, merged); err != nil {
		return fmt.Errorf("failed to create worktrees: %w", err)
	}

	fmt.Printf("Benchmark repository ready at %s\n", outputDir)
	fmt.Printf("  Main worktree: %s\n", mainDir)
	fmt.Printf("  Worktrees: %d\n", numWorktrees)

	return nil
}

func git(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func gitQuiet(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	return cmd.Run()
}

func generateFiles(dir string, count int) error {
	dirs := []string{"src", "pkg", "internal", "cmd", "test", "docs", "config"}

	for i := range count {
		subdir := dirs[i%len(dirs)]
		subPath := filepath.Join(dir, subdir)

		if err := os.MkdirAll(subPath, 0o755); err != nil {
			return err
		}

		filename := filepath.Join(subPath, fmt.Sprintf("file_%05d.go", i))
		content := generateFileContent(i)

		if err := os.WriteFile(filename, []byte(content), 0o644); err != nil {
			return err
		}
	}

	return nil
}

func generateFileContent(index int) string {
	return fmt.Sprintf(`package bench

// File%d is a benchmark file
type File%d struct {
	ID      int
	Name    string
	Data    []byte
	Counter int
}

func NewFile%d() *File%d {
	return &File%d{
		ID:   %d,
		Name: "file_%05d",
	}
}

func (f *File%d) Process() error {
	f.Counter++
	return nil
}
`, index, index, index, index, index, index, index, index)
}

func createCommits(dir string, count int) error {
	changeFile := filepath.Join(dir, "CHANGELOG.md")

	for i := range count {
		content := fmt.Sprintf("# Changelog\n\n## Commit %d\n\n- Change %d\n", i, i)
		if err := os.WriteFile(changeFile, []byte(content), 0o644); err != nil {
			return err
		}

		if err := gitQuiet(dir, "add", "CHANGELOG.md"); err != nil {
			return err
		}

		msg := fmt.Sprintf("Commit %d: update changelog", i)
		if err := gitQuiet(dir, "commit", "-m", msg); err != nil {
			return err
		}
	}

	return nil
}

func createTwigConfig(mainDir, baseDir string) error {
	twigDir := filepath.Join(mainDir, ".twig")
	if err := os.MkdirAll(twigDir, 0o755); err != nil {
		return err
	}

	config := fmt.Sprintf(`worktree_destination_base_dir = "%s"
default_source = "main"
symlinks = []
`, baseDir)

	return os.WriteFile(filepath.Join(twigDir, "settings.toml"), []byte(config), 0o644)
}

func createWorktrees(mainDir, baseDir string, count int, merged bool) error {
	for i := range count {
		branchName := fmt.Sprintf("bench/wt-%d", i)
		worktreePath := filepath.Join(baseDir, "bench", fmt.Sprintf("wt-%d", i))

		if err := gitQuiet(mainDir, "branch", branchName); err != nil {
			return fmt.Errorf("failed to create branch %s: %w", branchName, err)
		}

		if err := gitQuiet(mainDir, "worktree", "add", worktreePath, branchName); err != nil {
			return fmt.Errorf("failed to create worktree %s: %w", worktreePath, err)
		}

		if merged {
			changeFile := filepath.Join(worktreePath, fmt.Sprintf("feature_%d.txt", i))
			content := fmt.Sprintf("Feature %d content\n", i)

			if err := os.WriteFile(changeFile, []byte(content), 0o644); err != nil {
				return err
			}

			if err := gitQuiet(worktreePath, "add", "-A"); err != nil {
				return err
			}

			if err := gitQuiet(worktreePath, "commit", "-m", fmt.Sprintf("Add feature %d", i)); err != nil {
				return err
			}

			if err := gitQuiet(mainDir, "merge", "--no-ff", "-m", fmt.Sprintf("Merge branch %s", branchName), branchName); err != nil {
				if err := gitQuiet(mainDir, "merge", "--no-ff", "--allow-unrelated-histories", "-m", fmt.Sprintf("Merge branch %s", branchName), branchName); err != nil {
					return fmt.Errorf("failed to merge branch %s: %w", branchName, err)
				}
			}
		}
	}

	return nil
}
