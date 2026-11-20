package steward

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/outage"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/run"
)

type scheduledProgress struct {
	collection bool
	seat       bool
	reason     string
}

func (p *scheduledProgress) hold(seat bool) string {
	if p == nil {
		return "waiting for this runner's first scheduled tick"
	}
	if !p.collection || seat && !p.seat {
		return "waiting for a scheduled tick to reach this decision: " + p.reason
	}
	return ""
}

// Completion requests use the seat and collection owners without entering a
// tick. In particular they never observe health or age the evidence store.
func runLightDecisions(root string, cfg TickConfig, census WorkerCensus) (TickResult, error) {
	cfg = cfg.withDefaults()
	work := defaultTickContinuationDependencies(cfg).openWork
	if cfg.Seat != nil {
		seat := lightSeatDependencies(cfg)
		work.Seat = &seat
	}
	return runLightDecisionsWith(root, cfg, census, work, ReapContinuations)
}

func runLightDecisionsWith(root string, cfg TickConfig, census WorkerCensus, work openWorkDependencies, collect func(string) ([]ReapReport, error)) (TickResult, error) {
	held, err := AcquireArbitration(root)
	if err != nil {
		return TickResult{}, err
	}
	defer held.Release()
	if state := helm.Active(root); state.Active {
		return TickResult{Decision: Decision{VerdictHelm, ActNone, "human at the helm: " + state.By}}, nil
	}
	if reason := cfg.scheduledProgress.hold(false); reason != "" {
		return TickResult{Decision: Decision{VerdictDegraded, ActHold, reason}}, nil
	}
	if err := readLightPreconditions(root, cfg.now()); err != nil {
		return TickResult{Decision: Decision{VerdictDegraded, ActHold, err.Error()}}, nil
	}
	reaped, err := collect(root)
	if err != nil {
		return TickResult{}, err
	}
	if cfg.Stopping != nil && cfg.Stopping() {
		return TickResult{Reaped: reaped}, nil
	}
	result, err := lightSeatDecision(root, cfg, census, work)
	result.Reaped = reaped
	return result, err
}

func lightSeatDependencies(cfg TickConfig) seatDependencies {
	dependencies := seatDependenciesForConfig(cfg)
	if cfg.Situation != nil {
		dependencies.Situation = func(root string, now time.Time) (SituationRecord, error) {
			situation := cfg.Situation(root, now)
			return SituationRecord{Situation: situation, Composition: Compose(situation)}, nil
		}
	}
	return dependencies
}

func lightSeatDecision(root string, cfg TickConfig, census WorkerCensus, dependencies openWorkDependencies) (TickResult, error) {
	result := TickResult{Decision: Decision{VerdictHealthy, ActNone, "no seat successor is due"}}
	if reason := cfg.scheduledProgress.hold(true); reason != "" {
		result.Decision = Decision{VerdictDegraded, ActHold, reason}
		return result, nil
	}
	if _, err := currentMarksWithReader(root, readMarksGit); err != nil {
		result.Decision = Decision{VerdictDegraded, ActHold, err.Error()}
		return result, nil
	}
	cfg = cfg.withDefaults()
	dependencies.Now = cfg.now
	if cfg.WorkStateRoot != "" {
		dependencies.Busy = seatBusyReader(cfg.WorkStateRoot, cfg.busyTick(), cfg.Prober)
	}
	// Read the existing busy/collection owner even when this checkout has no
	// seat launcher. It keeps fresh starts and uncertain custody held.
	var state seatTickState
	if dependencies.Seat != nil {
		state = reapSeatLaunches(root, *dependencies.Seat, cfg.now())
	}
	work, reason, shared, err := readOpenWorkShared(root, dependencies)
	result.OpenWork = reason
	if err != nil || dependencies.Seat == nil || shared == nil {
		return result, err
	}
	state.Steward = stewardshipWork(root, *dependencies.Seat, cfg.now())
	workers := Workers{}
	if work == WorkOwned || work == WorkClaimable || work == WorkWaiting || state.Steward != nil {
		workers, err = census.Workers(root)
		if err != nil {
			workers = Workers{Unprovable: 1}
		}
	}
	providerCheck := func(runtime string, err error) (outage.Mark, bool) {
		if err != nil {
			return providerOutageFrom(outage.Providers{}, err, runtime, cfg.now(), nil)
		}
		return standingProviderOutage(runtime, cfg.now(), nil, cfg.ProviderHome)
	}
	if decision, selection, ok := decideSeat(root, cfg, work, *shared, workers, providerCheck, *dependencies.Seat, state); ok {
		result.Decision, result.Seat = decision, selection
		if selection != nil {
			if reason := lightSeatBudgetHold(root, cfg, *selection, dependencies.Seat.Project); reason != "" {
				result.Decision, result.Seat = Decision{VerdictDegraded, ActHold, reason}, nil
			}
		}
	}
	return result, nil
}

