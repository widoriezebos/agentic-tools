package steward

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

const stewardSignalHelperEnv = "METASYSTEM_STEWARD_SIGNAL_HELPER"

// The fenced-member flake's cause (fencedflake C1-C3): `steward disarm` ends
// the runner with SIGTERM, whose default action killed it in the middle of a
// goal transaction; a kill after MarkPushed left the entry pushed with its
// owner dead, and every later publish on that clone was refused. The runner
// now finishes the transaction in progress on SIGTERM. The witness is a real
// runner process, a real Git origin whose pre-receive hook holds the push —
// so the entry is durably pushed and its outcome not yet known — and a real
// SIGTERM delivered at that point: the entry ends terminal and the runner
// exits cleanly once the push is let through.
func TestStewardRunFinishesItsPushedGoalTransactionOnSIGTERM(t *testing.T) {
	t.Parallel()
	bed := newLedgerAttentionBed(t)
	control := t.TempDir()
	parked, release := filepath.Join(control, "parked"), filepath.Join(control, "release")
	hook := "#!/bin/sh\n: > '" + parked + "'\ni=0\nwhile [ ! -f '" + release + "' ]; do\n  i=$((i+1))\n  [ \"$i\" -gt 1200 ] && exit 1\n  sleep 0.05\ndone\nexit 0\n"
	if err := testexec.WriteFile(filepath.Join(bed.origin, "hooks", "pre-receive"), []byte(hook), 0o755); err != nil {
		t.Fatal(err)
	}
	loopRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(loopRoot, "metasystem.conf"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.WriteFile(release, nil, 0o644) })

	command := exec.Command(os.Args[0], "-test.run=^TestStewardRunSignalHelperProcess$", "-test.count=1", "--", loopRoot, bed.publisher)
	command.Env = append(os.Environ(), stewardSignalHelperEnv+"=1")
	stderr, err := command.StderrPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stdout strings.Builder
	command.Stdout = &stdout
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	lines := make(chan string, 64)
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			lines <- scanner.Text()
		}
		close(lines)
	}()
	exited := make(chan struct{})
	var waitErr error
	receivedSignal := make(chan struct{}, 1)
	var collected []string
	go func() {
		// Wait only after the stderr pipe has been drained, as exec requires.
		for line := range lines {
			collected = append(collected, line)
			if strings.Contains(line, "received") {
				select {
				case receivedSignal <- struct{}{}:
				default:
				}
			}
		}
		waitErr = command.Wait()
		close(exited)
	}()
	pid := command.Process.Pid
	t.Cleanup(func() {
		select {
		case <-exited:
		default:
			_ = syscall.Kill(pid, syscall.SIGKILL)
			<-exited
		}
	})

	// Bounded wait for the push to reach the origin's hook: the entry is then
	// durably pushed and the outcome unknown.
	for attempt := 0; ; attempt++ {
		if _, err := os.Stat(parked); err == nil {
			break
		}
		if attempt > 3000 {
			t.Fatalf("the runner's publish never reached the origin; output=%s", stdout.String())
		}
		time.Sleep(20 * time.Millisecond)
	}
	entries, err := goal.Entries(bed.publisher)
	if err != nil || len(entries) != 1 || entries[0].Phase != goal.PhasePushed || entries[0].Owner.Pid != int64(pid) {
		t.Fatalf("the parked transaction is not the runner's pushed entry: %+v %v", entries, err)
	}
	if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	select {
	case <-receivedSignal:
	case <-exited:
		after, _ := goal.Entries(bed.publisher)
		t.Fatalf("SIGTERM ended the runner mid-transaction (%v); journal=%+v", waitErr, after)
	case <-time.After(60 * time.Second):
		t.Fatal("the runner neither reported the signal nor exited")
	}
	if err := os.WriteFile(release, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	select {
	case <-exited:
		if waitErr != nil {
			t.Fatalf("the drained runner did not exit cleanly: %v; stderr=%q stdout=%s", waitErr, collected, stdout.String())
		}
	case <-time.After(90 * time.Second):
		t.Fatal("the drained runner did not exit after its transaction")
	}
	entries, err = goal.Entries(bed.publisher)
	if err != nil || len(entries) != 1 || entries[0].Phase != goal.PhaseTerminal || entries[0].Outcome != goal.OutcomeConfirmed {
		t.Fatalf("the signalled transaction did not end terminal: %+v %v", entries, err)
	}
	if strings.Count(stdout.String(), "published ") != 1 {
		t.Fatalf("the drained runner started new work after the signal: %s", stdout.String())
	}
}

