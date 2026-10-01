package lane

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

// livenessProber answers each pid from a table: listed pids are alive at
// start 1, every other pid is gone.
type livenessProber map[int64]bool

func (p livenessProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	if p[pid] {
		return identity.Exact{Pid: pid, StartedAt: time.Unix(1, 0)}, identity.Alive, nil
	}
	return identity.Exact{}, identity.Dead, nil
}

// writeLaneAttempt writes one proof attempt record into the lane's
// installation as landing prove's child leaves it while it runs (no
// terminal) or after it ended.
func writeLaneAttempt(t *testing.T, install, id, account string, pid int64, mutate func(*proofrun.Attempt)) {
	t.Helper()
	attempt := proofrun.Attempt{SchemaVersion: 4, AttemptID: id, GoalID: account, CandidateGoalID: account,
		StartedAt: "2026-10-01T17:14:09Z", ControlRoot: install,
		Launcher: proofrun.ProcessIdentity{Pid: pid, PidStartedAt: 1}}
	if mutate != nil {
		mutate(&attempt)
	}
	path, err := proofrun.AttemptPath(install, id)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(attempt)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestRunningProofHoldsOnlyTheLaneCheckout (moving main cancels a running
// lane proof, 2026-10-01): while a proof charged to the lane runs in the
// lane's installation, the lane checkout and its installation are held; a
// seat's checkout never is. Once the proof ended, was asked to cancel, or
// its launcher is gone, nothing holds.
func TestRunningProofHoldsOnlyTheLaneCheckout(t *testing.T) {
	t.Parallel()
	home, checkout, seat := laneDirs(t)
	register(t, home, checkout)
	layout, err := NewLayout(checkout)
	if err != nil {
		t.Fatal(err)
	}
	install, account := string(layout.Install), AccountID(string(layout.Checkout))
	prober := livenessProber{4242: true}

	if _, held, err := RunningProof(home, checkout, prober); held || err != nil {
		t.Fatalf("a lane with no proof attempt is held: %v %v", held, err)
	}

	writeLaneAttempt(t, install, "proof-running-0000000000000001", account, 4242, nil)
	for _, self := range []string{checkout, install} {
		hold, held, err := RunningProof(home, self, prober)
		if err != nil || !held || hold.Attempt != "proof-running-0000000000000001" || hold.StartedAt != "2026-10-01T17:14:09Z" {
			t.Fatalf("a running lane proof does not hold %s: %+v %v %v", self, hold, held, err)
		}
	}
	if _, held, err := RunningProof(home, seat, prober); held || err != nil {
		t.Fatalf("a seat checkout is held by the lane's proof: %v %v", held, err)
	}

	// The proof ends: the next read finds nothing running.
	writeLaneAttempt(t, install, "proof-running-0000000000000001", account, 4242, func(a *proofrun.Attempt) {
		a.EndedAt = "2026-10-01T17:44:09Z"
		a.Terminal = &proofrun.AttemptTerminal{Result: proofrun.TerminalSuccess, At: a.EndedAt}
	})
	if _, held, err := RunningProof(home, checkout, prober); held || err != nil {
		t.Fatalf("an ended proof still holds: %v %v", held, err)
	}

	// Neither a proof whose launcher is gone, one asked to cancel, nor a
	// proof charged to a goal holds the lane.
	writeLaneAttempt(t, install, "proof-dead-0000000000000002", account, 777, nil)
	writeLaneAttempt(t, install, "proof-cancel-0000000000000003", account, 4242, func(a *proofrun.Attempt) { a.CancellationIntent = "a person" })
	writeLaneAttempt(t, install, "proof-goal-0000000000000004", "some-goal", 4242, func(a *proofrun.Attempt) { a.CandidateGoalID = "some-goal" })
	if hold, held, err := RunningProof(home, checkout, prober); held || err != nil {
		t.Fatalf("a dead, cancelled or goal proof holds the lane: %+v %v %v", hold, held, err)
	}

	// A record that cannot be read holds: unknown is never a go.
	if err := os.WriteFile(filepath.Join(install, "artifacts", "agents", "proof-runs", "attempts", "proof-torn-0000000000000005.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, held, err := RunningProof(home, checkout, prober); err == nil {
		t.Fatalf("an unreadable attempt record reads as no proof (held %v)", held)
	}
	if _, held, err := RunningProof(home, seat, prober); held || err != nil {
		t.Fatalf("the lane's unreadable record holds a seat: %v %v", held, err)
	}
}

// With no lane registered nothing is held.
func TestRunningProofWithoutALaneHoldsNothing(t *testing.T) {
	t.Parallel()
	home, checkout, _ := laneDirs(t)
	if _, held, err := RunningProof(home, checkout, livenessProber{}); held || err != nil {
		t.Fatalf("no lane = %v %v", held, err)
	}
}