func startLightSeat(root string, cfg TickConfig, census WorkerCensus, selection SeatSelection) (SeatRecord, error) {
	if reason := cfg.scheduledProgress.hold(true); reason != "" {
		return SeatRecord{}, fmt.Errorf("%s", reason)
	}
	dependencies := lightSeatDependencies(cfg)
	work := defaultTickContinuationDependencies(cfg).openWork
	work.Seat = &dependencies
	dependencies.Recheck = func(records []SeatRecord) (Decision, *SeatSelection, error) {
		if err := readLightPreconditions(root, cfg.now()); err != nil {
			return Decision{}, nil, err
		}
		if _, err := currentMarksWithReader(root, readMarksGit); err != nil {
			return Decision{}, nil, err
		}
		decision, current, err := seatRecheck(root, cfg, census, work, records)
		if err == nil && current != nil {
			if reason := lightSeatBudgetHold(root, cfg, *current, dependencies.Project); reason != "" {
				return Decision{VerdictDegraded, ActHold, reason}, nil, nil
			}
		}
		return decision, current, err
	}
	return startSeatWithDependencies(root, selection, dependencies)
}

// These reads detect changed inputs without repeating observations, debt
// repair, digest publication or counselor delivery. Failures of those acts
// remain held by the last scheduled tick's progress until a tick retries them.
func readLightPreconditions(root string, now time.Time) error {
	store := &run.Store{Root: root}
	for _, path := range run.RecordFiles(root) {
		if _, err := store.Read(strings.TrimSuffix(filepath.Base(path), ".json")); err != nil {
			return fmt.Errorf("governed-obligation record unreadable: %w", err)
		}
	}
	if _, err := loadValidationWindow(root); err != nil {
		return fmt.Errorf("direct-validation state unreadable: %w", err)
	}
	if _, _, err := readRulingReviewRegister(root); err != nil {
		return fmt.Errorf("ruling reviews unreadable: %w", err)
	}
	if _, err := loadRulingSweep(root); err != nil {
		return fmt.Errorf("ruling review state unreadable: %w", err)
	}
	if _, err := counselorBriefCadenceHours(root); err != nil {
		return fmt.Errorf("counselor cadence unreadable: %w", err)
	}
	cursor, err := loadCounselorBriefCursor(root)
	if err != nil {
		return fmt.Errorf("counselor brief cursor unreadable: %w", err)
	}
	if cursor.RecordedAt != "" {
		at, _ := time.Parse(time.RFC3339, cursor.RecordedAt)
		if now.UTC().Truncate(time.Second).Before(at) {
			return fmt.Errorf("counselor brief clock regressed behind its durable cursor")
		}
	}
	return nil
}

// Budget projection reads the same retained spending as the tick's breach
// scanner. Requests leave stopping and all its writes to the scheduled tick.
func lightSeatBudgetHold(root string, cfg TickConfig, selection SeatSelection, project func(string, time.Time) (goal.Projection, error)) string {
	if selection.Goal == "" && len(selection.SeatHeld) == 0 {
		return ""
	}
	projection, err := project(root, cfg.now())
	if err != nil {
		return "cannot read the goal before a light seat start: " + err.Error()
	}
	if projection.Tree == nil {
		return "the goal ledger has no tree"
	}
	ids := append([]string{selection.Goal}, selection.SeatHeld...)
	seen := map[string]bool{"": true}
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		file := projection.Tree.Live[id]
		if file == nil {
			return "the selected goal is no longer live: " + id
		}
		if file.StopFence != nil {
			return "the selected goal has a breach stop: " + id
		}
		if file.State != goal.StateClaimed {
			continue
		}
		budget := dispatch.ProjectBudget(root, file, cfg.now(), cfg.WorkStateRoot)
		if budget.Status != dispatch.BudgetKnown {
			return "the selected goal's budget is unreadable: " + id
		}
		if budget.ElapsedState == dispatch.AdmissionClosedElapsed || budget.ElapsedState == dispatch.ElapsedBreach || len(budget.Breaches) > 0 {
			return "the selected goal's budget is spent; waiting for the scheduled tick: " + id
		}
	}
	return ""
}
