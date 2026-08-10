package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
)

// The mission-ledger family is the atomic owner of the stop-loss ledger
// (init, append, verify, count).

func runMissionLedgerInit(args []string) int {
	flags := flag.NewFlagSet("mission-ledger init", flag.ContinueOnError)
	file := flags.String("file", "", "ledger path")
	cycleBudget := flags.Int("cycle-budget", 0, "cycle budget")
	noGainBudget := flags.Int("no-gain-budget", 0, "no-gain budget")
	if flags.Parse(args) != nil {
		return 2
	}
	if err := mission.InitLedger(*file, *cycleBudget, *noGainBudget); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func runMissionLedgerAppend(args []string) int {
	flags := flag.NewFlagSet("mission-ledger append", flag.ContinueOnError)
	file := flags.String("file", "", "ledger path")
	cycle := flags.Int("cycle", 0, "cycle number (must be next)")
	classification := flags.String("classification", "", "cycle classification")
	sha := flags.String("candidate-sha", "", "resolved candidate git sha")
	observed := flags.String("observed", "", "observed measurement")
	if flags.Parse(args) != nil {
		return 2
	}
	if err := mission.AppendCycle(*file, *cycle, *classification, *sha, *observed); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func runMissionLedgerVerify(args []string) int {
	file, ok := singleFileFlag("mission-ledger verify", args)
	if !ok {
		return 2
	}
	_, _, cycles, err := mission.ParseLedger(file)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("mission ledger valid: %d cycles\n", len(cycles))
	return 0
}

func runMissionLedgerCount(args []string) int {
	file, ok := singleFileFlag("mission-ledger count", args)
	if !ok {
		return 2
	}
	_, _, cycles, err := mission.ParseLedger(file)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println(len(cycles))
	return 0
}

// The mission-state family owns the atomic, hash-chained mission state.

func runMissionStateInit(args []string) int {
	flags := flag.NewFlagSet("mission-state init", flag.ContinueOnError)
	state := flags.String("state", "", "state path")
	contract := flags.String("contract", "", "contract path")
	ledger := flags.String("ledger", "", "ledger path")
	lease := flags.String("lease", "", "runner lease reference")
	branch := flags.String("branch", "", "candidate branch override")
	if flags.Parse(args) != nil {
		return 2
	}
	if err := mission.InitState(*state, *contract, *ledger, *lease, *branch); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func runMissionStateWrite(args []string) int {
	flags := flag.NewFlagSet("mission-state write", flag.ContinueOnError)
	state := flags.String("state", "", "state path")
	source := flags.String("source", "", "proposed next state path")
	expect := flags.String("expect", "", "expected current state hash")
	if flags.Parse(args) != nil {
		return 2
	}
	if err := mission.WriteState(*state, *source, *expect); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func runMissionStateVerify(args []string) int {
	flags := flag.NewFlagSet("mission-state verify", flag.ContinueOnError)
	state := flags.String("state", "", "state path")
	repo := flags.String("repo", "", "repository (with --ledger, verifies the anchor)")
	ledger := flags.String("ledger", "", "ledger path (with --repo, verifies the anchor)")
	if flags.Parse(args) != nil {
		return 2
	}
	if (*repo == "") != (*ledger == "") {
		fmt.Fprintln(os.Stderr, "--repo and --ledger are required together for anchor verification")
		return 1
	}
	var (
		seq  int64
		hash string
		err  error
	)
	if *repo != "" {
		seq, hash, err = mission.VerifyStateWithAnchor(*state, *repo, *ledger)
	} else {
		seq, hash, err = mission.VerifyStateShape(*state)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("mission state valid: sequence=%d hash=%s\n", seq, hash)
	return 0
}

func runMissionStateAnchor(args []string) int {
	flags := flag.NewFlagSet("mission-state anchor", flag.ContinueOnError)
	state := flags.String("state", "", "state path")
	repo := flags.String("repo", "", "repository")
	ledger := flags.String("ledger", "", "ledger path")
	if flags.Parse(args) != nil {
		return 2
	}
	if err := mission.Anchor(*state, *repo, *ledger); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func runMissionStateReconcile(args []string) int {
	flags := flag.NewFlagSet("mission-state reconcile", flag.ContinueOnError)
	state := flags.String("state", "", "state path")
	repo := flags.String("repo", "", "repository")
	ledger := flags.String("ledger", "", "ledger path")
	if flags.Parse(args) != nil {
		return 2
	}
	code, err := mission.Reconcile(*state, *repo, *ledger)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		if code == 0 {
			code = 1
		}
	}
	return code
}

func singleFileFlag(name string, args []string) (string, bool) {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	file := flags.String("file", "", "ledger path")
	if flags.Parse(args) != nil || *file == "" {
		return "", false
	}
	return *file, true
}
