// Package delegatecustody reads the exact process identities a delegate job
// record holds: the supervisor the record names and every registered CLI
// custody process. The Stop hook's delegate check (internal/lease) and the
// cache domain (internal/cachedomain) authenticate a process's ancestry
// against them; environment hints are never authority.
package delegatecustody

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// Owned is one process a job record owns.
type Owned struct {
	JobID string
	Ref   identity.Ref
}

type processRecord struct {
	Pid               int64  `json:"pid"`
	PidStartedAt      int64  `json:"pidStartedAt"`
	PidStartedAtMicro int64  `json:"pidStartedAtExactMicro,omitempty"`
	PidStartTicks     int64  `json:"pidStartTicks,omitempty"`
	BootID            string `json:"bootId,omitempty"`
}

type jobRecord struct {
	JobID            string          `json:"jobId"`
	Pid              *int64          `json:"pid"`
	PidStartedAt     *int64          `json:"pidStartedAt"`
	PidStartedMicro  *int64          `json:"pidStartedAtExactMicro"`
	PidStartTicks    *int64          `json:"pidStartTicks"`
	BootID           *string         `json:"bootId"`
	CustodyProcesses []processRecord `json:"custodyProcesses"`
}

// Read returns the processes the job records under stateRoot own: only
// onlyJob's when it is set (its record must exist and name it), else every
// local job's. readFile is os.ReadFile when nil.
func Read(stateRoot, onlyJob string, readFile func(string) ([]byte, error)) ([]Owned, error) {
	if readFile == nil {
		readFile = os.ReadFile
	}
	jobsDir := filepath.Join(stateRoot, "artifacts", "agents", "jobs")
	var paths []string
	if onlyJob != "" {
		paths = []string{filepath.Join(jobsDir, onlyJob+".json")}
	} else {
		var err error
		paths, err = filepath.Glob(filepath.Join(jobsDir, "*.json"))
		if err != nil {
			return nil, fmt.Errorf("hook delegate query could not list job records: %w", err)
		}
		sort.Strings(paths)
	}
	var out []Owned
	for _, path := range paths {
		data, err := readFile(path)
		if err != nil {
			if onlyJob == "" && os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("hook delegate query could not read job record %s: %w", filepath.Base(path), err)
		}
		var record jobRecord
		if json.Unmarshal(data, &record) != nil || record.JobID == "" || (onlyJob != "" && record.JobID != onlyJob) {
			return nil, fmt.Errorf("hook delegate query found a corrupt or mismatched job record: %s", filepath.Base(path))
		}
		if record.Pid != nil || record.PidStartedAt != nil {
			if record.Pid == nil || record.PidStartedAt == nil {
				return nil, fmt.Errorf("hook delegate query found a partial job identity: %s", filepath.Base(path))
			}
			process := processRecord{Pid: *record.Pid, PidStartedAt: *record.PidStartedAt}
			if record.PidStartedMicro != nil {
				process.PidStartedAtMicro = *record.PidStartedMicro
			}
			if record.PidStartTicks != nil {
				process.PidStartTicks = *record.PidStartTicks
			}
			if record.BootID != nil {
				process.BootID = *record.BootID
			}
			ref, err := process.ref()
			if err != nil {
				return nil, fmt.Errorf("hook delegate query found an invalid job identity in %s: %w", filepath.Base(path), err)
			}
			out = append(out, Owned{JobID: record.JobID, Ref: ref})
		}
		for _, process := range record.CustodyProcesses {
			ref, err := process.ref()
			if err != nil {
				return nil, fmt.Errorf("hook delegate query found invalid custody in %s: %w", filepath.Base(path), err)
			}
			out = append(out, Owned{JobID: record.JobID, Ref: ref})
		}
	}
	return out, nil
}

func (p processRecord) ref() (identity.Ref, error) {
	ref := identity.Ref{Pid: p.Pid, StartedAtSec: p.PidStartedAt, StartedAtUnixMicro: p.PidStartedAtMicro, StartTicks: p.PidStartTicks, BootID: p.BootID}
	if p.Pid < 1 || p.PidStartedAt < 1 || ref.Mode() == identity.CompareInvalid {
		return identity.Ref{}, fmt.Errorf("invalid process identity for pid %d", p.Pid)
	}
	return ref, nil
}

// Walk reports the first process on the ancestry from pid (itself first)
// that one of owned matches by exact identity. probe reads a live process's
// identity (false when it cannot be read: the walk refuses), parent its
// parent pid.
func Walk(owned []Owned, pid int64, probe func(int64) (identity.Exact, bool), parent func(int64) (int64, bool)) (Owned, int64, identity.ComparisonMode, bool, error) {
	seen := map[int64]bool{}
	current := pid
	for current > 0 && !seen[current] {
		seen[current] = true
		exact, ok := probe(current)
		if !ok {
			return Owned{}, 0, "", false, fmt.Errorf("hook delegate query could not authenticate process %d", current)
		}
		for _, candidate := range owned {
			if comparison := identity.Compare(exact, candidate.Ref); comparison.Matches {
				return candidate, current, comparison.Mode, true, nil
			}
		}
		next, present := parent(current)
		if !present || next == current {
			break
		}
		current = next
	}
	return Owned{}, 0, "", false, nil
}
