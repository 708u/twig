//go:build integration

package gwt

import (
	"bytes"
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

		var stdout, stderr bytes.Buffer
		cmd := &AddCommand{
			FS:     osFS{},
			Git:    newTestGitRunner(mainDir, &stdout),
			Config: result.Config,
			Stdout: &stdout,
			Stderr: &stderr,
		}

		err = cmd.Run("feature/test")
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		wtPath := filepath.Join(repoDir, "feature-test")
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
		if !strings.Contains(out, "feature-test") {
			t.Errorf("worktree list does not contain feature-test: %s", out)
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

		var stdout, stderr bytes.Buffer
		cmd := &AddCommand{
			FS:     osFS{},
			Git:    newTestGitRunner(mainDir, &stdout),
			Config: result.Config,
			Stdout: &stdout,
			Stderr: &stderr,
		}

		err = cmd.Run("existing-branch")
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

		var stdout, stderr bytes.Buffer
		cmd := &AddCommand{
			FS:     osFS{},
			Git:    newTestGitRunner(mainDir, &stdout),
			Config: result.Config,
			Stdout: &stdout,
			Stderr: &stderr,
		}

		err = cmd.Run("test-branch")
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

		var stdout, stderr bytes.Buffer
		cmd := &AddCommand{
			FS:     osFS{},
			Git:    newTestGitRunner(mainDir, &stdout),
			Config: result.Config,
			Stdout: &stdout,
			Stderr: &stderr,
		}

		err = cmd.Run("feature/local-merge")
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		wtPath := filepath.Join(repoDir, "feature-local-merge")

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

	t.Run("GlobPatternSymlinks", func(t *testing.T) {
		t.Parallel()

		repoDir, mainDir := testutil.SetupTestRepo(t)

		matchFiles := []string{"config/app.toml", "config/env/dev.toml"}
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

		settingsContent := fmt.Sprintf(`symlinks = ["config/**/*.toml"]
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

		var stdout, stderr bytes.Buffer
		cmd := &AddCommand{
			FS:     osFS{},
			Git:    newTestGitRunner(mainDir, &stdout),
			Config: result.Config,
			Stdout: &stdout,
			Stderr: &stderr,
		}

		err = cmd.Run("feature/glob-test")
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		wtPath := filepath.Join(repoDir, "feature-glob-test")

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
}

func newTestGitRunner(dir string, stdout *bytes.Buffer) *GitRunner {
	runner := NewGitRunner(dir)
	runner.Stdout = stdout
	return runner
}
