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
	CleanStale          *bool    `toml:"clean_stale"`         // nil=unset, true=enable, false=disable
	Hooks               []string `toml:"hooks"`
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

// ShouldCleanStale returns whether --stale behavior is enabled by default for clean.
func (c *Config) ShouldCleanStale() bool {
	if c.CleanStale != nil {
		return *c.CleanStale
	}
	return false
}

// LoadConfigResult contains the loaded config and any warnings.
type LoadConfigResult struct {
	Config   *Config
	Warnings []string
}

type loadConfigOptions struct {
	mainWorktreeDir string
}

// LoadConfigOption configures LoadConfig behavior.
type LoadConfigOption func(*loadConfigOptions)

// WithMainWorktreeDir sets the main worktree directory used as the base
// for resolving relative WorktreeDestBaseDir paths and the default path.
// Without this option, the config load directory (dir) is used as fallback.
func WithMainWorktreeDir(dir string) LoadConfigOption {
	return func(o *loadConfigOptions) {
		o.mainWorktreeDir = dir
	}
}

func LoadConfig(dir string, opts ...LoadConfigOption) (*LoadConfigResult, error) {
	var o loadConfigOptions
	for _, opt := range opts {
		opt(&o)
	}

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

	// Resolve relative/default WorktreeDestBaseDir from main worktree root
	// so the result is consistent regardless of which worktree loads config.
	// Falls back to srcDir when main worktree dir is not provided.
	resolveBase := srcDir
	if o.mainWorktreeDir != "" {
		resolveBase = o.mainWorktreeDir
	}

	destBaseDir := destBaseDirConfig
	if destBaseDir == "" {
		repoName := filepath.Base(resolveBase)
		destBaseDir = filepath.Join(resolveBase, "..", repoName+"-worktree")
	} else if !filepath.IsAbs(destBaseDir) {
		destBaseDir = filepath.Join(resolveBase, destBaseDir)
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

	// clean_stale: local overrides project
	var cleanStale *bool
	if projCfg != nil && projCfg.CleanStale != nil {
		cleanStale = projCfg.CleanStale
	}
	if localCfg != nil && localCfg.CleanStale != nil {
		cleanStale = localCfg.CleanStale
	}

	// hooks: local overrides project
	var hooks []string
	if projCfg != nil && len(projCfg.Hooks) > 0 {
		hooks = projCfg.Hooks
	}
	if localCfg != nil && len(localCfg.Hooks) > 0 {
		hooks = localCfg.Hooks
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
			CleanStale:          cleanStale,
			Hooks:               hooks,
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
