package identity

import (
	"os"
	"os/exec"
	"slices"
	"syscall"
	"testing"
)

// The kernel table answers this process's group, session and parent as the
// kernel does.
func TestKernelProcessTableReadsThisProcess(t *testing.T) {
	t.Parallel()
	var table ProcessTable = KernelProcessTable{}
	self := int64(os.Getpid())
	pids, err := table.Pids()
	if err != nil || !slices.Contains(pids, self) {
		t.Fatalf("kernel table pids hold this process %d: %v (%d pids)", self, err, len(pids))
	}
	if group, err := table.Group(self); err != nil || group != int64(syscall.Getpgrp()) {
		t.Fatalf("group of this process = %d, %v; want %d", group, err, syscall.Getpgrp())
	}
	session, err := table.Session(self)
	if want, wantErr := syscall.Getsid(0); err != nil || wantErr != nil || session != int64(want) {
		t.Fatalf("session of this process = %d, %v; want %d, %v", session, err, want, wantErr)
	}
	if parent, known := table.Parent(self); !known || parent != int64(os.Getppid()) {
		t.Fatalf("parent of this process = %d, %v; want %d", parent, known, os.Getppid())
	}
}

// A listed table holds exactly the listed processes: no other process of
// the host appears in its pids or its census, and a listed process's group,
// session and parent are the kernel's.
func TestListedProcessTableHoldsOnlyTheListedProcesses(t *testing.T) {
	t.Parallel()
	child := exec.Command("sleep", "60")
	child.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = child.Process.Kill()
		_ = child.Wait()
	})
	pid := int64(child.Process.Pid)
	table := ListedProcessTable{pid}
	if pids, err := table.Pids(); err != nil || !slices.Equal(pids, []int64{pid}) {
		t.Fatalf("listed pids = %v, %v; want [%d]", pids, err, pid)
	}
	if group, err := table.Group(pid); err != nil || group != pid {
		t.Fatalf("group of the listed child = %d, %v; want its own %d", group, err, pid)
	}
	if parent, known := table.Parent(pid); !known || parent != int64(os.Getpid()) {
		t.Fatalf("parent of the listed child = %d, %v; want this process", parent, known)
	}
	census, err := CensusOf(table)
	if err != nil {
		t.Fatal(err)
	}
	if pids := census.Pids(); !slices.Equal(pids, []int64{pid}) {
		t.Fatalf("census of the listed table = %v; want only [%d]", pids, pid)
	}
	if parent, known := census.Parent(pid); !known || parent != int64(os.Getpid()) {
		t.Fatalf("census parent of the listed child = %d, %v", parent, known)
	}
	if _, err := table.Group(int64(os.Getpid())); err == nil {
		t.Fatal("an unlisted process must read as absent from a listed table")
	}
}

// The census of the kernel table is the kernel's one snapshot.
func TestCensusOfTheKernelTableIsTheKernelCensus(t *testing.T) {
	t.Parallel()
	census, err := CensusOf(KernelProcessTable{})
	if err != nil {
		t.Fatal(err)
	}
	self := int64(os.Getpid())
	if !slices.Contains(census.Pids(), self) {
		t.Fatalf("kernel census lacks this process %d", self)
	}
	if parent, known := census.Parent(self); !known || parent != int64(os.Getppid()) {
		t.Fatalf("kernel census parent = %d, %v", parent, known)
	}
}
