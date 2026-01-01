package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

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
		if applyDryRun {
			fmt.Println("apply: dry-run (no API calls, no writes)")
		}

		// Acquire repo lock
		release, err := state.AcquireRepoLock()
		if err != nil {
			return fmt.Errorf("failed to acquire repo lock: %w", err)
		}

		// Track if apply completes successfully
		applySuccess := false
		defer func() {
			if err := release(); err != nil {
				fmt.Printf("warning: failed to remove lock: %v\n", err)
			} else if applySuccess {
				// Only log on successful apply to avoid confusion
				fmt.Println("✓ Released lock")
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

		if applyDryRun {
			createCount, updateCount, deleteCount, skipCount := 0, 0, 0, 0
			for _, it := range plan {
				switch it.Type {
				case diff.ActionCreate:
					createCount++
				case diff.ActionUpdate:
					updateCount++
				case diff.ActionDelete:
					deleteCount++
				case diff.ActionSkip:
					skipCount++
				}
				reason := it.Reason
				target := it.Path
				if it.OldPath != "" {
					target = fmt.Sprintf("%s (from %s)", it.Path, it.OldPath)
				}
				symbol := ""
				switch it.Type {
				case diff.ActionCreate:
					symbol = "🟢"
				case diff.ActionUpdate:
					symbol = "🟡"
				case diff.ActionDelete:
					symbol = "🔴"
				case diff.ActionSkip:
					symbol = "⚪"
				}
				if reason != "" {
					fmt.Printf("%s %-6s %s — %s\n", symbol, it.Type, target, reason)
				} else {
					fmt.Printf("%s %-6s %s\n", symbol, it.Type, target)
				}
			}
			fmt.Printf("Summary: %d create, %d update, %d delete, %d skip\n", createCount, updateCount, deleteCount, skipCount)
			return nil
		}

		// Validate planned creations for missing/too-short titles before contacting API
		var bad []string
		for _, it := range plan {
			if it.Type != diff.ActionCreate {
				continue
			}

			// Resolve title using centralized function
			title, _ := state.ResolveTitleForPath(it.Path, s, st)

			// If title still empty or too short, record as bad
			if len(title) < 6 {
				bad = append(bad, it.Path)
			}
		}
		if len(bad) > 0 {
			return fmt.Errorf("the following staged files are missing valid titles (>=6 chars): %v. Add a 'title:' field to their frontmatter", bad)
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

		// Collect ledger updates to apply atomically at end
		type LedgerUpdate struct {
			path     string
			postID   string
			checksum string
			slug     string
			title    string
			isDelete bool
		}
		var ledgerUpdates []LedgerUpdate

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
				if _, derr := api.RemovePost(context.Background(), client, api.RemovePostInput{Id: remoteID}); derr != nil {
					return fmt.Errorf("delete failed for %s (remote id=%s): %w", it.Path, remoteID, derr)
				}
				// Queue ledger update
				ledgerUpdates = append(ledgerUpdates, LedgerUpdate{path: np, isDelete: true})
				fmt.Printf("Deleted post %s -> %s\n", it.Path, remoteID)
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
					snapStore := state.NewSnapshotStore()
					contentBytes, rerr = snapStore.Get(si.Snapshot)
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

				fm, bodyBytes, berr := state.ExtractFrontmatter(contentBytes)
				if berr != nil {
					return fmt.Errorf("failed to parse frontmatter for %s: %w", it.Path, berr)
				}
				content := string(bodyBytes)

				// Resolve title using centralized function
				title, _ := state.ResolveTitleForPath(it.Path, s, st)
				if title == "" {
					return fmt.Errorf("no title found for %s", it.Path)
				}

				// perform update via API (include title)
				if s == nil || s.Blog.PublicationID == "" {
					return fmt.Errorf("update failed for %s: publication id missing in ledger; run 'hashnode init'", it.Path)
				}
				pubID := s.Blog.PublicationID
				input := api.UpdatePostInput{Id: entry.RemotePostID, ContentMarkdown: &content, Title: &title, PublicationId: &pubID}
				applyFrontmatterToUpdateInput(&input, fm)
				if _, uerr := api.UpdatePost(context.Background(), client, input); uerr != nil {
					return fmt.Errorf("update failed for %s: %w", it.Path, uerr)
				}

				// Determine checksum to store
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
				// Queue ledger update
				ledgerUpdates = append(ledgerUpdates, LedgerUpdate{
					path:     np,
					postID:   entry.RemotePostID,
					checksum: checksum,
					slug:     slug,
					title:    title,
				})
				fmt.Printf("Updated post %s -> %s\n", it.Path, entry.RemotePostID)
			case diff.ActionCreate:
				// Prepare content
				var contentBytes []byte
				var rerr error
				if si, ok := st.Items[np]; ok && si.Snapshot != "" {
					snapStore := state.NewSnapshotStore()
					contentBytes, rerr = snapStore.Get(si.Snapshot)
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

				fm, bodyBytes, berr := state.ExtractFrontmatter(contentBytes)
				if berr != nil {
					return fmt.Errorf("failed to parse frontmatter for %s: %w", it.Path, berr)
				}
				content := string(bodyBytes)

				// Resolve title using centralized function
				title, _ := state.ResolveTitleForPath(it.Path, s, st)
				if title == "" {
					return fmt.Errorf("no title found for %s", it.Path)
				}

				input := api.PublishPostInput{Title: title, PublicationId: s.Blog.PublicationID, ContentMarkdown: content}
				applyFrontmatterToPublishInput(&input, fm)
				resp, perr := api.PublishPost(context.Background(), client, input)
				if perr != nil {
					return fmt.Errorf("publish failed for %s: %w", it.Path, perr)
				}
				if resp == nil || resp.PublishPost.Post.Id == "" {
					return fmt.Errorf("publish returned no id for %s", it.Path)
				}
				newID := resp.PublishPost.Post.Id

				var checksum string
				if si, ok := st.Items[np]; ok && si.Checksum != "" {
					checksum = si.Checksum
				} else {
					checksum = state.ChecksumFromContent(contentBytes)
				}

				// Record slug returned by publish API
				pubSlug := ""
				if resp != nil && resp.PublishPost.Post != nil {
					pubSlug = resp.PublishPost.Post.Slug
				}
				// Queue ledger update
				ledgerUpdates = append(ledgerUpdates, LedgerUpdate{
					path:     np,
					postID:   newID,
					checksum: checksum,
					slug:     pubSlug,
					title:    title,
				})
				fmt.Printf("Created post %s -> %s\n", it.Path, newID)
			}
		}

		// Apply all ledger updates atomically
		for _, update := range ledgerUpdates {
			if update.isDelete {
				s.RemoveArticle(update.path)
			} else {
				s.SetArticleWithTitle(update.path, update.postID, update.checksum, update.slug, update.title)
			}
		}

		// Persist updated sum (ledger) - single write
		if err := state.SaveSum(s); err != nil {
			return fmt.Errorf("failed to save hashnode.sum: %w", err)
		}

		// Clear stage on success
		st.Clear()
		if err := state.SaveStage(st); err != nil {
			return fmt.Errorf("failed to clear stage: %w", err)
		}

		// GC unreferenced snapshots now that stage is empty
		snapStore := state.NewSnapshotStore()
		if stats, gerr := snapStore.GC(false); gerr == nil && stats.RemovedCount > 0 {
			fmt.Printf("🧹 Removed %d old snapshot(s)\n", stats.RemovedCount)
		}

		// Mark as successful so lock release is logged
		applySuccess = true
		fmt.Println("apply: completed (created/updated posts and wrote hashnode.sum)")
		return nil
	},
}

