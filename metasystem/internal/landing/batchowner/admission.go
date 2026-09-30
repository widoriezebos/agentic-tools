package batchowner

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

	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/strictjson"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/verbresult"
)

var batchJoinAdmissionExecutable = os.Executable

// productionJoinAdmission runs the selected cheap phase after the member's
// claim reaches the landing control root. The complete delivery selection is
// still held by the joining unit for every later prefix and tip decision.
func productionJoinAdmission(root, batchID string, unit batch.Unit) (batch.JoinAdmission, error) {
	record, err := batch.NewStore(root, nil).Load(batchID)
	if err != nil {
		return batch.JoinAdmission{}, err
	}
	if err := AuthorizeBatchMember(root, batch.Record{BatchID: batchID, Seal: map[string]batch.Claim{unit.GoalID: unit.Claim}}, unit); err != nil {
		return batch.JoinAdmission{}, err
	}
	if unit.Admission == nil || unit.Admission.Tree == "" {
		return batch.JoinAdmission{}, fmt.Errorf("%s: %s has no exact admission tree", codeJoinTestDropped, unit.GoalID)
	}
	result, _, ran, err := runBatchAdmissionOnTree(root, batchID, record.BaseTree, unit.GoalID, unit.Claim, unit.Admission.Tree, "join-"+unit.GoalID,
		func(decision batch.JoinAdmission, maxAgeMS int64) (batch.JoinAdmission, error) {
			return retainJoinEpisode(root, batch.NewStore(root, nil), batchID, unit, decision, maxAgeMS)
		})
	if err != nil || !ran {
		return result, err
	}
	if err := AuthorizeBatchMember(root, batch.Record{BatchID: batchID, Seal: map[string]batch.Claim{unit.GoalID: unit.Claim}}, unit); err != nil {
		return batch.JoinAdmission{}, err
	}
	return result, nil
}

