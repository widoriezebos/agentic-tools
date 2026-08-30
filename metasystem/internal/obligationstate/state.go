// Package obligationstate owns durable execution state for governed
// obligations. Run records are evidence and may be pruned; this record is the
// non-prunable owner of terminal attempt spend, breaker state, and run-ID use.
package obligationstate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
)

const Schema = 1

var goalIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

// TerminalAttempt is the durable accounting half of a governed run. PrunedAt
// is written before deliberate run-record deletion; absence without that
// marker is therefore evidence loss and must fail closed at reconstruction.
type TerminalAttempt struct {
	RunID                string  `json:"runId"`
	Status               string  `json:"status"`
	StartedAt            string  `json:"startedAt"`
	EndedAt              string  `json:"endedAt"`
	AttemptOrdinal       uint64  `json:"attemptOrdinal"`
	ExecutionCostMinutes uint64  `json:"executionCostMinutes"`
	ObservedCostMinutes  uint64  `json:"observedCostMinutes"`
	WeightGeneration     uint64  `json:"weightGeneration"`
	BudgetEpoch          *uint64 `json:"budgetEpoch,omitempty"`
	Breaker              string  `json:"breaker"`
	Exhausted            bool    `json:"exhausted"`
	ExhaustionReason     string  `json:"exhaustionReason,omitempty"`
	RetroDebtRaised      bool    `json:"retroDebtRaised,omitempty"`
	PrunedAt             string  `json:"prunedAt,omitempty"`
}

type State struct {
	Schema             int               `json:"schema"`
	Generation         uint64            `json:"generation"`
	GoalID             string            `json:"goalId"`
	GoalRevision       uint64            `json:"goalRevision"`
	ObligationRevision uint64            `json:"obligationRevision"`
	Attempts           []TerminalAttempt `json:"terminalAttempts"`
}

func Dir(root string) string {
	return filepath.Join(root, "artifacts", "agents", "governed-obligations")
}

func Path(root, goalID string, goalRevision, obligationRevision uint64) string {
	name := fmt.Sprintf("%s.g%d.o%d.json", goalID, goalRevision, obligationRevision)
	return filepath.Join(Dir(root), name)
}

func logicalPath(root, path string) string {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(relative)
}

func validateTerminal(attempt TerminalAttempt) error {
	if !goalIDPattern.MatchString(attempt.RunID) || attempt.AttemptOrdinal == 0 ||
		attempt.ExecutionCostMinutes == 0 || attempt.ObservedCostMinutes == 0 {
		return fmt.Errorf("terminal attempt identity, ordinal, and costs must be complete")
	}
	switch attempt.Status {
	case "green", "red", "ended-unknown", "launch-failed":
	default:
		return fmt.Errorf("terminal attempt status %q is not terminal", attempt.Status)
	}
	started, err := time.Parse(time.RFC3339, attempt.StartedAt)
	if err != nil {
		return fmt.Errorf("terminal attempt startedAt is invalid")
	}
	ended, err := time.Parse(time.RFC3339, attempt.EndedAt)
	if err != nil || ended.Before(started) {
		return fmt.Errorf("terminal attempt endedAt is invalid")
	}
	switch attempt.Breaker {
	case "CLOSED", "EXHAUSTED", "ASSUMPTION_FAILED":
	default:
		return fmt.Errorf("terminal attempt breaker %q is invalid", attempt.Breaker)
	}
	if attempt.Exhausted != (attempt.Breaker == "EXHAUSTED") {
		return fmt.Errorf("terminal attempt exhaustion contradicts breaker %q", attempt.Breaker)
	}
	if attempt.BudgetEpoch != nil && attempt.WeightGeneration <= *attempt.BudgetEpoch {
		return fmt.Errorf("terminal attempt weight generation does not follow its budget epoch")
	}
	if attempt.PrunedAt != "" {
		pruned, err := time.Parse(time.RFC3339, attempt.PrunedAt)
		if err != nil || pruned.Before(ended) {
			return fmt.Errorf("terminal attempt prunedAt is invalid")
		}
	}
	return nil
}

