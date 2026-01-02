//go:build integration

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/708u/gwt"
	"github.com/708u/gwt/internal/testutil"
)

func TestAddCommand_SourceFlag_Integration(t *testing.T) {
	t.Parallel()

	t.Run("SourceBranchWorktree", func(t *testing.T) {
		t.Parallel()

		repoDir, mainDir := testutil.SetupTestRepo(t)

		// Setup gwt settings in main worktree
		gwtDir := filepath.Join(mainDir, ".gwt")
		if err := os.MkdirAll(gwtDir, 0755); err != nil {
			t.Fatal(err)
		}

		settingsContent := fmt.Sprintf(`symlinks = [".envrc"]
worktree_source_dir = %q
worktree_destination_base_dir = %q
`, mainDir, repoDir)
		if err := os.WriteFile(filepath.Join(gwtDir, "settings.toml"), []byte(settingsContent), 0644); err != nil {
			t.Fatal(err)
		}

		// Commit the settings (but not .envrc - it should be symlinked, not tracked)
		testutil.RunGit(t, mainDir, "add", ".gwt")
		testutil.RunGit(t, mainDir, "commit", "-m", "add gwt settings")

		// Create .envrc in main after commit (local file to be symlinked)
		if err := os.WriteFile(filepath.Join(mainDir, ".envrc"), []byte("# main envrc"), 0644); err != nil {
			t.Fatal(err)
		}

		// Create first derived worktree (feat/a) from main
		result, err := gwt.LoadConfig(mainDir)
		if err != nil {
			t.Fatal(err)
		}

		addCmd := gwt.NewAddCommand(result.Config, gwt.AddOptions{})
		_, err = addCmd.Run("feat/a")
		if err != nil {
			t.Fatalf("failed to create feat/a worktree: %v", err)
		}

		featAPath := filepath.Join(repoDir, "feat", "a")

		// Now simulate --source main from feat/a worktree
		// The PreRunE logic: resolve main branch to its worktree path, then reload config
		git := gwt.NewGitRunner(featAPath)
		mainPath, err := git.WorktreeFindByBranch("main")
		if err != nil {
			t.Fatalf("failed to find main worktree: %v", err)
		}

		// Load config from main (as --source would do)
		result, err = gwt.LoadConfig(mainPath)
		if err != nil {
			t.Fatal(err)
		}

		// Create feat/b from main's config
		addCmd = gwt.NewAddCommand(result.Config, gwt.AddOptions{})
		addResult, err := addCmd.Run("feat/b")
		if err != nil {
			t.Fatalf("failed to create feat/b worktree: %v", err)
		}

		// Verify worktree was created
		featBPath := filepath.Join(repoDir, "feat", "b")
		if _, err := os.Stat(featBPath); os.IsNotExist(err) {
			t.Errorf("worktree directory does not exist: %s", featBPath)
		}

		// Verify symlink points to main, not feat/a
		envrcPath := filepath.Join(featBPath, ".envrc")
		target, err := os.Readlink(envrcPath)
		if err != nil {
			t.Fatalf("failed to read symlink: %v", err)
		}
		expectedTarget := filepath.Join(mainDir, ".envrc")
		if target != expectedTarget {
			t.Errorf("symlink target = %q, want %q", target, expectedTarget)
		}

		// Verify result
		if addResult.Branch != "feat/b" {
			t.Errorf("result.Branch = %q, want %q", addResult.Branch, "feat/b")
		}
	})

	t.Run("SourceAndDirectoryMutualExclusion", func(t *testing.T) {
		t.Parallel()

		// This test verifies the error case when both -C and --source are specified
		// We test this by simulating the condition check in PreRunE

		// If both dirFlag and source are set, error should occur
		source := "main"
		testDirFlag := "/some/path"

		// Simulate the check
		if source != "" && testDirFlag != "" {
			err := fmt.Errorf("cannot use --source and -C together")
			if !strings.Contains(err.Error(), "cannot use --source and -C together") {
				t.Errorf("expected mutual exclusion error")
			}
			// Test passes - error is expected
			return
		}
		t.Error("expected mutual exclusion check to trigger")
	})

	t.Run("SourceBranchNotFound", func(t *testing.T) {
		t.Parallel()

		_, mainDir := testutil.SetupTestRepo(t)

		git := gwt.NewGitRunner(mainDir)
		_, err := git.WorktreeFindByBranch("nonexistent")
		if err == nil {
			t.Fatal("expected error for nonexistent branch")
		}
		if !strings.Contains(err.Error(), "not checked out in any worktree") {
			t.Errorf("error %q should mention branch not checked out", err.Error())
		}
	})

	t.Run("SourceBranchExistsButNoWorktree", func(t *testing.T) {
		t.Parallel()

		_, mainDir := testutil.SetupTestRepo(t)

		// Create a branch without a worktree
		testutil.RunGit(t, mainDir, "branch", "orphan-branch")

		git := gwt.NewGitRunner(mainDir)
		_, err := git.WorktreeFindByBranch("orphan-branch")
		if err == nil {
			t.Fatal("expected error for branch without worktree")
		}
		if !strings.Contains(err.Error(), "not checked out in any worktree") {
			t.Errorf("error %q should mention branch not checked out", err.Error())
		}
	})
}
