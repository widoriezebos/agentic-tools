package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/adapter"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/adapter/fakeadapter"
)

type failureProcesses struct {
	workProcesses
	commands  []launch.Command
	deny      int
	passAfter int
	moved     func()
}

func (p *failureProcesses) StartChild(c launch.Command) (launch.Child, identity.Ref, error) {
	p.commands = append(p.commands, c)
	if len(p.commands) <= p.deny {
		return nil, identity.Ref{}, os.ErrPermission
	}
	return failureChild{p.moved, p.passAfter > 0 && len(p.commands) > p.passAfter}, workProcessRef(20), nil
}

type failureChild struct {
	moved  func()
	passed bool
}

func (c failureChild) Wait() (int, error) {
	if c.moved != nil {
		c.moved()
	}
	if c.passed {
		return 0, nil
	}
	return 1, errors.New("exit status 1")
}

type failureSupervisor struct {
	bed        *workBed
	lost       bool
	unreadable bool
	rewrite    bool
	unready    bool
}

func (s failureSupervisor) StartSupervisor(id, state string) (identity.Ref, error) {
	r, err := s.bed.manager.Store.Read(id)
	if err != nil {
		return identity.Ref{}, err
	}
	if r.Kind != "proof" {
		return s.bed.starter.StartSupervisor(id, state)
	}
	if s.unready {
		return workProcessRef(10), nil
	}
	if s.unreadable {
		for _, input := range r.Inputs {
			if err := os.Remove(input.Path); err != nil {
				return identity.Ref{}, err
			}
		}
	}
	if s.lost {
		_, err := s.bed.manager.Store.Update(id, func(record *launch.Record) error {
			dead := workProcessRef(99)
			record.Supervisor, record.State = &dead, launch.Running
			return nil
		})
		return workProcessRef(99), err
	}
	// Supervise records the same process-start error production receives.
	_, err = s.bed.manager.Supervise(id)
	if err != nil {
		r, readErr := s.bed.manager.Store.Read(id)
		if readErr != nil || !r.State.Terminal() {
			return identity.Ref{}, err
		}
	}
	if s.rewrite && !strings.HasSuffix(id, "-retry") {
		original := filepath.Join(filepath.Dir(filepath.Dir(r.Inputs[0].Path)), "proof-check.json")
		if err := os.WriteFile(original, []byte(`{"argv":["altered-command"],"dir":"/changed","env":["CHANGED=yes"]}`), 0600); err != nil {
			return identity.Ref{}, err
		}
	}
	return workProcessRef(10), nil
}
func failedCommandBed(t *testing.T, deny int) (*workBed, *failureProcesses) {
	t.Helper()
	b := outcomeBed(t, "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n+new\n")
	p := &failureProcesses{deny: deny}
	b.manager.Processes = p
	b.manager.Adapters["plain-exec"] = launch.PlainExec{}
	b.manager.Supervisor = failureSupervisor{bed: b}
	return b, p
}
func failedCommandBuild(t *testing.T, b *workBed) (int, intentResult) {
	t.Helper()
	brief := b.brief("build.md", "Build the declared requirement.\n")
	// This argv is parsed by PlainExec; the injected process boundary supplies
	// permission and exit outcomes without executing Git or a real test suite.
	code, result, _ := b.work("work", "build", b.id, "outcome", "--brief", brief, "--lines", "1", "--check", "go", "test", "-timeout", "30m", "-run", "^TestDeclared$", "./...")
	return code, result
}
func failedCommandRecord(t *testing.T, b *workBed, result intentResult) launch.UnitRunRecord {
	t.Helper()
	run := resultData(t, result)["run"].(string)
	r, err := (&launch.UnitRunner{Root: b.unitRoot}).Status(run)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestIntentDeniedStepRetriesOnceWithoutRebuilding(t *testing.T) {
	t.Parallel()
	b, p := failedCommandBed(t, 2)
	b.manager.Supervisor = failureSupervisor{bed: b, rewrite: true}
	code, result := failedCommandBuild(t, b)
	r := failedCommandRecord(t, b, result)
	if code != 1 || len(r.Rounds) != 1 || r.Rounds[0].Cause != "environment" || len(p.commands) != 2 || !slices.Equal(b.starter.launched(), []string{"build"}) {
		t.Fatalf("denied command rebuilt or lost its stop: %d %+v executions=%d builds=%v", code, r, len(p.commands), b.starter.launched())
	}
	first, retry := p.commands[0], p.commands[1]
	if first.Program != retry.Program || !slices.Equal(first.Args, retry.Args) || first.Directory != retry.Directory || !slices.Equal(first.Environment, retry.Environment) || first.LogPath == retry.LogPath {
		t.Fatalf("retry changed inputs or reused output: %+v %+v", first, retry)
	}
	step := r.Rounds[0].Steps[1]
	if len(step.LaunchIDs) != 2 || step.LaunchIDs[0] == step.LaunchIDs[1] || step.LaunchID != step.LaunchIDs[1] {
		t.Fatalf("physical history lost: %+v", step)
	}
	a, _ := b.manager.Store.Read(step.LaunchIDs[0])
	z, _ := b.manager.Store.Read(step.LaunchIDs[1])
	if !slices.Equal(a.Inputs, z.Inputs) {
		t.Fatalf("retry changed retained inputs: %+v %+v", a, z)
	}
	for _, verb := range []string{"wait", "review", "wait"} {
		code, again, _ := b.work("work", verb, "run:"+r.ID)
		if code != 1 || len(p.commands) != 2 || len(b.starter.launched()) != 1 || again.Next == nil || !slices.Contains(again.Next.Argv, "--by") {
			t.Fatalf("third execution escaped stop: %d %+v", code, again)
		}
	}
	t.Logf("work build retained two denied executions %v with distinct logs; repeated wait/review launched nothing", step.LaunchIDs)
}

func TestIntentUnattributedRedHoldsForProvenPerson(t *testing.T) {
	t.Parallel()
	b, p := failedCommandBed(t, 0)
	code, result := failedCommandBuild(t, b)
	r := failedCommandRecord(t, b, result)
	if code != 1 || r.Rounds[0].Cause != "unclassified" || r.Rounds[0].Stop == nil || r.Rounds[0].Stop.Cause.Kind != "unclassified" || len(p.commands) != 1 {
		t.Fatalf("exit was charged as own: %d %+v", code, r)
	}
	questions, damaged := channel.WalkOpenQuestions(b.root())
	if len(damaged) != 0 || len(questions) != 1 || !strings.Contains(channel.ActCommand(questions[0]), "--reason TEXT --by NAME") {
		t.Fatalf("missing retained person remedy: %+v %v", questions, damaged)
	}
	brief := b.brief("correction.md", "Correct the retained failure.\n")
	code, denied, _ := b.work("work", "revise", b.id, "--work", "outcome", "--brief", brief)
	if code != 1 || len(p.commands) != 1 || len(b.starter.launched()) != 1 {
		t.Fatalf("agent bypassed hold: %d %+v", code, denied)
	}
	owners := b.workOwners()
	owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
		return humanauthority.Proof{}, errors.New("an agent started this shell")
	}
	args := []string{"work", "revise", b.id, "--work", "outcome", "--brief", brief, "--reason", "Repair the failed command", "--by", "Wido"}
	command, rest, _ := resolveIntentArgv(args)
	var out, stderr bytes.Buffer
	code = runIntentIn(command, append([]string{"--json"}, rest...), &out, &stderr, b.root(), owners)
	if code != 1 || len(p.commands) != 1 {
		t.Fatalf("by text bypassed person proof: %d %s", code, out.String())
	}
	code, admitted, impact := b.work(args...)
	after := failedCommandRecord(t, b, admitted)
	if code != 1 || len(after.Rounds) != 2 || len(after.Revisions) != 1 || after.Revisions[0].Person != "Wido" || !strings.Contains(impact, "Impact:") || *after.CorrectionBudget != *r.CorrectionBudget {
		t.Fatalf("person's remedy failed or reset budget: %d %+v %s", code, after, impact)
	}
	closed, err := channel.ReadQuestion(b.root(), questions[0].ID)
	if err != nil || closed.State != "closed" {
		t.Fatalf("older question hid new round: %+v %v", closed, err)
	}
	register, err := os.ReadFile(filepath.Join(r.Rounds[0].Directory, "stop-register.json"))
	if err != nil || !bytes.Contains(register, []byte(`"status":"cleared"`)) {
		t.Fatalf("older register stayed open: %s %v", register, err)
	}
	t.Logf("work build held an unclassified exit; agent revision refused; proven person admitted round 2 with unchanged correction allowance")
}

