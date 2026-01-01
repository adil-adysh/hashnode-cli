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

	"github.com/google/uuid"

	"adil-adysh/hashnode-cli/internal/log"
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
}

// Stage represents the declarative staging area
type Stage struct {
	Version int                   `yaml:"version"`
	Items   map[string]StagedItem `yaml:"items"`
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

// StageDir walks `dir` and stages tracked markdown files using BULK IO.
// This is O(1) IO operation on the stage file, regardless of file count.
func StageDir(dir string) ([]string, []string, error) {
	// 1. Load Stage ONCE
	st, err := LoadStage()
	if err != nil {
		return nil, nil, err
	}

	var staged []string
	var skipped []string

	// 2. Process Files
	err = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
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

		absPath, err := filepath.Abs(p)
		if err != nil || !inRepo(absPath) {
			skipped = append(skipped, p)
			return nil
		}

		// Read content to snapshot
		content, err := os.ReadFile(absPath)
		if err != nil {
			skipped = append(skipped, p)
			return nil
		}
		checksum := ChecksumFromContent(content)
		snapshotName := fmt.Sprintf("%s.md", checksum)

		if err := saveSnapshot(snapshotName, content); err != nil {
			return err
		}

		// Update Memory
		key := NormalizePath(p)
		st.Items[key] = StagedItem{
			Type:      TypeArticle,
			Key:       key,
			Operation: OpModify,
			Checksum:  checksum,
			Snapshot:  snapshotName,
			StagedAt:  time.Now(),
		}
		staged = append(staged, key)
		return nil
	})

	if err != nil {
		return staged, skipped, err
	}

	// 3. Save Stage ONCE
	if err := SaveStage(st); err != nil {
		return staged, skipped, err
	}

	return staged, skipped, nil
}

// StageAdd adds or updates a file in the stage (Single file).
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
	snapshotName := fmt.Sprintf("%s.md", checksum)
	if err := saveSnapshot(snapshotName, content); err != nil {
		return err
	}

	// 4. Update Stage Data
	st, err := LoadStage()
	if err != nil {
		return err
	}

	key := NormalizePath(path)
	st.Items[key] = StagedItem{
		Type:      TypeArticle,
		Key:       key,
		Operation: OpModify,
		Checksum:  checksum,
		Snapshot:  snapshotName,
		StagedAt:  time.Now(),
	}

	if err := SaveStage(st); err != nil {
		return err
	}
	return nil
}

// StageRemove marks a path for deletion in the stage.
func StageRemove(path string) error {
	st, err := LoadStage()
	if err != nil {
		return err
	}

	key := NormalizePath(path)
	st.Items[key] = StagedItem{
		Type:      TypeArticle,
		Key:       key,
		Operation: OpDelete,
		StagedAt:  time.Now(),
	}

	return SaveStage(st)
}

// Unstage removes an item from the stage completely.
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

// IsStagingItemStale returns true if local checksum differs from stage
func IsStagingItemStale(item StagedItem, path string) bool {
	fsPath := filepath.FromSlash(path)
	if !filepath.IsAbs(fsPath) {
		fsPath = filepath.Join(ProjectRootOrCwd(), fsPath)
	}
	info, err := os.Stat(fsPath)
	if err != nil || info.IsDir() {
		return item.Checksum != "" // Considered stale if missing/dir
	}
	data, err := os.ReadFile(fsPath)
	if err != nil {
		return true
	}
	return ChecksumFromContent(data) != item.Checksum
}

// Helper: GetSnapshotContent retrieves the frozen content
func GetSnapshotContent(filename string) ([]byte, error) {
	path := StatePath("snapshots", filename)
	return os.ReadFile(path)
}

// GCStaleSnapshots removes unreferenced snapshot files.
// Note: Requires LoadLock implementation (not provided in this file, assumed exists in package)
func GCStaleSnapshots() (int, error) {
	snapDir := StatePath("snapshots")
	entries, err := os.ReadDir(snapDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to read snapshots dir: %w", err)
	}

	keep := make(map[string]struct{})

	// 1. Keep items from Stage
	st, err := LoadStage()
	if err == nil {
		for _, it := range st.Items {
			if it.Snapshot != "" {
				keep[it.Snapshot] = struct{}{}
			}
		}
	}

	// 2. Keep items from Lock (assumes LoadLock exists in package)
	l, err := LoadLock()
	if err == nil {
		for _, a := range l.Staged.Articles {
			if a.Snapshot != "" {
				keep[strings.ToLower(a.Snapshot)] = struct{}{}
			}
		}
	}

	re := regexp.MustCompile(`(?i)^[a-f0-9]{64}\.md$`)
	removed := 0

	for _, e := range entries {
		name := e.Name()
		if !re.MatchString(name) {
			continue
		}
		lname := strings.ToLower(name)
		if _, ok := keep[lname]; ok {
			continue
		}

		p := filepath.Join(snapDir, name)
		if err := os.Remove(p); err != nil {
			// Log but don't fail hard
			log.Warnf("failed to remove stale snapshot %s: %v", name, err)
			continue
		}
		removed++
	}
	return removed, nil
}

// --- Utils (Ensure these match your utils.go or are kept here) ---

func inRepo(absPath string) bool {
	root := ProjectRootOrCwd()
	rel, err := filepath.Rel(root, absPath)
	return err == nil && !strings.HasPrefix(rel, "..")
}

func Slugify(name string) string {
	s := strings.ToLower(name)
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, " ", "-")
	re := regexp.MustCompile(`[^a-z0-9-]`)
	s = re.ReplaceAllString(s, "")
	return s
}

func ChecksumFromContent(content []byte) string {
	h := sha256.Sum256(content)
	return hex.EncodeToString(h[:])
}

// GenerateFilename ensures a deterministic filename
func GenerateFilename(title string, dir string) (string, error) {
	base := Slugify(title)
	if base == "" {
		base = uuid.NewString()[0:8]
	}
	base = strings.Trim(base, " .")

	// Double check slugify didn't leave empty after regex
	if base == "" {
		base = uuid.NewString()[0:8]
	}

	name := base + ".md"
	full := filepath.Join(dir, name)
	i := 1
	for {
		if _, err := os.Stat(full); os.IsNotExist(err) {
			return filepath.ToSlash(full), nil
		}
		name = fmt.Sprintf("%s-%d.md", base, i)
		full = filepath.Join(dir, name)
		i++
		if i > 1000 {
			return "", fmt.Errorf("too many filename collisions for %s", base)
		}
	}
}
