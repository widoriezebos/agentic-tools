package missionrunner

import "github.com/widoriezebos/agentic-tools/metasystem/internal/mission"

// missionContinuity owns the state and ledger checks that require the
// mission anchor. The runner keeps the shape-only read on the state file.
type missionContinuity interface {
	Reconcile(statePath, root, ledgerPath string) (int, error)
	LedgerPin(root, missionID string) (string, error)
	VerifyStateWithAnchor(statePath, root, ledgerPath string) (int64, string, error)
}

type defaultMissionContinuity struct{}

func (defaultMissionContinuity) Reconcile(statePath, root, ledgerPath string) (int, error) {
	return mission.Reconcile(statePath, root, ledgerPath)
}

func (defaultMissionContinuity) LedgerPin(root, missionID string) (string, error) {
	return mission.AnchoredLedgerSHA(root, missionID)
}

func (defaultMissionContinuity) VerifyStateWithAnchor(statePath, root, ledgerPath string) (int64, string, error) {
	return mission.VerifyStateWithAnchor(statePath, root, ledgerPath)
}

func (e *Engine) continuity() missionContinuity {
	if e.continuityFacts != nil {
		return e.continuityFacts
	}
	return defaultMissionContinuity{}
}
