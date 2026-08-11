package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gaterun"
)

// The gate family tracks gate runs: a running gate registers a marker so the
// turn-end report knows work is in flight, and check answers whether one still
// runs in this checkout.

func runGateRegister(args []string) int {
	flags := flag.NewFlagSet("gate register", flag.ContinueOnError)
	root := flags.String("root", "", "checkout root")
	gate := flags.String("gate", "", "gate name")
	pid := flags.Int64("pid", 0, "gate process pid")
	if flags.Parse(args) != nil {
		return 2
	}
	if *root == "" || *gate == "" || *pid == 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem gate register --root R --gate G --pid P")
		return 2
	}
	path, err := gaterun.Register(*root, *pid, *gate)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if path != "" {
		fmt.Println(path)
	}
	return 0
}

// runGateFence refuses when a foreign gate run is live in the checkout. The
// asking process passes its own pid so its own run's marker — registered by
// itself or an ancestor — never blocks it. Exit 0 means clear; exit 1 names
// every blocking run on stderr.
func runGateFence(args []string) int {
	flags := flag.NewFlagSet("gate fence", flag.ContinueOnError)
	root := flags.String("root", "", "checkout root")
	selfPid := flags.Int64("self-pid", 0, "asking process pid; markers in its own chain do not block")
	if flags.Parse(args) != nil {
		return 2
	}
	if *root == "" || *selfPid == 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem gate fence --root R --self-pid P")
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

func runGateCheck(args []string) int {
	flags := flag.NewFlagSet("gate check", flag.ContinueOnError)
	root := flags.String("root", "", "checkout root")
	if flags.Parse(args) != nil {
		return 2
	}
	if *root == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem gate check --root R")
		return 2
	}
	if gaterun.Running(*root) {
		fmt.Println("1")
	} else {
		fmt.Println("0")
	}
	return 0
}
