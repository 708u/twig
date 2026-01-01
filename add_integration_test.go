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

	// 他の正常ケースも探索
	t.Run("FullWorkflow", func(t *testing.T) {
		t.Parallel()

		repoDir, mainDir := testutil.SetupTestRepo(t)

		gwtDir := filepath.Join(mainDir, ".gwt")
		if err := os.MkdirAll(gwtDir, 0755); err != nil {
			t.Fatal(err)
		}

		settingsContent := fmt.Sprintf(`include = [".envrc"]
worktree_source_dir = %q
worktree_destination_base_dir = %q
`, mainDir, repoDir)
		if err := os.WriteFile(filepath.Join(gwtDir, "settings.toml"), []byte(settingsContent), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(mainDir, ".envrc"), []byte("# envrc"), 0644); err != nil {
			t.Fatal(err)
		}

		cfg, err := LoadConfig(mainDir)
		if err != nil {
			t.Fatal(err)
		}

		var stdout, stderr bytes.Buffer
		cmd := &AddCommand{
			FS:     osFS{},
			Git:    newTestGitRunner(mainDir, &stdout),
			Config: cfg,
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

		cfg, err := LoadConfig(mainDir)
		if err != nil {
			t.Fatal(err)
		}

		var stdout, stderr bytes.Buffer
		cmd := &AddCommand{
			FS:     osFS{},
			Git:    newTestGitRunner(mainDir, &stdout),
			Config: cfg,
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

		cfg, err := LoadConfig(mainDir)
		if err != nil {
			t.Fatal(err)
		}

		var stdout, stderr bytes.Buffer
		cmd := &AddCommand{
			FS:     osFS{},
			Git:    newTestGitRunner(mainDir, &stdout),
			Config: cfg,
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
}

func newTestGitRunner(dir string, stdout *bytes.Buffer) *GitRunner {
	runner := NewGitRunner(dir)
	runner.Stdout = stdout
	return runner
}