var applyYes bool
var applyDryRun bool

func init() {
	applyCmd.Flags().BoolVarP(&applyYes, "yes", "y", false, "Confirm and perform destructive deletions (required to remove remote posts)")
	applyCmd.Flags().BoolVar(&applyDryRun, "dry-run", false, "Preview apply without calling the API or writing state")
}

func applyFrontmatterToPublishInput(input *api.PublishPostInput, fm *state.Frontmatter) {
	if fm == nil {
		return
	}

	if fm.Subtitle != "" {
		input.Subtitle = strPtr(fm.Subtitle)
	}
	if fm.Slug != "" {
		input.Slug = strPtr(fm.Slug)
	}
	if fm.Canonical != "" {
		input.OriginalArticleURL = strPtr(fm.Canonical)
	}
	if fm.PublishedAt != nil {
		input.PublishedAt = fm.PublishedAt
	}
	if fm.DisableComments != nil {
		input.DisableComments = fm.DisableComments
	}

	if fm.CoverImageURL != "" || fm.CoverImageAttribution != "" || fm.CoverImagePhotographer != "" || fm.CoverImageHideAttribution || fm.CoverImageStickBottom {
		input.CoverImageOptions = &api.CoverImageOptionsInput{
			CoverImageURL:            strPtrOrNil(fm.CoverImageURL),
			CoverImageAttribution:    strPtrOrNil(fm.CoverImageAttribution),
			CoverImagePhotographer:   strPtrOrNil(fm.CoverImagePhotographer),
			IsCoverAttributionHidden: boolPtrOrNil(fm.CoverImageHideAttribution),
			StickCoverToBottom:       boolPtrOrNil(fm.CoverImageStickBottom),
		}
	}
	if fm.BannerImageURL != "" {
		input.BannerImageOptions = &api.BannerImageOptionsInput{BannerImageURL: strPtr(fm.BannerImageURL)}
	}

	if fm.MetaTitle != "" || fm.MetaDescription != "" || fm.MetaImage != "" {
		input.MetaTags = &api.MetaTagsInput{
			Title:       strPtrOrNil(fm.MetaTitle),
			Description: strPtrOrNil(fm.MetaDescription),
			Image:       strPtrOrNil(fm.MetaImage),
		}
	}

	if len(fm.Tags) > 0 {
		for _, t := range fm.Tags {
			tag := t
			input.Tags = append(input.Tags, api.PublishPostTagInput{Name: &tag})
		}
	}

	if fm.PublishAs != "" {
		input.PublishAs = strPtr(fm.PublishAs)
	}
	if len(fm.CoAuthors) > 0 {
		input.CoAuthors = append(input.CoAuthors, fm.CoAuthors...)
	}

	if fm.EnableToc != nil || fm.Newsletter != nil || fm.Delisted != nil || fm.Scheduled != nil || fm.SlugOverridden != nil || fm.Slug != "" {
		settings := api.PublishPostSettingsInput{}
		if fm.EnableToc != nil {
			settings.EnableTableOfContent = fm.EnableToc
		}
		if fm.Newsletter != nil {
			settings.IsNewsletterActivated = fm.Newsletter
		}
		if fm.Delisted != nil {
			settings.Delisted = fm.Delisted
		}
		if fm.Scheduled != nil {
			settings.Scheduled = fm.Scheduled
		}
		if fm.SlugOverridden != nil {
			settings.SlugOverridden = fm.SlugOverridden
		} else if fm.Slug != "" {
			settings.SlugOverridden = boolPtr(true)
		}
		input.Settings = &settings
	}
}

