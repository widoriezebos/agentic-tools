package deploy

// Decision 3 of the landing-deploys-the-engine design, row by row, driven
// through the fixture adapter: a failed deploy never undoes the push, a
// rollback always pauses, and what a rollback returns to is the deploy
// before the newest forward one.

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)

// startRun runs the runner by `by` in the background; started hears each
// adapter process it starts and is closed when the run ends.
func startRun(t *testing.T, b *bed, by string) (<-chan Active, <-chan Report) {
	t.Helper()
	started := make(chan Active, 32)
	done := make(chan Report, 1)
	runner := b.runner(by, started)
	go func() {
		defer close(started)
		report, err := runner.Run()
		if err != nil {
			report.Outcome = "error: " + err.Error()
		}
		done <- report
	}()
	return started, done
}

func requireRefusal(t *testing.T, err error, code string) {
	t.Helper()
	var refusal *Refusal
	if !errors.As(err, &refusal) || refusal.Code != code {
		t.Fatalf("err = %v; want the refusal %s", err, code)
	}
}

// First deploy on a computer: version answers none, the line's previous is
// null, and a rollback is refused as DEPLOY_NO_PREVIOUS.
func TestFirstDeployHasNoPreviousAndNoRollback(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	line := b.deployed("c1")
	if line.Previous != "" || !strings.Contains(b.file(recordPath(b.dir)), `"previous":null`) {
		t.Fatalf("the first deploy's previous = %q; want null in\n%s", line.Previous, b.file(recordPath(b.dir)))
	}
	if got, want := strings.Join(b.calls(), ","), "version c1,build c1,activate c1,version c1"; got != want {
		t.Fatalf("calls = %s; want %s", got, want)
	}
	record := b.file(recordPath(b.dir))
	_, err := b.runner("Wido", nil).Rollback()
	requireRefusal(t, err, CodeNoPrevious)
	if b.file(recordPath(b.dir)) != record || b.file(pausePath(b.dir)) != "" || b.count("rollback c1") != 0 {
		t.Fatal("a refused rollback changed the record, paused or called the adapter")
	}
}

// Build fails after a push: a build-failed line with the log's path; the
// previous deploy stays active; the next trigger tries again.
func TestBuildFailureKeepsThePreviousDeploy(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.deployed("c1")
	b.script("build", step{Exit: 1, Reason: "compile error in main.go"}, step{})
	b.main("c2")
	report := b.run("lane")
	failed := report.Lines[len(report.Lines)-1]
	if report.Outcome != RunFailed || failed.Outcome != OutcomeBuildFailed || !strings.Contains(failed.Detail, "compile error") || failed.Previous != "c1" {
		t.Fatalf("report = %+v", report)
	}
	if log := b.file(failed.Log); !strings.Contains(log, "build of c2") {
		t.Fatalf("the line's log %s holds %q; want the adapter's standard error", failed.Log, log)
	}
	if b.active() != b.artifact("c1") || b.count("activate c2") != 0 {
		t.Fatalf("a failed build changed what is active: %s", b.active())
	}
	if again := b.run("Wido"); again.Outcome != RunDeployed || b.active() != b.artifact("c2") {
		t.Fatalf("the next trigger = %+v; want c2 deployed", again)
	}
}

// The exit meanings: 64 is not supported; another exit or unreadable
// output leaves the state unknown, and version tells what is active.
func TestAdapterExitMeaningsDecideTheLine(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.deployed("c1")
	b.main("c2")
	b.script("build", step{Exit: 64})
	if line := b.run("lane").Lines[0]; line.Outcome != OutcomeBuildFailed || !strings.Contains(line.Detail, "does not support build") {
		t.Fatalf("exit 64 = %+v", line)
	}
	b.script("build", step{})
	b.script("activate", step{Exit: 3, Raw: "not json"})
	if line := b.run("lane").Lines[0]; line.Outcome != OutcomeActivateFailed || !strings.Contains(line.Detail, "version reports version v-c1") {
		t.Fatalf("an unreadable activate that changed nothing = %+v", line)
	}
	b.script("activate", step{Exit: 3, Apply: true})
	if line := b.run("lane").Lines[0]; line.Outcome != OutcomeActive || !strings.Contains(line.Detail, "exited 3") || b.active() != b.artifact("c2") {
		t.Fatalf("an unreadable activate that took effect = %+v", line)
	}
}

