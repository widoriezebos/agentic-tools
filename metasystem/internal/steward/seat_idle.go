package steward

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
)

// ClassifySeatStop distinguishes missing hook identity from a conflicting one.
func ClassifySeatStop(mainID, holderID string) string {
	if mainID == "" {
		return "unknown"
	}
	if mainID != holderID {
		return "mismatch"
	}
	return "match"
}

// SeatBusy adapts the observation to the Stop hook's busy-only reason.
// Unreadable work records never prevent an idle refusal.
func SeatBusy(root string, work goal.ClaimableBudgetedWork) (string, error) {
	resolved, err := goal.ResolveStateRoot(root)
	if err != nil {
		return "", err
	}
	home, err := HomeStateRoot()
	if err != nil {
		return "", nil
	}
	busy, reason, _ := SeatBusyAt(resolved, filepath.Join(home, "unit"), work, SeatBusyOptions{Now: time.Now()})
	if busy {
		return reason, nil
	}
	return "", nil
}

type SeatBusyOptions struct {
	Now   time.Time
	Alive func(identity.Ref) bool
	// AtBoundary holds every live step, including one past its budget.
	AtBoundary bool
}

// SeatBusyAt decodes only records whose goal header names a held goal.
// The unit and launch stores share the explicitly selected host state root.
func SeatBusyAt(root, units string, work goal.ClaimableBudgetedWork, options SeatBusyOptions) (busy bool, reason string, skipped int) {
	if options.Alive == nil {
		options.Alive = func(ref identity.Ref) bool { return identity.AliveRef(identity.KernelProber{}, ref) == identity.Alive }
	}
	defer func() {
		if skipped > 0 {
			reason = strings.TrimPrefix(reason+fmt.Sprintf("; unreadable records: %d", skipped), "; ")
		}
	}()
	held := map[string]bool{}
	for _, id := range append(append([]string(nil), work.Claimed...), work.Landing...) {
		held[id] = true
	}
	if len(held) == 0 {
		return
	}
	entries, err := os.ReadDir(units)
	if err != nil && !os.IsNotExist(err) {
		skipped++
	}
	store := launch.Store{Root: filepath.Join(filepath.Dir(units), "launch")}
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		var run launch.UnitRunRecord
		selected, err := readHeldSeatRecord(filepath.Join(units, entry.Name(), "run.json"), "goal", held, &run)
		if err != nil {
			skipped++
			continue
		}
		if !selected || run.State != "running" {
			continue
		}
		if _, err := store.StateDir(entry.Name()); err != nil || run.ID != entry.Name() {
			skipped++
			continue
		}
		step, started := currentSeatRunStep(run)
		if step == nil {
			continue
		}
		file, ok := work.OwnedClaim(run.Goal)
		var limit time.Duration
		if ok && file.Budget != nil {
			limit, _ = goal.ParseWorkingDuration(file.Budget.ElapsedLimit)
		}
		launchRecord, err := store.Read(step.LaunchID)
		if err != nil {
			skipped++
		}
		live := err == nil && launchRecord.State == launch.Running && launchRecord.Supervisor != nil && options.Alive(*launchRecord.Supervisor)
		if live && (options.AtBoundary || limit > 0 && !started.IsZero() && !options.Now.Before(started) && options.Now.Sub(started) < limit) {
			busy, reason = true, fmt.Sprintf("unit %s step %s for goal %s is running", run.ID, step.Name, run.Goal)
		} else if err := logOrphanSeatRun(root, fmt.Sprintf("orphan run %s of goal %s (step %s since %s)", run.ID, run.Goal, step.Name, step.StartedAt), options.Now); err != nil {
			skipped++
		}
	}
	paths, err := filepath.Glob(filepath.Join(root, "artifacts", "agents", "jobs", "code-critic-*.json"))
	if err != nil {
		skipped++
	}
	for _, path := range paths {
		var job struct {
			Status, GoalID                                           string
			Pid, PidStartedAt, PidStartedAtExactMicro, PidStartTicks int64
			BootID                                                   string
		}
		selected, err := readHeldSeatRecord(path, "goalId", held, &job)
		if err != nil {
			skipped++
			continue
		}
		ref := identity.Ref{Pid: job.Pid, StartedAtSec: job.PidStartedAt, StartedAtUnixMicro: job.PidStartedAtExactMicro, StartTicks: job.PidStartTicks, BootID: job.BootID}
		if selected && job.Status == "running" && options.Alive(ref) {
			busy, reason = true, fmt.Sprintf("critic %s for goal %s is running", filepath.Base(path), job.GoalID)
		}
	}
	return
}

func currentSeatRunStep(run launch.UnitRunRecord) (*launch.UnitStep, time.Time) {
	var first time.Time
	var current *launch.UnitStep
	for _, round := range run.Rounds {
		current = nil
		for _, step := range round.Steps {
			at, _ := time.Parse(time.RFC3339, step.StartedAt)
			if !at.IsZero() && (first.IsZero() || at.Before(first)) {
				first = at
			}
			if step.State == launch.StepRunning {
				copy := step
				current = &copy
			}
		}
	}
	return current, first
}

func readHeldSeatRecord(path, goalField string, held map[string]bool, record any) (bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return false, fmt.Errorf("record has no goal header")
	}
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return false, err
		}
		if key == goalField {
			var id string
			if err := decoder.Decode(&id); err != nil {
				return false, err
			}
			if !held[id] {
				return false, nil
			}
			if _, err := file.Seek(0, io.SeekStart); err != nil {
				return false, err
			}
			decoder = json.NewDecoder(file)
			if err := decoder.Decode(record); err != nil {
				return false, err
			}
			if err := decoder.Decode(new(any)); err != io.EOF {
				return false, fmt.Errorf("record has trailing data")
			}
			return true, nil
		}
		var ignored json.RawMessage
		if err := decoder.Decode(&ignored); err != nil {
			return false, err
		}
	}
	return false, fmt.Errorf("record has no goal header")
}

// The existing log suppresses duplicate reports across ticks, Stop hooks and
// runner restarts. Its lock makes concurrent observations produce one line.
func logOrphanSeatRun(root, message string, now time.Time, keys ...string) error {
	key := message
	if len(keys) > 0 {
		key = keys[0]
	}
	path := runnerLogPath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	held, err := lock.File(path+".lock", 0o600, lock.Exclusive)
	if err != nil {
		return err
	}
	defer held.Release()
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	reader := bufio.NewReader(file)
	for {
		line, err := reader.ReadString('\n')
		if strings.HasSuffix(strings.TrimSuffix(line, "\n"), key) {
			return nil
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	_, err = fmt.Fprintf(file, "%s %s\n", now.In(time.Local).Format(time.RFC3339), message)
	return err
}
