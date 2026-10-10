package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/repoproof"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// The subprocess runs the reporter in the proof's extracted checkout.
func TestLandingReplayReporterProcess(t *testing.T) {
	t.Parallel()
	if os.Getenv("LANDING_REPLAY_REPORTER") != "1" {
		return
	}
	var failed []plain.FailedUnit
	if err := json.Unmarshal([]byte(os.Getenv("LANDING_REPLAY_FAILURES")), &failed); err != nil {
		t.Fatal(err)
	}
	failure := func(unit string) *plain.FailedUnit {
		for i := range failed {
			if failed[i].Unit == unit {
				return &failed[i]
			}
		}
		return nil
	}
	redExit := func() error { return exec.Command("/usr/bin/false").Run() }
	hooks := repoproof.HostRunners{Environment: func() (string, error) { return "fixture toolchain", nil },
		Groups: func(ids []string) ([]proofrun.NamedGroupResult, error) {
			fmt.Fprintln(os.Stdout, "fixture groups "+strings.Join(ids, " "))
			var results []proofrun.NamedGroupResult
			for _, id := range ids {
				status := "green"
				if failure(id) != nil {
					status = "red"
				}
				results = append(results, proofrun.NamedGroupResult{ID: id, Status: status})
			}
			return results, nil
		},
		Native: func(r proofrun.NativeInventoryRequest) (proofrun.NativeInventoryResult, error) {
			fmt.Fprintln(os.Stdout, "fixture native "+strings.Join(r.Packages, " ")+" tests="+strings.Join(r.Tests, ","))
			packages := r.Packages
			if len(r.BuildTags) > 0 {
				return proofrun.NativeInventoryResult{Execution: []proofrun.PackageExecution{{Package: "fixture/tagged", Status: "ok", Shard: 1}}}, nil
			}
			if reflect.DeepEqual(packages, []string{"./..."}) {
				packages = []string{"internal/p", "cmd/metasystem", "internal/lease"}
			}
			var result proofrun.NativeInventoryResult
			for _, pkg := range packages {
				if !slices.Contains([]string{"internal/p", "cmd/metasystem", "internal/lease"}, pkg) {
					return result, fmt.Errorf("go package discovery omitted declared package directory %s", pkg)
				}
				unit := "metasystem/" + pkg
				execution := proofrun.PackageExecution{Package: "github.com/widoriezebos/agentic-tools/metasystem/" + pkg, Status: "ok", Shard: 1}
				names := map[string][]string{"internal/p": {"TestP"}, "cmd/metasystem": {"TestGoal"}}[pkg]
				status, action := "passed", "pass"
				if f := failure(unit); f != nil {
					execution.Status, result.Failed = "fail", true
					names, status, action = f.Tests, "failed", "fail"
				}
				for _, name := range names {
					result.Observed = append(result.Observed, proofrun.NativeTestIdentity{Classname: execution.Package, Name: name, Status: status})
					event, _ := json.Marshal(map[string]string{"Action": action, "Package": execution.Package, "Test": name})
					result.Output = append(result.Output, append(event, '\n')...)
				}
				event, _ := json.Marshal(map[string]string{"Action": action, "Package": execution.Package})
				result.Output = append(result.Output, append(event, '\n')...)
				result.Execution = append(result.Execution, execution)
			}
			return result, nil
		},
	}
	command := func(argv []string, stdout, stderr io.Writer) error {
		if !reflect.DeepEqual(argv, []string{"go", "run", "./cmd/devgate", "static"}) {
			return fmt.Errorf("unexpected reporter command: %v", argv)
		}
		if failure("fast-static-build") != nil {
			return redExit()
		}
		return nil
	}
	os.Exit(repoproof.RunHost(os.Stdout, os.Stderr, os.Getenv, command, hooks))
}

type oneGoalReplayBed struct {
	*replayVerbBed
	parent, main, sha      string
	failures, mainFailures []plain.FailedUnit
	move                   bool
}

