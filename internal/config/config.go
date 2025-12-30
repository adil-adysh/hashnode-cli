package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	PublicationID string `yaml:"publicationId"`
	Token         string `yaml:"token"`
}

// Changed to hashnode.yml as requested
const ConfigFile = "hashnode.yml"

// Load reads the config file from disk
func Load() (*Config, error) {
	data, err := os.ReadFile(ConfigFile)
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
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	// 0600 means only the owner can read/write this file
	return os.WriteFile(ConfigFile, data, 0600)
}
