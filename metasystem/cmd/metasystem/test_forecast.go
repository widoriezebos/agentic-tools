package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"time"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

var errRetainedCandidateEngineAbsent = errors.New("candidate engine digest is absent from retained evidence")

type costSelection struct {
	ID, Kind, Tree, GoalID string
	Requirements           []string
	Admission              bool
	FreshEpisode           string
	FreshExpiresAt         string
}

type costSelectionEvidence struct {
	Request batch.CostForecastRequest
	Groups  []batch.CostForecastGroup
}

type forecastTestingDependencies struct {
	workspace     gittree.Workspace
	candidateIO   candidateEngineIO
	openCandidate func(string, string) (proofrun.CandidateWorkspace, error)
	now           func() time.Time
}

// forecastTestingSelection uses the same protected selection and retained
// evaluator as test verify. It creates no proof attempt or native build/test.
// Existing toolchain metadata reads may run bounded version/environment tools.
func forecastTestingSelection(root string, selection costSelection, proofCapMinutes uint64) (_ costSelectionEvidence, err error) {
	detached, err := (gittree.Workspace{Dir: root}).NewDetachedWorktree(selection.Tree)
	if err != nil {
		return costSelectionEvidence{}, err
	}
	defer func() { err = errors.Join(err, detached.Close()) }()
	executionRoot := batch.ModuleRoot(detached.Workspace().Dir)
	request := testingSelectionRequest{Root: executionRoot, ControlRoot: batch.ModuleRoot(root), GoalID: selection.GoalID,
		Tree: selection.Tree, Mode: testpolicy.ModeAuto, Purpose: testpolicy.PurposeDelivery,
		BatchPrefixReceipt: !selection.Admission, BatchAdmission: selection.Admission,
		BatchRequirements: slices.Clone(selection.Requirements), FreshEpisode: selection.FreshEpisode,
		FreshExpiresAt: selection.FreshExpiresAt}
	prepared, err := prepareTestingForCommand(request)
	if err != nil {
		return costSelectionEvidence{}, err
	}
	commandClock, _, err := goalCommandClock(prepared.proofControlRoot())
	if err != nil {
		return costSelectionEvidence{}, err
	}
	return forecastTestingSelectionPrepared(selection, proofCapMinutes, request, prepared, forecastTestingDependencies{
		workspace: gittree.Workspace{Dir: prepared.ProjectRoot}, candidateIO: nativeCandidateEngineIO(), now: commandClock,
	})
}

