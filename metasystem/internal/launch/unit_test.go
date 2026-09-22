package launch

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/refusal"
)

type recordingGit struct {
	index, objects string
	diff           []byte
	calls          [][]string
	envs           [][]string
}

type recordingOSGit struct {
	runner OSGitRunner
	calls  [][]string
	envs   [][]string
}

func (git *recordingOSGit) Run(directory string, environment []string, args ...string) ([]byte, error) {
	git.calls = append(git.calls, append([]string(nil), args...))
	git.envs = append(git.envs, append([]string(nil), environment...))
	return git.runner.Run(directory, environment, args...)
}

func (git *recordingGit) Run(directory string, env []string, args ...string) ([]byte, error) {
	git.calls, git.envs = append(git.calls, append([]string(nil), args...)), append(git.envs, append([]string(nil), env...))
	if args[0] == "symbolic-ref" || args[0] == "rev-parse" && len(args) > 1 && args[1] == "--verify" {
		return (OSGitRunner{}).Run(directory, env, args...)
	}
	if args[0] == "rev-parse" && args[len(args)-1] == "index" {
		return []byte(git.index + "\n"), nil
	}
	if args[0] == "rev-parse" {
		return []byte(git.objects + "\n"), nil
	}
	if args[0] == "diff" {
		return git.diff, nil
	}
	return nil, nil
}

type completingStarter struct {
	m                  *Manager
	failKind, holdKind string
	order, ids         []string
	readOutput         string
	readCounts         []bool
	onStart            func(Record) error
}

func (starter *completingStarter) StartSupervisor(id, _ string) (identity.Ref, error) {
	starter.ids = append(starter.ids, id)
	record, _ := starter.m.Store.Read(id)
	starter.order = append(starter.order, record.Kind)
	if starter.onStart != nil {
		if err := starter.onStart(record); err != nil {
			return identity.Ref{}, err
		}
	}
	starter.m.Store.Update(id, func(current *Record) error {
		if record.Kind == starter.holdKind {
			supervisor, child := ref(10), ref(20)
			current.Supervisor, current.Child, current.State = &supervisor, &child, Running
			return nil
		}
		current.State = Completed
		code := 0
		if record.Kind == starter.failKind {
			current.State, current.Reason, code = Failed, "fixture-red", 1
		}
		current.ExitCode = &code
		if record.Kind == "read" {
			yes := true
			if len(starter.readCounts) > 0 {
				yes = starter.readCounts[0]
				starter.readCounts = starter.readCounts[1:]
			}
			current.VerdictCounts, current.Measurement.Verdict = &yes, "pass"
			if starter.readOutput != "" {
				current.Outputs = []Output{{Path: starter.readOutput, Bytes: 4}}
			}
		}
		return nil
	})
	return ref(10), nil
}

type unitFixture struct {
	runner   *UnitRunner
	manager  *Manager
	starter  *completingStarter
	git      *recordingGit
	plan     string
	worktree string
}

func newUnitFixture(t *testing.T, diff string) unitFixture {
	t.Helper()
	root := t.TempDir()
	worktree := filepath.Join(root, "work")
	os.MkdirAll(worktree, 0o700)
	initializeGoalRepository(t, worktree, "goal")
	brief := filepath.Join(root, "build.md")
	read := filepath.Join(root, "read.md")
	page := filepath.Join(root, "units.md")
	os.WriteFile(brief, []byte("Declared size: 2 changed lines\n"), 0o600)
	os.WriteFile(read, []byte("read it\n"), 0o600)
	os.WriteFile(page, []byte("| Unit | Size |\n|---|---|\n| U | 2 |\n"), 0o600)
	plan := filepath.Join(root, "plan.json")
	data, _ := json.Marshal(UnitPlan{Unit: "U", Goal: "goal", Worktree: worktree, Base: "base",
		Build: UnitBuildPlan{Brief: brief, Inputs: []string{}, Outputs: []string{}, UnitsPage: page, Units: []string{"U"}},
		Read:  UnitReadPlan{Brief: read, Inputs: []string{}, Outputs: []string{}, Model: "read-model"},
		Proof: []ProofCommand{{Name: "check", Dir: worktree, Argv: []string{"true"}, Env: []string{"A=B"}}}})
	os.WriteFile(plan, data, 0o600)
	m, _, _, _ := manager(t)
	m.Adapters["claude-headless"], m.Adapters["plain-exec"] = fakeAdapter{}, fakeAdapter{}
	m.Settings = DefaultSettings()
	m.Settings.WaitCapSeconds = 2
	starter := &completingStarter{m: m}
	m.Supervisor = starter
	index := filepath.Join(root, "source-index")
	os.WriteFile(index, []byte("index"), 0o600)
	objects := filepath.Join(root, "objects")
	os.MkdirAll(objects, 0o700)
	git := &recordingGit{index: index, objects: objects, diff: []byte(diff)}
	return unitFixture{runner: &UnitRunner{Manager: m, Git: git, Root: filepath.Join(root, "unit")}, manager: m, starter: starter, git: git, plan: plan, worktree: worktree}
}