// Activation interrupted: the runner's process dies after the adapter
// switched, while it activates or while version verifies it; no line was
// written and the lock died with the process. The next run asks version,
// records what is active, removes the leftover clean tree and continues,
// and the deploy before it stays the one a rollback returns to.
func TestInterruptedActivationIsRecordedByTheNextRun(t *testing.T) {
	t.Parallel()
	for name, hold := range map[string]func(b *bed, fifo string){
		"activate": func(b *bed, fifo string) { b.script("activate", step{HoldAfter: fifo}) },
		"version":  func(b *bed, fifo string) { b.script("version", step{}, step{HoldBefore: fifo}) },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b := newBed(t)
			b.deployed("c1")
			b.deployed("c2")
			hold(b, b.fifo("held"))
			child := b.child("run", "c3")
			b.crash(child, b.inFlight(name, "c3"), true)
			if got := outcomes(b.lines()); got != "deploy:c1:active deploy:c2:active" {
				t.Fatalf("the interrupted run wrote %s", got)
			}
			b.script("activate", step{})
			b.script("version", step{})
			b.main("c3")
			report := b.run("Wido")
			if report.Outcome != RunCurrent || len(report.Recorded) != 1 || report.Recorded[0].Commit != "c3" || report.Recorded[0].Previous != "c2" || report.Recorded[0].Kind != KindDeploy {
				t.Fatalf("the next run = %+v; want c3 recorded active over c2 and nothing built", report)
			}
			if b.count("build c3") != 1 {
				t.Fatalf("c3 was built %d times", b.count("build c3"))
			}
			if entries, _ := os.ReadDir(workDir(b.dir)); len(entries) != 0 {
				t.Fatalf("leftover clean trees remain: %v", entries)
			}
			if rollback, err := b.runner("Wido", nil).Rollback(); err != nil || rollback.Target.Commit != "c2" {
				t.Fatalf("rollback after the recorded activation = %+v %v; want c2", rollback, err)
			}
		})
	}
}

// A rollback interrupted after the adapter switched is recorded as the
// rollback it was, so it is never taken for a forward deploy: asked again
// after a failed redeploy, the rollback already holds.
func TestInterruptedRollbackIsRecordedAsARollback(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.deployed("c1")
	b.deployed("c2")
	b.script("rollback", step{HoldAfter: b.fifo("rollback")})
	child := b.child("rollback", "c2")
	b.crash(child, b.inFlight("rollback", "c1"), true)
	b.script("rollback", step{})
	if _, paused, _ := ReadPause(b.dir); !paused || outcomes(b.lines()) != "deploy:c1:active deploy:c2:active" {
		t.Fatalf("the interrupted rollback left paused %v and %s", paused, outcomes(b.lines()))
	}
	if _, _, err := b.runner("Wido", nil).Resume(); err != nil {
		t.Fatal(err)
	}
	b.script("build", step{Exit: 1, Reason: "flaky"})
	b.main("c2")
	report := b.run("Wido")
	if len(report.Recorded) != 1 || report.Recorded[0].Kind != KindRollback || report.Recorded[0].Commit != "c1" || report.Recorded[0].Previous != "c2" || report.Outcome != RunFailed {
		t.Fatalf("the run after resume = %+v; want the rollback to c1 recorded, then c2's failed build", report)
	}
	again, err := b.runner("Wido", nil).Rollback()
	if err != nil || again.Outcome != RollbackHolds || again.Target.Commit != "c1" || b.active() != b.artifact("c1") {
		t.Fatalf("the rollback asked again = %+v %v, active %s; want it to hold at c1", again, err, b.active())
	}
}

