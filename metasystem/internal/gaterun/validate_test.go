package gaterun

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

// validateBed is landing validate over recorded seams: every effect is
// logged in order, so a witness reads what happened and in which order.
type validateBed struct {
	t           *testing.T
	log         []string
	reservation *Validation
	custody     map[string]string // run id -> custody state
	barrierLive []string
	barrierUnk  []string
	outcome     map[string]RunOutcome
	gap         error
	planErr     error
	plan        ValidationPlan
	latest      *goal.CadenceStatus
	launchErr   error
	weight      WeightState
	weightErr   error
	discharge   error
	publishErr  error
	published   []goal.CadenceStatus
	runs        int
	// onWait runs when a call attaches to a live run.
	onWait func(Validation)
}

var validateNow = time.Date(2026, 9, 30, 21, 0, 0, 0, time.UTC)

const validateTree = "1111111111111111111111111111111111111111"

func greenResult(attempt string) proofrun.TestResult {
	return proofrun.TestResult{AttemptID: attempt, Groups: []proofrun.GroupResult{{ID: "deep", Status: "passed", CollectionComplete: true,
		ExecutionIdentity: strings.Repeat("a", 64)}}}
}

func redResult(attempt string) proofrun.TestResult {
	return proofrun.TestResult{AttemptID: attempt, Groups: []proofrun.GroupResult{{ID: "deep", Status: "failed", CollectionComplete: true,
		ExecutionIdentity: strings.Repeat("a", 64)}}}
}

func newValidateBed(t *testing.T) *validateBed {
	return &validateBed{t: t, custody: map[string]string{}, outcome: map[string]RunOutcome{},
		plan: ValidationPlan{Trunk: CadenceTrunk{Commit: strings.Repeat("2", 40), Tree: validateTree},
			Key: goal.CadenceClaimKey{TrunkTree: validateTree, WeightGeneration: 3, ForcedWindowStart: "2026-09-30T18:00:00Z"},
			Due: true, Trigger: goal.CadenceTriggerWeightDue, WeightDue: true, DeepOnly: []string{"deep"},
			Probes: []proofrun.GroupResult{{ID: "deep", Status: "passed", ExecutionIdentity: strings.Repeat("a", 64)}}},
		weight: WeightState{Generation: 3}}
}

func (bed *validateBed) seams() ValidateSeams {
	return ValidateSeams{
		Clock: func() time.Time { return validateNow },
		Reservation: func() (*Validation, error) {
			if bed.reservation == nil {
				return nil, nil
			}
			copied := *bed.reservation
			return &copied, nil
		},
		Reserve: func(v Validation) error {
			if bed.reservation != nil {
				return fmt.Errorf("already reserved")
			}
			bed.log = append(bed.log, "reserve "+v.RunID)
			bed.reservation = &v
			return nil
		},
		Record: func(v Validation) error {
			bed.log = append(bed.log, "record "+v.RunID+" "+v.Custody)
			bed.reservation = &v
			return nil
		},
		Clear: func(runID string) error {
			bed.log = append(bed.log, "clear "+runID)
			if bed.reservation != nil && bed.reservation.RunID == runID {
				bed.reservation = nil
			}
			return nil
		},
		Custody: func(v Validation) (string, string) {
			state := bed.custody[v.RunID]
			if state == "" {
				state = CustodyDead
			}
			return state, "run " + v.RunID + " is " + state
		},
		Barrier: func() ([]string, []string, error) { return bed.barrierLive, bed.barrierUnk, nil },
		Wait: func(v Validation) error {
			bed.log = append(bed.log, "wait "+v.RunID)
			if bed.onWait != nil {
				bed.onWait(v)
			}
			return nil
		},
		Outcome: func(v Validation) RunOutcome { return bed.outcome[v.RunID] },
		Gap:     func() error { return bed.gap },
		Claim: func(time.Time) (CadenceAuthority, error) {
			bed.log = append(bed.log, "claim")
			return CadenceAuthority{GoalID: "standing-validation", ObligationRevision: 2}, nil
		},
		Plan: func() (ValidationPlan, error) {
			if bed.planErr != nil {
				return ValidationPlan{}, bed.planErr
			}
			return bed.plan, nil
		},
		Latest: func() (*goal.CadenceStatus, error) { return bed.latest, nil },
		NewRunID: func() (string, error) {
			bed.runs++
			return fmt.Sprintf("cadence-run-%d", bed.runs), nil
		},
		Launch: func(v Validation) (string, error) {
			bed.log = append(bed.log, "launch "+v.RunID)
			if bed.launchErr != nil {
				return "", bed.launchErr
			}
			// The launched run ends green unless the bed says otherwise.
			if _, ok := bed.outcome[v.RunID]; !ok {
				bed.outcome[v.RunID] = RunOutcome{Usable: true, Result: greenResult("attempt-" + v.RunID)}
			}
			return "validate-" + v.RunID, nil
		},
		Weight: func() (WeightState, error) { return bed.weight, bed.weightErr },
		Discharge: func(_ CadenceAuthority, runID string, _ time.Time) error {
			bed.log = append(bed.log, "discharge "+runID)
			return bed.discharge
		},
		Publish: func(key goal.CadenceClaimKey, status goal.CadenceStatus, _ []goal.TrunkRedRecordGroup) error {
			bed.log = append(bed.log, "publish "+status.RunID)
			if bed.publishErr != nil {
				return bed.publishErr
			}
			bed.published = append(bed.published, status)
			return nil
		},
	}
}

