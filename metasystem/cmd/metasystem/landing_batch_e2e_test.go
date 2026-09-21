package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	goalbranch "github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/hostload"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

var batchE2EProcessEnvironment sync.Mutex
var batchE2ESharedEngine = &batchE2EEngine{path: filepath.Join(os.TempDir(), "metasystem-batch-e2e-"+strconv.Itoa(os.Getpid()))}

type batchE2EEngine struct {
	sync.Mutex
	path, commit string
	users        int
}

type batchE2EHarness struct {
	sync.Mutex
	fixtures map[string]*batchE2EFixture
}

func (harness *batchE2EHarness) fixture(root string) *batchE2EFixture {
	harness.Lock()
	defer harness.Unlock()
	return harness.fixtures[root]
}

type batchE2EFixture struct {
	t               *testing.T
	origin, landing string
	engine          string
	seats           map[string]string
	now             time.Time
	proofCalls      []batchProofLaunch
	proof           func(batchProofLaunch, int) proofrun.TestResult
	diagnostic      func(batch.DiagnosticRequest) batch.DiagnosticResult
	planObserve     func()
}

func TestBatchLandingLifecycleEndToEnd(t *testing.T) {
	retainBatchE2ESharedEngine(t)
	t.Parallel()
	batchE2EProcessEnvironment.Lock()
	t.Cleanup(batchE2EProcessEnvironment.Unlock)
	engine := batchE2ESharedEngine
	harness := &batchE2EHarness{fixtures: map[string]*batchE2EFixture{}}
	// The real prefix runner bounds its native phase by the reserved attempt
	// deadline. Seed the fixture from the same current clock used by that runner.
	now := time.Now().UTC().Truncate(time.Second)
	originalProof, originalDiagnostic := productionBatchProofDependencies, batchDiagnosticLauncher
	originalOwnerConstruct := batchOwnerConstruct
	batchOwnerConstruct = func(settings config.BatchLanding, held batchOwnerLease, inputs productionBatchOwnerInputs, clock func() time.Time) (*batch.Owner, error) {
		inputs.sample = func() proofrun.LoadSample {
			return proofrun.LoadSample{Sample: hostload.Sample{At: clock().UTC().Format(time.RFC3339Nano), Available: true, Cores: 18}, OverlapKnown: true}
		}
		inputs.lockDir = filepath.Join(settings.Root, "artifacts", "agents", "landing-batches", "fixture-proof-lock")
		inputs.queueDir = filepath.Join(settings.Root, "artifacts", "agents", "landing-batches", "fixture-proof-queue")
		return originalOwnerConstruct(settings, held, inputs, clock)
	}
	originalWait, originalStatus := batchWaitClock, batchStatusNow
	originalPrefix := batchPrefixReceiptExecutable
	originalPrefixVerify := batchVerifyPrefixEvidence
	originalGoalNow, originalCommandHelper, originalLineage := os.Getenv("METASYSTEM_GOAL_NOW"), os.Getenv("GO_WANT_BATCH_E2E_COMMAND"), os.Getenv("METASYSTEM_OWNER_LINEAGE")
	originalAdmissionDir, originalAdmissionRoot := os.Getenv("METASYSTEM_PROOF_ADMISSION_TEST_DIR"), os.Getenv("METASYSTEM_PROOF_ADMISSION_FIXTURE_ROOT")
	productionBatchProofDependencies = originalProof
	productionBatchProofDependencies.plan = func(root, goalID, tree string, mode testpolicy.Mode) (testpolicy.Plan, error) {
		if fixture := harness.fixture(root); fixture != nil && fixture.planObserve != nil {
			fixture.planObserve()
		}
		return originalProof.plan(root, goalID, tree, mode)
	}
	productionBatchProofDependencies.launch = func(request batchProofLaunch) (proofrun.TestResult, error) {
		fixture := harness.fixture(request.Root)
		fixture.proofCalls = append(fixture.proofCalls, request)
		if fixture.proof != nil {
			return fixture.proof(request, len(fixture.proofCalls)), nil
		}
		return batchE2EProofResult(request, true, nil), nil
	}
	batchDiagnosticLauncher = func(root, _ string, request batch.DiagnosticRequest) (batch.DiagnosticResult, error) {
		fixture := harness.fixture(root)
		if fixture.diagnostic != nil {
			return fixture.diagnostic(request), nil
		}
		return batch.DiagnosticResult{AttemptID: "diagnostic-green"}, nil
	}
	waitNow := now
	batchWaitClock = batch.WaitClock{Now: func() time.Time { return waitNow }, After: func(duration time.Duration) <-chan time.Time {
		waitNow = waitNow.Add(duration)
		ch := make(chan time.Time, 1)
		ch <- waitNow
		return ch
	}}
	batchStatusNow = func() time.Time { return now }
	batchPrefixReceiptExecutable = func() (string, error) { return engine.path, nil }
	batchVerifyPrefixEvidence = func(_ string, _ batch.Unit, _ string, _ batch.PrefixDecision) error { return nil }
	_ = os.Setenv("METASYSTEM_GOAL_NOW", now.Format(time.RFC3339))
	_ = os.Setenv("GO_WANT_BATCH_E2E_COMMAND", "1")
	_ = os.Setenv("METASYSTEM_OWNER_LINEAGE", "lineage-goal-b")
	signal.Ignore(syscall.SIGUSR1)
	t.Cleanup(func() {
		batchOwnerConstruct = originalOwnerConstruct
		productionBatchProofDependencies, batchDiagnosticLauncher = originalProof, originalDiagnostic
		batchWaitClock, batchStatusNow, batchPrefixReceiptExecutable, batchVerifyPrefixEvidence = originalWait, originalStatus, originalPrefix, originalPrefixVerify
		_ = os.Setenv("METASYSTEM_GOAL_NOW", originalGoalNow)
		_ = os.Setenv("GO_WANT_BATCH_E2E_COMMAND", originalCommandHelper)
		_ = os.Setenv("METASYSTEM_OWNER_LINEAGE", originalLineage)
		_ = os.Setenv("METASYSTEM_PROOF_ADMISSION_TEST_DIR", originalAdmissionDir)
		_ = os.Setenv("METASYSTEM_PROOF_ADMISSION_FIXTURE_ROOT", originalAdmissionRoot)
		signal.Reset(syscall.SIGUSR1)
	})
	observed, statusFailures := map[string]bool{}, []string{}

	t.Run("two-green", func(t *testing.T) {
		fixture := newBatchE2EFixture(t, harness, engine, now, "goal-a", "goal-b")
		batchID := fixture.join("goal-a")
		fixture.joinInto(batchID, "goal-b")
		observe := func(state string) {
			if observed[state] {
				return
			}
			observed[state] = true
			if err := fixture.statusAndWait(batchID, state); err != nil {
				statusFailures = append(statusFailures, err.Error())
			}
		}
		observe(batch.StateOpen)
		fixture.planObserve = func() { observe(batch.StateSealed) }
		fixture.proof = func(request batchProofLaunch, _ int) proofrun.TestResult {
			observe(batch.StateProving)
			return batchE2EProofResult(request, true, nil)
		}
		fixture.tick(batchID)
		fixture.assertState(batchID, batch.StateLanding)
		observe(batch.StateLanding)
		if len(fixture.proofCalls) != 1 || fixture.proofCalls[0].Tree != fixture.load(batchID).TipTree {
			t.Fatalf("tip proof calls = %+v, want one call for the union tree", fixture.proofCalls)
		}
		fixture.tick(batchID)
		fixture.assertState(batchID, batch.StateLanded)
		observe(batch.StateLanded)
		fixture.assertLandedUnits("goal-a", "goal-b")
		fixture.assertRemoteBranch("goal-a", false)
		fixture.assertRemoteBranch("goal-b", false)
	})

	t.Run("eject-red", func(t *testing.T) {
		fixture := newBatchE2EFixture(t, harness, engine, now, "goal-a", "goal-b", "goal-c")
		fixture.proof = func(request batchProofLaunch, call int) proofrun.TestResult {
			if call == 1 {
				return batchE2EProofResult(request, false, []string{"units/goal-b.txt"})
			}
			return batchE2EProofResult(request, true, nil)
		}
		fixture.diagnostic = func(request batch.DiagnosticRequest) batch.DiagnosticResult {
			if fixture.treeHas(request.Tree, "units/goal-b.txt") {
				return batch.DiagnosticResult{AttemptID: "diagnose-goal-b", Groups: []batch.RedGroup{{ID: "app-unit", Status: "failed", Failures: []batch.Failure{{Name: "TestGoalB", Status: "failed"}}}}}
			}
			return batch.DiagnosticResult{AttemptID: "diagnose-green"}
		}
		batchID := fixture.join("goal-a")
		fixture.joinInto(batchID, "goal-b")
		fixture.joinInto(batchID, "goal-c")
		ejectedTip := fixture.remoteBranchTip("goal-b")
		fixture.tick(batchID)
		fixture.assertState(batchID, batch.StateDiagnosing)
		fixture.tick(batchID)
		record := fixture.load(batchID)
		unit := batchE2EUnit(record, "goal-b")
		if unit.State != batch.UnitReturnPending || unit.Outcome != batch.UnitEjected || !strings.Contains(unit.Failure, "TestGoalB") {
			t.Fatalf("diagnosed unit = %+v", unit)
		}
		fixture.tick(batchID)
		fixture.assertState(batchID, batch.StateLanding)
		fixture.tick(batchID)
		fixture.assertState(batchID, batch.StateLanded)
		if len(fixture.proofCalls) != 2 {
			t.Fatalf("tip proof calls = %d, want red union and green survivors", len(fixture.proofCalls))
		}
		fixture.assertLandedUnits("goal-a", "goal-c")
		if fixture.originHas("units/goal-b.txt") {
			t.Fatal("the ejected unit reached origin/main")
		}
		fixture.assertRemoteBranch("goal-b", true)
		if got := fixture.remoteBranchTip("goal-b"); got != ejectedTip {
			t.Fatalf("ejected branch tip=%s want untouched %s", got, ejectedTip)
		}
	})

	t.Run("trunk-moved", func(t *testing.T) {
		fixture := newBatchE2EFixture(t, harness, engine, now, "goal-a", "goal-b")
		fixture.proof = func(request batchProofLaunch, _ int) proofrun.TestResult {
			return batchE2EProofResult(request, true, []string{"app/**"})
		}
		batchID := fixture.join("goal-a")
		fixture.joinInto(batchID, "goal-b")
		fixture.tick(batchID)
		proved := fixture.load(batchID)
		fixture.moveTrunk("app/trunk.txt")
		movedTip := fixture.originTip()
		fixture.tick(batchID)
		reopened := fixture.load(batchID)
		if reopened.State != batch.StateOpen || reopened.Proof != nil || fixture.originTip() != movedTip {
			t.Fatalf("moved trunk result = state %s proof=%+v origin=%s want open, nil, %s", reopened.State, reopened.Proof, fixture.originTip(), movedTip)
		}
		if proved.Proof == nil || proved.Proof.BaseCommit == movedTip {
			t.Fatalf("first proof did not bind the old base: %+v", proved.Proof)
		}
		fixture.tick(batchID)
		fixture.assertState(batchID, batch.StateLanding)
		fixture.tick(batchID)
		fixture.assertState(batchID, batch.StateLanded)
		if len(fixture.proofCalls) != 2 || fixture.proofCalls[0].Tree == fixture.proofCalls[1].Tree {
			t.Fatalf("proof calls did not rebind to the moved base: %+v", fixture.proofCalls)
		}
		fixture.assertLandedUnits("goal-a", "goal-b")
	})

	t.Run("withdraw", func(t *testing.T) {
		joinedAt := time.Now().UTC().Truncate(time.Second).Add(-2 * time.Minute)
		if err := os.Setenv("METASYSTEM_GOAL_NOW", joinedAt.Format(time.RFC3339)); err != nil {
			t.Fatal(err)
		}
		fixture := newBatchE2EFixture(t, harness, engine, now, "goal-a", "goal-b")
		batchID := fixture.join("goal-a")
		fixture.joinInto(batchID, "goal-b")
		withdrawnTip := fixture.remoteBranchTip("goal-b")
		joined := batchE2EUnit(fixture.load(batchID), "goal-b")
		seatRoot := joined.SeatRoot
		if err := os.Setenv("METASYSTEM_OWNER_LINEAGE", joined.Claim.Lineage); err != nil {
			t.Fatal(err)
		}
		code, _, stderr := captureCommandOutput(t, true, true, func() int {
			return runLandingBatch([]string{"withdraw", "--root", seatRoot, "--goal", "goal-b"})
		})
		if code != 0 {
			t.Fatalf("withdraw code=%d stderr=%q", code, stderr)
		}
		fixture.tick(batchID)
		withdrawn := batchE2EUnit(fixture.load(batchID), "goal-b")
		if withdrawn.State != batch.UnitWithdrawn || withdrawn.Outcome != batch.UnitWithdrawn {
			t.Fatalf("withdrawn unit = %+v", withdrawn)
		}
		fixture.assertState(batchID, batch.StateOpen)
		if err := os.Setenv("METASYSTEM_GOAL_NOW", joinedAt.Add(2*time.Minute).Format(time.RFC3339)); err != nil {
			t.Fatal(err)
		}
		fixture.tick(batchID)
		fixture.assertState(batchID, batch.StateLanding)
		if proof := fixture.load(batchID).Proof; proof == nil || proof.Window != "expired" {
			t.Fatalf("remaining unit proof window = %+v, want expired", proof)
		}
		fixture.tick(batchID)
		fixture.assertState(batchID, batch.StateLanded)
		fixture.assertLandedUnits("goal-a")
		if fixture.originHas("units/goal-b.txt") {
			t.Fatal("the withdrawn unit reached origin/main")
		}
		fixture.assertRemoteBranch("goal-b", true)
		if got := fixture.remoteBranchTip("goal-b"); got != withdrawnTip {
			t.Fatalf("withdrawn branch tip=%s want untouched %s", got, withdrawnTip)
		}
	})

	t.Run("status-and-wait", func(t *testing.T) {
		for _, state := range []string{batch.StateOpen, batch.StateSealed, batch.StateProving, batch.StateLanding, batch.StateLanded} {
			if !observed[state] {
				t.Fatalf("status and wait did not observe %s", state)
			}
		}
		if len(statusFailures) != 0 {
			t.Fatalf("status and wait failures: %s", strings.Join(statusFailures, "; "))
		}
	})
}

