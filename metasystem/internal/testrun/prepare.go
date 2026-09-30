// Package testrun is the testing selection and its verification: it
// prepares a test run from the committed testing contract (the protected
// contract, the trusted destination engine's policy decision, the candidate
// tree and the goal the run accounts to, after re-arming an engine that
// fell behind the landing ref by landed commits only), refuses a policy
// engine it cannot trust with the cause named, and composes the retained
// proof that covers a candidate without launching anything. The commands
// that parse flags, admit the attempt and print the result stay in
// cmd/metasystem.
package testrun

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	pathpkg "path"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/digest"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginecause"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	landinglane "github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/pathpattern"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/realpath"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/refusal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/shellquote"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// PolicyProbeWorkerEnvironment marks a worker started by the frozen
// public-v1 protection corpus: its policy child plans with it set, and the
// worker accepts only the literal probe shape under it.
const PolicyProbeWorkerEnvironment = "METASYSTEM_POLICY_PROBE_WORKER"

type Preparation struct {
	Installation, ControlRoot, ProjectRoot, Prefix, ConfPath string
	GoalID                                                   string
	AccountingRevision                                       uint64
	BaseCommit, PolicyBaseCommit, CandidateTree              string
	CandidateContract, BaseContract                          testpolicy.Contract
	EffectiveContract                                        testpolicy.Contract
	ContractDigest, BaseContractDigest                       string
	PolicyEngineDigest, BehaviorPolicyDigest                 string
	JudgeKey                                                 string
	EngineRearm                                              *proofrun.EngineRearm
	PolicyEngine                                             string
	FirstTestingTransition                                   bool
	Plan                                                     testpolicy.Plan
	Environment                                              []string
	AllGroups                                                bool
	AppAddress                                               string
	Workers, AdmissionMaximum                                int
	WorkerCapabilitiesChecked                                bool
	WorkerScratchPolicies                                    []string
	WorkerResultSchemas                                      []int
	UnmatchedInputs                                          []UnmatchedInput
}

type UnmatchedInput struct {
	Group   string `json:"group"`
	Pattern string `json:"pattern"`
}

type PlanOutput struct {
	SchemaVersion      int                `json:"schemaVersion"`
	ProjectRoot        string             `json:"projectRoot"`
	InstallationPrefix string             `json:"installationPrefix"`
	BaseCommit         string             `json:"baseCommit"`
	PolicyBaseCommit   string             `json:"policyBaseCommit"`
	CandidateTree      string             `json:"candidateTree"`
	ContractDigest     string             `json:"contractDigest"`
	BaseContractDigest string             `json:"baseContractDigest"`
	Plan               testpolicy.Plan    `json:"plan"`
	Groups             []testpolicy.Group `json:"groups"`
	UnmatchedInputs    []UnmatchedInput   `json:"unmatchedInputs,omitempty"`
}

func (prepared Preparation) ProofControlRoot() string {
	if prepared.ControlRoot != "" {
		return prepared.ControlRoot
	}
	return prepared.Installation
}

type SelectionRequest struct {
	Root, ControlRoot, GoalID, AuthorityGoalID, Tree, CapMin, RetryDecision, ResultPath string
	// LaneID charges the run to the landing lane instead of a goal: a batch
	// whose members are all changes (U11b).
	LaneID string
	// Verbose asks for the details behind a refusal: its code and facts.
	Verbose                                                    bool
	ExpectedGoalRevision, ExpectedAccountingRevision           uint64
	Mode                                                       testpolicy.Mode
	Purpose                                                    testpolicy.Purpose
	Groups, BatchRequirements                                  []string
	Carried                                                    bool
	NoReuse, ForceGroups, RequireDiagnosticHeadroom, AllGroups bool
	// AppAddress is the launch contract's check: the address of the
	// application run the named group is run against.
	AppAddress                                        string
	BatchPrefixReceipt, BatchTipProof, BatchAdmission bool
	FreshEpisode, FreshExpiresAt                      string
	// ExecutedWorkers is the worker allowance of the run a verification checks.
	// Group execution identity binds that allowance, and a fresh resolution can
	// differ because the default allowance follows available memory.
	ExecutedWorkers           int
	RequireWorkerCapabilities bool
	// CadencePreflight plans and revalidates the fetched tree before the cadence
	// tick claims standing authority. Governed cadence execution does not set it.
	CadencePreflight bool
	// LandedRearm is set by outermost plan and run commands. The pinned child
	// and the verify verb judge the engine as they find it.
	LandedRearm bool
	PolicyChild bool
	// Preparation is this invocation's preparation state, shared by every
	// preparation the invocation makes; nil gives one preparation its own.
	Preparation *PreparationState
	// Notes is the invocation's standard error, where a preparation says it
	// restarts; nil (a request no command parsed) is the process's own.
	Notes io.Writer
	// CallerPID is the supplied process a nested proof's parent
	// authentication starts from (design 6.2); zero is this process's parent,
	// the entry's own caller.
	CallerPID int64
}

