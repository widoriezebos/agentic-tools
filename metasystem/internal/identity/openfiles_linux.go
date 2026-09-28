package identity

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
)

// ProcessUse is what a live process has open for the disk sweeper's use
// census (Part B 3.1): its working directory, its executable and the path
// of every open file descriptor.
type ProcessUse struct {
	Cwd        string
	Executable string
	Files      []string
}

// ProcessUID is the owner of /proc/<pid>, the process's effective uid.
func ProcessUID(pid int64) (uint32, bool) {
	info, err := os.Stat("/proc/" + strconv.FormatInt(pid, 10))
	if err != nil {
		return 0, false
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, false
	}
	return stat.Uid, true
}

// ReadProcessUse reads /proc/<pid>/cwd, /proc/<pid>/exe and every
// /proc/<pid>/fd/* link. An error means the process could not be fully read.
func ReadProcessUse(pid int64) (ProcessUse, error) {
	base := "/proc/" + strconv.FormatInt(pid, 10)
	var use ProcessUse
	cwd, err := os.Readlink(base + "/cwd")
	if err != nil {
		return use, fmt.Errorf("pid %d: working directory unreadable: %w", pid, err)
	}
	use.Cwd = cwd
	executable, err := os.Readlink(base + "/exe")
	if err != nil {
		return use, fmt.Errorf("pid %d: executable unreadable: %w", pid, err)
	}
	use.Executable = executable
	entries, err := os.ReadDir(base + "/fd")
	if err != nil {
		return use, fmt.Errorf("pid %d: descriptor list unreadable: %w", pid, err)
	}
	for _, entry := range entries {
		target, err := os.Readlink(filepath.Join(base, "fd", entry.Name()))
		if os.IsNotExist(err) {
			continue // closed since the list was taken
		}
		if err != nil {
			return use, fmt.Errorf("pid %d: descriptor %s unreadable: %w", pid, entry.Name(), err)
		}
		if filepath.IsAbs(target) {
			use.Files = append(use.Files, target)
		}
	}
	return use, nil
}
