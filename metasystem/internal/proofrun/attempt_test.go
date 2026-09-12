package proofrun

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestProofRepeatDecisionProtocol(t *testing.T) {
	root, identity := proofAttemptFixture(t, "repeat")
	launcher, err := CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	request := AdmissionRequest{ControlRoot: root, ExecutionRoot: root, GoalID: "goal-a", GoalRevision: 3,
		AccountingRevision: 2, ReservedMinutes: 4, Identity: identity, Launcher: launcher, Now: now}
	attempt, result, err := ReserveLocked(request)
	if err != nil || result.Disposition != DispositionExecuted {
		t.Fatalf("initial reservation = %+v, %+v, %v", attempt, result, err)
	}
	_, duplicate, err := ReserveLocked(request)
	if err != nil || duplicate.Disposition != DispositionLiveDuplicate || duplicate.ExitStatus != ExitLiveDuplicate {
		t.Fatalf("live duplicate = %+v, %v", duplicate, err)
	}
	if _, err := FinalizeAttempt(root, attempt.AttemptID, TerminalSuccess, 0, "green", nil, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	_, reusable, err := ReserveLocked(request)
	if err != nil || reusable.Disposition != DispositionReusableSuccess || reusable.ExitStatus != ExitReusableSuccess {
		t.Fatalf("success duplicate = %+v, %v", reusable, err)
	}

	failedIdentity := identity
	failedIdentity.CommandClass = "failed-proof"
	failedIdentity.IdentityDigest = failedIdentity.digest()
	request.Identity = failedIdentity
	failed, _, err := ReserveLocked(request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := FinalizeAttempt(root, failed.AttemptID, TerminalFailed, 23, "gate failed", nil, now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	_, required, err := ReserveLocked(request)
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
	retry, executed, err := ReserveLocked(request)
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
	attempt, _, err := ReserveLocked(request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := FinalizeAttempt(root, attempt.AttemptID, TerminalFailed, 75, "child returned a generic nonzero status", nil, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	_, result, err := ReserveLocked(request)
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
	attempt, result, err := ReserveLocked(request)
	if err != nil || result.Disposition != DispositionExecuted {
		t.Fatalf("first runtime reservation = %+v %+v %v", attempt, result, err)
	}
	request.ExecutionRoot = secondRoot
	request.Identity = secondIdentity
	_, duplicate, err := ReserveLocked(request)
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
	attempt, _, err := ReserveLocked(AdmissionRequest{ControlRoot: root, ExecutionRoot: root, GoalID: "goal-a",
		GoalRevision: 2, AccountingRevision: 2, ReservedMinutes: 2, Identity: proofIdentity, Launcher: launcher, Now: now})
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
	staleAttempt, _, err := ReserveLocked(AdmissionRequest{ControlRoot: root, ExecutionRoot: root, GoalID: "goal-a",
		GoalRevision: 2, AccountingRevision: 2, ReservedMinutes: 2, Identity: staleIdentity, Launcher: staleLauncher, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AuthenticateContext(root, staleAttempt.AttemptID, int64(os.Getpid())); err == nil {
		t.Fatal("stale launcher identity authenticated a parent proof locator")
	}
	expiredIdentity := proofIdentity
	expiredIdentity.CommandClass = "expired-parent-custody"
	expiredIdentity.IdentityDigest = expiredIdentity.digest()
	expiredAttempt, _, err := ReserveLocked(AdmissionRequest{ControlRoot: root, ExecutionRoot: root, GoalID: "goal-a",
		GoalRevision: 2, AccountingRevision: 2, ReservedMinutes: 2, Identity: expiredIdentity, Launcher: launcher,
		Now: now.Add(-3 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AuthenticateContext(root, expiredAttempt.AttemptID, int64(os.Getpid())); err == nil {
		t.Fatal("expired absolute deadline authenticated a parent proof locator")
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
	attempt, _, err := ReserveLocked(AdmissionRequest{ControlRoot: root, ExecutionRoot: root, GoalID: "goal-a",
		GoalRevision: 2, AccountingRevision: 2, ReservedMinutes: 2, Identity: identity, Launcher: launcher, Now: now})
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
	path, _ := AttemptPath(root, attempt.AttemptID)
	data, err := os.ReadFile(path)
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
	newFormatWithoutDigest := result
	newFormatWithoutDigest.CandidateEngineDigest = ""
	if err := ValidateTestResult(newFormatWithoutDigest); err == nil {
		t.Fatal("new-format delivery result without its candidate engine digest was accepted")
	}

	legacyIdentity := identity
	legacyIdentity.CommandClass = "legacy"
	legacyIdentity.IdentityDigest = legacyIdentity.digest()
	legacy, _, err := ReserveLocked(AdmissionRequest{ControlRoot: root, ExecutionRoot: root, GoalID: "goal-a",
		GoalRevision: 2, AccountingRevision: 2, ReservedMinutes: 2, Identity: legacyIdentity, Launcher: launcher, Now: now})
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
	data, _ = json.Marshal(raw)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if read, err := ReadAttempt(root, legacy.AttemptID); err != nil || read.SchemaVersion != 1 || read.TestResult != nil {
		t.Fatalf("legacy schema-1 attempt was not readable without invented testing facts: %+v %v", read, err)
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
	attempt, _, err := ReserveLocked(request)
	if err != nil {
		t.Fatal(err)
	}
	passed := componentAttemptResult(attempt.AttemptID, "a", strings.Repeat("1", 64), "passed")
	if _, err := FinalizeAttemptWithTestResultLocked(root, attempt.AttemptID, TerminalSuccess, 0, "green", nil, &passed, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	request.Identity = BindIdentityInputs(baseIdentity, []string{"group:a:" + strings.Repeat("1", 64), "plan:two"})
	if decision, noChild, err := NoChildDecisionLocked(request); err != nil || !noChild || decision.ExitStatus != ExitReusableSuccess {
		t.Fatalf("successful component was not reused across a plan change: %+v noChild=%v err=%v", decision, noChild, err)
	}
	request.ComponentIdentities["new"] = strings.Repeat("2", 64)
	if decision, noChild, err := NoChildDecisionLocked(request); err != nil || noChild {
		t.Fatalf("partial component reuse suppressed missing work: %+v noChild=%v err=%v", decision, noChild, err)
	}

	failedIdentity := BindIdentityInputs(baseIdentity, []string{"group:failed:" + strings.Repeat("3", 64), "plan:failed"})
	failedRequest := request
	failedRequest.Identity = failedIdentity
	failedRequest.ComponentIdentities = map[string]string{"failed": strings.Repeat("3", 64)}
	failed, _, err := ReserveLocked(failedRequest)
	if err != nil {
		t.Fatal(err)
	}
	failedResult := componentAttemptResult(failed.AttemptID, "failed", strings.Repeat("3", 64), "failed")
	if _, err := FinalizeAttemptWithTestResultLocked(root, failed.AttemptID, TerminalFailed, 23, "red", nil, &failedResult, now.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	failedRequest.Identity = BindIdentityInputs(baseIdentity, []string{"group:failed:" + strings.Repeat("3", 64), "plan:superset"})
	if decision, noChild, err := NoChildDecisionLocked(failedRequest); err != nil || !noChild || decision.ExitStatus != ExitRetryRequired || decision.PriorAttempt != failed.AttemptID {
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

func TestExactReusableTestResultPreservesCommittedOuterOwner(t *testing.T) {
	root, identity := proofAttemptFixture(t, "testing")
	launcher, err := CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	groupIdentity := strings.Repeat("7", 64)
	attempt, _, err := ReserveLocked(AdmissionRequest{ControlRoot: root, ExecutionRoot: root, GoalID: "goal-a",
		GoalRevision: 2, AccountingRevision: 2, ReservedMinutes: 2, Identity: identity, Launcher: launcher, Now: now})
	if err != nil {
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
	attempt, _, err := ReserveLocked(AdmissionRequest{ControlRoot: root, ExecutionRoot: root, GoalID: "goal-a", GoalRevision: 2,
		AccountingRevision: 2, ReservedMinutes: 2, Identity: identity, Launcher: launcher, Now: now})
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
