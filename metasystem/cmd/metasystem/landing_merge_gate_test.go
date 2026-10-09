package main

import (
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
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func newMergeGateBed(t *testing.T) *replayVerbBed {
	t.Helper()
	b := newReplayVerbBed(t)
	b.owners.landing.person = func(string) (string, error) { return "", errors.New("no enrolled person") }
	git := b.owners.landing.plainProve.Git
	b.owners.landing.plainProve.Git = func(dir string, args ...string) (string, error) {
		if len(args) == 2 && args[0] == "show" && strings.HasSuffix(args[1], ":metasystem/metasystem.conf") {
			return "proof.full=full-fixture\nproof.cheap=cheap-fixture\n", nil
		}
		if len(args) == 4 && strings.Join(args[:3], " ") == "show -s --format=%P" {
			if args[3] == "merge-b" {
				return "merge-a sha-b", nil
			}
			return "main sha-a", nil
		}
		return git(dir, args...)
	}
	b.owners.landing.plainProve.RecordMain = func([]plain.Result) error { return nil }
	return b
}

func gateResult(t *testing.T, b *replayVerbBed, want int) plain.Result {
	t.Helper()
	b.prepareBatch(t)
	code, out := b.run(t, b.root, "prove", "--gate", "--wait", "--json")
	var result struct{ Data plain.Result }
	if err := json.Unmarshal([]byte(out), &result); err != nil || code != want || result.Data.Result == "" {
		t.Fatalf("gate = %d %s (%v)", code, out, err)
	}
	if result.Data.Scope != "gate" || result.Data.CountedFull || result.Data.FullAt != "" || result.Data.FullTree != "" {
		t.Fatalf("gate claimed full proof: %+v", result.Data)
	}
	return result.Data
}

func commandEnv(cmd *exec.Cmd, key string) string {
	for _, env := range cmd.Env {
		if value, ok := strings.CutPrefix(env, key+"="); ok {
			return value
		}
	}
	return ""
}

func TestLandingPausedPersonProofUnreadableLaunchRetainsFailedAdmission(t *testing.T) {
	t.Parallel()
	b := newMergeGateBed(t)
	helmMust(t, os.WriteFile(filepath.Join(b.install, "metasystem.conf"), []byte("metasystem.template=true\nproof.full=fixture\n"), 0600))
	b.owners.resolver = stateroot.NewResolver(func(string) (string, error) { return b.root, nil }, os.Executable)
	b.prepareBatch(t)
	_, err := lane.SetPause(b.home, "Wido", laneTestNow)
	helmMust(t, err)
	b.owners.prove = enrolledPersonProver(t, b.install, laneTestNow)
	b.owners.landing.plainProve.Executable = func() (string, error) { return "/fixture/engine", nil }
	b.owners.landing.plainProve.Launch = func([]string, string, string) (int64, error) {
		child := exec.Command("/usr/bin/true")
		if err := child.Run(); err != nil {
			return 0, err
		}
		return int64(child.Process.Pid), nil
	}
	code, text := b.run(t, b.root, "prove")
	if code != 1 || !strings.Contains(text, "environment") || !strings.Contains(text, "metasystem landing prove") {
		t.Fatalf("unreadable launch lacks its environment refusal and retry: exit=%d output=%s", code, text)
	}
	// Decision 4 keeps failed admission visible without spending an execution.
	failed, recorded, alive, err := plain.ReadRunning(b.install, b.owners.landing.plainProve)
	if err != nil || !recorded || alive || failed.Admission == nil || failed.Admission.State != "failed" {
		t.Fatalf("unreadable launch lost its failed admission: %+v alive=%v err=%v", failed, alive, err)
	}
	results, err := plain.Results(b.install)
	if err != nil || len(results) != 0 || len(b.runs) != 0 || b.full != 0 {
		t.Fatalf("launch failure ran a proof or used its allowance: results=%+v runs=%v full=%d err=%v", results, b.runs, b.full, err)
	}
	if _, paused := lane.ReadPause(b.home); !paused {
		t.Fatal("launch failure removed the person's pause")
	}
}

func TestLandingMergeGateBaselineAndLostProcessRepeat(t *testing.T) {
	t.Parallel()
	for _, lost := range []string{"not run", "killed", "dead detached"} {
		t.Run(lost, func(t *testing.T) {
			t.Parallel()
			b := newMergeGateBed(t)
			killed := exec.Command("sleep", "30")
			if err := killed.Start(); err != nil {
				t.Fatal(err)
			}
			if err := killed.Process.Kill(); err != nil {
				t.Fatal(err)
			}
			if err := killed.Wait(); err == nil {
				t.Fatal("process was not killed")
			}
			b.fail = func(cmd *exec.Cmd, only string) (string, error) {
				if cmd.Args[2] != "cheap-fixture" || commandEnv(cmd, "LANDING_PROOF_BASE") != "merge-a" || only != "" {
					t.Fatalf("gate command/environment: %v base=%q only=%q", cmd.Args, commandEnv(cmd, "LANDING_PROOF_BASE"), only)
				}
				if commandEnv(cmd, "LANDING_COMMIT") == "merge-a" {
					return "LANDING-CHECKED\t0\n", nil
				}
				if lost == "not run" {
					return "LANDING-NOT-RUN\tbusy\n", nil
				}
				return replayFailure, &exec.ExitError{ProcessState: killed.ProcessState}
			}
			if lost == "dead detached" {
				writeCauseProof(t, b.install, "gates.jsonl", plain.Result{Commit: "merge-a", Tree: "merge-a-tree", Scope: "gate", Result: plain.Green, At: laneTestNow.Format(time.RFC3339)})
				b.owners.landing.plainProve.Executable = func() (string, error) { return "/engine", nil }
				b.owners.landing.plainProve.Alive = func(plain.Running) bool { return false }
				b.owners.landing.plainProve.Launch = func(argv []string, _, _ string) (int64, error) {
					if !strings.Contains(strings.Join(argv, " "), "--gate") {
						t.Fatalf("detached gate lost its mode: %v", argv)
					}
					return int64(os.Getppid()), nil
				}
				b.prepareBatch(t)
				if code, out := b.run(t, b.root, "prove", "--gate"); code != 0 {
					t.Fatalf("start gate = %d %s", code, out)
				}
			} else {
				first := gateResult(t, b, 1)
				if first.Cause.Kind != "environment" || first.Repeat != "allowed" || !reflect.DeepEqual(b.runs, []string{"merge-a:", "merge-b:"}) {
					t.Fatalf("baseline/environment: %+v runs=%v", first, b.runs)
				}
			}
			second := gateResult(t, b, 1)
			if second.Cause.Kind != "environment" || second.Cause.Name != "lost-process" || second.Repeat != "started" {
				t.Fatalf("lost process repeat: %+v", second)
			}
			before := len(b.runs)
			if code, out := b.run(t, b.root, "prove", "--gate", "--wait"); code != 1 || !strings.Contains(out, "gets no other") || len(b.runs) != before {
				t.Fatalf("third gate = %d %s runs=%v", code, out, b.runs)
			}
			if code, _ := b.run(t, b.root, "return", "b", "--cause", "own"); code != 1 {
				t.Fatal("environment failure authorized a return")
			}
			entry, _, err := plain.Latest(b.install, "b")
			if err != nil || entry.State != plain.StateWaiting {
				t.Fatalf("lost process returned goal: %+v %v", entry, err)
			}
			if results, err := plain.Results(b.install); err != nil || len(results) != 0 {
				t.Fatalf("gate contaminated full proofs: %+v %v", results, err)
			}
		})
	}
}

func TestLandingMergeGateRegisteredFlakeRepeatsAlone(t *testing.T) {
	t.Parallel()
	b := newMergeGateBed(t)
	contract := testpolicy.Contract{SchemaVersion: 1,
		ProjectRisk: testpolicy.ProjectRisk{Severity: 1, Exposure: 1, Reversibility: "revert", Detection: "immediate", Recovery: "bounded"},
		Fallback:    "residual", Surfaces: []testpolicy.Surface{{ID: "unit", Paths: []string{"u/a/**"}, Standard: []string{"check"}}, {ID: "other", Paths: []string{"b/**"}}, {ID: "residual"}},
		Groups: []testpolicy.Group{{ID: "check", Kind: "unit", Adapter: "go", CWD: ".", Inputs: []string{"u/**"}, Platforms: []string{"any"}, TargetMS: 1, Packages: []string{"u/a"}, Tests: json.RawMessage(`"all"`)}},
		Always: testpolicy.Always{Canary: []string{"check"}}, Unknown: []string{"check"}, Cadence: []string{"check"}}
	if err := contract.Validate(); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(contract)
	if err != nil {
		t.Fatal(err)
	}
	entry := goal.TrunkRedEntry{ID: "flaky:u/a", Identity: "flaky:u/a", Group: "u/a", Status: "failed", Class: goal.TrunkRedClassPendingFlake,
		Failures: []goal.TrunkRedFailure{{Report: "red.log", Classname: "u/a", Name: "TestBroken"}}, Holds: []string{}, Opened: "2026-10-04T10:00:00Z",
		Sightings: []goal.TrunkRedSighting{{Attempt: "red", BaseCommit: "old", SeenAt: "2026-10-04T10:00:00Z", Opid: "01J5X0000000000000000000F1-lane-12345678"}}}
	if err := os.WriteFile(filepath.Join(b.install, "metasystem.conf"), []byte("testing.contract=testing.json\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git := b.owners.landing.plainProve.Git
	b.owners.landing.plainProve.Judge = nil
	b.owners.landing.plainProve.Git = func(dir string, args ...string) (string, error) {
		switch strings.Join(args, " ") {
		case "show origin/main:metasystem/testing.json":
			return string(data), nil
		case "show origin/main:metasystem/plans/goals/trunk-red.json":
			return string(goal.RenderTrunkRed([]goal.TrunkRedEntry{entry})), nil
		case "diff --name-only merge-b^1..merge-b":
			return "b/change.go", nil
		case "diff --name-only origin/main...merge-b":
			t.Fatal("gate judged earlier batch goals instead of the last merge")
		case "ls-tree -z origin/main:u/a":
			return "100644 blob abc\tunit.go", nil
		}
		return git(dir, args...)
	}
	var records []plain.FlakeRecord
	b.owners.landing.plainProve.RecordFlake = func(record plain.FlakeRecord) (plain.FlakeRecorded, error) {
		records = append(records, record)
		return plain.FlakeRecorded{Goal: "fix-flaky-u-a", Seen: 2}, nil
	}
	b.fail = func(cmd *exec.Cmd, only string) (string, error) {
		if commandEnv(cmd, "LANDING_COMMIT") == "merge-b" && only == "" {
			return flakeTestEvents([]string{"TestBroken"}, "fail") + replayFailure, exec.Command("false").Run()
		}
		return flakeTestEvents([]string{"TestBroken"}, "pass") + "LANDING-CHECKED\t0\n", nil
	}
	green := gateResult(t, b, 0)
	if !reflect.DeepEqual(b.runs, []string{"merge-a:", "merge-b:", "merge-b:u/a"}) || len(records) != 1 || len(records[0].Outputs) != 1 || len(records[0].RepeatOutputs) != 1 || records[0].Repeat != "alone" || records[0].Commit != "merge-b" || records[0].RepeatLog == records[0].Log || !strings.Contains(green.Reason, "fix-flaky-u-a") {
		t.Fatalf("flake: %+v records=%+v runs=%v", green, records, b.runs)
	}
	if _, err := os.Stat(records[0].RepeatLog); err != nil {
		t.Fatal(err)
	}
	if code, out := b.run(t, b.root, "prove", "--gate"); code != 0 || len(b.runs) != 3 || !strings.Contains(out, "already passed the cheap gate") || strings.Contains(out, "already proving") {
		t.Fatalf("green gate ran again: %d %s %v", code, out, b.runs)
	}
}

func TestLandingMergeGateOwnReturnAndBaselineFailure(t *testing.T) {
	t.Parallel()
	for _, cause := range []string{"own", "main", "unclassified", "no report", "main after baseline", "prior after baseline"} {
		t.Run(cause, func(t *testing.T) {
			t.Parallel()
			b := newMergeGateBed(t)
			if strings.HasPrefix(cause, "main") {
				b.head = "merge-a"
			}
			if strings.HasSuffix(cause, "after baseline") {
				parent := "merge-a"
				if b.head == "merge-a" {
					parent = "main"
				}
				writeCauseProof(t, b.install, "gates.jsonl", plain.Result{Commit: parent, Tree: parent + "-tree", Result: plain.Green, Scope: "gate", At: laneTestNow.Format(time.RFC3339)})
			}
			var incidents []plain.Result
			b.owners.landing.plainProve.RecordMain = func(checks []plain.Result) error { incidents = append(incidents, checks...); return nil }
			b.fail = func(cmd *exec.Cmd, only string) (string, error) {
				commit := commandEnv(cmd, "LANDING_COMMIT")
				if strings.Contains(cause, "after baseline") || cause == "main" || cause == "unclassified" || commit == "merge-b" {
					if cause == "no report" {
						return "", exec.Command("false").Run()
					}
					return replayFailure, exec.Command("false").Run()
				}
				return "LANDING-CHECKED\t0\n", nil
			}
			red := gateResult(t, b, 1)
			want := cause
			if cause == "no report" {
				want = "unclassified"
			} else if cause == "main after baseline" || cause == "prior after baseline" {
				want = "main"
			}
			if red.Cause.Kind != want {
				t.Fatalf("classification: %+v runs=%v", red, b.runs)
			}
			if cause == "own" {
				if red.Cause.Goal != "b" || red.Cause.SHA != "sha-b" || !reflect.DeepEqual(b.runs, []string{"merge-a:", "merge-b:", "merge-a:u/a"}) {
					t.Fatalf("gate replay: %+v runs=%v", red, b.runs)
				}
				data, err := os.ReadFile(red.Cause.Evidence)
				if err != nil || string(data) != replayFailure+"\nlanding prove: the proving command exited 1\n" || red.Cause.Evidence != red.Log {
					t.Fatalf("own evidence: %s %v", data, err)
				}
				b.head = "merge-a"
				if code, out := b.run(t, b.root, "return", "b", "--cause", "own"); code != 0 {
					t.Fatalf("own gate return: %d %s", code, out)
				}
				entry, _, err := plain.Latest(b.install, "b")
				if err != nil || entry.State != plain.StateReturned || entry.Cause.SHA != "sha-b" {
					t.Fatalf("return: %+v %v", entry, err)
				}
			} else {
				if code, _ := b.run(t, b.root, "return", "b", "--cause", "own"); code != 1 {
					t.Fatal("gate returned an unproven defect")
				}
				if cause == "unclassified" && strings.Contains(strings.Join(b.runs, ","), "merge-b:") {
					t.Fatal("gate ran on HEAD without a green baseline")
				}
			}
			if strings.HasPrefix(cause, "main") && (len(incidents) != 1 || incidents[0].Commit != "main") {
				t.Fatalf("main baseline red was not registered: %+v", incidents)
			}
		})
	}
}

func TestLandingMergeGateRequiresCheapDeclaration(t *testing.T) {
	t.Parallel()
	b := newMergeGateBed(t)
	git := b.owners.landing.plainProve.Git
	b.owners.landing.plainProve.Git = func(dir string, args ...string) (string, error) {
		if len(args) == 2 && args[0] == "show" && strings.HasSuffix(args[1], ":metasystem/metasystem.conf") {
			return "proof.full=true\n", nil
		}
		return git(dir, args...)
	}
	if code, out := b.run(t, b.root, "prove", "--gate", "--wait"); code != 1 || !strings.Contains(out, "declare proof.cheap in metasystem.conf") || strings.Contains(out, "settings set") || len(b.runs) != 0 {
		t.Fatalf("missing declaration = %d %s runs=%v", code, out, b.runs)
	}
}

func TestLandingMergeGateBaselineUsesParentsCommittedCommand(t *testing.T) {
	t.Parallel()
	b := newMergeGateBed(t)
	git := b.owners.landing.plainProve.Git
	b.owners.landing.plainProve.Git = func(dir string, args ...string) (string, error) {
		if len(args) == 2 && args[0] == "show" && strings.HasSuffix(args[1], ":metasystem/metasystem.conf") {
			if strings.HasPrefix(args[1], "merge-a:") {
				return "proof.cheap=parent-cheap\n", nil
			}
			return "proof.cheap=goal-cheap\n", nil
		}
		return git(dir, args...)
	}
	var commands []string
	b.fail = func(cmd *exec.Cmd, _ string) (string, error) {
		commands = append(commands, commandEnv(cmd, "LANDING_COMMIT")+":"+cmd.Args[len(cmd.Args)-1])
		return "LANDING-CHECKED\t0\n", nil
	}
	gateResult(t, b, 0)
	if !reflect.DeepEqual(commands, []string{"merge-a:parent-cheap", "merge-b:goal-cheap"}) {
		t.Fatalf("the baseline used the goal's changed declaration: %v", commands)
	}
}

func TestLandingMergeGateGreenCannotAuthorizePush(t *testing.T) {
	t.Parallel()
	bed := newPlainVerbBed(t)
	now := laneTestNow
	bed.owners.landing.plainProve.Now = func() time.Time { return now }
	conf := filepath.Join(bed.installation, "metasystem.conf")
	if err := os.WriteFile(conf, []byte("metasystem.template=true\nproof.full=exit 9\nproof.cheap=printf 'LANDING-CHECKED\\t0\\n'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bed.git(t, bed.checkout, "add", "metasystem/metasystem.conf")
	bed.git(t, bed.checkout, "commit", "--quiet", "-m", "proof declarations")
	bed.git(t, bed.checkout, "push", "--quiet", "origin", "main")
	bed.main = bed.git(t, bed.checkout, "rev-parse", "HEAD")
	bed.git(t, bed.checkout, "checkout", "--quiet", "-b", "goal/g")
	if err := os.WriteFile(filepath.Join(bed.installation, "goal.go"), []byte("package fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bed.git(t, bed.checkout, "add", "metasystem/goal.go")
	bed.git(t, bed.checkout, "commit", "--quiet", "-m", "goal")
	sha := bed.git(t, bed.checkout, "rev-parse", "HEAD")
	bed.git(t, bed.checkout, "checkout", "--quiet", "main")
	bed.git(t, bed.checkout, "merge", "--quiet", "--no-ff", "-m", "merge goal", sha)
	if _, _, err := plain.HandIn(bed.installation, plain.Line{Goal: "g", SHA: sha}); err != nil {
		t.Fatal(err)
	}
	record, present, err := lane.Read(bed.home)
	if err != nil || !present {
		t.Fatalf("fixture registration: %v %v", present, err)
	}
	if _, err := plain.SelectBatch(bed.installation, bed.checkout, record, plain.ProveSeams{}); err != nil {
		t.Fatalf("prepare fixture selection: %v", err)
	}
	var stdout, stderr strings.Builder
	code := runIntentIn(mustIntentCommand(t, "landing prove"), []string{"--gate", "--wait", "--json"}, &stdout, &stderr, bed.checkout, bed.owners)
	if code != 0 {
		t.Fatalf("real gate: %d %s%s", code, &stdout, &stderr)
	}
	gate, ok, err := plain.LastGate(bed.installation)
	if err != nil || !ok || gate.Result != plain.Green {
		t.Fatalf("real gate result: %+v %v", gate, err)
	}
	fingerprint, err := plain.KeeperFingerprint(bed.checkout)
	if err != nil {
		t.Fatal(err)
	}
	gate.At = now.Add(time.Minute).Format(time.RFC3339)
	writeCauseProof(t, bed.installation, "gates.jsonl", gate)
	after, err := plain.KeeperFingerprint(bed.checkout)
	if err != nil || after == fingerprint {
		t.Fatalf("keeper missed gate completion: %s %s %v", fingerprint, after, err)
	}
	reasons, err := plain.WakeReasons(bed.installation, bed.checkout, now, now.Add(time.Minute))
	if err != nil || !strings.Contains(strings.Join(reasons, ","), plain.WakeProofFinished) {
		t.Fatalf("gate did not wake keeper: %v %v", reasons, err)
	}
	log, err := plain.ProofLog(bed.installation, gate.Attempt)
	if err != nil || log != gate.Log {
		t.Fatalf("gate log: %s %v", log, err)
	}
	stdout.Reset()
	stderr.Reset()
	code = runIntentIn(mustIntentCommand(t, "landing push"), []string{"--json"}, &stdout, &stderr, bed.checkout, bed.owners)
	if code != 1 || !strings.Contains(stdout.String(), "was never proven") || bed.git(t, bed.origin, "rev-parse", "main") != bed.main {
		t.Fatalf("gate authorized push: %d %s%s", code, &stdout, &stderr)
	}
	if results, err := plain.Results(bed.installation); err != nil || len(results) != 0 {
		t.Fatalf("gate wrote full results: %+v %v", results, err)
	}
}

func TestLandingMergeGateStatusReadsItsResult(t *testing.T) {
	t.Parallel()
	b := newMergeGateBed(t)
	b.owners.landing.view = func(string) lane.View { return lane.View{Root: &b.root, Owner: lane.OwnerView{State: lane.OwnerIdle}} }
	b.fail = func(*exec.Cmd, string) (string, error) { return "LANDING-CHECKED\t0\n", nil }
	gate := gateResult(t, b, 0)
	code, out := b.run(t, b.root, "status", "--json")
	var status struct{ Data plain.Status }
	if code != 0 || json.Unmarshal([]byte(out), &status) != nil || status.Data.LastGate == nil || status.Data.LastGate.Tree != gate.Tree || status.Data.LastProof != nil {
		t.Fatalf("gate status: %d %s", code, out)
	}
	if code, out := b.run(t, b.root, "status", "--verbose"); code != 0 || !strings.Contains(out, "gate") {
		t.Fatalf("gate text status: %d %s", code, out)
	}
}

func TestLandingMergeGateUsesRecordedRedAndKeepsFullAllowance(t *testing.T) {
	t.Parallel()
	b := newMergeGateBed(t)
	writeCauseProof(t, b.install, "results.jsonl",
		plain.Result{Commit: "merge-a", Tree: "merge-a-tree", Scope: "full", Result: plain.Green, At: laneTestNow.Format(time.RFC3339)},
		plain.Result{Commit: "merge-b", Tree: "merge-b-tree", Result: plain.Red, Repeat: "started", At: laneTestNow.Format(time.RFC3339)})
	b.fail = func(cmd *exec.Cmd, only string) (string, error) {
		if commandEnv(cmd, "LANDING_COMMIT") == "merge-b" && only == "" {
			return replayFailure, exec.Command("false").Run()
		}
		return "LANDING-CHECKED\t0\n", nil
	}
	first := gateResult(t, b, 1)
	if first.Repeat != "" || first.Cause.Kind != "own" || first.Cause.Goal != "b" || !reflect.DeepEqual(b.runs, []string{"merge-a:", "merge-b:", "merge-a:u/a"}) {
		t.Fatalf("full proof supplied gate baseline or spent its repeat: %+v runs=%v", first, b.runs)
	}
	before := len(b.runs)
	if code, out := b.run(t, b.root, "prove", "--gate", "--wait"); code != 1 || !strings.Contains(out, "gets no other") || len(b.runs) != before {
		t.Fatalf("third gate ran: %d %s runs=%v", code, out, b.runs)
	}
	if full, err := plain.Results(b.install); err != nil || len(full) != 2 || full[1].Repeat != "started" {
		t.Fatalf("gate changed full allowance: %+v %v", full, err)
	}
}
