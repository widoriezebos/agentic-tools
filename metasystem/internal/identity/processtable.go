package identity

import (
	"slices"

	"golang.org/x/sys/unix"
)

// ProcessTable is the process table a host-wide scan reads: every pid, and
// each pid's process group, session and parent. Production scans read the
// kernel's; a test hands a scan a table holding exactly the processes it
// started, so the scan never sees what else runs on the host (another
// test's process, or a pid reused into a group or session the test
// recorded). A scan takes its table per call or per struct, never from a
// package variable.
type ProcessTable interface {
	Pids() ([]int64, error)
	// Group and Session answer unix.ESRCH for a process that is not in the
	// table (gone, or never there).
	Group(pid int64) (int64, error)
	Session(pid int64) (int64, error)
	// Parent is ParentPid's answer for a process in the table.
	Parent(pid int64) (int64, bool)
}

// KernelProcessTable is the host's live process table.
type KernelProcessTable struct{}

func (KernelProcessTable) Pids() ([]int64, error) { return AllPids() }

func (KernelProcessTable) Group(pid int64) (int64, error) {
	group, err := unix.Getpgid(int(pid))
	return int64(group), err
}

func (KernelProcessTable) Session(pid int64) (int64, error) {
	session, err := unix.Getsid(int(pid))
	return int64(session), err
}

func (KernelProcessTable) Parent(pid int64) (int64, bool) { return ParentPid(pid) }

// ListedProcessTable is the kernel's table narrowed to the listed pids: a
// listed process's group, session and parent are read live, and no other
// process exists. A test bed lists the processes it started.
type ListedProcessTable []int64

func (table ListedProcessTable) Pids() ([]int64, error) {
	return append([]int64(nil), table...), nil
}

func (table ListedProcessTable) Group(pid int64) (int64, error) {
	if !slices.Contains(table, pid) {
		return 0, unix.ESRCH
	}
	return KernelProcessTable{}.Group(pid)
}

func (table ListedProcessTable) Session(pid int64) (int64, error) {
	if !slices.Contains(table, pid) {
		return 0, unix.ESRCH
	}
	return KernelProcessTable{}.Session(pid)
}

func (table ListedProcessTable) Parent(pid int64) (int64, bool) {
	if !slices.Contains(table, pid) {
		return 0, false
	}
	return ParentPid(pid)
}

// CensusOf takes a census of table: the kernel's one snapshot for the
// kernel table (nil is the kernel's), otherwise the table's pids and their
// parents.
func CensusOf(table ProcessTable) (ProcessCensus, error) {
	if table == nil {
		return TakeProcessCensus()
	}
	if _, kernel := table.(KernelProcessTable); kernel {
		return TakeProcessCensus()
	}
	pids, err := table.Pids()
	if err != nil {
		return ProcessCensus{}, err
	}
	census := ProcessCensus{pids: pids, parents: make(map[int64]int64, len(pids))}
	for _, pid := range pids {
		if parent, known := table.Parent(pid); known {
			census.parents[pid] = parent
		}
	}
	return census, nil
}

// ProcessRow is one process of a FixedProcessTable.
type ProcessRow struct {
	Pid, Group, Session, Parent int64
}

// FixedProcessTable is a process table written out in full: it answers from
// its rows alone and reads nothing from the kernel. A unit test that needs
// processes it cannot start writes them here.
type FixedProcessTable []ProcessRow

func (table FixedProcessTable) row(pid int64) (ProcessRow, bool) {
	for _, row := range table {
		if row.Pid == pid {
			return row, true
		}
	}
	return ProcessRow{}, false
}

func (table FixedProcessTable) Pids() ([]int64, error) {
	pids := make([]int64, 0, len(table))
	for _, row := range table {
		pids = append(pids, row.Pid)
	}
	return pids, nil
}

func (table FixedProcessTable) Group(pid int64) (int64, error) {
	row, ok := table.row(pid)
	if !ok {
		return 0, unix.ESRCH
	}
	return row.Group, nil
}

func (table FixedProcessTable) Session(pid int64) (int64, error) {
	row, ok := table.row(pid)
	if !ok {
		return 0, unix.ESRCH
	}
	return row.Session, nil
}

func (table FixedProcessTable) Parent(pid int64) (int64, bool) {
	row, ok := table.row(pid)
	return row.Parent, ok
}
