package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

type testingPreparation struct {
	Installation, ProjectRoot, Prefix, ConfPath string
	GoalID                                      string
	AccountingRevision                          uint64
	BaseCommit, PolicyBaseCommit, CandidateTree string
	CandidateContract, BaseContract             testpolicy.Contract
	EffectiveContract                           testpolicy.Contract
	ContractDigest, BaseContractDigest          string
	PolicyEngineDigest, BehaviorPolicyDigest    string
	PolicyEngine                                string
	FirstTestingTransition                      bool
	Plan                                        testpolicy.Plan
	Environment                                 []string
}

type testingPlanOutput struct {
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
}

func runTestList(args []string) int {
	flags := flag.NewFlagSet("test list", flag.ContinueOnError)
	root := flags.String("root", "", "MetaSystem installation root")
	jsonOutput := flags.Bool("json", false, "emit structured JSON")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *root == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem test list --root INSTALLATION [--json]")
		return 2
	}
	installation, contract, path, err := loadPhysicalTestingContract(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "metasystem test list:", err)
		return 1
	}
	if *jsonOutput {
		printJSON(map[string]any{"schemaVersion": 1, "installation": installation, "contract": path, "groups": contract.Groups})
		return 0
	}
	for _, group := range contract.Groups {
		fmt.Printf("%s\t%s\t%s\n", group.ID, group.Kind, group.Adapter)
	}
	return 0
}

func runTestCheck(args []string) int {
	flags := flag.NewFlagSet("test check", flag.ContinueOnError)
	root := flags.String("root", "", "MetaSystem installation root")
	jsonOutput := flags.Bool("json", false, "emit structured JSON")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *root == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem test check --root INSTALLATION [--json]")
		return 2
	}
	installation, contract, path, err := loadPhysicalTestingContract(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "metasystem test check:", err)
		return 1
	}
	projectRoot, err := (gittree.Workspace{Dir: installation}).TopLevel()
	if err == nil {
		err = checkTestingTools(projectRoot, contract)
	}
	if err == nil {
		err = proofrun.CheckNativeDiscovery(projectRoot, installation, contract)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "metasystem test check:", err)
		return 1
	}
	if *jsonOutput {
		printJSON(map[string]any{"schemaVersion": 1, "status": "ready", "contract": path, "groupCount": len(contract.Groups)})
	} else {
		fmt.Printf("TEST-CONTRACT READY groups=%d contract=%s\n", len(contract.Groups), path)
	}
	return 0
}

func runTestPlan(args []string) int {
	request, jsonOutput, status := parseTestingSelection("test plan", args, false)
	if status != 0 {
		return status
	}
	prepared, err := prepareTesting(request)
	if err != nil {
		fmt.Fprintln(os.Stderr, "metasystem test plan:", err)
		return 1
	}
	output := planOutput(prepared)
	if jsonOutput {
		printJSON(output)
	} else {
		fmt.Printf("TEST-PLAN mode=%s required=%s tree=%s groups=%s\n", prepared.Plan.ExecutedMode,
			prepared.Plan.RequiredMode, prepared.CandidateTree, strings.Join(prepared.Plan.SelectedGroups, ","))
	}
	return 0
}

type testingSelectionRequest struct {
	Root, GoalID, Tree, CapMin, RetryDecision, ResultPath string
	Mode                                                  testpolicy.Mode
	Purpose                                               testpolicy.Purpose
	Groups                                                []string
}

func parseTestingSelection(name string, args []string, execution bool) (testingSelectionRequest, bool, int) {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	request := testingSelectionRequest{}
	flags.StringVar(&request.Root, "root", "", "MetaSystem installation root")
	flags.StringVar(&request.GoalID, "goal", "", "accepted goal owning delivery")
	flags.StringVar(&request.Tree, "tree", "", "exact whole-project candidate tree")
	mode := flags.String("mode", "auto", "auto, standard, deep, or diagnostic canary")
	purpose := flags.String("purpose", "delivery", "delivery, diagnostic, or cadence")
	groups := flags.String("groups", "", "comma-separated diagnostic groups")
	jsonOutput := flags.Bool("json", false, "emit structured JSON")
	if execution {
		flags.StringVar(&request.CapMin, "cap-min", "", "reserved proof minutes")
		flags.StringVar(&request.RetryDecision, "retry-decision", "", "accountable version-1 retry decision")
		flags.StringVar(&request.ResultPath, "result", "", "atomic result projection path")
	}
	if flags.Parse(args) != nil || flags.NArg() != 0 || request.Root == "" {
		fmt.Fprintf(os.Stderr, "usage: metasystem %s --root INSTALLATION [--goal ID] [--tree TREE] [--mode auto|standard|deep|canary] [--purpose delivery|diagnostic|cadence] [--groups ID,ID]\n", name)
		return request, false, 2
	}
	purposeSet := false
	flags.Visit(func(value *flag.Flag) { purposeSet = purposeSet || value.Name == "purpose" })
	if *mode == string(testpolicy.ModeCanary) && !purposeSet {
		*purpose = string(testpolicy.PurposeDiagnostic)
	}
	if *groups != "" {
		for _, id := range strings.Split(*groups, ",") {
			if strings.TrimSpace(id) == "" || id != strings.TrimSpace(id) {
				fmt.Fprintln(os.Stderr, "diagnostic groups must be a comma-separated list of exact identifiers")
				return request, false, 2
			}
			request.Groups = append(request.Groups, id)
		}
	}
	request.Mode, request.Purpose = testpolicy.Mode(*mode), testpolicy.Purpose(*purpose)
	return request, *jsonOutput, 0
}

