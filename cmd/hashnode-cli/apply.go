package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"github.com/Khan/genqlient/graphql"
	"github.com/spf13/cobra"

	"adil-adysh/hashnode-cli/internal/api"
	"adil-adysh/hashnode-cli/internal/config"
	"adil-adysh/hashnode-cli/internal/diff"
	"adil-adysh/hashnode-cli/internal/state"
)

// reuses authedTransport declared in init.go

var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply planned changes",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Acquire repo lock
		release, err := state.AcquireRepoLock()
		if err != nil {
			return fmt.Errorf("failed to acquire repo lock: %w", err)
		}
		defer func() {
			if err := release(); err != nil {
				fmt.Printf("warning: failed to remove lock: %v\n", err)
			}
		}()

		// Load user config for token
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load home config (run init): %w", err)
		}
		if cfg.Token == "" {
			return fmt.Errorf("no token configured; run 'hashnode init'")
		}

		httpClient := &http.Client{Transport: &authedTransport{token: cfg.Token, wrapped: http.DefaultTransport}}
		client := graphql.NewClient("https://gql.hashnode.com", httpClient)

		// Load article registry
		articles, err := state.LoadArticles()
		if err != nil {
			return fmt.Errorf("failed to load article registry: %w", err)
		}

		// Load stage and determine which paths are staged
		st, err := state.LoadStage()
		if err != nil {
			return fmt.Errorf("failed to load stage: %w", err)
		}
		// If no staged items, warn and exit without changes
		if len(st.Items) == 0 {
			fmt.Println("No staged changes found in hashnode.stage; nothing to apply.")
			return nil
		}

		// Compute plan from the Stage (intent) and Ledger (articles)
		plan := diff.GeneratePlan(articles, st)

		// Build set of staged include paths for quick reference
		stagedPaths := make(map[string]struct{})
		for p := range st.Items {
			stagedPaths[state.NormalizePath(p)] = struct{}{}
		}

		// Load or construct sum
		var s *state.Sum
		if ss, err := state.LoadSum(); err == nil {
			if err := ss.ValidateAgainstBlog(); err == nil {
				s = ss
			}
		}
		if s == nil {
			s, _ = state.NewSumFromBlog()
		}

		// Build lookup from existing registry
		regByPath := make(map[string]state.ArticleEntry)
		for _, a := range articles {
			regByPath[state.NormalizePath(a.MarkdownPath)] = a
		}

		var updatedArticles []state.ArticleEntry

		// Preserve unstaged entries as-is
		for _, a := range articles {
			if _, ok := stagedPaths[state.NormalizePath(a.MarkdownPath)]; !ok {
				updatedArticles = append(updatedArticles, a)
			}
		}

		// Apply plan items in order
		for _, it := range plan {
			np := state.NormalizePath(it.Path)
			switch it.Type {
			case diff.ActionSkip:
				// nothing to do
				continue
			case diff.ActionDelete:
				// delete remote post if exists
				var remoteID string
				if it.RemoteID != "" {
					remoteID = it.RemoteID
				} else if e, ok := regByPath[np]; ok {
					remoteID = e.RemotePostID
				}
				if remoteID == "" {
					// nothing to delete
					continue
				}
				if !applyYes {
					return fmt.Errorf("deletion required for %s (remote id=%s). Re-run with --yes to confirm deletions", it.Path, remoteID)
				}
				if _, derr := api.DeletePost(context.Background(), client, remoteID); derr != nil {
					return fmt.Errorf("delete failed for %s (remote id=%s): %w", it.Path, remoteID, derr)
				}
				s.RemoveArticle(np)
				fmt.Printf("Deleted remote post for %s -> %s\n", it.Path, remoteID)
			case diff.ActionUpdate:
				// find remote id and local metadata
				var entry state.ArticleEntry
				var ok bool
				if entry, ok = regByPath[np]; !ok && it.OldPath != "" {
					entry, ok = regByPath[state.NormalizePath(it.OldPath)]
				}
				if !ok {
					// nothing to update (shouldn't happen)
					continue
				}
				// staleness check using new staged item schema
				if si, ok := st.Items[np]; ok {
					if state.IsStagingItemStale(si, it.Path) {
						if !applyYes {
							return fmt.Errorf("staged content changed for %s; re-stage or rerun with --yes to force", it.Path)
						}
						fmt.Printf("warning: forcing apply despite staged content changes for %s\n", it.Path)
					}
				}
				// Load content from snapshot when available, otherwise disk
				var contentBytes []byte
				var rerr error
				if si, ok := st.Items[np]; ok && si.Snapshot != "" {
					contentBytes, rerr = state.GetSnapshotContent(si.Snapshot)
				} else {
					fsPath := filepath.FromSlash(np)
					if !filepath.IsAbs(fsPath) {
						fsPath = filepath.Join(state.ProjectRootOrCwd(), fsPath)
					}
					contentBytes, rerr = os.ReadFile(fsPath)
				}
				if rerr != nil {
					return fmt.Errorf("failed to read content for %s: %w", it.Path, rerr)
				}
				content := string(contentBytes)
				// perform update via API
				input := api.UpdatePostInput{Id: entry.RemotePostID, ContentMarkdown: &content}
				if _, uerr := api.UpdatePost(context.Background(), client, input); uerr != nil {
					return fmt.Errorf("update failed for %s: %w", it.Path, uerr)
				}
				// Determine checksum to store
				var checksum string
				if si, ok := st.Items[np]; ok && si.Checksum != "" {
					checksum = si.Checksum
				} else {
					checksum = state.ChecksumFromContent(contentBytes)
				}
				s.SetArticle(np, entry.RemotePostID, checksum)
				entry.MarkdownPath = np
				entry.Checksum = checksum
				entry.LastSyncedAt = time.Now().UTC().Format(time.RFC3339)
				updatedArticles = append(updatedArticles, entry)
				fmt.Printf("Updated post %s -> %s\n", it.Path, entry.RemotePostID)
			case diff.ActionCreate:
				// Prepare content
				var contentBytes []byte
				var rerr error
				if si, ok := st.Items[np]; ok && si.Snapshot != "" {
					contentBytes, rerr = state.GetSnapshotContent(si.Snapshot)
				} else {
					fsPath := filepath.FromSlash(np)
					if !filepath.IsAbs(fsPath) {
						fsPath = filepath.Join(state.ProjectRootOrCwd(), fsPath)
					}
					contentBytes, rerr = os.ReadFile(fsPath)
				}
				if rerr != nil {
					return fmt.Errorf("failed to read staged file %s: %w", it.Path, rerr)
				}
				content := string(contentBytes)
				input := api.PublishPostInput{Title: it.Title, PublicationId: s.Blog.PublicationID, ContentMarkdown: content}
				resp, perr := api.PublishPost(context.Background(), client, input)
				if perr != nil {
					return fmt.Errorf("publish failed for %s: %w", it.Path, perr)
				}
				if resp == nil || resp.PublishPost.Post.Id == "" {
					return fmt.Errorf("publish returned no id for %s", it.Path)
				}
				newID := resp.PublishPost.Post.Id
				localID := uuid.NewString()
				var checksum string
				if si, ok := st.Items[np]; ok && si.Checksum != "" {
					checksum = si.Checksum
				} else {
					checksum = state.ChecksumFromContent(contentBytes)
				}
				entry := state.ArticleEntry{LocalID: localID, Title: it.Title, MarkdownPath: np, RemotePostID: newID, Checksum: checksum, LastSyncedAt: time.Now().UTC().Format(time.RFC3339)}
				updatedArticles = append(updatedArticles, entry)
				s.SetArticle(np, newID, checksum)
				fmt.Printf("Created post %s -> %s\n", it.Path, newID)
			}
		}

		// Persist updated registries and sum
		if err := state.SaveArticles(updatedArticles); err != nil {
			return fmt.Errorf("failed to save article registry: %w", err)
		}
		if err := state.SaveSum(s); err != nil {
			return fmt.Errorf("failed to save hashnode.sum: %w", err)
		}

		// Clear stage on success
		st.Clear()
		if err := state.SaveStage(st); err != nil {
			return fmt.Errorf("failed to clear stage: %w", err)
		}

		fmt.Println("apply: completed (created/updated posts and wrote hashnode.sum)")
		return nil
	},
}

var applyYes bool

func init() {
	applyCmd.Flags().BoolVarP(&applyYes, "yes", "y", false, "Confirm and perform destructive deletions (required to remove remote posts)")
}
