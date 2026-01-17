//go:build integration

package twig

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/708u/twig/internal/testutil"
)

func TestRemoveCommand_Check_Integration(t *testing.T) {
	t.Parallel()

	t.Run("SkipsDirtySubmodule", func(t *testing.T) {
		t.Parallel()

		repoDir, mainDir := testutil.SetupTestRepo(t)

		// Create a submodule repository
		submoduleRepo := filepath.Join(repoDir, "submodule-repo")
		if err := os.MkdirAll(submoduleRepo, 0755); err != nil {
			t.Fatal(err)
		}
		testutil.RunGit(t, submoduleRepo, "init")
		testutil.RunGit(t, submoduleRepo, "config", "user.email", "test@example.com")
		testutil.RunGit(t, submoduleRepo, "config", "user.name", "Test")
		if err := os.WriteFile(filepath.Join(submoduleRepo, "file.txt"), []byte("content"), 0644); err != nil {
			t.Fatal(err)
		}
		testutil.RunGit(t, submoduleRepo, "add", ".")
		testutil.RunGit(t, submoduleRepo, "commit", "-m", "initial")

		// Create worktree and add submodule
		wtPath := filepath.Join(repoDir, "feature", "with-submodule")
		testutil.RunGit(t, mainDir, "worktree", "add", "-b", "feature/with-submodule", wtPath)
		testutil.RunGit(t, wtPath, "-c", "protocol.file.allow=always", "submodule", "add", submoduleRepo, "sub")
		testutil.RunGit(t, wtPath, "commit", "-m", "add submodule")

		// Make submodule dirty by advancing its commit (+ prefix = modified commit)
		// This creates a state where submodule is at a different commit than recorded
		submodulePath := filepath.Join(wtPath, "sub")
		if err := os.WriteFile(filepath.Join(submodulePath, "new.txt"), []byte("new"), 0644); err != nil {
			t.Fatal(err)
		}
		testutil.RunGit(t, submodulePath, "config", "user.email", "test@example.com")
		testutil.RunGit(t, submodulePath, "config", "user.name", "Test")
		testutil.RunGit(t, submodulePath, "add", ".")
		testutil.RunGit(t, submodulePath, "commit", "-m", "advance submodule")
		// Now submodule is at a different commit than recorded in parent (+ prefix)

		cfgResult, err := LoadConfig(mainDir)
		if err != nil {
			t.Fatal(err)
		}

		cmd := &RemoveCommand{
			FS:     osFS{},
			Git:    NewGitRunner(mainDir),
			Config: cfgResult.Config,
		}

		// Check should return SkipDirtySubmodule
		checkResult, err := cmd.Check(t.Context(), "feature/with-submodule", CheckOptions{
			Cwd: mainDir,
		})
		if err != nil {
			t.Fatalf("Check failed: %v", err)
		}

		if checkResult.CanRemove {
			t.Error("CanRemove should be false for dirty submodule")
		}
		if checkResult.SkipReason != SkipDirtySubmodule {
			t.Errorf("SkipReason = %v, want %v", checkResult.SkipReason, SkipDirtySubmodule)
		}

		// Run should also fail with SkipError
		_, err = cmd.Run(t.Context(), "feature/with-submodule", mainDir, RemoveOptions{})
		if err == nil {
			t.Fatal("expected error for dirty submodule")
		}

		var skipErr *SkipError
		if !errors.As(err, &skipErr) {
			t.Fatalf("expected SkipError, got %T: %v", err, err)
		}
		if skipErr.Reason != SkipDirtySubmodule {
			t.Errorf("SkipError.Reason = %v, want %v", skipErr.Reason, SkipDirtySubmodule)
		}
	})

	t.Run("ReturnsChangedFiles", func(t *testing.T) {
		t.Parallel()

		repoDir, mainDir := testutil.SetupTestRepo(t)

		wtPath := filepath.Join(repoDir, "feature", "changed-files-test")
		testutil.RunGit(t, mainDir, "worktree", "add", "-b", "feature/changed-files-test", wtPath)

		// Create a tracked file and commit it
		trackedFile := filepath.Join(wtPath, "tracked.txt")
		if err := os.WriteFile(trackedFile, []byte("initial content"), 0644); err != nil {
			t.Fatal(err)
		}
		testutil.RunGit(t, wtPath, "add", "tracked.txt")
		testutil.RunGit(t, wtPath, "commit", "-m", "add tracked file")

		// Modify the tracked file to create " M" status
		if err := os.WriteFile(trackedFile, []byte("modified content"), 0644); err != nil {
			t.Fatal(err)
		}

		// Create untracked file for "??" status
		untrackedFile := filepath.Join(wtPath, "untracked.txt")
		if err := os.WriteFile(untrackedFile, []byte("new content"), 0644); err != nil {
			t.Fatal(err)
		}

		cfgResult, err := LoadConfig(mainDir)
		if err != nil {
			t.Fatal(err)
		}

		cmd := &RemoveCommand{
			FS:     osFS{},
			Git:    NewGitRunner(mainDir),
			Config: cfgResult.Config,
		}

		// Test Check() returns ChangedFiles
		checkResult, err := cmd.Check(t.Context(), "feature/changed-files-test", CheckOptions{
			Cwd: mainDir,
		})
		if err != nil {
			t.Fatalf("Check failed: %v", err)
		}

		if len(checkResult.ChangedFiles) == 0 {
			t.Fatal("expected ChangedFiles to be populated")
		}

		// Verify specific files are in the list
		var foundUntracked, foundModified bool
		for _, f := range checkResult.ChangedFiles {
			if f.Path == "untracked.txt" && f.Status == "??" {
				foundUntracked = true
			}
			if f.Path == "tracked.txt" && strings.Contains(f.Status, "M") {
				foundModified = true
			}
		}
		if !foundUntracked {
			t.Error("expected untracked file in ChangedFiles")
		}
		if !foundModified {
			t.Error("expected modified file in ChangedFiles")
		}

		// Test Run() also returns ChangedFiles in result
		removeResult, err := cmd.Run(t.Context(), "feature/changed-files-test", mainDir, RemoveOptions{})
		if err == nil {
			t.Fatal("expected error for uncommitted changes")
		}

		// Even on error, ChangedFiles should be populated
		if len(removeResult.ChangedFiles) == 0 {
			t.Error("expected ChangedFiles to be populated in RemovedWorktree")
		}
	})
}
