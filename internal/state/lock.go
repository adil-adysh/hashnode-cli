package state

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const LockFile = "hashnode.lock"

// AcquireRepoLock creates a lock file at the project root. It returns a
// release function which should be deferred by the caller to remove the lock.
// If the lock file already exists, an error is returned.

// AcquireRepoLock creates a lock file at the repository root. It returns a
// release function which should be deferred by the caller to remove the lock.
// If the lock file already exists, an error is returned.
func AcquireRepoLock() (func() error, error) {
	// Ensure state dir exists at project root and place lock inside it for visibility
	root := ProjectRootOrCwd()
	stateDirPath := filepath.Join(root, StateDir)
	if err := os.MkdirAll(stateDirPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to ensure state dir: %w", err)
	}
	lockPath := filepath.Join(stateDirPath, LockFile)

	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		if os.IsExist(err) {
			return nil, fmt.Errorf("lock file %s already exists", lockPath)
		}
		return nil, err
	}

	// Write simple metadata (pid + timestamp)
	meta := fmt.Sprintf("pid=%d\ncreated=%s\n", os.Getpid(), time.Now().UTC().Format(time.RFC3339))
	if _, err := f.WriteString(meta); err != nil {
		f.Close()
		os.Remove(lockPath)
		return nil, err
	}
	f.Close()

	release := func() error {
		err := os.Remove(lockPath)
		if err == nil {
			fmt.Printf("removed lock: %s\n", lockPath)
		}
		return err
	}
	fmt.Printf("acquired lock: %s\n", lockPath)
	return release, nil
}
