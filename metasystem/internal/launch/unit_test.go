package launch

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

type recordingGit struct {
	index, objects string
	diff           []byte
	calls          [][]string
	envs           [][]string
}

func (git *recordingGit) Run(_ string, env []string, args ...string) ([]byte, error) {
	git.calls, git.envs = append(git.calls, append([]string(nil), args...)), append(git.envs, append([]string(nil), env...))
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
}

func (starter *completingStarter) StartSupervisor(id, _ string) (identity.Ref, error) {
	starter.ids = append(starter.ids, id)
	record, _ := starter.m.Store.Read(id)
	starter.order = append(starter.order, record.Kind)
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
	runner  *UnitRunner
	manager *Manager
	starter *completingStarter
	git     *recordingGit
	plan    string
}

func newUnitFixture(t *testing.T, diff string) unitFixture {
	t.Helper()
	root := t.TempDir()
	worktree := filepath.Join(root, "work")
	os.MkdirAll(worktree, 0o700)
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
	return unitFixture{&UnitRunner{Manager: m, Git: git, Root: filepath.Join(root, "unit")}, m, starter, git, plan}
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
	if !slices.Equal(names, []string{"build", "read", "proof:check"}) {
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

func TestReadRunsBeforeAnyProofCommand(t *testing.T) {
	fixture := newUnitFixture(t, "")
	_, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
	if err != nil || !slices.Equal(fixture.starter.order, []string{"build", "read", "proof"}) {
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
	read := second.Record.Rounds[1].Steps[1]
	launched, _ := fixture.manager.Store.Read(read.LaunchID)
	var paths []string
	for _, input := range launched.Inputs {
		paths = append(paths, input.Path)
	}
	if !slices.Contains(paths, output) || !slices.Contains(paths, second.Record.Rounds[1].FollowUp) || second.Record.Rounds[0].Steps[1].LaunchID == read.LaunchID {
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

func TestUnitRunNeverWritesToTheRepository(t *testing.T) {
	fixture := newUnitFixture(t, "")
	first, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
	if err != nil {
		t.Fatal(err)
	}
	follow := filepath.Join(t.TempDir(), "follow")
	os.WriteFile(follow, []byte("Declared size: 1 changed lines\n"), 0o600)
	if _, err := fixture.runner.Advance(UnitRequest{Resume: first.Record.ID, FollowUp: follow}); err != nil {
		t.Fatal(err)
	}
	for index, call := range fixture.git.calls {
		if !slices.Contains([]string{"rev-parse", "ls-files", "add", "diff"}, call[0]) {
			t.Fatalf("git call=%v", call)
		}
		if call[0] == "add" && !envOutside(fixture.git.envs[index], first.Record.Worktree) {
			t.Fatalf("add env=%v", fixture.git.envs[index])
		}
	}
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
	command := exec.Command("git", args...)
	command.Dir = dir
	out, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return string(out)
}
