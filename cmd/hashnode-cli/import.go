package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/Khan/genqlient/graphql"
	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"adil-adysh/hashnode-cli/internal/api"
	"adil-adysh/hashnode-cli/internal/cli/output"
	"adil-adysh/hashnode-cli/internal/config"
	"adil-adysh/hashnode-cli/internal/state"
)

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Import posts from Hashnode",
	RunE: func(cmd *cobra.Command, args []string) error {
		release, err := state.AcquireRepoLock()
		if err != nil {
			return fmt.Errorf("failed to acquire repo lock: %w", err)
		}
		defer func() {
			if err := release(); err != nil {
				fmt.Printf("warning: failed to remove lock: %v\n", err)
			}
		}()

		// Load home config for token
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load home config (run init): %w", err)
		}
		if cfg.Token == "" {
			return fmt.Errorf("no token configured; run 'hashnode init'")
		}

		httpClient := &http.Client{Transport: &authedTransport{token: cfg.Token, wrapped: http.DefaultTransport}}
		client := graphql.NewClient("https://gql.hashnode.com", httpClient)

		// Determine publication id from .hashnode/blog.yml
		sum, err := state.NewSumFromBlog()
		if err != nil {
			return fmt.Errorf("failed to read blog metadata: %w", err)
		}

		// Fetch publication data
		resp, err := api.GetPublicationData(context.Background(), client, sum.Blog.PublicationID)
		if err != nil {
			return fmt.Errorf("failed to fetch publication data: %w", err)
		}
		if resp == nil || resp.Publication == nil {
			return fmt.Errorf("no publication data returned")
		}

		// Prepare series map in sum
		if sum.Series == nil {
			sum.Series = make(map[string]state.SeriesEntry)
		}
		for _, edge := range resp.Publication.SeriesList.Edges {
			n := edge.Node
			slug := n.Slug
			sum.Series[slug] = state.SeriesEntry{SeriesID: n.Id, Name: n.Name, Slug: slug}
		}

		// Iterate posts and write files
		var articles []state.ArticleEntry
		for _, edge := range resp.Publication.Posts.Edges {
			post := edge.Node
			title := post.Title
			content := post.Content.Markdown

			// Determine target directory by published date if available, otherwise now
			published := time.Now().UTC()
			if post.PublishedAt != nil {
				published = *post.PublishedAt
			}
			year := published.Year()
			month := int(published.Month())
			outDir := fmt.Sprintf("%04d/%02d", year, month)

			// Choose filename under year/month
			outPath, err := state.GenerateFilename(title, outDir)
			if err != nil {
				return fmt.Errorf("failed to generate filename for %s: %w", title, err)
			}
			// Ensure directory exists
			if err := os.MkdirAll(filepath.Dir(filepath.FromSlash(outPath)), state.DirPerm); err != nil {
				return fmt.Errorf("failed to write file: %w", err)
			}
			if err := os.WriteFile(filepath.FromSlash(outPath), []byte(content), state.FilePerm); err != nil {
				return fmt.Errorf("failed to write file: %w", err)
			}

			checksum := state.ChecksumFromContent([]byte(content))

			// Local ID
			localID := uuid.NewString()

			// Series mapping
			var seriesID string
			if post.Series != nil {
				seriesID = post.Series.Id
				// ensure series present in sum map
				if _, ok := sum.Series[post.Series.Slug]; !ok {
					sum.Series[post.Series.Slug] = state.SeriesEntry{SeriesID: post.Series.Id, Name: post.Series.Name, Slug: post.Series.Slug}
				}
			}

			entry := state.ArticleEntry{
				LocalID:      localID,
				Title:        title,
				MarkdownPath: outPath,
				SeriesID:     seriesID,
				RemotePostID: post.Id,
				Checksum:     checksum,
				LastSyncedAt: time.Now().UTC().Format(time.RFC3339),
			}
			articles = append(articles, entry)
			sum.SetArticle(outPath, post.Id, checksum)
			output.Info("Imported %s -> %s\n", outPath, post.Id)
		}

		// Save article registry and sum
		if err := state.SaveArticles(articles); err != nil {
			return fmt.Errorf("failed to save article registry: %w", err)
		}
		if err := state.SaveSum(sum); err != nil {
			return fmt.Errorf("failed to save hashnode.sum: %w", err)
		}

		// Persist staged entries for imported files so they appear in `stage list`.
		st, err := state.LoadStage()
		if err != nil {
			return fmt.Errorf("failed to load stage: %w", err)
		}
		if st.Staged == nil {
			st.Staged = map[string]state.StagedArticle{}
		}
		for _, a := range articles {
			np := state.NormalizePath(a.MarkdownPath)
			// add to include if not present
			found := false
			for _, p := range st.Include {
				if p == np {
					found = true
					break
				}
			}
			if !found {
				st.Include = append(st.Include, np)
			}
			sType, localCS, remoteCS, serr := state.ComputeArticleState(a)
			if serr != nil {
				output.Info("ℹ️  could not compute staged state for %s: %v\n", a.MarkdownPath, serr)
				continue
			}
			if err := state.SetStagedEntry(a.MarkdownPath, a.RemotePostID, sType, localCS, remoteCS); err != nil {
				output.Info("ℹ️  could not persist staged entry for %s: %v\n", a.MarkdownPath, err)
			}
		}
		if err := state.SaveStage(st); err != nil {
			return fmt.Errorf("failed to save stage: %w", err)
		}

		// Migrate staged paths: if a staged entry referenced a remote post id that
		// was just imported, move the staged entry to the new path to preserve intent.
		if err := state.MigrateStagedPathsByRemote(articles); err != nil {
			return fmt.Errorf("failed to migrate staged paths: %w", err)
		}

		fmt.Println("import: completed")
		return nil
	},
}