func newOneGoalReplayBed(t *testing.T) *oneGoalReplayBed {
	t.Helper()
	b := &oneGoalReplayBed{replayVerbBed: newReplayVerbBed(t), parent: "main", main: "main", sha: "sha-g"}
	helmMust(t, os.Remove(filepath.Join(plain.Dir(b.install), "queue.jsonl")))
	_, _, err := plain.HandIn(b.install, plain.Line{Goal: "g", SHA: "sha-g"})
	helmMust(t, err)
	b.head = "merge-g"
	helmMust(t, os.WriteFile(filepath.Join(b.install, "metasystem.conf"), []byte("metasystem.template=true\nproof.full=fixture\n"), 0600))
	registration, _, err := lane.Read(b.home)
	helmMust(t, err)
	batch := plain.Batch{ID: "one-goal", Lane: registration, Base: "main", Members: []plain.GoalSHA{{Goal: "g", SHA: "sha-g"}}, State: plain.BatchPrepared}
	data, err := json.Marshal(batch)
	helmMust(t, err)
	helmMust(t, os.WriteFile(filepath.Join(plain.Dir(b.install), "batch.json"), data, 0600))
	contract := testpolicy.Contract{SchemaVersion: 2, ProjectRisk: testpolicy.ProjectRisk{Severity: 1, Exposure: 1, Reversibility: "revert", Detection: "immediate", Recovery: "bounded"},
		Surfaces: []testpolicy.Surface{{ID: "source", Paths: []string{"metasystem/**"}, Standard: []string{"go-tests"}}}, Unknown: []string{"go-tests"},
		Groups: []testpolicy.Group{{ID: "go-tests", Kind: "unit", Adapter: "go", CWD: ".", Phase: "acceptance", EnvironmentMode: "inherit", Inputs: []string{"metasystem/**"}, Platforms: []string{"any"}, TargetMS: 1, Packages: []string{"./..."}, Tests: json.RawMessage(`"all"`)}, {ID: "fast-static-build", Kind: "static", Adapter: "command", CWD: ".", Phase: "admission", EnvironmentMode: "inherit", Inputs: []string{"metasystem/**"}, Platforms: []string{"any"}, TargetMS: 1, Argv: []string{"fixture"}, Format: "exit-status"}}}
	data, err = json.Marshal(contract)
	helmMust(t, err)
	b.contract = string(data)
	helmMust(t, os.WriteFile(filepath.Join(b.install, "testing.json"), data, 0600))
	falseState := replayFalseState(t)
	git := b.owners.landing.plainProve.Git
	b.owners.landing.plainProve.Git = func(dir string, args ...string) (string, error) {
		switch {
		case args[0] == "rev-parse" && strings.Contains(args[len(args)-1], "origin/main"):
			return b.main, nil
		case len(args) == 4 && strings.Join(args[:3], " ") == "show -s --format=%P":
			return b.parent + " " + b.sha, nil
		case args[0] == "merge-base":
			ancestor, commit := args[2], args[3]
			if ancestor == commit || strings.HasPrefix(ancestor, "main") && (strings.HasPrefix(commit, "main") || strings.HasPrefix(commit, "merge")) || strings.HasPrefix(ancestor, "sha-g") && strings.HasPrefix(commit, "merge") {
				return "", nil
			}
			return "", fmt.Errorf("git merge-base: %w", &exec.ExitError{ProcessState: falseState})
		case args[0] == "log":
			if !strings.HasPrefix(args[len(args)-1], b.parent+"..") && !strings.HasPrefix(args[len(args)-1], "origin/main..") {
				t.Fatalf("replay walked wrong base: %v", args)
			}
			return b.head + " " + b.parent + " " + b.sha, nil
		case args[0] == "rev-list" && args[1] == "--first-parent":
			if strings.HasPrefix(args[len(args)-1], "main..main") {
				return "", nil
			}
			prefix := ""
			if b.parent != "main" {
				prefix = b.parent + " main\n"
			}
			return prefix + b.head + " " + b.parent + " " + b.sha, nil
		case args[0] == "worktree" && args[1] == "add":
			target := filepath.Join(args[3], "metasystem")
			helmMust(t, os.MkdirAll(target, 0700))
			helmMust(t, os.WriteFile(filepath.Join(target, "testing.json"), []byte(b.contract), 0600))
			return "", nil
		case args[0] == "diff":
			return "metasystem/internal/p/p.go\n", nil
		default:
			return git(dir, args...)
		}
	}
	b.owners.landing.plainProve.Command = func(cmd *exec.Cmd) error {
		commit, only := commandEnv(cmd, "LANDING_COMMIT"), commandEnv(cmd, "LANDING_ONLY")
		b.runs = append(b.runs, commit+":"+only)
		if only == "" {
			b.full++
		}
		failures := b.failures
		if strings.HasPrefix(commit, "main") {
			failures = b.mainFailures
		}
		data, err := json.Marshal(failures)
		helmMust(t, err)
		child := exec.Command(commandTestExecutable(t), "-test.run", "^TestLandingReplayReporterProcess$", "-test.timeout=30m")
		child.Dir, child.Stdout, child.Stderr = cmd.Dir, cmd.Stdout, cmd.Stderr
		child.Env = append(cmd.Env, "LANDING_REPLAY_REPORTER=1", "LANDING_REPLAY_FAILURES="+string(data))
		err = child.Run()
		if only == "" && b.move {
			b.main = "main-moved"
		}
		return err
	}
	return b
}

