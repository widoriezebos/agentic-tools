package plain

// What the keeper reads of the plain lane: why the landing agent would run
// now, and the one hold, a proof that runs. The keeper itself is
// internal/landing/lane's and does not change.

import (
	"fmt"
	"time"
)

// The wake reasons of the plain lane.
const (
	// WakeQueued: a hand-in is neither returned nor contained in main.
	WakeQueued = "queued"
	// WakeProofFinished: a proof ended after the agent's last launch and
	// queued work remains.
	WakeProofFinished = "proof-finished"
)

// WakeReasons are why the landing agent would run now: WakeQueued while a
// hand-in is pending (Pending), and WakeProofFinished besides when a result
// line ended after lastLaunch (zero: never launched).
func WakeReasons(install, checkout string, lastLaunch time.Time) ([]string, error) {
	pending, err := Pending(install, checkout)
	if err != nil {
		return nil, err
	}
	if len(pending) == 0 {
		return []string{}, nil
	}
	reasons := []string{WakeQueued}
	last, ok, err := LastResult(install)
	if err != nil {
		return nil, err
	}
	if ok {
		if at, err := time.Parse(time.RFC3339, last.At); err == nil && at.After(lastLaunch) {
			reasons = append(reasons, WakeProofFinished)
		}
	}
	return reasons, nil
}

// ProofHold is why the keeper holds the agent's start: running.json names
// a proof whose process runs; empty when none does.
func ProofHold(install string, seams ProveSeams) (string, error) {
	running, recorded, alive, err := ReadRunning(install, seams)
	if err != nil || !recorded || !alive {
		return "", err
	}
	return fmt.Sprintf("the proof of tree %s (attempt %s) runs since %s; the agent is woken when it ends", Short(running.Tree), running.Attempt, running.Since), nil
}
