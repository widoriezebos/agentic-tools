package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestTrustedPolicyEngineIsRequiredWithoutBuildingDuringReadOnlySelection(t *testing.T) {
	root := t.TempDir()
	if _, _, _, err := trustedPolicyEngine(root, strings.Repeat("a", 40), false); err == nil || !strings.Contains(err.Error(), "TEST_POLICY_ENGINE_REQUIRED") {
		t.Fatalf("missing retained policy engine was accepted: %v", err)
	}
	if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 {
		t.Fatalf("read-only policy selection created build inputs: entries=%v err=%v", entries, err)
	}
}

func TestCandidateEngineIsBuiltFromCandidateTreeAndBindsExecutionIdentity(t *testing.T) {
	fixture := newCandidateEngineFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	built, err := buildCandidateEngine(ctx, gittree.Workspace{Dir: fixture.projectRoot}, "metasystem", fixture.candidateTree, testingEnvironment(os.Environ()))
	if err != nil {
		t.Fatalf("build candidate proof engine: %v", err)
	}
	t.Cleanup(func() { _ = built.Close() })
	repeated, err := buildCandidateEngine(ctx, gittree.Workspace{Dir: fixture.projectRoot}, "metasystem", fixture.candidateTree, testingEnvironment(os.Environ()))
	if err != nil {
		t.Fatalf("repeat candidate proof engine build: %v", err)
	}
	t.Cleanup(func() { _ = repeated.Close() })
	if repeated.Commit != built.Commit || repeated.Digest != built.Digest {
		t.Fatalf("same candidate tree produced unstable proof engine identity: first=%+v repeated=%+v", built, repeated)
	}
	writeTestingFixtureFile(t, filepath.Join(fixture.installationRoot, "records", "counselor", "peer.md"), []byte("ledger-only move\n"), 0o644)
	testingFixtureGit(t, fixture.projectRoot, "add", "metasystem/records/counselor/peer.md")
	recordsTree, err := (gittree.Workspace{Dir: fixture.projectRoot}).StagedTree()
	if err != nil {
		t.Fatal(err)
	}
	recordsBuild, err := buildCandidateEngine(ctx, gittree.Workspace{Dir: fixture.projectRoot}, "metasystem", recordsTree, testingEnvironment(os.Environ()))
	if err != nil {
		t.Fatalf("build candidate proof engine after records-only move: %v", err)
	}
	t.Cleanup(func() { _ = recordsBuild.Close() })
	if recordsBuild.Commit != built.Commit || recordsBuild.Digest != built.Digest {
		t.Fatalf("records-only move changed engine build identity or bytes: first=%+v records=%+v", built, recordsBuild)
	}
	writeTestingFixtureFile(t, filepath.Join(fixture.installationRoot, "go.sum"), []byte("fixture.example/module v1.0.0 h1:changed\n"), 0o644)
	testingFixtureGit(t, fixture.projectRoot, "add", "metasystem/go.sum")
	moduleTree, err := (gittree.Workspace{Dir: fixture.projectRoot}).StagedTree()
	if err != nil {
		t.Fatal(err)
	}
	moduleBuild, err := buildCandidateEngine(ctx, gittree.Workspace{Dir: fixture.projectRoot}, "metasystem", moduleTree, testingEnvironment(os.Environ()))
	if err != nil {
		t.Fatalf("build candidate proof engine after go.sum move: %v", err)
	}
	t.Cleanup(func() { _ = moduleBuild.Close() })
	if moduleBuild.Commit == built.Commit {
		t.Fatalf("go.sum move did not change engine build identity: first=%s changed=%s", built.Commit, moduleBuild.Commit)
	}
	testingFixtureGit(t, fixture.projectRoot, "config", "i18n.commitEncoding", "ISO-8859-1")
	testingFixtureGit(t, fixture.projectRoot, "config", "author.name", "repository author")
	t.Run("foreign Git identity and encoding", func(t *testing.T) {
		for name, value := range map[string]string{
			"GIT_AUTHOR_NAME": "foreign author", "GIT_AUTHOR_EMAIL": "foreign-author@example.invalid",
			"GIT_COMMITTER_NAME": "foreign committer", "GIT_COMMITTER_EMAIL": "foreign-committer@example.invalid",
			"GIT_AUTHOR_DATE": "2010-01-02T03:04:05Z", "GIT_COMMITTER_DATE": "2011-02-03T04:05:06Z",
		} {
			t.Setenv(name, value)
		}
		foreignEnvironment, err := buildCandidateEngine(ctx, gittree.Workspace{Dir: fixture.projectRoot}, "metasystem", fixture.candidateTree, testingEnvironment(os.Environ()))
		if err != nil {
			t.Fatalf("build identical tree with foreign Git identity and encoding: %v", err)
		}
		t.Cleanup(func() { _ = foreignEnvironment.Close() })
		if foreignEnvironment.Commit != built.Commit || foreignEnvironment.Digest != built.Digest {
			t.Fatalf("same tree depended on ambient Git identity or encoding: first=%+v foreign=%+v", built, foreignEnvironment)
		}
	})
	testingFixtureGit(t, fixture.projectRoot, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "land candidate tree")
	afterLanding, err := buildCandidateEngine(ctx, gittree.Workspace{Dir: fixture.projectRoot}, "metasystem", fixture.candidateTree, testingEnvironment(os.Environ()))
	if err != nil {
		t.Fatalf("build identical tree after its checkout history moved: %v", err)
	}
	t.Cleanup(func() { _ = afterLanding.Close() })
	if afterLanding.Commit != built.Commit || afterLanding.Digest != built.Digest {
		t.Fatalf("same tree depended on its checkout history: first=%+v after-landing=%+v", built, afterLanding)
	}
	actualDigest, err := fileSHA256(built.Path)
	if err != nil || actualDigest != built.Digest || actualDigest == fixture.policyDigest {
		t.Fatalf("candidate engine digest=%s policy=%s actual=%s err=%v", built.Digest, fixture.policyDigest, actualDigest, err)
	}
	data, err := os.ReadFile(built.Path)
	if err != nil || !strings.Contains(string(data), "candidate engine source") || !strings.Contains(string(data), built.Commit) {
		t.Fatalf("candidate engine does not carry candidate source and commit: commit=%s data=%q err=%v", built.Commit, data, err)
	}

	group := testpolicy.Group{ID: "candidate-bed", Kind: "integration", Adapter: "section", CWD: "metasystem",
		Inputs: []string{"metasystem/cmd/metasystem/engine.txt"}, Obligations: []string{"candidate-engine"},
		Platforms: []string{"any"}, TargetMS: 1, Section: "candidate-bed"}
	contract := testpolicy.Contract{SchemaVersion: 1, Groups: []testpolicy.Group{group}}
	plan := testpolicy.Plan{Purpose: testpolicy.PurposeDelivery, RequestedMode: testpolicy.ModeStandard,
		RequiredMode: testpolicy.ModeStandard, ExecutedMode: testpolicy.ModeStandard,
		RequiredGroups: []string{group.ID}, SelectedGroups: []string{group.ID}, Stages: []testpolicy.Stage{{ID: "standard", Groups: []string{group.ID}}}}
	prepared := testingPreparation{ProjectRoot: fixture.projectRoot, Prefix: "metasystem", CandidateTree: fixture.candidateTree,
		BaseCommit: fixture.baseCommit, PolicyBaseCommit: fixture.baseCommit, EffectiveContract: contract, Plan: plan,
		ContractDigest: strings.Repeat("1", 64), BaseContractDigest: strings.Repeat("2", 64),
		PolicyEngineDigest: fixture.policyDigest, BehaviorPolicyDigest: strings.Repeat("3", 64)}
	request := testingRunRequest(prepared, "", "", built.Path, built.Digest, built.Commit)
	result := proofrun.NewTestResult(request)
	if result.PolicyEngineDigest != fixture.policyDigest || result.CandidateEngineDigest != built.Digest ||
		result.CandidateEngineBuildIdentity != built.Commit || result.CandidateTree != fixture.candidateTree {
		t.Fatalf("retained execution identity lost policy, candidate engine, or tree: %+v", result)
	}
	identities, err := proofrun.GroupExecutionIdentities(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	changedPolicy := request
	changedPolicy.PolicyEngineDigest = strings.Repeat("4", 64)
	policyIdentities, err := proofrun.GroupExecutionIdentities(ctx, changedPolicy)
	if err != nil {
		t.Fatal(err)
	}
	changedCandidate := request
	changedCandidate.CandidateEngineDigest = strings.Repeat("5", 64)
	candidateIdentities, err := proofrun.GroupExecutionIdentities(ctx, changedCandidate)
	if err != nil {
		t.Fatal(err)
	}
	if identities[group.ID] == policyIdentities[group.ID] || identities[group.ID] == candidateIdentities[group.ID] {
		t.Fatalf("group execution identity omitted one engine digest: current=%s policy-change=%s candidate-change=%s", identities[group.ID], policyIdentities[group.ID], candidateIdentities[group.ID])
	}
}

func TestCandidateEngineTrimpathIsReproducibleAcrossMaterializationDirectories(t *testing.T) {
	script, err := os.ReadFile(filepath.Join("..", "..", "scripts", "agents", "go-build.sh"))
	if err != nil {
		t.Fatal(err)
	}
	cacheRoot := t.TempDir()
	stamp := strings.Repeat("a", 40)
	build := func(name string) string {
		root := filepath.Join(t.TempDir(), name)
		writeTestingFixtureFile(t, filepath.Join(root, "scripts", "agents", "go-build.sh"), script, 0o755)
		writeTestingFixtureFile(t, filepath.Join(root, "go.mod"), []byte("module github.com/widoriezebos/agentic-tools/metasystem\n\ngo 1.26\n"), 0o644)
		writeTestingFixtureFile(t, filepath.Join(root, "cmd", "metasystem", "main.go"), []byte("package main\nfunc main() {}\n"), 0o644)
		output := filepath.Join(t.TempDir(), "metasystem")
		command := exec.Command("bash", "scripts/agents/go-build.sh", "--trimpath", "--out", output)
		command.Dir = root
		command.Env = append(candidateEngineBuildEnvironment(testingEnvironment(os.Environ()), stamp),
			"GOCACHE="+filepath.Join(cacheRoot, "build"), "GOMODCACHE="+filepath.Join(cacheRoot, "modules"))
		if combined, buildErr := command.CombinedOutput(); buildErr != nil {
			t.Fatalf("build identical tree in %s: %v\n%s", root, buildErr, combined)
		}
		digest, digestErr := fileSHA256(output)
		if digestErr != nil {
			t.Fatal(digestErr)
		}
		return digest
	}
	first, second := build("first-materialization"), build("second-materialization")
	if first != second {
		t.Fatalf("one source tree built in two directories had different candidate engine digests: first=%s second=%s", first, second)
	}
}

func TestCandidateEngineBuildEnvironmentIsPinnedWithoutDroppingProofCustody(t *testing.T) {
	stamp := strings.Repeat("a", 40)
	environment := candidateEngineBuildEnvironment([]string{
		"PATH=/fixture/bin", "GOFLAGS=-mod=vendor", "GOWORK=/foreign/workspace", "GOTOOLCHAIN=auto",
		"GOEXPERIMENT=fieldtrack", "GOENV=/foreign/goenv", "CGO_ENABLED=1", "GOAMD64=v4", "GOARM64=v9.5", "GOARM=5",
		"METASYSTEM_PROOF_CONTROL_ROOT=/proof", "METASYSTEM_PROOF_ATTEMPT=proof-attempt",
	}, stamp)
	values := map[string]string{}
	for _, entry := range environment {
		name, value, _ := strings.Cut(entry, "=")
		values[name] = value
	}
	want := map[string]string{"CGO_ENABLED": "0", "GOAMD64": "v1", "GOARM64": "v8.0", "GOARM": "7",
		"GOENV": "off", "GOEXPERIMENT": "", "GOFLAGS": "-mod=readonly", "GOTOOLCHAIN": "local", "GOWORK": "off", "METASYSTEM_BUILD_STAMP": stamp,
		"METASYSTEM_PROOF_CONTROL_ROOT": "/proof", "METASYSTEM_PROOF_ATTEMPT": "proof-attempt"}
	for name, value := range want {
		if values[name] != value {
			t.Fatalf("candidate build environment %s=%q, want %q: %v", name, values[name], value, environment)
		}
	}
}

func TestTestWorkerBuildIdentityCompatibilityDoorIsPolicyProbeOnly(t *testing.T) {
	root := t.TempDir()
	candidateEngine := filepath.Join(root, "candidate-engine")
	writeTestingFixtureFile(t, candidateEngine, []byte("candidate engine\n"), 0o755)
	candidateDigest, err := fileSHA256(candidateEngine)
	if err != nil {
		t.Fatal(err)
	}
	request := proofrun.TestRunRequest{
		CandidateEngine: candidateEngine, CandidateEngineDigest: candidateDigest,
	}
	packet, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	packetPath := filepath.Join(root, "request.json")
	writeTestingFixtureFile(t, packetPath, packet, 0o600)
	packetDigest, err := fileSHA256(packetPath)
	if err != nil {
		t.Fatal(err)
	}
	args := []string{"--packet", packetPath, "--packet-sha256", packetDigest, "--result", filepath.Join(root, "result.json")}

	t.Setenv(policyProbeWorkerEnvironment, "")
	stderr, code := captureStderr(t, func() int { return runTestWorker(args) })
	if code != 3 || !strings.Contains(stderr, "input-bound candidate engine is absent") {
		t.Fatalf("ordinary worker accepted a request without a candidate build identity: code=%d stderr=%q", code, stderr)
	}

	t.Setenv(policyProbeWorkerEnvironment, "1")
	stderr, code = captureStderr(t, func() int { return runTestWorker(args) })
	if code != 3 || strings.Contains(stderr, "input-bound candidate engine is absent") ||
		!strings.Contains(stderr, "input-bound policy engine changed") {
		t.Fatalf("legacy policy probe did not pass the build-identity request check: code=%d stderr=%q", code, stderr)
	}
}

func TestCandidateBuiltCommitPassesDispatchSkewPreflight(t *testing.T) {
	fixture := newCandidateEngineFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	built, err := buildCandidateEngine(ctx, gittree.Workspace{Dir: fixture.projectRoot}, "metasystem", fixture.candidateTree, testingEnvironment(os.Environ()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = built.Close() })
	detached, err := (gittree.Workspace{Dir: fixture.projectRoot}).NewDetachedWorktree(fixture.candidateTree)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = detached.Close() })
	candidateRoot := filepath.Join(detached.Workspace().Dir, "metasystem")
	installedEngine := filepath.Join(candidateRoot, "bin", "metasystem")
	data, err := os.ReadFile(built.Path)
	if err != nil {
		t.Fatal(err)
	}
	writeTestingFixtureFile(t, installedEngine, data, 0o755)
	preflight := func(stamp string) ([]byte, error) {
		command := exec.Command("bash", "scripts/agents/dispatch.sh", "__engine-skew-preflight", stamp)
		command.Dir = candidateRoot
		command.Env = append(testingEnvironment(os.Environ()), "METASYSTEM_BIN="+installedEngine)
		return command.CombinedOutput()
	}
	oldOutput, oldErr := preflight(fixture.baseCommit)
	if exit, ok := oldErr.(*exec.ExitError); !ok || exit.ExitCode() != 1 || !strings.Contains(string(oldOutput), "is older than checkout commit") {
		t.Fatalf("untouched refusal was not reproduced with the enrolled engine stamp: err=%v output=%s", oldErr, oldOutput)
	}
	if output, err := preflight(""); err != nil {
		t.Fatalf("installed candidate engine's reported stamp did not pass dispatch skew preflight: %v\n%s", err, output)
	}
	ancestry := exec.Command("git", "-C", detached.Workspace().Dir, "log", "--ancestry-path", built.Commit+"..HEAD")
	ancestry.Env = gittree.ScrubbedEnviron()
	if output, err := ancestry.CombinedOutput(); err != nil || len(strings.TrimSpace(string(output))) != 0 {
		t.Fatalf("candidate stamp unexpectedly has an ancestry path to its materialized checkout: err=%v output=%s", err, output)
	}
	for _, stamp := range []string{"witness-0123456789ab", "dev-0123456789ab-dirty", "dev", "adopted-target"} {
		if output, err := preflight(stamp); err != nil {
			t.Fatalf("non-commit engine stamp %q was refused: %v\n%s", stamp, err, output)
		}
	}
	freshProject := t.TempDir()
	freshCandidateRoot := filepath.Join(freshProject, "metasystem")
	for _, relative := range []string{"dispatch.sh", "checkout-execution-guard.sh"} {
		script, err := os.ReadFile(filepath.Join(candidateRoot, "scripts", "agents", relative))
		if err != nil {
			t.Fatal(err)
		}
		writeTestingFixtureFile(t, filepath.Join(freshCandidateRoot, "scripts", "agents", relative), script, 0o755)
	}
	freshEngine := filepath.Join(freshCandidateRoot, "bin", "metasystem")
	writeTestingFixtureFile(t, freshEngine, data, 0o755)
	testingFixtureGit(t, freshProject, "init", "-q", "-b", "main")
	testingFixtureGit(t, freshProject, "add", ".")
	testingFixtureGit(t, freshProject, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "fresh fixture repository")
	missingCandidate := exec.Command("git", "-C", freshProject, "cat-file", "-e", built.Commit+"^{commit}")
	missingCandidate.Env = gittree.ScrubbedEnviron()
	if err := missingCandidate.Run(); err == nil {
		t.Fatalf("fresh fixture repository unexpectedly contains candidate stamp %s", built.Commit)
	}
	freshPreflight := exec.Command("bash", "scripts/agents/dispatch.sh", "__engine-skew-preflight")
	freshPreflight.Dir = freshCandidateRoot
	freshPreflight.Env = append(testingEnvironment(os.Environ()), "METASYSTEM_BIN="+freshEngine)
	if output, err := freshPreflight.CombinedOutput(); err != nil {
		t.Fatalf("fresh repository refused its installed candidate engine's reported stamp: %v\n%s", err, output)
	}
}