func TestUnitRunStartsOnGoalBranch(t *testing.T) {
	t.Parallel()
	fixture := newUnitFixture(t, "")
	result, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
	if err != nil || result.Record.Rounds[0].Outcome != "green" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestUnitRunRefusesMainBranch(t *testing.T) {
	t.Parallel()
	fixture := newUnitFixture(t, "")
	runGit(t, fixture.worktree, "branch", "-m", "main")
	_, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
	want := "LAUNCH_UNIT_GOAL_BRANCH_REQUIRED worktree=\"" + fixture.worktree + "\" branch=\"main\" required=\"goal/goal\""
	if err == nil || err.Error() != want {
		t.Fatalf("error=%v want=%q", err, want)
	}
	wantRow := refusal.Row{Code: "LAUNCH_UNIT_GOAL_BRANCH_REQUIRED", Owner: "internal/launch", Site: "unit_run.go:227", Shape: refusal.Question}
	for _, row := range refusal.Rows {
		if row.Code == wantRow.Code {
			if row != wantRow {
				t.Fatalf("refusal row=%+v want=%+v", row, wantRow)
			}
			return
		}
	}
	t.Fatalf("missing refusal row %+v", wantRow)
}

func TestUnitRunRefusesDetachedHead(t *testing.T) {
	t.Parallel()
	fixture := newUnitFixture(t, "")
	head := strings.TrimSpace(runGit(t, fixture.worktree, "rev-parse", "HEAD"))
	runGit(t, fixture.worktree, "update-ref", "--no-deref", "HEAD", head)
	_, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
	if err == nil || !strings.Contains(err.Error(), "LAUNCH_UNIT_GOAL_BRANCH_REQUIRED") || !strings.Contains(err.Error(), `branch="detached HEAD"`) || !strings.Contains(err.Error(), `required="goal/goal"`) {
		t.Fatalf("error=%v", err)
	}
}

func TestPlanRefusesWhatItCannotRun(t *testing.T) {
	fixture := newUnitFixture(t, "")
	data, _ := os.ReadFile(fixture.plan)
	for name, mutation := range map[string]func(map[string]any){
		"unknown":        func(value map[string]any) { value["surprise"] = true },
		"missing":        func(value map[string]any) { delete(value, "goal") },
		"repeated proof": func(value map[string]any) { proof := value["proof"].([]any); value["proof"] = append(proof, proof[0]) },
	} {
		t.Run(name, func(t *testing.T) {
			var value map[string]any
			json.Unmarshal(data, &value)
			mutation(value)
			path := filepath.Join(t.TempDir(), "plan.json")
			changed, _ := json.Marshal(value)
			os.WriteFile(path, changed, 0o600)
			_, err := fixture.runner.Advance(UnitRequest{Plan: path})
			if err == nil || !strings.Contains(err.Error(), "UNIT_PLAN_INVALID") {
				t.Fatalf("err=%v", err)
			}
			entries, _ := os.ReadDir(fixture.runner.Root)
			if len(entries) != 0 {
				t.Fatalf("created run: %v", entries)
			}
		})
	}
}

func TestRunRecordNamesEveryStep(t *testing.T) {
	fixture := newUnitFixture(t, "")
	result, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, step := range result.Record.Rounds[0].Steps {
		names = append(names, step.Name)
	}
	if !slices.Equal(names, []string{"build", "proof:check", "read"}) {
		t.Fatalf("steps=%v", names)
	}
	if _, err := os.Stat(filepath.Join(fixture.runner.runDir(result.Record.ID), "run.json")); err != nil {
		t.Fatal(err)
	}
}

func TestRunStopsAtTheCapAndResumeContinues(t *testing.T) {
	fixture := newUnitFixture(t, "")
	fixture.starter.holdKind = "build"
	result, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
	if err != nil || !result.Capped || result.Step != "build" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	fixture.manager.Store.Update(result.Launch, func(record *Record) error { record.State = Completed; return nil })
	fixture.starter.holdKind = ""
	result, err = fixture.runner.Advance(UnitRequest{Resume: result.Record.ID})
	if err != nil || result.Record.State != "awaiting-judgement" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestProofRunsBeforeTheRead(t *testing.T) {
	fixture := newUnitFixture(t, "")
	_, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
	if err != nil || !slices.Equal(fixture.starter.order, []string{"build", "proof", "read"}) {
		t.Fatalf("order=%v err=%v", fixture.starter.order, err)
	}
}

func TestEveryOutcomeEndsAtAwaitingJudgement(t *testing.T) {
	for _, row := range []struct{ fail, want string }{{"build", "build-failed"}, {"read", "read-failed"}, {"proof", "proof-red"}, {"", "green"}} {
		t.Run(row.want, func(t *testing.T) {
			fixture := newUnitFixture(t, "")
			fixture.starter.failKind = row.fail
			result, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
			if err != nil || result.Record.State != "awaiting-judgement" || result.Record.Rounds[0].Outcome != row.want {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			if row.want == "proof-red" {
				for _, step := range result.Record.Rounds[0].Steps {
					if strings.HasPrefix(step.Name, "read") && (step.State != StepSkipped || step.Reason != "proof-red") {
						t.Fatalf("read step=%+v", step)
					}
				}
				if slices.Contains(fixture.starter.order, "read") {
					t.Fatalf("order=%v", fixture.starter.order)
				}
			}
			if row.want == "read-failed" && !slices.Contains(fixture.starter.order, "proof") {
				t.Fatalf("order=%v", fixture.starter.order)
			}
		})
	}
}

func TestRefusedBuildLeavesNoRunRecord(t *testing.T) {
	fixture := newUnitFixture(t, "")
	fixture.manager.Settings.BuildLinesCap = 1
	_, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
	entries, _ := os.ReadDir(fixture.runner.Root)
	if err == nil || !strings.Contains(err.Error(), "LAUNCH_BUILD_OVERSIZE") || len(entries) != 0 {
		t.Fatalf("entries=%v err=%v", entries, err)
	}
}

func TestResumeAfterAKillStartsNoSecondLaunch(t *testing.T) {
	fixture := newUnitFixture(t, "")
	fixture.runner.AfterWrite = func(record UnitRunRecord) error {
		for _, step := range record.Rounds[0].Steps {
			if step.State == StepStarting {
				return errors.New("killed")
			}
		}
		return nil
	}
	_, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
	if err == nil {
		t.Fatal("expected interruption")
	}
	entries, _ := os.ReadDir(fixture.runner.Root)
	id := entries[0].Name()
	fixture.runner.AfterWrite = nil
	if _, err := fixture.runner.Advance(UnitRequest{Resume: id}); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, id := range fixture.starter.ids {
		if seen[id] {
			t.Fatalf("launch started twice: %s", id)
		}
		seen[id] = true
	}
}

func TestSecondCallerRefusesBusy(t *testing.T) {
	fixture := newUnitFixture(t, "")
	result, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
	if err != nil {
		t.Fatal(err)
	}
	lock, err := fixture.runner.lock(result.Record.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	_, err = fixture.runner.Advance(UnitRequest{Resume: result.Record.ID})
	if err == nil || !strings.Contains(err.Error(), "UNIT_RUN_BUSY") {
		t.Fatalf("err=%v", err)
	}
}

func TestDiffCoversUntrackedFilesAndLeavesTheIndexAlone(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.email", "test@example.invalid")
	runGit(t, repo, "config", "user.name", "Test")
	os.WriteFile(filepath.Join(repo, "tracked"), []byte("old\n"), 0o600)
	runGit(t, repo, "add", "tracked")
	runGit(t, repo, "commit", "-m", "base")
	index := runGit(t, repo, "rev-parse", "--git-path", "index")
	before, _ := os.ReadFile(strings.TrimSpace(index))
	os.WriteFile(filepath.Join(repo, "tracked"), []byte("new\n"), 0o600)
	os.WriteFile(filepath.Join(repo, "untracked"), []byte("fresh\n"), 0o600)
	runner := UnitRunner{Git: OSGitRunner{}, Root: filepath.Join(t.TempDir(), "unit")}
	os.MkdirAll(filepath.Join(runner.Root, "r", "round-1"), 0o700)
	target := filepath.Join(runner.Root, "r", "round-1", "worktree.diff")
	if err := runner.writeDiff(repo, "HEAD", target); err != nil {
		t.Fatal(err)
	}
	diff, _ := os.ReadFile(target)
	after, _ := os.ReadFile(strings.TrimSpace(index))
	if !strings.Contains(string(diff), "tracked") || !strings.Contains(string(diff), "untracked") || string(before) != string(after) {
		t.Fatalf("diff=%s index changed=%t", diff, string(before) != string(after))
	}
}

func TestLargeDiffIsReadPerDirectory(t *testing.T) {
	t.Parallel()
	diff := "diff --git a/a/x.go b/a/x.go\n--- a/a/x.go\n+++ b/a/x.go\n+x\ndiff --git a/b/y.go b/b/y.go\n--- a/b/y.go\n+++ b/b/y.go\n+y\n"
	fixture := newUnitFixture(t, diff)
	fixture.manager.Settings.ReadSplitLines = 1
	result, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
	if err != nil {
		t.Fatal(err)
	}
	var reads []string
	for _, step := range result.Record.Rounds[0].Steps {
		if strings.HasPrefix(step.Name, "read:") {
			reads = append(reads, step.Name)
		}
	}
	if !slices.Equal(reads, []string{"read:a", "read:b"}) {
		t.Fatalf("reads=%v", reads)
	}
	fullDiff := filepath.Join(result.Record.Rounds[0].Directory, "worktree.diff")
	data, err := os.ReadFile(fullDiff)
	if err != nil {
		t.Fatal(err)
	}
	wantHash := fmt.Sprintf("%x", sha256.Sum256(data))
	for _, step := range result.Record.Rounds[0].Steps {
		if !strings.HasPrefix(step.Name, "read:") {
			continue
		}
		launched, err := fixture.manager.Store.Read(step.LaunchID)
		if err != nil {
			t.Fatal(err)
		}
		other := map[string]string{"a": "b", "b": "a"}[step.Package]
		packet := readString(launched.AdapterData, "unitReadPacket")
		command, commandErr := (ClaudeHeadless{}).Command(launched, filepath.Join(fixture.manager.Store.Root, launched.ID))
		for _, exact := range []string{
			"Full candidate diff: " + fullDiff,
			"Full candidate SHA-256: " + wantHash,
			"Selected package: " + step.Package,
			"Other package coverage: " + other + " (separate unit read steps)",
			"Diff: " + filepath.Join(fixture.manager.Store.Root, launched.ID, "read.diff"),
		} {
			if commandErr != nil || !strings.Contains(command.Stdin, exact) {
				t.Fatalf("step=%+v packet=%q stdin=%q missing=%q err=%v", step, packet, command.Stdin, exact, commandErr)
			}
		}
	}
}

func TestSplitReadsSharingAReportWaitForPriorCollection(t *testing.T) {
	t.Parallel()
	diff := "diff --git a/a/x.go b/a/x.go\n--- a/a/x.go\n+++ b/a/x.go\n+x\ndiff --git a/b/y.go b/b/y.go\n--- a/b/y.go\n+++ b/b/y.go\n+y\n"
	fixture := newUnitFixture(t, diff)
	fixture.manager.Settings.ReadSplitLines = 1
	plan, err := ReadUnitPlan(fixture.plan)
	if err != nil {
		t.Fatal(err)
	}
	plan.Read.Outputs = []string{filepath.Join(t.TempDir(), "shared-read.md")}
	data, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fixture.plan, data, 0o600); err != nil {
		t.Fatal(err)
	}
	firstCollected := false
	fixture.runner.AfterWrite = func(record UnitRunRecord) error {
		if len(record.Rounds) > 0 {
			for _, step := range record.Rounds[0].Steps {
				if step.Name == "read:a" && step.State == StepPassed {
					firstCollected = true
				}
			}
		}
		return nil
	}
	readStarts := 0
	fixture.starter.onStart = func(record Record) error {
		if record.Kind == "read" {
			readStarts++
			if readStarts == 2 && !firstCollected {
				t.Error("second writer started before the first report was collected")
			}
		}
		return nil
	}
	result, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
	if err != nil || result.Record.Rounds[0].Outcome != "green" || readStarts != 2 {
		t.Fatalf("outcome=%+v read starts=%d err=%v", result.Record.Rounds, readStarts, err)
	}
}

func TestEachRoundReadsFreshWithThePreviousReadAsInput(t *testing.T) {
	fixture := newUnitFixture(t, "")
	output := filepath.Join(t.TempDir(), "read.out")
	os.WriteFile(output, []byte("done"), 0o600)
	fixture.starter.readOutput = output
	first, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
	if err != nil {
		t.Fatal(err)
	}
	follow := filepath.Join(t.TempDir(), "follow.md")
	os.WriteFile(follow, []byte("Declared size: 1 changed lines\n"), 0o600)
	second, err := fixture.runner.Advance(UnitRequest{Resume: first.Record.ID, FollowUp: follow})
	if err != nil {
		t.Fatal(err)
	}
	read := stepNamed(t, second.Record.Rounds[1], "read")
	launched, _ := fixture.manager.Store.Read(read.LaunchID)
	var paths []string
	for _, input := range launched.Inputs {
		paths = append(paths, input.Path)
	}
	if !slices.Contains(paths, output) || !slices.Contains(paths, second.Record.Rounds[1].FollowUp) || stepNamed(t, second.Record.Rounds[0], "read").LaunchID == read.LaunchID {
		t.Fatalf("inputs=%v", paths)
	}
}

func TestFollowUpRefusedUnlessAwaitingJudgement(t *testing.T) {
	fixture := newUnitFixture(t, "")
	fixture.starter.holdKind = "build"
	result, _ := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
	follow := filepath.Join(t.TempDir(), "follow")
	os.WriteFile(follow, []byte("x"), 0o600)
	_, err := fixture.runner.Advance(UnitRequest{Resume: result.Record.ID, FollowUp: follow})
	if err == nil || !strings.Contains(err.Error(), "UNIT_RUN_NOT_AWAITING") {
		t.Fatalf("err=%v", err)
	}
	result.Record.State = "awaiting-judgement"
	if err := admitFollowUp(result.Record, filepath.Join(t.TempDir(), "missing")); err == nil || !strings.Contains(err.Error(), "UNIT_FOLLOW_UP_MISSING") {
		t.Fatalf("err=%v", err)
	}
}

func TestFollowUpStartsTheNextRoundOnTheSameWorktree(t *testing.T) {
	fixture := newUnitFixture(t, "")
	first, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
	if err != nil {
		t.Fatal(err)
	}
	follow := filepath.Join(t.TempDir(), "follow")
	os.WriteFile(follow, []byte("Declared size: 1 changed lines\n"), 0o600)
	second, err := fixture.runner.Advance(UnitRequest{Resume: first.Record.ID, FollowUp: follow})
	if err != nil || len(second.Record.Rounds) != 2 || second.Record.Worktree != first.Record.Worktree {
		t.Fatalf("result=%+v err=%v", second, err)
	}
	build, _ := fixture.manager.Store.Read(second.Record.Rounds[1].Steps[0].LaunchID)
	if build.WorkingDirectory != first.Record.Worktree {
		t.Fatalf("worktree=%s", build.WorkingDirectory)
	}
}

func TestProofThatMovesTheRepositoryEndsTheRound(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, moved string
		effect      func(*testing.T, string)
	}{
		{"commit", "head", func(t *testing.T, repo string) {
			writeFile(t, filepath.Join(repo, "committed"), "new\n")
			runGit(t, repo, "add", "committed")
			runGit(t, repo, "commit", "-m", "proof commit")
		}},
		{"ref", "refs", func(t *testing.T, repo string) { runGit(t, repo, "update-ref", "refs/heads/x", "HEAD") }},
		{"staged-blob-and-path", "index", func(t *testing.T, repo string) {
			writeFile(t, filepath.Join(repo, "staged"), "new\n")
			runGit(t, repo, "add", "staged")
		}},
		{"staged-mode", "index", func(t *testing.T, repo string) {
			if err := os.Chmod(filepath.Join(repo, "tracked"), 0o700); err != nil {
				t.Fatal(err)
			}
			runGit(t, repo, "add", "tracked")
		}},
		{"staged-rename", "index", func(t *testing.T, repo string) { runGit(t, repo, "mv", "tracked", "renamed") }},
		{"unmerged-stages", "index", func(t *testing.T, repo string) { setConflictStages(t, repo, "tracked") }},
		{"assume-unchanged", "index", func(t *testing.T, repo string) { runGit(t, repo, "update-index", "--assume-unchanged", "tracked") }},
		{"skip-worktree", "index", func(t *testing.T, repo string) { runGit(t, repo, "update-index", "--skip-worktree", "tracked") }},
		{"intent-to-add", "index", func(t *testing.T, repo string) {
			writeFile(t, filepath.Join(repo, "intent"), "")
			runGit(t, repo, "add", "-N", "intent")
		}},
		{"resolve-undo", "index", func(t *testing.T, repo string) {
			setConflictStages(t, repo, "tracked")
			runGit(t, repo, "add", "tracked")
			if output := runGit(t, repo, "ls-files", "--resolve-undo", "--", "tracked"); output == "" {
				t.Fatal("fixture did not create resolve-undo state")
			}
		}},
		{"tree", "tree", func(t *testing.T, repo string) { writeFile(t, filepath.Join(repo, "tracked"), "changed\n") }},
		{"stat-cache-refresh", "", func(t *testing.T, repo string) {
			before := indexFileDigest(t, repo)
			bumpTrackedMtime(t, repo)
			runGitEnv(t, repo, []string{"GIT_OPTIONAL_LOCKS=1"}, "status", "--short")
			if after := indexFileDigest(t, repo); after == before {
				t.Fatal("git status did not rewrite the fixture index cache")
			}
		}},
		{"optional-locks-read", "", func(t *testing.T, repo string) {
			before := indexFileDigest(t, repo)
			bumpTrackedMtime(t, repo)
			runGitEnv(t, repo, []string{"GIT_OPTIONAL_LOCKS=0"}, "status", "--short")
			if after := indexFileDigest(t, repo); after != before {
				t.Fatal("GIT_OPTIONAL_LOCKS=0 rewrote the fixture index")
			}
		}},
		{"none", "", nil},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			fixture, repo := newGitUnitFixture(t)
			fixture.starter.onStart = func(record Record) error {
				if record.Kind == "proof" && row.effect != nil {
					row.effect(t, repo)
				}
				return nil
			}
			result, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
			if err != nil {
				t.Fatal(err)
			}
			if row.moved == "" {
				if result.Record.Rounds[0].Outcome != "green" {
					t.Fatalf("record=%+v", result.Record)
				}
				return
			}
			if result.Record.Rounds[0].Outcome != "proof-wrote" || result.Record.State != "awaiting-judgement" {
				t.Fatalf("record=%+v", result.Record)
			}
			for _, step := range result.Record.Rounds[0].Steps {
				if strings.HasPrefix(step.Name, "read") && (step.State != StepSkipped || !strings.HasPrefix(step.Reason, "proof-wrote:") || !strings.Contains(step.Reason, row.moved)) {
					t.Fatalf("read step=%+v", step)
				}
			}
		})
	}
}

