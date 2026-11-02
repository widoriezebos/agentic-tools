package launch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/loopstop"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/processmeasure"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testgit"
)

type attributionGit struct {
	fixture        unitFixture
	tree, mainTree string
	baseRed        bool
	rootNext       bool
	baseline       *stubGit
	exports        int
}

func (git *attributionGit) Run(dir string, env []string, args ...string) ([]byte, error) {
	if slices.Equal(args, []string{"rev-parse", "--verify", "base^{tree}"}) {
		git.rootNext = true
		return []byte(git.tree), nil
	}
	if slices.Equal(args, []string{"rev-parse", "--show-toplevel"}) && dir == git.fixture.worktree && git.rootNext {
		git.rootNext = false
		return []byte(dir), nil
	}
	if slices.Equal(args, []string{"rev-parse", "--verify", "base^{commit}"}) {
		return []byte("base-commit"), nil
	}
	if dir == git.fixture.worktree && git.exports > 0 && git.fixture.git.snapshotNext == 0 && slices.Equal(args, []string{"rev-parse", "--path-format=absolute", "--git-path", "objects"}) {
		return git.fixture.git.snapshot[4].Result.Stdout, nil
	}
	if len(args) == 3 && slices.Equal(args[:2], []string{"init", "--quiet"}) {
		git.exports++
		root := args[2]
		if err := os.MkdirAll(filepath.Join(root, ".git", "objects", "info"), 0700); err != nil {
			return nil, err
		}
		if err := os.WriteFile(filepath.Join(root, ".git", "index"), []byte("index"), 0600); err != nil {
			return nil, err
		}
		expected := slices.Clone(git.fixture.git.snapshot)
		expected[3].Result.Stdout = []byte(filepath.Join(root, ".git", "index") + "\n")
		expected[4].Result.Stdout = []byte(filepath.Join(root, ".git", "objects") + "\n")
		for i := range expected {
			expected[i].Call.Dir = root
			if i == 0 {
				expected[i].Result.Stdout = []byte(root + "\n")
			}
			if expected[i].Check != nil {
				expected[i].Check = isolatedGitEnvironment(root, strings.TrimSpace(string(expected[4].Result.Stdout)), true)
			}
		}
		git.baseline = &stubGit{worktree: root, snapshot: expected, reporter: git.fixture.git.reporter, makeStub: func() *testgit.Stub { return testgit.New(git.fixture.git.reporter) }}
		return nil, nil
	}
	if slices.Equal(args, []string{"update-ref", "--no-deref", "HEAD", "base-commit"}) {
		return nil, nil
	}
	if slices.Equal(args, []string{"read-tree", "--reset", "-u", git.tree}) {
		if git.baseRed {
			return nil, os.WriteFile(filepath.Join(dir, "red"), nil, 0600)
		}
		return nil, nil
	}
	if slices.Equal(args, []string{"rev-parse", "--verify", "HEAD^{tree}"}) {
		return []byte(git.tree), nil
	}
	if slices.Equal(args, []string{"rev-parse", "--verify", "origin/main"}) {
		return []byte("main-commit"), nil
	}
	if slices.Equal(args, []string{"rev-parse", "--verify", "main-commit^{tree}"}) {
		return []byte(git.mainTree), nil
	}
	if dir != git.fixture.worktree {
		return git.baseline.Run(dir, env, args...)
	}
	return git.fixture.git.Run(dir, env, args...)
}

