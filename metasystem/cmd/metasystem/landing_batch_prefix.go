package main

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// A retained proof does not retain the right to land. Recheck the live
// handed-over claim and elapsed admission fence before consuming it.
func authorizeBatchMember(root string, record batch.Record, unit batch.Unit) error {
	controlRoot, now, projection, err := batchAuthorityProjection(root)
	if err != nil {
		return err
	}
	return authorizeBatchMemberInProjection(controlRoot, now, record, unit, projection)
}

func batchAuthorityProjection(root string) (string, time.Time, goal.Projection, error) {
	controlRoot := batch.ModuleRoot(root)
	now, err := goalCommandNow(controlRoot)
	if err != nil {
		return "", time.Time{}, goal.Projection{}, err
	}
	endpoint, err := goal.ResolveEndpoint(controlRoot)
	if err != nil {
		return "", time.Time{}, goal.Projection{}, err
	}
	// A handover or fence can advance the canonical ledger from another seat
	// while this checkout's accepted ref still names the previous claim.
	projection, err := goal.Project(endpoint, true, now)
	if err != nil {
		return "", time.Time{}, goal.Projection{}, err
	}
	return controlRoot, now, projection, nil
}

func authorizeBatchMemberInProjection(controlRoot string, now time.Time, record batch.Record, unit batch.Unit, projection goal.Projection) error {
	if projection.Tree == nil {
		return fmt.Errorf("BATCH_PREFIX_AUTHORITY_REFUSED: live goal projection is empty")
	}
	file := projection.Tree.Live[unit.GoalID]
	if file == nil || file.State != goal.StateClaimed || file.Claimed == nil {
		return &batch.PrefixRevisionRefusal{Reason: "BATCH_PREFIX_AUTHORITY_REFUSED: member " + unit.GoalID + " is no longer claimed"}
	}
	sealed, ok := record.Seal[unit.GoalID]
	claim := file.Claimed
	if !ok || sealed.Revision == 0 || sealed.AccountingRevision == 0 ||
		claim.Revision != sealed.Revision || claim.AccountingRevision != sealed.AccountingRevision ||
		claim.Revision != unit.Claim.Revision || claim.AccountingRevision != unit.Claim.AccountingRevision ||
		claim.HandedOver.Batch != record.BatchID || claim.HandedOver.FromMachine != unit.Claim.Machine ||
		claim.HandedOver.FromLineage != unit.Claim.Lineage || claim.HandedOver.FromEpoch != unit.Claim.Epoch {
		return &batch.PrefixRevisionRefusal{Reason: "BATCH_PREFIX_AUTHORITY_REFUSED: member " + unit.GoalID + " handed-over claim moved"}
	}
	if file.IsFencedClaim() {
		return &batch.PrefixFencedRefusal{Reason: "CANDIDATE_GOAL_REFUSED: candidate goal " + unit.GoalID + " state=fenced stopId=" + file.StopFence.StopID}
	}
	budget := dispatchcore.ProjectBudget(controlRoot, file, now)
	if budget.Status != dispatchcore.BudgetKnown {
		return &batch.PrefixAdmissionRefusal{Code: "BATCH_MEMBER_BUDGET_UNKNOWN", Reason: "BATCH_PREFIX_AUTHORITY_REFUSED: member " + unit.GoalID + " budget projection is unknown"}
	}
	if !file.IsLandingClaim() && (budget.ElapsedState == dispatchcore.AdmissionClosedElapsed || budget.ElapsedState == dispatchcore.ElapsedBreach) {
		return &batch.PrefixBudgetRefusal{Reason: "BATCH_MEMBER_BUDGET_REFUSED: member " + unit.GoalID + " elapsed admission budget closed"}
	}
	return nil
}

func authorizeBatchSeries(root string, store batch.Store, record batch.Record, actor string, at time.Time) error {
	if !slices.ContainsFunc(record.Units, func(unit batch.Unit) bool { return unit.State == batch.UnitJoined }) {
		return nil
	}
	// A series boundary fetches one fresh ledger snapshot and one clock. Reuse
	// is limited to this check; proof, rebase and push each call again.
	controlRoot, now, projection, err := batchAuthorityProjection(root)
	if err != nil {
		return err
	}
	for _, unit := range record.Units {
		if unit.State != batch.UnitJoined {
			continue
		}
		if err := authorizeBatchMemberInProjection(controlRoot, now, record, unit, projection); err != nil {
			if returnErr := batch.HandlePrefixMemberRefusal(store, record.BatchID, unit.GoalID, actor, at, err); returnErr != nil {
				return returnErr
			}
			return err
		}
	}
	return nil
}

// productionPrefixDecision selects the protected policy on the cumulative
// tree for every member present at this boundary. Earlier prefixes never
// inherit requirements introduced by a later member.
func productionPrefixDecision(root string, units []batch.Unit, tree string) (batch.PrefixDecision, error) {
	return planPrefixDecisionWith(root, units, tree, productionBatchTreePlanOutputWithGroups)
}

