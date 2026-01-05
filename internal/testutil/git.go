package testutil

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// SetupOption configures SetupTestRepo behavior.
type SetupOption func(*setupConfig)

type setupConfig struct {
	skipSettings  bool
	symlinks      []string
	extraSymlinks []string
	defaultSource string
}

// WithoutSettings skips creating .twig/settings.toml.
func WithoutSettings() SetupOption {
	return func(c *setupConfig) {
		c.skipSettings = true
	}
}

// Symlinks sets the symlinks patterns for settings.toml.
// If not called, no symlinks are configured.
func Symlinks(patterns ...string) SetupOption {
	return func(c *setupConfig) {
		c.symlinks = patterns
	}
}

// ExtraSymlinks sets the extra_symlinks patterns for settings.toml.
func ExtraSymlinks(patterns ...string) SetupOption {
	return func(c *setupConfig) {
		c.extraSymlinks = patterns
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
// By default, creates .twig/settings.toml without symlinks.
// Use Symlinks(...) to add symlink patterns.
// Use WithoutSettings() to skip creating settings entirely.
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

	twigDir := filepath.Join(mainDir, ".twig")
	if err := os.MkdirAll(twigDir, 0755); err != nil {
		t.Fatal(err)
	}

	content := fmt.Sprintf("worktree_destination_base_dir = %q\n", repoDir)

	if len(cfg.symlinks) > 0 {
		quoted := make([]string, len(cfg.symlinks))
		for i, s := range cfg.symlinks {
			quoted[i] = fmt.Sprintf("%q", s)
		}
		content += fmt.Sprintf("symlinks = [%s]\n", strings.Join(quoted, ", "))
	}

	if len(cfg.extraSymlinks) > 0 {
		quoted := make([]string, len(cfg.extraSymlinks))
		for i, s := range cfg.extraSymlinks {
			quoted[i] = fmt.Sprintf("%q", s)
		}
		content += fmt.Sprintf("extra_symlinks = [%s]\n", strings.Join(quoted, ", "))
	}

	if cfg.defaultSource != "" {
		content += fmt.Sprintf("default_source = %q\n", cfg.defaultSource)
	}

	settingsPath := filepath.Join(twigDir, "settings.toml")
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
