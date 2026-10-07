package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

type replayVerbBed struct {
	*resolveVerbFixture
	head  string
	trunk bool
	full  int
	runs  []string
	fail  func(*exec.Cmd, string) (string, error)
}

func newReplayVerbBed(t *testing.T) *replayVerbBed {
	t.Helper()
	b := &replayVerbBed{resolveVerbFixture: newResolveVerbFixture(t), head: "merge-b"}
	if err := os.WriteFile(filepath.Join(b.install, "metasystem.conf"), []byte("proof.full=fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Queue history differs from the assembly's merge order.
	for _, g := range []string{"c", "b", "a"} {
		if _, _, err := plain.HandIn(b.install, plain.Line{Goal: g, SHA: "sha-" + g}); err != nil {
			t.Fatal(err)
		}
	}
	b.owners.landing.keeper = func(home, root string) lane.AgentKeeper {
		return lane.AgentKeeper{Home: home, Self: root, Now: func() time.Time { return laneTestNow }, Running: func() (string, bool, error) { return "", false, nil }}
	}
	falseState := replayFalseState(t)
	id := 0
	b.owners.landing.plainProve = plain.ProveSeams{
		Now:   func() time.Time { return laneTestNow },
		NewID: func() string { id++; return fmt.Sprintf("proof-%d", id) },
		Judge: func(string, string, []plain.FailedUnit) (map[string]plain.UnitJudgement, error) { return nil, nil },
		Git: func(_ string, args ...string) (string, error) {
			joined := strings.Join(args, " ")
			switch {
			// Proof declarations come from the checked commit, including in replay fixtures.
			case len(args) == 2 && args[0] == "show" && strings.HasSuffix(args[1], ":metasystem/metasystem.conf"):
				body, err := os.ReadFile(filepath.Join(b.install, "metasystem.conf"))
				return string(body), err
			// Main's trunk-red register is absent in this bed (no incident open).
			case len(args) > 0 && args[0] == "ls-tree" && strings.HasSuffix(joined, "plans/goals/trunk-red.json"):
				return "", nil
			case joined == "rev-parse --verify HEAD^{commit}":
				return b.head, nil
			case joined == "rev-parse --verify HEAD^{tree}":
				return b.head + "-tree", nil
			case joined == "rev-parse --verify --quiet refs/remotes/origin/main^{commit}":
				return "main", nil
			case len(args) == 3 && args[0] == "rev-parse" && strings.HasSuffix(args[2], "^{tree}"):
				return strings.TrimSuffix(args[2], "^{tree}") + "-tree", nil
			case args[0] == "fetch":
				return "", nil
			case len(args) == 3 && args[0] == "cat-file":
				return "", nil
			case len(args) == 4 && args[0] == "merge-base":
				inside := args[2] == args[3] || args[2] == "main" && (strings.HasPrefix(args[3], "merge-") || strings.HasPrefix(args[3], "main")) || strings.TrimPrefix(args[2], "sha-") <= strings.TrimPrefix(args[3], "merge-") && strings.HasPrefix(args[3], "merge-")
				if inside {
					return "", nil
				}
				return "", fmt.Errorf("git merge-base: %w", &exec.ExitError{ProcessState: falseState})
			case args[0] == "log" || args[0] == "rev-list" && len(args) == 5 && args[1] == "--first-parent":
				var lines []string
				parent := "main"
				for _, g := range []string{"a", "b", "c"} {
					if "merge-"+g <= b.head {
						lines = append(lines, "merge-"+g+" "+parent+" sha-"+g)
						parent = "merge-" + g
					}
				}
				return strings.Join(lines, "\n"), nil
			case len(args) == 5 && args[0] == "worktree" && args[1] == "add":
				dir := filepath.Join(args[3], "metasystem")
				if err := os.MkdirAll(dir, 0o755); err != nil {
					return "", err
				}
				for _, g := range []string{"a", "b", "c"} {
					if args[4] >= "merge-"+g && args[4] != "main" {
						if err := os.WriteFile(filepath.Join(dir, g), []byte(g), 0o644); err != nil {
							return "", err
						}
					}
				}
				return "", nil
			case len(args) == 4 && args[0] == "worktree" && args[1] == "remove":
				return "", os.RemoveAll(args[3])
			case args[0] == "worktree":
				return "", nil
			case args[0] == "rev-list" || args[0] == "diff":
				return "", nil
			}
			t.Fatalf("unstubbed Git: %v", args)
			return "", nil
		},
		Command: func(cmd *exec.Cmd) error {
			only, commit := "", ""
			for _, env := range cmd.Env {
				if v, ok := strings.CutPrefix(env, "LANDING_ONLY="); ok {
					only = v
				}
				if v, ok := strings.CutPrefix(env, "LANDING_COMMIT="); ok {
					commit = v
				}
			}
			b.runs = append(b.runs, commit+":"+only)
			if only == "" {
				b.full++
			}
			report, err := b.fail(cmd, only)
			fmt.Fprint(cmd.Stdout, report)
			return err
		},
	}
	return b
}

// Replay isolates the recorded assembly order even when queue history differs.
func (b *replayVerbBed) prepareBatch(t *testing.T) {
	t.Helper()
	if selected, err := plain.ReadBatch(b.install); err != nil {
		t.Fatal(err)
	} else if selected != nil {
		return
	}
	record, present, err := lane.Read(b.home)
	if err != nil || !present {
		t.Fatalf("fixture registration: %v %v", present, err)
	}
	members := []plain.GoalSHA{{Goal: "a", SHA: "sha-a"}, {Goal: "b", SHA: "sha-b"}}
	if b.head == "merge-c" {
		members = append(members, plain.GoalSHA{Goal: "c", SHA: "sha-c"})
	}
	selected := plain.Batch{ID: "replay-selection", Lane: record, Base: "main", Members: members, Selector: plain.PolicyValue{Value: "auto", Source: "fixture"}, CreatedAt: laneTestNow.Format(time.RFC3339), State: plain.BatchPrepared}
	data, err := json.Marshal(selected)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plain.Dir(b.install), "batch.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
}

func replayFalseState(t *testing.T) *os.ProcessState {
	t.Helper()
	cmd := exec.Command("false")
	if err := cmd.Run(); err == nil {
		t.Fatal("false succeeded")
	}
	return cmd.ProcessState
}

const replayFailure = "LANDING-FAILED\tu/a\tTestBroken\nLANDING-CHECKED\t1\n"

func (b *replayVerbBed) prove(t *testing.T) plain.Result {
	t.Helper()
	words := []string{"prove", "--wait", "--json"}
	if b.trunk {
		words = append(words, "--trunk")
	} else {
		b.prepareBatch(t)
	}
	code, out := b.run(t, b.root, words...)
	var result struct{ Data plain.Result }
	if err := json.Unmarshal([]byte(out), &result); err != nil || code != 1 || result.Data.Result != plain.Red {
		t.Fatalf("prove = %d %s (%v)", code, out, err)
	}
	return result.Data
}

func TestLandingReplayFindsOwnOrMainThroughProve(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"own", "main"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			b := newReplayVerbBed(t)
			entries, err := plain.Entries(b.install)
			if err != nil || len(entries) != 3 || entries[0].Goal != "c" || entries[1].Goal != "b" || entries[2].Goal != "a" {
				t.Fatalf("queue order: %+v %v", entries, err)
			}
			b.fail = func(cmd *exec.Cmd, only string) (string, error) {
				_, exists := os.Stat(filepath.Join(cmd.Dir, "b"))
				if only == "" || kind == "main" || exists == nil {
					return replayFailure, errors.New("red")
				}
				return "LANDING-CHECKED\t0\n", nil
			}
			result := b.prove(t)
			if result.Cause == nil || result.Cause.Kind != kind || result.Repeat != "" || !result.CountedFull {
				t.Fatalf("classification: %+v", result)
			}
			if kind == "own" && (result.Cause.Goal != "b" || result.Cause.SHA != "sha-b" || strings.Join(b.runs, ",") != "merge-b:,main:u/a,merge-a:u/a,merge-b:u/a") {
				t.Fatalf("merge attribution: %+v runs=%v", result.Cause, b.runs)
			}
			data, err := os.ReadFile(result.Cause.Evidence)
			if err != nil || string(data) != replayFailure || filepath.Dir(result.Cause.Evidence) != filepath.Join(plain.Dir(b.install), "proofs") || result.Cause.Evidence == result.Log {
				t.Fatalf("isolated evidence: %q %v %s", result.Cause.Evidence, err, data)
			}
		})
	}
}