func (bed *validateBed) validate(force bool) ValidateOutcome {
	bed.t.Helper()
	outcome, err := Validate(force, bed.seams())
	if err != nil {
		bed.t.Fatalf("landing validate: %v", err)
	}
	return outcome
}

func (bed *validateBed) reserved(runID string, discharge bool) {
	bed.reservation = &Validation{Key: bed.plan.Key, RunID: runID, Custody: "validate-" + runID, Discharge: discharge, Trunk: bed.plan.Trunk,
		Trigger: bed.plan.Trigger, Authority: CadenceAuthority{GoalID: "standing-validation", ObligationRevision: 2}, DeepOnly: bed.plan.DeepOnly,
		Probes: bed.plan.Probes, ReservedAt: validateNow.Add(-time.Hour).Format(time.RFC3339)}
}

func (bed *validateBed) wrote(t *testing.T, want ...string) {
	t.Helper()
	if !slices.Equal(bed.log, want) {
		t.Fatalf("effects %q, want %q", bed.log, want)
	}
}

// The key and the run id are recorded before the run launches, and the run
// is finalized after it ends: a green run with weight due discharges it.
func TestValidateRecordsKeyAndRunBeforeLaunch(t *testing.T) {
	t.Parallel()
	bed := newValidateBed(t)
	outcome := bed.validate(false)
	if outcome.Result != ValidateFinalized || !outcome.Discharged || outcome.RunID != "cadence-run-1" || outcome.Key != bed.plan.Key {
		t.Fatalf("outcome = %+v", outcome)
	}
	bed.wrote(t, "claim", "reserve cadence-run-1", "launch cadence-run-1", "record cadence-run-1 validate-cadence-run-1",
		"discharge cadence-run-1", "publish cadence-run-1", "clear cadence-run-1")
}

// A reserved run whose custody is live is attached, not run again: the
// call waits for it and finalizes it.
func TestValidateAttachesToLiveRun(t *testing.T) {
	t.Parallel()
	bed := newValidateBed(t)
	bed.reserved("cadence-run-7", true)
	bed.custody["cadence-run-7"] = CustodyLive
	bed.outcome["cadence-run-7"] = RunOutcome{Usable: true, Result: greenResult("attempt-7")}
	bed.onWait = func(v Validation) { bed.custody[v.RunID] = CustodyDead }
	outcome := bed.validate(false)
	if outcome.Result != ValidateFinalized || !outcome.Attached || outcome.RunID != "cadence-run-7" {
		t.Fatalf("outcome = %+v", outcome)
	}
	bed.wrote(t, "wait cadence-run-7", "discharge cadence-run-7", "publish cadence-run-7", "clear cadence-run-7")
}

// A run that stays live while the call waits is left reserved: nothing is
// finalized, published or started.
func TestValidateLeavesARunThatStillRuns(t *testing.T) {
	t.Parallel()
	bed := newValidateBed(t)
	bed.reserved("cadence-run-7", true)
	bed.custody["cadence-run-7"] = CustodyLive
	bed.outcome["cadence-run-7"] = RunOutcome{Usable: true, Result: greenResult("attempt-7")}
	outcome := bed.validate(false)
	if outcome.Result != ValidateWaiting || outcome.Code != CodeValidateCustodyLive || bed.reservation == nil {
		t.Fatalf("outcome = %+v, reservation %+v", outcome, bed.reservation)
	}
	bed.wrote(t, "wait cadence-run-7")
}

// A key the ledger already completed is returned: nothing runs again.
func TestValidateReturnsCompletedKey(t *testing.T) {
	t.Parallel()
	bed := newValidateBed(t)
	bed.latest = &goal.CadenceStatus{TrunkTree: validateTree, WeightGeneration: 3, ForcedWindowStart: "2026-09-30T18:00:00Z", RunID: "cadence-run-0"}
	outcome := bed.validate(false)
	if outcome.Result != ValidateCompleted || outcome.Status != bed.latest || outcome.RunID != "cadence-run-0" {
		t.Fatalf("outcome = %+v", outcome)
	}
	bed.wrote(t)
}

