package steward

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopfence"
)

func newRepairsBed(t *testing.T) *replacementBed {
	t.Helper()
	b := newReplacementBed(t)
	b.runCurrentEngine(t)
	b.deps.clock = &manualRearmClock{now: b.now}
	b.deps.signal = func(pid int, sig syscall.Signal) error {
		if pid != int(b.old.Pid) {
			t.Errorf("signal to unexpected pid %d", pid)
		}
		b.signals = append(b.signals, sig)
		return nil
	}
	b.deps.launch = func(root string, _ *EnrolledBinary, _ string) (RunnerRecord, error) {
		b.launches++
		exact := b.replacement
		exact.Pid += int64(b.launches)
		exact.StartedAt = b.now
		b.old = exact
		b.processes[exact.Pid] = exact
		b.publishProgress(t, func(*RunnerProgress) {})
		return b.record(t, exact), nil
	}
	return b
}

func repairStepIs(t *testing.T, b *replacementBed, status string) {
	t.Helper()
	if got, err := b.step(t); err != nil || got.Status != status {
		t.Fatalf("want %s: %+v %v", status, got, err)
	}
}

func repairCountIs(t *testing.T, b *replacementBed, n int, since time.Time) {
	t.Helper()
	got, err := readRunnerRepairs(b.root)
	if err != nil || got.SinceSuccess != n || !got.Since.Equal(since) {
		t.Fatalf("count = %+v %v; want %d since %s", got, err, n, since)
	}
}

func TestStartsAreCountedUntilTheLimit(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"dead", "failing", "mixed", "never announces"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			b := newRepairsBed(t)
			since := b.now
			for n := 1; n <= 3; n++ {
				if mode == "failing" || mode == "mixed" && n == 2 {
					b.publishProgress(t, func(p *RunnerProgress) { p.FailedTicks = 3 })
					repairStepIs(t, b, RunnerRepairStopping)
				}
				delete(b.processes, b.old.Pid)
				repairStepIs(t, b, RunnerRepairLaunching)
				repairCountIs(t, b, n, since)
				if n == 3 && mode == "never announces" {
					if err := os.Remove(runnerRecordPath(b.root)); err != nil {
						t.Fatal(err)
					}
					for range 4 {
						b.now = b.now.Add(time.Hour)
						repairStepIs(t, b, RunnerRepairLaunching)
						repairCountIs(t, b, 3, since)
					}
					return
				}
				repairStepIs(t, b, "RESTORED")
				b.now = b.now.Add(time.Minute)
			}
			b.publishProgress(t, func(p *RunnerProgress) { p.FailedTicks = 3 })
			signals := len(b.signals)
			repairStepIs(t, b, AutoHealEnded)
			if b.launches != 3 || len(b.signals) != signals {
				t.Fatalf("exhausted healing: launches=%d signals=%v", b.launches, b.signals)
			}
		})
	}
}

func TestASuccessInALaterRunnerEndsTheCount(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"same second", "earlier runner", "no success", "missing", "unreadable"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			b := newRepairsBed(t)
			b.now = b.now.Add(500 * time.Millisecond)
			b.endAutoHealing(t)
			p := RunnerProgress{Pid: b.old.Pid, PidStartedAt: b.now.Unix(), OKTicks: 1}
			switch mode {
			case "earlier runner":
				p.PidStartedAt--
				p.OKTicks = 5
			case "no success":
				p.PidStartedAt++
				p.OKTicks = 0
			}
			if mode != "missing" {
				exact := b.processes[b.old.Pid]
				exact.StartedAt = time.Unix(p.PidStartedAt, 0)
				b.processes[exact.Pid] = exact
				b.record(t, exact)
				if err := writeJSONAtomic(runnerProgressPath(b.root), p); err != nil {
					t.Fatal(err)
				}
				if mode == "unreadable" {
					if err := os.WriteFile(runnerProgressPath(b.root), []byte("{"), 0o600); err != nil {
						t.Fatal(err)
					}
				}
			}
			want := AutoHealEnded
			if mode == "same second" {
				want = "CURRENT"
			}
			repairStepIs(t, b, want)
			_, err := os.Stat(runnerRepairsPath(b.root))
			if errors.Is(err, os.ErrNotExist) != (mode == "same second") {
				t.Fatalf("count removal: %v", err)
			}
		})
	}
}