func TestLandingReplayAllowsOnlyOneWholeRepeat(t *testing.T) {
	t.Parallel()
	b := newReplayVerbBed(t)
	b.fail = func(_ *exec.Cmd, only string) (string, error) {
		if only == "" {
			return replayFailure, errors.New("red")
		}
		return "LANDING-CHECKED\t0\n", nil
	}
	if first := b.prove(t); first.Repeat != "allowed" {
		t.Fatalf("first: %+v", first)
	}
	if second := b.prove(t); second.Repeat != "started" || second.Cause.Kind != "unclassified" {
		t.Fatalf("second: %+v", second)
	}
	code, out := b.run(t, b.root, "prove", "--wait")
	if code != 1 || !strings.Contains(out, "gets no other") || b.full != 2 {
		t.Fatalf("third = %d %s, whole runs=%d", code, out, b.full)
	}
}

func TestLandingReplayIncompleteIsolationHolds(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"green without report", "red without report", "not run"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b := newReplayVerbBed(t)
			b.fail = func(_ *exec.Cmd, only string) (string, error) {
				if only == "" {
					return replayFailure, errors.New("red")
				}
				switch name {
				case "green without report":
					return "", nil
				case "not run":
					return "LANDING-NOT-RUN\tbusy\n", errors.New("busy")
				default:
					return "", errors.New("red")
				}
			}
			result := b.prove(t)
			if result.Cause.Kind != "unclassified" || result.Repeat != "" || len(b.runs) != 2 {
				t.Fatalf("incomplete replay: %+v runs=%v", result, b.runs)
			}
			if code, _ := b.run(t, b.root, "prove", "--wait"); code != 1 || b.full != 1 {
				t.Fatalf("incomplete replay authorized another check: %d full=%d", code, b.full)
			}
		})
	}
}