// A runner that dies leaves its adapter running: that adapter holds the
// deploy, so status shows it and neither a second run nor a rollback starts
// beside it, until a person's pause ends its process group.
func TestAnAdapterLeftByADeadRunHoldsTheDeployUntilPaused(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.deployed("c1")
	b.script("build", step{HoldBefore: b.fifo("stalled")}, step{})
	child := b.child("run", "c2")
	building := b.inFlight("build", "")
	b.crash(child, building, false)
	t.Cleanup(func() { _ = syscall.Kill(-building.PID, syscall.SIGKILL) })
	b.awaitHeld()
	if status, err := ReadStatus(b.dir, 0); err != nil || status.Adapter == nil || status.Adapter.PID != building.PID {
		t.Fatalf("status with the adapter left running = %+v %v", status, err)
	}
	b.main("c2")
	if report := b.run("lane"); report.Outcome != RunBusy || !report.Orphaned || report.Holder.PID != building.PID || b.count("build c2") != 1 {
		t.Fatalf("a trigger beside the adapter left running = %+v, builds of c2 %d", report, b.count("build c2"))
	}
	_, err := b.runner("Wido", nil).Rollback()
	requireRefusal(t, err, CodeRunning)
	pause, err := b.runner("Wido", nil).Pause("the build its run left")
	if err != nil || pause.Stopped == nil || pause.Stopped.PID != building.PID {
		t.Fatalf("pause = %+v %v", pause, err)
	}
	if err := testenv.AwaitProcessTargetGone(-building.PID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := b.runner("Wido", nil).Resume(); err != nil {
		t.Fatal(err)
	}
	if report := b.run("Wido"); report.Outcome != RunDeployed || b.active() != b.artifact("c2") {
		t.Fatalf("after resume = %+v", report)
	}
}

// Trigger while paused: nothing is built, not even main is fetched; the
// runner exits at once and does not start over.
func TestTriggerWhilePausedBuildsNothing(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.deployed("c1")
	if _, err := b.runner("Wido", nil).Pause("maintenance"); err != nil {
		t.Fatal(err)
	}
	b.main("c2")
	calls, fetches := len(b.calls()), b.fetchCount()
	report := b.run("lane")
	if report.Outcome != RunPaused || len(report.Lines) != 0 || len(b.calls()) != calls || b.fetchCount() != fetches {
		t.Fatalf("a paused trigger = %+v, %d new calls, %d new fetches", report, len(b.calls())-calls, b.fetchCount()-fetches)
	}
	status, err := ReadStatus(b.dir, 0)
	if err != nil || status.Paused == nil || status.Paused.By != "Wido" || status.Paused.Reason != "maintenance" || status.Current.Commit != "c1" {
		t.Fatalf("status = %+v %v", status, err)
	}
}

// Two pushes close together: the trigger that finds the lock held exits;
// the holder fetches main after its attempt and deploys the newest tip. A
// commit main has moved past is never built.
func TestTwoTriggersDeployOnlyTheNewestTip(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.deployed("c1")
	hold := b.fifo("build")
	b.script("build", step{HoldBefore: hold}, step{})
	b.main("c2")
	started, done := startRun(t, b, "lane")
	awaitStart(t, started, "build", "c2")
	fetches := b.fetchCount()
	second := b.run("lane")
	if second.Outcome != RunBusy || second.Holder == nil || second.Holder.Operation != "build" || second.Holder.Commit != "c2" || b.fetchCount() != fetches {
		t.Fatalf("the second trigger = %+v; want it to exit at once, naming the run in progress", second)
	}
	// c3 and then c4 land while c2 builds: the holder's next fetch finds c4.
	b.main("c4")
	b.release(hold)
	report := <-done
	if report.Outcome != RunDeployed || outcomes(report.Lines) != "deploy:c2:active deploy:c4:active" {
		t.Fatalf("the holder's run = %+v", report)
	}
	if b.count("build c2") != 1 || b.count("build c4") != 1 || b.active() != b.artifact("c4") {
		t.Fatalf("calls = %v", b.calls())
	}
}

// Triggers out of order: a push that found the lock held just before the
// release is not lost, because the holder fetches main once more after it.
func TestRunStartsOverForATipThatLandedAsTheLockWasReleased(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.deployed("c1")
	b.main("c2", "c2", "c3")
	report := b.run("lane")
	if report.Outcome != RunDeployed || outcomes(report.Lines) != "deploy:c2:active deploy:c3:active" || report.Lines[1].Previous != "c2" {
		t.Fatalf("report = %+v", report)
	}
}

// deploy now when main's tip is already active: nothing is built or
// written.
func TestDeployOfTheActiveTipWritesNothing(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.deployed("c1")
	record, builds := b.file(recordPath(b.dir)), b.count("build c1")
	report := b.run("Wido")
	if report.Outcome != RunCurrent || len(report.Lines)+len(report.Recorded) != 0 || report.Tip != "c1" {
		t.Fatalf("report = %+v", report)
	}
	if b.file(recordPath(b.dir)) != record || b.count("build c1") != builds {
		t.Fatal("a deploy of the active tip built or wrote")
	}
}

// An adapter stalls: status shows the run, its start and its log's last
// growth; a person's pause ends the adapter's process group, and the run
// appends a stopped line. Whatever version reports stays, paused.
func TestPauseStopsABlockedAdapter(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.deployed("c1")
	b.script("build", step{HoldBefore: b.fifo("stalled")})
	b.main("c2")
	started, done := startRun(t, b, "lane")
	building := awaitStart(t, started, "build", "c2")
	t.Cleanup(func() { _ = syscall.Kill(-building.PID, syscall.SIGKILL) })
	status, err := ReadStatus(b.dir, 0)
	if err != nil || status.Adapter == nil || status.Adapter.PID != building.PID || status.Adapter.Operation != "build" || status.LogGrewAt == nil {
		t.Fatalf("status during the stall = %+v %v", status, err)
	}
	pause, err := b.runner("Wido", nil).Pause("the build hangs")
	if err != nil || pause.Already || pause.Stopped == nil || pause.Stopped.PID != building.PID {
		t.Fatalf("pause = %+v %v", pause, err)
	}
	report := <-done
	last := report.Lines[len(report.Lines)-1]
	if report.Outcome != RunStopped || last.Outcome != OutcomeStopped || last.Commit != "c2" || !strings.Contains(last.Detail, "deploy pause by Wido") {
		t.Fatalf("the stopped run = %+v", report)
	}
	if err := testenv.AwaitProcessTargetGone(-building.PID); err != nil {
		t.Fatal(err)
	}
	if b.active() != b.artifact("c1") {
		t.Fatalf("active = %s; want c1", b.active())
	}
}

// What a rollback returns to is the deploy before the newest forward one;
// asked again, it already holds and nothing is written.
func TestRollbackAskedAgainWritesNothing(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.deployed("c1")
	b.deployed("c2")
	b.deployed("c3")
	report, err := b.runner("Wido", nil).Rollback()
	if err != nil || report.Outcome != RollbackDone || report.Line == nil || report.Line.Kind != KindRollback || report.Line.Commit != "c2" || report.Line.Previous != "c3" {
		t.Fatalf("rollback = %+v %v", report, err)
	}
	if pause, paused, _ := ReadPause(b.dir); !paused || pause.By != "Wido" || b.active() != b.artifact("c2") {
		t.Fatalf("after the rollback paused = %v, active = %s", paused, b.active())
	}
	record, pause := b.file(recordPath(b.dir)), b.file(pausePath(b.dir))
	again, err := b.runner("Wido", nil).Rollback()
	if err != nil || again.Outcome != RollbackHolds || again.Target.Commit != "c2" {
		t.Fatalf("the repeated rollback = %+v %v", again, err)
	}
	if b.file(recordPath(b.dir)) != record || b.file(pausePath(b.dir)) != pause || b.count("rollback c2") != 1 {
		t.Fatal("the repeated rollback wrote or called the adapter")
	}
}

// Rollback with no previous, or whose previous artifact is gone or no
// longer has its checksum, is refused as DEPLOY_NO_PREVIOUS and changes
// nothing.
func TestRollbackWithNoPreviousIsRefused(t *testing.T) {
	t.Parallel()
	for name, spoil := range map[string]func(b *bed){
		"gone":     func(b *bed) { _ = os.Remove(b.artifact("c1")) },
		"checksum": func(b *bed) { _ = os.WriteFile(b.artifact("c1"), []byte("other bytes\n"), 0o644) },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b := newBed(t)
			b.deployed("c1")
			b.deployed("c2")
			spoil(b)
			record := b.file(recordPath(b.dir))
			_, err := b.runner("Wido", nil).Rollback()
			requireRefusal(t, err, CodeNoPrevious)
			if b.file(recordPath(b.dir)) != record || b.file(pausePath(b.dir)) != "" || b.count("rollback c1") != 0 {
				t.Fatal("a refused rollback changed something")
			}
		})
	}
}

