package testrun

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestRunOwnerSurvivesTestingWorkerFilters(t *testing.T) {
	const owner = "outer-exact-ref"
	prepared := Environment([]string{"PATH=/fixture/bin", identity.RunOwnerEnv + "=" + owner, "UNRELATED=drop"})
	if joined := strings.Join(prepared, "\n"); !strings.Contains(joined, identity.RunOwnerEnv+"="+owner) || strings.Contains(joined, "UNRELATED=") {
		t.Fatalf("testing environment filtered the run owner incorrectly: %v", prepared)
	}
	worker := InheritedEnvironment([]string{"PATH=/fixture/bin"}, []string{
		"METASYSTEM_PROOF_CONTROL_ROOT=/proof", identity.RunOwnerEnv + "=" + owner,
		identity.FixtureAttemptEnv + "=attempt-a", "UNRELATED=drop",
	})
	if joined := strings.Join(worker, "\n"); !strings.Contains(joined, identity.RunOwnerEnv+"="+owner) ||
		!strings.Contains(joined, identity.FixtureAttemptEnv+"=attempt-a") || strings.Contains(joined, "UNRELATED=") {
		t.Fatalf("worker inheritance filtered the run owner incorrectly: %v", worker)
	}
}

func TestVerifyRecoversCandidateDigestFromNewestSufficientAttempt(t *testing.T) {
	const groupID = "candidate-bed"
	digest := strings.Repeat("a", 64)
	executionIdentity := strings.Repeat("b", 64)
	candidateDigest := strings.Repeat("c", 64)
	buildIdentity := strings.Repeat("f", 40)
	candidateTree := strings.Repeat("e", 40)
	group := testpolicy.Group{ID: groupID, Kind: "unit", CWD: ".", Inputs: []string{"source.go"},
		Obligations: []string{"candidate-engine"}, Platforms: []string{"any"}, TargetMS: 1}
	contract := testpolicy.Contract{SchemaVersion: 1, Groups: []testpolicy.Group{group}}
	plan := testpolicy.Plan{Purpose: testpolicy.PurposeDelivery, RequestedMode: testpolicy.ModeAuto,
		RequiredMode: testpolicy.ModeStandard, ExecutedMode: testpolicy.ModeStandard,
		RequiredGroups: []string{groupID}, SelectedGroups: []string{groupID}}
	prepared := Preparation{CandidateTree: candidateTree, EffectiveContract: contract, Plan: plan,
		ContractDigest: digest, BaseContractDigest: digest, PolicyEngineDigest: digest, BehaviorPolicyDigest: digest,
		GoalID: "goal", AccountingRevision: 2}
	request := RunRequest(prepared, "successful-attempt", "", "", candidateDigest, buildIdentity)
	request.ProjectRoot, request.BaseCommit = "/project", "base"
	successful := proofrun.NewTestResult(request)
	zero := 0
	successful.Groups = []proofrun.GroupResult{{ID: groupID, Kind: group.Kind, Obligations: group.Obligations,
		InputManifest: group.Inputs, ExecutionIdentity: executionIdentity, Status: "passed", NativeLaunched: true,
		CollectionComplete: true, NativeExitStatus: &zero, ToolIdentities: map[string]string{}, ReportDigests: map[string]string{}}}
	// A sufficient attempt from an earlier plan remains a valid source for
	// the deterministic candidate engine; its groups are reused only while no
	// newer observation at the same identity contradicts them.
	successful.CandidateTree = strings.Repeat("9", 40)
	successful.PlanDigest = strings.Repeat("1", 64)
	successful.RecomputeDelivery()
	failed := successful
	failed.AttemptID = "later-failed-attempt"
	// A red battery is still a completed measurement of the deterministic
	// candidate engine. Carried landing needs its structured insufficiency;
	// sufficiency remains the later delivery decision, not an engine-identity
	// precondition.
	failed.CandidateEngineDigest = strings.Repeat("d", 64)
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
			StartedAt: now.Format(time.RFC3339Nano), Terminal: &proofrun.AttemptTerminal{Result: proofrun.TerminalFailed},
			PendingTestGroups: map[string]string{groupID: executionIdentity}, TestResult: &failed},
	}
	recovered, err := RetainedCandidateEngineDigest(prepared, attempts, buildIdentity, false)
	if err != nil || recovered != candidateDigest {
		t.Fatalf("later failed attempt hid the sufficient candidate engine: digest=%s err=%v", recovered, err)
	}
	carriedDigest, err := RetainedCandidateEngineDigest(prepared, attempts, buildIdentity, true)
	if err != nil || carriedDigest != failed.CandidateEngineDigest {
		t.Fatalf("carried verification did not retain the newest completed red measurement: digest=%s err=%v", carriedDigest, err)
	}
	templateRequest := RunRequest(prepared, "", "", "", recovered, buildIdentity)
	templateRequest.ProjectRoot, templateRequest.BaseCommit = "/project", "base"
	projection := proofrun.ReusedTestResult(proofrun.NewTestResult(templateRequest), attempts,
		map[string]string{groupID: executionIdentity}, contract)
	// The later attempt failed the same group at the same identity: that is
	// the newest observation, so the earlier pass is not reused (green then
	// red yields no reuse) while the candidate digest above is still recovered.
	if projection.Delivery.Sufficient || len(projection.Groups) != 1 || projection.Groups[0].Status != "not-run" || projection.Groups[0].NotRunReason != "newest-observation-failed" {
		t.Fatalf("verification composed the earlier pass although a newer attempt failed the group at the same identity: %+v", projection)
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
	prepared := Preparation{Installation: t.TempDir(), CandidateTree: candidateTree, EffectiveContract: contract, Plan: plan,
		ContractDigest: digest, BaseContractDigest: digest, PolicyEngineDigest: digest, BehaviorPolicyDigest: digest,
		GoalID: "goal", AccountingRevision: 2}
	request := RunRequest(prepared, "legacy-success", "", "", digest, strings.Repeat("c", 40))
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
	recovered, err := RetainedCandidateEngineDigest(prepared, nil, strings.Repeat("d", 40), false)
	if err == nil || recovered != "" || !strings.Contains(err.Error(), "candidate engine digest is absent") {
		t.Fatalf("legacy whole-tree receipt unexpectedly supplied a cross-tip engine identity: digest=%s err=%v", recovered, err)
	}
}

func TestDiagnosticsReadersFollowTheCandidatePair(t *testing.T) {
	candidate := proofrun.Attempt{SchemaVersion: proofrun.CandidateAttemptSchemaVersion,
		GoalID: "authority-c", AccountingRevision: 3, CandidateGoalID: "candidate-x", CandidateRevision: 7}
	if !AttemptAccountsFor(candidate, "candidate-x", 7) {
		t.Fatal("candidate-owned diagnostic attempt was not selected")
	}
	if AttemptAccountsFor(candidate, "authority-c", 3) {
		t.Fatal("diagnostic reader selected the authority pair instead of the candidate pair")
	}
	legacy := proofrun.Attempt{SchemaVersion: proofrun.AttemptSchemaVersion, GoalID: "legacy", AccountingRevision: 5}
	if !AttemptAccountsFor(legacy, "legacy", 5) {
		t.Fatal("diagnostic reader stopped selecting schema-2 authority accounting")
	}
}
