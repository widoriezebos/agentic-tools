//go:build linux

package proofrun

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

const (
	linuxATClockTicks = 17
	linuxUserHZ       = 100
)

var (
	linuxClockOnce    sync.Once
	linuxClockTicks   float64
	linuxClockAssumed bool
)

type linuxProcessTreeReader struct {
	ticks   float64
	assumed bool
}

func newProcessTreeReader() processTreeReader {
	linuxClockOnce.Do(func() {
		data, err := os.ReadFile("/proc/self/auxv")
		if err == nil {
			linuxClockTicks, err = parseAuxClockTicks(data)
		}
		if err != nil || linuxClockTicks <= 0 {
			linuxClockTicks, linuxClockAssumed = linuxUserHZ, true
		}
	})
	return &linuxProcessTreeReader{ticks: linuxClockTicks, assumed: linuxClockAssumed}
}

func (reader *linuxProcessTreeReader) ProgressRuleSuffix() string {
	if reader.assumed {
		return "ticks/assumed-100"
	}
	return ""
}

func parseAuxClockTicks(data []byte) (float64, error) {
	wordBytes := strconv.IntSize / 8
	pairBytes := wordBytes * 2
	for offset := 0; offset+pairBytes <= len(data); offset += pairBytes {
		var tag, value uint64
		if wordBytes == 8 {
			tag = binary.NativeEndian.Uint64(data[offset : offset+wordBytes])
			value = binary.NativeEndian.Uint64(data[offset+wordBytes : offset+pairBytes])
		} else {
			tag = uint64(binary.NativeEndian.Uint32(data[offset : offset+wordBytes]))
			value = uint64(binary.NativeEndian.Uint32(data[offset+wordBytes : offset+pairBytes]))
		}
		if tag == 0 {
			break
		}
		if tag == linuxATClockTicks && value > 0 {
			return float64(value), nil
		}
	}
	return 0, fmt.Errorf("AT_CLKTCK is absent from /proc/self/auxv")
}

func (reader *linuxProcessTreeReader) Sample(rootPID int) (processTreeSample, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return processTreeSample{}, fmt.Errorf("read /proc: %w", err)
	}
	parents := map[int]int{}
	partial := false
	for _, entry := range entries {
		pid, parseErr := strconv.Atoi(entry.Name())
		if parseErr != nil || pid < 1 {
			continue
		}
		data, readErr := os.ReadFile("/proc/" + entry.Name() + "/stat")
		if readErr != nil {
			if linuxProcessVanished(readErr) {
				partial = true
				continue
			}
			return processTreeSample{}, fmt.Errorf("read process tree member %d: %w", pid, readErr)
		}
		stat, parseErr := parseLinuxProcessStat(data)
		if parseErr != nil {
			return processTreeSample{}, fmt.Errorf("parse process tree member %d: %w", pid, parseErr)
		}
		parents[pid] = stat.ppid
	}
	members := descendantProcessIDs(rootPID, parents)
	sample := processTreeSample{Members: append([]int(nil), members...), Partial: partial, MemberCPU: make([]processMemberCPU, 0, len(members))}
	for _, pid := range members {
		data, readErr := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
		if readErr != nil {
			if linuxProcessVanished(readErr) {
				sample.Partial = true
				continue
			}
			return processTreeSample{}, fmt.Errorf("read process counter member %d: %w", pid, readErr)
		}
		stat, parseErr := parseLinuxProcessStat(data)
		if parseErr != nil {
			return processTreeSample{}, fmt.Errorf("parse process counter member %d: %w", pid, parseErr)
		}
		sample.MemberCPU = append(sample.MemberCPU, processMemberCPU{PID: pid, Started: strconv.FormatUint(stat.startTicks, 10), CPUSeconds: float64(linuxGroupMemberCPUTicks(stat)) / reader.ticks})
		sample.Stopped = sample.Stopped || stat.state == "T" || stat.state == "t"
		sample.Waiting = sample.Waiting || stat.state == "D"
	}
	return sample, nil
}

func linuxProcessVanished(err error) bool {
	return errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ESRCH)
}

type linuxProcessStat struct {
	state                  string
	ppid                   int
	ownCPUTicks            int64
	reapedChildrenCPUTicks int64
	startTicks             uint64
}

func parseLinuxProcessStat(data []byte) (linuxProcessStat, error) {
	closing := strings.LastIndexByte(string(data), ')')
	if closing < 0 {
		return linuxProcessStat{}, fmt.Errorf("missing command delimiter")
	}
	fields := strings.Fields(string(data[closing+1:]))
	if len(fields) < 20 {
		return linuxProcessStat{}, fmt.Errorf("stat has %d fields after command, need 20", len(fields))
	}
	ppid, err := strconv.Atoi(fields[1])
	if err != nil {
		return linuxProcessStat{}, fmt.Errorf("parse parent pid: %w", err)
	}
	cpuFields := make([]int64, 4)
	for fieldIndex, index := range []int{11, 12, 13, 14} {
		value, parseErr := strconv.ParseInt(fields[index], 10, 64)
		if parseErr != nil || value < 0 {
			return linuxProcessStat{}, fmt.Errorf("parse CPU field %d", index+3)
		}
		cpuFields[fieldIndex] = value
	}
	startTicks, err := strconv.ParseUint(fields[19], 10, 64)
	if err != nil {
		return linuxProcessStat{}, fmt.Errorf("parse process start time: %w", err)
	}
	return linuxProcessStat{
		state:                  fields[0],
		ppid:                   ppid,
		ownCPUTicks:            cpuFields[0] + cpuFields[1],
		reapedChildrenCPUTicks: cpuFields[2] + cpuFields[3],
		startTicks:             startTicks,
	}, nil
}

func linuxGroupMemberCPUTicks(stat linuxProcessStat) int64 {
	return stat.ownCPUTicks + stat.reapedChildrenCPUTicks
}

func descendantProcessIDs(rootPID int, parents map[int]int) []int {
	selected := map[int]bool{rootPID: true}
	changed := true
	for changed {
		changed = false
		for pid, ppid := range parents {
			if selected[ppid] && !selected[pid] {
				selected[pid], changed = true, true
			}
		}
	}
	result := make([]int, 0, len(selected))
	for pid := range selected {
		result = append(result, pid)
	}
	return uniqueProcessIDs(result, 0)
}
