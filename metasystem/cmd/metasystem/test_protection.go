package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

const policyProbeWorkerEnvironment = "METASYSTEM_POLICY_PROBE_WORKER"

// runFrozenPolicyProtectionCorpus is called by the authenticated test worker.
// On ordinary landings that worker is the retained destination engine, so the
// candidate executable cannot delete or relink these literal public-v1 cases.
func runFrozenPolicyProtectionCorpus(ctx context.Context, request proofrun.TestRunRequest) error {
	if os.Getenv(policyProbeWorkerEnvironment) == "1" || !containsString(request.Plan.SelectedGroups, "policy-protection") {
		return nil
	}
	for _, probe := range testpolicy.FrozenProtectionProbeCases() {
		var err error
		switch probe.ID {
		case "remove-required-provider", "shrink-dependency-graph", "lower-coverage-floor", "remove-required-test":
			err = runFrozenSelectionProbe(ctx, request, probe)
		case "emit-zero-tests", "forge-component-reuse":
			err = runFrozenWorkerProbe(ctx, request, probe)
		default:
			err = fmt.Errorf("unknown frozen probe")
		}
		if err != nil {
			return fmt.Errorf("protected public-v1 probe %s failed: %w", probe.ID, err)
		}
	}
	return nil
}

func runFrozenSelectionProbe(ctx context.Context, request proofrun.TestRunRequest, probe testpolicy.ProtectionProbeCase) error {
	workspace := gittree.Workspace{Dir: request.ProjectRoot}
	baseTree, err := workspace.TreeOf(request.PolicyBaseCommit)
	if err != nil {
		return err
	}
	contractRel, present, err := config.ConfLookup(filepath.Join(request.ControlRoot, "metasystem.conf"), "testing.contract")
	if err != nil || !present {
		return fmt.Errorf("read protected probe testing contract: %w", err)
	}
	treeContractPath := strings.TrimPrefix(filepath.ToSlash(filepath.Join(request.InstallationPrefix, contractRel)), "./")
	_, baseContractPresent, err := workspace.FileAt(baseTree, treeContractPath)
	if err != nil {
		return err
	}
	policyBaseCommit := request.PolicyBaseCommit
	var projectRoot string
	var cleanup func()
	if baseContractPresent {
		detached, detachErr := workspace.NewDetachedWorktree(baseTree)
		if detachErr != nil {
			return detachErr
		}
		projectRoot, err = canonicalPath(detached.Workspace().Dir)
		if err != nil {
			_ = detached.Close()
			return err
		}
		cleanup = func() { _ = detached.Close() }
		actualPolicyBase, policyErr := trustedTestingPolicyBase(projectRoot, gittree.Workspace{Dir: projectRoot})
		if policyErr != nil {
			cleanup()
			return fmt.Errorf("read protected probe destination without changing caller refs or configuration: %w", policyErr)
		}
		if actualPolicyBase != request.PolicyBaseCommit {
			cleanup()
			return fmt.Errorf("protected probe destination changed: got %s want %s", actualPolicyBase, request.PolicyBaseCommit)
		}
	} else {
		// A linked worktree would share the caller's refs and local config,
		// while the former freeze-and-init path invented a commit that could
		// not be the source stamp of the authenticated engine. Give the first
		// transition its own repository metadata but share the immutable object
		// database. The real candidate commit (or a dangling commit over the
		// exact candidate tree) and its engine-source ancestry therefore remain
		// resolvable without changing any caller ref or configuration byte.
		candidateCommit, commitErr := policyProbeCandidateCommit(ctx, request)
		if commitErr != nil {
			return commitErr
		}
		cloneParent, cloneErr := os.MkdirTemp("", "metasystem-policy-clone.")
		if cloneErr != nil {
			return cloneErr
		}
		projectRoot = filepath.Join(cloneParent, "source")
		cleanup = func() { _ = os.RemoveAll(cloneParent) }
		if _, cloneErr = runPolicyProbeGit(ctx, request.ProjectRoot, "clone", "-q", "--shared", "--no-checkout", request.ProjectRoot, projectRoot); cloneErr != nil {
			cleanup()
			return cloneErr
		}
		projectRoot, err = canonicalPath(projectRoot)
		if err != nil {
			cleanup()
			return err
		}
		if _, cloneErr = runPolicyProbeGit(ctx, projectRoot, "checkout", "-q", "--detach", candidateCommit); cloneErr != nil {
			cleanup()
			return cloneErr
		}
		policyBaseCommit = candidateCommit
		const isolatedPolicyRef = "refs/remotes/protection/base"
		if _, err := runPolicyProbeGit(ctx, projectRoot, "update-ref", isolatedPolicyRef, policyBaseCommit); err != nil {
			cleanup()
			return err
		}
		if _, err := runPolicyProbeGit(ctx, projectRoot, "config", "--local", "metasystem.steward.landing-ref", isolatedPolicyRef); err != nil {
			cleanup()
			return err
		}
	}
	defer cleanup()
	installation := projectRoot
	if request.InstallationPrefix != "" {
		installation = filepath.Join(projectRoot, filepath.FromSlash(request.InstallationPrefix))
	}
	_, base, contractPath, err := loadPhysicalTestingContract(installation)
	if err != nil {
		return err
	}
	baseBytes, err := json.Marshal(base)
	if err != nil {
		return err
	}
	var candidate testpolicy.Contract
	if err := json.Unmarshal(baseBytes, &candidate); err != nil {
		return err
	}
	var baseProtected testpolicy.Group
	for _, group := range base.Groups {
		if group.ID == "policy-protection" {
			baseProtected = group
			break
		}
	}
	switch probe.ID {
	case "remove-required-provider":
		candidate.Groups = removeTestingGroup(candidate.Groups, "policy-protection")
		removeTestingGroupReferences(&candidate, "policy-protection")
	case "shrink-dependency-graph":
		for index := range candidate.Surfaces {
			if candidate.Surfaces[index].ID == "proof-and-landing" {
				candidate.Surfaces[index].DependsOn = nil
			}
		}
	case "lower-coverage-floor":
	case "remove-required-test":
		for index := range candidate.Groups {
			if candidate.Groups[index].ID == "policy-protection" {
				if candidate.Groups[index].Adapter == "go" {
					candidate.Groups[index].Tests = json.RawMessage(`["TestCandidateReplacement"]`)
				} else {
					report := candidate.Groups[index].ExpectedTests[0].Report
					candidate.Groups[index].ExpectedTests = []testpolicy.ExpectedTest{{Report: report, Classname: "candidate", Name: "replacement"}}
				}
			}
		}
	}
	encoded, err := json.Marshal(candidate)
	if err != nil {
		return err
	}
	if err := os.WriteFile(contractPath, append(encoded, '\n'), 0o644); err != nil {
		return err
	}
	changedPaths := []string{contractPath}
	if probe.ID == "lower-coverage-floor" {
		ratchetPath := filepath.Join(installation, "scripts", "agents", "coverage-ratchet.json")
		data, readErr := os.ReadFile(ratchetPath)
		var baseline protectedCoverageBaseline
		if readErr != nil || json.Unmarshal(data, &baseline) != nil || len(baseline.Floors) == 0 {
			return fmt.Errorf("read frozen coverage baseline: %v", readErr)
		}
		for packageName, floor := range baseline.Floors {
			baseline.Floors[packageName] = floor - 0.1
			break
		}
		data, _ = json.Marshal(baseline)
		if err := os.WriteFile(ratchetPath, append(data, '\n'), 0o644); err != nil {
			return err
		}
		changedPaths = append(changedPaths, ratchetPath)
	}
	relativeContract, err := filepath.Rel(projectRoot, contractPath)
	if err != nil {
		return err
	}
	relativeChanges := []string{relativeContract}
	for _, path := range changedPaths[1:] {
		relative, relativeErr := filepath.Rel(projectRoot, path)
		if relativeErr != nil {
			return relativeErr
		}
		relativeChanges = append(relativeChanges, relative)
	}
	if _, err := runPolicyProbeGit(ctx, projectRoot, append([]string{"add", "--"}, relativeChanges...)...); err != nil {
		return err
	}
	candidateTree, err := (gittree.Workspace{Dir: projectRoot}).StagedTree()
	if err != nil {
		return err
	}
	policyEngine, err := canonicalPath(request.PolicyEngine)
	if err != nil {
		return err
	}
	digest, err := fileSHA256(policyEngine)
	if err != nil {
		return err
	}
	canonicalInstallation, err := canonicalProofRoot(installation)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(steward.RepoIdentityPath(installation)), 0o700); err != nil {
		return err
	}
	if err := steward.MintIdentity(steward.RepoIdentityPath(installation), steward.InstallIdentity{RepoIdentity: canonicalInstallation,
		Generation: 1, InstallPath: policyEngine, InstallDigest: "sha256:" + digest, MintedAt: request.CommandStartedAt,
		Enrollment: steward.EnrollmentFixture, EngineBuild: policyBaseCommit, LandedCommit: policyBaseCommit}); err != nil {
		return err
	}
	command := exec.CommandContext(ctx, request.CandidateEngine, "test", "plan", "--root", installation, "--tree", candidateTree,
		"--mode", "auto", "--purpose", "diagnostic", "--json")
	command.Env = append(inheritedTestingEnvironment(request.Environment, os.Environ()), policyProbeWorkerEnvironment+"=1")
	data, commandErr := command.CombinedOutput()
	status := processExitStatus(commandErr)
	if status != probe.ExpectedStatus {
		return fmt.Errorf("status=%d want=%d output=%s", status, probe.ExpectedStatus, strings.TrimSpace(string(data)))
	}
	if probe.ID == "lower-coverage-floor" {
		if !strings.Contains(string(data), "TEST_POLICY_COVERAGE_FLOOR_LOWERED") {
			return fmt.Errorf("stable floor refusal missing: %s", data)
		}
		return nil
	}
	var output testingPlanOutput
	if err := json.Unmarshal(data, &output); err != nil {
		return fmt.Errorf("decode plan: %w: %s", err, data)
	}
	byID := map[string]testpolicy.Group{}
	for _, group := range output.Groups {
		byID[group.ID] = group
	}
	switch probe.ID {
	case "remove-required-provider":
		if _, ok := byID["policy-protection"]; !ok {
			return fmt.Errorf("base provider disappeared")
		}
	case "shrink-dependency-graph":
		if !containsString(output.Plan.AffectedSurfaces, "proof-and-landing") {
			return fmt.Errorf("base reverse consumer disappeared")
		}
	case "remove-required-test":
		protected := byID["policy-protection"]
		if baseProtected.Adapter == "go" {
			baseAll, baseNames, baseErr := testpolicy.GoTests(baseProtected)
			actualAll, actualNames, actualErr := testpolicy.GoTests(protected)
			if baseErr != nil || actualErr != nil || (baseAll && !actualAll) || (!baseAll && !containsEvery(actualNames, baseNames)) {
				return fmt.Errorf("base required Go test disappeared: base=%v actual=%v errors=%v/%v", baseNames, actualNames, baseErr, actualErr)
			}
		} else if !containsEveryExpectedTest(protected.ExpectedTests, baseProtected.ExpectedTests) {
			return fmt.Errorf("base required terminal test disappeared")
		}
	}
	return nil
}

