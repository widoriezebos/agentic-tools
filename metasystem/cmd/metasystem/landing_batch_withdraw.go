package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
)

type batchWithdrawRequest struct {
	SeatRoot, LandingRoot, GoalID string
	At                            time.Time
}

type batchWithdrawDependencies struct {
	machine  func(string) (string, error)
	lineage  func() string
	withdraw func(batch.Store, string, string, string, string, string, time.Time) (batch.Record, error)
	ensure   func(string) error
}

var batchWithdrawDependenciesForCommand = productionBatchWithdrawDependencies
var batchWithdrawClock = goalCommandNow

func productionBatchWithdrawDependencies() batchWithdrawDependencies {
	return batchWithdrawDependencies{
		machine:  goal.ResolveMachine,
		lineage:  func() string { return os.Getenv("METASYSTEM_OWNER_LINEAGE") },
		withdraw: batch.RequestWithdrawal,
		ensure:   ensureBatchOwner,
	}
}

func executeBatchWithdraw(request batchWithdrawRequest, dependencies batchWithdrawDependencies) (batch.Record, error) {
	machine, err := dependencies.machine(request.SeatRoot)
	if err != nil {
		return batch.Record{}, err
	}
	lineage := dependencies.lineage()
	if lineage == "" {
		return batch.Record{}, fmt.Errorf("BATCH_WITHDRAW_REFUSED: export METASYSTEM_OWNER_LINEAGE for the recorded joiner")
	}
	record, err := dependencies.withdraw(batch.NewStore(request.LandingRoot, nil), request.GoalID, machine, lineage, request.SeatRoot, machine+"+"+lineage, request.At)
	if err != nil {
		return batch.Record{}, err
	}
	if err := dependencies.ensure(request.LandingRoot); err != nil {
		return batch.Record{}, err
	}
	return record, nil
}

func runBatchWithdraw(args []string) int {
	if !batchCapabilitiesAvailable() {
		return runBatchVerbSkeleton(nil)
	}
	flags := flag.NewFlagSet("landing batch withdraw", flag.ContinueOnError)
	root := pathFlag(flags, "root", ".", "seat checkout root")
	goalID := flags.String("goal", "", "joined goal id")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *goalID == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem landing batch withdraw --root ROOT --goal GOAL")
		return 2
	}
	now, err := batchWithdrawClock(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	settings, err := config.ResolveBatchLanding(filepath.Join(*root, "metasystem.conf"), *root, func() time.Time { return now })
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	record, err := executeBatchWithdraw(batchWithdrawRequest{SeatRoot: *root, LandingRoot: settings.Root, GoalID: *goalID, At: now}, batchWithdrawDependenciesForCommand())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	printJSON(map[string]any{"batchId": record.BatchID, "goalId": *goalID, "state": batch.UnitReturnPending, "outcome": batch.UnitWithdrawn})
	return 0
}
