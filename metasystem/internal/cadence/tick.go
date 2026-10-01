// Package cadence is the landing lane's deep validation cadence in
// production (landing validate): it revalidates the trunk's deep-only
// groups from retained evidence without building, and when due claims the
// standing authority and runs the deep validation as a governed run of this
// engine's own test run. gaterun decides; this package wires the decision to
// the ledger, the run store and the testing selection.
package cadence

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/digest"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gaterun"
	runpkg "github.com/widoriezebos/agentic-tools/metasystem/internal/run"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/candidateengine"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testrun"
)

const (
	// AuthorityGoal is the standing goal a cadence run is claimed under.
	AuthorityGoal = "standing-validation"
)

// Owner is the authority a cadence run is claimed under (the lane's claim
// identity), and the command-side reads it needs: its lineage and epoch,
// whether it still holds its authority, the trunk fetch, the weight
// threshold, the testing preparation and the worker policy.
type Owner struct {
	Epoch           int64
	Lineage         string
	Require         func() error
	FetchOrigin     func(root string) (commit, tree string, err error)
	WeightThreshold func(root string) int64
	Prepare         func(testrun.SelectionRequest) (testrun.Preparation, error)
	WorkerPolicy    func(confPath string) (testrun.WorkerPolicy, error)
	// Ledger is the publication boundary the cadence's goal writes go
	// through: the landing lane's (lane runtime design r10, K3); nil
	// publishes as the checkout's own.
	Ledger func(goal.Endpoint) goal.Endpoint
}

var cadenceBuildIdentity = candidateengine.BuildIdentity
var cadenceRetainedEngineDigest = testrun.RetainedCandidateEngineDigest

type cadenceRevalidationDependencies struct {
	prepare        func(testrun.SelectionRequest) (testrun.Preparation, error)
	readAttempts   func(string) ([]proofrun.Attempt, error)
	buildIdentity  func(context.Context, gittree.Workspace, string, string, []string) (string, error)
	retainedDigest func(testrun.Preparation, []proofrun.Attempt, string, bool) (string, error)
	openCandidate  func(projectRoot, candidateTree string) (proofrun.CandidateWorkspace, error)
	workerPolicy   func(confPath string) (testrun.WorkerPolicy, error)
}

func productionCadenceRevalidationDependencies(owner Owner) cadenceRevalidationDependencies {
	return cadenceRevalidationDependencies{prepare: owner.Prepare, readAttempts: proofrun.ReadAttempts,
		buildIdentity: cadenceBuildIdentity, retainedDigest: cadenceRetainedEngineDigest, workerPolicy: owner.WorkerPolicy}
}

func revalidateCadenceWith(root string, prepared testrun.Preparation, trunk gaterun.CadenceTrunk, deepOnly []string, dependencies cadenceRevalidationDependencies) (gaterun.CadenceRevalidation, error) {
	if prepared.CandidateTree != trunk.Tree {
		var err error
		prepared, err = dependencies.prepare(cadencePreparationRequest(root, trunk.Tree))
		if err != nil {
			return gaterun.CadenceRevalidation{}, err
		}
	}
	if _, err := testrun.ApplyWorkerPolicy(&prepared, dependencies.workerPolicy); err != nil {
		return gaterun.CadenceRevalidation{}, err
	}
	attempts, err := dependencies.readAttempts(prepared.Installation)
	if err != nil {
		return gaterun.CadenceRevalidation{}, err
	}
	buildIdentity, digest, exactEngine, err := cadenceCandidateEngineIdentityWith(prepared, trunk, attempts, dependencies)
	if err != nil {
		return gaterun.CadenceRevalidation{}, err
	}
	request := testrun.RunRequest(prepared, "", "", "", digest, buildIdentity)
	request.WithCandidateOpener(dependencies.openCandidate)
	identities, err := proofrun.RevalidateRetainedGroupExecutionIdentities(context.Background(), request, attempts)
	if err != nil {
		return gaterun.CadenceRevalidation{}, err
	}
	composed := proofrun.ReusedTestResult(proofrun.NewTestResult(request), attempts, identities, prepared.EffectiveContract)
	wanted := map[string]bool{}
	for _, id := range deepOnly {
		wanted[id] = true
	}
	groups := make([]proofrun.GroupResult, 0, len(deepOnly))
	for _, group := range composed.Groups {
		if wanted[group.ID] {
			if !exactEngine {
				group.Status, group.CollectionComplete, group.ReuseAttempt = "not-run", false, ""
				group.NotRunReason = "candidate engine digest has no retained evidence"
			}
			groups = append(groups, group)
		}
	}
	return gaterun.CadenceRevalidation{Groups: groups}, nil
}