// callerPID is the supplied caller, or this process's parent at an entry.
func (request SelectionRequest) EffectiveCallerPID() int64 {
	if request.CallerPID != 0 {
		return request.CallerPID
	}
	return int64(os.Getppid())
}

// PreparationState is invocation-local execution state (design 6.2,
// VOA-14): whether this invocation already restarted its preparation once
// after the landing ref moved. It lives in the request, never in the process
// environment, so a resident owner's earlier invocation cannot spend a later
// one's allowance.
type PreparationState struct {
	restarted bool
}

// The batch transport carries only exact group IDs. The current protected
// policy and prerequisite closure are recomputed by the selector.
func ParseBatchRequirements(value string) ([]string, error) {
	decoder := json.NewDecoder(strings.NewReader(value))
	opening, err := decoder.Token()
	if err != nil || opening != json.Delim('{') {
		return nil, fmt.Errorf("expected an object with a groups array")
	}
	key, err := decoder.Token()
	if err != nil || key != "groups" {
		return nil, fmt.Errorf("expected only the groups field")
	}
	var groups []string
	if err := decoder.Decode(&groups); err != nil || groups == nil {
		return nil, fmt.Errorf("groups must be an array of identifiers")
	}
	closing, err := decoder.Token()
	if err != nil || closing != json.Delim('}') {
		return nil, fmt.Errorf("unexpected batch requirements field")
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, fmt.Errorf("trailing batch requirements content")
	}
	seen := map[string]bool{}
	for _, id := range groups {
		if id == "" || id != strings.TrimSpace(id) || seen[id] {
			return nil, fmt.Errorf("groups must contain unique exact identifiers")
		}
		seen[id] = true
	}
	return groups, nil
}

func BatchRequirementsArgument(groups []string) string {
	if groups == nil {
		groups = []string{}
	}
	encoded, _ := json.Marshal(struct {
		Groups []string `json:"groups"`
	}{Groups: groups})
	return string(encoded)
}

type baseMove struct {
	ours, engine string
}

// noteStream is where a preparation tells what it does: the invocation's
// standard error, or the process's for a request no command parsed.
func (request SelectionRequest) noteStream() io.Writer {
	if request.Notes != nil {
		return request.Notes
	}
	return os.Stderr
}

func (move *baseMove) Error() string {
	return fmt.Sprintf("the landing ref moved under preparation from %s to %s", move.ours, move.engine)
}

type preparationAttempt func(SelectionRequest) (Preparation, error)

func Prepare(request SelectionRequest) (Preparation, error) {
	return prepareWith(request, prepareOnce)
}

func prepareWith(request SelectionRequest, attempt preparationAttempt) (Preparation, error) {
	state := request.Preparation
	if state == nil {
		state = &PreparationState{}
	}
	for {
		prepared, err := attempt(request)
		var move *baseMove
		if !errors.As(err, &move) {
			return prepared, err
		}
		if state.restarted {
			return Preparation{}, engineRefusal("base-moved", []enginecause.Fact{
				enginecause.Value("ours", move.ours), enginecause.Value("engine", move.engine), enginecause.Value("restarts", "1"),
			}, "the landing branch moved twice while this test run was starting")
		}
		fmt.Fprintf(request.noteStream(), "metasystem test run: the landing ref moved under the run (ours=%s engine=%s); restarting preparation once\n", move.ours, move.engine)
		state.restarted = true
	}
}

