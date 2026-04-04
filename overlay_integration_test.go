//go:build integration

package twig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/708u/twig/internal/testutil"
)

func TestOverlay_Integration(t *testing.T) {
	t.Parallel()

	t.Run("ApplyAndRestore", func(t *testing.T) {
		t.Parallel()

		_, mainDir := testutil.SetupTestRepo(t)

		// Create a file on main
		writeFile(t, mainDir, "main-only.txt", "main content")
		writeFile(t, mainDir, "shared.txt", "original content")
		testutil.RunGit(t, mainDir, "add", ".")
		testutil.RunGit(t, mainDir, "commit", "-m", "add files on main")

		// Create feature branch with changes
		testutil.RunGit(t, mainDir, "checkout", "-b", "feat/x")
		writeFile(t, mainDir, "shared.txt", "feature content")
		writeFile(t, mainDir, "feat-only.txt", "feature file")
		testutil.RunGit(t, mainDir, "add", ".")
		testutil.RunGit(t, mainDir, "commit", "-m", "feature changes")

		// Switch back to main
		testutil.RunGit(t, mainDir, "checkout", "main")

		// Apply overlay
		git := NewGitRunner(mainDir)
		cmd := NewOverlayCommand(osFS{}, git, nil)

		result, err := cmd.Run(t.Context(), "feat/x", mainDir, OverlayOptions{})
		if err != nil {
			t.Fatalf("apply failed: %v", err)
		}

		if result.SourceBranch != "feat/x" {
			t.Errorf("SourceBranch = %q, want feat/x", result.SourceBranch)
		}
		if result.ModifiedFiles == 0 {
			t.Error("expected modified files")
		}

		// Verify overlay: shared.txt should have feature content
		content := readFile(t, mainDir, "shared.txt")
		if content != "feature content" {
			t.Errorf("shared.txt = %q, want 'feature content'", content)
		}

		// feat-only.txt should exist
		if !fileExists(t, mainDir, "feat-only.txt") {
			t.Error("feat-only.txt should exist after overlay")
		}

		// Still on main branch
		branch := strings.TrimSpace(testutil.RunGit(t, mainDir, "rev-parse", "--abbrev-ref", "HEAD"))
		if branch != "main" {
			t.Errorf("branch = %q, want main", branch)
		}

		// No staged changes (unstaged by reset HEAD)
		staged := testutil.RunGit(t, mainDir, "diff", "--cached", "--name-only")
		if strings.TrimSpace(staged) != "" {
			t.Errorf("unexpected staged changes: %s", staged)
		}

		// Restore
		restoreResult, err := cmd.Run(t.Context(), "", mainDir, OverlayOptions{Restore: true})
		if err != nil {
			t.Fatalf("restore failed: %v", err)
		}
		if !restoreResult.Restored {
			t.Error("expected Restored=true")
		}

		// Verify restore: shared.txt should be back to original
		content = readFile(t, mainDir, "shared.txt")
		if content != "original content" {
			t.Errorf("shared.txt = %q after restore, want 'original content'", content)
		}

		// feat-only.txt should be gone
		if fileExists(t, mainDir, "feat-only.txt") {
			t.Error("feat-only.txt should not exist after restore")
		}
	})

	t.Run("WithDeletedFiles", func(t *testing.T) {
		t.Parallel()

		_, mainDir := testutil.SetupTestRepo(t)

		// Create files on main
		writeFile(t, mainDir, "keep.txt", "keep")
		writeFile(t, mainDir, "delete-me.txt", "will be deleted")
		testutil.RunGit(t, mainDir, "add", ".")
		testutil.RunGit(t, mainDir, "commit", "-m", "add files")

		// Create feature branch that deletes a file
		testutil.RunGit(t, mainDir, "checkout", "-b", "feat/del")
		testutil.RunGit(t, mainDir, "rm", "delete-me.txt")
		testutil.RunGit(t, mainDir, "commit", "-m", "delete file")

		// Switch back to main
		testutil.RunGit(t, mainDir, "checkout", "main")

		git := NewGitRunner(mainDir)
		cmd := NewOverlayCommand(osFS{}, git, nil)

		// Apply overlay
		result, err := cmd.Run(t.Context(), "feat/del", mainDir, OverlayOptions{})
		if err != nil {
			t.Fatalf("apply failed: %v", err)
		}

		// delete-me.txt should be deleted
		if fileExists(t, mainDir, "delete-me.txt") {
			t.Error("delete-me.txt should be deleted after overlay")
		}
		if len(result.DeletedFiles) == 0 {
			t.Error("expected deleted files in result")
		}

		// Restore
		_, err = cmd.Run(t.Context(), "", mainDir, OverlayOptions{Restore: true})
		if err != nil {
			t.Fatalf("restore failed: %v", err)
		}

		// delete-me.txt should be back
		if !fileExists(t, mainDir, "delete-me.txt") {
			t.Error("delete-me.txt should exist after restore")
		}
	})

	t.Run("WithNewFiles", func(t *testing.T) {
		t.Parallel()

		_, mainDir := testutil.SetupTestRepo(t)

		// Create file on main
		writeFile(t, mainDir, "existing.txt", "existing")
		testutil.RunGit(t, mainDir, "add", ".")
		testutil.RunGit(t, mainDir, "commit", "-m", "add file")

		// Create feature branch with new file
		testutil.RunGit(t, mainDir, "checkout", "-b", "feat/new")
		writeFile(t, mainDir, "brand-new.txt", "new content")
		testutil.RunGit(t, mainDir, "add", ".")
		testutil.RunGit(t, mainDir, "commit", "-m", "add new file")

		testutil.RunGit(t, mainDir, "checkout", "main")

		git := NewGitRunner(mainDir)
		cmd := NewOverlayCommand(osFS{}, git, nil)

		result, err := cmd.Run(t.Context(), "feat/new", mainDir, OverlayOptions{})
		if err != nil {
			t.Fatalf("apply failed: %v", err)
		}

		// brand-new.txt should appear
		if !fileExists(t, mainDir, "brand-new.txt") {
			t.Error("brand-new.txt should exist after overlay")
		}
		if len(result.AddedFiles) == 0 {
			t.Error("expected added files in result")
		}

		// Restore should remove it
		_, err = cmd.Run(t.Context(), "", mainDir, OverlayOptions{Restore: true})
		if err != nil {
			t.Fatalf("restore failed: %v", err)
		}
		if fileExists(t, mainDir, "brand-new.txt") {
			t.Error("brand-new.txt should not exist after restore")
		}
	})

	t.Run("PreservesUserFiles", func(t *testing.T) {
		t.Parallel()

		_, mainDir := testutil.SetupTestRepo(t)

		writeFile(t, mainDir, "main.txt", "main")
		testutil.RunGit(t, mainDir, "add", ".")
		testutil.RunGit(t, mainDir, "commit", "-m", "add main.txt")

		testutil.RunGit(t, mainDir, "checkout", "-b", "feat/y")
		writeFile(t, mainDir, "main.txt", "modified")
		testutil.RunGit(t, mainDir, "add", ".")
		testutil.RunGit(t, mainDir, "commit", "-m", "modify")

		testutil.RunGit(t, mainDir, "checkout", "main")

		git := NewGitRunner(mainDir)
		cmd := NewOverlayCommand(osFS{}, git, nil)

		_, err := cmd.Run(t.Context(), "feat/y", mainDir, OverlayOptions{})
		if err != nil {
			t.Fatalf("apply failed: %v", err)
		}

		// User creates a debug file after overlay
		writeFile(t, mainDir, "debug.log", "user debug")

		// Restore
		_, err = cmd.Run(t.Context(), "", mainDir, OverlayOptions{Restore: true})
		if err != nil {
			t.Fatalf("restore failed: %v", err)
		}

		// debug.log should be preserved
		if !fileExists(t, mainDir, "debug.log") {
			t.Error("user-created debug.log should be preserved after restore")
		}
	})

	t.Run("ForceOverlayOnDirtyTarget", func(t *testing.T) {
		t.Parallel()

		_, mainDir := testutil.SetupTestRepo(t)

		writeFile(t, mainDir, "file.txt", "original")
		testutil.RunGit(t, mainDir, "add", ".")
		testutil.RunGit(t, mainDir, "commit", "-m", "add file")

		testutil.RunGit(t, mainDir, "checkout", "-b", "feat/f")
		writeFile(t, mainDir, "file.txt", "feature")
		testutil.RunGit(t, mainDir, "add", ".")
		testutil.RunGit(t, mainDir, "commit", "-m", "feature change")

		testutil.RunGit(t, mainDir, "checkout", "main")

		// Make target dirty
		writeFile(t, mainDir, "file.txt", "dirty local edit")

		git := NewGitRunner(mainDir)
		cmd := NewOverlayCommand(osFS{}, git, nil)

		// Without --force: should refuse
		_, err := cmd.Run(t.Context(), "feat/f", mainDir, OverlayOptions{})
		if err == nil {
			t.Fatal("expected error for dirty target")
		}
		if !strings.Contains(err.Error(), "uncommitted changes") {
			t.Fatalf("unexpected error: %v", err)
		}

		// With --force: should succeed
		_, err = cmd.Run(t.Context(), "feat/f", mainDir, OverlayOptions{Force: true})
		if err != nil {
			t.Fatalf("force apply failed: %v", err)
		}

		// Verify overlay applied
		content := readFile(t, mainDir, "file.txt")
		if content != "feature" {
			t.Errorf("file.txt = %q after overlay, want 'feature'", content)
		}

		// Restore should work
		_, err = cmd.Run(t.Context(), "", mainDir, OverlayOptions{Restore: true})
		if err != nil {
			t.Fatalf("restore failed: %v", err)
		}

		// file.txt restored to committed state (dirty edit is lost,
		// which is the expected --force trade-off)
		content = readFile(t, mainDir, "file.txt")
		if content != "original" {
			t.Errorf("file.txt = %q after restore, want 'original'", content)
		}
	})

	t.Run("HeadMovedDuringOverlay", func(t *testing.T) {
		t.Parallel()

		_, mainDir := testutil.SetupTestRepo(t)

		writeFile(t, mainDir, "file.txt", "main")
		testutil.RunGit(t, mainDir, "add", ".")
		testutil.RunGit(t, mainDir, "commit", "-m", "add file")

		testutil.RunGit(t, mainDir, "checkout", "-b", "feat/h")
		writeFile(t, mainDir, "file.txt", "feature")
		testutil.RunGit(t, mainDir, "add", ".")
		testutil.RunGit(t, mainDir, "commit", "-m", "feature")

		testutil.RunGit(t, mainDir, "checkout", "main")

		git := NewGitRunner(mainDir)
		cmd := NewOverlayCommand(osFS{}, git, nil)

		// Apply overlay
		_, err := cmd.Run(t.Context(), "feat/h", mainDir, OverlayOptions{})
		if err != nil {
			t.Fatalf("apply failed: %v", err)
		}

		// Simulate accidental commit (empty commit moves HEAD)
		testutil.RunGit(t, mainDir, "commit", "--allow-empty", "-m", "accidental")

		// Restore without --force: should detect HEAD movement
		_, err = cmd.Run(t.Context(), "", mainDir, OverlayOptions{Restore: true})
		if err == nil {
			t.Fatal("expected error for HEAD movement")
		}
		if !strings.Contains(err.Error(), "HEAD has moved") {
			t.Fatalf("unexpected error: %v", err)
		}

		// Restore with --force: should succeed
		_, err = cmd.Run(t.Context(), "", mainDir, OverlayOptions{Restore: true, Force: true})
		if err != nil {
			t.Fatalf("force restore failed: %v", err)
		}

		// File should be restored
		content := readFile(t, mainDir, "file.txt")
		if content != "main" {
			t.Errorf("file.txt = %q after force restore, want 'main'", content)
		}

		// State file should be removed
		gitDirOut := strings.TrimSpace(testutil.RunGit(t, mainDir, "rev-parse", "--path-format=absolute", "--git-dir"))
		statePath := filepath.Join(gitDirOut, "twig-overlay")
		if _, err := os.Stat(statePath); !os.IsNotExist(err) {
			t.Error("state file should be removed after force restore")
		}
	})

	t.Run("OverlayStackingBlocked", func(t *testing.T) {
		t.Parallel()

		_, mainDir := testutil.SetupTestRepo(t)

		writeFile(t, mainDir, "file.txt", "main")
		testutil.RunGit(t, mainDir, "add", ".")
		testutil.RunGit(t, mainDir, "commit", "-m", "add file")

		testutil.RunGit(t, mainDir, "checkout", "-b", "feat/s1")
		writeFile(t, mainDir, "file.txt", "s1")
		testutil.RunGit(t, mainDir, "add", ".")
		testutil.RunGit(t, mainDir, "commit", "-m", "s1")

		testutil.RunGit(t, mainDir, "checkout", "main")
		testutil.RunGit(t, mainDir, "checkout", "-b", "feat/s2")
		writeFile(t, mainDir, "file.txt", "s2")
		testutil.RunGit(t, mainDir, "add", ".")
		testutil.RunGit(t, mainDir, "commit", "-m", "s2")

		testutil.RunGit(t, mainDir, "checkout", "main")

		git := NewGitRunner(mainDir)
		cmd := NewOverlayCommand(osFS{}, git, nil)

		// First overlay
		_, err := cmd.Run(t.Context(), "feat/s1", mainDir, OverlayOptions{})
		if err != nil {
			t.Fatalf("first overlay failed: %v", err)
		}

		// Second overlay with same source: blocked
		_, err = cmd.Run(t.Context(), "feat/s1", mainDir, OverlayOptions{})
		if err == nil {
			t.Fatal("expected error for stacking same source")
		}
		if !strings.Contains(err.Error(), "overlay already active") {
			t.Fatalf("unexpected error: %v", err)
		}

		// Second overlay with different source: also blocked
		_, err = cmd.Run(t.Context(), "feat/s2", mainDir, OverlayOptions{})
		if err == nil {
			t.Fatal("expected error for stacking different source")
		}

		// Even with --force: stacking blocked
		_, err = cmd.Run(t.Context(), "feat/s2", mainDir, OverlayOptions{Force: true})
		if err == nil {
			t.Fatal("expected error for stacking with --force")
		}

		// Clean up: restore first overlay
		_, err = cmd.Run(t.Context(), "", mainDir, OverlayOptions{Restore: true})
		if err != nil {
			t.Fatalf("restore failed: %v", err)
		}
	})

	t.Run("StateFilePersistence", func(t *testing.T) {
		t.Parallel()

		_, mainDir := testutil.SetupTestRepo(t)

		writeFile(t, mainDir, "file.txt", "content")
		testutil.RunGit(t, mainDir, "add", ".")
		testutil.RunGit(t, mainDir, "commit", "-m", "add file")

		testutil.RunGit(t, mainDir, "checkout", "-b", "feat/z")
		writeFile(t, mainDir, "file.txt", "changed")
		testutil.RunGit(t, mainDir, "add", ".")
		testutil.RunGit(t, mainDir, "commit", "-m", "change file")

		testutil.RunGit(t, mainDir, "checkout", "main")

		git := NewGitRunner(mainDir)
		cmd := NewOverlayCommand(osFS{}, git, nil)

		_, err := cmd.Run(t.Context(), "feat/z", mainDir, OverlayOptions{})
		if err != nil {
			t.Fatalf("apply failed: %v", err)
		}

		// State file should exist in .git directory
		gitDirOut := strings.TrimSpace(testutil.RunGit(t, mainDir, "rev-parse", "--path-format=absolute", "--git-dir"))
		statePath := filepath.Join(gitDirOut, "twig-overlay")
		if _, err := os.Stat(statePath); os.IsNotExist(err) {
			t.Errorf("state file not found at %s", statePath)
		}

		// Restore and verify state file is removed
		_, err = cmd.Run(t.Context(), "", mainDir, OverlayOptions{Restore: true})
		if err != nil {
			t.Fatalf("restore failed: %v", err)
		}
		if _, err := os.Stat(statePath); !os.IsNotExist(err) {
			t.Error("state file should be removed after restore")
		}
	})

	t.Run("DirtyApplyAndRestore", func(t *testing.T) {
		t.Parallel()

		repoDir, mainDir := testutil.SetupTestRepo(t)

		// Create committed content on main
		writeFile(t, mainDir, "shared.txt", "main content")
		testutil.RunGit(t, mainDir, "add", ".")
		testutil.RunGit(t, mainDir, "commit", "-m", "add files on main")

		// Create feature branch with committed changes
		testutil.RunGit(t, mainDir, "checkout", "-b", "feat/dirty")
		writeFile(t, mainDir, "shared.txt", "committed feature content")
		writeFile(t, mainDir, "committed-new.txt", "committed new")
		testutil.RunGit(t, mainDir, "add", ".")
		testutil.RunGit(t, mainDir, "commit", "-m", "feature changes")
		testutil.RunGit(t, mainDir, "checkout", "main")

		// Create worktree for feat/dirty
		wtPath := filepath.Join(repoDir, "feat-dirty-wt")
		testutil.RunGit(t, mainDir, "worktree", "add", wtPath, "feat/dirty")

		// Make uncommitted changes in the feature worktree
		writeFile(t, wtPath, "shared.txt", "dirty feature content")
		writeFile(t, wtPath, "untracked.txt", "untracked file")

		// Apply dirty overlay from feat/dirty onto main
		git := NewGitRunner(mainDir)
		cmd := NewOverlayCommand(osFS{}, git, nil)

		result, err := cmd.Run(t.Context(), "feat/dirty", mainDir, OverlayOptions{
			Dirty: true,
		})
		if err != nil {
			t.Fatalf("apply failed: %v", err)
		}

		if result.DirtyFiles != 2 {
			t.Errorf("DirtyFiles = %d, want 2", result.DirtyFiles)
		}

		// shared.txt should have the dirty (not committed) content
		content := readFile(t, mainDir, "shared.txt")
		if content != "dirty feature content" {
			t.Errorf("shared.txt = %q, want 'dirty feature content'", content)
		}

		// untracked.txt should exist (dirty untracked file)
		if !fileExists(t, mainDir, "untracked.txt") {
			t.Error("untracked.txt should exist after dirty overlay")
		}

		// committed-new.txt should also exist (from committed overlay)
		if !fileExists(t, mainDir, "committed-new.txt") {
			t.Error("committed-new.txt should exist from committed overlay")
		}

		// Restore
		_, err = cmd.Run(t.Context(), "", mainDir, OverlayOptions{Restore: true})
		if err != nil {
			t.Fatalf("restore failed: %v", err)
		}

		// shared.txt should be back to main content
		content = readFile(t, mainDir, "shared.txt")
		if content != "main content" {
			t.Errorf("shared.txt = %q after restore, want 'main content'", content)
		}

		// untracked.txt should be removed (was in AddedFiles)
		if fileExists(t, mainDir, "untracked.txt") {
			t.Error("untracked.txt should not exist after restore")
		}

		// committed-new.txt should also be removed
		if fileExists(t, mainDir, "committed-new.txt") {
			t.Error("committed-new.txt should not exist after restore")
		}
	})

	t.Run("DirtySameCommit", func(t *testing.T) {
		t.Parallel()

		repoDir, mainDir := testutil.SetupTestRepo(t)

		writeFile(t, mainDir, "file.txt", "original")
		testutil.RunGit(t, mainDir, "add", ".")
		testutil.RunGit(t, mainDir, "commit", "-m", "add file")

		// Create branch at same commit
		testutil.RunGit(t, mainDir, "branch", "feat/same")

		// Create worktree for feat/same
		wtPath := filepath.Join(repoDir, "feat-same-wt")
		testutil.RunGit(t, mainDir, "worktree", "add", wtPath, "feat/same")

		// Make dirty changes in the feature worktree
		writeFile(t, wtPath, "file.txt", "dirty content")
		writeFile(t, wtPath, "new-dirty.txt", "brand new")

		// Apply dirty overlay (same commit, only dirty files)
		git := NewGitRunner(mainDir)
		cmd := NewOverlayCommand(osFS{}, git, nil)

		result, err := cmd.Run(t.Context(), "feat/same", mainDir, OverlayOptions{
			Dirty: true,
		})
		if err != nil {
			t.Fatalf("apply failed: %v", err)
		}

		if result.ModifiedFiles != 0 {
			t.Errorf("ModifiedFiles = %d, want 0 (same commit)", result.ModifiedFiles)
		}
		if result.DirtyFiles != 2 {
			t.Errorf("DirtyFiles = %d, want 2", result.DirtyFiles)
		}

		// file.txt should have dirty content
		content := readFile(t, mainDir, "file.txt")
		if content != "dirty content" {
			t.Errorf("file.txt = %q, want 'dirty content'", content)
		}

		// Restore
		_, err = cmd.Run(t.Context(), "", mainDir, OverlayOptions{Restore: true})
		if err != nil {
			t.Fatalf("restore failed: %v", err)
		}

		// file.txt back to original
		content = readFile(t, mainDir, "file.txt")
		if content != "original" {
			t.Errorf("file.txt = %q after restore, want 'original'", content)
		}

		// new-dirty.txt should be cleaned up
		if fileExists(t, mainDir, "new-dirty.txt") {
			t.Error("new-dirty.txt should not exist after restore")
		}
	})

	t.Run("DirtyUntrackedFiles", func(t *testing.T) {
		t.Parallel()

		repoDir, mainDir := testutil.SetupTestRepo(t)

		writeFile(t, mainDir, "tracked.txt", "tracked")
		testutil.RunGit(t, mainDir, "add", ".")
		testutil.RunGit(t, mainDir, "commit", "-m", "add file")

		testutil.RunGit(t, mainDir, "branch", "feat/untracked")

		wtPath := filepath.Join(repoDir, "feat-untracked-wt")
		testutil.RunGit(t, mainDir, "worktree", "add", wtPath, "feat/untracked")

		// Create untracked files in subdirectory
		writeFile(t, wtPath, "subdir/new-file.txt", "new in subdir")
		writeFile(t, wtPath, "another-new.txt", "another")

		git := NewGitRunner(mainDir)
		cmd := NewOverlayCommand(osFS{}, git, nil)

		result, err := cmd.Run(t.Context(), "feat/untracked", mainDir, OverlayOptions{
			Dirty: true,
		})
		if err != nil {
			t.Fatalf("apply failed: %v", err)
		}

		if result.DirtyFiles != 2 {
			t.Errorf("DirtyFiles = %d, want 2", result.DirtyFiles)
		}

		// Untracked files should exist in target
		if !fileExists(t, mainDir, "subdir/new-file.txt") {
			t.Error("subdir/new-file.txt should exist")
		}
		if !fileExists(t, mainDir, "another-new.txt") {
			t.Error("another-new.txt should exist")
		}

		// Restore
		_, err = cmd.Run(t.Context(), "", mainDir, OverlayOptions{Restore: true})
		if err != nil {
			t.Fatalf("restore failed: %v", err)
		}

		// Untracked files should be cleaned up
		if fileExists(t, mainDir, "subdir/new-file.txt") {
			t.Error("subdir/new-file.txt should not exist after restore")
		}
		if fileExists(t, mainDir, "another-new.txt") {
			t.Error("another-new.txt should not exist after restore")
		}
	})
}

// Test helpers

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, dir, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func fileExists(t *testing.T, dir, name string) bool {
	t.Helper()
	_, err := os.Stat(filepath.Join(dir, name))
	return err == nil
}
