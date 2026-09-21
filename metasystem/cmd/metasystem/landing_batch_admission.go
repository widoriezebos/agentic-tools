package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

var batchJoinAdmissionExecutable = os.Executable

// productionJoinAdmission runs the selected cheap phase after the member's
// claim reaches the landing control root. The complete delivery selection is
// still held by the joining unit for every later prefix and tip decision.
func productionJoinAdmission(root, batchID string, unit batch.Unit) (batch.JoinAdmission, error) {
	controlRoot := batch.ModuleRoot(root)
	record, err := batch.NewStore(root, nil).Load(batchID)
	if err != nil {
		return batch.JoinAdmission{}, err
	}
	if err := authorizeBatchMember(root, batch.Record{BatchID: batchID, Seal: map[string]batch.Claim{unit.GoalID: unit.Claim}}, unit); err != nil {
		return batch.JoinAdmission{}, err
	}
	if unit.Admission == nil || unit.Admission.Tree == "" {
		return batch.JoinAdmission{}, fmt.Errorf("BATCH_JOIN_TEST_DROPPED: %s has no exact admission tree", unit.GoalID)
	}
	tree := unit.Admission.Tree
	planned, err := productionBatchTreePlanOutput(root, unit.GoalID, tree, testpolicy.ModeAuto)
	if err != nil {
		return batch.JoinAdmission{}, err
	}
	contract := testpolicy.Contract{SchemaVersion: testpolicy.SchemaVersion, Groups: planned.Groups}
	if slices.ContainsFunc(planned.Groups, func(group testpolicy.Group) bool { return group.Phase != "" }) {
		contract.SchemaVersion = testpolicy.ExecutionContractSchemaVersion
	}
	admission, err := testpolicy.AdmissionPlan(contract, planned.Plan)
	if err != nil {
		return batch.JoinAdmission{}, err
	}
	result := batch.JoinAdmission{Tree: tree, Status: "pending"}
	if len(admission.SelectedGroups) == 0 {
		result.Status = "verified"
		return result, nil
	}
	groupByID := map[string]testpolicy.Group{}
	for _, group := range planned.Groups {
		groupByID[group.ID] = group
	}
	maxAgeMS := int64(0)
	fresh := false
	for _, id := range admission.SelectedGroups {
		group := groupByID[id]
		if group.Freshness != "episode" {
			continue
		}
		fresh = true
		if group.FreshnessMaxAgeMS == nil || *group.FreshnessMaxAgeMS <= 0 {
			return batch.JoinAdmission{}, fmt.Errorf("BATCH_JOIN_TEST_DROPPED: fresh admission group %s has no positive max age", id)
		}
		if maxAgeMS == 0 || *group.FreshnessMaxAgeMS < maxAgeMS {
			maxAgeMS = *group.FreshnessMaxAgeMS
		}
	}
	decisionBytes, _ := json.Marshal(struct {
		BatchID, BaseTree, Tree, PolicyBaseCommit, ContractDigest, BaseContractDigest string
		Claim                                                                         batch.Claim
		Selection                                                                     testpolicy.Plan
		Groups                                                                        []testpolicy.Group
		Admission                                                                     testpolicy.Plan
	}{batchID, record.BaseTree, tree, planned.PolicyBaseCommit, planned.ContractDigest, planned.BaseContractDigest,
		unit.Claim, planned.Plan, planned.Groups, admission})
	digest := sha256.Sum256(decisionBytes)
	result.DecisionID = hex.EncodeToString(digest[:])
	if fresh {
		result, err = retainJoinEpisode(root, batch.NewStore(root, nil), batchID, unit, result, maxAgeMS)
		if err != nil {
			return batch.JoinAdmission{}, err
		}
	}
	detached, err := (gittree.Workspace{Dir: root}).NewDetachedWorktree(tree)
	if err != nil {
		return batch.JoinAdmission{}, err
	}
	defer detached.Close()
	executionRoot := batch.ModuleRoot(detached.Workspace().Dir)
	binary, err := batchJoinAdmissionExecutable()
	if err != nil {
		return batch.JoinAdmission{}, err
	}
	resultDir := filepath.Join(controlRoot, "artifacts", "agents", "proof-runs", "batch")
	if err := os.MkdirAll(resultDir, 0o700); err != nil {
		return batch.JoinAdmission{}, err
	}
	projection, err := os.CreateTemp(resultDir, batchID+"-join-"+unit.GoalID+"-*.json")
	if err != nil {
		return batch.JoinAdmission{}, err
	}
	result.ResultPath = projection.Name()
	_ = projection.Close()
	_ = os.Remove(result.ResultPath)
	args := []string{"test", "run", "--root", executionRoot, "--control-root", controlRoot, "--batch-admission",
		"--goal", unit.GoalID, "--tree", tree, "--mode", "auto", "--purpose", "delivery", "--result", result.ResultPath,
		"--expected-goal-revision", fmt.Sprint(unit.Claim.Revision), "--expected-accounting-revision", fmt.Sprint(unit.Claim.AccountingRevision)}
	if result.FreshEpisode != "" {
		args = append(args, "--fresh-episode", result.FreshEpisode, "--fresh-expires-at", result.FreshExpiresAt)
	}
	command := exec.Command(binary, args...)
	command.Dir, command.Env = executionRoot, append(gittree.ScrubbedEnviron(), "METASYSTEM_OWNER_LINEAGE="+landingOwnerLineage)
	output, runErr := command.CombinedOutput()
	if runErr != nil && command.ProcessState != nil && command.ProcessState.ExitCode() == proofrun.ExitAdmissionRefused {
		reason := strings.TrimSpace(string(output))
		code := batchAdmissionRefusalCode(reason)
		switch code {
		case "GOAL_REVISION_MOVED":
			return batch.JoinAdmission{}, &batch.PrefixRevisionRefusal{Reason: reason}
		case "BATCH_MEMBER_BUDGET_REFUSED", "BUDGET_REFUSED":
			return batch.JoinAdmission{}, &batch.PrefixBudgetRefusal{Reason: reason}
		case "CANDIDATE_GOAL_REFUSED":
			if strings.Contains(reason, "state=fenced") {
				return batch.JoinAdmission{}, &batch.PrefixFencedRefusal{Reason: reason}
			}
		}
		return batch.JoinAdmission{}, &batch.PrefixAdmissionRefusal{Code: code, Reason: reason}
	}
	var proof proofrun.TestResult
	if err := readStrictJSON(result.ResultPath, &proof); err != nil {
		return batch.JoinAdmission{}, fmt.Errorf("BATCH_JOIN_TEST_DROPPED: read admission result: %w; output=%s", err, strings.TrimSpace(string(output)))
	}
	if runErr != nil && (command.ProcessState == nil || !batchProofExitAccepted(command.ProcessState.ExitCode(), proof)) {
		if len(proof.Delivery.FailingGroups) != 0 {
			return batch.JoinAdmission{}, &batch.JoinAdmissionRed{Reason: "BATCH_JOIN_ADMISSION_RED: " + strings.Join(proof.Delivery.FailingGroups, ",")}
		}
		return batch.JoinAdmission{}, fmt.Errorf("BATCH_JOIN_TEST_DROPPED: %s: %w", strings.TrimSpace(string(output)), runErr)
	}
	request := testingSelectionRequest{Root: executionRoot, ControlRoot: controlRoot, GoalID: unit.GoalID, Tree: tree,
		Mode: testpolicy.ModeAuto, Purpose: testpolicy.PurposeDelivery, BatchAdmission: true,
		FreshEpisode: result.FreshEpisode, FreshExpiresAt: result.FreshExpiresAt}
	verified, err := verifyRetainedTesting(request)
	if err != nil || !verified.Delivery.Sufficient {
		return batch.JoinAdmission{}, fmt.Errorf("BATCH_JOIN_TEST_DROPPED: retained admission is incomplete: %w; missing=%v", err, verified.Delivery.MissingGroups)
	}
	if err := authorizeBatchMember(root, batch.Record{BatchID: batchID, Seal: map[string]batch.Claim{unit.GoalID: unit.Claim}}, unit); err != nil {
		return batch.JoinAdmission{}, err
	}
	result.AttemptID, result.Status = proof.AttemptID, "verified"
	return result, nil
}

