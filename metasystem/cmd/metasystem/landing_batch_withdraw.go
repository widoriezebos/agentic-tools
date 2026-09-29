package main

import (
	"os"
	"time"

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
