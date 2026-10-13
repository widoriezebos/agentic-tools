package plain

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/state"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
)

func TestLaneTornShadowNeverFailsCompletedWriters(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	s := shadowSeams()
	s.BudgetHome = b.home
	s.AgentRunning = func() (bool, error) { return true, nil }
	if _, err := RefreshShadow(b.home, bedNow, "keeper", s); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(Dir(b.install), "state.json"), []byte(`{torn`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, added, err := HandInChecked(b.install, Line{Goal: "A", SHA: "tip", At: bedNow.Format(time.RFC3339Nano)}, "", s); err != nil || !added {
		t.Fatalf("hand-in: added=%v err=%v", added, err)
	}
	finish, err := BeginAgentStep(b.install, "prove", nil, bedNow, s)
	if err != nil || finish == nil {
		t.Fatalf("step: finish=%v err=%v", finish == nil, err)
	}
	if err := finish(bedNow.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(Dir(b.install), "agent-step.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("step not ended: %v", err)
	}
	events := shadowEvents(t, b.install)
	if events[len(events)-1].Kind != "observation-failed" {
		t.Fatalf("state failure lost: %+v", events)
	}
	if _, err := RefreshShadow(b.home, bedNow.Add(time.Minute), "keeper", s); err != nil {
		t.Fatal(err)
	}
	if _, present, err := state.Read(Dir(b.install)); err != nil || !present {
		t.Fatalf("repair: %v %v", present, err)
	}
}

func TestLaneRunningBatchSelectionEmitsNoEmptyEvents(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	s := shadowSeams()
	s.BudgetHome = b.home
	s.Alive = func(Running) bool { return true }
	s.Lane = func() (lane.Record, error) { return b.record, nil }
	b.lines("queue.jsonl", Line{Goal: "A", SHA: "tip", At: bedNow.Format(time.RFC3339Nano)})
	b.lines("batch.json", Batch{ID: "gate-batch", Lane: b.record, Base: "main", State: BatchRunning, Open: true, Members: []GoalSHA{{Goal: "A", SHA: "tip"}}, CreatedAt: bedNow.Format(time.RFC3339Nano), RunningAt: bedNow.Format(time.RFC3339Nano)})
	b.lines("running.json", Running{Attempt: "gate", Gate: true, BatchID: "gate-batch", Pid: 42, Since: bedNow.Format(time.RFC3339Nano)})
	if _, err := RefreshShadow(b.home, bedNow, "keeper", s); err != nil {
		t.Fatal(err)
	}
	before := len(shadowEvents(t, b.install))
	old, _, _ := state.Read(Dir(b.install))
	for range 3 {
		if _, err := SelectBatch(b.install, b.checkout, b.record, s); err != nil {
			t.Fatal(err)
		}
	}
	next, _, _ := state.Read(Dir(b.install))
	if got := len(shadowEvents(t, b.install)) - before; got != 0 || old.Version != next.Version {
		t.Fatalf("repeated selection: events=%d versions=%d/%d", got, old.Version, next.Version)
	}
}

func TestLaneHandInHoldsStateLockAcrossObservations(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	s := shadowSeams()
	s.BudgetHome = b.home
	if _, err := RefreshShadow(b.home, bedNow, "keeper", s); err != nil {
		t.Fatal(err)
	}
	before := len(shadowEvents(t, b.install))
	observations := 0
	var refreshDone chan error
	s.Observed = func() {
		observations++
		held, err := lock.File(filepath.Join(Dir(b.install), "state.lock"), 0600, lock.TryExclusive)
		if held != nil {
			_ = held.Release()
			t.Error("observation did not hold state.lock")
		}
		if !lock.Busy(err) {
			t.Errorf("state lock: %v", err)
			return
		}
		if observations == 2 {
			refreshDone = make(chan error, 1)
			refresh := shadowSeams()
			attempted := make(chan struct{}, 1)
			refresh.BeforeShadowPublish = func() {
				select {
				case attempted <- struct{}{}:
				default:
				}
			}
			go func() { _, err := RefreshShadow(b.home, bedNow, "keeper", refresh); refreshDone <- err }()
			<-attempted
		}
	}
	if _, _, err := HandInChecked(b.install, Line{Goal: "A", SHA: "tip", At: bedNow.Format(time.RFC3339Nano)}, "", s); err != nil {
		t.Fatal(err)
	}
	if refreshDone != nil {
		if err := <-refreshDone; err != nil {
			t.Fatal(err)
		}
	}
	events := shadowEvents(t, b.install)[before:]
	if observations != 2 || len(events) != 1 || events[0].Kind != "hand_ins" || events[0].By != "hand-in" {
		t.Fatalf("hand-in events: observations=%d %+v", observations, events)
	}
}

func TestLaneCompletedWriteSurvivesStateFailures(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"state.lock", "state.json", "events.jsonl"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b := newStatusBed(t)
			s := shadowSeams()
			s.BudgetHome = b.home
			if _, err := RefreshShadow(b.home, bedNow, "keeper", s); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(Dir(b.install), name)
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(path, 0700); err != nil {
				t.Fatal(err)
			}
			if _, added, err := HandInChecked(b.install, Line{Goal: "A", SHA: "tip", At: bedNow.Format(time.RFC3339Nano)}, "", s); err != nil || !added {
				t.Fatalf("completed hand-in: %v %v", added, err)
			}
			entries, err := Entries(b.install)
			if err != nil || len(entries) != 1 {
				t.Fatalf("raw hand-in: %+v %v", entries, err)
			}
			if name == "events.jsonl" {
				status := ObserveStatus(b.home, b.record, s, nil)
				if !slices.ContainsFunc(status.Findings, func(line string) bool { return strings.Contains(line, "the lane transition could not be recorded:") }) {
					t.Fatalf("lost failure finding: %+v", status.Findings)
				}
			} else {
				events := shadowEvents(t, b.install)
				if events[len(events)-1].Kind != "observation-failed" {
					t.Fatalf("failure event: %+v", events)
				}
			}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
		})
	}
}
