//go:build integration

package twig

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/708u/twig/internal/testutil"
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

		twigDir := filepath.Join(mainDir, ".twig")
		if err := os.MkdirAll(twigDir, 0755); err != nil {
			t.Fatal(err)
		}

		// Empty config - worktree_destination_base_dir should default based on config load dir
		if err := os.WriteFile(filepath.Join(twigDir, "settings.toml"), []byte(""), 0644); err != nil {
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

	t.Run("LocalConfigSymlinksOverride", func(t *testing.T) {
		t.Parallel()

		repoDir, mainDir := testutil.SetupTestRepo(t, testutil.WithoutSettings())

		twigDir := filepath.Join(mainDir, ".twig")
		if err := os.MkdirAll(twigDir, 0755); err != nil {
			t.Fatal(err)
		}

		// Project config with .envrc and .config
		projectSettings := fmt.Sprintf(`symlinks = [".envrc", ".config"]
worktree_destination_base_dir = %q
`, repoDir)
		if err := os.WriteFile(filepath.Join(twigDir, "settings.toml"), []byte(projectSettings), 0644); err != nil {
			t.Fatal(err)
		}

		// Local config overrides with only .tool-versions
		localSettings := `symlinks = [".tool-versions"]
`
		if err := os.WriteFile(filepath.Join(twigDir, "settings.local.toml"), []byte(localSettings), 0644); err != nil {
			t.Fatal(err)
		}

		// Create source files
		if err := os.WriteFile(filepath.Join(mainDir, ".envrc"), []byte("# envrc"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(mainDir, ".config"), []byte("config"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(mainDir, ".tool-versions"), []byte("go 1.21"), 0644); err != nil {
			t.Fatal(err)
		}

		result, err := LoadConfig(mainDir)
		if err != nil {
			t.Fatal(err)
		}

		// Verify local overrides project (only .tool-versions)
		if len(result.Config.Symlinks) != 1 {
			t.Errorf("expected 1 symlink (local override), got %d: %v", len(result.Config.Symlinks), result.Config.Symlinks)
		}
		if result.Config.Symlinks[0] != ".tool-versions" {
			t.Errorf("expected symlink to be .tool-versions, got %v", result.Config.Symlinks)
		}

		cmd := &AddCommand{
			FS:     osFS{},
			Git:    NewGitRunner(mainDir),
			Config: result.Config,
		}

		_, err = cmd.Run("feature/local-override")
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		wtPath := filepath.Join(repoDir, "feature", "local-override")

		// Verify only .tool-versions is symlinked (local overrides project)
		linkPath := filepath.Join(wtPath, ".tool-versions")
		info, err := os.Lstat(linkPath)
		if err != nil {
			t.Errorf("symlink not created: .tool-versions")
		} else if info.Mode()&os.ModeSymlink == 0 {
			t.Errorf(".tool-versions is not a symlink")
		}

		// Verify .envrc and .config are NOT symlinked (overridden)
		for _, rel := range []string{".envrc", ".config"} {
			linkPath := filepath.Join(wtPath, rel)
			if _, err := os.Lstat(linkPath); err == nil {
				t.Errorf("%s should not be symlinked (local overrides project)", rel)
			}
		}
	})

	t.Run("ExtraSymlinksFromProjectConfig", func(t *testing.T) {
		t.Parallel()

		repoDir, mainDir := testutil.SetupTestRepo(t, testutil.WithoutSettings())

		twigDir := filepath.Join(mainDir, ".twig")
		if err := os.MkdirAll(twigDir, 0755); err != nil {
			t.Fatal(err)
		}

		// Project config with symlinks and extra_symlinks
		projectSettings := fmt.Sprintf(`symlinks = [".envrc"]
extra_symlinks = [".tool-versions"]
worktree_destination_base_dir = %q
`, repoDir)
		if err := os.WriteFile(filepath.Join(twigDir, "settings.toml"), []byte(projectSettings), 0644); err != nil {
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

		// Verify symlinks includes both
		if len(result.Config.Symlinks) != 2 {
			t.Errorf("expected 2 symlinks, got %d: %v", len(result.Config.Symlinks), result.Config.Symlinks)
		}

		cmd := &AddCommand{
			FS:     osFS{},
			Git:    NewGitRunner(mainDir),
			Config: result.Config,
		}

		_, err = cmd.Run("feature/extra-symlinks")
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		wtPath := filepath.Join(repoDir, "feature", "extra-symlinks")

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
		}
	})

	t.Run("ExtraSymlinksFromLocalConfig", func(t *testing.T) {
		t.Parallel()

		repoDir, mainDir := testutil.SetupTestRepo(t, testutil.WithoutSettings())

		twigDir := filepath.Join(mainDir, ".twig")
		if err := os.MkdirAll(twigDir, 0755); err != nil {
			t.Fatal(err)
		}

		// Project config with symlinks only
		projectSettings := fmt.Sprintf(`symlinks = [".envrc"]
worktree_destination_base_dir = %q
`, repoDir)
		if err := os.WriteFile(filepath.Join(twigDir, "settings.toml"), []byte(projectSettings), 0644); err != nil {
			t.Fatal(err)
		}

		// Local config adds extra_symlinks
		localSettings := `extra_symlinks = [".local-only"]
`
		if err := os.WriteFile(filepath.Join(twigDir, "settings.local.toml"), []byte(localSettings), 0644); err != nil {
			t.Fatal(err)
		}

		// Create source files
		if err := os.WriteFile(filepath.Join(mainDir, ".envrc"), []byte("# envrc"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(mainDir, ".local-only"), []byte("local"), 0644); err != nil {
			t.Fatal(err)
		}

		result, err := LoadConfig(mainDir)
		if err != nil {
			t.Fatal(err)
		}

		// Verify symlinks includes project symlinks + local extra_symlinks
		if len(result.Config.Symlinks) != 2 {
			t.Errorf("expected 2 symlinks, got %d: %v", len(result.Config.Symlinks), result.Config.Symlinks)
		}

		cmd := &AddCommand{
			FS:     osFS{},
			Git:    NewGitRunner(mainDir),
			Config: result.Config,
		}

		_, err = cmd.Run("feature/local-extra")
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		wtPath := filepath.Join(repoDir, "feature", "local-extra")

		// Verify both files are symlinked
		for _, rel := range []string{".envrc", ".local-only"} {
			linkPath := filepath.Join(wtPath, rel)
			info, err := os.Lstat(linkPath)
			if err != nil {
				t.Errorf("symlink not created: %s", rel)
				continue
			}
			if info.Mode()&os.ModeSymlink == 0 {
				t.Errorf("%s is not a symlink", rel)
			}
		}
	})

	t.Run("ExtraSymlinksFromBothConfigs", func(t *testing.T) {
		t.Parallel()

		repoDir, mainDir := testutil.SetupTestRepo(t, testutil.WithoutSettings())

		twigDir := filepath.Join(mainDir, ".twig")
		if err := os.MkdirAll(twigDir, 0755); err != nil {
			t.Fatal(err)
		}

		// Project config with symlinks and extra_symlinks
		projectSettings := fmt.Sprintf(`symlinks = [".envrc"]
extra_symlinks = [".project-extra"]
worktree_destination_base_dir = %q
`, repoDir)
		if err := os.WriteFile(filepath.Join(twigDir, "settings.toml"), []byte(projectSettings), 0644); err != nil {
			t.Fatal(err)
		}

		// Local config adds extra_symlinks
		localSettings := `extra_symlinks = [".local-extra"]
`
		if err := os.WriteFile(filepath.Join(twigDir, "settings.local.toml"), []byte(localSettings), 0644); err != nil {
			t.Fatal(err)
		}

		// Create source files
		if err := os.WriteFile(filepath.Join(mainDir, ".envrc"), []byte("# envrc"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(mainDir, ".project-extra"), []byte("project"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(mainDir, ".local-extra"), []byte("local"), 0644); err != nil {
			t.Fatal(err)
		}

		result, err := LoadConfig(mainDir)
		if err != nil {
			t.Fatal(err)
		}

		// Verify symlinks includes all: base + project extra + local extra
		if len(result.Config.Symlinks) != 3 {
			t.Errorf("expected 3 symlinks, got %d: %v", len(result.Config.Symlinks), result.Config.Symlinks)
		}

		cmd := &AddCommand{
			FS:     osFS{},
			Git:    NewGitRunner(mainDir),
			Config: result.Config,
		}

		_, err = cmd.Run("feature/both-extra")
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		wtPath := filepath.Join(repoDir, "feature", "both-extra")

		// Verify all files are symlinked
		for _, rel := range []string{".envrc", ".project-extra", ".local-extra"} {
			linkPath := filepath.Join(wtPath, rel)
			info, err := os.Lstat(linkPath)
			if err != nil {
				t.Errorf("symlink not created: %s", rel)
				continue
			}
			if info.Mode()&os.ModeSymlink == 0 {
				t.Errorf("%s is not a symlink", rel)
			}
		}
	})

	t.Run("LocalWorktreeDirOverride", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		tmpDir, _ = filepath.EvalSymlinks(tmpDir)
		mainDir := filepath.Join(tmpDir, "repo", "main")
		projectDestDir := filepath.Join(tmpDir, "project-worktrees")
		localDestDir := filepath.Join(tmpDir, "local-worktrees")

		if err := os.MkdirAll(mainDir, 0755); err != nil {
			t.Fatal(err)
		}

		testutil.RunGit(t, mainDir, "init")
		testutil.RunGit(t, mainDir, "config", "user.email", "test@example.com")
		testutil.RunGit(t, mainDir, "config", "user.name", "Test User")
		testutil.RunGit(t, mainDir, "commit", "--allow-empty", "-m", "initial")

		twigDir := filepath.Join(mainDir, ".twig")
		if err := os.MkdirAll(twigDir, 0755); err != nil {
			t.Fatal(err)
		}

		// Project config
		projectSettings := fmt.Sprintf(`worktree_destination_base_dir = %q
`, projectDestDir)
		if err := os.WriteFile(filepath.Join(twigDir, "settings.toml"), []byte(projectSettings), 0644); err != nil {
			t.Fatal(err)
		}

		// Local config overrides destination
		localSettings := fmt.Sprintf(`worktree_destination_base_dir = %q
`, localDestDir)
		if err := os.WriteFile(filepath.Join(twigDir, "settings.local.toml"), []byte(localSettings), 0644); err != nil {
			t.Fatal(err)
		}

		result, err := LoadConfig(mainDir)
		if err != nil {
			t.Fatal(err)
		}

		// Verify local override is applied
		if result.Config.WorktreeDestBaseDir != localDestDir {
			t.Errorf("WorktreeDestBaseDir = %q, want %q", result.Config.WorktreeDestBaseDir, localDestDir)
		}

		// Verify no warnings
		if len(result.Warnings) > 0 {
			t.Errorf("expected no warnings, got: %v", result.Warnings)
		}

		cmd := &AddCommand{
			FS:     osFS{},
			Git:    NewGitRunner(mainDir),
			Config: result.Config,
		}

		_, err = cmd.Run("feature/local-dest")
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Verify worktree created in local destination
		wtPath := filepath.Join(localDestDir, "feature", "local-dest")
		if _, err := os.Stat(wtPath); os.IsNotExist(err) {
			t.Errorf("worktree not created at local destination: %s", wtPath)
		}

		// Verify worktree NOT created in project destination
		projectWtPath := filepath.Join(projectDestDir, "feature", "local-dest")
		if _, err := os.Stat(projectWtPath); !os.IsNotExist(err) {
			t.Errorf("worktree should not exist at project destination: %s", projectWtPath)
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

		// Commit .twig/settings.toml first
		testutil.RunGit(t, mainDir, "add", ".twig")
		testutil.RunGit(t, mainDir, "commit", "-m", "add twig settings")

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

		// Commit .twig/settings.toml first
		testutil.RunGit(t, mainDir, "add", ".twig")
		testutil.RunGit(t, mainDir, "commit", "-m", "add twig settings")

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

		// Commit .twig/settings.toml first
		testutil.RunGit(t, mainDir, "add", ".twig")
		testutil.RunGit(t, mainDir, "commit", "-m", "add twig settings")

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

		// Commit .twig/settings.toml to ensure no uncommitted changes
		testutil.RunGit(t, mainDir, "add", ".twig")
		testutil.RunGit(t, mainDir, "commit", "-m", "add twig settings")

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

	t.Run("SyncSpecificFiles", func(t *testing.T) {
		t.Parallel()

		repoDir, mainDir := testutil.SetupTestRepo(t)

		// Commit .twig/settings.toml first
		testutil.RunGit(t, mainDir, "add", ".twig")
		testutil.RunGit(t, mainDir, "commit", "-m", "add twig settings")

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

		// Sync only *.go files
		cmd := &AddCommand{
			FS:           osFS{},
			Git:          NewGitRunner(mainDir),
			Config:       result.Config,
			Sync:         true,
			FilePatterns: []string{"*.go"},
		}

		addResult, err := cmd.Run("feature/sync-go-only")
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Verify ChangesSynced is true
		if !addResult.ChangesSynced {
			t.Error("expected ChangesSynced to be true")
		}

		// Verify worktree was created
		wtPath := filepath.Join(repoDir, "feature", "sync-go-only")
		if _, err := os.Stat(wtPath); os.IsNotExist(err) {
			t.Errorf("worktree directory does not exist: %s", wtPath)
		}

		// Verify main.go exists in new worktree (synced)
		syncedGoFile := filepath.Join(wtPath, "main.go")
		content, err := os.ReadFile(syncedGoFile)
		if err != nil {
			t.Fatalf("failed to read synced go file: %v", err)
		}
		if string(content) != "package main" {
			t.Errorf("synced go file content = %q, want %q", string(content), "package main")
		}

		// Verify readme.txt does NOT exist in new worktree (not synced)
		notSyncedTxtFile := filepath.Join(wtPath, "readme.txt")
		if _, err := os.Stat(notSyncedTxtFile); !os.IsNotExist(err) {
			t.Errorf("readme.txt should not exist in new worktree: %s", notSyncedTxtFile)
		}

		// Verify main.go still exists in source (unlike carry, sync keeps source files)
		sourceContent, err := os.ReadFile(goFile)
		if err != nil {
			t.Fatalf("failed to read source go file: %v", err)
		}
		if string(sourceContent) != "package main" {
			t.Errorf("source go file content = %q, want %q", string(sourceContent), "package main")
		}

		// Verify readme.txt still exists in source
		txtContent, err := os.ReadFile(txtFile)
		if err != nil {
			t.Fatalf("failed to read source txt file: %v", err)
		}
		if string(txtContent) != "readme content" {
			t.Errorf("source txt file content = %q, want %q", string(txtContent), "readme content")
		}
	})

	t.Run("SyncMultiplePatterns", func(t *testing.T) {
		t.Parallel()

		repoDir, mainDir := testutil.SetupTestRepo(t)

		// Commit .twig/settings.toml first
		testutil.RunGit(t, mainDir, "add", ".twig")
		testutil.RunGit(t, mainDir, "commit", "-m", "add twig settings")

		// Create directory structure with multiple file types
		cmdDir := filepath.Join(mainDir, "cmd")
		if err := os.MkdirAll(cmdDir, 0755); err != nil {
			t.Fatal(err)
		}

		// Create files: root *.go, cmd/app.go, readme.txt
		rootGo := filepath.Join(mainDir, "main.go")
		if err := os.WriteFile(rootGo, []byte("package main"), 0644); err != nil {
			t.Fatal(err)
		}
		cmdGo := filepath.Join(cmdDir, "app.go")
		if err := os.WriteFile(cmdGo, []byte("package cmd"), 0644); err != nil {
			t.Fatal(err)
		}
		txtFile := filepath.Join(mainDir, "readme.txt")
		if err := os.WriteFile(txtFile, []byte("readme"), 0644); err != nil {
			t.Fatal(err)
		}

		result, err := LoadConfig(mainDir)
		if err != nil {
			t.Fatal(err)
		}

		// Sync *.go and cmd/**
		cmd := &AddCommand{
			FS:           osFS{},
			Git:          NewGitRunner(mainDir),
			Config:       result.Config,
			Sync:         true,
			FilePatterns: []string{"*.go", "cmd/**"},
		}

		addResult, err := cmd.Run("feature/sync-multi-pattern")
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		if !addResult.ChangesSynced {
			t.Error("expected ChangesSynced to be true")
		}

		wtPath := filepath.Join(repoDir, "feature", "sync-multi-pattern")

		// Verify main.go synced
		if _, err := os.Stat(filepath.Join(wtPath, "main.go")); os.IsNotExist(err) {
			t.Error("main.go should exist in new worktree")
		}

		// Verify cmd/app.go synced
		if _, err := os.Stat(filepath.Join(wtPath, "cmd", "app.go")); os.IsNotExist(err) {
			t.Error("cmd/app.go should exist in new worktree")
		}

		// Verify readme.txt NOT synced
		if _, err := os.Stat(filepath.Join(wtPath, "readme.txt")); !os.IsNotExist(err) {
			t.Error("readme.txt should NOT exist in new worktree")
		}

		// Verify source files still exist (sync behavior)
		if _, err := os.Stat(rootGo); os.IsNotExist(err) {
			t.Error("main.go should still exist in source")
		}
		if _, err := os.Stat(cmdGo); os.IsNotExist(err) {
			t.Error("cmd/app.go should still exist in source")
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

		// Commit .twig/settings.toml first
		testutil.RunGit(t, mainDir, "add", ".twig")
		testutil.RunGit(t, mainDir, "commit", "-m", "add twig settings")

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
			FS:           osFS{},
			Git:          NewGitRunner(mainDir),
			Config:       result.Config,
			CarryFrom:    mainDir,
			FilePatterns: []string{"*.go"},
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

		// Commit .twig/settings.toml first
		testutil.RunGit(t, mainDir, "add", ".twig")
		testutil.RunGit(t, mainDir, "commit", "-m", "add twig settings")

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
			FS:           osFS{},
			Git:          NewGitRunner(mainDir),
			Config:       result.Config,
			CarryFrom:    mainDir,
			FilePatterns: []string{"*.go", "cmd/**"},
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

		// Commit .twig/settings.toml first
		testutil.RunGit(t, mainDir, "add", ".twig")
		testutil.RunGit(t, mainDir, "commit", "-m", "add twig settings")

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
			FS:           osFS{},
			Git:          NewGitRunner(mainDir),
			Config:       result.Config,
			CarryFrom:    mainDir,
			FilePatterns: []string{"**/*.go"},
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

	t.Run("RemoteBranchFetchAndCreateWorktree", func(t *testing.T) {
		t.Parallel()

		// Create a bare "origin" repository
		tmpDir := t.TempDir()
		tmpDir, _ = filepath.EvalSymlinks(tmpDir)
		originDir := filepath.Join(tmpDir, "origin.git")
		if err := os.MkdirAll(originDir, 0755); err != nil {
			t.Fatal(err)
		}
		testutil.RunGit(t, originDir, "init", "--bare")

		// Create main repo and push to origin
		mainDir := filepath.Join(tmpDir, "repo", "main")
		if err := os.MkdirAll(mainDir, 0755); err != nil {
			t.Fatal(err)
		}
		testutil.RunGit(t, mainDir, "init", "-b", "main")
		testutil.RunGit(t, mainDir, "config", "user.email", "test@example.com")
		testutil.RunGit(t, mainDir, "config", "user.name", "Test User")
		testutil.RunGit(t, mainDir, "commit", "--allow-empty", "-m", "initial")
		testutil.RunGit(t, mainDir, "remote", "add", "origin", originDir)
		testutil.RunGit(t, mainDir, "push", "-u", "origin", "main")

		// Create a branch on origin that doesn't exist locally
		// Clone origin to a temp location, create branch, push
		cloneDir := filepath.Join(tmpDir, "clone")
		testutil.RunGit(t, tmpDir, "clone", originDir, "clone")
		testutil.RunGit(t, cloneDir, "config", "user.email", "test@example.com")
		testutil.RunGit(t, cloneDir, "config", "user.name", "Test User")
		testutil.RunGit(t, cloneDir, "checkout", "-b", "feature/remote-only")
		if err := os.WriteFile(filepath.Join(cloneDir, "remote-file.txt"), []byte("from remote"), 0644); err != nil {
			t.Fatal(err)
		}
		testutil.RunGit(t, cloneDir, "add", ".")
		testutil.RunGit(t, cloneDir, "commit", "-m", "remote commit")
		testutil.RunGit(t, cloneDir, "push", "-u", "origin", "feature/remote-only")

		// Setup twig config
		twigDir := filepath.Join(mainDir, ".twig")
		if err := os.MkdirAll(twigDir, 0755); err != nil {
			t.Fatal(err)
		}
		repoDir := filepath.Join(tmpDir, "repo")
		settings := fmt.Sprintf(`worktree_destination_base_dir = %q
`, repoDir)
		if err := os.WriteFile(filepath.Join(twigDir, "settings.toml"), []byte(settings), 0644); err != nil {
			t.Fatal(err)
		}

		result, err := LoadConfig(mainDir)
		if err != nil {
			t.Fatal(err)
		}

		// Verify the branch doesn't exist locally
		out := testutil.RunGit(t, mainDir, "branch", "--list", "feature/remote-only")
		if strings.TrimSpace(out) != "" {
			t.Fatal("feature/remote-only should not exist locally before test")
		}

		// Fetch from origin to get remote-tracking branches
		// (like git checkout, twig checks local remote-tracking refs)
		testutil.RunGit(t, mainDir, "fetch", "origin")

		cmd := &AddCommand{
			FS:     osFS{},
			Git:    NewGitRunner(mainDir),
			Config: result.Config,
		}

		_, err = cmd.Run("feature/remote-only")
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Verify worktree was created
		wtPath := filepath.Join(repoDir, "feature", "remote-only")
		if _, err := os.Stat(wtPath); os.IsNotExist(err) {
			t.Errorf("worktree directory does not exist: %s", wtPath)
		}

		// Verify the remote file exists (proving we fetched from remote)
		remoteFile := filepath.Join(wtPath, "remote-file.txt")
		content, err := os.ReadFile(remoteFile)
		if err != nil {
			t.Fatalf("failed to read remote file: %v", err)
		}
		if string(content) != "from remote" {
			t.Errorf("remote file content = %q, want %q", string(content), "from remote")
		}

		// Verify worktree is listed
		listOut := testutil.RunGit(t, mainDir, "worktree", "list")
		if !strings.Contains(listOut, "feature/remote-only") {
			t.Errorf("worktree list should contain feature/remote-only: %s", listOut)
		}
	})

	t.Run("LocalBranchTakesPrecedenceOverRemote", func(t *testing.T) {
		t.Parallel()

		// Create a bare "origin" repository
		tmpDir := t.TempDir()
		tmpDir, _ = filepath.EvalSymlinks(tmpDir)
		originDir := filepath.Join(tmpDir, "origin.git")
		if err := os.MkdirAll(originDir, 0755); err != nil {
			t.Fatal(err)
		}
		testutil.RunGit(t, originDir, "init", "--bare")

		// Create main repo and push to origin
		mainDir := filepath.Join(tmpDir, "repo", "main")
		if err := os.MkdirAll(mainDir, 0755); err != nil {
			t.Fatal(err)
		}
		testutil.RunGit(t, mainDir, "init", "-b", "main")
		testutil.RunGit(t, mainDir, "config", "user.email", "test@example.com")
		testutil.RunGit(t, mainDir, "config", "user.name", "Test User")
		testutil.RunGit(t, mainDir, "commit", "--allow-empty", "-m", "initial")
		testutil.RunGit(t, mainDir, "remote", "add", "origin", originDir)
		testutil.RunGit(t, mainDir, "push", "-u", "origin", "main")

		// Create local branch with local-only content
		testutil.RunGit(t, mainDir, "branch", "feature/both-local-remote")
		testutil.RunGit(t, mainDir, "checkout", "feature/both-local-remote")
		if err := os.WriteFile(filepath.Join(mainDir, "local-file.txt"), []byte("from local"), 0644); err != nil {
			t.Fatal(err)
		}
		testutil.RunGit(t, mainDir, "add", ".")
		testutil.RunGit(t, mainDir, "commit", "-m", "local commit")
		testutil.RunGit(t, mainDir, "checkout", "main")

		// Push different content to origin under same branch name
		cloneDir := filepath.Join(tmpDir, "clone")
		testutil.RunGit(t, tmpDir, "clone", originDir, "clone")
		testutil.RunGit(t, cloneDir, "config", "user.email", "test@example.com")
		testutil.RunGit(t, cloneDir, "config", "user.name", "Test User")
		testutil.RunGit(t, cloneDir, "checkout", "-b", "feature/both-local-remote")
		if err := os.WriteFile(filepath.Join(cloneDir, "remote-file.txt"), []byte("from remote"), 0644); err != nil {
			t.Fatal(err)
		}
		testutil.RunGit(t, cloneDir, "add", ".")
		testutil.RunGit(t, cloneDir, "commit", "-m", "remote commit")
		testutil.RunGit(t, cloneDir, "push", "-u", "origin", "feature/both-local-remote")

		// Setup twig config
		twigDir := filepath.Join(mainDir, ".twig")
		if err := os.MkdirAll(twigDir, 0755); err != nil {
			t.Fatal(err)
		}
		repoDir := filepath.Join(tmpDir, "repo")
		settings := fmt.Sprintf(`worktree_destination_base_dir = %q
`, repoDir)
		if err := os.WriteFile(filepath.Join(twigDir, "settings.toml"), []byte(settings), 0644); err != nil {
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

		_, err = cmd.Run("feature/both-local-remote")
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Verify worktree was created
		wtPath := filepath.Join(repoDir, "feature", "both-local-remote")
		if _, err := os.Stat(wtPath); os.IsNotExist(err) {
			t.Errorf("worktree directory does not exist: %s", wtPath)
		}

		// Verify local file exists (proving we used local branch)
		localFile := filepath.Join(wtPath, "local-file.txt")
		content, err := os.ReadFile(localFile)
		if err != nil {
			t.Fatalf("failed to read local file: %v", err)
		}
		if string(content) != "from local" {
			t.Errorf("local file content = %q, want %q", string(content), "from local")
		}

		// Verify remote file does NOT exist (we didn't fetch from remote)
		remoteFile := filepath.Join(wtPath, "remote-file.txt")
		if _, err := os.Stat(remoteFile); !os.IsNotExist(err) {
			t.Errorf("remote-file.txt should not exist (local branch takes precedence)")
		}
	})

	t.Run("NoBranchAnywhere_CreatesNewBranch", func(t *testing.T) {
		t.Parallel()

		// Create a bare "origin" repository
		tmpDir := t.TempDir()
		tmpDir, _ = filepath.EvalSymlinks(tmpDir)
		originDir := filepath.Join(tmpDir, "origin.git")
		if err := os.MkdirAll(originDir, 0755); err != nil {
			t.Fatal(err)
		}
		testutil.RunGit(t, originDir, "init", "--bare")

		// Create main repo and push to origin
		mainDir := filepath.Join(tmpDir, "repo", "main")
		if err := os.MkdirAll(mainDir, 0755); err != nil {
			t.Fatal(err)
		}
		testutil.RunGit(t, mainDir, "init", "-b", "main")
		testutil.RunGit(t, mainDir, "config", "user.email", "test@example.com")
		testutil.RunGit(t, mainDir, "config", "user.name", "Test User")
		testutil.RunGit(t, mainDir, "commit", "--allow-empty", "-m", "initial")
		testutil.RunGit(t, mainDir, "remote", "add", "origin", originDir)
		testutil.RunGit(t, mainDir, "push", "-u", "origin", "main")

		// Setup twig config
		twigDir := filepath.Join(mainDir, ".twig")
		if err := os.MkdirAll(twigDir, 0755); err != nil {
			t.Fatal(err)
		}
		repoDir := filepath.Join(tmpDir, "repo")
		settings := fmt.Sprintf(`worktree_destination_base_dir = %q
`, repoDir)
		if err := os.WriteFile(filepath.Join(twigDir, "settings.toml"), []byte(settings), 0644); err != nil {
			t.Fatal(err)
		}

		result, err := LoadConfig(mainDir)
		if err != nil {
			t.Fatal(err)
		}

		// Verify the branch doesn't exist locally or on origin
		localOut := testutil.RunGit(t, mainDir, "branch", "--list", "feature/brand-new")
		if strings.TrimSpace(localOut) != "" {
			t.Fatal("feature/brand-new should not exist locally before test")
		}

		cmd := &AddCommand{
			FS:     osFS{},
			Git:    NewGitRunner(mainDir),
			Config: result.Config,
		}

		_, err = cmd.Run("feature/brand-new")
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Verify worktree was created
		wtPath := filepath.Join(repoDir, "feature", "brand-new")
		if _, err := os.Stat(wtPath); os.IsNotExist(err) {
			t.Errorf("worktree directory does not exist: %s", wtPath)
		}

		// Verify the branch was created (new local branch)
		branchOut := testutil.RunGit(t, mainDir, "branch", "--list", "feature/brand-new")
		if strings.TrimSpace(branchOut) == "" {
			t.Error("feature/brand-new should have been created as a new branch")
		}

		// Verify worktree is listed
		listOut := testutil.RunGit(t, mainDir, "worktree", "list")
		if !strings.Contains(listOut, "feature/brand-new") {
			t.Errorf("worktree list should contain feature/brand-new: %s", listOut)
		}
	})
}

// TestAddCommand_Submodules_Integration tests submodule initialization.
// Not parallel: uses t.Setenv for file:// protocol in local submodule URLs.
func TestAddCommand_Submodules_Integration(t *testing.T) {
	// Allow file:// protocol for local submodule URLs in tests
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "protocol.file.allow")
	t.Setenv("GIT_CONFIG_VALUE_0", "always")

	t.Run("InitSubmodulesEnabled", func(t *testing.T) {
		repoDir, mainDir := testutil.SetupTestRepo(t)

		// Create a submodule repository
		submoduleRepo := filepath.Join(repoDir, "submodule-repo")
		if err := os.MkdirAll(submoduleRepo, 0755); err != nil {
			t.Fatal(err)
		}
		testutil.RunGit(t, submoduleRepo, "init")
		testutil.RunGit(t, submoduleRepo, "config", "user.email", "test@example.com")
		testutil.RunGit(t, submoduleRepo, "config", "user.name", "Test")
		if err := os.WriteFile(filepath.Join(submoduleRepo, "submodule-file.txt"), []byte("submodule content"), 0644); err != nil {
			t.Fatal(err)
		}
		testutil.RunGit(t, submoduleRepo, "add", ".")
		testutil.RunGit(t, submoduleRepo, "commit", "-m", "initial")

		// Add submodule to main repo
		testutil.RunGit(t, mainDir, "submodule", "add", submoduleRepo, "mysub")
		testutil.RunGit(t, mainDir, "commit", "-m", "add submodule")

		// Setup twig config with init_submodules enabled
		twigDir := filepath.Join(mainDir, ".twig")
		settings := fmt.Sprintf(`worktree_destination_base_dir = %q
init_submodules = true
`, repoDir)
		if err := os.WriteFile(filepath.Join(twigDir, "settings.toml"), []byte(settings), 0644); err != nil {
			t.Fatal(err)
		}

		result, err := LoadConfig(mainDir)
		if err != nil {
			t.Fatal(err)
		}

		cmd := NewDefaultAddCommand(result.Config, AddOptions{})

		addResult, err := cmd.Run("feature/with-submodule")
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Verify worktree was created
		wtPath := filepath.Join(repoDir, "feature", "with-submodule")
		if _, err := os.Stat(wtPath); os.IsNotExist(err) {
			t.Errorf("worktree directory does not exist: %s", wtPath)
		}

		// Verify submodule was initialized
		submoduleFile := filepath.Join(wtPath, "mysub", "submodule-file.txt")
		content, err := os.ReadFile(submoduleFile)
		if err != nil {
			t.Fatalf("failed to read submodule file: %v", err)
		}
		if string(content) != "submodule content" {
			t.Errorf("submodule file content = %q, want %q", string(content), "submodule content")
		}

		// Verify result
		if !addResult.SubmoduleInit.Attempted {
			t.Error("expected SubmoduleInit.Attempted to be true")
		}
		if addResult.SubmoduleInit.Count != 1 {
			t.Errorf("SubmoduleInit.Count = %d, want 1", addResult.SubmoduleInit.Count)
		}
	})

	t.Run("InitSubmodulesDisabled", func(t *testing.T) {
		repoDir, mainDir := testutil.SetupTestRepo(t)

		// Create a submodule repository
		submoduleRepo := filepath.Join(repoDir, "submodule-repo")
		if err := os.MkdirAll(submoduleRepo, 0755); err != nil {
			t.Fatal(err)
		}
		testutil.RunGit(t, submoduleRepo, "init")
		testutil.RunGit(t, submoduleRepo, "config", "user.email", "test@example.com")
		testutil.RunGit(t, submoduleRepo, "config", "user.name", "Test")
		if err := os.WriteFile(filepath.Join(submoduleRepo, "submodule-file.txt"), []byte("submodule content"), 0644); err != nil {
			t.Fatal(err)
		}
		testutil.RunGit(t, submoduleRepo, "add", ".")
		testutil.RunGit(t, submoduleRepo, "commit", "-m", "initial")

		// Add submodule to main repo
		testutil.RunGit(t, mainDir, "submodule", "add", submoduleRepo, "mysub")
		testutil.RunGit(t, mainDir, "commit", "-m", "add submodule")

		// Setup twig config without init_submodules (default off)
		twigDir := filepath.Join(mainDir, ".twig")
		settings := fmt.Sprintf(`worktree_destination_base_dir = %q
`, repoDir)
		if err := os.WriteFile(filepath.Join(twigDir, "settings.toml"), []byte(settings), 0644); err != nil {
			t.Fatal(err)
		}

		result, err := LoadConfig(mainDir)
		if err != nil {
			t.Fatal(err)
		}

		cmd := NewDefaultAddCommand(result.Config, AddOptions{})

		addResult, err := cmd.Run("feature/no-submodule-init")
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Verify worktree was created
		wtPath := filepath.Join(repoDir, "feature", "no-submodule-init")
		if _, err := os.Stat(wtPath); os.IsNotExist(err) {
			t.Errorf("worktree directory does not exist: %s", wtPath)
		}

		// Verify submodule was NOT initialized (directory exists but empty)
		submoduleFile := filepath.Join(wtPath, "mysub", "submodule-file.txt")
		if _, err := os.Stat(submoduleFile); !os.IsNotExist(err) {
			t.Errorf("submodule file should not exist (submodule not initialized): %s", submoduleFile)
		}

		// Verify result
		if addResult.SubmoduleInit.Attempted {
			t.Error("expected SubmoduleInit.Attempted to be false")
		}
		if addResult.SubmoduleInit.Count != 0 {
			t.Errorf("SubmoduleInit.Count = %d, want 0", addResult.SubmoduleInit.Count)
		}
	})

	t.Run("InitSubmodulesCLIOverridesConfig", func(t *testing.T) {
		repoDir, mainDir := testutil.SetupTestRepo(t)

		// Create a submodule repository
		submoduleRepo := filepath.Join(repoDir, "submodule-repo")
		if err := os.MkdirAll(submoduleRepo, 0755); err != nil {
			t.Fatal(err)
		}
		testutil.RunGit(t, submoduleRepo, "init")
		testutil.RunGit(t, submoduleRepo, "config", "user.email", "test@example.com")
		testutil.RunGit(t, submoduleRepo, "config", "user.name", "Test")
		if err := os.WriteFile(filepath.Join(submoduleRepo, "submodule-file.txt"), []byte("submodule content"), 0644); err != nil {
			t.Fatal(err)
		}
		testutil.RunGit(t, submoduleRepo, "add", ".")
		testutil.RunGit(t, submoduleRepo, "commit", "-m", "initial")

		// Add submodule to main repo
		testutil.RunGit(t, mainDir, "submodule", "add", submoduleRepo, "mysub")
		testutil.RunGit(t, mainDir, "commit", "-m", "add submodule")

		// Setup twig config with init_submodules disabled
		twigDir := filepath.Join(mainDir, ".twig")
		settings := fmt.Sprintf(`worktree_destination_base_dir = %q
init_submodules = false
`, repoDir)
		if err := os.WriteFile(filepath.Join(twigDir, "settings.toml"), []byte(settings), 0644); err != nil {
			t.Fatal(err)
		}

		result, err := LoadConfig(mainDir)
		if err != nil {
			t.Fatal(err)
		}

		// CLI flag forces init_submodules (regardless of config)
		cmd := NewDefaultAddCommand(result.Config, AddOptions{
			InitSubmodules: true,
		})

		addResult, err := cmd.Run("feature/cli-override")
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Verify worktree was created
		wtPath := filepath.Join(repoDir, "feature", "cli-override")
		if _, err := os.Stat(wtPath); os.IsNotExist(err) {
			t.Errorf("worktree directory does not exist: %s", wtPath)
		}

		// Verify submodule WAS initialized (CLI override)
		submoduleFile := filepath.Join(wtPath, "mysub", "submodule-file.txt")
		content, err := os.ReadFile(submoduleFile)
		if err != nil {
			t.Fatalf("failed to read submodule file: %v", err)
		}
		if string(content) != "submodule content" {
			t.Errorf("submodule file content = %q, want %q", string(content), "submodule content")
		}

		// Verify result
		if !addResult.SubmoduleInit.Attempted {
			t.Error("expected SubmoduleInit.Attempted to be true")
		}
		if addResult.SubmoduleInit.Count != 1 {
			t.Errorf("SubmoduleInit.Count = %d, want 1", addResult.SubmoduleInit.Count)
		}
	})
}