// TestStewardRunSignalHelperProcess is the runner process of the witness
// above: the production loop and its production signal handling, with a
// tick that publishes one goal per pass.
func TestStewardRunSignalHelperProcess(t *testing.T) {
	t.Parallel()
	if os.Getenv(stewardSignalHelperEnv) != "1" {
		return
	}
	loopRoot, publisher := os.Args[len(os.Args)-2], os.Args[len(os.Args)-1]
	sequence := 0
	deps := runnerLoopDependencies{
		Tick: func(string, TickConfig, WorkerCensus) (TickResult, error) {
			sequence++
			request := goal.VerbRequest{
				Endpoint: goal.Endpoint{Root: publisher, Remote: "origin", Branch: "refs/heads/main"},
				Actor:    goal.Actor{Machine: "mac-a", Lineage: "signal-fixture"},
				Ulid:     fmt.Sprintf("%026d", 900+sequence),
				Now:      time.Date(2026, 9, 30, 8, sequence, 0, 0, time.UTC), ClaimEpoch: 1,
			}
			id := fmt.Sprintf("drained-%d", sequence)
			result, err := goal.Open(request, id, "Finish "+id+" safely.", goal.OriginMain, "Work on "+id+".")
			fmt.Printf("published %s outcome=%s err=%v\n", id, result.Outcome, err)
			return TickResult{}, err
		},
		DeliverPending: func(string) (int, error) { return 0, nil },
		Resumable:      func(string) (string, bool, error) { return "", false, nil },
		Channel:        func(context.Context, string) (int, error) { return 0, nil },
		Now:            time.Now, Sleep: time.Sleep,
		StopSignals: productionStopSignals(),
	}
	if err := runLoopWithDependencies(loopRoot, fakeCensus{}, nil, 50*time.Millisecond, TickConfig{}, deps); err != nil {
		t.Fatal(err)
	}
}

// fakeStopSignals is the signal source with every edge injected: the test
// delivers signals, fires the bound and observes the exit.
type fakeStopSignals struct {
	signals  chan chan<- os.Signal
	bound    chan time.Time
	exits    chan int
	reported chan string
	stopped  chan struct{}
}

func newFakeStopSignals() *fakeStopSignals {
	return &fakeStopSignals{signals: make(chan chan<- os.Signal, 1), bound: make(chan time.Time, 1),
		exits: make(chan int, 2), reported: make(chan string, 8), stopped: make(chan struct{}, 1)}
}

func (f *fakeStopSignals) source(t *testing.T) stopSignalSource {
	return stopSignalSource{
		Notify: func(relay chan<- os.Signal) func() {
			f.signals <- relay
			return func() { f.stopped <- struct{}{} }
		},
		After: func(bound time.Duration) <-chan time.Time {
			if bound != StopDrainBound {
				t.Errorf("drain bound = %s, want the compiled default %s", bound, StopDrainBound)
			}
			return f.bound
		},
		Exit:   func(code int) { f.exits <- code },
		Report: func(line string) { f.reported <- line },
		Bound:  StopDrainBound,
	}
}

func receiveWithin[T any](t *testing.T, from <-chan T, what string) T {
	t.Helper()
	select {
	case value := <-from:
		return value
	case <-time.After(30 * time.Second):
		t.Fatalf("no %s", what)
	}
	var zero T
	return zero
}

