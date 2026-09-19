package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

func TestBatchPushRejectionAppearsInStatus(t *testing.T) {
	settings, err := config.NewBatchLanding(t.TempDir(), time.Minute, func() time.Time { return time.Unix(10, 0) })
	if err != nil {
		t.Fatal(err)
	}
	record := batch.Record{BatchID: "batch", State: batch.StateLanding,
		Proof:   &batch.Proof{Status: "green", Failure: "endpoint push held: protected branch"},
		Landing: &batch.LandingProgress{BranchTip: "candidate", PushRejection: &batch.PushRejection{Text: "protected branch", At: time.Unix(4, 0).UTC().Format(time.RFC3339Nano), OriginTip: "origin"}},
	}
	view := batchRecordStatus(record, settings)
	if view.State != batch.StateLanding || view.ProofStatus != "green" || view.Reason != "endpoint push held: protected branch" {
		t.Fatalf("status=%+v", view)
	}
}

func TestFinishBatchLandingLeavesUnchangedPushRejectionQuiet(t *testing.T) {
	const batchID = "01j5x00000000000000000ba24"
	root := t.TempDir()
	record := batch.Record{Schema: 1, BatchID: batchID, State: batch.StateLanding, BaseTree: "base", TipTree: "tip",
		Units: []batch.Unit{{GoalID: "goal-a", Chain: "chain-a", State: batch.UnitJoined,
			Claim: batch.Claim{Machine: "landing", Lineage: "owner", Epoch: 1, Revision: 1, AccountingRevision: 1}}},
		Proof:   &batch.Proof{Status: "green", Failure: "endpoint push held: protected branch"},
		Landing: &batch.LandingProgress{Base: "base", BranchTip: "candidate", PushRejection: &batch.PushRejection{Text: "protected branch", At: time.Unix(4, 0).UTC().Format(time.RFC3339Nano), OriginTip: "origin"}},
	}
	store := batch.NewStore(root, nil)
	if err := store.Create(record); err != nil {
		t.Fatal(err)
	}
	if err := finishBatchLanding(root, store, batchID, "owner", time.Unix(5, 0)); err != nil {
		t.Fatalf("unchanged held tick produced output: %v", err)
	}
	stored, err := store.Load(batchID)
	if err != nil || stored.State != batch.StateLanding || stored.Landing == nil || stored.Landing.PushRejection == nil || stored.Proof.Status != "green" {
		t.Fatalf("stored=%+v error=%v", stored, err)
	}
}

func TestPrefixReceiptRetriesInfrastructureExitWithNonTerminalStatus(t *testing.T) {
	root, tree := batchPrefixReceiptTestRoot(t)
	if err := os.MkdirAll(filepath.Join(root, "artifacts", "agents", "proof-runs", "batch"), 0o755); err != nil {
		t.Fatal(err)
	}
	fake := filepath.Join(root, "fake-metasystem")
	script := `#!/usr/bin/env bash
set -euo pipefail
result=
while (( $# )); do
  if [[ "$1" == --result ]]; then result=$2; shift 2; else shift; fi
done
printf '%s\n' '{"attemptId":"interrupted-attempt","groups":[{"id":"group-a","status":"cancelled","nativeLaunched":true}]}' >"$result"
exit 2
`
	if err := testexec.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	original := batchPrefixReceiptExecutable
	t.Cleanup(func() { batchPrefixReceiptExecutable = original })
	batchPrefixReceiptExecutable = func() (string, error) { return fake, nil }
	record := batch.Record{Units: []batch.Unit{{GoalID: "goal-a", Claim: batch.Claim{Revision: 7, AccountingRevision: 5}}}}
	result, err := executeBatchPrefixReceipt(root, "batch", record, "goal-a", tree, []string{"group-a"})
	if err == nil || len(result.Red) != 0 {
		t.Fatalf("infrastructure result=%+v error=%v", result, err)
	}
}

func TestRecoveryFailureDoesNotAbortAmbientRebase(t *testing.T) {
	root := t.TempDir()
	if output, err := exec.Command("git", "init", "-q", root).CombinedOutput(); err != nil {
		t.Fatalf("init: %v: %s", err, output)
	}
	marker := filepath.Join(root, ".git", "rebase-merge", "ambient-owner")
	if err := os.MkdirAll(filepath.Dir(marker), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(marker, []byte("unrelated\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	recovery, err := reopenMovedBatchAfterRecoveryFailure(root, "batch", "tip", "origin", batch.PushRecovery{}, errors.New("landing recovery failed"))
	if err != nil || !recovery.Reopen {
		t.Fatalf("recovery=%+v error=%v", recovery, err)
	}
	if data, err := os.ReadFile(marker); err != nil || string(data) != "unrelated\n" {
		t.Fatalf("ambient rebase marker changed: data=%q error=%v", data, err)
	}
}

func TestBatchProofInputsMovedIgnoresSiblingEnginePaths(t *testing.T) {
	record := batch.Record{Proof: &batch.Proof{SelectedGroups: []string{"docs"}, InputManifests: map[string][]string{"docs": {"metasystem/docs/**"}}}}
	for _, outside := range []string{"internal/other/x.go", "cmd/metasystem/main.go", "go.mod"} {
		if batchProofInputsMoved(record, []string{outside}, "metasystem") {
			t.Fatalf("sibling path %q was mapped into the installation engine", outside)
		}
	}
}