func applyFrontmatterToUpdateInput(input *api.UpdatePostInput, fm *state.Frontmatter) {
	if fm == nil {
		return
	}

	if fm.Subtitle != "" {
		input.Subtitle = strPtr(fm.Subtitle)
	}
	if fm.Slug != "" {
		input.Slug = strPtr(fm.Slug)
	}
	if fm.Canonical != "" {
		input.OriginalArticleURL = strPtr(fm.Canonical)
	}
	if fm.PublishedAt != nil {
		input.PublishedAt = fm.PublishedAt
	}

	if fm.CoverImageURL != "" || fm.CoverImageAttribution != "" || fm.CoverImagePhotographer != "" || fm.CoverImageHideAttribution || fm.CoverImageStickBottom {
		input.CoverImageOptions = &api.CoverImageOptionsInput{
			CoverImageURL:            strPtrOrNil(fm.CoverImageURL),
			CoverImageAttribution:    strPtrOrNil(fm.CoverImageAttribution),
			CoverImagePhotographer:   strPtrOrNil(fm.CoverImagePhotographer),
			IsCoverAttributionHidden: boolPtrOrNil(fm.CoverImageHideAttribution),
			StickCoverToBottom:       boolPtrOrNil(fm.CoverImageStickBottom),
		}
	}
	if fm.BannerImageURL != "" {
		input.BannerImageOptions = &api.BannerImageOptionsInput{BannerImageURL: strPtr(fm.BannerImageURL)}
	}

	if fm.MetaTitle != "" || fm.MetaDescription != "" || fm.MetaImage != "" {
		input.MetaTags = &api.MetaTagsInput{
			Title:       strPtrOrNil(fm.MetaTitle),
			Description: strPtrOrNil(fm.MetaDescription),
			Image:       strPtrOrNil(fm.MetaImage),
		}
	}

	if len(fm.Tags) > 0 {
		for _, t := range fm.Tags {
			tag := t
			input.Tags = append(input.Tags, api.PublishPostTagInput{Name: &tag})
		}
	}

	if fm.PublishAs != "" {
		input.PublishAs = strPtr(fm.PublishAs)
	}
	if len(fm.CoAuthors) > 0 {
		input.CoAuthors = append(input.CoAuthors, fm.CoAuthors...)
	}

	if fm.EnableToc != nil || fm.Delisted != nil || fm.DisableComments != nil || fm.PinToBlog != nil {
		settings := api.UpdatePostSettingsInput{}
		if fm.EnableToc != nil {
			settings.IsTableOfContentEnabled = fm.EnableToc
		}
		if fm.Delisted != nil {
			settings.Delisted = fm.Delisted
		}
		if fm.DisableComments != nil {
			settings.DisableComments = fm.DisableComments
		}
		if fm.PinToBlog != nil {
			settings.PinToBlog = fm.PinToBlog
		}
		input.Settings = &settings
	}
}

func strPtr(v string) *string { return &v }
func strPtrOrNil(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

func boolPtr(v bool) *bool { return &v }
func boolPtrOrNil(v bool) *bool {
	if !v {
		return nil
	}
	return &v
}