func prepareTesting(request testingSelectionRequest) (testingPreparation, error) {
	installation, err := canonicalProofRoot(request.Root)
	if err != nil {
		return testingPreparation{}, err
	}
	confPath := filepath.Join(installation, "metasystem.conf")
	contractRel, present, err := config.ConfLookup(confPath, "testing.contract")
	if err != nil || !present {
		return testingPreparation{}, fmt.Errorf("testing.contract is required in committed metasystem.conf")
	}
	if filepath.IsAbs(contractRel) || filepath.ToSlash(filepath.Clean(contractRel)) != contractRel || strings.HasPrefix(contractRel, "../") {
		return testingPreparation{}, fmt.Errorf("testing.contract must be a relative normalized path")
	}
	installationWorkspace := gittree.Workspace{Dir: installation}
	projectRoot, err := installationWorkspace.TopLevel()
	if err != nil {
		return testingPreparation{}, err
	}
	prefix, err := installationWorkspace.Prefix()
	if err != nil {
		return testingPreparation{}, err
	}
	workspace := gittree.Workspace{Dir: projectRoot}
	baseCommit, unborn, err := workspace.HeadCommit()
	if err != nil || unborn {
		return testingPreparation{}, fmt.Errorf("testing requires a committed project HEAD")
	}
	goalID := request.GoalID
	if goalID != "" || request.Purpose == testpolicy.PurposeDelivery || request.Purpose == testpolicy.PurposeCadence {
		goalID, err = resolveTestingGoal(installation, request.GoalID)
		if err != nil {
			return testingPreparation{}, err
		}
	}
	policyBaseCommit, policyBaseErr := trustedTestingPolicyBase(projectRoot, workspace)
	if policyBaseErr != nil {
		if request.Purpose == testpolicy.PurposeDelivery {
			return testingPreparation{}, policyBaseErr
		}
		policyBaseCommit = baseCommit
	}
	policyBaseTree, err := workspace.TreeOf(policyBaseCommit)
	if err != nil {
		return testingPreparation{}, err
	}
	indexTree, err := workspace.StagedTree()
	if err != nil {
		return testingPreparation{}, fmt.Errorf("capture real candidate index: %w", err)
	}
	candidateTree := request.Tree
	if candidateTree == "" {
		candidateTree = indexTree
	} else {
		candidateTree, err = workspace.ResolveTree(candidateTree)
	}
	if err != nil {
		return testingPreparation{}, err
	}
	if request.Purpose == testpolicy.PurposeDelivery && candidateTree != indexTree {
		return testingPreparation{}, fmt.Errorf("delivery candidate must equal the real whole-project index: requested=%s index=%s", candidateTree, indexTree)
	}
	treeContractPath := filepath.ToSlash(filepath.Join(strings.TrimSuffix(prefix, "/"), contractRel))
	treeContractPath = strings.TrimPrefix(treeContractPath, "./")
	baseBytes, basePresent, err := workspace.FileAt(policyBaseTree, treeContractPath)
	if err != nil {
		return testingPreparation{}, err
	}
	candidateBytes, candidatePresent, err := workspace.FileAt(candidateTree, treeContractPath)
	if err != nil || !candidatePresent {
		return testingPreparation{}, fmt.Errorf("candidate testing contract %s is absent", treeContractPath)
	}
	candidateContract, err := testpolicy.Decode(candidateBytes)
	if err != nil {
		return testingPreparation{}, fmt.Errorf("candidate testing contract: %w", err)
	}
	baseContract := candidateContract
	baseContractDigest := bytesSHA256([]byte("absent\x00" + treeContractPath))
	if basePresent {
		baseContract, err = testpolicy.Decode(baseBytes)
		if err != nil {
			return testingPreparation{}, fmt.Errorf("base testing contract: %w", err)
		}
		baseContractDigest = bytesSHA256(baseBytes)
	}
	policyEngine, policyEngineDigest, currentIsPolicyEngine, err := trustedPolicyEngine(installation, policyBaseCommit, !basePresent)
	if err != nil {
		return testingPreparation{}, err
	}
	effective := testpolicy.ProtectedContract(baseContract, candidateContract)
	if err := effective.Validate(); err != nil {
		return testingPreparation{}, fmt.Errorf("protected testing contract: %w", err)
	}
	if basePresent {
		if err := protectCoverageRatchets(workspace, policyBaseTree, candidateTree, prefix); err != nil {
			return testingPreparation{}, err
		}
	}
	mergeBases, err := workspace.MergeBases(baseCommit, policyBaseCommit)
	if err != nil || len(mergeBases) != 1 {
		return testingPreparation{}, fmt.Errorf("testing implementation and destination bases have no unique merge base")
	}
	changeBaseTree, err := workspace.TreeOf(mergeBases[0])
	if err != nil {
		return testingPreparation{}, err
	}
	changedPaths, err := workspace.ChangedPaths(changeBaseTree, candidateTree)
	if err != nil {
		return testingPreparation{}, err
	}
	risk, accountingRevision, err := testingGoalRisk(installation, goalID)
	if err != nil {
		return testingPreparation{}, err
	}
	plan, err := testpolicy.Select(effective, testpolicy.SelectionRequest{ChangedPaths: changedPaths, GoalRisk: risk,
		RequestedMode: request.Mode, Purpose: request.Purpose, Groups: request.Groups})
	if err != nil {
		return testingPreparation{}, err
	}
	if basePresent && !currentIsPolicyEngine {
		basePlan, basePlanErr := planWithTrustedPolicyEngine(policyEngine, request, installation, candidateTree)
		if basePlanErr != nil {
			return testingPreparation{}, basePlanErr
		}
		if afterDigest, digestErr := fileSHA256(policyEngine); digestErr != nil || afterDigest != policyEngineDigest {
			return testingPreparation{}, fmt.Errorf("TEST_POLICY_ENGINE_REQUIRED: retained trusted-base engine changed during policy selection")
		}
		if basePlan.CandidateTree != candidateTree || basePlan.PolicyBaseCommit != policyBaseCommit ||
			basePlan.BaseContractDigest != baseContractDigest {
			return testingPreparation{}, fmt.Errorf("TEST_POLICY_ENGINE_REQUIRED: retained trusted-base engine returned a mismatched policy decision")
		}
		plan = basePlan.Plan
	}
	if !basePresent && request.Purpose == testpolicy.PurposeDelivery {
		plan, err = testpolicy.RequireFirstTransition(effective, plan)
		if err != nil {
			return testingPreparation{}, err
		}
	}
	if request.Purpose == testpolicy.PurposeDelivery && len(plan.Uncertainty) > 0 {
		return testingPreparation{}, fmt.Errorf("delivery impact is unresolved: %s; use diagnostic purpose for the bounded unknown groups", strings.Join(plan.Uncertainty, "; "))
	}
	if request.Purpose == testpolicy.PurposeDelivery {
		declarations, declarationErr := testingRelevantInputs(prefix, contractRel, effective, plan)
		if declarationErr != nil {
			return testingPreparation{}, declarationErr
		}
		workingTree, snapshotErr := workspace.SnapshotRelevant(candidateTree, declarations)
		if snapshotErr != nil {
			return testingPreparation{}, fmt.Errorf("capture relevant candidate working inputs: %w", snapshotErr)
		}
		if candidateTree != workingTree {
			return testingPreparation{}, fmt.Errorf("delivery candidate differs from relevant working-tree inputs: candidate=%s working=%s", candidateTree, workingTree)
		}
	}
	if policyBaseErr != nil {
		plan.Uncertainty = append(plan.Uncertainty, "trusted destination policy base unavailable: "+policyBaseErr.Error())
	}
	return testingPreparation{Installation: installation, ProjectRoot: projectRoot, Prefix: strings.TrimSuffix(prefix, "/"),
		GoalID: goalID, AccountingRevision: accountingRevision,
		ConfPath: confPath, BaseCommit: baseCommit, PolicyBaseCommit: policyBaseCommit, CandidateTree: candidateTree, BaseContract: baseContract,
		CandidateContract: candidateContract, EffectiveContract: effective, ContractDigest: bytesSHA256(candidateBytes),
		BaseContractDigest: baseContractDigest, PolicyEngineDigest: policyEngineDigest, PolicyEngine: policyEngine,
		FirstTestingTransition: !basePresent,
		BehaviorPolicyDigest:   bytesSHA256(behaviorsurface.Bytes()), Plan: plan, Environment: testingEnvironment(os.Environ())}, nil
}

