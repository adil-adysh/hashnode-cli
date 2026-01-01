package state

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"adil-adysh/hashnode-cli/internal/log"
)

// Snapshot represents a point-in-time capture of file content.
// Snapshots are content-addressable: filename = SHA256(content).md
type Snapshot struct {
	Checksum  string    // SHA256 hash of content
	Filename  string    // {checksum}.md
	CreatedAt time.Time // When snapshot was created
	Size      int64     // Content size in bytes
}

// SnapshotStore manages content-addressable snapshots in .hashnode/snapshots/
type SnapshotStore struct {
	dir string // Absolute path to snapshots directory
}

// NewSnapshotStore creates a snapshot store instance.
// Call EnsureDir() before first use to create the directory.
func NewSnapshotStore() *SnapshotStore {
	return &SnapshotStore{
		dir: StatePath("snapshots"),
	}
}

// EnsureDir creates the snapshots directory if it doesn't exist.
func (s *SnapshotStore) EnsureDir() error {
	if err := os.MkdirAll(s.dir, DirPerm); err != nil {
		return fmt.Errorf("failed to create snapshots dir: %w", err)
	}
	return nil
}

// Create saves content as a snapshot and returns its metadata.
// If a snapshot with the same checksum exists, it's reused (idempotent).
func (s *SnapshotStore) Create(content []byte) (*Snapshot, error) {
	// Compute checksum
	hash := sha256.Sum256(content)
	checksum := hex.EncodeToString(hash[:])
	filename := fmt.Sprintf("%s.md", checksum)

	snap := &Snapshot{
		Checksum:  checksum,
		Filename:  filename,
		CreatedAt: time.Now(),
		Size:      int64(len(content)),
	}

	// Ensure directory exists
	if err := s.EnsureDir(); err != nil {
		return nil, err
	}

	path := filepath.Join(s.dir, filename)

	// Check if snapshot already exists (content-addressable = deduplication)
	if info, err := os.Stat(path); err == nil {
		snap.CreatedAt = info.ModTime()
		snap.Size = info.Size()
		return snap, nil
	}

	// Write snapshot atomically
	if err := AtomicWriteFile(path, content, FilePerm); err != nil {
		return nil, fmt.Errorf("failed to write snapshot %s: %w", filename, err)
	}

	return snap, nil
}

// Get retrieves snapshot content by filename.
func (s *SnapshotStore) Get(filename string) ([]byte, error) {
	if filename == "" {
		return nil, fmt.Errorf("snapshot filename is empty")
	}

	path := filepath.Join(s.dir, filename)
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read snapshot %s: %w", filename, err)
	}

	return content, nil
}

// Validate checks if snapshot content matches its checksum-based filename.
func (s *SnapshotStore) Validate(filename string) error {
	content, err := s.Get(filename)
	if err != nil {
		return err
	}

	// Extract checksum from filename (remove .md extension)
	expectedChecksum := strings.TrimSuffix(filename, ".md")

	// Compute actual checksum
	hash := sha256.Sum256(content)
	actualChecksum := hex.EncodeToString(hash[:])

	if actualChecksum != expectedChecksum {
		return fmt.Errorf("snapshot integrity check failed: expected %s, got %s", expectedChecksum, actualChecksum)
	}

	return nil
}

// Delete removes a snapshot file.
func (s *SnapshotStore) Delete(filename string) error {
	if filename == "" {
		return nil
	}

	path := filepath.Join(s.dir, filename)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete snapshot %s: %w", filename, err)
	}

	return nil
}

// List returns all snapshot filenames in the store.
func (s *SnapshotStore) List() ([]string, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to read snapshots dir: %w", err)
	}

	re := regexp.MustCompile(`(?i)^[a-f0-9]{64}\.md$`)
	var snapshots []string

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if re.MatchString(e.Name()) {
			snapshots = append(snapshots, e.Name())
		}
	}

	return snapshots, nil
}

// GCStats contains statistics from garbage collection.
type GCStats struct {
	TotalSnapshots   int      // Total snapshot files found
	ReferencedCount  int      // Number of snapshots referenced by stage/lock
	RemovedCount     int      // Number of snapshots removed (or would be in dry-run)
	RemovedSnapshots []string // List of removed snapshot filenames
	Errors           []error  // Errors encountered during removal
	SkippedCount     int      // Snapshots that couldn't be verified or removed
}

