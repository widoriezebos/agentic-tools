package steward

// The revival, in the design's pinned order: mint the intent under
// the arbitration lock (recording the enrollment fence), deliver the
// operator notification — delivery gates everything after it — then
// re-take the lock for one critical section: fence check, full
// predicate re-run, intent consumption, launch, stamp. A crash
// between launch and stamp reconciles next tick as a notified
// unknown; a fence bump or a changed verdict cancels with the reason
// on record.

import (
	"fmt"
	"path/filepath"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/receipt"
)

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
	// The per-attempt receipt is its own durable record, distinct
	// from the intent: the repository's ordinary evidence stream
	// carries every intervention the steward ever attempts.
	if res := receipt.Add(receipt.Options{
		Root: repoRoot, File: filepath.Join(repoRoot, "plans", "receipts.log"),
		Type: "other", Outcome: "shipped",
		Skills: "steward", Verify: "skipped", Corrections: "0", StopLoss: "no",
		Note: fmt.Sprintf("steward revival: intent %s revives %s via job %s", it.Nonce, it.Goal, it.JobId),
	}); res.Code != 0 {
		// A half-prepared intent must not survive: it would suppress
		// the runner and let a manual retry skip preparation. A
		// cancel that ALSO fails leaves a live intent — say so.
		if cancelErr := CancelIntent(repoRoot, it.Nonce, "preparation failed at the receipt"); cancelErr != nil {
			return fmt.Errorf("the revival receipt did not write (%v) AND the intent could not cancel (%v): a live half-prepared authorization remains — operator attention needed", res.Err, cancelErr)
		}
		return fmt.Errorf("the revival receipt did not write: %v", res.Err)
	}
	if err := QueueNotification(repoRoot, PendingNotification{
		Nonce:   it.Nonce,
		Message: fmt.Sprintf("steward: reviving %s (worker provably dead); job %s", it.Goal, it.JobId),
	}); err != nil {
		if cancelErr := CancelIntent(repoRoot, it.Nonce, "preparation failed at the notification queue"); cancelErr != nil {
			return fmt.Errorf("the notification could not queue (%v) AND the intent could not cancel (%v): a live half-prepared authorization remains — operator attention needed", err, cancelErr)
		}
		return err
	}
	return nil
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
		it.Notified = true
		if err := UpdateIntent(repoRoot, *it); err != nil {
			return ReviveOutcome{}, err
		}
		_ = MarkDelivered(repoRoot, it.Nonce)
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
		if err := CancelIntent(repoRoot, it.Nonce, "a worker enrolled after the reservation"); err != nil {
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
		if err := CancelIntent(repoRoot, it.Nonce, "the world changed before launch: "+d.Reason); err != nil {
			return ReviveOutcome{}, err
		}
		return ReviveOutcome{Reason: "the world changed before launch: " + d.Reason}, nil
	}
	consumed, err := ConsumeIntent(repoRoot, it.Nonce)
	if err != nil {
		return ReviveOutcome{}, err
	}
	// The attempt counts against the dry cap the moment it is
	// irreversible — before dispatch, so no crash window between
	// launch and bookkeeping can spend attempts the cap never saw.
	ev = RecordRevival(ev)
	if err := SaveEvidence(EvidencePath(repoRoot), ev); err != nil {
		return ReviveOutcome{}, err
	}
	if err := launch(consumed); err != nil {
		return ReviveOutcome{Reason: "dispatch failed after consumption; next tick reconciles: " + err.Error()}, nil
	}
	if err := StampLaunch(repoRoot, consumed.Nonce); err != nil {
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
	activeConsumed, err := ConsumedActive(repoRoot)
	if err != nil {
		return Decision{VerdictDegraded, ActNotify, err.Error()}, workReason, nil
	}
	others := len(activeConsumed)
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

// ResumableIntent names a live intent whose notification already
// delivered — a revival stopped between its gate and its critical
// section (a notifier outage recovered, a crashed revive). Both
// schedulers resume it; without this, a recovered outage would
// strand the revival forever behind its own active-continuation
// guard.
func ResumableIntent(repoRoot string) (string, bool, error) {
	live, err := LiveIntents(repoRoot)
	if err != nil {
		return "", false, err
	}
	for _, it := range live {
		if it.Notified {
			return it.Nonce, true, nil
		}
	}
	return "", false, nil
}
