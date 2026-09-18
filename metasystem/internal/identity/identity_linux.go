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

// KernelProber reads process identity from Linux procfs: start ticks from
// /proc/<pid>/stat paired with the kernel boot ID, plus argv from cmdline.
type KernelProber struct{}

// BootClock returns the current boot identifier and elapsed time on that
// boot. Wait deadlines pair this clock with UTC so a wall-clock step cannot
// silently lengthen a registered wait.
func BootClock() (string, time.Duration, error) {
	id, err := bootID()
	if err != nil {
		return "", 0, err
	}
	raw, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return "", 0, fmt.Errorf("identity: read /proc/uptime: %w", err)
	}
	fields := strings.Fields(string(raw))
	if len(fields) == 0 {
		return "", 0, fmt.Errorf("identity: /proc/uptime is empty")
	}
	seconds, err := strconv.ParseFloat(fields[0], 64)
	if err != nil || seconds < 0 {
		return "", 0, fmt.Errorf("identity: invalid /proc/uptime value %q", fields[0])
	}
	return id, time.Duration(seconds * float64(time.Second)), nil
}

// userHZ is the userspace ABI clock-tick constant. It is 100 on all
// mainstream Linux architectures and independent of the kernel's internal
// CONFIG_HZ; there is no cgo-free sysconf, so it is named here rather than
// probed.
const userHZ = 100

func (KernelProber) ReadStart(pid int64) (Exact, Liveness, error) {
	if pid < 1 {
		return Exact{}, Unknown, fmt.Errorf("identity: invalid pid %d", pid)
	}
	stat, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		// ENOENT on the stat read is the definitive negative — with the
		// restricted-procfs arming refusal guaranteeing an unrestricted
		// procfs, a
		// missing entry means no such process. Anything else (EACCES,
		// EPERM, transient errors) is Unknown, never Dead.
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, unix.ESRCH) {
			return Exact{}, Dead, nil
		}
		return Exact{}, Unknown, fmt.Errorf("identity: read /proc/%d/stat: %w", pid, err)
	}
	statLine := string(stat)
	startTicks, _, err := parseProcStat(statLine)
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
	bootIdentity, err := bootID()
	if err != nil {
		return Exact{}, Unknown, fmt.Errorf("identity: %w", err)
	}
	started := time.Unix(boot, 0).Add(time.Duration(startTicks) * (time.Second / userHZ))
	return Exact{
		Pid: pid, StartedAt: started, StartTicks: startTicks, BootID: bootIdentity,
		Zombie: procStatZombie(statLine), Exiting: procStatExiting(statLine),
	}, Alive, nil
}

func (KernelProber) ReadArgv(pid int64) ([]string, bool) {
	cmdline, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil || len(cmdline) == 0 {
		return nil, false
	}
	parts := bytes.Split(bytes.TrimRight(cmdline, "\x00"), []byte{0})
	argv := make([]string, 0, len(parts))
	for _, part := range parts {
		argv = append(argv, string(part))
	}
	return argv, len(argv) > 0
}

func (p KernelProber) Probe(pid int64) (Exact, Liveness, error) {
	exact, state, err := p.ReadStart(pid)
	if err != nil || state != Alive {
		return exact, state, err
	}
	if argv, known := p.ReadArgv(pid); known {
		exact.Argv = argv
		exact.ArgvKnown = true
	}
	if environ, known := readEnviron(pid); known {
		exact.Environ = environ
		exact.EnvironKnown = true
	}
	if exe, known := kernelExecutablePath(pid); known {
		exact.Exe = exe
		exact.ExeKnown = true
	}
	return exact, Alive, nil
}

func readEnviron(pid int64) ([]string, bool) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/environ", pid))
	if err != nil {
		return nil, false
	}
	data = bytes.TrimRight(data, "\x00")
	if len(data) == 0 {
		return []string{}, true
	}
	parts := bytes.Split(data, []byte{0})
	environ := make([]string, len(parts))
	for index := range parts {
		environ[index] = string(parts[index])
	}
	return environ, true
}

func kernelExecutablePath(pid int64) (string, bool) {
	path, err := os.Readlink(fmt.Sprintf("/proc/%d/exe", pid))
	return path, err == nil && path != ""
}

// parseProcStat extracts fields 22 (starttime, clock ticks since boot) and
// 4 (ppid) from /proc/<pid>/stat content. Field 2 is the executable name in
// parentheses and MAY ITSELF CONTAIN SPACES AND CLOSING PARENTHESES, so the
// parse locates the LAST ')' and splits the remainder: remainder[0] is
// field 3, remainder[1] is field 4 (ppid), remainder[19] is field 22.
// Splitting on whitespace from the front would misparse such names.
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

func procStatZombie(stat string) bool {
	closing := strings.LastIndexByte(stat, ')')
	return closing >= 0 && strings.HasPrefix(strings.TrimSpace(stat[closing+1:]), "Z ")
}

func procStatExiting(stat string) bool {
	const processExitingFlag = 0x00000004 // PF_EXITING in Linux's sched.h.
	closing := strings.LastIndexByte(stat, ')')
	if closing < 0 {
		return false
	}
	fields := strings.Fields(stat[closing+1:])
	if len(fields) <= 6 {
		return false
	}
	flags, err := strconv.ParseUint(fields[6], 10, 64)
	return err == nil && flags&processExitingFlag != 0
}

// bootTimeEpoch reads btime (boot time, epoch seconds) from /proc/stat.
// The anchor is whole-second, so Linux start times carry a constant
// per-machine sub-second offset against true wall clock — harmless, since
// this binary is the system's only start-time clock and every comparison
// goes through it.
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

// bootID reads the kernel's per-boot identity token. Linux exact identity is
// the pair, so an unreadable or empty token makes the observation Unknown.
func bootID() (string, error) {
	data, err := os.ReadFile("/proc/sys/kernel/random/boot_id")
	if err != nil {
		return "", fmt.Errorf("read boot id: %w", err)
	}
	value := strings.TrimSpace(string(data))
	if value == "" {
		return "", fmt.Errorf("boot id is empty")
	}
	return value, nil
}