func TestCandidateEngineBuildFailureCannotFallBackToPolicyEngine(t *testing.T) {
	fixture := newCandidateEngineFixture(t)
	broken := []byte("#!/usr/bin/env bash\nset -euo pipefail\necho 'fixture candidate compile failed' >&2\nexit 23\n")
	writeTestingFixtureFile(t, filepath.Join(fixture.installationRoot, "scripts", "agents", "go-build.sh"), broken, 0o755)
	testingFixtureGit(t, fixture.projectRoot, "add", "metasystem/scripts/agents/go-build.sh")
	brokenTree, err := (gittree.Workspace{Dir: fixture.projectRoot}).StagedTree()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	built, err := buildCandidateEngine(ctx, gittree.Workspace{Dir: fixture.projectRoot}, "metasystem", brokenTree, testingEnvironment(os.Environ()))
	if built != nil || err == nil || !strings.Contains(err.Error(), "candidate engine build failed") || !strings.Contains(err.Error(), "fixture candidate compile failed") {
		t.Fatalf("candidate build failure did not remain an explicit insufficient outcome: build=%+v err=%v", built, err)
	}
	if digest, digestErr := fileSHA256(fixture.policyEngine); digestErr != nil || digest != fixture.policyDigest {
		t.Fatalf("candidate failure changed or substituted the policy engine: digest=%s err=%v", digest, digestErr)
	}
}