func attributionFixture(t *testing.T, baseRed bool, script string, resume ...bool) (unitFixture, *attributionGit) {
	t.Helper()
	events := []string{"branch", "round"}
	for range resume {
		events = append(events, "branch")
	}
	f := newUnitFixture(t, "", events...)
	git := &attributionGit{fixture: f, tree: "base-tree", mainTree: "main-tree", baseRed: baseRed}
	f.runner.Git = git
	f.manager.Adapters["plain-exec"] = PlainExec{}
	executable := filepath.Join(t.TempDir(), "check")
	if script == "" {
		script = "[ \"$CHECK_ENV\" = exact ] && [ \"$1\" = 'arg with spaces' ] || exit 3\nif [ -f red ]; then printf 'LANDING-FAILED\\tpkg/unit\\tTestRed\\nLANDING-CHECKED\\t1\\n'; exit 1; fi\n"
	}
	if err := testexec.WriteFile(executable, []byte("#!/bin/sh\n"+script), 0700); err != nil {
		t.Fatal(err)
	}
	plan, err := ReadUnitPlan(f.plan)
	if err != nil {
		t.Fatal(err)
	}
	plan.Read = UnitReadPlan{}
	plan.Proof[0].Argv, plan.Proof[0].Env = []string{executable, "arg with spaces"}, []string{"CHECK_ENV=exact"}
	f.runner.AdmitEstimate = func(plan *UnitPlan) error { plan.FullArgv = slices.Clone(plan.Proof[0].Argv); return nil }
	f.runner.RedUnits = func(record Record) ([]loopstop.RedUnit, bool, error) {
		state, err := f.manager.Store.StateDir(record.ID)
		if err != nil {
			return nil, false, err
		}
		data, err := os.ReadFile(filepath.Join(state, "exec.log"))
		var units []loopstop.RedUnit
		for _, line := range strings.Split(string(data), "\n") {
			fields := strings.Split(line, "\t")
			if len(fields) == 3 && fields[0] == "LANDING-FAILED" {
				units = append(units, loopstop.RedUnit{Unit: fields[1], Tests: strings.Fields(fields[2])})
			}
		}
		complete := strings.HasSuffix(string(data), fmt.Sprintf("LANDING-CHECKED\t%d\n", len(units)))
		if !complete {
			units = nil
		}
		return units, complete, err
	}
	data, _ := json.Marshal(plan)
	if err := os.WriteFile(f.plan, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.worktree, "red"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	f.starter.onStart = func(record Record) error {
		if record.Kind != "proof" {
			return nil
		}
		state, err := f.manager.Store.StateDir(record.ID)
		if err != nil {
			return err
		}
		spec, err := (PlainExec{}).Command(record, state)
		if err != nil {
			return err
		}
		command := exec.Command(spec.Program, spec.Args...)
		command.Dir, command.Env = spec.Directory, childEnvironment(os.Environ(), spec.Environment)
		data, err := command.CombinedOutput()
		var exit *exec.ExitError
		if err != nil && !errors.As(err, &exit) {
			return err
		}
		f.starter.failKind = ""
		if err != nil {
			f.starter.failKind = "proof"
		}
		if _, updateErr := f.manager.Store.Update(record.ID, func(r *Record) error { child := ref(20); r.Child = &child; return nil }); updateErr != nil {
			return updateErr
		}
		return os.WriteFile(spec.LogPath, data, 0600)
	}
	return f, git
}

func TestAFailedCheckSectionIsNeverAFlake(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"audits", "cheap-incomplete", "unreadable", "missing", "other-parent", "newest"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			counter := filepath.Join(t.TempDir(), "counter")
			script := fmt.Sprintf("if [ -f '%s' ]; then printf 'LANDING-CHECKED\\t0\\n'; exit 0; fi\ntouch '%s'\nprintf 'LANDING-FAILED\\tpkg/unit\\tTestRed\\nLANDING-CHECKED\\t1\\n'\nexit 1\n", counter, counter)
			if scenario == "cheap-incomplete" {
				script = strings.ReplaceAll(script, "LANDING-CHECKED\\t1\\n", "")
			}
			f, git := attributionFixture(t, false, script)
			plan, err := ReadUnitPlan(f.plan)
			if err != nil {
				t.Fatal(err)
			}
			plan.Check = &UnitCheck{Cheap: "check", Audits: "audits", Minutes: 1, Directory: f.worktree, Environment: []string{}}
			data, _ := json.Marshal(plan)
			if err := os.WriteFile(f.plan, data, 0600); err != nil {
				t.Fatal(err)
			}
			start := f.starter.onStart
			f.starter.onStart = func(record Record) error {
				if err := start(record); err != nil {
					return err
				}
				if record.Kind != "proof" || strings.HasSuffix(record.ID, "-base") || scenario == "missing" {
					return nil
				}
				round := filepath.Join(f.runner.Root, strings.Split(record.ID, "-r1-")[0], "round-1")
				parent := record.ID
				if scenario == "other-parent" {
					parent = "different-launch"
				}
				observation := CheckExecution{ExecutionID: "check-sections", Steps: []processmeasure.Step{
					{ID: record.ID + ":cheap", Parent: parent, Start: f.manager.Now().Format(time.RFC3339Nano), Outcome: "failed", Terminal: true},
					{ID: record.ID + ":audits", Parent: parent, Outcome: "failed", Terminal: true},
				}}
				if scenario == "cheap-incomplete" {
					observation.Steps[1].Outcome = "passed"
				}
				if scenario == "newest" {
					older := observation
					older.ExecutionID = "check-older"
					older.Steps = slices.Clone(observation.Steps)
					older.Steps[0].Start = f.manager.Now().Add(-time.Minute).Format(time.RFC3339Nano)
					older.Steps[1].Outcome = "passed"
					if err := writeUnitJSON(filepath.Join(round, older.ExecutionID, "observation.json"), older, f.runner.Root); err != nil {
						return err
					}
				}
				path := filepath.Join(round, observation.ExecutionID, "observation.json")
				if err := writeUnitJSON(path, observation, f.runner.Root); err != nil {
					return err
				}
				if scenario == "unreadable" {
					return os.WriteFile(path, []byte("{"), 0600)
				}
				return nil
			}
			result, err := f.runner.Advance(UnitRequest{Plan: f.plan})
			if err != nil {
				t.Fatal(err)
			}
			step := result.Record.Rounds[0].Steps[1]
			cause, exports, launches := "own", 1, 3
			if scenario == "unreadable" || scenario == "missing" || scenario == "other-parent" {
				cause, exports, launches = "unclassified", 0, 2
			}
			if step.State != StepFailed || step.FlakeRepeat || step.Cause != cause || git.exports != exports || len(f.starter.ids) != launches || result.Record.Rounds[0].Outcome != "proof-red" {
				t.Fatalf("failed section lost: step=%+v exports=%d launches=%v", step, git.exports, f.starter.ids)
			}
			if (scenario == "missing" || scenario == "other-parent") && !strings.Contains(step.Reason, "no frozen-check execution of launch "+step.LaunchID) {
				t.Fatalf("missing execution did not name its launch: %s", step.Reason)
			}
		})
	}
}

