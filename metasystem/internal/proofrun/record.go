package proofrun

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopfence"
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

// CurrentProcessIdentity records the exact identity of the calling process.
func CurrentProcessIdentity(prober identity.Prober) (ProcessIdentity, error) {
	if prober == nil {
		prober = identity.KernelProber{}
	}
	exact, state, err := prober.Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		return ProcessIdentity{}, fmt.Errorf("cannot record exact current process identity: %v (%s)", err, state)
	}
	pgid, err := syscall.Getpgid(os.Getpid())
	if err != nil {
		return ProcessIdentity{}, fmt.Errorf("read current process group: %w", err)
	}
	return processIdentity(exact, int64(pgid)), nil
}

// ProcessIdentityForPID records one exact live process identity.
func ProcessIdentityForPID(pid int64, prober identity.Prober) (ProcessIdentity, error) {
	if pid < 1 {
		return ProcessIdentity{}, fmt.Errorf("process identifier must be positive")
	}
	if prober == nil {
		prober = identity.KernelProber{}
	}
	exact, state, err := prober.Probe(pid)
	if err != nil || state != identity.Alive {
		return ProcessIdentity{}, fmt.Errorf("cannot record exact process identity for pid %d: %v (%s)", pid, err, state)
	}
	pgid, err := syscall.Getpgid(int(pid))
	if err != nil {
		return ProcessIdentity{}, err
	}
	return processIdentity(exact, int64(pgid)), nil
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
	ControlRoot     string          `json:"controlRoot,omitempty"`
	AttemptID       string          `json:"attemptId,omitempty"`
	LaunchID        string          `json:"launchId,omitempty"`
	FenceGeneration int64           `json:"fenceGeneration"`
	Launcher        ProcessIdentity `json:"launcher"`
	SuiteProcess    ProcessIdentity `json:"suiteProcess"`
	Watchdog        ProcessIdentity `json:"watchdog"`
	Status          string          `json:"status"`
}

// Key is unique for retained attempt records and preserves the legacy suite
// key for old records.
func (record Record) Key() string {
	if record.AttemptID == "" {
		return record.Suite
	}
	return record.AttemptID + "-" + record.LaunchID
}

// RecordPath returns the durable record path for a suite.
func RecordPath(root, suite string) (string, error) {
	if suite == "" || suite == "." || suite == ".." || filepath.Base(suite) != suite || strings.ContainsAny(suite, `/\`) {
		return "", errors.New("proof-run suite must be one path-safe name")
	}
	return filepath.Join(root, "artifacts", "agents", "proof-runs", suite+".json"), nil
}

func ProcessRecordPath(root, attemptID, launchID string) (string, error) {
	if !safeAttemptID(attemptID) || !safeAttemptID(launchID) {
		return "", errors.New("proof-run attempt and launch identifiers must be path-safe")
	}
	return filepath.Join(root, "artifacts", "agents", "proof-runs", "processes", attemptID+"-"+launchID+".json"), nil
}

func recordPath(record Record) (string, error) {
	if record.AttemptID == "" {
		return RecordPath(record.Root, record.Suite)
	}
	return ProcessRecordPath(record.ControlRoot, record.AttemptID, record.LaunchID)
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

// ReadProcessRecord reads one attempt-scoped process record by its durable key.
func ReadProcessRecord(root, key string) (Record, error) {
	paths, err := filepath.Glob(filepath.Join(root, "artifacts", "agents", "proof-runs", "processes", "*.json"))
	if err != nil {
		return Record{}, err
	}
	for _, path := range paths {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return Record{}, readErr
		}
		var record Record
		if err := json.Unmarshal(data, &record); err != nil {
			return Record{}, fmt.Errorf("read proof-run record %s: %w", path, err)
		}
		if record.Key() != key {
			continue
		}
		if err := validateRecord(record); err != nil {
			return Record{}, fmt.Errorf("read proof-run record %s: %w", path, err)
		}
		if expected, _ := recordPath(record); expected != path || record.ControlRoot != root {
			return Record{}, fmt.Errorf("read proof-run record %s: record identity does not match its path", path)
		}
		return record, nil
	}
	return Record{}, os.ErrNotExist
}

// AuthenticateWorker proves that a suite process is inside either an admitted
// attempt or the legacy launcher's live process/creation record. Ambient
// progress flags and locators are never authority by themselves.
func AuthenticateWorker(controlRoot, attemptID, recordKey, claimPath string, callerPID int64) error {
	if attemptID != "" {
		_, err := AuthenticateContext(controlRoot, attemptID, callerPID)
		return err
	}
	if recordKey != "" {
		record, err := ReadRecord(controlRoot, recordKey)
		if err == nil && record.Status == StatusRunning && lineageContains(callerPID, record.SuiteProcess.Ref()) {
			return nil
		}
	}
	if claimPath != "" {
		claims, problems, err := stopfence.InspectClaims(controlRoot, int64(^uint64(0)>>1))
		if err != nil {
			return err
		}
		for _, problem := range problems {
			if filepath.Clean(problem.Path) == filepath.Clean(claimPath) {
				return problem.Err
			}
		}
		for _, claim := range claims {
			if filepath.Clean(claim.Path) == filepath.Clean(claimPath) && claim.Verb == "proof-run-launch" &&
				lineageContains(callerPID, claim.Creator.Ref()) {
				return nil
			}
		}
	}
	return errors.New("caller is not inside an authenticated proof worker")
}

// ReadRecords reads the proof-run inventory in stable suite-name order.
func ReadRecords(root string) ([]Record, error) {
	paths, err := filepath.Glob(filepath.Join(root, "artifacts", "agents", "proof-runs", "*.json"))
	if err != nil {
		return nil, err
	}
	attemptPaths, err := filepath.Glob(filepath.Join(root, "artifacts", "agents", "proof-runs", "processes", "*.json"))
	if err != nil {
		return nil, err
	}
	paths = append(paths, attemptPaths...)
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
		expected, pathErr := recordPath(record)
		if pathErr != nil || expected != path || record.ControlRoot != "" && record.ControlRoot != root || record.ControlRoot == "" && record.Root != root {
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
	path, err := recordPath(record)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	anchor := record.Root
	if record.ControlRoot != "" {
		anchor = record.ControlRoot
	}
	_, err = atomicfile.WriteText(path, string(append(data, '\n')), anchor)
	return err
}

func markDone(root string, expected Record, launcher identity.Ref) error {
	var record Record
	var err error
	if expected.AttemptID == "" {
		record, err = ReadRecord(root, expected.Suite)
	} else {
		record, err = ReadProcessRecord(root, expected.Key())
	}
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
	if record.AttemptID != "" {
		if !safeAttemptID(record.AttemptID) || !safeAttemptID(record.LaunchID) || record.ControlRoot == "" {
			return errors.New("attempt-scoped proof-run record requires path-safe attempt and launch identifiers and a control root")
		}
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
