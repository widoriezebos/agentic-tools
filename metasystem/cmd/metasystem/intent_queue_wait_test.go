package main

import (
	"bytes"
	"encoding/json"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	metarun "github.com/widoriezebos/agentic-tools/metasystem/internal/run"
)

// TestIntentQueueOnly: land G --queue-only is exactly the real land-ready
// act on the goal ledger, with no proof, collection or publication (the bed
// gives no delivery owner, so any such call would fail). A machine whose one
// landing slot is taken is refused with the public landing of the goal that
// holds it.
func TestIntentQueueOnly(t *testing.T) {
	t.Parallel()
	bed := newIntentBed(t, false, nil)
	bed.lineage = "m1"
	if file := bed.goalFile(bedGoal); file.State != goal.StateClaimed || file.Landing != nil {
		t.Fatalf("the bed goal must be held and not queued: %+v", file)
	}
	code, result := bed.runJSON(bed.owners(), "land", bedGoal, "--queue-only")
	if code != 0 || result.Outcome != intentConfirmed || bed.goalFile(bedGoal).Landing == nil {
		t.Fatalf("queue-only = %d %+v", code, result)
	}
	if code, result = bed.runJSON(bed.owners(), "land", bedGoal, "--queue-only", "--through", "abc"); code != 2 || result.Outcome != intentRefused {
		t.Fatalf("queue-only with another choice = %d %+v", code, result)
	}

	second := newIntentBed(t, false, nil)
	second.lineage = "m1"
	earlier := second.goalFile(bedGoal)
	earlier.Id, earlier.Landing = "earlier-landing", &goal.LandingRecord{At: "2026-08-31T09:00:00Z", Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FC0", "mac-cli", "m1")}
	for index := range earlier.History {
		earlier.History[index].Targets = []string{"earlier-landing"}
	}
	second.addGoal(earlier)
	code, result = second.runJSON(second.owners(), "land", bedGoal, "--queue-only")
	if result.Outcome != intentRefused || result.Next == nil || !slices.Equal(result.Next.Argv, []string{"metasystem", "land", "earlier-landing"}) ||
		second.goalFile(bedGoal).Landing != nil {
		t.Fatalf("a taken landing slot = %d %+v", code, result)
	}
}

// TestIntentGoalEventWait: goal events are waited for with --for and
// --since, the old --event and --after spellings keep working, a bare wait
// with nothing running names the landing wait for a queued goal, and every
// goal selector reaches the wait owner through its real validator. The wait
// owner's registration is the bed's seam.
func TestIntentGoalEventWait(t *testing.T) {
	t.Parallel()
	bed := newWorkBed(t)
	bed.lineage = "m1"
	owners := bed.workOwners()
	var waitArgs []string
	owners.work.wait = func(args []string, print func(metarun.WaitResult, bool)) int {
		waitArgs = args
		print(metarun.WaitResult{SchemaVersion: 2, WaitID: "wait-1", ExitCode: metarun.ExitGreen, SourceOutcome: "landed"}, true)
		return metarun.ExitGreen
	}
	run := func(args ...string) (int, intentResult) {
		var stdout, stderr bytes.Buffer
		waitArgs = nil
		code := runIntentIn(mustIntentCommand(t, args[0]), append(args[1:], "--json"), &stdout, &stderr, bed.root(), owners)
		var result intentResult
		if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
			t.Fatalf("%v: %v %q %q", args, err, stdout.String(), stderr.String())
		}
		return code, result
	}
	if code, result := run("wait", bed.id, "--for", "landing", "--since", "abc"); code != 0 || result.Outcome != intentConfirmed ||
		!slices.Equal(waitArgs[2:], []string{"--goal", bed.id, "--event", "landing", "--after", "abc"}) {
		t.Fatalf("wait --for landing: %d %+v %v", code, result, waitArgs)
	}
	if code, result := run("wait", bed.id, "--for", "human-act", "--verb", "approve", "--since", "abc"); code != 0 ||
		!slices.Equal(waitArgs[2:], []string{"--goal", bed.id, "--event", "human-act", "--after", "abc", "--verb", "approve"}) {
		t.Fatalf("wait --for human-act: %d %+v %v", code, result, waitArgs)
	}
	if code, result := run("wait", bed.id, "--event", "landing", "--after", "abc"); code != 0 ||
		!slices.Equal(waitArgs[2:], []string{"--goal", bed.id, "--event", "landing", "--after", "abc"}) {
		t.Fatalf("legacy spellings: %d %+v %v", code, result, waitArgs)
	}
	if code, result := run("wait", bed.id, "--for", "lunch"); result.Outcome != intentRefused || waitArgs != nil {
		t.Fatalf("an unknown event reaches the owner: %d %+v", code, result)
	}
	// Nothing runs and the goal is held: the goal's status is offered.
	if _, result := run("wait", bed.id); result.Outcome != intentUnchanged || result.Next == nil || !slices.Contains(result.Next.Argv, "status") || waitArgs != nil {
		t.Fatalf("bare wait with nothing running: %+v", result)
	}
	// Once queued to land, the landing wait is offered instead.
	if code, result := run("land", bed.id, "--queue-only"); code != 0 {
		t.Fatalf("queue-only: %d %+v", code, result)
	}
	if _, result := run("wait", bed.id); result.Next == nil || !slices.Equal(result.Next.Argv, []string{"metasystem", "wait", "goal", bed.id, "--for", "landing"}) ||
		!strings.Contains(result.Summary, "no running work") {
		t.Fatalf("bare wait on a queued goal: %+v", result)
	}
}

