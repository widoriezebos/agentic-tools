package steward

import (
	"bytes"
	"errors"
	"os"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)

type replacementBed struct {
	*executionBed
	signals []syscall.Signal
}

func newReplacementBed(t *testing.T) *replacementBed {
	t.Helper()
	b := &replacementBed{executionBed: newExecutionBed(t)}
	b.deps.signal = func(pid int, sig syscall.Signal) error {
		if pid != int(b.old.Pid) {
			t.Errorf("signal to unexpected pid %d", pid)
		}
		b.signals = append(b.signals, sig)
		return nil
	}
	return b
}

func (b *replacementBed) step(t *testing.T) (RunnerRepairOutcome, error) {
	t.Helper()
	var got RunnerRepairOutcome
	var err error
	done := make(chan struct{})
	go func() {
		defer close(done)
		got, err = repairPinnedStep(b.root, b.pinned, b.clock, b.deps)
	}()
	testenv.Await(t, "replacement step to return", func() bool {
		select {
		case <-done:
			return true
		default:
			return false
		}
	})
	return got, err
}

func replacementBytes(t *testing.T, root string) []byte {
	t.Helper()
	data, err := os.ReadFile(runnerReplacementPath(root))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func noReplacement(t *testing.T, root string) {
	t.Helper()
	if _, err := os.Stat(runnerReplacementPath(root)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("replacement record remains: %v", err)
	}
}

func TestAStopIsSignalledOnceAndWaitsForTheExit(t *testing.T) {
	t.Parallel()
	b := newReplacementBed(t)
	sender := b.deps.signal
	b.deps.signal = func(pid int, sig syscall.Signal) error {
		pending, err := readRunnerReplacement(b.root)
		if err != nil || pending == nil || pending.Phase != "stopping" || pending.Cause != "obsolete-engine" || pending.Generation != 148 || pending.Runner != b.old.Ref() || !pending.PhaseSince.Equal(b.now) {
			t.Errorf("stop must be recorded before the first signal: %+v %v", pending, err)
		}
		marker, err := os.ReadFile(runnerStopPath(b.root))
		if err != nil || string(marker) != "restart\n" {
			t.Errorf("restart marker at signal: %q %v", marker, err)
		}
		return sender(pid, sig)
	}
	for _, elapsed := range []time.Duration{0, time.Second, 59 * time.Second} {
		b.now = time.Unix(1800000000, 0).Add(elapsed)
		got, err := b.step(t)
		if err != nil || got.Status != RunnerRepairStopping || got.Generation != 148 || got.PreviousPid != 41 || b.launches != 0 {
			t.Fatalf("at %s: %+v %v launches=%d", elapsed, got, err, b.launches)
		}
	}
	if err := os.Remove(runnerRecordPath(b.root)); err != nil {
		t.Fatal(err)
	}
	if got, err := b.step(t); err != nil || got.Status != RunnerRepairStopping || b.launches != 0 {
		t.Fatalf("missing announcement lost the draining identity: %+v %v", got, err)
	}
	delete(b.processes, 41)
	if got, err := b.step(t); err != nil || got.Status != RunnerRepairLaunching {
		t.Fatalf("launch: %+v %v", got, err)
	}
	got, err := b.step(t)
	if err != nil || got.Status != "RESTORED" || got.PreviousPid != 41 || got.ReplacementPid != 42 || b.launches != 1 || !reflect.DeepEqual(b.signals, []syscall.Signal{syscall.SIGTERM, syscall.SIGCONT}) {
		t.Fatalf("exit: %+v %v launches=%d signals=%v", got, err, b.launches, b.signals)
	}
	noReplacement(t, b.root)
}

func TestAStopTellsGoneFromReusedFromUnknown(t *testing.T) {
	t.Parallel()
	for _, state := range []string{"gone", "reused", "zombie", "unknown"} {
		t.Run("pending/"+state, func(t *testing.T) {
			t.Parallel()
			b := newReplacementBed(t)
			if got, err := b.step(t); err != nil || got.Status != RunnerRepairStopping {
				t.Fatalf("begin: %+v %v", got, err)
			}
			before := replacementBytes(t, b.root)
			switch state {
			case "gone":
				delete(b.processes, 41)
			case "reused", "zombie":
				exact := b.old
				if state == "reused" {
					exact.StartedAt = exact.StartedAt.Add(2 * time.Second)
				} else {
					exact.Zombie = true
				}
				b.processes[41] = exact
				if state == "zombie" {
					if err := os.Remove(runnerRecordPath(b.root)); err != nil {
						t.Fatal(err)
					}
				}
			case "unknown":
				b.deps.prober = censusSeamProbe(func(int64) (identity.Exact, identity.Liveness, error) {
					return identity.Exact{}, identity.Unknown, errors.New("unreadable")
				})
			}
			got, err := b.step(t)
			if state == "unknown" {
				if err == nil || !bytes.Equal(before, replacementBytes(t, b.root)) || b.launches != 0 {
					t.Fatalf("unknown changed transition: %+v %v launches=%d", got, err, b.launches)
				}
			} else {
				if err != nil || got.Status != RunnerRepairLaunching {
					t.Fatalf("launch: %+v %v", got, err)
				}
				got, err = b.step(t)
				if err != nil || got.Status != "RESTORED" || b.launches != 1 {
					t.Fatalf("%s must permit launch: %+v %v launches=%d", state, got, err, b.launches)
				}
			}
			if len(b.signals) != 2 {
				t.Fatalf("pending stop signalled again: %v", b.signals)
			}
		})
	}
	// Count the decision's observations without depending on the verdict's
	// internal number of probes. The last observation authorizes the stop.
	twin := newReplacementBed(t)
	probes, beforeStop := 0, 0
	twin.deps.prober = censusSeamProbe(func(pid int64) (identity.Exact, identity.Liveness, error) {
		if pid == 41 {
			probes++
		}
		return twin.processes.Probe(pid)
	})
	twin.deps.signal = func(_ int, sig syscall.Signal) error {
		if sig == syscall.SIGTERM {
			beforeStop = probes
		}
		return nil
	}
	if got, err := twin.step(t); err != nil || got.Status != RunnerRepairStopping || beforeStop == 0 {
		t.Fatalf("count stop observations: %+v %v probes=%d", got, err, beforeStop)
	}
	for _, state := range []string{"reused", "unknown"} {
		t.Run("before-stop/"+state, func(t *testing.T) {
			t.Parallel()
			b, calls := newReplacementBed(t), 0
			b.deps.prober = censusSeamProbe(func(pid int64) (identity.Exact, identity.Liveness, error) {
				if pid == 41 {
					calls++
					if calls >= beforeStop {
						if state == "unknown" {
							return identity.Exact{}, identity.Unknown, nil
						}
						exact := b.old
						exact.StartedAt = exact.StartedAt.Add(2 * time.Second)
						b.processes[41] = exact
					}
				}
				return b.processes.Probe(pid)
			})
			got, err := b.step(t)
			if state == "unknown" {
				if err == nil || b.launches != 0 {
					t.Fatalf("unknown authorized replacement: %+v %v", got, err)
				}
			} else {
				if err != nil || got.Status != RunnerRepairLaunching {
					t.Fatalf("launch: %+v %v", got, err)
				}
				got, err = b.step(t)
				if err != nil || got.Status != "RESTORED" || b.launches != 1 {
					t.Fatalf("reused pid prevented launch: %+v %v", got, err)
				}
			}
			noReplacement(t, b.root)
			if len(b.signals) != 0 {
				t.Fatalf("changed identity was signalled: %v", b.signals)
			}
		})
	}
}

func TestAnUnreadableTransitionIsAnError(t *testing.T) {
	t.Parallel()
	for _, data := range []string{"{", "null", "{}"} {
		t.Run(data, func(t *testing.T) {
			t.Parallel()
			b := newReplacementBed(t)
			if err := os.WriteFile(runnerReplacementPath(b.root), []byte(data), 0o600); err != nil {
				t.Fatal(err)
			}
			if got, err := b.step(t); err == nil || string(replacementBytes(t, b.root)) != data || len(b.signals) != 0 || b.launches != 0 {
				t.Fatalf("unreadable transition was acted on: %+v %v", got, err)
			}
		})
	}
}

func TestAPendingTransitionGoesBeforeTheBreakerAndTheVerdict(t *testing.T) {
	t.Parallel()
	for _, change := range []string{"breaker", "verdict"} {
		t.Run(change, func(t *testing.T) {
			t.Parallel()
			b := newReplacementBed(t)
			if change == "verdict" {
				exact := b.old
				exact.Exe = b.replacement.Exe
				b.processes[41] = exact
				if _, err := beginComponentAttempt(b.root, "steward-tick", 148, runnerRecordRef(b.record(t, exact)), b.now.Add(-time.Hour)); err != nil {
					t.Fatal(err)
				}
			}
			if got, err := b.step(t); err != nil || got.Status != RunnerRepairStopping {
				t.Fatalf("begin: %+v %v", got, err)
			}
			if change == "breaker" {
				if err := saveHealthRecord(b.root, HealthRecordPath(b.root), healthRecord{
					State:   HealthObservationState{Sequence: 5, ObservedAt: b.now, FailureCounts: map[HealthRole]int{RoleStewardRunner: healthFailureLimit}},
					Verdict: HealthVerdict{Schema: 1, ObservedAt: b.now, Observation: 5, Aggregate: "unhealthy"},
				}); err != nil {
					t.Fatal(err)
				}
				launch := b.deps.launch
				b.deps.launch = func(root string, pinned *EnrolledBinary, lineage string) (RunnerRecord, error) {
					pending, err := readRunnerReplacement(root)
					if err != nil || pending == nil || pending.Cause != "obsolete-engine" || pending.Phase != "launching" || !pending.PhaseSince.Equal(b.now) {
						t.Errorf("launch lost original cause: %+v %v", pending, err)
					}
					return launch(root, pinned, lineage)
				}
				delete(b.processes, 41)
				b.now = b.now.Add(time.Second)
				if got, err := b.step(t); err != nil || got.Status != RunnerRepairLaunching {
					t.Fatalf("launch: %+v %v", got, err)
				}
				if got, err := b.step(t); err != nil || got.Status != "RESTORED" {
					t.Fatalf("breaker interrupted pending launch: %+v %v", got, err)
				}
			} else {
				before := replacementBytes(t, b.root)
				if _, err := beginComponentAttempt(b.root, "steward-tick", 148, runnerRecordRef(b.record(t, b.processes[41])), b.now); err != nil {
					t.Fatal(err)
				}
				if verdict := checkStewardRunner(b.root, b.now, b.processes); verdict.Status != HealthAlive {
					t.Fatalf("fixture must have a fresh verdict: %+v", verdict)
				}
				if got, err := b.step(t); err != nil || got.Status != RunnerRepairStopping || !bytes.Equal(before, replacementBytes(t, b.root)) {
					t.Fatalf("fresh verdict interrupted stop: %+v %v", got, err)
				}
			}
		})
	}
}

func TestATransitionUnderAnotherEnrollmentIsAbandoned(t *testing.T) {
	t.Parallel()
	b := newReplacementBed(t)
	stale := runnerReplacement{Cause: "dead", Generation: 147, Runner: b.replacement.Ref(), Phase: "stopping", PhaseSince: b.now.Add(-time.Minute)}
	if err := writeJSONAtomic(runnerReplacementPath(b.root), stale); err != nil {
		t.Fatal(err)
	}
	got, err := b.step(t)
	pending, readErr := readRunnerReplacement(b.root)
	if err != nil || readErr != nil || got.Status != RunnerRepairStopping || pending == nil || pending.Generation != 148 || pending.Runner != b.old.Ref() || pending.Cause != "obsolete-engine" || len(b.signals) != 2 {
		t.Fatalf("abandonment: %+v %v pending=%+v %v signals=%v", got, err, pending, readErr, b.signals)
	}
}

func TestEnsureRunnerLeavesARunnerBeingStoppedAlone(t *testing.T) {
	t.Parallel()
	b := newReplacementBed(t)
	if got, err := b.step(t); err != nil || got.Status != RunnerRepairStopping {
		t.Fatalf("begin: %+v %v", got, err)
	}
	exact := b.old
	exact.Exe = b.replacement.Exe
	b.processes[41] = exact
	sleeps := 0
	got, err := ensureRunnerWithDependencies(b.root, b.pinned, b.deps, b.clock, func(d time.Duration) {
		sleeps++
		delete(b.processes, 41)
		b.sleep(d)
	})
	if err != nil || got.Action != "replaced" || got.Pid != 42 || sleeps != 2 || b.launches != 1 || len(b.signals) != 2 {
		t.Fatalf("ensure: %+v %v sleeps=%d launches=%d signals=%v", got, err, sleeps, b.launches, b.signals)
	}
	noReplacement(t, b.root)
}

func TestAFailedOrUnverifiedLaunchEndsTheTransition(t *testing.T) {
	t.Parallel()
	for _, fault := range []string{"start", "verification", "signal-term", "signal-cont", "resume-launch"} {
		t.Run(fault, func(t *testing.T) {
			t.Parallel()
			b := newReplacementBed(t)
			wantErr := ErrEnrollmentDrift
			switch fault {
			case "signal-term", "signal-cont":
				b.deps.signal = func(_ int, sig syscall.Signal) error {
					if sig == syscall.SIGCONT || fault == "signal-term" {
						return wantErr
					}
					return nil
				}
			default:
				delete(b.processes, 41)
				if fault == "start" {
					b.deps.launch = func(string, *EnrolledBinary, string) (RunnerRecord, error) { return RunnerRecord{}, wantErr }
				} else if fault == "verification" {
					b.replacement.Exe = b.old.Exe
				} else if err := writeJSONAtomic(runnerReplacementPath(b.root), runnerReplacement{Cause: "dead", Generation: 148, Phase: "launching", PhaseSince: b.now}); err != nil {
					t.Fatal(err)
				}
			}
			got, err := b.step(t)
			if fault == "resume-launch" || fault == "verification" {
				if err != nil || got.Status != RunnerRepairLaunching {
					t.Fatalf("launch: %+v %v", got, err)
				}
				got, err = b.step(t)
			}
			if fault == "resume-launch" {
				if err != nil || got.Status != "RESTORED" || b.launches != 1 {
					t.Fatalf("resume launch: %+v %v", got, err)
				}
			} else if fault == "verification" {
				if err == nil || !strings.Contains(err.Error(), "replacement runner execution not verified") {
					t.Fatalf("verification: %+v %v", got, err)
				}
			} else if !errors.Is(err, wantErr) {
				t.Fatalf("error lost its cause: %+v %v", got, err)
			}
			noReplacement(t, b.root)
		})
	}
}

func TestARunnerAliveAfterTheGraceIsForcedOnce(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"stopped", "never-told", "unknown"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			b := newReplacementBed(t)
			t0 := b.now
			if scenario == "never-told" {
				pending := runnerReplacement{Cause: "obsolete-engine", Generation: 148, Runner: b.old.Ref(), Phase: "stopping", PhaseSince: t0}
				if err := writeJSONAtomic(runnerReplacementPath(b.root), pending); err != nil {
					t.Fatal(err)
				}
				if _, err := os.Stat(runnerStopPath(b.root)); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("unexpected stop marker: %v", err)
				}
			} else if got, err := b.step(t); err != nil || got.Status != RunnerRepairStopping {
				t.Fatalf("begin: %+v %v", got, err)
			}
			sender := b.deps.signal
			b.deps.signal = func(pid int, sig syscall.Signal) error {
				pending, err := readRunnerReplacement(b.root)
				if sig != syscall.SIGKILL || err != nil || pending == nil || !pending.ForcedAt.Equal(t0.Add(StopDrainBound)) {
					t.Errorf("force must be saved before signal: %v %+v %v", sig, pending, err)
				}
				return sender(pid, sig)
			}
			initialSignals := len(b.signals)
			b.now = t0.Add(59 * time.Second)
			if got, err := b.step(t); err != nil || got.Status != RunnerRepairStopping || len(b.signals) != initialSignals {
				t.Fatalf("forced before grace: %+v %v signals=%v", got, err, b.signals)
			}
			if scenario == "unknown" {
				before := replacementBytes(t, b.root)
				b.deps.prober = censusSeamProbe(func(int64) (identity.Exact, identity.Liveness, error) { return identity.Exact{}, identity.Unknown, nil })
				b.now = t0.Add(2 * time.Minute)
				if got, err := b.step(t); err == nil || len(b.signals) != initialSignals || b.launches != 0 || !bytes.Equal(before, replacementBytes(t, b.root)) {
					t.Fatalf("unknown forced or changed: %+v %v signals=%v", got, err, b.signals)
				}
				return
			}
			for _, elapsed := range []time.Duration{60 * time.Second, 61 * time.Second, time.Hour} {
				b.now = t0.Add(elapsed)
				if got, err := b.step(t); err != nil || got.Status != RunnerRepairStopping || len(b.signals) != initialSignals+1 || b.launches != 0 {
					t.Fatalf("at %s: %+v %v signals=%v launches=%d", elapsed, got, err, b.signals, b.launches)
				}
			}
			delete(b.processes, 41)
			if got, err := b.step(t); err != nil || got.Status != RunnerRepairLaunching || b.launches != 1 {
				t.Fatalf("exit after force: %+v %v launches=%d", got, err, b.launches)
			}
		})
	}
}

