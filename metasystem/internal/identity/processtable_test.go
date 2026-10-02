package identity

import (
	"errors"
	"os"
	"os/exec"
	"slices"
	"syscall"
	"testing"

	"golang.org/x/sys/unix"
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
	if want, wantErr := unix.Getsid(0); err != nil || wantErr != nil || session != int64(want) {
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

// A fixed table answers from its rows alone: a row's group, session and
// parent as written, and an absent pid as ESRCH and an unknown parent.
func TestFixedProcessTableAnswersFromItsRows(t *testing.T) {
	t.Parallel()
	table := FixedProcessTable{{Pid: 4100, Group: 4100, Session: 4000, Parent: 1}, {Pid: 4101, Group: 4100, Session: 4000, Parent: 4100}}
	if pids, err := table.Pids(); err != nil || !slices.Equal(pids, []int64{4100, 4101}) {
		t.Fatalf("pids = %v, %v", pids, err)
	}
	if group, err := table.Group(4101); err != nil || group != 4100 {
		t.Fatalf("group = %d, %v", group, err)
	}
	if session, err := table.Session(4101); err != nil || session != 4000 {
		t.Fatalf("session = %d, %v", session, err)
	}
	if parent, known := table.Parent(4101); !known || parent != 4100 {
		t.Fatalf("parent = %d, %v", parent, known)
	}
	if _, err := table.Group(4242); !errors.Is(err, unix.ESRCH) {
		t.Fatalf("an absent pid's group reads %v; want ESRCH", err)
	}
	if _, known := table.Parent(4242); known {
		t.Fatal("an absent pid's parent reads known")
	}
}

// A scripted table answers from its rows and fails where the test scripts a
// failure: the whole table, or one process's group read.
func TestScriptedProcessTableFailsWhereScripted(t *testing.T) {
	t.Parallel()
	rows := FixedProcessTable{{Pid: 7, Group: 42, Session: 3, Parent: 1}, {Pid: 8, Group: 42}}
	table := ScriptedProcessTable{Rows: rows, GroupErr: map[int64]error{8: unix.EIO}}
	if pids, err := table.Pids(); err != nil || !slices.Equal(pids, []int64{7, 8}) {
		t.Fatalf("scripted pids = %v, %v; want [7 8]", pids, err)
	}
	if group, err := table.Group(7); err != nil || group != 42 {
		t.Fatalf("group of 7 = %d, %v; want 42", group, err)
	}
	if _, err := table.Group(8); !errors.Is(err, unix.EIO) {
		t.Fatalf("group of 8 = %v; want the scripted EIO", err)
	}
	if _, err := table.Group(9); !errors.Is(err, unix.ESRCH) {
		t.Fatalf("group of an absent pid = %v; want ESRCH", err)
	}
	if session, err := table.Session(7); err != nil || session != 3 {
		t.Fatalf("session of 7 = %d, %v; want 3", session, err)
	}
	if parent, known := table.Parent(7); !known || parent != 1 {
		t.Fatalf("parent of 7 = %d, %v; want 1", parent, known)
	}
	down := ScriptedProcessTable{Rows: rows, PidsErr: unix.EPERM}
	if pids, err := down.Pids(); !errors.Is(err, unix.EPERM) || pids != nil {
		t.Fatalf("a table scripted down = %v, %v; want no pids and EPERM", pids, err)
	}
}