type protectedCoverageBaseline struct {
	Floors map[string]float64 `json:"floors"`
}

func protectCoverageRatchets(workspace gittree.Workspace, baseTree, candidateTree, prefix string) error {
	for _, relative := range []string{"scripts/agents/coverage-ratchet.json", "scripts/agents/coverage-ratchet-linux.json"} {
		path := strings.TrimPrefix(filepath.ToSlash(filepath.Join(strings.TrimSuffix(prefix, "/"), relative)), "./")
		baseBytes, basePresent, err := workspace.FileAt(baseTree, path)
		if err != nil || !basePresent {
			continue
		}
		candidateBytes, candidatePresent, err := workspace.FileAt(candidateTree, path)
		if err != nil || !candidatePresent {
			return fmt.Errorf("TEST_POLICY_COVERAGE_FLOOR_LOWERED: protected coverage baseline %s is absent", path)
		}
		var base, candidate protectedCoverageBaseline
		if json.Unmarshal(baseBytes, &base) != nil || json.Unmarshal(candidateBytes, &candidate) != nil || len(base.Floors) == 0 || len(candidate.Floors) == 0 {
			return fmt.Errorf("TEST_POLICY_COVERAGE_FLOOR_LOWERED: protected coverage baseline %s is malformed", path)
		}
		for packageName, floor := range base.Floors {
			candidateFloor, present := candidate.Floors[packageName]
			if !present || candidateFloor < floor {
				return fmt.Errorf("TEST_POLICY_COVERAGE_FLOOR_LOWERED: %s floor %s changed from %.1f to %.1f", path, packageName, floor, candidateFloor)
			}
		}
	}
	return nil
}

func trustedPolicyEngine(installation, policyBaseCommit string, firstTransition bool) (string, string, bool, error) {
	current, err := os.Executable()
	if err != nil {
		return "", "", false, err
	}
	engine := current
	if !firstTransition {
		pinned, openErr := steward.OpenEnrolledBinary(installation)
		if openErr != nil {
			return "", "", false, fmt.Errorf("TEST_POLICY_ENGINE_REQUIRED: retained destination engine is not authenticated: %w", openErr)
		}
		identity := pinned.Install
		defer pinned.Close()
		if sourceErr := pinned.VerifySourceAtDestination(installation, policyBaseCommit); sourceErr != nil {
			return "", "", false, fmt.Errorf("TEST_POLICY_ENGINE_REQUIRED: retained destination engine does not bind the captured policy base: %w", sourceErr)
		}
		if prepareErr := pinned.PrepareForExecution(); prepareErr != nil {
			return "", "", false, fmt.Errorf("TEST_POLICY_ENGINE_REQUIRED: retain destination engine descriptor: %w", prepareErr)
		}
		engine = steward.EnrolledExecutionPath(installation, identity)
	}
	engineInfo, err := os.Stat(engine)
	if err != nil || !engineInfo.Mode().IsRegular() || engineInfo.Mode().Perm()&0o111 == 0 {
		return "", "", false, fmt.Errorf("TEST_POLICY_ENGINE_REQUIRED: retained immutable trusted-base engine is unavailable at %s", engine)
	}
	digest, err := fileSHA256(engine)
	if err != nil {
		return "", "", false, fmt.Errorf("TEST_POLICY_ENGINE_REQUIRED: hash retained trusted-base engine: %w", err)
	}
	currentInfo, currentErr := os.Stat(current)
	return engine, digest, currentErr == nil && os.SameFile(engineInfo, currentInfo), nil
}

