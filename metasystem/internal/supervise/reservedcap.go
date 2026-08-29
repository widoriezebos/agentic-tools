package supervise

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
)

// The reserved-cap arming gate: a watcher
// ceiling may be armed only when it strictly clears every live cap
// reservation, so an armed watcher can never kill a job whose cap was
// promised at or above that ceiling.

// ReservedCapBlocker is the highest live reservation at or above a proposed
// watcher ceiling.
type ReservedCapBlocker struct {
	Job string
	Cap int64
}

// BlockingReservedCap scans the agents directory's job records and mission
// fence reservations for a non-terminal reservation whose capMin is at or
// above the proposed watcher ceiling, returning the highest blocker (largest
// cap, ties to the lexically first job). ok is false when the ceiling clears
// every live reservation. An error refuses arming by name: a record this
// scan cannot read might carry the very reservation that blocks the ceiling
// (FAIL CLOSED) — only a record that vanished between glob
// and read is tolerable. The identity-fixture fence rides here
// too: this gate runs at every arming choke point, and a leaked
// fixture entry would keep a dead custodian reading Alive with no refusal
// anywhere.
func BlockingReservedCap(agentsDir string, ceiling int64) (ReservedCapBlocker, bool, error) {
	root := filepath.Dir(filepath.Dir(agentsDir))
	return BlockingReservedCapAt(agentsDir, root, ceiling)
}

// BlockingReservedCapAt scans repository state at agentsDir and checks fixture
// authority against the installed metasystem configuration.
func BlockingReservedCapAt(agentsDir, metasystemRoot string, ceiling int64) (ReservedCapBlocker, bool, error) {
	if os.Getenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE") != "" {
		if config.ConfValue(filepath.Join(metasystemRoot, "metasystem.conf"), "metasystem.runtimes", "") != "fake" {
			return ReservedCapBlocker{}, false, fmt.Errorf("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE is set but metasystem.runtimes is not fake")
		}
	}
	reserved := map[string]int64{}
	jobsDir := filepath.Join(agentsDir, "jobs")
	jobPaths, _ := filepath.Glob(filepath.Join(jobsDir, "*.json"))
	sort.Strings(jobPaths)
	for _, path := range jobPaths {
		record, err := readObjectFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return ReservedCapBlocker{}, false, fmt.Errorf("job record unreadable: %s: %v", path, err)
		}
		cap, capOK := intField(record["capMin"])
		status, _ := record["status"].(string)
		job, _ := record["jobId"].(string)
		if job == "" {
			job = strings.TrimSuffix(filepath.Base(path), ".json")
		}
		if !capOK && record["capMin"] != nil {
			return ReservedCapBlocker{}, false, fmt.Errorf("job record capMin is malformed: %s", path)
		}
		if capOK && cap >= ceiling && !dispatch.TerminalStatus(status) {
			reserved[job] = cap
		}
	}
	fencePaths, _ := filepath.Glob(filepath.Join(agentsDir, "missions", "*", "fences.json"))
	sort.Strings(fencePaths)
	for _, path := range fencePaths {
		fences, err := readObjectFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return ReservedCapBlocker{}, false, fmt.Errorf("fence counters unreadable: %s: %v", path, err)
		}
		reservations, _ := fences["reservations"].(map[string]any)
		for _, job := range sortedReservationKeys(reservations) {
			reservation, ok := reservations[job].(map[string]any)
			if !ok {
				return ReservedCapBlocker{}, false, fmt.Errorf("reservation %s is malformed in %s", job, path)
			}
			cap, capOK := intField(reservation["capMin"])
			if !capOK {
				return ReservedCapBlocker{}, false, fmt.Errorf("reservation %s capMin is malformed in %s", job, path)
			}
			if cap < ceiling {
				continue
			}
			status := ""
			if record, err := readObjectFile(filepath.Join(jobsDir, job+".json")); err == nil {
				status, _ = record["status"].(string)
			} else if !os.IsNotExist(err) {
				return ReservedCapBlocker{}, false, fmt.Errorf("reserved job record unreadable: %s: %v", job, err)
			}
			if !dispatch.TerminalStatus(status) && cap > reserved[job] {
				reserved[job] = cap
			}
		}
	}
	if len(reserved) == 0 {
		return ReservedCapBlocker{}, false, nil
	}
	jobs := make([]string, 0, len(reserved))
	for job := range reserved {
		jobs = append(jobs, job)
	}
	sort.Slice(jobs, func(i, j int) bool {
		if reserved[jobs[i]] != reserved[jobs[j]] {
			return reserved[jobs[i]] > reserved[jobs[j]]
		}
		return jobs[i] < jobs[j]
	})
	return ReservedCapBlocker{Job: jobs[0], Cap: reserved[jobs[0]]}, true, nil
}

func sortedReservationKeys(reservations map[string]any) []string {
	keys := make([]string, 0, len(reservations))
	for key := range reservations {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func readObjectFile(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	return doc, nil
}

// intField reads a decoded JSON number as an integral int64.
func intField(v any) (int64, bool) {
	f, ok := v.(float64)
	if !ok || f != float64(int64(f)) {
		return 0, false
	}
	return int64(f), true
}
