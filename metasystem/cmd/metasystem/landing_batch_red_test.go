package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
)

func TestBatchDiagnosticDiscardsStaleResultAfterFailedRun(t *testing.T) {
	root := t.TempDir()
	resultPath := filepath.Join(root, "artifacts", "agents", "proof-runs", "batch", "batch-stale-diagnostic.json")
	if err := os.MkdirAll(filepath.Dir(resultPath), 0o755); err != nil {
		t.Fatal(err)
	}
	stale := []byte(`{"attemptId":"stale","groups":[{"id":"fast","status":"failed"}]}`)
	if err := os.WriteFile(resultPath, stale, 0o644); err != nil {
		t.Fatal(err)
	}
	original := batchDiagnosticExecute
	t.Cleanup(func() { batchDiagnosticExecute = original })
	batchDiagnosticExecute = func(string, []string, string, []string) ([]byte, int, error) {
		return []byte("runner failed"), 1, errors.New("runner failed")
	}

	result, err := launchBatchDiagnostic(root, "batch-stale", batch.DiagnosticRequest{GoalID: "goal-a", Tree: "tree-a", Groups: []string{"fast"}})
	if err == nil || len(result.Groups) != 0 {
		t.Fatalf("result=%+v error=%v", result, err)
	}
}

func TestFirstTrunkRedHoldUsesLedgerOwnerOpid(t *testing.T) {
	root := syncedClaimedGoalFixture(t)
	machine, err := goal.ResolveMachine(root)
	if err != nil {
		t.Fatal(err)
	}
	owner, err := newLedgerTrunkRedOwner(root, machine, landingOwnerLineage)
	if err != nil {
		t.Fatal(err)
	}
	head := goalSyncMutationGit(t, root, "rev-parse", "HEAD")
	goalSyncMutationGit(t, root, "update-ref", "refs/remotes/origin/main", head)
	baseTree := goalSyncMutationGit(t, root, "rev-parse", "HEAD^{tree}")
	const batchID = "01j5x00000000000000000ba90"
	store := batch.NewStore(root, nil)
	if err := store.Create(batch.Record{Schema: 1, BatchID: batchID, State: batch.StateDiagnosing, BaseTree: baseTree,
		Proof: &batch.Proof{Status: "failed", RedGroups: []batch.RedGroup{{ID: "fast", Status: "failed"}}},
		Units: []batch.Unit{{GoalID: "standing-validation", Chain: "chain-a", State: batch.UnitJoined,
			Claim: batch.Claim{Machine: machine, Lineage: landingOwnerLineage, Epoch: 1, Revision: 1, AccountingRevision: 1}}}}); err != nil {
		t.Fatal(err)
	}
	originalOwner, originalLauncher := productionTrunkRedLedgerOwner, batchDiagnosticLauncher
	t.Cleanup(func() { productionTrunkRedLedgerOwner, batchDiagnosticLauncher = originalOwner, originalLauncher })
	productionTrunkRedLedgerOwner = func(string) batch.LedgerOwner { return owner }
	batchDiagnosticLauncher = func(string, string, batch.DiagnosticRequest) (batch.DiagnosticResult, error) {
		return batch.DiagnosticResult{AttemptID: "attempt-red", Groups: []batch.RedGroup{{ID: "fast", Status: "failed"}}}, nil
	}

	if err := executeBatchDiagnosis(root, batchID, landingOwnerLineage, time.Date(2026, 9, 17, 10, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	record, err := store.Load(batchID)
	if err != nil || record.State != batch.StateHeldTrunkRed || record.TrunkRed == nil || len(record.TrunkRed.Entries) != 1 {
		t.Fatalf("record=%+v error=%v", record, err)
	}
}
