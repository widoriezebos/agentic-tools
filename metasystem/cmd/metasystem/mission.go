package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
)

// The mission-ledger family ports mission-ledger.py: the atomic owner of the
// stop-loss ledger (init, append, verify, count).

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

func singleFileFlag(name string, args []string) (string, bool) {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	file := flags.String("file", "", "ledger path")
	if flags.Parse(args) != nil || *file == "" {
		return "", false
	}
	return *file, true
}
