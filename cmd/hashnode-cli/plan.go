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

		// Prefer deterministic sum file when present; merge with staged metadata for registry info
		var merged []diff.RegistryEntry

		sum, sumErr := state.LoadSum()
		st, serr := state.LoadStage()
		if serr != nil {
			fmt.Printf("❌ Error loading stage: %v\n", serr)
			os.Exit(1)
		}

		// Build map from staged items by path for quick merge
		regMap := make(map[string]diff.RegistryEntry)
		for _, it := range st.Items {
			if it.Type != state.TypeArticle {
				continue
			}
			var meta state.ArticleMeta
			if it.ArticleMeta != nil {
				meta = *it.ArticleMeta
			}
			regMap[it.Key] = diff.RegistryEntry{
				LocalID:      meta.LocalID,
				Title:        meta.Title,
				MarkdownPath: it.Key,
				SeriesID:     meta.SeriesID,
				RemotePostID: meta.RemotePostID,
				Checksum:     it.Checksum,
				LastSyncedAt: meta.LastSyncedAt,
			}
		}

		if sumErr == nil {
			if err := sum.ValidateAgainstBlog(); err != nil {
				fmt.Printf("⚠️  hashnode.sum validation failed: %v; falling back to staged registry\n", err)
			} else {
				// Merge sum entries with staged metadata
				for path, sa := range sum.Articles {
					entry := diff.RegistryEntry{
						MarkdownPath: path,
						RemotePostID: sa.PostID,
						Checksum:     sa.Checksum,
					}
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

		// If sum wasn't usable, but staged registry exists, use staged registry directly
		if len(merged) == 0 {
			if len(regMap) > 0 {
				for _, v := range regMap {
					merged = append(merged, v)
				}
			} else {
				fmt.Printf("❌ No registry data available (sum missing and no staged metadata)\n")
				os.Exit(1)
			}
		}

		// Deterministic ordering by MarkdownPath
		sort.Slice(merged, func(i, j int) bool { return merged[i].MarkdownPath < merged[j].MarkdownPath })

		// Full diff from authoritative applied state -> working tree (disk view)
		diskPlan := diff.FullDiff(merged)

		// Load stage and lock; trust lock staged state as source-of-truth for staged items
		st, err := state.LoadStage()
		if err != nil {
			fmt.Printf("⚠️  failed to load stage: %v\n", err)
			os.Exit(1)
		}

		// Plan used by apply: computed from Stage + Ledger
		stagedPlan := diff.GeneratePlan(merged, st)

		var stagedItems []diff.PlanItem
		var excludedItems []diff.PlanItem
		var unstagedItems []diff.PlanItem
		// build quick map from diskPlan by path for metadata
		planMap := make(map[string]diff.PlanItem)
		for _, item := range diskPlan {
			planMap[item.Path] = item
		}

		// Use stagedPlan entries for staged items, enrich with metadata from diskPlan when available.
		for _, it := range stagedPlan {
			if meta, ok := planMap[it.Path]; ok {
				if it.Title == "" {
					it.Title = meta.Title
				}
			}
			if si, ok := st.Items[it.Path]; ok {
				it.Reason = string(si.Operation)
			}
			stagedItems = append(stagedItems, it)
		}

		// classify diskPlan items as unstaged based on stage.Items
		for _, item := range diskPlan {
			if _, ok := st.Items[item.Path]; !ok {
				unstagedItems = append(unstagedItems, item)
			}
		}

		// Compute summary counts by staged state
		updates := 0
		newCount := 0
		noop := 0
		deletes := 0
		for _, it := range stagedItems {
			switch it.Type {
			case diff.ActionCreate:
				newCount++
			case diff.ActionUpdate:
				updates++
			case diff.ActionDelete:
				deletes++
			case diff.ActionSkip:
				noop++
			}
		}
		_ = len(excludedItems)

		// Compute counts for unstaged (working) items as well
		unstagedChanges := 0
		unstagedNoop := 0
		for _, it := range unstagedItems {
			if it.Type == diff.ActionSkip {
				unstagedNoop++
			} else {
				unstagedChanges++
			}
		}

		// Summary (visible within first 5 lines)
		if planShort {
			// short mode
			if len(stagedItems) == 0 {
				// no staged items: fall back to disk view summary
				diskUpdates := 0
				diskNoop := 0
				for _, it := range diskPlan {
					if it.Type == diff.ActionSkip {
						diskNoop++
					} else {
						diskUpdates++
					}
				}
				if diskNoop > 0 {
					fmt.Printf("✔ %d staged | 🟡 %d updates | ⚪ %d no-op\n", len(stagedItems), diskUpdates, diskNoop)
				} else {
					fmt.Printf("✔ %d staged | 🟡 %d updates\n", len(stagedItems), diskUpdates)
				}
				return
			}
			// staged items present
			if noop > 0 {
				if unstagedChanges > 0 {
					fmt.Printf("✔ %d staged | 🟡 %d updates | ⚪ %d no-op | 🟠 %d working\n", len(stagedItems), updates, noop, unstagedChanges)
				} else {
					fmt.Printf("✔ %d staged | 🟡 %d updates | ⚪ %d no-op\n", len(stagedItems), updates, noop)
				}
			} else {
				if unstagedChanges > 0 {
					fmt.Printf("✔ %d staged | 🟡 %d updates | 🟠 %d working\n", len(stagedItems), updates, unstagedChanges)
				} else {
					fmt.Printf("✔ %d staged | 🟡 %d updates\n", len(stagedItems), updates)
				}
			}
			return
		}

		// Build grouped lists
		var delItems, createItems, updateItemsList []diff.PlanItem
		for _, it := range stagedItems {
			switch it.Type {
			case diff.ActionDelete:
				delItems = append(delItems, it)
			case diff.ActionCreate:
				createItems = append(createItems, it)
			case diff.ActionUpdate:
				updateItemsList = append(updateItemsList, it)
			}
		}

		totalChanges := len(delItems) + len(createItems) + len(updateItemsList)

		// Header summary
		fmt.Println()
		fmt.Printf("📝  PLAN SUMMARY: %d changes to be applied\n", totalChanges)
		fmt.Println("---------------------------------------------------")
		fmt.Printf("   🔴  Deletes: %d\n", len(delItems))
		fmt.Printf("   🟢  Creates: %d\n", len(createItems))
		fmt.Printf("   🟡  Updates: %d\n", len(updateItemsList))
		fmt.Println("---------------------------------------------------")
		fmt.Println()

		// helper to choose reason text
		reasonFor := func(it diff.PlanItem) string {
			if si, ok := st.Items[it.Path]; ok {
				if si.Operation == state.OpDelete {
					return "Marked for removal in stage"
				}
				if it.Type == diff.ActionCreate {
					return "New draft (Local-only)"
				}
				if it.Type == diff.ActionUpdate {
					if si.Snapshot != "" {
						return "Content changed (Snapshot updated)"
					}
					return "Content changed"
				}
			}
			if it.Reason != "" {
				return it.Reason
			}
			return ""
		}

		// Deletions (first — high risk)
		if len(delItems) > 0 {
			fmt.Println("🔴  DELETIONS")
			for _, it := range delItems {
				title := it.Title
				if title == "" {
					title = state.NormalizePath(it.Path)
				}
				fmt.Printf("   %s (%s)\n", title, it.Path)
				fmt.Printf("     └─ Reason: %s\n\n", reasonFor(it))
			}
		}

		// Creations
		if len(createItems) > 0 {
			fmt.Println("🟢  CREATIONS")
			for _, it := range createItems {
				title := it.Title
				if title == "" {
					title = state.NormalizePath(it.Path)
				}
				fmt.Printf("   %s (%s)\n", title, it.Path)
				fmt.Printf("     └─ Reason: %s\n\n", reasonFor(it))
			}
		}

		// Updates
		if len(updateItemsList) > 0 {
			fmt.Println("🟡  UPDATES")
			for _, it := range updateItemsList {
				title := it.Title
				if title == "" {
					title = state.NormalizePath(it.Path)
				}
				fmt.Printf("   %s (%s)\n", title, it.Path)
				fmt.Printf("     └─ Reason: %s\n\n", reasonFor(it))
			}
		}

		fmt.Println("---------------------------------------------------")
		fmt.Println("Run 'hashnode apply' to execute these changes.")
	},
}

func init() {
	rootCmd.AddCommand(planCmd)
	planCmd.Flags().BoolVarP(&planShort, "short", "s", false, "Show compact summary only")
}

var planShort bool
