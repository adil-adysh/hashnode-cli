package state

import (
	"fmt"
	"os"
	"path/filepath"

	"adil-adysh/hashnode-cli/internal/log"

	"gopkg.in/yaml.v3"
)

// PostState tracks the link between a local file and a remote Hashnode Post
// PostState tracks the relationship between a local markdown file and
// the corresponding remote Hashnode post. It is persisted as YAML under
// the repository state directory (see `StateDir`).
type PostState struct {
	ID           string `yaml:"id"`           // Hashnode ID (source of truth)
	Slug         string `yaml:"slug"`         // The slug for this post
	LastChecksum string `yaml:"lastChecksum"` // Hash of content when last synced
	RemoteURL    string `yaml:"remoteUrl"`    // Optional remote URL for convenience
}

// StateDir is defined in consts.go
// EnsureStateDir ensures the repository state directory exists and is
// writable. It is safe to call repeatedly.
func EnsureStateDir() error {
	dir := StatePath()
	if err := os.MkdirAll(dir, DirPerm); err != nil {
		return fmt.Errorf("creating state dir %s: %w", dir, err)
	}
	return nil
}

// LoadState reads all YAML files under the repository state directory and
// returns a mapping keyed by post slug. Corrupt files are skipped with a
// warning so a single bad file doesn't prevent loading the rest of the
// registry.
func LoadState() (map[string]PostState, error) {
	// `registry` makes it immediately clear this holds the on-disk registry
	// of post states keyed by slug.
	registry := make(map[string]PostState)

	if err := EnsureStateDir(); err != nil {
		return nil, err
	}

	dir := StatePath()
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("listing state dir %s: %w", dir, err)
	}

	for _, fi := range files {
		if filepath.Ext(fi.Name()) != StateFileExt {
			continue
		}

		path := filepath.Join(dir, fi.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			// Prefer a short warning so a single bad file doesn't block load.
			log.Warnf("failed to read state file %s: %v", fi.Name(), err)
			continue
		}

		var pst PostState
		if err := yaml.Unmarshal(data, &pst); err != nil {
			log.Warnf("corrupt state file %s: %v", fi.Name(), err)
			continue
		}

		registry[pst.Slug] = pst
	}

	return registry, nil
}

// SaveState writes a single post's state to disk
// SaveState persists a single post state object to a file named
// `<slug>.yml` under the repository state directory. The write is
// performed atomically.
// SaveState writes `post` into its on-disk representation (<slug>.yml).
// Using a descriptive parameter name makes call-sites clearer and
// reduces mental overhead when scanning the implementation.
func SaveState(post PostState) error {
	if err := EnsureStateDir(); err != nil {
		return err
	}

	data, err := yaml.Marshal(post)
	if err != nil {
		return fmt.Errorf("marshal state for %s: %w", post.Slug, err)
	}

	filename := fmt.Sprintf("%s%s", post.Slug, StateFileExt)
	path := StatePath(filename)

	return AtomicWriteFile(path, data, FilePerm)
}