// Engine deploys but cannot start: a build whose start check fails is
// never activated; if version fails after activation, the runner rolls
// back, pauses and writes verify-failed.
func TestAnEngineThatCannotStartIsNeverLeftActive(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.deployed("c1")
	b.main("c2")
	b.script("build", step{Exit: 1, Reason: "the built engine does not start"})
	if line := b.run("lane").Lines[0]; line.Outcome != OutcomeBuildFailed || b.count("activate c2") != 0 {
		t.Fatalf("a build that does not start = %+v", line)
	}
	b.script("build", step{})
	b.script("version", step{}, step{Exit: 3})
	report := b.run("lane")
	line := report.Lines[0]
	if report.Outcome != RunFailed || line.Outcome != OutcomeVerifyFailed || !strings.Contains(line.Detail, "rolled back to c1") {
		t.Fatalf("version failing after activation = %+v", report)
	}
	pause, paused, _ := ReadPause(b.dir)
	if !paused || pause.By != "the deploy runner" || b.active() != b.artifact("c1") || b.count("rollback c1") != 1 {
		t.Fatalf("paused = %+v %v, active = %s", pause, paused, b.active())
	}
}

// Engine starts but misbehaves: a person's rollback returns to the
// previous engine and pauses, so the next landing does not deploy the bad
// commit again; only resume moves forward.
func TestRollbackPausesUntilAPersonResumes(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.deployed("c1")
	b.deployed("c2")
	if _, err := b.runner("Wido", nil).Rollback(); err != nil {
		t.Fatal(err)
	}
	b.main("c2")
	if report := b.run("lane"); report.Outcome != RunPaused || b.active() != b.artifact("c1") {
		t.Fatalf("a landing after the rollback = %+v, active %s", report, b.active())
	}
	if was, paused, err := b.runner("Wido", nil).Resume(); err != nil || !paused || was.By != "Wido" {
		t.Fatalf("resume = %+v %v %v", was, paused, err)
	}
	if report := b.run("Wido"); report.Outcome != RunDeployed || b.active() != b.artifact("c2") {
		t.Fatalf("after resume = %+v", report)
	}
	if got := outcomes(b.lines()); got != "deploy:c1:active deploy:c2:active rollback:c1:active deploy:c2:active" {
		t.Fatalf("record = %s", got)
	}
}

// deploy status --verify only reads: it takes no lock, so a trigger that
// comes while the adapter answers it still deploys main's tip, and it
// leaves neither its clean tree nor its log behind.
func TestVerifyLeavesTheLockToTriggers(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.deployed("c1")
	hold := b.fifo("verify")
	t.Cleanup(func() { b.releaseIfHeld(hold) })
	b.script("version", step{HoldBefore: hold}, step{})
	verified := make(chan error, 1)
	go func() {
		_, err := b.runner("Wido", nil).Verify()
		verified <- err
	}()
	testenv.Await(t, "verify's version call to start", func() bool { return b.count("version c1") == 3 })
	b.main("c2")
	if report := b.run("lane"); report.Outcome != RunDeployed || b.active() != b.artifact("c2") {
		t.Fatalf("a trigger during verify = %+v", report)
	}
	b.release(hold)
	if err := <-verified; err != nil {
		t.Fatal(err)
	}
	logs, _ := os.ReadDir(logDir(b.dir))
	trees, _ := os.ReadDir(filepath.Join(b.dir, "verify"))
	if len(trees) != 0 || len(logs) != 2 {
		t.Fatalf("verify left trees %v; logs %v (want one per deploy, of c1 and c2)", trees, logs)
	}
}

