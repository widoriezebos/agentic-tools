package plain

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/state"
)

func TestKeeperLifecycleRefusesAChangedQueue(t *testing.T) {
	t.Parallel()
	b := newStewardBed(t)
	at := bedNow.Format(time.RFC3339Nano)
	b.lines("stops.jsonl", Stop{Loop: "lane-return", Subject: "g", Tree: "sha", Decision: "stop", At: at})
	// The same commit was handed in again while the keeper waited for the lock.
	b.lines("queue.jsonl", Line{Goal: "g", SHA: "sha", At: at})
	status := Status{Queue: []Entry{{Goal: "g", SHA: "sha", State: StateReturned}}}
	status.Raw.Anchors.Lane = b.record
	status.Raw.Queue = []Line{{Goal: "g", SHA: "sha", Outcome: StateReturned, At: at}}
	err := retireLifecycle(b.install, status, bedNow, b.seams, "")
	var stale *state.Stale
	if !errors.As(err, &stale) {
		t.Fatalf("changed queue accepted: %v", err)
	}
	open, err := OpenStops(b.install)
	if err != nil || len(open) != 1 {
		t.Fatalf("current stop retired: %+v %v", open, err)
	}
	events, _, err := state.ReadEvents(Dir(b.install))
	if err != nil || len(events) != 0 {
		t.Fatalf("stale observation acted: %+v %v", events, err)
	}
}

func TestKeeperLifecycleRecords(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"loop-stop", "person-stop", "question", "fix", "submission", "result", "agent-step", "batch"} {
		for _, due := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/%t", kind, due), func(t *testing.T) {
				t.Parallel()
				b := newStewardBed(t)
				b.seams.AgentProbe = b.seams.Steward.Owner
				b.seams.Alive = func(Running) bool { return !due }
				git := b.seams.Git
				b.seams.Git = func(dir string, args ...string) (string, error) {
					if args[0] == "merge-base" && kind == "person-stop" && due {
						return "", nil
					}
					if args[0] == "merge-base" {
						return "", exec.CommandContext(t.Context(), "/usr/bin/false").Run()
					}
					return git(dir, args...)
				}
				at := bedNow.Add(-time.Hour).Format(time.RFC3339Nano)
				switch kind {
				case "loop-stop", "person-stop":
					loop := "lane-return"
					if kind == "person-stop" {
						loop = "person"
					}
					b.lines("stops.jsonl", Stop{Loop: loop, Subject: "g", Tree: "sha", Decision: "stop", At: at})
					b.lines("queue.jsonl", Line{Goal: "g", SHA: "sha", At: at})
					if due && kind != "person-stop" {
						if err := appendLine(queuePath(b.install), Line{Goal: "g", SHA: "sha", Outcome: StateReturned, At: at}); err != nil {
							t.Fatal(err)
						}
					}
				case "question":
					fact := "May I repair this?"
					if due {
						fact = "The checkout is not on main."
					}
					_, err := channel.Ask(channel.AskRequest{RepoRoot: b.install, About: "lane", Kind: "other", Machine: "landing", Lineage: lane.AgentLineage, Facts: []string{fact}, Now: bedNow.Add(-time.Hour)})
					if err != nil {
						t.Fatal(err)
					}
				case "fix":
					word := "building"
					if due {
						word = "done"
					}
					if err := WriteFix(b.install, &Fix{Attempt: "repair", State: word, Goal: "g", Job: "job", Units: []string{"unit"}}); err != nil {
						t.Fatal(err)
					}
				case "submission":
					if due {
						at = bedNow.Add(-25 * time.Hour).Format(time.RFC3339Nano)
					}
					b.lines("queue.jsonl", Line{Goal: "g", SHA: "sha", At: at}, Line{Goal: "g", SHA: "sha", Outcome: StateReturned, At: at})
				case "result":
					if due {
						at = bedNow.Add(-15 * 24 * time.Hour).Format(time.RFC3339Nano)
					}
					b.lines("results.jsonl", Result{Attempt: "proof", At: at, Result: Green})
					if due {
						if err := appendLine(resultsPath(b.install), Result{Attempt: "latest", At: bedNow.Format(time.RFC3339Nano), Result: Green}); err != nil {
							t.Fatal(err)
						}
					}
				case "agent-step":
					b.lines("agent-step.json", AgentStep{Verb: "landing prove", StartedAt: at, PID: 42})
				case "batch":
					word := BatchPrepared
					if due {
						word = BatchClosed
					}
					b.lines("batch.json", Batch{ID: "batch", State: word, Lane: b.record, Base: "main", CreatedAt: at})
				}
				if _, err := RefreshShadow(b.home, bedNow, "fixture", b.seams); err != nil {
					t.Fatal(err)
				}
				before := ObserveStatus(b.home, b.record, b.seams, nil)
				operational := lifecycleOperationalFiles(t, b.install)
				want := due && kind != "person-stop"
				if before.Health == nil || (healthRow(before.Health, "overdue-record") != nil) != want {
					t.Fatalf("before: %+v problems=%v", before.Health, before.Problems)
				}
				for range 2 {
					if err := RefreshLifecycle(b.home, bedNow, b.seams); err != nil {
						t.Fatal(err)
					}
				}
				after := ObserveStatus(b.home, b.record, b.seams, nil)
				if (!want || kind == "agent-step" || kind == "batch") && !reflect.DeepEqual(operational, lifecycleOperationalFiles(t, b.install)) {
					t.Fatal("a record that was not due changed")
				}
				if after.Health == nil || healthRow(after.Health, "overdue-record") != nil {
					t.Fatalf("after: %+v problems=%v", after.Health, after.Problems)
				}
				events, _, err := state.ReadEvents(Dir(b.install))
				if err != nil {
					t.Fatal(err)
				}
				count := 0
				for _, event := range events {
					if event.Kind == kind {
						count++
						if event.By != "keeper" || event.From != "present" || event.Evidence == "" || event.RecordID == "" {
							t.Fatalf("event: %+v", event)
						}
					}
				}
				if count != map[bool]int{false: 0, true: 1}[want] {
					t.Fatalf("retirement events=%d: %+v", count, events)
				}
				if kind == "submission" || kind == "result" {
					file := "queue.jsonl"
					if kind == "result" {
						file = "results.jsonl"
					}
					data, err := os.ReadFile(filepath.Join(Dir(b.install), file))
					if err != nil || len(data) == 0 {
						t.Fatalf("history lost: %s %v", data, err)
					}
					shadow, _, _ := state.Read(Dir(b.install))
					if kind == "submission" && (len(shadow.HandIns) == 0) != due {
						t.Fatalf("hand-ins: %+v", shadow.HandIns)
					}
					if kind == "result" && (after.LastProof == nil || due && after.LastProof.Attempt != "latest") {
						t.Fatalf("result view: %+v", after.LastProof)
					}
				}
			})
		}
	}
}

