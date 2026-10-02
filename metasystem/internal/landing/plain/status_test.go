package plain

import (
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

// ReadStatus is the lane as landing status --json and /api/board's lane both
// carry it (goal fleet-card-can-land-now: one reader for the terminal and
// the card): the view, whether the lane is paused and its agent alive, the
// queue with its states derived against origin's main, the running proof,
// the last proof and the last push.
func TestReadStatusIsTheLaneTheTerminalAndTheCardRead(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	before := b.originMain()
	shaA := b.seat("seat-a", "goal-a")
	b.handIn("m1e", "goal-a", shaA)
	head := b.merge("goal-a")
	b.prove(b.greenScript)
	if _, err := b.push(); err != nil {
		t.Fatal(err)
	}
	shaB := b.seat("seat-b", "goal-b")
	b.handIn("ui", "goal-b", shaB)
	b.git(b.checkout, "fetch", "--quiet", "origin")
	home := t.TempDir()
	record := lane.Record{Root: b.checkout, Install: b.install}
	view := lane.View{Root: &b.checkout, Owner: lane.OwnerView{State: lane.OwnerRunning}}

	status := ReadStatus(home, record, view, ProveSeams{})

	if status.Paused || !status.AgentAlive || status.Root == nil || *status.Root != b.checkout {
		t.Fatalf("paused %v alive %v root %v; want not paused, alive, the view's root", status.Paused, status.AgentAlive, status.Root)
	}
	states := map[string]string{}
	for _, entry := range status.Queue {
		states[entry.Goal] = entry.State
	}
	if len(status.Queue) != 2 || states["goal-a"] != StateLanded || states["goal-b"] != StateWaiting {
		t.Fatalf("queue %+v; want goal-a landed and goal-b waiting", status.Queue)
	}
	if status.RunningProof != nil {
		t.Fatalf("running proof %+v; want none", status.RunningProof)
	}
	if status.LastProof == nil || status.LastProof.Result != Green || status.LastProof.Commit != head {
		t.Fatalf("last proof %+v; want the green proof of %s", status.LastProof, head)
	}
	if status.LastPush == nil || status.LastPush.Old != before || status.LastPush.Commit != head {
		t.Fatalf("last push %+v; want %s to %s", status.LastPush, before, head)
	}

	if _, err := lane.SetPause(home, "Wido", bedNow); err != nil {
		t.Fatal(err)
	}
	idle := lane.View{Root: &b.checkout, Owner: lane.OwnerView{State: lane.OwnerStopped}}
	if paused := ReadStatus(home, record, idle, ProveSeams{}); !paused.Paused || paused.AgentAlive {
		t.Fatalf("paused %v alive %v; want paused and no agent", paused.Paused, paused.AgentAlive)
	}
}

// A lane with no root reads as its view and an empty queue, never null.
func TestReadStatusOfNoLaneIsAnEmptyQueue(t *testing.T) {
	t.Parallel()
	status := ReadStatus(t.TempDir(), lane.Record{}, lane.View{}, ProveSeams{})
	if status.Queue == nil || len(status.Queue) != 0 || status.LastProof != nil || status.LastPush != nil || status.RunningProof != nil {
		t.Fatalf("status of no lane = %+v", status)
	}
}
