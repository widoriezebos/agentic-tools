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
// Unknown dependent custody prevents automatic idle action.
func SeatBusy(root string, work goal.ClaimableBudgetedWork) (string, error) {
	resolved, err := goal.ResolveStateRoot(root)
	if err != nil {
		return "", err
	}
	home, err := HomeStateRoot()
	if err != nil {
		return "", err
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
	Probe func(identity.Ref) identity.Liveness
	// AtBoundary is retained for boundary callers; all callers hold live steps.
	AtBoundary bool
}

// SeatBusyAt decodes only records whose goal header names a held goal.
// The unit and launch stores share the explicitly selected host state root.
func SeatBusyAt(root, units string, work goal.ClaimableBudgetedWork, options SeatBusyOptions) (busy bool, reason string, skipped int) {
	if options.Probe == nil {
		options.Probe = func(ref identity.Ref) identity.Liveness {
			if options.Alive != nil {
				if options.Alive(ref) {
					return identity.Alive
				}
				return identity.Dead
			}
			return identity.AliveRef(identity.KernelProber{}, ref)
		}
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
		busy, reason = true, "dependent unit inventory is unknown"
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
			if selected {
				busy, reason = true, "dependent work custody is unknown"
			}
			continue
		}
		if !selected || run.State != "running" {
			continue
		}
		if _, err := store.StateDir(entry.Name()); err != nil || run.ID != entry.Name() {
			skipped++
			busy, reason = true, "dependent unit custody is unknown"
			continue
		}
		step, _ := currentSeatRunStep(run)
		if step == nil {
			continue
		}
		if step.State == launch.StepStarting {
			busy, reason = true, fmt.Sprintf("unit %s step %s for goal %s is starting", run.ID, step.Name, run.Goal)
			continue
		}
		launchRecord, err := store.Read(step.LaunchID)
		if err != nil || launchRecord.State == launch.Running && (launchRecord.Supervisor == nil || options.Probe(*launchRecord.Supervisor) == identity.Unknown) {
			skipped++
			busy, reason = true, "dependent launch custody is unknown"
			continue
		}
		live := launchRecord.State == launch.Starting || launchRecord.State == launch.Running && options.Probe(*launchRecord.Supervisor) == identity.Alive
		if live {
			busy, reason = true, fmt.Sprintf("unit %s step %s for goal %s is running", run.ID, step.Name, run.Goal)
		} else if err := logOrphanSeatRun(root, fmt.Sprintf("orphan run %s of goal %s (step %s since %s)", run.ID, run.Goal, step.Name, step.StartedAt), options.Now); err != nil {
			skipped++
		}
	}
	jobs := filepath.Join(root, "artifacts", "agents", "jobs")
	entries, err = os.ReadDir(jobs)
	if err != nil && !os.IsNotExist(err) {
		skipped++
		busy, reason = true, "dependent job inventory is unknown"
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(jobs, entry.Name())
		var job struct {
			Status, GoalID                                           string
			Pid, PidStartedAt, PidStartedAtExactMicro, PidStartTicks int64
			BootID                                                   string
		}
		selected, err := readHeldSeatRecord(path, "goalId", held, &job)
		if err != nil {
			skipped++
			if selected {
				busy, reason = true, "dependent job custody is unknown"
			}
			continue
		}
		ref := identity.Ref{Pid: job.Pid, StartedAtSec: job.PidStartedAt, StartedAtUnixMicro: job.PidStartedAtExactMicro, StartTicks: job.PidStartTicks, BootID: job.BootID}
		if selected && job.Status == "pending" {
			busy, reason = true, fmt.Sprintf("dependent job %s is starting", filepath.Base(path))
		}
		if selected && job.Status == "running" {
			switch options.Probe(ref) {
			case identity.Alive:
				busy, reason = true, fmt.Sprintf("dependent job %s is running", filepath.Base(path))
			case identity.Unknown:
				skipped++
				busy, reason = true, "dependent job custody is unknown"
			}
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
			if step.State == launch.StepRunning || step.State == launch.StepStarting {
				copy := step
				current = &copy
			}
		}
	}
	return current, first
}

func readHeldSeatRecord(path, goalField string, held map[string]bool, record any, ancestry ...map[string]bool) (bool, error) {
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
			selected := held[id]
			if !selected && (goalField != "goalId" || id != "") {
				return false, nil
			}
			if !selected {
				seen := map[string]bool{}
				if len(ancestry) > 0 {
					seen = ancestry[0]
				}
				if seen[path] {
					return false, fmt.Errorf("job ancestry has a cycle")
				}
				seen[path] = true
				if _, err := file.Seek(0, io.SeekStart); err != nil {
					return false, err
				}
				var job struct{ ParentJob string }
				if err := json.NewDecoder(file).Decode(&job); err != nil {
					return false, err
				}
				if job.ParentJob == "" {
					return false, nil
				}
				if filepath.Base(job.ParentJob) != job.ParentJob || strings.ContainsAny(job.ParentJob, "/\\") {
					return false, fmt.Errorf("job ancestry is invalid")
				}
				var parent any
				var err error
				selected, err = readHeldSeatRecord(filepath.Join(filepath.Dir(path), job.ParentJob+".json"), goalField, held, &parent, seen)
				if err != nil || !selected {
					return selected, err
				}
			}
			if _, err := file.Seek(0, io.SeekStart); err != nil {
				return selected, err
			}
			decoder = json.NewDecoder(file)
			if err := decoder.Decode(record); err != nil {
				return selected, err
			}
			if err := decoder.Decode(new(any)); err != io.EOF {
				return selected, fmt.Errorf("record has trailing data")
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