func prepareOnce(request SelectionRequest) (Preparation, error) {
	installation, err := realpath.Canonical(request.Root)
	if err != nil {
		return Preparation{}, err
	}
	controlRoot := installation
	if request.ControlRoot != "" {
		controlRoot, err = batchPrefixProofControlRoot(installation, request.ControlRoot)
		if err != nil {
			return Preparation{}, err
		}
	}
	confPath := filepath.Join(installation, "metasystem.conf")
	contractRel, present, err := config.CommittedLookup(confPath, "testing.contract")
	if err != nil || !present {
		return Preparation{}, fmt.Errorf("testing.contract is required in committed metasystem.conf")
	}
	if filepath.IsAbs(contractRel) || filepath.ToSlash(filepath.Clean(contractRel)) != contractRel || strings.HasPrefix(contractRel, "../") {
		return Preparation{}, fmt.Errorf("testing.contract must be a relative normalized path")
	}
	installationWorkspace := gittree.Workspace{Dir: installation}
	projectRoot, err := installationWorkspace.TopLevel()
	if err != nil {
		return Preparation{}, err
	}
	prefix, err := installationWorkspace.Prefix()
	if err != nil {
		return Preparation{}, err
	}
	workspace := gittree.Workspace{Dir: projectRoot}
	baseCommit, unborn, err := workspace.HeadCommit()
	if err != nil || unborn {
		return Preparation{}, fmt.Errorf("testing requires a committed project HEAD")
	}
	if landinglane.IsAccount(request.GoalID) {
		// An in-process caller names the lane where a goal would go (U11b).
		request.LaneID, request.GoalID = request.GoalID, ""
	}
	goalID := request.GoalID
	accountToGoal, err := accountsToGoal(request)
	if err != nil {
		return Preparation{}, err
	}
	if request.LaneID != "" {
		// Charged to the lane (U11b): no goal is resolved, and its attempts
		// are accounted to the lane's identity.
		goalID, accountToGoal = request.LaneID, false
	}
	if accountToGoal {
		goalID, err = resolveGoalFor(installation, request.GoalID, request.EffectiveCallerPID())
		if err != nil {
			return Preparation{}, err
		}
		// The trusted policy engine must judge the same candidate even when a
		// parent attempt, rather than an explicit flag, supplied its identity.
		request.GoalID = goalID
	}
	// A landed engine is trusted by its landing: when the enrolled engine is
	// behind the landing ref by landed commits only, the run fetches,
	// fast-forwards, rebuilds and re-arms before it judges anything.
	var engineRearm *proofrun.EngineRearm
	_, policyBaseBeforeRearmErr := TrustedPolicyBase(projectRoot, workspace)
	// A non-delivery plan remains informative without a configured destination;
	// delivery still enters re-arm so missing landing authority is a refusal.
	if request.LandedRearm && (request.Purpose == testpolicy.PurposeDelivery || policyBaseBeforeRearmErr == nil) {
		namedDeliveryTree := request.Tree != "" && request.Purpose == testpolicy.PurposeDelivery
		rearm, rearmErr := landedRearm(request.noteStream(), installation, projectRoot, prefix, namedDeliveryTree)
		if rearmErr != nil {
			return Preparation{}, rearmErr
		}
		engineRearm = rearm
	}
	policyBaseCommit, policyBaseErr := TrustedPolicyBase(projectRoot, workspace)
	if policyBaseErr != nil {
		if request.Purpose == testpolicy.PurposeDelivery {
			return Preparation{}, policyBaseErr
		}
		policyBaseCommit = baseCommit
	}
	policyBaseTree, err := workspace.TreeOf(policyBaseCommit)
	if err != nil {
		return Preparation{}, err
	}
	indexTree, err := workspace.StagedTree()
	if err != nil {
		return Preparation{}, fmt.Errorf("capture real candidate index: %w", err)
	}
	candidateTree := request.Tree
	if candidateTree == "" {
		candidateTree = indexTree
	} else {
		candidateTree, err = workspace.ResolveTree(candidateTree)
	}
	if err != nil {
		return Preparation{}, err
	}
	if request.Purpose == testpolicy.PurposeDelivery && candidateTree != indexTree {
		return Preparation{}, fmt.Errorf("delivery candidate must equal the real whole-project index: requested=%s index=%s", candidateTree, indexTree)
	}
	treeContractPath := filepath.ToSlash(filepath.Join(strings.TrimSuffix(prefix, "/"), contractRel))
	treeContractPath = strings.TrimPrefix(treeContractPath, "./")
	baseBytes, basePresent, err := workspace.FileAt(policyBaseTree, treeContractPath)
	if err != nil {
		return Preparation{}, err
	}
	candidateBytes, candidatePresent, err := workspace.FileAt(candidateTree, treeContractPath)
	if err != nil || !candidatePresent {
		return Preparation{}, fmt.Errorf("candidate testing contract %s is absent", treeContractPath)
	}
	candidateContract, err := testpolicy.Decode(candidateBytes)
	if err != nil {
		return Preparation{}, fmt.Errorf("candidate testing contract: %w", err)
	}
	baseContract := candidateContract
	baseContractDigest := digest.SHA256([]byte("absent\x00" + treeContractPath))
	if basePresent {
		baseContract, err = testpolicy.Decode(baseBytes)
		if err != nil {
			return Preparation{}, fmt.Errorf("base testing contract: %w", err)
		}
		baseContractDigest = digest.SHA256(baseBytes)
	}
	policyEngine, policyEngineDigest, currentIsPolicyEngine, err := TrustedPolicyEngine(installation, policyBaseCommit, !basePresent)
	if err != nil {
		return Preparation{}, err
	}
	workerCapabilitiesChecked, workerCapabilities := false, WorkerCapabilities{}
	if request.RequireWorkerCapabilities {
		capabilityContext, cancelCapabilities := context.WithCancel(context.Background())
		var capabilityErr error
		workerCapabilities, capabilityErr = RequireWorkerCapabilities(capabilityContext, policyEngine, Environment(os.Environ()))
		cancelCapabilities()
		if capabilityErr != nil {
			return Preparation{}, capabilityErr
		}
		workerCapabilitiesChecked = true
	}
	effective := protectedContractWithCandidateFallback(baseContract, candidateContract)
	if err := effective.Validate(); err != nil {
		return Preparation{}, fmt.Errorf("protected testing contract: %w", err)
	}
	if basePresent {
		if err := protectCoverageRatchets(workspace, policyBaseTree, candidateTree, prefix); err != nil {
			return Preparation{}, err
		}
	}
	mergeBases, err := workspace.MergeBases(baseCommit, policyBaseCommit)
	if err != nil || len(mergeBases) != 1 {
		return Preparation{}, fmt.Errorf("testing implementation and destination bases have no unique merge base")
	}
	changeBaseTree, err := workspace.TreeOf(mergeBases[0])
	if err != nil {
		return Preparation{}, err
	}
	changedPaths, err := workspace.ChangedPaths(changeBaseTree, candidateTree)
	if err != nil {
		return Preparation{}, err
	}
	// Expand protected Go package selectors against the exact trees before
	// policy selection. Each affected package becomes its own reusable group.
	selectionEnvironment := Environment(os.Environ())
	effective, err = proofrun.ExpandGoPackageGroupsWithEnvironment(effective, projectRoot, changeBaseTree, candidateTree, selectionEnvironment)
	if err != nil {
		return Preparation{}, err
	}
	risk, accountingRevision := testpolicy.GoalRisk{}, uint64(0)
	if landinglane.IsAccount(goalID) {
		// The lane has no goal risk; its one accounting revision is 1.
		accountingRevision = 1
	} else if accountToGoal {
		risk, accountingRevision, err = GoalRisk(installation, goalID)
		if err != nil {
			return Preparation{}, err
		}
	}
	plan, err := testpolicy.Select(effective, testpolicy.SelectionRequest{ChangedPaths: changedPaths, GoalRisk: risk,
		RequestedMode: request.Mode, Purpose: request.Purpose, Groups: request.Groups})
	if err != nil {
		return Preparation{}, err
	}
	if basePresent && !currentIsPolicyEngine {
		basePlan, basePlanErr := PlanWithTrustedPolicyEngine(policyEngine, TrustedPolicyFloorRequest(request), installation, candidateTree)
		if basePlanErr != nil {
			return Preparation{}, basePlanErr
		}
		if afterDigest, digestErr := digest.FileSHA256(policyEngine); digestErr != nil || afterDigest != policyEngineDigest {
			return Preparation{}, engineRefusal("enrollment-drift", engineCheckoutFacts(installation), "the pinned engine changed on disk while it chose the tests")
		}
		if mismatch := compareTrustedPolicyDecision(installation, projectRoot, candidateTree, policyBaseCommit, baseContractDigest, basePlan); mismatch != nil {
			return Preparation{}, mismatch
		}
		plan = basePlan.Plan
	}
	if len(request.BatchRequirements) > 0 {
		plan, err = testpolicy.WithBatchRequirements(effective, plan, request.BatchRequirements)
		if err != nil {
			return Preparation{}, err
		}
	}
	if !basePresent && request.Purpose == testpolicy.PurposeDelivery {
		plan, err = testpolicy.RequireFirstTransition(effective, plan)
		if err != nil {
			return Preparation{}, err
		}
	}
	if request.Purpose == testpolicy.PurposeDelivery && len(plan.Uncertainty) > 0 {
		return Preparation{}, fmt.Errorf("delivery impact is unresolved: %s; use diagnostic purpose for the bounded unknown groups", strings.Join(plan.Uncertainty, "; "))
	}
	if request.Purpose == testpolicy.PurposeDelivery {
		if err := CheckDeliveryInputParity(candidateTree, prefix, contractRel, effective, plan, deliveryParitySnapshot(workspace, prefix)); err != nil {
			return Preparation{}, err
		}
	}
	// Admission is a phase of the fully protected delivery decision. Preserve
	// the complete floor through trusted policy comparison and relevant-input
	// checks, then execute only the contract's admission subset. Legacy v1
	// contracts conservatively retain their whole selected plan here.
	if request.BatchAdmission {
		plan, err = testpolicy.AdmissionPlan(effective, plan)
		if err != nil {
			return Preparation{}, err
		}
	}
	unmatchedInputs, err := UnmatchedInputs(workspace, candidateTree, effective, plan)
	if err != nil {
		return Preparation{}, err
	}
	if policyBaseErr != nil {
		plan.Uncertainty = append(plan.Uncertainty, "trusted destination policy base unavailable: "+policyBaseErr.Error())
	}
	return Preparation{Installation: installation, ControlRoot: controlRoot, ProjectRoot: projectRoot, Prefix: strings.TrimSuffix(prefix, "/"),
		GoalID: goalID, AccountingRevision: accountingRevision,
		ConfPath: confPath, BaseCommit: baseCommit, PolicyBaseCommit: policyBaseCommit, CandidateTree: candidateTree, BaseContract: baseContract,
		CandidateContract: candidateContract, EffectiveContract: effective, ContractDigest: digest.SHA256(candidateBytes),
		BaseContractDigest: baseContractDigest, PolicyEngineDigest: policyEngineDigest, PolicyEngine: policyEngine,
		JudgeKey:                  proofrun.ComputeJudgeKey(context.Background(), projectRoot, policyBaseCommit, strings.TrimSuffix(prefix, "/")),
		EngineRearm:               engineRearm,
		WorkerCapabilitiesChecked: workerCapabilitiesChecked,
		WorkerScratchPolicies:     workerCapabilities.ScratchEnvironmentPolicies,
		WorkerResultSchemas:       workerCapabilities.TestResultSchemaVersions,
		UnmatchedInputs:           unmatchedInputs,
		FirstTestingTransition:    !basePresent,
		BehaviorPolicyDigest:      digest.SHA256(behaviorsurface.Bytes()), Plan: plan, Environment: selectionEnvironment,
		AllGroups: request.AllGroups, AppAddress: request.AppAddress}, nil
}

