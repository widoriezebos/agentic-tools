package plain

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/state"
)

func TestRunningProofRefreshDoesNotPublishRenderProgress(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	shadowRegister(t, b.home, b.record)
	b.lines("running.json", Running{Attempt: "proof", Commit: "head", Tree: "tree", Trunk: true, Pid: 42, Since: bedNow.Format(time.RFC3339Nano)})
	seams := shadowSeams()
	seams.Alive = func(Running) bool { return true }
	if _, err := RefreshShadow(b.home, bedNow, "fixture", seams); err != nil {
		t.Fatal(err)
	}
	record, _, err := state.Read(Dir(b.install))
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(Dir(b.install), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	events, err := os.ReadFile(filepath.Join(Dir(b.install), "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	later := bedNow.Add(5 * time.Minute)
	changed, err := RefreshShadow(b.home, later, "fixture", seams)
	if err != nil || len(changed) != 0 {
		t.Fatalf("refresh %v: %v", changed, err)
	}
	after, _, err := state.Read(Dir(b.install))
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(Dir(b.install), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	rows, err := os.ReadFile(filepath.Join(Dir(b.install), "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if after.Version != record.Version || !bytes.Equal(before, data) || !bytes.Equal(events, rows) {
		t.Fatalf("refresh published a render change: %d -> %d", record.Version, after.Version)
	}
	seams.Now = func() time.Time { return later }
	status := ObserveStatus(b.home, b.record, seams, nil)
	if healthRow(status.Health, "drift") != nil {
		t.Fatalf("elapsed minutes became drift: %+v", status.Health.Rows())
	}
	if status.RunningProof == nil || status.RunningProof.Minutes == nil || *status.RunningProof.Minutes != 5 {
		t.Fatalf("render lost elapsed minutes: %+v", status.RunningProof)
	}
}

func TestLandingHealthTextKeepsMeasuredFacts(t *testing.T) {
	t.Parallel()
	minutes := 17.0
	h := Health{State: "proving", Goals: []GoalHealth{{Goal: "G", State: StateReturned, Minutes: &minutes, Reason: "own failure"}}, Batch: &BatchHealth{ID: "batch-x", State: BatchRunning, Members: []GoalSHA{{Goal: "G"}}, Minutes: &minutes, Waiting: []BatchArrival{{Goal: "H", Boundary: "after seal", Admission: "next batch", Reason: "sealed"}}}, Agent: AgentHealth{Alive: true, Verb: "prove", Minutes: &minutes, WaitingOn: "checks"}, Paused: &PauseHealth{Reason: "inspect main", Minutes: &minutes}, Questions: []QuestionHealth{{ID: "q", Premise: "current", Minutes: 23, WaitsFor: "main at the recorded commit"}}}
	text := h.Text()
	for _, want := range []string{"goals:\n  G: returned (17 min); own failure", "batch:\n  batch-x: running (17 min); members: G; admission: sealed", "H: next batch", "boundary: after seal", "agent:\n  alive true", "last step: prove; waiting-on: checks", "paused:\n  lane: paused (17 min); inspect main", "questions:\n  q: current (23 min); main at the recorded commit"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q: %s", want, text)
		}
	}
	empty := (Health{}).Text()
	for _, header := range []string{"goals:", "batch:", "paused:", "questions:", "incidents:", "interventions:"} {
		if strings.Contains(empty, header) {
			t.Errorf("empty header %q: %s", header, empty)
		}
	}
}

func TestProofBudgetRefusalDistinguishesUnenrolledCallerFromAgent(t *testing.T) {
	t.Parallel()
	for _, agent := range []bool{true, false} {
		t.Run(map[bool]string{true: "agent", false: "unenrolled terminal"}[agent], func(t *testing.T) {
			t.Parallel()
			install := t.TempDir()
			goals := []GoalSHA{{Goal: "G", SHA: "sha"}}
			if _, _, err := HandIn(install, Line{Goal: "G", SHA: "sha"}); err != nil {
				t.Fatal(err)
			}
			for _, attempt := range []string{"one", "two"} {
				if err := appendLine(resultsPath(install), Result{Attempt: attempt, Commit: attempt, Tree: attempt, Result: Red, Goals: goals, CountedFull: true, BatchBase: "base"}); err != nil {
					t.Fatal(err)
				}
			}
			seams := ProveSeams{Now: func() time.Time { return bedNow }, Git: func(string, ...string) (string, error) { return "", nil }}
			if !agent {
				seams.CallerRefusal = "this terminal is not enrolled for the landing install; type: metasystem system enroll --name NAME"
			}
			running := Running{Attempt: "three", Commit: "three", Tree: "tree", BatchMembers: goals, BatchBase: "base"}
			err := admitExecutionLocked(install, "checkout", "proof", &running, scopeDecision{scopeRecord: scopeRecord{Scope: "full"}}, seams)
			var refused *Refusal
			if !errors.As(err, &refused) || refused.Code != "LANE_PROOF_BUDGET" {
				t.Fatalf("refusal: %v", err)
			}
			stops, err := OpenStops(install)
			if err != nil {
				t.Fatal(err)
			}
			if err := SyncPolicyQuestion(install, func(string) (string, error) { return "fixture", nil }, bedNow, seams); err != nil {
				t.Fatal(err)
			}
			questions, unreadable := channel.WalkOpenQuestions(install)
			if len(unreadable) != 0 || agent && len(questions) != 1 || !agent && len(questions) != 0 {
				t.Fatalf("budget question: %v unreadable %v", questions, unreadable)
			}
			if agent {
				if !strings.Contains(refused.Reason, "a person decides") || len(stops) != 1 {
					t.Fatalf("agent refusal: %+v stops %+v", refused, stops)
				}
			} else if refused.Reason != seams.CallerRefusal || refused.Next != "" || len(stops) != 0 {
				t.Fatalf("enrollment refusal: %+v stops %+v", refused, stops)
			}
		})
	}
}

func TestShadowRedTrunkResultNeverPublishesFlakes(t *testing.T) {
	t.Parallel()
	for _, result := range []string{Red, Green} {
		t.Run(result, func(t *testing.T) {
			t.Parallel()
			root := "synthetic checkout"
			proof := Result{Trunk: true, Commit: "main", Result: result, Flakes: []FlakeRecord{{FailedUnit: FailedUnit{Tests: []string{"TestUnstable"}}}}}
			got := ShadowOf(Status{View: lane.View{Root: &root}, LastProof: &proof}, RawFacts{Now: bedNow, Main: "main", Results: []Result{proof}})
			if got.Main.Word != result || (got.Main.Flakes != "") != (result == Green) {
				t.Fatalf("main: %+v", got.Main)
			}
		})
	}
}
