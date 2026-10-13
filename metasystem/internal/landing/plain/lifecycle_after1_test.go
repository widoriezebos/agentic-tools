package plain

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/hostload"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/state"
)

func TestKeeperRetirementPreservesRecordsWrittenAfterObservation(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"agent-step", "batch"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			b := newStewardBed(t)
			b.seams.Alive = func(Running) bool { return false }
			at := bedNow.Add(-time.Hour).Format(time.RFC3339Nano)
			b.lines("agent-step.json", AgentStep{Verb: "landing prove", StartedAt: at, PID: 42})
			b.lines("batch.json", Batch{ID: "batch", State: BatchClosed, ClosedAt: at, Base: "main", Lane: b.record})
			before := ObserveStatus(b.home, b.record, b.seams, nil)
			if len(before.Problems) != 0 {
				t.Fatal(before.Problems)
			}
			name := "agent-step.json"
			if kind == "agent-step" {
				b.lines(name, AgentStep{Verb: "landing push", StartedAt: bedNow.Format(time.RFC3339Nano), PID: 43})
			} else {
				name = "batch.json"
				members := []GoalSHA{{Goal: "g", SHA: "sha"}}
				person := &ActProvenance{Kind: "selection", Person: "person", Root: b.checkout, CheckedAt: bedNow, TerminalGeneration: 1, TerminalRef: humanauthority.ProcessRef{PID: 1}, Destination: b.record, Subject: members}
				b.lines(name, Batch{ID: "person-selection", State: BatchPrepared, Base: "main", Members: members, Lane: b.record, Person: person})
			}
			path := filepath.Join(Dir(b.install), name)
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := retireLifecycle(b.install, before, bedNow, b.seams, kind); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(got, want) {
				t.Fatalf("new record changed: %s %v", got, err)
			}
		})
	}
}

func TestKeeperAbandonedFixWaitsForItsSubject(t *testing.T) {
	t.Parallel()
	for _, outcome := range []string{StateLanded, StateReturned, StateSuperseded} {
		t.Run(outcome, func(t *testing.T) {
			t.Parallel()
			b := newStewardBed(t)
			fix := Fix{Attempt: "repair", State: "abandoned", Goal: "g", Tip: "sha", Commit: "main", Reason: "its resolution was abandoned"}
			if err := WriteFix(b.install, &fix); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(Dir(b.install), "fixes", "repair.json")
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			status := Status{Queue: []Entry{{Goal: "g", SHA: "sha", State: StateWaiting}}}
			for tick := range 10 {
				if err := retireLifecycle(b.install, status, bedNow.Add(time.Duration(tick)*time.Minute), b.seams, "fix"); err != nil {
					t.Fatal(err)
				}
				got, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(got, want) {
					t.Fatalf("tick %d lost abandonment: %s %v", tick, got, err)
				}
			}
			status.Queue[0].State = outcome
			if err := retireLifecycle(b.install, status, bedNow, b.seams, "fix"); err != nil {
				t.Fatal(err)
			}
			got, err := FixForAttempt(b.install, "repair")
			if err != nil || got.State != "closed" || got.Reason != fix.Reason {
				t.Fatalf("subject ended: %+v %v", got, err)
			}
		})
	}
}

func TestKeeperAbandonedFixRemainsVisibleAcrossTenTicks(t *testing.T) {
	t.Parallel()
	b := newStewardBed(t)
	landed := false
	notAncestor := exec.CommandContext(t.Context(), "/usr/bin/false").Run()
	git := b.seams.Git
	b.seams.Git = func(dir string, args ...string) (string, error) {
		if args[0] == "merge-base" {
			if landed {
				return "", nil
			}
			return "", notAncestor
		}
		return git(dir, args...)
	}
	at := bedNow.Format(time.RFC3339Nano)
	b.lines("queue.jsonl", Line{Goal: "g", SHA: "sha", At: at})
	fix := Fix{Attempt: "repair", Goal: "g", Tip: "sha", State: "abandoned", Reason: "its resolution was abandoned"}
	if err := WriteFix(b.install, &fix); err != nil {
		t.Fatal(err)
	}
	for tick := range 10 {
		if err := RefreshLifecycle(b.home, bedNow.Add(time.Duration(tick)*time.Minute), b.seams); err != nil {
			t.Fatal(err)
		}
		status := ObserveStatus(b.home, b.record, b.seams, nil)
		if len(status.Problems) != 0 || status.Raw.Fix.State != "abandoned" || status.Health == nil || status.Health.AbandonedResolution == "" || status.Shadow == nil || status.Shadow.Resolution.Word != "abandoned" {
			t.Fatalf("tick %d hid the abandoned resolution: %+v", tick, status)
		}
	}
	landed = true
	if err := RefreshLifecycle(b.home, bedNow.Add(10*time.Minute), b.seams); err != nil {
		t.Fatal(err)
	}
	got, err := FixForAttempt(b.install, "repair")
	if err != nil || got == nil || got.State != "closed" {
		t.Fatalf("landed repair stayed open: %+v %v", got, err)
	}
}