// A second signal while the runner drains ends it at once, with the
// conventional signal exit code.
func TestASecondStopSignalEndsTheDrainingRunnerAtOnce(t *testing.T) {
	t.Parallel()
	fake := newFakeStopSignals()
	drain := &runnerDrain{}
	stop := fake.source(t).watch(drain)
	relay := receiveWithin(t, fake.signals, "signal relay")
	relay <- syscall.SIGTERM
	if line := receiveWithin(t, fake.reported, "drain report"); !strings.Contains(line, "terminated received") || !drain.Requested() {
		t.Fatalf("first signal: report=%q requested=%v", line, drain.Requested())
	}
	select {
	case code := <-fake.exits:
		t.Fatalf("the first signal ended the runner (%d)", code)
	default:
	}
	relay <- syscall.SIGINT
	if code := receiveWithin(t, fake.exits, "exit"); code != 130 {
		t.Fatalf("second signal exit code = %d, want 130", code)
	}
	stop()
}

// The drain is bounded: when the bound passes before the transaction ends,
// the runner exits regardless.
func TestTheDrainBoundEndsTheRunner(t *testing.T) {
	t.Parallel()
	fake := newFakeStopSignals()
	drain := &runnerDrain{}
	stop := fake.source(t).watch(drain)
	relay := receiveWithin(t, fake.signals, "signal relay")
	relay <- syscall.SIGTERM
	receiveWithin(t, fake.reported, "drain report")
	fake.bound <- time.Time{}
	if code := receiveWithin(t, fake.exits, "exit"); code != 143 {
		t.Fatalf("bound exit code = %d, want 143", code)
	}
	if line := receiveWithin(t, fake.reported, "bound report"); !strings.Contains(line, StopDrainBound.String()) {
		t.Fatalf("bound report %q does not name the bound", line)
	}
	stop()
}

// A signal during a tick lets the tick finish and then ends the loop before
// any post-tick work — no delivery, no channel, no wait, no second tick.
func TestASignalledRunnerEndsAfterTheTickInProgress(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	fake := newFakeStopSignals()
	ticks := 0
	deps := runnerLoopDependencies{
		Tick: func(string, TickConfig, WorkerCensus) (TickResult, error) {
			ticks++
			relay := receiveWithin(t, fake.signals, "signal relay")
			relay <- syscall.SIGTERM
			receiveWithin(t, fake.reported, "drain report")
			return TickResult{}, nil
		},
		DeliverPending: func(string) (int, error) { t.Error("delivery ran after the signal"); return 0, nil },
		Resumable:      func(string) (string, bool, error) { t.Error("resume ran after the signal"); return "", false, nil },
		Channel:        func(context.Context, string) (int, error) { t.Error("channel ran after the signal"); return 0, nil },
		Now:            time.Now,
		Sleep:          func(time.Duration) { t.Error("the runner waited after the signal") },
		StopSignals:    fake.source(t),
	}
	if err := runLoopWithDependencies(root, fakeCensus{}, nil, time.Hour, TickConfig{}, deps); err != nil {
		t.Fatal(err)
	}
	if ticks != 1 {
		t.Fatalf("ticks = %d, want the one in progress", ticks)
	}
	receiveWithin(t, fake.stopped, "signal relay stop")
	select {
	case code := <-fake.exits:
		t.Fatalf("an orderly drain exited through the hard path (%d)", code)
	default:
	}
}

// Inside a tick the breach-stop pass starts no further stop once the runner
// is stopping: the stop in progress finishes, the rest wait for the next
// runner, by name.
func TestAStoppingRunnerStartsNoFurtherBreachStop(t *testing.T) {
	t.Parallel()
	stopping := false
	var stopped []string
	stop := func(goalID string, _ uint64) (string, error) {
		stopped = append(stopped, goalID)
		stopping = true
		return "stopped " + goalID, nil
	}
	scanner := func(string, time.Time) ([]dispatch.StopRoute, error) {
		return []dispatch.StopRoute{{GoalID: "goal-b", Revision: 2}, {GoalID: "goal-c", Revision: 3}}, nil
	}
	reports := runBreachStopCustodianWithScanner(t.TempDir(), time.Date(2026, 9, 30, 8, 0, 0, 0, time.UTC), scanner, stop, func() bool { return stopping })
	if strings.Join(stopped, ",") != "goal-b" {
		t.Fatalf("stops run = %v, want only the one in progress", stopped)
	}
	if len(reports) != 2 || reports[0].State != "COMPLETE" || reports[1].State != "DEFERRED" || !strings.Contains(reports[1].Detail, "stopping") {
		t.Fatalf("reports = %+v", reports)
	}
}