func TestIntentSupervisorReadyTimeoutRetriesOnce(t *testing.T) {
	t.Parallel()
	b, p := failedCommandBed(t, 0)
	b.manager.StartCap = time.Second
	b.manager.Supervisor = failureSupervisor{bed: b, unready: true}
	code, result := failedCommandBuild(t, b)
	r := failedCommandRecord(t, b, result)
	if code != 1 || len(p.commands) != 0 || len(r.Rounds[0].Steps[1].LaunchIDs) != 2 || r.Rounds[0].Cause != "environment" || !strings.Contains(r.Rounds[0].Stop.Class, "supervisor-ready-timeout") {
		t.Fatalf("readiness timeout lost environment retry: %d %+v executions=%d", code, r, len(p.commands))
	}
}

func TestIntentMovedTreeRetryUsesCurrentSnapshot(t *testing.T) {
	t.Parallel()
	for _, pass := range []bool{true, false} {
		t.Run(fmt.Sprintf("passing-retry=%t", pass), func(t *testing.T) {
			t.Parallel()
			b, p := failedCommandBed(t, 0)
			if pass {
				p.passAfter = 1
			}
			p.moved = func() { b.head = "moved-head" }
			code, result := failedCommandBuild(t, b)
			r := failedCommandRecord(t, b, result)
			want, outcome := "unclassified", "proof-red"
			if pass {
				want, outcome = "", "green"
			}
			if code != map[bool]int{true: 0, false: 1}[pass] || len(p.commands) != 2 || len(b.starter.launched()) != 1 || r.Rounds[0].Cause != want || r.Rounds[0].Outcome != outcome {
				t.Fatalf("retry judged against first tree: %d %+v executions=%d", code, r, len(p.commands))
			}
		})
	}
}