func protectedContractWithCandidateFallback(baseContract, candidateContract testpolicy.Contract) testpolicy.Contract {
	effective := testpolicy.ProtectedContract(baseContract, candidateContract)
	if effective.Fallback == "" {
		effective.Fallback = candidateContract.Fallback
	}
	return effective
}

func accountsToGoal(request SelectionRequest) (bool, error) {
	if request.CadencePreflight {
		if request.Purpose != testpolicy.PurposeCadence {
			return false, fmt.Errorf("cadence preflight requires cadence purpose")
		}
		return false, nil
	}
	return request.GoalID != "" || request.Purpose == testpolicy.PurposeDelivery || request.Purpose == testpolicy.PurposeCadence, nil
}

func compareTrustedPolicyDecision(installation, projectRoot, candidateTree, policyBaseCommit, baseContractDigest string, decision PlanOutput) error {
	seconds := 0
	limit := func() int {
		if seconds == 0 {
			seconds = steward.RearmResolveSeconds(installation)
		}
		return seconds
	}
	return compareTrustedPolicyDecisionWithReaders(candidateTree, policyBaseCommit, baseContractDigest, decision, projectRoot, policyBaseMoveReaders{
		isAncestor: func(root, ours, engine string) (bool, error) {
			_, err := landedRearmGitStep(context.Background(), landedRearmClock, limit(), "compare-moved-policy-base-ancestry", root,
				"merge-base", "--is-ancestor", ours, engine)
			if err != nil {
				var exitError *exec.ExitError
				if errors.As(err, &exitError) && exitError.ExitCode() == 1 {
					return false, nil
				}
				return false, err
			}
			return true, nil
		},
		localLandingRef: func(root string) (string, error) {
			return readLocalLandingRefText(context.Background(), landedRearmClock, limit(), root)
		},
		commitAtRef: func(root, ref string) (string, error) {
			return landedRearmGitStep(context.Background(), landedRearmClock, limit(), "reread-moved-policy-base", root,
				"rev-parse", "--verify", ref+"^{commit}")
		},
	})
}