func planWithTrustedPolicyEngine(engine string, request testingSelectionRequest, installation, candidateTree string) (testingPlanOutput, error) {
	args := []string{"test", "plan", "--root", installation, "--tree", candidateTree, "--mode", string(request.Mode), "--purpose", string(request.Purpose), "--json"}
	if request.GoalID != "" {
		args = append(args, "--goal", request.GoalID)
	}
	if len(request.Groups) > 0 {
		args = append(args, "--groups", strings.Join(request.Groups, ","))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, engine, args...)
	command.Env = testingEnvironment(os.Environ())
	data, err := command.CombinedOutput()
	if err != nil {
		return testingPlanOutput{}, fmt.Errorf("TEST_POLICY_ENGINE_REQUIRED: retained trusted-base engine could not decide version-1 policy: %v: %s", err, strings.TrimSpace(string(data)))
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	var output testingPlanOutput
	if err := decoder.Decode(&output); err != nil {
		return testingPlanOutput{}, fmt.Errorf("TEST_POLICY_ENGINE_REQUIRED: retained trusted-base engine returned malformed policy output: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return testingPlanOutput{}, fmt.Errorf("TEST_POLICY_ENGINE_REQUIRED: retained trusted-base engine returned trailing policy output")
	}
	return output, nil
}

func testingRelevantInputs(prefix, contractRel string, contract testpolicy.Contract, plan testpolicy.Plan) ([]string, error) {
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
		if group.Adapter == "section" {
			values[joinPrefix("scripts/agents/validate-section-selector.sh")] = true
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

func planOutput(prepared testingPreparation) testingPlanOutput {
	groupsByID := map[string]testpolicy.Group{}
	for _, group := range prepared.EffectiveContract.Groups {
		groupsByID[group.ID] = group
	}
	groups := make([]testpolicy.Group, 0, len(prepared.Plan.SelectedGroups))
	for _, id := range prepared.Plan.SelectedGroups {
		groups = append(groups, groupsByID[id])
	}
	return testingPlanOutput{SchemaVersion: 1, ProjectRoot: prepared.ProjectRoot, InstallationPrefix: prepared.Prefix,
		BaseCommit: prepared.BaseCommit, PolicyBaseCommit: prepared.PolicyBaseCommit, CandidateTree: prepared.CandidateTree, ContractDigest: prepared.ContractDigest,
		BaseContractDigest: prepared.BaseContractDigest, Plan: prepared.Plan, Groups: groups}
}

func testingRunRequest(prepared testingPreparation, attemptID, logRoot, candidateEngine, candidateEngineDigest string) proofrun.TestRunRequest {
	return proofrun.TestRunRequest{ProjectRoot: prepared.ProjectRoot, InstallationPrefix: prepared.Prefix,
		ControlRoot:   prepared.Installation,
		CandidateTree: prepared.CandidateTree, BaseCommit: prepared.BaseCommit, PolicyBaseCommit: prepared.PolicyBaseCommit,
		Contract: prepared.EffectiveContract, Plan: prepared.Plan, AttemptID: attemptID, Environment: prepared.Environment,
		LogRoot: logRoot, ContractDigest: prepared.ContractDigest, BaseContractDigest: prepared.BaseContractDigest,
		PolicyEngineDigest: prepared.PolicyEngineDigest, PolicyEngine: prepared.PolicyEngine, BehaviorPolicyDigest: prepared.BehaviorPolicyDigest,
		CandidateEngine: candidateEngine, CandidateEngineDigest: candidateEngineDigest}
}

func runTestRun(args []string) int {
	commandStarted := time.Now().UTC()
	request, _, status := parseTestingSelection("test run", args, true)
	if status != 0 {
		return status
	}
	prepared, err := prepareTesting(request)
	if err != nil {
		fmt.Fprintln(os.Stderr, "metasystem test run:", err)
		return 1
	}
	limits, err := resolveProofRunLimits(prepared.ConfPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "metasystem test run:", err)
		return 1
	}
	engine, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "metasystem test run:", err)
		return 1
	}
	engineDigest, err := fileSHA256(engine)
	if err != nil {
		fmt.Fprintln(os.Stderr, "metasystem test run:", err)
		return 1
	}
	planDigest := proofrun.TestPlanDigest(prepared.EffectiveContract, prepared.Plan, prepared.CandidateTree)
	manifestDigest, err := testingCandidateManifest(gittree.Workspace{Dir: prepared.ProjectRoot}, prepared.CandidateTree)
	if err != nil {
		fmt.Fprintln(os.Stderr, "metasystem test run: capture candidate manifest:", err)
		return proofrun.ExitAdmissionRefused
	}
	preRequest := testingRunRequest(prepared, "", "", engine, engineDigest)
	metadataStarted := time.Now()
	metadataContext, cancelMetadata := context.WithTimeout(context.Background(), limits.sectionCap)
	identities, preparedGroups, preparationLaunches, identityErr := proofrun.PrepareGroupExecutionIdentities(metadataContext, preRequest)
	cancelMetadata()
	preparationDuration := time.Since(metadataStarted).Milliseconds()
	preRequest.PreparedGroups, preRequest.PreparationLaunches = preparedGroups, preparationLaunches
	preRequest.PreparationDurationMS = preparationDuration
	preRequest.CommandStartedAt = commandStarted.Format(time.RFC3339Nano)
	var identityInputs []string
	if identityErr == nil {
		for _, id := range prepared.Plan.SelectedGroups {
			identityInputs = append(identityInputs, "group:"+id+":"+identities[id])
		}
	} else {
		identityInputs = append(identityInputs, "group-identity-unavailable:"+identityErr.Error())
	}
	if identityErr != nil {
		fmt.Fprintln(os.Stderr, "metasystem test run: bounded testing metadata preparation:", identityErr)
		return proofrun.ExitAdmissionRefused
	}
	attempt, decision, joined, err := admitProofLaunch(proofLaunchAdmission{ControlRoot: prepared.Installation,
		ExecutionRoot: prepared.ProjectRoot, ConfPath: prepared.ConfPath, GoalID: request.GoalID, RetryDecision: request.RetryDecision,
		CapMin:     request.CapMin,
		ScopeClass: "selected", CommandClass: "testing", IdentityInputs: append([]string{prepared.CandidateTree, prepared.ContractDigest,
			prepared.BaseContractDigest, prepared.PolicyEngineDigest, prepared.BehaviorPolicyDigest, planDigest}, identityInputs...), Environment: prepared.Environment,
		SharedEngine: engine, SharedManifestDigest: manifestDigest, ComponentIdentities: identities})
	if err != nil {
		fmt.Fprintln(os.Stderr, "metasystem test run:", err)
		return proofrun.ExitAdmissionRefused
	}
	if decision.Disposition != proofrun.DispositionExecuted {
		if decision.Disposition == proofrun.DispositionReusableSuccess {
			attempts, readErr := proofrun.ReadAttempts(prepared.Installation)
			if readErr != nil || identityErr != nil {
				fmt.Fprintln(os.Stderr, "metasystem test run: reusable component evidence is unreadable")
				return 1
			}
			template := proofrun.NewTestResult(preRequest)
			projection, exact := proofrun.ExactReusableTestResult(template, attempts, identities, prepared.GoalID, prepared.AccountingRevision)
			if !exact {
				projection = proofrun.ReusedTestResult(template, attempts, identities, prepared.EffectiveContract, prepared.GoalID, prepared.AccountingRevision)
			}
			if !projection.Delivery.Sufficient {
				fmt.Fprintln(os.Stderr, "metasystem test run: retained component evidence is incomplete")
				return 1
			}
			if err := publishTestingResult(request.ResultPath, projection); err != nil {
				fmt.Fprintln(os.Stderr, "metasystem test run:", err)
				return 1
			}
			return decision.ExitStatus
		}
		if err := proofrun.EncodeResult(os.Stdout, request.ResultPath, decision); err != nil {
			fmt.Fprintln(os.Stderr, "metasystem test run: publish no-child result:", err)
			return 1
		}
		return decision.ExitStatus
	}
	prepared.GoalID, prepared.AccountingRevision = attempt.GoalID, attempt.AccountingRevision
	preRequest = testingRunRequest(prepared, "", "", engine, engineDigest)
	preRequest.PreparedGroups, preRequest.PreparationLaunches = preparedGroups, preparationLaunches
	preRequest.PreparationDurationMS, preRequest.CommandStartedAt = preparationDuration, commandStarted.Format(time.RFC3339Nano)
	reusedGroups := map[string]proofrun.GroupResult{}
	if identityErr == nil {
		attempts, readErr := proofrun.ReadAttempts(prepared.Installation)
		if readErr != nil {
			fmt.Fprintln(os.Stderr, "metasystem test run: read reusable component evidence:", readErr)
			return retainIncompleteProofAttempt(prepared.Installation, attempt.AttemptID, joined, 1)
		}
		reused := proofrun.ReusedTestResultExcluding(proofrun.NewTestResult(preRequest), attempts, identities,
			prepared.EffectiveContract, prepared.GoalID, prepared.AccountingRevision, attempt.AttemptID)
		for _, group := range reused.Groups {
			if group.Status == "reused" {
				reusedGroups[group.ID] = group
			}
		}
	}
	pathsRoot := filepath.Join(prepared.Installation, "artifacts", "agents", "proof-runs", attempt.AttemptID, "testing", planDigest)
	if err := os.MkdirAll(pathsRoot, 0o700); err != nil {
		fmt.Fprintln(os.Stderr, "metasystem test run:", err)
		return retainIncompleteProofAttempt(prepared.Installation, attempt.AttemptID, joined, 1)
	}
	runRequest := testingRunRequest(prepared, attempt.AttemptID, filepath.Join(pathsRoot, "groups"), engine, engineDigest)
	runRequest.ProgressPath = filepath.Join(pathsRoot, "progress.jsonl")
	runRequest.Reused, runRequest.ComponentIdentities = reusedGroups, identities
	runRequest.PreparedGroups, runRequest.PreparationLaunches = preparedGroups, preparationLaunches
	runRequest.PreparationDurationMS, runRequest.CommandStartedAt = preparationDuration, preRequest.CommandStartedAt
	runRequest.EvidenceTimeoutMS, runRequest.EvidenceMaxBytes = limits.evidenceTimeout.Milliseconds(), limits.evidenceMax
	packetPath, workerResultPath := filepath.Join(pathsRoot, "request.json"), filepath.Join(pathsRoot, "result.json")
	if err := writePrivateJSON(packetPath, runRequest); err != nil {
		fmt.Fprintln(os.Stderr, "metasystem test run:", err)
		return retainIncompleteProofAttempt(prepared.Installation, attempt.AttemptID, joined, 1)
	}
	packetDigest, err := fileSHA256(packetPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "metasystem test run:", err)
		return retainIncompleteProofAttempt(prepared.Installation, attempt.AttemptID, joined, 1)
	}
	deadline, _ := time.Parse(time.RFC3339Nano, attempt.Deadline)
	var retained *proofrun.TestResult
	workerEngine := engine
	if !prepared.FirstTestingTransition {
		workerEngine = prepared.PolicyEngine
	}
	launchStatus := proofrun.LaunchSuite(proofrun.LaunchOptions{Suite: "testing", Root: prepared.ProjectRoot,
		ControlRoot: prepared.Installation, AttemptID: attempt.AttemptID, JoinedAttempt: joined, Deadline: deadline, ConfPath: prepared.ConfPath,
		ProgressPath: runRequest.ProgressPath, LogPath: filepath.Join(pathsRoot, "launcher.log"),
		Banner: "TESTING-CONTRACT plan=" + planDigest, Silence: limits.silence, SectionCap: limits.sectionCap,
		EvidenceTimeout: limits.evidenceTimeout, EvidenceMax: limits.evidenceMax, Poll: time.Second, TermGrace: 5 * time.Second,
		KillGrace: time.Second, Command: []string{workerEngine, "test", "worker", "--packet", packetPath, "--packet-sha256", packetDigest, "--result", workerResultPath},
		Environment: prepared.Environment, Output: os.Stdout, ErrorOutput: os.Stderr,
		PrepareSuccess: func(completion proofrun.CompletionContext) (json.RawMessage, error) {
			result, readErr := readTestingWorkerResult(workerResultPath)
			if readErr != nil {
				return nil, readErr
			}
			retained = &result
			if joined || !result.Delivery.Sufficient {
				return nil, nil
			}
			receipt, payload, prepareErr := landing.PrepareTestingReceiptPayload(prepared.Installation,
				prepared.CandidateTree, result, completion.CompletedAt)
			if prepareErr == nil && receipt.Testing != nil {
				updated := *receipt.Testing
				retained = &updated
			}
			return payload, prepareErr
		},
		CommitTerminal: func(completion proofrun.CompletionContext, receipt json.RawMessage) error {
			if retained == nil {
				result, readErr := readTestingWorkerResult(workerResultPath)
				if readErr != nil {
					return readErr
				}
				retained = &result
			}
			return commitProofTerminalWithTestResult(completion, receipt, retained)
		}})
	launchStatus = retainIncompleteProofAttempt(prepared.Installation, attempt.AttemptID, joined, launchStatus)
	if retained == nil {
		if result, readErr := readTestingWorkerResult(workerResultPath); readErr == nil {
			retained = &result
		}
	}
	if retained != nil {
		if joined {
			if _, err := proofrun.RecordTestResult(prepared.Installation, attempt.AttemptID, *retained); err != nil {
				fmt.Fprintln(os.Stderr, "metasystem test run: retain joined result:", err)
				return 1
			}
		}
		if err := publishTestingResult(request.ResultPath, *retained); err != nil {
			fmt.Fprintln(os.Stderr, "metasystem test run:", err)
			return 1
		}
	}
	return launchStatus
}