func TestARedProofIsToldFromMainsWithOneReplayRun(t *testing.T) {
	t.Parallel()
	for _, moved := range []bool{false, true} {
		t.Run(fmt.Sprintf("main-moved=%t", moved), func(t *testing.T) {
			t.Parallel()
			b := newOneGoalReplayBed(t)
			b.move = moved
			b.failures = []plain.FailedUnit{{Unit: "metasystem/internal/p", Tests: []string{"TestP"}}, {Unit: "metasystem/cmd/metasystem", Tests: []string{"TestGoal"}}, {Unit: "metasystem/internal/lease"}, {Unit: "fast-static-build"}}
			b.mainFailures = b.failures[:1]
			var incidents []plain.Result
			b.owners.landing.plainProve.RecordMain = func(results []plain.Result) error { incidents = append(incidents, results...); return nil }
			result := b.prove(t)
			selections := "fast-static-build metasystem/cmd/metasystem metasystem/internal/lease metasystem/internal/p"
			if result.Cause.Kind != "main" || !strings.Contains(result.Cause.Name, "internal/p") || len(result.Cause.Tests) != 4 || !slices.Contains(result.Cause.Tests, "metasystem/internal/lease (package)") || !reflect.DeepEqual(b.runs, []string{"merge-g:", "main:" + selections}) {
				t.Fatalf("main attribution: cause=%+v runs=%v", result.Cause, b.runs)
			}
			if len(incidents) != 1 || incidents[0].Commit != "main" || len(incidents[0].Failed) != 1 || incidents[0].Failed[0].Unit != "metasystem/internal/p" {
				t.Fatalf("main incident=%+v", incidents)
			}
			log, err := os.ReadFile(result.Cause.Evidence)
			helmMust(t, err)
			if !strings.Contains(string(log), "fixture groups fast-static-build") || strings.Count(string(log), "fixture native ") != 3 {
				t.Fatalf("reporter replay seams: %s", log)
			}
			batch, err := plain.ReadBatch(b.install)
			helmMust(t, err)
			if batch.State != plain.BatchRunning {
				t.Fatalf("main red closed batch: %+v", batch)
			}
			if code, out := b.run(t, b.root, "return", "g", "--cause", "own"); code != 1 || !strings.Contains(out, "main itself is red") {
				t.Fatalf("main red returned goal: %d %s", code, out)
			}
			b.main, b.parent, b.head, b.move = "main-fixed", "main-fixed", "merge-fixed", false
			b.mainFailures, b.failures = nil, b.failures[1:]
			b.runs = nil
			fixed := b.prove(t)
			if fixed.Cause.Kind != "own" || fixed.Cause.Goal != "g" || len(fixed.Cause.Tests) != 3 || len(b.runs) != 2 || !strings.HasPrefix(b.runs[1], "main-fixed:") {
				t.Fatalf("fixed main attribution: %+v runs=%v", fixed, b.runs)
			}
			if code, out := b.run(t, b.root, "return", "g", "--cause", "own"); code != 0 {
				t.Fatalf("own return exit=%d: %s", code, out)
			}
			entry, _, err := plain.Latest(b.install, "g")
			helmMust(t, err)
			for _, red := range []string{"metasystem/cmd/metasystem TestGoal", "metasystem/internal/lease (package)", "fast-static-build (package)"} {
				if !strings.Contains(entry.Reason, red) {
					t.Fatalf("return omitted %q: %+v", red, entry)
				}
			}
		})
	}
	for _, moved := range []bool{false, true} {
		t.Run(fmt.Sprintf("gate/main-moved=%t", moved), func(t *testing.T) {
			t.Parallel()
			b := newOneGoalReplayBed(t)
			script := filepath.Join(t.TempDir(), "cheap")
			helmMust(t, testexec.WriteFile(script, []byte("#!/bin/sh\nif [ \"$LANDING_PROOF_BASE\" != main ]; then printf 'bad base: %s\\n' \"$LANDING_PROOF_BASE\"; exit 2; fi\nif [ \"$LANDING_COMMIT\" = main ] && [ -z \"$LANDING_ONLY\" ]; then printf 'landing group unit/internal/p green 1\\nLANDING-CHECKED\\t0\\n'; exit 0; fi\nprintf 'landing group unit/internal/p red 1\\nLANDING-FAILED\\tmetasystem/internal/p\\tTestP\\nLANDING-CHECKED\\t1\\n'\nexit 1\n"), 0700))
			git := b.owners.landing.plainProve.Git
			b.owners.landing.plainProve.Git = func(dir string, args ...string) (string, error) {
				if len(args) == 2 && args[0] == "show" && strings.HasSuffix(args[1], ":metasystem/metasystem.conf") {
					return "proof.full=fixture\nproof.cheap=" + script + "\n", nil
				}
				return git(dir, args...)
			}
			b.owners.landing.plainProve.ImpactCost = func(_ string, plan string) (int, bool, int, string, error) {
				if !strings.Contains(plan, "selection: internal/p\n") {
					t.Fatalf("gate cost read the wrong plan: %s", plan)
				}
				return 10, true, 110, "", nil
			}
			var statics []string
			var incidents []plain.Result
			b.owners.landing.plainProve.RecordMain = func(results []plain.Result) error {
				incidents = append(incidents, results...)
				return nil
			}
			b.owners.landing.plainProve.Command = func(cmd *exec.Cmd) error {
				if len(cmd.Args) > 1 && cmd.Args[1] == "test" {
					writeImpactPlanResult(t, cmd, "plan: base main\nselection: internal/p\n")
					return nil
				}
				commit, only := commandEnv(cmd, "LANDING_COMMIT"), commandEnv(cmd, "LANDING_ONLY")
				if strings.HasSuffix(cmd.Args[len(cmd.Args)-1], " test groups fast-static-build") {
					statics = append(statics, commit)
					fmt.Fprint(cmd.Stdout, "landing environment fixture toolchain\nlanding group fast-static-build green 1\n")
					return nil
				}
				if commandEnv(cmd, "LANDING_PROOF_BASE") != "main" && only == "" {
					t.Fatalf("gate base=%q, want parent's commit", commandEnv(cmd, "LANDING_PROOF_BASE"))
				}
				// Replay has no selection base: give this recording script its pinned parent.
				if only != "" {
					cmd.Env = append(cmd.Env, "LANDING_PROOF_BASE=main")
				}
				b.runs = append(b.runs, commit+":"+only)
				err := cmd.Run()
				if commit == "merge-g" && moved {
					b.main = "main-moved"
				}
				return err
			}
			result := gateResult(t, b.replayVerbBed, 1)
			if result.Cause.Kind != "main" || !reflect.DeepEqual(b.runs, []string{"main:", "merge-g:", "main:metasystem/internal/p"}) {
				t.Fatalf("gate attribution: %+v runs=%v", result, b.runs)
			}
			if len(incidents) != 1 || incidents[0].Commit != "main" || len(incidents[0].Failed) != 1 || incidents[0].Failed[0].Unit != "metasystem/internal/p" {
				t.Fatalf("gate main incident=%+v", incidents)
			}
			if !reflect.DeepEqual(statics, []string{"merge-g", "main"}) || result.Static != plain.Green || result.Requested != "merge-g" || result.Attributed != "merge-g" {
				t.Fatalf("gate static checks and request: %+v statics=%v", result, statics)
			}
			status := gateStatus(t, b.replayVerbBed)
			if status.Result != plain.Red || status.Requested != "merge-g" || status.Attributed != "merge-g" || status.Cause == nil || status.Cause.Kind != "main" {
				t.Fatalf("gate status lost the merge's main failure: %+v", status)
			}
		})
	}
	for _, scenario := range []string{"red-green-moved", "two-reds", "flake-green"} {
		t.Run("budget/"+scenario, func(t *testing.T) {
			t.Parallel()
			b := newOneGoalReplayBed(t)
			b.failures = []plain.FailedUnit{{Unit: "metasystem/internal/p", Tests: []string{"TestP"}}}
			first := b.prove(t)
			if first.Cause.Kind != "own" {
				t.Fatalf("first red: %+v", first)
			}
			if code, out := b.run(t, b.root, "return", "g", "--cause", "own"); code != 0 {
				t.Fatalf("first return: %d %s", code, out)
			}
			b.sha = "sha-g-fixed"
			_, _, err := plain.HandIn(b.install, plain.Line{Goal: "g", SHA: b.sha})
			helmMust(t, err)
			b.head = "merge-second"
			registered, _, err := lane.Read(b.home)
			helmMust(t, err)
			selected, err := plain.SelectBatch(b.install, b.root, registered, b.owners.landing.plainProve)
			helmMust(t, err)
			if selected == nil || len(selected.Members) != 1 || selected.Members[0].SHA != b.sha {
				t.Fatalf("fixed hand-in selection: %+v", selected)
			}
			if scenario == "red-green-moved" {
				b.failures = nil
			}
			if scenario == "flake-green" {
				b.owners.landing.plainProve.Judge = func(string, string, []plain.FailedUnit) (map[string]plain.UnitJudgement, error) {
					return map[string]plain.UnitJudgement{"metasystem/internal/p": {Known: true}}, nil
				}
				b.owners.landing.plainProve.RecordFlake = func(plain.FlakeRecord) (plain.FlakeRecorded, error) {
					return plain.FlakeRecorded{Goal: "fix-flake", Seen: 1}, nil
				}
				command := b.owners.landing.plainProve.Command
				b.owners.landing.plainProve.Command = func(cmd *exec.Cmd) error {
					failures := b.failures
					if commandEnv(cmd, "LANDING_ONLY") != "" {
						b.failures = nil
					}
					err := command(cmd)
					b.failures = failures
					return err
				}
			}
			code, out := b.run(t, b.root, "prove", "--wait", "--json")
			if scenario == "two-reds" {
				if code != 1 || b.full != 2 {
					t.Fatalf("second red: %d %s", code, out)
				}
			} else if code != 0 {
				t.Fatalf("second green: %d %s", code, out)
			}
			b.head, b.main, b.parent = "merge-third", "main-new", "main-new"
			b.failures = nil
			calls := b.full
			code, out = b.run(t, b.root, "prove", "--wait", "--json")
			if scenario == "two-reds" {
				if code != 1 || b.full != calls || !strings.Contains(out, "metasystem landing prove") || !strings.Contains(out, "two full checks") || strings.Contains(out, "metasystem landing run") {
					t.Fatalf("budget hold: %d %s runs=%d", code, out, b.full)
				}
				b.owners.resolver = stateroot.NewResolver(func(string) (string, error) { return b.root, nil }, os.Executable)
				layout, err := b.owners.resolver.ResolveLayout(b.root)
				helmMust(t, err)
				b.owners.prove = enrolledPersonProver(t, checkoutAuthorityRoot(layout), laneTestNow)
				code, out = b.run(t, b.root, "prove", "--wait", "--json")
				if code != 0 || b.full != calls+1 {
					t.Fatalf("person's extra proof: %d %s runs=%d", code, out, b.full)
				}
			} else if code != 0 || b.full != calls+1 {
				t.Fatalf("green spent an attempt: %d %s runs=%d", code, out, b.full)
			}
		})
	}

}
