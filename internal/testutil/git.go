package testutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// SetupTestRepo creates a temporary git repository for testing.
// Returns repoDir (parent directory) and mainDir (git repository root).
//
// TODO: Add option to create .gwt/settings.toml automatically.
// Many tests duplicate the same setup code for gwt settings.
func SetupTestRepo(t *testing.T) (repoDir, mainDir string) {
	t.Helper()

	tmpDir := t.TempDir()
	// Resolve symlinks for macOS (/var -> /private/var)
	tmpDir, _ = filepath.EvalSymlinks(tmpDir)
	repoDir = filepath.Join(tmpDir, "repo")
	mainDir = filepath.Join(repoDir, "main")

	if err := os.MkdirAll(mainDir, 0755); err != nil {
		t.Fatal(err)
	}

	RunGit(t, mainDir, "init")
	RunGit(t, mainDir, "config", "user.email", "test@example.com")
	RunGit(t, mainDir, "config", "user.name", "Test User")
	RunGit(t, mainDir, "commit", "--allow-empty", "-m", "initial")
	RunGit(t, mainDir, "branch", "-M", "main")

	return repoDir, mainDir
}

// RunGit executes a git command in the specified directory.
// Fails the test if the command fails.
func RunGit(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
	return string(out)
}
