package deploy

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// The kinds and outcomes of a record line.
const (
	KindDeploy   = "deploy"
	KindRollback = "rollback"

	OutcomeActive         = "active"
	OutcomeBuildFailed    = "build-failed"
	OutcomeActivateFailed = "activate-failed"
	OutcomeVerifyFailed   = "verify-failed"
	// OutcomeVersionFailed ends a run or an act whose first version call
	// could not tell what is active, so nothing else was asked.
	OutcomeVersionFailed = "version-failed"
	OutcomeStopped       = "stopped"
	// OutcomeNone is version's answer that nothing is active, recorded
	// when the record named a current deploy: it is current no more.
	OutcomeNone = "none"
)

// Line is one finished attempt in deploys.jsonl. The current deploy is the
// newest line whose outcome is active, unless a newer none line says
// nothing is active; there is no second "current" file.
type Line struct {
	Kind      string    `json:"kind"`
	Commit    string    `json:"commit"`
	Version   string    `json:"version,omitempty"`
	Artifact  string    `json:"artifact,omitempty"`
	Digest    string    `json:"digest,omitempty"`
	Previous  CommitRef `json:"previous"`
	By        string    `json:"by"`
	StartedAt time.Time `json:"startedAt"`
	EndedAt   time.Time `json:"endedAt"`
	Outcome   string    `json:"outcome"`
	Detail    string    `json:"detail,omitempty"`
	Log       string    `json:"log,omitempty"`
}

// CommitRef is a commit that may be absent: null in the record before the
// first deploy.
type CommitRef string

func (c CommitRef) MarshalJSON() ([]byte, error) {
	if c == "" {
		return []byte("null"), nil
	}
	return json.Marshal(string(c))
}

func (c *CommitRef) UnmarshalJSON(data []byte) error {
	var value *string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*c = ""
	if value != nil {
		*c = CommitRef(*value)
	}
	return nil
}

// Pause holds every deploy of the project until a person resumes.
type Pause struct {
	By     string    `json:"by"`
	At     time.Time `json:"at"`
	Reason string    `json:"reason,omitempty"`
}

// Active is the adapter process a run has in flight, kept in run.json while
// it runs so status can show it and pause can end its process group. Each
// process is named by its id and its birth time from the kernel, so a
// process id the kernel has given to another process is never taken for it.
type Active struct {
	Runner     int       `json:"runner"`
	RunnerBorn time.Time `json:"runnerBorn"`
	Kind       string    `json:"kind"`
	Operation  string    `json:"operation"`
	Commit     string    `json:"commit"`
	PID        int       `json:"pid"`
	Born       time.Time `json:"born"`
	StartedAt  time.Time `json:"startedAt"`
	Log        string    `json:"log"`
}

// AdapterAlive says whether the adapter run.json names still runs.
func (a Active) AdapterAlive() bool { return sameProcess(a.PID, a.Born) }

func sameProcess(pid int, born time.Time) bool {
	if pid <= 0 || born.IsZero() {
		return false
	}
	now, ok := identity.ProcessBirth(int64(pid))
	return ok && now.Equal(born)
}

func recordPath(dir string) string { return filepath.Join(dir, "deploys.jsonl") }
func lockPath(dir string) string   { return filepath.Join(dir, "lock") }
func pausePath(dir string) string  { return filepath.Join(dir, "pause.json") }
func activePath(dir string) string { return filepath.Join(dir, "run.json") }

// pendingPath holds the line of an attempt from its activation until the
// line is written, so the next run can record it if this one dies between.
func pendingPath(dir string) string { return filepath.Join(dir, "pending.json") }
func workDir(dir string) string     { return filepath.Join(dir, "work") }
func logDir(dir string) string      { return filepath.Join(dir, "logs") }

// Lines reads the record, oldest first; a project never deployed has none.
func Lines(dir string) ([]Line, error) {
	file, err := os.Open(recordPath(dir))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var lines []Line
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		if len(scanner.Bytes()) == 0 {
			continue
		}
		var line Line
		if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
			return nil, fmt.Errorf("%s holds a line that is not a deploy: %w", recordPath(dir), err)
		}
		lines = append(lines, line)
	}
	return lines, scanner.Err()
}

// Current is the newest active line, or nil before the first deploy and
// while the newest line that is not a failure says nothing is active.
func Current(lines []Line) *Line {
	if state := newestState(lines); state != nil && state.Outcome == OutcomeActive {
		return state
	}
	return nil
}

// newestState is the newest line that says what is active: active, or none.
func newestState(lines []Line) *Line {
	for index := len(lines) - 1; index >= 0; index-- {
		if !lines[index].Failed() {
			line := lines[index]
			return &line
		}
	}
	return nil
}

// Failed says whether the line is an attempt that did not end active.
func (l Line) Failed() bool { return l.Outcome != OutcomeActive && l.Outcome != OutcomeNone }

// appendLine adds one line; the caller holds the project's lock.
func appendLine(dir string, line Line) error {
	data, err := json.Marshal(line)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(recordPath(dir), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.Write(append(data, '\n')); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

// ReadPause reads pause.json; ok is false while deploys are not paused.
func ReadPause(dir string) (Pause, bool, error) {
	var pause Pause
	ok, err := readJSON(pausePath(dir), &pause)
	return pause, ok, err
}

func writePause(dir string, pause Pause) error {
	return writeJSON(pausePath(dir), pause)
}

// ReadActive reads run.json: the adapter a run has in flight, if any.
func ReadActive(dir string) (Active, bool, error) {
	var active Active
	ok, err := readJSON(activePath(dir), &active)
	return active, ok, err
}

func readJSON(path string, target any) (bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := json.Unmarshal(data, target); err != nil {
		return false, fmt.Errorf("%s is not readable: %w", path, err)
	}
	return true, nil
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	_, err = atomicfile.WriteFile(path, append(data, '\n'), 0o600, filepath.Dir(path))
	return err
}

func removeFile(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
