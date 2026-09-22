package identity

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// The cross-platform enumeration contract:
// untagged, so darwin and linux are held to one standard.

func TestAllPidsContainsSelf(t *testing.T) {
	pids, err := AllPids()
	if err != nil {
		if errors.Is(err, syscall.EPERM) {
			t.Skipf("process enumeration is restricted: %v", err)
		}
		t.Fatal(err)
	}
	self := int64(os.Getpid())
	for _, pid := range pids {
		if pid == self {
			return
		}
	}
	t.Fatalf("AllPids did not contain self (%d) among %d pids", self, len(pids))
}

func TestParentPidMatchesGetppid(t *testing.T) {
	ppid, ok := ParentPid(int64(os.Getpid()))
	if !ok || ppid != int64(os.Getppid()) {
		t.Fatalf("ParentPid(self) = (%d, %v), want (%d, true)", ppid, ok, os.Getppid())
	}
}

func TestParentPidReportsProcessTreeRootAsKnownNone(t *testing.T) {
	parent, ok := ParentPid(1)
	if !ok || parent != 0 {
		t.Fatalf("ParentPid(process-tree root) = (%d, %v), want (0, true)", parent, ok)
	}
}

func TestProcessOwnerAnswersForLiveAndDeadProcesses(t *testing.T) {
	owner, ok := ProcessOwner(int64(os.Getpid()))
	if !ok || owner != uint32(os.Geteuid()) {
		t.Fatalf("ProcessOwner(self) = (%d, %v), want (%d, true)", owner, ok, os.Geteuid())
	}
	owner, ok = ProcessOwner(1)
	if !ok || owner != 0 {
		t.Fatalf("ProcessOwner(pid 1) = (%d, %v), want (0, true)", owner, ok)
	}
	if _, ok := ProcessOwner(1 << 30); ok {
		t.Fatal("ProcessOwner claimed an owner for a nonexistent pid")
	}
}

func TestProcessesWithWithheldArgumentsHaveReadableIdentityFacts(t *testing.T) {
	pids, err := AllPids()
	if err != nil {
		t.Fatal(err)
	}
	subjects := make([]int64, 0)
	liveProcessCount := 0
	for _, pid := range pids {
		exact, state, err := (KernelProber{}).Probe(pid)
		if err != nil || state != Alive {
			continue
		}
		liveProcessCount++
		if pid != 1 && !exact.ArgvKnown {
			subjects = append(subjects, pid)
		}
	}
	if len(subjects) == 0 {
		if os.Geteuid() != 0 {
			t.Fatalf("no live non-init process with withheld arguments for euid %d among %d live processes", os.Geteuid(), liveProcessCount)
		}
		exact, state, err := (KernelProber{}).Probe(1)
		if err != nil || state != Alive || !exact.ArgvKnown {
			t.Fatalf("root Probe(1) = (state %v, argv known %v, err %v), want alive with arguments readable", state, exact.ArgvKnown, err)
		}
		return
	}
	assertedWhileAlive := 0
	departed := 0
	for _, pid := range subjects {
		owner, ownerKnown := ProcessOwner(pid)
		parent, parentKnown := ParentPid(pid)
		executable, executableKnown := ExecutablePath(pid)
		if !ownerKnown || !parentKnown || !executableKnown {
			_, state, err := (KernelProber{}).Probe(pid)
			// A departed process is not a defect. The guard never excuses a
			// missing required fact for a process that remains alive.
			if err != nil || state != Alive {
				departed++
				continue
			}
		}
		valid := true
		if !ownerKnown {
			t.Errorf("ProcessOwner(%d) = (%d, false), want an answer for a live process with withheld arguments", pid, owner)
			valid = false
		}
		if !parentKnown || parent == pid {
			t.Errorf("ParentPid(%d) = (%d, %v), want an answer other than the process itself", pid, parent, parentKnown)
			valid = false
		}
		// Linux kernel threads have no executable path, so an explicit unknown
		// answer is valid when its accompanying path is empty.
		if executableKnown {
			if executable == "" || !filepath.IsAbs(executable) {
				t.Errorf("ExecutablePath(%d) = (%q, true), want a non-empty absolute path", pid, executable)
				valid = false
			}
		} else if executable != "" {
			t.Errorf("ExecutablePath(%d) = (%q, false), want an empty path when unknown", pid, executable)
			valid = false
		}
		if valid {
			assertedWhileAlive++
		}
	}
	if len(subjects) > 0 && assertedWhileAlive == 0 {
		t.Fatalf("none of %d withheld-argument subjects remained alive through all identity assertions (%d departed)", len(subjects), departed)
	}

	current := int64(os.Getpid())
	seen := map[int64]bool{}
	for current > 0 && !seen[current] {
		seen[current] = true
		exact, state, err := (KernelProber{}).Probe(current)
		owner, ownerKnown := ProcessOwner(current)
		if current != 1 && err == nil && state == Alive && ownerKnown && owner == 0 && !exact.ArgvKnown {
			if parent, ok := ParentPid(current); !ok || parent < 1 || parent == current {
				t.Fatalf("ParentPid(%d) did not answer for a live root-owned process with withheld arguments", current)
			}
			if executable, ok := ExecutablePath(current); !ok || executable == "" {
				t.Fatalf("ExecutablePath(%d) did not answer for a live root-owned process with withheld arguments", current)
			}
			if confirmedOwner, ok := ProcessOwner(current); !ok || confirmedOwner != 0 {
				t.Fatalf("ProcessOwner(%d) = (%d, %v), want (0, true)", current, confirmedOwner, ok)
			}
			return
		}
		parent, ok := ParentPid(current)
		if !ok {
			break
		}
		current = parent
	}
}

func TestProcessCwdMatchesWorkingDirectory(t *testing.T) {
	cwd, ok := ProcessCwd(int64(os.Getpid()))
	if !ok {
		t.Fatal("ProcessCwd(self) not readable")
	}
	expected, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// Both sides resolve symlinks (macOS /tmp vs /private/tmp).
	cwdReal, _ := filepath.EvalSymlinks(cwd)
	expectedReal, _ := filepath.EvalSymlinks(expected)
	if cwdReal != expectedReal {
		t.Fatalf("ProcessCwd(self) = %q, want %q", cwdReal, expectedReal)
	}
}

func TestProbeSelfHasStableNativeIdentity(t *testing.T) {
	prober := KernelProber{}
	self := int64(os.Getpid())
	exact, state, err := prober.Probe(self)
	if err != nil || state != Alive {
		t.Fatalf("Probe(self) = (%v, %v)", state, err)
	}
	start, startState, startErr := prober.ReadStart(self)
	birth, birthOK := ProcessBirth(self)
	if startErr != nil || startState != Alive || !SameIdentity(start, exact.Ref()) ||
		!birthOK || birth.IsZero() || !start.Ref().NativeExact() || !exact.Ref().NativeExact() {
		t.Fatalf("self identity mismatch: probe=%+v start=%+v/%s/%v birth=%s/%v", exact.Ref(), start.Ref(), startState, startErr, birth, birthOK)
	}
}

// The three-way guarantee, the property most worth locking down: a pid that
// cannot exist is a definitive negative — Dead with a nil error.
func TestProbeImpossiblePidIsDead(t *testing.T) {
	_, state, err := (KernelProber{}).Probe(1 << 30)
	if state != Dead || err != nil {
		t.Fatalf("Probe(impossible pid) = (%v, %v), want (Dead, nil)", state, err)
	}
}
