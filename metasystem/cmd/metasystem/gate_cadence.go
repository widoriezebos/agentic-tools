package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gaterun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalrevision"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	runpkg "github.com/widoriezebos/agentic-tools/metasystem/internal/run"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

const (
	cadenceOwnerUnconfigured = "CADENCE_OWNER_UNCONFIGURED"
	cadenceNotLandingOwner   = "CADENCE_NOT_LANDING_OWNER"
	cadenceFetchRefused      = "CADENCE_FETCH_REFUSED"
	cadenceLedgerUnreadable  = "CADENCE_LEDGER_UNREADABLE"
	cadenceAuthorityGoal     = "standing-validation"
)

type cadenceRefusal struct{ code, detail string }

func (refusal cadenceRefusal) Error() string { return refusal.code + ": " + refusal.detail }

type cadenceTickOutput struct {
	Trunk gaterun.CadenceTrunk
	Tick  gaterun.CadenceTickResult
}

var cadenceTick = runProductionCadenceTick
var cadenceBuildIdentity = candidateEngineBuildIdentity
var cadenceRetainedEngineDigest = retainedCandidateEngineDigest

type cadenceRevalidationDependencies struct {
	prepare        func(testingSelectionRequest) (testingPreparation, error)
	readAttempts   func(string) ([]proofrun.Attempt, error)
	buildIdentity  func(context.Context, gittree.Workspace, string, string, []string) (string, error)
	retainedDigest func(testingPreparation, []proofrun.Attempt, string, bool) (string, error)
	openCandidate  func(projectRoot, candidateTree string) (proofrun.CandidateWorkspace, error)
}

func productionCadenceRevalidationDependencies() cadenceRevalidationDependencies {
	return cadenceRevalidationDependencies{prepare: prepareTestingForCommand, readAttempts: proofrun.ReadAttempts,
		buildIdentity: cadenceBuildIdentity, retainedDigest: cadenceRetainedEngineDigest}
}

func runGateCadenceTick(args []string) int {
	flags := flag.NewFlagSet("gate cadence-tick", flag.ContinueOnError)
	root := pathFlag(flags, "root", "", "configured landing-owner checkout")
	if flags.Parse(args) != nil || *root == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem gate cadence-tick --root REPO")
		return 2
	}
	owner, err := configuredCadenceOwner(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 3
	}
	if owner != canonicalCadenceRoot(*root) {
		fmt.Fprintf(os.Stderr, "%s: configured landing owner is %s\n", cadenceNotLandingOwner, owner)
		return 3
	}
	held, err := batchOwnerAcquire(owner)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 3
	}
	defer held.retire()
	output, err := cadenceTick(owner, held, cadenceProductionClock)
	if err != nil {
		var refusal cadenceRefusal
		if errors.As(err, &refusal) {
			fmt.Fprintln(os.Stderr, refusal)
		} else {
			fmt.Fprintln(os.Stderr, cadenceRefusal{cadenceLedgerUnreadable, err.Error()})
		}
		return 3
	}
	trigger, happened, attempt := cadenceResultWords(output.Tick)
	fmt.Printf("cadence trigger=%s happened=%s trunk-commit=%s trunk-tree=%s", trigger, happened, output.Trunk.Commit, output.Trunk.Tree)
	if attempt != "" {
		fmt.Printf(" attempt=%s", attempt)
	}
	fmt.Println()
	return 0
}

func canonicalCadenceRoot(root string) string {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return filepath.Clean(root)
	}
	if resolved, resolveErr := filepath.EvalSymlinks(absolute); resolveErr == nil {
		absolute = resolved
	}
	return filepath.Clean(absolute)
}

func configuredCadenceOwner(root string) (string, error) {
	value, _, err := config.Get(config.GetParams{Key: config.BatchRootKey, ConfPath: filepath.Join(root, "metasystem.conf"), Default: "", DefaultSet: true})
	if err != nil || strings.TrimSpace(value) == "" {
		return "", cadenceRefusal{cadenceOwnerUnconfigured, "landing.batch-root is not configured"}
	}
	if !filepath.IsAbs(value) {
		return "", cadenceRefusal{cadenceOwnerUnconfigured, "landing.batch-root must be absolute"}
	}
	return canonicalCadenceRoot(value), nil
}

func cadenceResultWords(result gaterun.CadenceTickResult) (trigger, happened, attempt string) {
	trigger, happened = "none", "reused"
	selected := result.Trigger
	if selected == "" && result.Status != nil {
		selected = result.Status.Trigger
	}
	switch selected {
	case goal.CadenceTriggerIdentityChanged:
		trigger = "identity"
	case goal.CadenceTriggerWeightDue:
		trigger = "weight"
	case goal.CadenceTriggerForcedWindow:
		trigger = "six-hour"
	}
	if result.Status != nil {
		if result.Status.Trigger == goal.CadenceTriggerRevalidation {
			happened = "revalidation-only"
		} else if result.Published && !result.Executed && result.ClaimOutcome == goal.CadenceClaimAcquired {
			happened = strings.Join([]string{"authority", "unavailable"}, "-")
		} else if result.Executed {
			attempt = result.Status.AttemptID
			happened = "executed"
			allReused := len(result.Status.Groups) != 0
			for _, group := range result.Status.Groups {
				allReused = allReused && group.Status == "reused"
			}
			if allReused {
				happened = "reused"
			}
		}
	}
	switch result.ClaimOutcome {
	case goal.CadenceClaimJoined, goal.CadenceClaimOccupied:
		happened = "joined"
	case goal.CadenceClaimComplete:
		happened = "terminal-read"
		if result.Status != nil {
			attempt = result.Status.AttemptID
		}
	}
	return trigger, happened, attempt
}

