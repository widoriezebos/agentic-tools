package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// productionEarlySeams are the owner's early acts on the lane at root (D14,
// R27; U10b-3): the join's cheap phase on a waiting batch's recorded tip,
// and one early delivery proof of it.
func productionEarlySeams(root string) batch.EarlySeams {
	return batch.EarlySeams{
		Cheap: func(record batch.Record) (batch.EarlyResult, error) {
			return earlyCheapPhase(root, record, runBatchAdmissionOnTree)
		},
		Prove: func(record batch.Record) (batch.EarlyResult, error) {
			return earlyProof(root, record, productionBatchPlan, launchBatchTipProof)
		},
		Budget: func(record batch.Record) (bool, string) {
			now, err := goalCommandNow(batch.ModuleRoot(root))
			if err != nil {
				return false, "no early proof: the clock is unreadable (" + err.Error() + ")"
			}
			return earlyBudget(root, record, now, batchBudgetProjection, proofCostCap)
		},
	}
}

type batchAdmissionRun func(root, batchID, baseTree, goalID string, claim batch.Claim, tree, label string,
	episode func(batch.JoinAdmission, int64) (batch.JoinAdmission, error)) (batch.JoinAdmission, proofrun.TestResult, bool, error)

// earlyCheapPhase runs the join's cheap phase once over the tip the joins
// recorded, which no join ran: each join proves its unit's tree alone. It is
// charged to the head member like the tip proof, and a fresh group gets an
// episode of its own that nothing retains.
func earlyCheapPhase(root string, record batch.Record, run batchAdmissionRun) (batch.EarlyResult, error) {
	head, _, err := earlyHead(record)
	if err != nil {
		return batch.EarlyResult{}, err
	}
	charge, err := batchChargeID(root, head, nil)
	if err != nil {
		return batch.EarlyResult{}, err
	}
	result, proof, _, err := run(root, record.BatchID, record.BaseTree, charge, head.Claim, record.TipTree, "early-cheap",
		func(decision batch.JoinAdmission, maxAgeMS int64) (batch.JoinAdmission, error) {
			token, err := newTestingFreshEpisode()
			if err != nil {
				return batch.JoinAdmission{}, err
			}
			now, err := goalCommandNow(batch.ModuleRoot(root))
			if err != nil {
				return batch.JoinAdmission{}, err
			}
			decision.FreshEpisode = token
			decision.FreshExpiresAt = now.Add(time.Duration(maxAgeMS) * time.Millisecond).UTC().Format(time.RFC3339Nano)
			return decision, nil
		})
	var red *batch.JoinAdmissionRed
	if errors.As(err, &red) {
		return batch.EarlyResult{Attempt: proof.AttemptID, Failing: batchRedAdapters(root, batch.ResultToRedGroups(proof))}, nil
	}
	if err != nil {
		return batch.EarlyResult{}, err
	}
	return batch.EarlyResult{Attempt: result.AttemptID}, nil
}

func earlyHead(record batch.Record) (batch.Unit, []batch.Unit, error) {
	joined := slices.DeleteFunc(slices.Clone(record.Units), func(unit batch.Unit) bool { return unit.State != batch.UnitJoined })
	if len(joined) == 0 {
		return batch.Unit{}, nil, fmt.Errorf("batch %s has no joined member", record.BatchID)
	}
	// The early acts are charged as the tip proof is: to the last goal
	// member, or, for a batch of changes, to the lane (U11b).
	return batch.ChargeUnit(joined), joined, nil
}

// earlyProof is one delivery attempt on the waiting batch's recorded tip,
// launched exactly as the tip proof is (its own detached checkout, the head
// member's claim revisions, the members' union plan) with no diagnostic
// headroom reserved, since it is nobody's tip. No seal, no proof record, no
// token: the batch proof takes from it only what identity-exact reuse takes.
func earlyProof(root string, record batch.Record, plan func(string, string, string, testpolicy.Mode) (testpolicy.Plan, error),
	launch func(batchProofLaunch) (proofrun.TestResult, error)) (batch.EarlyResult, error) {
	head, joined, err := earlyHead(record)
	if err != nil {
		return batch.EarlyResult{}, err
	}
	charge, err := batchChargeID(root, head, nil)
	if err != nil {
		return batch.EarlyResult{}, err
	}
	union, err := planBatchMemberUnion(root, record.TipTree, joined, charge, testpolicy.ModeAuto, plan)
	if err == nil && union.RequiredMode == testpolicy.ModeDeep {
		union, err = planBatchMemberUnion(root, record.TipTree, joined, charge, testpolicy.ModeDeep, plan)
	}
	if err != nil {
		return batch.EarlyResult{}, err
	}
	controlRoot := batch.ModuleRoot(root)
	resultPath := filepath.Join(controlRoot, "artifacts", "agents", "proof-runs", "batch", record.BatchID+"-early.json")
	if err := os.MkdirAll(filepath.Dir(resultPath), 0o700); err != nil {
		return batch.EarlyResult{}, err
	}
	if err := os.Remove(resultPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return batch.EarlyResult{}, err
	}
	result, launchErr := launch(batchProofLaunch{Root: controlRoot, BatchID: record.BatchID, GoalID: charge, Tree: record.TipTree,
		ResultPath: resultPath, Mode: union.ExecutedMode, Groups: slices.Clone(union.SelectedGroups),
		GoalRevision: head.Claim.Revision, AccountingRevision: head.Claim.AccountingRevision, Early: true})
	early := batch.EarlyResult{Attempt: result.AttemptID, Failing: batchRedAdapters(root, batch.ResultToRedGroups(result))}
	if launchErr != nil && len(early.Failing) == 0 {
		return early, launchErr
	}
	return early, nil
}

