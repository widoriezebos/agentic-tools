package proofrun

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	metarun "github.com/widoriezebos/agentic-tools/metasystem/internal/run"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func candidateAdmission(request AdmissionRequest) AdmissionRequest {
	request.CandidateGoalID = request.GoalID
	request.CandidateRevision = request.AccountingRevision
	request.CandidateBudgetEpoch = request.BudgetEpoch
	if request.Identity.CommandClass == "testing" && request.CandidateTree == "" {
		if tree, ok := CandidateTreeFromProofIdentity(request.Identity); ok {
			request.CandidateTree = tree
		} else {
			request.CandidateTree = strings.Repeat("b", 40)
		}
	}
	return request
}

// Fixture reservations share one process-local host guard even when their
// proof roots differ. Parallel tests serialize only this test fixture seam.
var fixtureHostAdmissionMu sync.Mutex

func TestWaitAttemptRequiresCommittedTerminal(t *testing.T) {
	root, proofIdentity := proofAttemptFixture(t, "wait-terminal")
	launcher, err := CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	attempt, _, err := ReserveLocked(candidateAdmission(AdmissionRequest{ControlRoot: root, ExecutionRoot: root, GoalID: "goal-a", GoalRevision: 2, AccountingRevision: 2, ReservedMinutes: 2, Identity: proofIdentity, Launcher: launcher, Now: now}))
	if err != nil {
		t.Fatal(err)
	}
	selector := metarun.WaitSelector{Kind: "attempt", TargetID: attempt.AttemptID}
	pending, err := ObserveAttempt(context.Background(), root, selector, metarun.WaiterTarget{}, "")
	if err != nil || !pending.Pending || pending.Incarnation.ProofDigest != proofIdentity.IdentityDigest {
		t.Fatalf("pending attempt observation = %+v err=%v", pending, err)
	}
	if _, err := FinalizeAttempt(root, attempt.AttemptID, TerminalCancelled, 130, "cancelled by owner", nil, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	terminal, err := ObserveAttempt(context.Background(), root, selector, pending.Incarnation, "")
	if err != nil || terminal.Pending || terminal.ExitCode != metarun.ExitLaunchFailed || terminal.TerminalStamp == "" {
		t.Fatalf("terminal attempt observation = %+v err=%v", terminal, err)
	}
}

func TestProofRepeatDecisionProtocol(t *testing.T) {
	root, identity := proofAttemptFixture(t, "repeat")
	launcher, err := CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	request := AdmissionRequest{ControlRoot: root, ExecutionRoot: root, GoalID: "goal-a", GoalRevision: 3,
		AccountingRevision: 2, ReservedMinutes: 4, Identity: identity, Launcher: launcher, Now: now}
	attempt, result, err := ReserveLocked(candidateAdmission(request))
	if err != nil || result.Disposition != DispositionExecuted {
		t.Fatalf("initial reservation = %+v, %+v, %v", attempt, result, err)
	}
	_, duplicate, err := ReserveLocked(candidateAdmission(request))
	if err != nil || duplicate.Disposition != DispositionLiveDuplicate || duplicate.ExitStatus != ExitLiveDuplicate {
		t.Fatalf("live duplicate = %+v, %v", duplicate, err)
	}
	if _, err := FinalizeAttempt(root, attempt.AttemptID, TerminalSuccess, 0, "green", nil, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	_, reusable, err := ReserveLocked(candidateAdmission(request))
	if err != nil || reusable.Disposition != DispositionReusableSuccess || reusable.ExitStatus != ExitReusableSuccess {
		t.Fatalf("success duplicate = %+v, %v", reusable, err)
	}

	failedIdentity := identity
	failedIdentity.CommandClass = "failed-proof"
	failedIdentity.IdentityDigest = failedIdentity.digest()
	request.Identity = failedIdentity
	failed, _, err := ReserveLocked(candidateAdmission(request))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := FinalizeAttempt(root, failed.AttemptID, TerminalFailed, 23, "gate failed", nil, now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	_, required, err := ReserveLocked(candidateAdmission(request))
	if err != nil || required.Disposition != DispositionRetryRequired || required.ExitStatus != ExitRetryRequired {
		t.Fatalf("failed repeat = %+v, %v", required, err)
	}
	evidencePath := filepath.Join(root, "failure.log")
	if err := os.WriteFile(evidencePath, []byte("resource startup failed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	decisionPath := filepath.Join(root, "retry.json")
	decision := RetryDecision{SchemaVersion: 1, PriorAttempt: failed.AttemptID, Cause: "temporary process-resource refusal",
		EvidencePath: filepath.Base(evidencePath), Rationale: "the resource holder has exited"}
	encoded, _ := json.Marshal(decision)
	if err := os.WriteFile(decisionPath, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	request.RetryDecisionPath = decisionPath
	retry, executed, err := ReserveLocked(candidateAdmission(request))
	if err != nil || executed.Disposition != DispositionExecuted || retry.PreviousAttempt != failed.AttemptID || retry.Retry == nil {
		t.Fatalf("diagnosed retry = %+v, %+v, %v", retry, executed, err)
	}
	digest := sha256.Sum256([]byte("resource startup failed\n"))
	if retry.Retry.EvidenceSHA256 != hex.EncodeToString(digest[:]) {
		t.Fatalf("retry evidence digest = %s", retry.Retry.EvidenceSHA256)
	}
}

func TestProofTransientRetryBound(t *testing.T) {
	root, identity := proofAttemptFixture(t, "transient")
	launcher, err := CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	request := AdmissionRequest{ControlRoot: root, ExecutionRoot: root, GoalID: "goal-a", GoalRevision: 2,
		AccountingRevision: 2, ReservedMinutes: 2, Identity: identity, Launcher: launcher, Now: now}
	attempt, _, err := ReserveLocked(candidateAdmission(request))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := FinalizeAttempt(root, attempt.AttemptID, TerminalFailed, 75, "child returned a generic nonzero status", nil, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	_, result, err := ReserveLocked(candidateAdmission(request))
	if err != nil || result.Disposition != DispositionRetryRequired || result.PriorAttempt != attempt.AttemptID {
		t.Fatalf("generic failure was retried automatically: %+v, %v", result, err)
	}
	attempts, err := ReadAttempts(root)
	if err != nil || len(attempts) != 1 {
		t.Fatalf("generic failure created %d attempts, error %v", len(attempts), err)
	}
}

func TestProofContextAcrossRuntimesAndRoots(t *testing.T) {
	firstRoot, firstIdentity := proofAttemptFixture(t, "shared-runtime-proof")
	secondRoot, secondIdentity := proofAttemptFixture(t, "shared-runtime-proof")
	if firstIdentity.IdentityDigest != secondIdentity.IdentityDigest {
		t.Fatalf("byte-identical scratch roots produced different proof identities: %s != %s",
			firstIdentity.IdentityDigest, secondIdentity.IdentityDigest)
	}
	controlRoot := t.TempDir()
	launcher, err := CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	request := AdmissionRequest{ControlRoot: controlRoot, ExecutionRoot: firstRoot, GoalID: "goal-a",
		GoalRevision: 2, AccountingRevision: 2, ReservedMinutes: 4, Identity: firstIdentity, Launcher: launcher, Now: now}
	attempt, result, err := ReserveLocked(candidateAdmission(request))
	if err != nil || result.Disposition != DispositionExecuted {
		t.Fatalf("first runtime reservation = %+v %+v %v", attempt, result, err)
	}
	request.ExecutionRoot = secondRoot
	request.Identity = secondIdentity
	_, duplicate, err := ReserveLocked(candidateAdmission(request))
	if err != nil || duplicate.ExitStatus != ExitLiveDuplicate || duplicate.AttemptID != attempt.AttemptID {
		t.Fatalf("renamed runtime scratch root evaded retained attempt: %+v %v", duplicate, err)
	}
	attempts, err := ReadAttempts(controlRoot)
	if err != nil || len(attempts) != 1 {
		t.Fatalf("scratch-root change created %d reservations: %v", len(attempts), err)
	}
	if _, err := FinalizeAttempt(controlRoot, attempt.AttemptID, TerminalFailed, 1, "fixture cleanup", nil, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
}

func TestProofParentCustody(t *testing.T) {
	root, proofIdentity := proofAttemptFixture(t, "parent-custody")
	launcher, err := CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	attempt, _, err := ReserveLocked(candidateAdmission(AdmissionRequest{ControlRoot: root, ExecutionRoot: root, GoalID: "goal-a",
		GoalRevision: 2, AccountingRevision: 2, ReservedMinutes: 2, Identity: proofIdentity, Launcher: launcher, Now: now}))

	if err != nil {
		t.Fatal(err)
	}
	if joined, err := AuthenticateContext(root, attempt.AttemptID, int64(os.Getpid())); err != nil || joined.AttemptID != attempt.AttemptID {
		t.Fatalf("exact launcher ancestry did not authenticate: attempt=%+v err=%v", joined, err)
	}
	staleLauncher := launcher
	if staleLauncher.PidStartedAtMicro > 0 {
		staleLauncher.PidStartedAtMicro++
	} else if staleLauncher.PidStartTicks > 0 {
		staleLauncher.PidStartTicks++
	} else {
		staleLauncher.PidStartedAt++
	}
	staleIdentity := proofIdentity
	staleIdentity.CommandClass = "stale-parent-custody"
	staleIdentity.IdentityDigest = staleIdentity.digest()
	staleAttempt, _, err := ReserveLocked(candidateAdmission(AdmissionRequest{ControlRoot: root, ExecutionRoot: root, GoalID: "goal-a",
		GoalRevision: 2, AccountingRevision: 2, ReservedMinutes: 2, Identity: staleIdentity, Launcher: staleLauncher, Now: now}))

	if err != nil {
		t.Fatal(err)
	}
	if _, err := AuthenticateContext(root, staleAttempt.AttemptID, int64(os.Getpid())); err == nil {
		t.Fatal("stale launcher identity authenticated a parent proof locator")
	}
	expiredIdentity := proofIdentity
	expiredIdentity.CommandClass = "expired-parent-custody"
	expiredIdentity.IdentityDigest = expiredIdentity.digest()
	expiredAttempt, _, err := ReserveLocked(candidateAdmission(AdmissionRequest{ControlRoot: root, ExecutionRoot: root, GoalID: "goal-a",
		GoalRevision: 2, AccountingRevision: 2, ReservedMinutes: 2, Identity: expiredIdentity, Launcher: launcher,
		Now: now.Add(-3 * time.Minute)}))

	if err != nil {
		t.Fatal(err)
	}
	// A top-level attempt past its reservation horizon still authenticates
	// its parent locator (the deadline governs only a job-capped
	// reservation); the governed shape is proven in TestGovernedDeadlineStillGoverns.
	if _, err := AuthenticateContext(root, expiredAttempt.AttemptID, int64(os.Getpid())); err != nil {
		t.Fatalf("a top-level attempt past its reservation refused its parent locator: %v", err)
	}
	if err := RequestCancellation(root, attempt.AttemptID, "controlled cancellation"); err != nil {
		t.Fatal(err)
	}
	if _, err := AuthenticateContext(root, attempt.AttemptID, int64(os.Getpid())); err == nil {
		t.Fatal("cancelled attempt remained valid as a parent proof locator")
	}
	if _, err := FinalizeAttempt(root, attempt.AttemptID, TerminalCancelled, 1, "fixture cleanup", nil, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := FinalizeAttempt(root, staleAttempt.AttemptID, TerminalFailed, 1, "fixture cleanup", nil, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := FinalizeAttempt(root, expiredAttempt.AttemptID, TerminalFailed, 1, "fixture cleanup", nil, now); err != nil {
		t.Fatal(err)
	}
}

func TestProofIdentityUsesEffectiveConfiguration(t *testing.T) {
	root, first := proofAttemptFixture(t, "effective-configuration")
	conf := filepath.Join(root, "metasystem.conf")
	if err := os.WriteFile(conf+".local", []byte("dispatch.cap-max=90\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	local, err := BuildProofIdentity(root, conf, "full", "effective-configuration", []string{"gate"}, behaviorsurface.SupportedVersion)
	if err != nil {
		t.Fatal(err)
	}
	if first.Configuration == local.Configuration {
		t.Fatal("local effective configuration did not change proof identity")
	}
	t.Setenv(config.EnvName("dispatch.cap-max"), "75")
	environment, err := BuildProofIdentity(root, conf, "full", "effective-configuration", []string{"gate"}, behaviorsurface.SupportedVersion)
	if err != nil {
		t.Fatal(err)
	}
	if local.Configuration == environment.Configuration {
		t.Fatal("environment effective configuration did not change proof identity")
	}
	t.Setenv("METASYSTEM_PROOF_ATTEMPT", "runtime-only-id")
	t.Setenv("METASYSTEM_PROOF_CONTROL_ROOT", filepath.Join(root, "runtime-only-root"))
	runtimeOnly, err := BuildProofIdentity(root, conf, "full", "effective-configuration", []string{"gate"}, behaviorsurface.SupportedVersion)
	if err != nil {
		t.Fatal(err)
	}
	if environment.Configuration != runtimeOnly.Configuration || environment.IdentityDigest != runtimeOnly.IdentityDigest {
		t.Fatal("runtime proof locators changed effective input identity")
	}
}

func TestProofIdentityBindsNormalizedConsumerInputs(t *testing.T) {
	_, identity := proofAttemptFixture(t, "coverage-delta")
	first := BindIdentityInputs(identity, []string{"coverage-package=internal/proofrun", "coverage-ratchet=abc"})
	reordered := BindIdentityInputs(identity, []string{"coverage-ratchet=abc", "coverage-package=internal/proofrun"})
	changed := BindIdentityInputs(identity, []string{"coverage-package=internal/landing", "coverage-ratchet=abc"})
	if first.IdentityDigest != reordered.IdentityDigest {
		t.Fatal("consumer input order changed proof identity")
	}
	if first.IdentityDigest == identity.IdentityDigest || first.IdentityDigest == changed.IdentityDigest {
		t.Fatal("selected coverage package and ratchet inputs were not bound to proof identity")
	}
}

func TestAttemptSchemaTwoAtomicallyRetainsTestingAndReadsSchemaOne(t *testing.T) {
	root, identity := proofAttemptFixture(t, "testing")
	launcher, err := CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	attempt, _, err := ReserveLocked(candidateAdmission(AdmissionRequest{ControlRoot: root, ExecutionRoot: root, GoalID: "goal-a",
		GoalRevision: 2, AccountingRevision: 2, ReservedMinutes: 2, Identity: identity, Launcher: launcher, Now: now}))
	if err != nil {
		t.Fatal(err)
	}
	path, _ := AttemptPath(root, attempt.AttemptID)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	newReservation := append([]byte(nil), data...)
	var schemaTwo map[string]any
	if json.Unmarshal(data, &schemaTwo) != nil {
		t.Fatal("decode schema-3 reservation")
	}
	schemaTwo["schemaVersion"] = float64(AttemptSchemaVersion)
	delete(schemaTwo, "candidateGoalId")
	delete(schemaTwo, "candidateRevision")
	delete(schemaTwo, "candidateBudgetEpoch")
	delete(schemaTwo, "candidateTree")
	data, _ = json.Marshal(schemaTwo)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	attempt, err = ReadAttempt(root, attempt.AttemptID)
	if err != nil {
		t.Fatal(err)
	}
	zero := 0
	digest := strings.Repeat("a", 64)
	result := TestResult{SchemaVersion: 1, CandidateEngineIdentityVersion: CandidateEngineIdentitySchemaVersion,
		AttemptID: attempt.AttemptID, Purpose: "delivery", RequestedMode: "auto",
		RequiredMode: "standard", ExecutedMode: "standard", ProjectRoot: root, BaseCommit: "base", CandidateTree: strings.Repeat("b", 40),
		ContractDigest: digest, BaseContractDigest: digest, PolicyEngineDigest: digest, CandidateEngineDigest: digest,
		CandidateEngineBuildIdentity: strings.Repeat("c", 40), BehaviorPolicyDigest: digest, PlanDigest: digest,
		SelectedGroups: []string{"smoke"}, RequiredGroups: []string{"smoke"}, LaunchCounts: LaunchCounts{CountsComplete: true}, Cost: TestCost{DeclaredTargetMS: 1},
		Groups: []GroupResult{{ID: "smoke", Kind: "unit", InputManifest: []string{"source"}, Status: "passed", NativeLaunched: true, CollectionComplete: true, NativeExitStatus: &zero,
			ToolIdentities: map[string]string{}, ReportDigests: map[string]string{}}}}
	result.RecomputeDelivery()
	if _, err := FinalizeAttemptWithTestResultLocked(root, attempt.AttemptID, TerminalSuccess, 0, "green", nil, &result, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	stored, err := ReadAttempt(root, attempt.AttemptID)
	if err != nil || stored.SchemaVersion != 2 || stored.TestResult == nil || stored.TestResult.AttemptID != attempt.AttemptID {
		t.Fatalf("schema-2 testing result was not retained atomically: attempt=%+v err=%v", stored, err)
	}
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var oldFormat map[string]any
	if json.Unmarshal(data, &oldFormat) != nil {
		t.Fatal("decode retained testing fixture")
	}
	testingResult := oldFormat["testResult"].(map[string]any)
	delete(testingResult, "candidateEngineIdentityVersion")
	delete(testingResult, "candidateEngineDigest")
	delete(testingResult, "candidateEngineBuildIdentity")
	data, _ = json.Marshal(oldFormat)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if read, err := ReadAttempts(root); err != nil || len(read) != 1 || read[0].TestResult == nil || read[0].TestResult.CandidateEngineDigest != "" {
		t.Fatalf("old-format delivery result was not readable: attempts=%+v err=%v", read, err)
	}
	if err := os.WriteFile(path, newReservation, 0o600); err != nil {
		t.Fatal(err)
	}
	result.SchemaVersion = PreviousTestResultSchemaVersion
	result.Groups[0].IdentityVersion = PreviousGroupExecutionIdentityVersion
	if _, err := FinalizeAttemptWithTestResultLocked(root, attempt.AttemptID, TerminalSuccess, 0, "green", nil, &result, now.Add(time.Second)); err != nil {
		t.Fatalf("candidate attempt did not upgrade while retaining schema-2 identity evidence: %v", err)
	}
	previous, err := ReadAttempt(root, attempt.AttemptID)
	if err != nil || previous.SchemaVersion != IdentityAttemptSchemaVersion || previous.TestResult == nil || previous.TestResult.SchemaVersion != PreviousTestResultSchemaVersion {
		t.Fatalf("schema-2 result did not require the identity attempt schema: %+v %v", previous, err)
	}
	downgraded := previous
	downgraded.SchemaVersion = CandidateAttemptSchemaVersion
	if err := validateAttempt(downgraded); err == nil || !strings.Contains(err.Error(), "requires attempt schema") {
		t.Fatalf("schema-2 identity evidence accepted a candidate attempt record: %v", err)
	}
	if err := os.WriteFile(path, newReservation, 0o600); err != nil {
		t.Fatal(err)
	}
	result.SchemaVersion = TestResultSchemaVersion
	result.Groups[0].IdentityVersion = GroupExecutionIdentityVersion
	applyCurrentWorkerPolicy(&result)
	if _, err := FinalizeAttemptWithTestResultLocked(root, attempt.AttemptID, TerminalSuccess, 0, "green", nil, &result, now.Add(time.Second)); err != nil {
		t.Fatalf("schema-4 attempt did not retain schema-3 result: %v", err)
	}
	if read, err := ReadAttempt(root, attempt.AttemptID); err != nil || read.SchemaVersion != IdentityAttemptSchemaVersion ||
		read.TestResult == nil || read.TestResult.SchemaVersion != TestResultSchemaVersion {
		t.Fatalf("versioned attempt/result did not round-trip: %+v %v", read, err)
	}
	versioned, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var future map[string]any
	if err := json.Unmarshal(versioned, &future); err != nil {
		t.Fatal(err)
	}
	future["schemaVersion"] = float64(IdentityAttemptSchemaVersion + 1)
	data, _ = json.Marshal(future)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadAttempt(root, attempt.AttemptID); err == nil || !strings.Contains(err.Error(), "unsupported future attempt schema") {
		t.Fatalf("future attempt schema did not get a named refusal: %v", err)
	}
	future["schemaVersion"] = float64(IdentityAttemptSchemaVersion)
	future["testResult"].(map[string]any)["schemaVersion"] = float64(TestResultSchemaVersion + 1)
	data, _ = json.Marshal(future)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadAttempt(root, attempt.AttemptID); err == nil || !strings.Contains(err.Error(), "unsupported future test result schema") {
		t.Fatalf("future test result schema did not get a named refusal: %v", err)
	}
	if err := os.WriteFile(path, versioned, 0o600); err != nil {
		t.Fatal(err)
	}
	newFormatWithoutDigest := result
	newFormatWithoutDigest.CandidateEngineDigest = ""
	if err := ValidateTestResult(newFormatWithoutDigest); err == nil {
		t.Fatal("new-format delivery result without its candidate engine digest was accepted")
	}

	legacyIdentity := identity
	legacyIdentity.CommandClass = "legacy"
	legacyIdentity.IdentityDigest = legacyIdentity.digest()
	legacy, _, err := ReserveLocked(candidateAdmission(AdmissionRequest{ControlRoot: root, ExecutionRoot: root, GoalID: "goal-a",
		GoalRevision: 2, AccountingRevision: 2, ReservedMinutes: 2, Identity: legacyIdentity, Launcher: launcher, Now: now}))

	if err != nil {
		t.Fatal(err)
	}
	path, _ = AttemptPath(root, legacy.AttemptID)
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if json.Unmarshal(data, &raw) != nil {
		t.Fatal("decode schema-2 fixture")
	}
	raw["schemaVersion"] = float64(1)
	delete(raw, "candidateGoalId")
	delete(raw, "candidateRevision")
	delete(raw, "candidateBudgetEpoch")
	delete(raw, "candidateTree")
	data, _ = json.Marshal(raw)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if read, err := ReadAttempt(root, legacy.AttemptID); err != nil || read.SchemaVersion != 1 || read.TestResult != nil {
		t.Fatalf("legacy schema-1 attempt was not readable without invented testing facts: %+v %v", read, err)
	}
}

func TestAttemptWithoutCandidateFieldsReadsAsToday(t *testing.T) {
	epoch := uint64(7)
	for _, schema := range []int{LegacyAttemptSchemaVersion, AttemptSchemaVersion} {
		attempt := Attempt{SchemaVersion: schema, GoalID: "authority", AccountingRevision: 4, BudgetEpoch: &epoch}
		if attempt.AccountedGoal() != "authority" || attempt.AccountedRevision() != 4 ||
			attempt.AccountedBudgetEpoch() == nil || *attempt.AccountedBudgetEpoch() != epoch {
			t.Fatalf("schema %d did not retain authority accounting: %+v", schema, attempt)
		}
	}
	tree := strings.Repeat("1", 40)
	identity := ProofIdentity{CommandClass: "testing", IdentityInputs: []string{candidateTreeIdentityPrefix + tree}}
	if got, ok := (Attempt{SchemaVersion: AttemptSchemaVersion, ProofIdentity: identity}).CandidateTreeDigest(); !ok || got != tree {
		t.Fatalf("schema-2 tagged candidate tree = %q, %t", got, ok)
	}
	identity.IdentityInputs[0] = tree
	if got, ok := (Attempt{SchemaVersion: LegacyAttemptSchemaVersion, ProofIdentity: identity}).CandidateTreeDigest(); !ok || got != tree {
		t.Fatalf("schema-1 raw candidate tree = %q, %t", got, ok)
	}
	if _, ok := (Attempt{SchemaVersion: AttemptSchemaVersion}).CandidateTreeDigest(); ok {
		t.Fatal("candidate-less legacy attempt invented a tree")
	}
}

func TestProofIdentityCandidateTreeFindsPrefixedInputAnywhere(t *testing.T) {
	tree, bare := strings.Repeat("2", 40), strings.Repeat("1", 40)
	identity := ProofIdentity{CommandClass: "testing", IdentityInputs: []string{
		strings.Repeat("0", 64),
		bare,
		candidateTreeIdentityPrefix + tree,
	}}
	if got, ok := CandidateTreeFromProofIdentity(identity); !ok || got != tree {
		t.Fatalf("prefixed candidate tree after a sorted digest = %q, %t", got, ok)
	}

	identity.IdentityInputs = append(identity.IdentityInputs, candidateTreeIdentityPrefix+strings.Repeat("3", 40))
	if got, ok := CandidateTreeFromProofIdentity(identity); ok {
		t.Fatalf("conflicting prefixed candidate trees produced %q", got)
	}

	identity.IdentityInputs = []string{bare, candidateTreeIdentityPrefix + "invalid"}
	if got, ok := CandidateTreeFromProofIdentity(identity); ok {
		t.Fatalf("bare fallback bypassed a malformed prefixed input with %q", got)
	}
}

func TestProofIdentityCandidateTreeAcceptsOnlyOneBareLegacyTree(t *testing.T) {
	tree := strings.Repeat("2", 40)
	identity := ProofIdentity{CommandClass: "testing", IdentityInputs: []string{
		strings.Repeat("0", 64),
		tree,
		"plan:legacy",
	}}
	if got, ok := CandidateTreeFromProofIdentity(identity); !ok || got != tree {
		t.Fatalf("unique bare legacy candidate tree = %q, %t", got, ok)
	}

	identity.IdentityInputs = []string{tree, strings.Repeat("3", 40)}
	if got, ok := CandidateTreeFromProofIdentity(identity); ok {
		t.Fatalf("ambiguous bare legacy candidate trees produced %q", got)
	}

	identity.IdentityInputs = []string{strings.Repeat("4", 64)}
	if got, ok := CandidateTreeFromProofIdentity(identity); ok {
		t.Fatalf("64-hex identity digest produced candidate tree %q", got)
	}
}

func TestSchemaThreeCandidateTreeIgnoresSortedIdentityDigests(t *testing.T) {
	root, identity := proofAttemptFixture(t, "testing")
	identity = BindIdentityInputs(identity, []string{
		strings.Repeat("0", 64),
		strings.Repeat("1", 64),
	})
	launcher, err := CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
	tree := strings.Repeat("2", 40)
	attempt, decision, err := ReserveLocked(AdmissionRequest{
		ControlRoot: root, ExecutionRoot: root, GoalID: "goal-a", GoalRevision: 2, AccountingRevision: 2,
		CandidateGoalID: "goal-a", CandidateRevision: 2, CandidateTree: tree,
		ReservedMinutes: 2, Identity: identity, Launcher: launcher, Now: now,
	})
	if err != nil || decision.Disposition != DispositionExecuted || attempt.CandidateTree != tree {
		t.Fatalf("schema-3 attempt with sorted identity digests = %+v decision=%+v err=%v", attempt, decision, err)
	}
}

func TestAttemptSchemaThreeCarriesTheCandidateTupleAtomically(t *testing.T) {
	root, identity := proofAttemptFixture(t, "testing")
	launcher, err := CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	epoch := uint64(9)
	tree := strings.Repeat("3", 40)
	request := AdmissionRequest{ControlRoot: root, ExecutionRoot: root, GoalID: "authority", GoalRevision: 5, AccountingRevision: 4,
		BudgetEpoch: &epoch, CandidateGoalID: "authority", CandidateRevision: 4, CandidateBudgetEpoch: &epoch, CandidateTree: tree,
		ReservedMinutes: 2, Identity: identity, Launcher: launcher, Now: time.Now().UTC()}
	attempt, decision, err := ReserveLocked(request)
	if err != nil || decision.Disposition != DispositionExecuted || attempt.SchemaVersion != IdentityAttemptSchemaVersion ||
		attempt.AccountedGoal() != "authority" || attempt.AccountedRevision() != 4 || attempt.CandidateTree != tree ||
		attempt.AccountedBudgetEpoch() == nil || *attempt.AccountedBudgetEpoch() != epoch {
		t.Fatalf("schema-3 reservation = %+v decision=%+v err=%v", attempt, decision, err)
	}

	missingGoal := request
	missingGoal.AttemptID, missingGoal.CandidateGoalID = "missing-candidate-goal", ""
	if _, _, err := ReserveLocked(missingGoal); err == nil || !strings.Contains(err.Error(), "reservation requires a complete candidate tuple") {
		t.Fatalf("reservation without candidate goal was not refused at its boundary: %v", err)
	}
	missingRevision := request
	missingRevision.AttemptID, missingRevision.CandidateRevision = "missing-candidate-revision", 0
	if _, _, err := ReserveLocked(missingRevision); err == nil || !strings.Contains(err.Error(), "reservation requires a complete candidate tuple") {
		t.Fatalf("reservation without candidate revision was not refused at its boundary: %v", err)
	}

	invalid := attempt
	invalid.CandidateGoalID = ""
	if err := validateAttempt(invalid); err == nil || invalid.AccountedGoal() != "" {
		t.Fatalf("schema-3 record without candidate goal was accepted or reattributed: goal=%q err=%v", invalid.AccountedGoal(), err)
	}
	invalid = attempt
	invalid.CandidateRevision = 0
	if err := validateAttempt(invalid); err == nil || invalid.AccountedRevision() != 0 {
		t.Fatalf("schema-3 record without candidate revision was accepted or reattributed: revision=%d err=%v", invalid.AccountedRevision(), err)
	}
	invalid = attempt
	invalid.CandidateTree = ""
	if err := validateAttempt(invalid); err == nil {
		t.Fatal("schema-3 testing record without candidate tree was accepted")
	}
	schemaTwo := attempt
	schemaTwo.SchemaVersion = AttemptSchemaVersion
	schemaTwo.CandidateGoalID, schemaTwo.CandidateRevision, schemaTwo.CandidateBudgetEpoch, schemaTwo.CandidateTree = "", 0, nil, ""
	for name, mutate := range map[string]func(*Attempt){
		"goal":     func(row *Attempt) { row.CandidateGoalID = "candidate" },
		"revision": func(row *Attempt) { row.CandidateRevision = 1 },
		"epoch":    func(row *Attempt) { row.CandidateBudgetEpoch = &epoch },
		"tree":     func(row *Attempt) { row.CandidateTree = tree },
	} {
		row := schemaTwo
		mutate(&row)
		if err := validateAttempt(row); err == nil {
			t.Fatalf("schema-2 record carrying candidate %s was accepted", name)
		}
	}
	legacy := Attempt{SchemaVersion: AttemptSchemaVersion, GoalID: "authority", AccountingRevision: 4, BudgetEpoch: &epoch}
	if legacy.AccountedGoal() != "authority" || legacy.AccountedRevision() != 4 || legacy.AccountedBudgetEpoch() != &epoch {
		t.Fatalf("schema-2 accessors did not read authority tuple: %+v", legacy)
	}
}

func TestCandidateTreeFieldsMustAgree(t *testing.T) {
	root, identity := proofAttemptFixture(t, "testing")
	launcher, err := CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	tree, other := strings.Repeat("1", 40), strings.Repeat("2", 40)
	attempt, _, err := ReserveLocked(AdmissionRequest{ControlRoot: root, ExecutionRoot: root, GoalID: "goal-a", GoalRevision: 2,
		AccountingRevision: 2, CandidateGoalID: "goal-a", CandidateRevision: 2, CandidateTree: tree,
		ReservedMinutes: 2, Identity: identity, Launcher: launcher, Now: time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}
	withIdentity := attempt
	withIdentity.ProofIdentity = BindIdentityInputs(identity, []string{candidateTreeIdentityPrefix + other})
	want := "proof attempt candidate tree " + tree + " disagrees with proof identity tree " + other
	if err := validateAttempt(withIdentity); err == nil || err.Error() != want {
		t.Fatalf("candidate field disagreed with identity without refusal: %v", err)
	}
	withResult := attempt
	result := componentAttemptResult(attempt.AttemptID, "group", strings.Repeat("a", 64), "passed")
	result.CandidateTree = other
	result.RecomputeDelivery()
	withResult.TestResult = &result
	if err := validateAttempt(withResult); err == nil || !strings.Contains(err.Error(), "testing evidence names candidate tree") {
		t.Fatalf("candidate field disagreed with result without refusal: %v", err)
	}
}

func TestWithdrawReservationRemovesAReservedOnlyRecord(t *testing.T) {
	root, identity := proofAttemptFixture(t, "withdraw")
	launcher, err := CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	request := candidateAdmission(AdmissionRequest{ControlRoot: root, ExecutionRoot: root, GoalID: "goal-a", GoalRevision: 2,
		AccountingRevision: 2, ReservedMinutes: 2, Identity: identity, Launcher: launcher, Now: time.Now().UTC()})
	reserved, _, err := ReserveLocked(request)
	if err != nil {
		t.Fatal(err)
	}
	if err := WithdrawReservationLocked(root, reserved.AttemptID); err != nil {
		t.Fatal(err)
	}
	if attempts, err := ReadAttempts(root); err != nil || len(attempts) != 0 {
		t.Fatalf("withdrawn reservation remained visible: attempts=%+v err=%v", attempts, err)
	}

	withProcess := request
	withProcess.AttemptID = "reserved-with-process"
	processAttempt, _, err := ReserveLocked(withProcess)
	if err != nil {
		t.Fatal(err)
	}
	if err := UpdateAttemptProcesses(root, processAttempt.AttemptID, launcher.Ref(), []string{"child-process"}); err != nil {
		t.Fatal(err)
	}
	if err := WithdrawReservationLocked(root, processAttempt.AttemptID); err == nil {
		t.Fatal("reservation with a process observation was withdrawn")
	}

	withResult := request
	withResult.AttemptID = "reserved-with-result"
	withResult.Identity.CommandClass = "withdraw-result"
	withResult.Identity.IdentityDigest = withResult.Identity.digest()
	resultAttempt, _, err := ReserveLocked(withResult)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := FinalizeAttempt(root, resultAttempt.AttemptID, TerminalFailed, 1, "observed", nil, request.Now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := WithdrawReservationLocked(root, resultAttempt.AttemptID); err == nil {
		t.Fatal("reservation with a terminal result was withdrawn")
	}
}

func TestComponentRepeatDecisionSpansPlanChanges(t *testing.T) {
	root, baseIdentity := proofAttemptFixture(t, "testing")
	launcher, err := CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	identity := BindIdentityInputs(baseIdentity, []string{"group:a:" + strings.Repeat("1", 64), "plan:one"})
	request := AdmissionRequest{ControlRoot: root, ExecutionRoot: root, GoalID: "goal-a", GoalRevision: 2, AccountingRevision: 2,
		ReservedMinutes: 2, Identity: identity, Launcher: launcher, Now: now, ComponentIdentities: map[string]string{"a": strings.Repeat("1", 64)}}
	attempt, _, err := ReserveLocked(candidateAdmission(request))
	if err != nil {
		t.Fatal(err)
	}
	passed := componentAttemptResult(attempt.AttemptID, "a", strings.Repeat("1", 64), "passed")
	if _, err := FinalizeAttemptWithTestResultLocked(root, attempt.AttemptID, TerminalSuccess, 0, "green", nil, &passed, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	request.Identity = BindIdentityInputs(baseIdentity, []string{"group:a:" + strings.Repeat("1", 64), "plan:two"})
	if decision, noChild, err := NoChildDecisionLocked(candidateAdmission(request)); err != nil || !noChild || decision.ExitStatus != ExitReusableSuccess {
		t.Fatalf("successful component was not reused across a plan change: %+v noChild=%v err=%v", decision, noChild, err)
	}
	request.ComponentIdentities["new"] = strings.Repeat("2", 64)
	if decision, noChild, err := NoChildDecisionLocked(candidateAdmission(request)); err != nil || noChild {
		t.Fatalf("partial component reuse suppressed missing work: %+v noChild=%v err=%v", decision, noChild, err)
	}

	failedIdentity := BindIdentityInputs(baseIdentity, []string{"group:failed:" + strings.Repeat("3", 64), "plan:failed"})
	failedRequest := request
	failedRequest.Identity = failedIdentity
	failedRequest.ComponentIdentities = map[string]string{"failed": strings.Repeat("3", 64)}
	failed, _, err := ReserveLocked(candidateAdmission(failedRequest))
	if err != nil {
		t.Fatal(err)
	}
	failedResult := componentAttemptResult(failed.AttemptID, "failed", strings.Repeat("3", 64), "failed")
	if _, err := FinalizeAttemptWithTestResultLocked(root, failed.AttemptID, TerminalFailed, 23, "red", nil, &failedResult, now.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	failedRequest.Identity = BindIdentityInputs(baseIdentity, []string{"group:failed:" + strings.Repeat("3", 64), "plan:superset"})
	if decision, noChild, err := NoChildDecisionLocked(candidateAdmission(failedRequest)); err != nil || !noChild || decision.ExitStatus != ExitRetryRequired || decision.PriorAttempt != failed.AttemptID {
		t.Fatalf("failed component did not require diagnosis across a plan change: %+v noChild=%v err=%v", decision, noChild, err)
	}
	attempts, err := ReadAttempts(root)
	if err != nil {
		t.Fatal(err)
	}
	template := componentAttemptResult("", "failed", strings.Repeat("3", 64), "passed")
	template.Groups = nil
	contract := testpolicy.Contract{Groups: []testpolicy.Group{{ID: "failed", Kind: "unit", Inputs: []string{"source"}, Obligations: []string{"failed"}}}}
	projection := ReusedTestResult(template, attempts, map[string]string{"failed": strings.Repeat("3", 64)}, contract)
	if projection.Groups[0].Status != "not-run" || projection.Delivery.Sufficient {
		t.Fatalf("an older success masked the newest matching component failure: %+v", projection)
	}
}

func TestRedAttemptFencesOnlyFailedOrNonterminalComponents(t *testing.T) {
	root, baseIdentity := proofAttemptFixture(t, "testing")
	launcher, err := CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	tree := strings.Repeat("1", 40)
	identities := map[string]string{"failed": strings.Repeat("a", 64), "passed": strings.Repeat("b", 64), "not-run": strings.Repeat("c", 64)}
	request := componentAdmissionRequest(root, baseIdentity, launcher, now, tree, "red", identities)
	attempt := retainComponentAttempt(t, request, tree, []componentStatus{{"failed", "failed"}, {"passed", "passed"}, {"not-run", "not-run"}}, now.Add(time.Second))

	passed := componentAdmissionRequest(root, baseIdentity, launcher, now.Add(2*time.Second), tree, "passed-only", map[string]string{"passed": identities["passed"]})
	if decision, noChild, err := NoChildDecisionLocked(candidateAdmission(passed)); err != nil || !noChild || decision.ExitStatus != ExitReusableSuccess {
		t.Fatalf("passing component of red attempt was not reusable: decision=%+v noChild=%t err=%v", decision, noChild, err)
	}
	for _, id := range []string{"failed", "not-run"} {
		component := componentAdmissionRequest(root, baseIdentity, launcher, now.Add(2*time.Second), tree, id+"-only", map[string]string{id: identities[id]})
		if decision, noChild, err := NoChildDecisionLocked(candidateAdmission(component)); err != nil || !noChild || decision.ExitStatus != ExitRetryRequired || decision.PriorAttempt != attempt.AttemptID {
			t.Fatalf("%s component did not retain its retry fence: decision=%+v noChild=%t err=%v", id, decision, noChild, err)
		}
	}
}

func TestGLEBlockedReservationDoesNotBecomeFailedProducer(t *testing.T) {
	t.Parallel()
	fixtureHostAdmissionMu.Lock()
	previousAdmissionDirectory := hostAdmissionDirectoryForTest
	hostAdmissionDirectoryForTest = filepath.Join(t.TempDir(), "host-admission")
	defer func() {
		hostAdmissionDirectoryForTest = previousAdmissionDirectory
		fixtureHostAdmissionMu.Unlock()
	}()
	root, baseIdentity := proofAttemptFixture(t, "testing")
	launcher, err := CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	firstTree, repairedTree := strings.Repeat("1", 40), strings.Repeat("2", 40)
	harnessIdentity, repairedHarnessIdentity := strings.Repeat("a", 64), strings.Repeat("d", 64)
	dependentIdentity, siblingIdentity := strings.Repeat("b", 64), strings.Repeat("c", 64)
	first := componentAdmissionRequest(root, baseIdentity, launcher, now, firstTree, "blocked-dependent",
		map[string]string{"harness": harnessIdentity, "dependent": dependentIdentity, "sibling": siblingIdentity})
	first.SharedComponents = true
	failed := retainComponentAttempt(t, first, firstTree, []componentStatus{
		{"harness", "failed"}, {"dependent", "blocked"}, {"sibling", "passed"}}, now.Add(time.Second))
	repaired := componentAdmissionRequest(root, baseIdentity, launcher, now.Add(2*time.Second), repairedTree, "repaired-harness",
		map[string]string{"harness": repairedHarnessIdentity, "dependent": dependentIdentity, "sibling": siblingIdentity})
	repaired.SharedComponents = true
	if decision, noChild, err := NoChildDecisionLocked(candidateAdmission(repaired)); err != nil || noChild {
		t.Fatalf("blocked dependent prevented first native execution after prerequisite repair: decision=%+v noChild=%t err=%v", decision, noChild, err)
	}
	unchangedFailure := componentAdmissionRequest(root, baseIdentity, launcher, now.Add(2*time.Second), firstTree, "same-failed-harness",
		map[string]string{"harness": harnessIdentity})
	unchangedFailure.SharedComponents = true
	if decision, noChild, err := NoChildDecisionLocked(candidateAdmission(unchangedFailure)); err != nil || !noChild ||
		decision.ExitStatus != ExitRetryRequired || decision.PriorAttempt != failed.AttemptID {
		t.Fatalf("failed native harness lost its retry fence: decision=%+v noChild=%t err=%v", decision, noChild, err)
	}
}

func TestRetryDecisionIgnoresFailuresFromOtherCandidateTrees(t *testing.T) {
	root, baseIdentity := proofAttemptFixture(t, "testing")
	launcher, err := CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	firstIdentity, secondIdentity := strings.Repeat("a", 64), strings.Repeat("b", 64)
	firstTree, secondTree, candidateTree := strings.Repeat("1", 40), strings.Repeat("2", 40), strings.Repeat("3", 40)
	first := componentAdmissionRequest(root, baseIdentity, launcher, now, firstTree, "first", map[string]string{"g1": firstIdentity})
	retainComponentAttempt(t, first, firstTree, []componentStatus{{"g1", "failed"}}, now.Add(time.Second))
	second := componentAdmissionRequest(root, baseIdentity, launcher, now.Add(2*time.Second), secondTree, "second", map[string]string{"g2": secondIdentity})
	retainComponentAttempt(t, second, secondTree, []componentStatus{{"g2", "failed"}}, now.Add(3*time.Second))

	request := componentAdmissionRequest(root, baseIdentity, launcher, now.Add(4*time.Second), candidateTree, "candidate",
		map[string]string{"g1": firstIdentity, "g2": secondIdentity})
	request.RetryDecisionPath = filepath.Join(root, "not-needed.json")
	if decision, noChild, err := NoChildDecisionLocked(candidateAdmission(request)); err != nil || noChild {
		t.Fatalf("other candidates' failures affected retry admission: decision=%+v noChild=%t err=%v", decision, noChild, err)
	}
}

func TestComponentRetryDecisionIgnoresAnotherCandidatesFailure(t *testing.T) {
	root, baseIdentity := proofAttemptFixture(t, "testing")
	launcher, err := CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	treeOne, treeTwo := strings.Repeat("1", 40), strings.Repeat("2", 40)
	gIdentity, hIdentity := strings.Repeat("a", 64), strings.Repeat("b", 64)
	first := componentAdmissionRequest(root, baseIdentity, launcher, now, treeOne, "first", map[string]string{"g": gIdentity})
	retainComponentAttempt(t, first, treeOne, []componentStatus{{"g", "failed"}}, now.Add(time.Second))
	second := componentAdmissionRequest(root, baseIdentity, launcher, now.Add(2*time.Second), treeTwo, "second", map[string]string{"h": hIdentity})
	prior := retainComponentAttempt(t, second, treeTwo, []componentStatus{{"h", "failed"}}, now.Add(3*time.Second))

	evidencePath := filepath.Join(root, "candidate-two-failure.log")
	if err := os.WriteFile(evidencePath, []byte("candidate two failure diagnosed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	decisionPath := filepath.Join(root, "candidate-two-retry.json")
	encoded, _ := json.Marshal(RetryDecision{SchemaVersion: 1, PriorAttempt: prior.AttemptID, Cause: "deterministic group failure",
		EvidencePath: filepath.Base(evidencePath), Rationale: "the candidate-two failure was reviewed"})
	if err := os.WriteFile(decisionPath, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	request := componentAdmissionRequest(root, baseIdentity, launcher, now.Add(4*time.Second), treeTwo, "combined",
		map[string]string{"g": gIdentity, "h": hIdentity})
	request.RetryDecisionPath = decisionPath
	retry, decision, err := ReserveLocked(request)
	if err != nil || decision.Disposition != DispositionExecuted || retry.PreviousAttempt != prior.AttemptID {
		t.Fatalf("other candidate made retry ambiguous: retry=%+v decision=%+v err=%v", retry, decision, err)
	}
}

func TestComponentSuccessIsReusableAcrossCandidateTrees(t *testing.T) {
	root, baseIdentity := proofAttemptFixture(t, "testing")
	launcher, err := CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	identity := strings.Repeat("a", 64)
	first := componentAdmissionRequest(root, baseIdentity, launcher, now, strings.Repeat("1", 40), "first", map[string]string{"g": identity})
	attempt, _, err := ReserveLocked(first)
	if err != nil {
		t.Fatal(err)
	}
	result := componentAttemptResult(attempt.AttemptID, "g", identity, "passed")
	result.CandidateTree = first.CandidateTree
	result.RecomputeDelivery()
	if _, err := FinalizeAttemptWithTestResultLocked(root, attempt.AttemptID, TerminalSuccess, 0, "green", nil, &result, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	second := componentAdmissionRequest(root, baseIdentity, launcher, now.Add(2*time.Second), strings.Repeat("2", 40), "second", map[string]string{"g": identity})
	if decision, decided, err := NoChildDecisionLocked(second); err != nil || !decided || decision.Disposition != DispositionReusableSuccess {
		t.Fatalf("candidate-two did not reuse candidate-one success: decision=%+v decided=%t err=%v", decision, decided, err)
	}
}

func TestLiveComponentBlocksAnotherCandidateTree(t *testing.T) {
	root, baseIdentity := proofAttemptFixture(t, "testing")
	launcher, err := CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	identity := strings.Repeat("a", 64)
	first := componentAdmissionRequest(root, baseIdentity, launcher, now, strings.Repeat("1", 40), "first", map[string]string{"g": identity})
	live, _, err := ReserveLocked(first)
	if err != nil {
		t.Fatal(err)
	}
	second := componentAdmissionRequest(root, baseIdentity, launcher, now.Add(time.Second), strings.Repeat("2", 40), "second", map[string]string{"g": identity})
	if decision, decided, err := NoChildDecisionLocked(second); err != nil || !decided ||
		decision.Disposition != DispositionLiveDuplicate || decision.AttemptID != live.AttemptID {
		t.Fatalf("live component did not block another tree: decision=%+v decided=%t err=%v", decision, decided, err)
	}
}

func TestRetryDecisionPriorMustBeOnTheRequestsTree(t *testing.T) {
	root, baseIdentity := proofAttemptFixture(t, "testing")
	launcher, err := CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	identity := strings.Repeat("a", 64)
	first := componentAdmissionRequest(root, baseIdentity, launcher, now, strings.Repeat("1", 40), "same-plan", map[string]string{"g": identity})
	prior := retainComponentAttempt(t, first, first.CandidateTree, []componentStatus{{"g", "failed"}}, now.Add(time.Second))
	evidencePath := filepath.Join(root, "failure.log")
	if err := os.WriteFile(evidencePath, []byte("reviewed failure\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	decisionPath := filepath.Join(root, "retry.json")
	encoded, _ := json.Marshal(RetryDecision{SchemaVersion: 1, PriorAttempt: prior.AttemptID, Cause: "deterministic failure",
		EvidencePath: filepath.Base(evidencePath), Rationale: "the prior failure was reviewed"})
	if err := os.WriteFile(decisionPath, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	request := componentAdmissionRequest(root, baseIdentity, launcher, now.Add(2*time.Second), strings.Repeat("2", 40), "same-plan", map[string]string{"g": identity})
	if _, err := readRetryDecision(decisionPath, root, request, prior.ProofIdentity.IdentityDigest, prior.AttemptID); err == nil ||
		!strings.Contains(err.Error(), "RETRY_PRIOR_OUTSIDE_TREE") {
		t.Fatalf("cross-tree retry prior was not refused by token: %v", err)
	}
}

func TestAttemptRejectsCandidateTreeDisagreement(t *testing.T) {
	root, baseIdentity := proofAttemptFixture(t, "testing")
	launcher, err := CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	identity := strings.Repeat("a", 64)
	request := componentAdmissionRequest(root, baseIdentity, launcher, now, strings.Repeat("1", 40), "mismatch", map[string]string{"g1": identity})
	attempt, decision, err := ReserveLocked(candidateAdmission(request))
	if err != nil || decision.Disposition != DispositionExecuted {
		t.Fatalf("reserve candidate-bound attempt: decision=%+v err=%v", decision, err)
	}
	result := componentAttemptResult(attempt.AttemptID, "g1", identity, "failed")
	result.CandidateTree = strings.Repeat("2", 40)
	result.RecomputeDelivery()
	if _, err := FinalizeAttemptWithTestResultLocked(root, attempt.AttemptID, TerminalFailed, 23, "red", nil, &result, now.Add(time.Second)); err == nil ||
		!strings.Contains(err.Error(), "testing evidence names candidate tree") {
		t.Fatalf("candidate-tree disagreement was retained: %v", err)
	}
	stored, err := ReadAttempt(root, attempt.AttemptID)
	if err != nil || stored.Terminal != nil || stored.TestResult != nil {
		t.Fatalf("failed candidate-tree binding changed the retained attempt: attempt=%+v err=%v", stored, err)
	}
}

func TestEjectAndReproveCandidateScopesRetryFence(t *testing.T) {
	root, baseIdentity := proofAttemptFixture(t, "testing")
	launcher, err := CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	treeOne, treeTwo := strings.Repeat("1", 40), strings.Repeat("2", 40)
	failedOne, changedOne, passedTwo := strings.Repeat("a", 64), strings.Repeat("c", 64), strings.Repeat("b", 64)
	first := componentAdmissionRequest(root, baseIdentity, launcher, now, treeOne, "tree-one",
		map[string]string{"g1": failedOne, "g2": passedTwo})
	prior := retainComponentAttempt(t, first, treeOne, []componentStatus{{"g1", "failed"}, {"g2", "passed"}}, now.Add(time.Second))

	newTree := componentAdmissionRequest(root, baseIdentity, launcher, now.Add(2*time.Second), treeTwo, "tree-two",
		map[string]string{"g1": changedOne, "g2": passedTwo})
	if decision, noChild, err := NoChildDecisionLocked(candidateAdmission(newTree)); err != nil || noChild {
		t.Fatalf("eject-and-reprove tree was not admitted: decision=%+v noChild=%t err=%v", decision, noChild, err)
	}
	sameTree := componentAdmissionRequest(root, baseIdentity, launcher, now.Add(2*time.Second), treeOne, "tree-one-retry",
		map[string]string{"g1": failedOne, "g2": passedTwo})
	if decision, noChild, err := NoChildDecisionLocked(candidateAdmission(sameTree)); err != nil || !noChild || decision.ExitStatus != ExitRetryRequired || decision.PriorAttempt != prior.AttemptID {
		t.Fatalf("same-tree red did not require its typed retry decision: decision=%+v noChild=%t err=%v", decision, noChild, err)
	}

	evidencePath := filepath.Join(root, "failure.log")
	if err := os.WriteFile(evidencePath, []byte("failed group diagnosed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	decisionPath := filepath.Join(root, "retry.json")
	encoded, _ := json.Marshal(RetryDecision{SchemaVersion: 1, PriorAttempt: prior.AttemptID, Cause: "deterministic group failure",
		EvidencePath: filepath.Base(evidencePath), Rationale: "the failing group input has been reviewed"})
	if err := os.WriteFile(decisionPath, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	sameTree.RetryDecisionPath = decisionPath
	retry, decision, err := ReserveLocked(candidateAdmission(sameTree))
	if err != nil || decision.Disposition != DispositionExecuted || retry.PreviousAttempt != prior.AttemptID || retry.Retry == nil {
		t.Fatalf("typed same-tree retry decision did not open execution: retry=%+v decision=%+v err=%v", retry, decision, err)
	}
}

type componentStatus struct {
	id, status string
}

func componentAdmissionRequest(root string, base ProofIdentity, launcher ProcessIdentity, now time.Time, tree, plan string, identities map[string]string) AdmissionRequest {
	inputs := []string{"plan:" + plan}
	for id, identity := range identities {
		inputs = append(inputs, "group:"+id+":"+identity)
	}
	return AdmissionRequest{ControlRoot: root, ExecutionRoot: root, GoalID: "goal-a", GoalRevision: 2, AccountingRevision: 2,
		CandidateGoalID: "goal-a", CandidateRevision: 2, CandidateTree: tree,
		ReservedMinutes: 2, Identity: BindIdentityInputs(base, inputs), Launcher: launcher, Now: now, ComponentIdentities: identities}
}

func retainComponentAttempt(t *testing.T, request AdmissionRequest, tree string, statuses []componentStatus, ended time.Time) Attempt {
	t.Helper()
	attempt, decision, err := ReserveLocked(candidateAdmission(request))
	if err != nil || decision.Disposition != DispositionExecuted {
		t.Fatalf("reserve component attempt: decision=%+v err=%v", decision, err)
	}
	result := componentAttemptResult(attempt.AttemptID, statuses[0].id, request.ComponentIdentities[statuses[0].id], "passed")
	result.CandidateTree = tree
	result.Groups = nil
	result.SelectedGroups, result.RequiredGroups = nil, nil
	result.LaunchCounts.Test = 0
	for _, observed := range statuses {
		group := componentAttemptResult(attempt.AttemptID, observed.id, request.ComponentIdentities[observed.id], "passed").Groups[0]
		switch observed.status {
		case "failed":
			exit := 23
			group.Status, group.NativeExitStatus = "failed", &exit
		case "not-run":
			group.Status, group.NotRunReason = "not-run", "delivery attempt stopped at the first failed group failed"
			group.NativeLaunched, group.CollectionComplete, group.NativeExitStatus = false, false, nil
		case "blocked":
			result.SchemaVersion = TestResultSchemaVersion
			applyCurrentWorkerPolicy(&result)
			group.IdentityVersion = GroupExecutionIdentityVersion
			group.Status, group.NotRunReason, group.BlockingGroups = "blocked", "prerequisite failed: harness", []string{"harness"}
			group.NativeLaunched, group.CollectionComplete, group.NativeExitStatus = false, false, nil
		}
		result.Groups = append(result.Groups, group)
		result.SelectedGroups = append(result.SelectedGroups, observed.id)
		result.RequiredGroups = append(result.RequiredGroups, observed.id)
		if observed.status != "not-run" && observed.status != "blocked" {
			result.LaunchCounts.Test++
		}
	}
	if result.SchemaVersion == TestResultSchemaVersion {
		for index := range result.Groups {
			result.Groups[index].IdentityVersion = GroupExecutionIdentityVersion
		}
	}
	result.StoppedAtFirstFailure = stoppedAtFirstFailure(result.Groups)
	result.RecomputeDelivery()
	retained, err := FinalizeAttemptWithTestResultLocked(request.ControlRoot, attempt.AttemptID, TerminalFailed, 23, "red", nil, &result, ended)
	if err != nil {
		t.Fatal(err)
	}
	return retained
}

func TestExactReusableTestResultPreservesCommittedOuterOwner(t *testing.T) {
	root, identity := proofAttemptFixture(t, "testing")
	launcher, err := CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	groupIdentity := strings.Repeat("7", 64)
	attempt, _, err := ReserveLocked(candidateAdmission(AdmissionRequest{ControlRoot: root, ExecutionRoot: root, GoalID: "goal-a",
		GoalRevision: 2, AccountingRevision: 2, ReservedMinutes: 2, Identity: identity, Launcher: launcher, Now: now}))

	if err != nil {
		t.Fatal(err)
	}
	if _, err := BindJoinedTestComponentsLocked(root, attempt.AttemptID, map[string]string{"application": groupIdentity}); err != nil {
		t.Fatal(err)
	}
	result := componentAttemptResult(attempt.AttemptID, "application", groupIdentity, "passed")
	payload := json.RawMessage(`{"schemaVersion":2,"marker":"byte-exact"}`)
	if _, err := FinalizeAttemptWithTestResultLocked(root, attempt.AttemptID, TerminalSuccess, 0, "green", payload, &result, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	attempts, err := ReadAttempts(root)
	if err != nil {
		t.Fatal(err)
	}
	template := result
	template.AttemptID = ""
	exact, ok := ExactReusableTestResult(template, attempts, map[string]string{"application": groupIdentity}, "goal-a", 2)
	if !ok || exact.AttemptID != attempt.AttemptID || exact.EndedAt != result.EndedAt {
		t.Fatalf("exact reuse lost its terminal owner or original result: ok=%v result=%+v", ok, exact)
	}
	composed := ReusedTestResult(template, attempts, map[string]string{"application": groupIdentity},
		testpolicy.Contract{Groups: []testpolicy.Group{{ID: "application", Kind: "unit", Inputs: []string{"source"}}}})
	if composed.AttemptID != "" {
		t.Fatalf("component composition invented an outer owner: %+v", composed)
	}
}

func TestJoinedTestingResultAttachesWithoutClosingParent(t *testing.T) {
	root, identity := proofAttemptFixture(t, "parent")
	launcher, err := CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	attempt, _, err := ReserveLocked(candidateAdmission(AdmissionRequest{ControlRoot: root, ExecutionRoot: root, GoalID: "goal-a", GoalRevision: 2,
		AccountingRevision: 2, ReservedMinutes: 2, Identity: identity, Launcher: launcher, Now: now}))

	if err != nil {
		t.Fatal(err)
	}
	result := componentAttemptResult(attempt.AttemptID, "joined", strings.Repeat("4", 64), "passed")
	attached, err := RecordTestResult(root, attempt.AttemptID, result)
	if err != nil || attached.Terminal != nil || attached.TestResult == nil {
		t.Fatalf("joined result did not remain subordinate to the live parent: attempt=%+v err=%v", attached, err)
	}
	if _, err := RecordTestResult(root, attempt.AttemptID, result); err == nil {
		t.Fatal("joined result silently replaced prior evidence")
	}
	final, err := FinalizeAttempt(root, attempt.AttemptID, TerminalSuccess, 0, "parent complete", nil, now.Add(time.Second))
	if err != nil || final.TestResult == nil || final.Terminal == nil || final.Terminal.Result != TerminalSuccess {
		t.Fatalf("parent terminal transition lost its joined test result: attempt=%+v err=%v", final, err)
	}
}

func componentAttemptResult(attemptID, groupID, executionIdentity, status string) TestResult {
	digest := strings.Repeat("a", 64)
	exit := 0
	group := GroupResult{ID: groupID, Kind: "unit", InputManifest: []string{"source"}, ExecutionIdentity: executionIdentity,
		Status: status, NativeLaunched: true, CollectionComplete: true, NativeExitStatus: &exit,
		ToolIdentities: map[string]string{}, ReportDigests: map[string]string{}}
	if status == "failed" {
		exit = 23
	}
	result := TestResult{SchemaVersion: 1, CandidateEngineIdentityVersion: CandidateEngineIdentitySchemaVersion,
		AttemptID: attemptID, Purpose: "delivery", RequestedMode: "auto", RequiredMode: "standard",
		ExecutedMode: "standard", ProjectRoot: "/project", BaseCommit: "base", CandidateTree: strings.Repeat("b", 40),
		ContractDigest: digest, BaseContractDigest: digest, PolicyEngineDigest: digest, CandidateEngineDigest: digest, BehaviorPolicyDigest: digest, PlanDigest: digest,
		CandidateEngineBuildIdentity: strings.Repeat("c", 40),
		SelectedGroups:               []string{groupID}, RequiredGroups: []string{groupID}, Groups: []GroupResult{group},
		LaunchCounts: LaunchCounts{Test: 1, CountsComplete: true}, Cost: TestCost{DeclaredTargetMS: 1}}
	result.RecomputeDelivery()
	return result
}

func proofAttemptFixture(t *testing.T, commandClass string) (string, ProofIdentity) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "scripts", "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("dispatch.cap-max=120\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"coverage-ratchet.json", "coverage-ratchet-linux.json"} {
		if err := os.WriteFile(filepath.Join(root, "scripts", "agents", name), []byte(`{"floors":{"internal/proofrun":1},"exempt":{}}`), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	identity, err := BuildProofIdentity(root, filepath.Join(root, "metasystem.conf"), "full", commandClass, []string{"gate"}, behaviorsurface.SupportedVersion)
	if err != nil {
		t.Fatal(err)
	}
	return root, identity
}

func TestProofConfigurationToleratesTemplateModelKey(t *testing.T) {
	conf := filepath.Join(t.TempDir(), "metasystem.conf")
	template := "role.code-critic.runtime=<runtime>\nrole.code-critic.model.<runtime>=<model>\n"
	if err := os.WriteFile(conf, []byte(template), 0o600); err != nil {
		t.Fatal(err)
	}
	writeLocal := func(model string) {
		t.Helper()
		if err := os.WriteFile(conf+".local", []byte("role.code-critic.runtime=claude\nrole.code-critic.model.claude="+model+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	writeLocal("model-one")
	first, err := effectiveProofConfigurationDigest(conf, nil)
	if err != nil {
		t.Fatalf("template metadata prevented effective local configuration: %v", err)
	}
	writeLocal("model-two")
	second, err := effectiveProofConfigurationDigest(conf, nil)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("effective model change did not change proof configuration")
	}
	overridden, err := effectiveProofConfigurationDigest(conf, []string{"METASYSTEM_ROLE_CODE_CRITIC_MODEL_CLAUDE=model-one"})
	if err != nil {
		t.Fatal(err)
	}
	if overridden != first {
		t.Fatal("environment model override did not restore the same effective configuration")
	}
	if err := os.WriteFile(conf, []byte(template+"unexpected.<key>=value\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := effectiveProofConfigurationDigest(conf, nil); err == nil {
		t.Fatal("unrecognized invalid configuration key was silently accepted")
	}
}

// TestAnAttemptPastItsReservationStillLaunchesAuthenticatesAndFinalizes is
// row 10 of the hang-detection design: the deadline is a reservation
// figure. An attempt reserved for two minutes three minutes ago still
// authorises a child, authenticates its parent locator, and commits a
// success accounted at the minutes it ran.
func TestAnAttemptPastItsReservationStillLaunchesAuthenticatesAndFinalizes(t *testing.T) {
	root, proofIdentity := proofAttemptFixture(t, "past-reservation")
	launcher, err := CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	attempt, _, err := ReserveLocked(candidateAdmission(AdmissionRequest{ControlRoot: root, ExecutionRoot: root, GoalID: "goal-a", GoalRevision: 2,
		AccountingRevision: 2, ReservedMinutes: 2, Identity: proofIdentity, Launcher: launcher, Now: now.Add(-3 * time.Minute)}))

	if err != nil {
		t.Fatal(err)
	}
	if err := attemptLaunchAllowedLocked(root, attempt.AttemptID, launcher.Ref(), now); err != nil {
		t.Fatalf("a child was refused past the reservation: %v", err)
	}
	if _, err := AuthenticateContext(root, attempt.AttemptID, int64(os.Getpid())); err != nil {
		t.Fatalf("a parent locator was refused past the reservation: %v", err)
	}
	finished, err := FinalizeAttempt(root, attempt.AttemptID, TerminalSuccess, 0, "green, late", nil, now)
	if err != nil || finished.Terminal == nil || finished.Terminal.Result != TerminalSuccess || finished.ObservedMinutes != 3 {
		t.Fatalf("a success past the reservation = %+v, %v", finished.Terminal, err)
	}
}

func TestReserveLockedRefusesGovernedReservationWhoseCapHasPassed(t *testing.T) {
	root, proofIdentity := proofAttemptFixture(t, "passed-governed-cap")
	launcher, err := CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
	ownerDeadline := now.Add(-time.Minute)
	owner := &ReservationOwner{
		ControlRoot: root, RunID: "governed-proof", RunGeneration: 1, LaunchNonce: "nonce",
		GoalRevision: 2, ObligationRevision: 1, AttemptOrdinal: 1,
		Deadline: ownerDeadline.Format(time.RFC3339Nano),
	}
	_, _, err = ReserveLocked(candidateAdmission(AdmissionRequest{
		ControlRoot: root, ExecutionRoot: root, GoalID: "goal-a", GoalRevision: 2, AccountingRevision: 2,
		ReservedMinutes: 2, Identity: proofIdentity, Launcher: launcher, ReservationOwner: owner, Now: now,
	}))

	if err == nil || !strings.Contains(err.Error(), "governed reservation owner's cap has passed") ||
		!strings.Contains(err.Error(), ownerDeadline.Format(time.RFC3339Nano)) || !strings.Contains(err.Error(), now.Format(time.RFC3339Nano)) {
		t.Fatalf("passed governed cap refusal = %v", err)
	}
	var passed *ReservationOwnerCapPassedError
	if !errors.As(err, &passed) || !passed.OwnerDeadline.Equal(ownerDeadline) || !passed.RequestNow.Equal(now) {
		t.Fatalf("passed governed cap did not retain typed facts: %#v, %v", passed, err)
	}
}