func TestIntentLostProcessRetriesOnce(t *testing.T) {
	t.Parallel()
	b, p := failedCommandBed(t, 0)
	b.manager.Supervisor = failureSupervisor{bed: b, lost: true}
	code, result := failedCommandBuild(t, b)
	r := failedCommandRecord(t, b, result)
	if code != 1 || r.Rounds[0].Cause != "environment" || len(r.Rounds[0].Steps[1].LaunchIDs) != 2 || len(p.commands) != 0 || len(b.starter.launched()) != 1 {
		t.Fatalf("lost process escaped retry bound: %d %+v", code, r)
	}
}

func TestIntentUnreadableRetainedInputHoldsUnknown(t *testing.T) {
	t.Parallel()
	b, p := failedCommandBed(t, 0)
	b.manager.Supervisor = failureSupervisor{bed: b, unreadable: true}
	code, result := failedCommandBuild(t, b)
	r := failedCommandRecord(t, b, result)
	if code != 1 || r.State != "awaiting-judgement" || r.Rounds[0].Cause != "unclassified" || r.Rounds[0].Stop == nil || len(p.commands) != 0 || len(b.starter.launched()) != 1 {
		t.Fatalf("unreadable input lost its unknown hold: %d %+v", code, r)
	}
}

func TestIntentFailedStepStopsBeforeLaterCommands(t *testing.T) {
	t.Parallel()
	b, p := failedCommandBed(t, 2)
	bound := fakeadapter.New()
	bound.Steps = []adapter.GateStep{{Name: "first", Args: []string{"first-command"}}}
	bound.Scripted.Root = b.worktree
	b.testingAdapter = bound
	code, result := failedCommandBuild(t, b)
	r := failedCommandRecord(t, b, result)
	if code != 1 || len(p.commands) != 2 || r.Rounds[0].Steps[1].Name != "proof:first" || r.Rounds[0].Steps[2].State != launch.StepSkipped {
		t.Fatalf("held step launched a later command: %d %+v executions=%d", code, r, len(p.commands))
	}
}

