package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

func repairCLIFixture(t *testing.T) (*selectionBed, plain.ProveSeams) {
	t.Helper()
	b := newSelectionBed(t)
	// Repair replacement requires main to lack applicable proof.
	helmMust(t, os.Remove(filepath.Join(plain.Dir(b.lane), "results.jsonl")))
	b.owners.prove = enrolledPersonProver(t, b.lane, b.now)
	s := b.owners.landing.plainProve
	git := s.Git
	missing := exec.Command("/bin/sh", "-c", "exit 128")
	_ = missing.Run()
	s.Git = func(dir string, args ...string) (string, error) {
		if strings.Contains(strings.Join(args, " "), "MERGE_HEAD") {
			return "", &exec.ExitError{ProcessState: missing.ProcessState}
		}
		if args[0] == "show" {
			return "proof.full=fixture-proof\n", nil
		}
		return git(dir, args...)
	}
	s.Incidents = func(string, string, string) ([]goal.TrunkRedEntry, error) {
		return []goal.TrunkRedEntry{{Identity: "red", Opened: b.now.Format(time.RFC3339)}}, nil
	}
	s.Executable = func() (string, error) { return "fixture-engine", nil }
	s.Launch = func(_ []string, _ string, log string) (int64, error) {
		helmMust(t, os.WriteFile(log, nil, 0600))
		return int64(os.Getpid()), nil
	}
	return b, s
}

func overrideEntries(t *testing.T, install string) []plain.Intervention {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(plain.Dir(install), "interventions.jsonl"))
	helmMust(t, err)
	var entries []plain.Intervention
	for _, line := range bytes.Split(bytes.TrimSpace(data), []byte{'\n'}) {
		var entry plain.Intervention
		helmMust(t, json.Unmarshal(line, &entry))
		entries = append(entries, entry)
	}
	return entries
}

type repairStderrFailure struct{}

func (repairStderrFailure) Write([]byte) (int, error) { return 0, errors.New("stderr unavailable") }

func TestLandingResumeAndTrunkRepairOverrideRequiresVisibleNotice(t *testing.T) {
	t.Parallel()
	for _, route := range []string{"prove", "resume"} {
		t.Run(route, func(t *testing.T) {
			t.Parallel()
			b, s := repairCLIFixture(t)
			checkpoint := plain.Running{Checkpoint: true, Attempt: "candidate", Tree: "candidate-tree", Commit: "candidate"}
			writeQuestionFixture(t, filepath.Join(plain.Dir(b.lane), "running.json"), checkpoint)
			writeCauseProof(t, b.lane, "results.jsonl", plain.Result{Attempt: checkpoint.Attempt, Tree: checkpoint.Tree, Commit: checkpoint.Commit, Result: plain.Red})
			helmMust(t, plain.WriteFix(b.lane, &plain.Fix{Attempt: checkpoint.Attempt, Goal: "repair", Units: []string{"unit"}, Job: "job", Commit: "candidate", State: "building"}))
			s.NotifyRepairOverride = func(string) error { t.Fatal("observer called after failed stderr write"); return nil }
			s.Launch = func([]string, string, string) (int64, error) {
				t.Fatal("launch after failed stderr write")
				return 0, nil
			}
			b.owners.landing.plainProve = s
			args := []string{}
			if route == "prove" {
				args = append(args, "--trunk")
			}
			var stdout bytes.Buffer
			code := runIntentIn(mustIntentCommand(t, "landing "+route), args, &stdout, repairStderrFailure{}, b.lane, b.owners)
			stored, present, _, err := plain.ReadRunning(b.lane, s)
			fix, fixErr := plain.FixForAttempt(b.lane, checkpoint.Attempt)
			if code == 0 || err != nil || fixErr != nil || !present || !reflect.DeepEqual(stored, checkpoint) || fix == nil || fix.State != "building" {
				t.Fatalf("exit=%d running=%+v fix=%+v errors=%v/%v stdout=%s", code, stored, fix, err, fixErr, &stdout)
			}
			entries := overrideEntries(t, b.lane)
			if len(entries) != 2 || entries[1].Outcome != "fix not closed, checkpoint retained, launch not attempted" {
				t.Fatalf("effects: %+v", entries)
			}
			for _, entry := range entries {
				if entry.Person == nil || entry.Person.Person != "Wido" || !strings.Contains(entry.What, "tree candidate-tr (fix repair)") {
					t.Fatalf("lost override request: %+v", entry)
				}
			}
		})
	}
}