func TestVerifyRecoversCandidateDigestFromNewestSufficientAttempt(t *testing.T) {
	const groupID = "candidate-bed"
	digest := strings.Repeat("a", 64)
	executionIdentity := strings.Repeat("b", 64)
	candidateDigest := strings.Repeat("c", 64)
	buildIdentity := strings.Repeat("f", 40)
	failedDigest := strings.Repeat("d", 64)
	candidateTree := strings.Repeat("e", 40)
	group := testpolicy.Group{ID: groupID, Kind: "unit", CWD: ".", Inputs: []string{"source.go"},
		Obligations: []string{"candidate-engine"}, Platforms: []string{"any"}, TargetMS: 1}
	contract := testpolicy.Contract{SchemaVersion: 1, Groups: []testpolicy.Group{group}}
	plan := testpolicy.Plan{Purpose: testpolicy.PurposeDelivery, RequestedMode: testpolicy.ModeAuto,
		RequiredMode: testpolicy.ModeStandard, ExecutedMode: testpolicy.ModeStandard,
		RequiredGroups: []string{groupID}, SelectedGroups: []string{groupID}}
	prepared := testingPreparation{CandidateTree: candidateTree, EffectiveContract: contract, Plan: plan,
		ContractDigest: digest, BaseContractDigest: digest, PolicyEngineDigest: digest, BehaviorPolicyDigest: digest,
		GoalID: "goal", AccountingRevision: 2}
	request := testingRunRequest(prepared, "successful-attempt", "", "", candidateDigest, buildIdentity)
	request.ProjectRoot, request.BaseCommit = "/project", "base"
	successful := proofrun.NewTestResult(request)
	zero := 0
	successful.Groups = []proofrun.GroupResult{{ID: groupID, Kind: group.Kind, Obligations: group.Obligations,
		InputManifest: group.Inputs, ExecutionIdentity: executionIdentity, Status: "passed", NativeLaunched: true,
		CollectionComplete: true, NativeExitStatus: &zero, ToolIdentities: map[string]string{}, ReportDigests: map[string]string{}}}
	// A sufficient attempt from an earlier plan remains a valid source for
	// the deterministic candidate engine and for independently matching groups.
	successful.CandidateTree = strings.Repeat("9", 40)
	successful.PlanDigest = strings.Repeat("1", 64)
	successful.RecomputeDelivery()
	failed := successful
	failed.AttemptID = "later-failed-attempt"
	failed.CandidateEngineDigest = failedDigest
	exit := 23
	failed.Groups = append([]proofrun.GroupResult(nil), successful.Groups...)
	failed.Groups[0].Status, failed.Groups[0].NativeExitStatus = "failed", &exit
	failed.RecomputeDelivery()
	now := time.Now().UTC()
	attempts := []proofrun.Attempt{
		{AttemptID: successful.AttemptID, GoalID: prepared.GoalID, AccountingRevision: prepared.AccountingRevision,
			StartedAt: now.Add(-time.Minute).Format(time.RFC3339Nano), Terminal: &proofrun.AttemptTerminal{Result: proofrun.TerminalSuccess},
			PendingTestGroups: map[string]string{groupID: executionIdentity}, TestResult: &successful},
		{AttemptID: failed.AttemptID, GoalID: prepared.GoalID, AccountingRevision: prepared.AccountingRevision,
			StartedAt: now.Format(time.RFC3339Nano), Terminal: &proofrun.AttemptTerminal{Result: proofrun.TerminalFailed}, TestResult: &failed},
	}
	recovered, err := retainedCandidateEngineDigest(prepared, attempts, buildIdentity)
	if err != nil || recovered != candidateDigest {
		t.Fatalf("later failed attempt hid the sufficient candidate engine: digest=%s err=%v", recovered, err)
	}
	templateRequest := testingRunRequest(prepared, "", "", "", recovered, buildIdentity)
	templateRequest.ProjectRoot, templateRequest.BaseCommit = "/project", "base"
	projection := proofrun.ReusedTestResult(proofrun.NewTestResult(templateRequest), attempts,
		map[string]string{groupID: executionIdentity}, contract, prepared.GoalID, prepared.AccountingRevision)
	if !projection.Delivery.Sufficient || len(projection.Groups) != 1 || projection.Groups[0].ReuseAttempt != successful.AttemptID {
		t.Fatalf("verification did not compose the earlier sufficient group after a failed attempt: %+v", projection)
	}
}

func TestVerifyDoesNotKeyLegacyCandidateDigestByWholeTreeReceipt(t *testing.T) {
	const groupID = "candidate-bed"
	digest := strings.Repeat("a", 64)
	candidateTree := strings.Repeat("b", 40)
	group := testpolicy.Group{ID: groupID, Kind: "unit", CWD: ".", Inputs: []string{"source.go"},
		Obligations: []string{"candidate-engine"}, Platforms: []string{"any"}, TargetMS: 1}
	contract := testpolicy.Contract{SchemaVersion: 1, Groups: []testpolicy.Group{group}}
	plan := testpolicy.Plan{Purpose: testpolicy.PurposeDelivery, RequestedMode: testpolicy.ModeAuto,
		RequiredMode: testpolicy.ModeStandard, ExecutedMode: testpolicy.ModeStandard,
		RequiredGroups: []string{groupID}, SelectedGroups: []string{groupID}}
	prepared := testingPreparation{Installation: t.TempDir(), CandidateTree: candidateTree, EffectiveContract: contract, Plan: plan,
		ContractDigest: digest, BaseContractDigest: digest, PolicyEngineDigest: digest, BehaviorPolicyDigest: digest,
		GoalID: "goal", AccountingRevision: 2}
	request := testingRunRequest(prepared, "legacy-success", "", "", digest, strings.Repeat("c", 40))
	request.ProjectRoot, request.BaseCommit = "/project", "base"
	legacy := proofrun.NewTestResult(request)
	legacy.CandidateEngineIdentityVersion = 0
	legacy.CandidateEngineDigest = ""
	zero := 0
	legacy.Groups = []proofrun.GroupResult{{ID: groupID, Kind: group.Kind, Obligations: group.Obligations,
		InputManifest: group.Inputs, ExecutionIdentity: digest, Status: "passed", NativeLaunched: true,
		CollectionComplete: true, NativeExitStatus: &zero, ToolIdentities: map[string]string{}, ReportDigests: map[string]string{}}}
	legacy.RecomputeDelivery()
	receipt := landing.TestReceipt{SchemaVersion: 2, Tree: candidateTree, ProvedTree: candidateTree, Testing: &legacy}
	payload, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	path := landing.TestReceiptPath(prepared.Installation, candidateTree)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	recovered, err := retainedCandidateEngineDigest(prepared, nil, strings.Repeat("d", 40))
	if err == nil || recovered != "" || !strings.Contains(err.Error(), "candidate engine digest is absent") {
		t.Fatalf("legacy whole-tree receipt unexpectedly supplied a cross-tip engine identity: digest=%s err=%v", recovered, err)
	}
}

type candidateEngineFixture struct {
	projectRoot, installationRoot, baseCommit, candidateTree, policyEngine, policyDigest string
}

