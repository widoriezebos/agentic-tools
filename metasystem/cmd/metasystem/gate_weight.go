package main

// The battery-weight verbs: every landing folds its measured weight
// into the accumulator, check answers whether enough feature gravity
// has accumulated to make the milestone battery worth its cost, and
// only a green battery resets the window. The due answer is a nudge
// the boundary prints, never a refusal.

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gaterun"
)

const weightThresholdKey = "battery.weight-threshold"

// weightThreshold resolves the declared threshold; 0 disables the
// nudge. The default asks for the battery after roughly three or four
// feature landings of today's typical weight.
func weightThreshold(root string) int64 {
	value, code, _ := config.Get(config.GetParams{
		Key:        weightThresholdKey,
		Default:    "60",
		DefaultSet: true,
		ConfPath:   filepath.Join(root, "metasystem.conf"),
	})
	if code != 0 {
		return 60
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed < 0 {
		return 60
	}
	return parsed
}

func runGateWeightAdd(args []string) int {
	flags := flag.NewFlagSet("gate weight-add", flag.ContinueOnError)
	root := flags.String("root", "", "checkout root")
	commit := flags.String("commit", "", "the landed commit")
	if flags.Parse(args) != nil {
		return 2
	}
	if *root == "" || *commit == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem gate weight-add --root R --commit SHA  (numstat on stdin)")
		return 2
	}
	numstat, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	state, due, err := gaterun.WeightAdd(*root, *commit, string(numstat), weightThreshold(*root))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("battery weight %d over %d landing(s) since %s\n", state.Accumulated, state.Landings, state.SinceUTC)
	if due {
		fmt.Printf("battery weight reached (threshold %d): a milestone battery run is worth its cost now — findings fix forward\n", weightThreshold(*root))
	}
	return 0
}

func runGateWeightCheck(args []string) int {
	flags := flag.NewFlagSet("gate weight-check", flag.ContinueOnError)
	root := flags.String("root", "", "checkout root")
	if flags.Parse(args) != nil {
		return 2
	}
	if *root == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem gate weight-check --root R")
		return 2
	}
	threshold := weightThreshold(*root)
	state, due := gaterun.WeightCheck(*root, threshold)
	fmt.Printf("battery weight %d of %d over %d landing(s) since %s\n", state.Accumulated, threshold, state.Landings, state.SinceUTC)
	if due {
		return 1
	}
	return 0
}

func runGateWeightReset(args []string) int {
	flags := flag.NewFlagSet("gate weight-reset", flag.ContinueOnError)
	root := flags.String("root", "", "checkout root")
	commit := flags.String("commit", "", "the commit the green battery proved")
	if flags.Parse(args) != nil {
		return 2
	}
	if *root == "" || *commit == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem gate weight-reset --root R --commit SHA")
		return 2
	}
	state, err := gaterun.WeightReset(*root, *commit)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("battery weight reset at %s (window since %s)\n", state.LastCommit, state.SinceUTC)
	return 0
}
