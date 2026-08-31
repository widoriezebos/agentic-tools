package steward

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/outage"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/supervise"
)

// One tick: read the world, fold the evidence, decide, and put
// every notify-only verdict on the durable queue. Revive decisions remain
// silent until the scheduler has tried the repair.

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
	// ProviderOutage reports a standing outage mark, for the narration
	// and the long-outage noticing; Outage carries the mark itself.
	ProviderOutage bool
	Outage         outage.Mark
	Health         HealthVerdict
	GoalStops      []BreachStopReport
}

// BreachStopReport is machinery history for one heal-before-notify stop pass.
type BreachStopReport struct {
	GoalID   string `json:"goalId"`
	Revision uint64 `json:"revision"`
	StopID   string `json:"stopId,omitempty"`
	State    string `json:"state"`
	Detail   string `json:"detail,omitempty"`
}

func runBreachStopCustodian(repoRoot string, now time.Time) []BreachStopReport {
	routes, err := dispatch.FindBreachStops(repoRoot, now)
	if err != nil {
		return []BreachStopReport{{State: "FAILED", Detail: err.Error()}}
	}
	reports := make([]BreachStopReport, 0, len(routes))
	for _, route := range routes {
		report := BreachStopReport{GoalID: route.GoalID, Revision: route.Revision, StopID: route.StopID}
		if route.Condition == dispatch.StopRouteIndeterminate {
			report.State, report.Detail = "INDETERMINATE", route.Failure
			reports = append(reports, report)
			continue
		}
		cmd := exec.Command(filepath.Join(repoRoot, "scripts", "agents", "dispatch.sh"),
			"__breach-stop-goal", "--goal", route.GoalID, "--revision", fmt.Sprint(route.Revision))
		cmd.Dir = repoRoot
		out, runErr := cmd.CombinedOutput()
		if runErr != nil {
			report.State = "FAILED"
			report.Detail = strings.TrimSpace(string(out))
			if report.Detail == "" {
				report.Detail = runErr.Error()
			}
		} else {
			report.State = "COMPLETE"
			report.Detail = strings.TrimSpace(string(out))
		}
		reports = append(reports, report)
	}
	return reports
}

// RunTick folds one observation into the persisted evidence and
// returns the decision. The evidence store is written back before
// returning, so a crash after the tick never replays its aging.
func RunTick(repoRoot string, cfg TickConfig, census WorkerCensus) (result TickResult, returnErr error) {
	cfg = cfg.withDefaults()

	// One tick at a time per repository: the CLI seam and the
	// resident runner share the evidence store and the pending
	// queue, and neither may age or drain it under the other.
	tickLock, err := AcquireArbitration(repoRoot)
	if err != nil {
		return TickResult{}, err
	}
	defer tickLock.Release()

	selfExact, selfState, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || selfState != identity.Alive {
		return TickResult{}, fmt.Errorf("the steward tick cannot read its own process identity")
	}
	generation, generationErr := installedGeneration(repoRoot)
	if generationErr != nil {
		// An unarmed manual tick remains useful for diagnosis, but generation
		// zero can never satisfy the armed-runner health check.
		generation = 0
	}
	tickAttempt, err := beginComponentAttempt(repoRoot, "steward-tick", generation, selfExact.Ref(), time.Now())
	if err != nil {
		return TickResult{}, fmt.Errorf("record tick attempt: %w", err)
	}
	tickCompleted := false
	defer func() {
		if result.Health.Schema == 0 {
			if healthErr := completeTickHealth(repoRoot, &result, generation, selfExact.Ref()); healthErr != nil && returnErr == nil {
				returnErr = healthErr
			}
		}
		if tickCompleted {
			return
		}
		evidence := "tick did not complete"
		if returnErr != nil {
			evidence = returnErr.Error()
		}
		if _, completeErr := completeComponentAttempt(repoRoot, "steward-tick", generation, tickAttempt.AttemptSeq,
			ComponentError, "TICK_FAILED", evidence, time.Now()); completeErr != nil && returnErr == nil {
			returnErr = fmt.Errorf("record failed tick completion: %w", completeErr)
		}
	}()

	// Budget healing runs before health and notification. A successful stop is
	// machinery history only; a failure remains visible to the ordinary health
	// breaker, which is the sole escalation owner.
	goalStops := runBreachStopCustodian(repoRoot, time.Now())
	governedRefreshFailures := refreshGovernedObligations(repoRoot, time.Now())
	if len(governedRefreshFailures) > 0 {
		return degradedTick(repoRoot, "governed-obligation observation failed: "+strings.Join(governedRefreshFailures, "; "))
	}
	if err := observeDirectValidationWindow(repoRoot, time.Now()); err != nil {
		return degradedTick(repoRoot, "direct-validation observation failed: "+err.Error())
	}
	if err := sweepRulingReviews(repoRoot, time.Now()); err != nil {
		return degradedTick(repoRoot, "ruling review sweep failed: "+err.Error())
	}
	if err := sweepCounselorBrief(repoRoot, time.Now()); err != nil {
		return degradedTick(repoRoot, "counselor brief carriage failed: "+err.Error())
	}

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
	// A standing provider outage pauses the aging, never the reset:
	// progress during an outage still counts, but the absence of
	// progress stops accusing local machinery while the provider is
	// the one down. The mark lapses on its own horizon, so a paused
	// clock can never outlive the outage's evidence. ONE sample
	// governs the whole tick — aging, decision, and narration must
	// tell the same story even when the mark moves mid-tick.
	outageMark, providerOutage := outage.StandingAt(repoRoot, time.Now())
	if providerOutage && marks == prev.Marks {
		ev = prev
	}

	d, workReason, err := decideNow(repoRoot, cfg, census, ev, providerOutage)
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
	result = TickResult{Decision: d, Evidence: ev, OpenWork: workReason,
		Reaped: reaped, ProviderOutage: providerOutage, Outage: outageMark, GoalStops: goalStops}
	if err := NarrateDigest(repoRoot, prev, result, time.Now()); err != nil {
		return result, fmt.Errorf("write narrator digest: %w", err)
	}
	if err := SaveEvidence(repoRoot, evPath, ev); err != nil {
		return result, err
	}
	// The running plain-English account rides every tick, strictly
	// best-effort: the storyteller never fails the shift. What the
	// narration notices also reaches the operator, one gated message
	// per building condition.
	Narrate(repoRoot, result, cfg)
	ReachTheHuman(repoRoot, noticings(result, cfg))
	if err := completeTickHealth(repoRoot, &result, generation, selfExact.Ref()); err != nil {
		return result, err
	}
	if _, err := completeComponentAttempt(repoRoot, "steward-tick", generation, tickAttempt.AttemptSeq,
		ComponentOK, "PASS_COMPLETE", result.Health.FindingDigest, time.Now()); err != nil {
		return result, fmt.Errorf("record tick completion: %w", err)
	}
	tickCompleted = true
	return result, nil
}