func TestTheSuccessIsReadBeforeASuccessorStarts(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"dead", "obsolete", "draining"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			b := newRepairsBed(t)
			exact := b.processes[b.old.Pid]
			exact.StartedAt, b.old.StartedAt = b.now, b.now
			b.processes[exact.Pid] = exact
			b.record(t, exact)
			if err := chargeRunnerRepair(b.root, b.now); err != nil {
				t.Fatal(err)
			}
			if mode == "draining" {
				b.publishProgress(t, func(p *RunnerProgress) { p.FailedTicks = 3 })
				repairStepIs(t, b, RunnerRepairStopping)
			}
			b.publishProgress(t, func(p *RunnerProgress) { p.PidStartedAt, p.OKTicks = b.now.Unix(), 1 })
			b.now = b.now.Add(time.Minute)
			if mode == "obsolete" {
				exact := b.processes[b.old.Pid]
				exact.Exe = b.old.Exe
				b.processes[exact.Pid] = exact
				repairStepIs(t, b, RunnerRepairStopping)
				if _, err := os.Stat(runnerRepairsPath(b.root)); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("obsolete replacement did not first end the count: %v", err)
				}
				return
			}
			delete(b.processes, b.old.Pid)
			repairStepIs(t, b, RunnerRepairLaunching)
			repairCountIs(t, b, 1, b.now)
		})
	}
}

func TestEveryStartIsCharged(t *testing.T) {
	t.Parallel()
	t.Run("failed start and crash between writes", func(t *testing.T) {
		t.Parallel()
		b := newRepairsBed(t)
		delete(b.processes, b.old.Pid)
		launch := b.deps.launch
		b.deps.launch = func(string, *EnrolledBinary, string) (RunnerRecord, error) {
			repairCountIs(t, b, 1, b.now)
			return RunnerRecord{}, errors.New("start failed")
		}
		if _, err := b.step(t); err == nil || !strings.Contains(err.Error(), "start failed") {
			t.Fatalf("failed start: %v", err)
		}
		noReplacement(t, b.root)
		repairCountIs(t, b, 1, b.now)
		b.deps.launch = launch
		repairStepIs(t, b, RunnerRepairLaunching)
		repairCountIs(t, b, 2, b.now)
		pending, err := readRunnerReplacement(b.root)
		if err != nil || pending == nil {
			t.Fatalf("pending: %+v %v", pending, err)
		}
		pending.Launched.Pid = 0
		delete(b.processes, b.old.Pid)
		if err := writeJSONAtomic(runnerReplacementPath(b.root), pending); err != nil {
			t.Fatal(err)
		}
		repairStepIs(t, b, RunnerRepairLaunching)
		repairCountIs(t, b, 3, b.now)
	})
	t.Run("obsolete engine", func(t *testing.T) {
		t.Parallel()
		b := newReplacementBed(t)
		repairStepIs(t, b, RunnerRepairStopping)
		delete(b.processes, b.old.Pid)
		repairStepIs(t, b, RunnerRepairLaunching)
		if _, err := os.Stat(runnerRepairsPath(b.root)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("obsolete engine charged: %v", err)
		}
	})
	t.Run("arm", func(t *testing.T) {
		t.Parallel()
		b := newRepairsBed(t)
		delete(b.processes, b.old.Pid)
		got, err := armWithRearmDeps(b.root, b.pinned.Install.InstallPath, false, false, true, "", humanMintDecision("human-terminal", "", "", EnrollmentHumanTerminal), b.deps)
		if err != nil || got.RunnerPid == 0 || b.launches != 1 {
			t.Fatalf("arm: %+v %v launches=%d", got, err, b.launches)
		}
		if _, err := os.Stat(runnerRepairsPath(b.root)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("person's arm charged: %v", err)
		}
	})
	t.Run("unwritable charge", func(t *testing.T) {
		t.Parallel()
		b := newRepairsBed(t)
		delete(b.processes, b.old.Pid)
		if err := os.Mkdir(runnerRepairsPath(b.root), 0o700); err != nil {
			t.Fatal(err)
		}
		_, err := launchReplacement(b.root, b.pinned, runnerReplacement{Cause: "dead", Generation: 148, Phase: "launching", PhaseSince: b.now}, b.now, b.deps)
		if err == nil || b.launches != 0 {
			t.Fatalf("unwritable count authorized start: %v launches=%d", err, b.launches)
		}
	})
}

