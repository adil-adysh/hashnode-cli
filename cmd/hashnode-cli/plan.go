package main

import (
	"fmt"
	"os"
	"sort"

	"adil-adysh/hashnode-cli/internal/diff"
	"adil-adysh/hashnode-cli/internal/state"

	"github.com/spf13/cobra"
)

var planCmd = &cobra.Command{
	Use:   "plan",
	Short: "Show planned changes between local and last sync",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("📋 Publish plan summary")

		// Prefer deterministic sum file when present; merge with article registry for metadata
		var merged []state.ArticleEntry

		sum, sumErr := state.LoadSum()
		registry, regErr := state.LoadArticles()

		// Build map from registry by path for quick merge
		regMap := make(map[string]state.ArticleEntry)
		if regErr == nil {
			for _, a := range registry {
				regMap[a.MarkdownPath] = a
			}
		}

		if sumErr == nil {
			if err := sum.ValidateAgainstBlog(); err != nil {
				fmt.Printf("⚠️  hashnode.sum validation failed: %v; falling back to article registry\n", err)
			} else {
				// Merge sum entries with registry metadata (title, local id)
				for path, sa := range sum.Articles {
					entry := state.ArticleEntry{
						MarkdownPath: path,
						RemotePostID: sa.PostID,
						Checksum:     sa.Checksum,
					}
					if reg, ok := regMap[path]; ok {
						// copy metadata from registry
						entry.Title = reg.Title
						entry.LocalID = reg.LocalID
						entry.SeriesID = reg.SeriesID
						entry.LastSyncedAt = reg.LastSyncedAt
					}
					merged = append(merged, entry)
					// mark consumed
					delete(regMap, path)
				}
				// Any remaining registry entries (not present in sum) should be included
				for _, rem := range regMap {
					merged = append(merged, rem)
				}
			}
		}

		// If sum wasn't usable, but registry exists, use registry directly
		if len(merged) == 0 {
			if regErr == nil {
				merged = registry
			} else {
				fmt.Printf("❌ Error loading article registry: %v\n", regErr)
				os.Exit(1)
			}
		}

		// Deterministic ordering by MarkdownPath
		sort.Slice(merged, func(i, j int) bool { return merged[i].MarkdownPath < merged[j].MarkdownPath })

		// Full diff from authoritative applied state -> working tree
		plan := diff.GeneratePlan(merged)

		// Load stage and partition plan according to include/exclude
		st, err := state.LoadStage()
		if err != nil {
			fmt.Printf("⚠️  failed to load stage: %v\n", err)
			os.Exit(1)
		}

		var stagedItems []diff.PlanItem
		var excludedItems []diff.PlanItem
		var unstagedItems []diff.PlanItem
		for _, item := range plan {
			if st.IsIncluded(item.Path) {
				stagedItems = append(stagedItems, item)
				continue
			}
			if st.IsExcluded(item.Path) {
				excludedItems = append(excludedItems, item)
				continue
			}
			unstagedItems = append(unstagedItems, item)
		}

		// Compute summary counts
		updates := 0
		noop := 0
		for _, it := range stagedItems {
			if it.Type == diff.ActionCreate || it.Type == diff.ActionUpdate {
				updates++
			} else if it.Type == diff.ActionSkip {
				noop++
			}
		}
		excludedCount := len(excludedItems)

		// Summary (visible within first 5 lines)
		if planShort {
			// short mode
			fmt.Printf("✔ %d staged | 🟡 %d updates | ⚪ %d no-op\n", len(stagedItems), updates, noop)
			return
		}

		fmt.Println()
		fmt.Printf("🟡 Updates: %d\n", updates)
		fmt.Printf("⚪ No-op: %d\n", noop)
		if excludedCount > 0 {
			fmt.Printf("🚫 Excluded: %d\n", excludedCount)
		}
		fmt.Println()
		fmt.Println("Details:")

		// Details grouped by outcome — only show create/update/delete (CRUD)
		// Updates (CREATE/UPDATE)
		if updates > 0 {
			fmt.Println()
			fmt.Println("🟡 UPDATE")
			for _, it := range stagedItems {
				if it.Type == diff.ActionCreate || it.Type == diff.ActionUpdate {
					title := it.Title
					if title == "" {
						title = state.NormalizePath(it.Path)
					}
					fmt.Printf("  - %s\n    Reason: %s\n", title, it.Reason)
				}
			}
		}

		// Deletes
		delCount := 0
		for _, it := range stagedItems {
			if it.Type == diff.ActionDelete {
				delCount++
			}
		}
		if delCount > 0 {
			fmt.Println()
			fmt.Println("🔴 DELETE")
			for _, it := range stagedItems {
				if it.Type == diff.ActionDelete {
					title := it.Title
					if title == "" {
						title = state.NormalizePath(it.Path)
					}
					fmt.Printf("  - %s\n    Reason: %s\n", title, it.Reason)
				}
			}
		}

		// Excluded (only show if non-empty)
		if excludedCount > 0 {
			fmt.Println()
			fmt.Println("🚫 EXCLUDED")
			for _, it := range excludedItems {
				title := it.Title
				if title == "" {
					title = state.NormalizePath(it.Path)
				}
				fmt.Printf("  - %s\n", title)
			}
		}

		fmt.Println()
		fmt.Println("Ready to publish:")
		fmt.Println("  hashnode apply")
	},
}

func init() {
	rootCmd.AddCommand(planCmd)
	planCmd.Flags().BoolVarP(&planShort, "short", "s", false, "Show compact summary only")
}

var planShort bool
