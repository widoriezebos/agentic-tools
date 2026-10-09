package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

type unitProofStarter struct {
	bed *workBed
	t   *testing.T
}

func (s unitProofStarter) StartSupervisor(id, state string) (identity.Ref, error) {
	record, err := s.bed.manager.Store.Read(id)
	if err != nil {
		return identity.Ref{}, err
	}
	if record.Kind == "proof" {
		spec, err := (launch.PlainExec{}).Command(record, state)
		if err != nil {
			return identity.Ref{}, err
		}
		code, result := declaredCheckAt(s.t, s.bed, spec.Directory, "proof", spec.Args...)
		var data []byte
		for _, raw := range resultData(s.t, result)["exits"].([]any) {
			command := raw.(map[string]any)
			data = append(data, []byte(command["output"].(string))...)
		}
		s.bed.starter.fail["proof"] = code != 0
		if err := os.WriteFile(spec.LogPath, data, 0600); err != nil {
			return identity.Ref{}, err
		}
		if _, err := s.bed.manager.Store.Update(id, func(r *launch.Record) error { child := workProcessRef(20); r.Child = &child; return nil }); err != nil {
			return identity.Ref{}, err
		}
	}
	return s.bed.starter.StartSupervisor(id, state)
}

func TestUnitBuildRepeatsFlakeFromMainRegister(t *testing.T) {
	t.Parallel()
	unitFlakeBuild(t, false, false)
}

func TestUnitBuildAttributesAffectedRegisteredFlake(t *testing.T) {
	t.Parallel()
	unitFlakeBuild(t, true, false)
}

func TestUnitBuildAttributesUnitNewOnMainAfterJudgeError(t *testing.T) {
	t.Parallel()
	unitFlakeBuild(t, true, true)
}

func unitFlakeBuild(t *testing.T, affected, newOnMain bool) *workBed {
	t.Helper()
	bed := newWorkBed(t)
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
				if newOnMain {
					return goal.RenderTrunkRed(nil), nil
				}
				return goal.RenderTrunkRed([]goal.TrunkRedEntry{entry}), nil
			case strings.Join(args, " ") == "diff --name-only origin/main...HEAD":
				if affected {
					return []byte("u/a/unit.go\n"), nil
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
	if err := os.WriteFile(filepath.Join(bed.worktree, "u", "a", "red"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\nif [ ! -f red ]; then exit 0; fi\nif [ -f '" + counter + "' ]; then exit 0; fi\ntouch '" + counter + "'\nprintf 'LANDING-FAILED\\tu/a\\tTestBroken\\nLANDING-CHECKED\\t1\\n'\nexit 1\n"
	if affected {
		script = "#!/bin/sh\nif [ ! -f red ]; then exit 0; fi\nprintf 'LANDING-FAILED\\tu/a\\tTestBroken\\nLANDING-CHECKED\\t1\\n'\nexit 1\n"
	}
	if err := testexec.WriteFile(check, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	brief := filepath.Join(bed.root(), bed.brief("flake.md", "Build the unit.\n"))
	bed.declaredCheap = shellCommand([]string{check})
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
	if affected {
		outcome, wantCode = "proof-red", 1
	}
	if code != wantCode || resultData(t, result)["outcome"] != outcome || reads != 2 {
		t.Fatalf("build exit=%d result=%+v main reads=%d", code, result, reads)
	}
	if affected {
		record, err := (&launch.UnitRunner{Root: bed.unitRoot}).Status(resultData(t, result)["run"].(string))
		if err != nil || record.Rounds[0].Cause != "own" || record.Rounds[0].Steps[1].FlakeRepeat || record.Rounds[0].Steps[1].Comparison == nil {
			t.Fatalf("failed check attribution: %+v err=%v", record, err)
		}
		if newOnMain {
			step := record.Rounds[0].Steps[1]
			if missingUnitReads != 1 || step.Cause != "own" || !step.Comparison.Verified {
				t.Fatalf("new unit attribution: missing unit reads=%d step=%+v", missingUnitReads, step)
			}
			base, err := bed.manager.Store.Read(step.Comparison.LaunchID)
			if err != nil || base.State != launch.Completed || base.ExitCode == nil || *base.ExitCode != 0 {
				t.Fatalf("new unit base comparison: %+v err=%v", base, err)
			}
		}
	}
	kinds := bed.starter.launched()
	if strings.Join(kinds, ",") != "build,proof,proof" {
		t.Fatalf("launches=%v", kinds)
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
