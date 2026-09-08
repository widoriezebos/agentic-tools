package proofrun

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

const (
	StatusRunning = "running"
	StatusDone    = "done"
)

// ProcessIdentity is the durable, kernel-authenticated identity of one
// process in a proof run. Pgid is an action target only; Ref is the identity
// that must be proved again before that target is signalled.
type ProcessIdentity struct {
	Pid               int64  `json:"pid"`
	Pgid              int64  `json:"pgid,omitempty"`
	PidStartedAt      int64  `json:"pidStartedAt"`
	PidStartedAtMicro int64  `json:"pidStartedAtMicro,omitempty"`
	PidStartTicks     int64  `json:"pidStartTicks,omitempty"`
	BootID            string `json:"bootId,omitempty"`
}

func processIdentity(exact identity.Exact, pgid int64) ProcessIdentity {
	ref := exact.Ref()
	return ProcessIdentity{
		Pid: exact.Pid, Pgid: int64(pgid), PidStartedAt: ref.StartedAtSec,
		PidStartedAtMicro: ref.StartedAtUnixMicro, PidStartTicks: ref.StartTicks,
		BootID: ref.BootID,
	}
}

func (p ProcessIdentity) Ref() identity.Ref {
	return identity.Ref{
		Pid: p.Pid, StartedAtSec: p.PidStartedAt,
		StartedAtUnixMicro: p.PidStartedAtMicro, StartTicks: p.PidStartTicks,
		BootID: p.BootID,
	}
}

// Record is the current proof-run process set for one suite.
type Record struct {
	Suite           string          `json:"suite"`
	Root            string          `json:"root"`
	FenceGeneration int64           `json:"fenceGeneration"`
	Launcher        ProcessIdentity `json:"launcher"`
	SuiteProcess    ProcessIdentity `json:"suiteProcess"`
	Watchdog        ProcessIdentity `json:"watchdog"`
	Status          string          `json:"status"`
}

// RecordPath returns the durable record path for a suite.
func RecordPath(root, suite string) (string, error) {
	if suite == "" || suite == "." || suite == ".." || filepath.Base(suite) != suite || strings.ContainsAny(suite, `/\`) {
		return "", errors.New("proof-run suite must be one path-safe name")
	}
	return filepath.Join(root, "artifacts", "agents", "proof-runs", suite+".json"), nil
}

// ReadRecord reads one proof-run record.
func ReadRecord(root, suite string) (Record, error) {
	path, err := RecordPath(root, suite)
	if err != nil {
		return Record{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Record{}, err
	}
	var record Record
	if err := json.Unmarshal(data, &record); err != nil {
		return Record{}, fmt.Errorf("read proof-run record %s: %w", path, err)
	}
	if err := validateRecord(record); err != nil {
		return Record{}, fmt.Errorf("read proof-run record %s: %w", path, err)
	}
	if record.Root != root || record.Suite != suite {
		return Record{}, fmt.Errorf("read proof-run record %s: record identity does not match its path", path)
	}
	return record, nil
}

// ReadRecords reads the proof-run inventory in stable suite-name order.
func ReadRecords(root string) ([]Record, error) {
	paths, err := filepath.Glob(filepath.Join(root, "artifacts", "agents", "proof-runs", "*.json"))
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	records := make([]Record, 0, len(paths))
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var record Record
		if err := json.Unmarshal(data, &record); err != nil {
			return nil, fmt.Errorf("read proof-run record %s: %w", path, err)
		}
		if err := validateRecord(record); err != nil {
			return nil, fmt.Errorf("read proof-run record %s: %w", path, err)
		}
		name := filepath.Base(path)
		if record.Root != root || name != record.Suite+".json" {
			return nil, fmt.Errorf("read proof-run record %s: record identity does not match its path", path)
		}
		records = append(records, record)
	}
	return records, nil
}

func writeRecord(record Record) error {
	if err := validateRecord(record); err != nil {
		return err
	}
	path, err := RecordPath(record.Root, record.Suite)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	_, err = atomicfile.WriteText(path, string(append(data, '\n')), record.Root)
	return err
}

func markDone(root, suite string, launcher identity.Ref) error {
	record, err := ReadRecord(root, suite)
	if err != nil {
		return err
	}
	if record.Launcher.Ref() != launcher {
		return errors.New("proof-run record now belongs to another launcher")
	}
	record.Status = StatusDone
	return writeRecord(record)
}

func validateRecord(record Record) error {
	if record.Suite == "" || record.Root == "" || record.FenceGeneration < 0 {
		return errors.New("proof-run record requires suite, root, and a non-negative fence generation")
	}
	if record.Status != StatusRunning && record.Status != StatusDone {
		return fmt.Errorf("proof-run status %q is invalid", record.Status)
	}
	for name, process := range map[string]ProcessIdentity{
		"launcher": record.Launcher, "suite": record.SuiteProcess, "watchdog": record.Watchdog,
	} {
		if process.Pid < 1 || process.Ref().Mode() == identity.CompareInvalid {
			return fmt.Errorf("proof-run %s identity is invalid", name)
		}
	}
	if record.SuiteProcess.Pgid < 1 {
		return errors.New("proof-run suite process group is invalid")
	}
	return nil
}