// earlyBudget admits an early act (the cheap phase or the early proof) only
// when the head member's budget, the one every early act is charged to,
// still leaves the batch proof its two attempts and its reserved minutes of
// diagnostic headroom after one more attempt (U3-01; the tip proof's own
// admission needs two); otherwise the act is skipped and the line says why.
func earlyBudget(root string, record batch.Record, at time.Time, project func(string, batch.Unit, *batch.Unit, time.Time) (batchCostBudgetProjection, error),
	capMinutes func(string) (uint64, error)) (bool, string) {
	head, _, err := earlyHead(record)
	if err != nil {
		return false, "no early work: " + err.Error()
	}
	if head.IsChange() {
		// A batch of changes is charged to the lane, which has no box.
		return true, ""
	}
	view, err := project(root, head, nil, at)
	if err != nil {
		return false, "no early work: " + head.GoalID + "'s budget is unreadable (" + err.Error() + ")"
	}
	projection := view.Budget
	if projection.Status != dispatchcore.BudgetKnown {
		return false, "no early work: " + head.GoalID + "'s budget is unknown"
	}
	if !view.LandingClaim && (projection.ElapsedState != "" || projection.Elapsed >= projection.Limits.ElapsedDuration()) {
		return false, "no early work: " + head.GoalID + "'s elapsed budget is spent"
	}
	cap, err := capMinutes(root)
	if err != nil {
		return false, "no early work: the proof cap is unreadable (" + err.Error() + ")"
	}
	attempts := saturatingLeft(projection.Limits.AttemptLimit, projection.Attempts)
	minutes := saturatingLeft(projection.Limits.ReservedJobMinutesLimit, projection.ReservedJobMinutes)
	if attempts < 3 || minutes < 3*cap {
		return false, fmt.Sprintf("no early work: %s has %d attempts and %d reserved minutes left, kept for the batch proof", head.GoalID, attempts, minutes)
	}
	return true, ""
}

// batchTipRetryAttempts reads the retained proof store the tip proof's retry
// decision is looked up in.
var batchTipRetryAttempts = proofrun.ReadAttempts

// tipRetryDecision writes the accountable retry decision for a batch proof
// whose tree an earlier attempt of its head member already failed, the
// batch's early proof above all (U3-03): the shared-component admission then
// re-executes that failed producer's group instead of refusing, and never
// reuses the red. It is read from the retained proof store, the newest such
// attempt, so it holds whatever the batch record kept (a red that ended after
// the start, an owner restart). Empty when no attempt on the tree failed.
func tipRetryDecision(controlRoot string, record batch.Record, charge string, readAttempts func(string) ([]proofrun.Attempt, error)) (string, error) {
	attempts, err := readAttempts(controlRoot)
	if err != nil {
		return "", err
	}
	var prior proofrun.Attempt
	group := ""
	for _, attempt := range attempts {
		tree, known := attempt.CandidateTreeDigest()
		if !known || tree != record.TipTree || attempt.AccountedGoal() != charge || attempt.Terminal == nil || attempt.TestResult == nil ||
			prior.AttemptID != "" && attempt.TestAdmission <= prior.TestAdmission {
			continue
		}
		for _, result := range attempt.TestResult.Groups {
			if result.Status == "failed" {
				prior, group = attempt, result.ID
				break
			}
		}
	}
	if prior.AttemptID == "" {
		return "", nil
	}
	evidence, err := proofrun.AttemptPath(controlRoot, prior.AttemptID)
	if err != nil {
		return "", err
	}
	decision, err := json.Marshal(proofrun.RetryDecision{SchemaVersion: 1, PriorAttempt: prior.AttemptID,
		Cause:        "attempt " + prior.AttemptID + " failed " + group + " on the tree of batch " + record.BatchID + ", its early proof's or an earlier one's",
		EvidencePath: evidence,
		Rationale:    "the batch proof re-executes what an earlier attempt of the identical tree failed; a red is never reused (R27, U3-03)"})
	if err != nil {
		return "", err
	}
	path := filepath.Join(controlRoot, "artifacts", "agents", "proof-runs", "batch", record.BatchID+"-tip-retry.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	return path, os.WriteFile(path, decision, 0o600)
}