// runBatchAdmissionOnTree runs the join's cheap phase, the admission subset
// of goalID's delivery plan, on one exact tree in a detached worktree of its
// own, and verifies it through the retained verifier. ran is false when the
// plan selects no admission group. A red is a *batch.JoinAdmissionRed with
// the run's result beside it; episode binds fresh groups to an episode.
func runBatchAdmissionOnTree(root, batchID, baseTree, goalID string, claim batch.Claim, tree, label string,
	episode func(batch.JoinAdmission, int64) (batch.JoinAdmission, error)) (batch.JoinAdmission, proofrun.TestResult, bool, error) {
	controlRoot := batch.ModuleRoot(root)
	planned, err := productionBatchTreePlanOutput(root, goalID, tree, testpolicy.ModeAuto)
	if err != nil {
		return batch.JoinAdmission{}, proofrun.TestResult{}, false, err
	}
	contract := testpolicy.Contract{SchemaVersion: testpolicy.SchemaVersion, Groups: planned.Groups}
	if slices.ContainsFunc(planned.Groups, func(group testpolicy.Group) bool { return group.Phase != "" }) {
		contract.SchemaVersion = testpolicy.ExecutionContractSchemaVersion
	}
	admission, err := testpolicy.AdmissionPlan(contract, planned.Plan)
	if err != nil {
		return batch.JoinAdmission{}, proofrun.TestResult{}, false, err
	}
	result := batch.JoinAdmission{Tree: tree, Status: "pending"}
	if len(admission.SelectedGroups) == 0 {
		result.Status = "verified"
		return result, proofrun.TestResult{}, false, nil
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
			return batch.JoinAdmission{}, proofrun.TestResult{}, false, fmt.Errorf("%s: fresh admission group %s has no positive max age", codeJoinTestDropped, id)
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
	}{batchID, baseTree, tree, planned.PolicyBaseCommit, planned.ContractDigest, planned.BaseContractDigest,
		claim, planned.Plan, planned.Groups, admission})
	digest := sha256.Sum256(decisionBytes)
	result.DecisionID = hex.EncodeToString(digest[:])
	if fresh {
		result, err = episode(result, maxAgeMS)
		if err != nil {
			return batch.JoinAdmission{}, proofrun.TestResult{}, false, err
		}
	}
	detached, err := (gittree.Workspace{Dir: root}).NewDetachedWorktree(tree)
	if err != nil {
		return batch.JoinAdmission{}, proofrun.TestResult{}, false, err
	}
	defer detached.Close()
	executionRoot := batch.ModuleRoot(detached.Workspace().Dir)
	binary, err := batchJoinAdmissionExecutable()
	if err != nil {
		return batch.JoinAdmission{}, proofrun.TestResult{}, false, err
	}
	resultDir := filepath.Join(controlRoot, "artifacts", "agents", "proof-runs", "batch")
	if err := os.MkdirAll(resultDir, 0o700); err != nil {
		return batch.JoinAdmission{}, proofrun.TestResult{}, false, err
	}
	projection, err := os.CreateTemp(resultDir, batchID+"-"+label+"-*.json")
	if err != nil {
		return batch.JoinAdmission{}, proofrun.TestResult{}, false, err
	}
	result.ResultPath = projection.Name()
	_ = projection.Close()
	_ = os.Remove(result.ResultPath)
	args := append(append([]string{"internal", "test", "run", "--root", executionRoot, "--control-root", controlRoot, "--batch-admission"},
		accountFlag(goalID)...), "--tree", tree, "--mode", "auto", "--purpose", "delivery", "--result", result.ResultPath)
	args = append(args, accountRevisions(goalID, claim)...)
	if result.FreshEpisode != "" {
		args = append(args, "--fresh-episode", result.FreshEpisode, "--fresh-expires-at", result.FreshExpiresAt)
	}
	command := exec.Command(binary, append(args, "--json")...)
	command.Dir, command.Env = executionRoot, append(gittree.ScrubbedEnviron(), "METASYSTEM_OWNER_LINEAGE="+LandingOwnerLineage)
	child, err := runTestRunChild(command)
	if err != nil {
		return batch.JoinAdmission{}, proofrun.TestResult{}, false, fmt.Errorf("%s: %w", codeJoinTestDropped, err)
	}
	if child.Outcome == verbresult.Refused {
		return batch.JoinAdmission{}, proofrun.TestResult{}, false, JoinAdmissionRefusal(child)
	}
	var proof proofrun.TestResult
	if err := strictjson.Read(result.ResultPath, &proof); err != nil {
		return batch.JoinAdmission{}, proofrun.TestResult{}, false, fmt.Errorf("%s: read admission result: %w; %s", codeJoinTestDropped, err, child.Summary)
	}
	if !BatchProofOutcomeAccepted(child, proof) {
		if len(proof.Delivery.FailingGroups) != 0 {
			return batch.JoinAdmission{}, proof, true, &batch.JoinAdmissionRed{Reason: "BATCH_JOIN_ADMISSION_RED: " + strings.Join(proof.Delivery.FailingGroups, ",")}
		}
		return batch.JoinAdmission{}, proof, true, fmt.Errorf("%s: %w", codeJoinTestDropped, child.Err())
	}
	request := testrun.SelectionRequest{Root: executionRoot, ControlRoot: controlRoot, GoalID: goalID, Tree: tree,
		Mode: testpolicy.ModeAuto, Purpose: testpolicy.PurposeDelivery, BatchAdmission: true,
		FreshEpisode: result.FreshEpisode, FreshExpiresAt: result.FreshExpiresAt, ExecutedWorkers: proof.Workers}
	verified, err := Engine.VerifyRetainedTesting(request)
	if err != nil || !verified.Delivery.Sufficient {
		return batch.JoinAdmission{}, proof, true, fmt.Errorf("%s: retained admission is incomplete: %w; missing=%v", codeJoinTestDropped, err, verified.Delivery.MissingGroups)
	}
	result.AttemptID, result.Status = proof.AttemptID, "verified"
	return result, proof, true, nil
}

// JoinAdmissionRefusal is the batch's refusal for a refused admission run.
func JoinAdmissionRefusal(child verbresult.Result) error {
	return admissionRefusal(child)
}

func retainJoinEpisode(root string, store batch.Store, batchID string, unit batch.Unit, decision batch.JoinAdmission, maxAgeMS int64) (batch.JoinAdmission, error) {
	now, err := fixtureauth.GoalNow(batch.ModuleRoot(root))
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
				return fmt.Errorf("%s: %s changed before episode retention", codeJoinPending, unit.GoalID)
			}
			if current.Admission.DecisionID == decision.DecisionID && current.Admission.FreshEpisode != "" {
				if expiry, parseErr := time.Parse(time.RFC3339Nano, current.Admission.FreshExpiresAt); parseErr == nil && now.Before(expiry) {
					decision.FreshEpisode, decision.FreshExpiresAt = current.Admission.FreshEpisode, current.Admission.FreshExpiresAt
					return nil
				}
			}
			token, tokenErr := testrun.NewFreshEpisode()
			if tokenErr != nil {
				return tokenErr
			}
			decision.FreshEpisode = token
			decision.FreshExpiresAt = now.Add(time.Duration(maxAgeMS) * time.Millisecond).UTC().Format(time.RFC3339Nano)
			current.Admission.DecisionID, current.Admission.FreshEpisode, current.Admission.FreshExpiresAt = decision.DecisionID, decision.FreshEpisode, decision.FreshExpiresAt
			return nil
		}
		return fmt.Errorf("%s: %s is absent", codeJoinPending, unit.GoalID)
	})
	return decision, err
}
