package identity

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The cross-platform enumeration contract:
// untagged, so darwin and linux are held to one standard.

func TestAllPidsContainsSelf(t *testing.T) {
	pids, err := AllPids()
	if err != nil {
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