// TestIntentWaitObservers: wait proof REF and wait file PATH reach the wait
// owner's attempt and path observers (the owner's registration is the bed's
// seam); a relative path resolves against the current directory.
func TestIntentWaitObservers(t *testing.T) {
	t.Parallel()
	bed := newWorkBed(t)
	owners := bed.workOwners()
	var waitArgs []string
	owners.work.wait = func(args []string, print func(metarun.WaitResult, bool)) int {
		waitArgs = args
		print(metarun.WaitResult{SchemaVersion: 2, WaitID: "wait-9", ExitCode: metarun.ExitWaitDeadline, SourceOutcome: "wait-deadline"}, true)
		return metarun.ExitWaitDeadline
	}
	run := func(args ...string) (int, intentResult) {
		var stdout, stderr bytes.Buffer
		waitArgs = nil
		code := runIntentIn(mustIntentCommand(t, args[0]), append(args[1:], "--json"), &stdout, &stderr, bed.root(), owners)
		var result intentResult
		if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
			t.Fatalf("%v: %v %q %q", args, err, stdout.String(), stderr.String())
		}
		return code, result
	}
	if _, result := run("wait", "file", "out/ready.txt", "--until", "present"); result.Outcome != intentInProgress ||
		!slices.Contains(waitArgs, bed.root()+"/out/ready.txt") || !slices.Contains(waitArgs, "present") ||
		result.Next == nil || !slices.Equal(result.Next.Argv, []string{"metasystem", "wait", "resume", "wait-9"}) {
		t.Fatalf("wait file: %+v %v", result, waitArgs)
	}
	if _, result := run("wait", "proof", "attempt-7"); result.Outcome != intentInProgress || !slices.Equal(waitArgs[2:], []string{"--attempt", "attempt-7"}) {
		t.Fatalf("wait proof: %+v %v", result, waitArgs)
	}
	for _, args := range [][]string{{"wait", "file", "x"}, {"wait", "file", "x", "--until", "maybe"}, {"wait", "proof", "a", "--until", "present"}, {"wait", bed.id, "--until", "present"}} {
		if code, result := run(args...); code != 2 || result.Outcome != intentRefused || waitArgs != nil {
			t.Fatalf("%v = %d %+v", args, code, result)
		}
	}
}