func (b *replacementBed) startWithoutAnnouncement(t *testing.T) {
	t.Helper()
	b.deps.launch = func(root string, _ *EnrolledBinary, _ string) (RunnerRecord, error) {
		pending, err := readRunnerReplacement(root)
		if err != nil || pending == nil || pending.Phase != "launching" || pending.Launched.Pid != 0 {
			t.Errorf("launch must be recorded before start: %+v %v", pending, err)
		}
		b.launches++
		b.processes[42] = b.replacement
		return RunnerRecord{Pid: 42, PidStartedAt: b.replacement.StartedAt.Unix(), StartTicks: b.replacement.StartTicks, BootID: b.replacement.BootID}, nil
	}
}

func TestALaunchIsObservedAcrossSteps(t *testing.T) {
	t.Parallel()
	b := newReplacementBed(t)
	delete(b.processes, 41)
	if err := os.Remove(runnerRecordPath(b.root)); err != nil {
		t.Fatal(err)
	}
	b.startWithoutAnnouncement(t)
	t0 := b.now
	for _, elapsed := range []time.Duration{0, time.Minute, time.Hour, 24 * time.Hour} {
		b.now = t0.Add(elapsed)
		got, err := b.step(t)
		pending, readErr := readRunnerReplacement(b.root)
		if err != nil || got.Status != RunnerRepairLaunching || b.launches != 1 || readErr != nil || pending == nil || pending.Launched.Pid != 42 {
			t.Fatalf("at %s: %+v %v launches=%d pending=%+v %v", elapsed, got, err, b.launches, pending, readErr)
		}
	}
	b.record(t, b.replacement)
	if got, err := b.step(t); err != nil || got.Status != "RESTORED" || got.ReplacementPid != 42 {
		t.Fatalf("announcement: %+v %v", got, err)
	}
	noReplacement(t, b.root)
}

