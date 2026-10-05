package steward

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/outage"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/seat"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
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
	// Now is set only by fixture-authorized command boundaries. A zero value
	// keeps each tick operation on the wall clock.
	Now time.Time
	// ArmedLineage is the lineage steward arm handed to steward run. Only
	// RunLoop reads it, and only to fill Runner.
	ArmedLineage string
	// Runner is the resident runner's own context, filled ONLY by RunLoop
	// from the identity it was enrolled with. A tick with no runner context
	// publishes no presence, so a command process running the manual tick
	// verb can never publish this machine's rollout confirmation.
	Runner *seat.RunnerContext
	// BreachStop closes one breached goal revision and completes its
	// cancellation pass through the delegate lifecycle, returning the
	// lifecycle's report. The lifecycle composes above the steward (design
	// 6.3), so the command layer supplies it; a tick without one reports
	// every stoppable route FAILED by name.
	BreachStop func(goalID string, revision uint64) (string, error)
	// BreachStopReady reports whether this process may act as the stop
	// custodian yet. The resident runner defers its breach-stop passes until
	// its standing is its own (a session leader no recognized ancestor
	// claims), because while the arming process is still its parent the
	// stop-custodian gate classifies that ancestor and the pass would only
	// write a FAILED report. A deferred pass scans nothing and reports
	// nothing; the breach stays for the next tick. nil is always ready (the
	// external tick verb).
	BreachStopReady func() bool
	// KeepLandingLane is one step of the host landing lane's keeper: it
	// wakes the lane's landing agent when the lane has work (lane design
	// r10 §3) and returns its outcome and line. The command layer supplies
	// it; nil keeps nothing. RunLoop calls it once per cycle outside the
	// helm, and between cycles every laneRecheck while the lane waits.
	KeepLandingLane func() lane.AgentRun
	// ProbeProvider answers one provider call without a limit; nil probes nothing.
	ProbeProvider func(top string) (bool, error)
	// ProbeRuntime runs the admission owner's capability probe without a job.
	// The command layer supplies it because delegation depends on the steward.
	ProbeRuntime func(root, runtime string) error
	// Stopping reports that the resident runner received a stop signal:
	// the tick finishes the stop in progress and starts no further one.
	// Only RunLoop sets it; nil is never stopping.
	Stopping func() bool
	// Seat starts and reads the steward's seat launches (g1-s77); nil starts
	// no seat and keeps today's notification for ready work.
	Seat SeatLauncher
	// Patterns is the behaviour-pattern pass (design
	// steward-acts-on-behaviour-patterns): it reports and decides nothing.
	// It runs after the tick's health pass and, as a report, under the helm
	// too (D2). The command layer supplies it; nil runs no pattern.
	Patterns func(repoRoot string, now time.Time) error
	// StuckUnits reports this seat's stuck units after health, including
	// under the helm. It never stops a launch.
	StuckUnits        func(repoRoot string, now time.Time) error
	narrationLocation *time.Location
}

var tickHealthNow = time.Now

func (c TickConfig) withDefaults() TickConfig {
	if c.StaleTicks <= 0 {
		c.StaleTicks = 5
	}
	if c.MaxRevivals <= 0 {
		c.MaxRevivals = 3
	}
	return c
}

func (c TickConfig) now() time.Time {
	if c.Now.IsZero() {
		return time.Now().UTC()
	}
	return c.Now.UTC()
}

func (c TickConfig) localNarrationLocation() *time.Location {
	if c.narrationLocation != nil {
		return c.narrationLocation
	}
	return time.Local
}

// TickResult is everything the calling verb needs to act and report.
type TickResult struct {
	Decision Decision
	Evidence Evidence
	OpenWork string       // the open-work reason, for the report
	Reaped   []ReapReport // continuations this tick closed
	// ProviderOutage reports a standing outage mark, for the narration
	// and the long-outage noticing; Outage carries the mark itself.
	ProviderOutage  bool
	Outage          outage.Mark
	Health          HealthVerdict
	GoalStops       []BreachStopReport
	LedgerAttention LedgerAttentionReport
	SeatPresence    SeatPresenceReport
	// Seat is the goal a revive decision's seat start names; nil for every
	// other decision.
	Seat *SeatSelection
}

