package deploy

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// These outcomes name the lane's launch and a run that ended in an error.
const (
	AutomaticLaunched = "launched"
	RunError          = "error"
)

// Automatic records a launch or the outcome of the lane's bound run.
type Automatic struct {
	Commit  string    `json:"commit"`
	Outcome string    `json:"outcome"`
	Reason  string    `json:"reason,omitempty"`
	At      time.Time `json:"at"`
}

// Settles says whether this outcome ends its commit's automatic deploy.
func (a Automatic) Settles() bool {
	return a.Outcome != AutomaticLaunched && a.Outcome != RunBusy && a.Outcome != RunPaused && a.Outcome != RunError
}

func automaticPath(dir string) string { return filepath.Join(dir, "automatic.jsonl") }

// AppendAutomatic records one launch or outcome, creating its directory.
func AppendAutomatic(dir string, line Automatic) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return appendTo(automaticPath(dir), line)
}

// AutomaticLines reads every outcome, oldest first; damage is an error.
func AutomaticLines(dir string) (lines []Automatic, err error) {
	path := automaticPath(dir)
	defer func() {
		if err != nil {
			err = fmt.Errorf("%s can't be read: %w", path, err)
		}
	}()
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		var line Automatic
		if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
			return nil, err
		}
		lines = append(lines, line)
	}
	return lines, scanner.Err()
}

// AutomaticState keeps the newest settling outcome and the newest line.
type AutomaticState struct {
	Settled *Automatic
	Last    Automatic
}

// AutomaticStates folds lines in record order, independently per commit.
func AutomaticStates(lines []Automatic) map[string]AutomaticState {
	states := make(map[string]AutomaticState)
	for _, line := range lines {
		state := states[line.Commit]
		state.Last = line
		if line.Settles() {
			state.Settled = &line
		}
		states[line.Commit] = state
	}
	return states
}
