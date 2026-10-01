package lane

import (
	"errors"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
)

// queuedBatch is a lane's batch record with a member waiting.
var queuedBatch = []batch.Record{{BatchID: "b-one", State: batch.StateOpen, Units: []batch.Unit{{GoalID: "g-one", State: batch.UnitJoined}}}}

// TestResumeGrantsAFreshAllowanceAndKeepsHistory (K10, R8-08): a batch's
// fifth execution is refused and owes an alert, without pausing the lane;
// a person's grant gives a fresh allowance of four, and the history of the
// first is kept.
func TestResumeGrantsAFreshAllowanceAndKeepsHistory(t *testing.T) {
	t.Parallel()
	home, _, _ := nestedLaneDirs(t)
	charge := func() error {
		return Gate(home, OpProve, AuthorityAgent, func(Record) error { return ChargeHeld(home, "b-one", laneNow) })
	}
	for index := 0; index < AllowanceExecutions; index++ {
		if err := charge(); err != nil {
			t.Fatalf("execution %d: %v", index+1, err)
		}
	}
	var refusal *Refusal
	if err := charge(); !errors.As(err, &refusal) || refusal.Code != CodeAllowanceSpent {
		t.Fatalf("the fifth execution = %v; want the allowance's refusal", err)
	}
	if _, paused := ReadPause(home); paused {
		t.Fatal("a spent allowance paused the lane")
	}
	if batch, spent, err := AllowanceSpent(home); err != nil || !spent || batch != "b-one" {
		t.Fatalf("spent allowance = %q %t %v; want b-one", batch, spent, err)
	}
	before, _ := ReadStopLoss(home)
	if err := Grant(home, "Wido", laneNow); err != nil {
		t.Fatal(err)
	}
	if _, spent, _ := AllowanceSpent(home); spent {
		t.Fatal("a fresh grant still reads the allowance spent")
	}
	for index := 0; index < AllowanceExecutions; index++ {
		if err := charge(); err != nil {
			t.Fatalf("after the resume, execution %d: %v", index+1, err)
		}
	}
	after, _ := ReadStopLoss(home)
	if after.Grant != before.Grant+1 || len(after.History) <= len(before.History) || !slices.Equal(after.History[:len(before.History)], before.History) ||
		len(after.Hits) != 1 {
		t.Fatalf("after the resume: grant %d (was %d), history %d (was %d), hits %+v; want a new grant over the kept history",
			after.Grant, before.Grant, len(after.History), len(before.History), after.Hits)
	}
}

// TestOneFreshSessionPerBatch (D4): a session takes up one batch; begin of
// another batch in the same session is refused, and the next session
// takes it up.
func TestOneFreshSessionPerBatch(t *testing.T) {
	t.Parallel()
	home, _, module := nestedLaneDirs(t)
	clock := laneNow
	agent := &fakeAgent{}
	keeper := agent.keeper(home, module, &clock, WakeSources{Records: func(string) ([]batch.Record, error) { return queuedBatch, nil }})
	keeper.Step()
	begin := func(batchID string) error {
		return Gate(home, OpBegin, AuthorityAgent, func(Record) error { return BindBatchHeld(home, batchID, clock) })
	}
	if err := begin("b-one"); err != nil {
		t.Fatal(err)
	}
	if err := begin("b-one"); err != nil {
		t.Fatalf("the same batch begun again: %v", err)
	}
	var refusal *Refusal
	if err := begin("b-two"); !errors.As(err, &refusal) || refusal.Code != CodeFreshSession {
		t.Fatalf("a second batch in one session = %v; want refused", err)
	}
	// The session ends; the next batch is queued: a fresh session starts at
	// once and takes it up.
	agent.running, clock = "", clock.Add(time.Minute)
	if line := keeper.Step(); len(agent.starts) != 2 {
		t.Fatalf("after the session that landed its batch: %q; want a fresh session at once", line)
	}
	if err := begin("b-two"); err != nil {
		t.Fatalf("the fresh session's batch: %v", err)
	}
}

// TestRepeatedBindLeavesTheStoreUnchanged (R-129-ui): begin repeated for the
// batch its session already took up is success with no second record: the
// stop-loss store is not rewritten, with or without a running session.
func TestRepeatedBindLeavesTheStoreUnchanged(t *testing.T) {
	t.Parallel()
	for _, withSession := range []bool{false, true} {
		home, _, module := nestedLaneDirs(t)
		clock := laneNow
		if withSession {
			agent := &fakeAgent{}
			keeper := agent.keeper(home, module, &clock, WakeSources{Records: func(string) ([]batch.Record, error) { return queuedBatch, nil }})
			keeper.Step()
		}
		begin := func() error {
			return Gate(home, OpBegin, AuthorityAgent, func(Record) error { return BindBatchHeld(home, "b-one", clock) })
		}
		if err := begin(); err != nil {
			t.Fatal(err)
		}
		first, err := os.ReadFile(stopLossPath(home))
		if err != nil {
			t.Fatal(err)
		}
		clock = clock.Add(time.Minute)
		if err := begin(); err != nil {
			t.Fatalf("session=%v: the same batch begun again: %v", withSession, err)
		}
		again, err := os.ReadFile(stopLossPath(home))
		if err != nil {
			t.Fatal(err)
		}
		if string(again) != string(first) {
			t.Fatalf("session=%v: a repeated begin rewrote the stop-loss store:\n%s\nwas\n%s", withSession, again, first)
		}
	}
}
