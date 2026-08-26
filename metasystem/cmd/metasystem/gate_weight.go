package main

// The battery-weight verbs expose the accumulator's complete transaction:
// landings add, one isolated runner checkpoints, non-green runs abandon, and
// a green run consumes only its checkpointed share.

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gaterun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

const weightThresholdKey = "battery.weight-threshold"

func weightThreshold(root string) int64 {
	value, code, _ := config.Get(config.GetParams{
		Key: weightThresholdKey, Default: "60", DefaultSet: true,
		ConfPath: filepath.Join(root, "metasystem.conf"),
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
	prefix := flags.String("prefix", "", "metasystem path relative to the Git toplevel")
	if flags.Parse(args) != nil {
		return 2
	}
	if *root == "" || *commit == "" || flags.NArg() != 0 {
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
	state, due, err := gaterun.WeightCheck(*root, threshold)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("battery weight %d of %d over %d landing(s) since %s\n", state.Accumulated, threshold, state.Landings, state.SinceUTC)
	if due {
		return 1
	}
	return 0
}

func runGateWeightCheckpoint(args []string) int {
	flags := flag.NewFlagSet("gate weight-checkpoint", flag.ContinueOnError)
	root := flags.String("root", "", "real checkout root containing the accumulator")
	runID := flags.String("run-id", "", "battery run identity")
	subject := flags.String("subject", "", "exact subject commit")
	runnerPID := flags.Int64("runner-pid", 0, "durable controller pid")
	envelope := flags.String("envelope", "", "absolute durable envelope destination")
	if flags.Parse(args) != nil {
		return 2
	}
	if *root == "" || *runID == "" || *subject == "" || *runnerPID <= 0 || *envelope == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem gate weight-checkpoint --root R --run-id ID --subject SHA --runner-pid PID --envelope DIR")
		return 2
	}
	result, err := gaterun.WeightCheckpointOpen(*root, gaterun.CheckpointRequest{
		RunID: *runID, Subject: *subject, RunnerPID: *runnerPID, RepairDestination: *envelope,
	}, identity.KernelProber{})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		if errors.Is(err, gaterun.ErrCheckpointLive) || errors.Is(err, gaterun.ErrCheckpointUnknown) {
			return 3
		}
		return 1
	}
	printJSON(result)
	return 0
}

func runGateWeightAbandon(args []string) int {
	flags := flag.NewFlagSet("gate weight-abandon", flag.ContinueOnError)
	root := flags.String("root", "", "real checkout root containing the accumulator")
	runID := flags.String("run-id", "", "battery run identity")
	reason := flags.String("reason", "", "non-green terminal reason")
	bestEffort := flags.Bool("best-effort-appendix", false, "terminalize even when evidence-copy failure prevents appendix publication")
	if flags.Parse(args) != nil {
		return 2
	}
	if *root == "" || *runID == "" || *reason == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem gate weight-abandon --root R --run-id ID --reason REASON [--best-effort-appendix]")
		return 2
	}
	result, err := gaterun.WeightAbandon(*root, *runID, *reason, *bestEffort)
	printJSON(result)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func runGateWeightReset(args []string) int {
	flags := flag.NewFlagSet("gate weight-reset", flag.ContinueOnError)
	root := flags.String("root", "", "real checkout root containing the accumulator")
	runID := flags.String("run-id", "", "the checkpointed battery run")
	if flags.Parse(args) != nil {
		return 2
	}
	if *root == "" || *runID == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem gate weight-reset --root R --run-id ID")
		return 2
	}
	_, result, err := gaterun.WeightReset(*root, *runID)
	if result.RunID != "" {
		printJSON(result)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		var pending *gaterun.ResetAppendixPendingError
		if errors.As(err, &pending) {
			return 4
		}
		return 1
	}
	return 0
}
