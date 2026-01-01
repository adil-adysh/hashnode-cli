package state

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Sum represents the deterministic mapping between repo artifacts and remote IDs
type Sum struct {
	Version  int                    `yaml:"version"`
	Blog     BlogEntry              `yaml:"blog"`
	Series   map[string]SeriesEntry `yaml:"series"`
	Articles map[string]ArticleSum  `yaml:"articles"`
}

type BlogEntry struct {
	PublicationID   string `yaml:"publication_id"`
	PublicationSlug string `yaml:"publication_slug"`
}

type ArticleSum struct {
	PostID   string `yaml:"post_id"`
	Checksum string `yaml:"checksum"`
	Slug     string `yaml:"slug,omitempty"`
}

// SeriesEntry is defined in state.go, but we ensure it works here.
// Note: If SeriesEntry isn't defined in state.go, define it here.
// Based on previous files, it seems to be shared.
// If it is missing, uncomment the struct below:
/*
type SeriesEntry struct {
	SeriesID    string `yaml:"series_id"`
	Name        string `yaml:"name"`
	Slug        string `yaml:"slug"`
	Description string `yaml:"description,omitempty"`
}
*/

// LoadSum parses hashnode.sum in repo root. Returns os.ErrNotExist if missing.
func LoadSum() (*Sum, error) {
	// Resolve project root and use the SumFile at the repository root
	root := ProjectRootOrCwd()
	path := filepath.Join(root, SumFile)

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, err
	}

	var sum Sum
	if err := ReadYAML(path, &sum); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", SumFile, err)
	}

	// 4. Initialize Maps (Safety)
	if sum.Series == nil {
		sum.Series = make(map[string]SeriesEntry)
	}
	if sum.Articles == nil {
		sum.Articles = make(map[string]ArticleSum)
	}

	return &sum, nil
}

// SaveSum writes the sum file deterministically.
func SaveSum(s *Sum) error {
	// Ensure Version is set
	if s.Version == 0 {
		s.Version = 1
	}

	// Initialize maps to ensure empty maps serialize as "{}" or omit properly
	// rather than null if that's preferred, though yaml handles nil fine.
	if s.Series == nil {
		s.Series = make(map[string]SeriesEntry)
	}
	if s.Articles == nil {
		s.Articles = make(map[string]ArticleSum)
	}

	root := ProjectRootOrCwd()
	path := filepath.Join(root, SumFile)
	return WriteYAML(path, s)
}

// NewSumFromBlog constructs a Sum with Blog info from .hashnode/blog.yml
func NewSumFromBlog() (*Sum, error) {
	var blog struct {
		PublicationID   string `yaml:"publication_id"`
		PublicationSlug string `yaml:"publication_slug"`
	}

	blogPath := StatePath("blog.yml")
	if err := ReadYAML(blogPath, &blog); err != nil {
		return nil, fmt.Errorf("failed to read blog.yml: %w", err)
	}

	return &Sum{
		Version:  1,
		Blog:     BlogEntry{PublicationID: blog.PublicationID, PublicationSlug: blog.PublicationSlug},
		Series:   make(map[string]SeriesEntry),
		Articles: make(map[string]ArticleSum),
	}, nil
}

// SetArticle sets or updates an article entry in the sum
func (s *Sum) SetArticle(path, postID, checksum, slug string) {
	if s.Articles == nil {
		s.Articles = make(map[string]ArticleSum)
	}
	s.Articles[path] = ArticleSum{PostID: postID, Checksum: checksum, Slug: slug}
}

// RemoveArticle deletes an article entry from the sum
func (s *Sum) RemoveArticle(path string) {
	if s.Articles == nil {
		return
	}
	delete(s.Articles, path)
}

// SeriesSlug is a helper to deterministically produce a slug for series
func SeriesSlug(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = strings.ReplaceAll(s, " ", "-")
	return s
}

// ValidateAgainstBlog ensures the sum's blog entry matches .hashnode/blog.yml
func (s *Sum) ValidateAgainstBlog() error {
	var blog struct {
		PublicationID string `yaml:"publication_id"`
	}

	path := StatePath("blog.yml")
	if err := ReadYAML(path, &blog); err != nil {
		// If blog.yml is missing but we have a valid Sum in memory, we might proceed,
		// but typically this indicates a corrupt environment.
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read %s: %w", path, err)
	}

	if blog.PublicationID != "" && blog.PublicationID != s.Blog.PublicationID {
		return fmt.Errorf("integrity error: hashnode.sum ID (%s) != blog.yml ID (%s)", s.Blog.PublicationID, blog.PublicationID)
	}
	return nil
}
