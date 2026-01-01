package main

import (
	"adil-adysh/hashnode-cli/internal/diff"
	"adil-adysh/hashnode-cli/internal/state"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	st, err := state.LoadStage()
	if err != nil {
		fmt.Println("LoadStage err:", err)
		return
	}
	var articles []diff.RegistryEntry
	for _, it := range st.Items {
		if it.Type != state.TypeArticle {
			continue
		}
		var meta state.ArticleMeta
		if it.ArticleMeta != nil {
			meta = *it.ArticleMeta
		}
		articles = append(articles, diff.RegistryEntry{
			LocalID: meta.LocalID, Title: meta.Title, MarkdownPath: it.Key, SeriesID: meta.SeriesID, RemotePostID: meta.RemotePostID, Checksum: it.Checksum,
		})
	}
	plan := diff.GeneratePlan(articles, st)
	for _, it := range plan {
		if it.Type != diff.ActionCreate {
			continue
		}
		fmt.Println("Plan create:", it.Path)
		np := state.NormalizePath(it.Path)
		si, ok := st.Items[np]
		if !ok {
			fmt.Println(" no staged item")
			continue
		}
		fmt.Println(" staged snapshot:", si.Snapshot)
		if si.Snapshot != "" {
			c, err := state.GetSnapshotContent(si.Snapshot)
			if err != nil {
				fmt.Println(" snapshot read err:", err)
			} else {
				// show raw fm parse
				if len(c) > 0 {
					fmt.Println(" raw snapshot content:\n", string(c))
					if t, _ := state.ParseTitleFromFrontmatter(c); true {
						fmt.Printf(" title from snapshot: %q\n", t)
					}
				}
			}
		}
		// disk
		fsPath := filepath.FromSlash(it.Path)
		if !filepath.IsAbs(fsPath) {
			fsPath = filepath.Join(state.ProjectRootOrCwd(), fsPath)
		}
		c, err := os.ReadFile(fsPath)
		fmt.Println(" disk read err:", err)
		if err == nil {
			t, _ := state.ParseTitleFromFrontmatter(c)
			fmt.Printf(" title from disk: %q\n", t)
		}
	}
}