func TestKeeperRetirementLeavesOnlyEventHistory(t *testing.T) {
	t.Parallel()
	b := newStewardBed(t)
	at := bedNow.Add(-time.Hour).Format(time.RFC3339Nano)
	b.lines("queue.jsonl", Line{Goal: "g", SHA: "sha", At: at}, Line{Goal: "g", SHA: "sha", Outcome: StateReturned, At: at})
	if _, err := RefreshShadow(b.home, bedNow, "fixture", b.seams); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(Dir(b.install), "state.json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	history, err := os.ReadFile(queuePath(b.install))
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := RefreshLifecycle(b.home, bedNow.Add(25*time.Hour), b.seams); err != nil {
			t.Fatal(err)
		}
	}
	after, err := os.ReadFile(path)
	if err != nil || bytes.Contains(after, []byte(`"retired"`)) || len(after) >= len(before) {
		t.Fatalf("history remained in shadow: before=%d after=%s %v", len(before), after, err)
	}
	record, _, err := state.Read(Dir(b.install))
	if err != nil || len(record.HandIns) != 0 {
		t.Fatalf("retired hand-in remained: %+v %v", record.HandIns, err)
	}
	got, err := os.ReadFile(queuePath(b.install))
	if err != nil || !bytes.Equal(got, history) {
		t.Fatalf("queue history changed: %s %v", got, err)
	}
	events, _, err := state.ReadEvents(Dir(b.install))
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, event := range events {
		if event.Kind == "submission" && event.To == "retired" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("retirement events=%d: %+v", count, events)
	}
}

func TestKeeperLatestResultsSurviveRetention(t *testing.T) {
	t.Parallel()
	b := newStewardBed(t)
	at := bedNow.Add(-20 * 24 * time.Hour).Format(time.RFC3339Nano)
	b.lines("results.jsonl", Result{Attempt: "old-trunk", Trunk: true, Commit: "main", At: at, Result: Green}, Result{Attempt: "trunk", Trunk: true, Commit: "main", At: at, Result: Red}, Result{Attempt: "batch", Commit: "batch", At: at, Result: Green})
	b.lines("gates.jsonl", Result{Attempt: "gate", Requested: "main", At: at, Result: Green})
	b.lines("pushes.jsonl", Pushed{Commit: "main", At: at})
	if err := RefreshLifecycle(b.home, bedNow, b.seams); err != nil {
		t.Fatal(err)
	}
	s := ObserveStatus(b.home, b.record, b.seams, nil)
	if s.TrunkProof == nil || s.TrunkProof.Attempt != "trunk" || s.LastProof == nil || s.LastProof.Attempt != "batch" || s.LastGate == nil || s.LastGate.Attempt != "gate" || s.LastPush == nil || s.LastPush.At != at || !strings.Contains(s.ProofHeadline, "main is red") {
		t.Fatalf("latest results disappeared: %+v", s)
	}
	if s.Health == nil || healthRow(s.Health, "overdue-record") != nil {
		t.Fatalf("latest result considered overdue: %+v", s.Health)
	}
	events, _, err := state.ReadEvents(Dir(b.install))
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, event := range events {
		if event.Kind == "result" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("result retirements=%d: %+v", count, events)
	}
}

func TestKeeperRunningProofDoesNotRetireTheLatestResult(t *testing.T) {
	t.Parallel()
	b := newStewardBed(t)
	b.seams.Alive = func(Running) bool { return true }
	b.lines("results.jsonl", Result{Attempt: "trunk", Trunk: true, Commit: "main", At: bedNow.Add(-20 * 24 * time.Hour).Format(time.RFC3339Nano), Result: Red})
	b.lines("running.json", Running{Attempt: "next-trunk", Trunk: true, Commit: "main", Since: bedNow.Format(time.RFC3339Nano), Pid: 42})
	records, err := lifecycleRecords(b.install, Status{}, bedNow, b.seams)
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		if record.kind == "result" {
			t.Fatalf("running proof displaced a result: %+v", record)
		}
	}
}

func TestKeeperLatestTrunkResultStillBoundsSteward(t *testing.T) {
	t.Parallel()
	b := newStewardBed(t)
	b.lines("results.jsonl", Result{Attempt: "trunk", Trunk: true, Commit: "main", At: bedNow.Add(-20 * 24 * time.Hour).Format(time.RFC3339Nano), Result: Red, LoadStart: &hostload.Sample{Available: true, Cores: 8, Load1m: 9}, Failed: []FailedUnit{{Unit: "unit", Tests: []string{"TestRed"}}}})
	if err := RefreshLifecycle(b.home, bedNow, b.seams); err != nil {
		t.Fatal(err)
	}
	run := b.step(bedNow)
	if len(b.calls) != 0 || len(run.Interventions) != 0 || !strings.Contains(run.Line, "bounded: current main already proved") {
		t.Fatalf("old current proof lost the steward bound: %+v calls=%v", run, b.calls)
	}
}
