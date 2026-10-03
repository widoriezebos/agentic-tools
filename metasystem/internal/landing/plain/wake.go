package plain

// What the keeper reads of the plain lane: why the landing agent would run
// now, and the one hold, a proof that runs. The keeper itself is
// internal/landing/lane's and does not change.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
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
	return fmt.Sprintf("tree %s is being proven (attempt %s, since %s); the agent is woken when it ends", Short(running.Tree), running.Attempt, running.Since), nil
}

// KeeperWake is the lane keeper's wake source for the host lane at home:
// WakeReasons over the lane checkout at root, against the keeper's last
// launch of the landing agent. landing status reads the same.
func KeeperWake(home string) lane.WakeSources {
	return lane.WakeSources{Reasons: func(root string) ([]string, error) {
		layout, err := lane.NewLayout(root)
		if err != nil {
			return nil, err
		}
		state, err := lane.ReadAgentState(home)
		if err != nil {
			return nil, err
		}
		launched, _ := time.Parse(time.RFC3339, state.StartedAt)
		return WakeReasons(string(layout.Install), string(layout.Checkout), launched)
	}}
}

// KeeperProofHold is the keeper's hold while a proof runs in the lane
// checkout at root (ProofHold).
func KeeperProofHold(root string) (string, error) {
	layout, err := lane.NewLayout(root)
	if err != nil {
		return "", err
	}
	return ProofHold(string(layout.Install), ProveSeams{})
}

// KeeperFingerprint is the lane at root as its landing agent can change it:
// its queue (hand-ins and returns), its newest result and push, and a proof
// it started. The keeper compares it around a launch; equal means the launch
// moved nothing.
func KeeperFingerprint(root string) (string, error) {
	layout, err := lane.NewLayout(root)
	if err != nil {
		return "", err
	}
	install := string(layout.Install)
	entries, err := Entries(install)
	if err != nil {
		return "", err
	}
	last, _, err := LastResult(install)
	if err != nil {
		return "", err
	}
	pushed, _, err := LastPush(install)
	if err != nil {
		return "", err
	}
	running, recorded, _, err := ReadRunning(install, ProveSeams{})
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(struct {
		Entries  []Entry `json:"entries"`
		Last     Result  `json:"last"`
		Pushed   Pushed  `json:"pushed"`
		Running  Running `json:"running"`
		Recorded bool    `json:"recorded"`
	}{entries, last, pushed, running, recorded})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
