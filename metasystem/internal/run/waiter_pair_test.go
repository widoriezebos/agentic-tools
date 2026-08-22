package run

import (
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// pairProber replays a btime step: the same live process, constant pair,
// drifting seconds.
type pairProber struct{ shift int64 }

func (p pairProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	return identity.Exact{
		Pid: pid, StartedAt: time.Unix(5000+p.shift, 0),
		StartTicks: 4242, BootID: "boot-w",
	}, identity.Alive, nil
}

// Issue #1 run-package pairing: a pair-bearing waiter stays LIVE under
// clock drift (registration reports busy, LiveWaiter true), and cleanup
// still removes its own record after drift instead of orphaning it.
func TestWaiterPairSurvivesDrift(t *testing.T) {
	root := t.TempDir()
	store := &Store{Root: root, Prober: pairProber{0}}
	owner := Caller{MainId: "main-5000-1-abc123", SessionId: "s"}
	target := WaiterTarget{Generation: 1, LaunchNonce: "n"}
	if err := store.RegisterWaiter("supervise", "demo", owner, target); err != nil {
		t.Fatalf("register: %v", err)
	}
	drifted := &Store{Root: root, Prober: pairProber{4}}
	if err := drifted.RegisterWaiter("supervise", "demo", owner, target); err == nil {
		t.Fatal("a live pair-bearing waiter must read busy under drift")
	}
	if !LiveWaiter(root, pairProber{4}, "supervise", "demo", owner.MainId, target) {
		t.Fatal("pair-bearing waiter read dead under drift")
	}
	drifted.RemoveWaiter("supervise", "demo", owner)
	if LiveWaiter(root, pairProber{4}, "supervise", "demo", owner.MainId, target) {
		t.Fatal("cleanup under drift orphaned the waiter record")
	}
}
