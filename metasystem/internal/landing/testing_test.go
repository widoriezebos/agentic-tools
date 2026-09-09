package landing

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestSchemaTwoReceiptRequiresSuccessfulTerminalGroupOwners(t *testing.T) {
	f := newObserveFixture(t)
	f.write("metasystem.conf", "testing.contract=testing.json\ndispatch.cap-max=120\n")
	f.git("add", ".", "../development/metasystem-design.md")
	projectRoot, err := (gittree.Workspace{Dir: f.root}).TopLevel()
	if err != nil {
		t.Fatal(err)
	}
	tree, err := (gittree.Workspace{Dir: projectRoot}).StagedTree()
	if err != nil {
		t.Fatal(err)
	}
	subtree, err := (gittree.Workspace{Dir: f.root}).TreeOf(tree)
	if err != nil {
		t.Fatal(err)
	}
	head := f.git("rev-parse", "HEAD")
	identity, err := proofrun.BuildProofIdentity(f.root, filepath.Join(f.root, "metasystem.conf"), "selected", "testing", nil, behaviorsurface.SupportedVersion)
	if err != nil {
		t.Fatal(err)
	}
	launcher, err := proofrun.CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	attempt, _, err := proofrun.ReserveLocked(proofrun.AdmissionRequest{ControlRoot: f.root, ExecutionRoot: f.root,
		GoalID: "goal", GoalRevision: 2, AccountingRevision: 2, ReservedMinutes: 5, Identity: identity, Launcher: launcher, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	zero := 0
	digest := strings.Repeat("a", 64)
	result := proofrun.TestResult{SchemaVersion: proofrun.TestResultSchemaVersion, AttemptID: attempt.AttemptID,
		Purpose: testpolicy.PurposeDelivery, RequestedMode: testpolicy.ModeAuto, RequiredMode: testpolicy.ModeStandard, ExecutedMode: testpolicy.ModeStandard,
		ProjectRoot: projectRoot, BaseCommit: head, CandidateTree: tree, PolicyBaseCommit: head,
		ContractDigest: digest, BaseContractDigest: digest, PolicyEngineDigest: digest, BehaviorPolicyDigest: digest, PlanDigest: digest,
		RequiredGroups: []string{"application"}, SelectedGroups: []string{"application"}, LaunchCounts: proofrun.LaunchCounts{Test: 1, CountsComplete: true},
		StartedAt: now.Add(-2 * time.Second).Format(time.RFC3339Nano), Cost: proofrun.TestCost{DeclaredTargetMS: 1}, Groups: []proofrun.GroupResult{{ID: "application", Kind: "unit", Obligations: []string{"behavior"},
			InputDigest: digest, InputManifest: []string{"source/**"}, ExecutionIdentity: digest, CWD: ".", ToolIdentities: map[string]string{},
			Status: "passed", NativeLaunched: true, NativeExitStatus: &zero, CollectionComplete: true, ReportDigests: map[string]string{}}}}
	result.RecomputeDelivery()
	f.write("records/narrator-digest.log", "ordinary append\n")
	completedAt := now.Add(time.Second)
	preparedReceipt, payload, err := PrepareTestingReceiptPayload(f.root, tree, result, completedAt)
	if err != nil || preparedReceipt.Time != completedAt.Format(time.RFC3339Nano) {
		t.Fatalf("prepare atomic schema-2 receipt: receipt=%+v err=%v", preparedReceipt, err)
	}
	if preparedReceipt.Testing == nil || preparedReceipt.Testing.Cost.PublicationDurationMS < 1 ||
		preparedReceipt.Testing.Cost.ActualDurationMS < 2000 || preparedReceipt.Testing.EndedAt == "" {
		t.Fatalf("schema-2 terminal preparation did not retain whole-command timing: %+v", preparedReceipt.Testing)
	}
	if _, err := PublishCommittedReceipt(f.root, attempt.AttemptID); err == nil {
		t.Fatal("live schema-2 attempt published before terminal success")
	}
	if _, err := proofrun.FinalizeAttemptWithTestResultLocked(f.root, attempt.AttemptID, proofrun.TerminalSuccess, 0, "fixture", payload, preparedReceipt.Testing, completedAt); err != nil {
		t.Fatal(err)
	}
	published, err := PublishCommittedReceipt(f.root, attempt.AttemptID)
	if err != nil || published.Time != preparedReceipt.Time {
		t.Fatalf("publish atomic schema-2 receipt: receipt=%+v err=%v", published, err)
	}
	projected, err := os.ReadFile(TestReceiptPath(f.root, tree))
	if err != nil || !bytes.Equal(bytes.TrimSpace(projected), bytes.TrimSpace(payload)) {
		t.Fatalf("schema-2 projection changed committed payload: err=%v\nprojected=%s\npayload=%s", err, projected, payload)
	}
	receipt, err := CreateTestingReceipt(f.root, tree, result)
	if err != nil || receipt.SchemaVersion != 2 || len(receipt.AttemptIDs) != 1 || !fullReceiptCommandAccepted(receipt) {
		t.Fatalf("schema-2 receipt=%+v err=%v", receipt, err)
	}
	if _, err := readTestReceipt(ObserveParams{RepoRoot: f.root, CandidateTree: subtree, TestReceipt: TestReceiptPath(f.root, tree)}); err != nil {
		t.Fatalf("schema-2 receipt consumer: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(projectRoot, "source"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "source", "changed.go"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := PrepareTestingReceiptPayload(f.root, tree, result, completedAt); err == nil {
		t.Fatal("relevant source mutation accepted")
	}
	if _, err := PublishCommittedReceipt(f.root, attempt.AttemptID); err == nil {
		t.Fatal("recovery accepted relevant source mutation")
	}
	failed := result
	failed.AttemptID = "missing-attempt"
	if _, err := CreateTestingReceipt(f.root, tree, failed); err == nil {
		t.Fatal("receipt projection accepted a group without terminal outer authority")
	}
}

func TestAdoptionRulingsPreserveApplicationAndLandingAuthority(t *testing.T) {
	source := newObserveFixture(t)
	target := t.TempDir()
	data, err := AdoptionRulings(source.root, target)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "| R-1 |") || !strings.Contains(string(data), "| R-35-m0 |") || !strings.Contains(string(data), "| R-54-m1 |") {
		t.Fatalf("fresh register has incorrect authority rows: %s", data)
	}
	path := filepath.Join(target, "memory", "rulings.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	custom := "# Application history\n\n| R-900-app | tailored application ruling |\n"
	if err := os.WriteFile(path, []byte(custom), 0o644); err != nil {
		t.Fatal(err)
	}
	data, err = AdoptionRulings(source.root, target)
	if err != nil || !strings.HasPrefix(string(data), custom) {
		t.Fatalf("existing application memory changed: %s, %v", data, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	repeated, err := AdoptionRulings(source.root, target)
	if err != nil || string(repeated) != string(data) {
		t.Fatalf("re-adoption duplicates or changes rulings: %s, %v", repeated, err)
	}
	conflict := custom + "| R-35-m0 | different application authority |\n"
	if err := os.WriteFile(path, []byte(conflict), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := AdoptionRulings(source.root, target); err == nil {
		t.Fatal("conflicting authority was accepted")
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != conflict {
		t.Fatalf("conflicting target was modified: %s, %v", after, err)
	}
}
