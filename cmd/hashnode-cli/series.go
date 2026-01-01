package main

import (
	"fmt"
	"os"
	"strings"
	"time"

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

		if strings.TrimSpace(name) == "" {
			fmt.Fprintln(os.Stderr, "❌ --name is required")
			os.Exit(1)
		}

		slug := state.Slugify(name)

		// Load ledger to check for existing series
		sum, err := state.LoadSum()
		if err != nil && !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "❌ Failed to load ledger: %v\n", err)
			os.Exit(1)
		}
		if sum == nil {
			sum, _ = state.NewSumFromBlog()
		}

		// Check if series already exists in ledger
		if _, exists := sum.Series[slug]; exists {
			fmt.Printf("No-op: series '%s' (slug=%s) already exists in ledger\n", name, slug)
			return
		}

		// Add to ledger
		sum.Series[slug] = state.SeriesEntry{
			SeriesID:    "", // Will be set on apply
			Name:        name,
			Slug:        slug,
			Description: "",
		}

		if err := state.SaveSum(sum); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Failed to save ledger: %v\n", err)
			os.Exit(1)
		}

		// Also stage it so apply knows to create
		st, err := state.LoadStage()
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ Failed to load stage: %v\n", err)
			os.Exit(1)
		}

		st.Items[slug] = state.StagedItem{
			Type:      state.TypeSeries,
			Key:       slug,
			Operation: state.OpModify,
			StagedAt:  time.Now(),
		}

		if err := state.SaveStage(st); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Failed to save stage: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Created series '%s' (slug=%s) - run 'hn apply' to publish\n", name, slug)
	},
}

func init() {
	seriesCreateCmd.Flags().StringP("name", "n", "", "Series name")
	seriesCreateCmd.Flags().StringP("description", "d", "", "Series description")
	seriesCreateCmd.MarkFlagRequired("name")

	seriesCmd.AddCommand(seriesCreateCmd)
	rootCmd.AddCommand(seriesCmd)
}
