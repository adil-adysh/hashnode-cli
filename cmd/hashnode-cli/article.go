package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"adil-adysh/hashnode-cli/internal/state"

	"github.com/google/uuid"
)

var articleCmd = &cobra.Command{
	Use:   "article",
	Short: "Manage articles (declarative)",
}

var articleCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new article (register + generate markdown)",
	Run: func(cmd *cobra.Command, args []string) {
		name, _ := cmd.Flags().GetString("name")
		series, _ := cmd.Flags().GetString("series")

		if strings.TrimSpace(name) == "" {
			fmt.Fprintln(os.Stderr, "❌ --name is required")
			os.Exit(1)
		}

		// Load existing articles
		list, err := state.LoadArticles()
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ Failed to load article registry: %v\n", err)
			os.Exit(1)
		}

		// Idempotent by title
		for _, a := range list {
			if a.Title == name {
				fmt.Printf("No-op: article with title '%s' already registered at %s\n", a.Title, a.MarkdownPath)
				return
			}
		}

		// Determine file path in current directory
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ Failed to get cwd: %v\n", err)
			os.Exit(1)
		}

		mdPath, err := state.GenerateFilename(name, cwd)
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ Failed to determine markdown filename: %v\n", err)
			os.Exit(1)
		}

		// Prepare content
		content := []byte(fmt.Sprintf("# %s\n\nWrite your article here.\n", name))

		if err := os.WriteFile(mdPath, content, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Failed to write markdown file: %v\n", err)
			os.Exit(1)
		}

		rel, err := filepath.Rel(cwd, mdPath)
		if err != nil {
			rel = mdPath
		}

		entry := state.ArticleEntry{
			LocalID:      uuid.NewString(),
			Title:        name,
			MarkdownPath: rel,
			SeriesID:     series,
			RemotePostID: "",
			Checksum:     state.ChecksumFromContent(content),
			LastSyncedAt: "",
		}

		list = append(list, entry)
		if err := state.SaveArticles(list); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Failed to save article registry: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Registered article '%s' -> %s\n", name, rel)
	},
}

func init() {
	articleCreateCmd.Flags().StringP("name", "n", "", "Article Title")
	articleCreateCmd.Flags().StringP("series", "s", "", "Series ID to associate")
	articleCreateCmd.MarkFlagRequired("name")

	articleCmd.AddCommand(articleCreateCmd)
	rootCmd.AddCommand(articleCmd)
}
