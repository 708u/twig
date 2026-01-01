//go:build integration

package gwt

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/708u/gwt/internal/testutil"
)

func TestRemoveCommand_Integration(t *testing.T) {
	t.Parallel()

	t.Run("RemoveWorktreeAndBranch", func(t *testing.T) {
		t.Parallel()

		repoDir, mainDir := testutil.SetupTestRepo(t)

		gwtDir := filepath.Join(mainDir, ".gwt")
		if err := os.MkdirAll(gwtDir, 0755); err != nil {
			t.Fatal(err)
		}

		settingsContent := fmt.Sprintf(`worktree_source_dir = %q
worktree_destination_base_dir = %q
`, mainDir, repoDir)
		if err := os.WriteFile(filepath.Join(gwtDir, "settings.toml"), []byte(settingsContent), 0644); err != nil {
			t.Fatal(err)
		}

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

	t.Run("DryRun", func(t *testing.T) {
		t.Parallel()

		repoDir, mainDir := testutil.SetupTestRepo(t)

		gwtDir := filepath.Join(mainDir, ".gwt")
		if err := os.MkdirAll(gwtDir, 0755); err != nil {
			t.Fatal(err)
		}

		settingsContent := fmt.Sprintf(`worktree_source_dir = %q
worktree_destination_base_dir = %q
`, mainDir, repoDir)
		if err := os.WriteFile(filepath.Join(gwtDir, "settings.toml"), []byte(settingsContent), 0644); err != nil {
			t.Fatal(err)
		}

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

		removeResult, err := cmd.Run("feature/dry-run-test", mainDir, RemoveOptions{DryRun: true})
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
		if !removeResult.DryRun {
			t.Error("result.DryRun should be true")
		}
	})

	t.Run("ForceRemoveWithUncommittedChanges", func(t *testing.T) {
		t.Parallel()

		repoDir, mainDir := testutil.SetupTestRepo(t)

		gwtDir := filepath.Join(mainDir, ".gwt")
		if err := os.MkdirAll(gwtDir, 0755); err != nil {
			t.Fatal(err)
		}

		settingsContent := fmt.Sprintf(`worktree_source_dir = %q
worktree_destination_base_dir = %q
`, mainDir, repoDir)
		if err := os.WriteFile(filepath.Join(gwtDir, "settings.toml"), []byte(settingsContent), 0644); err != nil {
			t.Fatal(err)
		}

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

		_, err = cmd.Run("feature/force-test", mainDir, RemoveOptions{Force: true})
		if err != nil {
			t.Fatalf("Run with force failed: %v", err)
		}

		if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
			t.Errorf("worktree directory should be removed: %s", wtPath)
		}
	})

	t.Run("ErrorWhenInsideWorktree", func(t *testing.T) {
		t.Parallel()

		repoDir, mainDir := testutil.SetupTestRepo(t)

		gwtDir := filepath.Join(mainDir, ".gwt")
		if err := os.MkdirAll(gwtDir, 0755); err != nil {
			t.Fatal(err)
		}

		settingsContent := fmt.Sprintf(`worktree_source_dir = %q
worktree_destination_base_dir = %q
`, mainDir, repoDir)
		if err := os.WriteFile(filepath.Join(gwtDir, "settings.toml"), []byte(settingsContent), 0644); err != nil {
			t.Fatal(err)
		}

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
		if !strings.Contains(err.Error(), "cannot remove: current directory is inside worktree") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("ErrorBranchNotInWorktree", func(t *testing.T) {
		t.Parallel()

		_, mainDir := testutil.SetupTestRepo(t)

		gwtDir := filepath.Join(mainDir, ".gwt")
		if err := os.MkdirAll(gwtDir, 0755); err != nil {
			t.Fatal(err)
		}

		settingsContent := fmt.Sprintf(`worktree_source_dir = %q
`, mainDir)
		if err := os.WriteFile(filepath.Join(gwtDir, "settings.toml"), []byte(settingsContent), 0644); err != nil {
			t.Fatal(err)
		}

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
}
