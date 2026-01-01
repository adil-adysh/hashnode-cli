package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"adil-adysh/hashnode-cli/internal/state"
)

var (
	gcDryRun bool
	gcVerify bool
)

var gcCmd = &cobra.Command{
	Use:   "gc",
	Short: "Garbage collect unreferenced snapshots",
	Long: `Remove snapshot files that are no longer referenced by staged items or locks.

Snapshots are content-addressable backups created when staging files.
Over time, unreferenced snapshots can accumulate (e.g., after unstaging or modifying files).
This command cleans them up to free disk space.

Examples:
  # Show what would be removed (dry-run)
  hn gc --dry-run

  # Actually remove unreferenced snapshots
  hn gc

  # Remove with integrity verification of referenced snapshots
  hn gc --verify

  # Dry-run with verification
  hn gc --dry-run --verify`,
	RunE: func(cmd *cobra.Command, args []string) error {
		snapStore := state.NewSnapshotStore()

		var stats *state.GCStats
		var err error

		if gcVerify {
			stats, err = snapStore.GCWithVerification(gcDryRun, true)
		} else {
			stats, err = snapStore.GC(gcDryRun)
		}

		if err != nil {
			return fmt.Errorf("GC failed: %w", err)
		}

		// Display results
		fmt.Printf("Snapshot Garbage Collection %s\n", map[bool]string{true: "(DRY RUN)", false: ""}[gcDryRun])
		fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		fmt.Printf("Total snapshots:      %d\n", stats.TotalSnapshots)
		fmt.Printf("Referenced:           %d\n", stats.ReferencedCount)
		fmt.Printf("Unreferenced:         %d\n", stats.TotalSnapshots-stats.ReferencedCount)

		if gcDryRun {
			fmt.Printf("Would remove:         %d\n", stats.RemovedCount)
		} else {
			fmt.Printf("Removed:              %d\n", stats.RemovedCount)
		}

		if stats.SkippedCount > 0 {
			fmt.Printf("Skipped (errors):     %d\n", stats.SkippedCount)
		}

		// Show removed snapshots if any
		if len(stats.RemovedSnapshots) > 0 {
			fmt.Printf("\n%s snapshots:\n", map[bool]string{true: "Would remove", false: "Removed"}[gcDryRun])
			for _, filename := range stats.RemovedSnapshots {
				fmt.Printf("  • %s\n", filename)
			}
		}

		// Show errors if any
		if len(stats.Errors) > 0 {
			fmt.Printf("\n⚠️  Errors encountered:\n")
			for _, e := range stats.Errors {
				fmt.Printf("  • %v\n", e)
			}
		}

		// Summary
		if gcDryRun && stats.RemovedCount > 0 {
			fmt.Printf("\n💡 Run without --dry-run to actually remove %d snapshots\n", stats.RemovedCount)
		} else if !gcDryRun && stats.RemovedCount > 0 {
			fmt.Printf("\n✅ Freed disk space by removing %d unreferenced snapshots\n", stats.RemovedCount)
		} else if stats.RemovedCount == 0 {
			fmt.Printf("\n✨ All snapshots are referenced - nothing to clean up!\n")
		}

		return nil
	},
}

func init() {
	gcCmd.Flags().BoolVar(&gcDryRun, "dry-run", false, "Show what would be removed without deleting")
	gcCmd.Flags().BoolVar(&gcVerify, "verify", false, "Verify integrity of referenced snapshots")
	rootCmd.AddCommand(gcCmd)
}