func TestLandingTrunkFixOnlyOverrideDescribesLifecycle(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"unmatched ordinary", "unmatched merge", "newest green", "older green newer red", "ordinary running"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			b, s := repairCLIFixture(t)
			fix := &plain.Fix{Attempt: "fix-attempt", Goal: "repair", Units: []string{"unit"}, Job: "job", Commit: "candidate", State: "building"}
			want := "this check supersedes the pending repair of repair; its record closes at the next lifecycle refresh"
			if scenario == "unmatched merge" || scenario == "older green newer red" {
				fix.Tip = "merge-tip"
				want = "this check runs beside the pending fix of repair; the fix stays open"
			}
			if scenario == "newest green" || scenario == "older green newer red" {
				results := []plain.Result{{Attempt: fix.Attempt, Result: plain.Green, Commit: "candidate", Tree: "candidate-tree"}}
				if scenario == "newest green" {
					want = "the pending fix of repair is closed by this check at Wido's request"
				} else {
					results = append(results, plain.Result{Attempt: "newer-red", Result: plain.Red, Commit: "candidate", Tree: "candidate-tree"})
				}
				writeCauseProof(t, b.lane, "results.jsonl", results...)
			}
			if scenario == "ordinary running" {
				// A failed non-checkpoint attempt still owns the matching fix.
				want = "the pending fix of repair is closed by this check at Wido's request"
				writeQuestionFixture(t, filepath.Join(plain.Dir(b.lane), "running.json"), plain.Running{Attempt: fix.Attempt, Tree: "ordinary-tree", Admission: &plain.ExecutionAdmission{State: "failed"}})
			}
			helmMust(t, plain.WriteFix(b.lane, fix))
			s.NotifyRepairOverride = func(impact string) error {
				if impact != want || strings.Contains(impact, "tree") {
					t.Fatalf("impact=%q want=%q", impact, want)
				}
				return nil
			}
			b.owners.landing.plainProve = s
			code, output := b.run(t, b.lane, "landing", "prove", "--trunk")
			if code != 0 || !strings.Contains(output, want) {
				t.Fatalf("exit=%d output=%s", code, output)
			}
			immediate, err := plain.FixForAttempt(b.lane, fix.Attempt)
			closed := scenario == "newest green" || scenario == "ordinary running"
			if err != nil || immediate == nil || (immediate.State == "closed") != closed {
				t.Fatalf("immediate fix effect: %+v closed=%v err=%v", immediate, closed, err)
			}
			retained := fix.Tip != ""
			check := func() {
				t.Helper()
				active, err := plain.ReadFix(b.lane)
				if err != nil || (active != nil) != retained {
					t.Fatalf("ReadFix=%+v retained=%v err=%v", active, retained, err)
				}
			}
			check()
			helmMust(t, plain.RefreshLifecycle(b.home, b.now, s))
			check()
			stored, err := plain.FixForAttempt(b.lane, fix.Attempt)
			if err != nil || stored == nil || (stored.State == "closed") == retained {
				t.Fatalf("after refresh: %+v err=%v", stored, err)
			}
			entries := overrideEntries(t, b.lane)
			if len(entries) != 2 || entries[1].What != want {
				t.Fatalf("request lost: %+v", entries)
			}
		})
	}
}
