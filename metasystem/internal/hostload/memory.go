package hostload

import (
	"context"
	"errors"
	"math"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	procMeminfoPath          = "/proc/meminfo"
	procSelfCgroupPath       = "/proc/self/cgroup"
	cgroupV2MountPath        = "/sys/fs/cgroup"
	procMeminfoSource        = "linux:/proc/meminfo"
	procMeminfoCgroupSource  = "linux:/proc/meminfo+cgroup-v2"
	darwinVMStatMemorySource = "darwin:vm_stat"
	darwinVMStatTimeout      = 2 * time.Second
)

type memoryFileReader func(string) ([]byte, error)
type memoryCommandRunner func(context.Context, string, ...string) ([]byte, error)

// AvailableMemory takes one native snapshot and reports the bytes available
// for new work. Unsupported or invalid snapshots are deliberately unknown.
func AvailableMemory() (bytes uint64, source string, available bool) {
	return availableMemoryForOS(runtime.GOOS, os.ReadFile, func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return exec.CommandContext(ctx, name, args...).Output()
	})
}

func availableMemoryForOS(goos string, read memoryFileReader, run memoryCommandRunner) (uint64, string, bool) {
	switch goos {
	case "linux":
		return linuxAvailableMemory(read)
	case "darwin":
		if run == nil {
			return 0, "", false
		}
		ctx, cancel := context.WithTimeout(context.Background(), darwinVMStatTimeout)
		defer cancel()
		output, err := run(ctx, "vm_stat")
		if err != nil {
			return 0, "", false
		}
		available, ok := parseDarwinVMStat(string(output))
		if !ok {
			return 0, "", false
		}
		return available, darwinVMStatMemorySource, true
	default:
		return 0, "", false
	}
}

func linuxAvailableMemory(read memoryFileReader) (uint64, string, bool) {
	if read == nil {
		return 0, "", false
	}
	meminfo, err := read(procMeminfoPath)
	if err != nil {
		return 0, "", false
	}
	available, ok := parseProcMemAvailable(string(meminfo))
	if !ok {
		return 0, "", false
	}

	membershipData, err := read(procSelfCgroupPath)
	if err != nil {
		return 0, "", false
	}
	membership, ok := parseCgroupV2Membership(string(membershipData))
	if !ok {
		return 0, "", false
	}
	// The bounded Linux probe supports membership paths that map directly
	// beneath the conventional cgroup-v2 mount. Missing levels are unknown.
	directory := path.Join(cgroupV2MountPath, strings.TrimPrefix(membership, "/"))
	limited := false
	for {
		maximumData, maximumErr := read(path.Join(directory, "memory.max"))
		if maximumErr != nil {
			if directory == cgroupV2MountPath && errors.Is(maximumErr, os.ErrNotExist) {
				break
			}
			return 0, "", false
		}
		maximumText := strings.TrimSpace(string(maximumData))
		if maximumText != "max" {
			maximum, parseErr := strconv.ParseUint(maximumText, 10, 64)
			if parseErr != nil {
				return 0, "", false
			}
			currentData, currentErr := read(path.Join(directory, "memory.current"))
			if currentErr != nil {
				return 0, "", false
			}
			current, parseErr := strconv.ParseUint(strings.TrimSpace(string(currentData)), 10, 64)
			if parseErr != nil {
				return 0, "", false
			}
			remaining := uint64(0)
			if maximum > current {
				remaining = maximum - current
			}
			available = min(available, remaining)
			limited = true
		}
		if directory == cgroupV2MountPath {
			break
		}
		directory = path.Dir(directory)
	}
	if limited {
		return available, procMeminfoCgroupSource, true
	}
	return available, procMeminfoSource, true
}

func parseCgroupV2Membership(contents string) (string, bool) {
	membership := ""
	found := false
	for _, rawLine := range strings.Split(contents, "\n") {
		line := strings.TrimSuffix(rawLine, "\r")
		if line == "" {
			continue
		}
		fields := strings.SplitN(line, ":", 3)
		if len(fields) != 3 {
			return "", false
		}
		if fields[0] != "0" {
			continue
		}
		candidate := fields[2]
		if found || fields[1] != "" || !path.IsAbs(candidate) || path.Clean(candidate) != candidate || strings.ContainsRune(candidate, '\x00') {
			return "", false
		}
		membership, found = candidate, true
	}
	return membership, found
}

func parseProcMemAvailable(contents string) (uint64, bool) {
	found := false
	var available uint64
	for _, line := range strings.Split(contents, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || fields[0] != "MemAvailable:" {
			continue
		}
		if found || len(fields) != 3 || fields[2] != "kB" {
			return 0, false
		}
		kilobytes, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil || kilobytes > math.MaxUint64/1024 {
			return 0, false
		}
		available, found = kilobytes*1024, true
	}
	return available, found
}

func parseDarwinVMStat(contents string) (uint64, bool) {
	lines := strings.Split(contents, "\n")
	if len(lines) == 0 {
		return 0, false
	}
	const pageSizePrefix = "page size of "
	pageSizeStart := strings.Index(lines[0], pageSizePrefix)
	if pageSizeStart < 0 {
		return 0, false
	}
	pageSizeFields := strings.Fields(lines[0][pageSizeStart+len(pageSizePrefix):])
	if len(pageSizeFields) < 2 || pageSizeFields[1] != "bytes)" {
		return 0, false
	}
	pageSize, err := strconv.ParseUint(pageSizeFields[0], 10, 64)
	if err != nil || pageSize == 0 {
		return 0, false
	}

	wanted := map[string]uint64{
		"Pages free":        0,
		"Pages inactive":    0,
		"Pages speculative": 0,
	}
	seen := make(map[string]bool, len(wanted))
	for _, line := range lines[1:] {
		name, raw, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		if _, needed := wanted[name]; !needed {
			continue
		}
		if seen[name] {
			return 0, false
		}
		text := strings.TrimSuffix(strings.TrimSpace(raw), ".")
		pages, parseErr := strconv.ParseUint(text, 10, 64)
		if parseErr != nil {
			return 0, false
		}
		wanted[name], seen[name] = pages, true
	}

	pages := uint64(0)
	for name, count := range wanted {
		if !seen[name] || count > math.MaxUint64-pages {
			return 0, false
		}
		pages += count
	}
	if pages > math.MaxUint64/pageSize {
		return 0, false
	}
	return pages * pageSize, true
}