func retainBatchE2ESharedEngine(t *testing.T) {
	t.Helper()
	batchE2ESharedEngine.Lock()
	batchE2ESharedEngine.users++
	batchE2ESharedEngine.Unlock()
	t.Cleanup(func() {
		batchE2ESharedEngine.Lock()
		defer batchE2ESharedEngine.Unlock()
		batchE2ESharedEngine.users--
		if batchE2ESharedEngine.users != 0 {
			return
		}
		if err := os.Remove(batchE2ESharedEngine.path); err != nil && !os.IsNotExist(err) {
			t.Errorf("remove shared batch lifecycle engine: %v", err)
		}
		batchE2ESharedEngine.commit = ""
	})
}

func newBatchE2EFixture(t *testing.T, harness *batchE2EHarness, engine *batchE2EEngine, now time.Time, goals ...string) *batchE2EFixture {
	t.Helper()
	fixture := &batchE2EFixture{t: t, seats: map[string]string{}, now: now}
	seed := filepath.Join(t.TempDir(), "seed")
	fixture.origin = filepath.Join(t.TempDir(), "origin.git")
	batchE2EGit(t, "", "init", "-q", "--bare", fixture.origin)
	batchE2EGit(t, "", "init", "-q", "-b", "main", seed)
	batchE2EConfigureGit(t, seed, "seed")
	fixture.writeSeed(seed, []string{"goal-a", "goal-b", "goal-c"})
	batchE2EGit(t, seed, "add", ".")
	batchE2EGitAt(t, seed, "commit", "-qm", "seed batch lifecycle")
	batchE2EGit(t, seed, "remote", "add", "origin", fixture.origin)
	batchE2EGit(t, seed, "push", "-q", "origin", "main")
	batchE2EGit(t, fixture.origin, "symbolic-ref", "HEAD", "refs/heads/main")

	fixture.landing = filepath.Join(t.TempDir(), "landing")
	fixture.clone(fixture.landing, "landing")
	if err := os.Setenv("METASYSTEM_PROOF_ADMISSION_TEST_DIR", filepath.Join(t.TempDir(), "host-admission")); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("METASYSTEM_PROOF_ADMISSION_FIXTURE_ROOT", fixture.landing); err != nil {
		t.Fatal(err)
	}
	fixture.enrollPolicyEngine(batchE2EGit(t, fixture.landing, "rev-parse", "HEAD"), engine)
	fixture.holdLanding()
	for _, goalID := range goals {
		root := filepath.Join(t.TempDir(), goalID)
		fixture.clone(root, goalID)
		fixture.seats[goalID] = root
		fixture.announce(root, "lineage-"+goalID)
		fixture.addGoalBranch(root, goalID)
	}
	harness.Lock()
	harness.fixtures[fixture.landing] = fixture
	if canonical, err := filepath.EvalSymlinks(fixture.landing); err == nil {
		harness.fixtures[canonical] = fixture
	}
	harness.Unlock()
	return fixture
}

