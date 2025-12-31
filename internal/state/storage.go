package state

import (
	"fmt"
	"os"
	"path/filepath"

	"adil-adysh/hashnode-cli/internal/log"

	"gopkg.in/yaml.v3"
)

// RemoteIdentity tracks the connection between local file and remote API
// RemoteIdentity represents the minimal persisted identity linking a local
// article to a remote Hashnode post. Kept minimal so the registry is
// easy to audit by hand.
type RemoteIdentity struct {
	ID           string `yaml:"id"`           // The Hashnode Post ID
	Slug         string `yaml:"slug"`         // The slug used in filenames
	LastChecksum string `yaml:"lastChecksum"` // Hash when last synced successfully
}

// LoadIdentities reads all .yml files in .hnsync/
func LoadIdentities() (map[string]RemoteIdentity, error) {
	// `idents` clarifies this is an in-memory lookup of identities keyed by slug.
	idents := make(map[string]RemoteIdentity)

	if err := EnsureStateDir(); err != nil {
		return nil, err
	}
	dir := StatePath()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != StateFileExt {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			// Read failure for a single file should not abort the entire load.
			log.Warnf("failed to read identity file %s: %v", entry.Name(), err)
			continue
		}

		var rid RemoteIdentity
		if err := yaml.Unmarshal(data, &rid); err != nil {
			log.Warnf("corrupt identity file %s: %v", entry.Name(), err)
			continue
		}

		idents[rid.Slug] = rid
	}

	return idents, nil
}

// SaveIdentity writes a single identity file (e.g., .hnsync/my-post.yml)
// SaveIdentity writes a single identity file into the state directory.
func SaveIdentity(id RemoteIdentity) error {
	if err := EnsureStateDir(); err != nil {
		return err
	}

	data, err := yaml.Marshal(id)
	if err != nil {
		return err
	}

	filename := fmt.Sprintf("%s%s", id.Slug, StateFileExt)
	return AtomicWriteFile(StatePath(filename), data, FilePerm)
}
