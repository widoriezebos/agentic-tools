package dispatch

import (
	"fmt"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopfence"
)

// The pending-setup record writer lost its verb (dispatch build-setup) in
// a6bc7c7e0; the delegation path now publishes reservations. These tests keep
// it as the fixture that writes a reservation record in the shape the live
// readers consume.

// BuildSetup writes the pending-setup reservation record that reserves a job
// id before the full record is assembled. Cap authority is already final when
// this record is built, so publication immediately creates a complete
// attempt-and-minute spending fact. A non-empty parent marks a follow-up.
func BuildSetup(repoRoot, output, job, role, parent, mainID, claimEpoch, goalID string, goalRevision uint64, goalTier uint8, capResolution, machineID, approvedRef string) error {
	return buildSetupWithGoalReads(repoRoot, output, job, role, parent, mainID, claimEpoch, goalID, goalRevision, goalTier, capResolution, machineID, approvedRef, concreteGoalAdmissionReads())
}

func buildSetupWithGoalReads(repoRoot, output, job, role, parent, mainID, claimEpoch, goalID string, goalRevision uint64, goalTier uint8, capResolution, machineID, approvedRef string, reads goalAdmissionReads) error {
	fenceGeneration := int64(0)
	if repoRoot != "" {
		fence, err := stopfence.Read(repoRoot)
		if err != nil {
			return fmt.Errorf("cannot read the process-creation fence for setup: %w", err)
		}
		fenceGeneration = fence.Generation
	}
	gateWidth := ""
	if goalID != "" {
		gateWidth = "area"
		if binding, bindErr := resolveGoalBindingWithReads(repoRoot, goalID, time.Now().UTC(), reads); bindErr == nil && binding.Revision == goalRevision {
			gateWidth = binding.GateWidth
		}
	}
	epoch, err := nullableEpoch(claimEpoch)
	if err != nil {
		return err
	}
	revision, err := nullableGoalRevision(goalID, goalRevision)
	if err != nil {
		return err
	}
	tier, err := nullableGoalTier(goalID, goalTier)
	if err != nil {
		return err
	}
	if (goalID != "" && gateWidth != "area" && gateWidth != "full") || (goalID == "" && gateWidth != "") {
		return fmt.Errorf("goal-bound setup requires gateWidth area or full")
	}
	authority, err := readCapAuthority(capResolution)
	if err != nil {
		return err
	}
	if goalID == "" && machineID != "" {
		return fmt.Errorf("machineId requires a goalId")
	}
	capMinutes, ok := numInt(authority.capMin)
	if !ok || capMinutes < 1 {
		return fmt.Errorf("cap resolution has no positive capMin")
	}
	if approvedRef != "" && goalID == "" {
		return fmt.Errorf("approvedRef requires a goalId and positive goalRevision")
	}
	var approvalClaim *SliceApprovalClaim
	if approvedRef != "" {
		if repoRoot == "" {
			return fmt.Errorf("approvedRef requires the checkout root for approval proof")
		}
		approvalClaim, err = proveSliceApprovalClaim(repoRoot, uint64(capMinutes), approvedRef, goalID, goalRevision)
		if err != nil {
			return err
		}
	}
	record := map[string]any{
		"jobId":              job,
		"operationId":        job,
		"role":               role,
		"status":             "pending-setup",
		"phase":              "setup",
		"error":              nil,
		"mainId":             nullableString(mainID),
		"claimEpoch":         epoch,
		"goalId":             nullableString(goalID),
		"goalRevision":       revision,
		"goalTier":           tier,
		"gateWidth":          nullableString(gateWidth),
		"machineId":          nullableString(machineID),
		"approvedRef":        nullableString(approvedRef),
		"fenceGeneration":    fenceGeneration,
		"sliceApprovalClaim": sliceApprovalClaim(approvalClaim),
		"capMin":             authority.capMin,
		"createdAt":          nowISO(),
	}
	if parent != "" {
		record["parentJob"] = parent
	}
	return writeRecord(output, record)
}
