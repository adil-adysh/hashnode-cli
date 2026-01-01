package tools

import (
	"fmt"
	"log"

	"adil-adysh/hashnode-cli/internal/state"
)

func GcMain() {
	removed, err := state.GCStaleSnapshots()
	if err != nil {
		log.Fatalf("GC failed: %v", err)
	}
	fmt.Printf("GC removed %d snapshots\n", removed)
}
