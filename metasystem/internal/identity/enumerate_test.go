package identity

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
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

func TestRootAncestorWithWithheldArgumentsHasReadableIdentityFacts(t *testing.T) {
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
	t.Skip("this process ancestry has no non-init root-owned process with arguments withheld by the operating system")
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

func TestProbeSelfIsAliveAndRecent(t *testing.T) {
	exact, state, err := (KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != Alive {
		t.Fatalf("Probe(self) = (%v, %v)", state, err)
	}
	age := time.Since(exact.StartedAt)
	if age < 0 || age > 24*time.Hour {
		t.Fatalf("self start time implausible: %v ago", age)
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