func testingCandidateManifest(workspace gittree.Workspace, candidateTree string) (string, error) {
	detached, err := workspace.NewDetachedWorktree(candidateTree)
	if err != nil {
		return "", err
	}
	digest, digestErr := proofrun.FullDigest(detached.Workspace().Dir)
	closeErr := detached.Close()
	if digestErr != nil {
		return "", digestErr
	}
	if closeErr != nil {
		return "", closeErr
	}
	return digest, nil
}

func runTestWorker(args []string) int {
	flags := flag.NewFlagSet("test worker", flag.ContinueOnError)
	packet := flags.String("packet", "", "private testing request")
	packetDigest := flags.String("packet-sha256", "", "SHA-256 identity of the immutable testing request")
	resultPath := flags.String("result", "", "private testing result")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *packet == "" || *packetDigest == "" || *resultPath == "" {
		return 2
	}
	actualPacketDigest, err := fileSHA256(*packet)
	if err != nil || actualPacketDigest != *packetDigest {
		fmt.Fprintln(os.Stderr, "metasystem test worker: immutable request identity mismatch")
		return 3
	}
	var request proofrun.TestRunRequest
	if err := readStrictJSON(*packet, &request); err != nil {
		fmt.Fprintln(os.Stderr, "metasystem test worker:", err)
		return 2
	}
	if request.CandidateEngine == "" || request.CandidateEngineDigest == "" {
		fmt.Fprintln(os.Stderr, "metasystem test worker: input-bound candidate engine is absent")
		return 3
	}
	policyDigest, policyDigestErr := fileSHA256(request.PolicyEngine)
	if request.PolicyEngine == "" || policyDigestErr != nil || policyDigest != request.PolicyEngineDigest {
		fmt.Fprintln(os.Stderr, "metasystem test worker: input-bound policy engine changed")
		return 3
	}
	engineInfo, statErr := os.Stat(request.CandidateEngine)
	engineDigest, digestErr := fileSHA256(request.CandidateEngine)
	if statErr != nil || !engineInfo.Mode().IsRegular() || engineInfo.Mode()&0o111 == 0 || digestErr != nil || engineDigest != request.CandidateEngineDigest {
		fmt.Fprintln(os.Stderr, "metasystem test worker: input-bound candidate engine changed")
		return 3
	}
	controlRoot, attemptID := os.Getenv("METASYSTEM_PROOF_CONTROL_ROOT"), os.Getenv("METASYSTEM_PROOF_ATTEMPT")
	canonicalControl, err := canonicalProofRoot(controlRoot)
	if err != nil || canonicalControl == "" || attemptID == "" || attemptID != request.AttemptID {
		fmt.Fprintln(os.Stderr, "metasystem test worker: attempt-bound proof locator mismatch")
		return 3
	}
	if err := proofrun.AuthenticateWorker(canonicalControl, attemptID, os.Getenv("METASYSTEM_PROOF_RECORD_KEY"),
		os.Getenv("METASYSTEM_PROOF_CREATION_CLAIM"), int64(os.Getppid())); err != nil {
		fmt.Fprintln(os.Stderr, "metasystem test worker:", err)
		return 3
	}
	attempt, err := proofrun.ReadAttempt(canonicalControl, attemptID)
	if err != nil {
		fmt.Fprintln(os.Stderr, "metasystem test worker:", err)
		return 3
	}
	deadline, err := time.Parse(time.RFC3339Nano, attempt.Deadline)
	if err != nil || !time.Now().UTC().Before(deadline) {
		fmt.Fprintln(os.Stderr, "metasystem test worker: admitted deadline is invalid or expired")
		return 3
	}
	request.Environment = inheritedTestingEnvironment(request.Environment, os.Environ())
	workerContext, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	if err := runFrozenPolicyProtectionCorpus(workerContext, request); err != nil {
		fmt.Fprintln(os.Stderr, "metasystem test worker:", err)
		return 1
	}
	result, status, err := proofrun.RunTestPlan(workerContext, request)
	if err != nil {
		fmt.Fprintln(os.Stderr, "metasystem test worker:", err)
		return 1
	}
	if err := writePrivateJSON(*resultPath, result); err != nil {
		fmt.Fprintln(os.Stderr, "metasystem test worker:", err)
		return 1
	}
	printTestingSummary(result)
	return status
}

