package gwt

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

const (
	configFileName      = ".gwt.toml"
	localConfigFileName = ".gwt.local.toml"
)

type Config struct {
	Include []string `toml:"include"`
}

func LoadConfig(dir string) (*Config, error) {
	config := &Config{Include: []string{}}

	projectConfig, err := loadConfigFile(filepath.Join(dir, configFileName))
	if err != nil {
		return nil, err
	}
	if projectConfig != nil {
		config.Include = append(config.Include, projectConfig.Include...)
	}

	localConfig, err := loadConfigFile(filepath.Join(dir, localConfigFileName))
	if err != nil {
		return nil, err
	}
	if localConfig != nil {
		config.Include = append(config.Include, localConfig.Include...)
	}

	return config, nil
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
