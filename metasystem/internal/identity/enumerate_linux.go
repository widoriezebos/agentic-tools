//go:build linux

package identity

import (
	"fmt"
	"os"
	"strconv"
)

// AllPids returns every process id on the machine by reading /proc and
// keeping the entirely numeric entries. Kernel threads are included,
// matching darwin's kern.proc.all; they are filtered downstream by the
// census's empty-command rejection, so no extra filtering belongs here.
func AllPids() ([]int64, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, fmt.Errorf("identity: read /proc: %w", err)
	}
	var pids []int64
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.ParseInt(entry.Name(), 10, 64)
		if err != nil || pid < 1 {
			continue
		}
		pids = append(pids, pid)
	}
	return pids, nil
}

// ProcessCwd returns a process's current working directory by resolving
// /proc/<pid>/cwd. ok is false when it cannot be read — reading another
// user's process fails with EACCES, which the census treats as a denial,
// not as absence, exactly like the darwin proc_info path.
func ProcessCwd(pid int64) (string, bool) {
	target, err := os.Readlink(fmt.Sprintf("/proc/%d/cwd", pid))
	if err != nil || target == "" {
		return "", false
	}
	return target, true
}

// ParentPid returns a process's parent pid from /proc/<pid>/stat field 4,
// parsed with the same last-parenthesis rule as the prober. The returned
// bool is false when the pid cannot be read or when the parent is not a
// real distinct ancestor — ppid <= 0 or ppid == pid — so the caller stops
// its ancestry walk where there is no distinct live ancestor. Pid 1 has
// ppid 0, which correctly returns false.
func ParentPid(pid int64) (int64, bool) {
	stat, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return 0, false
	}
	_, ppid, err := parseProcStat(string(stat))
	if err != nil || ppid <= 0 || ppid == pid {
		return 0, false
	}
	return ppid, true
}
