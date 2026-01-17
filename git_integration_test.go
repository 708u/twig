//go:build integration

package twig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/708u/twig/internal/testutil"
)

func TestGitRunner_WorktreeFindByBranch_Integration(t *testing.T) {
	t.Parallel()

	t.Run("Found", func(t *testing.T) {
		t.Parallel()

		repoDir, mainDir := testutil.SetupTestRepo(t, testutil.WithoutSettings())

		wtPath := filepath.Join(repoDir, "feature-wt")
		testutil.RunGit(t, mainDir, "worktree", "add", wtPath, "-b", "feature/test")

		runner := NewGitRunner(mainDir)

		got, err := runner.WorktreeFindByBranch(t.Context(), "feature/test")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Path != wtPath {
			t.Errorf("got %q, want %q", got.Path, wtPath)
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		t.Parallel()

		_, mainDir := testutil.SetupTestRepo(t, testutil.WithoutSettings())

		runner := NewGitRunner(mainDir)

		_, err := runner.WorktreeFindByBranch(t.Context(), "nonexistent")
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

		repoDir, mainDir := testutil.SetupTestRepo(t, testutil.WithoutSettings())

		wtPath := filepath.Join(repoDir, "to-remove")
		testutil.RunGit(t, mainDir, "worktree", "add", wtPath, "-b", "to-remove")

		runner := NewGitRunner(mainDir)

		_, err := runner.WorktreeRemove(t.Context(), wtPath)
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

		_, mainDir := testutil.SetupTestRepo(t, testutil.WithoutSettings())

		runner := NewGitRunner(mainDir)

		_, err := runner.WorktreeRemove(t.Context(), "/nonexistent/path")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "failed to remove worktree") {
			t.Errorf("error %q should contain 'failed to remove worktree'", err.Error())
		}
	})
}

func TestGitRunner_ChangedFiles_Integration(t *testing.T) {
	t.Parallel()

	writeFile := func(t *testing.T, dir, name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("NoChanges", func(t *testing.T) {
		t.Parallel()

		_, mainDir := testutil.SetupTestRepo(t, testutil.WithoutSettings())

		runner := NewGitRunner(mainDir)

		files, err := runner.ChangedFiles(t.Context())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(files) != 0 {
			t.Errorf("expected empty list, got %v", files)
		}
	})

	t.Run("StagedFile", func(t *testing.T) {
		t.Parallel()

		_, mainDir := testutil.SetupTestRepo(t, testutil.WithoutSettings())

		writeFile(t, mainDir, "staged.txt", "content")
		testutil.RunGit(t, mainDir, "add", "staged.txt")

		runner := NewGitRunner(mainDir)

		files, err := runner.ChangedFiles(t.Context())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(files) != 1 || files[0].Path != "staged.txt" {
			t.Errorf("expected [staged.txt], got %v", files)
		}
	})

	t.Run("UnstagedFile", func(t *testing.T) {
		t.Parallel()

		_, mainDir := testutil.SetupTestRepo(t, testutil.WithoutSettings())

		// Create and commit a file first
		writeFile(t, mainDir, "tracked.txt", "original")
		testutil.RunGit(t, mainDir, "add", "tracked.txt")
		testutil.RunGit(t, mainDir, "commit", "-m", "add tracked file")

		// Modify it
		writeFile(t, mainDir, "tracked.txt", "modified")

		runner := NewGitRunner(mainDir)

		files, err := runner.ChangedFiles(t.Context())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(files) != 1 || files[0].Path != "tracked.txt" {
			t.Errorf("expected [tracked.txt], got %v", files)
		}
	})

	t.Run("UntrackedFile", func(t *testing.T) {
		t.Parallel()

		_, mainDir := testutil.SetupTestRepo(t, testutil.WithoutSettings())

		writeFile(t, mainDir, "untracked.txt", "content")

		runner := NewGitRunner(mainDir)

		files, err := runner.ChangedFiles(t.Context())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(files) != 1 || files[0].Path != "untracked.txt" {
			t.Errorf("expected [untracked.txt], got %v", files)
		}
	})

	t.Run("MixedChanges", func(t *testing.T) {
		t.Parallel()

		_, mainDir := testutil.SetupTestRepo(t, testutil.WithoutSettings())

		// Tracked and modified (commit first, then modify)
		writeFile(t, mainDir, "tracked.txt", "original")
		testutil.RunGit(t, mainDir, "add", "tracked.txt")
		testutil.RunGit(t, mainDir, "commit", "-m", "add tracked")
		writeFile(t, mainDir, "tracked.txt", "modified")

		// Staged (add after commit)
		writeFile(t, mainDir, "staged.txt", "content")
		testutil.RunGit(t, mainDir, "add", "staged.txt")

		// Untracked
		writeFile(t, mainDir, "untracked.txt", "content")

		runner := NewGitRunner(mainDir)

		files, err := runner.ChangedFiles(t.Context())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(files) != 3 {
			t.Errorf("expected 3 files, got %v", files)
		}
	})

	t.Run("RenamedFile", func(t *testing.T) {
		t.Parallel()

		_, mainDir := testutil.SetupTestRepo(t, testutil.WithoutSettings())

		// Create and commit a file
		writeFile(t, mainDir, "old.txt", "content")
		testutil.RunGit(t, mainDir, "add", "old.txt")
		testutil.RunGit(t, mainDir, "commit", "-m", "add old file")

		// Rename it
		testutil.RunGit(t, mainDir, "mv", "old.txt", "new.txt")

		runner := NewGitRunner(mainDir)

		files, err := runner.ChangedFiles(t.Context())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Should return the new name
		if len(files) != 1 || files[0].Path != "new.txt" {
			t.Errorf("expected [new.txt], got %v", files)
		}
	})
}

func TestGitRunner_BranchDelete_Integration(t *testing.T) {
	t.Parallel()

	t.Run("SafeDelete", func(t *testing.T) {
		t.Parallel()

		_, mainDir := testutil.SetupTestRepo(t, testutil.WithoutSettings())

		testutil.RunGit(t, mainDir, "branch", "to-delete")

		runner := NewGitRunner(mainDir)

		_, err := runner.BranchDelete(t.Context(), "to-delete")
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

		_, mainDir := testutil.SetupTestRepo(t, testutil.WithoutSettings())

		testutil.RunGit(t, mainDir, "checkout", "-b", "unmerged")
		testutil.RunGit(t, mainDir, "commit", "--allow-empty", "-m", "unmerged commit")
		testutil.RunGit(t, mainDir, "checkout", "main")

		runner := NewGitRunner(mainDir)

		_, err := runner.BranchDelete(t.Context(), "unmerged", WithForceDelete())
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

		_, mainDir := testutil.SetupTestRepo(t, testutil.WithoutSettings())

		runner := NewGitRunner(mainDir)

		_, err := runner.BranchDelete(t.Context(), "nonexistent")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "failed to delete branch") {
			t.Errorf("error %q should contain 'failed to delete branch'", err.Error())
		}
	})
}
