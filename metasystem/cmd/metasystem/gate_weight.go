package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gaterun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalrevision"
)

const weightThresholdKey = "validation.weight-threshold"

func weightThreshold(root string) int64 {
	value, code, _ := config.Get(config.GetParams{Key: weightThresholdKey, Default: "60", DefaultSet: true,
		ConfPath: filepath.Join(root, "metasystem.conf")})
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
	commit := flags.String("commit", "", "landed commit")
	prefix := flags.String("prefix", "", "metasystem path relative to Git toplevel")
	if flags.Parse(args) != nil || *root == "" || *commit == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem gate weight-add --root R --commit SHA [--prefix PREFIX]  (numstat -z on stdin)")
		return 2
	}
	numstat, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	state, due, err := gaterun.WeightAdd(*root, *commit, numstat, *prefix, weightThreshold(*root))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("validation weight %d over %d landing(s) since %s\n", state.Accumulated, state.Landings, state.SinceUTC)
	if due {
		fmt.Printf("validation weight reached (threshold %d): run the governed direct validator; findings fix forward\n", weightThreshold(*root))
	}
	return 0
}

func runGateWeightCheck(args []string) int {
	flags := flag.NewFlagSet("gate weight-check", flag.ContinueOnError)
	root := flags.String("root", "", "checkout root")
	if flags.Parse(args) != nil || *root == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem gate weight-check --root R")
		return 2
	}
	threshold := weightThreshold(*root)
	state, due, err := gaterun.WeightCheck(*root, threshold)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("validation weight %d of %d over %d landing(s) since %s\n", state.Accumulated, threshold, state.Landings, state.SinceUTC)
	if due {
		return 1
	}
	return 0
}

func runGateWeightDischarge(args []string) int {
	flags := flag.NewFlagSet("gate weight-discharge", flag.ContinueOnError)
	root := flags.String("root", "", "checkout root")
	goalID := flags.String("goal", "", "goal owning the governed validation")
	revision := flags.Uint64("obligation-revision", 0, "exact obligation revision")
	runID := flags.String("run-id", "", "green governed run")
	if flags.Parse(args) != nil || *root == "" || *goalID == "" || *revision == 0 || *runID == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem gate weight-discharge --root R --goal ID --obligation-revision N --run-id ID")
		return 2
	}
	binding, err := dispatchcore.ResolveGoalBinding(*root, *goalID, time.Now().UTC())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	held, err := goalrevision.Acquire(*root, *goalID, binding.Revision, "validation-weight-discharge")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer held.Release()
	result, err := gaterun.WeightDischarge(*root, *goalID, *revision, *runID)
	printJSON(result)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if !result.Decision.Applied {
		return 3
	}
	return 0
}
