package tools

import (
	"adil-adysh/hashnode-cli/internal/diff"
	"adil-adysh/hashnode-cli/internal/state"
	"fmt"
	"os"
	"path/filepath"
)

func ValidateMain() {
	st, err := state.LoadStage()
	if err != nil {
		fmt.Println("LoadStage err:", err)
		return
	}
	sum, _ := state.LoadSum()

	var articles []diff.RegistryEntry
	for _, it := range st.Items {
		if it.Type != state.TypeArticle {
			continue
		}
		var entry diff.RegistryEntry
		entry.MarkdownPath = it.Key
		entry.Checksum = it.Checksum

		// Get metadata from ledger
		if sum != nil {
			if a, ok := sum.Articles[it.Key]; ok {
				entry.RemotePostID = a.PostID
				entry.Title = a.Title
			}
		}
		articles = append(articles, entry)
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
			snapStore := state.NewSnapshotStore()
			c, err := snapStore.Get(si.Snapshot)
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
			fmt.Println(" title from disk:'", t, "'")
		}
	}
}
