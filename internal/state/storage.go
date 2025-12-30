package state

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// RemoteIdentity tracks the connection between local file and remote API
type RemoteIdentity struct {
	ID           string `yaml:"id"`           // The Hashnode Post ID
	Slug         string `yaml:"slug"`         // The Slug
	LastChecksum string `yaml:"lastChecksum"` // Hash when we last synced successfully
}

// LoadIdentities reads all .yml files in .hnsync/
func LoadIdentities() (map[string]RemoteIdentity, error) {
	identities := make(map[string]RemoteIdentity)

	if err := EnsureStateDir(); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(StateDir)
	if err != nil {
		return nil, err
	}

	for _, e := range entries {
		if filepath.Ext(e.Name()) != ".yml" {
			continue
		}

		path := filepath.Join(StateDir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read state %s: %w", e.Name(), err)
		}

		var id RemoteIdentity
		if err := yaml.Unmarshal(data, &id); err != nil {
			fmt.Printf("⚠️  Corrupt state file ignored: %s\n", e.Name())
			continue
		}

		identities[id.Slug] = id
	}

	return identities, nil
}

// SaveIdentity writes a single identity file (e.g., .hnsync/my-post.yml)
func SaveIdentity(id RemoteIdentity) error {
	if err := EnsureStateDir(); err != nil {
		return err
	}

	data, err := yaml.Marshal(id)
	if err != nil {
		return err
	}

	filename := fmt.Sprintf("%s.yml", id.Slug)
	return os.WriteFile(filepath.Join(StateDir, filename), data, 0644)
}