func forecastTestingSelectionPrepared(selection costSelection, proofCapMinutes uint64, request testingSelectionRequest,
	prepared testingPreparation, dependencies forecastTestingDependencies) (costSelectionEvidence, error) {
	semanticNow := dependencies.now()
	if _, err := resolveTestingPreparationWorkerPolicy(&prepared); err != nil {
		return costSelectionEvidence{}, err
	}
	attempts, err := proofrun.ReadAttempts(prepared.proofControlRoot())
	if err != nil {
		return costSelectionEvidence{}, err
	}
	ctx := context.Background()
	buildIdentity, err := candidateEngineBuildIdentityUsing(ctx, dependencies.workspace, prepared.Prefix, prepared.CandidateTree, prepared.Environment, dependencies.candidateIO)
	if err != nil {
		return costSelectionEvidence{}, err
	}
	engineDigest, err := retainedCandidateEngineDigest(prepared, attempts, buildIdentity, false)
	engineKnown := err == nil
	if err != nil && !errors.Is(err, errRetainedCandidateEngineAbsent) {
		return costSelectionEvidence{}, err
	}
	run := testingRunRequest(prepared, "", "", "", engineDigest, buildIdentity)
	run.WithCandidateOpener(dependencies.openCandidate)
	run.FreshnessEpisode, run.FreshnessExpiresAt = selection.FreshEpisode, selection.FreshExpiresAt
	freshGroups, _ := testingFreshGroups(prepared, request)
	run.FreshGroups = freshGroups
	if selection.FreshEpisode != "" {
		if err := bindTestingFreshnessProjectionWithWorkspace(&run, prepared.Installation, dependencies.workspace); err != nil {
			return costSelectionEvidence{}, err
		}
	}
	facts, err := proofrun.RevalidateRetainedGroupIdentityFacts(ctx, run, attempts)
	if err != nil {
		return costSelectionEvidence{}, err
	}
	identities := make(map[string]string, len(facts))
	byID := make(map[string]testpolicy.Group, len(prepared.EffectiveContract.Groups))
	for _, group := range prepared.EffectiveContract.Groups {
		byID[group.ID] = group
	}
	for id, fact := range facts {
		if fact.Known && (engineKnown || !proofrun.GroupConsumesCandidateEngine(run, byID[id], prepared.ProjectRoot)) {
			identities[id] = fact.Identity
		}
	}
	run.FreshnessBinding = testingFreshnessBinding(run, identities, selection.FreshEpisode)
	template := proofrun.NewTestResultAt(run, semanticNow)
	reused := proofrun.ReusedTestResult(template, attempts, identities, prepared.EffectiveContract)
	evidence := costSelectionEvidence{Request: batch.CostForecastRequest{ID: selection.ID, Kind: selection.Kind,
		Tree: selection.Tree, ChargeGoal: selection.GoalID, SelectedGroups: slices.Clone(prepared.Plan.SelectedGroups)}}
	for _, judged := range reused.Groups {
		definition := byID[judged.ID]
		identity := identities[judged.ID]
		resourceClass := definition.Resources.Class
		if resourceClass == "" {
			resourceClass = "cheap"
		}
		if definition.Adapter == "go" {
			resourceClass = "heavy"
		}
		row := batch.CostForecastGroup{RequestID: selection.ID, Tree: selection.Tree, ChargeGoal: selection.GoalID,
			GroupID: judged.ID, ExecutionIdentity: identity, IdentityKnown: identity != "", Status: judged.Status,
			Reason: judged.NotRunReason, TargetMS: definition.TargetMS, ResourceClass: resourceClass,
			ExclusiveResources: slices.Clone(definition.Resources.Exclusive), FreshEpisode: selection.FreshEpisode}
		if definition.Freshness == "episode" && selection.FreshEpisode == "" {
			row.Status, row.Reason = "missing", "fresh-episode-not-created"
		} else if identity == "" {
			row.Status, row.Reason = "unknown", "retained-execution-metadata-absent"
			if !engineKnown && proofrun.GroupConsumesCandidateEngine(run, definition, prepared.ProjectRoot) {
				row.Reason = "retained-candidate-engine-absent"
			}
		} else {
			observation := proofrun.ForecastRetainedGroupObservation(template, attempts, judged.ID, identity)
			row.ObservedDurationMS = observation.DurationMS
			if judged.Status == "reused" {
				row.Status, row.Reason = "reusable", ""
			} else if observation.Status == "live" {
				row.Status, row.Reason = "live-wait", "newer-live-producer"
			} else if observation.Status == "failed" {
				row.Status, row.Reason = "missing", "newer-red-observation"
			} else {
				row.Status = "missing"
			}
		}
		if row.Status != "reusable" {
			row.DeclaredAllowanceMS = int64(proofCapMinutes) * int64(time.Minute/time.Millisecond)
		}
		evidence.Groups = append(evidence.Groups, row)
	}
	return evidence, nil
}

func costSelectionForPrefix(id, kind, tree, goalID string, requirements []string) costSelection {
	return costSelection{ID: id, Kind: kind, Tree: tree, GoalID: goalID, Requirements: slices.Clone(requirements)}
}

func proofCostCap(root string) (uint64, error) {
	capMinutes, _, _, err := dispatchcore.ResolveCap(filepath.Join(batch.ModuleRoot(root), "metasystem.conf"), "proof", "main", "proof", "", "")
	if err != nil || capMinutes < 1 {
		return 0, fmt.Errorf("resolve declared proof cap: %w", err)
	}
	return uint64(capMinutes), nil
}