// TestIntentWaitRealOwner: the public file wait registers through the real
// wait owner (this process is the announced lease holder of the synthetic
// root, and the repository's fake adapter answers delivery), reaches its
// deadline with a durable wait record, wait resume continues that same
// record, and once the file exists the resumed wait confirms.
func TestIntentWaitRealOwner(t *testing.T) {
	t.Parallel()
	bed := newWorkBed(t)
	bed.lineage = "m1"
	announceProofFixtureHolder(t, bed.root())
	// The holder's runtime adapter is the repository's own fake adapter,
	// whose wait-delivery answers for the fixture runtime.
	adapters := filepath.Join(bed.root(), "scripts", "agents", "adapters")
	os.MkdirAll(adapters, 0o755)
	for _, name := range []string{"fake.sh", "runtime-common.sh"} {
		data, err := os.ReadFile(filepath.Join("..", "..", "scripts", "agents", "adapters", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := testexec.WriteFile(filepath.Join(adapters, name), data, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	owners := bed.workOwners()
	owners.work.wait = nil
	var lastErr string
	run := func(args ...string) (int, intentResult) {
		var stdout, stderr bytes.Buffer
		defer func() { lastErr = stderr.String() }()
		code := runIntentIn(mustIntentCommand(t, args[0]), append(args[1:], "--json"), &stdout, &stderr, bed.root(), owners)
		var result intentResult
		if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
			t.Fatalf("%v: %v %q %q", args, err, stdout.String(), stderr.String())
		}
		return code, result
	}
	code, result := run("wait", "file", "out/ready.txt", "--until", "present", "--timeout", "1s")
	data, _ := result.Data.(map[string]any)
	id, _ := data["waitId"].(string)
	if result.Outcome != intentInProgress || id == "" || result.Next == nil || !slices.Equal(result.Next.Argv, []string{"metasystem", "wait", "resume", id}) {
		t.Fatalf("file wait through the real owner: code=%d %+v stderr=%q", code, result, lastErr)
	}
	code, resumed := run("wait", "resume", id, "--timeout", "1s")
	if resumed.Outcome != intentInProgress || resumed.Next == nil || resumed.Next.Argv[3] != id {
		t.Fatalf("resume of the same wait record: code=%d %+v", code, resumed)
	}
	os.MkdirAll(filepath.Join(bed.root(), "out"), 0o755)
	os.WriteFile(filepath.Join(bed.root(), "out", "ready.txt"), []byte("ready\n"), 0o644)
	if code, done := run("wait", "resume", id, "--timeout", "5s"); code != 0 || done.Outcome != intentConfirmed {
		t.Fatalf("the resumed wait after the file appeared: code=%d %+v", code, done)
	}
}

// TestIntentReservedGoalNames: a goal named like a target word is served by
// the explicit goal forms, which the generated continuations use; the
// target words keep their own meaning. Nothing guesses by goal existence.
func TestIntentReservedGoalNames(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"ui", "file", "proof", "job", "run", "question", "resume", "checkout", "goal", "changes", "diff", "commit", "design"} {
		bed := newWorkBedWith(t, func(file *goal.GoalFile) { workApprovedBox(file) })
		bed.id = name
		file := bed.goalFile(bedGoal)
		file.Id = name
		for index := range file.History {
			file.History[index].Targets = []string{name}
		}
		bed.addGoal(file)
		owners := bed.workOwners()
		run := func(args ...string) intentResult {
			var stdout, stderr bytes.Buffer
			runIntentIn(mustIntentCommand(t, args[0]), append(args[1:], "--json"), &stdout, &stderr, bed.root(), owners)
			var result intentResult
			if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
				t.Fatalf("%v: %v %q %q", args, err, stdout.String(), stderr.String())
			}
			return result
		}
		if status := run("status", "goal", name); status.Outcome != intentConfirmed || !strings.Contains(status.Summary, "goal "+name) {
			t.Fatalf("status goal %s: %+v", name, status)
		}
		if slices.Contains(reviewSubjectWords, name) {
			if words := reviewGoalWords(name); !slices.Equal(words, []string{"review", "goal", name}) {
				t.Fatalf("review continuation for goal %s: %v", name, words)
			}
			if reviewed := run("review", "goal", name); reviewed.Outcome == intentRefused && strings.Contains(reviewed.Summary, "review has no subject") {
				t.Fatalf("review goal %s did not reach the goal: %+v", name, reviewed)
			}
			var built intentResult
			{
				var stdout, stderr bytes.Buffer
				runIntentIn(mustIntentCommand(t, "build"), append([]string{name, "--json", "--brief", bed.brief("b.md", "Build it.\n"), "--lines", "5"}, workCheck...), &stdout, &stderr, bed.root(), owners)
				if err := json.Unmarshal(stdout.Bytes(), &built); err != nil {
					t.Fatalf("build %s: %v %q %q", name, err, stdout.String(), stderr.String())
				}
			}
			if built.Outcome != intentConfirmed || built.Next == nil || !slices.Equal(built.Next.Argv[:4], []string{"metasystem", "review", "goal", name}) {
				t.Fatalf("build of goal %s continues with its unambiguous review: %+v", name, built)
			}
			followed := run(built.Next.Argv[1:]...)
			for _, wrong := range []string{"review names what to review", "--work names a goal's work", "review has no subject", "diagnostic"} {
				if strings.Contains(followed.Summary, wrong) {
					t.Fatalf("following %v reached another subject: %+v", built.Next.Argv, followed)
				}
			}
			if !slices.ContainsFunc(followed.Targets, func(target intentTarget) bool { return target.Kind == "goal" && target.ID == name }) {
				t.Fatalf("following %v did not act on goal %s: %+v", built.Next.Argv, name, followed)
			}
		}
		if waited := run("wait", "goal", name); waited.Outcome != intentUnchanged || waited.Next == nil || !slices.Equal(waited.Next.Argv[:4], []string{"metasystem", "status", "goal", name}) {
			t.Fatalf("wait goal %s: %+v", name, waited)
		}
	}
}

// TestIntentProofReferenceFeedsWaitProof: the public test command names the
// proof attempt its runner reports, and a run that has not ended continues
// with wait proof of exactly that attempt, which the wait owner receives.
func TestIntentProofReferenceFeedsWaitProof(t *testing.T) {
	t.Parallel()
	bed := newWorkBed(t)
	owners := bed.workOwners()
	exit := metarun.ExitInterrupted
	owners.work.subprocess = func(dir string, argv []string, stderr io.Writer) ([]byte, int, error) {
		return []byte(`{"attemptId":"proof-20260926-a1","status":"running"}`), exit, nil
	}
	var waitArgs []string
	owners.work.wait = func(args []string, print func(metarun.WaitResult, bool)) int {
		waitArgs = args
		print(metarun.WaitResult{SchemaVersion: 2, ExitCode: metarun.ExitGreen, SourceOutcome: "green"}, true)
		return metarun.ExitGreen
	}
	run := func(args ...string) (int, intentResult) {
		var stdout, stderr bytes.Buffer
		code := runIntentIn(mustIntentCommand(t, args[0]), append(args[1:], "--json"), &stdout, &stderr, bed.root(), owners)
		var result intentResult
		if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
			t.Fatalf("%v: %v %q %q", args, err, stdout.String(), stderr.String())
		}
		return code, result
	}
	_, tested := run("test")
	if tested.Outcome != intentInProgress || !slices.Contains(tested.Targets, intentTarget{Kind: "proof", ID: "proof-20260926-a1"}) ||
		tested.Next == nil || !slices.Equal(tested.Next.Argv, []string{"metasystem", "wait", "proof", "proof-20260926-a1"}) {
		t.Fatalf("an unfinished proof names its attempt and its wait: %+v", tested)
	}
	if code, waited := run(tested.Next.Argv[1:]...); code != 0 || waited.Outcome != intentConfirmed || !slices.Equal(waitArgs[2:], []string{"--attempt", "proof-20260926-a1"}) {
		t.Fatalf("the printed wait reaches the wait owner with the attempt: %d %+v %v", code, waited, waitArgs)
	}
	exit = 0
	if _, done := run("test"); done.Outcome != intentConfirmed || done.Next != nil || !slices.Contains(done.Targets, intentTarget{Kind: "proof", ID: "proof-20260926-a1"}) {
		t.Fatalf("a finished proof still names its attempt: %+v", done)
	}
}

