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
	sourceRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	// Freeze the actual source under test, whether it is committed or still
	// being edited. The executable and both commits must name these bytes.
	frozen, err := proofrun.Freeze(filepath.Join(sourceRoot, "metasystem"))
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
	budget := &goal.Budget{ElapsedLimit: "1h", AttemptLimit: 2, ReservedJobMinutesLimit: 2, ActiveJobLimit: 1, ReviewRoundLimit: 2}
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
		"--mode", "auto", "--purpose", "delivery", "--goal", "policy-corpus", "--cap-min", "1")
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
	for name, target := range map[string]string{"git": gitPath, "sh": shPath, "tar": tarPath} {
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
	if err := steward.MintIdentity(steward.RepoIdentityPath(root), steward.InstallIdentity{RepoIdentity: canonicalRoot, Generation: 1,
		InstallPath: canonicalEngine, InstallDigest: "sha256:" + digest, MintedAt: "2026-09-09T00:00:00Z", Enrollment: steward.EnrollmentFixture,
		EngineBuild: head[:12], LandedCommit: head}); err != nil {
		t.Fatal(err)
	}
	writeTestingFixtureFile(t, filepath.Join(root, "README.md"), []byte("coordination-only destination advancement\n"), 0o644)
	testingFixtureGit(t, root, "add", "README.md")
	testingFixtureGit(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-m", "record-only destination advancement")
	recordOnlyDestination := strings.TrimSpace(testingFixtureGit(t, root, "rev-parse", "HEAD"))
	if _, _, _, err := trustedPolicyEngine(root, recordOnlyDestination, false); err != nil {
		t.Fatalf("record-only destination advancement did not reuse the genuinely source-bound engine: %v", err)
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
