package main

import (
	"fmt"
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
)

// runConfigCanonicalModel ports canonical-model.py: print the canonical model
// key for a name given as the sole positional argument.
func runConfigCanonicalModel(args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: metasystem config canonical-model <name>")
		return 2
	}
	fmt.Println(config.CanonicalModel(args[0]))
	return 0
}