func TestIntentEnvironmentPersonActRerunsRetainedStep(t *testing.T) {
	t.Parallel()
	b, p := failedCommandBed(t, 2)
	code, result := failedCommandBuild(t, b)
	r := failedCommandRecord(t, b, result)
	if code != 1 || result.Next == nil {
		t.Fatalf("missing failed-step remedy: %d %+v", code, result)
	}
	questions, damaged := channel.WalkOpenQuestions(b.root())
	if len(damaged) != 0 || len(questions) != 1 || channel.ActCommand(questions[0]) != shellCommand(result.Next.Argv) {
		t.Fatalf("question and printed remedy differ: %+v %+v", questions, result.Next)
	}
	args := append([]string(nil), result.Next.Argv[1:]...)
	for i := range args {
		if args[i] == "TEXT" {
			args[i] = "The environment is repaired"
		}
		if args[i] == "NAME" {
			args[i] = "Wido"
		}
	}
	owners := b.workOwners()
	owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
		return humanauthority.Proof{}, errors.New("an agent started this shell")
	}
	command, rest, _ := resolveIntentArgv(args)
	var out, stderr bytes.Buffer
	denied := runIntentIn(command, append([]string{"--json"}, rest...), &out, &stderr, b.root(), owners)
	if denied != 1 || len(p.commands) != 2 {
		t.Fatalf("agent admitted environment retry: %d %s", denied, out.String())
	}
	p.passAfter = 2
	b.head = "repaired-head"
	code, admitted, impact := b.work(args...)
	after := failedCommandRecord(t, b, admitted)
	if code != 0 || len(after.Rounds) != 1 || after.Rounds[0].Outcome != "green" || len(p.commands) != 3 || len(b.starter.launched()) != 1 || len(after.Revisions) != 0 || !strings.Contains(impact, "Impact:") {
		t.Fatalf("person act rebuilt or failed: %d %+v commands=%d builds=%v %s", code, after, len(p.commands), b.starter.launched(), impact)
	}
	snapshot, err := os.ReadFile(filepath.Join(after.Rounds[0].Directory, "proof-after.json"))
	var tree struct{ Head string }
	if err != nil || json.Unmarshal(snapshot, &tree) != nil || strings.TrimSpace(tree.Head) != b.head {
		t.Fatalf("retry retained an older result: %s %v", snapshot, err)
	}
	first, last := p.commands[0], p.commands[2]
	if first.Program != last.Program || !slices.Equal(first.Args, last.Args) || first.Directory != last.Directory || !slices.Equal(first.Environment, last.Environment) || first.LogPath == last.LogPath || p.commands[1].LogPath == last.LogPath {
		t.Fatalf("person retry changed inputs or reused output: %+v %+v", first, last)
	}
	step := after.Rounds[0].Steps[1]
	if len(step.LaunchIDs) != 3 || step.RetryBy != "Wido" || *after.CorrectionBudget != *r.CorrectionBudget {
		t.Fatalf("retry history/allowance lost: %+v", after)
	}
	closed, err := channel.ReadQuestion(b.root(), questions[0].ID)
	if err != nil || closed.State != "closed" {
		t.Fatalf("older question stayed open: %+v %v", closed, err)
	}
	register, err := os.ReadFile(filepath.Join(r.Rounds[0].Directory, "stop-register.json"))
	if err != nil || !bytes.Contains(register, []byte(`"status":"cleared"`)) {
		t.Fatalf("older stop stayed open: %s %v", register, err)
	}
	code, _, _ = b.work(args...)
	if code != 0 || len(p.commands) != 3 || len(b.starter.launched()) != 1 {
		t.Fatalf("replay launched again: %d commands=%d", code, len(p.commands))
	}
	t.Logf("printed person act reran proof only; three distinct logs, retained inputs, same round and allowance; question and stop cleared")
}