func retainJoinEpisode(root string, store batch.Store, batchID string, unit batch.Unit, decision batch.JoinAdmission, maxAgeMS int64) (batch.JoinAdmission, error) {
	now, err := goalCommandNow(batch.ModuleRoot(root))
	if err != nil {
		return batch.JoinAdmission{}, err
	}
	err = store.Update(batchID, func(record *batch.Record) error {
		for index := range record.Units {
			current := &record.Units[index]
			if current.GoalID != unit.GoalID {
				continue
			}
			if current.State != batch.UnitJoining || current.Admission == nil || current.Admission.Tree != decision.Tree || current.Claim != unit.Claim {
				return fmt.Errorf("BATCH_JOIN_PENDING: %s changed before episode retention", unit.GoalID)
			}
			if current.Admission.DecisionID == decision.DecisionID && current.Admission.FreshEpisode != "" {
				if expiry, parseErr := time.Parse(time.RFC3339Nano, current.Admission.FreshExpiresAt); parseErr == nil && now.Before(expiry) {
					decision.FreshEpisode, decision.FreshExpiresAt = current.Admission.FreshEpisode, current.Admission.FreshExpiresAt
					return nil
				}
			}
			token, tokenErr := newTestingFreshEpisode()
			if tokenErr != nil {
				return tokenErr
			}
			decision.FreshEpisode = token
			decision.FreshExpiresAt = now.Add(time.Duration(maxAgeMS) * time.Millisecond).UTC().Format(time.RFC3339Nano)
			current.Admission.DecisionID, current.Admission.FreshEpisode, current.Admission.FreshExpiresAt = decision.DecisionID, decision.FreshEpisode, decision.FreshExpiresAt
			return nil
		}
		return fmt.Errorf("BATCH_JOIN_PENDING: %s is absent", unit.GoalID)
	})
	return decision, err
}
