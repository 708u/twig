//go:build integration

package twig

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/708u/twig/internal/testutil"
)

func TestSyncCommand_CwdInWorktreeSubdir_Integration(t *testing.T) {
	t.Parallel()

	t.Run("SelectsCorrectWorktreeFromSubdir", func(t *testing.T) {
		t.Parallel()

		repoDir, mainDir := testutil.SetupTestRepo(t, testutil.Symlinks(".envrc"), testutil.DefaultSource("main"))

		// Create .envrc in main
		if err := os.WriteFile(filepath.Join(mainDir, ".envrc"), []byte("# envrc"), 0644); err != nil {
			t.Fatal(err)
		}

		// Create worktree for feat/x
		testutil.RunGit(t, mainDir, "worktree", "add", filepath.Join(repoDir, "feat", "x"), "-b", "feat/x")
		wtPath := filepath.Join(repoDir, "feat", "x")

		// Create a subdirectory inside the worktree
		subdir := filepath.Join(wtPath, "subdir", "nested")
		if err := os.MkdirAll(subdir, 0755); err != nil {
			t.Fatal(err)
		}

		// Load config from main
		result, err := LoadConfig(mainDir)
		if err != nil {
			t.Fatal(err)
		}

		cmd := NewSyncCommand(osFS{}, NewGitRunner(mainDir), nil)

		// Run sync with cwd set to nested subdirectory inside worktree
		syncResult, err := cmd.Run(t.Context(), nil, subdir, SyncOptions{
			Source:     result.Config.DefaultSource,
			SourcePath: mainDir,
			Symlinks:   result.Config.Symlinks,
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Verify the correct worktree (feat/x) was selected
		if len(syncResult.Targets) != 1 {
			t.Fatalf("expected 1 target, got %d", len(syncResult.Targets))
		}
		if syncResult.Targets[0].Branch != "feat/x" {
			t.Errorf("target branch = %q, want %q", syncResult.Targets[0].Branch, "feat/x")
		}
		if syncResult.Targets[0].WorktreePath != wtPath {
			t.Errorf("target path = %q, want %q", syncResult.Targets[0].WorktreePath, wtPath)
		}
	})

	t.Run("SimilarPrefixWorktreesAreDistinguished", func(t *testing.T) {
		t.Parallel()

		// This test verifies that worktrees with similar path prefixes are correctly distinguished
		// e.g., /repo and /repo-worktree should not be confused
		tmpDir := t.TempDir()
		tmpDir, _ = filepath.EvalSymlinks(tmpDir)

		// Create main repo at /tmp/repo
		mainDir := filepath.Join(tmpDir, "repo")
		if err := os.MkdirAll(mainDir, 0755); err != nil {
			t.Fatal(err)
		}
		testutil.RunGit(t, mainDir, "init")
		testutil.RunGit(t, mainDir, "config", "user.email", "test@example.com")
		testutil.RunGit(t, mainDir, "config", "user.name", "Test User")
		testutil.RunGit(t, mainDir, "commit", "--allow-empty", "-m", "initial")
		testutil.RunGit(t, mainDir, "branch", "-M", "main")

		// Create .twig/settings.toml
		twigDir := filepath.Join(mainDir, ".twig")
		if err := os.MkdirAll(twigDir, 0755); err != nil {
			t.Fatal(err)
		}
		settings := `worktree_destination_base_dir = "` + filepath.Join(tmpDir, "repo-worktree") + `"
default_source = "main"
symlinks = [".envrc"]
`
		if err := os.WriteFile(filepath.Join(twigDir, "settings.toml"), []byte(settings), 0644); err != nil {
			t.Fatal(err)
		}

		// Create .envrc
		if err := os.WriteFile(filepath.Join(mainDir, ".envrc"), []byte("# envrc"), 0644); err != nil {
			t.Fatal(err)
		}

		// Create worktree at /tmp/repo-worktree/feat/x (note: "repo-worktree", not "repo/worktree")
		wtBaseDir := filepath.Join(tmpDir, "repo-worktree")
		if err := os.MkdirAll(wtBaseDir, 0755); err != nil {
			t.Fatal(err)
		}
		wtPath := filepath.Join(wtBaseDir, "feat", "x")
		testutil.RunGit(t, mainDir, "worktree", "add", wtPath, "-b", "feat/x")

		// Create subdirectory in worktree
		subdir := filepath.Join(wtPath, "subdir")
		if err := os.MkdirAll(subdir, 0755); err != nil {
			t.Fatal(err)
		}

		result, err := LoadConfig(mainDir)
		if err != nil {
			t.Fatal(err)
		}

		cmd := NewSyncCommand(osFS{}, NewGitRunner(mainDir), nil)

		// Run sync with cwd in worktree subdirectory
		// This should select feat/x, not be confused with main repo (/repo vs /repo-worktree)
		syncResult, err := cmd.Run(t.Context(), nil, subdir, SyncOptions{
			Source:     result.Config.DefaultSource,
			SourcePath: mainDir,
			Symlinks:   result.Config.Symlinks,
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Verify feat/x was selected
		if len(syncResult.Targets) != 1 {
			t.Fatalf("expected 1 target, got %d", len(syncResult.Targets))
		}
		if syncResult.Targets[0].Branch != "feat/x" {
			t.Errorf("target branch = %q, want %q", syncResult.Targets[0].Branch, "feat/x")
		}
	})
}