func TestTheRepairLimitIsASetting(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"1", "0", "bad", "default", "unreadable"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			b := newRepairsBed(t)
			delete(b.processes, b.old.Pid)
			if mode != "default" && mode != "unreadable" {
				if err := os.WriteFile(filepath.Join(b.root, "metasystem.conf"), []byte("steward.runner-repair-limit="+mode+"\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "unreadable" {
				if err := os.WriteFile(runnerRepairsPath(b.root), []byte("{"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			got, err := b.step(t)
			if mode == "0" || mode == "bad" || mode == "unreadable" {
				if err == nil || !strings.HasPrefix(err.Error(), "read steward repair count: ") || b.launches != 0 {
					t.Fatalf("invalid state: %+v %v launches=%d", got, err, b.launches)
				}
				return
			}
			if err != nil || got.Status != RunnerRepairLaunching {
				t.Fatalf("start: %+v %v", got, err)
			}
			repairStepIs(t, b, "RESTORED")
			delete(b.processes, b.old.Pid)
			if mode == "1" {
				repairStepIs(t, b, AutoHealEnded)
			} else if limit, err := RunnerRepairLimit(b.root); err != nil || limit != 3 {
				t.Fatalf("compiled default: %d %v", limit, err)
			}
		})
	}
}

func TestAnExhaustedCountEndsTheBlockingEntryAtOnce(t *testing.T) {
	t.Parallel()
	b := newRepairsBed(t)
	b.endAutoHealing(t)
	delete(b.processes, b.old.Pid)
	got, err := repairPinnedRunnerWithDependencies(b.root, b.pinned, nil, b.clock, func(time.Duration) { t.Fatal("exhausted repair slept") }, b.deps)
	if err != nil || got.Status != AutoHealEnded || b.launches != 0 {
		t.Fatalf("exhausted repair: %+v %v launches=%d", got, err, b.launches)
	}
}

func TestTheHealthCounterNoLongerEndsRunnerHealing(t *testing.T) {
	t.Parallel()
	b := newRepairsBed(t)
	if err := saveHealthRecord(b.root, HealthRecordPath(b.root), healthRecord{
		State:   HealthObservationState{Sequence: 5, ObservedAt: b.now, UnknownCounts: map[HealthRole]int{}, FailureCounts: map[HealthRole]int{RoleStewardRunner: healthFailureLimit}},
		Verdict: HealthVerdict{Schema: 1, ObservedAt: b.now, Observation: 5, Aggregate: "unhealthy"},
	}); err != nil {
		t.Fatal(err)
	}
	if record, err := loadHealthRecord(HealthRecordPath(b.root)); err != nil || record.State.FailureCounts[RoleStewardRunner] != healthFailureLimit {
		t.Fatalf("fixture must have an exhausted health counter: %+v %v", record, err)
	}
	delete(b.processes, b.old.Pid)
	repairStepIs(t, b, RunnerRepairLaunching)
	repairCountIs(t, b, 1, b.now)
}

func TestClearingTheRepairRecords(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"clear", "busy", "unreadable"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			b := newRepairsBed(t)
			b.endAutoHealing(t)
			pending := runnerReplacement{Cause: "dead", Generation: 148, Phase: "launching", PhaseSince: b.now}
			if err := writeJSONAtomic(runnerReplacementPath(b.root), pending); err != nil {
				t.Fatal(err)
			}
			if mode == "busy" {
				held, err := lock.File(filepath.Join(runnerDir(b.root), "arm.flock"), 0o600, lock.Exclusive)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = held.Release() })
			}
			if mode == "unreadable" {
				if err := os.Remove(runnerRepairsPath(b.root)); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(runnerRepairsPath(b.root), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			err := ClearRunnerRepairs(b.root)
			if mode != "clear" {
				if err == nil || mode == "busy" && !errors.Is(err, ErrRunnerRepairBusy) {
					t.Fatalf("clear %s: %v", mode, err)
				}
				for _, path := range []string{runnerReplacementPath(b.root), runnerRepairsPath(b.root)} {
					if _, err := os.Stat(path); err != nil {
						t.Fatalf("failed clear removed %s: %v", path, err)
					}
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			noReplacement(t, b.root)
			repairCountIs(t, b, 0, time.Time{})
			delete(b.processes, b.old.Pid)
			repairStepIs(t, b, RunnerRepairLaunching)
			repairCountIs(t, b, 1, b.now)
		})
	}
}

func TestARunnerStartedBeforeARestartEndsItself(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	now := time.Unix(1800000000, 0)
	deps := runnerLoopDependencies{Now: func() time.Time { return now }, Self: censusSeamProbe(func(pid int64) (identity.Exact, identity.Liveness, error) {
		return identity.Exact{Pid: pid, StartedAt: now}, identity.Alive, nil
	}), Tick: func(string, TickConfig, WorkerCensus) (TickResult, error) {
		t.Fatal("restarted runner reached its first tick")
		return TickResult{}, nil
	}, AfterRecordPublished: func() {
		closeProcessFence(t, root, 1)
		if err := stopfence.Write(root, stopfence.Record{State: stopfence.StateOpen, Phase: stopfence.PhaseArmed, Generation: 2, ChangedAt: now.UTC().Format(time.RFC3339), Checkout: root}); err != nil {
			t.Fatal(err)
		}
	}}
	err := runLoopWithDependencies(root, fakeCensus{}, nil, time.Minute, TickConfig{Now: now}, deps)
	if err == nil || !strings.Contains(err.Error(), "was restarted while the steward started") {
		t.Fatalf("runner survived restart: %v", err)
	}
	if _, err := os.Stat(runnerRecordPath(root)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("restarted runner announcement remains: %v", err)
	}
}