// GC removes unreferenced snapshots with optional integrity verification.
// A snapshot is considered referenced if it appears in stage or lock.
// In dry-run mode, no files are deleted but stats show what would be removed.
func (s *SnapshotStore) GC(dryRun bool) (*GCStats, error) {
	stats := &GCStats{
		RemovedSnapshots: make([]string, 0),
		Errors:           make([]error, 0),
	}

	// Get all snapshots
	allSnapshots, err := s.List()
	if err != nil {
		return stats, fmt.Errorf("failed to list snapshots: %w", err)
	}
	stats.TotalSnapshots = len(allSnapshots)

	// Early return if no snapshots
	if len(allSnapshots) == 0 {
		return stats, nil
	}

	// Build reference set from stage and lock
	referenced := s.buildReferenceSet()
	stats.ReferencedCount = len(referenced)

	// Early return if all snapshots are referenced
	if stats.ReferencedCount >= stats.TotalSnapshots {
		return stats, nil
	}

	// Remove unreferenced snapshots
	for _, filename := range allSnapshots {
		lowerName := strings.ToLower(filename)
		if referenced[lowerName] {
			continue // Keep referenced snapshots
		}

		if dryRun {
			stats.RemovedSnapshots = append(stats.RemovedSnapshots, filename)
			stats.RemovedCount++
		} else {
			if err := s.Delete(filename); err != nil {
				log.Warnf("failed to remove snapshot %s: %v", filename, err)
				stats.Errors = append(stats.Errors, fmt.Errorf("delete %s: %w", filename, err))
				stats.SkippedCount++
			} else {
				stats.RemovedSnapshots = append(stats.RemovedSnapshots, filename)
				stats.RemovedCount++
			}
		}
	}

	return stats, nil
}

// buildReferenceSet collects all snapshot references from stage and lock.
func (s *SnapshotStore) buildReferenceSet() map[string]bool {
	referenced := make(map[string]bool)

	// Collect from stage
	if st, err := LoadStage(); err == nil {
		for _, item := range st.Items {
			if item.Snapshot != "" {
				// Normalize to lowercase for case-insensitive comparison
				referenced[strings.ToLower(item.Snapshot)] = true
			}
		}
	}

	// Collect from lock (if exists)
	if lock, err := LoadLock(); err == nil {
		for _, article := range lock.Staged.Articles {
			if article.Snapshot != "" {
				referenced[strings.ToLower(article.Snapshot)] = true
			}
		}
	}

	return referenced
}

// GCWithVerification removes unreferenced snapshots and optionally verifies integrity.
func (s *SnapshotStore) GCWithVerification(dryRun, verify bool) (*GCStats, error) {
	stats := &GCStats{
		RemovedSnapshots: make([]string, 0),
		Errors:           make([]error, 0),
	}

	// Get all snapshots
	allSnapshots, err := s.List()
	if err != nil {
		return stats, fmt.Errorf("failed to list snapshots: %w", err)
	}
	stats.TotalSnapshots = len(allSnapshots)

	if len(allSnapshots) == 0 {
		return stats, nil
	}

	// Build reference set
	referenced := s.buildReferenceSet()
	stats.ReferencedCount = len(referenced)

	// Process snapshots
	for _, filename := range allSnapshots {
		lowerName := strings.ToLower(filename)
		isReferenced := referenced[lowerName]

		// Verify integrity if requested and referenced
		if verify && isReferenced {
			if err := s.Validate(filename); err != nil {
				log.Warnf("snapshot %s failed integrity check: %v", filename, err)
				stats.Errors = append(stats.Errors, fmt.Errorf("integrity %s: %w", filename, err))
				// Don't remove corrupted referenced snapshots automatically
				stats.SkippedCount++
				continue
			}
		}

		if isReferenced {
			continue // Keep referenced snapshots
		}

		// Remove unreferenced
		if dryRun {
			stats.RemovedSnapshots = append(stats.RemovedSnapshots, filename)
			stats.RemovedCount++
		} else {
			if err := s.Delete(filename); err != nil {
				log.Warnf("failed to remove snapshot %s: %v", filename, err)
				stats.Errors = append(stats.Errors, fmt.Errorf("delete %s: %w", filename, err))
				stats.SkippedCount++
			} else {
				stats.RemovedSnapshots = append(stats.RemovedSnapshots, filename)
				stats.RemovedCount++
			}
		}
	}

	return stats, nil
}

// GetContentByChecksum retrieves content by checksum (without .md extension).
func (s *SnapshotStore) GetContentByChecksum(checksum string) ([]byte, error) {
	filename := fmt.Sprintf("%s.md", checksum)
	return s.Get(filename)
}

// Exists checks if a snapshot file exists.
func (s *SnapshotStore) Exists(filename string) bool {
	path := filepath.Join(s.dir, filename)
	_, err := os.Stat(path)
	return err == nil
}
