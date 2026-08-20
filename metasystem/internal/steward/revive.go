package steward

// The revival, in the design's pinned order: mint the intent under
// the arbitration lock (recording the enrollment fence), deliver the
// operator notification — delivery gates everything after it — then
// re-take the lock for one critical section: fence check, full
// predicate re-run, intent consumption, launch, stamp. A crash
// between launch and stamp reconciles next tick as a notified
// unknown; a fence bump or a changed verdict cancels with the reason
// on record.

import "fmt"

// LaunchSeam performs the dispatch. The shell glue supplies the real
// dispatcher; fixtures supply an observable fake.
type LaunchSeam func(Intent) error

// ReviveOutcome says what happened, for the report and the receipt.
type ReviveOutcome struct {
	Launched bool
	Reason   string
}

// PrepareIntent mints the durable record under the lock, capturing
// the enrollment fence, and queues the operator notification. It
// performs no delivery and no launch.
func PrepareIntent(repoRoot string, it Intent) error {
	arb, err := AcquireArbitration(repoRoot)
	if err != nil {
		return err
	}
	defer arb.Release()
	fence, err := ReadEnrollmentFence(repoRoot)
	if err != nil {
		return err
	}
	it.FenceAtMint = fence
	if err := MintIntent(repoRoot, it); err != nil {
		return err
	}
	return QueueNotification(repoRoot, PendingNotification{
		Nonce:   it.Nonce,
		Message: fmt.Sprintf("steward: reviving %s (worker provably dead); job %s", it.Goal, it.JobId),
	})
}

// CompleteRevival runs the delivery gate and the critical section
// for a live intent. Safe to call again after a notifier outage —
// the intent survives until it launches or cancels.
func CompleteRevival(repoRoot string, cfg TickConfig, census WorkerCensus, nonce string, launch LaunchSeam) (ReviveOutcome, error) {
	intents, err := LiveIntents(repoRoot)
	if err != nil {
		return ReviveOutcome{}, err
	}
	var it *Intent
	for i := range intents {
		if intents[i].Nonce == nonce {
			it = &intents[i]
		}
	}
	if it == nil {
		return ReviveOutcome{Reason: "intent is not live (already consumed or cancelled)"}, nil
	}

	// The delivery gate: no launch before the operator heard.
	if !it.Notified {
		if err := Deliver(repoRoot, fmt.Sprintf("steward: reviving %s (worker provably dead); job %s", it.Goal, it.JobId)); err != nil {
			return ReviveOutcome{Reason: "notification not delivered; launch stays gated: " + err.Error()}, nil
		}
		_ = MarkDelivered(repoRoot, it.Nonce)
		it.Notified = true
		if err := UpdateIntent(repoRoot, *it); err != nil {
			return ReviveOutcome{}, err
		}
	}

	// The critical section: fence, verdict, consume, launch, stamp.
	arb, err := AcquireArbitration(repoRoot)
	if err != nil {
		return ReviveOutcome{}, err
	}
	defer arb.Release()

	fence, err := ReadEnrollmentFence(repoRoot)
	if err != nil {
		return ReviveOutcome{}, err
	}
	if fence != it.FenceAtMint {
		if _, err := ConsumeIntent(repoRoot, it.Nonce); err != nil {
			return ReviveOutcome{}, err
		}
		return ReviveOutcome{Reason: "a worker enrolled after the reservation; revival cancelled"}, nil
	}
	ev, err := LoadEvidence(EvidencePath(repoRoot))
	if err != nil {
		return ReviveOutcome{}, err
	}
	// The one-active-continuation guard must not count OUR OWN intent.
	d, _, err := decideForRevival(repoRoot, cfg, census, ev, it.Nonce)
	if err != nil {
		return ReviveOutcome{}, err
	}
	if d.Action != ActRevive {
		if _, err := ConsumeIntent(repoRoot, it.Nonce); err != nil {
			return ReviveOutcome{}, err
		}
		return ReviveOutcome{Reason: "the world changed before launch: " + d.Reason}, nil
	}
	consumed, err := ConsumeIntent(repoRoot, it.Nonce)
	if err != nil {
		return ReviveOutcome{}, err
	}
	if err := launch(consumed); err != nil {
		return ReviveOutcome{Reason: "dispatch failed after consumption; next tick reconciles: " + err.Error()}, nil
	}
	ev = RecordRevival(ev)
	if err := SaveEvidence(EvidencePath(repoRoot), ev); err != nil {
		return ReviveOutcome{}, err
	}
	return ReviveOutcome{Launched: true, Reason: "continuation dispatched for " + consumed.Goal}, nil
}

// decideForRevival is decideNow with one intent excluded from the
// active-continuation guard — an intent must not suppress itself.
func decideForRevival(repoRoot string, cfg TickConfig, census WorkerCensus, ev Evidence, excludeNonce string) (Decision, string, error) {
	cfg = cfg.withDefaults()
	work, workReason, err := LegacyOpenWork(repoRoot)
	if err != nil {
		return Decision{}, "", err
	}
	workers := Workers{}
	if work == WorkOwned {
		w, err := census.Workers(repoRoot)
		if err != nil {
			w = Workers{Unprovable: 1}
		}
		workers = w
	}
	live, err := LiveIntents(repoRoot)
	if err != nil {
		return Decision{VerdictDegraded, ActNotify, err.Error()}, workReason, nil
	}
	others := 0
	for _, l := range live {
		if l.Nonce != excludeNonce {
			others++
		}
	}
	return Decide(Snapshot{
		Work:               work,
		Workers:            workers,
		TicksSinceProgress: ev.TicksSinceAdvance,
		StaleTicks:         cfg.StaleTicks,
		DryRevivals:        ev.DryRevivals,
		MaxRevivals:        cfg.MaxRevivals,
		ActiveContinuation: others > 0,
	}), workReason, nil
}
