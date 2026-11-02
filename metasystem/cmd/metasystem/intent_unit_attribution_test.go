package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

type unitProofStarter struct {
	bed    *workBed
	t      *testing.T
	before func(launch.Record)
}

func (s unitProofStarter) StartSupervisor(id, state string) (identity.Ref, error) {
	record, err := s.bed.manager.Store.Read(id)
	if err != nil {
		return identity.Ref{}, err
	}
	if record.Kind == "proof" {
		if s.before != nil {
			s.before(record)
		}
		spec, err := (launch.PlainExec{}).Command(record, state)
		if err != nil {
			return identity.Ref{}, err
		}
		child := identity.Exact{Pid: int64(os.Getpid()), StartedAt: s.bed.manager.Now()}.Ref()
		if _, err := s.bed.manager.Store.Update(id, func(r *launch.Record) error { r.Child = &child; return nil }); err != nil {
			return identity.Ref{}, err
		}
		code := 0
		var data []byte
		if _, _, public := resolveIntentArgv(spec.Args); public {
			var result intentResult
			code, result = declaredCheckAt(s.t, s.bed, spec.Directory, "proof", spec.Args...)
			for _, raw := range resultData(s.t, result)["exits"].([]any) {
				data = append(data, []byte(raw.(map[string]any)["output"].(string))...)
			}
		} else {
			command := exec.Command(spec.Program, spec.Args...)
			command.Dir, command.Env = spec.Directory, append(os.Environ(), spec.Environment...)
			data, err = command.CombinedOutput()
			var exit *exec.ExitError
			if errors.As(err, &exit) {
				code = exit.ExitCode()
			} else if err != nil {
				return identity.Ref{}, err
			}
		}
		s.bed.starter.fail["proof"] = code != 0
		if err := os.WriteFile(spec.LogPath, data, 0600); err != nil {
			return identity.Ref{}, err
		}
		if _, err := s.bed.manager.Store.Update(id, func(r *launch.Record) error { child := workProcessRef(20); r.Child = &child; return nil }); err != nil {
			return identity.Ref{}, err
		}
	}
	supervisor, err := s.bed.starter.StartSupervisor(id, state)
	if err == nil && record.Kind == "read" {
		_, err = s.bed.manager.Store.Update(id, func(r *launch.Record) error { r.Measurement.Verdict = "VERDICT: LAND"; return nil })
	}
	return supervisor, err
}

type unitProofProber struct {
	workProber
	exact identity.Exact
}

func (p unitProofProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	if pid == p.exact.Pid {
		return p.exact, identity.Alive, nil
	}
	return p.workProber.Probe(pid)
}

func TestAUnitCheckFlakeStopsNothingAndAsksNobody(t *testing.T) {
	t.Parallel()
	bed := unitFlakeBuild(t, true, false, true)
	paths, err := filepath.Glob(filepath.Join(bed.unitRoot, "*", "run.json"))
	if err != nil || len(paths) != 1 {
		t.Fatalf("runs=%v err=%v", paths, err)
	}
	record, err := (&launch.UnitRunner{Root: bed.unitRoot}).Status(filepath.Base(filepath.Dir(paths[0])))
	if err != nil {
		t.Fatal(err)
	}
	round := record.Rounds[0]
	if round.Cause != "" || round.Stop == nil || round.Stop.Loop != "unit-round" || round.Stop.Decision != "close" || round.Steps[2].Verdict != "VERDICT: LAND" || round.Steps[2].State != launch.StepPassed || len(round.Reads) != 1 || round.Reads[0].ID != round.Steps[2].LaunchID {
		t.Fatalf("flake did not reach a clean read: %+v", round)
	}
	read, err := bed.manager.Store.Read(round.Steps[2].LaunchID)
	if err != nil || read.State != launch.Completed || len(read.Outputs) == 0 {
		t.Fatalf("read result not collected: %+v err=%v", read, err)
	}
	executions, unknown := launch.ReadCheckExecutions(round.Directory)
	if len(unknown) != 0 || len(executions) != 1 {
		t.Fatalf("check executions=%+v unknown=%v", executions, unknown)
	}
	for _, execution := range executions {
		if !slices.Contains(round.Steps[1].LaunchIDs, execution.Steps[0].Parent) {
			t.Fatalf("check has no parent launch: %+v", execution)
		}
	}
	questions, problems := channel.WalkQuestions(bed.root())
	if len(questions) != 0 || len(problems) != 0 {
		t.Fatalf("flake asked somebody: %+v %v", questions, problems)
	}
	if _, err := os.Stat(filepath.Join(round.Directory, "stop-register.json")); !os.IsNotExist(err) {
		t.Fatalf("flake left a stop register: %v", err)
	}
}

