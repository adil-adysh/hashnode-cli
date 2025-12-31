package state

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Sum represents the deterministic mapping between repo artifacts and remote IDs
type Sum struct {
	Blog     BlogEntry
	Series   map[string]SeriesEntry // key: slug
	Articles map[string]ArticleSum  // key: relative path
}

type BlogEntry struct {
	PublicationID   string
	PublicationSlug string
}

type ArticleSum struct {
	PostID   string
	Checksum string
}

// SumFile is defined in consts.go

// LoadSum parses hashnode.sum in repo root. Returns os.ErrNotExist if missing.
func LoadSum() (*Sum, error) {
	repoRoot, err := ProjectRoot()
	if err != nil {
		return nil, err
	}
	sumPath := filepath.Join(repoRoot, SumFile)
	file, err := os.Open(sumPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Build an empty Sum with maps preallocated for deterministic behavior.
	sum := &Sum{Series: make(map[string]SeriesEntry), Articles: make(map[string]ArticleSum)}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		switch fields[0] {
		case "blog":
			// blog <slug> id=<id>
			if len(fields) < 3 {
				return nil, fmt.Errorf("invalid blog line: %s", line)
			}
			slug := fields[1]
			id := parseKeyVal(fields[2], "id")
			sum.Blog = BlogEntry{PublicationID: id, PublicationSlug: slug}
		case "series":
			// series <name> id=<id>
			if len(fields) < 3 {
				return nil, fmt.Errorf("invalid series line: %s", line)
			}
			name := fields[1]
			id := parseKeyVal(fields[2], "id")
			slug := SeriesSlug(name)
			sum.Series[slug] = SeriesEntry{SeriesID: id, Name: name, Slug: slug}
		case "article":
			// article <path> id=<id> checksum=sha256:<hex>
			if len(fields) < 3 {
				return nil, fmt.Errorf("invalid article line: %s", line)
			}
			path := fields[1]
			var id, checksum string
			for _, token := range fields[2:] {
				if strings.HasPrefix(token, "id=") {
					id = parseKeyVal(token, "id")
				}
				if strings.HasPrefix(token, "checksum=") {
					checksum = parseKeyVal(token, "checksum")
				}
			}
			sum.Articles[path] = ArticleSum{PostID: id, Checksum: checksum}
		default:
			// ignore unknown lines to remain forward compatible
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return sum, nil
}

func parseKeyVal(token, key string) string {
	parts := strings.SplitN(token, "=", 2)
	if len(parts) != 2 {
		return ""
	}
	return parts[1]
}

// SaveSum writes the sum file deterministically. Overwrites existing file.
func SaveSum(s *Sum) error {
	// Build lines in deterministic order
	var lines []string
	lines = append(lines, "# hashnode integrity ledger v1")
	// blog
	lines = append(lines, fmt.Sprintf("blog %s id=%s", s.Blog.PublicationSlug, s.Blog.PublicationID))

	// series: sort by slug
	var seriesKeys []string
	for k := range s.Series {
		seriesKeys = append(seriesKeys, k)
	}
	sort.Strings(seriesKeys)
	for _, k := range seriesKeys {
		e := s.Series[k]
		lines = append(lines, fmt.Sprintf("series %s id=%s", e.Name, e.SeriesID))
	}

	// articles: sort by path
	var artKeys []string
	for k := range s.Articles {
		artKeys = append(artKeys, k)
	}
	sort.Strings(artKeys)
	for _, k := range artKeys {
		a := s.Articles[k]
		lines = append(lines, fmt.Sprintf("article %s id=%s checksum=%s", k, a.PostID, a.Checksum))
	}

	// Write file
	// Build file contents
	var sb strings.Builder
	for _, l := range lines {
		sb.WriteString(l)
		sb.WriteString("\n")
	}
	repoRoot, err := ProjectRoot()
	if err != nil {
		return err
	}
	sumPath := filepath.Join(repoRoot, SumFile)
	return AtomicWriteFile(sumPath, []byte(sb.String()), FilePerm)
}

// NewSumFromBlog attempts to construct a Sum with Blog info from .hashnode/blog.yml
func NewSumFromBlog() (*Sum, error) {
	var blog struct {
		PublicationID   string `yaml:"publication_id"`
		PublicationSlug string `yaml:"publication_slug"`
	}
	blogPath := StatePath("blog.yml")
	if err := ReadYAML(blogPath, &blog); err != nil {
		return nil, err
	}
	return &Sum{
		Blog:     BlogEntry{PublicationID: blog.PublicationID, PublicationSlug: blog.PublicationSlug},
		Series:   make(map[string]SeriesEntry),
		Articles: make(map[string]ArticleSum),
	}, nil
}

// SetArticle sets or updates an article entry in the sum
func (s *Sum) SetArticle(path, postID, checksum string) {
	if s.Articles == nil {
		s.Articles = make(map[string]ArticleSum)
	}
	s.Articles[path] = ArticleSum{PostID: postID, Checksum: checksum}
}

// RemoveArticle deletes an article entry from the sum
func (s *Sum) RemoveArticle(path string) {
	if s.Articles == nil {
		return
	}
	delete(s.Articles, path)
}

// SeriesSlug is a helper to deterministically produce a slug for series when needed
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
		if os.IsNotExist(err) {
			return fmt.Errorf("%s not found: %w", path, err)
		}
		return fmt.Errorf("failed to read %s: %w", path, err)
	}
	if blog.PublicationID == "" {
		return fmt.Errorf("%s missing publication_id", path)
	}
	if blog.PublicationID != s.Blog.PublicationID {
		return fmt.Errorf("hashnode.sum publication_id (%s) does not match %s (%s)", s.Blog.PublicationID, path, blog.PublicationID)
	}
	return nil
}
