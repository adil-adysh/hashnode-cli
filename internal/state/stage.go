package state

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ItemType allows us to distinguish between content and containers
type ItemType string

const (
	TypeArticle ItemType = "ARTICLE"
	TypeSeries  ItemType = "SERIES"
)

// Operation explicitly tracks intent
type Operation string

const (
	OpModify Operation = "MODIFY" // Add or Update (Content exists)
	OpDelete Operation = "DELETE" // Intent to remove
)

// StagedItem represents a unit of work waiting to be planned
type StagedItem struct {
	Type      ItemType  `yaml:"type"`
	Key       string    `yaml:"key"` // Path (Article) or Slug (Series)
	Checksum  string    `yaml:"checksum,omitempty"`
	Snapshot  string    `yaml:"snapshot,omitempty"` // Filename in .hashnode/snapshots/
	Operation Operation `yaml:"operation"`
	StagedAt  time.Time `yaml:"staged_at"`
	// Metadata previously stored in article registry. Keeping here
	// allows the stage to be the single source of truth for both
	// intent and persisted metadata.
	LocalID      string `yaml:"local_id,omitempty"`
	Title        string `yaml:"title,omitempty"`
	SeriesID     string `yaml:"series_id,omitempty"`
	RemotePostID string `yaml:"remote_post_id,omitempty"`
	LastSyncedAt string `yaml:"last_synced_at,omitempty"`
}

// Stage represents the declarative staging area
type Stage struct {
	Version int                   `yaml:"version"`
	Items   map[string]StagedItem `yaml:"items"`
	// New schema: Items holds all staged entries keyed by normalized path
}

// LoadStage reads the stage file or returns a fresh empty stage
func LoadStage() (*Stage, error) {
	path := StatePath(StageFilename)
	var s Stage

	// If stage file doesn't exist, return a fresh empty stage.
	if _, err := os.Stat(path); os.IsNotExist(err) {
		s = Stage{
			Version: 2,
			Items:   make(map[string]StagedItem),
		}
		return &s, nil
	}

	if err := ReadYAML(path, &s); err != nil {
		return nil, fmt.Errorf("failed to read stage: %w", err)
	}

	if s.Items == nil {
		s.Items = make(map[string]StagedItem)
	}
	return &s, nil
}

// SaveStage persists the stage to disk atomically
func SaveStage(s *Stage) error {
	if err := EnsureStateDir(); err != nil {
		return err
	}
	if s.Items == nil {
		s.Items = make(map[string]StagedItem)
	}
	return WriteYAML(StatePath(StageFilename), s)
}

// Clear resets the in-memory stage to empty state.
func (s *Stage) Clear() {
	s.Items = make(map[string]StagedItem)
}

// SetStagedEntry is a compatibility helper to persist an old-style staged entry.
// MigrateStagedPathsByRemote is removed; use external migration tooling if needed.

// (Legacy helpers removed) Use `Stage.Items` and `StageAdd`/`StageDir` directly.

// StageDir walks `dir` and stages tracked markdown files. Returns lists of staged and skipped paths.
func StageDir(dir string) ([]string, []string, error) {
	var staged []string
	var skipped []string
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(p))
		if ext != ".md" && ext != ".markdown" {
			skipped = append(skipped, p)
			return nil
		}
		if err := StageAdd(p); err != nil {
			skipped = append(skipped, p)
			return nil
		}
		staged = append(staged, NormalizePath(p))
		return nil
	})
	return staged, skipped, err
}

// StageAdd adds or updates a file in the stage.
// It captures a snapshot of the content to ensure consistency.
func StageAdd(path string) error {
	// 1. Validation
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}
	if !inRepo(absPath) {
		return fmt.Errorf("path is outside repository")
	}

	// 2. Read Content & Compute Hash
	content, err := os.ReadFile(absPath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}
	checksum := ChecksumFromContent(content)

	// 3. Create Snapshot
	// We save the content to .hashnode/snapshots/<hash>.md
	// This ensures that even if the user edits the file later, the 'Plan' uses this version.
	snapshotName := fmt.Sprintf("%s.md", checksum)
	if err := saveSnapshot(snapshotName, content); err != nil {
		return err
	}

	// 4. Update Stage Data
	st, err := LoadStage()
	if err != nil {
		return err
	}

	// Normalized Path is the Key
	key := NormalizePath(path)

	st.Items[key] = StagedItem{
		Type:      TypeArticle,
		Key:       key,
		Operation: OpModify,
		Checksum:  checksum,
		Snapshot:  snapshotName,
		StagedAt:  time.Now(),
	}

	return SaveStage(st)
}

// StageRemove marks a path for deletion in the stage.
// It does NOT need the file to exist on disk.
func StageRemove(path string) error {
	st, err := LoadStage()
	if err != nil {
		return err
	}

	key := NormalizePath(path)

	// We record the deletion intent even if the file is already gone from disk.
	st.Items[key] = StagedItem{
		Type:      TypeArticle,
		Key:       key,
		Operation: OpDelete,
		StagedAt:  time.Now(),
		// No checksum or snapshot needed for a deletion
	}

	return SaveStage(st)
}

// Unstage removes an item from the stage completely (reverting to unstaged state).
// This is different from StageRemove (which intends to delete remotely).
func Unstage(path string) error {
	st, err := LoadStage()
	if err != nil {
		return err
	}
	key := NormalizePath(path)

	if _, ok := st.Items[key]; ok {
		delete(st.Items, key)
		return SaveStage(st)
	}
	return nil
}

// Helper: saveSnapshot writes content to .hashnode/snapshots/
func saveSnapshot(filename string, content []byte) error {
	snapDir := StatePath("snapshots")
	if err := os.MkdirAll(snapDir, 0755); err != nil {
		return fmt.Errorf("failed to create snapshot dir: %w", err)
	}

	path := filepath.Join(snapDir, filename)

	// Optimization: If snapshot exists, don't re-write (Content Addressable)
	if _, err := os.Stat(path); err == nil {
		return nil
	}

	return AtomicWriteFile(path, content, 0644)
}

// IsStagingItemStale returns true if the current local checksum differs from the staged item's checksum
func IsStagingItemStale(item StagedItem, path string) bool {
	fsPath := filepath.FromSlash(path)
	if !filepath.IsAbs(fsPath) {
		fsPath = filepath.Join(ProjectRootOrCwd(), fsPath)
	}
	info, err := os.Stat(fsPath)
	if err != nil || info.IsDir() {
		return item.Checksum != ""
	}
	data, err := os.ReadFile(fsPath)
	if err != nil {
		return true
	}
	cur := ChecksumFromContent(data)
	return cur != item.Checksum
}

// Helper: GetSnapshotContent retrieves the frozen content for planning/pushing
func GetSnapshotContent(filename string) ([]byte, error) {
	path := StatePath("snapshots", filename)
	return os.ReadFile(path)
}

// AtomicWriteFile writes data to a temp file and renames it to ensure atomicity
// NOTE: You likely have this in your utils, but included here for completeness

// inRepo checks if path is inside project root
func inRepo(absPath string) bool {
	root := ProjectRootOrCwd()
	rel, err := filepath.Rel(root, absPath)
	return err == nil && !strings.HasPrefix(rel, "..")
}
