//go:build integration

package twig

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/708u/twig/internal/testutil"
)

func TestListCommand_Integration(t *testing.T) {
	t.Parallel()

	t.Run("ListsAllWorktrees", func(t *testing.T) {
		t.Parallel()

		repoDir, mainDir := testutil.SetupTestRepo(t)

		// Create additional worktrees
		wtPathA := filepath.Join(repoDir, "feature", "a")
		wtPathB := filepath.Join(repoDir, "feature", "b")
		testutil.RunGit(t, mainDir, "worktree", "add", "-b", "feature/a", wtPathA)
		testutil.RunGit(t, mainDir, "worktree", "add", "-b", "feature/b", wtPathB)

		cmd := NewDefaultListCommand(mainDir, NewNopLogger())
		result, err := cmd.Run(t.Context(), ListOptions{})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Should have 3 worktrees: main + 2 feature branches
		if len(result.Worktrees) != 3 {
			t.Errorf("expected 3 worktrees, got %d", len(result.Worktrees))
		}

		// Verify main worktree is included
		var branches []string
		for _, wt := range result.Worktrees {
			branches = append(branches, wt.Branch)
		}

		if !slices.Contains(branches, "main") {
			t.Error("main worktree should be included")
		}
		if !slices.Contains(branches, "feature/a") {
			t.Error("feature/a worktree should be included")
		}
		if !slices.Contains(branches, "feature/b") {
			t.Error("feature/b worktree should be included")
		}
	})

	t.Run("ListsSingleWorktree", func(t *testing.T) {
		t.Parallel()

		_, mainDir := testutil.SetupTestRepo(t)

		cmd := NewDefaultListCommand(mainDir, NewNopLogger())
		result, err := cmd.Run(t.Context(), ListOptions{})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Should have only the main worktree
		if len(result.Worktrees) != 1 {
			t.Errorf("expected 1 worktree, got %d", len(result.Worktrees))
		}

		if result.Worktrees[0].Branch != "main" {
			t.Errorf("expected main branch, got %q", result.Worktrees[0].Branch)
		}

		if result.Worktrees[0].Path != mainDir {
			t.Errorf("expected path %q, got %q", mainDir, result.Worktrees[0].Path)
		}
	})

	t.Run("FormatGitWorktreeListCompatible", func(t *testing.T) {
		t.Parallel()

		repoDir, mainDir := testutil.SetupTestRepo(t)

		wtPath := filepath.Join(repoDir, "feature", "test")
		testutil.RunGit(t, mainDir, "worktree", "add", "-b", "feature/test", wtPath)

		cmd := NewDefaultListCommand(mainDir, NewNopLogger())
		result, err := cmd.Run(t.Context(), ListOptions{})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		formatted := result.Format(FormatOptions{})

		// Should contain full paths and branch names
		if formatted.Stdout == "" {
			t.Error("formatted output should not be empty")
		}

		// Verify format contains path, hash, and branch
		lines := strings.Split(strings.TrimSpace(formatted.Stdout), "\n")
		for _, line := range lines {
			// Format: /path/to/worktree  abc1234 [branch]
			if !strings.Contains(line, "[") || !strings.Contains(line, "]") {
				t.Errorf("line should contain branch in brackets: %s", line)
			}
		}

		// Verify paths are absolute
		for _, wt := range result.Worktrees {
			if !filepath.IsAbs(wt.Path) {
				t.Errorf("path should be absolute: %s", wt.Path)
			}
		}
	})

	t.Run("WorktreeHasHEAD", func(t *testing.T) {
		t.Parallel()

		_, mainDir := testutil.SetupTestRepo(t)

		cmd := NewDefaultListCommand(mainDir, NewNopLogger())
		result, err := cmd.Run(t.Context(), ListOptions{})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		for _, wt := range result.Worktrees {
			if wt.HEAD == "" {
				t.Error("HEAD should not be empty")
			}
			if len(wt.HEAD) != 40 {
				t.Errorf("HEAD should be 40 characters, got %d: %s", len(wt.HEAD), wt.HEAD)
			}
		}
	})

	t.Run("QuietFormatOutputsPathsOnly", func(t *testing.T) {
		t.Parallel()

		repoDir, mainDir := testutil.SetupTestRepo(t)

		wtPath := filepath.Join(repoDir, "feature", "quiet-test")
		testutil.RunGit(t, mainDir, "worktree", "add", "-b", "feature/quiet-test", wtPath)

		cmd := NewDefaultListCommand(mainDir, NewNopLogger())
		result, err := cmd.Run(t.Context(), ListOptions{})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		formatted := result.Format(FormatOptions{Quiet: true})

		lines := strings.Split(strings.TrimSpace(formatted.Stdout), "\n")

		// Should have 2 worktrees
		if len(lines) != 2 {
			t.Errorf("expected 2 lines, got %d: %v", len(lines), lines)
		}

		// Each line should be just a path (no brackets, no hash)
		for _, line := range lines {
			if strings.Contains(line, "[") || strings.Contains(line, "]") {
				t.Errorf("quiet output should not contain brackets: %s", line)
			}
			if !filepath.IsAbs(line) {
				t.Errorf("quiet output should be absolute path: %s", line)
			}
		}

		// Verify paths match worktree paths
		for _, wt := range result.Worktrees {
			if !slices.Contains(lines, wt.Path) {
				t.Errorf("quiet output should contain path %s", wt.Path)
			}
		}
	})

	t.Run("VerboseShowsChangedFiles", func(t *testing.T) {
		t.Parallel()

		repoDir, mainDir := testutil.SetupTestRepo(t)

		// Create a worktree with uncommitted changes
		wtPath := filepath.Join(repoDir, "feature", "verbose-test")
		testutil.RunGit(t, mainDir, "worktree", "add", "-b", "feature/verbose-test", wtPath)

		// Create uncommitted changes in the worktree
		if err := os.WriteFile(filepath.Join(wtPath, "new-file.txt"), []byte("new"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(wtPath, "initial.txt"), []byte("modified"), 0644); err != nil {
			t.Fatal(err)
		}

		cmd := NewDefaultListCommand(mainDir, NewNopLogger())
		result, err := cmd.Run(t.Context(), ListOptions{Verbose: true})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Find the verbose-test worktree
		var verboseWT *ListWorktreeInfo
		for i, wt := range result.Worktrees {
			if wt.Branch == "feature/verbose-test" {
				verboseWT = &result.Worktrees[i]
				break
			}
		}

		if verboseWT == nil {
			t.Fatal("feature/verbose-test worktree not found")
		}

		if len(verboseWT.ChangedFiles) == 0 {
			t.Error("expected changed files in verbose mode")
		}

		// Verify format includes changed files
		formatted := result.Format(FormatOptions{Verbose: true})
		if !strings.Contains(formatted.Stdout, "new-file.txt") {
			t.Errorf("verbose output should contain new-file.txt, got %q", formatted.Stdout)
		}
	})

	t.Run("VerboseShowsLockReason", func(t *testing.T) {
		t.Parallel()

		repoDir, mainDir := testutil.SetupTestRepo(t)

		// Create a locked worktree
		wtPath := filepath.Join(repoDir, "feature", "locked-test")
		testutil.RunGit(t, mainDir, "worktree", "add", "-b", "feature/locked-test", wtPath)
		testutil.RunGit(t, mainDir, "worktree", "lock", "--reason", "USB drive work", wtPath)

		cmd := NewDefaultListCommand(mainDir, NewNopLogger())
		result, err := cmd.Run(t.Context(), ListOptions{Verbose: true})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Find the locked worktree
		var lockedWT *ListWorktreeInfo
		for i, wt := range result.Worktrees {
			if wt.Branch == "feature/locked-test" {
				lockedWT = &result.Worktrees[i]
				break
			}
		}

		if lockedWT == nil {
			t.Fatal("feature/locked-test worktree not found")
		}

		if !lockedWT.Locked {
			t.Error("worktree should be locked")
		}

		if lockedWT.LockReason != "USB drive work" {
			t.Errorf("lock reason = %q, want %q", lockedWT.LockReason, "USB drive work")
		}

		// Verify format includes lock reason
		formatted := result.Format(FormatOptions{Verbose: true})
		if !strings.Contains(formatted.Stdout, "lock reason: USB drive work") {
			t.Errorf("verbose output should contain lock reason, got %q", formatted.Stdout)
		}
	})

	t.Run("VerboseCleanWorktreeNoDetailLines", func(t *testing.T) {
		t.Parallel()

		_, mainDir := testutil.SetupTestRepo(t)

		// Commit .twig/ directory so it doesn't appear as untracked
		testutil.RunGit(t, mainDir, "add", ".twig")
		testutil.RunGit(t, mainDir, "commit", "-m", "add twig settings")

		cmd := NewDefaultListCommand(mainDir, NewNopLogger())
		result, err := cmd.Run(t.Context(), ListOptions{Verbose: true})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Main worktree should have no changed files
		if len(result.Worktrees[0].ChangedFiles) != 0 {
			t.Errorf("clean worktree should have no changed files, got %d",
				len(result.Worktrees[0].ChangedFiles))
		}

		// Verbose format should be same as default for clean worktrees
		verboseFormatted := result.Format(FormatOptions{Verbose: true})
		defaultFormatted := result.Format(FormatOptions{})

		if verboseFormatted.Stdout != defaultFormatted.Stdout {
			t.Errorf("verbose output for clean worktree should match default\nverbose: %q\ndefault: %q",
				verboseFormatted.Stdout, defaultFormatted.Stdout)
		}
	})
}
