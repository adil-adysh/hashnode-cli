package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"adil-adysh/hashnode-cli/internal/state"
)

var stageCmd = &cobra.Command{
	Use:   "stage",
	Short: "Manage staging area (select what will be applied)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// top-level convenience: `hn stage <path>` behaves like `hn stage add <path>`
		if len(args) == 0 {
			return cmd.Usage()
		}
		p := args[0]
		info, err := os.Stat(p)
		if err == nil && info.IsDir() {
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
		if err := state.StageAdd(p); err != nil {
			return err
		}
		fmt.Printf("✔ 1 article staged (%s)\n", state.NormalizePath(p))
		fmt.Println("Next: hashnode stage list | hashnode plan")
		return nil
	},
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
		if err := state.StageAdd(p); err != nil {
			return err
		}
		fmt.Printf("✔ 1 article staged (%s)\n", state.NormalizePath(p))
		fmt.Println("Next: hashnode stage list | hashnode plan")
		return nil
	},
}

var stageAddVerbose bool

var deleteCmd = &cobra.Command{
	Use:   "delete <path>",
	Short: "Mark a file for remote deletion (stage delete)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p := args[0]
		info, err := os.Stat(p)
		isDir := false
		if err == nil && info.IsDir() {
			isDir = true
		}

		if isDir {
			// Walk directory and mark tracked markdown files for deletion
			var marked int
			err := filepath.WalkDir(p, func(path string, d os.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return nil
				}
				ext := strings.ToLower(filepath.Ext(path))
				if ext != ".md" && ext != ".markdown" {
					return nil
				}
				if serr := state.StageRemove(path); serr == nil {
					marked++
				}
				return nil
			})
			if err != nil {
				return err
			}
			fmt.Printf("✔ %d articles marked for deletion under %s\n", marked, p)
			return nil
		}

		// single file
		if err := state.StageRemove(p); err != nil {
			return err
		}
		fmt.Printf("✔ 1 article marked for deletion (%s)\n", state.NormalizePath(p))
		return nil
	},
}

var unstageTopCmd = &cobra.Command{
	Use:   "unstage <path>",
	Short: "Remove a file from the stage (Cancel intent)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p := args[0]
		info, err := os.Stat(p)
		isDir := false
		if err == nil && info.IsDir() {
			isDir = true
		}

		if isDir {
			dirNorm := state.NormalizePath(p)
			dirPrefix := dirNorm
			if dirPrefix != "./" && !strings.HasSuffix(dirPrefix, "/") {
				dirPrefix = dirPrefix + "/"
			}
			st, err := state.LoadStage()
			if err != nil {
				return err
			}
			var removed []string
			for k := range st.Items {
				if dirPrefix == "./" || strings.HasPrefix(k, dirPrefix) {
					removed = append(removed, k)
					delete(st.Items, k)
				}
			}
			if err := state.SaveStage(st); err != nil {
				return err
			}
			// Clean up any unreferenced snapshots now that items were unstaged
			if n, cerr := state.GCStaleSnapshots(); cerr == nil && n > 0 {
				fmt.Printf("🧹 Removed %d unreferenced snapshots\n", n)
			}
			fmt.Printf("✔ %d articles removed from stage under %s\n", len(removed), p)
			return nil
		}

		if err := state.Unstage(p); err != nil {
			return err
		}
		// After unstaging a single file, run snapshot GC to free unused snapshots
		if n, cerr := state.GCStaleSnapshots(); cerr == nil && n > 0 {
			fmt.Printf("🧹 Removed %d unreferenced snapshots\n", n)
		}
		fmt.Printf("✔ 1 article removed from stage (%s)\n", state.NormalizePath(p))
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

		// build merged entries map (sum + staged metadata) for titles/metadata
		sum, _ := state.LoadSum()
		mergedMap := map[string]struct {
			Title        string
			LocalID      string
			Checksum     string
			RemotePostID string
		}{}
		if sum != nil {
			if err := sum.ValidateAgainstBlog(); err == nil {
				for path, sa := range sum.Articles {
					mergedMap[path] = struct{ Title, LocalID, Checksum, RemotePostID string }{
						Title:        "",
						LocalID:      "",
						Checksum:     sa.Checksum,
						RemotePostID: sa.PostID,
					}
				}
			}
		}
		// overlay staged metadata for titles/local ids/checksums
		for _, it := range st.Items {
			if it.Type != state.TypeArticle {
				continue
			}
			var title, localID string
			if it.ArticleMeta != nil {
				title = it.ArticleMeta.Title
				localID = it.ArticleMeta.LocalID
			}
			mergedMap[it.Key] = struct{ Title, LocalID, Checksum, RemotePostID string }{
				Title:    title,
				LocalID:  localID,
				Checksum: it.Checksum,
				RemotePostID: func() string {
					if it.ArticleMeta != nil {
						return it.ArticleMeta.RemotePostID
					}
					return ""
				}(),
			}
		}

		// Collect staged entries from include list
		var newItems []string
		var updateItems []string
		var noopItems []string
		var deleteItems []string
		for p, si := range st.Items {
			// prefer staged item's declared operation
			switch si.Operation {
			case state.OpModify:
				// compute semantic state if we have metadata
				if e, ok := mergedMap[p]; ok {
					stt, _, _, _ := state.ComputeArticleState(p, e.Checksum, e.RemotePostID)
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
				} else {
					// default to update for modify operations without metadata
					updateItems = append(updateItems, p)
				}
			case state.OpDelete:
				deleteItems = append(deleteItems, p)
			default:
				noopItems = append(noopItems, p)
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

		return nil
	},
}

func init() {
	rootCmd.AddCommand(stageCmd)
	stageCmd.AddCommand(stageAddCmd)
	stageCmd.AddCommand(stageListCmd)
	// top-level delete command for staging deletions
	rootCmd.AddCommand(deleteCmd)
	// top-level unstage convenience
	rootCmd.AddCommand(unstageTopCmd)
	stageAddCmd.Flags().BoolVarP(&stageAddVerbose, "verbose", "v", false, "Print every staged and skipped file")
}
