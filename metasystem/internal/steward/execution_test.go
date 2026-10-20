package steward

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginepin/testdata/pinfixture"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

type executionBed struct {
	root             string
	pinned           *EnrolledBinary
	processes        pinfixture.Prober
	deps             rearmResolverDeps
	now              time.Time
	launches, stops  int
	old, replacement identity.Exact
}

func newExecutionBed(t *testing.T) *executionBed {
	t.Helper()
	b := &executionBed{root: canonicalPath(t.TempDir()), now: time.Unix(1800000000, 0)}
	oldPath, _ := pinfixture.Pin(t, b.root, 147, "old-source")
	newPath, digest := pinfixture.Pin(t, b.root, 148, "new-source")
	b.old, b.replacement = pinfixture.Process(41, oldPath), pinfixture.Process(42, newPath)
	b.processes = pinfixture.Prober{41: b.old}
	installed := InstallIdentity{RepoIdentity: b.root, Generation: 148, InstallPath: newPath, InstallDigest: digest, MintedAt: b.now.UTC().Format(time.RFC3339), Enrollment: EnrollmentFixture}
	if err := MintIdentity(RepoIdentityPath(b.root), installed); err != nil {
		t.Fatal(err)
	}
	var err error
	b.pinned, err = OpenEnrolledBinary(b.root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = b.pinned.Close() })
	b.record(t, b.old)
	if _, err := beginComponentAttempt(b.root, "steward-tick", 148, identity.Ref{Pid: b.old.Pid, StartedAtSec: b.old.StartedAt.Unix()}, b.now); err != nil {
		t.Fatal(err)
	}
	b.deps = rearmResolverDeps{runnerExcluded: func(string, bool) (string, bool) { return "", false }, notifyAvailable: func(string) bool { return true }}
	b.deps.prober = b.processes
	b.deps.signal = func(pid int, signal syscall.Signal) error {
		if signal == syscall.SIGTERM {
			b.stops++
			delete(b.processes, int64(pid))
		}
		return nil
	}
	b.deps.launch = func(root string, pinned *EnrolledBinary, lineage string) (RunnerRecord, error) {
		if _, alive := liveRunnerWithProber(root, b.processes); alive {
			t.Fatal("replacement started before the obsolete process stopped")
		}
		if pinned.Install != installed {
			t.Fatal("launch changed enrollment")
		}
		b.launches++
		b.processes[b.replacement.Pid] = b.replacement
		if _, err := beginComponentAttempt(b.root, "steward-tick", 148, identity.Ref{Pid: b.replacement.Pid, StartedAtSec: b.replacement.StartedAt.Unix()}, b.now); err != nil {
			t.Fatal(err)
		}
		return b.record(t, b.replacement), nil
	}
	return b
}

func (b *executionBed) record(t *testing.T, exact identity.Exact) RunnerRecord {
	t.Helper()
	record := RunnerRecord{Pid: exact.Pid, PidStartedAt: exact.StartedAt.Unix(), StartedAt: b.now.UTC().Format(time.RFC3339)}
	if err := writeJSONAtomic(runnerRecordPath(b.root), record); err != nil {
		t.Fatal(err)
	}
	return record
}

func (b *executionBed) clock() time.Time      { return b.now }
func (b *executionBed) sleep(d time.Duration) { b.now = b.now.Add(d) }

func TestEnsureRunnerReplacesObsoleteExecutionAndRepeatsUnchanged(t *testing.T) {
	t.Parallel()
	b := newExecutionBed(t)
	for i, action := range []string{"replaced", "verified"} {
		got, err := ensureRunnerWithDependencies(b.root, b.pinned, b.deps, b.clock, b.sleep)
		if err != nil || got.Action != action || got.Pid != b.replacement.Pid || got.Generation != 148 {
			t.Fatalf("ensure %d: %+v %v", i, got, err)
		}
	}
	if b.launches != 1 || b.stops != 1 {
		t.Fatalf("launches=%d stops=%d", b.launches, b.stops)
	}
	observed := ReadRunnerExecution(b.root, b.processes)
	if observed.Digest != b.pinned.Install.InstallDigest || observed.Generation != 148 || observed.Source != "new-source" {
		t.Fatalf("replacement observation: %+v", observed)
	}
}

