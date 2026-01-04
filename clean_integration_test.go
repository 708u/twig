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

func TestCleanCommand_Integration(t *testing.T) {
	t.Parallel()

	t.Run("FindsMergedCandidates", func(t *testing.T) {
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

		// Create a merged branch
		wtPath := filepath.Join(repoDir, "feature", "merged")
		testutil.RunGit(t, mainDir, "worktree", "add", "-b", "feature/merged", wtPath)

		// Make a commit on the branch
		testFile := filepath.Join(wtPath, "test.txt")
		if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}
		testutil.RunGit(t, wtPath, "add", "test.txt")
		testutil.RunGit(t, wtPath, "commit", "-m", "test commit")

		// Merge the branch to main
		testutil.RunGit(t, mainDir, "merge", "feature/merged")

		cfgResult, err := LoadConfig(mainDir)
		if err != nil {
			t.Fatal(err)
		}

		cmd := &CleanCommand{
			FS:     osFS{},
			Git:    NewGitRunner(mainDir),
			Config: cfgResult.Config,
		}

		// Run in default mode (dry-run equivalent)
		result, err := cmd.Run(mainDir, CleanOptions{})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Should find the merged branch as candidate
		if len(result.Candidates) != 1 {
			t.Errorf("expected 1 candidate, got %d", len(result.Candidates))
		}

		if result.Candidates[0].Branch != "feature/merged" {
			t.Errorf("expected branch feature/merged, got %s", result.Candidates[0].Branch)
		}

		if result.Candidates[0].Skipped {
			t.Errorf("merged branch should not be skipped, reason: %s", result.Candidates[0].SkipReason)
		}
	})

	t.Run("SkipsUnmergedBranches", func(t *testing.T) {
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

		// Create an unmerged branch
		wtPath := filepath.Join(repoDir, "feature", "unmerged")
		testutil.RunGit(t, mainDir, "worktree", "add", "-b", "feature/unmerged", wtPath)

		// Make a commit that is NOT merged to main
		testFile := filepath.Join(wtPath, "test.txt")
		if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}
		testutil.RunGit(t, wtPath, "add", "test.txt")
		testutil.RunGit(t, wtPath, "commit", "-m", "unmerged commit")

		cfgResult, err := LoadConfig(mainDir)
		if err != nil {
			t.Fatal(err)
		}

		cmd := &CleanCommand{
			FS:     osFS{},
			Git:    NewGitRunner(mainDir),
			Config: cfgResult.Config,
		}

		result, err := cmd.Run(mainDir, CleanOptions{Verbose: true})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		if len(result.Candidates) != 1 {
			t.Errorf("expected 1 candidate, got %d", len(result.Candidates))
		}

		if !result.Candidates[0].Skipped {
			t.Error("unmerged branch should be skipped")
		}

		if result.Candidates[0].SkipReason != SkipNotMerged {
			t.Errorf("skip reason should be %s, got %s", SkipNotMerged, result.Candidates[0].SkipReason)
		}
	})

	t.Run("SkipsBranchWithChanges", func(t *testing.T) {
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

		// Create a branch (already merged since no new commits)
		wtPath := filepath.Join(repoDir, "feature", "with-changes")
		testutil.RunGit(t, mainDir, "worktree", "add", "-b", "feature/with-changes", wtPath)

		// Create uncommitted changes
		testFile := filepath.Join(wtPath, "uncommitted.txt")
		if err := os.WriteFile(testFile, []byte("uncommitted"), 0644); err != nil {
			t.Fatal(err)
		}

		cfgResult, err := LoadConfig(mainDir)
		if err != nil {
			t.Fatal(err)
		}

		cmd := &CleanCommand{
			FS:     osFS{},
			Git:    NewGitRunner(mainDir),
			Config: cfgResult.Config,
		}

		result, err := cmd.Run(mainDir, CleanOptions{Verbose: true})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		if len(result.Candidates) != 1 {
			t.Errorf("expected 1 candidate, got %d", len(result.Candidates))
		}

		if !result.Candidates[0].Skipped {
			t.Error("branch with changes should be skipped")
		}

		if result.Candidates[0].SkipReason != SkipHasChanges {
			t.Errorf("skip reason should be %s, got %s", SkipHasChanges, result.Candidates[0].SkipReason)
		}
	})

	t.Run("SkipsLockedWorktrees", func(t *testing.T) {
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

		wtPath := filepath.Join(repoDir, "feature", "locked")
		testutil.RunGit(t, mainDir, "worktree", "add", "-b", "feature/locked", wtPath)

		// Lock the worktree
		testutil.RunGit(t, mainDir, "worktree", "lock", wtPath)

		cfgResult, err := LoadConfig(mainDir)
		if err != nil {
			t.Fatal(err)
		}

		cmd := &CleanCommand{
			FS:     osFS{},
			Git:    NewGitRunner(mainDir),
			Config: cfgResult.Config,
		}

		result, err := cmd.Run(mainDir, CleanOptions{Verbose: true})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		if len(result.Candidates) != 1 {
			t.Errorf("expected 1 candidate, got %d", len(result.Candidates))
		}

		if !result.Candidates[0].Skipped {
			t.Error("locked worktree should be skipped")
		}

		if result.Candidates[0].SkipReason != SkipLocked {
			t.Errorf("skip reason should be %s, got %s", SkipLocked, result.Candidates[0].SkipReason)
		}
	})

	t.Run("SkipsCurrentDirectory", func(t *testing.T) {
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

		wtPath := filepath.Join(repoDir, "feature", "current")
		testutil.RunGit(t, mainDir, "worktree", "add", "-b", "feature/current", wtPath)

		cfgResult, err := LoadConfig(mainDir)
		if err != nil {
			t.Fatal(err)
		}

		cmd := &CleanCommand{
			FS:     osFS{},
			Git:    NewGitRunner(mainDir),
			Config: cfgResult.Config,
		}

		// Run with cwd inside the worktree
		result, err := cmd.Run(wtPath, CleanOptions{Verbose: true})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		if len(result.Candidates) != 1 {
			t.Errorf("expected 1 candidate, got %d", len(result.Candidates))
		}

		if !result.Candidates[0].Skipped {
			t.Error("current directory should be skipped")
		}

		if result.Candidates[0].SkipReason != SkipCurrentDir {
			t.Errorf("skip reason should be %s, got %s", SkipCurrentDir, result.Candidates[0].SkipReason)
		}
	})

	t.Run("ExecutesWithYesFlag", func(t *testing.T) {
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

		// Create a merged branch
		wtPath := filepath.Join(repoDir, "feature", "to-clean")
		testutil.RunGit(t, mainDir, "worktree", "add", "-b", "feature/to-clean", wtPath)

		cfgResult, err := LoadConfig(mainDir)
		if err != nil {
			t.Fatal(err)
		}

		cmd := &CleanCommand{
			FS:     osFS{},
			Git:    NewGitRunner(mainDir),
			Config: cfgResult.Config,
		}

		// Run with --yes
		result, err := cmd.Run(mainDir, CleanOptions{Yes: true})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Worktree should be removed
		if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
			t.Errorf("worktree should be removed: %s", wtPath)
		}

		// Branch should be deleted
		out := testutil.RunGit(t, mainDir, "branch", "--list", "feature/to-clean")
		if strings.TrimSpace(out) != "" {
			t.Errorf("branch should be deleted, got: %s", out)
		}

		// Should have one removed entry
		if len(result.Removed) != 1 {
			t.Errorf("expected 1 removed, got %d", len(result.Removed))
		}

		// Prune should be called
		if !result.Pruned {
			t.Error("prune should have been called")
		}
	})

	t.Run("UsesTargetFlag", func(t *testing.T) {
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

		// Create a develop branch
		developPath := filepath.Join(repoDir, "develop")
		testutil.RunGit(t, mainDir, "worktree", "add", "-b", "develop", developPath)

		// Make a commit on develop
		testFile := filepath.Join(developPath, "develop.txt")
		if err := os.WriteFile(testFile, []byte("develop"), 0644); err != nil {
			t.Fatal(err)
		}
		testutil.RunGit(t, developPath, "add", "develop.txt")
		testutil.RunGit(t, developPath, "commit", "-m", "develop commit")

		// Create a feature branch from develop
		featurePath := filepath.Join(repoDir, "feature", "from-develop")
		testutil.RunGit(t, developPath, "worktree", "add", "-b", "feature/from-develop", featurePath)

		// Merge feature to develop
		testutil.RunGit(t, developPath, "merge", "feature/from-develop")

		cfgResult, err := LoadConfig(mainDir)
		if err != nil {
			t.Fatal(err)
		}

		cmd := &CleanCommand{
			FS:     osFS{},
			Git:    NewGitRunner(mainDir),
			Config: cfgResult.Config,
		}

		// Check against develop (should find feature as merged)
		result, err := cmd.Run(mainDir, CleanOptions{Target: "develop"})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Should find feature/from-develop as cleanable (merged to develop)
		found := false
		for _, c := range result.Candidates {
			if c.Branch == "feature/from-develop" && !c.Skipped {
				found = true
				break
			}
		}
		if !found {
			t.Error("feature/from-develop should be cleanable when checked against develop")
		}
	})

	t.Run("UsesDefaultSourceConfig", func(t *testing.T) {
		t.Parallel()

		repoDir, mainDir := testutil.SetupTestRepo(t)

		gwtDir := filepath.Join(mainDir, ".gwt")
		if err := os.MkdirAll(gwtDir, 0755); err != nil {
			t.Fatal(err)
		}

		settingsContent := fmt.Sprintf(`worktree_source_dir = %q
worktree_destination_base_dir = %q
default_source = "main"
`, mainDir, repoDir)
		if err := os.WriteFile(filepath.Join(gwtDir, "settings.toml"), []byte(settingsContent), 0644); err != nil {
			t.Fatal(err)
		}

		wtPath := filepath.Join(repoDir, "feature", "test")
		testutil.RunGit(t, mainDir, "worktree", "add", "-b", "feature/test", wtPath)

		cfgResult, err := LoadConfig(mainDir)
		if err != nil {
			t.Fatal(err)
		}

		cmd := &CleanCommand{
			FS:     osFS{},
			Git:    NewGitRunner(mainDir),
			Config: cfgResult.Config,
		}

		result, err := cmd.Run(mainDir, CleanOptions{})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Target should be from config
		if result.TargetBranch != "main" {
			t.Errorf("target should be main from config, got %s", result.TargetBranch)
		}
	})

	t.Run("AutoDetectsTarget", func(t *testing.T) {
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

		wtPath := filepath.Join(repoDir, "feature", "test")
		testutil.RunGit(t, mainDir, "worktree", "add", "-b", "feature/test", wtPath)

		cfgResult, err := LoadConfig(mainDir)
		if err != nil {
			t.Fatal(err)
		}

		cmd := &CleanCommand{
			FS:     osFS{},
			Git:    NewGitRunner(mainDir),
			Config: cfgResult.Config,
		}

		result, err := cmd.Run(mainDir, CleanOptions{})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Target should be auto-detected as main
		if result.TargetBranch != "main" {
			t.Errorf("target should be auto-detected as main, got %s", result.TargetBranch)
		}
	})

	t.Run("CleansMultipleWorktrees", func(t *testing.T) {
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

		// Create multiple merged branches
		branches := []string{"feature/clean-a", "feature/clean-b", "feature/clean-c"}
		wtPaths := make([]string, len(branches))
		for i, branch := range branches {
			wtPaths[i] = filepath.Join(repoDir, "feature", fmt.Sprintf("clean-%c", 'a'+i))
			testutil.RunGit(t, mainDir, "worktree", "add", "-b", branch, wtPaths[i])
		}

		cfgResult, err := LoadConfig(mainDir)
		if err != nil {
			t.Fatal(err)
		}

		cmd := &CleanCommand{
			FS:     osFS{},
			Git:    NewGitRunner(mainDir),
			Config: cfgResult.Config,
		}

		// Run with --yes
		result, err := cmd.Run(mainDir, CleanOptions{Yes: true})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// All worktrees should be removed
		if len(result.Removed) != 3 {
			t.Errorf("expected 3 removed, got %d", len(result.Removed))
		}

		for _, wtPath := range wtPaths {
			if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
				t.Errorf("worktree should be removed: %s", wtPath)
			}
		}
	})

	t.Run("DryRunDoesNotRemove", func(t *testing.T) {
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

		wtPath := filepath.Join(repoDir, "feature", "dry-run")
		testutil.RunGit(t, mainDir, "worktree", "add", "-b", "feature/dry-run", wtPath)

		cfgResult, err := LoadConfig(mainDir)
		if err != nil {
			t.Fatal(err)
		}

		cmd := &CleanCommand{
			FS:     osFS{},
			Git:    NewGitRunner(mainDir),
			Config: cfgResult.Config,
		}

		// Run with --dry-run
		result, err := cmd.Run(mainDir, CleanOptions{DryRun: true})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Worktree should still exist
		if _, err := os.Stat(wtPath); os.IsNotExist(err) {
			t.Error("worktree should still exist in dry-run mode")
		}

		// Should have candidates but no removed
		if len(result.Candidates) != 1 {
			t.Errorf("expected 1 candidate, got %d", len(result.Candidates))
		}

		if len(result.Removed) != 0 {
			t.Errorf("expected 0 removed in dry-run, got %d", len(result.Removed))
		}
	})
}