func runTestVerify(args []string) int {
	request, jsonOutput, status := parseTestingSelection("test verify", args, false)
	if status != 0 {
		return status
	}
	if request.Tree == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem test verify --root INSTALLATION [--goal ID] --tree TREE [--json]")
		return 2
	}
	prepared, err := prepareTesting(request)
	if err != nil {
		fmt.Fprintln(os.Stderr, "metasystem test verify:", err)
		return 1
	}
	engine, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "metasystem test verify:", err)
		return 1
	}
	engineDigest, err := fileSHA256(engine)
	if err != nil {
		fmt.Fprintln(os.Stderr, "metasystem test verify:", err)
		return 1
	}
	runRequest := testingRunRequest(prepared, "", "", engine, engineDigest)
	limits, err := resolveProofRunLimits(prepared.ConfPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "metasystem test verify:", err)
		return 1
	}
	attempts, err := proofrun.ReadAttempts(prepared.Installation)
	if err != nil {
		fmt.Fprintln(os.Stderr, "metasystem test verify:", err)
		return 1
	}
	metadataContext, cancelMetadata := context.WithTimeout(context.Background(), limits.sectionCap)
	identities, err := proofrun.RevalidateRetainedGroupExecutionIdentities(metadataContext, runRequest, attempts)
	cancelMetadata()
	if err != nil {
		fmt.Fprintln(os.Stderr, "metasystem test verify:", err)
		return 1
	}
	result := proofrun.ReusedTestResult(proofrun.NewTestResult(runRequest), attempts, identities, prepared.EffectiveContract, prepared.GoalID, prepared.AccountingRevision)
	if jsonOutput {
		printJSON(result)
	} else {
		printTestingSummary(result)
	}
	if !result.Delivery.Sufficient {
		fmt.Fprintf(os.Stderr, "missing required proof; run metasystem test run --root %s --goal %s --tree %s --mode auto; missing groups: %s\n",
			prepared.Installation, request.GoalID, prepared.CandidateTree, strings.Join(result.Delivery.MissingGroups, ","))
		return 1
	}
	return 0
}

