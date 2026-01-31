package twig

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

const (
	configDir           = ".twig"
	configFileName      = "settings.toml"
	localConfigFileName = "settings.local.toml"
)

// Config holds the merged configuration for the application.
// All path fields are resolved to absolute paths by LoadConfig.
type Config struct {
	Symlinks            []string `toml:"symlinks"`
	ExtraSymlinks       []string `toml:"extra_symlinks"`
	WorktreeDestBaseDir string   `toml:"worktree_destination_base_dir"`
	DefaultSource       string   `toml:"default_source"`
	WorktreeSourceDir   string   // Set by LoadConfig to the config load directory
	InitSubmodules      *bool    `toml:"init_submodules"`     // nil=unset, true=enable, false=disable
	SubmoduleReference  *bool    `toml:"submodule_reference"` // nil=unset, true=enable, false=disable
}

// ShouldInitSubmodules returns whether submodule initialization is enabled.
func (c *Config) ShouldInitSubmodules() bool {
	if c.InitSubmodules != nil {
		return *c.InitSubmodules
	}
	return false
}

// ShouldUseSubmoduleReference returns whether to use --reference for submodule init.
func (c *Config) ShouldUseSubmoduleReference() bool {
	if c.SubmoduleReference != nil {
		return *c.SubmoduleReference
	}
	return false
}

// LoadConfigResult contains the loaded config and any warnings.
type LoadConfigResult struct {
	Config   *Config
	Warnings []string
}

func LoadConfig(dir string) (*LoadConfigResult, error) {
	var warnings []string

	projCfg, err := loadConfigFile(filepath.Join(dir, configDir, configFileName))
	if err != nil {
		return nil, err
	}

	localCfg, err := loadConfigFile(filepath.Join(dir, configDir, localConfigFileName))
	if err != nil {
		return nil, err
	}

	// symlinks: local overrides project if local has any symlinks
	var symlinks []string
	if localCfg != nil && len(localCfg.Symlinks) > 0 {
		symlinks = localCfg.Symlinks
	} else if projCfg != nil {
		symlinks = projCfg.Symlinks
	}

	// extra_symlinks: collect from both configs, deduplicate, append to symlinks
	seen := make(map[string]bool)
	for _, s := range symlinks {
		seen[s] = true
	}
	var extraSymlinks []string
	if projCfg != nil {
		for _, s := range projCfg.ExtraSymlinks {
			if !seen[s] {
				seen[s] = true
				extraSymlinks = append(extraSymlinks, s)
			}
		}
	}
	if localCfg != nil {
		for _, s := range localCfg.ExtraSymlinks {
			if !seen[s] {
				seen[s] = true
				extraSymlinks = append(extraSymlinks, s)
			}
		}
	}
	symlinks = append(symlinks, extraSymlinks...)

	// default_source: local overrides project
	var defaultSource string
	if projCfg != nil && projCfg.DefaultSource != "" {
		defaultSource = projCfg.DefaultSource
	}
	if localCfg != nil && localCfg.DefaultSource != "" {
		defaultSource = localCfg.DefaultSource
	}

	// SourceDir is always the directory where config is loaded from
	srcDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve source directory: %w", err)
	}

	// worktree_destination_base_dir: local overrides project
	var destBaseDirConfig string
	if projCfg != nil && projCfg.WorktreeDestBaseDir != "" {
		destBaseDirConfig = projCfg.WorktreeDestBaseDir
	}
	if localCfg != nil && localCfg.WorktreeDestBaseDir != "" {
		destBaseDirConfig = localCfg.WorktreeDestBaseDir
	}
	destBaseDir := destBaseDirConfig
	if destBaseDir == "" {
		repoName := filepath.Base(srcDir)
		destBaseDir = filepath.Join(srcDir, "..", repoName+"-worktree")
	} else if !filepath.IsAbs(destBaseDir) {
		// Resolve relative paths based on the config file directory
		destBaseDir = filepath.Join(srcDir, destBaseDir)
	}
	destBaseDir, err = filepath.Abs(destBaseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve worktree destination base directory: %w", err)
	}

	// init_submodules: local overrides project
	var initSubmodules *bool
	if projCfg != nil && projCfg.InitSubmodules != nil {
		initSubmodules = projCfg.InitSubmodules
	}
	if localCfg != nil && localCfg.InitSubmodules != nil {
		initSubmodules = localCfg.InitSubmodules
	}

	// submodule_reference: local overrides project
	var submoduleReference *bool
	if projCfg != nil && projCfg.SubmoduleReference != nil {
		submoduleReference = projCfg.SubmoduleReference
	}
	if localCfg != nil && localCfg.SubmoduleReference != nil {
		submoduleReference = localCfg.SubmoduleReference
	}

	return &LoadConfigResult{
		Config: &Config{
			Symlinks:            symlinks,
			ExtraSymlinks:       extraSymlinks,
			WorktreeDestBaseDir: destBaseDir,
			DefaultSource:       defaultSource,
			WorktreeSourceDir:   srcDir,
			InitSubmodules:      initSubmodules,
			SubmoduleReference:  submoduleReference,
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
