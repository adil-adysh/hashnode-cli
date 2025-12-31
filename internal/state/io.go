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

// NormalizePath returns a repository-relative, slash-separated path for
// consistent keys. If the path cannot be made relative to the project root,
// it returns an absolute path with forward slashes.
func NormalizePath(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return filepath.ToSlash(p)
	}
	root := ProjectRootOrCwd()
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return filepath.ToSlash(abs)
	}
	return filepath.ToSlash(rel)
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

// LoadYAMLOrEmpty behaves like ReadYAML but treats missing files as an
// empty value for `out`. This simplifies callers that want to treat absent
// registry files as empty lists/maps instead of errors.
func LoadYAMLOrEmpty(path string, out interface{}) error {
	if err := ReadYAML(path, out); err != nil {
		if os.IsNotExist(err) {
			// Intentionally return nil so callers get the zero-value in `out`.
			return nil
		}
		return err
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
	return AtomicWriteFile(path, data, FilePerm)
}

// AtomicWriteFile writes the provided data to a temp file in the same
// directory and renames it into place. This ensures atomic replacement.
func AtomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, DirPerm); err != nil {
		return err
	}
	// Use a deterministic temp filename in the same directory to ensure the
	// rename is atomic on the same filesystem. Keep the name short so it's
	// easy to spot during debugging.
	tmpPath := filepath.Join(dir, ".tmp-"+filepath.Base(path))
	if err := os.WriteFile(tmpPath, data, perm); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
