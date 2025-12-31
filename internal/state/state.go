package state

import (
	"fmt"
	"os"
	"path/filepath"

	"adil-adysh/hashnode-cli/internal/log"
	"gopkg.in/yaml.v3"
)

// PostState tracks the link between a local file and a remote Hashnode Post
type PostState struct {
	ID           string `yaml:"id"`           // Hashnode ID (The Source of Truth)
	Slug         string `yaml:"slug"`         // The slug we expect
	LastChecksum string `yaml:"lastChecksum"` // Hash of content when we last synced
	RemoteURL    string `yaml:"remoteUrl"`    // For user convenience
}

// StateDir is defined in consts.go
// EnsureStateDir creates the hidden folder if it doesn't exist
func EnsureStateDir() error {
	dir := StatePath()
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return os.MkdirAll(dir, DirPerm)
	}
	return nil
}

// LoadState reads all .yml files in .hnsync/ and returns a map[Slug]PostState
func LoadState() (map[string]PostState, error) {
	state := make(map[string]PostState)

	// Ensure dir exists first
	if err := EnsureStateDir(); err != nil {
		return nil, err
	}

	dir := StatePath()
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, f := range files {
		if filepath.Ext(f.Name()) != ".yml" {
			continue
		}

		path := filepath.Join(dir, f.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read state %s: %w", f.Name(), err)
		}

		var s PostState
		if err := yaml.Unmarshal(data, &s); err != nil {
			// Don't crash, just warn (in a real app)
			log.Warnf("⚠️ Warning: Corrupt state file %s\n", f.Name())
			continue
		}

		// Map key is the slug from the state file
		state[s.Slug] = s
	}

	return state, nil
}

// SaveState writes a single post's state to disk
func SaveState(s PostState) error {
	if err := EnsureStateDir(); err != nil {
		return err
	}

	data, err := yaml.Marshal(s)
	if err != nil {
		return err
	}

	// Filename under state dir
	filename := fmt.Sprintf("%s.yml", s.Slug)
	path := StatePath(filename)

	return AtomicWriteFile(path, data, FilePerm)
}
