package steward

import (
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// ledgerAttentionRepository is the set of goal repository operations used by
// one attention pass. State files and attention policy remain owned here.
type ledgerAttentionRepository struct {
	ResolveEndpoint   func(string) (goal.Endpoint, error)
	AcceptedLedgerTip func(string) (string, bool, error)
	ResolveMachine    func(string) (string, error)
	ProjectAt         func(string, string, ...time.Time) (goal.Projection, error)
	Entries           func(string) ([]goal.Entry, error)
	LedgerChanges     func(string, string, string) ([]goal.LedgerChange, error)
	CaptureTipBounded func(goal.Endpoint, time.Duration) (goal.BoundedCapture, error)
	CleanupRefs       func(goal.Endpoint, string)
	SyncModeGate      func(goal.Endpoint, string) error
	AcceptanceGates   func(string, string, string) error
	ValidateCommit    func(string, string) error
	AdvanceAccepted   func(string, string) error
	IsAncestor        func(string, string, string) (bool, error)
}

func defaultLedgerAttentionRepository() *ledgerAttentionRepository {
	return &ledgerAttentionRepository{
		ResolveEndpoint:   goal.ResolveEndpoint,
		AcceptedLedgerTip: goal.AcceptedLedgerTip,
		ResolveMachine:    goal.ResolveMachine,
		ProjectAt:         goal.ProjectAt,
		Entries:           goal.Entries,
		LedgerChanges:     goal.LedgerChanges,
		CaptureTipBounded: goal.CaptureTipBounded,
		CleanupRefs:       goal.CleanupRefs,
		SyncModeGate:      goal.SyncModeGate,
		AcceptanceGates:   goal.AcceptanceGates,
		ValidateCommit:    goal.ValidateCommit,
		AdvanceAccepted:   goal.AdvanceAccepted,
		IsAncestor:        goal.IsAncestor,
	}
}
