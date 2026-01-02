package gwt

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

const (
	configDir           = ".gwt"
	configFileName      = "settings.toml"
	localConfigFileName = "settings.local.toml"
)

// Config holds the merged configuration for the application.
// All path fields are resolved to absolute paths by LoadConfig.
type Config struct {
	Symlinks            []string `toml:"symlinks"`
	WorktreeDestBaseDir string   `toml:"worktree_destination_base_dir"`
	WorktreeSourceDir   string   `toml:"worktree_source_dir"`
	DefaultSource       string   `toml:"default_source"`
}

// LoadConfigResult contains the loaded config and any warnings.
type LoadConfigResult struct {
	Config   *Config
	Warnings []string
}

func LoadConfig(dir string) (*LoadConfigResult, error) {
	seen := make(map[string]bool)
	var symlinks []string
	var warnings []string

	projCfg, err := loadConfigFile(filepath.Join(dir, configDir, configFileName))
	if err != nil {
		return nil, err
	}
	if projCfg != nil {
		for _, s := range projCfg.Symlinks {
			if !seen[s] {
				seen[s] = true
				symlinks = append(symlinks, s)
			}
		}
	}

	localCfg, err := loadConfigFile(filepath.Join(dir, configDir, localConfigFileName))
	if err != nil {
		return nil, err
	}
	// DefaultSource: project config value, can be overridden by local config
	var defaultSource string
	if projCfg != nil && projCfg.DefaultSource != "" {
		defaultSource = projCfg.DefaultSource
	}

	if localCfg != nil {
		for _, s := range localCfg.Symlinks {
			if !seen[s] {
				seen[s] = true
				symlinks = append(symlinks, s)
			}
		}
		// Local config can override default_source
		if localCfg.DefaultSource != "" {
			defaultSource = localCfg.DefaultSource
		}
		// Warn if local config contains project-level settings
		if localCfg.WorktreeDestBaseDir != "" {
			warnings = append(warnings, localConfigFileName+": 'worktree_destination_base_dir' is ignored (project-level setting)")
		}
		if localCfg.WorktreeSourceDir != "" {
			warnings = append(warnings, localConfigFileName+": 'worktree_source_dir' is ignored (project-level setting)")
		}
	}

	srcDir := dir
	if projCfg != nil && projCfg.WorktreeSourceDir != "" {
		srcDir = projCfg.WorktreeSourceDir
	}
	srcDir, err = filepath.Abs(srcDir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve worktree source directory: %w", err)
	}

	destBaseDir := ""
	if projCfg != nil && projCfg.WorktreeDestBaseDir != "" {
		destBaseDir = projCfg.WorktreeDestBaseDir
	}
	if destBaseDir == "" {
		repoName := filepath.Base(srcDir)
		destBaseDir = filepath.Join(srcDir, "..", repoName+"-worktree")
	}
	destBaseDir, err = filepath.Abs(destBaseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve worktree destination base directory: %w", err)
	}

	return &LoadConfigResult{
		Config: &Config{
			Symlinks:            symlinks,
			WorktreeDestBaseDir: destBaseDir,
			WorktreeSourceDir:   srcDir,
			DefaultSource:       defaultSource,
		},
		Warnings: warnings,
	}, nil
}

func loadConfigFile(path string) (*Config, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, nil
	}

	var config Config
	if _, err := toml.DecodeFile(path, &config); err != nil {
		return nil, err
	}

	return &config, nil
}
