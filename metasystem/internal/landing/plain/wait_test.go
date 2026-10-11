package plain

import (
	"os"
	"reflect"
	"testing"
)

func TestWaitReturnsFinalRecordedResultAfterClassification(t *testing.T) {
	t.Parallel()
	install := t.TempDir()
	running := Running{Attempt: "attempt", Commit: "commit", Tree: "tree"}
	raw := Result{Attempt: running.Attempt, Result: Red, ClassificationPending: true}
	final := Result{Attempt: running.Attempt, Commit: running.Commit, Tree: running.Tree, Result: Green, Reason: "recorded result"}
	if err := os.MkdirAll(Dir(install), 0755); err != nil {
		t.Fatal(err)
	}
	if err := writeRunning(install, running); err != nil {
		t.Fatal(err)
	}
	if err := appendLine(resultsPath(install), raw); err != nil {
		t.Fatal(err)
	}
	polls := 0
	seams := ProveSeams{Alive: func(Running) bool { return polls == 0 }, WaitPoll: func() {
		polls++
		if err := appendLine(resultsPath(install), final); err != nil {
			t.Fatal(err)
		}
	}}
	got, err := Wait(install, install, running, seams)
	if err != nil || polls != 1 || !reflect.DeepEqual(got, final) {
		t.Fatalf("wait returned intermediate result: %+v polls=%d err=%v", got, polls, err)
	}
}

func TestWaitLostDetachedProcessRecordsRed(t *testing.T) {
	t.Parallel()
	b := newRepeatBed(t)
	running := Running{Attempt: "lost", Commit: "commit", Tree: "tree", Log: "lost.log"}
	if err := os.MkdirAll(Dir(b.install), 0755); err != nil {
		t.Fatal(err)
	}
	if err := writeRunning(b.install, running); err != nil {
		t.Fatal(err)
	}
	probes := 0
	b.seams.Alive = func(Running) bool { probes++; return false }
	b.seams.WaitPoll = func() {}
	got, err := Wait(b.install, b.checkout, running, b.seams)
	saved, found, readErr := LastResult(b.install)
	if err != nil || readErr != nil || !found || got.Result != Red || got.Cause == nil || got.Cause.Name != "lost-process" || !reflect.DeepEqual(got, saved) || probes > 4 {
		t.Fatalf("lost process wait: %+v saved=%+v probes=%d err=%v %v", got, saved, probes, err, readErr)
	}
}

func TestWaitRetainsItsResultWhenNextAttemptTakesTheSlot(t *testing.T) {
	t.Parallel()
	install := t.TempDir()
	if err := os.MkdirAll(Dir(install), 0755); err != nil {
		t.Fatal(err)
	}
	original := Running{Attempt: "original", Commit: "commit", Tree: "tree"}
	next := Running{Attempt: "next", Commit: "other", Tree: "other-tree"}
	final := Result{Attempt: original.Attempt, Result: Green}
	if err := writeRunning(install, next); err != nil {
		t.Fatal(err)
	}
	if err := appendLine(resultsPath(install), final); err != nil {
		t.Fatal(err)
	}
	ended := false
	seams := ProveSeams{Alive: func(r Running) bool { return r.Attempt != "original" || !ended }, WaitPoll: func() { ended = true }}
	got, err := Wait(install, install, original, seams)
	current, recorded, _, readErr := ReadRunning(install, seams)
	if err != nil || readErr != nil || !reflect.DeepEqual(got, final) || !recorded || !reflect.DeepEqual(current, next) {
		t.Fatalf("next attempt displaced waited result: got=%+v current=%+v err=%v %v", got, current, err, readErr)
	}
}

func TestWaitReturnsAttributedGateBaselineResult(t *testing.T) {
	t.Parallel()
	install := t.TempDir()
	if err := os.MkdirAll(Dir(install), 0755); err != nil {
		t.Fatal(err)
	}
	running := Running{Gate: true, Attempt: "merge-check", Commit: "merge", Tree: "merge-tree"}
	baseline := Result{Attempt: "merge-check-baseline", Requested: "merge", Attributed: "parent", Commit: "parent", Tree: "parent-tree", Result: Red, Executions: []ExecutionAdmission{{Attempt: running.Attempt}}}
	if err := appendLine(gatesPath(install), baseline); err != nil {
		t.Fatal(err)
	}
	got, err := Wait(install, install, running, ProveSeams{Alive: func(Running) bool { return false }})
	if err != nil || !reflect.DeepEqual(got, baseline) {
		t.Fatalf("wait lost the gate's attributed baseline: %+v %v", got, err)
	}
}
