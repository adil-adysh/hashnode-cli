package main

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "hn",
	Short: "hn - Hashnode Git Sync",
	Long:  "hn is a CLI to manage Hashnode blogs from a git repo.",
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(applyCmd)
	rootCmd.AddCommand(importCmd)
	rootCmd.PersistentFlags().StringP("token", "t", "", "Hashnode API token (env HASHNODE_TOKEN preferred)")
}
