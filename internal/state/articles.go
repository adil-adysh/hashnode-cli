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
)

// ArticleEntry represents a registry entry for a local article
type ArticleEntry struct {
	LocalID      string `yaml:"local_id"`
	Title        string `yaml:"title"`
	MarkdownPath string `yaml:"markdown_path"`
	SeriesID     string `yaml:"series_id,omitempty"`
	RemotePostID string `yaml:"remote_post_id,omitempty"`
	Checksum     string `yaml:"checksum"`
	LastSyncedAt string `yaml:"last_synced_at,omitempty"`
}

// ArticlesFile is defined in consts.go

func articlesPath() string {
	return StatePath(ArticlesFile)
}

// LoadArticles reads the article registry. Returns empty slice if file doesn't exist.
func LoadArticles() ([]ArticleEntry, error) {
	path := articlesPath()
	var list []ArticleEntry
	if err := ReadYAML(path, &list); err != nil {
		if os.IsNotExist(err) {
			return []ArticleEntry{}, nil
		}
		return nil, fmt.Errorf("failed to read article registry: %w", err)
	}
	return list, nil
}

// SaveArticles writes the provided list to the article registry file. Use
// `articlesPath()` to compute the file location under the repo state dir.
func SaveArticles(list []ArticleEntry) error {
	path := articlesPath()
	if err := WriteYAML(path, list); err != nil {
		return fmt.Errorf("failed to write article registry: %w", err)
	}
	return nil
}

// GenerateFilename ensures a deterministic filename from title and avoids collisions
func GenerateFilename(title string, dir string) (string, error) {
	base := Slugify(title)
	if base == "" {
		base = uuid.NewString()[0:8]
	}
	// ensure no leading/trailing dots or spaces
	base = strings.Trim(base, " .")
	// final safety: collapse any remaining unsafe chars
	re := regexp.MustCompile(`[^a-z0-9-]`)
	base = re.ReplaceAllString(base, "")
	if base == "" {
		base = uuid.NewString()[0:8]
	}
	name := base + ".md"
	full := filepath.Join(dir, name)
	i := 1
	for {
		if _, err := os.Stat(full); os.IsNotExist(err) {
			// return path using forward slashes for registry portability
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

// ChecksumFromContent returns SHA256 checksum hex of provided content
func ChecksumFromContent(content []byte) string {
	h := sha256.Sum256(content)
	return hex.EncodeToString(h[:])
}

func nowISO() string {
	return time.Now().UTC().Format(time.RFC3339)
}
