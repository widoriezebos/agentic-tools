package testrun

import (
	"context"
	"errors"
	"fmt"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"path/filepath"
	"slices"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/candidateengine"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// CostSelection is one tree whose testing cost a batch forecast asks for:
// a prefix's batch-prefix proof or a join's admission checks.
type CostSelection struct {
	ID, Kind, Tree, GoalID string
	Requirements           []string
	Admission              bool
	FreshEpisode           string
	FreshExpiresAt         string
}

// CostEvidence is a selection's forecast request and its per-group rows.
type CostEvidence struct {
	Request batch.CostForecastRequest
	Groups  []batch.CostForecastGroup
}

// Forecasting is what ForecastPrepared reads through: the project
// workspace, the candidate engine's seams, the candidate opener, the
// semantic clock, the forecast's own scratch run (nil keeps host temp) and
// the worker policy resolver.
type Forecasting struct {
	Workspace     gittree.Workspace
	CandidateIO   candidateengine.IO
	OpenCandidate func(string, string) (proofrun.CandidateWorkspace, error)
	Now           func() time.Time
	Scratch       *proofrun.ScratchRun
	WorkerPolicy  func(confPath string) (WorkerPolicy, error)
}

// ForecastPrepared forecasts selection's cost on prepared with the same
// protected selection and retained evaluator as verification: each selected
// group is reusable, live, missing or unknown, and a group that must run is
// charged proofCapMinutes. It creates no attempt and builds or runs nothing.
func ForecastPrepared(selection CostSelection, proofCapMinutes uint64, request SelectionRequest,
	prepared Preparation, dependencies Forecasting) (CostEvidence, error) {
	semanticNow := dependencies.Now()
	if _, err := ApplyWorkerPolicy(&prepared, dependencies.WorkerPolicy); err != nil {
		return CostEvidence{}, err
	}
	attempts, err := proofrun.ReadAttempts(prepared.ProofControlRoot())
	if err != nil {
		return CostEvidence{}, err
	}
	ctx := context.Background()
	if dependencies.Scratch != nil {
		ctx = proofrun.WithScratchRun(ctx, dependencies.Scratch)
	}
	buildIdentity, err := candidateengine.BuildIdentityUsing(ctx, dependencies.Workspace, prepared.Prefix, prepared.CandidateTree, prepared.Environment, dependencies.CandidateIO)
	if err != nil {
		return CostEvidence{}, err
	}
	engineDigest, err := RetainedCandidateEngineDigest(prepared, attempts, buildIdentity, false)
	engineKnown := err == nil
	if err != nil && !errors.Is(err, ErrRetainedCandidateEngineAbsent) {
		return CostEvidence{}, err
	}
	run := RunRequest(prepared, "", "", "", engineDigest, buildIdentity)
	run.WithCandidateOpener(dependencies.OpenCandidate)
	if dependencies.Scratch != nil {
		// A managed run's retained environment digest names its scratch
		// paths as stable tokens; only a prepared descriptor reproduces it.
		if err := PrepareScratch(ctx, &run, dependencies.Scratch, prepared); err != nil {
			return CostEvidence{}, err
		}
	}
	run.FreshnessEpisode, run.FreshnessExpiresAt = selection.FreshEpisode, selection.FreshExpiresAt
	freshGroups, _ := FreshGroups(prepared, request)
	run.FreshGroups = freshGroups
	if selection.FreshEpisode != "" {
		if err := BindFreshnessProjectionWithWorkspace(&run, prepared.Installation, dependencies.Workspace); err != nil {
			return CostEvidence{}, err
		}
	}
	facts, err := proofrun.RevalidateRetainedGroupIdentityFacts(ctx, run, attempts)
	if err != nil {
		return CostEvidence{}, err
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
	run.FreshnessBinding = FreshnessBinding(run, identities, selection.FreshEpisode)
	template := proofrun.NewTestResultAt(run, semanticNow)
	reused := proofrun.ReusedTestResult(template, attempts, identities, prepared.EffectiveContract)
	evidence := CostEvidence{Request: batch.CostForecastRequest{ID: selection.ID, Kind: selection.Kind,
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

// PrefixCostSelection is the cost selection of a batch prefix tree.
func PrefixCostSelection(id, kind, tree, goalID string, requirements []string) CostSelection {
	return CostSelection{ID: id, Kind: kind, Tree: tree, GoalID: goalID, Requirements: slices.Clone(requirements)}
}

// ProofCostCap is the declared proof cap in minutes of the installation at
// or below root, the charge of a group that must run.
func ProofCostCap(root string) (uint64, error) {
	capMinutes, _, _, err := dispatchcore.ResolveCap(filepath.Join(batch.ModuleRoot(root), "metasystem.conf"), "proof", "main", "proof", "", "")
	if err != nil || capMinutes < 1 {
		return 0, fmt.Errorf("the test-run time cap in metasystem.conf cannot be read: %w", err)
	}
	return uint64(capMinutes), nil
}