func TestUnitBuildRepeatsFlakeFromMainRegister(t *testing.T) {
	t.Parallel()
	unitFlakeBuild(t, false, false, false)
}

func TestUnitBuildAttributesAffectedRegisteredFlake(t *testing.T) {
	t.Parallel()
	unitFlakeBuild(t, true, false, false)
}

func TestUnitBuildAttributesUnitNewOnMainAfterJudgeError(t *testing.T) {
	t.Parallel()
	unitFlakeBuild(t, true, true, false)
}

func unitFlakeBuild(t *testing.T, affected, newOnMain, flake bool) *workBed {
	t.Helper()
	bed := newWorkBed(t)
	bed.manager.Prober = unitProofProber{exact: identity.Exact{Pid: int64(os.Getpid()), StartedAt: bed.manager.Now()}}
	bed.manager.Adapters["plain-exec"] = launch.PlainExec{}
	bed.manager.Supervisor = unitProofStarter{bed: bed, t: t}
	if err := os.WriteFile(filepath.Join(bed.root(), "metasystem.conf"), []byte("testing.contract=testing.json\n"), 0600); err != nil {
		t.Fatal(err)
	}
	contract := testpolicy.Contract{SchemaVersion: 1,
		ProjectRisk: testpolicy.ProjectRisk{Severity: 1, Exposure: 1, Reversibility: "revert", Detection: "immediate", Recovery: "bounded"},
		Fallback:    "unit", Surfaces: []testpolicy.Surface{{ID: "unit", Standard: []string{"check"}}},
		Groups: []testpolicy.Group{{ID: "check", Kind: "unit", Adapter: "go", CWD: ".", Inputs: []string{"u/**"}, Platforms: []string{"any"}, TargetMS: 1, Packages: []string{"u/a"}, Tests: json.RawMessage(`"all"`)}},
		Always: testpolicy.Always{Canary: []string{"check"}}, Unknown: []string{"check"}, Cadence: []string{"check"}}
	data, err := json.Marshal(contract)
	if err != nil {
		t.Fatal(err)
	}
	entry := goal.TrunkRedEntry{ID: "flaky:u/a", Identity: "flaky:u/a", Group: "u/a", Status: "failed", Class: goal.TrunkRedClassPendingFlake,
		Failures: []goal.TrunkRedFailure{{Report: "red.log", Classname: "u/a", Name: "TestBroken"}}, Holds: []string{}, Opened: "2026-10-04T10:00:00Z",
		Sightings: []goal.TrunkRedSighting{{Attempt: "red", BaseCommit: "old", SeenAt: "2026-10-04T10:00:00Z", Opid: "01J5X0000000000000000000F1-lane-12345678"}}}
	reads := 0
	missingUnitReads := 0
	bed.workOwnersHook = func(owners *intentWorkOwners) {
		units := owners.units
		owners.units = func(layout stateroot.Layout) *launch.UnitRunner {
			runner := units(layout)
			runner.Git = &unitProofBaseGit{workGit: workGit{bed}}
			return runner
		}
		previous := owners.git
		owners.git = func(dir string, args ...string) ([]byte, error) {
			switch {
			case strings.Join(args, " ") == "rev-parse --show-toplevel":
				return []byte(bed.worktree + "\n"), nil
			case len(args) == 2 && args[0] == "show" && args[1] == "origin/main:testing.json":
				if dir != bed.worktree {
					t.Fatalf("main register read in %s, want repository root %s", dir, bed.worktree)
				}
				reads++
				return data, nil
			case len(args) == 2 && args[0] == "show" && args[1] == "origin/main:plans/goals/trunk-red.json":
				if dir != bed.worktree {
					t.Fatalf("main register read in %s, want repository root %s", dir, bed.worktree)
				}
				reads++
				if newOnMain || flake {
					return goal.RenderTrunkRed(nil), nil
				}
				return goal.RenderTrunkRed([]goal.TrunkRedEntry{entry}), nil
			case strings.Join(args, " ") == "diff --name-only origin/main...HEAD":
				if affected {
					return []byte("u/a/unit.go\nu/b/unit.go\n"), nil
				}
				return nil, nil
			case strings.Join(args, " ") == "ls-tree -z origin/main:u/a":
				if newOnMain {
					missingUnitReads++
					return nil, errors.New("git ls-tree: exit status 128: u/a does not exist on origin/main")
				}
				return []byte("100644 blob abc\tunit.go\x00"), nil
			}
			return previous(dir, args...)
		}
	}
	check := filepath.Join(t.TempDir(), "check")
	counter := filepath.Join(t.TempDir(), "counter")
	for _, root := range []string{bed.root(), bed.worktree} {
		if err := os.MkdirAll(filepath.Join(root, "u", "a"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	// A declared check runs from the worktree's installation folder (U3 item
	// 11), which in this bed is the checkout root, whatever folder the build
	// is called from.
	if err := os.WriteFile(filepath.Join(bed.worktree, "red"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\nif [ ! -f red ] || [ -f '" + counter + "' ]; then printf 'LANDING-CHECKED\\t0\\n'; exit 0; fi\ntouch '" + counter + "'\nprintf 'LANDING-FAILED\\tu/a\\tTestBroken\\nLANDING-CHECKED\\t1\\n'\nexit 1\n"
	if affected && !flake {
		script = "#!/bin/sh\nif [ ! -f red ]; then printf 'LANDING-CHECKED\\t0\\n'; exit 0; fi\nprintf 'LANDING-FAILED\\tu/a\\tTestBroken\\nLANDING-CHECKED\\t1\\n'\nexit 1\n"
	}
	if flake {
		script = strings.Replace(script, "if [ ! -f red ]", "if [ -n \"$LANDING_ONLY\" ]; then\nfor pair in u/a=TestA u/b=TestB u/a=TestOther; do\ncase \" $LANDING_ONLY \" in *\" $pair \"*) printf '%s %s\\n' \"${pair%=*}\" \"${pair#*=}\" >> '"+counter+".selected';; esac\ndone\n[ \"$LANDING_ONLY_TESTS\" = 'TestA TestB' ] || exit 3\nfi\nif [ ! -f red ]", 1)
		script = strings.ReplaceAll(script, "LANDING-FAILED\\tu/a\\tTestBroken\\nLANDING-CHECKED\\t1", "LANDING-FAILED\\tu/a\\tTestA\\nLANDING-FAILED\\tu/b\\tTestB\\nLANDING-CHECKED\\t2")
	}
	if err := testexec.WriteFile(check, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	text := "Build the unit.\n"
	if flake {
		text += "\nRead each round: yes\n"
	}
	brief := filepath.Join(bed.root(), bed.brief("flake.md", text))
	bed.declaredCheap = shellCommand([]string{check})
	bed.declaredFull = bed.declaredCheap + " # full"
	command, rest, ok := resolveIntentArgv([]string{"work", "build", bed.id, "flake", "--brief", brief, "--lines", "5"})
	if !ok {
		t.Fatal("build command unavailable")
	}
	var stdout, stderr bytes.Buffer
	code := runIntentIn(command, append([]string{"--json"}, rest...), &stdout, &stderr, filepath.Join(bed.root(), "u", "a"), bed.workOwners())
	var result intentResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("build JSON: %v stdout=%s stderr=%s", err, &stdout, &stderr)
	}
	outcome, wantCode := "green", 0
	if affected && !flake {
		outcome, wantCode = "proof-red", 1
	}
	wantReads := 2
	if affected && !flake {
		wantReads = 4
	}
	if code != wantCode || resultData(t, result)["outcome"] != outcome || reads != wantReads {
		t.Fatalf("build exit=%d result=%+v main reads=%d", code, result, reads)
	}
	if affected && !flake {
		record, err := (&launch.UnitRunner{Root: bed.unitRoot}).Status(resultData(t, result)["run"].(string))
		if err != nil || record.Rounds[0].Cause != "own" || !record.Rounds[0].Steps[1].FlakeRepeat || record.Rounds[0].Steps[1].Comparison == nil {
			t.Fatalf("failed check attribution: %+v err=%v", record, err)
		}
		if newOnMain {
			step := record.Rounds[0].Steps[1]
			if missingUnitReads != 2 || step.Cause != "own" || !step.Comparison.Verified {
				t.Fatalf("new unit attribution: missing unit reads=%d step=%+v", missingUnitReads, step)
			}
			base, err := bed.manager.Store.Read(step.Comparison.LaunchID)
			if err != nil || base.State != launch.Completed || base.ExitCode == nil || *base.ExitCode != 0 {
				t.Fatalf("new unit base comparison: %+v err=%v", base, err)
			}
		}
	}
	kinds := bed.starter.launched()
	wantKinds := "build,proof,proof"
	if affected && !flake {
		wantKinds += ",proof"
	}
	if flake {
		wantKinds += ",read"
	}
	if strings.Join(kinds, ",") != wantKinds {
		t.Fatalf("launches=%v", kinds)
	}
	input, problem := parseIntentArgs(command, rest)
	if problem != nil {
		t.Fatal(problem)
	}
	inv := &intentInvocation{command: command, input: input, raw: rest, owners: bed.workOwners(), cwd: bed.root(), stdout: &stdout, stderr: &stderr}
	factsRunner := inv.unitRunner()
	paths, _ := filepath.Glob(filepath.Join(bed.unitRoot, "*", "run.json"))
	if len(paths) != 1 || factsRunner.RedUnits == nil {
		t.Fatalf("missing run or facts reader: %v", paths)
	}
	record, err := factsRunner.Status(filepath.Base(filepath.Dir(paths[0])))
	if err != nil {
		t.Fatal(err)
	}
	first, err := bed.manager.Store.Read(record.Rounds[0].Steps[1].LaunchIDs[0])
	if err != nil {
		t.Fatal(err)
	}
	units, complete, err := factsRunner.RedUnits(first)
	registration := ""
	if !newOnMain && !flake {
		registration = goal.FlakeIdentity("u/a", "TestBroken")
	}
	wantUnits, wantTest := 1, "TestBroken"
	if flake {
		wantUnits, wantTest = 2, "TestA"
	}
	if err != nil || !complete || len(units) != wantUnits || !slices.Equal(units[0].Tests, []string{wantTest}) || !slices.Equal(units[0].Registered, []string{registration}) {
		t.Fatalf("red facts=%+v complete=%v err=%v", units, complete, err)
	}
	step := record.Rounds[0].Steps[1]
	if step.Alone == nil || step.Alone.LaunchID != step.LaunchID+"-alone" || !slices.Equal(step.LaunchIDs, []string{step.LaunchID, step.Alone.LaunchID}) {
		t.Fatalf("second proof is not the retained rerun alone: %+v", step)
	}
	if flake {
		if step.Alone.Outcome != string(launch.Completed) || !slices.Equal(step.Flakes, []string{"u/a TestA", "u/b TestB"}) || !strings.Contains(step.Reason, "passed alone on this tree") {
			t.Fatalf("flake evidence=%+v", step)
		}
		selection, err := os.ReadFile(counter + ".selected")
		if err != nil || string(selection) != "u/a TestA\nu/b TestB\n" {
			t.Fatalf("rerun selection=%q err=%v", selection, err)
		}
		reservation, err := os.ReadFile(filepath.Join(bed.root(), "artifacts", "agents", "jobs", step.Alone.LaunchID+".json"))
		if err != nil || !bytes.Contains(reservation, []byte(`"unitExecuted": true`)) || !bytes.Contains(reservation, []byte(`"status": "completed"`)) {
			t.Fatalf("rerun was not collected: %s err=%v", reservation, err)
		}
	}
	return bed
}

type unitProofBaseGit struct {
	workGit
	directory string
}

func (git *unitProofBaseGit) Run(dir string, env []string, args ...string) ([]byte, error) {
	if len(args) == 3 && slices.Equal(args[:2], []string{"init", "--quiet"}) {
		git.directory = args[2]
		if err := os.MkdirAll(filepath.Join(git.directory, "u", "a"), 0700); err != nil {
			return nil, err
		}
		if err := os.MkdirAll(filepath.Join(git.directory, ".git", "objects", "info"), 0700); err != nil {
			return nil, err
		}
		return nil, os.WriteFile(filepath.Join(git.directory, ".git", "index"), []byte("index"), 0600)
	}
	if slices.Equal(args, []string{"rev-parse", "--verify", git.bed.head + "^{tree}"}) || slices.Equal(args, []string{"rev-parse", "--verify", "HEAD^{tree}"}) {
		return []byte("base-tree"), nil
	}
	if slices.Equal(args, []string{"rev-parse", "--verify", git.bed.head + "^{commit}"}) {
		return []byte(git.bed.head), nil
	}
	if git.directory != "" && (dir == git.directory || strings.HasPrefix(dir, git.directory+string(filepath.Separator))) {
		if slices.Equal(args, []string{"rev-parse", "--show-toplevel"}) {
			return []byte(git.directory), nil
		}
		if slices.Equal(args, []string{"rev-parse", "--path-format=absolute", "--git-path", "index"}) {
			return []byte(filepath.Join(git.directory, ".git", "index")), nil
		}
		if slices.Equal(args, []string{"rev-parse", "--path-format=absolute", "--git-path", "objects"}) {
			return []byte(filepath.Join(git.directory, ".git", "objects")), nil
		}
	}
	return git.workGit.Run(dir, env, args...)
}
