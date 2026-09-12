package steward

// The revival mints the intent under the arbitration lock, then heals before
// any operator notification. It re-takes the lock for one critical section:
// fence check, full predicate re-run, intent consumption, launch, stamp. A crash
// between launch and stamp reconciles next tick as an unknown launch
// outcome; a fence bump or a changed verdict cancels with the reason
// on record.

import (
	"fmt"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/outage"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/receipt"
)

// LaunchSeam performs the dispatch. The shell glue supplies the real
// dispatcher; fixtures supply an observable fake.
type LaunchSeam func(Intent) error

// SeatIdleClaimSeam performs the claim deferred by a bounded Stop verdict.
// It runs in the steward critical section, never in the Stop hook child.
type SeatIdleClaimSeam func(Intent) error

// ReviveOutcome says what happened, for the report and the receipt.
type ReviveOutcome struct {
	Launched bool
	Escalate bool
	Reason   string
}

// PrepareIntent mints the durable record under the lock and captures the
// enrollment fence. It performs no notification and no launch.
func PrepareIntent(repoRoot, receiptFile string, it Intent) error {
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
		Root: repoRoot, File: receiptFile,
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
func CompleteRevival(repoRoot string, cfg TickConfig, census WorkerCensus, nonce string, launch LaunchSeam, claimOption ...SeatIdleClaimSeam) (ReviveOutcome, error) {
	// The critical section: fence, verdict, consume, launch, stamp.
	arb, err := AcquireArbitration(repoRoot)
	if err != nil {
		return ReviveOutcome{}, err
	}
	defer arb.Release()
	intents, err := LiveIntents(repoRoot)
	if err != nil {
		return ReviveOutcome{}, err
	}
	var it *Intent
	for i := range intents {
		if intents[i].Nonce == nonce {
			it = &intents[i]
			break
		}
	}
	if it == nil {
		return ReviveOutcome{Reason: "intent is not live (already consumed or cancelled)"}, nil
	}

	// Retire notification intents written by older binaries before recovery.
	// Their presence must not preserve an alert-before-heal path after upgrade.
	_ = MarkDelivered(repoRoot, it.Nonce)
	_ = MarkDelivered(repoRoot, "verdict-"+string(VerdictStalledDead))

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
	d, _, err := decideForRevival(repoRoot, cfg, census, ev, *it)
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
	if it.Reason == "seatIdle" && it.ClaimNeeded {
		claim := func(intent Intent) error { return claimSeatIdleGoal(repoRoot, intent) }
		if len(claimOption) > 0 && claimOption[0] != nil {
			claim = claimOption[0]
		}
		if claimErr := claim(*it); claimErr != nil {
			reason := "the deferred seat-idle claim was refused: " + claimErr.Error()
			if err := CancelIntent(repoRoot, it.Nonce, reason); err != nil {
				return ReviveOutcome{}, err
			}
			if err := QueueNotification(repoRoot, PendingNotification{
				Nonce:   "verdict-" + string(VerdictIdleBacklogDead),
				Message: fmt.Sprintf("steward: %s — seat-idle claim for goal %s was refused: %s", VerdictIdleBacklogDead, it.Goal, claimErr),
			}); err != nil {
				return ReviveOutcome{}, err
			}
			return ReviveOutcome{Reason: reason + "; the human idle alarm was queued"}, nil
		}
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

func claimSeatIdleGoal(repoRoot string, intent Intent) error {
	if intent.SeatActor == nil || intent.SeatActor.Machine == "" || intent.SeatActor.Lineage == "" || intent.SeatClaimEpoch < 1 {
		return fmt.Errorf("the recorded seat actor or its positive checkout lease epoch is unavailable")
	}
	endpoint, err := goal.ResolveEndpoint(repoRoot)
	if err != nil {
		return err
	}
	operationID, err := goal.NewOperationULID()
	if err != nil {
		return err
	}
	result, err := goal.Claim(goal.VerbRequest{
		Endpoint: endpoint,
		Actor: goal.Actor{
			Machine: intent.SeatActor.Machine,
			Lineage: intent.SeatActor.Lineage,
		},
		Ulid: operationID, Now: time.Now(), ClaimEpoch: intent.SeatClaimEpoch,
	}, intent.Goal)
	if err != nil {
		return err
	}
	if result.Outcome != goal.OutcomeConfirmed && result.Outcome != goal.OutcomeConfirmedLate {
		return fmt.Errorf("goal claim returned %s: %s", result.Outcome, result.Detail)
	}
	return nil
}

// decideForRevival is decideNow with one intent excluded from the
// active-continuation guard — an intent must not suppress itself.
func decideForRevival(repoRoot string, cfg TickConfig, census WorkerCensus, ev Evidence, intent Intent) (Decision, string, error) {
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
	// Reconcile earlier continuations after the census and before the guard is
	// computed. Dead continuations stop suppressing dispatch; live or uncertain
	// ones remain active.
	if _, err := ReapContinuations(repoRoot); err != nil {
		return Decision{}, "", err
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
		if l.Nonce != intent.Nonce {
			others++
		}
	}
	_, providerOutage := outage.StandingAt(repoRoot, time.Now())
	decision := Decide(Snapshot{
		Work:               work,
		Workers:            workers,
		TicksSinceProgress: ev.TicksSinceAdvance,
		StaleTicks:         cfg.StaleTicks,
		DryRevivals:        ev.DryRevivals,
		MaxRevivals:        cfg.MaxRevivals,
		ActiveContinuation: others > 0,
		ProviderOutage:     providerOutage,
	})
	if intent.Reason != "seatIdle" {
		return decision, workReason, nil
	}
	// A seatIdle intent is the seat's explicit handoff: the main is expected
	// to remain alive. Re-check the exact held claim or claimable target and
	// every safety guard, then bypass only the ordinary live-main suppression.
	shared, err := goal.ReadClaimableBudgetedWork(repoRoot, time.Now())
	if err != nil {
		return Decision{VerdictDegraded, ActNotify, "seatIdle claim could not be re-read: " + err.Error()}, workReason, nil
	}
	targetPresent := false
	targets := append(append([]string(nil), shared.Claimed...), shared.Landing...)
	missingReason := "seatIdle intent no longer names a claim held by this machine"
	if intent.ClaimNeeded {
		targets = shared.Claimable
		missingReason = "seatIdle intent no longer names a claimable goal"
	}
	for _, id := range targets {
		if id == intent.Goal {
			targetPresent = true
			break
		}
	}
	switch {
	case !targetPresent:
		return Decision{VerdictStalledIdle, ActNotify, missingReason}, workReason, nil
	case shared.HasDelegateJobInFlight():
		return Decision{VerdictHealthy, ActNone, "seatIdle claim now has a delegate job in flight"}, workReason, nil
	case others > 0:
		return Decision{VerdictStalledIdle, ActNotify, "seatIdle handoff is blocked because a continuation is already open and unreaped"}, workReason, nil
	case ev.DryRevivals >= cfg.MaxRevivals:
		return Decision{VerdictStalledIdle, ActNotify, fmt.Sprintf("seatIdle handoff is blocked because %d revivals produced no progress", ev.DryRevivals)}, workReason, nil
	case providerOutage:
		return Decision{VerdictStalledIdle, ActNotify, "seatIdle handoff is blocked while the model provider is overloaded"}, workReason, nil
	default:
		return Decision{VerdictStalledIdle, ActRevive, "the live seat handed its claimed goal to a steward continuation through seatIdle"}, workReason, nil
	}
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
