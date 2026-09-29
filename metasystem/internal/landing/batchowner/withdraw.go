package batchowner

import (
	"os"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
)

type BatchWithdrawRequest struct {
	SeatRoot, LandingRoot, GoalID string
	At                            time.Time
}

type BatchWithdrawDependencies struct {
	Machine  func(string) (string, error)
	Lineage  func() string
	Withdraw func(batch.Store, string, string, string, string, string, time.Time) (batch.Record, error)
	Ensure   func(string) error
}

var BatchWithdrawDependenciesForCommand = ProductionBatchWithdrawDependencies
var BatchWithdrawClock = fixtureauth.GoalNow

func ProductionBatchWithdrawDependencies() BatchWithdrawDependencies {
	return BatchWithdrawDependencies{
		Machine:  goal.ResolveMachine,
		Lineage:  func() string { return os.Getenv("METASYSTEM_OWNER_LINEAGE") },
		Withdraw: batch.RequestWithdrawal,
		Ensure:   EnsureBatchOwner,
	}
}
