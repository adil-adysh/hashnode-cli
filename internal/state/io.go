package state

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// StatePath returns a path under the repository state directory (.hashnode)
func StatePath(parts ...string) string {
	// Prefer an absolute path anchored at the repository root, if available.
	if root, err := ProjectRoot(); err == nil {
		elems := append([]string{root, StateDir}, parts...)
		return filepath.Join(elems...)
	}
	// Fallback: return a relative path under .hashnode (preserves prior behavior)
	elems := append([]string{StateDir}, parts...)
	return filepath.Join(elems...)
}

// ReadYAML reads a YAML file at path and unmarshals into out.
// Caller may check os.IsNotExist on the returned error.
func ReadYAML(path string, out interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := yaml.Unmarshal(data, out); err != nil {
		return fmt.Errorf("invalid yaml %s: %w", path, err)
	}
	return nil
}

// WriteYAML marshals v and writes it to path, ensuring the state dir exists.
func WriteYAML(path string, v interface{}) error {
	if err := EnsureStateDir(); err != nil {
		return err
	}
	data, err := yaml.Marshal(v)
	if err != nil {
		return fmt.Errorf("failed to marshal %s: %w", path, err)
	}
	return os.WriteFile(path, data, 0644)
}