func validate(state State) error {
	if state.Schema != Schema || !goalIDPattern.MatchString(state.GoalID) ||
		state.GoalRevision == 0 || state.ObligationRevision == 0 {
		return fmt.Errorf("obligation execution record identity is incomplete")
	}
	seenRuns := map[string]bool{}
	seenOrdinals := map[string]bool{}
	for _, attempt := range state.Attempts {
		if err := validateTerminal(attempt); err != nil {
			return fmt.Errorf("run %q: %w", attempt.RunID, err)
		}
		epoch := "initial"
		if attempt.BudgetEpoch != nil {
			epoch = fmt.Sprintf("weight-%d", *attempt.BudgetEpoch)
		}
		ordinal := fmt.Sprintf("%s:%d", epoch, attempt.AttemptOrdinal)
		if seenRuns[attempt.RunID] || seenOrdinals[ordinal] {
			return fmt.Errorf("obligation execution record repeats a run or attempt ordinal")
		}
		seenRuns[attempt.RunID] = true
		seenOrdinals[ordinal] = true
	}
	return nil
}

func loadPath(path string) (State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return State{}, err
	}
	var state State
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&state); err != nil {
		return State{}, fmt.Errorf("malformed obligation execution record: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return State{}, fmt.Errorf("malformed obligation execution record: trailing JSON")
	}
	if err := validate(state); err != nil {
		return State{}, fmt.Errorf("malformed obligation execution record: %w", err)
	}
	return state, nil
}

func Load(root, goalID string, goalRevision, obligationRevision uint64) (State, bool, error) {
	path := Path(root, goalID, goalRevision, obligationRevision)
	state, err := loadPath(path)
	if os.IsNotExist(err) {
		return State{}, false, nil
	}
	if err != nil {
		return State{}, false, err
	}
	if state.GoalID != goalID || state.GoalRevision != goalRevision || state.ObligationRevision != obligationRevision {
		return State{}, false, fmt.Errorf("obligation execution record identity contradicts %s", logicalPath(root, path))
	}
	return state, true, nil
}

// LoadGoal returns every durable obligation record for a goal. Every matching
// filename is parsed before revision filtering so a corrupt authoritative
// record can never disappear from reconstruction.
func LoadGoal(root, goalID string) ([]State, error) {
	paths, err := filepath.Glob(filepath.Join(Dir(root), goalID+".g*.o*.json"))
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	states := make([]State, 0, len(paths))
	for _, path := range paths {
		state, err := loadPath(path)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", logicalPath(root, path), err)
		}
		if state.GoalID != goalID || Path(root, state.GoalID, state.GoalRevision, state.ObligationRevision) != path {
			return nil, fmt.Errorf("%s: record identity contradicts its path", logicalPath(root, path))
		}
		states = append(states, state)
	}
	return states, nil
}

func save(root string, state State) error {
	if err := validate(state); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	durable, err := atomicfile.WriteText(Path(root, state.GoalID, state.GoalRevision, state.ObligationRevision), string(data)+"\n", root)
	if err != nil {
		return err
	}
	if !durable {
		return fmt.Errorf("obligation execution record publication has unknown durability")
	}
	return nil
}

type stateLock struct{ file *os.File }

