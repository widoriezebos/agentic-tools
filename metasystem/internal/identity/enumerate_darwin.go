//go:build darwin

package identity

import (
	"encoding/binary"
	"errors"
	"fmt"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

// TakeProcessCensus reads every process id and parent link from one Darwin
// process-table snapshot.
func TakeProcessCensus() (ProcessCensus, error) {
	var raw []byte
	var err error
	// The sysctl sizes its buffer in one call and fills it in another; a
	// process table that grows between the two answers ENOMEM, which on a
	// box running a parallel battery is a moment, not a state (2026-09-11:
	// TestProcLaunchRealChild inside the pooled cadence). A few retries
	// separate that moment from a real enumeration failure.
	for attempt := 0; attempt < 5; attempt++ {
		raw, err = sysctlRaw("kern.proc.all")
		if err == nil || !errors.Is(err, unix.ENOMEM) {
			break
		}
		time.Sleep(time.Duration(10*(attempt+1)) * time.Millisecond)
	}
	if err != nil {
		return ProcessCensus{}, fmt.Errorf("identity: sysctl kern.proc.all: %w", err)
	}
	return decodeProcessCensus(raw)
}

// AllPids returns every process id on the machine via sysctl kern.proc.all —
// the native replacement for shelling out to `ps`.
func AllPids() ([]int64, error) {
	census, err := TakeProcessCensus()
	if err != nil {
		return nil, err
	}
	return census.Pids(), nil
}

// sysctlRaw is the kernel query; a test replaces it to shape its answers.
var sysctlRaw = unix.SysctlRaw

func decodeAllPids(raw []byte) ([]int64, error) {
	census, err := decodeProcessCensus(raw)
	if err != nil {
		return nil, err
	}
	return census.Pids(), nil
}

func decodeProcessCensus(raw []byte) (ProcessCensus, error) {
	if len(raw) == 0 {
		return ProcessCensus{}, nil
	}
	const (
		pPidOffset      = int(unsafe.Offsetof(unix.ExternProc{}.P_pid))
		parentPidOffset = int(unsafe.Offsetof(unix.KinfoProc{}.Eproc) + unsafe.Offsetof(unix.Eproc{}.Ppid))
		kinfoProcSize   = unix.SizeofKinfoProc
	)
	if pPidOffset != 40 {
		return ProcessCensus{}, fmt.Errorf("identity: kern.proc.all pid offset is %d, want Darwin ABI offset 40 (ABI drift?)", pPidOffset)
	}
	if len(raw)%kinfoProcSize != 0 {
		return ProcessCensus{}, fmt.Errorf("identity: kern.proc.all returned %d bytes, not a multiple of %d (ABI drift?)", len(raw), kinfoProcSize)
	}
	census := ProcessCensus{parents: make(map[int64]int64, len(raw)/kinfoProcSize)}
	for offset := 0; offset+pPidOffset+4 <= len(raw); offset += kinfoProcSize {
		pid := int32(binary.LittleEndian.Uint32(raw[offset+pPidOffset:]))
		if pid > 0 {
			processID := int64(pid)
			parentID := int64(int32(binary.LittleEndian.Uint32(raw[offset+parentPidOffset:])))
			census.pids = append(census.pids, processID)
			census.parents[processID] = parentID
		}
	}
	return census, nil
}

// ProcessCwd returns a process's current working directory via the
// proc_info syscall (PROC_PIDVNODEPATHINFO) — the same call `lsof` makes,
// done natively. ok is false when the cwd cannot be read (a permission
// denial or a gone process), which the census treats as an lsof-denial.
//
// proc_vnodepathinfo is 2352 bytes: pvi_cdir (a vnode_info_path) then
// pvi_rdir. The cwd path is the null-terminated vip_path inside pvi_cdir, at
// offset 152 (sizeof vnode_info), up to MAXPATHLEN.
func ProcessCwd(pid int64) (string, bool) {
	const (
		sysProcInfo          = 336 // SYS_proc_info
		procInfoCallPidInfo  = 2   // PROC_INFO_CALL_PIDINFO
		procPidVnodePathInfo = 9   // PROC_PIDVNODEPATHINFO
		vnodePathInfoSize    = 2352
		cwdPathOffset        = 152
	)
	buffer := make([]byte, vnodePathInfoSize)
	r1, _, errno := unix.Syscall6(
		sysProcInfo,
		uintptr(procInfoCallPidInfo),
		uintptr(pid),
		uintptr(procPidVnodePathInfo),
		0,
		uintptr(bytesPointer(buffer)),
		uintptr(len(buffer)),
	)
	if errno != 0 || int(r1) < cwdPathOffset {
		return "", false
	}
	path := buffer[cwdPathOffset:]
	end := 0
	for end < len(path) && path[end] != 0 {
		end++
	}
	if end == 0 {
		return "", false
	}
	return string(path[:end]), true
}

// ParentPid returns a process's parent pid from its kern.proc.pid entry. The
// returned bool is false only when the process is gone or the kernel does not
// disclose its parent. A reported parent of zero or the process itself is the
// known top of the process tree and is normalized to parent zero with ok true.
func ParentPid(pid int64) (int64, bool) {
	if pid < 1 {
		return 0, false
	}
	kinfo, err := unix.SysctlKinfoProc("kern.proc.pid", int(pid))
	if err != nil || int64(kinfo.Proc.P_pid) != pid {
		return 0, false
	}
	ppid := int64(kinfo.Eproc.Ppid)
	if ppid == 0 || ppid == pid {
		return 0, true
	}
	if ppid < 0 {
		return 0, false
	}
	return ppid, true
}

// ProcessOwner returns the effective user id recorded for a live process.
func ProcessOwner(pid int64) (uint32, bool) {
	if pid < 1 {
		return 0, false
	}
	kinfo, err := unix.SysctlKinfoProc("kern.proc.pid", int(pid))
	if err != nil || int64(kinfo.Proc.P_pid) != pid {
		return 0, false
	}
	return kinfo.Eproc.Ucred.Uid, true
}

func bytesPointer(b []byte) unsafe.Pointer {
	return unsafe.Pointer(&b[0])
}
