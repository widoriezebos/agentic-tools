package steward

import "fmt"

// One tick: read the world, fold the evidence, decide, and put
// every notify verdict on the durable queue. The tick
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
	OpenWork string       // the open-work reason, for the report
	Reaped   []ReapReport // continuations this tick closed
}

// RunTick folds one observation into the persisted evidence and
// returns the decision. The evidence store is written back before
// returning, so a crash after the tick never replays its aging.
func RunTick(repoRoot string, cfg TickConfig, census WorkerCensus) (TickResult, error) {
	cfg = cfg.withDefaults()

	// One tick at a time per repository: the CLI seam and the
	// resident runner share the evidence store and the pending
	// queue, and neither may age or drain it under the other.
	tickLock, err := AcquireArbitration(repoRoot)
	if err != nil {
		return TickResult{}, err
	}
	defer tickLock.Release()

	// Close finished continuations first: the guard a reap frees must
	// not suppress this same tick's decision.
	reaped, err := ReapContinuations(repoRoot)
	if err != nil {
		return degradedTick(repoRoot, "reaping failed: "+err.Error())
	}

	evPath := EvidencePath(repoRoot)
	prev, err := LoadEvidence(evPath)
	if err != nil {
		// A torn store degrades honestly: report, do not guess ages.
		return degradedTick(repoRoot, err.Error())
	}
	marks, err := CurrentMarks(repoRoot)
	if err != nil {
		return degradedTick(repoRoot, err.Error())
	}
	ev := Observe(prev, marks)

	d, workReason, err := decideNow(repoRoot, cfg, census, ev)
	if err != nil {
		return TickResult{}, err
	}
	if d.Action == ActNotify {
		// A notify verdict IS the visibility the invariant promises:
		// it goes to the queue, keyed by its verdict so the standing
		// condition holds one pending message (redelivered after each
		// successful delivery, held durably through an outage).
		if err := QueueNotification(repoRoot, PendingNotification{
			Nonce:   "verdict-" + string(d.Verdict),
			Message: fmt.Sprintf("steward: %s — %s", d.Verdict, d.Reason),
		}); err != nil {
			return TickResult{}, err
		}
	}
	if err := SaveEvidence(evPath, ev); err != nil {
		return TickResult{}, err
	}
	return TickResult{Decision: d, Evidence: ev, OpenWork: workReason, Reaped: reaped}, nil
}

// degradedTick is every degraded early exit's one shape: the
// incident reaches the durable queue BEFORE the tick returns, so a
// broken store or unreadable repository is never a silent verdict.
func degradedTick(repoRoot, reason string) (TickResult, error) {
	d := Decision{VerdictDegraded, ActNotify, reason}
	if err := QueueNotification(repoRoot, PendingNotification{
		Nonce:   "verdict-" + string(VerdictDegraded),
		Message: fmt.Sprintf("steward: %s — %s", d.Verdict, d.Reason),
	}); err != nil {
		// Neither queued nor silent: the caller surfaces a tick that
		// could not even record its own degradation.
		return TickResult{Decision: d}, fmt.Errorf("degraded (%s) and the incident could not queue: %v", reason, err)
	}
	return TickResult{Decision: d}, nil
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
	activeConsumed, err := ConsumedActive(repoRoot)
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
		ActiveContinuation: len(live) > 0 || len(activeConsumed) > 0,
	}), workReason, nil
}
