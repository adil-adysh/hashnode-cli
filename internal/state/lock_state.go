package state

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// ArticleState represents the staged intent for an article
type ArticleState string

const (
	ArticleStateNew    ArticleState = "NEW"
	ArticleStateUpdate ArticleState = "UPDATE"
	ArticleStateDelete ArticleState = "DELETE"
	ArticleStateNoop   ArticleState = "NOOP"
)

type checksumPair struct {
	Local  string `yaml:"local,omitempty"`
	Remote string `yaml:"remote,omitempty"`
}

type StagedArticle struct {
	ID       string       `yaml:"id,omitempty"`
	State    ArticleState `yaml:"state"`
	Checksum checksumPair `yaml:"checksum,omitempty"`
}

type lockStaged struct {
	Articles map[string]StagedArticle `yaml:"articles"`
}

// LockData is the YAML structure persisted in .hashnode/hashnode.lock
type LockData struct {
	PID     int        `yaml:"pid,omitempty"`
	Created string     `yaml:"created,omitempty"`
	Staged  lockStaged `yaml:"staged,omitempty"`
}

func lockPath() string {
	return StatePath(LockFile)
}

// LoadLock reads the lock file if present or returns an empty LockData
func LoadLock() (*LockData, error) {
	path := lockPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &LockData{Staged: lockStaged{Articles: make(map[string]StagedArticle)}}, nil
		}
		return nil, fmt.Errorf("failed to read lock %s: %w", path, err)
	}
	var l LockData
	if err := yaml.Unmarshal(data, &l); err != nil {
		return nil, fmt.Errorf("invalid yaml %s: %w", path, err)
	}
	if l.Staged.Articles == nil {
		l.Staged.Articles = make(map[string]StagedArticle)
	}
	return &l, nil
}

// SaveLock writes the lock data deterministically (overwrites existing lock)
func SaveLock(l *LockData) error {
	// ensure state dir exists
	if err := EnsureStateDir(); err != nil {
		return fmt.Errorf("failed to ensure state dir: %w", err)
	}
	// ensure nested map
	if l.Staged.Articles == nil {
		l.Staged.Articles = make(map[string]StagedArticle)
	}
	// set created time if missing
	if l.Created == "" {
		l.Created = time.Now().UTC().Format(time.RFC3339)
	}
	data, err := yaml.Marshal(l)
	if err != nil {
		return fmt.Errorf("failed to marshal lock: %w", err)
	}
	return AtomicWriteFile(lockPath(), data, FilePerm)
}

// ComputeArticleState computes the semantic state for an article given known metadata.
// It returns the state, local checksum, remote checksum (may be empty), and error.
func ComputeArticleState(a ArticleEntry) (ArticleState, string, string, error) {
	// Determine local checksum
	info, err := os.Stat(a.MarkdownPath)
	if err != nil || info.IsDir() {
		// missing local file
		if a.RemotePostID != "" {
			return ArticleStateDelete, "", a.Checksum, nil
		}
		return ArticleStateNoop, "", a.Checksum, nil
	}
	data, err := os.ReadFile(a.MarkdownPath)
	if err != nil {
		return ArticleStateNoop, "", a.Checksum, fmt.Errorf("failed reading local file: %w", err)
	}
	local := ChecksumFromContent(data)
	remote := a.Checksum
	if a.RemotePostID == "" {
		return ArticleStateNew, local, remote, nil
	}
	if local != remote {
		return ArticleStateUpdate, local, remote, nil
	}
	return ArticleStateNoop, local, remote, nil
}

// IsStagingStale returns true if the current local checksum differs from the saved staged checksum
func IsStagingStale(s StagedArticle, path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		// if local missing but staged expected local, consider stale
		return s.Checksum.Local != ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return true
	}
	cur := ChecksumFromContent(data)
	return cur != s.Checksum.Local
}