func TestALaunchThatEndsWithoutAnnouncingIsReported(t *testing.T) {
	t.Parallel()
	for _, state := range []string{"dead", "unknown"} {
		t.Run(state, func(t *testing.T) {
			t.Parallel()
			b := newReplacementBed(t)
			delete(b.processes, 41)
			b.startWithoutAnnouncement(t)
			if got, err := b.step(t); err != nil || got.Status != RunnerRepairLaunching {
				t.Fatalf("launch: %+v %v", got, err)
			}
			before := replacementBytes(t, b.root)
			if state == "unknown" {
				b.deps.prober = censusSeamProbe(func(int64) (identity.Exact, identity.Liveness, error) { return identity.Exact{}, identity.Unknown, nil })
			} else {
				delete(b.processes, 42)
			}
			got, err := b.step(t)
			if err == nil || b.launches != 1 {
				t.Fatalf("launch failure not reported: %+v %v launches=%d", got, err, b.launches)
			}
			if state == "unknown" {
				if !bytes.Equal(before, replacementBytes(t, b.root)) {
					t.Fatal("unknown process ended the transition")
				}
				return
			}
			if !strings.Contains(err.Error(), "the runner died before guarding the repository") {
				t.Fatalf("wrong exit error: %v", err)
			}
			noReplacement(t, b.root)
			if got, err := b.step(t); err != nil || got.Status != RunnerRepairLaunching || b.launches != 2 {
				t.Fatalf("next launch: %+v %v launches=%d", got, err, b.launches)
			}
		})
	}
}

