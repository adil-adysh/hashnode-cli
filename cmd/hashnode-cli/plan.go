package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"adil-adysh/hashnode-cli/internal/diff"
	"adil-adysh/hashnode-cli/internal/state"
)

var planCmd = &cobra.Command{
	Use:   "plan",
	Short: "Show planned changes between local and last sync",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("📂 Scanning 'posts/' directory...")
		localPosts, err := state.ScanDirectory("posts")
		if err != nil {
			fmt.Printf("❌ Error scanning posts: %v\n", err)
			os.Exit(1)
		}

		storedState, err := state.LoadIdentities()
		if err != nil {
			fmt.Printf("❌ Error loading state: %v\n", err)
			os.Exit(1)
		}

		plan := diff.GeneratePlan(localPosts, storedState)
		diff.PrintPlanSummary(plan)
	},
}

func init() {
	rootCmd.AddCommand(planCmd)
}
