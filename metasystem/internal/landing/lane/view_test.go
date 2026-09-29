package lane

import (
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
)

func viewSources(home string, alive bool, records []batch.Record) ViewSources {
	return ViewSources{Home: home, Now: laneNow,
		Owner: func(string) (OwnerProbe, error) {
			if !alive {
				return OwnerProbe{}, nil
			}
			return OwnerProbe{Alive: true, PID: 4242, Since: laneNow.Add(-time.Hour)}, nil
		},
		Records: func(string) ([]batch.Record, error) { return records, nil }}
}

func keysOf(t *testing.T, raw json.RawMessage) []string {
	t.Helper()
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		t.Fatal(err)
	}
	var keys []string
	for key := range object {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// The API's shape is fixed: every key is present, absent values are null.
func TestViewShapeWithoutALane(t *testing.T) {
	t.Parallel()
	home, _, _ := laneDirs(t)
	view := BuildView(viewSources(home, false, nil))
	data, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := keysOf(t, data), []string{"batch", "next", "owner", "registered_at", "registered_by", "root", "spend", "summary"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("lane keys = %v, want %v", got, want)
	}
	var object map[string]json.RawMessage
	_ = json.Unmarshal(data, &object)
	for _, key := range []string{"root", "registered_by", "registered_at", "batch", "next", "spend"} {
		if string(object[key]) != "null" {
			t.Errorf("%s = %s; want null", key, object[key])
		}
	}
	if got, want := keysOf(t, object["owner"]), []string{"last_exit", "pid", "restarts", "retry_hint", "since", "state", "stopped_by"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("owner keys = %v, want %v", got, want)
	}
	if view.Owner.State != OwnerNotStarted || !strings.Contains(view.Summary, "no landing lane is registered") {
		t.Fatalf("view = %+v", view)
	}
}

func joinedUnit(goal, seat string) batch.Unit {
	return batch.Unit{GoalID: goal, State: batch.UnitJoined, Claim: batch.Claim{Machine: seat}}
}

// A running owner with one batch proving and the next collecting.
func TestViewProvingBatchAndNext(t *testing.T) {
	t.Parallel()
	home, root, _ := laneDirs(t)
	if _, err := Resolve(home, root, "m1e", laneNow, true); err != nil {
		t.Fatal(err)
	}
	started := laneNow.Add(-5 * time.Minute).Format(time.RFC3339Nano)
	proving := batch.Record{BatchID: "b1", State: batch.StateProving, StartReason: "nothing else is underway",
		Units:   []batch.Unit{joinedUnit("g1", "m1e"), joinedUnit("g2", "ui"), {GoalID: "g0", State: batch.UnitEjected}},
		History: []batch.HistoryEntry{{At: laneNow.Add(-time.Hour).Format(time.RFC3339Nano), Verb: "join", From: "open", To: "open"}, {At: started, Verb: "seal", From: "open", To: batch.StateProving}}}
	expected := laneNow.Add(20 * time.Minute)
	collecting := batch.Record{BatchID: "b2", State: batch.StateOpen, Units: []batch.Unit{joinedUnit("g3", "m1e")},
		Wait: &batch.WaitState{Reason: "g4", Since: laneNow.Add(-2 * time.Minute), For: []batch.Waited{{Goal: "g4", Seat: "ui", ExpectedAt: expected}}}}
	view := BuildView(viewSources(home, true, []batch.Record{collecting, proving}))
	if view.Root == nil || *view.Root != resolved(root) || view.RegisteredBy == nil || *view.RegisteredBy != "m1e" {
		t.Fatalf("registration = %+v", view)
	}
	if view.Owner.State != OwnerRunning || view.Owner.PID == nil || *view.Owner.PID != 4242 || view.Owner.Since == nil {
		t.Fatalf("owner = %+v", view.Owner)
	}
	if view.Batch == nil || view.Batch.ID != "b1" || view.Batch.State != BatchProving || view.Batch.Since != started ||
		!reflect.DeepEqual(view.Batch.Members, []Member{{Goal: "g1", Seat: "m1e"}, {Goal: "g2", Seat: "ui"}}) || view.Batch.WaitingFor == nil {
		t.Fatalf("batch = %+v", view.Batch)
	}
	if view.Next == nil || view.Next.ID != "b2" || !reflect.DeepEqual(view.Next.Members, []Member{{Goal: "g3", Seat: "m1e"}}) {
		t.Fatalf("next = %+v", view.Next)
	}
	for _, want := range []string{resolved(root), "owner running", "batch b1 proving", "next b2"} {
		if !strings.Contains(view.Summary, want) {
			t.Errorf("summary %q lacks %q", view.Summary, want)
		}
	}
	// With only the collecting batch, it is the current one and names what it waits for.
	view = BuildView(viewSources(home, true, []batch.Record{collecting}))
	if view.Batch == nil || view.Batch.State != BatchWaiting || len(view.Batch.WaitingFor) != 1 || view.Batch.WaitingFor[0].Expected == nil ||
		*view.Batch.WaitingFor[0].Expected != expected.Format(time.RFC3339) || view.Next != nil {
		t.Fatalf("waiting batch = %+v next %+v", view.Batch, view.Next)
	}
}

// A paused lane and a given-up keeper each say what a person runs.
func TestViewPausedAndGivenUp(t *testing.T) {
	t.Parallel()
	home, root, _ := laneDirs(t)
	if _, err := Resolve(home, root, "m1e", laneNow, true); err != nil {
		t.Fatal(err)
	}
	if _, err := SetPause(home, "Wido", laneNow); err != nil {
		t.Fatal(err)
	}
	view := BuildView(viewSources(home, true, nil))
	if view.Owner.State != OwnerStopped || view.Owner.StoppedBy == nil || *view.Owner.StoppedBy != "Wido" || view.Owner.RetryHint == nil ||
		!strings.Contains(*view.Owner.RetryHint, "metasystem landing start") || !strings.Contains(view.Summary, "stopped by Wido") {
		t.Fatalf("paused view = %+v %+v", view.Owner, view.Summary)
	}
	if _, err := ClearPause(home); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(home, keeperPath(home), KeeperState{Failures: 5, Restarts: 4, Since: laneNow.Format(time.RFC3339), GaveUp: laneNow.Format(time.RFC3339), LastError: "up refused"}); err != nil {
		t.Fatal(err)
	}
	view = BuildView(viewSources(home, false, nil))
	if view.Owner.State != OwnerGivenUp || view.Owner.Restarts != 4 || view.Owner.LastExit == nil || *view.Owner.LastExit != "up refused" ||
		view.Owner.RetryHint == nil || !strings.Contains(view.Summary, "metasystem landing start") {
		t.Fatalf("given-up view = %+v %q", view.Owner, view.Summary)
	}
}

// F-6: a batch waiting for the proving flock shows "waiting" throughout the
// wait, joins included, until its state changes.
func TestViewShowsTheProvingWaitAcrossJoins(t *testing.T) {
	t.Parallel()
	home, root, _ := laneDirs(t)
	if _, err := Resolve(home, root, "m1e", laneNow, true); err != nil {
		t.Fatal(err)
	}
	at := laneNow.Format(time.RFC3339Nano)
	waiting := batch.Record{BatchID: "b2", State: batch.StateOpen, Units: []batch.Unit{joinedUnit("g3", "m1e"), joinedUnit("g4", "ui")},
		History: []batch.HistoryEntry{
			{At: at, Verb: batch.ProvingWaitVerb, From: batch.StateOpen, To: batch.StateOpen, Detail: "another batch proves on this host (pid 99); this batch starts when that proof ends"},
			{At: at, Verb: "join", From: batch.StateOpen, To: batch.StateOpen, Detail: "g4 joined"},
		}}
	view := BuildView(viewSources(home, true, []batch.Record{waiting}))
	if view.Batch == nil || view.Batch.State != BatchWaiting || !strings.Contains(view.Batch.Reason, "pid 99") {
		t.Fatalf("a join ended the shown wait: %+v", view.Batch)
	}
}

// F-4: a registered lane whose checkout is gone tells the page the command
// that fixes it.
func TestViewOfAGoneLaneNamesTheFix(t *testing.T) {
	t.Parallel()
	home, root, _ := laneDirs(t)
	if _, err := Resolve(home, root, "m1e", laneNow, true); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(root); err != nil {
		t.Fatal(err)
	}
	view := BuildView(viewSources(home, false, nil))
	if view.Root == nil || view.Owner.RetryHint == nil || !strings.Contains(*view.Owner.RetryHint, "metasystem landing set PATH") || !strings.Contains(view.Summary, "no longer exists") {
		t.Fatalf("gone lane view = %+v %q", view.Owner, view.Summary)
	}
}

// TestLaneViewShowsChangeMembers (U11b): a change member is listed as its
// change id from the asker's seat, and a change that left the batch is
// listed with its outcome and reason, which is where its asker reads it.
func TestLaneViewShowsChangeMembers(t *testing.T) {
	t.Parallel()
	home, root, _ := laneDirs(t)
	if _, err := Resolve(home, root, "m1e", laneNow, true); err != nil {
		t.Fatal(err)
	}
	change := batch.NewChangeUnit(batch.ChangeMember{Commit: "abcdef0123456789abcdef0123456789abcdef01", AskedBy: "m1e+human"}, "/seat", "m1e", "human", nil, nil)
	change.State = batch.UnitJoined
	ejected := batch.NewChangeUnit(batch.ChangeMember{Commit: "1234567890ab1234567890ab1234567890ab1234", AskedBy: "ui+human"}, "/ui", "ui", "human", nil, nil)
	ejected.State = batch.UnitEjected
	open := batch.Record{BatchID: "b1", State: batch.StateOpen, Units: []batch.Unit{joinedUnit("g1", "m1b"), change, ejected}}
	open.Units[2].Outcome, open.Units[2].Failure = batch.UnitEjected, "EJECTED from landing batch b1: TestNotes failed on the batch tip"
	view := BuildView(viewSources(home, true, []batch.Record{open}))
	if view.Batch == nil || !reflect.DeepEqual(view.Batch.Members, []Member{{Goal: "g1", Seat: "m1b"}, {Goal: "change:abcdef012345", Seat: "m1e"}}) {
		t.Fatalf("members = %+v", view.Batch)
	}
	want := []Returned{{Goal: "change:1234567890ab", Seat: "ui", Outcome: batch.UnitEjected, Reason: "EJECTED from landing batch b1: TestNotes failed on the batch tip"}}
	if !reflect.DeepEqual(view.Batch.Returned, want) {
		t.Fatalf("returned = %+v", view.Batch.Returned)
	}
}

// TestLaneViewUnreadableRecordIsNotFewerMembers (U11b, fail closed): a batch
// record the lane cannot read makes the summary say so; it never reads as a
// lane with fewer members.
func TestLaneViewUnreadableRecordIsNotFewerMembers(t *testing.T) {
	t.Parallel()
	home, root, _ := laneDirs(t)
	if _, err := Resolve(home, root, "m1e", laneNow, true); err != nil {
		t.Fatal(err)
	}
	store := batch.NewStore(root, nil)
	if err := store.Create(batch.Record{Schema: 1, BatchID: "01j5x00000000000000000ba01", State: batch.StateOpen,
		Units: []batch.Unit{{GoalID: "g1", Chain: "c1", State: batch.UnitJoined, Claim: batch.Claim{Machine: "m1e", Lineage: "l", Epoch: 1, Revision: 1, AccountingRevision: 1}}}}); err != nil {
		t.Fatal(err)
	}
	broken := root + "/artifacts/agents/landing-batches/01j5x00000000000000000ba02.json"
	if err := os.WriteFile(broken, []byte("{\"schema\": 1, \"batchId\""), 0o644); err != nil {
		t.Fatal(err)
	}
	sources := viewSources(home, true, nil)
	sources.Records = nil
	view := BuildView(sources)
	if !strings.Contains(view.Summary, "its batches are unreadable") {
		t.Fatalf("an unreadable record read as fewer members: %q", view.Summary)
	}
}

// TestLaneViewShowsTheLanesSpend (U11b): the proofs a lane charged to its own
// account (batches of changes) are its spend, shown beside the lane, with no
// goal named; without a lane the key is null.
func TestLaneViewShowsTheLanesSpend(t *testing.T) {
	t.Parallel()
	home, root, _ := laneDirs(t)
	if _, err := Resolve(home, root, "m1e", laneNow, true); err != nil {
		t.Fatal(err)
	}
	sources := viewSources(home, true, nil)
	var asked string
	sources.Spend = func(laneRoot, account string) (Spend, error) {
		asked = laneRoot + "|" + account
		return Spend{Account: account, Attempts: 2, ReservedMinutes: 90}, nil
	}
	view := BuildView(sources)
	if asked != resolved(root)+"|"+AccountID(root) || view.Spend == nil || *view.Spend != (Spend{Account: AccountID(root), Attempts: 2, ReservedMinutes: 90}) {
		t.Fatalf("asked=%q spend=%+v", asked, view.Spend)
	}
}