func TestTheBlockingEntryWaitsForTheAnnouncement(t *testing.T) {
	t.Parallel()
	for _, entry := range []string{"repair", "ensure"} {
		t.Run(entry, func(t *testing.T) {
			t.Parallel()
			b := newReplacementBed(t)
			delete(b.processes, 41)
			b.startWithoutAnnouncement(t)
			sleeps := 0
			sleep := func(d time.Duration) {
				sleeps++
				b.sleep(d)
				if sleeps == 3 {
					b.record(t, b.replacement)
				}
			}
			if entry == "repair" {
				if got, err := repairPinnedRunnerWithDependencies(b.root, b.pinned, nil, b.clock, sleep, b.deps); err != nil || got.Status != "RESTORED" || got.ReplacementPid != 42 {
					t.Fatalf("repair: %+v %v", got, err)
				}
			} else if got, err := ensureRunnerWithDependencies(b.root, b.pinned, b.deps, b.clock, sleep); err != nil || got.Action != "started" || got.Pid != 42 {
				t.Fatalf("ensure: %+v %v", got, err)
			}
			if sleeps != 3 || b.launches != 1 {
				t.Fatalf("sleeps=%d launches=%d", sleeps, b.launches)
			}
			noReplacement(t, b.root)
		})
	}
}