func newCandidateEngineFixture(t *testing.T) candidateEngineFixture {
	t.Helper()
	projectRoot := t.TempDir()
	installationRoot := filepath.Join(projectRoot, "metasystem")
	buildScript := `#!/usr/bin/env bash
set -euo pipefail
[[ "$1" == --trimpath && "$2" == --out && -n "${3:-}" ]]
[[ "${CGO_ENABLED+x}:$CGO_ENABLED" == x:0 ]]
[[ "${GOENV+x}:$GOENV" == x:off ]]
[[ "${GOEXPERIMENT+x}:$GOEXPERIMENT" == x: ]]
[[ "${GOFLAGS+x}:$GOFLAGS" == x:-mod=readonly ]]
[[ "${GOTOOLCHAIN+x}:$GOTOOLCHAIN" == x:local ]]
[[ "${GOWORK+x}:$GOWORK" == x:off ]]
stamp=$(git rev-parse HEAD)
[[ "$METASYSTEM_BUILD_STAMP" == "$stamp" ]]
source=$(cat cmd/metasystem/engine.txt)
{
  printf '#!/usr/bin/env bash\nstamp=%q\n' "$stamp"
  cat <<'ENGINE'
if [[ "${1:-}" == supervise && "${2:-}" == status ]]; then
  printf '{"engineBuild":"%s"}\n' "$stamp"
  exit 0
fi
if [[ "${1:-}" == json && "${2:-}" == get ]]; then
  printf '%s\n' "$stamp"
  exit 0
fi
exit 0
ENGINE
  printf '# %s\n' "$source"
} >"$3"
chmod +x "$3"
`
	writeTestingFixtureFile(t, filepath.Join(installationRoot, "scripts", "agents", "go-build.sh"), []byte(buildScript), 0o755)
	for _, relative := range []string{"dispatch.sh", "checkout-execution-guard.sh"} {
		data, err := os.ReadFile(filepath.Join("..", "..", "scripts", "agents", relative))
		if err != nil {
			t.Fatal(err)
		}
		writeTestingFixtureFile(t, filepath.Join(installationRoot, "scripts", "agents", relative), data, 0o755)
	}
	writeTestingFixtureFile(t, filepath.Join(installationRoot, "scripts", "agents", "validate-section-selector.sh"), []byte("#!/usr/bin/env bash\nexit 0\n"), 0o755)
	writeTestingFixtureFile(t, filepath.Join(installationRoot, "cmd", "metasystem", "engine.txt"), []byte("enrolled engine source\n"), 0o644)
	testingFixtureGit(t, projectRoot, "init", "-q", "-b", "main")
	testingFixtureGit(t, projectRoot, "add", ".")
	testingFixtureGit(t, projectRoot, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "enrolled engine")
	baseCommit := strings.TrimSpace(testingFixtureGit(t, projectRoot, "rev-parse", "HEAD"))
	policyEngine := filepath.Join(t.TempDir(), "metasystem")
	writeTestingFixtureFile(t, policyEngine, []byte("#!/usr/bin/env bash\n# enrolled engine\nexit 0\n"), 0o755)
	policyDigest, err := fileSHA256(policyEngine)
	if err != nil {
		t.Fatal(err)
	}
	writeTestingFixtureFile(t, filepath.Join(installationRoot, "cmd", "metasystem", "engine.txt"), []byte("candidate engine source\n"), 0o644)
	testingFixtureGit(t, projectRoot, "add", "metasystem/cmd/metasystem/engine.txt")
	candidateTree, err := (gittree.Workspace{Dir: projectRoot}).StagedTree()
	if err != nil {
		t.Fatal(err)
	}
	return candidateEngineFixture{projectRoot: projectRoot, installationRoot: installationRoot, baseCommit: baseCommit,
		candidateTree: candidateTree, policyEngine: policyEngine, policyDigest: policyDigest}
}

func TestTestingPlanAdoptsCandidateFallbackOnlyWhenBaseHasNone(t *testing.T) {
	candidate := testFallbackContract()
	for _, test := range []struct {
		name             string
		baseFallback     bool
		expectedFallback string
	}{
		{name: "candidate-fallback-fills-empty-base", expectedFallback: "residual"},
		{name: "base-fallback-remains-protected", baseFallback: true, expectedFallback: "trusted"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			base := candidate
			base.Surfaces = append([]testpolicy.Surface(nil), candidate.Surfaces...)
			base.Groups = append([]testpolicy.Group(nil), candidate.Groups...)
			if test.baseFallback {
				base.Fallback = "trusted"
				base.Surfaces[1].Paths = []string{"candidate-owned/**"}
				base.Surfaces = append(base.Surfaces, testpolicy.Surface{ID: "trusted", Paths: []string{}, Standard: []string{"trusted"}})
				base.Groups = append(base.Groups, testFallbackGroup("trusted", "unowned.txt"))
			} else {
				base.Fallback = ""
				base.Surfaces = append([]testpolicy.Surface(nil), candidate.Surfaces[:1]...)
				base.Groups = append([]testpolicy.Group(nil), candidate.Groups[:1]...)
			}
			baseBytes, err := json.Marshal(base)
			if err != nil {
				t.Fatal(err)
			}
			candidateBytes, err := json.Marshal(candidate)
			if err != nil {
				t.Fatal(err)
			}
			writeTestingFixtureFile(t, filepath.Join(root, "metasystem.conf"), []byte("testing.contract=testing.json\n"), 0o644)
			writeTestingFixtureFile(t, filepath.Join(root, "testing.json"), baseBytes, 0o644)
			writeTestingFixtureFile(t, filepath.Join(root, "owned", "source.go"), []byte("package owned\n"), 0o644)
			testingFixtureGit(t, root, "init", "-q", "-b", "main")
			testingFixtureGit(t, root, "add", ".")
			testingFixtureGit(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "base")
			baseCommit := strings.TrimSpace(testingFixtureGit(t, root, "rev-parse", "HEAD"))
			const landingRef = "refs/remotes/origin/main"
			testingFixtureGit(t, root, "update-ref", landingRef, baseCommit)
			testingFixtureGit(t, root, "config", "--local", "metasystem.steward.landing-ref", landingRef)

			engine := filepath.Join(t.TempDir(), "metasystem")
			build := exec.Command("go", "build", "-buildvcs=false", "-ldflags",
				"-X github.com/widoriezebos/agentic-tools/metasystem/internal/supervise.BuildStamp="+baseCommit, "-o", engine, ".")
			if output, buildErr := build.CombinedOutput(); buildErr != nil {
				t.Fatalf("build fallback policy engine: %v\n%s", buildErr, output)
			}
			canonicalRoot, err := canonicalProofRoot(root)
			if err != nil {
				t.Fatal(err)
			}
			canonicalEngine, err := canonicalPath(engine)
			if err != nil {
				t.Fatal(err)
			}
			digest, err := fileSHA256(canonicalEngine)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Dir(steward.RepoIdentityPath(root)), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := steward.MintIdentity(steward.RepoIdentityPath(root), steward.InstallIdentity{RepoIdentity: canonicalRoot, Generation: 1,
				InstallPath: canonicalEngine, InstallDigest: "sha256:" + digest, MintedAt: "2026-09-10T00:00:00Z", Enrollment: steward.EnrollmentFixture,
				EngineBuild: baseCommit, LandedCommit: baseCommit, LandingRef: landingRef}); err != nil {
				t.Fatal(err)
			}

			writeTestingFixtureFile(t, filepath.Join(root, "testing.json"), candidateBytes, 0o644)
			writeTestingFixtureFile(t, filepath.Join(root, "unowned.txt"), []byte("changed\n"), 0o644)
			testingFixtureGit(t, root, "add", "testing.json", "unowned.txt")
			candidateTree, err := (gittree.Workspace{Dir: root}).StagedTree()
			if err != nil {
				t.Fatal(err)
			}
			t.Setenv("GIT_OBJECT_DIRECTORY", filepath.Join(root, ".git", "objects"))
			t.Setenv("GIT_ALTERNATE_OBJECT_DIRECTORIES", "")
			t.Setenv("GIT_CONFIG_COUNT", "0")
			prepared, err := prepareTesting(testingSelectionRequest{Root: root, Tree: candidateTree, Mode: testpolicy.ModeAuto, Purpose: testpolicy.PurposeDiagnostic})
			if err != nil {
				t.Fatal(err)
			}
			if prepared.EffectiveContract.Fallback != test.expectedFallback {
				t.Fatalf("effective fallback = %q, want %q", prepared.EffectiveContract.Fallback, test.expectedFallback)
			}
			if len(prepared.Plan.Uncertainty) != 0 || !containsString(prepared.Plan.AffectedSurfaces, test.expectedFallback) || !containsString(prepared.Plan.SelectedGroups, test.expectedFallback) {
				t.Fatalf("unowned path did not select the fallback without uncertainty: %+v", prepared.Plan)
			}
			if test.baseFallback && containsString(prepared.Plan.AffectedSurfaces, candidate.Fallback) {
				t.Fatalf("candidate fallback replaced the protected base fallback: %+v", prepared.Plan)
			}
		})
	}
}

func testFallbackContract() testpolicy.Contract {
	return testpolicy.Contract{SchemaVersion: 1,
		ProjectRisk: testpolicy.ProjectRisk{Severity: 1, Exposure: 1, Reversibility: "revert", Detection: "immediate", Recovery: "bounded"},
		Fallback:    "residual",
		Surfaces: []testpolicy.Surface{
			{ID: "app", Paths: []string{"owned/**"}, Standard: []string{"app"}},
			{ID: "residual", Paths: []string{}, Standard: []string{"residual"}},
		},
		Groups:  []testpolicy.Group{testFallbackGroup("app", "owned/**"), testFallbackGroup("residual", "unowned.txt")},
		Always:  testpolicy.Always{Canary: []string{"app"}},
		Unknown: []string{"app"},
		Cadence: []string{"app"},
	}
}

