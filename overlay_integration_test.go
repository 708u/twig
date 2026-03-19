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
