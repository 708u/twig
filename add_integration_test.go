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

func TestAddCommand_Integration(t *testing.T) {
	t.Parallel()

	t.Run("FullWorkflow", func(t *testing.T) {
		t.Parallel()

		repoDir, mainDir := testutil.SetupTestRepo(t)

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
		if err := os.WriteFile(filepath.Join(mainDir, ".envrc"), []byte("# envrc"), 0644); err != nil {
			t.Fatal(err)
		}

		result, err := LoadConfig(mainDir)
		if err != nil {
			t.Fatal(err)
		}

		cmd := &AddCommand{
			FS:     osFS{},
			Git:    NewGitRunner(mainDir),
			Config: result.Config,
		}

		addResult, err := cmd.Run("feature/test")
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		wtPath := filepath.Join(repoDir, "feature", "test")
		if _, err := os.Stat(wtPath); os.IsNotExist(err) {
			t.Errorf("worktree directory does not exist: %s", wtPath)
		}

		envrcPath := filepath.Join(wtPath, ".envrc")
		info, err := os.Lstat(envrcPath)
		if err != nil {
			t.Fatalf("failed to stat .envrc: %v", err)
		}
		if info.Mode()&os.ModeSymlink == 0 {
			t.Errorf(".envrc is not a symlink")
		}

		target, err := os.Readlink(envrcPath)
		if err != nil {
			t.Fatalf("failed to read symlink: %v", err)
		}
		expectedTarget := filepath.Join(mainDir, ".envrc")
		if target != expectedTarget {
			t.Errorf("symlink target = %q, want %q", target, expectedTarget)
		}

		out := testutil.RunGit(t, mainDir, "worktree", "list")
		if !strings.Contains(out, "feature/test") {
			t.Errorf("worktree list does not contain feature/test: %s", out)
		}

		// Verify result
		if addResult.Branch != "feature/test" {
			t.Errorf("result.Branch = %q, want %q", addResult.Branch, "feature/test")
		}
		if addResult.WorktreePath != wtPath {
			t.Errorf("result.WorktreePath = %q, want %q", addResult.WorktreePath, wtPath)
		}
	})

	t.Run("DefaultDestinationBaseDir", func(t *testing.T) {
		t.Parallel()

		repoDir, mainDir := testutil.SetupTestRepo(t)

		gwtDir := filepath.Join(mainDir, ".gwt")
		if err := os.MkdirAll(gwtDir, 0755); err != nil {
			t.Fatal(err)
		}

		// Omit worktree_destination_base_dir - should default to parent of srcDir
		settingsContent := fmt.Sprintf(`worktree_source_dir = %q
`, mainDir)
		if err := os.WriteFile(filepath.Join(gwtDir, "settings.toml"), []byte(settingsContent), 0644); err != nil {
			t.Fatal(err)
		}

		result, err := LoadConfig(mainDir)
		if err != nil {
			t.Fatal(err)
		}

		// Verify destBaseDir is set to default value
		expectedDestBaseDir := filepath.Join(repoDir, "main-worktree")
		if result.Config.WorktreeDestBaseDir != expectedDestBaseDir {
			t.Errorf("expected WorktreeDestBaseDir %q, got %q", expectedDestBaseDir, result.Config.WorktreeDestBaseDir)
		}

		cmd := &AddCommand{
			FS:     osFS{},
			Git:    NewGitRunner(mainDir),
			Config: result.Config,
		}

		_, err = cmd.Run("feature/default-dest")
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Worktree should be created in ${repoName}-worktree/${branch}
		expectedPath := filepath.Join(repoDir, "main-worktree", "feature", "default-dest")
		if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
			t.Errorf("worktree not created at expected path: %s", expectedPath)
		}

		out := testutil.RunGit(t, mainDir, "worktree", "list")
		if !strings.Contains(out, "feature/default-dest") {
			t.Errorf("worktree list does not contain feature/default-dest: %s", out)
		}
	})

	t.Run("ExistingBranch", func(t *testing.T) {
		t.Parallel()

		repoDir, mainDir := testutil.SetupTestRepo(t)

		testutil.RunGit(t, mainDir, "branch", "existing-branch")

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

		result, err := LoadConfig(mainDir)
		if err != nil {
			t.Fatal(err)
		}

		cmd := &AddCommand{
			FS:     osFS{},
			Git:    NewGitRunner(mainDir),
			Config: result.Config,
		}

		_, err = cmd.Run("existing-branch")
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		wtPath := filepath.Join(repoDir, "existing-branch")
		if _, err := os.Stat(wtPath); os.IsNotExist(err) {
			t.Errorf("worktree directory does not exist: %s", wtPath)
		}
	})

	t.Run("BranchAlreadyCheckedOut", func(t *testing.T) {
		t.Parallel()

		repoDir, mainDir := testutil.SetupTestRepo(t)

		testutil.RunGit(t, mainDir, "worktree", "add", filepath.Join(repoDir, "other-wt"), "-b", "test-branch")

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

		result, err := LoadConfig(mainDir)
		if err != nil {
			t.Fatal(err)
		}

		cmd := &AddCommand{
			FS:     osFS{},
			Git:    NewGitRunner(mainDir),
			Config: result.Config,
		}

		_, err = cmd.Run("test-branch")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "already checked out") {
			t.Errorf("error %q should contain 'already checked out'", err.Error())
		}
	})

	t.Run("LocalConfigSymlinksMerge", func(t *testing.T) {
		t.Parallel()

		repoDir, mainDir := testutil.SetupTestRepo(t)

		gwtDir := filepath.Join(mainDir, ".gwt")
		if err := os.MkdirAll(gwtDir, 0755); err != nil {
			t.Fatal(err)
		}

		// Project config with .envrc
		projectSettings := fmt.Sprintf(`symlinks = [".envrc"]
worktree_source_dir = %q
worktree_destination_base_dir = %q
`, mainDir, repoDir)
		if err := os.WriteFile(filepath.Join(gwtDir, "settings.toml"), []byte(projectSettings), 0644); err != nil {
			t.Fatal(err)
		}

		// Local config with .tool-versions and duplicate .envrc
		localSettings := `symlinks = [".tool-versions", ".envrc"]
`
		if err := os.WriteFile(filepath.Join(gwtDir, "settings.local.toml"), []byte(localSettings), 0644); err != nil {
			t.Fatal(err)
		}

		// Create source files
		if err := os.WriteFile(filepath.Join(mainDir, ".envrc"), []byte("# envrc"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(mainDir, ".tool-versions"), []byte("go 1.21"), 0644); err != nil {
			t.Fatal(err)
		}

		result, err := LoadConfig(mainDir)
		if err != nil {
			t.Fatal(err)
		}

		// Verify merged symlinks with deduplication
		if len(result.Config.Symlinks) != 2 {
			t.Errorf("expected 2 symlinks, got %d: %v", len(result.Config.Symlinks), result.Config.Symlinks)
		}

		cmd := &AddCommand{
			FS:     osFS{},
			Git:    NewGitRunner(mainDir),
			Config: result.Config,
		}

		_, err = cmd.Run("feature/local-merge")
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		wtPath := filepath.Join(repoDir, "feature", "local-merge")

		// Verify both files are symlinked
		for _, rel := range []string{".envrc", ".tool-versions"} {
			linkPath := filepath.Join(wtPath, rel)
			info, err := os.Lstat(linkPath)
			if err != nil {
				t.Errorf("symlink not created: %s", rel)
				continue
			}
			if info.Mode()&os.ModeSymlink == 0 {
				t.Errorf("%s is not a symlink", rel)
			}

			target, err := os.Readlink(linkPath)
			if err != nil {
				t.Errorf("failed to read symlink %s: %v", rel, err)
				continue
			}
			expectedTarget := filepath.Join(mainDir, rel)
			if target != expectedTarget {
				t.Errorf("symlink target = %q, want %q", target, expectedTarget)
			}
		}
	})

	t.Run("NoMatchPatternWarning", func(t *testing.T) {
		t.Parallel()

		repoDir, mainDir := testutil.SetupTestRepo(t)

		gwtDir := filepath.Join(mainDir, ".gwt")
		if err := os.MkdirAll(gwtDir, 0755); err != nil {
			t.Fatal(err)
		}

		// Configure symlinks with non-existent file pattern
		settingsContent := fmt.Sprintf(`symlinks = ["nonexistent.txt", ".envrc"]
worktree_source_dir = %q
worktree_destination_base_dir = %q
`, mainDir, repoDir)
		if err := os.WriteFile(filepath.Join(gwtDir, "settings.toml"), []byte(settingsContent), 0644); err != nil {
			t.Fatal(err)
		}

		// Only create .envrc (not nonexistent.txt)
		if err := os.WriteFile(filepath.Join(mainDir, ".envrc"), []byte("# envrc"), 0644); err != nil {
			t.Fatal(err)
		}

		result, err := LoadConfig(mainDir)
		if err != nil {
			t.Fatal(err)
		}

		cmd := &AddCommand{
			FS:     osFS{},
			Git:    NewGitRunner(mainDir),
			Config: result.Config,
		}

		addResult, err := cmd.Run("feature/warn-test")
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Verify worktree was created successfully
		wtPath := filepath.Join(repoDir, "feature", "warn-test")
		if _, err := os.Stat(wtPath); os.IsNotExist(err) {
			t.Errorf("worktree directory does not exist: %s", wtPath)
		}

		// Verify warning in result
		var foundWarning bool
		for _, s := range addResult.Symlinks {
			if s.Skipped && strings.Contains(s.Reason, "nonexistent.txt") &&
				strings.Contains(s.Reason, "does not match any files") {
				foundWarning = true
				break
			}
		}
		if !foundWarning {
			t.Errorf("expected warning about nonexistent.txt in result.Symlinks")
		}

		// Verify .envrc was symlinked (matching pattern should still work)
		envrcPath := filepath.Join(wtPath, ".envrc")
		info, err := os.Lstat(envrcPath)
		if err != nil {
			t.Fatalf("failed to stat .envrc: %v", err)
		}
		if info.Mode()&os.ModeSymlink == 0 {
			t.Errorf(".envrc is not a symlink")
		}
	})

	t.Run("MultipleSymlinkPatterns", func(t *testing.T) {
		t.Parallel()

		repoDir, mainDir := testutil.SetupTestRepo(t)

		// Mix of literal files and glob-matched files
		literalFiles := []string{".envrc", ".tool-versions"}
		globMatchFiles := []string{"config/app.toml", "config/env/dev.toml"}
		matchFiles := append(literalFiles, globMatchFiles...)
		noMatchFiles := []string{"config/readme.md", "other.txt"}

		// Create nested directory structure for glob testing
		if err := os.MkdirAll(filepath.Join(mainDir, "config", "env"), 0755); err != nil {
			t.Fatal(err)
		}
		for _, f := range matchFiles {
			if err := os.WriteFile(filepath.Join(mainDir, f), []byte("content"), 0644); err != nil {
				t.Fatal(err)
			}
		}
		for _, f := range noMatchFiles {
			if err := os.WriteFile(filepath.Join(mainDir, f), []byte("content"), 0644); err != nil {
				t.Fatal(err)
			}
		}

		gwtDir := filepath.Join(mainDir, ".gwt")
		if err := os.MkdirAll(gwtDir, 0755); err != nil {
			t.Fatal(err)
		}

		// Multiple patterns: literal files + glob pattern
		settingsContent := fmt.Sprintf(`symlinks = [".envrc", ".tool-versions", "config/**/*.toml"]
worktree_source_dir = %q
worktree_destination_base_dir = %q
`, mainDir, repoDir)
		if err := os.WriteFile(filepath.Join(gwtDir, "settings.toml"), []byte(settingsContent), 0644); err != nil {
			t.Fatal(err)
		}

		result, err := LoadConfig(mainDir)
		if err != nil {
			t.Fatal(err)
		}

		cmd := &AddCommand{
			FS:     osFS{},
			Git:    NewGitRunner(mainDir),
			Config: result.Config,
		}

		_, err = cmd.Run("feature/glob-test")
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		wtPath := filepath.Join(repoDir, "feature", "glob-test")

		// Verify symlinks created for glob matches
		for _, rel := range matchFiles {
			linkPath := filepath.Join(wtPath, rel)
			info, err := os.Lstat(linkPath)
			if err != nil {
				t.Errorf("symlink not created: %s", rel)
				continue
			}
			if info.Mode()&os.ModeSymlink == 0 {
				t.Errorf("%s is not a symlink", rel)
			}

			target, err := os.Readlink(linkPath)
			if err != nil {
				t.Errorf("failed to read symlink %s: %v", rel, err)
				continue
			}
			expectedTarget := filepath.Join(mainDir, rel)
			if target != expectedTarget {
				t.Errorf("symlink target = %q, want %q", target, expectedTarget)
			}
		}

		// Verify non-matching files are NOT symlinked
		for _, rel := range noMatchFiles {
			linkPath := filepath.Join(wtPath, rel)
			if _, err := os.Lstat(linkPath); err == nil {
				t.Errorf("file should not be symlinked: %s", rel)
			}
		}
	})

	t.Run("SyncUncommittedChanges", func(t *testing.T) {
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

		// Commit .gwt/settings.toml first
		testutil.RunGit(t, mainDir, "add", ".gwt")
		testutil.RunGit(t, mainDir, "commit", "-m", "add gwt settings")

		// Create uncommitted changes in source
		modifiedFile := filepath.Join(mainDir, "modified.txt")
		if err := os.WriteFile(modifiedFile, []byte("uncommitted content"), 0644); err != nil {
			t.Fatal(err)
		}

		result, err := LoadConfig(mainDir)
		if err != nil {
			t.Fatal(err)
		}

		cmd := &AddCommand{
			FS:     osFS{},
			Git:    NewGitRunner(mainDir),
			Config: result.Config,
			Sync:   true,
		}

		addResult, err := cmd.Run("feature/sync-test")
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Verify ChangesSynced is true
		if !addResult.ChangesSynced {
			t.Error("expected ChangesSynced to be true")
		}

		// Verify worktree was created
		wtPath := filepath.Join(repoDir, "feature", "sync-test")
		if _, err := os.Stat(wtPath); os.IsNotExist(err) {
			t.Errorf("worktree directory does not exist: %s", wtPath)
		}

		// Verify the file exists in new worktree
		syncedFile := filepath.Join(wtPath, "modified.txt")
		content, err := os.ReadFile(syncedFile)
		if err != nil {
			t.Fatalf("failed to read synced file: %v", err)
		}
		if string(content) != "uncommitted content" {
			t.Errorf("synced file content = %q, want %q", string(content), "uncommitted content")
		}

		// Verify the file still exists in source (restored via stash pop)
		sourceContent, err := os.ReadFile(modifiedFile)
		if err != nil {
			t.Fatalf("failed to read source file: %v", err)
		}
		if string(sourceContent) != "uncommitted content" {
			t.Errorf("source file content = %q, want %q", string(sourceContent), "uncommitted content")
		}
	})

	t.Run("SyncWithNoChanges", func(t *testing.T) {
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

		// Commit .gwt/settings.toml to ensure no uncommitted changes
		testutil.RunGit(t, mainDir, "add", ".gwt")
		testutil.RunGit(t, mainDir, "commit", "-m", "add gwt settings")

		result, err := LoadConfig(mainDir)
		if err != nil {
			t.Fatal(err)
		}

		cmd := &AddCommand{
			FS:     osFS{},
			Git:    NewGitRunner(mainDir),
			Config: result.Config,
			Sync:   true,
		}

		addResult, err := cmd.Run("feature/no-changes")
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Verify ChangesSynced is false (no changes to sync)
		if addResult.ChangesSynced {
			t.Error("expected ChangesSynced to be false when no changes")
		}

		// Verify worktree was created
		wtPath := filepath.Join(repoDir, "feature", "no-changes")
		if _, err := os.Stat(wtPath); os.IsNotExist(err) {
			t.Errorf("worktree directory does not exist: %s", wtPath)
		}
	})

	t.Run("QuietOutputsOnlyPath", func(t *testing.T) {
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

		result, err := LoadConfig(mainDir)
		if err != nil {
			t.Fatal(err)
		}

		cmd := &AddCommand{
			FS:     osFS{},
			Git:    NewGitRunner(mainDir),
			Config: result.Config,
		}

		addResult, err := cmd.Run("feature/quiet-test")
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Verify worktree path matches expected
		wtPath := filepath.Join(repoDir, "feature", "quiet-test")
		if addResult.WorktreePath != wtPath {
			t.Errorf("WorktreePath = %q, want %q", addResult.WorktreePath, wtPath)
		}

		// Verify Format with Quiet option outputs only path
		formatted := addResult.Format(AddFormatOptions{Quiet: true})
		expectedOutput := wtPath + "\n"
		if formatted.Stdout != expectedOutput {
			t.Errorf("Format(Quiet: true) = %q, want %q", formatted.Stdout, expectedOutput)
		}

		// Verify the path is a valid directory
		if _, err := os.Stat(wtPath); os.IsNotExist(err) {
			t.Errorf("worktree directory does not exist: %s", wtPath)
		}
	})
}