type policyBaseMoveReaders struct {
	isAncestor      func(projectRoot, ours, engine string) (bool, error)
	localLandingRef func(projectRoot string) (string, error)
	commitAtRef     func(projectRoot, ref string) (string, error)
}

func compareTrustedPolicyDecisionWithReaders(candidateTree, policyBaseCommit, baseContractDigest string, decision PlanOutput, projectRoot string, readers policyBaseMoveReaders) error {
	mismatch := decisionMismatchRefusal(candidateTree, policyBaseCommit, baseContractDigest, decision)
	if mismatch == nil {
		return nil
	}
	// CandidateTree cannot be explained by a moved policy base. The base
	// contract digest can: it is read from that base's testing.json.
	if candidateTree != decision.CandidateTree || policyBaseCommit == decision.PolicyBaseCommit {
		return mismatch
	}
	moved, err := authenticatedPolicyBaseMove(projectRoot, policyBaseCommit, decision.PolicyBaseCommit, readers)
	if err != nil || !moved {
		return mismatch
	}
	return &baseMove{ours: policyBaseCommit, engine: decision.PolicyBaseCommit}
}

func TrustedPolicyFloorRequest(request SelectionRequest) SelectionRequest {
	request.BatchRequirements = nil
	request.BatchPrefixReceipt = false
	return request
}

func authenticatedPolicyBaseMove(projectRoot, ours, engine string, readers policyBaseMoveReaders) (bool, error) {
	ancestor, err := readers.isAncestor(projectRoot, ours, engine)
	if err != nil || !ancestor {
		return false, err
	}
	refText, err := readers.localLandingRef(projectRoot)
	if err != nil {
		return false, err
	}
	ref, _, _, err := parseLandingRefParts(refText)
	if err != nil {
		return false, err
	}
	current, err := readers.commitAtRef(projectRoot, ref)
	if err != nil {
		return false, err
	}
	return current == engine, nil
}

type CoverageBaseline struct {
	Floors map[string]float64 `json:"floors"`
}

