package steward

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

func TestARepairStepNeverWaitsForALock(t *testing.T) {
	t.Parallel()
	for _, held := range []string{"arm", "preparation", "lease"} {
		t.Run(held, func(t *testing.T) {
			t.Parallel()
			root := canonicalPath(t.TempDir())
			engine := filepath.Join(root, "engine")
			if err := testexec.WriteFile(engine, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
				t.Fatal(err)
			}
			digest, err := installDigest(engine)
			if err != nil {
				t.Fatal(err)
			}
			installed := InstallIdentity{RepoIdentity: root, Generation: 148, InstallPath: engine, InstallDigest: digest, MintedAt: time.Unix(1800000000, 0).UTC().Format(time.RFC3339)}
			if err := MintIdentity(RepoIdentityPath(root), installed); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(runnerDir(root), "arm.flock")
			if held == "preparation" {
				path = filepath.Join(runnerDir(root), "engine-pins", ".prepare.flock")
			}
			if held == "lease" {
				pinned, err := OpenEnrolledBinary(root)
				if err != nil {
					t.Fatal(err)
				}
				err = pinned.PrepareForExecution()
				_ = pinned.Close()
				if err != nil {
					t.Fatal(err)
				}
				path = EnrolledExecutionPath(root, installed)
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			file, err := os.OpenFile(path, os.O_CREATE|os.O_RDONLY, 0o600)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = file.Close() })
			if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
				t.Fatal(err)
			}
			type result struct {
				outcome RunnerRepairOutcome
				err     error
			}
			done := make(chan result, 1)
			go func() {
				outcome, err := RepairStep(root)
				done <- result{outcome, err}
			}()
			var got result
			testenv.Await(t, "repair step to return while the "+held+" lock is held", func() bool {
				select {
				case got = <-done:
					return true
				default:
					return false
				}
			})
			if got.err != nil || got.outcome.Status != RunnerRepairBusy || got.outcome.Generation != 148 {
				t.Fatalf("held %s: %+v %v", held, got.outcome, got.err)
			}
			if _, err := os.Stat(runnerRecordPath(root)); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("busy repair wrote a runner record: %v", err)
			}
		})
	}
}

func TestTheBlockingEntryWaitsOutABusyLock(t *testing.T) {
	t.Parallel()
	bed := newExecutionBed(t)
	delete(bed.processes, bed.old.Pid)
	held, err := lock.File(filepath.Join(runnerDir(bed.root), "arm.flock"), 0o600, lock.Exclusive)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = held.Release() })
	sleeps, hooks := 0, 0
	outcome, err := repairPinnedRunnerWithDependencies(bed.root, bed.pinned, func() { hooks++ }, bed.clock, func(d time.Duration) {
		sleeps++
		if d != 50*time.Millisecond {
			t.Errorf("repair poll = %s", d)
		}
		if err := held.Release(); err != nil {
			t.Fatal(err)
		}
		bed.sleep(d)
	}, bed.deps)
	if err != nil || outcome.Status != "RESTORED" || bed.launches != 1 || sleeps != 2 || hooks != 1 {
		t.Fatalf("blocking repair: %+v %v, launches=%d sleeps=%d hooks=%d", outcome, err, bed.launches, sleeps, hooks)
	}
}

func TestTheBlockingEntryReturnsEachOutcomeFromOneCall(t *testing.T) {
	t.Parallel()
	for _, status := range []string{"CURRENT", AutoHealEnded, "EXCLUDED", "ENROLLMENT_CHANGED", "FENCED", "FENCED-prelude", "NOT_ENROLLED", "unknown", "RESTORED"} {
		t.Run(status, func(t *testing.T) {
			t.Parallel()
			bed := newExecutionBed(t)
			switch status {
			case "CURRENT":
				process := bed.old
				process.Exe = bed.replacement.Exe
				bed.processes[process.Pid] = process
			case AutoHealEnded:
				bed.runCurrentEngine(t)
				bed.publishProgress(t, func(r *RunnerProgress) { r.FailedTicks = 3 })
				bed.endAutoHealing(t)
			case "EXCLUDED":
				bed.deps.runnerExcluded = func(string, bool) (string, bool) { return "fixture", true }
			case "ENROLLMENT_CHANGED":
				installed := bed.pinned.Install
				installed.Generation++
				if err := MintIdentity(RepoIdentityPath(bed.root), installed); err != nil {
					t.Fatal(err)
				}
			case "FENCED", "FENCED-prelude":
				closeProcessFence(t, bed.root, 148)
			case "unknown":
				process := bed.old
				process.ExeKnown = false
				bed.processes[process.Pid] = process
			case "RESTORED":
				delete(bed.processes, bed.old.Pid)
			}
			sleeps := 0
			sleep := func(d time.Duration) { sleeps++; bed.sleep(d) }
			var outcome RunnerRepairOutcome
			var err error
			if status == "NOT_ENROLLED" {
				outcome, err = repairEnrolledRunnerWithClock(t.TempDir(), nil, bed.clock, sleep)
			} else if status == "FENCED-prelude" {
				outcome, err = repairEnrolledRunnerWithClock(bed.root, nil, bed.clock, sleep)
			} else {
				outcome, err = repairPinnedRunnerWithDependencies(bed.root, bed.pinned, nil, bed.clock, sleep, bed.deps)
			}
			want := strings.TrimSuffix(status, "-prelude")
			if status == "unknown" {
				if err == nil || !strings.Contains(err.Error(), "execution is unknown") {
					t.Fatalf("unknown execution: %+v %v", outcome, err)
				}
			} else if err != nil || outcome.Status != want {
				t.Fatalf("want %s: %+v %v", want, outcome, err)
			}
			wantSleeps := 0
			if want == "RESTORED" {
				wantSleeps = 1
			}
			if sleeps != wantSleeps {
				t.Fatalf("terminal outcome slept %d times", sleeps)
			}
			if want == "RESTORED" && bed.launches != 1 {
				t.Fatalf("restored after %d launches", bed.launches)
			}
		})
	}
}
