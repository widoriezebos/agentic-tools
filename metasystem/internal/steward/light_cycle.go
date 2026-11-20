package steward

import (
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/outage"
)

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
	}
	return result, nil
}

func startLightSeat(root string, cfg TickConfig, census WorkerCensus, selection SeatSelection) (SeatRecord, error) {
	dependencies := lightSeatDependencies(cfg)
	work := defaultTickContinuationDependencies(cfg).openWork
	work.Seat = &dependencies
	dependencies.Recheck = func(records []SeatRecord) (Decision, *SeatSelection, error) {
		return seatRecheck(root, cfg, census, work, records)
	}
	return startSeatWithDependencies(root, selection, dependencies)
}