// A pause that comes after build returned and before activate starts: no
// activate runs, no active line is written, and c1 stays active.
func TestPauseBetweenBuildAndActivateStartsNoActivation(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.deployed("c1")
	b.main("c2")
	runner := b.runner("lane", nil)
	starting, proceed := make(chan struct{}, 1), make(chan struct{})
	runner.Starting = func(active Active) {
		if active.Operation == "activate" {
			starting <- struct{}{}
			<-proceed
		}
	}
	done := make(chan Report, 1)
	go func() {
		report, _ := runner.Run()
		done <- report
	}()
	select {
	case <-starting:
	case report := <-done:
		t.Fatalf("the run ended before its activate: %+v", report)
	}
	type answer struct {
		report PauseReport
		err    error
	}
	paused := make(chan answer, 1)
	go func() {
		report, err := b.runner("Wido", nil).Pause("between build and activate")
		paused <- answer{report, err}
	}()
	testenv.Await(t, "the pause to be written", func() bool { _, ok, _ := ReadPause(b.dir); return ok })
	close(proceed)
	report, pause := <-done, <-paused
	if pause.err != nil || pause.report.Stopped != nil {
		t.Fatalf("pause = %+v %v; want no adapter call stopped", pause.report, pause.err)
	}
	if report.Outcome != RunStopped || b.count("activate c2") != 0 || b.active() != b.artifact("c1") {
		t.Fatalf("the run = %+v, activations of c2 %d, active %s", report, b.count("activate c2"), b.active())
	}
	if got := outcomes(b.lines()); got != "deploy:c1:active deploy:c2:stopped" {
		t.Fatalf("record = %s", got)
	}
}

// A rollback straight after a run died mid-activation of c3, with version
// reporting c3 and the record's newest active line c2: it records c3 first
// and returns to c2, the deploy before the newest one.
func TestRollbackAfterAnInterruptedActivationReturnsToTheDeployBeforeIt(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.deployed("c1")
	b.deployed("c2")
	b.script("activate", step{HoldAfter: b.fifo("held")})
	child := b.child("run", "c3")
	b.crash(child, b.inFlight("activate", "c3"), true)
	b.script("activate", step{})
	rollback, err := b.runner("Wido", nil).Rollback()
	if err != nil || rollback.Outcome != RollbackDone || rollback.Target.Commit != "c2" || len(rollback.Recorded) != 1 || rollback.Recorded[0].Commit != "c3" {
		t.Fatalf("rollback = %+v %v; want c3 recorded and c2 active again", rollback, err)
	}
	if got := outcomes(b.lines()); got != "deploy:c1:active deploy:c2:active deploy:c3:active rollback:c2:active" || b.active() != b.artifact("c2") {
		t.Fatalf("record = %s, active %s", got, b.active())
	}
}

// Neither a rollback nor a resume runs beside a run: while a run's build is
// held, both refuse as DEPLOY_RUNNING, naming deploy pause, and change
// nothing; once a pause has stopped the run, the rollback is made.
func TestRollbackAndResumeDuringARunAreRefusedUntilAPauseStopsIt(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.deployed("c1")
	b.deployed("c2")
	b.script("build", step{HoldBefore: b.fifo("build")})
	b.main("c3")
	started, done := startRun(t, b, "lane")
	building := awaitStart(t, started, "build", "c3")
	t.Cleanup(func() { _ = syscall.Kill(-building.PID, syscall.SIGKILL) })
	record := b.file(recordPath(b.dir))
	_, err := b.runner("Wido", nil).Rollback()
	requireRefusal(t, err, CodeRunning)
	if !strings.Contains(err.Error(), "deploy pause stops it") {
		t.Fatalf("the refusal %v does not name deploy pause", err)
	}
	_, _, err = b.runner("Wido", nil).Resume()
	requireRefusal(t, err, CodeRunning)
	if b.file(recordPath(b.dir)) != record || b.file(pausePath(b.dir)) != "" || !building.AdapterAlive() || b.count("rollback c1") != 0 {
		t.Fatal("a refused rollback or resume changed the record, the pause or the run")
	}
	if pause, err := b.runner("Wido", nil).Pause("roll back"); err != nil || pause.Stopped == nil || pause.Stopped.PID != building.PID {
		t.Fatalf("pause = %+v %v; want the build stopped", pause, err)
	}
	if report := <-done; report.Outcome != RunStopped {
		t.Fatalf("the stopped run = %+v", report)
	}
	if rollback, err := b.runner("Wido", nil).Rollback(); err != nil || rollback.Outcome != RollbackDone || rollback.Target.Commit != "c1" || b.active() != b.artifact("c1") {
		t.Fatalf("the repeated rollback = %+v %v, active %s; want c1", rollback, err, b.active())
	}
	if got := outcomes(b.lines()); got != "deploy:c1:active deploy:c2:active deploy:c3:stopped rollback:c1:active" {
		t.Fatalf("record = %s", got)
	}
}

