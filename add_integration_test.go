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

		repoDir, mainDir := testutil.SetupTestRepo(t, testutil.Symlinks(".envrc"))

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

		repoDir, mainDir := testutil.SetupTestRepo(t, testutil.WithoutSettings())

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

		repoDir, mainDir := testutil.SetupTestRepo(t, testutil.WithoutSettings())

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

		repoDir, mainDir := testutil.SetupTestRepo(t,
			testutil.Symlinks("nonexistent.txt", ".envrc"))

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

		repoDir, mainDir := testutil.SetupTestRepo(t,
			testutil.Symlinks(".envrc", ".tool-versions", "config/**/*.toml"))

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

	t.Run("CarryUncommittedChanges", func(t *testing.T) {
		t.Parallel()

		repoDir, mainDir := testutil.SetupTestRepo(t)

		// Commit .gwt/settings.toml first
		testutil.RunGit(t, mainDir, "add", ".gwt")
		testutil.RunGit(t, mainDir, "commit", "-m", "add gwt settings")

		// Create uncommitted changes in source
		modifiedFile := filepath.Join(mainDir, "carried.txt")
		if err := os.WriteFile(modifiedFile, []byte("carried content"), 0644); err != nil {
			t.Fatal(err)
		}

		result, err := LoadConfig(mainDir)
		if err != nil {
			t.Fatal(err)
		}

		cmd := &AddCommand{
			FS:        osFS{},
			Git:       NewGitRunner(mainDir),
			Config:    result.Config,
			CarryFrom: mainDir,
		}

		addResult, err := cmd.Run("feature/carry-test")
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Verify ChangesCarried is true
		if !addResult.ChangesCarried {
			t.Error("expected ChangesCarried to be true")
		}

		// Verify worktree was created
		wtPath := filepath.Join(repoDir, "feature", "carry-test")
		if _, err := os.Stat(wtPath); os.IsNotExist(err) {
			t.Errorf("worktree directory does not exist: %s", wtPath)
		}

		// Verify the file exists in new worktree
		carriedFile := filepath.Join(wtPath, "carried.txt")
		content, err := os.ReadFile(carriedFile)
		if err != nil {
			t.Fatalf("failed to read carried file: %v", err)
		}
		if string(content) != "carried content" {
			t.Errorf("carried file content = %q, want %q", string(content), "carried content")
		}

		// Verify the file does NOT exist in source (carried away, not synced)
		if _, err := os.Stat(modifiedFile); !os.IsNotExist(err) {
			t.Errorf("source file should not exist after carry: %s", modifiedFile)
		}

		// Verify source is clean
		status := testutil.RunGit(t, mainDir, "status", "--porcelain")
		if strings.TrimSpace(status) != "" {
			t.Errorf("source should be clean after carry, got: %q", status)
		}
	})

	t.Run("CarryFromDifferentWorktree", func(t *testing.T) {
		t.Parallel()

		repoDir, mainDir := testutil.SetupTestRepo(t)

		// Commit .gwt/settings.toml first
		testutil.RunGit(t, mainDir, "add", ".gwt")
		testutil.RunGit(t, mainDir, "commit", "-m", "add gwt settings")

		// Create a feature worktree with uncommitted changes
		featureWtPath := filepath.Join(repoDir, "feature", "source")
		testutil.RunGit(t, mainDir, "worktree", "add", featureWtPath, "-b", "feature/source")

		// Create uncommitted changes in feature worktree
		modifiedFile := filepath.Join(featureWtPath, "from-feature.txt")
		if err := os.WriteFile(modifiedFile, []byte("content from feature"), 0644); err != nil {
			t.Fatal(err)
		}

		result, err := LoadConfig(mainDir)
		if err != nil {
			t.Fatal(err)
		}

		// Carry from feature worktree, create new worktree based on main
		cmd := &AddCommand{
			FS:        osFS{},
			Git:       NewGitRunner(mainDir),
			Config:    result.Config,
			CarryFrom: featureWtPath, // Carry from different worktree
		}

		addResult, err := cmd.Run("feature/carry-from-other")
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Verify ChangesCarried is true
		if !addResult.ChangesCarried {
			t.Error("expected ChangesCarried to be true")
		}

		// Verify worktree was created
		wtPath := filepath.Join(repoDir, "feature", "carry-from-other")
		if _, err := os.Stat(wtPath); os.IsNotExist(err) {
			t.Errorf("worktree directory does not exist: %s", wtPath)
		}

		// Verify the file exists in new worktree
		carriedFile := filepath.Join(wtPath, "from-feature.txt")
		content, err := os.ReadFile(carriedFile)
		if err != nil {
			t.Fatalf("failed to read carried file: %v", err)
		}
		if string(content) != "content from feature" {
			t.Errorf("carried file content = %q, want %q", string(content), "content from feature")
		}

		// Verify the file does NOT exist in feature worktree (carried away)
		if _, err := os.Stat(modifiedFile); !os.IsNotExist(err) {
			t.Errorf("source file should not exist after carry: %s", modifiedFile)
		}

		// Verify feature worktree is clean
		status := testutil.RunGit(t, featureWtPath, "status", "--porcelain")
		if strings.TrimSpace(status) != "" {
			t.Errorf("feature worktree should be clean after carry, got: %q", status)
		}

		// Verify main worktree is still clean (wasn't affected)
		mainStatus := testutil.RunGit(t, mainDir, "status", "--porcelain")
		if strings.TrimSpace(mainStatus) != "" {
			t.Errorf("main worktree should still be clean, got: %q", mainStatus)
		}
	})

	t.Run("SyncWithNoChanges", func(t *testing.T) {
		t.Parallel()

		repoDir, mainDir := testutil.SetupTestRepo(t)

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

	t.Run("LockWorktree", func(t *testing.T) {
		t.Parallel()

		_, mainDir := testutil.SetupTestRepo(t)

		result, err := LoadConfig(mainDir)
		if err != nil {
			t.Fatal(err)
		}

		cmd := &AddCommand{
			FS:     osFS{},
			Git:    NewGitRunner(mainDir),
			Config: result.Config,
			Lock:   true,
		}

		_, err = cmd.Run("feature/locked")
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Verify worktree is locked
		out := testutil.RunGit(t, mainDir, "worktree", "list", "--porcelain")
		if !strings.Contains(out, "locked") {
			t.Errorf("worktree should be locked, got: %s", out)
		}
	})

	t.Run("LockWorktreeWithReason", func(t *testing.T) {
		t.Parallel()

		_, mainDir := testutil.SetupTestRepo(t)

		result, err := LoadConfig(mainDir)
		if err != nil {
			t.Fatal(err)
		}

		cmd := &AddCommand{
			FS:         osFS{},
			Git:        NewGitRunner(mainDir),
			Config:     result.Config,
			Lock:       true,
			LockReason: "USB drive work",
		}

		_, err = cmd.Run("feature/locked-reason")
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Verify worktree is locked with reason
		out := testutil.RunGit(t, mainDir, "worktree", "list", "--porcelain")
		if !strings.Contains(out, "locked USB drive work") {
			t.Errorf("worktree should be locked with reason, got: %s", out)
		}
	})

	t.Run("CarrySpecificFiles", func(t *testing.T) {
		t.Parallel()

		repoDir, mainDir := testutil.SetupTestRepo(t)

		// Commit .gwt/settings.toml first
		testutil.RunGit(t, mainDir, "add", ".gwt")
		testutil.RunGit(t, mainDir, "commit", "-m", "add gwt settings")

		// Create multiple uncommitted files
		goFile := filepath.Join(mainDir, "main.go")
		if err := os.WriteFile(goFile, []byte("package main"), 0644); err != nil {
			t.Fatal(err)
		}
		txtFile := filepath.Join(mainDir, "readme.txt")
		if err := os.WriteFile(txtFile, []byte("readme content"), 0644); err != nil {
			t.Fatal(err)
		}

		result, err := LoadConfig(mainDir)
		if err != nil {
			t.Fatal(err)
		}

		// Carry only *.go files
		cmd := &AddCommand{
			FS:         osFS{},
			Git:        NewGitRunner(mainDir),
			Config:     result.Config,
			CarryFrom:  mainDir,
			CarryFiles: []string{"*.go"},
		}

		addResult, err := cmd.Run("feature/carry-go-only")
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Verify ChangesCarried is true
		if !addResult.ChangesCarried {
			t.Error("expected ChangesCarried to be true")
		}

		// Verify worktree was created
		wtPath := filepath.Join(repoDir, "feature", "carry-go-only")
		if _, err := os.Stat(wtPath); os.IsNotExist(err) {
			t.Errorf("worktree directory does not exist: %s", wtPath)
		}

		// Verify main.go exists in new worktree (carried)
		carriedGoFile := filepath.Join(wtPath, "main.go")
		content, err := os.ReadFile(carriedGoFile)
		if err != nil {
			t.Fatalf("failed to read carried go file: %v", err)
		}
		if string(content) != "package main" {
			t.Errorf("carried go file content = %q, want %q", string(content), "package main")
		}

		// Verify readme.txt does NOT exist in new worktree (not carried)
		notCarriedTxtFile := filepath.Join(wtPath, "readme.txt")
		if _, err := os.Stat(notCarriedTxtFile); !os.IsNotExist(err) {
			t.Errorf("readme.txt should not exist in new worktree: %s", notCarriedTxtFile)
		}

		// Verify main.go does NOT exist in source (carried away)
		if _, err := os.Stat(goFile); !os.IsNotExist(err) {
			t.Errorf("main.go should not exist in source after carry: %s", goFile)
		}

		// Verify readme.txt still exists in source (not carried)
		sourceContent, err := os.ReadFile(txtFile)
		if err != nil {
			t.Fatalf("failed to read source txt file: %v", err)
		}
		if string(sourceContent) != "readme content" {
			t.Errorf("source txt file content = %q, want %q", string(sourceContent), "readme content")
		}

		// Verify source has remaining changes (readme.txt)
		status := testutil.RunGit(t, mainDir, "status", "--porcelain")
		if !strings.Contains(status, "readme.txt") {
			t.Errorf("source should have readme.txt as untracked, got: %q", status)
		}
	})

	t.Run("CarryMultiplePatterns", func(t *testing.T) {
		t.Parallel()

		repoDir, mainDir := testutil.SetupTestRepo(t)

		// Commit .gwt/settings.toml first
		testutil.RunGit(t, mainDir, "add", ".gwt")
		testutil.RunGit(t, mainDir, "commit", "-m", "add gwt settings")

		// Create directory structure with multiple file types
		cmdDir := filepath.Join(mainDir, "cmd")
		if err := os.MkdirAll(cmdDir, 0755); err != nil {
			t.Fatal(err)
		}

		// Create files to carry
		file1 := filepath.Join(mainDir, "main.go")
		if err := os.WriteFile(file1, []byte("package main"), 0644); err != nil {
			t.Fatal(err)
		}
		file2 := filepath.Join(cmdDir, "app.go")
		if err := os.WriteFile(file2, []byte("package cmd"), 0644); err != nil {
			t.Fatal(err)
		}
		// File to leave behind
		file3 := filepath.Join(mainDir, "config.toml")
		if err := os.WriteFile(file3, []byte("[config]"), 0644); err != nil {
			t.Fatal(err)
		}

		result, err := LoadConfig(mainDir)
		if err != nil {
			t.Fatal(err)
		}

		// Carry *.go and cmd/**
		cmd := &AddCommand{
			FS:         osFS{},
			Git:        NewGitRunner(mainDir),
			Config:     result.Config,
			CarryFrom:  mainDir,
			CarryFiles: []string{"*.go", "cmd/**"},
		}

		_, err = cmd.Run("feature/carry-multiple")
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		wtPath := filepath.Join(repoDir, "feature", "carry-multiple")

		// Verify carried files exist in new worktree
		if _, err := os.Stat(filepath.Join(wtPath, "main.go")); os.IsNotExist(err) {
			t.Error("main.go should exist in new worktree")
		}
		if _, err := os.Stat(filepath.Join(wtPath, "cmd", "app.go")); os.IsNotExist(err) {
			t.Error("cmd/app.go should exist in new worktree")
		}

		// Verify config.toml does NOT exist in new worktree
		if _, err := os.Stat(filepath.Join(wtPath, "config.toml")); !os.IsNotExist(err) {
			t.Error("config.toml should not exist in new worktree")
		}

		// Verify config.toml still exists in source
		if _, err := os.Stat(file3); os.IsNotExist(err) {
			t.Error("config.toml should still exist in source")
		}
	})

	t.Run("CarryGlobstarPattern", func(t *testing.T) {
		t.Parallel()

		repoDir, mainDir := testutil.SetupTestRepo(t)

		// Commit .gwt/settings.toml first
		testutil.RunGit(t, mainDir, "add", ".gwt")
		testutil.RunGit(t, mainDir, "commit", "-m", "add gwt settings")

		// Create directory structure with Go files at different levels
		subDir := filepath.Join(mainDir, "sub", "deep")
		if err := os.MkdirAll(subDir, 0755); err != nil {
			t.Fatal(err)
		}

		// Files to carry (all .go)
		rootGo := filepath.Join(mainDir, "root.go")
		if err := os.WriteFile(rootGo, []byte("package main"), 0644); err != nil {
			t.Fatal(err)
		}
		subGo := filepath.Join(mainDir, "sub", "sub.go")
		if err := os.WriteFile(subGo, []byte("package sub"), 0644); err != nil {
			t.Fatal(err)
		}
		deepGo := filepath.Join(subDir, "deep.go")
		if err := os.WriteFile(deepGo, []byte("package deep"), 0644); err != nil {
			t.Fatal(err)
		}
		// File to leave behind
		otherFile := filepath.Join(mainDir, "other.txt")
		if err := os.WriteFile(otherFile, []byte("other"), 0644); err != nil {
			t.Fatal(err)
		}

		result, err := LoadConfig(mainDir)
		if err != nil {
			t.Fatal(err)
		}

		// Carry **/*.go - should match ALL Go files including root
		cmd := &AddCommand{
			FS:         osFS{},
			Git:        NewGitRunner(mainDir),
			Config:     result.Config,
			CarryFrom:  mainDir,
			CarryFiles: []string{"**/*.go"},
		}

		_, err = cmd.Run("feature/globstar")
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		wtPath := filepath.Join(repoDir, "feature", "globstar")

		// Verify ALL Go files exist in new worktree (including root)
		for _, rel := range []string{"root.go", "sub/sub.go", "sub/deep/deep.go"} {
			path := filepath.Join(wtPath, rel)
			if _, err := os.Stat(path); os.IsNotExist(err) {
				t.Errorf("%s should exist in new worktree", rel)
			}
		}

		// Verify other.txt does NOT exist in new worktree
		if _, err := os.Stat(filepath.Join(wtPath, "other.txt")); !os.IsNotExist(err) {
			t.Error("other.txt should not exist in new worktree")
		}

		// Verify Go files do NOT exist in source (carried away)
		for _, path := range []string{rootGo, subGo, deepGo} {
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Errorf("%s should not exist in source after carry", path)
			}
		}

		// Verify other.txt still exists in source
		if _, err := os.Stat(otherFile); os.IsNotExist(err) {
			t.Error("other.txt should still exist in source")
		}
	})
}