func cadenceCandidateEngineIdentityWith(prepared testrun.Preparation, trunk gaterun.CadenceTrunk, attempts []proofrun.Attempt, dependencies cadenceRevalidationDependencies) (string, string, bool, error) {
	environment := testrun.InheritedEnvironment(prepared.Environment, os.Environ())
	buildIdentity, err := dependencies.buildIdentity(context.Background(), gittree.Workspace{Dir: prepared.ProjectRoot}, prepared.Prefix, trunk.Tree, environment)
	if err != nil {
		return "", "", false, err
	}
	retained, retainedErr := dependencies.retainedDigest(prepared, attempts, buildIdentity, false)
	if retainedErr == nil {
		return buildIdentity, retained, true, nil
	}
	// Revalidation runs before the shared claim and must remain a no-build
	// probe. The marker forces a claimed native run; that run's exact group
	// identities replace these deliberately non-green probe identities.
	return buildIdentity, digest.SHA256([]byte("cadence-missing-engine-evidence\x00" + buildIdentity)), false, nil
}

func cadencePreparationRequest(root, tree string) testrun.SelectionRequest {
	return testrun.SelectionRequest{Root: root, Tree: tree, Mode: testpolicy.ModeDeep, Purpose: testpolicy.PurposeCadence, CadencePreflight: true}
}

func cadenceRequest(endpoint goal.Endpoint, actor goal.Actor, epoch int64, at time.Time) (goal.VerbRequest, error) {
	ulid, err := goal.NewOperationULID()
	return goal.VerbRequest{Endpoint: endpoint, Actor: actor, Ulid: ulid, Now: at.UTC(), ClaimEpoch: epoch}, err
}

func claimCadenceAuthority(endpoint goal.Endpoint, actor goal.Actor, epoch int64, at time.Time) (gaterun.CadenceAuthority, error) {
	if binding, err := dispatchcore.ResolveGoalBinding(endpoint.Root, AuthorityGoal, at); err == nil {
		if binding.Machine != actor.Machine || binding.Lineage != actor.Lineage || binding.File.Obligation == nil {
			return gaterun.CadenceAuthority{}, fmt.Errorf("standing cadence authority is held elsewhere")
		}
		return gaterun.CadenceAuthority{GoalID: AuthorityGoal, ObligationRevision: binding.File.Obligation.Revision}, nil
	}
	request, err := cadenceRequest(endpoint, actor, epoch, at)
	if err != nil {
		return gaterun.CadenceAuthority{}, err
	}
	if _, err = goal.Claim(request, AuthorityGoal); err != nil {
		var noChange goal.NothingToDo
		if !errors.As(err, &noChange) {
			return gaterun.CadenceAuthority{}, err
		}
	}
	binding, err := dispatchcore.ResolveGoalBinding(endpoint.Root, AuthorityGoal, at)
	if err != nil || binding.Machine != actor.Machine || binding.Lineage != actor.Lineage || binding.File.Obligation == nil {
		return gaterun.CadenceAuthority{}, fmt.Errorf("standing cadence authority is unavailable: %v", err)
	}
	return gaterun.CadenceAuthority{GoalID: AuthorityGoal, ObligationRevision: binding.File.Obligation.Revision}, nil
}

func cadenceRunStore(root string, owner Owner, clock func() time.Time) *runpkg.Store {
	return &runpkg.Store{Root: root, Now: clock, CurrentEpoch: func() (*int64, bool) {
		if owner.Require() != nil {
			return nil, false
		}
		epoch := owner.Epoch
		return &epoch, true
	},
		AdmitGoverned: func(request runpkg.GovernedAdmissionRequest) (runpkg.GovernedAdmissionResult, error) {
			return dispatchcore.EvaluateGovernedRunAdmission(root, request, clock().UTC())
		}, ObserveGoverned: func(record *runpkg.Record, now time.Time) runpkg.AssumptionObservation {
			return dispatchcore.ObserveGovernedRun(root, record, now)
		}, ProjectSpend: func(record *runpkg.Record, now time.Time) (runpkg.SpendSnapshot, string) {
			return dispatchcore.SettledSpendAtConclusion(root, record, now)
		}}
}

// cadenceRunCommand is a deep validation run of this engine's own test run,
// wrapped as the governed run runID, leading its own session.
func cadenceRunCommand(root, runID, nonce, log, goalID, tree, resultPath string, forceGroups bool) (*exec.Cmd, error) {
	self, err := os.Executable()
	if err != nil {
		return nil, err
	}
	testArgs := []string{"internal", "test", "run", "--root", root, "--goal", goalID, "--tree", tree,
		"--mode", "deep", "--purpose", "cadence", "--result", resultPath}
	if forceGroups {
		testArgs = append(testArgs, "--force-groups")
	}
	wrapArgs := append([]string{"run", "wrap", "--root", root, "--id", runID, "--nonce", nonce, "--log", log, "--", self}, testArgs...)
	command := exec.Command(self, wrapArgs...)
	command.Dir, command.Env = root, os.Environ()
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return command, nil
}