// TestIntentWaitJobKeepsTheSelectedOwner follows the wait commands status
// work lists. A launch reference is awaited by the launch manager (its
// bounded wait continues with the same reference, a later wait sees it end,
// and a launch whose processes are dead is failed as lost), while the
// dispatch job with the same raw id is awaited by the durable wait owner
// with that raw id. Neither is sent to the other's owner.
func TestIntentWaitJobKeepsTheSelectedOwner(t *testing.T) {
	t.Parallel()
	b := newProcessBed(t)
	owners := b.owners()
	clock := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	settings := launch.DefaultSettings()
	settings.WaitCapSeconds = 600
	manager := &launch.Manager{Store: launch.Store{Root: b.launchDir}, Settings: settings, Prober: workProber{}, Poll: time.Second,
		Now: func() time.Time { return clock }}
	finishOnSleep := ""
	manager.Sleep = func(d time.Duration) {
		clock = clock.Add(d)
		if finishOnSleep != "" {
			manager.Store.Update(finishOnSleep, func(current *launch.Record) error {
				code := 0
				current.State, current.ExitCode = launch.Completed, &code
				return nil
			})
		}
	}
	owners.processes.launches = func() *launch.Manager { return manager }
	supervisor, child, lost := workRef(10), workRef(20), workRef(30)
	for _, record := range []launch.Record{
		{ID: "solo-1", Kind: "read", State: launch.Running, Supervisor: &supervisor, Child: &child},
		{ID: "pair-1", Kind: "build", State: launch.Running, Supervisor: &supervisor, Child: &child},
		{ID: "lost-1", Kind: "read", State: launch.Running, Supervisor: &lost, Child: &lost},
	} {
		if err := manager.Store.Create(record); err != nil {
			t.Fatal(err)
		}
	}
	jobs := filepath.Join(b.root(), "artifacts", "agents", "jobs")
	os.MkdirAll(jobs, 0o755)
	os.WriteFile(filepath.Join(jobs, "pair-1.json"), []byte(`{"status":"running","job":"pair-1","role":"code-critic"}`), 0o644)
	var dispatchWaits [][]string
	owners.work.wait = func(args []string, print func(metarun.WaitResult, bool)) int {
		dispatchWaits = append(dispatchWaits, args)
		print(metarun.WaitResult{SchemaVersion: 2, ExitCode: metarun.ExitGreen, SourceOutcome: "dispatch-completed"}, true)
		return metarun.ExitGreen
	}
	run := func(args ...string) (int, intentResult) { return b.runJSON(owners, args...) }

	_, listed := run("status", "work")
	waits := map[string][]string{}
	for _, job := range listed.Data.(map[string]any)["jobs"].([]any) {
		view := job.(map[string]any)
		if argv, ok := view["wait"].([]any); ok {
			words := []string{}
			for _, word := range argv {
				words = append(words, word.(string))
			}
			waits[view["reference"].(string)] = words
		}
	}
	for _, ref := range []string{"j1:solo-1", "j1:pair-1", "j2:pair-1"} {
		if waits[ref] == nil {
			t.Fatalf("status work lists no wait for %s: %v", ref, waits)
		}
	}
	// The launch-only reference: a bounded wait continues with the same
	// reference, and the continued wait sees the launch end.
	code, bounded := run(waits["j1:solo-1"][1:]...)
	if bounded.Outcome != intentInProgress || bounded.Next == nil || !slices.Equal(bounded.Next.Argv[1:4], []string{"wait", "job", "j1:solo-1"}) || len(dispatchWaits) != 0 {
		t.Fatalf("bounded launch wait: code=%d %+v dispatch=%v", code, bounded, dispatchWaits)
	}
	finishOnSleep = "solo-1"
	code, ended := run(append(slices.Clone(bounded.Next.Argv[1:]), "--timeout", "1m")...)
	if code != 0 || ended.Outcome != intentConfirmed || !strings.Contains(ended.Summary, "launch solo-1 ended: completed") || len(dispatchWaits) != 0 {
		t.Fatalf("continued launch wait: code=%d %+v", code, ended)
	}
	// The same raw id in both stores: each reference reaches its own owner.
	finishOnSleep = "pair-1"
	code, launchEnded := run(append(slices.Clone(waits["j1:pair-1"][1:]), "--timeout", "1m")...)
	if code != 0 || !strings.Contains(launchEnded.Summary, "launch pair-1 ended: completed") || len(dispatchWaits) != 0 {
		t.Fatalf("launch half of the pair: code=%d %+v", code, launchEnded)
	}
	code, dispatchEnded := run(waits["j2:pair-1"][1:]...)
	if code != 0 || !strings.Contains(dispatchEnded.Summary, "dispatch-completed") || len(dispatchWaits) != 1 ||
		!slices.Equal(dispatchWaits[0][2:4], []string{"--job", "pair-1"}) || dispatchEnded.Targets[0].ID != "j2:pair-1" {
		t.Fatalf("dispatch half of the pair: code=%d %+v %v", code, dispatchEnded, dispatchWaits)
	}
	// A launch whose supervisor and child are dead is failed as lost by its
	// owner, never reported running.
	finishOnSleep = ""
	code, lostWait := run("wait", "job", "j1:lost-1")
	if lostWait.Outcome != intentConfirmed || !strings.Contains(lostWait.Summary, "failed") || len(dispatchWaits) != 1 {
		t.Fatalf("a launch with dead processes: code=%d %+v", code, lostWait)
	}
	if record, _ := manager.Store.Read("lost-1"); record.State != launch.Failed || record.Reason != "supervisor-lost" {
		t.Fatalf("the launch owner did not record the loss: %+v", record)
	}
}