func TestAZombiesLeftoverRecordIsNotTheNewRunner(t *testing.T) {
	t.Parallel()
	for _, zombie := range []string{"old", "new"} {
		t.Run(zombie, func(t *testing.T) {
			t.Parallel()
			b := newReplacementBed(t)
			b.startWithoutAnnouncement(t)
			if got, err := b.step(t); err != nil || got.Status != RunnerRepairStopping {
				t.Fatalf("stop: %+v %v", got, err)
			}
			exact := b.old
			exact.Zombie = true
			b.processes[41] = exact
			if got, err := b.step(t); err != nil || got.Status != RunnerRepairLaunching || b.launches != 1 {
				t.Fatalf("launch beside zombie: %+v %v launches=%d", got, err, b.launches)
			}
			if got, err := b.step(t); err != nil || got.Status != RunnerRepairLaunching || b.launches != 1 {
				t.Fatalf("old announcement confirmed launch: %+v %v launches=%d", got, err, b.launches)
			}
			// A live announcement at another pid cannot confirm this child either.
			other := b.old
			other.StartedAt = other.StartedAt.Add(2 * time.Second)
			b.processes[41] = other
			b.record(t, other)
			if got, err := b.step(t); err != nil || got.Status != RunnerRepairLaunching || b.launches != 1 {
				t.Fatalf("another pid confirmed launch: %+v %v launches=%d", got, err, b.launches)
			}
			b.record(t, b.replacement)
			if zombie == "new" {
				exact = b.replacement
				exact.Zombie = true
				b.processes[42] = exact
			}
			got, err := b.step(t)
			if zombie == "new" {
				if err == nil || !strings.Contains(err.Error(), "the runner died before guarding the repository") {
					t.Fatalf("zombie announcement: %+v %v", got, err)
				}
			} else if err != nil || got.Status != "RESTORED" || got.PreviousPid != 41 || got.ReplacementPid != 42 {
				t.Fatalf("new announcement: %+v %v", got, err)
			}
			noReplacement(t, b.root)
		})
	}
}

