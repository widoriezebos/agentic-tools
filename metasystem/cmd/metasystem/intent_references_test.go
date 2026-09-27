package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	metarun "github.com/widoriezebos/agentic-tools/metasystem/internal/run"
)

// referenceBed holds one record of each store under the same raw id where
// the test wants a collision.
type referenceBed struct {
	*processBed
}

func newReferenceBed(t *testing.T) *referenceBed {
	t.Helper()
	b := &referenceBed{processBed: newProcessBed(t)}
	// The unit-run store is the launch store's sibling directory.
	b.launchDir = filepath.Join(t.TempDir(), "launch")
	if err := os.MkdirAll(b.launchDir, 0o755); err != nil {
		t.Fatal(err)
	}
	return b
}

func (b *referenceBed) launchRecord(id string, state launch.State) {
	b.t.Helper()
	if err := (launch.Store{Root: b.launchDir}).Create(launch.Record{ID: id, Kind: "read", State: state}); err != nil {
		b.t.Fatal(err)
	}
}

func (b *referenceBed) unitRun(id, goalID string) {
	b.t.Helper()
	dir := filepath.Join(filepath.Dir(b.launchDir), "unit", id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		b.t.Fatal(err)
	}
	data, _ := json.Marshal(launch.UnitRunRecord{ID: id, Unit: "main", Goal: goalID, State: "awaiting-review"})
	if err := os.WriteFile(filepath.Join(dir, "run.json"), data, 0o644); err != nil {
		b.t.Fatal(err)
	}
}

func (b *referenceBed) dispatchJob(id, status string) {
	b.t.Helper()
	jobs := filepath.Join(b.root(), "artifacts", "agents", "jobs")
	if err := os.MkdirAll(jobs, 0o755); err != nil {
		b.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jobs, id+".json"), []byte(`{"jobId":"`+id+`","status":"`+status+`","role":"implementer"}`), 0o644); err != nil {
		b.t.Fatal(err)
	}
}

// invocation is a public invocation of one action, for resolving references
// without running its owner.
func (b *referenceBed) invocation(name string) *intentInvocation {
	b.t.Helper()
	command, ok := findIntentCommand(name)
	if !ok {
		b.t.Fatalf("no action %s", name)
	}
	return &intentInvocation{command: command, cwd: b.root(), owners: b.owners(), input: intentInput{values: map[string][]string{}}}
}

// TestIntentReferenceCollisionAcrossStores: a raw id that names a launch and
// a unit run is refused with both qualified references; each qualified form
// then reaches exactly its own store.
func TestIntentReferenceCollisionAcrossStores(t *testing.T) {
	t.Parallel()
	b := newReferenceBed(t)
	b.launchRecord("shared-1", launch.Completed)
	b.unitRun("shared-1", bedGoal)
	owners := b.owners()
	code, result := b.runJSON(owners, "work", "status", "shared-1")
	candidates, _ := result.Data.(map[string]any)["candidates"].([]any)
	if code != 1 || result.Outcome != intentRefused || !slices.Equal(candidates, []any{"j1:shared-1", "run:shared-1"}) ||
		!strings.Contains(result.Decision, "j1:shared-1 or run:shared-1") {
		t.Fatalf("colliding raw id = %d %+v", code, result)
	}
	choices, _ := json.Marshal(result.Data.(map[string]any)["choices"])
	if !strings.Contains(string(choices), `["metasystem","work","status","j1:shared-1"]`) || !strings.Contains(string(choices), `["metasystem","work","status","run:shared-1"]`) {
		t.Fatalf("the refusal does not offer each qualified command: %s", choices)
	}
	if code, result := b.runJSON(owners, "work", "status", "j1:shared-1"); code != 0 || !strings.HasPrefix(result.Summary, "launch j1:shared-1:") || result.Targets[0].ID != "j1:shared-1" {
		t.Fatalf("j1:shared-1 = %d %+v", code, result)
	}
	if code, result := b.runJSON(owners, "work", "status", "run:shared-1"); code != 0 || !strings.HasPrefix(result.Summary, "unit run run:shared-1 ") || result.Targets[0].ID != "run:shared-1" {
		t.Fatalf("run:shared-1 = %d %+v", code, result)
	}
}

// TestIntentReferenceGoalNameFirst: a bare word that is a goal name and also
// a raw record id is the goal.
func TestIntentReferenceGoalNameFirst(t *testing.T) {
	t.Parallel()
	b := newReferenceBed(t)
	b.launchRecord(bedGoal, launch.Running)
	b.dispatchJob(bedGoal, "running")
	for _, name := range []string{"work status", "work wait", "work land", "work review", "work revise", "work build"} {
		inv := b.invocation(name)
		ref, problem := inv.resolveWorkRef(bedGoal, inv.command.accepts)
		if problem != nil || ref.kind != refGoal || ref.id != bedGoal {
			t.Errorf("%s %s = %+v %+v; a goal name comes first", name, bedGoal, ref, problem)
		}
	}
	// A qualified reference is never read as a goal.
	inv := b.invocation("work status")
	if ref, problem := inv.resolveWorkRef("j2:"+bedGoal, inv.command.accepts); problem != nil || ref.kind != refJ2 || ref.id != bedGoal {
		t.Errorf("j2:%s = %+v %+v", bedGoal, ref, problem)
	}
}