func TestRepositorySnapshotIgnoresIndexCacheRefresh(t *testing.T) {
	t.Parallel()
	fixture, repo := newGitUnitFixture(t)
	before, err := fixture.runner.snapshotRepository(repo)
	if err != nil {
		t.Fatal(err)
	}
	rawBefore := indexFileDigest(t, repo)
	bumpTrackedMtime(t, repo)
	runGitEnv(t, repo, []string{"GIT_OPTIONAL_LOCKS=1"}, "status", "--short")
	if rawAfter := indexFileDigest(t, repo); rawAfter == rawBefore {
		t.Fatal("git status did not rewrite the fixture index cache")
	}
	after, err := fixture.runner.snapshotRepository(repo)
	if err != nil {
		t.Fatal(err)
	}
	if changed := before.changed(after); len(changed) != 0 {
		t.Fatalf("cache refresh changed repository snapshot: %v", changed)
	}
}

func TestNestedWorktreeProtectsRepositoryWideIndexState(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name   string
		moved  bool
		effect func(*testing.T, string, string)
	}{
		{"staged-sibling", true, func(t *testing.T, repo, _ string) {
			writeFile(t, filepath.Join(repo, "staged-sibling"), "new\n")
			runGit(t, repo, "add", "staged-sibling")
		}},
		{"assume-unchanged-sibling", true, func(t *testing.T, repo, _ string) {
			runGit(t, repo, "update-index", "--assume-unchanged", "tracked")
		}},
		{"root-stat-cache-refresh", false, func(t *testing.T, repo, _ string) {
			before := indexFileDigest(t, repo)
			bumpTrackedMtime(t, repo)
			runGitEnv(t, repo, []string{"GIT_OPTIONAL_LOCKS=1"}, "status", "--short")
			if after := indexFileDigest(t, repo); after == before {
				t.Fatal("root status did not rewrite the fixture index cache")
			}
		}},
		{"nested-stat-cache-refresh", false, func(t *testing.T, repo, nested string) {
			before := indexFileDigest(t, repo)
			path := filepath.Join(nested, "tracked")
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			changed := info.ModTime().Add(24 * time.Hour)
			if err := os.Chtimes(path, changed, changed); err != nil {
				t.Fatal(err)
			}
			runGitEnv(t, nested, []string{"GIT_OPTIONAL_LOCKS=1"}, "status", "--short")
			if after := indexFileDigest(t, repo); after == before {
				t.Fatal("nested status did not rewrite the fixture index cache")
			}
		}},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			fixture, repo := newGitUnitFixture(t)
			nested := filepath.Join(repo, "nested")
			if err := os.Mkdir(nested, 0o700); err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(nested, "tracked"), "original\n")
			runGit(t, repo, "add", "nested/tracked")
			runGit(t, repo, "commit", "-m", "add nested worktree")
			setUnitPlanWorktree(t, &fixture, nested)
			fixture.starter.onStart = func(record Record) error {
				if record.Kind == "proof" {
					row.effect(t, repo, nested)
				}
				return nil
			}
			result, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
			if err != nil {
				t.Fatal(err)
			}
			if row.moved && result.Record.Rounds[0].Outcome != "proof-wrote" {
				t.Fatalf("repository-wide index mutation was not detected: %+v", result.Record)
			}
			if !row.moved && result.Record.Rounds[0].Outcome != "green" {
				t.Fatalf("harmless cache refresh stopped the read: %+v", result.Record)
			}
		})
	}
}