func TestLandingReplayEnvironmentRepeatIsUncountedAndSpent(t *testing.T) {
	t.Parallel()
	b := newReplayVerbBed(t)
	b.fail = func(_ *exec.Cmd, _ string) (string, error) { return "LANDING-NOT-RUN\tbusy\n", errors.New("busy") }
	first, second := b.prove(t), b.prove(t)
	if first.Cause.Kind != "environment" || first.Repeat != "allowed" || first.CountedFull || second.Cause.Kind != "environment" || second.Repeat != "started" || second.CountedFull {
		t.Fatalf("environment allowance: first=%+v second=%+v", first, second)
	}
	if code, _ := b.run(t, b.root, "prove", "--wait"); code != 1 || b.full != 2 {
		t.Fatalf("third environment check: %d full=%d", code, b.full)
	}
}

func TestLandingReplayLostProcessPrecedesCompleteReport(t *testing.T) {
	t.Parallel()
	b := newReplayVerbBed(t)
	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err == nil || cmd.ProcessState.Exited() {
		t.Fatal("fixture was not killed")
	}
	b.fail = func(_ *exec.Cmd, _ string) (string, error) {
		if b.full == 1 {
			return replayFailure, &exec.ExitError{ProcessState: cmd.ProcessState}
		}
		return "LANDING-CHECKED\t0\n", nil
	}
	first := b.prove(t)
	if first.Cause.Kind != "environment" || first.Repeat != "allowed" || first.CountedFull || len(first.Failed) != 0 || len(b.runs) != 1 {
		t.Fatalf("lost process was replayed or counted: %+v runs=%v", first, b.runs)
	}
	if code, out := b.run(t, b.root, "prove", "--wait"); code != 0 {
		t.Fatalf("environment repeat recorded a flake: %d %s", code, out)
	}
}

func TestLandingReplayBudgetHoldsShrinkingBatchAndOnlyPersonRunReopens(t *testing.T) {
	t.Parallel()
	b := newReplayVerbBed(t)
	lineage := lane.AgentLineage
	b.owners.dependencies.ownerLineage = func() string { return lineage }
	b.owners.landing.keeper = func(home, root string) lane.AgentKeeper {
		return lane.AgentKeeper{Home: home, Self: root, Now: func() time.Time { return laneTestNow }, Running: func() (string, bool, error) { return "landing-session", true, nil }}
	}
	b.head = "merge-c"
	culprit := "c"
	b.fail = func(cmd *exec.Cmd, only string) (string, error) {
		_, exists := os.Stat(filepath.Join(cmd.Dir, culprit))
		if only == "" || exists == nil {
			return replayFailure, errors.New("red")
		}
		return "LANDING-CHECKED\t0\n", nil
	}
	for _, g := range []string{"c", "b"} {
		culprit = g
		if result := b.prove(t); result.Cause.Kind != "own" || result.Cause.Goal != g {
			t.Fatalf("culprit %s: %+v", g, result)
		}
		if _, _, err := plain.ReturnProven(b.install, g, "own", "", false, "", laneTestNow); err != nil {
			t.Fatal(err)
		}
		b.head = "merge-b"
	}
	b.head, culprit = "merge-a", "a"
	for _, args := range [][]string{{"prove", "--wait"}, {"prove"}} {
		code, out := b.run(t, b.root, args...)
		if code != 1 || !strings.Contains(out, "two full checks") || !strings.Contains(out, "metasystem landing run") || b.full != 2 {
			t.Fatalf("third = %d %s, whole runs=%d", code, out, b.full)
		}
	}
	code, out := b.run(t, b.root, "run", "--json")
	var run struct{ Data landingRunData }
	if code != 0 || json.Unmarshal([]byte(out), &run) != nil || run.Data.Outcome != lane.AgentRunning {
		t.Fatalf("agent's run = %d %s", code, out)
	}
	for _, args := range [][]string{{"prove", "--wait"}, {"prove"}} {
		if code, out := b.run(t, b.root, args...); code != 1 || !strings.Contains(out, "two full checks") || !strings.Contains(out, "a person") || b.full != 2 {
			t.Fatalf("agent reopened its own budget: %d %s, whole runs=%d", code, out, b.full)
		}
	}
	lineage = ""
	if code, out := b.run(t, b.root, "run"); code != 0 {
		t.Fatalf("reopen = %d %s", code, out)
	}
	if result := b.prove(t); result.Cause.Kind != "own" || result.Cause.Goal != "a" || b.full != 3 {
		t.Fatalf("reopened: %+v, whole runs=%d", result, b.full)
	}
}
