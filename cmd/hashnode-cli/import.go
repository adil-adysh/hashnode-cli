package main

import (
	"fmt"
	"github.com/spf13/cobra"
)

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Import posts from Hashnode",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("import: not yet implemented")
		return nil
	},
}