func TestRepairRunnerReplacesObsoleteExecutionAndRepeatsCurrent(t *testing.T) {
	t.Parallel()
	b := newExecutionBed(t)
	for i, status := range []string{"RESTORED", "CURRENT"} {
		got, err := repairPinnedRunnerWithDependencies(b.root, b.pinned, nil, b.clock, b.sleep, b.deps)
		if err != nil || got.Status != status || got.ReplacementPid != b.replacement.Pid || got.Generation != 148 {
			t.Fatalf("repair %d: %+v %v", i, got, err)
		}
	}
	if b.launches != 1 || b.stops != 1 {
		t.Fatalf("launches=%d stops=%d", b.launches, b.stops)
	}
}

func TestRunnerReplacementRequiresObservedStartup(t *testing.T) {
	t.Parallel()
	for _, path := range []string{"ensure", "repair"} {
		for _, fault := range []string{"stale", "uninspectable", "corrupt"} {
			t.Run(path+"/"+fault, func(t *testing.T) {
				t.Parallel()
				b := newExecutionBed(t)
				switch fault {
				case "stale":
					b.replacement.Exe = b.old.Exe
				case "uninspectable":
					b.replacement.ExeKnown = false
				case "corrupt":
					if err := os.Chmod(b.replacement.Exe, 0o600); err != nil {
						t.Fatal(err)
					}
					if err := testexec.WriteFile(b.replacement.Exe, []byte("corrupt"), 0o500); err != nil {
						t.Fatal(err)
					}
					if err := testexec.Locked(func() error { return os.Chmod(b.replacement.Exe, 0o500) }); err != nil {
						t.Fatal(err)
					}
				}
				var err error
				if path == "ensure" {
					_, err = ensureRunnerWithDependencies(b.root, b.pinned, b.deps, b.clock, b.sleep)
				} else {
					_, err = repairPinnedRunnerWithDependencies(b.root, b.pinned, nil, b.clock, b.sleep, b.deps)
				}
				if err == nil || !strings.Contains(err.Error(), "replacement runner execution not verified") || b.launches != 1 {
					t.Fatalf("%s/%s must refuse unverified startup: launches=%d err=%v", path, fault, b.launches, err)
				}
			})
		}
	}
}

func TestRunnerUnknownExecutionNeverAcceptedOrReplaced(t *testing.T) {
	t.Parallel()
	b := newExecutionBed(t)
	exact := b.processes[41]
	exact.ExeKnown = false
	b.processes[41] = exact
	if _, err := ensureRunnerWithDependencies(b.root, b.pinned, b.deps, b.clock, b.sleep); err == nil || !strings.Contains(err.Error(), "execution is unknown") {
		t.Fatalf("ensure unknown: %v", err)
	}
	if _, err := repairPinnedRunnerWithDependencies(b.root, b.pinned, nil, b.clock, b.sleep, b.deps); err == nil || !strings.Contains(err.Error(), "execution is unknown") {
		t.Fatalf("repair unknown: %v", err)
	}
	if b.launches != 0 || b.stops != 0 {
		t.Fatal("unknown execution authorized replacement")
	}
	if err := os.Remove(filepath.Join(runnerDir(b.root), "runner.json")); err != nil {
		t.Fatal(err)
	}
	if got := ReadRunnerExecution(b.root, b.processes); got.Digest != "" || got.Cause == "" {
		t.Fatalf("missing record: %+v", got)
	}
}

