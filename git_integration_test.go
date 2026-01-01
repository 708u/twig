//go:build integration

package gwt

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/708u/gwt/internal/testutil"
)

func TestGitRunner_WorktreeFindByBranch_Integration(t *testing.T) {
	t.Parallel()

	t.Run("Found", func(t *testing.T) {
		t.Parallel()

		repoDir, mainDir := testutil.SetupTestRepo(t)

		wtPath := filepath.Join(repoDir, "feature-wt")
		testutil.RunGit(t, mainDir, "worktree", "add", wtPath, "-b", "feature/test")

		runner := NewGitRunner(mainDir)

		got, err := runner.WorktreeFindByBranch("feature/test")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != wtPath {
			t.Errorf("got %q, want %q", got, wtPath)
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		t.Parallel()

		_, mainDir := testutil.SetupTestRepo(t)

		runner := NewGitRunner(mainDir)

		_, err := runner.WorktreeFindByBranch("nonexistent")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "not checked out in any worktree") {
			t.Errorf("error %q should contain 'not checked out in any worktree'", err.Error())
		}
	})
}

func TestGitRunner_WorktreeRemove_Integration(t *testing.T) {
	t.Parallel()

	t.Run("Success", func(t *testing.T) {
		t.Parallel()

		repoDir, mainDir := testutil.SetupTestRepo(t)

		wtPath := filepath.Join(repoDir, "to-remove")
		testutil.RunGit(t, mainDir, "worktree", "add", wtPath, "-b", "to-remove")

		runner := NewGitRunner(mainDir)

		_, err := runner.WorktreeRemove(wtPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		out := testutil.RunGit(t, mainDir, "worktree", "list")
		if strings.Contains(out, "to-remove") {
			t.Errorf("worktree should be removed, but still in list: %s", out)
		}
	})

	t.Run("NotExists", func(t *testing.T) {
		t.Parallel()

		_, mainDir := testutil.SetupTestRepo(t)

		runner := NewGitRunner(mainDir)

		_, err := runner.WorktreeRemove("/nonexistent/path")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "failed to remove worktree") {
			t.Errorf("error %q should contain 'failed to remove worktree'", err.Error())
		}
	})
}

func TestGitRunner_BranchDelete_Integration(t *testing.T) {
	t.Parallel()

	t.Run("SafeDelete", func(t *testing.T) {
		t.Parallel()

		_, mainDir := testutil.SetupTestRepo(t)

		testutil.RunGit(t, mainDir, "branch", "to-delete")

		runner := NewGitRunner(mainDir)

		_, err := runner.BranchDelete("to-delete")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		out := testutil.RunGit(t, mainDir, "branch", "--list")
		if strings.Contains(out, "to-delete") {
			t.Errorf("branch should be deleted, but still in list: %s", out)
		}
	})

	t.Run("ForceDelete", func(t *testing.T) {
		t.Parallel()

		_, mainDir := testutil.SetupTestRepo(t)

		testutil.RunGit(t, mainDir, "checkout", "-b", "unmerged")
		testutil.RunGit(t, mainDir, "commit", "--allow-empty", "-m", "unmerged commit")
		testutil.RunGit(t, mainDir, "checkout", "main")

		runner := NewGitRunner(mainDir)

		_, err := runner.BranchDelete("unmerged", WithForceDelete())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		out := testutil.RunGit(t, mainDir, "branch", "--list")
		if strings.Contains(out, "unmerged") {
			t.Errorf("branch should be deleted, but still in list: %s", out)
		}
	})

	t.Run("NotExists", func(t *testing.T) {
		t.Parallel()

		_, mainDir := testutil.SetupTestRepo(t)

		runner := NewGitRunner(mainDir)

		_, err := runner.BranchDelete("nonexistent")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "failed to delete branch") {
			t.Errorf("error %q should contain 'failed to delete branch'", err.Error())
		}
	})
}
