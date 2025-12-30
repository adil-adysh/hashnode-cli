package main

import (
	"fmt"
	"github.com/spf13/cobra"
)

var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply planned changes",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("apply: not yet implemented")
		return nil
	},
}