func TestAnArmWaitsForALaunchInProgress(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name    string
		replace bool
		sleeps  int
	}{
		{name: "a_announced_plain", sleeps: 1},
		{name: "b_announced_restart", replace: true, sleeps: 1},
		{name: "c_child_exits", sleeps: 1},
		{name: "d_leftover_announcement", sleeps: 2},
		{name: "e_no_record"},
		{name: "e_stopping"},
		{name: "e_no_launched_pid"},
		{name: "e_unreadable"},
		{name: "e_unknown"},
		{name: "f_arbitration", sleeps: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			b := newReplacementBed(t)
			delete(b.processes, 41)
			b.startWithoutAnnouncement(t)
			if got, err := b.step(t); err != nil || got.Status != RunnerRepairLaunching {
				t.Fatalf("launch: %+v %v", got, err)
			}
			pending, err := readRunnerReplacement(b.root)
			if err != nil || pending == nil || pending.Launched.Pid != 42 {
				t.Fatalf("pending launch: %+v %v", pending, err)
			}
			switch test.name {
			case "d_leftover_announcement":
				zombie := b.old
				zombie.Zombie = true
				b.processes[41] = zombie
			case "e_no_record":
				if err := os.Remove(runnerReplacementPath(b.root)); err != nil {
					t.Fatal(err)
				}
			case "e_stopping", "e_no_launched_pid":
				if test.name == "e_stopping" {
					pending.Phase = "stopping"
				} else {
					pending.Launched = identity.Ref{}
				}
				if err := writeJSONAtomic(runnerReplacementPath(b.root), pending); err != nil {
					t.Fatal(err)
				}
			case "e_unreadable":
				if err := os.WriteFile(runnerReplacementPath(b.root), []byte("{"), 0o600); err != nil {
					t.Fatal(err)
				}
			case "e_unknown":
				b.deps.prober = censusSeamProbe(func(pid int64) (identity.Exact, identity.Liveness, error) {
					if pid == 42 {
						return identity.Exact{}, identity.Unknown, nil
					}
					return b.processes.Probe(pid)
				})
			}
			before, beforeErr := os.ReadFile(runnerReplacementPath(b.root))
			deps := b.deps
			sleeps, launches, stops := 0, 0, 0
			stopSleep := make(chan struct{})
			deps.sleep = func(d time.Duration) {
				sleeps++
				if test.sleeps == 0 {
					// Release an erroneous wait only during cleanup, after Await fails.
					<-stopSleep
					_ = os.Remove(runnerReplacementPath(b.root))
					return
				}
				if d != repairPoll {
					t.Errorf("poll = %s; want %s", d, repairPoll)
				}
				if test.name == "f_arbitration" {
					arbitration, err := TryAcquireArbitration(b.root)
					if err != nil {
						t.Errorf("arm holds arbitration while waiting: %v", err)
					} else {
						arbitration.Release()
					}
				}
				b.sleep(d)
				if test.name == "c_child_exits" {
					delete(b.processes, 42)
				} else if sleeps == test.sleeps {
					b.record(t, b.replacement)
				}
			}
			deps.signal = func(pid int, sig syscall.Signal) error {
				installed, err := VerifyIdentity(RepoIdentityPath(b.root), b.root)
				if pid != 42 || err != nil || installed.Generation != 148 {
					t.Errorf("stop must precede mint: pid=%d installed=%+v err=%v", pid, installed, err)
				}
				if sig == syscall.SIGTERM {
					stops++
					delete(b.processes, 42)
					return os.Remove(runnerRecordPath(b.root))
				}
				return nil
			}
			deps.launch = func(_ string, pinned *EnrolledBinary, lineage string) (RunnerRecord, error) {
				launches++
				if pinned.Install.Generation != 149 || lineage != "arm-lineage" || (test.replace && stops != 1) {
					t.Errorf("arm launch: generation=%d lineage=%q stops=%d", pinned.Install.Generation, lineage, stops)
				}
				exact := b.replacement
				exact.Pid = 43
				b.processes[43] = exact
				if _, err := beginComponentAttempt(b.root, "steward-tick", 149, exact.Ref(), b.now); err != nil {
					t.Fatal(err)
				}
				return b.record(t, exact), nil
			}
			var got armOutcome
			var armErr error
			done := make(chan struct{})
			go func() {
				defer close(done)
				got, armErr = armWithRearmDeps(b.root, b.pinned.Install.InstallPath, test.replace, false, true, "arm-lineage",
					humanMintDecision("human-terminal", "", "", EnrollmentFixture), deps)
			}()
			t.Cleanup(func() { close(stopSleep); <-done })
			testenv.Await(t, "arm to return", func() bool {
				select {
				case <-done:
					return true
				default:
					return false
				}
			})
			wantPid, wantGeneration, wantLaunches := int64(43), 149, 1
			if test.sleeps > 0 && !test.replace && test.name != "c_child_exits" {
				wantPid, wantGeneration, wantLaunches = 42, 148, 0
			}
			installed, err := VerifyIdentity(RepoIdentityPath(b.root), b.root)
			if armErr != nil || got.RunnerPid != wantPid || err != nil || installed.Generation != wantGeneration || launches != wantLaunches || sleeps != test.sleeps {
				t.Fatalf("arm: %+v %v installed=%+v %v launches=%d sleeps=%d", got, armErr, installed, err, launches, sleeps)
			}
			if wantPid == 42 && !strings.HasPrefix(got.Message, "already armed (runner pid 42)") {
				t.Fatalf("plain arm: %q", got.Message)
			}
			if wantPid == 43 && (!strings.Contains(got.Message, "armed (runner pid 43)") || strings.Contains(got.Message, "runner pid 42")) {
				t.Fatalf("minted arm: %q", got.Message)
			}
			after, afterErr := os.ReadFile(runnerReplacementPath(b.root))
			if !bytes.Equal(before, after) || errors.Is(beforeErr, os.ErrNotExist) != errors.Is(afterErr, os.ErrNotExist) {
				t.Fatalf("arm changed the replacement record: before=%q %v after=%q %v", before, beforeErr, after, afterErr)
			}
			if wantGeneration == 149 {
				current, err := OpenEnrolledBinary(b.root)
				if err != nil {
					t.Fatal(err)
				}
				_ = b.pinned.Close()
				b.pinned = current
			}
			if test.sleeps > 0 {
				got, err := b.step(t)
				if err != nil || (wantGeneration == 148 && got.Status != "RESTORED") {
					t.Fatalf("repair after arm: %+v %v", got, err)
				}
				noReplacement(t, b.root)
			} else if test.name == "e_unreadable" {
				if _, err := b.step(t); err == nil {
					t.Fatal("repair accepted the unreadable record")
				}
			}
		})
	}
}