func TestUnitRunNeverWritesToTheRepository(t *testing.T) {
	fixture, repo := newGitUnitFixture(t)
	before := repositoryDigest(t, repo)
	first, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
	if err != nil {
		t.Fatal(err)
	}
	follow := filepath.Join(t.TempDir(), "follow")
	os.WriteFile(follow, []byte("Declared size: 1 changed lines\n"), 0o600)
	if _, err := fixture.runner.Advance(UnitRequest{Resume: first.Record.ID, FollowUp: follow}); err != nil {
		t.Fatal(err)
	}
	after := repositoryDigest(t, repo)
	if before != after {
		t.Fatalf("repository changed\nbefore=%s\nafter=%s", before, after)
	}
	git := fixture.runner.Git.(*recordingOSGit)
	for index, call := range git.calls {
		if !slices.Contains([]string{"symbolic-ref", "rev-parse", "for-each-ref", "ls-files", "diff-index", "add", "diff"}, call[0]) {
			t.Fatalf("git call=%v", call)
		}
		if call[0] == "add" && !envOutside(git.envs[index], first.Record.Worktree) {
			t.Fatalf("add env=%v", git.envs[index])
		}
	}
}

func newGitUnitFixture(t *testing.T) (unitFixture, string) {
	t.Helper()
	fixture := newUnitFixture(t, "")
	repo := t.TempDir()
	initializeGoalRepository(t, repo, "goal")
	writeFile(t, filepath.Join(repo, "tracked"), "original\n")
	runGit(t, repo, "add", "tracked")
	runGit(t, repo, "commit", "-m", "base")
	data, err := os.ReadFile(fixture.plan)
	if err != nil {
		t.Fatal(err)
	}
	var plan UnitPlan
	if err := json.Unmarshal(data, &plan); err != nil {
		t.Fatal(err)
	}
	plan.Worktree = repo
	plan.Base = strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))
	plan.Proof[0].Dir = repo
	data, err = json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fixture.plan, data, 0o600); err != nil {
		t.Fatal(err)
	}
	fixture.runner.Git = &recordingOSGit{}
	fixture.worktree = repo
	return fixture, repo
}