// TestIntentReferenceKindsPerAction: each kind an action declares reaches
// it; each kind it does not declare refuses before any effect and names the
// actions that take it.
func TestIntentReferenceKindsPerAction(t *testing.T) {
	t.Parallel()
	b := newReferenceBed(t)
	b.launchRecord("rec-1", launch.Completed)
	b.dispatchJob("rec-1", "completed")
	accepting := map[string][]string{}
	for _, command := range publicIntentCommands() {
		for _, kind := range command.accepts {
			accepting[kind] = append(accepting[kind], command.name)
		}
	}
	for _, kind := range []string{refGoal, refJ1, refJ2, refRun, refRead, refWait, refProof} {
		if len(accepting[kind]) == 0 {
			t.Errorf("no action takes a %s reference", kind)
		}
	}
	for _, command := range publicIntentCommands() {
		if len(command.accepts) == 0 {
			continue
		}
		for _, kind := range []string{refJ1, refJ2, refRun, refRead, refWait, refProof} {
			inv := b.invocation(command.name)
			ref, problem := inv.resolveWorkRef(kind+":rec-1", command.accepts)
			if slices.Contains(command.accepts, kind) {
				if problem != nil || ref.kind != kind || ref.id != "rec-1" || ref.qualified() != kind+":rec-1" {
					t.Errorf("%s %s:rec-1 = %+v %+v; the action takes the kind", command.name, kind, ref, problem)
				}
				continue
			}
			if problem == nil || problem.code != 2 || !strings.Contains(problem.Summary, "does not take a "+refKindNames[kind]+" reference") {
				t.Errorf("%s %s:rec-1 = %+v; a kind the action does not take is refused", command.name, kind, problem)
				continue
			}
			for _, name := range accepting[kind] {
				if !strings.Contains(problem.Decision, "metasystem "+name) {
					t.Errorf("%s %s:rec-1 refusal does not name %s: %q", command.name, kind, name, problem.Decision)
				}
			}
		}
	}
	// The refusal happens through the public command before any owner runs.
	owners := b.owners()
	owners.processes.cancelDispatch = func(string, string) (map[string]any, int, error) {
		t.Fatal("a refused reference reached the cancel owner")
		return nil, 0, nil
	}
	for _, args := range [][]string{{"work", "land", "run:rec-1"}, {"work", "stop", "run:rec-1"}, {"test", "wait", "j2:rec-1"}, {"work", "finish", "read:read-000000000000000000000000"}} {
		if code, result := b.runJSON(owners, args...); code != 2 || result.Outcome != intentRefused || result.Decision == "" {
			t.Errorf("%v = %d %+v", args, code, result)
		}
	}
	// An unknown qualified job is refused, never passed on.
	if code, result := b.runJSON(owners, "work", "stop", "j2:no-such"); code != 1 || result.Outcome != intentRefused || !strings.Contains(result.Summary, "no dispatch job names") {
		t.Errorf("unknown j2 = %d %+v", code, result)
	}
}

// TestIntentReferenceWaitResumesOrStarts: work wait wait:ID resumes that
// durable wait; work wait on a job starts a new wait; the two reach
// different owner forms.
func TestIntentReferenceWaitResumesOrStarts(t *testing.T) {
	t.Parallel()
	b := newReferenceBed(t)
	b.dispatchJob("job-c", "running")
	owners := b.owners()
	var calls [][]string
	owners.work.wait = func(args []string, emit func(metarun.WaitResult, bool)) int {
		calls = append(calls, slices.Clone(args))
		emit(metarun.WaitResult{WaitID: "0123456789abcdef0123456789abcdef", ExitCode: metarun.ExitWaitDeadline}, false)
		return metarun.ExitWaitDeadline
	}
	code, resumed := b.runJSON(owners, "work", "wait", "wait:0123456789abcdef0123456789abcdef", "--timeout", "1s")
	if code != metarun.ExitWaitDeadline || resumed.Outcome != intentInProgress || len(calls) != 1 ||
		!slices.Contains(calls[0], "--resume") || slices.Contains(calls[0], "--job") {
		t.Fatalf("wait:ID = %d %+v %v", code, resumed, calls)
	}
	if resumed.Next == nil || !slices.Equal(resumed.Next.Argv[:4], []string{"metasystem", "work", "wait", "wait:0123456789abcdef0123456789abcdef"}) {
		t.Fatalf("a resumed wait continues with its qualified reference: %+v", resumed.Next)
	}
	code, started := b.runJSON(owners, "work", "wait", "j2:job-c", "--timeout", "1s")
	if code != metarun.ExitWaitDeadline || len(calls) != 2 || !slices.Contains(calls[1], "--job") || slices.Contains(calls[1], "--resume") || started.Targets[0].ID != "j2:job-c" {
		t.Fatalf("j2:job-c = %d %+v %v", code, started, calls)
	}
	// The continuation of a new wait is the resume of the wait it recorded.
	if started.Next == nil || !strings.HasPrefix(started.Next.Argv[3], "wait:") {
		t.Fatalf("a new wait's continuation is not a wait: reference: %+v", started.Next)
	}
}
