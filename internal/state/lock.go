package state

import (
	"fmt"
	"os"
	"time"

	"adil-adysh/hashnode-cli/internal/log"
)

// LockFile is defined in consts.go

// AcquireRepoLock creates a lock file at the project root. It returns a
// release function which should be deferred by the caller to remove the lock.
// If the lock file already exists, an error is returned.

// AcquireRepoLock creates a lock file at the repository root. It returns a
// release function which should be deferred by the caller to remove the lock.
// If the lock file already exists, an error is returned.
func AcquireRepoLock() (func() error, error) {
	// Ensure state dir exists at project root and place lock inside it for visibility
	if err := EnsureStateDir(); err != nil {
		return nil, fmt.Errorf("failed to ensure state dir: %w", err)
	}
	lockPath := StatePath(LockFile)

	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, FilePerm)
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
		// Don't log on every release - let caller decide
		return err
	}
	log.Printf("acquired lock: %s\n", lockPath)
	return release, nil
}
