package state

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// SeriesEntry represents a declarative local series registry entry
type SeriesEntry struct {
	SeriesID    string `yaml:"series_id"`
	Name        string `yaml:"name"`
	Slug        string `yaml:"slug"`
	Description string `yaml:"description"`
}

// seriesFile is the repo-local registry file under .hashnode/
const seriesFile = "series.yml"

// seriesPath returns the path to .hashnode/series.yml in the current working directory
func seriesPath() string {
	return StatePath(seriesFile)
}

// LoadSeries reads the series registry. Returns empty slice if file doesn't exist.
func LoadSeries() ([]SeriesEntry, error) {
	path := seriesPath()
	var list []SeriesEntry
	if err := ReadYAML(path, &list); err != nil {
		if os.IsNotExist(err) {
			return []SeriesEntry{}, nil
		}
		return nil, fmt.Errorf("failed to read series registry: %w", err)
	}
	return list, nil
}

// SaveSeries writes the provided list to .hashnode/series.yml (overwrites atomically)
func SaveSeries(list []SeriesEntry) error {
	path := seriesPath()
	if err := WriteYAML(path, list); err != nil {
		return fmt.Errorf("failed to write series registry: %w", err)
	}
	return nil
}

// Slugify converts a human name into a deterministic slug used for local mapping
func Slugify(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	// replace non-alphanumeric characters with hyphens
	// keep letters, numbers and spaces
	re := regexp.MustCompile(`[^a-z0-9\s-]`)
	s = re.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, " ", "-")
	// collapse multiple hyphens
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	s = strings.Trim(s, "-")
	if s == "" {
		return fmt.Sprintf("untitled-%d", os.Getpid())
	}
	return s
}