// A refused rollback starts nothing: with c1 active and main at c2 it is
// refused as DEPLOY_NO_PREVIOUS, c2 is not built, and nothing is paused.
func TestRefusedRollbackStartsNoDeploy(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.deployed("c1")
	b.main("c2")
	record := b.file(recordPath(b.dir))
	_, err := b.runner("Wido", nil).Rollback()
	requireRefusal(t, err, CodeNoPrevious)
	if b.count("build c2") != 0 || b.file(recordPath(b.dir)) != record || b.file(pausePath(b.dir)) != "" {
		t.Fatalf("a refused rollback built c2 (%d), wrote or paused", b.count("build c2"))
	}
}

// A pause while a rollback's adapter call is held answers at once, while
// the rollback still runs: it ends that call, and the rollback answers
// failed with what version then reports, c1 switched to before the pause,
// and appends its one line, active for c1, so status names c1; its pause
// holds.
func TestPauseEndsAStalledRollback(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.deployed("c1")
	b.deployed("c2")
	before := len(b.lines())
	hold, verify := b.fifo("rollback"), b.fifo("verify")
	t.Cleanup(func() { b.releaseIfHeld(hold); b.releaseIfHeld(verify) })
	b.script("rollback", step{HoldAfter: hold})
	b.script("version", step{}, step{HoldBefore: verify})
	type answer struct {
		report RollbackReport
		err    error
	}
	started, rolled := make(chan Active, 32), make(chan answer, 1)
	go func() {
		report, err := b.runner("Wido", started).Rollback()
		rolled <- answer{report, err}
	}()
	rolling := awaitStart(t, started, "rollback", "c1")
	t.Cleanup(func() { _ = syscall.Kill(-rolling.PID, syscall.SIGKILL) })
	testenv.Await(t, "c1 switched back to in the adapter", func() bool {
		return strings.Contains(b.file(filepath.Join(b.state, "active.json")), b.artifact("c1"))
	})
	pause, err := b.runner("Ann", nil).Pause("the rollback hangs")
	if err != nil || !pause.Already || pause.Stopped == nil || pause.Stopped.PID != rolling.PID {
		t.Fatalf("pause = %+v %v; want the rollback's call stopped", pause, err)
	}
	select {
	case rollback := <-rolled:
		t.Fatalf("the rollback answered %+v before its version call was released", rollback)
	default:
	}
	b.release(verify)
	rollback := <-rolled
	if line := rollback.report.Line; rollback.err != nil || rollback.report.Outcome != RollbackFailed || line == nil || line.Outcome != OutcomeActive ||
		!strings.Contains(line.Detail, "after the switch; version then reports version v-c1 ("+b.artifact("c1")+")") {
		t.Fatalf("rollback = %+v %v; want it failed and its line active for c1, as version reports it", rollback.report, rollback.err)
	}
	requireOneLineNaming(t, b, before, "c1")
	if err := testenv.AwaitProcessTargetGone(-rolling.PID); err != nil {
		t.Fatal(err)
	}
	b.main("c2")
	if pause, held, _ := ReadPause(b.dir); !held || pause.By != "Wido" || b.run("lane").Outcome != RunPaused || b.active() != b.artifact("c1") {
		t.Fatalf("after the pause paused %v by %s, active %s", held, pause.By, b.active())
	}
	if got := outcomes(b.lines()); got != "deploy:c1:active deploy:c2:active rollback:c1:active" {
		t.Fatalf("record = %s", got)
	}
}

// A pause that ends a run's activate after it switched to c2: the run asks
// version all the same and appends its one line, active for c2, so status
// names c2.
func TestPauseAfterTheSwitchRecordsTheActivation(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.deployed("c1")
	before := len(b.lines())
	hold := b.fifo("activate")
	t.Cleanup(func() { b.releaseIfHeld(hold) })
	b.script("activate", step{HoldAfter: hold})
	b.main("c2")
	_, done := startRun(t, b, "lane")
	activating := b.inFlight("activate", "c2")
	t.Cleanup(func() { _ = syscall.Kill(-activating.PID, syscall.SIGKILL) })
	// The pause ends the activate, or the run does when the pause came as
	// the activate started.
	if pause, err := b.runner("Ann", nil).Pause("the activation hangs"); err != nil || pause.Stopped != nil && pause.Stopped.PID != activating.PID {
		t.Fatalf("pause = %+v %v; want the activate stopped", pause, err)
	}
	report := <-done
	if report.Outcome != RunDeployed || len(report.Lines) != 1 || !strings.Contains(report.Lines[0].Detail, "a pause ended the adapter's activate after the switch") {
		t.Fatalf("the run = %+v; want c2 active as version reports it", report)
	}
	requireOneLineNaming(t, b, before, "c2")
}

