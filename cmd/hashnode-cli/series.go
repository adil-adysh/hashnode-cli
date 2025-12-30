package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"adil-adysh/hashnode-cli/internal/state"
)

var seriesCmd = &cobra.Command{
	Use:   "series",
	Short: "Manage series (declarative)",
}

var seriesCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new series (declarative, idempotent by name)",
	Run: func(cmd *cobra.Command, args []string) {
		name, _ := cmd.Flags().GetString("name")
		desc, _ := cmd.Flags().GetString("description")

		if strings.TrimSpace(name) == "" {
			fmt.Fprintln(os.Stderr, "❌ --name is required")
			os.Exit(1)
		}

		// Load existing registry
		list, err := state.LoadSeries()
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ Failed to load series registry: %v\n", err)
			os.Exit(1)
		}

		// Check idempotency by exact name
		for _, e := range list {
			if e.Name == name {
				fmt.Printf("No-op: series with name '%s' already exists (slug=%s)\n", e.Name, e.Slug)
				return
			}
		}

		slug := state.Slugify(name)

		entry := state.SeriesEntry{
			SeriesID:    "",
			Name:        name,
			Slug:        slug,
			Description: desc,
		}

		list = append(list, entry)
		if err := state.SaveSeries(list); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Failed to save series registry: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Created series '%s' (slug=%s) in .hashnode/series.yml\n", name, slug)
	},
}

func init() {
	seriesCreateCmd.Flags().StringP("name", "n", "", "Series name")
	seriesCreateCmd.Flags().StringP("description", "d", "", "Series description")
	seriesCreateCmd.MarkFlagRequired("name")

	seriesCmd.AddCommand(seriesCreateCmd)
	rootCmd.AddCommand(seriesCmd)
}
