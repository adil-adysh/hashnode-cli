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

		// Determine publication id and existing ledger from .hashnode/blog.yml / hashnode.sum
		var sum *state.Sum
		sum, err = state.LoadSum()
		if err != nil {
			// If sum is missing, fall back to building one from blog.yml
			if os.IsNotExist(err) {
				sum, err = state.NewSumFromBlog()
				if err != nil {
					return fmt.Errorf("failed to read blog metadata: %w", err)
				}
			} else {
				return fmt.Errorf("failed to load hashnode.sum: %w", err)
			}
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

		// Load existing article registry to allow merging and reuse of local ids/paths
		existingRegs, regErr := state.LoadArticles()
		regByPath := make(map[string]state.ArticleEntry)
		regByRemote := make(map[string]state.ArticleEntry)
		if regErr == nil {
			for _, r := range existingRegs {
				regByPath[state.NormalizePath(r.MarkdownPath)] = r
				if r.RemotePostID != "" {
					regByRemote[r.RemotePostID] = r
				}
			}
		}

		// Build postID -> path map from sum for quick lookup
		postIDToPath := make(map[string]string)
		if sum != nil {
			for p, a := range sum.Articles {
				if a.PostID != "" {
					postIDToPath[a.PostID] = p
				}
			}
		}

		// Iterate posts and write/merge files
		var newRegsMap = make(map[string]state.ArticleEntry) // keyed by normalized path
		for _, edge := range resp.Publication.Posts.Edges {
			post := edge.Node
			title := post.Title
			content := post.Content.Markdown

			checksum := state.ChecksumFromContent([]byte(content))

			// Decide where to place the file: reuse existing path if this post was imported
			var outPath string
			var localID string
			if p, ok := postIDToPath[post.Id]; ok {
				outPath = p
				if e, ok2 := regByPath[state.NormalizePath(outPath)]; ok2 {
					localID = e.LocalID
				} else if e2, ok3 := regByRemote[post.Id]; ok3 {
					localID = e2.LocalID
					outPath = e2.MarkdownPath
				} else {
					localID = uuid.NewString()
				}
			} else {
				// New import: choose filename under year/month
				published := time.Now().UTC()
				if post.PublishedAt != nil {
					published = *post.PublishedAt
				}
				year := published.Year()
				month := int(published.Month())
				outDir := fmt.Sprintf("%04d/%02d", year, month)

				outPath, err = state.GenerateFilename(title, outDir)
				if err != nil {
					return fmt.Errorf("failed to generate filename for %s: %w", title, err)
				}
				localID = uuid.NewString()
			}

			// Ensure directory exists
			if err := os.MkdirAll(filepath.Dir(filepath.FromSlash(outPath)), state.DirPerm); err != nil {
				return fmt.Errorf("failed to write file: %w", err)
			}

			// If file already exists and checksum matches registry, skip rewrite
			writeFile := true
			if e, ok := regByPath[state.NormalizePath(outPath)]; ok {
				if e.Checksum == checksum {
					writeFile = false
				}
			}
			if writeFile {
				if err := os.WriteFile(filepath.FromSlash(outPath), []byte(content), state.FilePerm); err != nil {
					return fmt.Errorf("failed to write file: %w", err)
				}
			}

			// Series mapping
			var seriesID string
			if post.Series != nil {
				seriesID = post.Series.Id
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

			normPath := state.NormalizePath(outPath)
			newRegsMap[normPath] = entry
			sum.SetArticle(normPath, post.Id, checksum)
			output.Info("Imported %s -> %s\n", outPath, post.Id)
		}

		// Merge existing registry entries that were not part of this import
		for _, r := range existingRegs {
			np := state.NormalizePath(r.MarkdownPath)
			if _, ok := newRegsMap[np]; !ok {
				newRegsMap[np] = r
			}
		}

		// Convert newRegsMap back to slice for SaveArticles
		var articles []state.ArticleEntry
		for _, v := range newRegsMap {
			articles = append(articles, v)
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
		if st.Items == nil {
			st.Items = make(map[string]state.StagedItem)
		}
		for _, a := range articles {
			np := state.NormalizePath(a.MarkdownPath)
			// add staged item representing current local state
			sType, localCS, _, serr := state.ComputeArticleState(a)
			if serr != nil {
				output.Info("ℹ️  could not compute staged state for %s: %v\n", a.MarkdownPath, serr)
				continue
			}
			op := state.OpModify
			if sType == state.ArticleStateDelete {
				op = state.OpDelete
			}
			st.Items[np] = state.StagedItem{
				Type:      state.TypeArticle,
				Key:       np,
				Checksum:  localCS,
				Snapshot:  "",
				Operation: op,
				StagedAt:  time.Now(),
			}
		}
		if err := state.SaveStage(st); err != nil {
			return fmt.Errorf("failed to save stage: %w", err)
		}

		fmt.Println("import: completed")
		return nil
	},
}
