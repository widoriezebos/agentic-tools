//go:build darwin

package identity

import (
	"encoding/binary"
	"fmt"
	"unsafe"

	"golang.org/x/sys/unix"
)

// AllPids returns every process id on the machine via sysctl kern.proc.all —
// the native replacement for shelling out to `ps`. The kernel returns an
// array of kinfo_proc structs; p_pid sits at a fixed offset in each, with a
// fixed stride, so no per-process subprocess is needed.
//
// kinfo_proc has a stable darwin ABI: the extern_proc begins each struct and
// p_pid is at offset 40 (after the 16-byte p_starttime union, two 8-byte
// pointers, p_flag int, and p_stat char with alignment); sizeof(kinfo_proc)
// is 648. The tests assert AllPids finds our own pid and that the pids
// resolve, so an ABI drift is caught rather than silently corrupting.
func AllPids() ([]int64, error) {
	raw, err := unix.SysctlRaw("kern.proc.all")
	if err != nil {
		return nil, fmt.Errorf("identity: sysctl kern.proc.all: %w", err)
	}
	if len(raw) == 0 {
		return nil, nil
	}
	const pPidOffset = 40
	const kinfoProcSize = 648
	if len(raw)%kinfoProcSize != 0 {
		return nil, fmt.Errorf("identity: kern.proc.all returned %d bytes, not a multiple of %d (ABI drift?)", len(raw), kinfoProcSize)
	}
	var pids []int64
	for offset := 0; offset+pPidOffset+4 <= len(raw); offset += kinfoProcSize {
		pid := int32(binary.LittleEndian.Uint32(raw[offset+pPidOffset:]))
		if pid > 0 {
			pids = append(pids, int64(pid))
		}
	}
	return pids, nil
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

// ParentPid returns a process's parent pid via the proc_info syscall
// (PROC_PIDTBSDINFO), the native replacement for `ps -p PID -o ppid=`. The
// returned bool is false when the pid cannot be read (gone or denied) or when
// the parent is not a real distinct ancestor — ppid <= 0 or ppid == pid — so
// the caller stops its ancestry walk exactly where the python's parent_pid
// returned None.
//
// proc_bsdinfo is a stable public darwin ABI: pbi_ppid is the fifth uint32,
// at offset 16 (after pbi_flags, pbi_status, pbi_xstatus, pbi_pid). The whole
// struct is 136 bytes; the kernel returns that count on success, so a short
// return is treated as a read failure rather than trusted.
func ParentPid(pid int64) (int64, bool) {
	const (
		sysProcInfo         = 336 // SYS_proc_info
		procInfoCallPidInfo = 2   // PROC_INFO_CALL_PIDINFO
		procPidTBSDInfo     = 3   // PROC_PIDTBSDINFO
		bsdInfoSize         = 136
		ppidOffset          = 16
	)
	buffer := make([]byte, bsdInfoSize)
	r1, _, errno := unix.Syscall6(
		sysProcInfo,
		uintptr(procInfoCallPidInfo),
		uintptr(pid),
		uintptr(procPidTBSDInfo),
		0,
		uintptr(bytesPointer(buffer)),
		uintptr(len(buffer)),
	)
	if errno != 0 || int(r1) < bsdInfoSize {
		return 0, false
	}
	ppid := int64(int32(binary.LittleEndian.Uint32(buffer[ppidOffset:])))
	if ppid <= 0 || ppid == pid {
		return 0, false
	}
	return ppid, true
}

func bytesPointer(b []byte) unsafe.Pointer {
	return unsafe.Pointer(&b[0])
}
