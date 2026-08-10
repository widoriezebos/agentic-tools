package main

import (
	"fmt"
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/hooks"
)

// runHooksCheck ports check-own-hooks.py: verify this repository runs under the
// metasystem it ships. Two positional args: the live settings and shipped hooks.
func runHooksCheck(args []string) int {
	if len(args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: metasystem hooks check <live settings> <shipped hooks>")
		return 2
	}
	if err := hooks.CheckOwnHooks(args[0], args[1]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
