//go:build integration

package gwt

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/708u/gwt/internal/testutil"
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

		cmd := NewListCommand(mainDir)
		result, err := cmd.Run()
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

		cmd := NewListCommand(mainDir)
		result, err := cmd.Run()
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

	t.Run("FormatWithPath", func(t *testing.T) {
		t.Parallel()

		repoDir, mainDir := testutil.SetupTestRepo(t)

		wtPath := filepath.Join(repoDir, "feature", "test")
		testutil.RunGit(t, mainDir, "worktree", "add", "-b", "feature/test", wtPath)

		cmd := NewListCommand(mainDir)
		result, err := cmd.Run()
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Format with path
		formatted := result.Format(ListFormatOptions{ShowPath: true})

		// Should contain full paths
		if formatted.Stdout == "" {
			t.Error("formatted output should not be empty")
		}

		// Verify paths are absolute
		for _, wt := range result.Worktrees {
			if !filepath.IsAbs(wt.Path) {
				t.Errorf("path should be absolute: %s", wt.Path)
			}
		}
	})
}
