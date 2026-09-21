package main

import (
	"fmt"
	"os"
	"strings"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// coldBuildBudgetRefusal is a performance preflight, never a reservation.
// The final locked proof admission remains the authority for starting work.
type coldBuildBudgetRefusal struct{ detail string }

func (refusal *coldBuildBudgetRefusal) Error() string { return "BUDGET_REFUSED: " + refusal.detail }

// refuseKnownColdBuildBudget only stops a cold candidate-engine build when a
// selected group has no retained successful observation at all. In that case
// the existing attempt-based reuse path cannot provide the whole selection.
// Unknown evidence, an available budget extension, or a transient active-job
// limit leaves the decision to the ordinary final admission.
func refuseKnownColdBuildBudget(prepared testingPreparation, request testingSelectionRequest) error {
	if prepared.GoalID == "" || len(prepared.Plan.SelectedGroups) == 0 ||
		os.Getenv("METASYSTEM_PROOF_CONTROL_ROOT") != "" || os.Getenv("METASYSTEM_PROOF_ATTEMPT") != "" ||
		os.Getenv("METASYSTEM_PROOF_RUN_ROOT") != "" || os.Getenv("METASYSTEM_PROOF_RUN_ID") != "" ||
		os.Getenv("METASYSTEM_HOOK_DELEGATE_STATE_ROOT") != "" || os.Getenv("METASYSTEM_HOOK_DELEGATE_INSTALLATION_ROOT") != "" ||
		os.Getenv("METASYSTEM_HOOK_DELEGATE_JOB") != "" {
		return nil
	}
	attempts, err := proofrun.ReadAttempts(prepared.proofControlRoot())
	if err != nil || retainedSuccessCouldCoverSelection(attempts, prepared.Plan.SelectedGroups, request) {
		return nil
	}
	now, err := goalCommandNow(prepared.proofControlRoot())
	if err != nil {
		return nil
	}
	roles, err := resolveProofGoalRoles(prepared.proofControlRoot(), prepared.GoalID, request.AuthorityGoalID, now)
	if err != nil || roles.CandidateRevision != prepared.AccountingRevision {
		return nil
	}
	binding, err := dispatchcore.ResolveGoalBinding(prepared.proofControlRoot(), roles.Authority.Id, now)
	accountingRevision := uint64(0)
	if err == nil && binding.File.Claimed != nil {
		accountingRevision = binding.File.Claimed.AccountingRevision
		if accountingRevision == 0 {
			accountingRevision = binding.Revision
		}
	}
	if err != nil || request.ExpectedGoalRevision != 0 &&
		(binding.Revision != request.ExpectedGoalRevision ||
			accountingRevision != request.ExpectedAccountingRevision) {
		return nil
	}
	capMinutes, _, _, err := dispatchcore.ResolveCap(prepared.ConfPath, "proof", "main", "proof", "", request.CapMin)
	if err != nil || capMinutes < 1 {
		return nil
	}
	verdict, err := dispatchcore.EvaluateProofAdmissionForDispatch(prepared.proofControlRoot(), roles.Authority.Id,
		binding.Revision, roles.Candidate, roles.CandidateRevision, uint64(capMinutes), now,
		"implementer", "fresh", dispatchcore.HazardMechanical)
	if err != nil || !permanentBudgetRefusal(verdict.Authority) && !permanentBudgetRefusal(verdict.Candidate) {
		return nil
	}
	return &coldBuildBudgetRefusal{detail: fmt.Sprintf("goal %s before candidate-engine build: %s",
		prepared.GoalID, strings.Join(dispatchcore.FormatProofAdmission(verdict), "; "))}
}

func retainedSuccessCouldCoverSelection(attempts []proofrun.Attempt, groups []string, request testingSelectionRequest) bool {
	if request.NoReuse || request.ForceGroups || request.Purpose == testpolicy.PurposeCadence {
		return false
	}
	seen := make(map[string]bool, len(groups))
	for _, attempt := range attempts {
		if attempt.TestResult == nil {
			continue
		}
		for _, group := range attempt.TestResult.Groups {
			if group.CollectionComplete && (group.Status == "passed" || group.Status == "reused") {
				seen[group.ID] = true
			}
		}
	}
	for _, id := range groups {
		if !seen[id] {
			return false
		}
	}
	return true
}

func permanentBudgetRefusal(verdict dispatchcore.GoalRevisionAdmission) bool {
	if verdict.Refusal == nil || verdict.Refusal.Unknown != nil || verdict.Extension != nil {
		return false
	}
	for _, breach := range verdict.Refusal.Breaches {
		switch breach.Field {
		case "attemptLimit", "reservedJobMinutesLimit", "elapsedLimit":
			return true
		}
	}
	return false
}
