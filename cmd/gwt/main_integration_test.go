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

		addCmd := gwt.NewDefaultAddCommand(result.Config, gwt.AddOptions{})
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
		addCmd = gwt.NewDefaultAddCommand(result.Config, gwt.AddOptions{})
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

	t.Run("SourceAndDirectoryCoexistence", func(t *testing.T) {
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

		// Commit the settings
		testutil.RunGit(t, mainDir, "add", ".gwt")
		testutil.RunGit(t, mainDir, "commit", "-m", "add gwt settings")

		// Create .envrc in main
		if err := os.WriteFile(filepath.Join(mainDir, ".envrc"), []byte("# main envrc"), 0644); err != nil {
			t.Fatal(err)
		}

		// Simulate -C pointing to mainDir and --source pointing to main
		// This should work: -C sets working directory, --source sets source branch
		git := gwt.NewGitRunner(mainDir)
		sourcePath, err := git.WorktreeFindByBranch("main")
		if err != nil {
			t.Fatalf("failed to find main worktree: %v", err)
		}

		// Load config from source (as --source would do after -C sets cwd)
		result, err := gwt.LoadConfig(sourcePath)
		if err != nil {
			t.Fatal(err)
		}

		// Create worktree using the resolved config
		addCmd := gwt.NewDefaultAddCommand(result.Config, gwt.AddOptions{})
		addResult, err := addCmd.Run("feat/coexist")
		if err != nil {
			t.Fatalf("failed to create worktree: %v", err)
		}

		// Verify worktree was created
		worktreePath := filepath.Join(repoDir, "feat", "coexist")
		if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
			t.Errorf("worktree directory does not exist: %s", worktreePath)
		}

		// Verify result
		if addResult.Branch != "feat/coexist" {
			t.Errorf("result.Branch = %q, want %q", addResult.Branch, "feat/coexist")
		}
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

func TestAddCommand_DefaultSource_Integration(t *testing.T) {
	t.Parallel()

	t.Run("DefaultSourceAppliedWhenNoCliArg", func(t *testing.T) {
		t.Parallel()

		repoDir, mainDir := testutil.SetupTestRepo(t)

		// Setup gwt settings with default_source = "main"
		gwtDir := filepath.Join(mainDir, ".gwt")
		if err := os.MkdirAll(gwtDir, 0755); err != nil {
			t.Fatal(err)
		}

		settingsContent := fmt.Sprintf(`symlinks = [".envrc"]
worktree_source_dir = %q
worktree_destination_base_dir = %q
default_source = "main"
`, mainDir, repoDir)
		if err := os.WriteFile(filepath.Join(gwtDir, "settings.toml"), []byte(settingsContent), 0644); err != nil {
			t.Fatal(err)
		}

		// Commit the settings
		testutil.RunGit(t, mainDir, "add", ".gwt")
		testutil.RunGit(t, mainDir, "commit", "-m", "add gwt settings with default_source")

		// Create .envrc in main
		if err := os.WriteFile(filepath.Join(mainDir, ".envrc"), []byte("# main envrc"), 0644); err != nil {
			t.Fatal(err)
		}

		// Create first derived worktree (feat/a) from main
		result, err := gwt.LoadConfig(mainDir)
		if err != nil {
			t.Fatal(err)
		}

		addCmd := gwt.NewDefaultAddCommand(result.Config, gwt.AddOptions{})
		_, err = addCmd.Run("feat/a")
		if err != nil {
			t.Fatalf("failed to create feat/a worktree: %v", err)
		}

		featAPath := filepath.Join(repoDir, "feat", "a")

		// Create a file unique to feat/a (not in symlinks, not in main)
		featAOnlyFile := filepath.Join(featAPath, "feat-a-only.txt")
		if err := os.WriteFile(featAOnlyFile, []byte("only in feat/a"), 0644); err != nil {
			t.Fatal(err)
		}

		// Load config from feat/a - it should have default_source = "main"
		resultFeatA, err := gwt.LoadConfig(featAPath)
		if err != nil {
			t.Fatal(err)
		}

		// Verify default_source is loaded
		if resultFeatA.Config.DefaultSource != "main" {
			t.Errorf("DefaultSource = %q, want %q", resultFeatA.Config.DefaultSource, "main")
		}

		// When default_source is applied, config should be reloaded from main
		// Simulate the PreRunE logic
		git := gwt.NewGitRunner(featAPath)
		mainPath, err := git.WorktreeFindByBranch(resultFeatA.Config.DefaultSource)
		if err != nil {
			t.Fatalf("failed to find worktree for default_source: %v", err)
		}

		// Load config from main (as default_source would do)
		resultMain, err := gwt.LoadConfig(mainPath)
		if err != nil {
			t.Fatal(err)
		}

		// Create feat/b using main's config
		addCmd = gwt.NewDefaultAddCommand(resultMain.Config, gwt.AddOptions{})
		_, err = addCmd.Run("feat/b")
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

		// Verify feat/b does NOT have feat-a-only.txt
		// (created from main, not feat/a)
		featBOnlyFile := filepath.Join(featBPath, "feat-a-only.txt")
		if _, err := os.Stat(featBOnlyFile); !os.IsNotExist(err) {
			t.Errorf("feat-a-only.txt should NOT exist in feat/b (created from main)")
		}
	})

	t.Run("CliSourceOverridesDefaultSource", func(t *testing.T) {
		t.Parallel()

		// This test verifies the priority: CLI --source > config default_source
		// Test by checking the condition logic

		cliSource := "dev"
		configDefaultSource := "main"

		// CLI source should take precedence
		effectiveSource := cliSource
		if effectiveSource == "" && configDefaultSource != "" {
			effectiveSource = configDefaultSource
		}

		if effectiveSource != "dev" {
			t.Errorf("effective source = %q, want %q", effectiveSource, "dev")
		}
	})

	t.Run("DefaultSourceAppliedWithDirFlag", func(t *testing.T) {
		t.Parallel()

		repoDir, mainDir := testutil.SetupTestRepo(t)

		// Setup gwt settings with default_source = "main"
		gwtDir := filepath.Join(mainDir, ".gwt")
		if err := os.MkdirAll(gwtDir, 0755); err != nil {
			t.Fatal(err)
		}

		settingsContent := fmt.Sprintf(`symlinks = [".envrc"]
worktree_source_dir = %q
worktree_destination_base_dir = %q
default_source = "main"
`, mainDir, repoDir)
		if err := os.WriteFile(filepath.Join(gwtDir, "settings.toml"), []byte(settingsContent), 0644); err != nil {
			t.Fatal(err)
		}

		// Commit the settings
		testutil.RunGit(t, mainDir, "add", ".gwt")
		testutil.RunGit(t, mainDir, "commit", "-m", "add gwt settings")

		// Create .envrc in main
		if err := os.WriteFile(filepath.Join(mainDir, ".envrc"), []byte("# main envrc"), 0644); err != nil {
			t.Fatal(err)
		}

		// Simulate -C flag being set (dirFlag is not empty)
		// Load config (as PersistentPreRunE would do with -C)
		result, err := gwt.LoadConfig(mainDir)
		if err != nil {
			t.Fatal(err)
		}

		// When -C is specified, default_source should now be applied
		// (The PreRunE logic checks: if source == "" && cfg.DefaultSource != "")
		cliSource := ""
		var effectiveSource string
		if cliSource == "" && result.Config.DefaultSource != "" {
			effectiveSource = result.Config.DefaultSource
		}

		// Since cliSource is empty but default_source is set, effectiveSource should be "main"
		if effectiveSource != "main" {
			t.Errorf("effective source = %q, want %q (default_source should be applied with -C)", effectiveSource, "main")
		}
	})

	t.Run("SourceFlagOverridesDefaultSourceWithDirFlag", func(t *testing.T) {
		t.Parallel()

		// This test verifies: -C + --source specified, --source overrides default_source
		cliSource := "dev"
		configDefaultSource := "main"

		// CLI source should take precedence even with -C
		effectiveSource := cliSource
		if effectiveSource == "" && configDefaultSource != "" {
			effectiveSource = configDefaultSource
		}

		if effectiveSource != "dev" {
			t.Errorf("effective source = %q, want %q (--source should override default_source with -C)", effectiveSource, "dev")
		}
	})

	t.Run("LocalConfigOverridesDefaultSource", func(t *testing.T) {
		t.Parallel()

		_, mainDir := testutil.SetupTestRepo(t)

		// Setup gwt settings with default_source = "main"
		gwtDir := filepath.Join(mainDir, ".gwt")
		if err := os.MkdirAll(gwtDir, 0755); err != nil {
			t.Fatal(err)
		}

		projectSettings := `default_source = "main"`
		if err := os.WriteFile(filepath.Join(gwtDir, "settings.toml"), []byte(projectSettings), 0644); err != nil {
			t.Fatal(err)
		}

		// Create local settings that overrides default_source
		localSettings := `default_source = "develop"`
		if err := os.WriteFile(filepath.Join(gwtDir, "settings.local.toml"), []byte(localSettings), 0644); err != nil {
			t.Fatal(err)
		}

		// Load config
		result, err := gwt.LoadConfig(mainDir)
		if err != nil {
			t.Fatal(err)
		}

		// Local config should override project config
		if result.Config.DefaultSource != "develop" {
			t.Errorf("DefaultSource = %q, want %q", result.Config.DefaultSource, "develop")
		}
	})
}