func lifecycleOperationalFiles(t *testing.T, install string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(Dir(install), func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || entry.Name() == "state.json" || entry.Name() == "events.jsonl" || strings.HasSuffix(entry.Name(), ".lock") {
			return nil
		}
		data, err := os.ReadFile(path)
		files[path] = string(data)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func TestWithdrawnStopQuestionIsNotReasked(t *testing.T) {
	t.Parallel()
	b := newStewardBed(t)
	stop := Stop{Loop: "lane-proof", Subject: "g", ProofAttempt: "proof", Decision: "stop", Handoff: "ask lane", At: bedNow.Format(time.RFC3339Nano)}
	b.lines("stops.jsonl", stop)
	machine := func(string) (string, error) { return "landing", nil }
	if err := SyncPolicyQuestion(b.install, machine, bedNow); err != nil {
		t.Fatal(err)
	}
	questions, problems := channel.WalkQuestions(b.install)
	if len(problems) != 0 || len(questions) != 1 {
		t.Fatalf("questions=%+v problems=%v", questions, problems)
	}
	if _, err := channel.Withdraw(b.install, questions[0].ID, "person declined", nil, channel.DestinationConfig{}); err != nil {
		t.Fatal(err)
	}
	// Rebuilding the index models an engine restart.
	if err := os.Remove(filepath.Join(Dir(b.install), "stop-question")); err != nil {
		t.Fatal(err)
	}
	stop.Evidence = "new rendering of the same stop"
	b.lines("stops.jsonl", stop)
	for range 2 {
		if err := SyncPolicyQuestion(b.install, machine, bedNow); err != nil {
			t.Fatal(err)
		}
	}
	if err := RefreshLifecycle(b.home, bedNow, b.seams); err != nil {
		t.Fatal(err)
	}
	questions, problems = channel.WalkQuestions(b.install)
	if len(problems) != 0 || len(questions) != 1 || questions[0].State != "closed" {
		t.Fatalf("withdrawal forgotten: %+v %v", questions, problems)
	}
	events, _, err := state.ReadEvents(Dir(b.install))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range events {
		if e.Kind == "question" {
			if !strings.Contains(e.Evidence, "person declined") {
				t.Fatalf("withdrawal decision absent from event: %+v", e)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("keeper did not record the person's withdrawal")
	}
	data, _ := json.Marshal(questions[0])
	if !strings.Contains(string(data), "person declined") {
		t.Fatal("withdrawal decision lost")
	}
}
