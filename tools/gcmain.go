package tools

import (
	"fmt"
	"log"

	"adil-adysh/hashnode-cli/internal/state"
)

func GcMain() {
	snapStore := state.NewSnapshotStore()
	stats, err := snapStore.GC(false)
	if err != nil {
		log.Fatalf("GC failed: %v", err)
	}
	fmt.Printf("GC removed %d snapshots (scanned %d, %d referenced)\n",
		stats.RemovedCount, stats.TotalSnapshots, stats.ReferencedCount)
	for _, removed := range stats.RemovedSnapshots {
		fmt.Printf("  - %s\n", removed)
	}
}
