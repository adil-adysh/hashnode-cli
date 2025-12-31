package main

import (
	"adil-adysh/hashnode-cli/internal/cli/output"
	"adil-adysh/hashnode-cli/internal/state"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

// articleCmd is the parent command for article-related subcommands.
var articleCmd = &cobra.Command{
	Use:   "article",
	Short: "Manage articles (declarative)",
}

// articleCreateCmd creates a new local article file and registers it
// in the article registry. It is idempotent based on title.
var articleCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new article (register + generate markdown)",
	Run: func(cmd *cobra.Command, args []string) {
		name, _ := cmd.Flags().GetString("name")
		series, _ := cmd.Flags().GetString("series")
		if strings.TrimSpace(name) == "" {
			output.Error("--name is required\n")
			os.Exit(1)
		}
		list, err := state.LoadArticles()
		if err != nil {
			output.Error("Failed to load article registry: %v\n", err)
			os.Exit(1)
		}
		for _, a := range list {
			if a.Title == name {
				output.Info("No-op: article with title '%s' already registered at %s\n", a.Title, a.MarkdownPath)
				return
			}
		}
		cwd, err := os.Getwd()
		if err != nil {
			output.Error("Failed to get cwd: %v\n", err)
			os.Exit(1)
		}
		mdPath, err := state.GenerateFilename(name, cwd)
		if err != nil {
			output.Error("Failed to determine markdown filename: %v\n", err)
			os.Exit(1)
		}
		content := []byte("# " + name + "\n\nWrite your article here.\n")
		if err := os.WriteFile(mdPath, content, state.FilePerm); err != nil {
			output.Error("Failed to write markdown file: %v\n", err)
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
			output.Error("Failed to save article registry: %v\n", err)
			os.Exit(1)
		}
		output.Success("Registered article '%s' -> %s\n", name, rel)
	},
}

func init() {
	articleCreateCmd.Flags().StringP("name", "n", "", "Article Title")
	articleCreateCmd.Flags().StringP("series", "s", "", "Series ID to associate")
	articleCreateCmd.MarkFlagRequired("name")
	articleCmd.AddCommand(articleCreateCmd)
	rootCmd.AddCommand(articleCmd)
}