func setUnitPlanWorktree(t *testing.T, fixture *unitFixture, worktree string) {
	t.Helper()
	data, err := os.ReadFile(fixture.plan)
	if err != nil {
		t.Fatal(err)
	}
	var plan UnitPlan
	if err := json.Unmarshal(data, &plan); err != nil {
		t.Fatal(err)
	}
	plan.Worktree = worktree
	plan.Base = strings.TrimSpace(runGit(t, worktree, "rev-parse", "HEAD"))
	plan.Proof[0].Dir = worktree
	data, err = json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fixture.plan, data, 0o600); err != nil {
		t.Fatal(err)
	}
	fixture.worktree = worktree
}

func initializeGoalRepository(t *testing.T, directory, goal string) {
	t.Helper()
	runGit(t, directory, "init")
	runGit(t, directory, "symbolic-ref", "HEAD", "refs/heads/goal/"+goal)
	runGit(t, directory, "config", "user.email", "test@example.invalid")
	runGit(t, directory, "config", "user.name", "Test")
	runGit(t, directory, "commit", "--allow-empty", "-m", "base")
}

func repositoryDigest(t *testing.T, repo string) string {
	t.Helper()
	digest := sha256.New()
	err := filepath.WalkDir(repo, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		relative, err := filepath.Rel(repo, path)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(digest, "%s %d %x\n", relative, info.Size(), sum)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	head := runGit(t, repo, "rev-parse", "HEAD")
	refs := runGit(t, repo, "for-each-ref", "--format=%(refname) %(objectname)")
	indexPath := strings.TrimSpace(runGit(t, repo, "rev-parse", "--path-format=absolute", "--git-path", "index"))
	index, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	indexSum := sha256.Sum256(index)
	_, _ = fmt.Fprintf(digest, "head=%srefs=%sindex=%x\n", head, refs, indexSum)
	return fmt.Sprintf("%x", digest.Sum(nil))
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func setConflictStages(t *testing.T, repo, path string) {
	t.Helper()
	oid := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD:"+path))
	zero := strings.Repeat("0", len(oid))
	input := fmt.Sprintf("0 %s\t%s\n100644 %s 1\t%s\n100644 %s 2\t%s\n100644 %s 3\t%s\n", zero, path, oid, path, oid, path, oid, path)
	runGitInput(t, repo, input, "update-index", "--index-info")
}

func bumpTrackedMtime(t *testing.T, repo string) {
	t.Helper()
	path := filepath.Join(repo, "tracked")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	changed := info.ModTime().Add(24 * time.Hour)
	if err := os.Chtimes(path, changed, changed); err != nil {
		t.Fatal(err)
	}
}

func indexFileDigest(t *testing.T, repo string) string {
	t.Helper()
	path := strings.TrimSpace(runGit(t, repo, "rev-parse", "--path-format=absolute", "--git-path", "index"))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

func stepNamed(t *testing.T, round UnitRound, name string) UnitStep {
	t.Helper()
	for _, step := range round.Steps {
		if step.Name == name {
			return step
		}
	}
	t.Fatalf("step %s not found in %+v", name, round.Steps)
	return UnitStep{}
}

func envOutside(env []string, worktree string) bool {
	for _, entry := range env {
		if strings.HasPrefix(entry, "GIT_INDEX_FILE=") {
			return !strings.HasPrefix(strings.TrimPrefix(entry, "GIT_INDEX_FILE="), worktree+string(os.PathSeparator))
		}
	}
	return false
}
func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	return runGitEnv(t, dir, nil, args...)
}

func runGitEnv(t *testing.T, dir string, environment []string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	command.Env = childEnvironment(os.Environ(), environment)
	out, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return string(out)
}

func runGitInput(t *testing.T, dir, input string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	command.Stdin = strings.NewReader(input)
	out, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return string(out)
}