func planPrefixDecisionWith(root string, units []batch.Unit, tree string, plan func(string, string, string, testpolicy.Mode, []string) (testingPlanOutput, error)) (batch.PrefixDecision, error) {
	if len(units) == 0 {
		return batch.PrefixDecision{}, fmt.Errorf("BATCH_PREFIX_PLAN_REFUSED: no joined members")
	}
	outputs := make([]testingPlanOutput, 0, len(units))
	deep := false
	for _, unit := range units {
		output, err := plan(root, unit.GoalID, tree, testpolicy.ModeAuto, unit.SelectedGroups)
		if err != nil {
			return batch.PrefixDecision{}, err
		}
		if output.Plan.RequiredMode == testpolicy.ModeDeep {
			deep = true
		}
		outputs = append(outputs, output)
	}
	if deep {
		for index, unit := range units {
			output, err := plan(root, unit.GoalID, tree, testpolicy.ModeDeep, unit.SelectedGroups)
			if err != nil {
				return batch.PrefixDecision{}, err
			}
			outputs[index] = output
		}
	}
	groups := []string{}
	freshRequired, freshMaxAgeMS := false, int64(0)
	for index, unit := range units {
		output := outputs[index]
		if output.CandidateTree != tree || output.ContractDigest == "" || output.PolicyBaseCommit == "" {
			return batch.PrefixDecision{}, fmt.Errorf("BATCH_PREFIX_PLAN_REFUSED: incomplete policy decision for %s on %s", unit.GoalID, tree)
		}
		groups = append(groups, output.Plan.SelectedGroups...)
		for _, admitted := range unit.SelectedGroups {
			if !slices.Contains(output.Plan.RequiredGroups, admitted) {
				return batch.PrefixDecision{}, fmt.Errorf("BATCH_PREFIX_PLAN_REFUSED: admitted group %s is absent from %s's required delivery plan", admitted, unit.GoalID)
			}
		}
		for _, group := range output.Groups {
			if group.Freshness != "episode" {
				continue
			}
			freshRequired = true
			if group.FreshnessMaxAgeMS != nil && (freshMaxAgeMS == 0 || *group.FreshnessMaxAgeMS < freshMaxAgeMS) {
				freshMaxAgeMS = *group.FreshnessMaxAgeMS
			}
		}
	}
	slices.Sort(groups)
	groups = slices.Compact(groups)
	if freshRequired && freshMaxAgeMS <= 0 {
		return batch.PrefixDecision{}, fmt.Errorf("BATCH_PREFIX_PLAN_REFUSED: fresh group has no positive freshness max age")
	}
	identityTree := tree
	if root != "" {
		var err error
		identityTree, err = landing.ProjectWorkspaceTree(batch.ModuleRoot(root), tree)
		if err != nil {
			return batch.PrefixDecision{}, err
		}
	}
	contexts := make([]struct {
		PolicyBaseCommit, CandidateTree, ContractDigest, BaseContractDigest string
		Plan                                                                testpolicy.Plan
		Groups                                                              []testpolicy.Group
	}, 0, len(outputs))
	for _, output := range outputs {
		contexts = append(contexts, struct {
			PolicyBaseCommit, CandidateTree, ContractDigest, BaseContractDigest string
			Plan                                                                testpolicy.Plan
			Groups                                                              []testpolicy.Group
		}{output.PolicyBaseCommit, identityTree, output.ContractDigest, output.BaseContractDigest, output.Plan, output.Groups})
	}
	context, err := json.Marshal(contexts)
	if err != nil {
		return batch.PrefixDecision{}, err
	}
	return batch.PrefixDecision{Groups: groups, PolicyContext: string(context), IdentityTree: identityTree, FreshRequired: freshRequired, FreshMaxAgeMS: freshMaxAgeMS}, nil
}

// verifyBatchPrefix asks the retained-result consumer to recompute the current
// protected floor and validate every named observation on the exact index.
func verifyBatchPrefix(root string, unit batch.Unit, tree string, decision batch.PrefixDecision) error {
	detached, err := (gittree.Workspace{Dir: root}).NewDetachedWorktree(tree)
	if err != nil {
		return err
	}
	defer detached.Close()
	request := testingSelectionRequest{Root: batch.ModuleRoot(detached.Workspace().Dir), ControlRoot: batch.ModuleRoot(root), GoalID: unit.GoalID,
		Tree: tree, Mode: testpolicy.ModeAuto, Purpose: testpolicy.PurposeDelivery, BatchRequirements: slices.Clone(decision.Groups), BatchPrefixReceipt: true,
		FreshEpisode: decision.FreshEpisode, FreshExpiresAt: decision.FreshExpiresAt}
	result, err := verifyRetainedTesting(request)
	if err != nil {
		return err
	}
	if !result.Delivery.Sufficient {
		return fmt.Errorf("BATCH_PREFIX_PROOF_REFUSED: goal %s tree %s missing %s", unit.GoalID, tree, strings.Join(result.Delivery.MissingGroups, ","))
	}
	return nil
}

