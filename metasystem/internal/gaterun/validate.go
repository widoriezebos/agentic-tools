package gaterun

// landing validate's decision (lane runtime design r10 §4, with the r5
// Finalize sequence it keeps). One call reads the durable reservation
// first, so the same function serves a fresh call and a call after a
// crash:
//
//   - a reserved run whose custody is live is attached: the call waits
//     for it and then finalizes it;
//   - a reserved run whose custody settled and whose run ended green or red
//     with a readable result is finalized: a green run with weight due
//     discharges it, a red run publishes without a reset, a moved weight
//     generation or a typed WEIGHT_* refusal settles without claiming a
//     reset, and an unreadable weight state stays pending;
//   - a reserved run whose custody settled without a usable result is
//     cleared and retried under a new run id;
//   - with nothing reserved, a key the ledger already completed is
//     returned, never run again; a due key is reserved durably before
//     anything launches.
//
// Custody that can't be read refuses the call unless a person forces it.
// Unavailable (no standing authority, a plan that can't be made, a launch
// that failed) writes nothing to main.

import (
	"errors"
	"fmt"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

// Validation is landing validate's durable reservation: the key and run id
// recorded before the run launches, the custody record of its execution,
// and what finalizing it needs after a restart.
type Validation struct {
	Key   goal.CadenceClaimKey `json:"key"`
	RunID string               `json:"runId"`
	// Custody is the run's custody record; "" until the launch recorded it.
	Custody string `json:"custody,omitempty"`
	// Discharge: the weight was due when the run was reserved.
	Discharge   bool                   `json:"discharge"`
	Trunk       CadenceTrunk           `json:"trunk"`
	Trigger     goal.CadenceTrigger    `json:"trigger"`
	ForceGroups bool                   `json:"forceGroups,omitempty"`
	Authority   CadenceAuthority       `json:"authority"`
	DeepOnly    []string               `json:"deepOnly"`
	Probes      []proofrun.GroupResult `json:"probes"`
	ReservedAt  string                 `json:"reservedAt"`
}

// ValidationPlan is what a due check found: the trunk, its key, whether a
// run is due and why, and the revalidated deep-only groups.
type ValidationPlan struct {
	Trunk       CadenceTrunk
	Key         goal.CadenceClaimKey
	Due         bool
	Trigger     goal.CadenceTrigger
	ForceGroups bool
	WeightDue   bool
	DeepOnly    []string
	Probes      []proofrun.GroupResult
}

// PlanValidation decides the key and whether a validation is due, from the
// latest published status, the weight and the revalidated groups.
func PlanValidation(now time.Time, trunk CadenceTrunk, latest *goal.CadenceStatus, weight WeightState, weightDue bool, deepOnly []string, revalidation CadenceRevalidation) (ValidationPlan, error) {
	if !cadenceObjectID(trunk.Commit) || !cadenceObjectID(trunk.Tree) {
		return ValidationPlan{}, fmt.Errorf("fetched cadence trunk is incomplete")
	}
	if len(deepOnly) == 0 {
		return ValidationPlan{}, fmt.Errorf("the deep-only validation inventory is empty")
	}
	probes, err := cadenceProbeMap(deepOnly, revalidation.Groups)
	if err != nil {
		return ValidationPlan{}, err
	}
	trigger, forceGroups, window, due, err := cadenceTrigger(now.UTC().Truncate(time.Second), latest, weightDue, deepOnly, probes)
	if err != nil {
		return ValidationPlan{}, err
	}
	plan := ValidationPlan{Trunk: trunk, Key: goal.CadenceClaimKey{TrunkTree: trunk.Tree, WeightGeneration: weight.Generation, ForcedWindowStart: window},
		Due: due, Trigger: trigger, ForceGroups: forceGroups, WeightDue: weightDue, DeepOnly: deepOnly}
	for _, id := range deepOnly {
		plan.Probes = append(plan.Probes, probes[id])
	}
	return plan, nil
}

// The states of a reserved run's custody (custody.Live/Dead/Unknown).
const (
	CustodyLive    = "live"
	CustodyDead    = "dead"
	CustodyUnknown = "unknown"
)

// RunOutcome is how a settled run ended, read from the run store and its
// result file.
type RunOutcome struct {
	// Usable: the run is terminal and its result is readable; Result is it.
	Usable bool
	Result proofrun.TestResult
	// Why says why it is not usable (launch-failed, ended-unknown, an
	// unreadable record or result).
	Why string
}

// ValidateSeams are landing validate's reads and effects.
type ValidateSeams struct {
	Clock func() time.Time
	// Reservation reads the durable reservation; nil when none.
	Reservation func() (*Validation, error)
	// Reserve writes a reservation durably; it refuses when another is
	// there. Record updates the reservation of the same run id.
	Reserve func(Validation) error
	Record  func(Validation) error
	// Clear removes the reservation of runID only.
	Clear func(runID string) error
	// Custody reads a reserved run's custody: by its record, or, when the
	// launch did not get to record it, by the run id it was opened for.
	Custody func(Validation) (state, why string)
	// Barrier reads the lane's whole custody before a new run launches:
	// live and unknown lists (lane.Settlement).
	Barrier func() (live, unknown []string, err error)
	// Wait blocks while the run's custody is live.
	Wait func(Validation) error
	// Outcome reads how a settled run ended.
	Outcome func(Validation) RunOutcome
	// Authority reads the standing goal's authority gap (nil when a run
	// may be claimed under it) and claims it.
	Gap   func() error
	Claim func(time.Time) (CadenceAuthority, error)
	// Plan fetches the trunk and decides the key and whether a run is due.
	Plan func() (ValidationPlan, error)
	// Latest is the ledger's latest published status.
	Latest func() (*goal.CadenceStatus, error)
	// NewRunID mints a run id.
	NewRunID func() (string, error)
	// Launch starts the reserved run under custody and returns its custody
	// record id; it does not wait.
	Launch func(Validation) (string, error)
	// Weight reads the weight state; Discharge resets it for a green run.
	Weight    func() (WeightState, error)
	Discharge func(CadenceAuthority, string, time.Time) error
	// Publish publishes a status, idempotent by its key.
	Publish func(goal.CadenceClaimKey, goal.CadenceStatus, []goal.TrunkRedRecordGroup) error
}

// What landing validate did.
const (
	ValidateCompleted   = "completed"   // the key was already published; nothing ran
	ValidateFinalized   = "finalized"   // a run was finalized and published
	ValidateNotDue      = "not-due"     // nothing is due
	ValidatePending     = "pending"     // a finalization waits on a read that failed
	ValidateUnavailable = "unavailable" // it could not run; nothing was written to main
	ValidateWaiting     = "waiting"     // other custody is live; nothing new started
)

// The refusal codes of landing validate.
const (
	CodeValidateCustodyUnknown = "LANDING_VALIDATE_CUSTODY_UNKNOWN"
	CodeValidateCustodyLive    = "LANDING_VALIDATE_CUSTODY_LIVE"
	CodeValidateAuthority      = "LANDING_VALIDATE_AUTHORITY_UNAVAILABLE"
	CodeValidateUnavailable    = "LANDING_VALIDATE_UNAVAILABLE"
	CodeValidatePending        = "LANDING_VALIDATE_FINALIZE_PENDING"
)

// ValidateGap is why the standing validation authority can't carry a run:
// the plain reason and the one command that closes it.
type ValidateGap struct{ Reason, Command string }

func (gap *ValidateGap) Error() string { return gap.Reason }

// ValidateOutcome is what one landing validate call did.
type ValidateOutcome struct {
	Result string `json:"result"`
	Code   string `json:"code,omitempty"`
	Reason string `json:"reason,omitempty"`
	// Fix is the one command that closes an authority gap.
	Fix    string               `json:"fix,omitempty"`
	Key    goal.CadenceClaimKey `json:"key"`
	RunID  string               `json:"runId,omitempty"`
	Status *goal.CadenceStatus  `json:"status,omitempty"`
	// Discharged: this call reset the weight with the run.
	Discharged bool `json:"discharged,omitempty"`
	// Retried: a settled run without a result was cleared first.
	Retried  string   `json:"retried,omitempty"`
	Attached bool     `json:"attached,omitempty"`
	Live     []string `json:"live,omitempty"`
	Unknown  []string `json:"unknown,omitempty"`
}

// Validate is one landing validate. force is a person's word past custody
// that can't be read; it never goes past live custody.
func Validate(force bool, seams ValidateSeams) (ValidateOutcome, error) {
	reserved, err := seams.Reservation()
	if err != nil {
		return ValidateOutcome{Result: ValidateUnavailable, Code: CodeValidateUnavailable,
			Reason: "the validation reservation can't be read: " + err.Error()}, nil
	}
	retried := ""
	if reserved != nil {
		outcome, done, err := resume(*reserved, force, seams)
		if err != nil || done {
			return outcome, err
		}
		retried = reserved.RunID
	}
	outcome, err := fresh(force, seams)
	outcome.Retried = retried
	return outcome, err
}

// resume settles a reserved run: done is false when it was cleared for a
// retry.
func resume(reserved Validation, force bool, seams ValidateSeams) (ValidateOutcome, bool, error) {
	outcome := ValidateOutcome{Key: reserved.Key, RunID: reserved.RunID}
	state, why := seams.Custody(reserved)
	if state == CustodyLive {
		outcome.Attached = true
		if err := seams.Wait(reserved); err != nil {
			return outcome, true, err
		}
		state, why = seams.Custody(reserved)
	}
	switch {
	case state == CustodyLive:
		outcome.Result, outcome.Code, outcome.Reason = ValidateWaiting, CodeValidateCustodyLive, why
		return outcome, true, nil
	case state != CustodyDead && !force:
		outcome.Result, outcome.Code, outcome.Reason = ValidateUnavailable, CodeValidateCustodyUnknown, why
		outcome.Unknown = []string{why}
		return outcome, true, nil
	}
	// The whole lane's custody must be settled too: the run's descendants
	// may hold leases after its own process group emptied.
	live, unknown, err := seams.Barrier()
	if err != nil {
		unknown = append(unknown, err.Error())
	}
	if len(live) > 0 {
		outcome.Result, outcome.Code, outcome.Live = ValidateWaiting, CodeValidateCustodyLive, live
		outcome.Reason = "the run ended, but custody it may have left still runs"
		return outcome, true, nil
	}
	if len(unknown) > 0 && !force {
		outcome.Result, outcome.Code, outcome.Unknown = ValidateUnavailable, CodeValidateCustodyUnknown, unknown
		outcome.Reason = "the run ended, but whether custody it may have left still runs can't be read"
		return outcome, true, nil
	}
	ended := seams.Outcome(reserved)
	if !ended.Usable {
		// A settled attempt without a result retries under a new run id;
		// the old governed run is never reused.
		if err := seams.Clear(reserved.RunID); err != nil {
			return outcome, true, err
		}
		return outcome, false, nil
	}
	final, err := finalize(reserved, ended.Result, seams)
	final.Attached = outcome.Attached
	return final, true, err
}

// finalize is the one Finalize sequence, after a normal completion and
// after a restart.
func finalize(reserved Validation, result proofrun.TestResult, seams ValidateSeams) (ValidateOutcome, error) {
	outcome := ValidateOutcome{Key: reserved.Key, RunID: reserved.RunID}
	now := seams.Clock().UTC().Truncate(time.Second)
	green := cadenceRunGreen(result)
	if green && reserved.Discharge {
		discharged, pending, err := discharge(reserved, now, seams)
		if err != nil {
			return outcome, err
		}
		if pending != "" {
			outcome.Result, outcome.Code, outcome.Reason = ValidatePending, CodeValidatePending, pending
			return outcome, nil
		}
		outcome.Discharged = discharged
	}
	probes := map[string]proofrun.GroupResult{}
	for _, group := range reserved.Probes {
		probes[group.ID] = group
	}
	started, _ := time.Parse(time.RFC3339, reserved.ReservedAt)
	if started.IsZero() || now.Before(started) {
		started = now
	}
	status := cadenceStatus(reserved.Trunk, reserved.Trigger, reserved.RunID, result, started, now, reserved.Key, reserved.DeepOnly, probes)
	red := batch.RedGroupsToRecordGroups(batch.ResultToRedGroups(result))
	if err := seams.Publish(reserved.Key, status, red); err != nil {
		outcome.Result, outcome.Code = ValidatePending, CodeValidatePending
		outcome.Reason = "the validation result couldn't be published: " + err.Error()
		return outcome, nil
	}
	if err := seams.Clear(reserved.RunID); err != nil {
		return outcome, err
	}
	outcome.Result, outcome.Status = ValidateFinalized, &status
	return outcome, nil
}

// discharge resets the weight with a green run. pending names a read that
// failed (the finalization is tried again); a moved generation, a typed
// refusal or a discharge already recorded settle it without a reset.
func discharge(reserved Validation, now time.Time, seams ValidateSeams) (bool, string, error) {
	state, err := seams.Weight()
	if err != nil {
		return false, "the validation weight can't be read: " + err.Error(), nil
	}
	for _, proof := range state.ConsumedProofs {
		if proof.RunID == reserved.RunID {
			return false, "", nil
		}
	}
	if state.Generation != reserved.Key.WeightGeneration {
		// The run validated an earlier count: it is obsolete for the
		// weight, which stays due under its new key.
		return false, "", nil
	}
	err = seams.Discharge(reserved.Authority, reserved.RunID, now)
	var refusal *WeightRefusal
	switch {
	case err == nil:
		return true, "", nil
	case errors.As(err, &refusal):
		return false, "", nil
	default:
		return false, "the validation weight couldn't be reset: " + err.Error(), nil
	}
}

// fresh runs when nothing is reserved.
func fresh(force bool, seams ValidateSeams) (ValidateOutcome, error) {
	if gap := seams.Gap(); gap != nil {
		outcome := ValidateOutcome{Result: ValidateUnavailable, Code: CodeValidateAuthority, Reason: gap.Error()}
		var named *ValidateGap
		if errors.As(gap, &named) {
			outcome.Fix = named.Command
		}
		return outcome, nil
	}
	plan, err := seams.Plan()
	if err != nil {
		return ValidateOutcome{Result: ValidateUnavailable, Code: CodeValidateUnavailable, Reason: "the validation could not be planned: " + err.Error()}, nil
	}
	outcome := ValidateOutcome{Key: plan.Key}
	latest, err := seams.Latest()
	if err != nil {
		outcome.Result, outcome.Code, outcome.Reason = ValidateUnavailable, CodeValidateUnavailable, "the latest validation can't be read: "+err.Error()
		return outcome, nil
	}
	if latest != nil && latest.TrunkTree == plan.Key.TrunkTree && latest.WeightGeneration == plan.Key.WeightGeneration && latest.ForcedWindowStart == plan.Key.ForcedWindowStart {
		outcome.Result, outcome.Status, outcome.RunID = ValidateCompleted, latest, latest.RunID
		return outcome, nil
	}
	if !plan.Due {
		outcome.Result = ValidateNotDue
		return outcome, nil
	}
	live, unknown, err := seams.Barrier()
	if err != nil {
		unknown = append(unknown, err.Error())
	}
	if len(live) > 0 {
		outcome.Result, outcome.Code, outcome.Live = ValidateWaiting, CodeValidateCustodyLive, live
		outcome.Reason = "other landing work still runs, so no validation started"
		return outcome, nil
	}
	if len(unknown) > 0 && !force {
		outcome.Result, outcome.Code, outcome.Unknown = ValidateUnavailable, CodeValidateCustodyUnknown, unknown
		outcome.Reason = "whether other landing work still runs can't be read, so no validation started"
		return outcome, nil
	}
	now := seams.Clock().UTC().Truncate(time.Second)
	authority, err := seams.Claim(now)
	if err != nil || authority.GoalID == "" || authority.ObligationRevision == 0 {
		reason := "the standing validation authority couldn't be claimed"
		if err != nil {
			reason += ": " + err.Error()
		}
		outcome.Result, outcome.Code, outcome.Reason = ValidateUnavailable, CodeValidateAuthority, reason
		return outcome, nil
	}
	runID, err := seams.NewRunID()
	if err != nil {
		return outcome, err
	}
	reservation := Validation{Key: plan.Key, RunID: runID, Discharge: plan.WeightDue, Trunk: plan.Trunk, Trigger: plan.Trigger,
		ForceGroups: plan.ForceGroups, Authority: authority, DeepOnly: plan.DeepOnly, Probes: plan.Probes, ReservedAt: now.Format(time.RFC3339)}
	if err := seams.Reserve(reservation); err != nil {
		return outcome, err
	}
	outcome.RunID = runID
	custody, err := seams.Launch(reservation)
	if custody != "" {
		reservation.Custody = custody
		if recordErr := seams.Record(reservation); recordErr != nil {
			return outcome, errors.Join(err, recordErr)
		}
	}
	if err != nil {
		// Nothing started, or its start is in custody: the next call
		// settles the reservation by custody and retries.
		outcome.Result, outcome.Code = ValidateUnavailable, CodeValidateUnavailable
		outcome.Reason = "the validation run couldn't be started: " + err.Error()
		return outcome, nil
	}
	resumed, done, err := resume(reservation, force, seams)
	if err != nil || done {
		return resumed, err
	}
	resumed.Result, resumed.Code = ValidateUnavailable, CodeValidateUnavailable
	resumed.Reason = "the validation run ended without a result; the next landing validate runs it again"
	resumed.Retried = runID
	return resumed, nil
}
