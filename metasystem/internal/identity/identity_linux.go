//go:build linux

package identity

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

// KernelProber reads process identity from Linux procfs: start time from
// /proc/<pid>/stat field 22 anchored to /proc/stat's btime (10-millisecond
// tick resolution — every decision path compares whole seconds, so the
// coarser-than-darwin resolution is inert; see the package doc), and argv
// from /proc/<pid>/cmdline.
type KernelProber struct{}

// userHZ is the userspace ABI clock-tick constant. It is 100 on all
// mainstream Linux architectures and independent of the kernel's internal
// CONFIG_HZ; there is no cgo-free sysconf, so it is named here rather than
// probed (plans/linux-portability.md).
const userHZ = 100

func (KernelProber) Probe(pid int64) (Exact, Liveness, error) {
	if pid < 1 {
		return Exact{}, Unknown, fmt.Errorf("identity: invalid pid %d", pid)
	}
	stat, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		// ENOENT on the stat read is the definitive negative — with the
		// B2 arming refusal guaranteeing an unrestricted procfs, a
		// missing entry means no such process. Anything else (EACCES,
		// EPERM, transient errors) is Unknown, never Dead.
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, unix.ESRCH) {
			return Exact{}, Dead, nil
		}
		return Exact{}, Unknown, fmt.Errorf("identity: read /proc/%d/stat: %w", pid, err)
	}
	startTicks, _, err := parseProcStat(string(stat))
	if err != nil {
		return Exact{}, Unknown, fmt.Errorf("identity: /proc/%d/stat: %w", pid, err)
	}
	boot, err := bootTimeEpoch()
	if err != nil {
		return Exact{}, Unknown, fmt.Errorf("identity: %w", err)
	}
	if boot <= 0 || startTicks < 0 {
		return Exact{}, Unknown, fmt.Errorf("identity: pid %d start time is implausible (btime=%d ticks=%d)", pid, boot, startTicks)
	}
	started := time.Unix(boot, 0).Add(time.Duration(startTicks) * (time.Second / userHZ))
	exact := Exact{Pid: pid, StartedAt: started, StartTicks: startTicks, BootID: bootID()}
	// Argv is best-effort at probe time: a process we cannot read the argv
	// of is still alive, and the KILL decision separately demands a
	// readable, claim-consistent argv (REG-6). An empty cmdline (kernel
	// threads, zombies) is an argv read failure, not a liveness failure —
	// ArgvKnown stays false (B1).
	if cmdline, readErr := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid)); readErr == nil && len(cmdline) > 0 {
		parts := bytes.Split(bytes.TrimRight(cmdline, "\x00"), []byte{0})
		argv := make([]string, 0, len(parts))
		for _, part := range parts {
			argv = append(argv, string(part))
		}
		if len(argv) > 0 {
			exact.Argv = argv
			exact.ArgvKnown = true
		}
	}
	return exact, Alive, nil
}

// parseProcStat extracts fields 22 (starttime, clock ticks since boot) and
// 4 (ppid) from /proc/<pid>/stat content. Field 2 is the executable name in
// parentheses and MAY ITSELF CONTAIN SPACES AND CLOSING PARENTHESES, so the
// parse locates the LAST ')' and splits the remainder: remainder[0] is
// field 3, remainder[1] is field 4 (ppid), remainder[19] is field 22
// (plans/linux-portability.md — the parsing trap).
func parseProcStat(stat string) (startTicks int64, ppid int64, err error) {
	closing := strings.LastIndexByte(stat, ')')
	if closing < 0 {
		return 0, 0, fmt.Errorf("no comm delimiter in stat line")
	}
	fields := strings.Fields(stat[closing+1:])
	if len(fields) < 20 {
		return 0, 0, fmt.Errorf("stat line has %d fields after comm, need 20", len(fields))
	}
	ppid, err = strconv.ParseInt(fields[1], 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("ppid unparsable: %w", err)
	}
	startTicks, err = strconv.ParseInt(fields[19], 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("starttime unparsable: %w", err)
	}
	return startTicks, ppid, nil
}

// bootTimeEpoch reads btime (boot time, epoch seconds) from /proc/stat.
// The anchor is whole-second, so Linux start times carry a constant
// per-machine sub-second offset against true wall clock — harmless, since
// this binary is the system's only start-time clock and every comparison
// goes through it (plans/linux-portability.md).
func bootTimeEpoch() (int64, error) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, fmt.Errorf("read /proc/stat: %w", err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		if rest, ok := strings.CutPrefix(line, "btime "); ok {
			return strconv.ParseInt(strings.TrimSpace(rest), 10, 64)
		}
	}
	return 0, fmt.Errorf("/proc/stat has no btime line")
}

// bootID reads the kernel's per-boot identity token. Best-effort: an
// unreadable boot id degrades comparisons to the legacy seconds rule
// rather than failing the probe — absence weakens, never lies.
func bootID() string {
	data, err := os.ReadFile("/proc/sys/kernel/random/boot_id")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
