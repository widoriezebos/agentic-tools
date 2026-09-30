package batchowner

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testrun"
)

// A retained proof does not retain the right to land. Recheck the live
// handed-over claim and elapsed admission fence before consuming it.
func AuthorizeBatchMember(root string, record batch.Record, unit batch.Unit) error {
	controlRoot, now, projection, err := batchAuthorityProjection(root)
	if err != nil {
		return err
	}
	return AuthorizeBatchMemberInProjection(controlRoot, now, record, unit, projection)
}

func batchAuthorityProjection(root string) (string, time.Time, goal.Projection, error) {
	controlRoot := batch.ModuleRoot(root)
	now, err := fixtureauth.GoalNow(controlRoot)
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

func AuthorizeBatchMemberInProjection(controlRoot string, now time.Time, record batch.Record, unit batch.Unit, projection goal.Projection) error {
	if unit.IsChange() {
		return authorizeChangeInProjection(controlRoot, unit, projection)
	}
	if projection.Tree == nil {
		return fmt.Errorf("%s: live goal projection is empty", codePrefixAuthorityRefused)
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
		return &batch.PrefixBudgetRefusal{Reason: "BATCH_MEMBER_BUDGET_REFUSED: member " + unit.GoalID + " elapsed admission budget closed; a person raises it with metasystem goal budget " + unit.GoalID + " BOX"}
	}
	// The landing gate is read again at every publication and retry against
	// this fresh ledger (g1-s70 D2): a hold published while the batch proved,
	// or a word given at another tip, takes the member out of the batch.
	settings, err := goal.ResolveGateSettings(filepath.Join(controlRoot, "metasystem.conf"))
	if err != nil {
		return err
	}
	if _, err := goal.Gate(file, unit.BranchTip, settings); err != nil {
		return &batch.PrefixRevisionRefusal{Reason: "BATCH_PREFIX_AUTHORITY_REFUSED: member " + unit.GoalID + " " + err.Error()}
	}
	return nil
}

// authorizeChangeInProjection is a change's publication authority: a change
// holds no claim, so a goal-less change needs none; one made in G's name
// leaves the batch when G's claim is no longer the asker's at the revision it
// committed under, or G's landing gate now refuses (U11b).
func authorizeChangeInProjection(controlRoot string, unit batch.Unit, projection goal.Projection) error {
	return AuthorizeChangeWith(controlRoot, unit, projection, changeGoalBranchTip)
}

// changeGoalBranchTip reads a goal's branch tip at origin as it is now: the
// tip a goal-bound change's landing gate is read at before each push (N-6).
func changeGoalBranchTip(controlRoot, goalID string) (string, error) {
	endpoint, err := branch.MainEndpoint(controlRoot)
	if err != nil {
		return "", err
	}
	tip, _, err := branch.OriginTip(controlRoot, endpoint, goalID)
	return tip, err
}

func AuthorizeChangeWith(controlRoot string, unit batch.Unit, projection goal.Projection, branchTip func(string, string) (string, error)) error {
	change := unit.Change
	if change.Goal == "" {
		return nil
	}
	if projection.Tree == nil {
		return fmt.Errorf("%s: live goal projection is empty", codePrefixAuthorityRefused)
	}
	file := projection.Tree.Live[change.Goal]
	// A person's commit holds no claim of the goal (held only warns on it),
	// so a person's change answers to the goal's landing gate alone.
	person := strings.HasSuffix(change.AskedBy, "+human")
	if !person && (file == nil || file.State != goal.StateClaimed || file.Claimed == nil || file.Claimed.Machine+"+"+file.Claimed.Lineage != change.AskedBy ||
		file.Claimed.Revision != change.GoalRevision) {
		return &batch.PrefixRevisionRefusal{Reason: "BATCH_PREFIX_AUTHORITY_REFUSED: change " + unit.GoalID + " was committed in goal " + change.Goal +
			"'s name by " + change.AskedBy + " at revision " + fmt.Sprint(change.GoalRevision) + ", and that claim moved; commit it again and land it"}
	}
	settings, err := goal.ResolveGateSettings(filepath.Join(controlRoot, "metasystem.conf"))
	if err != nil {
		return err
	}
	tip, err := branchTip(controlRoot, change.Goal)
	if err != nil {
		return fmt.Errorf("%s: goal %s's branch tip cannot be read for change %s's landing gate: %w", codePrefixAuthorityRefused, change.Goal, unit.GoalID, err)
	}
	if _, err := goal.Gate(file, tip, settings); err != nil {
		return &batch.PrefixRevisionRefusal{Reason: "BATCH_PREFIX_AUTHORITY_REFUSED: change " + unit.GoalID + " " + err.Error()}
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
		if err := AuthorizeBatchMemberInProjection(controlRoot, now, record, unit, projection); err != nil {
			if returnErr := batch.HandlePrefixMemberRefusal(store, record.BatchID, unit.GoalID, actor, at, err); returnErr != nil {
				return returnErr
			}
			return err
		}
	}
	return nil
}

// ProductionPrefixDecision selects the protected policy on the cumulative
// tree for every member present at this boundary. Earlier prefixes never
// inherit requirements introduced by a later member.
func ProductionPrefixDecision(root string, units []batch.Unit, tree string) (batch.PrefixDecision, error) {
	return PlanPrefixDecisionWith(root, units, tree, productionBatchTreePlanOutputWithGroups)
}

func PlanPrefixDecisionWith(root string, units []batch.Unit, tree string, plan func(string, string, string, testpolicy.Mode, []string) (testrun.PlanOutput, error)) (batch.PrefixDecision, error) {
	if len(units) == 0 {
		return batch.PrefixDecision{}, fmt.Errorf("%s: no joined members", codePrefixPlanRefused)
	}
	// A change is planned by no goal of its own: the goal members' plans on
	// the prefix tree hold every change before them (U11b).
	planned := slices.DeleteFunc(slices.Clone(units), func(unit batch.Unit) bool { return unit.IsChange() })
	if len(planned) == 0 && len(units) != 0 {
		// A prefix of changes alone is planned on the lane's account.
		charge, err := batchChargeID(root, units[len(units)-1], nil)
		if err != nil {
			return batch.PrefixDecision{}, err
		}
		planned = []batch.Unit{{GoalID: charge}}
	}
	units = planned
	if len(units) == 0 {
		return batch.PrefixDecision{}, fmt.Errorf("%s: no joined members", codePrefixPlanRefused)
	}
	outputs := make([]testrun.PlanOutput, 0, len(units))
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
			// A member whose auto plan already selected deep keeps that plan
			// and its honest requested mode; replanning would repeat the same
			// selection on the same tree.
			if testpolicy.AutoPlanSelectsDeep(outputs[index].Plan) {
				continue
			}
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
			return batch.PrefixDecision{}, fmt.Errorf("%s: incomplete policy decision for %s on %s", codePrefixPlanRefused, unit.GoalID, tree)
		}
		groups = append(groups, output.Plan.SelectedGroups...)
		for _, admitted := range unit.SelectedGroups {
			if !slices.Contains(output.Plan.RequiredGroups, admitted) {
				return batch.PrefixDecision{}, fmt.Errorf("%s: admitted group %s is absent from %s's required delivery plan", codePrefixPlanRefused, admitted, unit.GoalID)
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
		return batch.PrefixDecision{}, fmt.Errorf("%s: fresh group has no positive freshness max age", codePrefixPlanRefused)
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
	charge, err := batchChargeID(root, unit, nil)
	if err != nil {
		return err
	}
	request := testrun.SelectionRequest{Root: batch.ModuleRoot(detached.Workspace().Dir), ControlRoot: batch.ModuleRoot(root), GoalID: charge,
		Tree: tree, Mode: testpolicy.ModeAuto, Purpose: testpolicy.PurposeDelivery, BatchRequirements: slices.Clone(decision.Groups), BatchPrefixReceipt: true,
		FreshEpisode: decision.FreshEpisode, FreshExpiresAt: decision.FreshExpiresAt}
	result, err := Engine.VerifyRetainedTesting(request)
	if err != nil {
		return err
	}
	if !result.Delivery.Sufficient {
		return fmt.Errorf("%s: goal %s tree %s missing %s", codePrefixProofRefused, unit.GoalID, tree, strings.Join(result.Delivery.MissingGroups, ","))
	}
	return nil
}

var BatchVerifyPrefixEvidence = verifyBatchPrefix

// VerifyBatchSeries re-plans every final prefix, including the tip, before
// publication. A retained receipt binds one decision but cannot certify a
// changed destination policy or a rebased series by itself.
func VerifyBatchSeries(root string, record batch.Record, trees []string) error {
	return VerifyBatchSeriesWith(root, record, trees, ProductionPrefixDecision, BatchVerifyPrefixEvidence)
}

// VerifyBatchSeriesWith is verifyBatchSeries with its planner and verifier.
func VerifyBatchSeriesWith(root string, record batch.Record, trees []string,
	decide func(string, []batch.Unit, string) (batch.PrefixDecision, error), verify func(string, batch.Unit, string, batch.PrefixDecision) error) error {
	units := []batch.Unit{}
	for _, unit := range record.Units {
		if unit.State == batch.UnitJoined {
			units = append(units, unit)
		}
	}
	if len(units) == 0 || len(trees) != len(units) {
		return fmt.Errorf("%s: incomplete final prefix series", codePrefixProofRefused)
	}
	for index, unit := range units {
		last := index == len(units)-1
		if unit.IsChange() && !last {
			// A replayed change carries no receipt of its own.
			continue
		}
		charge := batch.ChargeUnit(units[:index+1])
		decision, err := decide(root, units[:index+1], trees[index])
		if err != nil {
			return err
		}
		if last {
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
					return fmt.Errorf("%s: tip %s requires a new freshness episode", codePrefixDecisionMoved, unit.GoalID)
				}
				decision.FreshEpisode, decision.FreshExpiresAt = episode.Token, episode.ExpiresAt
			}
		} else {
			receipt, ok := record.Receipts[unit.GoalID]
			if !ok {
				return fmt.Errorf("%s: %s has no receipt", codePrefixProofRefused, unit.GoalID)
			}
			id, err := batch.PrefixDecisionID(record.BaseTree, trees[index], units[:index+1], record.Seal, decision)
			if err != nil {
				return err
			}
			if receipt.Tree != trees[index] || receipt.DecisionID != id {
				return fmt.Errorf("%s: %s requires a new receipt", codePrefixDecisionMoved, unit.GoalID)
			}
			if decision.FreshRequired {
				episode, ok := record.PrefixEpisodes[unit.GoalID]
				if !ok || episode.DecisionID != id || episode.Token != receipt.FreshEpisode || episode.ExpiresAt != receipt.FreshExpiresAt {
					return fmt.Errorf("%s: %s requires a new freshness episode", codePrefixDecisionMoved, unit.GoalID)
				}
				decision.FreshEpisode, decision.FreshExpiresAt = episode.Token, episode.ExpiresAt
			}
		}
		if err := verify(root, charge, trees[index], decision); err != nil {
			return err
		}
	}
	return nil
}

func verifyBatchCommittedSeries(root string, record batch.Record, units []batch.Unit, commits map[string]string) error {
	for index, unit := range units {
		commit := commits[unit.GoalID]
		if commit == "" {
			return fmt.Errorf("%s: %s has no final commit", codePrefixProofRefused, unit.GoalID)
		}
		if unit.IsChange() && index < len(units)-1 {
			// A replayed change carries no receipt of its own.
			continue
		}
		charge := batch.ChargeUnit(units[:index+1])
		tree, err := GitOutput(root, "rev-parse", commit+"^{tree}")
		if err != nil {
			return err
		}
		decision, err := ProductionPrefixDecision(root, units[:index+1], tree)
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
					return fmt.Errorf("%s: final tip %s requires a new freshness episode", codePrefixDecisionMoved, unit.GoalID)
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
				return fmt.Errorf("%s: %s has no final freshness episode", codePrefixDecisionMoved, unit.GoalID)
			}
			decision.FreshEpisode, decision.FreshExpiresAt = episode.Token, episode.ExpiresAt
		}
		if err := BatchVerifyPrefixEvidence(root, charge, tree, decision); err != nil {
			return err
		}
	}
	return nil
}
