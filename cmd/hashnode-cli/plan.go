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

		// Load stage and lock; trust lock staged state as source-of-truth for staged items
		st, err := state.LoadStage()
		if err != nil {
			fmt.Printf("⚠️  failed to load stage: %v\n", err)
			os.Exit(1)
		}

		var stagedItems []diff.PlanItem
		var excludedItems []diff.PlanItem
		var unstagedItems []diff.PlanItem
		// build quick map from plan by path for metadata
		planMap := make(map[string]diff.PlanItem)
		for _, item := range plan {
			planMap[item.Path] = item
		}

		// For each included path, if there's a staged entry in lock, use that state.
		for _, p := range st.Include {
			if s, ok := st.Staged[p]; ok {
				// map staged state to PlanItem.Type
				var t diff.ActionType
				switch s.State {
				case state.ArticleStateNew:
					t = diff.ActionCreate
				case state.ArticleStateUpdate:
					t = diff.ActionUpdate
				case state.ArticleStateDelete:
					t = diff.ActionDelete
				case state.ArticleStateNoop:
					t = diff.ActionSkip
				default:
					t = diff.ActionSkip
				}
				base := planMap[p]
				base.Type = t
				// annotate reason from staged state
				base.Reason = string(s.State)
				stagedItems = append(stagedItems, base)
				continue
			}
			// no staged entry; fall back to scanning plan
			if it, ok := planMap[p]; ok {
				stagedItems = append(stagedItems, it)
				continue
			}
		}

		for _, item := range plan {
			if st.IsExcluded(item.Path) {
				excludedItems = append(excludedItems, item)
				continue
			}
			// if not included, treat as unstaged
			if !st.IsIncluded(item.Path) {
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
					// check for staleness
					staleNote := ""
					if s, ok := st.Staged[it.Path]; ok {
						if state.IsStagingStale(s, it.Path) {
							staleNote = "\n    ⚠️  Article changed after staging — re-stage required"
						}
					}
					fmt.Printf("  - %s\n    Reason: %s%s\n", title, it.Reason, staleNote)
				}
			}
		}

		// Deletes
		if deletes > 0 {
			fmt.Println()
			fmt.Println("🔴 DELETE")
			for _, it := range stagedItems {
				if it.Type == diff.ActionDelete {
					title := it.Title
					if title == "" {
						title = state.NormalizePath(it.Path)
					}
					staleNote := ""
					if s, ok := st.Staged[it.Path]; ok {
						if state.IsStagingStale(s, it.Path) {
							staleNote = "\n    ⚠️  Article changed after staging — re-stage required"
						}
					}
					fmt.Printf("  - %s\n    Reason: %s%s\n", title, it.Reason, staleNote)
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