func loadPhysicalTestingContract(root string) (string, testpolicy.Contract, string, error) {
	installation, err := canonicalProofRoot(root)
	if err != nil {
		return "", testpolicy.Contract{}, "", err
	}
	confPath := filepath.Join(installation, "metasystem.conf")
	contractRel, found, err := config.ConfLookup(confPath, "testing.contract")
	if err != nil || !found {
		return "", testpolicy.Contract{}, "", fmt.Errorf("testing.contract is required in committed metasystem.conf")
	}
	if filepath.IsAbs(contractRel) || filepath.ToSlash(filepath.Clean(contractRel)) != contractRel || strings.HasPrefix(contractRel, "../") {
		return "", testpolicy.Contract{}, "", fmt.Errorf("testing.contract must be a relative normalized path")
	}
	path := filepath.Join(installation, filepath.FromSlash(contractRel))
	contract, err := testpolicy.Load(path)
	return installation, contract, path, err
}

func checkTestingTools(projectRoot string, contract testpolicy.Contract) error {
	var unavailable []string
	for _, group := range contract.Groups {
		cwd := filepath.Join(projectRoot, filepath.FromSlash(group.CWD))
		if info, err := os.Stat(cwd); err != nil || !info.IsDir() {
			unavailable = append(unavailable, group.ID+":cwd")
			continue
		}
		environment := proofrun.TestingEnvironment(testingEnvironment(os.Environ()), group.Env)
		for _, tool := range group.Tools {
			if _, err := proofrun.ResolveTestingExecutable(context.Background(), cwd, environment, []string{tool.Executable}); err != nil {
				unavailable = append(unavailable, group.ID+":"+tool.ID)
			}
		}
	}
	if len(unavailable) > 0 {
		sort.Strings(unavailable)
		return fmt.Errorf("declared testing tools or directories unavailable: %s", strings.Join(unavailable, ","))
	}
	return nil
}

