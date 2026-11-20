package steward

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

// UnitBoundary is a completed unit's durable identity.
type UnitBoundary struct {
	Seat      string `json:"seat"`
	Session   string `json:"session"`
	Goal      string `json:"goal"`
	Unit      string `json:"unit"`
	Outcome   string `json:"outcome"`
	Handoff   string `json:"handoff,omitempty"`
	Signalled bool   `json:"signalled,omitempty"`
}

func ReadUnitBoundaries(root string) ([]UnitBoundary, error) {
	var events []UnitBoundary
	err := readJSON(filepath.Join(runnerDir(root), "unit-boundaries.json"), &events)
	if os.IsNotExist(err) {
		err = nil
	}
	if err != nil {
		return nil, err
	}
	var readable []UnitBoundary
	for _, event := range events {
		if event.Seat == canonicalPath(root) && (event.Session == "" || event.Goal == "" || event.Unit == "" || (!launch.UnitReviewReadyOutcomes[event.Outcome] && event.Outcome != "settled")) {
			err = errors.Join(err, fmt.Errorf("unit boundary record is incomplete"))
			continue
		}
		readable = append(readable, event)
	}
	return readable, err
}

func writeUnitBoundaries(root string, events []UnitBoundary) error {
	data, err := json.Marshal(events)
	if err != nil {
		return err
	}
	durable, err := atomicfile.WriteFile(filepath.Join(runnerDir(root), "unit-boundaries.json"), data, 0o600, root)
	if err == nil && !durable {
		err = fmt.Errorf("unit boundary persistence is uncertain; no session may end")
	}
	return err
}

// ObserveUnitBoundary runs under the same arbitration as handoff capture.
// Units supplies the current public work stages, including independent reads.
func ObserveUnitBoundary(root, home, session string, started time.Time, work goal.ClaimableBudgetedWork, units func(string, string) ([]UnitStage, error), now time.Time, prober identity.Prober) error {
	held, err := AcquireArbitration(root)
	if err != nil {
		return err
	}
	defer held.Release()
	if session == "" || started.IsZero() {
		return nil
	}
	ready, err := SeatAtUnitBoundary(root, home, now, prober, work)
	if err != nil || !ready {
		return err
	}
	questions, unreadable := channel.WalkOpenQuestions(root)
	if len(unreadable) > 0 {
		return fmt.Errorf("unit boundary questions are unreadable: %v", unreadable)
	}
	if len(questions) > 0 {
		return nil
	}
	events, err := ReadUnitBoundaries(root)
	if err != nil {
		return err
	}
	retained := len(events)
	complete, runs, err := heldWorkCompleted(root, home, work, units, prober)
	if err != nil || !complete {
		return err
	}
	for _, run := range runs {
		round := run.Rounds[len(run.Rounds)-1]
		finished, err := unitBoundaryCompletion(round)
		if err != nil {
			return fmt.Errorf("unit %s completion time is unknown: %w", run.ID, err)
		}
		if finished.Before(started) {
			continue
		}
		event := UnitBoundary{Seat: canonicalPath(root), Session: goal.NormalizeSession(session), Goal: run.Goal, Unit: fmt.Sprintf("%s/%d", run.ID, round.Number), Outcome: round.Outcome}
		found := false
		for _, previous := range events {
			if previous.Seat == event.Seat && previous.Session == event.Session && previous.Goal == event.Goal && previous.Unit == event.Unit {
				found = true
			}
		}
		if !found {
			events = append(events, event)
		}
	}
	if len(runs) == 0 {
		// An all-retired or hand-written goal has no completed builder run.
		// Record the observed boundary without inventing a successful read.
		for _, id := range append(append([]string(nil), work.Claimed...), work.Landing...) {
			stages, err := units(root, id)
			if err != nil {
				return err
			}
			for _, stage := range stages {
				if !unitBoundarySettled(stage.Stage) {
					return nil
				}
				event := UnitBoundary{Seat: canonicalPath(root), Session: goal.NormalizeSession(session), Goal: id, Unit: "unit:" + stage.Unit, Outcome: "settled"}
				found := false
				for _, prior := range events {
					if prior.Seat == event.Seat && prior.Session == event.Session && prior.Goal == event.Goal && prior.Unit == event.Unit {
						found = true
					}
				}
				if !found {
					events = append(events, event)
				}
			}
		}
	}
	if len(events) == retained {
		return nil
	}
	return writeUnitBoundaries(root, events)
}