// completeTickHealth performs the mandatory end of every tick: one durable
// health observation, one durable narration line, and a queued alert whenever
// the observation's dead or persistent-unknown boundary requires one.
func completeTickHealth(repoRoot string, result *TickResult, generation int, process identity.Ref) error {
	health, err := ObserveHealth(repoRoot, time.Now(), identity.KernelProber{})
	if err != nil {
		return fmt.Errorf("compute health: %w", err)
	}
	result.Health = health
	if err := requestWatcherRepair(repoRoot, health, time.Now()); err != nil {
		return fmt.Errorf("request watcher repair: %w", err)
	}
	narratorAttempt, err := beginComponentAttempt(repoRoot, "narrator", generation, process, time.Now())
	if err != nil {
		return fmt.Errorf("record narrator attempt: %w", err)
	}
	line := health.Line()
	if err := NarrateHealthLine(repoRoot, line); err != nil {
		_, _ = completeComponentAttempt(repoRoot, "narrator", generation, narratorAttempt.AttemptSeq,
			ComponentError, "WRITE_FAILED", err.Error(), time.Now())
		return fmt.Errorf("narrate health: %w", err)
	}
	if _, err := completeComponentAttempt(repoRoot, "narrator", generation, narratorAttempt.AttemptSeq,
		ComponentOK, "EMITTED", line, time.Now()); err != nil {
		return fmt.Errorf("record narrator completion: %w", err)
	}
	if _, err := UpdateAlertEpisodes(repoRoot, health, line, time.Now()); err != nil {
		return fmt.Errorf("update health alert episodes: %w", err)
	}
	return nil
}

func requestWatcherRepair(repoRoot string, health HealthVerdict, now time.Time) error {
	for _, role := range health.Roles {
		if role.Role != RoleRepoWatcher || role.Status != HealthDead {
			continue
		}
		if role.FailureEscalation == AutoHealEnded {
			return supervise.EndWatcherRestart(repoRoot, "the health breaker ended automatic watcher repair", now)
		}
		if !strings.Contains(role.Reason, "recorded pid") &&
			!strings.Contains(role.Reason, "lastSuccess is stale") &&
			!strings.Contains(role.Reason, "latest attempt passed its deadline") {
			return nil
		}
		return supervise.RequestWatcherRestart(repoRoot, role.Reason, now)
	}
	return nil
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
// first and calls this; revive's re-arbitration goes through
// decideForRevival, so a re-check can never advance the clock it is
// checking. The caller supplies the outage sample so one observation
// governs its whole decision.
func decideNow(repoRoot string, cfg TickConfig, census WorkerCensus, ev Evidence, providerOutage bool) (Decision, string, error) {
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
		ProviderOutage:     providerOutage,
	}), workReason, nil
}
