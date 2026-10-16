package plain

import (
	"errors"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/state"
)

func TestClosedBatchPathsLeaveCurrentStatus(t *testing.T) {
	t.Parallel()
	for _, path := range []string{"confirmed push", "recorded return", "external landing", "supersession", "deferral"} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			b := newStatusBed(t)
			s := shadowSeams()
			policyEnd := errors.New("stop before selecting the next batch")
			s.Policy = func(string) (PolicyValue, error) { return PolicyValue{}, policyEnd }
			notAncestor := exec.Command("/usr/bin/false").Run()
			original := s.Git
			s.Git = func(dir string, args ...string) (string, error) {
				if args[0] == "merge-base" {
					if path == "external landing" {
						return "", nil
					}
					return "", notAncestor
				}
				if args[0] == "fetch" || args[0] == "cat-file" {
					return "", nil
				}
				return original(dir, args...)
			}
			stamp := bedNow.Add(-time.Minute).Format(time.RFC3339Nano)
			member := GoalSHA{Goal: "goal", SHA: "tip"}
			batch := &Batch{ID: "selected", Lane: b.record, Base: "main", State: BatchRunning, SealedAt: stamp, CreatedAt: stamp, Waiting: []string{"closed-batch wait"}, Members: []GoalSHA{member}}
			b.lines("queue.jsonl", Line{Goal: member.Goal, SHA: member.SHA, At: stamp})
			if err := writeBatch(b.install, batch); err != nil {
				t.Fatal(err)
			}
			switch path {
			case "confirmed push":
				if err := completePush(b.install, b.checkout, batch, "main", "head", "tree", bedNow, s); err != nil {
					t.Fatal(err)
				}
			case "recorded return":
				b.lines("queue.jsonl", Line{Goal: member.Goal, SHA: member.SHA, At: stamp}, Line{Goal: member.Goal, SHA: member.SHA, Outcome: StateReturned, At: bedNow.Format(time.RFC3339Nano), Reason: "repair"})
			case "supersession":
				b.lines("queue.jsonl", Line{Goal: member.Goal, SHA: member.SHA, At: stamp}, Line{Goal: member.Goal, SHA: "new-tip", At: bedNow.Format(time.RFC3339Nano)})
			case "deferral":
				b.lines("queue.jsonl", Line{Goal: member.Goal, SHA: member.SHA, At: stamp}, Line{Goal: member.Goal, SHA: member.SHA, Outcome: StateWaiting, Held: true, After: []GoalSHA{{Goal: "prerequisite", SHA: "prerequisite-tip"}}, At: bedNow.Format(time.RFC3339Nano)})
			}
			if path != "confirmed push" {
				if _, err := SelectBatch(b.install, b.checkout, b.record, s); err == nil || !strings.Contains(err.Error(), policyEnd.Error()) {
					t.Fatal(err)
				}
			}
			s.Policy = nil
			retained, err := ReadBatch(b.install)
			if err != nil || retained == nil || retained.State != BatchClosed || retained.ClosedAt != bedNow.Format(time.RFC3339Nano) || !slices.Equal(retained.Members, []GoalSHA{member}) || retained.SealedAt != stamp {
				t.Fatalf("retained closure: %+v %v", retained, err)
			}
			status := ReadStatus(b.home, b.record, b.view, s)
			shadow := ShadowOf(status, status.Raw)
			if shadow.Batch.ID != "" || strings.Contains(status.Summary, "batch selected") || strings.Contains(status.Summary, "batch sealed") || strings.Contains(status.Summary, "closed-batch wait") {
				t.Fatalf("closed batch remained current: %+v %+v %s", status.Batch, shadow.Batch, status.Summary)
			}
			due, _ := state.LifecycleDue("batch", state.LifecycleFacts{Word: BatchClosed, Empty: false}, bedNow)
			if !due {
				t.Fatal("closed batch with members never retires")
			}
			if err := RefreshLifecycle(b.home, bedNow, s); err != nil {
				t.Fatal(err)
			}
			after, err := ReadBatch(b.install)
			if err != nil || !slices.Equal(after.Members, retained.Members) || after.ClosedAt != retained.ClosedAt {
				t.Fatalf("retirement changed history: %+v %v", after, err)
			}
		})
	}
}

func TestReadStatusQueueHistoryPreservesOutcomes(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	old := bedNow.Add(-72 * time.Hour).Format(time.RFC3339)
	returned := bedNow.Add(-48 * time.Hour).Format(time.RFC3339)
	recent := bedNow.Add(-time.Hour).Format(time.RFC3339)
	b.lines("queue.jsonl",
		Line{Goal: "unresolved", SHA: "u", At: old}, Line{Goal: "unresolved", SHA: "u", Outcome: StateReturned, At: returned},
		Line{Goal: "same", SHA: "s", At: old}, Line{Goal: "same", SHA: "s", Outcome: StateReturned, At: returned}, Line{Goal: "same", SHA: "s", At: recent},
		Line{Goal: "changed", SHA: "c1", At: old}, Line{Goal: "changed", SHA: "c1", Outcome: StateReturned, At: returned}, Line{Goal: "changed", SHA: "c2", At: recent},
		Line{Goal: "old-landing", SHA: "l1", At: old}, Line{Goal: "recent-landing", SHA: "l2", At: old}, Line{Goal: "external", SHA: "e", At: old})
	b.lines("pushes.jsonl", Pushed{Old: "m0", Commit: "m1", At: returned}, Pushed{Old: "m1", Commit: "m2", At: recent})
	status := b.read(ProveSeams{Now: func() time.Time { return bedNow }}, b.git([]string{"l1", "l2", "e"}, map[string][]string{"m0..m1": {"l1"}, "m1..m2": {"l2"}}))
	queue, history := QueueHistory(status.Queue)
	if len(status.Problems) > 0 || len(queue) != 3 || len(history) != 5 || queue[0].Goal != "unresolved" || queue[0].ReturnedAt != returned || queue[1].SHA != "s" || queue[2].SHA != "c2" {
		t.Fatalf("queue=%+v history=%+v problems=%v", queue, history, status.Problems)
	}
	for _, e := range history {
		want := returned
		if e.Goal == "recent-landing" {
			want = recent
		}
		if e.Goal == "external" {
			want = ""
		}
		got := e.LandedAt
		if e.State == StateReturned {
			got = e.ReturnedAt
		}
		if got != want {
			t.Errorf("event time %s: %q want %q", e.Goal, got, want)
		}
	}
}
