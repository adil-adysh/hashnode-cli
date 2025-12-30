package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Publication struct {
	ID    string `yaml:"id"`
	Title string `yaml:"title"`
	URL   string `yaml:"url"`
}

type Config struct {
	Publications []Publication `yaml:"publications"`
	Token        string        `yaml:"token"`
}

func configDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		panic("Unable to determine user home directory")
	}
	return filepath.Join(home, ".hashnode-cli")
}

func ConfigPath() string {
	return filepath.Join(configDir(), "hashnode.yml")
}

// Load reads the config file from disk
func Load() (*Config, error) {
	data, err := os.ReadFile(ConfigPath())
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("invalid config file: %w", err)
	}
	return &cfg, nil
}

// Save writes the config to disk with restricted permissions
func (c *Config) Save() error {
	dir := configDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	// 0600 means only the owner can read/write this file
	return os.WriteFile(ConfigPath(), data, 0600)
}
