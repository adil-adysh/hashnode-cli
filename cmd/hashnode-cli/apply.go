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
		// Load or construct sum (ledger) and build the registry slice from it.
		var articles []diff.RegistryEntry
		var s *state.Sum
		if ss, err := state.LoadSum(); err == nil {
			if err := ss.ValidateAgainstBlog(); err == nil {
				s = ss
			}
		}
		if s == nil {
			s, _ = state.NewSumFromBlog()
		}

		for path, a := range s.Articles {
			articles = append(articles, diff.RegistryEntry{
				MarkdownPath: path,
				RemotePostID: a.PostID,
				Checksum:     a.Checksum,
			})
		}

		plan := diff.GeneratePlan(articles, st)

		// Validate planned creations for missing/too-short titles before contacting API
		var bad []string
		for _, it := range plan {
			if it.Type != diff.ActionCreate {
				continue
			}
			// first prefer plan title
			title := it.Title
			// then staged metadata
			if title == "" {
				if si, ok := st.Items[state.NormalizePath(it.Path)]; ok && si.ArticleMeta != nil {
					title = si.ArticleMeta.Title
				}
			}
			// then try frontmatter from snapshot or disk
			if title == "" {
				var content []byte
				var fromSnapshot bool
				if si, ok := st.Items[state.NormalizePath(it.Path)]; ok && si.Snapshot != "" {
					c, err := state.GetSnapshotContent(si.Snapshot)
					if err == nil {
						content = c
						fromSnapshot = true
					}
				}
				if len(content) == 0 {
					fsPath := filepath.FromSlash(it.Path)
					if !filepath.IsAbs(fsPath) {
						fsPath = filepath.Join(state.ProjectRootOrCwd(), fsPath)
					}
					c, err := os.ReadFile(fsPath)
					if err == nil {
						content = c
					}
				}
				if len(content) > 0 {
					if t, _ := state.ParseTitleFromFrontmatter(content); t != "" {
						title = t
					}
				}
				// If the title was missing in the staged snapshot but present on disk,
				// restage the file to refresh the snapshot (safer than overriding silently).
				if title == "" && fromSnapshot {
					// Try reading disk explicitly
					fsPath := filepath.FromSlash(it.Path)
					if !filepath.IsAbs(fsPath) {
						fsPath = filepath.Join(state.ProjectRootOrCwd(), fsPath)
					}
					if c, err := os.ReadFile(fsPath); err == nil {
						if t, _ := state.ParseTitleFromFrontmatter(c); t != "" {
							// update stage by re-staging the file so snapshot/checksum are refreshed
							if serr := state.StageAdd(it.Path); serr != nil {
								return fmt.Errorf("failed to restage %s: %w", it.Path, serr)
							}
							// reload stage
							if newSt, lerr := state.LoadStage(); lerr == nil {
								st = newSt
								title = t
							}
						}
					}
				}
			}
			// If title still empty or too short, record as bad
			if len(title) < 6 {
				bad = append(bad, it.Path)
			}
		}
		if len(bad) > 0 {
			return fmt.Errorf("the following staged files are missing valid titles (>=6 chars): %v. Add a 'title:' field to their frontmatter or update staged metadata", bad)
		}

		// Build set of staged include paths for quick reference
		stagedPaths := make(map[string]struct{})
		for p := range st.Items {
			stagedPaths[state.NormalizePath(p)] = struct{}{}
		}

		// `s` (ledger) was loaded above when generating the plan.

		// Build lookup from existing staged registry metadata
		regByPath := make(map[string]diff.RegistryEntry)
		for _, a := range articles {
			regByPath[state.NormalizePath(a.MarkdownPath)] = a
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
				var entry diff.RegistryEntry
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
				// Determine title to send with update (use same resolution as create)
				title := it.Title
				if title == "" {
					if si, ok := st.Items[np]; ok && si.ArticleMeta != nil {
						title = si.ArticleMeta.Title
					}
				}
				if title == "" {
					if si, ok := st.Items[np]; ok && si.Snapshot != "" {
						if c, err := state.GetSnapshotContent(si.Snapshot); err == nil {
							if t, _ := state.ParseTitleFromFrontmatter(c); t != "" {
								title = t
							}
						}
					}
				}
				if title == "" {
					fsPath := filepath.FromSlash(np)
					if !filepath.IsAbs(fsPath) {
						fsPath = filepath.Join(state.ProjectRootOrCwd(), fsPath)
					}
					if c, err := os.ReadFile(fsPath); err == nil {
						if t, _ := state.ParseTitleFromFrontmatter(c); t != "" {
							title = t
						}
					}
				}
				if title == "" {
					return fmt.Errorf("no title found for %s", it.Path)
				}

				// perform update via API (include title)
				if s == nil || s.Blog.PublicationID == "" {
					return fmt.Errorf("update failed for %s: publication id missing in ledger; run 'hashnode init'", it.Path)
				}
				pubID := s.Blog.PublicationID
				input := api.UpdatePostInput{Id: entry.RemotePostID, ContentMarkdown: &content, Title: &title, PublicationId: &pubID}
				if _, uerr := api.UpdatePost(context.Background(), client, input); uerr != nil {
					return fmt.Errorf("update failed for %s: %w", it.Path, uerr)
				}

				// persist staged metadata title
				si := st.Items[np]
				if si.ArticleMeta == nil {
					si.ArticleMeta = &state.ArticleMeta{}
				}
				si.ArticleMeta.Title = title
				st.Items[np] = si
				// Determine checksum to store and update staged metadata
				var checksum string
				if stored, ok := st.Items[np]; ok && stored.Checksum != "" {
					checksum = stored.Checksum
				} else {
					checksum = state.ChecksumFromContent(contentBytes)
				}
				// Preserve existing slug if present in the ledger
				slug := ""
				if le, ok := s.Articles[np]; ok {
					slug = le.Slug
				}
				s.SetArticle(np, entry.RemotePostID, checksum, slug)
				// update staged metadata (reuse si variable)
				if si.ArticleMeta == nil {
					si.ArticleMeta = &state.ArticleMeta{}
				}
				si.ArticleMeta.LastSyncedAt = time.Now().UTC().Format(time.RFC3339)
				si.Checksum = checksum
				st.Items[np] = si
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
				// Determine title to publish (prefer plan -> staged metadata -> snapshot -> disk)
				title := it.Title
				if title == "" {
					if si, ok := st.Items[np]; ok && si.ArticleMeta != nil {
						title = si.ArticleMeta.Title
					}
				}
				if title == "" {
					if si, ok := st.Items[np]; ok && si.Snapshot != "" {
						if c, err := state.GetSnapshotContent(si.Snapshot); err == nil {
							if t, _ := state.ParseTitleFromFrontmatter(c); t != "" {
								title = t
							}
						}
					}
				}
				if title == "" {
					fsPath := filepath.FromSlash(np)
					if !filepath.IsAbs(fsPath) {
						fsPath = filepath.Join(state.ProjectRootOrCwd(), fsPath)
					}
					if c, err := os.ReadFile(fsPath); err == nil {
						if t, _ := state.ParseTitleFromFrontmatter(c); t != "" {
							title = t
						}
					}
				}
				if title == "" {
					return fmt.Errorf("no title found for %s", it.Path)
				}
				input := api.PublishPostInput{Title: title, PublicationId: s.Blog.PublicationID, ContentMarkdown: content}
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
				// persist into staged metadata
				si := st.Items[np]
				si.Type = state.TypeArticle
				si.Key = np
				si.Checksum = checksum
				if si.ArticleMeta == nil {
					si.ArticleMeta = &state.ArticleMeta{}
				}
				si.ArticleMeta.LocalID = localID
				si.ArticleMeta.Title = it.Title
				si.ArticleMeta.RemotePostID = newID
				si.ArticleMeta.LastSyncedAt = time.Now().UTC().Format(time.RFC3339)
				st.Items[np] = si
				// record slug returned by publish API
				pubSlug := ""
				if resp != nil && resp.PublishPost.Post != nil {
					pubSlug = resp.PublishPost.Post.Slug
				}
				s.SetArticle(np, newID, checksum, pubSlug)
				fmt.Printf("Created post %s -> %s\n", it.Path, newID)
			}
		}
		// Persist updated stage and sum
		if err := state.SaveStage(st); err != nil {
			return fmt.Errorf("failed to save stage: %w", err)
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