// A pause that ends a run's rollback after a failed verification of c2,
// once the rollback switched back to c1: the run asks version all the
// same and appends its one line, a return to c1 away from c2, so status
// names c1 current and c2 its previous.
func TestPauseAfterTheRollbackRecordsTheReturn(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.deployed("c1")
	before := len(b.lines())
	hold := b.fifo("rollback")
	t.Cleanup(func() { b.releaseIfHeld(hold) })
	b.script("version", step{}, step{Artifact: "elsewhere"}, step{})
	b.script("rollback", step{HoldAfter: hold})
	b.main("c2")
	_, done := startRun(t, b, "lane")
	rolling := b.inFlight("rollback", "c1")
	t.Cleanup(func() { _ = syscall.Kill(-rolling.PID, syscall.SIGKILL) })
	if _, err := b.runner("Ann", nil).Pause("the rollback hangs"); err != nil {
		t.Fatal(err)
	}
	report := <-done
	if lines := report.Lines; report.Outcome != RunStopped || len(lines) != 1 || lines[0].Kind != KindRollback || lines[0].Outcome != OutcomeActive || lines[0].Previous != "c2" ||
		!strings.Contains(lines[0].Detail, "a pause ended the adapter's rollback after the switch; version then reports version v-c1") {
		t.Fatalf("the run = %+v; want it stopped and its line the return to c1 from c2 as version reports it", report)
	}
	requireOneLineNaming(t, b, before, "c1")
	if status, err := ReadStatus(b.dir, 0); err != nil || status.Current.Previous != "c2" || status.Previous != nil {
		t.Fatalf("status = %+v (%v); want c1 current with c2, never active, its previous", status, err)
	}
}

// A pause that ends the build of c2, while the record holds no line and
// version reports c1, makes nothing active: the run's line is stopped,
// naming c1's artifact as version reports it, and status names no c2.
func TestPauseEndingABuildRecordsNoSwitch(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.deployed("c1")
	if err := os.Remove(recordPath(b.dir)); err != nil {
		t.Fatal(err)
	}
	b.script("build", step{HoldBefore: b.fifo("build")})
	b.main("c2")
	stalled(t, b, "build c2", func(_ *bed, r *Runner) { _, _ = r.Run() })
	status, err := ReadStatus(b.dir, 0)
	if lines := b.lines(); err != nil || len(lines) != 1 || lines[0].Outcome != OutcomeStopped || lines[0].Artifact != "" ||
		!strings.Contains(lines[0].Detail, "version then reports version v-c1 ("+b.artifact("c1")+")") || status.Current != nil {
		t.Fatalf("record %+v, status names %+v (%v); want one stopped line naming c1's artifact and nothing current", lines, status.Current, err)
	}
}

// requireOneLineNaming requires one line appended to the record's first
// before lines, and status naming commit, at its artifact, as current.
func requireOneLineNaming(t *testing.T, b *bed, before int, commit string) {
	t.Helper()
	status, err := ReadStatus(b.dir, 0)
	if lines := b.lines(); err != nil || len(lines) != before+1 || status.Current == nil || status.Current.Commit != commit || status.Current.Artifact != b.artifact(commit) {
		t.Fatalf("status names %+v (%v) after %s; want one line appended and %s current", status.Current, err, outcomes(lines[before:]), commit)
	}
}

// A run that records an interrupted activation from pending.json removes
// that file only once the recovered active line is in the record: when the
// line can't be written, pending.json stays, and the next run records the
// activation from it.
func TestRecoveredActiveLineIsWrittenBeforePendingIsRemoved(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("permission bits cannot bite as root")
	}
	b := newBed(t)
	b.deployed("c1")
	b.deployed("c2")
	b.script("activate", step{HoldAfter: b.fifo("held")})
	child := b.child("run", "c3")
	b.crash(child, b.inFlight("activate", "c3"), true)
	b.script("activate", step{})
	pending := b.file(pendingPath(b.dir))
	if err := os.Chmod(recordPath(b.dir), 0o400); err != nil {
		t.Fatal(err)
	}
	b.main("c3")
	if _, err := b.runner("Wido", nil).Run(); err == nil || pending == "" || b.file(pendingPath(b.dir)) != pending {
		t.Fatalf("a run that can't write the recovered line = %v; want it failed with pending.json in place", err)
	}
	if err := os.Chmod(recordPath(b.dir), 0o600); err != nil {
		t.Fatal(err)
	}
	report := b.run("Wido")
	if len(report.Recorded) != 1 || report.Recorded[0].Commit != "c3" || report.Recorded[0].Previous != "c2" || b.file(pendingPath(b.dir)) != "" || b.count("build c3") != 1 {
		t.Fatalf("the next run = %+v; want c3 recorded from pending.json, which then goes", report)
	}
}

