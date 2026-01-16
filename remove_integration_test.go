//go:build integration

package twig

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/708u/twig/internal/testutil"
)

func TestRemoveCommand_Integration(t *testing.T) {
	t.Parallel()

	t.Run("RemoveWorktreeAndBranch", func(t *testing.T) {
		t.Parallel()

		repoDir, mainDir := testutil.SetupTestRepo(t)

		wtPath := filepath.Join(repoDir, "feature", "to-remove")
		testutil.RunGit(t, mainDir, "worktree", "add", "-b", "feature/to-remove", wtPath)

		out := testutil.RunGit(t, mainDir, "worktree", "list")
		if !strings.Contains(out, "feature/to-remove") {
			t.Fatalf("worktree was not created: %s", out)
		}

		result, err := LoadConfig(mainDir)
		if err != nil {
			t.Fatal(err)
		}

		cmd := &RemoveCommand{
			FS:     osFS{},
			Git:    NewGitRunner(mainDir),
			Config: result.Config,
		}

		removeResult, err := cmd.Run("feature/to-remove", mainDir, RemoveOptions{})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
			t.Errorf("worktree directory should be removed: %s", wtPath)
		}

		out = testutil.RunGit(t, mainDir, "branch", "--list", "feature/to-remove")
		if strings.TrimSpace(out) != "" {
			t.Errorf("branch should be deleted, got: %s", out)
		}

		// Verify result
		if removeResult.Branch != "feature/to-remove" {
			t.Errorf("result.Branch = %q, want %q", removeResult.Branch, "feature/to-remove")
		}
	})

	t.Run("Check", func(t *testing.T) {
		t.Parallel()

		repoDir, mainDir := testutil.SetupTestRepo(t)

		wtPath := filepath.Join(repoDir, "feature", "dry-run-test")
		testutil.RunGit(t, mainDir, "worktree", "add", "-b", "feature/dry-run-test", wtPath)

		result, err := LoadConfig(mainDir)
		if err != nil {
			t.Fatal(err)
		}

		cmd := &RemoveCommand{
			FS:     osFS{},
			Git:    NewGitRunner(mainDir),
			Config: result.Config,
		}

		removeResult, err := cmd.Run("feature/dry-run-test", mainDir, RemoveOptions{Check: true})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		if _, err := os.Stat(wtPath); os.IsNotExist(err) {
			t.Errorf("worktree should still exist in dry-run mode")
		}

		out := testutil.RunGit(t, mainDir, "branch", "--list", "feature/dry-run-test")
		if strings.TrimSpace(out) == "" {
			t.Errorf("branch should still exist in dry-run mode")
		}

		// Verify result
		if !removeResult.Check {
			t.Error("result.Check should be true")
		}
	})

	t.Run("ForceRemoveWithUncommittedChanges", func(t *testing.T) {
		t.Parallel()

		repoDir, mainDir := testutil.SetupTestRepo(t)

		wtPath := filepath.Join(repoDir, "feature", "force-test")
		testutil.RunGit(t, mainDir, "worktree", "add", "-b", "feature/force-test", wtPath)

		uncommittedFile := filepath.Join(wtPath, "uncommitted.txt")
		if err := os.WriteFile(uncommittedFile, []byte("uncommitted changes"), 0644); err != nil {
			t.Fatal(err)
		}

		result, err := LoadConfig(mainDir)
		if err != nil {
			t.Fatal(err)
		}

		cmd := &RemoveCommand{
			FS:     osFS{},
			Git:    NewGitRunner(mainDir),
			Config: result.Config,
		}

		// First, verify removal without --force fails with SkipError
		_, err = cmd.Run("feature/force-test", mainDir, RemoveOptions{})
		if err == nil {
			t.Fatal("expected error for uncommitted changes without --force")
		}

		var skipErr *SkipError
		if !errors.As(err, &skipErr) {
			t.Fatalf("expected SkipError, got %T: %v", err, err)
		}
		if skipErr.Reason != SkipHasChanges {
			t.Errorf("SkipError.Reason = %v, want %v", skipErr.Reason, SkipHasChanges)
		}

		// Now verify -f (WorktreeForceLevelUnclean) succeeds for uncommitted changes
		_, err = cmd.Run("feature/force-test", mainDir, RemoveOptions{Force: WorktreeForceLevelUnclean})
		if err != nil {
			t.Fatalf("Run with force failed: %v", err)
		}

		if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
			t.Errorf("worktree directory should be removed: %s", wtPath)
		}
	})

	t.Run("ErrorWithHintForLockedWorktree", func(t *testing.T) {
		t.Parallel()

		repoDir, mainDir := testutil.SetupTestRepo(t)

		wtPath := filepath.Join(repoDir, "feature", "locked-test")
		testutil.RunGit(t, mainDir, "worktree", "add", "-b", "feature/locked-test", wtPath)

		// Lock the worktree
		testutil.RunGit(t, mainDir, "worktree", "lock", wtPath)

		result, err := LoadConfig(mainDir)
		if err != nil {
			t.Fatal(err)
		}

		cmd := &RemoveCommand{
			FS:     osFS{},
			Git:    NewGitRunner(mainDir),
			Config: result.Config,
		}

		// Removal without --force should fail with SkipError
		_, err = cmd.Run("feature/locked-test", mainDir, RemoveOptions{})
		if err == nil {
			t.Fatal("expected error for locked worktree without --force")
		}

		var skipErr *SkipError
		if !errors.As(err, &skipErr) {
			t.Fatalf("expected SkipError, got %T: %v", err, err)
		}
		if skipErr.Reason != SkipLocked {
			t.Errorf("SkipError.Reason = %v, want %v", skipErr.Reason, SkipLocked)
		}

		// Verify worktree is still locked
		out := testutil.RunGit(t, mainDir, "worktree", "list", "--porcelain")
		if !strings.Contains(out, "locked") {
			t.Fatalf("worktree should still be locked: %s", out)
		}

		// Verify -f (WorktreeForceLevelUnclean) still fails for locked worktree
		_, err = cmd.Run("feature/locked-test", mainDir, RemoveOptions{Force: WorktreeForceLevelUnclean})
		if err == nil {
			t.Fatal("expected error for locked worktree with single -f")
		}

		// Now verify -ff (WorktreeForceLevelLocked) removes the locked worktree
		_, err = cmd.Run("feature/locked-test", mainDir, RemoveOptions{Force: WorktreeForceLevelLocked})
		if err != nil {
			t.Fatalf("force remove of locked worktree with -ff failed: %v", err)
		}

		// Verify worktree is removed
		if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
			t.Errorf("locked worktree should be removed: %s", wtPath)
		}

		// Verify branch is deleted
		out = testutil.RunGit(t, mainDir, "branch", "--list", "feature/locked-test")
		if strings.TrimSpace(out) != "" {
			t.Errorf("branch should be deleted, got: %s", out)
		}
	})

	t.Run("ErrorWhenInsideWorktree", func(t *testing.T) {
		t.Parallel()

		repoDir, mainDir := testutil.SetupTestRepo(t)

		wtPath := filepath.Join(repoDir, "feature", "inside-test")
		testutil.RunGit(t, mainDir, "worktree", "add", "-b", "feature/inside-test", wtPath)

		result, err := LoadConfig(mainDir)
		if err != nil {
			t.Fatal(err)
		}

		cmd := &RemoveCommand{
			FS:     osFS{},
			Git:    NewGitRunner(mainDir),
			Config: result.Config,
		}

		_, err = cmd.Run("feature/inside-test", wtPath, RemoveOptions{})
		if err == nil {
			t.Fatal("expected error when inside worktree, got nil")
		}
		var skipErr *SkipError
		if !errors.As(err, &skipErr) {
			t.Fatalf("expected SkipError, got %T: %v", err, err)
		}
		if skipErr.Reason != SkipCurrentDir {
			t.Errorf("SkipError.Reason = %v, want %v", skipErr.Reason, SkipCurrentDir)
		}
	})

	t.Run("ErrorBranchNotInWorktree", func(t *testing.T) {
		t.Parallel()

		_, mainDir := testutil.SetupTestRepo(t)

		testutil.RunGit(t, mainDir, "branch", "orphan-branch")

		result, err := LoadConfig(mainDir)
		if err != nil {
			t.Fatal(err)
		}

		cmd := &RemoveCommand{
			FS:     osFS{},
			Git:    NewGitRunner(mainDir),
			Config: result.Config,
		}

		_, err = cmd.Run("orphan-branch", mainDir, RemoveOptions{})
		if err == nil {
			t.Fatal("expected error for branch not in worktree, got nil")
		}
		if !strings.Contains(err.Error(), "is not checked out in any worktree") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("RemoveMultipleWorktrees", func(t *testing.T) {
		t.Parallel()

		repoDir, mainDir := testutil.SetupTestRepo(t)

		// Create multiple worktrees
		branches := []string{"feature/multi-a", "feature/multi-b", "feature/multi-c"}
		wtPaths := make([]string, len(branches))
		for i, branch := range branches {
			wtPaths[i] = filepath.Join(repoDir, "feature", fmt.Sprintf("multi-%c", 'a'+i))
			testutil.RunGit(t, mainDir, "worktree", "add", "-b", branch, wtPaths[i])
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

		// Remove all worktrees
		var removeResult RemoveResult
		for _, branch := range branches {
			wt, err := cmd.Run(branch, mainDir, RemoveOptions{})
			if err != nil {
				wt.Branch = branch
				wt.Err = err
			}
			removeResult.Removed = append(removeResult.Removed, wt)
		}

		if removeResult.HasErrors() {
			t.Fatalf("unexpected errors: %v", removeResult.Removed)
		}

		if len(removeResult.Removed) != 3 {
			t.Errorf("expected 3 removed, got %d", len(removeResult.Removed))
		}

		// Verify all worktrees are removed
		for _, wtPath := range wtPaths {
			if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
				t.Errorf("worktree directory should be removed: %s", wtPath)
			}
		}

		// Verify all branches are deleted
		for _, branch := range branches {
			out := testutil.RunGit(t, mainDir, "branch", "--list", branch)
			if strings.TrimSpace(out) != "" {
				t.Errorf("branch should be deleted: %s", branch)
			}
		}
	})

	t.Run("RemoveMultipleWithPartialFailure", func(t *testing.T) {
		t.Parallel()

		repoDir, mainDir := testutil.SetupTestRepo(t)

		// Create one valid worktree
		validBranch := "feature/valid"
		validWtPath := filepath.Join(repoDir, "feature", "valid")
		testutil.RunGit(t, mainDir, "worktree", "add", "-b", validBranch, validWtPath)

		// Create a branch without worktree (will fail)
		invalidBranch := "feature/no-worktree"
		testutil.RunGit(t, mainDir, "branch", invalidBranch)

		cfgResult, err := LoadConfig(mainDir)
		if err != nil {
			t.Fatal(err)
		}

		cmd := &RemoveCommand{
			FS:     osFS{},
			Git:    NewGitRunner(mainDir),
			Config: cfgResult.Config,
		}

		// Try to remove both
		var removeResult RemoveResult
		for _, branch := range []string{validBranch, invalidBranch} {
			wt, err := cmd.Run(branch, mainDir, RemoveOptions{})
			if err != nil {
				wt.Branch = branch
				wt.Err = err
			}
			removeResult.Removed = append(removeResult.Removed, wt)
		}

		// Should have 2 entries (1 success, 1 error)
		if len(removeResult.Removed) != 2 {
			t.Errorf("expected 2 entries, got %d", len(removeResult.Removed))
		}
		if removeResult.ErrorCount() != 1 {
			t.Errorf("expected 1 error, got %d", removeResult.ErrorCount())
		}

		// Valid worktree should be removed
		if _, err := os.Stat(validWtPath); !os.IsNotExist(err) {
			t.Errorf("valid worktree should be removed: %s", validWtPath)
		}

		// First entry should be success, second should be error
		if removeResult.Removed[0].Err != nil {
			t.Errorf("first entry should be success, got error: %v", removeResult.Removed[0].Err)
		}
		if removeResult.Removed[1].Err == nil {
			t.Error("second entry should be error, got success")
		}
		if removeResult.Removed[1].Branch != invalidBranch {
			t.Errorf("error should be for %s, got %s", invalidBranch, removeResult.Removed[1].Branch)
		}
	})

	t.Run("CleanupEmptyParentDirs", func(t *testing.T) {
		t.Parallel()

		repoDir, mainDir := testutil.SetupTestRepo(t)

		// Create a deeply nested worktree (3 levels) to verify arbitrary depth cleanup
		wtPath := filepath.Join(repoDir, "feat", "nested", "very", "deep")
		testutil.RunGit(t, mainDir, "worktree", "add", "-b", "feat/nested/very/deep", wtPath)

		// Verify parent directories exist
		parentDir := filepath.Join(repoDir, "feat", "nested", "very")
		grandparentDir := filepath.Join(repoDir, "feat", "nested")
		greatGrandparentDir := filepath.Join(repoDir, "feat")
		if _, err := os.Stat(parentDir); os.IsNotExist(err) {
			t.Fatalf("parent directory should exist: %s", parentDir)
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

		removeResult, err := cmd.Run("feat/nested/very/deep", mainDir, RemoveOptions{})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Verify worktree is removed
		if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
			t.Errorf("worktree directory should be removed: %s", wtPath)
		}

		// Verify all 3 empty parent directories are removed
		if _, err := os.Stat(parentDir); !os.IsNotExist(err) {
			t.Errorf("empty parent directory should be removed: %s", parentDir)
		}
		if _, err := os.Stat(grandparentDir); !os.IsNotExist(err) {
			t.Errorf("empty grandparent directory should be removed: %s", grandparentDir)
		}
		if _, err := os.Stat(greatGrandparentDir); !os.IsNotExist(err) {
			t.Errorf("empty great-grandparent directory should be removed: %s", greatGrandparentDir)
		}

		// Verify CleanedDirs in result (3 directories cleaned)
		if len(removeResult.CleanedDirs) != 3 {
			t.Errorf("expected 3 cleaned dirs, got %d: %v", len(removeResult.CleanedDirs), removeResult.CleanedDirs)
		}
	})

	t.Run("PreserveNonEmptyParentDirs", func(t *testing.T) {
		t.Parallel()

		repoDir, mainDir := testutil.SetupTestRepo(t)

		// Create two worktrees in same parent
		wtPath1 := filepath.Join(repoDir, "feat", "test1")
		wtPath2 := filepath.Join(repoDir, "feat", "test2")
		testutil.RunGit(t, mainDir, "worktree", "add", "-b", "feat/test1", wtPath1)
		testutil.RunGit(t, mainDir, "worktree", "add", "-b", "feat/test2", wtPath2)

		parentDir := filepath.Join(repoDir, "feat")

		cfgResult, err := LoadConfig(mainDir)
		if err != nil {
			t.Fatal(err)
		}

		cmd := &RemoveCommand{
			FS:     osFS{},
			Git:    NewGitRunner(mainDir),
			Config: cfgResult.Config,
		}

		// Remove first worktree
		removeResult, err := cmd.Run("feat/test1", mainDir, RemoveOptions{})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// First worktree should be removed
		if _, err := os.Stat(wtPath1); !os.IsNotExist(err) {
			t.Errorf("first worktree should be removed: %s", wtPath1)
		}

		// Parent directory should still exist (has sibling worktree)
		if _, err := os.Stat(parentDir); os.IsNotExist(err) {
			t.Errorf("parent directory should still exist: %s", parentDir)
		}

		// No directories should be cleaned
		if len(removeResult.CleanedDirs) != 0 {
			t.Errorf("expected 0 cleaned dirs, got %d: %v", len(removeResult.CleanedDirs), removeResult.CleanedDirs)
		}

		// Now remove second worktree
		removeResult, err = cmd.Run("feat/test2", mainDir, RemoveOptions{})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Parent directory should now be removed
		if _, err := os.Stat(parentDir); !os.IsNotExist(err) {
			t.Errorf("parent directory should be removed after last worktree: %s", parentDir)
		}

		// One directory should be cleaned
		if len(removeResult.CleanedDirs) != 1 {
			t.Errorf("expected 1 cleaned dir, got %d: %v", len(removeResult.CleanedDirs), removeResult.CleanedDirs)
		}
	})

	t.Run("CheckShowsCleanupInfo", func(t *testing.T) {
		t.Parallel()

		repoDir, mainDir := testutil.SetupTestRepo(t)

		// Create a nested worktree
		wtPath := filepath.Join(repoDir, "feat", "dry-cleanup")
		testutil.RunGit(t, mainDir, "worktree", "add", "-b", "feat/dry-cleanup", wtPath)

		cfgResult, err := LoadConfig(mainDir)
		if err != nil {
			t.Fatal(err)
		}

		cmd := &RemoveCommand{
			FS:     osFS{},
			Git:    NewGitRunner(mainDir),
			Config: cfgResult.Config,
		}

		removeResult, err := cmd.Run("feat/dry-cleanup", mainDir, RemoveOptions{Check: true})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Worktree should still exist
		if _, err := os.Stat(wtPath); os.IsNotExist(err) {
			t.Errorf("worktree should still exist in dry-run: %s", wtPath)
		}

		// CleanedDirs should show predicted cleanup
		if len(removeResult.CleanedDirs) != 1 {
			t.Errorf("expected 1 predicted cleanup dir, got %d: %v", len(removeResult.CleanedDirs), removeResult.CleanedDirs)
		}

		// Format should include cleanup info
		formatted := removeResult.Format(FormatOptions{})
		if !strings.Contains(formatted.Stdout, "Would remove empty directory") {
			t.Errorf("dry-run output should include cleanup info, got: %s", formatted.Stdout)
		}
	})

	t.Run("RemovePrunableWorktree", func(t *testing.T) {
		t.Parallel()

		repoDir, mainDir := testutil.SetupTestRepo(t)

		// Create a worktree
		wtPath := filepath.Join(repoDir, "feature", "prunable-test")
		testutil.RunGit(t, mainDir, "worktree", "add", "-b", "feature/prunable-test", wtPath)

		// Verify worktree exists
		out := testutil.RunGit(t, mainDir, "worktree", "list")
		if !strings.Contains(out, "feature/prunable-test") {
			t.Fatalf("worktree was not created: %s", out)
		}

		// Delete worktree directory externally (simulate rm -rf)
		if err := os.RemoveAll(wtPath); err != nil {
			t.Fatalf("failed to remove worktree directory: %v", err)
		}

		// Verify directory is deleted
		if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
			t.Fatalf("worktree directory should be deleted")
		}

		// Verify worktree is now prunable
		out = testutil.RunGit(t, mainDir, "worktree", "list", "--porcelain")
		if !strings.Contains(out, "prunable") {
			t.Fatalf("worktree should be prunable: %s", out)
		}

		// Load config and run remove command
		cfgResult, err := LoadConfig(mainDir)
		if err != nil {
			t.Fatal(err)
		}

		cmd := &RemoveCommand{
			FS:     osFS{},
			Git:    NewGitRunner(mainDir),
			Config: cfgResult.Config,
		}

		removeResult, err := cmd.Run("feature/prunable-test", mainDir, RemoveOptions{})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Verify result indicates prunable
		if !removeResult.Pruned {
			t.Error("result.Prunable should be true")
		}

		// Verify branch is deleted
		out = testutil.RunGit(t, mainDir, "branch", "--list", "feature/prunable-test")
		if strings.TrimSpace(out) != "" {
			t.Errorf("branch should be deleted, got: %s", out)
		}

		// Verify worktree record is pruned
		out = testutil.RunGit(t, mainDir, "worktree", "list", "--porcelain")
		if strings.Contains(out, "feature/prunable-test") {
			t.Errorf("worktree record should be pruned: %s", out)
		}
	})

	t.Run("RemovePrunableWorktreeCheck", func(t *testing.T) {
		t.Parallel()

		repoDir, mainDir := testutil.SetupTestRepo(t)

		// Create and externally delete a worktree
		wtPath := filepath.Join(repoDir, "feature", "prunable-dry-run")
		testutil.RunGit(t, mainDir, "worktree", "add", "-b", "feature/prunable-dry-run", wtPath)
		if err := os.RemoveAll(wtPath); err != nil {
			t.Fatalf("failed to remove worktree directory: %v", err)
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

		removeResult, err := cmd.Run("feature/prunable-dry-run", mainDir, RemoveOptions{Check: true})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Verify result
		if !removeResult.Pruned {
			t.Error("result.Prunable should be true")
		}
		if !removeResult.Check {
			t.Error("result.Check should be true")
		}

		// Branch should still exist (dry-run)
		out := testutil.RunGit(t, mainDir, "branch", "--list", "feature/prunable-dry-run")
		if strings.TrimSpace(out) == "" {
			t.Error("branch should still exist in dry-run mode")
		}

		// Format should show appropriate dry-run message
		formatted := removeResult.Format(FormatOptions{})
		if !strings.Contains(formatted.Stdout, "Would prune stale worktree record") {
			t.Errorf("dry-run output should show prune message, got: %s", formatted.Stdout)
		}
		if !strings.Contains(formatted.Stdout, "Would delete branch") {
			t.Errorf("dry-run output should show branch deletion, got: %s", formatted.Stdout)
		}
	})

	t.Run("RemovePrunableWorktreeWithForce", func(t *testing.T) {
		t.Parallel()

		repoDir, mainDir := testutil.SetupTestRepo(t)

		// Create and externally delete a worktree
		wtPath := filepath.Join(repoDir, "feature", "prunable-force")
		testutil.RunGit(t, mainDir, "worktree", "add", "-b", "feature/prunable-force", wtPath)
		if err := os.RemoveAll(wtPath); err != nil {
			t.Fatalf("failed to remove worktree directory: %v", err)
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

		// Remove with force option
		removeResult, err := cmd.Run("feature/prunable-force", mainDir, RemoveOptions{
			Force: WorktreeForceLevelUnclean,
		})
		if err != nil {
			t.Fatalf("Run with force failed: %v", err)
		}

		// Verify result
		if !removeResult.Pruned {
			t.Error("result.Prunable should be true")
		}

		// Branch should be deleted
		out := testutil.RunGit(t, mainDir, "branch", "--list", "feature/prunable-force")
		if strings.TrimSpace(out) != "" {
			t.Errorf("branch should be deleted, got: %s", out)
		}
	})

	t.Run("RemoveWorktreeWithCleanSubmodule", func(t *testing.T) {
		t.Parallel()

		repoDir, mainDir := testutil.SetupTestRepo(t)

		// Create a submodule in the main repo
		submoduleDir := t.TempDir()
		testutil.RunGit(t, submoduleDir, "init")
		testutil.RunGit(t, submoduleDir, "config", "user.email", "test@example.com")
		testutil.RunGit(t, submoduleDir, "config", "user.name", "Test User")
		if err := os.WriteFile(filepath.Join(submoduleDir, "README.md"), []byte("# Submodule"), 0644); err != nil {
			t.Fatal(err)
		}
		testutil.RunGit(t, submoduleDir, "add", ".")
		testutil.RunGit(t, submoduleDir, "commit", "-m", "initial")

		// Add submodule to main repo (use -c to allow file protocol)
		testutil.RunGit(t, mainDir, "-c", "protocol.file.allow=always", "submodule", "add", submoduleDir, "vendor/lib")
		testutil.RunGit(t, mainDir, "commit", "-m", "add submodule")

		// Create a worktree
		wtPath := filepath.Join(repoDir, "feature", "with-submodule")
		testutil.RunGit(t, mainDir, "worktree", "add", "-b", "feature/with-submodule", wtPath)

		// Initialize submodule in the new worktree (use -c to allow file protocol)
		testutil.RunGit(t, wtPath, "-c", "protocol.file.allow=always", "submodule", "update", "--init", "--recursive")

		// Verify submodule is initialized
		out := testutil.RunGit(t, wtPath, "submodule", "status")
		if !strings.Contains(out, "vendor/lib") {
			t.Fatalf("submodule should be initialized: %s", out)
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

		// Remove without --force should succeed (clean submodule auto-forces)
		_, err = cmd.Run("feature/with-submodule", mainDir, RemoveOptions{})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Verify worktree is removed
		if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
			t.Errorf("worktree should be removed: %s", wtPath)
		}

		// Verify branch is deleted
		out = testutil.RunGit(t, mainDir, "branch", "--list", "feature/with-submodule")
		if strings.TrimSpace(out) != "" {
			t.Errorf("branch should be deleted, got: %s", out)
		}
	})

	t.Run("RemoveWorktreeWithDirtySubmodule", func(t *testing.T) {
		t.Parallel()

		repoDir, mainDir := testutil.SetupTestRepo(t)

		// Create a submodule in the main repo
		submoduleDir := t.TempDir()
		testutil.RunGit(t, submoduleDir, "init")
		testutil.RunGit(t, submoduleDir, "config", "user.email", "test@example.com")
		testutil.RunGit(t, submoduleDir, "config", "user.name", "Test User")
		if err := os.WriteFile(filepath.Join(submoduleDir, "README.md"), []byte("# Submodule"), 0644); err != nil {
			t.Fatal(err)
		}
		testutil.RunGit(t, submoduleDir, "add", ".")
		testutil.RunGit(t, submoduleDir, "commit", "-m", "initial")

		// Add submodule to main repo (use -c to allow file protocol)
		testutil.RunGit(t, mainDir, "-c", "protocol.file.allow=always", "submodule", "add", submoduleDir, "vendor/lib")
		testutil.RunGit(t, mainDir, "commit", "-m", "add submodule")

		// Create a worktree
		wtPath := filepath.Join(repoDir, "feature", "dirty-submodule")
		testutil.RunGit(t, mainDir, "worktree", "add", "-b", "feature/dirty-submodule", wtPath)

		// Initialize submodule in the new worktree (use -c to allow file protocol)
		testutil.RunGit(t, wtPath, "-c", "protocol.file.allow=always", "submodule", "update", "--init", "--recursive")

		// Make uncommitted changes in the submodule
		submoduleInWt := filepath.Join(wtPath, "vendor", "lib")
		if err := os.WriteFile(filepath.Join(submoduleInWt, "dirty.txt"), []byte("uncommitted"), 0644); err != nil {
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

		// Remove without --force should fail with hint
		_, err = cmd.Run("feature/dirty-submodule", mainDir, RemoveOptions{})
		if err == nil {
			t.Fatal("expected error for dirty submodule without --force")
		}

		var gitErr *GitError
		if !errors.As(err, &gitErr) {
			t.Fatalf("expected GitError, got %T: %v", err, err)
		}

		hint := gitErr.Hint()
		if !strings.Contains(hint, "force") {
			t.Errorf("hint should mention force, got: %s", hint)
		}

		// Verify worktree still exists
		if _, err := os.Stat(wtPath); os.IsNotExist(err) {
			t.Error("worktree should still exist")
		}

		// Now remove with --force should succeed
		_, err = cmd.Run("feature/dirty-submodule", mainDir, RemoveOptions{
			Force: WorktreeForceLevelUnclean,
		})
		if err != nil {
			t.Fatalf("Run with force failed: %v", err)
		}

		// Verify worktree is removed
		if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
			t.Errorf("worktree should be removed: %s", wtPath)
		}
	})

	t.Run("RemoveWorktreeWithUninitializedSubmodule", func(t *testing.T) {
		t.Parallel()

		repoDir, mainDir := testutil.SetupTestRepo(t)

		// Create a submodule in the main repo
		submoduleDir := t.TempDir()
		testutil.RunGit(t, submoduleDir, "init")
		testutil.RunGit(t, submoduleDir, "config", "user.email", "test@example.com")
		testutil.RunGit(t, submoduleDir, "config", "user.name", "Test User")
		if err := os.WriteFile(filepath.Join(submoduleDir, "README.md"), []byte("# Submodule"), 0644); err != nil {
			t.Fatal(err)
		}
		testutil.RunGit(t, submoduleDir, "add", ".")
		testutil.RunGit(t, submoduleDir, "commit", "-m", "initial")

		// Add submodule to main repo (use -c to allow file protocol)
		testutil.RunGit(t, mainDir, "-c", "protocol.file.allow=always", "submodule", "add", submoduleDir, "vendor/lib")
		testutil.RunGit(t, mainDir, "commit", "-m", "add submodule")

		// Create a worktree WITHOUT initializing the submodule
		wtPath := filepath.Join(repoDir, "feature", "uninit-submodule")
		testutil.RunGit(t, mainDir, "worktree", "add", "-b", "feature/uninit-submodule", wtPath)

		// Don't initialize the submodule - leave it uninitialized
		// Verify submodule is NOT initialized (shows with - prefix)
		out := testutil.RunGit(t, wtPath, "submodule", "status")
		if !strings.HasPrefix(strings.TrimSpace(out), "-") {
			t.Fatalf("submodule should be uninitialized (prefix -): %s", out)
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

		// Remove without --force should succeed (uninitialized submodule doesn't require force)
		_, err = cmd.Run("feature/uninit-submodule", mainDir, RemoveOptions{})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Verify worktree is removed
		if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
			t.Errorf("worktree should be removed: %s", wtPath)
		}

		// Verify branch is deleted
		out = testutil.RunGit(t, mainDir, "branch", "--list", "feature/uninit-submodule")
		if strings.TrimSpace(out) != "" {
			t.Errorf("branch should be deleted, got: %s", out)
		}
	})
}
