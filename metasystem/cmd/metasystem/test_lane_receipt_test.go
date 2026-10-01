package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testrun"
)

// TestLaneProofWhoseGroupsPassIsRecordedGreen: a lane proof runs in a
// detached worktree of the lane checkout and keeps its attempt records at
// the lane's control root, a nested installation (checkout/metasystem) that
// is not the worktree's. When every group passes, the run's receipt is
// prepared from the attempt where it was written, and the attempt is
// committed a success carrying it — the green that landing push accepts. A
// seat proof, whose control root is its own installation, is unchanged.
func TestLaneProofWhoseGroupsPassIsRecordedGreen(t *testing.T) {
	git := func(dir string, args ...string) string {
		t.Helper()
		command := exec.Command("git", append([]string{"-C", dir}, args...)...)
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, output)
		}
		return strings.TrimSpace(string(output))
	}
	write := func(path, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := testexec.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	canonical := func(path string) string {
		t.Helper()
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			t.Fatal(err)
		}
		return resolved
	}
	checkout := canonical(t.TempDir())
	git(checkout, "init", "-q", "-b", "main")
	git(checkout, "config", "user.name", "lane-fixture")
	git(checkout, "config", "user.email", "lane@example.invalid")
	write(filepath.Join(checkout, "metasystem", "metasystem.conf"), "metasystem.runtimes=fake\ntesting.contract=testing.json\n")
	write(filepath.Join(checkout, "metasystem", "testing.json"), "{}\n")
	write(filepath.Join(checkout, "metasystem", "app", "a.txt"), "green a\n")
	write(filepath.Join(checkout, ".gitignore"), "metasystem/artifacts/\nmetasystem/records/\nmetasystem/memory/\n")
	git(checkout, "add", ".")
	git(checkout, "commit", "-qm", "lane checkout")
	tree := git(checkout, "rev-parse", "HEAD^{tree}")
	head := git(checkout, "rev-parse", "HEAD")
	worktree := filepath.Join(canonical(t.TempDir()), "proof")
	git(checkout, "worktree", "add", "-q", "--detach", worktree, "HEAD")

	controlRoot := filepath.Join(checkout, "metasystem")
	installation := filepath.Join(worktree, "metasystem")
	prepared := testrun.Preparation{Installation: installation, ControlRoot: controlRoot, ProjectRoot: worktree,
		ConfPath: filepath.Join(installation, "metasystem.conf"), CandidateTree: tree}
	if prepared.ProofControlRoot() == prepared.Installation {
		t.Fatal("the fixture's lane proof does not keep its records apart from its worktree")
	}

	identity, err := proofrun.BuildProofIdentity(worktree, prepared.ConfPath, "selected", "testing", nil, 2)
	if err != nil {
		t.Fatal(err)
	}
	launcher, err := proofrun.CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	episode, binding := strings.Repeat("1", 64), strings.Repeat("2", 64)
	expires := now.Add(time.Hour)
	request := proofrun.AdmissionRequest{ControlRoot: controlRoot, ExecutionRoot: worktree, CandidateTree: tree,
		GoalID: "goal", GoalRevision: 2, AccountingRevision: 2, CandidateGoalID: "goal", CandidateRevision: 2,
		ReservedMinutes: 5, Identity: identity, Launcher: launcher, Now: now,
		SharedComponents: true, ComponentIdentities: map[string]string{"application": strings.Repeat("a", 64)},
		FreshGroups: map[string]bool{"application": true}, FreshnessEpisode: episode, FreshnessBinding: binding,
		FreshnessExpiresAt: expires.Format(time.RFC3339Nano)}
	request = proofrun.WithTestHostAdmissionDirectory(proofrun.WithTestHostLoadSampler(request, "0"), filepath.Join(t.TempDir(), "host-admission"))
	attempt, decision, err := proofrun.ReserveLocked(request)
	if err != nil || decision.Disposition == proofrun.DispositionAdmissionRefused {
		t.Fatalf("reserve the lane proof: decision=%+v err=%v", decision, err)
	}
	if _, err := proofrun.ReadAttempt(installation, attempt.AttemptID); err == nil {
		t.Fatal("the fixture's attempt landed in the proof worktree, not the lane's control root")
	}

	zero, admissionMaximum := 0, 0
	digest := strings.Repeat("a", 64)
	result := proofrun.TestResult{SchemaVersion: proofrun.TestResultSchemaVersion,
		WorkerPolicyVersion: proofrun.TestWorkerPolicyVersion, Workers: 1, AdmissionMaximum: &admissionMaximum,
		CandidateEngineIdentityVersion: proofrun.CandidateEngineIdentitySchemaVersion, AttemptID: attempt.AttemptID,
		FreshnessEpisode: episode, FreshnessBinding: binding, FreshnessExpiresAt: expires.Format(time.RFC3339Nano), FreshGroups: map[string]bool{"application": true},
		Purpose: testpolicy.PurposeDelivery, RequestedMode: testpolicy.ModeAuto, RequiredMode: testpolicy.ModeStandard, ExecutedMode: testpolicy.ModeStandard,
		ProjectRoot: worktree, BaseCommit: head, CandidateTree: tree, PolicyBaseCommit: head,
		ContractDigest: digest, BaseContractDigest: digest, PolicyEngineDigest: digest, CandidateEngineDigest: strings.Repeat("e", 64),
		CandidateEngineBuildIdentity: strings.Repeat("f", 40), BehaviorPolicyDigest: digest, PlanDigest: digest,
		RequiredGroups: []string{"application"}, SelectedGroups: []string{"application"}, LaunchCounts: proofrun.LaunchCounts{Test: 1, CountsComplete: true},
		StartedAt: now.Add(-2 * time.Second).Format(time.RFC3339Nano), Cost: proofrun.TestCost{DeclaredTargetMS: 1},
		Groups: []proofrun.GroupResult{{ID: "application", Kind: "unit", Obligations: []string{"behavior"}, IdentityVersion: proofrun.GroupExecutionIdentityVersion,
			InputDigest: digest, InputManifest: []string{"app/**"}, ExecutionIdentity: digest, CWD: ".", ToolIdentities: map[string]string{},
			Status: "passed", NativeLaunched: true, NativeExitStatus: &zero, CollectionComplete: true, ReportDigests: map[string]string{}}}}
	result.RecomputeDelivery()
	if !result.Delivery.Sufficient {
		t.Fatalf("the fixture's passing result is not sufficient: %+v", result.Delivery)
	}

	completedAt := now.Add(time.Second)
	receipt, payload, err := prepareTestingRunReceipt(prepared, result, completedAt)
	if err != nil || receipt.Testing == nil || len(receipt.AttemptIDs) != 1 || receipt.AttemptIDs[0] != attempt.AttemptID {
		t.Fatalf("a lane proof whose groups all passed was not recorded green: receipt=%+v err=%v", receipt, err)
	}
	if _, err := proofrun.FinalizeAttemptWithTestResultLocked(controlRoot, attempt.AttemptID, proofrun.TerminalSuccess, 0,
		"proof launcher completed", payload, receipt.Testing, completedAt); err != nil {
		t.Fatal(err)
	}
	committed, err := proofrun.ReadAttempt(controlRoot, attempt.AttemptID)
	if err != nil || committed.Terminal == nil || committed.Terminal.Result != proofrun.TerminalSuccess ||
		committed.TestResult == nil || !committed.TestResult.Delivery.Sufficient || len(proofrun.CommittedDeliveryReceipt(committed)) == 0 {
		t.Fatalf("the lane proof's attempt is not a committed green with its receipt: attempt=%+v err=%v", committed, err)
	}

	// A seat proof keeps its records in its own installation: the same
	// preparation reads them there, as before.
	seat := prepared
	seat.ControlRoot = ""
	if _, _, err := prepareTestingRunReceipt(seat, result, completedAt); err == nil || !strings.Contains(err.Error(), "no successful terminal outer attempt") {
		t.Fatalf("a seat proof read attempt records outside its installation: %v", err)
	}
}
