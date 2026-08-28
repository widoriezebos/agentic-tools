package steward

// The revival mints the intent under the arbitration lock, then heals before
// any operator notification. It re-takes the lock for one critical section:
// fence check, full predicate re-run, intent consumption, launch, stamp. A crash
// between launch and stamp reconciles next tick as an unknown launch
// outcome; a fence bump or a changed verdict cancels with the reason
// on record.

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/outage"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/receipt"
)

// LaunchSeam performs the dispatch. The shell glue supplies the real
// dispatcher; fixtures supply an observable fake.
type LaunchSeam func(Intent) error

// ReviveOutcome says what happened, for the report and the receipt.
type ReviveOutcome struct {
	Launched bool
	Escalate bool
	Reason   string
}

// PrepareIntent mints the durable record under the lock and captures the
// enrollment fence. It performs no notification and no launch.
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
	return nil
}

// CompleteRevival runs the critical section for a live intent. The intent
// survives a crash until it launches or cancels, without waking the operator
// merely to announce work that the machinery can do itself.
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

	// Retire notification intents written by older binaries before recovery.
	// Their presence must not preserve an alert-before-heal path after upgrade.
	_ = MarkDelivered(repoRoot, it.Nonce)
	_ = MarkDelivered(repoRoot, "verdict-"+string(VerdictStalledDead))

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
	// The last look before the point of no return: outage writers do
	// not share this arbitration lock, so a mark can land after the
	// predicate re-ran. Rechecking here shrinks that window to the
	// consume-and-launch calls themselves; an outage beginning inside
	// it costs at most one dry revival, which the hint's contract
	// accepts.
	if _, standing := outage.StandingAt(repoRoot, time.Now()); standing {
		reason := "the model provider is overloaded; holding revival until the provider recovers"
		if err := CancelIntent(repoRoot, it.Nonce, reason); err != nil {
			return ReviveOutcome{}, err
		}
		return ReviveOutcome{Reason: reason}, nil
	}
	consumed, err := ConsumeIntent(repoRoot, it.Nonce)
	if err != nil {
		return ReviveOutcome{}, err
	}
	// The attempt counts against the dry cap the moment it is
	// irreversible — before dispatch, so no crash window between
	// launch and bookkeeping can spend attempts the cap never saw.
	ev = RecordRevival(ev)
	if err := SaveEvidence(repoRoot, EvidencePath(repoRoot), ev); err != nil {
		return ReviveOutcome{}, err
	}
	if err := launch(consumed); err != nil {
		return ReviveOutcome{Escalate: true, Reason: "dispatch failed after consumption; next tick reconciles: " + err.Error()}, nil
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
	work, workReason, err := ReadOpenWork(repoRoot)
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
	_, providerOutage := outage.StandingAt(repoRoot, time.Now())
	return Decide(Snapshot{
		Work:               work,
		Workers:            workers,
		TicksSinceProgress: ev.TicksSinceAdvance,
		StaleTicks:         cfg.StaleTicks,
		DryRevivals:        ev.DryRevivals,
		MaxRevivals:        cfg.MaxRevivals,
		ActiveContinuation: others > 0,
		ProviderOutage:     providerOutage,
	}), workReason, nil
}

// ResumableIntent names a live intent whose repair did not reach its critical
// section. Both schedulers resume it so a crash after preparation cannot
// strand healing behind the active-continuation guard.
func ResumableIntent(repoRoot string) (string, bool, error) {
	live, err := LiveIntents(repoRoot)
	if err != nil {
		return "", false, err
	}
	for _, it := range live {
		return it.Nonce, true, nil
	}
	return "", false, nil
}
