package steward

import (
	"fmt"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

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
	if d.Action == ActNotify || d.Action == ActRevive {
		// A notify verdict IS the visibility the invariant promises:
		// it goes to the queue, keyed by its verdict so the standing
		// condition holds one pending message (redelivered after each
		// successful delivery, held durably through an outage). The
		// revive verdict queues too — the invariant says DEAD is
		// notified within one tick, and the revival's own gated
		// message must not be the only channel: a revival that fails
		// before minting would otherwise loop in silence forever.
		if err := QueueNotification(repoRoot, PendingNotification{
			Nonce:   "verdict-" + string(d.Verdict),
			Message: fmt.Sprintf("steward: %s — %s", d.Verdict, d.Reason),
		}); err != nil {
			return TickResult{}, err
		}
	}
	// The appetite covenant rides the tick: breaches are computed
	// from the synced ledger at read time (the projection banners
	// them on every machine), and the steward's duty is the belt —
	// the phone hears what every goal next already shows. A goal
	// world that cannot be read skips silently HERE because the
	// banners remain on the read path; the steward's own liveness
	// invariant must not degrade on a goal-read hiccup.
	if e, endpointErr := goal.ResolveEndpoint(repoRoot); endpointErr == nil {
		if proj, projErr := goal.Project(e, false, time.Now()); projErr == nil {
			for _, banner := range proj.Banners {
				if !strings.Contains(banner, "APPETITE BREACH") {
					continue
				}
				if err := QueueNotification(repoRoot, PendingNotification{
					Nonce:   "appetite-" + shortBannerKey(banner),
					Message: "steward covenant: " + banner,
				}); err != nil {
					return TickResult{}, err
				}
			}
		}
	}
	if err := SaveEvidence(evPath, ev); err != nil {
		return TickResult{}, err
	}
	result := TickResult{Decision: d, Evidence: ev, OpenWork: workReason, Reaped: reaped}
	// The running plain-English account rides every tick, strictly
	// best-effort: the storyteller never fails the shift. What the
	// narration notices also reaches the operator, one gated message
	// per building condition.
	Narrate(repoRoot, result, cfg)
	ReachTheHuman(repoRoot, noticings(result, cfg))
	return result, nil
}

// shortBannerKey keys one pending message per breached goal: the
// goal id sits between "BREACH: " and " claimed".
func shortBannerKey(banner string) string {
	rest := banner
	if i := strings.Index(rest, "BREACH: "); i >= 0 {
		rest = rest[i+len("BREACH: "):]
	}
	if i := strings.Index(rest, " "); i >= 0 {
		rest = rest[:i]
	}
	return rest
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
	work, workReason, err := ReadOpenWork(repoRoot)
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
