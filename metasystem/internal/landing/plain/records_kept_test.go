package plain

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
)

// newRecordsMoveBed is the keeper bed of main with trees b and c equal to a
// but for one design page, so each move of main is a records-only move that
// is not goal records.
func newRecordsMoveBed(t *testing.T) *flakeMoveBed {
	t.Helper()
	b := newFlakeMoveBed(t)
	for _, tree := range []string{"b", "c"} {
		b.trees[tree] = map[string]string{}
		for path, data := range b.trees["a"] {
			b.trees[tree][path] = data
		}
	}
	b.trees["b"]["metasystem/plans/designs/x.md"] = "First design."
	b.trees["c"]["metasystem/plans/designs/x.md"] = "Second design."
	return b
}

// admitted is the admission of the newest launch.
func (b *flakeMoveBed) admitted(t *testing.T) *ExecutionAdmission {
	t.Helper()
	running, recorded, _, err := ReadRunning(b.install, b.seams)
	if err != nil || !recorded || running.Admission == nil {
		t.Fatalf("no admitted launch: %+v recorded=%v error=%v", running, recorded, err)
	}
	return running.Admission
}

// proven writes the row the detached prover writes for a scoped check of
// main's tree, then ends the run as the prover does.
func (b *flakeMoveBed) proven(t *testing.T, tree string, owedSince time.Time) {
	t.Helper()
	admission := b.admitted(t)
	if admission.Scope != "scoped" || admission.Tree != "tree-"+tree {
		t.Fatalf("the launch to prove is %+v, want a scoped check of tree-%s", admission, tree)
	}
	r := b.base
	r.Commit, r.Tree, r.Scope, r.ScopeReason, r.Base = "commit-"+tree, "tree-"+tree, "scoped", admission.Reason, "tree-a"
	r.Attempt, r.At, r.OwedSince = admission.Attempt, b.at.Format(time.RFC3339), owedSince
	b.save(t, r)
	if err := removeRunning(b.install, b.seams); err != nil {
		t.Fatal(err)
	}
}

func (b *flakeMoveBed) clockReason(t *testing.T) string {
	t.Helper()
	state, err := readClock(b.install)
	if err != nil {
		t.Fatal(err)
	}
	return state.Reason
}

func TestARecordsOnlyMoveKeepsMainsFullGreen(t *testing.T) {
	t.Parallel()
	kept := fmt.Sprintf("main moved by records only; 2 checks run, the rest holds from the %s check", bedNow.Local().Format("15:04"))
	debt := func(at time.Time) string {
		return "the hourly debt of the " + at.Local().Format("15:04") + " check is due"
	}
	// designMove runs main's first records move through the keeper and holds
	// main's green past the hour; it leaves the bed at 1h+1s with one launch.
	designMove := func(t *testing.T, b *flakeMoveBed) {
		t.Helper()
		b.move("b", 5*time.Minute)
		b.checkWake(t, 1, false)
		if a := b.admitted(t); a.Scope != "scoped" || a.Reason != kept {
			t.Fatalf("records move admission: %+v, want scoped with %q", a, kept)
		}
		b.proven(t, "b", time.Time{})
		b.at = bedNow.Add(time.Hour + time.Second)
		b.checkWake(t, 1, false)
	}
	t.Run("a design page", func(t *testing.T) {
		t.Parallel()
		b := newRecordsMoveBed(t)
		designMove(t, b)
		if due, err := scopedProofDue(b.install, b.at); err != nil || !due {
			t.Fatalf("the shared hourly debt is gone after main's records check: %v %v", due, err)
		}
		d := proofScope(b.install, b.checkout, Running{BatchID: "cheap", Commit: "commit-candidate", Tree: "tree-candidate"}, b.seams)
		if d.Scope != "full" || !strings.HasPrefix(d.ScopeReason, "full proof overdue since") {
			t.Fatalf("a cheap batch after main's records check: %+v", d.scopeRecord)
		}
		b.at = bedNow.Add(3*time.Hour + 59*time.Minute)
		b.checkWake(t, 1, false)
		b.at = bedNow.Add(4 * time.Hour)
		b.checkWake(t, 2, true)
		if last := b.argv[len(b.argv)-1]; !slices.Contains(last, "--trunk") || !slices.Contains(last, "--fresh") {
			t.Fatalf("four-hour launch argv=%v", last)
		}
	})
	t.Run("a second records move after the hour", func(t *testing.T) {
		t.Parallel()
		b := newRecordsMoveBed(t)
		designMove(t, b)
		b.move("c", 2*time.Hour)
		b.checkWake(t, 2, false)
		if a := b.admitted(t); a.Scope != "scoped" || a.Reason != kept {
			t.Fatalf("second records move admission: %+v, want scoped with %q", a, kept)
		}
	})
	t.Run("an executable change after the hour", func(t *testing.T) {
		t.Parallel()
		b := newRecordsMoveBed(t)
		b.trees["c"]["metasystem/internal/x/x.go"] = "package x"
		designMove(t, b)
		b.move("c", 2*time.Hour)
		b.wake(t)
		if a := b.admitted(t); b.trunks != 2 || a.Scope != "full" || a.Reason != "an executable input changed: metasystem/internal/x/x.go" {
			t.Fatalf("executable change admission: launches=%d %+v", b.trunks, a)
		}
	})
	t.Run("a standing debt is not kept", func(t *testing.T) {
		t.Parallel()
		b := newRecordsMoveBed(t)
		// Main's newer full green keeps the standing row current for the
		// observation, so only the debt owed since 5 minutes is due.
		b.base.Attempt, b.base.At = "full-a-again", bedNow.Add(20*time.Minute).Format(time.RFC3339)
		b.base.FullAt = b.base.At
		b.save(t, b.base)
		b.move("b", 25*time.Minute)
		b.checkWake(t, 1, false)
		owed := bedNow.Add(5 * time.Minute)
		b.proven(t, "b", owed)
		b.at = owed.Add(61 * time.Minute)
		b.checkWake(t, 2, true)
		reasons, err := WakeReasons(b.install, b.checkout, time.Time{}, b.at, b.seams)
		if err != nil || !slices.Contains(reasons, WakeFullDue) {
			t.Fatalf("a standing debt was kept: reasons=%v error=%v", reasons, err)
		}
		if last := b.argv[len(b.argv)-1]; !slices.Contains(last, "--fresh") || b.clockReason(t) != debt(owed) {
			t.Fatalf("the keeper did not pay the standing debt: argv=%v clock=%q", last, b.clockReason(t))
		}
	})
	t.Run("a scoped push, then main's records check", func(t *testing.T) {
		t.Parallel()
		b := newRecordsMoveBed(t)
		b.at = bedNow.Add(10 * time.Minute)
		push := b.base
		push.Trunk, push.Commit, push.Tree, push.Scope, push.Attempt = false, "commit-b", "tree-b", "scoped", "batch-b"
		push.At = b.at.Format(time.RFC3339)
		b.save(t, push)
		writePushRecords(t, b.install, Pushed{Commit: push.Commit, Tree: push.Tree, At: push.At, BatchID: "batch-b"})
		b.move("c", 20*time.Minute)
		b.checkWake(t, 1, false)
		b.proven(t, "c", time.Time{})
		b.at = bedNow.Add(61 * time.Minute)
		b.checkWake(t, 2, true)
		if last := b.argv[len(b.argv)-1]; !slices.Contains(last, "--fresh") || b.clockReason(t) != debt(bedNow) {
			t.Fatalf("main's records check hid the push's debt: argv=%v clock=%q", last, b.clockReason(t))
		}
	})
}