func policyProbeCandidateCommit(ctx context.Context, request proofrun.TestRunRequest) (string, error) {
	head, err := runPolicyProbeGit(ctx, request.ProjectRoot, "rev-parse", "HEAD")
	if err == nil {
		headCommit := strings.TrimSpace(string(head))
		tree, treeErr := runPolicyProbeGit(ctx, request.ProjectRoot, "rev-parse", headCommit+"^{tree}")
		if treeErr == nil && strings.TrimSpace(string(tree)) == request.CandidateTree {
			return headCommit, nil
		}
	}
	data, err := runPolicyProbeGit(ctx, request.ProjectRoot,
		"-c", "user.name=MetaSystem", "-c", "user.email=metasystem@invalid",
		"commit-tree", request.CandidateTree, "-p", request.PolicyBaseCommit, "-m", "initial protected policy source")
	if err != nil {
		return "", fmt.Errorf("materialize exact initial protected policy tree: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

func containsEvery(values, required []string) bool {
	for _, wanted := range required {
		if !containsString(values, wanted) {
			return false
		}
	}
	return true
}

func containsEveryExpectedTest(values, required []testpolicy.ExpectedTest) bool {
	for _, wanted := range required {
		found := false
		for _, value := range values {
			found = found || value == wanted
		}
		if !found {
			return false
		}
	}
	return true
}

func runFrozenWorkerProbe(ctx context.Context, outer proofrun.TestRunRequest, probe testpolicy.ProtectionProbeCase) error {
	root, err := os.MkdirTemp("", "metasystem-policy-probe.")
	if err != nil {
		return err
	}
	defer os.RemoveAll(root)
	if _, err := runPolicyProbeGit(ctx, root, "init", "-q", "-b", "main"); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(root, "source.txt"), []byte("literal protected input\n"), 0o644); err != nil {
		return err
	}
	if _, err := runPolicyProbeGit(ctx, root, "add", "source.txt"); err != nil {
		return err
	}
	if _, err := runPolicyProbeGit(ctx, root, "-c", "user.name=MetaSystem", "-c", "user.email=metasystem@invalid", "commit", "-qm", "probe"); err != nil {
		return err
	}
	tree, err := (gittree.Workspace{Dir: root}).HeadTree()
	if err != nil {
		return err
	}
	zero := 0
	group := testpolicy.Group{ID: "literal", Kind: "unit", Adapter: "command", CWD: ".", Inputs: []string{"source.txt"}, Outputs: []string{"reports"},
		Obligations: []string{"literal-protection"}, Platforms: []string{"any"}, TargetMS: 1,
		Argv: []string{"sh", "-c", `mkdir -p reports; printf '%s\n' '<testsuite></testsuite>' >reports/result.xml`}, Reports: []string{"reports"}, Format: "junit-xml",
		ExpectedTests: []testpolicy.ExpectedTest{{Report: "reports/result.xml", Classname: "protection", Name: "required"}}}
	plan := testpolicy.Plan{Purpose: testpolicy.PurposeDelivery, RequestedMode: testpolicy.ModeStandard, RequiredMode: testpolicy.ModeStandard,
		ExecutedMode: testpolicy.ModeStandard, RequiredGroups: []string{"literal"}, SelectedGroups: []string{"literal"},
		Stages: []testpolicy.Stage{{ID: "standard", Groups: []string{"literal"}}}}
	request := proofrun.TestRunRequest{ControlRoot: outer.ControlRoot, ProjectRoot: root, CandidateTree: tree, BaseCommit: outer.BaseCommit,
		PolicyBaseCommit: outer.PolicyBaseCommit, Contract: testpolicy.Contract{SchemaVersion: 1, Groups: []testpolicy.Group{group}}, Plan: plan,
		AttemptID: outer.AttemptID, Environment: outer.Environment, LogRoot: filepath.Join(root, "logs"), PolicyEngine: outer.PolicyEngine,
		PolicyEngineDigest: outer.PolicyEngineDigest, CandidateEngine: outer.CandidateEngine, CandidateEngineDigest: outer.CandidateEngineDigest,
		CandidateEngineBuildIdentity: outer.CandidateEngineBuildIdentity,
		BehaviorPolicyDigest:         outer.BehaviorPolicyDigest, CommandStartedAt: outer.CommandStartedAt}
	if probe.ID == "forge-component-reuse" {
		request.Reused = map[string]proofrun.GroupResult{"literal": {ID: "literal", Kind: "unit", Obligations: []string{"literal-protection"},
			InputDigest: strings.Repeat("a", 64), InputManifest: []string{"source.txt"}, ExecutionIdentity: strings.Repeat("b", 64), CWD: ".",
			Status: "passed", NativeLaunched: true, NativeExitStatus: &zero, CollectionComplete: true, ReuseAttempt: "forged-success",
			ToolIdentities: map[string]string{}, ReportDigests: map[string]string{}}}
	}
	packet, resultPath := filepath.Join(root, "request.json"), filepath.Join(root, "result.json")
	if err := writePrivateJSON(packet, request); err != nil {
		return err
	}
	packetDigest, err := fileSHA256(packet)
	if err != nil {
		return err
	}
	command := exec.CommandContext(ctx, outer.CandidateEngine, "test", "worker", "--packet", packet, "--packet-sha256", packetDigest, "--result", resultPath)
	command.Env = append(inheritedTestingEnvironment(outer.Environment, os.Environ()), policyProbeWorkerEnvironment+"=1")
	data, commandErr := command.CombinedOutput()
	status := processExitStatus(commandErr)
	if status != probe.ExpectedStatus {
		return fmt.Errorf("status=%d want=%d output=%s", status, probe.ExpectedStatus, strings.TrimSpace(string(data)))
	}
	result, err := readTestingWorkerResult(resultPath)
	if err != nil || len(result.Groups) != 1 || result.Groups[0].CollectionComplete || result.Groups[0].Status != "invalid" {
		return fmt.Errorf("incomplete negative result=%+v err=%v output=%s", result, err, data)
	}
	if probe.ID == "forge-component-reuse" && result.Groups[0].ReuseAttempt != "" {
		return fmt.Errorf("forged reusable owner survived")
	}
	return nil
}

func removeTestingGroup(groups []testpolicy.Group, id string) []testpolicy.Group {
	result := make([]testpolicy.Group, 0, len(groups))
	for _, group := range groups {
		if group.ID != id {
			result = append(result, group)
		}
	}
	return result
}

func removeTestingGroupReferences(contract *testpolicy.Contract, id string) {
	contract.Always.Canary = removeString(contract.Always.Canary, id)
	contract.Always.Standard = removeString(contract.Always.Standard, id)
	contract.Unknown = removeString(contract.Unknown, id)
	contract.Cadence = removeString(contract.Cadence, id)
	for index := range contract.Surfaces {
		contract.Surfaces[index].Standard = removeString(contract.Surfaces[index].Standard, id)
		contract.Surfaces[index].Deep = removeString(contract.Surfaces[index].Deep, id)
	}
}

func removeString(values []string, unwanted string) []string {
	result := values[:0]
	for _, value := range values {
		if value != unwanted {
			result = append(result, value)
		}
	}
	return result
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func runPolicyProbeGit(ctx context.Context, root string, args ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, "git", append([]string{"-C", root}, args...)...)
	command.Env = gittree.ScrubbedEnviron()
	data, err := command.CombinedOutput()
	if err != nil {
		return data, fmt.Errorf("git %v: %w: %s", args, err, data)
	}
	return data, nil
}

func processExitStatus(err error) int {
	if err == nil {
		return 0
	}
	if exit, ok := err.(*exec.ExitError); ok {
		return exit.ExitCode()
	}
	return -1
}