func protectCoverageRatchets(workspace gittree.Workspace, baseTree, candidateTree, prefix string) error {
	inTree := func(relative string) string {
		return strings.TrimPrefix(filepath.ToSlash(filepath.Join(strings.TrimSuffix(prefix, "/"), relative)), "./")
	}
	for _, relative := range testpolicy.CoverageFloorsFiles() {
		path := inTree(relative)
		// A lowered floor is refused in plain words; the command restores
		// the file from the tree it is judged against.
		lowered := func(reason, background string) error {
			restore := "git -C " + shellquote.Word(workspace.Dir) + " restore --source=" + baseTree + " --staged --worktree -- " + shellquote.Word(path)
			return &refusal.Coded{Code: "TEST_POLICY_COVERAGE_FLOOR_LOWERED", Reason: errors.New(reason), Run: restore + "  (then run metasystem test run again)", Background: background}
		}
		// A base from before the floors moved beside testing.json keeps them
		// at the legacy path; the landing that moves them is judged by those.
		baseBytes, basePresent, err := workspace.FileAt(baseTree, path)
		if err == nil && !basePresent {
			baseBytes, basePresent, err = workspace.FileAt(baseTree, inTree(testpolicy.LegacyCoverageFloorsFile(relative)))
		}
		if err != nil || !basePresent {
			continue
		}
		candidateBytes, candidatePresent, err := workspace.FileAt(candidateTree, path)
		if err != nil || !candidatePresent {
			return lowered(fmt.Sprintf("this change deletes the coverage floors in %s; floors only rise", path), "")
		}
		var base, candidate CoverageBaseline
		if json.Unmarshal(baseBytes, &base) != nil || json.Unmarshal(candidateBytes, &candidate) != nil || len(base.Floors) == 0 || len(candidate.Floors) == 0 {
			return lowered(fmt.Sprintf("the coverage floors in %s cannot be read after this change", path), "")
		}
		for packageName, floor := range base.Floors {
			candidateFloor, present := candidate.Floors[packageName]
			if !present {
				// A floor leaves with its package: once the candidate tree
				// holds no Go file in the package's directory there is
				// nothing to measure, and the floor may go too.
				gone, err := coveragePackageGone(workspace, candidateTree, inTree(packageName))
				if err != nil {
					return lowered(fmt.Sprintf("this change removes the coverage floor of %s, whose package still exists", packageName), err.Error())
				}
				if gone {
					continue
				}
			}
			if !present || candidateFloor < floor {
				return lowered(fmt.Sprintf("this change lowers the coverage floor of %s from %.1f to %.1f; floors only rise", packageName, floor, candidateFloor), path)
			}
		}
	}
	return nil
}

// coveragePackageGone reports whether a tree holds no Go file directly in a
// package directory: the package was deleted, so its coverage floor has
// nothing left to measure.
func coveragePackageGone(workspace gittree.Workspace, tree, directory string) (bool, error) {
	entries, err := workspace.Entries(tree, []string{directory})
	if err != nil {
		return false, err
	}
	for path := range entries {
		if pathpkg.Dir(path) == directory && strings.HasSuffix(path, ".go") {
			return false, nil
		}
	}
	return true, nil
}

// retainedPolicyEngines hold their pins' preparation leases for the
// process's life; the kernel releases them when it exits.
var retainedPolicyEngines struct {
	sync.Mutex
	binaries []*steward.EnrolledBinary
}

func retainPolicyEngine(binary *steward.EnrolledBinary) {
	retainedPolicyEngines.Lock()
	defer retainedPolicyEngines.Unlock()
	retainedPolicyEngines.binaries = append(retainedPolicyEngines.binaries, binary)
}

func TrustedPolicyEngine(installation, policyBaseCommit string, firstTransition bool) (string, string, bool, error) {
	current, err := os.Executable()
	if err != nil {
		return "", "", false, err
	}
	engine := current
	if !firstTransition {
		enrollmentRoot := installation
		pinned, openErr := steward.OpenEnrolledBinary(enrollmentRoot)
		if openErr != nil {
			if borrowed, linked := linkedEnrollmentRoot(installation); linked {
				enrollmentRoot = borrowed
				pinned, openErr = steward.OpenEnrolledBinary(enrollmentRoot)
			}
		}
		if openErr != nil {
			return "", "", false, enrollmentRefusal(installation, openErr)
		}
		identity := pinned.Install
		if sourceErr := pinned.VerifySourceAtDestination(enrollmentRoot, policyBaseCommit); sourceErr != nil {
			_ = pinned.Close()
			facts := append(engineCheckoutFacts(installation), enginecause.Value("destination", policyBaseCommit))
			return "", "", false, judgmentRefusal(sourceErr, facts, "the pinned engine was not built from the landing branch this run tests against")
		}
		if prepareErr := pinned.PrepareForExecution(); prepareErr != nil {
			_ = pinned.Close()
			return "", "", false, engineRefusal(enginecause.TokenEngineUnavailable, engineCheckoutFacts(installation), "the pinned engine could not be opened to run", prepareErr.Error())
		}
		// The policy engine runs the plan and every worker of this run, long
		// after this returns: its pin keeps the preparation lease until the
		// process ends (engine-owns-disk-lifetimes 3.5), so no disk pass
		// removes it between two starts.
		retainPolicyEngine(pinned)
		engine = steward.EnrolledExecutionPath(enrollmentRoot, identity)
	}
	engineInfo, err := os.Stat(engine)
	if err != nil || !engineInfo.Mode().IsRegular() || engineInfo.Mode().Perm()&0o111 == 0 {
		facts := append(engineCheckoutFacts(installation), enginecause.Path("engine", engine))
		return "", "", false, engineRefusal(enginecause.TokenEngineUnavailable, facts, "the pinned engine is missing or not executable")
	}
	sum, err := digest.FileSHA256(engine)
	if err != nil {
		facts := append(engineCheckoutFacts(installation), enginecause.Path("engine", engine))
		return "", "", false, engineRefusal(enginecause.TokenEngineUnavailable, facts, "the pinned engine could not be read", err.Error())
	}
	currentInfo, currentErr := os.Stat(current)
	return engine, sum, currentErr == nil && os.SameFile(engineInfo, currentInfo), nil
}