// A red run publishes and makes no reset, with weight due.
func TestValidateRedPublishesWithoutReset(t *testing.T) {
	t.Parallel()
	bed := newValidateBed(t)
	bed.reserved("cadence-run-7", true)
	bed.outcome["cadence-run-7"] = RunOutcome{Usable: true, Result: redResult("attempt-7")}
	outcome := bed.validate(false)
	if outcome.Result != ValidateFinalized || outcome.Discharged || len(bed.published) != 1 || bed.published[0].Groups[0].Status != "failed" {
		t.Fatalf("outcome = %+v, published %+v", outcome, bed.published)
	}
	bed.wrote(t, "publish cadence-run-7", "clear cadence-run-7")
}

// A run that validated an earlier weight count settles without a reset:
// the weight stays due for a later key.
func TestValidateMovedGenerationSettlesWithoutReset(t *testing.T) {
	t.Parallel()
	bed := newValidateBed(t)
	bed.reserved("cadence-run-7", true)
	bed.weight.Generation = 4
	bed.outcome["cadence-run-7"] = RunOutcome{Usable: true, Result: greenResult("attempt-7")}
	outcome := bed.validate(false)
	if outcome.Result != ValidateFinalized || outcome.Discharged {
		t.Fatalf("outcome = %+v", outcome)
	}
	bed.wrote(t, "publish cadence-run-7", "clear cadence-run-7")
}

// A typed WEIGHT_* refusal settles without claiming a reset; an uncoded
// discharge error and an unreadable weight state stay pending (nothing
// published, the reservation kept).
func TestValidateWeightRefusalsSettleReadFailuresStayPending(t *testing.T) {
	t.Parallel()
	refused := newValidateBed(t)
	refused.reserved("cadence-run-7", true)
	refused.outcome["cadence-run-7"] = RunOutcome{Usable: true, Result: greenResult("attempt-7")}
	refused.discharge = &WeightRefusal{Code: WeightResetNotAllowed, Message: "not allowed"}
	if outcome := refused.validate(false); outcome.Result != ValidateFinalized || outcome.Discharged {
		t.Fatalf("typed refusal: outcome = %+v", outcome)
	}
	refused.wrote(t, "discharge cadence-run-7", "publish cadence-run-7", "clear cadence-run-7")

	failed := newValidateBed(t)
	failed.reserved("cadence-run-7", true)
	failed.outcome["cadence-run-7"] = RunOutcome{Usable: true, Result: greenResult("attempt-7")}
	failed.discharge = errors.New("the weight lock can't be taken")
	if outcome := failed.validate(false); outcome.Result != ValidatePending || outcome.Code != CodeValidatePending || failed.reservation == nil {
		t.Fatalf("uncoded discharge error: outcome = %+v", outcome)
	}
	failed.wrote(t, "discharge cadence-run-7")

	unreadable := newValidateBed(t)
	unreadable.reserved("cadence-run-7", true)
	unreadable.outcome["cadence-run-7"] = RunOutcome{Usable: true, Result: greenResult("attempt-7")}
	unreadable.weightErr = errors.New("validation weight has trailing JSON")
	if outcome := unreadable.validate(false); outcome.Result != ValidatePending || unreadable.reservation == nil {
		t.Fatalf("unreadable weight: outcome = %+v", outcome)
	}
	unreadable.wrote(t)
}

// A discharge already recorded for the run (a crash between the discharge
// and the publication) is not made twice.
func TestValidateFinalizeAcrossDischargeCrash(t *testing.T) {
	t.Parallel()
	bed := newValidateBed(t)
	bed.reserved("cadence-run-7", true)
	bed.weight = WeightState{Generation: 4, ConsumedProofs: []ConsumedProof{{RunID: "cadence-run-7", WeightGeneration: 3}}}
	bed.outcome["cadence-run-7"] = RunOutcome{Usable: true, Result: greenResult("attempt-7")}
	if outcome := bed.validate(false); outcome.Result != ValidateFinalized {
		t.Fatalf("outcome = %+v", outcome)
	}
	bed.wrote(t, "publish cadence-run-7", "clear cadence-run-7")
}

