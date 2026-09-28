//go:build darwin

package identity

import (
	"encoding/binary"

	"fmt"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

// Darwin's proc_info call and the flavors the process-tree readers use
// (sys/proc_info.h, libproc).
const (
	sysProcInfo            = 336 // SYS_proc_info
	procInfoCallPidInfo    = 2   // PROC_INFO_CALL_PIDINFO
	procInfoCallPidRusage  = 9   // PROC_INFO_CALL_PIDRUSAGE
	procPidListThreads     = 6   // PROC_PIDLISTTHREADS
	procPidThreadInfo      = 5   // PROC_PIDTHREADINFO
	rusageInfoV2           = 2   // RUSAGE_INFO_V2
	rusageInfoV2Size       = 160 // sizeof(struct rusage_info_v2)
	rusageUserTimeOffset   = 16  // ri_user_time, after the 16-byte ri_uuid
	rusageSystemTimeOffset = 24  // ri_system_time
	threadInfoSize         = 112 // sizeof(struct proc_threadinfo)
	threadRunStateOffset   = 24  // pth_run_state
	threadStateRunning     = 1   // TH_STATE_RUNNING
	threadStateUninterrupt = 4   // TH_STATE_UNINTERRUPTIBLE
	processStoppedStatus   = 4   // SSTOP in Darwin's sys/proc.h
)

var (
	timebaseOnce      sync.Once
	timebaseFrequency uint64
	timebaseErr       error
)

// machTicksPerSecond is the Mach absolute-time frequency the kernel charges
// CPU time in (hw.tbfrequency: 24 MHz on Apple silicon, 1 GHz on Intel,
// where a tick is a nanosecond).
func machTicksPerSecond() (uint64, error) {
	timebaseOnce.Do(func() {
		timebaseFrequency, timebaseErr = unix.SysctlUint64("hw.tbfrequency")
		if timebaseErr == nil && timebaseFrequency == 0 {
			timebaseErr = fmt.Errorf("identity: hw.tbfrequency is zero")
		}
	})
	return timebaseFrequency, timebaseErr
}

func procInfo(call, pid, flavor int, arg uint64, buffer []byte) (int, unix.Errno) {
	var pointer unsafe.Pointer
	if len(buffer) > 0 {
		pointer = bytesPointer(buffer)
	}
	r1, _, errno := unix.Syscall6(sysProcInfo, uintptr(call), uintptr(pid), uintptr(flavor), uintptr(arg), uintptr(pointer), uintptr(len(buffer)))
	return int(r1), errno
}

// ProcessCPUSeconds returns the user plus system CPU time the kernel has
// charged to pid's own threads (proc_pid_rusage), the figure `ps -o cputime`
// shows. Reaped children are not included, as `ps -S` did not include them.
// A process that is gone reports ErrNoSuchProcess.
func ProcessCPUSeconds(pid int64) (float64, error) {
	if pid < 1 {
		return 0, fmt.Errorf("identity: invalid pid %d", pid)
	}
	ticks, err := machTicksPerSecond()
	if err != nil {
		return 0, fmt.Errorf("identity: read the Mach timebase: %w", err)
	}
	buffer := make([]byte, rusageInfoV2Size)
	// proc_pid_rusage passes no buffer size: the flavor fixes it.
	_, _, errno := unix.Syscall6(sysProcInfo, procInfoCallPidRusage, uintptr(pid), rusageInfoV2, 0, uintptr(bytesPointer(buffer)), 0)
	if errno != 0 {
		if errno == unix.ESRCH {
			return 0, fmt.Errorf("identity: CPU of pid %d: %w", pid, ErrNoSuchProcess)
		}
		return 0, fmt.Errorf("identity: CPU of pid %d: %w", pid, errno)
	}
	charged := binary.LittleEndian.Uint64(buffer[rusageUserTimeOffset:]) + binary.LittleEndian.Uint64(buffer[rusageSystemTimeOffset:])
	return float64(charged) / float64(ticks), nil
}

// ProcessEntry is one row of a process-table snapshot: the facts a
// process-tree reader decides on, read from kern.proc.all.
type ProcessEntry struct {
	Pid, Parent int64
	// Started is the kernel's start time, the member's identity across
	// samples (a reused pid has a different start).
	Started time.Time
	// Stopped is the SSTOP status `ps` shows as T.
	Stopped bool
}

// TakeProcessTable reads every process's parent, start time and stop status
// from one kern.proc.all snapshot.
func TakeProcessTable() ([]ProcessEntry, error) {
	raw, err := readProcessTable()
	if err != nil {
		return nil, err
	}
	return decodeProcessTable(raw)
}

func decodeProcessTable(raw []byte) ([]ProcessEntry, error) {
	const (
		pPidOffset      = int(unsafe.Offsetof(unix.ExternProc{}.P_pid))
		parentPidOffset = int(unsafe.Offsetof(unix.KinfoProc{}.Eproc) + unsafe.Offsetof(unix.Eproc{}.Ppid))
		statusOffset    = int(unsafe.Offsetof(unix.ExternProc{}.P_stat))
	)
	if statusOffset != kinfoStatOffset || pPidOffset != 40 {
		return nil, fmt.Errorf("identity: kern.proc.all status offset %d and pid offset %d disagree with the Darwin ABI (ABI drift?)", statusOffset, pPidOffset)
	}
	if len(raw)%unix.SizeofKinfoProc != 0 {
		return nil, fmt.Errorf("identity: kern.proc.all returned %d bytes, not a multiple of %d (ABI drift?)", len(raw), unix.SizeofKinfoProc)
	}
	entries := make([]ProcessEntry, 0, len(raw)/unix.SizeofKinfoProc)
	for offset := 0; offset < len(raw); offset += unix.SizeofKinfoProc {
		record := raw[offset : offset+unix.SizeofKinfoProc]
		pid := int64(int32(binary.LittleEndian.Uint32(record[pPidOffset:])))
		if pid <= 0 {
			continue
		}
		sec := int64(binary.LittleEndian.Uint64(record[kinfoStartSecOffset:]))
		usec := int64(int32(binary.LittleEndian.Uint32(record[kinfoStartUsecOffset:])))
		entries = append(entries, ProcessEntry{
			Pid:     pid,
			Parent:  int64(int32(binary.LittleEndian.Uint32(record[parentPidOffset:]))),
			Started: time.Unix(sec, usec*1000),
			Stopped: record[statusOffset] == processStoppedStatus,
		})
	}
	return entries, nil
}

// ProcessUninterruptible reports whether pid is in an uninterruptible wait
// the way `ps` derives its U state: no thread is running and at least one
// thread is in an uninterruptible wait. A process that is gone reports
// ErrNoSuchProcess.
func ProcessUninterruptible(pid int64) (bool, error) {
	if pid < 1 {
		return false, fmt.Errorf("identity: invalid pid %d", pid)
	}
	handles := make([]byte, 8*256)
	for {
		written, errno := procInfo(procInfoCallPidInfo, int(pid), procPidListThreads, 0, handles)
		if errno == unix.ESRCH {
			return false, fmt.Errorf("identity: threads of pid %d: %w", pid, ErrNoSuchProcess)
		}
		if errno != 0 {
			return false, fmt.Errorf("identity: threads of pid %d: %w", pid, errno)
		}
		if written < len(handles) {
			handles = handles[:written]
			break
		}
		handles = make([]byte, 2*len(handles))
	}
	uninterruptible := false
	info := make([]byte, threadInfoSize)
	for offset := 0; offset+8 <= len(handles); offset += 8 {
		handle := binary.LittleEndian.Uint64(handles[offset:])
		written, errno := procInfo(procInfoCallPidInfo, int(pid), procPidThreadInfo, handle, info)
		if errno != 0 || written < threadRunStateOffset+4 {
			// A thread that ended between the listing and the read has no
			// state to contribute.
			continue
		}
		switch int32(binary.LittleEndian.Uint32(info[threadRunStateOffset:])) {
		case threadStateRunning:
			return false, nil
		case threadStateUninterrupt:
			uninterruptible = true
		}
	}
	return uninterruptible, nil
}
