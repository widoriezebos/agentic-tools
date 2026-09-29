package diskstore

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// RecordedProcesses indexes the processes the metasystem started or
// recorded, for the census's "not ours" judgement (design owner,
// 2026-09-29). A record's pid counts with the start time the record
// carries, when it carries one, so a reused pid does not match; a pid
// recorded without a start time matches whatever runs under it now, which
// errs toward "ours" and so toward keeping stores.
type RecordedProcesses struct {
	starts map[int64][]int64
	// StartOf is the kernel's start second of a live process.
	StartOf func(pid int64) (int64, bool)
	// Engine reports a process running a metasystem engine binary.
	Engine func(pid int64) bool
}

// Add records one pid with its start second (0: unknown).
func (r *RecordedProcesses) Add(pid, start int64) {
	if pid <= 1 {
		return
	}
	if r.starts == nil {
		r.starts = map[int64][]int64{}
	}
	r.starts[pid] = append(r.starts[pid], start)
}

// Len is how many pids are recorded.
func (r *RecordedProcesses) Len() int { return len(r.starts) }

// Ours reports whether pid is a recorded process or runs an engine binary.
func (r *RecordedProcesses) Ours(pid int64) bool {
	if r.Engine != nil && r.Engine(pid) {
		return true
	}
	starts, recorded := r.starts[pid]
	if !recorded {
		return false
	}
	now, known := int64(0), false
	if r.StartOf != nil {
		now, known = r.StartOf(pid)
	}
	for _, start := range starts {
		if start == 0 || !known || start == now || start+1 == now || start-1 == now {
			return true
		}
	}
	return false
}

// ScanFiles records every pid a JSON record names: a numeric member whose
// name is pid or ends in Pid/pid, with the start second of a sibling
// pidStartedAt, PidStartedAt, startedAtSec or StartedAtSec when present.
// Unreadable or non-JSON files are skipped: they name no process.
func (r *RecordedProcesses) ScanFiles(paths []string) {
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() || info.Size() > 4<<20 {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var value any
		if json.Unmarshal(data, &value) != nil {
			continue
		}
		r.scan(value)
	}
}

func (r *RecordedProcesses) scan(value any) {
	switch typed := value.(type) {
	case []any:
		for _, element := range typed {
			r.scan(element)
		}
	case map[string]any:
		start := int64(0)
		for _, key := range []string{"pidStartedAt", "PidStartedAt", "startedAtSec", "StartedAtSec"} {
			if number, ok := typed[key].(float64); ok && number > 0 {
				start = int64(number)
			}
		}
		for key, member := range typed {
			number, ok := member.(float64)
			lower := strings.ToLower(key)
			if ok && (lower == "pid" || strings.HasSuffix(lower, "pid")) && !strings.HasSuffix(lower, "ppid") {
				if lower == "pid" {
					r.Add(int64(number), start)
				} else {
					r.Add(int64(number), 0)
				}
				continue
			}
			r.scan(member)
		}
	}
}

// RecordFiles lists the JSON files directly in each directory, and in each
// launch directory of launchRoot that has no result.json (a launch still
// running or unfinished).
func RecordFiles(directories []string, launchRoot string) []string {
	var files []string
	for _, directory := range directories {
		entries, err := os.ReadDir(directory)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.Type().IsRegular() && (strings.HasSuffix(entry.Name(), ".json") || strings.HasPrefix(entry.Name(), "lease-")) {
				files = append(files, filepath.Join(directory, entry.Name()))
			}
		}
	}
	if launchRoot != "" {
		entries, _ := os.ReadDir(launchRoot)
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			launch := filepath.Join(launchRoot, entry.Name())
			if _, err := os.Stat(filepath.Join(launch, "result.json")); err == nil {
				continue
			}
			files = append(files, filepath.Join(launch, "record.json"))
		}
	}
	return files
}

// EngineExecutable reports a path that is a metasystem engine binary: an
// installation's bin/metasystem, an engine pin or a candidate engine.
func EngineExecutable(path string) bool {
	return filepath.Base(path) == "metasystem" || strings.Contains(path, string(filepath.Separator)+"engine-pins"+string(filepath.Separator)) ||
		strings.Contains(path, string(filepath.Separator)+"candidate-engines"+string(filepath.Separator))
}