// Every adapter call reads deploy.json from, and runs in, the clean tree of
// the commit it is about: version that of the current deploy (main's tip
// while nothing is current), build and activate that of the tip, rollback
// that of the deploy it returns to. The invoking checkout's deploy.json,
// behind main or dirty, only says that a deploy is declared.
func TestEachCallRunsTheAdapterOfItsCommit(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.adapters = map[string]string{"c1": "adapter-v1", "c2": "adapter-v2", "c3": "adapter-v3"}
	checkout := filepath.Join(b.base, "checkout")
	declare := func(argv ...string) {
		t.Helper()
		contract, err := json.Marshal(Contract{Schema: 1, Adapter: Adapter{Argv: argv}})
		if err == nil {
			err = fakeTree(argv)(checkout, "dirty")
		}
		if err == nil {
			err = os.WriteFile(filepath.Join(checkout, "deploy.json"), contract, 0o644)
		}
		if installation, declared := Declared(checkout, checkout); err != nil || declared != nil || installation != "." {
			t.Fatalf("the checkout declares %q %v %v", installation, declared, err)
		}
	}
	declare("./adapter-v1", b.state)
	b.deployed("c1")
	b.deployed("c2")
	declare("./adapter-dirty", filepath.Join(b.base, "dirty"))
	b.main("c3")
	if report, err := b.runner("Wido", nil).Rollback(); err != nil || report.Outcome != RollbackDone || b.active() != b.artifact("c1") {
		t.Fatalf("rollback = %+v %v", report, err)
	}
	want := "adapter-v1 version c1 in c1,adapter-v1 build c1 in c1,adapter-v1 activate c1 in c1,adapter-v1 version c1 in c1," +
		"adapter-v1 version c1 in c1,adapter-v2 build c2 in c2,adapter-v2 activate c2 in c2,adapter-v2 version c2 in c2," +
		"adapter-v2 version c2 in c2,adapter-v1 rollback c1 in c1,adapter-v1 version c1 in c1"
	if got := strings.ReplaceAll(strings.TrimSpace(b.file(filepath.Join(b.state, "via"))), "\n", ","); got != want {
		t.Fatalf("adapter calls = %s; want %s", got, want)
	}
}

// A landing whose adapter fails every call leaves the deploys before it to
// their own adapters: the current deploy's adapter answers version, so only
// the new tip's build fails, and a rollback returns to the deploy before
// the current one by that deploy's adapter.
func TestABrokenLandingsAdapterLeavesTheRollbackToTheAdapterBefore(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.adapters = map[string]string{"c1": "adapter-v1", "c2": "adapter-v2", "c3": "broken-v3"}
	b.deployed("c1")
	b.deployed("c2")
	b.main("c3")
	if report := b.run("Wido"); report.Outcome != RunFailed || len(report.Lines) != 1 || report.Lines[0].Commit != "c3" ||
		report.Lines[0].Outcome != OutcomeBuildFailed || !strings.Contains(report.Lines[0].Detail, "this adapter fails every call") {
		t.Fatalf("deploy now of c3 = %+v; want c3's build failed by c3's adapter", report)
	}
	if report, err := b.runner("Wido", nil).Rollback(); err != nil || report.Outcome != RollbackDone || report.Target.Commit != "c1" || b.active() != b.artifact("c1") {
		t.Fatalf("rollback = %+v %v; want c1 active again", report, err)
	}
	calls := strings.Split(strings.TrimSpace(b.file(filepath.Join(b.state, "via"))), "\n")
	want := "adapter-v2 version c2 in c2,broken-v3 build c3 in c3,adapter-v2 version c2 in c2,adapter-v1 rollback c1 in c1,adapter-v1 version c1 in c1"
	if got := strings.Join(calls[min(8, len(calls)):], ","); got != want {
		t.Fatalf("adapter calls after the deploys of c1 and c2 = %s; want %s", got, want)
	}
	if got := outcomes(b.lines()); got != "deploy:c1:active deploy:c2:active deploy:c3:build-failed rollback:c1:active" {
		t.Fatalf("record = %s", got)
	}
}

// version answering none after a deploy is the truth: a none line makes
// nothing current, a repeated rollback rolls back again instead of holding,
// and a run deploys main's tip as a first deploy.
func TestVersionAnsweringNoneMeansNothingIsActive(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.deployed("c1")
	b.deployed("c2")
	if _, err := b.runner("Wido", nil).Rollback(); err != nil {
		t.Fatal(err)
	}
	gone := func() {
		t.Helper()
		if err := os.Remove(filepath.Join(b.state, "active.json")); err != nil {
			t.Fatal(err)
		}
	}
	gone()
	again, err := b.runner("Wido", nil).Rollback()
	if err != nil || again.Outcome != RollbackDone || again.Target.Commit != "c1" || b.active() != b.artifact("c1") ||
		len(again.Recorded) != 1 || again.Recorded[0].Outcome != OutcomeNone {
		t.Fatalf("the repeated rollback = %+v %v; want nothing active recorded, then c1 activated again", again, err)
	}
	gone()
	if _, _, err := b.runner("Wido", nil).Resume(); err != nil {
		t.Fatal(err)
	}
	if report := b.run("lane"); report.Outcome != RunDeployed || report.Lines[0].Commit != "c2" || report.Lines[0].Previous != "" {
		t.Fatalf("the run after none = %+v; want c2 deployed as a first deploy", report)
	}
	if got := outcomes(b.lines()); got != "deploy:c1:active deploy:c2:active rollback:c1:active rollback:c1:none rollback:c1:active rollback:c1:none deploy:c2:active" {
		t.Fatalf("record = %s", got)
	}
}