func acquire(root string) (*stateLock, error) {
	if err := os.MkdirAll(Dir(root), 0o755); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(filepath.Join(Dir(root), ".lock"), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	for {
		err = unix.Flock(int(file.Fd()), unix.LOCK_EX)
		if err != unix.EINTR {
			break
		}
	}
	if err != nil {
		file.Close()
		return nil, err
	}
	return &stateLock{file: file}, nil
}

func (lock *stateLock) release() {
	_ = unix.Flock(int(lock.file.Fd()), unix.LOCK_UN)
	_ = lock.file.Close()
}

// RecordTerminal commits the terminal spend before its prunable run evidence.
// Repeating the same transition is idempotent; conflicting reuse fails closed.
func RecordTerminal(root, goalID string, goalRevision, obligationRevision uint64, attempt TerminalAttempt) error {
	lock, err := acquire(root)
	if err != nil {
		return err
	}
	defer lock.release()
	state, found, err := Load(root, goalID, goalRevision, obligationRevision)
	if err != nil {
		return err
	}
	if !found {
		state = State{Schema: Schema, GoalID: goalID, GoalRevision: goalRevision, ObligationRevision: obligationRevision}
	}
	for _, existing := range state.Attempts {
		if existing.RunID != attempt.RunID {
			continue
		}
		if reflect.DeepEqual(existing, attempt) {
			return nil
		}
		return fmt.Errorf("governed run id %s already owns different terminal obligation state", attempt.RunID)
	}
	state.Generation++
	state.Attempts = append(state.Attempts, attempt)
	sort.Slice(state.Attempts, func(i, j int) bool { return state.Attempts[i].AttemptOrdinal < state.Attempts[j].AttemptOrdinal })
	return save(root, state)
}

func updateAttempt(root, goalID string, goalRevision, obligationRevision uint64, runID string, mutate func(*TerminalAttempt) error) error {
	lock, err := acquire(root)
	if err != nil {
		return err
	}
	defer lock.release()
	state, found, err := Load(root, goalID, goalRevision, obligationRevision)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("governed run %s has no durable obligation state", runID)
	}
	for index := range state.Attempts {
		if state.Attempts[index].RunID != runID {
			continue
		}
		before := state.Attempts[index]
		if err := mutate(&state.Attempts[index]); err != nil {
			return err
		}
		if reflect.DeepEqual(before, state.Attempts[index]) {
			return nil
		}
		state.Generation++
		return save(root, state)
	}
	return fmt.Errorf("governed run %s has no terminal attempt in its durable obligation state", runID)
}

func MarkRetroDebt(root, goalID string, goalRevision, obligationRevision uint64, runID string) error {
	return updateAttempt(root, goalID, goalRevision, obligationRevision, runID, func(attempt *TerminalAttempt) error {
		attempt.RetroDebtRaised = true
		return nil
	})
}

func MarkPruned(root, goalID string, goalRevision, obligationRevision uint64, runID string, now time.Time) error {
	return updateAttempt(root, goalID, goalRevision, obligationRevision, runID, func(attempt *TerminalAttempt) error {
		if attempt.PrunedAt == "" {
			attempt.PrunedAt = now.UTC().Format(time.RFC3339)
		}
		return nil
	})
}

// FindRun proves whether a run ID has ever terminalized as governed state.
// Duplicate claims or a malformed state are errors, never absence.
func FindRun(root, runID string) (*TerminalAttempt, string, error) {
	paths, err := filepath.Glob(filepath.Join(Dir(root), "*.json"))
	if err != nil {
		return nil, "", err
	}
	sort.Strings(paths)
	var found *TerminalAttempt
	foundPath := ""
	for _, path := range paths {
		state, err := loadPath(path)
		if err != nil {
			return nil, logicalPath(root, path), err
		}
		for index := range state.Attempts {
			if state.Attempts[index].RunID != runID {
				continue
			}
			if found != nil {
				return nil, logicalPath(root, path), fmt.Errorf("governed run id %s is claimed by multiple obligation records", runID)
			}
			copy := state.Attempts[index]
			found = &copy
			foundPath = logicalPath(root, path)
		}
	}
	return found, foundPath, nil
}

func RelativePath(root string, state State) string {
	return strings.TrimPrefix(logicalPath(root, Path(root, state.GoalID, state.GoalRevision, state.ObligationRevision)), "./")
}
