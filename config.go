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

type Config struct {
	Include             []string `toml:"include"`
	WorktreeDestBaseDir string   `toml:"worktree_destination_base_dir"`
	WorktreeSourceDir   string   `toml:"worktree_source_dir"`
}

func LoadConfig(dir string) (*Config, error) {
	seen := make(map[string]bool)
	var includes []string

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
	}

	// TODO: projectとlocalで共通の値をどうするか考える
	var destBaseDir string
	if projCfg != nil && projCfg.WorktreeDestBaseDir != "" {
		destBaseDir = projCfg.WorktreeDestBaseDir
	}
	if localCfg != nil && localCfg.WorktreeDestBaseDir != "" {
		destBaseDir = localCfg.WorktreeDestBaseDir
	}

	srcDir := dir
	if projCfg != nil && projCfg.WorktreeSourceDir != "" {
		srcDir = projCfg.WorktreeSourceDir
	}
	if localCfg != nil && localCfg.WorktreeSourceDir != "" {
		srcDir = localCfg.WorktreeSourceDir
	}

	return &Config{
		Include:             includes,
		WorktreeDestBaseDir: destBaseDir,
		WorktreeSourceDir:   srcDir,
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
