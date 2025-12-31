package state

import (
	"fmt"
	"os"
)

// ArticleEntry is a local compatibility type for callers that expect the
// legacy registry shape. New code should prefer `StagedItem` and `ArticleMeta`.
type ArticleEntry struct {
	LocalID      string `yaml:"local_id"`
	Title        string `yaml:"title"`
	MarkdownPath string `yaml:"markdown_path"`
	SeriesID     string `yaml:"series_id,omitempty"`
	RemotePostID string `yaml:"remote_post_id,omitempty"`
	Checksum     string `yaml:"checksum"`
	LastSyncedAt string `yaml:"last_synced_at,omitempty"`
}

// ArticlesPath helper
func articlesPath() string {
	return StatePath(ArticlesFile)
}

// LoadArticles loads article entries from the stage-backed items.
func LoadArticles() ([]ArticleEntry, error) {
	st, err := LoadStage()
	if err != nil {
		return nil, err
	}
	var out []ArticleEntry
	for _, item := range st.Items {
		if item.Type != TypeArticle {
			continue
		}
		var meta ArticleMeta
		if item.ArticleMeta != nil {
			meta = *item.ArticleMeta
		}
		out = append(out, ArticleEntry{
			LocalID:      meta.LocalID,
			Title:        meta.Title,
			MarkdownPath: item.Key,
			SeriesID:     meta.SeriesID,
			RemotePostID: meta.RemotePostID,
			Checksum:     item.Checksum,
			LastSyncedAt: meta.LastSyncedAt,
		})
	}
	return out, nil
}

// SaveArticles persists article entries into staged items.
func SaveArticles(list []ArticleEntry) error {
	st, err := LoadStage()
	if err != nil {
		return err
	}
	if st.Items == nil {
		st.Items = make(map[string]StagedItem)
	}
	for _, a := range list {
		key := NormalizePath(a.MarkdownPath)
		si := st.Items[key]
		si.Type = TypeArticle
		si.Key = key
		si.ArticleMeta = &ArticleMeta{
			LocalID:      a.LocalID,
			Title:        a.Title,
			SeriesID:     a.SeriesID,
			RemotePostID: a.RemotePostID,
			LastSyncedAt: a.LastSyncedAt,
		}
		si.Checksum = a.Checksum
		st.Items[key] = si
	}
	return SaveStage(st)
}


// Series registry helpers (simple file-backed list)
func seriesPath() string { return StatePath(SeriesFile) }

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

func SaveSeries(list []SeriesEntry) error {
	path := seriesPath()
	if err := WriteYAML(path, list); err != nil {
		return fmt.Errorf("failed to write series registry: %w", err)
	}
	return nil
}
