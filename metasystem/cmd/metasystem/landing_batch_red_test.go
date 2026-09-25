package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
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
	const machine = "mac-cli"
	clock := time.Date(2026, 9, 17, 10, 0, 0, 0, time.UTC)
	repository := newProofAdmissionRepositoryFixture(t, time.Date(2026, 8, 30, 9, 0, 0, 0, time.UTC), false)
	root := repository.root
	owner := proofLedgerTrunkRedOwner(t, repository, machine, landingOwnerLineage, clock.Add(time.Hour))
	baseTree, baseCommit := strings.Repeat("a", 40), strings.Repeat("b", 40)
	const batchID = "01j5x00000000000000000ba90"
	store := batch.NewStore(root, nil)
	if err := store.Create(batch.Record{Schema: 1, BatchID: batchID, State: batch.StateDiagnosing, BaseTree: baseTree,
		Proof: &batch.Proof{Status: "failed", RedGroups: []batch.RedGroup{{ID: "fast", Status: "failed"}}},
		Units: []batch.Unit{{GoalID: "standing-validation", Chain: "chain-a", State: batch.UnitJoined,
			Claim: batch.Claim{Machine: machine, Lineage: landingOwnerLineage, Epoch: 1, Revision: 1, AccountingRevision: 1}}}}); err != nil {
		t.Fatal(err)
	}
	originalOwner, originalLauncher, originalDiagnosis := productionTrunkRedLedgerOwner, batchDiagnosticLauncher, batchDiagnosisSeams
	t.Cleanup(func() {
		productionTrunkRedLedgerOwner, batchDiagnosticLauncher, batchDiagnosisSeams = originalOwner, originalLauncher, originalDiagnosis
	})
	productionTrunkRedLedgerOwner = func(gotRoot string) (batch.LedgerOwner, error) {
		if gotRoot != root {
			t.Fatalf("ledger owner root = %q, want %q", gotRoot, root)
		}
		return owner, nil
	}
	baseReads, machineReads, launches := 0, 0, 0
	batchDiagnosisSeams.commitForTree = func(gotRoot, ref, tree string) (string, error) {
		if gotRoot != root || ref != "origin/main" || tree != baseTree || baseReads != 0 {
			t.Fatalf("base commit read %d: root=%q ref=%q tree=%q", baseReads, gotRoot, ref, tree)
		}
		baseReads++
		return baseCommit, nil
	}
	batchDiagnosticLauncher = func(gotRoot, gotID string, request batch.DiagnosticRequest) (batch.DiagnosticResult, error) {
		if gotRoot != root || gotID != batchID || request.Tree != baseTree || launches != 0 {
			t.Fatalf("diagnostic launch %d: root=%q id=%q request=%+v", launches, gotRoot, gotID, request)
		}
		launches++
		return batch.DiagnosticResult{AttemptID: "attempt-red", Groups: []batch.RedGroup{{ID: "fast", Status: "failed"}}}, nil
	}
	lookup := func(gotRoot, key string) (string, error) {
		if gotRoot != root || key != "metasystem.goal.machine" || machineReads != 0 {
			t.Fatalf("machine config read %d: root=%q key=%q", machineReads, gotRoot, key)
		}
		machineReads++
		return machine, nil
	}

	if err := executeBatchDiagnosisWithConfig(root, batchID, landingOwnerLineage, clock, lookup); err != nil {
		t.Fatal(err)
	}
	if baseReads != 1 || machineReads != 1 || launches != 1 {
		t.Fatalf("raw reads and diagnostic launches: base=%d machine=%d launches=%d", baseReads, machineReads, launches)
	}
	record, err := store.Load(batchID)
	if err != nil || record.State != batch.StateHeldTrunkRed || record.TrunkRed == nil || len(record.TrunkRed.Entries) != 1 {
		t.Fatalf("record=%+v error=%v", record, err)
	}
	if record.TrunkRed.Red.BaseCommit != baseCommit || record.TrunkRed.Red.BaseTree != baseTree {
		t.Fatalf("recorded base = %q %q, want %q %q", record.TrunkRed.Red.BaseCommit, record.TrunkRed.Red.BaseTree, baseCommit, baseTree)
	}
	if _, err := owner.request(record.TrunkRed.Opid); err != nil {
		t.Fatalf("held operation is not attributed to the landing owner: %v", err)
	}
	present, err := repository.TrailerPresent(proofLedgerCanonicalTip(repository), record.TrunkRed.Opid)
	if err != nil || !present {
		t.Fatalf("held operation has no published ledger commit: present=%t error=%v", present, err)
	}
}

func TestBatchFencedDiagnosticAuthorityRequiresExactSealedHandover(t *testing.T) {
	t.Parallel()
	const batchID = "01j5x00000000000000000ba91"
	claim := batch.Claim{Machine: "source", Lineage: "source-lineage", Epoch: 3, Revision: 2, AccountingRevision: 2}
	unit := batch.Unit{GoalID: "goal-b", State: batch.UnitJoined, Claim: claim}
	record := batch.Record{BatchID: batchID, Seal: map[string]batch.Claim{"goal-b": claim}}
	file := &goal.GoalFile{State: goal.StateClaimed,
		Claimed: &goal.ClaimRecord{Machine: "landing", Lineage: landingOwnerLineage, Revision: 2, AccountingRevision: 2,
			HandedOver: goal.HandedOver{FromMachine: claim.Machine, FromLineage: claim.Lineage, FromEpoch: claim.Epoch, Batch: batchID}},
		StopFence: &goal.StopFence{StopID: "stop-goal-b-r2"}}
	projection := goal.Projection{Tree: &goal.TreeGoals{Live: map[string]*goal.GoalFile{"goal-b": file}}}
	assertFenced := func(want bool) {
		t.Helper()
		authority := authorizeBatchMemberInProjection("", time.Unix(1, 0), record, unit, projection)
		var fenced *batch.PrefixFencedRefusal
		if errors.As(authority, &fenced) != want {
			t.Fatalf("fenced authority=%v, want fenced=%t", authority, want)
		}
	}
	assertFenced(true)
	file.Claimed.Revision++
	assertFenced(false)
	file.Claimed.Revision--
	file.Claimed.HandedOver.Batch = "another-batch"
	assertFenced(false)
	file.Claimed.HandedOver.Batch = batchID
	record.Seal["goal-b"] = batch.Claim{Revision: 9, AccountingRevision: 2}
	assertFenced(false)
}
