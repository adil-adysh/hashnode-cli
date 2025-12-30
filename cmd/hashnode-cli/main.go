package main

import (
	"fmt"
	"os"
)

// This is the global variable other commands (init, plan) attach to.

func main() {
	if err := Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