func runProductionCadenceTick(root string, held batchOwnerLease, clock func() time.Time) (cadenceTickOutput, error) {
	commit, tree, err := fetchBatchOrigin(root)
	if err != nil {
		return cadenceTickOutput{}, cadenceRefusal{cadenceFetchRefused, err.Error()}
	}
	trunk := gaterun.CadenceTrunk{Commit: commit, Tree: tree}
	now := clock().UTC()
	endpoint, err := goal.ResolveEndpoint(root)
	if err != nil {
		return cadenceTickOutput{}, cadenceRefusal{cadenceLedgerUnreadable, err.Error()}
	}
	projection, err := goal.Project(endpoint, false, now)
	if err != nil {
		return cadenceTickOutput{}, cadenceRefusal{cadenceLedgerUnreadable, err.Error()}
	}
	prepared, err := prepareTestingForCommand(cadencePreparationRequest(root, tree))
	if err != nil {
		return cadenceTickOutput{}, cadenceRefusal{cadenceLedgerUnreadable, err.Error()}
	}
	deepOnly, err := testpolicy.DeepOnlySectionGroupIDs(prepared.EffectiveContract)
	if err != nil {
		return cadenceTickOutput{}, cadenceRefusal{cadenceLedgerUnreadable, err.Error()}
	}
	weight, due, err := gaterun.WeightCheckAt(root, weightThreshold(root), now)
	if err != nil {
		return cadenceTickOutput{}, cadenceRefusal{cadenceLedgerUnreadable, err.Error()}
	}
	machine, err := goal.ResolveMachine(root)
	if err != nil {
		return cadenceTickOutput{}, cadenceRefusal{cadenceLedgerUnreadable, err.Error()}
	}
	actor := goal.Actor{Machine: machine, Lineage: landingOwnerLineage}
	deps := gaterun.CadenceDependencies{
		Clock: clock, Fetch: func() (gaterun.CadenceTrunk, error) { return trunk, nil },
		Revalidate: func(fetched gaterun.CadenceTrunk) (gaterun.CadenceRevalidation, error) {
			return revalidateCadence(root, prepared, fetched, deepOnly)
		},
		Ledger: gaterun.GoalCadenceLedger{Endpoint: endpoint, Actor: actor},
	}
	deps.ClaimAuthority = func(at time.Time) (gaterun.CadenceAuthority, error) {
		return claimCadenceAuthority(endpoint, actor, held.epoch, at)
	}
	deps.ReleaseAuthority = func(authority gaterun.CadenceAuthority, at time.Time) error {
		return releaseCadenceAuthority(endpoint, actor, authority, at)
	}
	deps.Run = func(request gaterun.CadenceRunRequest) (gaterun.CadenceRunResult, error) {
		return executeCadenceRun(root, held, clock, request)
	}
	deps.DischargeWeight = func(authority gaterun.CadenceAuthority, runID string, _ uint64, at time.Time) error {
		_, dischargeErr := gaterun.WeightDischargeAt(root, authority.GoalID, authority.ObligationRevision, runID, at)
		return dischargeErr
	}
	result, err := gaterun.RunCadenceTick(gaterun.CadenceTickInput{Latest: projection.Tree.Cadence, Weight: weight, WeightDue: due,
		DeepOnlyGroups: deepOnly, Lease: gaterun.CadenceForcedInterval}, deps)
	return cadenceTickOutput{Trunk: trunk, Tick: result}, err
}

func revalidateCadence(root string, prepared testingPreparation, trunk gaterun.CadenceTrunk, deepOnly []string) (gaterun.CadenceRevalidation, error) {
	return revalidateCadenceWith(root, prepared, trunk, deepOnly, productionCadenceRevalidationDependencies())
}

