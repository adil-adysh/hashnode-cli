package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"adil-adysh/hashnode-cli/internal/state"
)

var stageCmd = &cobra.Command{
	Use:   "stage",
	Short: "Manage staging area (select what will be applied)",
}

var stageAddCmd = &cobra.Command{
	Use:   "add <path>",
	Short: "Add a tracked file to the staging include list",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p := args[0]
		info, err := os.Stat(p)
		if err != nil {
			return fmt.Errorf("path does not exist: %s", p)
		}

		// directory: stage all tracked files under it
		if info.IsDir() {
			fmt.Printf("➕ Staging tracked articles under %s\n\n", p)
			staged, skipped, err := state.StageDir(p)
			if err != nil {
				return err
			}
			fmt.Printf("✔ %d articles staged\n", len(staged))
			fmt.Printf("ℹ️  %d files ignored (not Hashnode articles)\n\n", len(skipped))
			fmt.Println("Next:")
			fmt.Println("  • Review staged changes: hashnode stage list")
			fmt.Println("  • Preview publish plan: hashnode plan")

			if stageAddVerbose {
				if len(staged) > 0 {
					fmt.Println("Staged:")
					for _, s := range staged {
						fmt.Printf("  - %s\n", s)
					}
				}
				if len(skipped) > 0 {
					fmt.Println("Ignored:")
					for _, s := range skipped {
						fmt.Printf("  - %s\n", s)
					}
				}
			}
			return nil
		}

		// file: stage single file
		if err := state.StageFile(p); err != nil {
			return err
		}
		fmt.Printf("✔ 1 article staged (%s)\n", state.NormalizePath(p))
		fmt.Println("Next: hashnode stage list | hashnode plan")
		return nil
	},
}

var stageAddVerbose bool

var stageRemoveCmd = &cobra.Command{
	Use:   "remove <path>",
	Short: "Remove a file from include and add to exclude",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p := args[0]
		st, err := state.LoadStage()
		if err != nil {
			return err
		}

		info, err := os.Stat(p)
		isDir := false
		if err == nil && info.IsDir() {
			isDir = true
		}

		if isDir {
			// remove any staged entries under this directory
			dirNorm := state.NormalizePath(p)
			// ensure prefix ends with '/'
			dirPrefix := dirNorm
			if dirPrefix != "./" && !strings.HasSuffix(dirPrefix, "/") {
				dirPrefix = dirPrefix + "/"
			}
			var newInc []string
			var removed []string
			for _, i := range st.Include {
				if dirPrefix == "./" || strings.HasPrefix(i, dirPrefix) {
					removed = append(removed, i)
					continue
				}
				newInc = append(newInc, i)
			}
			st.Include = newInc
			// add removed entries to exclude so they won't be restaged accidentally
			for _, r := range removed {
				st.Exclude = append(st.Exclude, r)
			}
			if err := state.SaveStage(st); err != nil {
				return err
			}
			fmt.Printf("✔ %d articles removed from stage under %s\n", len(removed), p)
			return nil
		}

		// file: remove single file from include and add to exclude
		norm := state.NormalizePath(p)
		var newInc []string
		removed := false
		for _, i := range st.Include {
			if i == norm {
				removed = true
				continue
			}
			newInc = append(newInc, i)
		}
		st.Include = newInc
		st.Exclude = append(st.Exclude, norm)
		if err := state.SaveStage(st); err != nil {
			return err
		}
		if removed {
			fmt.Printf("✔ 1 article removed from stage (%s)\n", norm)
		} else {
			fmt.Printf("ℹ️  article not staged: %s\n", norm)
		}
		return nil
	},
}

