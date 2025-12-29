package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "hnsync",
	Short: "hnsync - Hashnode Git Sync",
	Long:  "hnsync is a CLI to manage Hashnode blogs from a git repo.",
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(planCmd)
	rootCmd.AddCommand(applyCmd)
	rootCmd.AddCommand(importCmd)
	rootCmd.PersistentFlags().StringP("token", "t", "", "Hashnode API token (env HASHNODE_TOKEN preferred)")
}
