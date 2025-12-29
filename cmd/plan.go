package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
)

var planCmd = &cobra.Command{
	Use:   "plan",
	Short: "Show plan of changes",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("plan: not yet implemented")
		return nil
	},
}
