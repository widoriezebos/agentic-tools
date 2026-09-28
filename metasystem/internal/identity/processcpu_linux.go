//go:build linux

package identity

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// ProcessCPUSeconds returns the user plus system CPU time charged to pid's
// own threads (/proc/<pid>/stat utime and stime, in userHZ ticks). A process that is gone
// reports ErrNoSuchProcess.
func ProcessCPUSeconds(pid int64) (float64, error) {
	if pid < 1 {
		return 0, fmt.Errorf("identity: invalid pid %d", pid)
	}
	stat, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, fmt.Errorf("identity: CPU of pid %d: %w", pid, ErrNoSuchProcess)
		}
		return 0, fmt.Errorf("identity: CPU of pid %d: %w", pid, err)
	}
	closing := strings.LastIndexByte(string(stat), ')')
	if closing < 0 {
		return 0, fmt.Errorf("identity: CPU of pid %d: no comm delimiter in stat line", pid)
	}
	fields := strings.Fields(string(stat[closing+1:]))
	if len(fields) < 13 {
		return 0, fmt.Errorf("identity: CPU of pid %d: stat line has %d fields after comm", pid, len(fields))
	}
	user, userErr := strconv.ParseUint(fields[11], 10, 64)
	system, systemErr := strconv.ParseUint(fields[12], 10, 64)
	if userErr != nil || systemErr != nil {
		return 0, fmt.Errorf("identity: CPU of pid %d: utime or stime unparsable", pid)
	}
	return float64(user+system) / userHZ, nil
}