func unitBoundaryCompletion(round launch.UnitRound) (time.Time, error) {
	steps := round.Steps
	if round.ReadCheckout == "" {
		// Serial rounds need only their final step's timestamp.
		return time.Parse(time.RFC3339Nano, steps[len(steps)-1].FinishedAt)
	}
	var latest time.Time
	for _, step := range steps {
		if step.State == launch.StepSkipped && step.FinishedAt == "" {
			continue
		}
		end, err := time.Parse(time.RFC3339Nano, step.FinishedAt)
		if err != nil {
			return time.Time{}, fmt.Errorf("%s: %w", step.Name, err)
		}
		if end.After(latest) {
			latest = end
		}
	}
	return latest, nil
}

// heldWorkCompleted requires published independent reads and terminal steps.
func heldWorkCompleted(root, home string, work goal.ClaimableBudgetedWork, units func(string, string) ([]UnitStage, error), prober identity.Prober) (bool, []launch.UnitRunRecord, error) {
	ready, err := SeatAtUnitBoundary(root, home, time.Time{}, prober, work)
	if err != nil || !ready {
		return false, nil, err
	}
	var completedRuns []launch.UnitRunRecord
	completed := map[string]bool{}
	settled := 0
	for _, id := range append(append([]string(nil), work.Claimed...), work.Landing...) {
		stages, err := units(root, id)
		if err != nil {
			return false, nil, err
		}
		for _, stage := range stages {
			if !unitBoundarySettled(stage.Stage) {
				return false, nil, nil
			}
			settled++
			if stage.Run != "" && (strings.HasPrefix(stage.Stage, "reviewed; its read is collected and published") || strings.HasPrefix(stage.Stage, "superseded by ")) {
				completed[id+"/"+stage.Run] = true
			}
		}
	}
	entries, err := os.ReadDir(filepath.Join(home, "unit"))
	if err != nil && !os.IsNotExist(err) {
		return false, nil, err
	}
	heldGoals := map[string]bool{}
	for _, id := range append(append([]string(nil), work.Claimed...), work.Landing...) {
		heldGoals[id] = true
	}
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		var run launch.UnitRunRecord
		selected, err := readHeldSeatRecord(filepath.Join(home, "unit", entry.Name(), "run.json"), "goal", heldGoals, &run)
		if err != nil {
			return false, nil, err
		}
		if !selected {
			continue
		}
		if !completed[run.Goal+"/"+run.ID] {
			continue
		}
		if run.ID != entry.Name() || run.State != "awaiting-judgement" || len(run.Rounds) == 0 {
			return false, nil, nil
		}
		round := run.Rounds[len(run.Rounds)-1]
		if !launch.UnitReviewReadyOutcomes[round.Outcome] || len(round.Steps) == 0 {
			return false, nil, nil
		}
		for _, step := range round.Steps {
			if step.State != launch.StepPassed && step.State != launch.StepFailed && step.State != launch.StepSkipped {
				return false, nil, nil
			}
		}
		delete(completed, run.Goal+"/"+run.ID)
		completedRuns = append(completedRuns, run)
	}
	if len(completed) > 0 {
		return false, nil, nil
	}
	return settled > 0, completedRuns, nil
}

// unitBoundarySettled consumes the public projection: cancelled work with a
// surviving commit is projected as committed work, and pending drops stay open.
// Quota accounting has a different contract: stopped work still consumes no
// active build slot, while it cannot authorize a completion boundary.
func unitBoundarySettled(stage string) bool {
	return stage == "committed, ready to land without a read" ||
		strings.HasPrefix(stage, "reviewed; its read is collected and published") ||
		strings.HasPrefix(stage, "superseded by ") ||
		stage == "cancelled; new work starts under another name" ||
		strings.HasPrefix(stage, "dropped; prior reads retained") ||
		stage == "transferred; its destinations remain required"
}
