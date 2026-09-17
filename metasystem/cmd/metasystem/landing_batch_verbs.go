package main

import (
	"fmt"
	"os"
)

var landingBatchVerbs = map[string]func([]string) int{
	"join": runBatchJoin, "status": runBatchStatus,
	"withdraw": runBatchWithdraw, "owner": runBatchOwner,
	"tick": runBatchTick, "wait": runBatchWait,
}

func runLandingBatch(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem landing batch <join|status|withdraw|owner|tick|wait>")
		return 2
	}
	run, ok := landingBatchVerbs[args[0]]
	if !ok {
		fmt.Fprintf(os.Stderr, "metasystem landing batch: unknown verb %q\n", args[0])
		return 2
	}
	return run(args[1:])
}

func runGoalHandover(args []string) int { return runGoalHandoverMutation(args) }

func runBatchVerbSkeleton(_ []string) int {
	if !batchCapabilitiesAvailable() {
		fmt.Fprintln(os.Stderr, "BATCH_UNAVAILABLE: automatic batch landing is not compiled in")
		return 1
	}
	return 0
}

func runBatchWithdraw(_ []string) int {
	if !batchCapabilitiesAvailable() {
		fmt.Fprintln(os.Stderr, "BATCH_UNAVAILABLE: automatic batch landing is not compiled in")
		return 1
	}
	fmt.Fprintln(os.Stderr, "BATCH_WITHDRAW_UNBUILT: landing batch withdraw is not built yet; nothing was withdrawn")
	return 1
}