var batchVerifyPrefixEvidence = verifyBatchPrefix

// verifyBatchSeries re-plans every final prefix, including the tip, before
// publication. A retained receipt binds one decision but cannot certify a
// changed destination policy or a rebased series by itself.
func verifyBatchSeries(root string, record batch.Record, trees []string) error {
	units := []batch.Unit{}
	for _, unit := range record.Units {
		if unit.State == batch.UnitJoined {
			units = append(units, unit)
		}
	}
	if len(units) == 0 || len(trees) != len(units) {
		return fmt.Errorf("BATCH_PREFIX_PROOF_REFUSED: incomplete final prefix series")
	}
	for index, unit := range units {
		decision, err := productionPrefixDecision(root, units[:index+1], trees[index])
		if err != nil {
			return err
		}
		if index == len(units)-1 {
			for _, required := range record.SelectedGroups {
				if !slices.Contains(decision.Groups, required) {
					decision.Groups = append(decision.Groups, required)
				}
			}
			slices.Sort(decision.Groups)
			decision.Groups = slices.Compact(decision.Groups)
			if decision.FreshRequired {
				id, err := batch.PrefixDecisionID(record.BaseTree, trees[index], units[:index+1], record.Seal, decision)
				if err != nil {
					return err
				}
				episode, ok := record.PrefixEpisodes[unit.GoalID]
				if !ok || episode.DecisionID != id {
					return fmt.Errorf("BATCH_PREFIX_DECISION_MOVED: tip %s requires a new freshness episode", unit.GoalID)
				}
				decision.FreshEpisode, decision.FreshExpiresAt = episode.Token, episode.ExpiresAt
			}
		} else {
			receipt, ok := record.Receipts[unit.GoalID]
			if !ok {
				return fmt.Errorf("BATCH_PREFIX_PROOF_REFUSED: %s has no receipt", unit.GoalID)
			}
			id, err := batch.PrefixDecisionID(record.BaseTree, trees[index], units[:index+1], record.Seal, decision)
			if err != nil {
				return err
			}
			if receipt.Tree != trees[index] || receipt.DecisionID != id {
				return fmt.Errorf("BATCH_PREFIX_DECISION_MOVED: %s requires a new receipt", unit.GoalID)
			}
			if decision.FreshRequired {
				episode, ok := record.PrefixEpisodes[unit.GoalID]
				if !ok || episode.DecisionID != id || episode.Token != receipt.FreshEpisode || episode.ExpiresAt != receipt.FreshExpiresAt {
					return fmt.Errorf("BATCH_PREFIX_DECISION_MOVED: %s requires a new freshness episode", unit.GoalID)
				}
				decision.FreshEpisode, decision.FreshExpiresAt = episode.Token, episode.ExpiresAt
			}
		}
		if err := batchVerifyPrefixEvidence(root, unit, trees[index], decision); err != nil {
			return err
		}
	}
	return nil
}

func verifyBatchCommittedSeries(root string, record batch.Record, units []batch.Unit, commits map[string]string) error {
	for index, unit := range units {
		commit := commits[unit.GoalID]
		if commit == "" {
			return fmt.Errorf("BATCH_PREFIX_PROOF_REFUSED: %s has no final commit", unit.GoalID)
		}
		tree, err := gitOutput(root, "rev-parse", commit+"^{tree}")
		if err != nil {
			return err
		}
		decision, err := productionPrefixDecision(root, units[:index+1], tree)
		if err != nil {
			return err
		}
		if index == len(units)-1 {
			decision.Groups = append(decision.Groups, record.SelectedGroups...)
			slices.Sort(decision.Groups)
			decision.Groups = slices.Compact(decision.Groups)
			if decision.FreshRequired {
				id, err := batch.PrefixDecisionID(record.BaseTree, tree, units[:index+1], record.Seal, decision)
				if err != nil {
					return err
				}
				episode, ok := record.PrefixEpisodes[unit.GoalID]
				if !ok || episode.DecisionID != id {
					return fmt.Errorf("BATCH_PREFIX_DECISION_MOVED: final tip %s requires a new freshness episode", unit.GoalID)
				}
				decision.FreshEpisode, decision.FreshExpiresAt = episode.Token, episode.ExpiresAt
			}
		}
		if index < len(units)-1 && decision.FreshRequired {
			id, err := batch.PrefixDecisionID(record.BaseTree, tree, units[:index+1], record.Seal, decision)
			if err != nil {
				return err
			}
			episode, ok := record.PrefixEpisodes[unit.GoalID]
			if !ok || episode.DecisionID != id {
				return fmt.Errorf("BATCH_PREFIX_DECISION_MOVED: %s has no final freshness episode", unit.GoalID)
			}
			decision.FreshEpisode, decision.FreshExpiresAt = episode.Token, episode.ExpiresAt
		}
		if err := batchVerifyPrefixEvidence(root, unit, tree, decision); err != nil {
			return err
		}
	}
	return nil
}
