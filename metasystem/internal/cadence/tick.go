// Package cadence is the landing owner's deep validation cadence in
// production: one tick fetches the trunk, revalidates its deep-only groups
// from retained evidence without building, and when due claims the standing
// authority and runs the deep validation as a governed run of this engine's
// own test run. gaterun decides; this package wires the decision to the
// ledger, the run store and the testing selection.
package cadence

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/digest"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gaterun"
	runpkg "github.com/widoriezebos/agentic-tools/metasystem/internal/run"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/candidateengine"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalrevision"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testrun"
)

const (
	// FetchRefused is a tick that could not fetch the trunk.
	FetchRefused            = "CADENCE_FETCH_REFUSED"
	cadenceLedgerUnreadable = "CADENCE_LEDGER_UNREADABLE"
	// AuthorityGoal is the standing goal a cadence run is claimed under.
	AuthorityGoal = "standing-validation"
)

// Refusal is a tick refused before it ran, by code.
type Refusal struct{ Code, Detail string }

// Error is the plain words; the code is RefusalCode ("Messages a Person
// Reads"), and RefusalDetail the code-first line records keep.
func (refusal Refusal) Error() string {
	if refusal.Code == FetchRefused {
		return "the cadence tick could not fetch the trunk: " + refusal.Detail + "\nnothing to do now; the next tick fetches again"
	}
	return "the cadence tick could not read its ledger: " + refusal.Detail + "\nnothing to do by command: repair the named ledger record; the next tick reads it again"
}
func (refusal Refusal) RefusalCode() string   { return refusal.Code }
func (refusal Refusal) RefusalDetail() string { return refusal.Code + ": " + refusal.Detail }

// TickOutput is the trunk a production tick judged and its result.
type TickOutput struct {
	Trunk gaterun.CadenceTrunk
	Tick  gaterun.CadenceTickResult
}

