package lane

import (
	"encoding/json"
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
	home, _, _ := laneDirs(t)
	view := BuildView(viewSources(home, false, nil))
	data, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := keysOf(t, data), []string{"batch", "next", "owner", "registered_at", "registered_by", "root", "summary"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("lane keys = %v, want %v", got, want)
	}
	var object map[string]json.RawMessage
	_ = json.Unmarshal(data, &object)
	for _, key := range []string{"root", "registered_by", "registered_at", "batch", "next"} {
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