var stageListCmd = &cobra.Command{
	Use:   "list",
	Short: "List staged, excluded and unstaged tracked files",
	RunE: func(cmd *cobra.Command, args []string) error {
		st, err := state.LoadStage()
		if err != nil {
			return err
		}

		// use staged data persisted in `hashnode.stage`

		// build merged entries map (sum + registry) for titles/metadata
		sum, _ := state.LoadSum()
		registry, _ := state.LoadArticles()
		regMap := make(map[string]state.ArticleEntry)
		for _, a := range registry {
			regMap[a.MarkdownPath] = a
		}
		mergedMap := map[string]state.ArticleEntry{}
		if sum != nil {
			if err := sum.ValidateAgainstBlog(); err == nil {
				for path, sa := range sum.Articles {
					entry := state.ArticleEntry{MarkdownPath: path, RemotePostID: sa.PostID, Checksum: sa.Checksum}
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
			for _, r := range registry {
				mergedMap[r.MarkdownPath] = r
			}
		}

		// Collect staged entries from include list
		var newItems []string
		var updateItems []string
		var noopItems []string
		var deleteItems []string
		for _, p := range st.Include {
			// prefer persisted staged data
			if s, ok := st.Staged[p]; ok {
				switch s.State {
				case state.ArticleStateNew:
					newItems = append(newItems, p)
				case state.ArticleStateUpdate:
					updateItems = append(updateItems, p)
				case state.ArticleStateDelete:
					deleteItems = append(deleteItems, p)
				case state.ArticleStateNoop:
					noopItems = append(noopItems, p)
				}
				continue
			}
			// compute on-the-fly if no lock entry
			if e, ok := mergedMap[p]; ok {
				stt, _, _, _ := state.ComputeArticleState(e)
				switch stt {
				case state.ArticleStateNew:
					newItems = append(newItems, p)
				case state.ArticleStateUpdate:
					updateItems = append(updateItems, p)
				case state.ArticleStateDelete:
					deleteItems = append(deleteItems, p)
				case state.ArticleStateNoop:
					noopItems = append(noopItems, p)
				}
			}
		}

		total := len(newItems) + len(updateItems) + len(noopItems) + len(deleteItems)
		if total == 0 {
			fmt.Println("Staged articles (0):\n  (none)")
			return nil
		}

		fmt.Printf("Staged articles (%d):\n\n", total)
		if len(updateItems) > 0 {
			fmt.Printf("🟡 UPDATE (%d)\n", len(updateItems))
			for _, p := range updateItems {
				title := p
				if e, ok := mergedMap[p]; ok && e.Title != "" {
					title = e.Title
				}
				fmt.Printf("  - %s (%s)\n", title, p)
			}
			fmt.Println()
		}
		if len(newItems) > 0 {
			fmt.Printf("🆕 NEW (%d)\n", len(newItems))
			for _, p := range newItems {
				title := p
				if e, ok := mergedMap[p]; ok && e.Title != "" {
					title = e.Title
				}
				fmt.Printf("  - %s (%s)\n", title, p)
			}
			fmt.Println()
		}
		if len(noopItems) > 0 {
			fmt.Printf("⚪ NO CHANGE (%d)\n", len(noopItems))
			for _, p := range noopItems {
				title := p
				if e, ok := mergedMap[p]; ok && e.Title != "" {
					title = e.Title
				}
				fmt.Printf("  - %s (%s)\n", title, p)
			}
			fmt.Println()
		}
		if len(deleteItems) > 0 {
			fmt.Printf("🔴 DELETE (%d)\n", len(deleteItems))
			for _, p := range deleteItems {
				title := p
				if e, ok := mergedMap[p]; ok && e.Title != "" {
					title = e.Title
				}
				fmt.Printf("  - %s (%s)\n", title, p)
			}
			fmt.Println()
		}

		// show excluded paths
		if len(st.Exclude) > 0 {
			fmt.Printf("🚫 EXCLUDED (%d)\n", len(st.Exclude))
			for _, p := range st.Exclude {
				title := p
				if e, ok := mergedMap[p]; ok && e.Title != "" {
					title = e.Title
				}
				fmt.Printf("  - %s (%s)\n", title, p)
			}
			fmt.Println()
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(stageCmd)
	stageCmd.AddCommand(stageAddCmd)
	stageCmd.AddCommand(stageRemoveCmd)
	stageCmd.AddCommand(stageListCmd)
	stageAddCmd.Flags().BoolVarP(&stageAddVerbose, "verbose", "v", false, "Print every staged and skipped file")
}
