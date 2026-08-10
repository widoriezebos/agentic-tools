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
