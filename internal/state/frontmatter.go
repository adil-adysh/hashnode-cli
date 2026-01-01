package state

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ParseTitleFromFrontmatter extracts the `title` field from YAML frontmatter
// if present. It returns empty string when no title is found.
func ParseTitleFromFrontmatter(content []byte) (string, error) {
	// Be forgiving about leading whitespace and CRLFs before/after the delimiters.
	// Trim leading whitespace so frontmatter like "\n---" is accepted.
	s := bytes.TrimLeft(content, " \t\r\n")
	if !bytes.HasPrefix(s, []byte("---")) {
		return "", nil
	}
	// Skip the opening '---'
	s = s[len("---"):]
	// Drop an optional CRLF or LF immediately after the opening marker
	if bytes.HasPrefix(s, []byte("\r\n")) {
		s = s[2:]
	} else if bytes.HasPrefix(s, []byte("\n")) {
		s = s[1:]
	}
	// find closing delimiter (preceded by a newline)
	idx := bytes.Index(s, []byte("\n---"))
	if idx < 0 {
		// try CRLF variant
		idx = bytes.Index(s, []byte("\r\n---"))
		if idx < 0 {
			return "", nil
		}
	}
	fm := s[:idx]
	var m map[string]interface{}
	if err := yaml.Unmarshal(fm, &m); err != nil {
		return "", fmt.Errorf("invalid frontmatter: %w", err)
	}
	if t, ok := m["title"]; ok {
		if s, ok := t.(string); ok {
			return strings.TrimSpace(s), nil
		}
	}
	return "", nil
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