// Unavailable writes nothing: no standing authority, no plan, or a launch
// that failed leave main untouched (no claim, no publication).
func TestValidateUnavailableWritesNothing(t *testing.T) {
	t.Parallel()
	gap := newValidateBed(t)
	gap.gap = errors.New("goal standing-validation is not approved, so no validation runs; run: metasystem goal approve standing-validation")
	if outcome := gap.validate(false); outcome.Result != ValidateUnavailable || outcome.Code != CodeValidateAuthority || !strings.Contains(outcome.Reason, "metasystem goal approve standing-validation") {
		t.Fatalf("gap: outcome = %+v", outcome)
	}
	gap.wrote(t)

	plan := newValidateBed(t)
	plan.planErr = errors.New("the trunk can't be fetched")
	if outcome := plan.validate(false); outcome.Result != ValidateUnavailable {
		t.Fatalf("plan: outcome = %+v", outcome)
	}
	plan.wrote(t)

	launch := newValidateBed(t)
	launch.launchErr = errors.New("fork failed")
	if outcome := launch.validate(false); outcome.Result != ValidateUnavailable || len(launch.published) != 0 {
		t.Fatalf("launch: outcome = %+v", outcome)
	}
	launch.wrote(t, "claim", "reserve cadence-run-1", "launch cadence-run-1")
}

// A settled run without a usable result is cleared and run again under a
// new run id; the old run is never reused.
func TestValidateSettledAttemptRetriesWithNewRunID(t *testing.T) {
	t.Parallel()
	bed := newValidateBed(t)
	bed.reserved("cadence-run-7", true)
	bed.outcome["cadence-run-7"] = RunOutcome{Why: "the run ended launch-failed"}
	outcome := bed.validate(false)
	if outcome.Result != ValidateFinalized || outcome.RunID != "cadence-run-1" || outcome.Retried != "cadence-run-7" {
		t.Fatalf("outcome = %+v", outcome)
	}
	bed.wrote(t, "clear cadence-run-7", "claim", "reserve cadence-run-1", "launch cadence-run-1", "record cadence-run-1 validate-cadence-run-1",
		"discharge cadence-run-1", "publish cadence-run-1", "clear cadence-run-1")
}

// Custody that can't be read blocks a new validation and the finalization
// of a reserved one, until a person forces it; live custody elsewhere
// makes a new validation wait, force or not.
func TestValidateUnknownCustodyNeedsAPerson(t *testing.T) {
	t.Parallel()
	unknown := newValidateBed(t)
	unknown.barrierUnk = []string{"prove b1 (prove-1): its launcher died before it recorded what it started"}
	if outcome := unknown.validate(false); outcome.Result != ValidateUnavailable || outcome.Code != CodeValidateCustodyUnknown {
		t.Fatalf("unknown: outcome = %+v", outcome)
	}
	unknown.wrote(t)
	if outcome := unknown.validate(true); outcome.Result != ValidateFinalized {
		t.Fatalf("forced: outcome = %+v", outcome)
	}

	reserved := newValidateBed(t)
	reserved.reserved("cadence-run-7", true)
	reserved.custody["cadence-run-7"] = CustodyUnknown
	reserved.outcome["cadence-run-7"] = RunOutcome{Usable: true, Result: greenResult("attempt-7")}
	if outcome := reserved.validate(false); outcome.Result != ValidateUnavailable || outcome.Code != CodeValidateCustodyUnknown || reserved.reservation == nil {
		t.Fatalf("reserved unknown: outcome = %+v", outcome)
	}
	reserved.wrote(t)

	live := newValidateBed(t)
	live.barrierLive = []string{"prove b1 (prove-1): its process (pid 42) still runs"}
	for _, force := range []bool{false, true} {
		if outcome := live.validate(force); outcome.Result != ValidateWaiting || outcome.Code != CodeValidateCustodyLive {
			t.Fatalf("live, force %v: outcome = %+v", force, outcome)
		}
	}
	live.wrote(t)
}

// A reserved run whose own process ended can still have left custody (a
// lease its descendants hold): it is not finalized while that runs.
func TestValidateWaitsForCustodyTheRunLeft(t *testing.T) {
	t.Parallel()
	bed := newValidateBed(t)
	bed.reserved("cadence-run-7", true)
	bed.outcome["cadence-run-7"] = RunOutcome{Usable: true, Result: greenResult("attempt-7")}
	bed.barrierLive = []string{"proof lease lease-heavy-1: a live owner, custodian or worker holds the lease"}
	if outcome := bed.validate(false); outcome.Result != ValidateWaiting || bed.reservation == nil {
		t.Fatalf("outcome = %+v", outcome)
	}
	bed.wrote(t)
}

// Not due: nothing runs and nothing is written.
func TestValidateNotDueWritesNothing(t *testing.T) {
	t.Parallel()
	bed := newValidateBed(t)
	bed.plan.Due = false
	if outcome := bed.validate(false); outcome.Result != ValidateNotDue {
		t.Fatalf("outcome = %+v", outcome)
	}
	bed.wrote(t)
}
