//go:build darwin

package identity

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

// KernelProber reads process identity from the darwin kernel:
// start time from kern.proc.pid (microsecond resolution, the exactness
// the Go ruling exists for) and argv from kern.procargs2 for REG-6's
// claim-consistency factor.
type KernelProber struct{}

// kinfoProc's layout: extern_proc begins the struct, and its first
// field (inside the p_un union) is p_starttime, a struct timeval
// { tv_sec int64; tv_usec int32; pad int32 } on 64-bit darwin.
const kinfoStartSecOffset = 0
const kinfoStartUsecOffset = 8

func (KernelProber) Probe(pid int64) (Exact, Liveness, error) {
	if pid < 1 {
		return Exact{}, Unknown, fmt.Errorf("identity: invalid pid %d", pid)
	}
	raw, err := unix.SysctlRaw("kern.proc.pid", int(pid))
	if err != nil {
		if errors.Is(err, unix.ESRCH) || errors.Is(err, unix.ENOENT) {
			return Exact{}, Dead, nil
		}
		return Exact{}, Unknown, fmt.Errorf("identity: sysctl kern.proc.pid %d: %w", pid, err)
	}
	// A successful call with an empty result is the kernel saying "no
	// such process": a definitive negative.
	if len(raw) == 0 {
		return Exact{}, Dead, nil
	}
	if len(raw) < kinfoStartUsecOffset+4 {
		return Exact{}, Unknown, fmt.Errorf("identity: kern.proc.pid %d returned %d bytes", pid, len(raw))
	}
	sec := int64(binary.LittleEndian.Uint64(raw[kinfoStartSecOffset:]))
	usec := int32(binary.LittleEndian.Uint32(raw[kinfoStartUsecOffset:]))
	if sec <= 0 || usec < 0 || usec > 999999 {
		return Exact{}, Unknown, fmt.Errorf("identity: pid %d start time is implausible (sec=%d usec=%d)", pid, sec, usec)
	}
	exact := Exact{Pid: pid, StartedAt: time.Unix(sec, int64(usec)*1000)}
	// Argv is best-effort at probe time: a process we cannot read the
	// argv of is still alive, and the KILL decision separately demands
	// a readable, claim-consistent argv (REG-6) — absence there means
	// no kill, not a guess. ArgvKnown carries the distinction between
	// "read and empty-of-tag" and "unreadable" to consumers (B1).
	if argv, err := procArgs(pid); err == nil {
		exact.Argv = argv
		exact.ArgvKnown = true
	}
	return exact, Alive, nil
}

// procArgs reads argv via KERN_PROCARGS2: the buffer holds argc, the
// executable path, NUL padding, then the NUL-separated argv and env.
func procArgs(pid int64) ([]string, error) {
	raw, err := unix.SysctlRaw("kern.procargs2", int(pid))
	if err != nil {
		return nil, fmt.Errorf("identity: procargs2 %d: %w", pid, err)
	}
	if len(raw) < 4 {
		return nil, fmt.Errorf("identity: procargs2 %d: short read", pid)
	}
	argc := int(*(*int32)(unsafe.Pointer(&raw[0])))
	if argc < 1 || argc > 4096 {
		return nil, fmt.Errorf("identity: procargs2 %d: implausible argc %d", pid, argc)
	}
	rest := raw[4:]
	// Skip the executable path and its NUL padding.
	cut := bytes.IndexByte(rest, 0)
	if cut < 0 {
		return nil, fmt.Errorf("identity: procargs2 %d: unterminated exec path", pid)
	}
	rest = rest[cut:]
	for len(rest) > 0 && rest[0] == 0 {
		rest = rest[1:]
	}
	var argv []string
	for len(rest) > 0 && len(argv) < argc {
		cut = bytes.IndexByte(rest, 0)
		if cut < 0 {
			break
		}
		argv = append(argv, string(rest[:cut]))
		rest = rest[cut+1:]
	}
	if len(argv) != argc {
		return nil, fmt.Errorf("identity: procargs2 %d: read %d of %d argv words", pid, len(argv), argc)
	}
	return argv, nil
}
