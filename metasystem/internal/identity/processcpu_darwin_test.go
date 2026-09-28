//go:build darwin

package identity

import (
	"errors"
	"os"
	"testing"
)

// TestProcessTableAndCPUReadTheKernel: the table row for this process names
// its real parent and the start the prober reads, its own CPU is readable and
// it is running rather than in an uninterruptible wait, and a pid that is not
// there reads as ErrNoSuchProcess rather than a reader failure.
func TestProcessTableAndCPUReadTheKernel(t *testing.T) {
	table, err := TakeProcessTable()
	if err != nil {
		t.Fatal(err)
	}
	self := int64(os.Getpid())
	exact, liveness, err := KernelProber{}.ReadStart(self)
	if err != nil || liveness != Alive {
		t.Fatalf("read own start: %v %v", liveness, err)
	}
	found := false
	for _, entry := range table {
		if entry.Pid != self {
			continue
		}
		found = true
		if entry.Parent != int64(os.Getppid()) || !entry.Started.Equal(exact.StartedAt) || entry.Stopped {
			t.Fatalf("own row = %+v, want parent %d started %s running", entry, os.Getppid(), exact.StartedAt)
		}
	}
	if !found {
		t.Fatalf("the table of %d rows has no row for this process", len(table))
	}
	if seconds, err := ProcessCPUSeconds(self); err != nil || seconds <= 0 {
		t.Fatalf("own CPU = %v, %v", seconds, err)
	}
	if waiting, err := ProcessUninterruptible(self); err != nil || waiting {
		t.Fatalf("a running test process read as waiting=%v err=%v", waiting, err)
	}
	absent := self + 1_000_000
	if _, err := ProcessCPUSeconds(absent); !errors.Is(err, ErrNoSuchProcess) {
		t.Fatalf("absent pid CPU error = %v, want ErrNoSuchProcess", err)
	}
	if _, err := ProcessUninterruptible(absent); !errors.Is(err, ErrNoSuchProcess) {
		t.Fatalf("absent pid thread error = %v, want ErrNoSuchProcess", err)
	}
}