// Owner is the landing owner a production tick runs under, and the
// command-side reads it needs: its lineage and lease epoch, whether it still
// holds that lease, the trunk fetch, the weight threshold, the testing
// preparation and the worker policy.
type Owner struct {
	Epoch           int64
	Lineage         string
	Require         func() error
	FetchOrigin     func(root string) (commit, tree string, err error)
	WeightThreshold func(root string) int64
	Prepare         func(testrun.SelectionRequest) (testrun.Preparation, error)
	WorkerPolicy    func(confPath string) (testrun.WorkerPolicy, error)
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

// RunTick is one deep cadence tick of the landing owner:
// fetch the trunk, revalidate its deep-only groups from retained evidence,
// and, when due, claim the standing authority and run the deep validation
// as a governed run of this engine's own test run.
func RunTick(root string, owner Owner, clock func() time.Time) (TickOutput, error) {
	commit, tree, err := owner.FetchOrigin(root)
	if err != nil {
		return TickOutput{}, Refusal{FetchRefused, err.Error()}
	}
	trunk := gaterun.CadenceTrunk{Commit: commit, Tree: tree}
	now := clock().UTC()
	endpoint, err := goal.ResolveEndpoint(root)
	if err != nil {
		return TickOutput{}, Refusal{cadenceLedgerUnreadable, err.Error()}
	}
	projection, err := goal.Project(endpoint, false, now)
	if err != nil {
		return TickOutput{}, Refusal{cadenceLedgerUnreadable, err.Error()}
	}
	prepared, err := owner.Prepare(cadencePreparationRequest(root, tree))
	if err != nil {
		return TickOutput{}, Refusal{cadenceLedgerUnreadable, err.Error()}
	}
	deepOnly, err := testpolicy.DeepOnlySectionGroupIDs(prepared.EffectiveContract)
	if err != nil {
		return TickOutput{}, Refusal{cadenceLedgerUnreadable, err.Error()}
	}
	weight, due, err := gaterun.WeightCheckAt(root, owner.WeightThreshold(root), now)
	if err != nil {
		return TickOutput{}, Refusal{cadenceLedgerUnreadable, err.Error()}
	}
	machine, err := goal.ResolveMachine(root)
	if err != nil {
		return TickOutput{}, Refusal{cadenceLedgerUnreadable, err.Error()}
	}
	actor := goal.Actor{Machine: machine, Lineage: owner.Lineage}
	deps := gaterun.CadenceDependencies{
		Clock: clock, Fetch: func() (gaterun.CadenceTrunk, error) { return trunk, nil },
		Revalidate: func(fetched gaterun.CadenceTrunk) (gaterun.CadenceRevalidation, error) {
			return revalidateCadenceWith(root, prepared, fetched, deepOnly, productionCadenceRevalidationDependencies(owner))
		},
		Ledger: gaterun.GoalCadenceLedger{Endpoint: endpoint, Actor: actor},
	}
	deps.ClaimAuthority = func(at time.Time) (gaterun.CadenceAuthority, error) {
		return claimCadenceAuthority(endpoint, actor, owner.Epoch, at)
	}
	deps.ReleaseAuthority = func(authority gaterun.CadenceAuthority, at time.Time) error {
		return releaseCadenceAuthority(endpoint, actor, authority, at)
	}
	deps.Run = func(request gaterun.CadenceRunRequest) (gaterun.CadenceRunResult, error) {
		return executeCadenceRun(root, owner, clock, request)
	}
	deps.DischargeWeight = func(authority gaterun.CadenceAuthority, runID string, _ uint64, at time.Time) error {
		_, dischargeErr := gaterun.WeightDischargeAt(root, authority.GoalID, authority.ObligationRevision, runID, at)
		return dischargeErr
	}
	result, err := gaterun.RunCadenceTick(gaterun.CadenceTickInput{Latest: projection.Tree.Cadence, Weight: weight, WeightDue: due,
		DeepOnlyGroups: deepOnly, Lease: gaterun.CadenceForcedInterval}, deps)
	return TickOutput{Trunk: trunk, Tick: result}, err
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

func releaseCadenceAuthority(endpoint goal.Endpoint, actor goal.Actor, authority gaterun.CadenceAuthority, at time.Time) error {
	request, err := cadenceRequest(endpoint, actor, 0, at)
	if err != nil {
		return err
	}
	_, err = goal.Release(request, authority.GoalID)
	return err
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

func executeCadenceRun(root string, owner Owner, clock func() time.Time, request gaterun.CadenceRunRequest) (gaterun.CadenceRunResult, error) {
	binding, err := dispatchcore.ResolveGoalBinding(root, request.Authority.GoalID, clock().UTC())
	if err != nil {
		return gaterun.CadenceRunResult{}, err
	}
	lock, err := goalrevision.Acquire(root, request.Authority.GoalID, binding.Revision, "cadence-run")
	if err != nil {
		return gaterun.CadenceRunResult{}, err
	}
	defer lock.Release()
	ulid, err := goal.NewOperationULID()
	if err != nil {
		return gaterun.CadenceRunResult{}, err
	}
	runID := "cadence-" + strings.ToLower(ulid)
	store := cadenceRunStore(root, owner, clock)
	creation, err := store.BeginCreation("cadence-run")
	if err != nil {
		return gaterun.CadenceRunResult{}, err
	}
	defer creation.Close()
	epoch := owner.Epoch
	nonce, err := store.Launch(runpkg.Caller{Class: "MAIN", MainId: owner.Lineage, OwnerLineage: owner.Lineage, ClaimEpoch: &epoch}, runpkg.LaunchParams{
		Id: runID, Kind: "suite", Display: "deep cadence validation", Log: filepath.Join("artifacts", "agents", "runs", runID+".log"),
		GoalId: request.Authority.GoalID, ObligationRevision: request.Authority.ObligationRevision, StandingShared: true,
		FenceGeneration: &creation.Generation,
	})
	if err != nil {
		return gaterun.CadenceRunResult{}, err
	}
	record, err := store.Read(runID)
	if err != nil || record == nil {
		return gaterun.CadenceRunResult{}, fmt.Errorf("cadence governed run record is unreadable: %v", err)
	}
	resultPath := filepath.Join(root, "artifacts", "agents", "proof-runs", "cadence", runID+".json")
	self, err := os.Executable()
	if err != nil {
		return gaterun.CadenceRunResult{}, err
	}
	testArgs := []string{"internal", "test", "run", "--root", root, "--goal", request.Authority.GoalID, "--tree", request.Trunk.Tree,
		"--mode", "deep", "--purpose", "cadence", "--result", resultPath}
	if request.ForceGroups {
		testArgs = append(testArgs, "--force-groups")
	}
	wrapArgs := append([]string{"run", "wrap", "--root", root, "--id", runID, "--nonce", nonce, "--log", record.Log, "--", self}, testArgs...)
	command := exec.Command(self, wrapArgs...)
	command.Dir, command.Env = root, os.Environ()
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := command.Start(); err != nil {
		_ = store.FailLaunch(runID, "wrapper spawn failed: "+err.Error())
		return gaterun.CadenceRunResult{}, err
	}
	if err := store.CompleteLaunch(runID, creation.Generation); err != nil {
		_ = command.Process.Kill()
		_ = command.Wait()
		return gaterun.CadenceRunResult{}, err
	}
	_ = command.Wait()
	if _, err := store.Assess(runID); err != nil {
		return gaterun.CadenceRunResult{}, err
	}
	record, err = store.Read(runID)
	if err != nil || record == nil || !runpkg.Terminal(record.Status) {
		return gaterun.CadenceRunResult{}, fmt.Errorf("cadence governed run did not reach a terminal record: %v", err)
	}
	data, err := os.ReadFile(resultPath)
	if err != nil {
		return gaterun.CadenceRunResult{}, err
	}
	var result proofrun.TestResult
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil {
		return gaterun.CadenceRunResult{}, err
	}
	return gaterun.CadenceRunResult{RunID: runID, Result: result}, nil
}
