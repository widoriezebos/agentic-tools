package steward

// One tick: read the world, fold the evidence, decide. The tick
// itself performs no action — the verb that calls it notifies,
// revives, or stays quiet per the decision, so every rule stays
// testable without a scheduler or a dispatcher.

// WorkerCensus answers the liveness question for this repository's
// workers: enrolled sessions, their delegate jobs, live gates,
// mission runners, monitored runs.
type WorkerCensus interface {
	Workers(repoRoot string) (Workers, error)
}

// TickConfig carries the thresholds; zero values take the defaults.
type TickConfig struct {
	StaleTicks  int
	MaxRevivals int
}

func (c TickConfig) withDefaults() TickConfig {
	if c.StaleTicks <= 0 {
		c.StaleTicks = 5
	}
	if c.MaxRevivals <= 0 {
		c.MaxRevivals = 3
	}
	return c
}

// TickResult is everything the calling verb needs to act and report.
type TickResult struct {
	Decision Decision
	Evidence Evidence
	OpenWork string // the open-work reason, for the report
}

// RunTick folds one observation into the persisted evidence and
// returns the decision. The evidence store is written back before
// returning, so a crash after the tick never replays its aging.
func RunTick(repoRoot string, cfg TickConfig, census WorkerCensus) (TickResult, error) {
	cfg = cfg.withDefaults()

	evPath := EvidencePath(repoRoot)
	prev, err := LoadEvidence(evPath)
	if err != nil {
		// A torn store degrades honestly: report, do not guess ages.
		return TickResult{Decision: Decision{VerdictDegraded, ActNotify, err.Error()}}, nil
	}
	marks, err := CurrentMarks(repoRoot)
	if err != nil {
		return TickResult{}, err
	}
	ev := Observe(prev, marks)

	d, workReason, err := decideNow(repoRoot, cfg, census, ev)
	if err != nil {
		return TickResult{}, err
	}
	if err := SaveEvidence(evPath, ev); err != nil {
		return TickResult{}, err
	}
	return TickResult{Decision: d, Evidence: ev, OpenWork: workReason}, nil
}

// decideNow assembles one snapshot over the given evidence and
// decides — without aging or persisting anything. The tick ages
// first and calls this; revive's re-arbitration calls it directly,
// so a re-check can never advance the clock it is checking.
func decideNow(repoRoot string, cfg TickConfig, census WorkerCensus, ev Evidence) (Decision, string, error) {
	cfg = cfg.withDefaults()
	work, workReason, err := LegacyOpenWork(repoRoot)
	if err != nil {
		return Decision{}, "", err
	}

	workers := Workers{}
	if work == WorkOwned {
		w, err := census.Workers(repoRoot)
		if err != nil {
			// An unreadable census can never prove death.
			w = Workers{Unprovable: 1}
		}
		workers = w
	}

	live, err := LiveIntents(repoRoot)
	if err != nil {
		return Decision{VerdictDegraded, ActNotify, err.Error()}, workReason, nil
	}

	return Decide(Snapshot{
		Work:               work,
		Workers:            workers,
		TicksSinceProgress: ev.TicksSinceAdvance,
		StaleTicks:         cfg.StaleTicks,
		DryRevivals:        ev.DryRevivals,
		MaxRevivals:        cfg.MaxRevivals,
		ActiveContinuation: len(live) > 0,
	}), workReason, nil
}
