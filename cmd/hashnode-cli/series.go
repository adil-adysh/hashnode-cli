package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
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

		// Load existing stage and check for existing series by name
		st, err := state.LoadStage()
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ Failed to load stage: %v\n", err)
			os.Exit(1)
		}
		for _, it := range st.Items {
			if it.Type != state.TypeSeries || it.SeriesMeta == nil {
				continue
			}
			if it.SeriesMeta.Title == name {
				fmt.Printf("No-op: series with name '%s' already exists (slug=%s)\n", it.SeriesMeta.Title, it.SeriesMeta.Slug)
				return
			}
		}

		slug := state.Slugify(name)
		// create staged series entry
		key := slug
		si := state.StagedItem{
			Type:      state.TypeSeries,
			Key:       key,
			Operation: state.OpModify,
			StagedAt:  time.Now(),
			SeriesMeta: &state.SeriesMeta{
				LocalID: uuid.NewString(),
				Title:   name,
				Slug:    slug,
			},
		}
		st.Items[key] = si
		if err := state.SaveStage(st); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Failed to save stage: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Created series '%s' (slug=%s) in staged metadata\n", name, slug)
	},
}

func init() {
	seriesCreateCmd.Flags().StringP("name", "n", "", "Series name")
	seriesCreateCmd.Flags().StringP("description", "d", "", "Series description")
	seriesCreateCmd.MarkFlagRequired("name")

	seriesCmd.AddCommand(seriesCreateCmd)
	rootCmd.AddCommand(seriesCmd)
}
