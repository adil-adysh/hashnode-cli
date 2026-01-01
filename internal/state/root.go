package state

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

var (
	cachedRoot string
	cachedErr  error
	rootOnce   sync.Once
)

// FindProjectRoot searches upward from start (or CWD if empty) for a directory
// that indicates a project root. A directory is considered a project root if
// either a `hashnode.sum` file exists there or a `.hashnode` directory exists.
// Returns os.ErrNotExist if no project root is found.
func FindProjectRoot(start string) (string, error) {
	var err error
	if start == "" {
		start, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to get working dir: %w", err)
		}
	}
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path %s: %w", start, err)
	}

	for {
		// Accept either a hashnode.sum file or a .hashnode directory as project root
		sumPath := filepath.Join(dir, SumFile)
		if _, err := os.Stat(sumPath); err == nil {
			return dir, nil
		}
		statePath := filepath.Join(dir, StateDir)
		if fi, err := os.Stat(statePath); err == nil && fi.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", os.ErrNotExist
}

// ProjectRoot returns the project root by searching from the current working
// directory. Convenience wrapper around FindProjectRoot.
func ProjectRoot() (string, error) {
	// memoize filesystem walk
	var root string
	var err error
	rootOnce.Do(func() {
		root, err = FindProjectRoot("")
		if err == nil {
			cachedRoot = root
		}
		cachedErr = err
	})
	if cachedRoot != "" {
		return cachedRoot, nil
	}
	return "", cachedErr
}

// ProjectRootOrCwd returns the absolute path to the project root if found,
// otherwise returns the current working directory as a fallback. This mirrors
// prior callers that expected a string-returning `repoRoot()` helper.
func ProjectRootOrCwd() string {
	if cachedRoot != "" {
		return cachedRoot
	}
	if root, err := ProjectRoot(); err == nil {
		return root
	}
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	return "."
}

// ResetProjectRootCache clears the cached project root.
// This is primarily for testing purposes.
func ResetProjectRootCache() {
	cachedRoot = ""
	cachedErr = nil
	rootOnce = sync.Once{}
}
