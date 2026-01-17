package twig

import (
	"context"
	"fmt"
	"path/filepath"
)

const settingsTemplate = `# twig project configuration
# See: https://github.com/708u/twig-worktree

# Default source branch for new worktrees (prevents symlink chaining)
default_source = "main"

# Symlink patterns to create in new worktrees
# Recommend: [".twig/settings.local.toml"] to share local settings across worktrees
symlinks = []

# Worktree destination base directory (default: ../<repo-name>-worktree)
# worktree_destination_base_dir = "../my-worktrees"

# Additional symlink patterns (collected from both project and local configs)
# extra_symlinks = [".envrc", ".tool-versions"]

# Initialize submodules when creating worktrees (default: false)
# init_submodules = true
`

// InitCommand initializes twig configuration in a directory.
type InitCommand struct {
	FS FileSystem
}

// InitOptions holds options for the init command.
type InitOptions struct {
	Force bool
}

// InitResult holds the result of the init command.
type InitResult struct {
	ConfigDir    string
	SettingsPath string
	Created      bool
	Skipped      bool
	Overwritten  bool
}

// InitFormatOptions holds formatting options for InitResult.
type InitFormatOptions struct {
	Verbose bool
}

// NewInitCommand creates an InitCommand with explicit dependencies (for testing).
func NewInitCommand(fs FileSystem) *InitCommand {
	return &InitCommand{
		FS: fs,
	}
}

// NewDefaultInitCommand creates an InitCommand with production defaults.
func NewDefaultInitCommand() *InitCommand {
	return NewInitCommand(osFS{})
}

// Run executes the init command.
func (c *InitCommand) Run(ctx context.Context, dir string, opts InitOptions) (InitResult, error) {
	configDirPath := filepath.Join(dir, configDir)
	settingsPath := filepath.Join(configDirPath, configFileName)

	result := InitResult{
		ConfigDir:    configDirPath,
		SettingsPath: settingsPath,
	}

	// Check if settings file already exists
	_, err := c.FS.Stat(settingsPath)
	exists := err == nil || !c.FS.IsNotExist(err)

	if exists && !opts.Force {
		result.Skipped = true
		return result, nil
	}

	// Create config directory
	if err := c.FS.MkdirAll(configDirPath, 0755); err != nil {
		return result, fmt.Errorf("failed to create config directory: %w", err)
	}

	// Write settings file
	if err := c.FS.WriteFile(settingsPath, []byte(settingsTemplate), 0644); err != nil {
		return result, fmt.Errorf("failed to write settings file: %w", err)
	}

	result.Created = true
	if exists {
		result.Overwritten = true
	}

	return result, nil
}

// Format formats the result for output.
func (r InitResult) Format(opts InitFormatOptions) FormatResult {
	var stdout string

	relPath := filepath.Join(configDir, configFileName)

	switch {
	case r.Skipped:
		stdout = fmt.Sprintf("Skipped %s (already exists)\n", relPath)
	case r.Overwritten:
		stdout = fmt.Sprintf("Created %s (overwritten)\n", relPath)
	case r.Created:
		stdout = fmt.Sprintf("Created %s\n", relPath)
	}

	return FormatResult{
		Stdout: stdout,
	}
}