func testFallbackGroup(id, input string) testpolicy.Group {
	return testpolicy.Group{ID: id, Kind: "unit", Adapter: "go", CWD: ".", Inputs: []string{input}, Outputs: []string{}, Tools: []testpolicy.Tool{},
		Obligations: []string{}, Platforms: []string{"any"}, TargetMS: 1000, Packages: []string{"."}, Tests: json.RawMessage(`"all"`)}
}

func TestProtectedCoverageFloorCannotFallOrDisappear(t *testing.T) {
	root := t.TempDir()
	writeTestingFixtureFile(t, filepath.Join(root, "scripts", "agents", "coverage-ratchet.json"), []byte(`{"floors":{"internal/app":80.0}}`), 0o644)
	writeTestingFixtureFile(t, filepath.Join(root, "scripts", "agents", "coverage-ratchet-linux.json"), []byte(`{"floors":{"internal/app":79.0}}`), 0o644)
	testingFixtureGit(t, root, "init")
	testingFixtureGit(t, root, "add", ".")
	testingFixtureGit(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-m", "base")
	workspace := gittree.Workspace{Dir: root}
	base, err := workspace.HeadTree()
	if err != nil {
		t.Fatal(err)
	}
	writeTestingFixtureFile(t, filepath.Join(root, "scripts", "agents", "coverage-ratchet.json"), []byte(`{"floors":{"internal/app":79.9}}`), 0o644)
	testingFixtureGit(t, root, "add", ".")
	candidate, err := workspace.StagedTree()
	if err != nil {
		t.Fatal(err)
	}
	if err := protectCoverageRatchets(workspace, base, candidate, ""); err == nil || !strings.Contains(err.Error(), "TEST_POLICY_COVERAGE_FLOOR_LOWERED") {
		t.Fatalf("lowered base floor was accepted: %v", err)
	}
	writeTestingFixtureFile(t, filepath.Join(root, "scripts", "agents", "coverage-ratchet.json"), []byte(`{"floors":{"internal/app":80.1,"internal/new":50.0}}`), 0o644)
	testingFixtureGit(t, root, "add", ".")
	candidate, err = workspace.StagedTree()
	if err != nil || protectCoverageRatchets(workspace, base, candidate, "") != nil {
		t.Fatalf("raised protected floor was refused: %v", err)
	}
}

func TestFrozenPublicVersionOneProtectionCorpusIsComplete(t *testing.T) {
	cases := testpolicy.FrozenProtectionProbeCases()
	want := []string{"remove-required-provider", "shrink-dependency-graph", "lower-coverage-floor", "remove-required-test", "emit-zero-tests", "forge-component-reuse"}
	if len(cases) != len(want) {
		t.Fatalf("frozen corpus has %d cases, want %d", len(cases), len(want))
	}
	for index, id := range want {
		if cases[index].ID != id || len(cases[index].PublicArgv) != 2 || cases[index].PublicArgv[0] != "test" || cases[index].ResultField == "" {
			t.Fatalf("frozen corpus case %d = %+v", index, cases[index])
		}
	}
}

func TestFrozenWorkerProbeReaderAcceptsCandidateGroupFields(t *testing.T) {
	group := testpolicy.Group{ID: "literal", Kind: "unit", Inputs: []string{"source.txt"}, TargetMS: 1}
	request := proofrun.TestRunRequest{
		ProjectRoot:           t.TempDir(),
		CandidateTree:         strings.Repeat("a", 40),
		BaseCommit:            strings.Repeat("b", 40),
		Contract:              testpolicy.Contract{SchemaVersion: 1, Groups: []testpolicy.Group{group}},
		Plan:                  testpolicy.Plan{Purpose: testpolicy.PurposeDelivery, RequestedMode: testpolicy.ModeStandard, RequiredMode: testpolicy.ModeStandard, ExecutedMode: testpolicy.ModeStandard, RequiredGroups: []string{group.ID}, SelectedGroups: []string{group.ID}},
		CandidateEngineDigest: strings.Repeat("c", 64),
	}
	result := proofrun.NewTestResult(request)
	result.Groups = []proofrun.GroupResult{{ID: group.ID, Kind: group.Kind, InputManifest: group.Inputs, Status: "invalid"}}
	result.RecomputeDelivery()

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var candidate map[string]any
	if err := json.Unmarshal(data, &candidate); err != nil {
		t.Fatal(err)
	}
	candidateGroups := candidate["groups"].([]any)
	// A field only a newer candidate engine writes; it must stay unknown to
	// this engine's strict reader, so it is not any field the shape has since
	// adopted (progressRule joined the shape on 2026-09-11).
	candidateGroups[0].(map[string]any)["candidateOnlyField"] = "from-a-newer-engine"
	data, err = json.Marshal(candidate)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "result.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	probeResult, err := readFrozenWorkerProbeResult(path)
	if err != nil || len(probeResult.Groups) != 1 || probeResult.Groups[0].Status != "invalid" || probeResult.Groups[0].CollectionComplete {
		t.Fatalf("probe reader lost the negative worker judgment: result=%+v err=%v", probeResult, err)
	}
	if _, err := readTestingWorkerResult(path); err == nil || !strings.Contains(err.Error(), `unknown field "candidateOnlyField"`) {
		t.Fatalf("strict destination worker reader accepted the candidate field: %v", err)
	}
}

func TestFrozenPublicVersionOneSelectionProbesRunAgainstCandidateExecutable(t *testing.T) {
	root := t.TempDir()
	group := testpolicy.Group{ID: "policy-protection", Kind: "unit", Adapter: "go", CWD: ".", Inputs: []string{"source.go"},
		Outputs: []string{}, Tools: []testpolicy.Tool{}, Obligations: []string{"testing-policy-protected"}, Platforms: []string{"any"},
		TargetMS: 1000, Packages: []string{"."}, Tests: json.RawMessage(`["TestGuard"]`)}
	smoke := testpolicy.Group{ID: "candidate-smoke", Kind: "unit", Adapter: "go", CWD: ".", Inputs: []string{"source.go"},
		Outputs: []string{}, Tools: []testpolicy.Tool{}, Obligations: []string{"candidate-smoke", "testing-policy-protected"}, Platforms: []string{"any"},
		TargetMS: 1000, Packages: []string{"."}, Tests: json.RawMessage(`["TestSmoke"]`)}
	contract := testpolicy.Contract{SchemaVersion: 1,
		ProjectRisk: testpolicy.ProjectRisk{Severity: 1, Exposure: 1, Reversibility: "revert", Detection: "immediate", Recovery: "bounded"},
		Surfaces: []testpolicy.Surface{
			{ID: "testing-policy", Paths: []string{"testing.json", "scripts/agents/coverage-ratchet.json", "scripts/agents/coverage-ratchet-linux.json"}, Standard: []string{"policy-protection", "candidate-smoke"}, Deep: []string{"policy-protection", "candidate-smoke"}, Critical: []string{"testing-policy-protected"}},
			{ID: "proof-and-landing", Paths: []string{"source.go"}, DependsOn: []string{"testing-policy"}, Standard: []string{"policy-protection", "candidate-smoke"}, Deep: []string{"policy-protection", "candidate-smoke"}, Critical: []string{"testing-policy-protected"}},
		}, Groups: []testpolicy.Group{group, smoke}, Always: testpolicy.Always{Canary: []string{"policy-protection", "candidate-smoke"}, Standard: []string{"policy-protection", "candidate-smoke"}},
		Unknown: []string{"policy-protection", "candidate-smoke"}, Cadence: []string{"policy-protection", "candidate-smoke"}}
	data, err := json.Marshal(contract)
	if err != nil {
		t.Fatal(err)
	}
	writeTestingFixtureFile(t, filepath.Join(root, "metasystem.conf"), []byte("testing.contract=testing.json\nmetasystem.runtimes=fake\n"), 0o644)
	writeTestingFixtureFile(t, filepath.Join(root, "testing.json"), data, 0o644)
	writeTestingFixtureFile(t, filepath.Join(root, "source.go"), []byte("package fixture\n"), 0o644)
	for _, name := range []string{"coverage-ratchet.json", "coverage-ratchet-linux.json"} {
		writeTestingFixtureFile(t, filepath.Join(root, "scripts", "agents", name), []byte(`{"floors":{"internal/app":80.0}}`), 0o644)
	}
	writeTestingFixtureFile(t, filepath.Join(root, ".gitignore"), []byte("artifacts/\nbin/\n"), 0o644)
	testingFixtureGit(t, root, "init")
	testingFixtureGit(t, root, "add", ".")
	testingFixtureGit(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-m", "base")
	head, unborn, err := (gittree.Workspace{Dir: root}).HeadCommit()
	if err != nil || unborn {
		t.Fatal(err)
	}
	const landingRef = "refs/remotes/origin/main"
	testingFixtureGit(t, root, "update-ref", landingRef, head)
	testingFixtureGit(t, root, "config", "--local", "metasystem.steward.landing-ref", landingRef)
	refsBefore := testingFixtureGit(t, root, "for-each-ref", "--format=%(refname) %(objectname)")
	configBefore := testingFixtureGit(t, root, "config", "--local", "--list")
	goPath, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}
	engine := filepath.Join(t.TempDir(), "metasystem")
	build := exec.Command(goPath, "build", "-buildvcs=false", "-ldflags",
		"-X github.com/widoriezebos/agentic-tools/metasystem/internal/supervise.BuildStamp="+head, "-o", engine, ".")
	if output, buildErr := build.CombinedOutput(); buildErr != nil {
		t.Fatalf("build public-v1 probe candidate: %v\n%s", buildErr, output)
	}
	engine, err = canonicalPath(engine)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := fileSHA256(engine)
	if err != nil {
		t.Fatal(err)
	}
	request := proofrun.TestRunRequest{ControlRoot: root, ProjectRoot: root, PolicyBaseCommit: head, PolicyEngine: engine, PolicyEngineDigest: digest,
		CandidateEngine: engine, CandidateEngineDigest: digest, Environment: testingEnvironment(os.Environ())}
	for _, probe := range testpolicy.FrozenProtectionProbeCases()[:4] {
		t.Run(probe.ID, func(t *testing.T) {
			if err := runFrozenSelectionProbe(context.Background(), request, probe); err != nil {
				t.Fatal(err)
			}
			if refsAfter := testingFixtureGit(t, root, "for-each-ref", "--format=%(refname) %(objectname)"); refsAfter != refsBefore {
				t.Fatalf("successful protected probe changed caller refs:\nbefore:\n%safter:\n%s", refsBefore, refsAfter)
			}
			if configAfter := testingFixtureGit(t, root, "config", "--local", "--list"); configAfter != configBefore {
				t.Fatalf("successful protected probe changed caller configuration:\nbefore:\n%safter:\n%s", configBefore, configAfter)
			}
		})
	}
	failedRequest := request
	failedRequest.PolicyBaseCommit = strings.Repeat("b", 40)
	if err := runFrozenSelectionProbe(context.Background(), failedRequest, testpolicy.FrozenProtectionProbeCases()[0]); err == nil {
		t.Fatal("protected probe accepted a destination that differed from the real caller ref")
	}
	if refsAfter := testingFixtureGit(t, root, "for-each-ref", "--format=%(refname) %(objectname)"); refsAfter != refsBefore {
		t.Fatalf("failed protected probe changed caller refs:\nbefore:\n%safter:\n%s", refsBefore, refsAfter)
	}
	if configAfter := testingFixtureGit(t, root, "config", "--local", "--list"); configAfter != configBefore {
		t.Fatalf("failed protected probe changed caller configuration:\nbefore:\n%safter:\n%s", configBefore, configAfter)
	}
}

func TestFrozenPublicVersionOneCorpusRunsAllSixCasesThroughFirstTransitionWorker(t *testing.T) {
	// The module root is the metasystem directory wherever the package sits:
	// under the repository (<repo>/metasystem) or at the root of the gate's
	// extracted snapshot, which has no parent repository around it.
	moduleRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	// Freeze the actual source under test, whether it is committed or still
	// being edited. The executable and both commits must name these bytes.
	frozen, err := proofrun.Freeze(moduleRoot)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(filepath.Dir(frozen.Root)) })
	projectRoot, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(projectRoot, "metasystem")
	if err := os.Rename(frozen.Root, root); err != nil {
		t.Fatal(err)
	}
	testingFixtureGit(t, projectRoot, "init", "-q", "-b", "main")
	testingFixtureGit(t, projectRoot, "config", "user.name", "Test")
	testingFixtureGit(t, projectRoot, "config", "user.email", "test@example.invalid")
	if err := os.Remove(filepath.Join(root, "testing.json")); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	testingFixtureGit(t, projectRoot, "add", "-A")
	testingFixtureGit(t, projectRoot, "-c", "core.hooksPath=/dev/null", "commit", "-qm", "source without testing contract")
	base := strings.TrimSpace(testingFixtureGit(t, projectRoot, "rev-parse", "HEAD"))
	if _, present, err := (gittree.Workspace{Dir: projectRoot}).FileAt(base, "metasystem/testing.json"); err != nil || present {
		t.Fatalf("first-transition base contains a testing contract: present=%v err=%v", present, err)
	}

	command := `mkdir -p reports; printf '%s\n' '<testsuite><testcase classname="protection" name="required"/></testsuite>' > reports/result.xml`
	group := testpolicy.Group{ID: "policy-protection", Kind: "unit", Adapter: "command", CWD: "metasystem",
		Inputs: []string{"metasystem/testing.json", "metasystem/internal/testpolicy/**"}, Outputs: []string{"metasystem/reports"}, Obligations: []string{"testing-policy-protected"},
		Platforms: []string{"any"}, TargetMS: 1000, Argv: []string{"sh", "-c", command}, Reports: []string{"metasystem/reports"}, Format: "junit-xml",
		ExpectedTests: []testpolicy.ExpectedTest{{Report: "metasystem/reports/result.xml", Classname: "protection", Name: "required"}}}
	smoke := testpolicy.Group{ID: "candidate-smoke", Kind: "unit", Adapter: "command", CWD: "metasystem",
		Inputs: []string{"metasystem/testing.json"}, Outputs: []string{"metasystem/reports-smoke"}, Obligations: []string{"candidate-smoke", "testing-policy-protected"}, Platforms: []string{"any"}, TargetMS: 1000,
		Argv:    []string{"sh", "-c", `mkdir -p reports-smoke; printf '%s\n' '<testsuite><testcase classname="candidate" name="smoke"/></testsuite>' > reports-smoke/result.xml`},
		Reports: []string{"metasystem/reports-smoke"}, Format: "junit-xml", ExpectedTests: []testpolicy.ExpectedTest{{Report: "metasystem/reports-smoke/result.xml", Classname: "candidate", Name: "smoke"}}}
	groups := []testpolicy.Group{group, smoke}
	for index, id := range []string{"fast-static-build", "section/dispatcher-adapter-and-mission-runner-fixtures", "section/goal-cli-fixtures", "section/land-fixtures", "section/adoption-fixtures"} {
		reportDir := fmt.Sprintf("reports-transition-%d", index)
		projectReportDir := "metasystem/" + reportDir
		fixtureCommand := fmt.Sprintf(`test -s testing.json && mkdir -p %s && printf '%%s\n' '<testsuite><testcase classname="transition" name="case-%d"/></testsuite>' > %s/result.xml`, reportDir, index, reportDir)
		groups = append(groups, testpolicy.Group{ID: id, Kind: "integration", Adapter: "command", CWD: "metasystem",
			Inputs: []string{"metasystem/testing.json"}, Outputs: []string{projectReportDir}, Obligations: []string{"first-transition-" + id}, Platforms: []string{"any"}, TargetMS: 1000,
			Argv: []string{"sh", "-c", fixtureCommand}, Reports: []string{projectReportDir}, Format: "junit-xml",
			ExpectedTests: []testpolicy.ExpectedTest{{Report: projectReportDir + "/result.xml", Classname: "transition", Name: fmt.Sprintf("case-%d", index)}}})
	}
	contract := testpolicy.Contract{SchemaVersion: 1,
		ProjectRisk: testpolicy.ProjectRisk{Severity: 1, Exposure: 1, Reversibility: "revert", Detection: "immediate", Recovery: "bounded"},
		Surfaces: []testpolicy.Surface{
			{ID: "testing-policy", Paths: []string{"metasystem/testing.json", "metasystem/metasystem.conf", "metasystem/.gitignore", "metasystem/plans/goals/**", "metasystem/internal/testpolicy/**", "metasystem/scripts/agents/coverage-ratchet.json", "metasystem/scripts/agents/coverage-ratchet-linux.json"}, Standard: []string{"policy-protection", "candidate-smoke"}, Deep: []string{"policy-protection", "candidate-smoke"}, Critical: []string{"testing-policy-protected"}},
			{ID: "proof-and-landing", Paths: []string{"metasystem/cmd/metasystem/test.go"}, DependsOn: []string{"testing-policy"}, Standard: []string{"policy-protection", "candidate-smoke"}, Deep: []string{"policy-protection", "candidate-smoke"}, Critical: []string{"testing-policy-protected"}},
		}, Groups: groups, Always: testpolicy.Always{Canary: []string{"policy-protection", "candidate-smoke"}, Standard: []string{"policy-protection", "candidate-smoke"}},
		Unknown: []string{"policy-protection", "candidate-smoke"}, Cadence: []string{"policy-protection", "candidate-smoke"}}
	data, err := json.Marshal(contract)
	if err != nil {
		t.Fatal(err)
	}
	writeTestingFixtureFile(t, filepath.Join(root, "testing.json"), data, 0o644)
	confPath := filepath.Join(root, "metasystem.conf")
	writeTestingFixtureFile(t, confPath, []byte("metasystem.version=1\nmetasystem.runtimes=fake\nrole.code-critic.runtime=fake\ntesting.contract=testing.json\ndispatch.cap-min=1\ndispatch.cap-max=120\n"), 0o644)
	ignorePath := filepath.Join(root, ".gitignore")
	ignore, err := os.ReadFile(ignorePath)
	if err != nil {
		t.Fatal(err)
	}
	writeTestingFixtureFile(t, ignorePath, append(ignore, []byte("reports*/\n")...), 0o644)
	now := time.Now().UTC()
	risk := &goal.RiskRecord{Severity: 1, Novelty: 1, Exposure: 1, Accumulation: 1, Basis: "The fixture runs one bounded policy corpus."}
	// Six frozen cases through a real worker take about two and a half
	// minutes under the race detector on a loaded box; a one-minute cap made
	// this a wall-clock test that failed only inside the race gate.
	budget := &goal.Budget{ElapsedLimit: "1h", AttemptLimit: 2, ReservedJobMinutesLimit: 6, ActiveJobLimit: 1, ReviewRoundLimit: 2}
	intent := "Run the frozen policy corpus through the first testing transition."
	rootRecord := &goal.RootRecord{Identity: "01ARZ3NDEKTSV4RRFFQ69G5FB0", FormatVersion: "1", SyncMode: goal.SyncLocal, Revision: 1}
	goalFile := &goal.GoalFile{Id: "policy-corpus", State: goal.StateClaimed, Tier: 1, Risk: risk, Intent: intent, Origin: goal.OriginMain,
		NextStep: "Run the authenticated worker.", OpenedAt: now.Add(-2 * time.Minute).Format(time.RFC3339), Revision: 3, Budget: budget,
		Claimed: &goal.ClaimRecord{Machine: "fixture-machine", Lineage: "policy-corpus", At: now.Add(-time.Minute).Format(time.RFC3339), Revision: 2, AccountingRevision: 2},
		Approved: &goal.ApprovalRecord{By: "human:fixture", At: now.Add(-30 * time.Second).Format(time.RFC3339), Revision: 3,
			Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FB3", "fixture-machine", "policy-corpus"), Authority: goal.ApprovalAuthorityProven,
			Digest: goal.ApprovalDigest(intent, 1, *budget, risk)},
		StopCapability: &goal.StopCapability{Generation: 3, Revision: 2, Machine: "fixture-machine", ClaimEpoch: 1},
		History: []goal.HistoryLine{
			{At: now.Add(-2 * time.Minute).Format(time.RFC3339), Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FB1", "fixture-machine", "policy-corpus"), Verb: "open", Actor: "fixture-machine+policy-corpus", Keep: -1},
			{At: now.Add(-time.Minute).Format(time.RFC3339), Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FB2", "fixture-machine", "policy-corpus"), Verb: "claim", Actor: "fixture-machine+policy-corpus", Keep: -1},
			{At: now.Add(-30 * time.Second).Format(time.RFC3339), Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FB3", "fixture-machine", "policy-corpus"), Verb: "approve", Actor: "human:fixture", Keep: -1},
		}}
	writeTestingFixtureFile(t, filepath.Join(root, "plans", "goals", "backlog.md"), goal.RenderRoot(rootRecord), 0o644)
	writeTestingFixtureFile(t, filepath.Join(root, "plans", "goals", "policy-corpus.md"), goal.RenderFile(goalFile), 0o644)
	testingFixtureGit(t, projectRoot, "config", "goal.sync-remote", "local")
	testingFixtureGit(t, projectRoot, "config", "goal.sync-branch", goal.LocalLedgerBranch)
	testingFixtureGit(t, projectRoot, "add", "-A", "metasystem/testing.json", "metasystem/metasystem.conf", "metasystem/.gitignore", "metasystem/internal/testpolicy", "metasystem/plans/goals")
	testingFixtureGit(t, projectRoot, "commit", "-qm", "reviewed testing contract")
	candidateCommit := strings.TrimSpace(testingFixtureGit(t, projectRoot, "rev-parse", "HEAD"))
	candidate := strings.TrimSpace(testingFixtureGit(t, projectRoot, "rev-parse", "HEAD^{tree}"))
	testingFixtureGit(t, projectRoot, "update-ref", "refs/remotes/origin/main", base)
	testingFixtureGit(t, projectRoot, "update-ref", goal.LocalLedgerBranch, candidateCommit)
	testingFixtureGit(t, projectRoot, "update-ref", goal.AcceptedRef, candidateCommit)
	testingFixtureGit(t, projectRoot, "config", "--local", "metasystem.steward.landing-ref", "refs/remotes/origin/main")

	engine := filepath.Join(t.TempDir(), "metasystem")
	build := exec.Command("go", "build", "-buildvcs=false", "-ldflags",
		"-X github.com/widoriezebos/agentic-tools/metasystem/internal/supervise.BuildStamp="+candidateCommit, "-o", engine, ".")
	build.Dir = filepath.Join(root, "cmd", "metasystem")
	build.Env = gittree.ScrubbedEnviron()
	if output, buildErr := build.CombinedOutput(); buildErr != nil {
		t.Fatalf("build first-transition worker: %v\n%s", buildErr, output)
	}
	identityTable := filepath.Join(t.TempDir(), "process-identities.json")
	identities := map[string]map[string]any{
		fmt.Sprint(os.Getpid()): {"terminal": true},
	}
	seen := map[int64]bool{int64(os.Getpid()): true}
	current, ok := identity.ParentPid(int64(os.Getpid()))
	for ok && !seen[current] {
		seen[current] = true
		identities[fmt.Sprint(current)] = map[string]any{"pidStartedAt": 1, "command": "fixture-neutral-ancestor"}
		current, ok = identity.ParentPid(current)
	}
	identityData, err := json.Marshal(identities)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(identityTable, identityData, 0o600); err != nil {
		t.Fatal(err)
	}
	fixtureEnvironment := append(receiptCanaryEnvironment(), "METASYSTEM_FAKE_PROCESS_IDENTITY_FILE="+identityTable)
	// Public admission owns the attempt, authenticated worker, input parity,
	// and atomic terminal receipt. A manually reserved parent would bind a
	// different source/configuration context from the actual testing command.
	public := exec.Command(engine, "test", "run", "--root", root, "--tree", candidate,
		"--mode", "auto", "--purpose", "delivery", "--goal", "policy-corpus", "--cap-min", "5")
	public.Dir = projectRoot
	public.Env = fixtureEnvironment
	output, runErr := public.CombinedOutput()
	if runErr != nil {
		t.Fatalf("authenticated first-transition worker did not complete all six frozen cases: %v\n%s", runErr, output)
	}
	attempts, err := proofrun.ReadAttempts(root)
	if err != nil || len(attempts) != 1 {
		t.Fatalf("first-transition reservation count=%d err=%v", len(attempts), err)
	}
	stored := attempts[0]
	if stored.Terminal == nil || stored.Terminal.Result != proofrun.TerminalSuccess ||
		stored.TestResult == nil || !stored.TestResult.Delivery.Sufficient || len(stored.TestResult.Groups) != 7 ||
		len(stored.DeliveryReceiptBytes) == 0 {
		t.Fatalf("first-transition worker result is incomplete: attempt=%+v", stored)
	}
}

func TestAmbientTrustedPolicyDecisionCannotBypassRetainedEngine(t *testing.T) {
	root := t.TempDir()
	contract := testpolicy.Contract{SchemaVersion: 1,
		ProjectRisk: testpolicy.ProjectRisk{Severity: 1, Exposure: 1, Reversibility: "revert", Detection: "immediate", Recovery: "bounded"},
		Surfaces:    []testpolicy.Surface{{ID: "app", Paths: []string{"source.txt"}, Standard: []string{"app"}, Critical: []string{"app"}}},
		Groups: []testpolicy.Group{{ID: "app", Kind: "unit", Adapter: "command", CWD: ".", Inputs: []string{"source.txt"}, Outputs: []string{"reports"},
			Obligations: []string{"app"}, Platforms: []string{"any"}, TargetMS: 1, Argv: []string{"false"}, Reports: []string{"reports"}, Format: "junit-xml",
			ExpectedTests: []testpolicy.ExpectedTest{{Report: "reports/result.xml", Classname: "app", Name: "required"}}}},
		Always: testpolicy.Always{Canary: []string{"app"}}, Unknown: []string{"app"}, Cadence: []string{"app"}}
	data, err := json.Marshal(contract)
	if err != nil {
		t.Fatal(err)
	}
	writeTestingFixtureFile(t, filepath.Join(root, "metasystem.conf"), []byte("testing.contract=testing.json\n"), 0o644)
	writeTestingFixtureFile(t, filepath.Join(root, "testing.json"), data, 0o644)
	writeTestingFixtureFile(t, filepath.Join(root, "source.txt"), []byte("source\n"), 0o644)
	testingFixtureGit(t, root, "init", "-q", "-b", "main")
	testingFixtureGit(t, root, "add", ".")
	testingFixtureGit(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "base")
	head := strings.TrimSpace(testingFixtureGit(t, root, "rev-parse", "HEAD"))
	tree := strings.TrimSpace(testingFixtureGit(t, root, "rev-parse", "HEAD^{tree}"))
	const landingRef = "refs/remotes/origin/main"
	testingFixtureGit(t, root, "update-ref", landingRef, head)
	testingFixtureGit(t, root, "config", "--local", "metasystem.steward.landing-ref", landingRef)
	t.Setenv("METASYSTEM_TRUSTED_POLICY_DECISION", "1")
	_, err = prepareTesting(testingSelectionRequest{Root: root, Tree: tree, Mode: testpolicy.ModeStandard, Purpose: testpolicy.PurposeDiagnostic})
	if err == nil || !strings.Contains(err.Error(), "TEST_POLICY_ENGINE_REQUIRED") {
		t.Fatalf("ordinary caller bypassed retained engine authentication with an ambient flag: %v", err)
	}
	engine := filepath.Join(t.TempDir(), "metasystem")
	build := exec.Command("go", "build", "-buildvcs=false", "-o", engine, ".")
	if output, buildErr := build.CombinedOutput(); buildErr != nil {
		t.Fatalf("build public flag-negative engine: %v\n%s", buildErr, output)
	}
	public := exec.Command(engine, "test", "plan", "--root", root, "--tree", tree, "--mode", "standard", "--purpose", "diagnostic")
	public.Env = append(os.Environ(), "METASYSTEM_TRUSTED_POLICY_DECISION=1")
	output, publicErr := public.CombinedOutput()
	if publicErr == nil || !strings.Contains(string(output), "TEST_POLICY_ENGINE_REQUIRED") {
		t.Fatalf("public caller bypassed retained engine authentication with an ambient flag: err=%v output=%s", publicErr, output)
	}
}

func TestTestListCheckPlanAndVerifyWithoutLaunching(t *testing.T) {
	// This package test is itself not a dispatching metasystem executable. The
	// separate retained-engine process boundary is covered by the public
	// binary fixture; this test covers zero application launches.
	root := t.TempDir()
	t.Setenv("GIT_OBJECT_DIRECTORY", filepath.Join(root, ".git", "objects"))
	t.Setenv("GIT_ALTERNATE_OBJECT_DIRECTORIES", filepath.Join(root, ".git", "objects"))
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	shPath, err := exec.LookPath("sh")
	if err != nil {
		t.Fatal(err)
	}
	tarPath, err := exec.LookPath("tar")
	if err != nil {
		t.Fatal(err)
	}
	goPath, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}
	pathOnly := filepath.Join(root, "path")
	if err := os.Mkdir(pathOnly, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, target := range map[string]string{"git": gitPath, "go": goPath, "sh": shPath, "tar": tarPath} {
		if err := os.Symlink(target, filepath.Join(pathOnly, name)); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", pathOnly)
	marker := filepath.Join(root, "test-command-launched")
	contract := testpolicy.Contract{SchemaVersion: 1,
		ProjectRisk: testpolicy.ProjectRisk{Severity: 1, Exposure: 1, Reversibility: "revert", Detection: "immediate", Recovery: "bounded"},
		Surfaces:    []testpolicy.Surface{{ID: "app", Paths: []string{"src/**"}, Standard: []string{"app-command"}, Critical: []string{"app-output"}}},
		Groups: []testpolicy.Group{{ID: "app-command", Kind: "unit", Adapter: "command", CWD: ".", Inputs: []string{"src/**"}, Outputs: []string{"reports"},
			Tools: []testpolicy.Tool{{ID: "shell", Executable: "sh", VersionArgs: []string{"-c", "printf shell-v1"}}}, Obligations: []string{"app-output"},
			Platforms: []string{"any"}, TargetMS: 1000, Argv: []string{"sh", "-c", "touch " + marker}, Reports: []string{"reports"},
			Format: "junit-xml", ExpectedTests: []testpolicy.ExpectedTest{{Report: "reports/result.xml", Classname: "app", Name: "smoke"}}}},
		Always: testpolicy.Always{Canary: []string{"app-command"}}, Unknown: []string{"app-command"}, Cadence: []string{"app-command"}}
	contractBytes, err := json.Marshal(contract)
	if err != nil {
		t.Fatal(err)
	}
	writeTestingFixtureFile(t, filepath.Join(root, "metasystem.conf"), []byte("testing.contract=testing.json\nmetasystem.runtimes=fake\n"), 0o644)
	writeTestingFixtureFile(t, filepath.Join(root, "testing.json"), contractBytes, 0o644)
	writeTestingFixtureFile(t, filepath.Join(root, "src", "output.txt"), []byte("v1\n"), 0o644)
	writeTestingFixtureFile(t, filepath.Join(root, ".gitignore"), []byte("artifacts/\n"), 0o644)
	testingFixtureGit(t, root, "init")
	testingFixtureGit(t, root, "add", ".")
	testingFixtureGit(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-m", "base")
	head, unborn, err := (gittree.Workspace{Dir: root}).HeadCommit()
	if err != nil || unborn {
		t.Fatal(err)
	}
	currentEngine := filepath.Join(t.TempDir(), "metasystem")
	build := exec.Command(goPath, "build", "-buildvcs=false", "-ldflags",
		"-X github.com/widoriezebos/agentic-tools/metasystem/internal/supervise.BuildStamp="+head, "-o", currentEngine, ".")
	build.Env = append(os.Environ(), "PATH="+os.Getenv("PATH"))
	if output, buildErr := build.CombinedOutput(); buildErr != nil {
		t.Fatalf("build source-bound policy engine: %v\n%s", buildErr, output)
	}
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(currentEngine, filepath.Join(root, "bin", "metasystem")); err != nil {
		t.Fatal(err)
	}
	digest, err := fileSHA256(currentEngine)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(steward.RepoIdentityPath(root)), 0o700); err != nil {
		t.Fatal(err)
	}
	canonicalRoot, err := canonicalProofRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	canonicalEngine, err := canonicalPath(currentEngine)
	if err != nil {
		t.Fatal(err)
	}
	writeTestingFixtureFile(t, filepath.Join(root, "plans", "goals", "peer.md"), []byte("ledger-only destination advancement\n"), 0o644)
	testingFixtureGit(t, root, "add", "plans/goals/peer.md")
	testingFixtureGit(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-m", "ledger-only destination advancement")
	recordOnlyDestination := strings.TrimSpace(testingFixtureGit(t, root, "rev-parse", "HEAD"))
	if err := steward.MintIdentity(steward.RepoIdentityPath(root), steward.InstallIdentity{RepoIdentity: canonicalRoot, Generation: 1,
		InstallPath: canonicalEngine, InstallDigest: "sha256:" + digest, MintedAt: "2026-09-09T00:00:00Z", Enrollment: steward.EnrollmentFixture,
		MintedBy: "machine-rebuild", EngineBuild: head[:12], LandedCommit: recordOnlyDestination, LandingRef: "refs/remotes/origin/main"}); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(testingFixtureGit(t, root, "rev-parse", "--verify", head+"^{commit}")); got != head {
		t.Fatalf("fixture source commit moved: got=%s want=%s", got, head)
	}
	if _, _, _, err := trustedPolicyEngine(root, recordOnlyDestination, false); err != nil {
		t.Fatalf("record-only destination advancement did not reuse the genuinely source-bound engine built at %s: %v", head, err)
	}
	tree, err := (gittree.Workspace{Dir: root}).HeadTree()
	if err != nil {
		t.Fatal(err)
	}
	if status := runTestList([]string{"--root", root, "--json"}); status != 0 {
		t.Fatalf("test list status = %d", status)
	}
	if status := runTestCheck([]string{"--root", root, "--json"}); status != 0 {
		t.Fatalf("test check status = %d", status)
	}
	if status := runTestPlan([]string{"--root", root, "--tree", tree, "--purpose", "diagnostic", "--json"}); status != 0 {
		t.Fatalf("test plan status = %d", status)
	}
	if status := runTestVerify([]string{"--root", root, "--tree", tree, "--purpose", "diagnostic", "--json"}); status != 1 {
		t.Fatalf("test verify without proof status = %d, want 1", status)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("metadata-only commands launched the declared test command: %v", err)
	}
	writeTestingFixtureFile(t, filepath.Join(root, "internal", "policy.txt"), []byte("changed engine input\n"), 0o644)
	testingFixtureGit(t, root, "add", "internal/policy.txt")
	testingFixtureGit(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-m", "change engine projection")
	changedEngineDestination := strings.TrimSpace(testingFixtureGit(t, root, "rev-parse", "HEAD"))
	if _, _, _, err := trustedPolicyEngine(root, changedEngineDestination, false); err == nil || !strings.Contains(err.Error(), "different ENGINE projections") {
		t.Fatalf("destination with changed engine inputs reused an older policy engine: %v", err)
	}
}

func writeTestingFixtureFile(t *testing.T, path string, data []byte, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, mode); err != nil {
		t.Fatal(err)
	}
}

func testingFixtureGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	command.Env = gittree.ScrubbedEnviron()
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return string(output)
}
