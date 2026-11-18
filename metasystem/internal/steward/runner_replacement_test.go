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
		got, err = repairPinnedStep(b.root, b.pinned, b.clock, func(d time.Duration) { t.Errorf("step slept for %s", d) }, b.deps)
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
			} else if err != nil || got.Status != "RESTORED" || b.launches != 1 {
				t.Fatalf("%s must permit launch: %+v %v launches=%d", state, got, err, b.launches)
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
			} else if err != nil || got.Status != "RESTORED" || b.launches != 1 {
				t.Fatalf("reused pid prevented launch: %+v %v", got, err)
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
	if err != nil || got.Action != "replaced" || got.Pid != 42 || sleeps != 1 || b.launches != 1 || len(b.signals) != 2 {
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
