package state

import (
	"bytes"
	"fmt"
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
			return s, nil
		}
	}
	return "", nil
}