func TestUnitProofAttributesBaseAndOwn(t *testing.T) {
	t.Parallel()
	for _, baseRed := range []bool{true, false} {
		t.Run(fmt.Sprint(baseRed), func(t *testing.T) {
			t.Parallel()
			f, git := attributionFixture(t, baseRed, "")
			incidents := 0
			f.runner.RecordMain = func(Record, string, string) error { incidents++; return nil }
			result, err := f.runner.Advance(UnitRequest{Plan: f.plan})
			if err != nil {
				t.Fatal(err)
			}
			cause, charged := "own", 1
			if baseRed {
				cause, charged = "other", 0
			}
			round := result.Record.Rounds[0]
			count, _ := countedRounds(result.Record)
			if round.Cause != cause || count != charged || round.Stop == nil || incidents != 0 || git.exports != 1 || len(f.starter.ids) != 4 {
				t.Fatalf("cause=%s counted=%d incident=%d exports=%d launches=%v round=%+v", round.Cause, count, incidents, git.exports, f.starter.ids, round)
			}
		})
	}
}

func TestUnitProofRecoversRetainedComparison(t *testing.T) {
	t.Parallel()
	f, git := attributionFixture(t, false, "", true)
	start := f.starter.onStart
	f.starter.onStart = func(r Record) error {
		if err := start(r); err != nil {
			return err
		}
		if strings.HasSuffix(r.ID, "-base") {
			f.starter.holdKind = "proof"
		}
		return nil
	}
	first, err := f.runner.Advance(UnitRequest{Plan: f.plan})
	if err != nil || !first.Capped {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	comparison := first.Record.Rounds[0].Steps[1].Comparison
	_, err = f.manager.Store.Update(comparison.LaunchID, func(r *Record) error { code := 0; r.State, r.ExitCode = Completed, &code; return nil })
	if err != nil {
		t.Fatal(err)
	}
	second, err := f.runner.Advance(UnitRequest{Resume: first.Record.ID})
	if err != nil || second.Record.Rounds[0].Cause != "own" || len(f.starter.ids) != 4 || git.exports != 1 {
		t.Fatalf("recovery=%+v launches=%v exports=%d err=%v", second, f.starter.ids, git.exports, err)
	}
	retention := &Retention{Manager: f.manager, UnitRoot: f.runner.Root}
	items, err := retention.Plan(context.Background(), &diskstore.Pass{Now: f.manager.Now().Add(24 * time.Hour)})
	if err != nil || !slices.ContainsFunc(items, func(item diskstore.Item) bool {
		return item.Key == comparison.LaunchID && item.Verdict.Decision == diskstore.Keep && strings.Contains(item.Verdict.Reason, "unit record's round names")
	}) {
		t.Fatalf("comparison evidence lost its retention owner: %v %v", items, err)
	}
}

func TestUnitProofRefusesStaleComparison(t *testing.T) {
	t.Parallel()
	for _, stale := range []string{"base", "tree", "argv", "environment", "directory"} {
		t.Run(stale, func(t *testing.T) {
			t.Parallel()
			f, git := attributionFixture(t, false, "", true)
			start := f.starter.onStart
			f.starter.onStart = func(r Record) error {
				if err := start(r); err != nil {
					return err
				}
				if strings.HasSuffix(r.ID, "-base") {
					f.starter.holdKind = "proof"
				}
				return nil
			}
			result, err := f.runner.Advance(UnitRequest{Plan: f.plan})
			if err != nil || !result.Capped {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			record := result.Record
			comparison := record.Rounds[0].Steps[1].Comparison
			switch stale {
			case "base":
				comparison.Base = "different-base"
			case "tree":
				comparison.Tree = "different-tree"
			case "argv":
				comparison.Command.Argv[1] = "different-argument"
			case "environment":
				comparison.Command.Env[0] = "CHECK_ENV=different"
			case "directory":
				comparison.Command.Dir = filepath.Dir(comparison.Command.Dir)
			}
			if err := f.runner.save(record); err != nil {
				t.Fatal(err)
			}
			_, err = f.manager.Store.Update(comparison.LaunchID, func(r *Record) error { code := 0; r.State, r.ExitCode = Completed, &code; return nil })
			if err != nil {
				t.Fatal(err)
			}
			result, err = f.runner.Advance(UnitRequest{Resume: record.ID})
			if err != nil {
				t.Fatal(err)
			}
			round := result.Record.Rounds[0]
			count, _ := countedRounds(result.Record)
			if round.Cause != "unclassified" || count != 0 || !strings.Contains(round.Steps[1].Reason, "different base tree or command") || git.exports != 1 || len(f.starter.ids) != 4 {
				t.Fatalf("stale %s accepted: round=%+v counted=%d launches=%v", stale, round, count, f.starter.ids)
			}
		})
	}
}

func TestUnitProofRepeatsRegisteredFlakeOnce(t *testing.T) {
	t.Parallel()
	for _, repeatGreen := range []bool{true, false} {
		t.Run(fmt.Sprint(repeatGreen), func(t *testing.T) {
			t.Parallel()
			counter := filepath.Join(t.TempDir(), "counter")
			script := fmt.Sprintf("if [ -f '%s' ]; then %s; fi\ntouch '%s'\nprintf 'LANDING-FAILED\\tpkg/unit\\tTestRed\\nLANDING-CHECKED\\t1\\n'\nexit 1\n", counter, map[bool]string{true: "printf 'LANDING-CHECKED\\t0\\n'; exit 0", false: "exit 1"}[repeatGreen], counter)
			if !repeatGreen {
				script = "if [ -f red ]; then printf 'LANDING-FAILED\\tpkg/unit\\tTestRed\\nLANDING-CHECKED\\t1\\n'; exit 1; fi\n"
			}
			f, git := attributionFixture(t, false, script)
			result, err := f.runner.Advance(UnitRequest{Plan: f.plan})
			if err != nil {
				t.Fatal(err)
			}
			round := result.Record.Rounds[0]
			launches, exports := 3, 0
			if !repeatGreen {
				launches, exports = 4, 1
			}
			if len(f.starter.ids) != launches || len(round.Steps[1].LaunchIDs) != 2 || git.exports != exports || !round.Steps[1].FlakeRepeat {
				t.Fatalf("repeat=%+v launches=%v exports=%d", round, f.starter.ids, git.exports)
			}
			cause, outcome := "own", "proof-red"
			if repeatGreen {
				cause, outcome = "", "green"
			}
			if round.Cause != cause || round.Outcome != outcome {
				t.Fatalf("round=%+v", round)
			}
			if !repeatGreen {
				if _, err := f.manager.Store.Update(round.Steps[1].LaunchID, func(r *Record) error { r.ExitCode = nil; return nil }); err != nil {
					t.Fatal(err)
				}
				plan, err := ReadUnitPlan(f.plan)
				if err != nil {
					t.Fatal(err)
				}
				_, err = f.runner.compareProof(&result.Record, &result.Record.Rounds[0], 1, plan, f.manager.Now().Add(time.Minute))
				step := result.Record.Rounds[0].Steps[1]
				if err != nil || step.Cause != "environment" || !strings.Contains(step.Reason, "the check repeat has no completed exit evidence") {
					t.Fatalf("unfinished repeat reason=%q cause=%s err=%v", step.Reason, step.Cause, err)
				}
			}
		})
	}
}

func TestUnitProofJoinsOnlyMatchingMainTree(t *testing.T) {
	t.Parallel()
	f, git := attributionFixture(t, true, "")
	git.mainTree = git.tree
	incidents := 0
	f.runner.RecordMain = func(record Record, commit, tree string) error {
		if commit != "main-commit" || tree != "base-tree" || !strings.HasSuffix(record.ID, "-base") {
			t.Fatalf("incident=%+v commit=%s tree=%s", record, commit, tree)
		}
		incidents++
		return nil
	}
	result, err := f.runner.Advance(UnitRequest{Plan: f.plan})
	if err != nil {
		t.Fatal(err)
	}
	if incidents != 1 || result.Record.Rounds[0].Cause != "main" {
		t.Fatalf("result=%+v incidents=%d", result, incidents)
	}
}

func TestUnitProofResumesAfterMainRecording(t *testing.T) {
	t.Parallel()
	f, git := attributionFixture(t, true, "", true)
	git.mainTree = git.tree
	sightings := 0
	runID := ""
	f.runner.AfterWrite = func(record UnitRunRecord) error { runID = record.ID; return nil }
	const lost = "lost process after recording main"
	f.runner.RecordMain = func(Record, string, string) error {
		sightings++
		panic(lost)
	}
	func() {
		defer func() {
			if recovered := recover(); recovered != lost {
				t.Fatalf("lost process: %v", recovered)
			}
		}()
		_, _ = f.runner.Advance(UnitRequest{Plan: f.plan})
	}()
	record, err := f.runner.Status(runID)
	if err != nil {
		t.Fatal(err)
	}
	comparison := record.Rounds[0].Steps[1].Comparison
	if comparison == nil || !comparison.MainRecorded || record.Rounds[0].Steps[1].Cause != "unclassified" {
		t.Fatalf("recording was not saved before the call: %+v", record)
	}
	f.runner.RecordMain = func(Record, string, string) error { sightings++; return nil }
	result, err := f.runner.Advance(UnitRequest{Resume: record.ID})
	if err != nil || sightings != 1 || result.Record.Rounds[0].Cause != "main" || len(f.starter.ids) != 4 {
		t.Fatalf("resume: %+v sightings=%d launches=%v err=%v", result, sightings, f.starter.ids, err)
	}
}

// Real Git is needed here because only Git itself can prove that an export
// leaves the source repository's worktree registry unchanged.
type attributionExportGit struct {
	*attributionGit
	commit, directory string
	exportObjects     bool
}

func (git *attributionExportGit) Run(dir string, env []string, args ...string) ([]byte, error) {
	canonical, _ := filepath.EvalSymlinks(git.directory)
	if git.directory != "" && (dir == git.directory || dir == canonical || strings.HasPrefix(dir, git.directory+string(filepath.Separator)) || canonical != "" && strings.HasPrefix(dir, canonical+string(filepath.Separator))) {
		return (OSGitRunner{}).Run(dir, env, args...)
	}
	if len(args) == 5 && slices.Equal(args[:3], []string{"worktree", "add", "--detach"}) {
		git.directory = args[3]
		git.exports++
		return (OSGitRunner{}).Run(dir, env, args...)
	}
	if len(args) == 3 && slices.Equal(args[:2], []string{"init", "--quiet"}) {
		git.directory, git.exportObjects = args[2], true
		git.exports++
		return (OSGitRunner{}).Run(dir, env, args...)
	}
	if git.exportObjects && slices.Equal(args, []string{"rev-parse", "--path-format=absolute", "--git-path", "objects"}) {
		git.exportObjects = false
		return (OSGitRunner{}).Run(dir, env, args...)
	}
	if slices.Equal(args, []string{"rev-parse", "--verify", "base^{commit}"}) {
		return []byte(git.commit), nil
	}
	return git.attributionGit.Run(dir, env, args...)
}

func TestUnitProofExportGitAdapterIntegration(t *testing.T) {
	t.Parallel()
	f, stub := attributionFixture(t, false, "")
	runGit(t, f.worktree, "init", "--quiet")
	tree := strings.TrimSpace(runGit(t, f.worktree, "write-tree"))
	commit := strings.TrimSpace(runGitEnv(t, f.worktree, []string{"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.invalid", "GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.invalid"}, "commit-tree", tree, "-m", "fixture base"))
	runGit(t, f.worktree, "update-ref", "refs/heads/base", commit)
	stub.tree = tree
	git := &attributionExportGit{attributionGit: stub, commit: commit}
	f.runner.Git = git
	before := runGit(t, f.worktree, "worktree", "list", "--porcelain")
	result, err := f.runner.Advance(UnitRequest{Plan: f.plan})
	after := runGit(t, f.worktree, "worktree", "list", "--porcelain")
	if before != after {
		t.Fatalf("worktree registry changed: before=%s after=%s", before, after)
	}
	if err != nil || result.Record.Rounds[0].Cause != "own" || git.exports != 1 {
		t.Fatalf("export: %+v exports=%d err=%v", result, git.exports, err)
	}
	if _, err := os.Stat(filepath.Join(result.Record.Rounds[0].Directory, "base-1")); !os.IsNotExist(err) {
		t.Fatalf("base folder remains: %v", err)
	}
	comparison := result.Record.Rounds[0].Steps[1].Comparison
	baseline, err := f.manager.Store.Read(comparison.LaunchID)
	if err != nil || baseline.ExitCode == nil || *baseline.ExitCode != 0 || !comparison.Verified {
		t.Fatalf("base exit not retained: %+v comparison=%+v err=%v", baseline, comparison, err)
	}
}
