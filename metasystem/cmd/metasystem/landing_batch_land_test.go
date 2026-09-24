package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

func TestBatchLandReceiptsRunFromNestedModuleRoot(t *testing.T) {
	t.Parallel()
	repository := t.TempDir()
	module := filepath.Join(repository, "metasystem")
	if err := os.MkdirAll(filepath.Join(module, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(module, "go.mod"), []byte("module fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(module, "scripts", "receipt.sh")
	if err := testexec.WriteFile(script, []byte("#!/bin/sh\npwd > receipt-root.txt\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	seams := batchLandSeams(repository, "batch", batch.Record{}, "base", "actor")
	if err := seams.AppendReceipt(batch.Unit{GoalID: "goal-a"}, batch.PrefixReceipt{Tree: "tree"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(module, "receipt-root.txt"))
	want, canonicalErr := canonicalPath(module)
	if err != nil || canonicalErr != nil || strings.TrimSpace(string(data)) != want {
		t.Fatalf("receipt root=%q error=%v canonical-error=%v, want %s", data, err, canonicalErr, want)
	}
}

func TestBatchLandCommitWrapperRunsFromNestedModuleRoot(t *testing.T) {
	t.Parallel()
	repository := t.TempDir()
	module := filepath.Join(repository, "metasystem")
	if err := os.MkdirAll(filepath.Join(module, "scripts", "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(module, "go.mod"), []byte("module fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	wrapper := filepath.Join(module, "scripts", "agents", "commit.sh")
	script := `#!/bin/sh
while [ "$#" -gt 0 ]; do
  case "$1" in --chain|--goal|--test-receipt) shift 2;; *) break;; esac
done
pwd > wrapper-root.txt
git commit -q "$@"
`
	if err := testexec.WriteFile(wrapper, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("git", "init", "-q", "-b", "main", repository).CombinedOutput(); err != nil {
		t.Fatalf("init: %v: %s", err, output)
	}
	for _, args := range [][]string{{"config", "user.name", "Fixture"}, {"config", "user.email", "fixture@example.invalid"}} {
		if output, err := exec.Command("git", append([]string{"-C", repository}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
	}
	product := filepath.Join(module, "product.go")
	if err := os.WriteFile(product, []byte("package fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("git", "-C", repository, "add", ".").CombinedOutput(); err != nil {
		t.Fatalf("add base: %v: %s", err, output)
	}
	if output, err := exec.Command("git", "-C", repository, "commit", "-qm", "base").CombinedOutput(); err != nil {
		t.Fatalf("commit base: %v: %s", err, output)
	}
	if err := os.WriteFile(product, []byte("package fixture\n\n// changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("git", "-C", repository, "add", "metasystem/product.go").CombinedOutput(); err != nil {
		t.Fatalf("stage product: %v: %s", err, output)
	}
	seams := batchLandSeams(repository, "batch", batch.Record{}, "base", "actor")
	if _, err := seams.Commit(batch.Unit{GoalID: "goal-a", Chain: "chain-a", AuthorName: "Owner", AuthorEmail: "owner@example.invalid"}, batch.PrefixReceipt{}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(module, "wrapper-root.txt"))
	want, canonicalErr := canonicalPath(module)
	if err != nil || canonicalErr != nil || strings.TrimSpace(string(data)) != want {
		t.Fatalf("wrapper root=%q error=%v canonical-error=%v, want %s", data, err, canonicalErr, want)
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
	t.Parallel()
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
	dependencies := batchTestExecutionDependencies(t, root, tree, fake)
	record := batch.Record{Units: []batch.Unit{{GoalID: "goal-a", Claim: batch.Claim{Revision: 7, AccountingRevision: 5}}}}
	result, err := executeBatchPrefixReceiptWithDependencies(root, "batch", record, "goal-a", tree, batch.PrefixDecision{Groups: []string{"group-a"}}, dependencies)
	if err == nil || len(result.Red) != 0 {
		t.Fatalf("infrastructure result=%+v error=%v", result, err)
	}
}

func TestRecoveryFailureDoesNotAbortAmbientRebase(t *testing.T) {
	root := t.TempDir()
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
