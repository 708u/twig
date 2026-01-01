package gwt

import (
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
type Config struct {
	Include             []string `toml:"include"`
	WorktreeDestBaseDir string   `toml:"worktree_destination_base_dir"`
	WorktreeSourceDir   string   `toml:"worktree_source_dir"`
}

// LoadConfigResult contains the loaded config and any warnings.
type LoadConfigResult struct {
	Config   *Config
	Warnings []string
}

func LoadConfig(dir string) (*LoadConfigResult, error) {
	seen := make(map[string]bool)
	var includes []string
	var warnings []string

	projCfg, err := loadConfigFile(filepath.Join(dir, configDir, configFileName))
	if err != nil {
		return nil, err
	}
	if projCfg != nil {
		for _, inc := range projCfg.Include {
			if !seen[inc] {
				seen[inc] = true
				includes = append(includes, inc)
			}
		}
	}

	localCfg, err := loadConfigFile(filepath.Join(dir, configDir, localConfigFileName))
	if err != nil {
		return nil, err
	}
	if localCfg != nil {
		for _, inc := range localCfg.Include {
			if !seen[inc] {
				seen[inc] = true
				includes = append(includes, inc)
			}
		}
		// Warn if local config contains project-level settings
		if localCfg.WorktreeDestBaseDir != "" {
			warnings = append(warnings, localConfigFileName+": 'worktree_destination_base_dir' is ignored (project-level setting)")
		}
		if localCfg.WorktreeSourceDir != "" {
			warnings = append(warnings, localConfigFileName+": 'worktree_source_dir' is ignored (project-level setting)")
		}
	}

	var destBaseDir string
	if projCfg != nil && projCfg.WorktreeDestBaseDir != "" {
		destBaseDir = projCfg.WorktreeDestBaseDir
	}

	srcDir := dir
	if projCfg != nil && projCfg.WorktreeSourceDir != "" {
		srcDir = projCfg.WorktreeSourceDir
	}

	return &LoadConfigResult{
		Config: &Config{
			Include:             includes,
			WorktreeDestBaseDir: destBaseDir,
			WorktreeSourceDir:   srcDir,
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
