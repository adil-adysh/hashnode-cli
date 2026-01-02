package main

import (
	"context"
	"bytes"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"
    
	"gopkg.in/yaml.v3"

	"github.com/Khan/genqlient/graphql"
	"github.com/spf13/cobra"

	"adil-adysh/hashnode-cli/internal/api"
	"adil-adysh/hashnode-cli/internal/cli/output"
	"adil-adysh/hashnode-cli/internal/config"
	"adil-adysh/hashnode-cli/internal/state"
)

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Import posts from Hashnode and sync Ledger",
	RunE: func(cmd *cobra.Command, args []string) error {
		// 1. Locking (Global Mutex)
		release, err := state.AcquireRepoLock()
		if err != nil {
			return fmt.Errorf("failed to acquire repo lock: %w", err)
		}
		defer func() {
			if err := release(); err != nil {
				fmt.Printf("warning: failed to remove lock: %v\n", err)
			}
		}()

		// 2. Setup Client
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load home config (run init): %w", err)
		}
		if cfg.Token == "" {
			return fmt.Errorf("no token configured; run 'hashnode init'")
		}

		httpClient := &http.Client{Transport: &authedTransport{token: cfg.Token, wrapped: http.DefaultTransport}}
		client := graphql.NewClient("https://gql.hashnode.com", httpClient)

		// 3. Load Ledger (The Source of Truth)
		sum, err := state.LoadSum()
		if err != nil {
			if os.IsNotExist(err) {
				sum, err = state.NewSumFromBlog()
				if err != nil {
					return fmt.Errorf("failed to read blog metadata: %w", err)
				}
			} else {
				return fmt.Errorf("failed to load hashnode.sum: %w", err)
			}
		}

		// 4. API Call (paginated: API max page size is 50)
		output.Info("Fetching publication data (paginated)...")
		var allPosts []api.GetPublicationDataPublicationPostsPublicationPostConnectionEdgesPostEdge
		var seriesEdges []api.GetPublicationDataPublicationSeriesListSeriesConnectionEdgesSeriesEdge
		var after *string
		for {
			resp, rerr := api.GetPublicationData(context.Background(), client, sum.Blog.PublicationID, 50, after)
			if rerr != nil {
				return fmt.Errorf("failed to fetch publication data: %w", rerr)
			}
			if resp == nil || resp.Publication == nil {
				return fmt.Errorf("no publication data returned")
			}
			if seriesEdges == nil {
				seriesEdges = resp.Publication.SeriesList.Edges
			}
			pageCount := len(resp.Publication.Posts.Edges)
			output.Info("Fetched page: %d posts (hasNext=%v)", pageCount, resp.Publication.Posts.PageInfo.HasNextPage != nil && *resp.Publication.Posts.PageInfo.HasNextPage)
			allPosts = append(allPosts, resp.Publication.Posts.Edges...)
			if resp.Publication.Posts.PageInfo.HasNextPage == nil || !*resp.Publication.Posts.PageInfo.HasNextPage {
				break
			}
			after = resp.Publication.Posts.PageInfo.EndCursor
		}

		// Diagnostic: show total and date range
		if len(allPosts) > 0 {
			first := allPosts[0].Node.PublishedAt
			last := allPosts[len(allPosts)-1].Node.PublishedAt
			output.Info("Total posts fetched: %d; first=%s last=%s", len(allPosts), first.UTC().Format(time.RFC3339), last.UTC().Format(time.RFC3339))
		} else {
			output.Info("Total posts fetched: 0")
		}

		// 5. Update Ledger Series
		// We trust the API as the source of truth for Series structure
		if sum.Series == nil {
			sum.Series = make(map[string]state.SeriesEntry)
		}
		for _, edge := range seriesEdges {
			n := edge.Node
			sum.Series[n.Slug] = state.SeriesEntry{
				SeriesID: n.Id,
				Name:     n.Name,
				Slug:     n.Slug,
			}
		}

		// 6. Build quick lookups for existing mappings
		// We need to know if we already have this post mapped to a file
		remoteIDToPath := make(map[string]string)
		for path, entry := range sum.Articles {
			if entry.PostID != "" {
				remoteIDToPath[entry.PostID] = path
			}
		}

		// 7. Process Posts (The Core Loop)
		for _, edge := range allPosts {
			post := edge.Node
			content := post.Content.Markdown
			checksum := state.ChecksumFromContent([]byte(content))

			// Determine Local Path
			// A. Check Ledger: Do we already know this post?
			var outPath string
			if existingPath, ok := remoteIDToPath[post.Id]; ok {
				outPath = existingPath
			} else {
				// B. New Post: Generate standardized filename
				published := post.PublishedAt
				if published.IsZero() {
					published = time.Now().UTC()
				}
				year, month := published.Year(), int(published.Month())
				outDir := fmt.Sprintf("%04d/%02d", year, month)

				// GenerateFilename ensures no collisions
				generated, err := state.GenerateFilename(post.Title, outDir)
				if err != nil {
					return fmt.Errorf("filename generation failed: %w", err)
				}
				outPath = generated
			}

			// Write to Disk
			// Ensure dir exists
			if err := os.MkdirAll(filepath.Dir(filepath.FromSlash(outPath)), 0755); err != nil {
				return fmt.Errorf("failed to ensure dir: %w", err)
			}

			// Only rewrite if content changed (Optimization)
			// Check current file on disk if it exists
			shouldWrite := true
			if currentBytes, err := os.ReadFile(filepath.FromSlash(outPath)); err == nil {
				if state.ChecksumFromContent(currentBytes) == checksum {
					shouldWrite = false
				}
			}

			if shouldWrite {
				// Build frontmatter from remote post metadata
				fm := state.Frontmatter{
					Title: post.Title,
					Slug:  post.Slug,
				}
				// Tags
				if len(post.Tags) > 0 {
					var tags []string
					for _, t := range post.Tags {
						tags = append(tags, t.Name)
					}
					fm.Tags = tags
				}
				// PublishedAt
				if !post.PublishedAt.IsZero() {
					t := post.PublishedAt
					fm.PublishedAt = &t
				}
				// Cover image
				if post.CoverImage != nil && post.CoverImage.Url != "" {
					fm.CoverImageURL = post.CoverImage.Url
				}
				// Series
				if post.Series != nil {
					fm.Series = post.Series.Name
				}

				// Marshal YAML frontmatter
				yf, err := yaml.Marshal(&fm)
				if err != nil {
					return fmt.Errorf("failed to marshal frontmatter for %s: %w", outPath, err)
				}

				var buf bytes.Buffer
				buf.WriteString("---\n")
				buf.Write(yf)
				buf.WriteString("---\n\n")
				buf.WriteString(content)

				if err := os.WriteFile(filepath.FromSlash(outPath), buf.Bytes(), 0644); err != nil {
					return fmt.Errorf("failed to write file %s: %w", outPath, err)
				}
			}

			// Update LEDGER (hashnode.sum)
			// This is the critical step: Linking Path <-> RemoteID and slug
			normPath := state.NormalizePath(outPath)
			sum.SetArticle(normPath, post.Id, checksum, post.Slug)

			// If post has a series, ensure local mapping knows it (optional, but good for completeness)
			if post.Series != nil {
				// You might want to track which article belongs to which series in sum.yml logic
				// but that's handled by your specific sum schema details.
			}

			output.Info("Synced: %s", outPath)
		}

		// 8. Stage Reconciliation (The "Dumb Stage" Fix)
		// If an imported file perfectly matches what was staged, we remove it from the stage.
		// Only create an empty stage if the stage file truly does not exist. For other
		// LoadStage errors (parse/IO), abort to avoid clobbering potentially valid data.
		st, err := state.LoadStage()
		if err != nil {
			if os.IsNotExist(err) {
				st = &state.Stage{Version: 2, Items: make(map[string]state.StagedItem)}
			} else {
				return fmt.Errorf("failed to load stage: %w", err)
			}
		}

		for path, item := range st.Items {
			if ledgerEntry, ok := sum.Articles[path]; ok {
				if ledgerEntry.Checksum == item.Checksum {
					delete(st.Items, path)
					output.Info("Unstaged %s (matched imported content)", path)
				}
			}
		}

		// 9. Persist Ledger (root/hashnode.sum)
		if err := state.SaveSum(sum); err != nil {
			return fmt.Errorf("failed to save ledger: %w", err)
		}

		// Always persist the stage to ensure .hashnode/hashnode.stage exists.
		if err := state.SaveStage(st); err != nil {
			return fmt.Errorf("failed to init stage: %w", err)
		}

		output.Success("Import completed successfully")
		return nil
	},
}