func testingGoalRisk(root, id string) (testpolicy.GoalRisk, uint64, error) {
	if id == "" {
		return testpolicy.GoalRisk{}, 0, nil
	}
	endpoint, err := goal.ResolveEndpoint(root)
	if err != nil {
		return testpolicy.GoalRisk{}, 0, err
	}
	projection, err := goal.Project(endpoint, false, time.Now().UTC())
	if err != nil {
		return testpolicy.GoalRisk{}, 0, err
	}
	file := projection.Tree.Live[id]
	if file == nil || file.Risk == nil || file.Claimed == nil {
		return testpolicy.GoalRisk{}, 0, fmt.Errorf("goal %s has no accepted risk and accounting lineage", id)
	}
	accountingRevision := file.Claimed.AccountingRevision
	if accountingRevision == 0 {
		return testpolicy.GoalRisk{}, 0, fmt.Errorf("goal %s has no accepted accounting revision", id)
	}
	return testpolicy.GoalRisk{Severity: int(file.Risk.Severity), Novelty: int(file.Risk.Novelty),
		Exposure: int(file.Risk.Exposure), Accumulation: int(file.Risk.Accumulation)}, accountingRevision, nil
}

func resolveTestingGoal(root, requested string) (string, error) {
	if requested != "" {
		return requested, nil
	}
	parentRoot, parentAttempt := os.Getenv("METASYSTEM_PROOF_CONTROL_ROOT"), os.Getenv("METASYSTEM_PROOF_ATTEMPT")
	if parentRoot != "" || parentAttempt != "" {
		if parentRoot == "" || parentAttempt == "" {
			return "", fmt.Errorf("proof parent locator is incomplete")
		}
		canonicalParent, err := canonicalProofRoot(parentRoot)
		if err != nil || canonicalParent != root {
			return "", fmt.Errorf("proof parent control root does not match testing root")
		}
		attempt, err := proofrun.AuthenticateContext(root, parentAttempt, int64(os.Getppid()))
		if err != nil {
			return "", err
		}
		return attempt.GoalID, nil
	}
	return uniqueActiveProofGoal(root, time.Now().UTC())
}

func trustedTestingPolicyBase(projectRoot string, workspace gittree.Workspace) (string, error) {
	command := exec.Command("git", "-C", projectRoot, "config", "--local", "--no-includes", "--get", "metasystem.steward.landing-ref")
	command.Env = gittree.ScrubbedEnviron()
	data, err := command.Output()
	ref := strings.TrimSpace(string(data))
	tail := strings.TrimPrefix(ref, "refs/remotes/")
	remote, branch, qualified := strings.Cut(tail, "/")
	if err != nil || tail == ref || !qualified || remote == "" || branch == "" {
		return "", fmt.Errorf("trusted testing policy base requires local metasystem.steward.landing-ref shaped refs/remotes/<remote>/<branch>")
	}
	commit, err := workspace.ResolveCommit(ref)
	if err != nil {
		return "", fmt.Errorf("resolve trusted testing policy base %s: %w", ref, err)
	}
	return commit, nil
}

func testingEnvironment(environment []string) []string {
	allowed := map[string]bool{"PATH": true, "HOME": true, "TMPDIR": true, "TMP": true, "TEMP": true,
		"GOCACHE": true, "GOMODCACHE": true, "GOPATH": true, "GOROOT": true, "GOFLAGS": true, "GOWORK": true,
		"CGO_ENABLED": true, "GOTOOLCHAIN": true, "GOEXPERIMENT": true, "JAVA_HOME": true, "LANG": true,
		"LC_ALL": true, "SYSTEMROOT": true, "TZ": true}
	var result []string
	for _, entry := range environment {
		name, _, ok := strings.Cut(entry, "=")
		if ok && allowed[name] {
			result = append(result, entry)
		}
	}
	sort.Strings(result)
	return result
}

func inheritedTestingEnvironment(prepared, inherited []string) []string {
	allowed := map[string]bool{"METASYSTEM_PROOF_CONTROL_ROOT": true, "METASYSTEM_PROOF_ATTEMPT": true,
		"METASYSTEM_PROOF_RECORD_KEY": true, "METASYSTEM_PROOF_CREATION_CLAIM": true, "METASYSTEM_PROOF_AUTH_BIN": true}
	values := make(map[string]string, len(prepared)+len(allowed))
	for _, entry := range prepared {
		name, value, ok := strings.Cut(entry, "=")
		if ok {
			values[name] = value
		}
	}
	for _, entry := range inherited {
		name, value, ok := strings.Cut(entry, "=")
		if ok && allowed[name] {
			values[name] = value
		}
	}
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	result := make([]string, 0, len(names))
	for _, name := range names {
		result = append(result, name+"="+values[name])
	}
	return result
}

func readTestingWorkerResult(path string) (proofrun.TestResult, error) {
	var result proofrun.TestResult
	if err := readStrictJSON(path, &result); err != nil {
		return proofrun.TestResult{}, err
	}
	if err := proofrun.ValidateTestResult(result); err != nil {
		return proofrun.TestResult{}, err
	}
	return result, nil
}

func readStrictJSON(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("trailing JSON in %s", path)
	}
	return nil
}

func writePrivateJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return atomicfile.WriteVolatile(path, string(data)+"\n")
}

func publishTestingResult(path string, result proofrun.TestResult) error {
	if path != "" {
		return writeIdentityJSON(path, result)
	}
	printJSON(result)
	return nil
}

func printTestingSummary(result proofrun.TestResult) {
	fmt.Printf("TEST-RESULT sufficient=%t tree=%s selected=%s\n", result.Delivery.Sufficient, result.CandidateTree, strings.Join(result.SelectedGroups, ","))
	for _, group := range result.Groups {
		if group.Status != "passed" && group.Status != "reused" {
			fmt.Printf("TEST-GROUP %s status=%s reason=%s log=%s\n", group.ID, group.Status, group.NotRunReason, group.LogPath)
		}
	}
}

func bytesSHA256(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func fileSHA256(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return bytesSHA256(data), nil
}
