package plain

// What the keeper reads of the plain lane: why the landing agent would run
// now, and the one hold, a proof that runs. The keeper itself is
// internal/landing/lane's and checks the recorded continuation scope.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

// The wake reasons of the plain lane.
const (
	// WakeQueued: a hand-in is neither returned nor contained in main.
	WakeQueued = "queued"
	// WakeMissingHandIn names a hand-in the selection could not obtain.
	WakeMissingHandIn = "missing-hand-in"
	// WakeProofFinished: a proof ended after the agent's last launch and
	// queued work remains.
	WakeProofFinished = "proof-finished"
	// WakeFullDue: main is due for a fresh full check.
	WakeFullDue = "full-due"
)

// WakeReasons are why the landing agent would run now: WakeQueued while a
// hand-in is pending (Pending), and WakeProofFinished besides when a result
// line ended after lastLaunch (zero: never launched). WakeFullDue also wakes
// an idle lane when a scoped push's full proof is more than an hour old.
// When incidents alone hold all waiting lines, one fetch refreshes main
// before the keeper decides. A failed fetch leaves the lines held and is
// reported by landing status through the wake's unread sources.
func WakeReasons(install, checkout string, lastLaunch, now time.Time, effects ...ProveSeams) ([]string, error) {
	seams := ProveSeams{}
	if len(effects) > 0 {
		seams = effects[0]
	}
	pending, err := pendingQueue(install, checkout, seams, true)
	waiting, waitingErr := Waiting(install)
	missing := handInCommits(install, checkout, waiting, seams, false)
	reasons, reasonErr := wakeReasons(install, len(pending) > 0, lastLaunch, now, seams.TimerHeld != nil && seams.TimerHeld())
	if missing != nil {
		reasons = append(reasons, WakeMissingHandIn)
	}
	return reasons, errors.Join(err, waitingErr, missing, reasonErr)
}

func wakeReasons(install string, queued bool, lastLaunch, now time.Time, timerHeld ...bool) ([]string, error) {
	reasons := []string{}
	if queued {
		reasons = append(reasons, WakeQueued)
		last, ok, err := LastResult(install)
		if err != nil {
			return nil, err
		}
		gate, gateOK, err := LastGate(install)
		if err != nil {
			return nil, err
		}
		if gateOK && (!ok || resultTime(gate).After(resultTime(last))) {
			last, ok = gate, true
		}
		if ok {
			if at, err := time.Parse(time.RFC3339, last.At); err == nil && at.After(lastLaunch) {
				reasons = append(reasons, WakeProofFinished)
			}
		}
	}
	drain, drainErr := ReadDrain(install)
	if len(timerHeld) > 0 && timerHeld[0] || drain != nil || drainErr != nil {
		return reasons, nil
	}
	due, err := fullProofDue(install, now)
	if err != nil {
		return nil, err
	}
	if due {
		reasons = append(reasons, WakeFullDue)
	}
	return reasons, nil
}

// fullProofDue preserves the hourly debt and checks main's independent clock.
func fullProofDue(install string, now time.Time) (bool, error) {
	due, err := scopedProofDue(install, now)
	if err != nil || due {
		return due, err
	}
	raw, _, err := config.Get(config.GetParams{Key: "proof.trunk-every", ConfPath: filepath.Join(install, "metasystem.conf")})
	if err != nil {
		return false, err
	}
	every, err := time.ParseDuration(raw)
	if err != nil || every <= 0 {
		return false, fmt.Errorf("main's full check interval must be a positive duration; %s is %q", "proof.trunk-every", raw)
	}
	results, err := Results(install)
	if err != nil {
		return false, err
	}
	pushes, err := readLines[Pushed](pushesPath(install))
	if err != nil {
		return false, err
	}
	last := time.Time{}
	for _, proof := range results {
		if proof.Trunk {
			if at, err := time.Parse(time.RFC3339, proof.At); err == nil && at.After(last) {
				last = at
			}
		}
	}
	for _, push := range pushes {
		pushedAt, err := time.Parse(time.RFC3339, push.At)
		if err != nil {
			continue
		}
		var proof Result
		for _, r := range results {
			at, err := time.Parse(time.RFC3339, r.At)
			if r.Tree == push.Tree && err == nil && !at.After(pushedAt) {
				proof = r
			}
		}
		if proof.Result == Green && proof.Scope == "full" && proof.FullTree == proof.Tree && proof.FullAt == proof.At && !strings.HasPrefix(proof.Reason, "inherits green from tree ") && pushedAt.After(last) {
			last = pushedAt
		}
	}
	return last.IsZero() || now.Sub(last) >= every, nil
}

// scopedProofDue reads only the newest push. Any later full green pays its
// debt, including a batch proof, because every lane tree contains main.
func scopedProofDue(install string, now time.Time) (bool, error) {
	push, ok, err := LastPush(install)
	if err != nil || !ok {
		return false, err
	}
	result, ok, err := ResultFor(install, push.Tree)
	if err != nil || !ok || result.Result != Green || result.Scope != "scoped" {
		return false, err
	}
	fullAt, err := time.Parse(time.RFC3339, result.FullAt)
	if err != nil || now.Sub(fullAt) <= time.Hour {
		return false, nil
	}
	pushedAt, err := time.Parse(time.RFC3339, push.At)
	if err != nil {
		return false, nil
	}
	results, err := Results(install)
	if err != nil {
		return false, err
	}
	for _, proof := range results {
		if proof.Scope == "full" && proof.Result == Green {
			if at, err := time.Parse(time.RFC3339, proof.At); err == nil && at.After(pushedAt) {
				return false, nil
			}
		}
	}
	return true, nil
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
func KeeperWake(home string, effects ...ProveSeams) lane.WakeSources {
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
		seams := ProveSeams{}
		if len(effects) > 0 {
			seams = effects[0]
		}
		return WakeReasons(string(layout.Install), string(layout.Checkout), launched, seams.now(), seams)
	}}
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
	gate, _, err := LastGate(install)
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
		Gate     Result  `json:"gate"`
		Pushed   Pushed  `json:"pushed"`
		Running  Running `json:"running"`
		Recorded bool    `json:"recorded"`
	}{entries, last, gate, pushed, running, recorded})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// DrainWake rechecks the atomic fence at the keeper's start claim.
func DrainWake(install, checkout string, wake lane.Wake, effects ...ProveSeams) (lane.Wake, error) {
	drain, err := ReadDrain(install)
	if err != nil {
		return wake, err
	}
	if drain == nil {
		return wake, nil
	}
	seams := ProveSeams{}
	if len(effects) > 0 {
		seams = effects[0]
	}
	pending, err := pending(install, checkout, seams)
	if err != nil {
		return wake, err
	}
	reasons := []string{}
	if drain.State != DrainHeld && len(pending) > 0 {
		for _, reason := range wake.Reasons {
			if reason != WakeFullDue {
				reasons = append(reasons, reason)
			}
		}
	}
	wake.Reasons = reasons
	return wake, nil
}
