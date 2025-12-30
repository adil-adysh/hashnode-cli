package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

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

		// Compute full diff from applied state -> working tree
		plan := diff.GeneratePlan(articles)

		// Load stage and determine which paths are staged
		st, err := state.LoadStage()
		if err != nil {
			return fmt.Errorf("failed to load stage: %w", err)
		}
		// If stage include is empty, warn and exit without changes
		if len(st.Include) == 0 {
			fmt.Println("No staged changes found in hashnode.stage; nothing to apply.")
			return nil
		}

		// Build set of paths that have actionable plan (CREATE/UPDATE/DELETE)
		plannedPaths := make(map[string]struct{})
		for _, it := range plan {
			if it.Type != diff.ActionSkip {
				plannedPaths[state.NormalizePath(it.Path)] = struct{}{}
			}
		}

		// Intersect stage include with plannedPaths to produce final stagedPaths
		stagedPaths := make(map[string]struct{})
		for _, p := range st.Include {
			np := state.NormalizePath(p)
			if _, ok := plannedPaths[np]; ok {
				stagedPaths[np] = struct{}{}
			}
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

		// Iterate articles and create/update/delete via API for staged files only. Build new registry slice.
		var updatedArticles []state.ArticleEntry
		for _, a := range articles {
			// Determine if this path is staged
			if _, ok := stagedPaths[state.NormalizePath(a.MarkdownPath)]; !ok {
				// Not staged: leave unchanged in registry
				updatedArticles = append(updatedArticles, a)
				continue
			}
			// Try reading markdown content
			contentBytes, err := os.ReadFile(a.MarkdownPath)
			if err != nil {
				if os.IsNotExist(err) {
					// File missing locally
					if a.RemotePostID != "" {
						// Require explicit confirmation for destructive deletes
						if !applyYes {
							return fmt.Errorf("deletion required for %s (remote id=%s). Re-run with --yes to confirm deletions", a.MarkdownPath, a.RemotePostID)
						}
						// Delete remote post
						resp, derr := api.DeletePost(context.Background(), client, a.RemotePostID)
						if derr != nil {
							return fmt.Errorf("delete failed for %s (remote id=%s): %w", a.MarkdownPath, a.RemotePostID, derr)
						}
						// If response exists, check success if present
						if resp != nil {
							// ignore resp.DeletePost.Success absent semantics; treat nil err as success
						}
						// Remove from sum
						s.RemoveArticle(a.MarkdownPath)
						fmt.Printf("Deleted remote post for %s -> %s\n", a.MarkdownPath, a.RemotePostID)
						// Skip adding to updatedArticles to remove it from registry
						continue
					}
					return fmt.Errorf("markdown file missing: %s", a.MarkdownPath)
				}
				return fmt.Errorf("failed to read %s: %w", a.MarkdownPath, err)
			}
			content := string(contentBytes)

			if a.RemotePostID == "" {
				// Create
				input := api.PublishPostInput{
					Title:           a.Title,
					PublicationId:   s.Blog.PublicationID,
					ContentMarkdown: content,
				}
				resp, err := api.PublishPost(context.Background(), client, input)
				if err != nil {
					return fmt.Errorf("publish failed for %s: %w", a.MarkdownPath, err)
				}
				if resp == nil || resp.PublishPost.Post.Id == "" {
					return fmt.Errorf("publish returned no id for %s", a.MarkdownPath)
				}
				newID := resp.PublishPost.Post.Id
				a.RemotePostID = newID
				a.LastSyncedAt = time.Now().UTC().Format(time.RFC3339)
				s.SetArticle(a.MarkdownPath, newID, a.Checksum)
				fmt.Printf("Created post %s -> %s\n", a.MarkdownPath, newID)
				updatedArticles = append(updatedArticles, a)
			} else {
				// Update
				contentPtr := content
				input := api.UpdatePostInput{
					Id:              a.RemotePostID,
					ContentMarkdown: &contentPtr,
				}
				resp, err := api.UpdatePost(context.Background(), client, input)
				if err != nil {
					return fmt.Errorf("update failed for %s: %w", a.MarkdownPath, err)
				}
				if resp == nil || resp.UpdatePost.Post.Id == "" {
					return fmt.Errorf("update returned no id for %s", a.MarkdownPath)
				}
				a.LastSyncedAt = time.Now().UTC().Format(time.RFC3339)
				s.SetArticle(a.MarkdownPath, a.RemotePostID, a.Checksum)
				fmt.Printf("Updated post %s -> %s\n", a.MarkdownPath, a.RemotePostID)
				updatedArticles = append(updatedArticles, a)
			}
		}
		// Replace articles with updated registry (deleted entries removed)
		articles = updatedArticles

		// Persist updated registries and sum
		if err := state.SaveArticles(articles); err != nil {
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