// BreachStopReport is machinery history for one heal-before-notify stop pass.
type BreachStopReport struct {
	GoalID   string `json:"goalId"`
	Revision uint64 `json:"revision"`
	StopID   string `json:"stopId,omitempty"`
	State    string `json:"state"`
	Detail   string `json:"detail,omitempty"`
}

// custodialBreachStops is the tick's breach-stop pass: deferred, with no
// scan and no report, while the configured custodian is not ready.
func custodialBreachStops(repoRoot string, cfg TickConfig, scanner func(string, time.Time) ([]dispatch.StopRoute, error)) []BreachStopReport {
	if cfg.BreachStopReady != nil && !cfg.BreachStopReady() {
		return nil
	}
	reports := runBreachStopCustodianWithScanner(repoRoot, cfg.now(), scanner, cfg.BreachStop, cfg.Stopping)
	// The stop producer is unchanged; a completed stop also tells the
	// person, once per stopped goal revision.
	reportBreachStopNotices(repoRoot, reports, cfg.now())
	return reports
}

func runBreachStopCustodianWithScanner(repoRoot string, now time.Time,
	scanner func(string, time.Time) ([]dispatch.StopRoute, error), stop func(string, uint64) (string, error), stopping func() bool) []BreachStopReport {
	routes, err := scanner(repoRoot, now)
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
		if stopping != nil && stopping() {
			report.State, report.Detail = "DEFERRED", "the runner is stopping; the next runner's tick takes this breach"
			reports = append(reports, report)
			continue
		}
		if stop == nil {
			report.State, report.Detail = "FAILED", "the steward tick has no delegate lifecycle wired to run the breach stop"
			reports = append(reports, report)
			continue
		}
		out, stopErr := stop(route.GoalID, route.Revision)
		if stopErr != nil {
			report.State = "FAILED"
			report.Detail = strings.TrimSpace(out)
			if report.Detail == "" {
				report.Detail = stopErr.Error()
			}
		} else {
			report.State = "COMPLETE"
			report.Detail = strings.TrimSpace(out)
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
	tickAttempt, err := beginComponentAttempt(repoRoot, "steward-tick", generation, selfExact.Ref(), cfg.now())
	if err != nil {
		return TickResult{}, fmt.Errorf("record tick attempt: %w", err)
	}
	// A seat at the helm is the person's: the tick publishes presence,
	// completes its attempt as HELM and decides nothing (HM-7). The check
	// sits after the attempt exists and before the completion defer, so no
	// health, alert or narration pass runs either; helmTick makes the one
	// report-only pattern call (D2).
	if state := helm.Active(repoRoot); state.Active {
		return helmTick(repoRoot, cfg, generation, selfExact.Ref(), tickAttempt.AttemptSeq, state)
	}
	tickCompleted := false
	defer func() {
		reportErr := runTickReports(repoRoot, cfg, func() error {
			if result.Health.Schema == 0 {
				return completeTickHealthWithConfig(repoRoot, &result, generation, selfExact.Ref(), cfg)
			}
			return nil
		})
		if reportErr != nil && returnErr == nil {
			returnErr = reportErr
		}
		if tickCompleted {
			return
		}
		evidence := "tick did not complete"
		if returnErr != nil {
			evidence = returnErr.Error()
		}
		if _, completeErr := completeComponentAttempt(repoRoot, "steward-tick", generation, tickAttempt.AttemptSeq,
			ComponentError, "TICK_FAILED", evidence, nil, cfg.now()); completeErr != nil && returnErr == nil {
			returnErr = fmt.Errorf("record failed tick completion: %w", completeErr)
		}
	}()

	// Budget healing runs before health and notification. A successful stop is
	// machinery history only; a failure remains visible to the ordinary health
	// breaker, which is the sole escalation owner.
	goalStops := custodialBreachStops(repoRoot, cfg, dispatch.FindBreachStops)
	governedRefreshFailures := refreshGovernedObligations(repoRoot, cfg.now())
	if len(governedRefreshFailures) > 0 {
		return degradedTick(repoRoot, "governed-obligation observation failed: "+strings.Join(governedRefreshFailures, "; "))
	}
	if err := observeDirectValidationWindow(repoRoot, cfg.now()); err != nil {
		return degradedTick(repoRoot, "direct-validation observation failed: "+err.Error())
	}
	if err := sweepRulingReviews(repoRoot, cfg.now()); err != nil {
		return degradedTick(repoRoot, "ruling review sweep failed: "+err.Error())
	}
	if err := sweepCounselorBrief(repoRoot, cfg.now()); err != nil {
		return degradedTick(repoRoot, "counselor brief carriage failed: "+err.Error())
	}
	result, tickCompleted, returnErr = runTickAfterCustodial(repoRoot, cfg, census, generation, selfExact.Ref(), tickAttempt.AttemptSeq, goalStops, defaultTickContinuationDependencies())
	return result, returnErr
}

// VerdictHelm is the tick's verdict for a seat at the helm: nothing decided.
const VerdictHelm Verdict = "helm"

// helmTick is the whole tick of a seat at the helm: presence is still
// published (the fleet sees the seat alive) and the tick attempt completes
// with outcome HELM. No breach stop, reap, ledger attention, decision or
// narration runs; the one notification it may send is the pattern pass's
// report (D2).
func helmTick(repoRoot string, cfg TickConfig, generation int, process identity.Ref, attemptSeq int64, state helm.State) (TickResult, error) {
	reason := "human at the helm: " + state.By
	result := TickResult{Decision: Decision{VerdictHelm, ActNone, reason}}
	seatReport, presenceErr := seatPresenceComponent(repoRoot, cfg, generation, process)
	result.SeatPresence = seatReport
	if _, err := completeComponentAttempt(repoRoot, "steward-tick", generation, attemptSeq,
		ComponentOK, "HELM", reason, nil, cfg.now()); err != nil {
		return result, fmt.Errorf("record helm tick completion: %w", err)
	}
	// The one exception to HM-7 (design D2): the pattern pass reports under
	// the helm, because the person at the helm is who needs the report. A
	// report is not a decision. It runs after the attempt completed, so a
	// slow fetch never reads as a stuck tick.
	if patternErr := runPatterns(repoRoot, cfg); patternErr != nil && presenceErr == nil {
		presenceErr = patternErr
	}
	return result, presenceErr
}

// runTickReports observes health before running either report-only pass.
// A failed health observation does not prevent a detector from reporting.
func runTickReports(repoRoot string, cfg TickConfig, health func() error) error {
	healthErr := health()
	patternErr := runPatterns(repoRoot, cfg)
	if healthErr != nil {
		return healthErr
	}
	return patternErr
}

// runPatterns runs both report-only passes, even when one fails.
func runPatterns(repoRoot string, cfg TickConfig) error {
	var result error
	if cfg.Patterns != nil {
		if err := cfg.Patterns(repoRoot, cfg.now()); err != nil {
			result = fmt.Errorf("behaviour patterns: %w", err)
		}
	}
	if cfg.StuckUnits != nil {
		if err := cfg.StuckUnits(repoRoot, cfg.now()); err != nil {
			result = errors.Join(result, fmt.Errorf("stuck units: %w", err))
		}
	}
	return result
}

// seatPresenceComponent runs the seat-presence component with its own
// attempt record. A component that could not write its own durable evidence
// follows the tick's rule for every component: the observation is not
// recorded, so the tick does not claim it was.
func seatPresenceComponent(repoRoot string, cfg TickConfig, generation int, process identity.Ref) (SeatPresenceReport, error) {
	seatAttempt, err := beginComponentAttempt(repoRoot, "seat-presence", generation, process, cfg.now())
	if err != nil {
		return SeatPresenceReport{}, fmt.Errorf("record seat-presence attempt: %w", err)
	}
	seatReport, seatEvidenceErr := RunSeatPresence(repoRoot, cfg.Runner, generation, cfg.now())
	seatResult, seatOutcome := ComponentOK, "PASS_COMPLETE"
	var seatEvidence string
	switch {
	case seatEvidenceErr != nil:
		seatResult, seatOutcome, seatEvidence = ComponentError, "STATE_WRITE_FAILED", seatEvidenceErr.Error()
	case seatReport.Outcome == seat.OutcomeSkipped:
		seatOutcome, seatEvidence = "SKIPPED", "skipped: "+seatReport.Reason
	case seatReport.Outcome == seat.OutcomeFailed:
		seatResult, seatOutcome, seatEvidence = ComponentError, "PUBLISH_FAILED", "failed: "+seatReport.Detail
	default:
		seatEvidence = fmt.Sprintf("published on rung %d", seatReport.Rung)
		if seatReport.Detail != "" {
			seatEvidence += "; " + seatReport.Detail
		}
	}
	if _, err := completeComponentAttempt(repoRoot, "seat-presence", generation, seatAttempt.AttemptSeq,
		seatResult, seatOutcome, seatEvidence, nil, cfg.now()); err != nil {
		return seatReport, fmt.Errorf("record seat-presence completion: %w", err)
	}
	if seatEvidenceErr != nil {
		return seatReport, fmt.Errorf("record seat-presence evidence: %w", seatEvidenceErr)
	}
	return seatReport, nil
}

type tickContinuationDependencies struct {
	ledgerRepository *ledgerAttentionRepository
	ledgerWriter     ledgerAttentionStateWriter
	readMarks        func(string, ...string) ([]byte, error)
	openWork         openWorkDependencies
	resolveMachine   func(string) (string, error)
	resolveLayout    func(string) (stateroot.Layout, error)
	health           tickHealthDependencies
}

func defaultTickContinuationDependencies() tickContinuationDependencies {
	return tickContinuationDependencies{
		ledgerRepository: defaultLedgerAttentionRepository(),
		ledgerWriter:     atomicfile.WriteText,
		readMarks:        readMarksGit,
		openWork: openWorkDependencies{
			NewWorld:                  goal.NewWorld,
			ReadClaimableBudgetedWork: goal.ReadClaimableBudgetedWork,
			HandoffProber:             identity.KernelProber{},
		},
		resolveMachine: goal.ResolveMachine,
		resolveLayout:  stateroot.ResolveLayout,
		health: tickHealthDependencies{
			evaluate: evaluateHealthRoles,
			now:      tickHealthNow,
			deliver:  deliver,
			lane:     hostLaneSilence,
		},
	}
}

func runTickAfterCustodial(repoRoot string, cfg TickConfig, census WorkerCensus, generation int, process identity.Ref, tickAttemptSeq int64, goalStops []BreachStopReport, dependencies tickContinuationDependencies) (TickResult, bool, error) {
	dependencies.health.probeRuntime = cfg.ProbeRuntime
	dependencies.health.examineLedger = func(root string, now time.Time) error {
		return examineLedgerMoveWithRepositoryAndWriter(root, now, dependencies.ledgerRepository, dependencies.ledgerWriter)
	}
	// Close finished continuations first: the guard a reap frees must
	// not suppress this same tick's decision.
	reaped, err := ReapContinuations(repoRoot)
	if err != nil {
		result, degradedErr := degradedTick(repoRoot, "reaping failed: "+err.Error())
		return result, false, degradedErr
	}
	ledgerAttempt, err := beginComponentAttempt(repoRoot, "ledger-attention", generation, process, cfg.now())
	if err != nil {
		return TickResult{}, false, fmt.Errorf("record ledger-attention attempt: %w", err)
	}
	ledgerReport := runLedgerAttentionWithRepositoryAndWriter(repoRoot, cfg.now(), dependencies.ledgerRepository, dependencies.ledgerWriter)
	ledgerResult, ledgerOutcome, ledgerEvidence := ComponentOK, "PASS_COMPLETE", ledgerReport.Outcome
	if ledgerReport.Outcome == "failed" {
		ledgerResult, ledgerOutcome, ledgerEvidence = ComponentError, ledgerReport.FailureKind, ledgerReport.Failure
	}
	if _, err := completeComponentAttempt(repoRoot, "ledger-attention", generation, ledgerAttempt.AttemptSeq,
		ledgerResult, ledgerOutcome, ledgerEvidence, nil, cfg.now()); err != nil {
		return TickResult{}, false, fmt.Errorf("record ledger-attention completion: %w", err)
	}

	// Presence rides here, after the ledger-attention completion record and
	// before the evidence-store reads whose failure ends the tick degraded,
	// so a torn evidence store on this machine does not make it read dead to
	// its peers. The component owns both its fetch and its publish.
	seatReport, err := seatPresenceComponent(repoRoot, cfg, generation, process)
	if err != nil {
		return TickResult{}, false, err
	}

	evPath := EvidencePath(repoRoot)
	prev, err := LoadEvidence(evPath)
	if err != nil {
		// A torn store degrades honestly: report, do not guess ages.
		result, degradedErr := degradedTick(repoRoot, err.Error())
		return result, false, degradedErr
	}
	marks, err := currentMarksWithReader(repoRoot, dependencies.readMarks)
	if err != nil {
		result, degradedErr := degradedTick(repoRoot, err.Error())
		return result, false, degradedErr
	}
	if cfg.Seat != nil {
		seat := defaultSeatDependencies(cfg.Seat)
		dependencies.openWork.Seat = &seat
	}
	result, err := decideTickWithDependencies(repoRoot, cfg, census, prev, marks, dependencies.openWork)
	if err != nil {
		return TickResult{}, false, err
	}
	result.Reaped = reaped
	result.GoalStops = goalStops
	result.LedgerAttention = ledgerReport
	result.SeatPresence = seatReport
	if err := narrateDigestWithReaders(repoRoot, prev, result, cfg.now(), dependencies.resolveMachine, dependencies.resolveLayout); err != nil {
		return result, false, fmt.Errorf("the narrator summary could not be written: %w", err)
	}
	machine := "this machine"
	if enrolled, err := dependencies.resolveMachine(repoRoot); err == nil {
		machine = enrolled
	}
	var surfaced []string
	for _, event := range ledgerReport.Pending {
		if err := QueueNotification(repoRoot, ledgerAttentionNotification(event, machine)); err != nil {
			return result, false, fmt.Errorf("queue ledger-attention notification: %w", err)
		}
		surfaced = append(surfaced, event.SourceID)
	}
	if err := PersistLedgerAttentionMark(repoRoot, surfaced); err != nil {
		return result, false, fmt.Errorf("mark ledger-attention events surfaced: %w", err)
	}
	if err := SaveEvidence(repoRoot, evPath, result.Evidence); err != nil {
		return result, false, err
	}
	// The running plain-English account rides every tick, strictly
	// best-effort: the storyteller never fails the shift. What the
	// narration notices also reaches the operator, one gated message
	// per building condition.
	narrateWithMachineReader(repoRoot, result, cfg, dependencies.resolveMachine)
	ReachTheHuman(repoRoot, noticingsAt(repoRoot, result, cfg, cfg.now()))
	if err := completeTickHealthWithDependencies(repoRoot, &result, generation, process, cfg.Now, dependencies.health); err != nil {
		return result, false, err
	}
	if _, err := completeComponentAttempt(repoRoot, "steward-tick", generation, tickAttemptSeq,
		ComponentOK, "PASS_COMPLETE", result.Health.FindingDigest, nil, cfg.now()); err != nil {
		return result, false, fmt.Errorf("record tick completion: %w", err)
	}
	return result, true, nil
}

// degradedNoticeTicks is how many ticks in a row a degraded verdict holds
// before it queues its notice.
const degradedNoticeTicks = 2

func decideTickWithDependencies(repoRoot string, cfg TickConfig, census WorkerCensus, prev Evidence, marks Marks, dependencies openWorkDependencies) (TickResult, error) {
	ev := Observe(prev, marks)
	// A standing provider outage pauses the aging, never the reset:
	// progress during an outage still counts, but the absence of
	// progress stops accusing local machinery while the provider is
	// the one down. The mark lapses on its own horizon, so a paused
	// clock can never outlive the outage's evidence. ONE sample
	// governs the whole tick — aging, decision, and narration must
	// tell the same story even when the mark moves mid-tick.
	// The seat launches are reaped before the outage is sampled: a seat the
	// provider stopped feeds the mark this same tick holds by.
	var seat *seatTickState
	if dependencies.Seat != nil {
		state := reapSeatLaunches(repoRoot, *dependencies.Seat, cfg.now())
		seat = &state
	}
	var log func(string)
	if dependencies.Seat != nil {
		log = dependencies.Seat.Log
	}
	outageMark, providerOutage := standingProviderOutage(repoRoot, cfg.now(), log)
	if providerOutage && marks == prev.Marks {
		ev = prev
	}

	d, selection, workReason, err := decideNowWithSeat(repoRoot, cfg, census, ev, providerOutage, dependencies, seat)
	if err != nil {
		return TickResult{}, err
	}
	// One degraded read, such as a ledger read inside a burst of ledger
	// commits, reads fine at the next tick: the verdict is reported every
	// tick, and its notice waits until it holds degradedNoticeTicks in a row.
	ev.Degraded = 0
	if d.Verdict == VerdictDegraded {
		ev.Degraded = prev.Degraded + 1
	}
	if d.Action == ActNotify && (d.Verdict != VerdictDegraded || ev.Degraded >= degradedNoticeTicks) {
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
	return TickResult{Decision: d, Evidence: ev, OpenWork: workReason,
		ProviderOutage: providerOutage, Outage: outageMark, Seat: selection}, nil
}

// completeTickHealth performs the mandatory end of every tick: one durable
// health observation, one durable narration line, and a queued alert whenever
// the observation's dead or persistent-unknown boundary requires one.
func completeTickHealthWithConfig(repoRoot string, result *TickResult, generation int, process identity.Ref, cfg TickConfig) error {
	return completeTickHealthWithDependencies(repoRoot, result, generation, process, cfg.Now, tickHealthDependencies{
		evaluate:     evaluateHealthRoles,
		now:          tickHealthNow,
		deliver:      deliver,
		lane:         hostLaneSilence,
		probeRuntime: cfg.ProbeRuntime,
		examineLedger: func(root string, now time.Time) error {
			return examineLedgerMoveWithRepositoryAndWriter(root, now, defaultLedgerAttentionRepository(), atomicfile.WriteText)
		},
	})
}

type tickHealthDependencies struct {
	evaluate healthRoleEvaluator
	now      func() time.Time
	deliver  func(string, string) error
	// lane reads the host landing lane for the lane-silent signal; nil
	// reads no lane.
	lane          func(string, time.Time) LaneSilence
	probeRuntime  func(root, runtime string) error
	lookPath      func(string) (string, error)
	examineLedger func(string, time.Time) error
}

func completeTickHealthWithDependencies(repoRoot string, result *TickResult, generation int, process identity.Ref, now time.Time, dependencies tickHealthDependencies) error {
	healthNow := func() time.Time {
		if now.IsZero() {
			return dependencies.now()
		}
		return now.UTC()
	}
	health, err := observeHealthWithEvaluation(repoRoot, healthNow(), identity.KernelProber{}, dependencies.evaluate)
	if err != nil {
		return fmt.Errorf("compute health: %w", err)
	}
	result.Health = health
	if err := requestWatcherRepair(repoRoot, health, healthNow()); err != nil {
		return fmt.Errorf("request watcher repair: %w", err)
	}
	remedyErr := remedyHealthRoles(repoRoot, health, generation, process, healthNow, dependencies)
	health.FindingDigest = healthFindingDigest(health.Roles)
	if health.FindingDigest != result.Health.FindingDigest {
		if err := saveRemediedHealth(repoRoot, health); err != nil {
			return fmt.Errorf("record remedy health: %w", err)
		}
	}
	result.Health = health
	narratorAttempt, err := beginComponentAttempt(repoRoot, "narrator", generation, process, healthNow())
	if err != nil {
		return fmt.Errorf("record narrator attempt: %w", err)
	}
	line := health.Line()
	if err := NarrateHealthLine(repoRoot, line); err != nil {
		_, _ = completeComponentAttempt(repoRoot, "narrator", generation, narratorAttempt.AttemptSeq,
			ComponentError, "WRITE_FAILED", err.Error(), nil, healthNow())
		return fmt.Errorf("narrate health: %w", err)
	}
	if _, err := completeComponentAttempt(repoRoot, "narrator", generation, narratorAttempt.AttemptSeq,
		ComponentOK, "EMITTED", line, nil, healthNow()); err != nil {
		return fmt.Errorf("record narrator completion: %w", err)
	}
	if _, err := updateAlertEpisodesWith(repoRoot, health, line, healthNow(), dependencies.deliver); err != nil {
		return fmt.Errorf("update health alert episodes: %w", err)
	}
	if err := fileStandingDefects(repoRoot, health, healthNow(), dependencies.deliver); err != nil {
		return fmt.Errorf("file standing health defects: %w", err)
	}
	if err := updateSpendEpisodesWith(repoRoot, health.Spend, healthNow(), dependencies.deliver); err != nil {
		return fmt.Errorf("update spend alert episodes: %w", err)
	}
	if dependencies.lane != nil {
		at := healthNow()
		if err := updateLaneSilentEpisodes(repoRoot, dependencies.lane(repoRoot, at), at); err != nil {
			return fmt.Errorf("update lane-silent alert episode: %w", err)
		}
	}
	return remedyErr
}

func remedyHealthRoles(repoRoot string, health HealthVerdict, generation int, process identity.Ref, now func() time.Time, dependencies tickHealthDependencies) error {
	var failures error
	for index := range health.Roles {
		role := &health.Roles[index]
		if role.Status != HealthDead || role.FailureEscalation == AutoHealEnded {
			continue
		}
		switch role.Role {
		case RoleCapabilitySnapshots:
			if dependencies.probeRuntime == nil {
				continue
			}
			lookPath := dependencies.lookPath
			if lookPath == nil {
				lookPath = exec.LookPath
			}
			_, stale := capabilitySnapshotStatus(repoRoot, repoRoot, now(), lookPath)
			for _, runtime := range stale {
				failures = errors.Join(failures, recordHealthRemedy(repoRoot, "capability-probe-"+runtime, role, generation, process, now,
					func() error { return dependencies.probeRuntime(repoRoot, runtime) }))
			}
		case RoleLedgerAttention:
			if dependencies.examineLedger != nil {
				failures = errors.Join(failures, recordHealthRemedy(repoRoot, "ledger-examination", role, generation, process, now,
					func() error { return dependencies.examineLedger(repoRoot, now()) }))
			}
		}
	}
	return failures
}

// A remedy's result belongs to its health role. Only a failure to record
// that result prevents the tick from completing its mandatory work.
func recordHealthRemedy(root, component string, role *RoleVerdict, generation int, process identity.Ref, now func() time.Time, run func() error) error {
	attempt, err := beginComponentAttempt(root, component, generation, process, now())
	if err != nil {
		return err
	}
	role.Reason = strings.ReplaceAll(role.Reason, "; "+component+": "+attempt.LastFailure, "")
	result, outcome, evidence := ComponentOK, "PASS_COMPLETE", "remedy completed"
	if err := run(); err != nil {
		result, outcome, evidence = ComponentError, "FAILED", err.Error()
		role.Reason += "; " + component + ": " + evidence
	}
	_, err = completeComponentAttempt(root, component, generation, attempt.AttemptSeq, result, outcome, evidence, nil, now())
	return err
}

func requestWatcherRepair(repoRoot string, health HealthVerdict, now time.Time) error {
	for _, role := range health.Roles {
		if role.Role != RoleRepoWatcher || role.Status != HealthDead {
			continue
		}
		if role.FailureEscalation == AutoHealEnded {
			return supervise.EndWatcherRestart(repoRoot, "the health breaker ended automatic watcher repair", now)
		}
		if !watcherRepairable(role) {
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

// decideNowWithSeat is the ladder with the seat ladder in it (g1-s77): the
// census is read for claimable work as for owned work, and with the seat
// wiring present claimable work, and owned work under the seat lineage, are
// the seat ladder's to decide.
func decideNowWithSeat(repoRoot string, cfg TickConfig, census WorkerCensus, ev Evidence, providerOutage bool, dependencies openWorkDependencies, seat *seatTickState) (Decision, *SeatSelection, string, error) {
	cfg = cfg.withDefaults()
	work, workReason, shared, err := readOpenWorkShared(repoRoot, dependencies)
	if err != nil {
		return Decision{}, nil, "", err
	}

	workers := Workers{}
	if work == WorkOwned || work == WorkClaimable {
		w, err := census.Workers(repoRoot)
		if err != nil {
			// An unreadable census can never prove death.
			w = Workers{Unprovable: 1}
		}
		workers = w
	}

	if dependencies.Seat != nil && seat != nil && shared != nil {
		if d, selection, ok := decideSeat(repoRoot, cfg, work, *shared, workers, providerOutage, *dependencies.Seat, *seat); ok {
			return d, selection, workReason, nil
		}
	}

	live, err := LiveIntents(repoRoot)
	if err != nil {
		return Decision{VerdictDegraded, ActNotify, err.Error()}, nil, workReason, nil
	}
	activeConsumed, err := ConsumedActive(repoRoot)
	if err != nil {
		return Decision{VerdictDegraded, ActNotify, err.Error()}, nil, workReason, nil
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
	}), nil, workReason, nil
}