func (fixture *batchE2EFixture) writeSeed(root string, goals []string) {
	t := fixture.t
	write := func(path, body string, mode os.FileMode) {
		path = filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), mode); err != nil {
			t.Fatal(err)
		}
	}
	contract := testpolicy.Contract{SchemaVersion: 1,
		ProjectRisk: testpolicy.ProjectRisk{Severity: 1, Exposure: 1, Reversibility: "revert", Detection: "immediate", Recovery: "bounded"},
		Surfaces:    []testpolicy.Surface{{ID: "app", Paths: []string{"units/**", "records/**", "plans/goals/**", "memory/**", "metasystem/**"}, Standard: []string{"app-unit"}, Critical: []string{"batch-lifecycle"}}},
		Groups: []testpolicy.Group{{ID: "app-unit", Kind: "unit", Adapter: "command", CWD: ".", Inputs: []string{"go.mod", "app/**", "units/**", "metasystem/**", "scripts/agents/e2e-proof.sh"}, Outputs: []string{"reports"}, Tools: []testpolicy.Tool{},
			Obligations: []string{"batch-lifecycle"}, Platforms: []string{"any"}, TargetMS: 1000,
			Argv: []string{"./scripts/agents/e2e-proof.sh"}, Reports: []string{"reports"}, Format: "junit-xml",
			ExpectedTests: []testpolicy.ExpectedTest{{Report: "reports/app.xml", Classname: "batch", Name: "healthy"}}}},
		Always: testpolicy.Always{Canary: []string{"app-unit"}, Standard: []string{}}, Unknown: []string{"app-unit"}, Cadence: []string{"app-unit"}}
	contractData, err := json.MarshalIndent(contract, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	write("go.mod", "module batchfixture\n\ngo 1.25\n", 0o644)
	write("go.sum", "", 0o644)
	write("app/app.go", "package app\n\nfunc Healthy() bool { return true }\n", 0o644)
	write("app/app_test.go", "package app\n\nimport \"testing\"\n\nfunc TestSmoke(t *testing.T) { if !Healthy() { t.Fatal(\"unhealthy\") } }\n", 0o644)
	write("testing.json", string(contractData)+"\n", 0o644)
	write("scripts/agents/fixture-bed-groups.tsv", "", 0o644)
	write("scripts/agents/go-gate.sh", "#!/usr/bin/env bash\nset -euo pipefail\ngo test ./...\n", 0o755)
	write("scripts/agents/e2e-proof.sh", "#!/usr/bin/env bash\nset -euo pipefail\ngrep -q 'func Healthy' app/app.go\nmkdir -p reports\nprintf '%s\\n' '<testsuite><testcase classname=\"batch\" name=\"healthy\"/></testsuite>' > reports/app.xml\n", 0o755)
	write("scripts/agents/pre-commit-guard.sh", "#!/usr/bin/env bash\nexit 0\n", 0o755)
	engine, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	write("scripts/agents/go-build.sh", fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
if (( $# == 0 )); then
  mkdir -p bin
  out=bin/metasystem
	printf '#!/usr/bin/env bash\nif [[ "${1:-}" == up ]]; then exit 0; fi\nexport GO_WANT_BATCH_E2E_COMMAND=1\nexec "%%s" "$@"\n' %s >"$out"
else
  [[ "$1" == --trimpath && "$2" == --out && -n "${3:-}" ]]
  out=$3
	printf '#!/usr/bin/env bash\n# candidate commit %%s\nexport GO_WANT_BATCH_E2E_COMMAND=1\nexec "%%s" "$@"\n' "$METASYSTEM_BUILD_STAMP" %s >"$out"
fi
chmod +x "$out"
`, strconv.Quote(engine), strconv.Quote(engine)), 0o755)
	write("scripts/receipt.sh", "#!/usr/bin/env bash\nset -euo pipefail\nmkdir -p memory\nprintf '%s\\n' \"$*\" >> memory/receipts.log\ngit add memory/receipts.log\n", 0o755)
	write("scripts/agents/commit.sh", batchE2ECommitScript, 0o755)
	write("metasystem.conf", "metasystem.runtimes=fake\ntesting.contract=testing.json\ngoal.human.wido=Wido Example <wido@example.invalid>\nproof.admission.top-level-max=0\n", 0o644)
	write(".gitignore", "artifacts/\nbin/\nmetasystem.conf.local\n", 0o644)
	rootRecord := &goal.RootRecord{Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "1", SyncMode: goal.SyncRemote, Revision: 1}
	write("plans/goals/backlog.md", string(goal.RenderRoot(rootRecord)), 0o644)
	for index, goalID := range goals {
		intent := "Land the batch lifecycle unit for " + goalID + "."
		risk := &goal.RiskRecord{Severity: 1, Novelty: 1, Exposure: 1, Accumulation: 1, Basis: "The fixture changes one isolated text file."}
		budget := &goal.Budget{ElapsedLimit: "4h", AttemptLimit: 4, ReservedJobMinutesLimit: 1000, ActiveJobLimit: 1, ReviewRoundLimit: 2}
		opened := fixture.now.Add(-10 * time.Minute).Format(time.RFC3339)
		claimed := fixture.now.Add(-5 * time.Minute).Format(time.RFC3339)
		approved := fixture.now.Add(-4 * time.Minute).Format(time.RFC3339)
		approvalOpid := batchE2EOpid(index+8, "human", "terminal")
		file := &goal.GoalFile{Id: goalID, State: goal.StateClaimed, Tier: 1, Risk: risk, Intent: intent, Origin: goal.OriginMain,
			NextStep: "Land the certified unit.", OpenedAt: opened, Revision: 3, Budget: budget,
			Approved:       &goal.ApprovalRecord{By: "human:wido", At: approved, Revision: 3, Opid: approvalOpid, Authority: goal.ApprovalAuthorityProven, Digest: goal.ApprovalDigest(intent, 1, *budget, risk)},
			Claimed:        &goal.ClaimRecord{Machine: goalID, Lineage: "lineage-" + goalID, At: claimed, Revision: 2, AccountingRevision: 2},
			StopCapability: &goal.StopCapability{Generation: 2, Revision: 2, Machine: goalID, ClaimEpoch: 1},
			History: []goal.HistoryLine{{At: opened, Opid: batchE2EOpid(index, "human", "terminal"), Verb: "open", Actor: "human:wido", Keep: -1},
				{At: claimed, Opid: batchE2EOpid(index, goalID, "lineage-"+goalID), Verb: "claim", Actor: goalID + "+lineage-" + goalID, Keep: -1},
				{At: approved, Opid: approvalOpid, Verb: "approve", Actor: "human:wido", Keep: -1}}}
		write("plans/goals/"+goalID+".md", string(goal.RenderFile(file)), 0o644)
	}
}

func (fixture *batchE2EFixture) clone(root, machine string) {
	batchE2EGit(fixture.t, "", "clone", "-q", fixture.origin, root)
	batchE2EConfigureGit(fixture.t, root, machine)
	batchE2EGit(fixture.t, root, "config", "metasystem.goal.machine", machine)
	batchE2EGit(fixture.t, root, "config", "goal.sync-remote", "origin")
	batchE2EGit(fixture.t, root, "config", "goal.sync-branch", "refs/heads/main")
	batchE2EGit(fixture.t, root, "config", "metasystem.steward.landing-ref", "refs/remotes/origin/main")
	batchE2EGit(fixture.t, root, "update-ref", goal.AcceptedRef, "origin/main")
	conf := filepath.Join(root, "metasystem.conf.local")
	if err := os.WriteFile(conf, []byte("landing.batch-root="+fixture.landing+"\nlanding.batch-max-wait=1m\n"), 0o644); err != nil {
		fixture.t.Fatal(err)
	}
}

func (fixture *batchE2EFixture) enrollPolicyEngine(commit string, shared *batchE2EEngine) {
	fixture.t.Helper()
	shared.Lock()
	defer shared.Unlock()
	fixture.engine = shared.path
	if shared.commit != "" && shared.commit != commit {
		fixture.t.Fatalf("shared fixture engine commit=%s want %s", shared.commit, commit)
	}
	if shared.commit == "" {
		shared.commit = commit
		fixture.buildPolicyEngine(commit, shared.path)
	}
	canonicalRoot, err := canonicalProofRoot(fixture.landing)
	if err != nil {
		fixture.t.Fatal(err)
	}
	canonicalEngine, err := canonicalPath(shared.path)
	if err != nil {
		fixture.t.Fatal(err)
	}
	digest, err := fileSHA256(canonicalEngine)
	if err != nil {
		fixture.t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(steward.RepoIdentityPath(canonicalRoot)), 0o700); err != nil {
		fixture.t.Fatal(err)
	}
	if err := steward.MintIdentity(steward.RepoIdentityPath(canonicalRoot), steward.InstallIdentity{
		RepoIdentity: canonicalRoot, Generation: 1, InstallPath: canonicalEngine,
		InstallDigest: "sha256:" + digest, MintedAt: fixture.now.Format(time.RFC3339),
		Enrollment: steward.EnrollmentFixture, EngineBuild: commit,
	}); err != nil {
		fixture.t.Fatal(err)
	}
}

func (fixture *batchE2EFixture) buildPolicyEngine(commit, engine string) {
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		fixture.t.Fatal("locate metasystem source")
	}
	sourceRoot := filepath.Clean(filepath.Join(filepath.Dir(source), "..", ".."))
	linker := "-X github.com/widoriezebos/agentic-tools/metasystem/internal/supervise.BuildStamp=" + commit
	command := exec.Command("go", "build", "-buildvcs=false", "-ldflags", linker, "-o", engine, "./cmd/metasystem")
	command.Dir = sourceRoot
	command.Env = os.Environ()
	if output, buildErr := command.CombinedOutput(); buildErr != nil {
		fixture.t.Fatalf("build enrolled fixture engine: %v: %s", buildErr, output)
	}
}

func (fixture *batchE2EFixture) announce(root, lineage string) {
	exact, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		fixture.t.Fatalf("probe fixture process: state=%s err=%v", state, err)
	}
	if _, err := lease.AnnounceWithPair(root, "session-"+lineage, int64(os.Getpid()), exact.StartedAt.Unix(), exact.StartTicks, exact.BootID, "batch-e2e", "metasystem", lineage); err != nil {
		fixture.t.Fatal(err)
	}
}

func (fixture *batchE2EFixture) holdLanding() {
	fixture.t.Helper()
	fixture.announce(fixture.landing, landingOwnerLineage)
	holder, err := lease.RequireHolder(fixture.landing, int64(os.Getpid()), nil)
	if err != nil || !holder.Holder || holder.ClaimEpoch == nil {
		fixture.t.Fatalf("hold landing checkout: holder=%+v err=%v", holder, err)
	}
}

func (fixture *batchE2EFixture) addGoalBranch(root, goalID string) {
	t := fixture.t
	base := batchE2EGit(t, root, "rev-parse", "origin/main")
	path := "units/" + goalID + ".txt"
	if err := os.MkdirAll(filepath.Join(root, "units"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, path), []byte(goalID+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	batchE2EGit(t, root, "add", "--", path)
	commit, err := goalbranch.CommitStaged(goalbranch.CommitRequest{Repo: root, Remote: "origin", EndpointTip: base, GoalID: goalID, Unit: "u1", OpID: "build-" + goalID, Kind: goalbranch.Unit, CheckClaim: func() error { return nil }})
	if err != nil {
		t.Fatal(err)
	}
	subject, present, err := dispatchcore.ComputeReadSubject(dispatchcore.ReadSubjectRequest{RepoRoot: root, Role: "code-critic", Reviews: "commit:" + commit})
	if err != nil || !present {
		t.Fatalf("compute %s read subject: present=%t err=%v", goalID, present, err)
	}
	job := "critic-" + goalID
	fixture.writeJSON(filepath.Join(root, "artifacts", "agents", "jobs", job+".json"), map[string]any{"jobId": job, "operationId": job + "-reservation", "goalId": goalID, "goalRevision": 2, "capMin": 1, "role": "code-critic", "parentJob": nil, "reviewChainCounted": true, "round": 1, "status": "completed", "chainClosed": true, "findingRegister": []any{}, "findingRegisterRound": 1, "findingRegisterSubjectDigest": subject.Digest(), "closure": map[string]any{"criticRoot": job, "round": 1, "subject": subject, "mechanism": "clean"}})
	fixture.writeJSON(filepath.Join(root, "artifacts", "agents", job, "rounds", "1", "subject.json"), subject)
	fixture.writeJSON(filepath.Join(root, "artifacts", "agents", job, "rounds", "1", "return.json"), map[string]any{"jobId": job, "round": 1, "reviewedTree": subject.Tree})
	if _, _, err := goalbranch.CommitRead(goalbranch.CommitReadRequest{Repo: root, Remote: "origin", EndpointTip: base, GoalID: goalID, Unit: "u1", OpID: "read-" + goalID, RootJob: job, GateRunID: "fast-" + goalID, GateTree: subject.Tree, CheckClaim: func() error { return nil }}); err != nil {
		t.Fatal(err)
	}
	batchE2EGit(t, root, "push", "-q", "origin", "refs/heads/goal/"+goalID+":refs/heads/goal/"+goalID)
}

func (fixture *batchE2EFixture) writeJSON(path string, value any) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		fixture.t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		fixture.t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		fixture.t.Fatal(err)
	}
}

func batchE2EProofResult(request batchProofLaunch, green bool, manifest []string) proofrun.TestResult {
	admissionMaximum := 0
	status := "passed"
	if !green {
		status = "failed"
	}
	base, _ := exec.Command("git", "-C", request.Root, "rev-parse", "refs/remotes/origin/main").Output()
	result := proofrun.TestResult{SchemaVersion: proofrun.TestResultSchemaVersion, AttemptID: fmt.Sprintf("batch-e2e-%s", request.BatchID), Purpose: testpolicy.PurposeDelivery,
		WorkerPolicyVersion: proofrun.TestWorkerPolicyVersion, Workers: 1, AdmissionMaximum: &admissionMaximum,
		RequestedMode: request.Mode, RequiredMode: request.Mode, ExecutedMode: request.Mode, ProjectRoot: request.Root,
		BaseCommit: strings.TrimSpace(string(base)), CandidateTree: request.Tree,
		RequiredGroups: []string{"app-unit"}, SelectedGroups: []string{"app-unit"},
		Groups:       []proofrun.GroupResult{{ID: "app-unit", Kind: "unit", Obligations: []string{"batch-lifecycle"}, InputManifest: slices.Clone(manifest), IdentityVersion: proofrun.GroupExecutionIdentityVersion, Status: status, NativeLaunched: true, CollectionComplete: true}},
		LaunchCounts: proofrun.LaunchCounts{Test: 1, CountsComplete: true}}
	result.RecomputeDelivery()
	return result
}

func (fixture *batchE2EFixture) join(goalID string) string {
	code, stdout, stderr := captureCommandOutput(fixture.t, true, true, func() int {
		return runLandingBatch([]string{"join", "--root", fixture.seats[goalID], "--goal", goalID, "--last"})
	})
	if code != 0 {
		fixture.t.Fatalf("join %s code=%d stderr=%q", goalID, code, stderr)
	}
	var result struct {
		BatchID string `json:"batchId"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil || result.BatchID == "" {
		fixture.t.Fatalf("join %s output=%q err=%v", goalID, stdout, err)
	}
	return result.BatchID
}

func (fixture *batchE2EFixture) joinInto(batchID, goalID string) {
	if got := fixture.join(goalID); got != batchID {
		fixture.t.Fatalf("join %s batch=%s want %s", goalID, got, batchID)
	}
}

func (fixture *batchE2EFixture) tick(batchID string) {
	code := runLandingBatch([]string{"tick", "--root", fixture.seats[firstSeat(fixture.seats)], "--landing-root", fixture.landing, "--max-wait", "1m", "--batch", batchID})
	if code != 0 {
		record, _ := batch.NewStore(fixture.landing, nil).Load(batchID)
		status, _ := exec.Command("git", "-C", fixture.landing, "status", "--short", "--branch").CombinedOutput()
		log, _ := exec.Command("git", "--git-dir", fixture.origin, "log", "-3", "--format=%H%n%B%n--sources=%(trailers:key=Goal-Source,valueonly)", "refs/heads/main").CombinedOutput()
		results, _ := filepath.Glob(filepath.Join(fixture.landing, "artifacts", "agents", "proof-runs", "batch", batchID+"-prefix-*.json"))
		var evidence strings.Builder
		for _, path := range results {
			data, _ := os.ReadFile(path)
			fmt.Fprintf(&evidence, "%s=%s\n", filepath.Base(path), data)
		}
		_ = filepath.WalkDir(filepath.Join(fixture.landing, "artifacts", "agents", "proof-runs"), func(path string, entry os.DirEntry, err error) error {
			if err == nil && !entry.IsDir() && (strings.HasSuffix(path, "launcher.log") || strings.HasSuffix(path, "attempt.json")) {
				data, _ := os.ReadFile(path)
				fmt.Fprintf(&evidence, "%s=%s\n", path, data)
			}
			return nil
		})
		fixture.t.Fatalf("tick %s failed: state=%s landing=%+v git=%s log=%s results=%s", batchID, record.State, record.Landing, status, log, evidence.String())
	}
}

func firstSeat(seats map[string]string) string {
	ids := make([]string, 0, len(seats))
	for id := range seats {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids[0]
}

func (fixture *batchE2EFixture) load(batchID string) batch.Record {
	record, err := batch.NewStore(fixture.landing, nil).Load(batchID)
	if err != nil {
		fixture.t.Fatal(err)
	}
	return record
}

func (fixture *batchE2EFixture) assertState(batchID, want string) {
	fixture.t.Helper()
	if record := fixture.load(batchID); record.State != want {
		fixture.t.Fatalf("batch %s state=%s want %s; proof=%+v; units=%+v; history=%+v", batchID, record.State, want, record.Proof, record.Units, record.History)
	}
}

func (fixture *batchE2EFixture) statusAndWait(batchID, want string) error {
	root := fixture.seats[firstSeat(fixture.seats)]
	code, stdout, stderr := captureCommandOutput(fixture.t, true, true, func() int {
		return runLandingBatch([]string{"status", "--root", root, "--batch", batchID})
	})
	if code != 0 {
		return fmt.Errorf("status %s: %s", want, stderr)
	}
	var status batchStatusOutput
	if err := json.Unmarshal([]byte(stdout), &status); err != nil || len(status.Batches) != 1 || status.Batches[0].State != want {
		return fmt.Errorf("status at %s = %q err=%v parsed=%+v", want, stdout, err, status)
	}
	code, stdout, stderr = captureCommandOutput(fixture.t, true, true, func() int {
		return runLandingBatch([]string{"wait", "--root", root, "--batch", batchID, "--bound", "1s"})
	})
	if want == batch.StateLanded {
		var view batchStatusView
		if code != 0 || json.Unmarshal([]byte(stdout), &view) != nil || view.State != want {
			return fmt.Errorf("terminal wait at %s code=%d stdout=%q stderr=%q", want, code, stdout, stderr)
		}
	} else if code == 0 || !strings.Contains(stderr, want) {
		return fmt.Errorf("bounded wait at %s code=%d stdout=%q stderr=%q", want, code, stdout, stderr)
	}
	return nil
}

func (fixture *batchE2EFixture) treeHas(tree, path string) bool {
	command := exec.Command("git", "-C", fixture.landing, "cat-file", "-e", tree+":"+path)
	return command.Run() == nil
}

func (fixture *batchE2EFixture) originHas(path string) bool {
	command := exec.Command("git", "--git-dir", fixture.origin, "cat-file", "-e", "refs/heads/main:"+path)
	return command.Run() == nil
}

func (fixture *batchE2EFixture) originTip() string {
	return batchE2EGit(fixture.t, fixture.origin, "rev-parse", "refs/heads/main")
}

func (fixture *batchE2EFixture) assertLandedUnits(want ...string) {
	out := batchE2EGit(fixture.t, fixture.origin, "log", "--first-parent", "--format=%(trailers:key=Goal-Unit,valueonly)", "refs/heads/main")
	var got []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			goalID, _, _ := strings.Cut(line, "/")
			got = append(got, goalID)
		}
	}
	slices.Reverse(got)
	if !slices.Equal(got, want) {
		fixture.t.Fatalf("landed unit order=%v want %v", got, want)
	}
	for _, goalID := range want {
		if !fixture.originHas("units/" + goalID + ".txt") {
			fixture.t.Fatalf("origin lacks %s unit", goalID)
		}
	}
	receipts := batchE2EGit(fixture.t, fixture.origin, "show", "refs/heads/main:memory/receipts.log")
	for _, goalID := range want {
		if !strings.Contains(receipts, "--goal "+goalID+" ") {
			fixture.t.Fatalf("receipt log lacks %s: %s", goalID, receipts)
		}
	}
}

func (fixture *batchE2EFixture) assertRemoteBranch(goalID string, present bool) {
	err := exec.Command("git", "--git-dir", fixture.origin, "show-ref", "--verify", "--quiet", "refs/heads/goal/"+goalID).Run()
	if (err == nil) != present {
		fixture.t.Fatalf("goal branch %s present=%t want %t", goalID, err == nil, present)
	}
}

func (fixture *batchE2EFixture) remoteBranchTip(goalID string) string {
	return batchE2EGit(fixture.t, fixture.origin, "rev-parse", "refs/heads/goal/"+goalID)
}

func (fixture *batchE2EFixture) moveTrunk(path string) {
	root := filepath.Join(fixture.t.TempDir(), "mover")
	fixture.clone(root, "mover")
	full := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		fixture.t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte("trunk moved\n"), 0o644); err != nil {
		fixture.t.Fatal(err)
	}
	batchE2EGit(fixture.t, root, "add", "--", path)
	batchE2EGit(fixture.t, root, "commit", "-qm", "move proof input")
	batchE2EGit(fixture.t, root, "push", "-q", "origin", "main")
}

func batchE2EUnit(record batch.Record, goalID string) batch.Unit {
	for _, unit := range record.Units {
		if unit.GoalID == goalID {
			return unit
		}
	}
	return batch.Unit{}
}

func batchE2EConfigureGit(t *testing.T, root, machine string) {
	t.Helper()
	batchE2EGit(t, root, "config", "user.name", "Batch "+machine)
	batchE2EGit(t, root, "config", "user.email", machine+"@example.invalid")
}

func batchE2EGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	commandArgs := append([]string(nil), args...)
	if root != "" {
		commandArgs = append([]string{"-C", root}, commandArgs...)
	}
	command := exec.Command("git", commandArgs...)
	command.Env = append(os.Environ(), "LC_ALL=C")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(commandArgs, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}

func batchE2EGitAt(t *testing.T, root string, args ...string) string {
	t.Helper()
	commandArgs := append([]string{"-C", root}, args...)
	command := exec.Command("git", commandArgs...)
	command.Env = append(os.Environ(), "LC_ALL=C", "GIT_AUTHOR_DATE=2026-09-19T16:00:00Z", "GIT_COMMITTER_DATE=2026-09-19T16:00:00Z")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(commandArgs, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}

func batchE2EOpid(offset int, machine, lineage string) string {
	const alphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	return goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FA"+string(alphabet[offset%len(alphabet)]), machine, lineage)
}

const batchE2ECommitScript = `#!/usr/bin/env bash
set -euo pipefail
goal= receipt= message= attested= snapshot= base=
while (($#)); do
  case "$1" in
    --goal) goal=$2; shift 2 ;;
    --test-receipt) receipt=$2; shift 2 ;;
    --attested) attested=$2; shift 2 ;;
    --attested-snapshot) snapshot=$2; shift 2 ;;
    --attested-base) base=$2; shift 2 ;;
    -F) message=$2; shift 2 ;;
    *) shift ;;
  esac
done
claim=$(git show "HEAD:plans/goals/$goal.md" | sed -n 's/^- Claimed: machine=\([^ ]*\) lineage=\([^ ]*\).* revision=\([0-9][0-9]*\).*/\1+\2 \3/p')
actor=${claim% *}
revision=${claim##* }
{
  cat "$message"
  printf 'Machine: %s\nGoal-Item: %s\nGoal-Revision: %s\n' "$actor" "$goal" "$revision"
  printf 'Landing-Provenance: attested=%s snapshot=%s base=%s receipt=%s\n' "$attested" "$snapshot" "$base" "$receipt"
  printf 'Landing-Provenance-Verdict: pass\nLanded-By: %s\n' "$METASYSTEM_LANDED_BY"
} > .batch-message
git commit --no-verify -F .batch-message
rm -f .batch-message
`
