package testutil

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// SetupOption configures SetupTestRepo behavior.
type SetupOption func(*setupConfig)

type setupConfig struct {
	skipSettings   bool
	symlinks       []string
	symlinksCalled bool
	defaultSource  string
}

// WithoutSettings skips creating .gwt/settings.toml.
func WithoutSettings() SetupOption {
	return func(c *setupConfig) {
		c.skipSettings = true
	}
}

// Symlinks sets the symlinks patterns for settings.toml.
// If not called, defaults to [".envrc"].
func Symlinks(patterns ...string) SetupOption {
	return func(c *setupConfig) {
		c.symlinksCalled = true
		c.symlinks = patterns
	}
}

// DefaultSource sets the default_source field in settings.toml.
func DefaultSource(branch string) SetupOption {
	return func(c *setupConfig) {
		c.defaultSource = branch
	}
}

// SetupTestRepo creates a temporary git repository for testing.
// Returns repoDir (parent directory) and mainDir (git repository root).
//
// By default, creates .gwt/settings.toml with symlinks = [".envrc"].
// Use WithoutSettings() to skip creating settings.
func SetupTestRepo(t *testing.T, opts ...SetupOption) (repoDir, mainDir string) {
	t.Helper()

	cfg := &setupConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

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

	if !cfg.skipSettings {
		createSettings(t, repoDir, mainDir, cfg)
	}

	return repoDir, mainDir
}

func createSettings(t *testing.T, repoDir, mainDir string, cfg *setupConfig) {
	t.Helper()

	gwtDir := filepath.Join(mainDir, ".gwt")
	if err := os.MkdirAll(gwtDir, 0755); err != nil {
		t.Fatal(err)
	}

	symlinks := cfg.symlinks
	if !cfg.symlinksCalled {
		symlinks = []string{".envrc"}
	}

	content := fmt.Sprintf("worktree_source_dir = %q\n", mainDir)
	content += fmt.Sprintf("worktree_destination_base_dir = %q\n", repoDir)

	if len(symlinks) > 0 {
		content += "symlinks = ["
		for i, s := range symlinks {
			if i > 0 {
				content += ", "
			}
			content += fmt.Sprintf("%q", s)
		}
		content += "]\n"
	}

	if cfg.defaultSource != "" {
		content += fmt.Sprintf("default_source = %q\n", cfg.defaultSource)
	}

	settingsPath := filepath.Join(gwtDir, "settings.toml")
	if err := os.WriteFile(settingsPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
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