func TestRunnerExecutionReplacementWithNativePins(t *testing.T) {
	t.Parallel()
	engines := canonicalPath(t.TempDir())
	oldEngine, newEngine := filepath.Join(engines, "old"), filepath.Join(engines, "new")
	buildFakeRunner(t, oldEngine, "old-source")
	buildFakeRunner(t, newEngine, "new-source")
	for _, path := range []string{"ensure", "repair"} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			root := canonicalPath(t.TempDir())
			reapStewardRunnerFixture(t, root)
			now := time.Unix(1800000000, 0)
			clock := func() time.Time { return now }
			// Kernel progress, rather than elapsed wall time, ends these waits.
			sleep := func(time.Duration) { runtime.Gosched() }
			open := func(engine string, generation int) *EnrolledBinary {
				t.Helper()
				digest, err := installDigest(engine)
				if err != nil {
					t.Fatal(err)
				}
				if err := MintIdentity(RepoIdentityPath(root), InstallIdentity{RepoIdentity: root, Generation: generation, InstallPath: engine, InstallDigest: digest, Enrollment: EnrollmentFixture, MintedAt: now.UTC().Format(time.RFC3339)}); err != nil {
					t.Fatal(err)
				}
				pinned, err := OpenEnrolledBinary(root)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = pinned.Close() })
				if err := pinned.PrepareForExecution(); err != nil {
					t.Fatal(err)
				}
				return pinned
			}
			old := open(oldEngine, 147)
			before, err := launchRunner(root, old, "", sleep)
			if err != nil {
				t.Fatal(err)
			}
			current := open(newEngine, 148)
			for i := 0; i < 2; i++ {
				if path == "ensure" {
					deps := rearmResolverDeps{runnerExcluded: func(string, bool) (string, bool) { return "", false }, notifyAvailable: func(string) bool { return true }, prober: identity.KernelProber{}}
					got, err := ensureRunnerWithDependencies(root, current, deps, clock, sleep)
					want := []string{"replaced", "verified"}[i]
					if err != nil || got.Action != want {
						t.Fatalf("ensure %d: %+v %v", i, got, err)
					}
				} else {
					got, err := repairEnrolledRunnerWithClock(root, nil, clock, sleep)
					want := []string{"RESTORED", "CURRENT"}[i]
					if err != nil || got.Status != want {
						t.Fatalf("repair %d: %+v %v", i, got, err)
					}
				}
				if i == 0 {
					record, alive := liveRunner(root)
					if !alive {
						t.Fatal("replacement lost its process before the first tick")
					}
					ref := identity.Ref{Pid: record.Pid, StartedAtSec: record.PidStartedAt, StartTicks: record.StartTicks, BootID: record.BootID}
					if _, err := beginComponentAttempt(root, "steward-tick", 148, ref, now); err != nil {
						t.Fatal(err)
					}
				}
			}
			after, alive := liveRunner(root)
			observed := ReadRunnerExecution(root, identity.KernelProber{})
			if !alive || after.Pid == before.Pid || observed.Digest != current.Install.InstallDigest || observed.Source != "new-source" || observed.Generation != 148 {
				t.Fatalf("native replacement: before=%+v after=%+v alive=%t execution=%+v", before, after, alive, observed)
			}
		})
	}
}

func TestWatcherRepairLaunchesNothingWhereTheRunnerIsExcluded(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"absent", "live", "not-excluded"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b := newExecutionBed(t)
			b.deps.runnerExcluded = func(string, bool) (string, bool) { return "fixture runtime", name != "not-excluded" }
			path := runnerRecordPath(b.root)
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if name != "live" {
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			}
			signals := 0
			signal := b.deps.signal
			b.deps.signal = func(pid int, sig syscall.Signal) error {
				signals++
				return signal(pid, sig)
			}
			got, err := repairPinnedRunnerWithDependencies(b.root, b.pinned, nil, b.clock, b.sleep, b.deps)
			if name == "not-excluded" {
				if err != nil || got.Status != "RESTORED" || got.Generation != 148 || b.launches != 1 || signals != 0 {
					t.Fatalf("ordinary repair: %+v err=%v launches=%d signals=%d", got, err, b.launches, signals)
				}
				return
			}
			if err != nil || got.Status != "EXCLUDED" || got.Generation != 148 || b.launches != 0 || signals != 0 {
				t.Fatalf("excluded repair: %+v err=%v launches=%d signals=%d", got, err, b.launches, signals)
			}
			after, err := os.ReadFile(path)
			if name == "live" {
				if err != nil || string(after) != string(before) {
					t.Fatalf("excluded live record changed: %s %v", after, err)
				}
			} else if !os.IsNotExist(err) {
				t.Fatalf("excluded repair wrote a runner record: %s %v", after, err)
			}
		})
	}
}

func TestRunnerExecutionDistinguishesMissingAndUnreadableRecords(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"missing", "malformed", "unreadable", "stale"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			path := runnerRecordPath(root)
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			switch name {
			case "malformed":
				if err := os.WriteFile(path, []byte("{"), 0600); err != nil {
					t.Fatal(err)
				}
			case "unreadable":
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			case "stale":
				if err := writeJSONAtomic(path, RunnerRecord{Pid: 41, PidStartedAt: 1800000000}); err != nil {
					t.Fatal(err)
				}
			}
			got := ReadRunnerExecution(root, pinfixture.Prober{})
			if got.Absent != (name == "missing") || got.Digest != "" || got.Cause == "" {
				t.Fatalf("%s record: %+v", name, got)
			}
		})
	}
}
