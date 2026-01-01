package state

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Frontmatter captures supported YAML fields for posts.
// Only fields present in the markdown will be set (zero values remain nil).
type Frontmatter struct {
	Title                     string     `yaml:"title"`
	Subtitle                  string     `yaml:"subtitle"`
	Slug                      string     `yaml:"slug"`
	Tags                      []string   `yaml:"tags"`
	Canonical                 string     `yaml:"canonical"`
	CoverImageURL             string     `yaml:"cover_image_url"`
	CoverImageAttribution     string     `yaml:"cover_image_attribution"`
	CoverImagePhotographer    string     `yaml:"cover_image_photographer"`
	CoverImageStickBottom     bool       `yaml:"cover_image_stick_bottom"`
	CoverImageHideAttribution bool       `yaml:"cover_image_hide_attribution"`
	BannerImageURL            string     `yaml:"banner_image_url"`
	DisableComments           *bool      `yaml:"disable_comments"`
	PublishedAt               *time.Time `yaml:"published_at"`
	MetaTitle                 string     `yaml:"meta_title"`
	MetaDescription           string     `yaml:"meta_description"`
	MetaImage                 string     `yaml:"meta_image"`
	PublishAs                 string     `yaml:"publish_as"`
	CoAuthors                 []string   `yaml:"co_authors"`
	Series                    string     `yaml:"series"`
	EnableToc                 *bool      `yaml:"toc"`
	Newsletter                *bool      `yaml:"newsletter"`
	Delisted                  *bool      `yaml:"delisted"`
	Scheduled                 *bool      `yaml:"scheduled"`
	SlugOverridden            *bool      `yaml:"slug_overridden"`
	PinToBlog                 *bool      `yaml:"pin_to_blog"`
}

// ParseTitleFromFrontmatter extracts the `title` field from YAML frontmatter
// if present. It returns empty string when no title is found.
func ParseTitleFromFrontmatter(content []byte) (string, error) {
	fm, _, err := ExtractFrontmatter(content)
	if err != nil {
		return "", err
	}
	if fm == nil {
		return "", nil
	}
	return strings.TrimSpace(fm.Title), nil
}

// StripFrontmatter removes YAML frontmatter from markdown content and returns the body.
// If no frontmatter is present, it returns the original content. Invalid frontmatter
// yields an error so callers can prevent posting malformed payloads.
// ExtractFrontmatter returns the parsed frontmatter (if present) and the markdown body without frontmatter.
// When no frontmatter exists, fm is nil and body is the original content.
func ExtractFrontmatter(content []byte) (*Frontmatter, []byte, error) {
	s := bytes.TrimLeft(content, " \t\r\n")
	if !bytes.HasPrefix(s, []byte("---")) {
		return nil, content, nil
	}

	// Skip opening delimiter
	s = s[len("---"):]
	if bytes.HasPrefix(s, []byte("\r\n")) {
		s = s[2:]
	} else if bytes.HasPrefix(s, []byte("\n")) {
		s = s[1:]
	}

	endDelim := []byte("\n---")
	idx := bytes.Index(s, endDelim)
	consumed := len(endDelim)
	if idx < 0 {
		endDelim = []byte("\r\n---")
		idx = bytes.Index(s, endDelim)
		consumed = len(endDelim)
		if idx < 0 {
			return nil, content, fmt.Errorf("frontmatter end delimiter not found")
		}
	}

	fmBytes := s[:idx]
	var fm Frontmatter
	if err := yaml.Unmarshal(fmBytes, &fm); err != nil {
		return nil, content, fmt.Errorf("invalid frontmatter: %w", err)
	}

	body := s[idx+consumed:]
	// Trim a single leading newline after the closing delimiter
	if bytes.HasPrefix(body, []byte("\r\n")) {
		body = body[2:]
	} else if bytes.HasPrefix(body, []byte("\n")) {
		body = body[1:]
	}
	// Drop any remaining leading blank lines
	body = bytes.TrimLeft(body, "\r\n")

	return &fm, body, nil
}

// StripFrontmatter removes YAML frontmatter and returns the body (compat helper).
func StripFrontmatter(content []byte) ([]byte, error) {
	_, body, err := ExtractFrontmatter(content)
	return body, err
}

// ResolveTitleForPath resolves the title for a file path.
// It tries (in order): ledger cache, snapshot, then disk frontmatter.
// Returns error only if file can't be read; empty title is valid.
func ResolveTitleForPath(path string, sum *Sum, stage *Stage) (string, error) {
	normPath := NormalizePath(path)

	// 1. Try ledger cache first
	if sum != nil {
		if entry, ok := sum.Articles[normPath]; ok && entry.Title != "" {
			return entry.Title, nil
		}
	}

	// 2. Try snapshot if staged
	if stage != nil {
		if si, ok := stage.Items[normPath]; ok && si.Snapshot != "" {
			snapStore := NewSnapshotStore()
			content, err := snapStore.Get(si.Snapshot)
			if err == nil {
				if title, _ := ParseTitleFromFrontmatter(content); title != "" {
					return title, nil
				}
			}
		}
	}

	// 3. Parse from disk
	fsPath := filepath.FromSlash(path)
	if !filepath.IsAbs(fsPath) {
		fsPath = filepath.Join(ProjectRootOrCwd(), fsPath)
	}

	content, err := os.ReadFile(fsPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	title, _ := ParseTitleFromFrontmatter(content)
	return title, nil
}
