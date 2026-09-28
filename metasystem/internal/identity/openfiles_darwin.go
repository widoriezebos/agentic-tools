package identity

import (
	"encoding/binary"
	"fmt"

	"golang.org/x/sys/unix"
)

// ProcessUse is what a live process has open for the disk sweeper's use
// census (Part B 3.1): its working directory, its executable and the path
// of every vnode descriptor.
type ProcessUse struct {
	Cwd        string
	Executable string
	Files      []string
}

// ProcessUID is the effective uid of a process from its kern.proc.pid
// entry; ok is false when the process is gone or undisclosed.
func ProcessUID(pid int64) (uint32, bool) {
	if pid < 1 {
		return 0, false
	}
	kinfo, err := unix.SysctlKinfoProc("kern.proc.pid", int(pid))
	if err != nil || int64(kinfo.Proc.P_pid) != pid {
		return 0, false
	}
	return kinfo.Eproc.Ucred.Uid, true
}

// ReadProcessUse reads a process's cwd, executable and open vnode paths
// through proc_info (PROC_PIDLISTFDS, then PROC_PIDFDVNODEPATHINFO per vnode
// descriptor; the family ProcessCwd uses). An error means the process could
// not be fully read; the census then is incomplete for it.
func ReadProcessUse(pid int64) (ProcessUse, error) {
	const (
		sysProcInfo            = 336 // SYS_proc_info
		procInfoCallPidInfo    = 2   // PROC_INFO_CALL_PIDINFO
		procInfoCallPidFDInfo  = 3   // PROC_INFO_CALL_PIDFDINFO
		procPidListFDs         = 1   // PROC_PIDLISTFDS
		procPidFDVnodePathInfo = 2   // PROC_PIDFDVNODEPATHINFO
		proxFDTypeVnode        = 1   // PROX_FDTYPE_VNODE
		fdInfoSize             = 8   // struct proc_fdinfo
		vnodeFDInfoSize        = 1200
		vnodePathOffset        = 24 + 152 // proc_fileinfo, then vnode_info
	)
	var use ProcessUse
	cwd, ok := ProcessCwd(pid)
	if !ok {
		return use, fmt.Errorf("pid %d: working directory unreadable", pid)
	}
	use.Cwd = cwd
	executable, err := executablePathErr(pid)
	if err != nil {
		return use, fmt.Errorf("pid %d: executable unreadable: %w", pid, err)
	}
	use.Executable = executable
	size, _, errno := unix.Syscall6(sysProcInfo, procInfoCallPidInfo, uintptr(pid), procPidListFDs, 0, 0, 0)
	if errno != 0 {
		return use, fmt.Errorf("pid %d: descriptor list unreadable: %v", pid, errno)
	}
	// Room for descriptors opened between the size query and the list.
	buffer := make([]byte, int(size)+32*fdInfoSize)
	filled, _, errno := unix.Syscall6(sysProcInfo, procInfoCallPidInfo, uintptr(pid), procPidListFDs, 0,
		uintptr(bytesPointer(buffer)), uintptr(len(buffer)))
	if errno != 0 {
		return use, fmt.Errorf("pid %d: descriptor list unreadable: %v", pid, errno)
	}
	for offset := 0; offset+fdInfoSize <= int(filled); offset += fdInfoSize {
		fd := int32(binary.LittleEndian.Uint32(buffer[offset:]))
		kind := binary.LittleEndian.Uint32(buffer[offset+4:])
		if kind != proxFDTypeVnode {
			continue
		}
		info := make([]byte, vnodeFDInfoSize)
		read, _, errno := unix.Syscall6(sysProcInfo, procInfoCallPidFDInfo, uintptr(pid), procPidFDVnodePathInfo,
			uintptr(fd), uintptr(bytesPointer(info)), uintptr(len(info)))
		if errno == unix.EBADF || errno == unix.ENOENT {
			// Closed since the list was taken, or a vnode with no path left
			// (an unlinked file): neither can lie inside a store's path.
			continue
		}
		if errno != 0 || int(read) < vnodePathOffset {
			return use, fmt.Errorf("pid %d: descriptor %d unreadable: %v", pid, fd, errno)
		}
		path := info[vnodePathOffset:]
		end := 0
		for end < len(path) && path[end] != 0 {
			end++
		}
		if end > 0 {
			use.Files = append(use.Files, string(path[:end]))
		}
	}
	return use, nil
}

// executablePathErr reads PROC_PIDPATHINFO with its errno: an executable
// that no longer has a path (replaced on disk, ENOENT) is an empty path, not
// an unreadable process.
func executablePathErr(pid int64) (string, error) {
	const (
		sysProcInfo         = 336
		procInfoCallPidInfo = 2
		procPidPathInfo     = 11
	)
	buffer := make([]byte, 4096)
	_, _, errno := unix.Syscall6(sysProcInfo, procInfoCallPidInfo, uintptr(pid), procPidPathInfo, 0,
		uintptr(bytesPointer(buffer)), uintptr(len(buffer)))
	if errno == unix.ENOENT {
		return "", nil
	}
	if errno != 0 {
		return "", errno
	}
	end := 0
	for end < len(buffer) && buffer[end] != 0 {
		end++
	}
	return string(buffer[:end]), nil
}