func PlanWithTrustedPolicyEngine(engine string, request SelectionRequest, installation, candidateTree string) (PlanOutput, error) {
	args := []string{"test", "plan", "--root", installation, "--tree", candidateTree, "--mode", string(request.Mode), "--purpose", string(request.Purpose), "--json", "--policy-child"}
	if request.LaneID != "" {
		// A run charged to the lane is planned on the lane (U11b).
		args = append(append([]string{"internal"}, args...), "--lane", request.LaneID)
	} else if request.GoalID != "" {
		args = append(args, "--goal", request.GoalID)
	}
	if len(request.Groups) > 0 {
		args = append(args, "--groups", strings.Join(request.Groups, ","))
	}
	if request.BatchPrefixReceipt {
		args = append(args, "--batch-prefix")
	}
	// No clock on the planning engine: a plan that never returns is ended
	// by the attempt's cancellation, never by a wall bound under load.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	command := exec.CommandContext(ctx, engine, args...)
	command.Env = Environment(os.Environ())
	if os.Getenv(PolicyProbeWorkerEnvironment) == "1" {
		command.Env = append(command.Env, PolicyProbeWorkerEnvironment+"=1")
	}
	data, err := command.CombinedOutput()
	if err != nil && request.LaneID != "" && strings.Contains(string(data), "flag provided but not defined: -lane") {
		return PlanOutput{}, laneEngineTooOld(engine)
	}
	if err != nil {
		return PlanOutput{}, engineRefusal("child-failed", []enginecause.Fact{enginecause.Path("engine", engine), enginecause.Value("command", enginecause.Command(append([]string{engine}, args...)...))},
			fmt.Sprintf("the pinned engine failed while choosing which tests to run (%v)", err), strings.TrimSpace(string(data)))
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	var output PlanOutput
	if err := decoder.Decode(&output); err != nil {
		return PlanOutput{}, engineRefusal("child-output", []enginecause.Fact{enginecause.Path("engine", engine), enginecause.Value("command", enginecause.Command(append([]string{engine}, args...)...))}, "the pinned engine's choice of tests could not be read", err.Error())
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return PlanOutput{}, engineRefusal("child-output", []enginecause.Fact{enginecause.Path("engine", engine), enginecause.Value("command", enginecause.Command(append([]string{engine}, args...)...))}, "the pinned engine printed more than its choice of tests")
	}
	return output, nil
}

func RelevantInputs(prefix, contractRel string, contract testpolicy.Contract, plan testpolicy.Plan) ([]string, error) {
	joinPrefix := func(path string) string {
		return strings.TrimPrefix(filepath.ToSlash(filepath.Join(strings.TrimSuffix(prefix, "/"), path)), "./")
	}
	values := map[string]bool{
		joinPrefix(contractRel):       true,
		joinPrefix("metasystem.conf"): true,
	}
	groups := map[string]testpolicy.Group{}
	for _, group := range contract.Groups {
		groups[group.ID] = group
	}
	for _, id := range plan.SelectedGroups {
		group, ok := groups[id]
		if !ok {
			return nil, fmt.Errorf("selected testing group %s is absent", id)
		}
		for _, input := range group.Inputs {
			values[input] = true
		}
		if group.Adapter == "command" && len(group.Argv) > 0 && strings.Contains(group.Argv[0], "/") && !filepath.IsAbs(group.Argv[0]) {
			commandPath := filepath.ToSlash(filepath.Clean(filepath.Join(group.CWD, group.Argv[0])))
			if commandPath == ".." || strings.HasPrefix(commandPath, "../") {
				return nil, fmt.Errorf("testing group %s command executable escapes the project", id)
			}
			values[commandPath] = true
		}
	}
	paths := make([]string, 0, len(values))
	for path := range values {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths, nil
}

// deliveryParitySnapshot projects the relevant working-tree inputs the way
// the commit boundary judges the bytes it records: whatever the LANDING
// projection leaves out (the engine's per-run state, local configuration,
// built engines, coordination records hooks append to) stays at candidate
// bytes, even when a group declares the whole installation as its input.
// Otherwise a seat could never deliver: those bytes are on every seat's disk
// and in no commit.
func deliveryParitySnapshot(workspace gittree.Workspace, prefix string) func(string, []string) (string, error) {
	return func(candidateTree string, declarations []string) (string, error) {
		policy, err := behaviorsurface.Load()
		if err != nil {
			return "", err
		}
		return workspace.SnapshotRelevantExcluding(candidateTree, declarations, policy.LandingExclusions(prefix))
	}
}

func CheckDeliveryInputParity(candidateTree, prefix, contractRel string, contract testpolicy.Contract, plan testpolicy.Plan, snapshot func(candidateTree string, declarations []string) (string, error)) error {
	declarations, err := RelevantInputs(prefix, contractRel, contract, plan)
	if err != nil {
		return err
	}
	workingTree, err := snapshot(candidateTree, declarations)
	if err != nil {
		return fmt.Errorf("capture relevant candidate working inputs: %w", err)
	}
	if candidateTree != workingTree {
		return fmt.Errorf("delivery candidate differs from relevant working-tree inputs: candidate=%s working=%s", candidateTree, workingTree)
	}
	return nil
}

func UnmatchedInputs(workspace gittree.Workspace, tree string, contract testpolicy.Contract, plan testpolicy.Plan) ([]UnmatchedInput, error) {
	entries, err := workspace.Entries(tree, []string{"."})
	if err != nil {
		return nil, fmt.Errorf("inspect candidate input patterns: %w", err)
	}
	selected := map[string]bool{}
	for _, id := range plan.SelectedGroups {
		selected[id] = true
	}
	var unmatched []UnmatchedInput
	for _, group := range contract.Groups {
		if !selected[group.ID] {
			continue
		}
		for _, declaration := range group.Inputs {
			pattern, err := pathpattern.Parse(declaration)
			if err != nil {
				return nil, fmt.Errorf("testing group %s input %q: %w", group.ID, declaration, err)
			}
			found := false
			for name := range entries {
				if pattern.Covers(name) {
					found = true
					break
				}
			}
			if !found {
				unmatched = append(unmatched, UnmatchedInput{Group: group.ID, Pattern: pattern.String()})
			}
		}
	}
	return unmatched, nil
}

func PlanOutputOf(prepared Preparation) PlanOutput {
	groupsByID := map[string]testpolicy.Group{}
	for _, group := range prepared.EffectiveContract.Groups {
		groupsByID[group.ID] = group
	}
	groups := make([]testpolicy.Group, 0, len(prepared.Plan.SelectedGroups))
	for _, id := range prepared.Plan.SelectedGroups {
		groups = append(groups, groupsByID[id])
	}
	return PlanOutput{SchemaVersion: 1, ProjectRoot: prepared.ProjectRoot, InstallationPrefix: prepared.Prefix,
		BaseCommit: prepared.BaseCommit, PolicyBaseCommit: prepared.PolicyBaseCommit, CandidateTree: prepared.CandidateTree, ContractDigest: prepared.ContractDigest,
		BaseContractDigest: prepared.BaseContractDigest, Plan: prepared.Plan, Groups: groups, UnmatchedInputs: prepared.UnmatchedInputs}
}

func RunRequest(prepared Preparation, attemptID, logRoot, candidateEngine, candidateEngineDigest, candidateEngineBuildIdentity string) proofrun.TestRunRequest {
	request := proofrun.TestRunRequest{ProjectRoot: prepared.ProjectRoot, InstallationPrefix: prepared.Prefix,
		ControlRoot:   prepared.ProofControlRoot(),
		CandidateTree: prepared.CandidateTree, BaseCommit: prepared.BaseCommit, PolicyBaseCommit: prepared.PolicyBaseCommit,
		Contract: prepared.EffectiveContract, Plan: prepared.Plan, AttemptID: attemptID, Environment: prepared.Environment,
		LogRoot: logRoot, ContractDigest: prepared.ContractDigest, BaseContractDigest: prepared.BaseContractDigest,
		PolicyEngineDigest: prepared.PolicyEngineDigest, JudgeKey: prepared.JudgeKey, PolicyEngine: prepared.PolicyEngine, BehaviorPolicyDigest: prepared.BehaviorPolicyDigest,
		EngineRearm:     prepared.EngineRearm,
		CandidateEngine: candidateEngine, CandidateEngineDigest: candidateEngineDigest,
		CandidateEngineBuildIdentity: candidateEngineBuildIdentity, AllGroups: prepared.AllGroups,
		AppAddress: prepared.AppAddress}
	request.Workers, request.AdmissionMaximum = prepared.Workers, prepared.AdmissionMaximum
	request.ResultSchemaVersion = chooseTestResultSchema(prepared)
	return request
}
