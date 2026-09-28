package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gaterun"
)

// The gate family tracks gate runs: a running gate's marker tells the turn-end
// report work is in flight, and fence answers whether a foreign one still runs
// in this checkout.

// runGateFence refuses when a foreign gate run is live in the checkout. The
// asking process passes its own pid so its own run's marker — registered by
// itself or an ancestor — never blocks it. Exit 0 means clear; exit 1 names
// every blocking run on stderr.
func runGateFence(args []string) int {
	flags := flag.NewFlagSet("gate fence", flag.ContinueOnError)
	root := pathFlag(flags, "root", "", "checkout root")
	selfPid := flags.Int64("self-pid", 0, "asking process pid; markers in its own chain do not block")
	if flags.Parse(args) != nil {
		return 2
	}
	if *root == "" || *selfPid == 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem internal gate fence --root R --self-pid P")
		return 2
	}
	holders := gaterun.Fence(*root, *selfPid)
	for _, holder := range holders {
		fmt.Fprintf(os.Stderr, "gate %s is running as pid %d\n", holder.Gate, holder.Pid)
	}
	if len(holders) > 0 {
		return 1
	}
	return 0
}
