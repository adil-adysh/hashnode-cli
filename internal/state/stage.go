package state

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const StageFilename = "hashnode.stage"

// Stage represents the declarative staging area (no runtime metadata)
type Stage struct {
	Version int      `yaml:"version"`
	Include []string `yaml:"include"`
	Exclude []string `yaml:"exclude"`
}

// stagePath returns the repo-root path to hashnode.stage
func stagePath() string {
	return filepath.Join(repoRoot(), StateDir, StageFilename)
}

// NormalizePath returns a repository-relative, forward-slash prefixed path
// e.g. ./posts/foo.md
func NormalizePath(p string) string {
	// make clean and relative
	rp := filepath.ToSlash(filepath.Clean(p))
	if len(rp) == 0 {
		return ""
	}
	// if absolute, make relative to cwd
	if filepath.IsAbs(p) {
		if rel, err := filepath.Rel(getCwdOrDot(), p); err == nil {
			rp = filepath.ToSlash(filepath.Clean(rel))
		}
	}
	if rp[0] != '.' && rp[0] != '/' {
		rp = "./" + rp
	}
	return rp
}

func getCwdOrDot() string {
	return repoRoot()
}

// LoadStage reads hashnode.stage; if missing, returns an empty Stage (version 1)
func LoadStage() (*Stage, error) {
	path := stagePath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Stage{Version: 1, Include: []string{}, Exclude: []string{}}, nil
		}
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}
	var s Stage
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("invalid yaml %s: %w", path, err)
	}
	if s.Version == 0 {
		s.Version = 1
	}
	return &s, nil
}

// SaveStage writes the stage file deterministically
func SaveStage(s *Stage) error {
	// ensure state dir exists at repo root
	dir := filepath.Join(repoRoot(), StateDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to ensure state dir: %w", err)
	}
	// normalize and sort entries for determinism
	uniq := func(list []string) []string {
		m := make(map[string]struct{})
		var out []string
		for _, p := range list {
			np := NormalizePath(p)
			if np == "" {
				continue
			}
			if _, ok := m[np]; ok {
				continue
			}
			m[np] = struct{}{}
			out = append(out, np)
		}
		sort.Strings(out)
		return out
	}
	s.Include = uniq(s.Include)
	s.Exclude = uniq(s.Exclude)
	// marshal
	data, err := yaml.Marshal(s)
	if err != nil {
		return fmt.Errorf("failed to marshal stage: %w", err)
	}
	return os.WriteFile(stagePath(), data, 0644)
}

// IsIncluded reports whether path is explicitly included in the stage
func (s *Stage) IsIncluded(path string) bool {
	np := NormalizePath(path)
	for _, p := range s.Include {
		if p == np {
			return true
		}
	}
	return false
}

// IsExcluded reports whether path is explicitly excluded in the stage
func (s *Stage) IsExcluded(path string) bool {
	np := NormalizePath(path)
	for _, p := range s.Exclude {
		if p == np {
			return true
		}
	}
	return false
}

// Clear empties the stage (used after successful apply)
func (s *Stage) Clear() {
	s.Include = []string{}
	s.Exclude = []string{}
}

// repoRoot locates the repository root by finding a folder that contains StateDir.
// If not found, returns the current working directory.
// The repoRoot function is defined in lock.go and should be used instead.

// inRepo reports whether the absolute path is under the repository root
func inRepo(abs string) bool {
	root := repoRoot()
	rabs, err := filepath.Abs(root)
	if err != nil {
		rabs = root
	}
	pabs, err := filepath.Abs(abs)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(rabs, pabs)
	if err != nil {
		return false
	}
	return rel == "." || !strings.HasPrefix(rel, "..")
}

// StageFile stages a single file. It fails if the file is not tracked.
func StageFile(path string) error {
	// ensure exists and is file
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("path does not exist: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("path is a directory")
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}
	if !inRepo(abs) {
		return fmt.Errorf("path is outside repository: %s", path)
	}

	// load registry to check tracked
	arts, err := LoadArticles()
	if err != nil {
		return fmt.Errorf("failed to load article registry: %w", err)
	}
	tracked := make(map[string]struct{})
	for _, a := range arts {
		tracked[NormalizePath(a.MarkdownPath)] = struct{}{}
	}

	np := NormalizePath(path)
	if _, ok := tracked[np]; !ok {
		return fmt.Errorf("file is not tracked; cannot stage: %s", path)
	}

	st, err := LoadStage()
	if err != nil {
		return err
	}
	// remove from exclude
	var newEx []string
	for _, e := range st.Exclude {
		if e == np {
			continue
		}
		newEx = append(newEx, e)
	}
	st.Exclude = newEx
	st.Include = append(st.Include, np)
	return SaveStage(st)
}

// StageDir enumerates files under directory and stages tracked ones.
// Returns lists of staged and skipped (untracked or excluded) paths.
func StageDir(dir string) ([]string, []string, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, nil, fmt.Errorf("path does not exist: %w", err)
	}
	if !info.IsDir() {
		return nil, nil, fmt.Errorf("path is not a directory: %s", dir)
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to resolve dir: %w", err)
	}
	if !inRepo(absDir) {
		return nil, nil, fmt.Errorf("path is outside repository: %s", dir)
	}

	// load registry and stage
	arts, err := LoadArticles()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load article registry: %w", err)
	}
	st, err := LoadStage()
	if err != nil {
		return nil, nil, err
	}

	tracked := make(map[string]struct{})
	for _, a := range arts {
		tracked[NormalizePath(a.MarkdownPath)] = struct{}{}
	}

	var staged []string
	var skipped []string

	// Walk the directory and consider files
	err = filepath.WalkDir(dir, func(p string, d os.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		if d.IsDir() {
			return nil
		}
		np := NormalizePath(p)
		// if explicitly excluded, skip
		if st.IsExcluded(np) {
			skipped = append(skipped, np)
			return nil
		}
		// if tracked, stage
		if _, ok := tracked[np]; ok {
			st.Include = append(st.Include, np)
			staged = append(staged, np)
			return nil
		}
		// else untracked skip
		skipped = append(skipped, np)
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	// persist stage deterministically
	if err := SaveStage(st); err != nil {
		return nil, nil, err
	}
	return staged, skipped, nil
}
