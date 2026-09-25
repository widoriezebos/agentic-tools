package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

// workerAuthorizedFileAttemptFixture retains a real worker-root reservation
// over ordinary files for tests that do not exercise a section worktree.
func workerAuthorizedFileAttemptFixture(t *testing.T, candidateTree ...string) (string, string, proofrun.Attempt) {
	t.Helper()
	tree := ""
	if len(candidateTree) != 0 {
		tree = candidateTree[0]
	}
	controlRoot, err := canonicalProofRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return workerAuthorizedFileAttemptFixtureAt(t, controlRoot, tree)
}

func workerAuthorizedFileAttemptFixtureAt(t *testing.T, controlRoot, tree string) (string, string, proofrun.Attempt) {
	t.Helper()
	executionRoot, err := canonicalProofRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	conf := filepath.Join(controlRoot, "metasystem.conf")
	if err := os.WriteFile(conf, []byte("metasystem.runtimes=fake\nmetasystem.governance.correlation-policy=A\nmetasystem.budget.tier-3=8h/1/1200m/1/3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pinProofBinaryFixture(t, controlRoot)
	receiptAt := time.Date(2026, 8, 30, 9, 0, 0, 0, time.UTC).Add(-time.Hour)
	receipt := fmt.Sprintf("%d|%s|RECEIPT|type=implement|outcome=shipped|goal=standing-validation|note=proof fixture\n",
		receiptAt.Unix(), receiptAt.Format(time.RFC3339))
	receiptPath := filepath.Join(controlRoot, "memory", "receipts.log")
	if err := os.MkdirAll(filepath.Dir(receiptPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(receiptPath, []byte(receipt), 0o644); err != nil {
		t.Fatal(err)
	}
	commandClass := "worker-root"
	if tree != "" {
		commandClass = "testing"
	}
	proofIdentity, err := proofrun.BuildProofIdentity(controlRoot, conf, "full", commandClass, nil, 2)
	if err != nil {
		t.Fatal(err)
	}
	launcher, err := proofrun.ProcessIdentityForPID(int64(os.Getppid()), nil)
	if err != nil {
		t.Fatal(err)
	}
	attempt, decision, err := proofrun.ReserveLocked(privateProofAdmissionRequest(proofrun.WithTestHostLoadSampler(proofrun.AdmissionRequest{
		ControlRoot: controlRoot, ExecutionRoot: executionRoot, CandidateTree: tree, GoalID: "standing-validation", GoalRevision: 2,
		AccountingRevision: 2, CandidateGoalID: "standing-validation", CandidateRevision: 2,
		ReservedMinutes: 2, Identity: proofIdentity, Launcher: launcher,
		Now: time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC),
	}, "0")))
	if err != nil {
		t.Fatal(err)
	}
	if decision.Disposition == proofrun.DispositionAdmissionRefused {
		t.Fatalf("worker-authorized file attempt was admission-refused: %+v", decision)
	}
	if _, err := proofrun.ReadAttempt(controlRoot, attempt.AttemptID); err != nil {
		t.Fatalf("read retained worker-authorized attempt: %v", err)
	}
	setOwnedGoGateProcessEnvironment(t, "METASYSTEM_PROOF_CONTROL_ROOT", controlRoot)
	setOwnedGoGateProcessEnvironment(t, "METASYSTEM_PROOF_ATTEMPT", attempt.AttemptID)
	setOwnedGoGateProcessEnvironment(t, "METASYSTEM_PROOF_RECORD_KEY", "")
	setOwnedGoGateProcessEnvironment(t, "METASYSTEM_PROOF_CREATION_CLAIM", "")
	setOwnedGoGateProcessEnvironment(t, proofWitnessExecutionRootEnv, "")
	setOwnedGoGateProcessEnvironment(t, "METASYSTEM_GATE_WITNESS_WRITE", "")
	return controlRoot, executionRoot, attempt
}
