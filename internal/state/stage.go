package state

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// StageFilename is defined in consts.go

// Stage represents the declarative staging area (no runtime metadata)
type Stage struct {
	Version int                      `yaml:"version"`
	Include []string                 `yaml:"include"`
	Exclude []string                 `yaml:"exclude"`
	Staged  map[string]StagedArticle `yaml:"staged,omitempty"`
}

// stagePath returns the project-root path to hashnode.stage
func stagePath() string {
	return StatePath(StageFilename)
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
	return ProjectRootOrCwd()
}

// LoadStage reads hashnode.stage; if missing, returns an empty Stage (version 1)
func LoadStage() (*Stage, error) {
	path := stagePath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Stage{Version: 1, Include: []string{}, Exclude: []string{}, Staged: map[string]StagedArticle{}}, nil
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
	if s.Staged == nil {
		s.Staged = map[string]StagedArticle{}
	}
	return &s, nil
}

// SaveStage writes the stage file deterministically
func SaveStage(s *Stage) error {
	if err := EnsureStateDir(); err != nil {
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
	if s.Staged == nil {
		s.Staged = map[string]StagedArticle{}
	}
	// marshal
	data, err := yaml.Marshal(s)
	if err != nil {
		return fmt.Errorf("failed to marshal stage: %w", err)
	}
	return AtomicWriteFile(stagePath(), data, FilePerm)
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

// The project root is located by `ProjectRoot` (requires .hashnode and hashnode.sum)
// Callers that previously used a string-returning helper should use
// `ProjectRootOrCwd()` which falls back to the current working directory.

// inRepo reports whether the absolute path is under the repository root
func inRepo(abs string) bool {
	root := ProjectRootOrCwd()
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

	// persist stage file
	if err := SaveStage(st); err != nil {
		return err
	}

	// compute staged article state and save to lock
	// build merged entry from sum + registry like plan
	mergedMap := map[string]ArticleEntry{}
	sum, _ := LoadSum()
	reg, _ := LoadArticles()
	regMap := make(map[string]ArticleEntry)
	for _, a := range reg {
		regMap[a.MarkdownPath] = a
	}
	if sum != nil {
		if err := sum.ValidateAgainstBlog(); err == nil {
			for path, sa := range sum.Articles {
				entry := ArticleEntry{MarkdownPath: path, RemotePostID: sa.PostID, Checksum: sa.Checksum}
				if r, ok := regMap[path]; ok {
					entry.Title = r.Title
					entry.LocalID = r.LocalID
					entry.SeriesID = r.SeriesID
					entry.LastSyncedAt = r.LastSyncedAt
				}
				mergedMap[path] = entry
				delete(regMap, path)
			}
			for _, rem := range regMap {
				mergedMap[rem.MarkdownPath] = rem
			}
		}
	}
	if len(mergedMap) == 0 {
		for _, r := range reg {
			mergedMap[r.MarkdownPath] = r
		}
	}

	entry, ok := mergedMap[np]
	if !ok {
		// not tracked; should not happen because we checked earlier
		return fmt.Errorf("file is not tracked; cannot stage: %s", path)
	}

	stateType, localCS, remoteCS, err := ComputeArticleState(entry)
	if err != nil {
		return err
	}

	// persist staged state into the stage file
	if st.Staged == nil {
		st.Staged = map[string]StagedArticle{}
	}
	st.Staged[np] = StagedArticle{
		ID:    entry.RemotePostID,
		State: stateType,
		Checksum: checksumPair{
			Local:  localCS,
			Remote: remoteCS,
		},
	}
	if err := SaveStage(st); err != nil {
		return err
	}
	return nil
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

	// compute staged states and persist into the stage file
	// (persisted per-article state stored in Stage.Staged)

	// build merged map like StageFile
	regMap := make(map[string]ArticleEntry)
	for _, a := range arts {
		regMap[a.MarkdownPath] = a
	}
	sum, _ := LoadSum()
	mergedMap := map[string]ArticleEntry{}
	if sum != nil {
		if err := sum.ValidateAgainstBlog(); err == nil {
			for path, sa := range sum.Articles {
				entry := ArticleEntry{MarkdownPath: path, RemotePostID: sa.PostID, Checksum: sa.Checksum}
				if r, ok := regMap[path]; ok {
					entry.Title = r.Title
					entry.LocalID = r.LocalID
					entry.SeriesID = r.SeriesID
					entry.LastSyncedAt = r.LastSyncedAt
				}
				mergedMap[path] = entry
				delete(regMap, path)
			}
			for _, rem := range regMap {
				mergedMap[rem.MarkdownPath] = rem
			}
		}
	}
	if len(mergedMap) == 0 {
		for _, r := range arts {
			mergedMap[r.MarkdownPath] = r
		}
	}

	for _, p := range staged {
		if entry, ok := mergedMap[p]; ok {
			s, localCS, remoteCS, err := ComputeArticleState(entry)
			if err != nil {
				// skip computing this one but continue
				continue
			}
			st.Staged[p] = StagedArticle{
				ID:    entry.RemotePostID,
				State: s,
				Checksum: checksumPair{
					Local:  localCS,
					Remote: remoteCS,
				},
			}
		}
	}
	if err := SaveStage(st); err != nil {
		return nil, nil, err
	}

	return staged, skipped, nil
}

// MigrateStagedPathsByRemote updates staged include/exclude and staged map keys
// when articles are imported/renamed. It matches by RemotePostID and moves the
// staged entry to the new path so intent is preserved across imports.
func MigrateStagedPathsByRemote(newArticles []ArticleEntry) error {
	st, err := LoadStage()
	if err != nil {
		return err
	}
	if st.Staged == nil {
		st.Staged = map[string]StagedArticle{}
	}

	// build map remoteID -> newPath
	remoteToPath := make(map[string]string)
	for _, a := range newArticles {
		if a.RemotePostID != "" {
			remoteToPath[a.RemotePostID] = NormalizePath(a.MarkdownPath)
		}
	}

	// For each staged entry, if it has an ID that matches a new article, rename key
	moved := 0
	for oldPath, sa := range st.Staged {
		if sa.ID == "" {
			continue
		}
		if newPath, ok := remoteToPath[sa.ID]; ok {
			if oldPath == newPath {
				continue
			}
			// move staged entry
			st.Staged[newPath] = sa
			delete(st.Staged, oldPath)
			// update include list
			for i, p := range st.Include {
				if p == oldPath {
					st.Include[i] = newPath
				}
			}
			// update exclude list
			for i, p := range st.Exclude {
				if p == oldPath {
					st.Exclude[i] = newPath
				}
			}
			moved++
		}
	}

	if moved > 0 {
		return SaveStage(st)
	}
	return nil
}

// SetStagedEntry adds or updates a staged article entry and ensures it's included
// in the stage include list. Path should be repository-relative or absolute.
func SetStagedEntry(path string, id string, astate ArticleState, localChecksum, remoteChecksum string) error {
	st, err := LoadStage()
	if err != nil {
		return err
	}
	if st.Staged == nil {
		st.Staged = map[string]StagedArticle{}
	}
	np := NormalizePath(path)
	// add to include if missing
	found := false
	for _, p := range st.Include {
		if p == np {
			found = true
			break
		}
	}
	if !found {
		st.Include = append(st.Include, np)
	}
	st.Staged[np] = StagedArticle{
		ID:    id,
		State: astate,
		Checksum: checksumPair{
			Local:  localChecksum,
			Remote: remoteChecksum,
		},
	}
	return SaveStage(st)
}