func revalidateCadenceWith(root string, prepared testingPreparation, trunk gaterun.CadenceTrunk, deepOnly []string, dependencies cadenceRevalidationDependencies) (gaterun.CadenceRevalidation, error) {
	if prepared.CandidateTree != trunk.Tree {
		var err error
		prepared, err = dependencies.prepare(cadencePreparationRequest(root, trunk.Tree))
		if err != nil {
			return gaterun.CadenceRevalidation{}, err
		}
	}
	if _, err := resolveTestingPreparationWorkerPolicy(&prepared); err != nil {
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
	request := testingRunRequest(prepared, "", "", "", digest, buildIdentity)
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

func cadenceCandidateEngineIdentity(prepared testingPreparation, trunk gaterun.CadenceTrunk, attempts []proofrun.Attempt) (string, string, bool, error) {
	return cadenceCandidateEngineIdentityWith(prepared, trunk, attempts, productionCadenceRevalidationDependencies())
}

func cadenceCandidateEngineIdentityWith(prepared testingPreparation, trunk gaterun.CadenceTrunk, attempts []proofrun.Attempt, dependencies cadenceRevalidationDependencies) (string, string, bool, error) {
	environment := inheritedTestingEnvironment(prepared.Environment, os.Environ())
	buildIdentity, err := dependencies.buildIdentity(context.Background(), gittree.Workspace{Dir: prepared.ProjectRoot}, prepared.Prefix, trunk.Tree, environment)
	if err != nil {
		return "", "", false, err
	}
	digest, retainedErr := dependencies.retainedDigest(prepared, attempts, buildIdentity, false)
	if retainedErr == nil {
		return buildIdentity, digest, true, nil
	}
	// Revalidation runs before the shared claim and must remain a no-build
	// probe. The marker forces a claimed native run; that run's exact group
	// identities replace these deliberately non-green probe identities.
	return buildIdentity, bytesSHA256([]byte("cadence-missing-engine-evidence\x00" + buildIdentity)), false, nil
}

func cadencePreparationRequest(root, tree string) testingSelectionRequest {
	return testingSelectionRequest{Root: root, Tree: tree, Mode: testpolicy.ModeDeep, Purpose: testpolicy.PurposeCadence, CadencePreflight: true}
}

func cadenceRequest(endpoint goal.Endpoint, actor goal.Actor, epoch int64, at time.Time) (goal.VerbRequest, error) {
	ulid, err := goal.NewOperationULID()
	return goal.VerbRequest{Endpoint: endpoint, Actor: actor, Ulid: ulid, Now: at.UTC(), ClaimEpoch: epoch}, err
}

func claimCadenceAuthority(endpoint goal.Endpoint, actor goal.Actor, epoch int64, at time.Time) (gaterun.CadenceAuthority, error) {
	if binding, err := dispatchcore.ResolveGoalBinding(endpoint.Root, cadenceAuthorityGoal, at); err == nil {
		if binding.Machine != actor.Machine || binding.Lineage != actor.Lineage || binding.File.Obligation == nil {
			return gaterun.CadenceAuthority{}, fmt.Errorf("standing cadence authority is held elsewhere")
		}
		return gaterun.CadenceAuthority{GoalID: cadenceAuthorityGoal, ObligationRevision: binding.File.Obligation.Revision}, nil
	}
	request, err := cadenceRequest(endpoint, actor, epoch, at)
	if err != nil {
		return gaterun.CadenceAuthority{}, err
	}
	if _, err = goal.Claim(request, cadenceAuthorityGoal); err != nil {
		var noChange goal.NothingToDo
		if !errors.As(err, &noChange) {
			return gaterun.CadenceAuthority{}, err
		}
	}
	binding, err := dispatchcore.ResolveGoalBinding(endpoint.Root, cadenceAuthorityGoal, at)
	if err != nil || binding.Machine != actor.Machine || binding.Lineage != actor.Lineage || binding.File.Obligation == nil {
		return gaterun.CadenceAuthority{}, fmt.Errorf("standing cadence authority is unavailable: %v", err)
	}
	return gaterun.CadenceAuthority{GoalID: cadenceAuthorityGoal, ObligationRevision: binding.File.Obligation.Revision}, nil
}

func releaseCadenceAuthority(endpoint goal.Endpoint, actor goal.Actor, authority gaterun.CadenceAuthority, at time.Time) error {
	request, err := cadenceRequest(endpoint, actor, 0, at)
	if err != nil {
		return err
	}
	_, err = goal.Release(request, authority.GoalID)
	return err
}

func cadenceRunStore(root string, held batchOwnerLease, clock func() time.Time) *runpkg.Store {
	return &runpkg.Store{Root: root, Now: clock, CurrentEpoch: func() (*int64, bool) {
		if batchOwnerRequire(held) != nil {
			return nil, false
		}
		epoch := held.epoch
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

func executeCadenceRun(root string, held batchOwnerLease, clock func() time.Time, request gaterun.CadenceRunRequest) (gaterun.CadenceRunResult, error) {
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
	store := cadenceRunStore(root, held, clock)
	creation, err := store.BeginCreation("cadence-run")
	if err != nil {
		return gaterun.CadenceRunResult{}, err
	}
	defer creation.Close()
	epoch := held.epoch
	nonce, err := store.Launch(runpkg.Caller{Class: "MAIN", MainId: landingOwnerLineage, OwnerLineage: landingOwnerLineage, ClaimEpoch: &epoch}, runpkg.LaunchParams{
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
	testArgs := []string{"test", "run", "--root", root, "--goal", request.Authority.GoalID, "--tree", request.Trunk.Tree,
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
