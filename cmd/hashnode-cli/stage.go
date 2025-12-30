package main

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
	"strings"

	"adil-adysh/hashnode-cli/internal/diff"
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

		// Merge applied state and registry like plan does
		sum, sumErr := state.LoadSum()
		registry, regErr := state.LoadArticles()
		regMap := make(map[string]state.ArticleEntry)
		if regErr == nil {
			for _, a := range registry {
				regMap[a.MarkdownPath] = a
			}
		}
		var merged []state.ArticleEntry
		if sumErr == nil {
			if err := sum.ValidateAgainstBlog(); err == nil {
				for path, sa := range sum.Articles {
					entry := state.ArticleEntry{MarkdownPath: path, RemotePostID: sa.PostID, Checksum: sa.Checksum}
					if reg, ok := regMap[path]; ok {
						entry.Title = reg.Title
						entry.LocalID = reg.LocalID
						entry.SeriesID = reg.SeriesID
						entry.LastSyncedAt = reg.LastSyncedAt
					}
					merged = append(merged, entry)
					delete(regMap, path)
				}
				for _, rem := range regMap {
					merged = append(merged, rem)
				}
			}
		}
		if len(merged) == 0 {
			if regErr == nil {
				merged = registry
			} else {
				return regErr
			}
		}

		plan := diff.GeneratePlan(merged)

		// Collect staged plan items
		var staged []diff.PlanItem
		for _, it := range plan {
			if st.IsIncluded(it.Path) {
				staged = append(staged, it)
			}
		}

		// Group staged items: updates (CREATE/UPDATE) vs no change (SKIP)
		var willUpdate []diff.PlanItem
		var noChange []diff.PlanItem
		var excludedItems []diff.PlanItem
		for _, it := range plan {
			if st.IsExcluded(it.Path) {
				excludedItems = append(excludedItems, it)
				continue
			}
		}
		for _, it := range staged {
			if it.Type == diff.ActionUpdate || it.Type == diff.ActionCreate {
				willUpdate = append(willUpdate, it)
			} else if it.Type == diff.ActionSkip {
				noChange = append(noChange, it)
			}
		}

		total := len(staged)
		if total == 0 {
			fmt.Println("Staged articles (0):\n  (none)")
			return nil
		}

		fmt.Printf("Staged articles (%d):\n\n", total)
		if len(willUpdate) > 0 {
			fmt.Printf("🟡 Will update (%d)\n", len(willUpdate))
			for _, it := range willUpdate {
				title := it.Title
				if title == "" {
					title = state.NormalizePath(it.Path)
				}
				fmt.Printf("  - %s (%s)\n", title, it.Path)
			}
			fmt.Println()
		}
		if len(noChange) > 0 {
			fmt.Printf("⚪ No changes (%d)\n", len(noChange))
			for _, it := range noChange {
				title := it.Title
				if title == "" {
					title = state.NormalizePath(it.Path)
				}
				fmt.Printf("  - %s (%s)\n", title, it.Path)
			}
			fmt.Println()
		}

		// Excluded (explicitly excluded in stage)
		if len(excludedItems) > 0 {
			fmt.Printf("🚫 Excluded (%d)\n", len(excludedItems))
			for _, it := range excludedItems {
				title := it.Title
				if title == "" {
					title = state.NormalizePath(it.Path)
				}
				fmt.Printf("  - %s (%s)\n", title, it.Path)
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
