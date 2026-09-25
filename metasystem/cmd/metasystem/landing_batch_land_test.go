package main

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
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
{
  pwd
  printf 'arg=%s\n' "$@"
  printf 'author-name=%s\n' "$GIT_AUTHOR_NAME"
  printf 'author-email=%s\n' "$GIT_AUTHOR_EMAIL"
  printf 'committer-name=%s\n' "$GIT_COMMITTER_NAME"
  printf 'committer-email=%s\n' "$GIT_COMMITTER_EMAIL"
  printf 'landed-by=%s\n' "$METASYSTEM_LANDED_BY"
} > wrapper-marker.txt
`
	if err := testexec.WriteFile(wrapper, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(module, "wrapper-marker.txt")
	reads := []struct {
		root, result string
		args         []string
	}{
		{module, "before", []string{"rev-parse", "HEAD"}},
		{module, "after", []string{"rev-parse", "HEAD"}},
		{module, "before", []string{"rev-parse", "after^"}},
		{repository, "after", []string{"rev-parse", "HEAD"}},
	}
	readCount := 0
	readGit := func(root string, args ...string) (string, error) {
		if readCount >= len(reads) || root != reads[readCount].root || !reflect.DeepEqual(args, reads[readCount].args) {
			t.Fatalf("unexpected Git read %d: root=%q args=%v", readCount, root, args)
		}
		_, err := os.Stat(marker)
		if readCount == 0 && !os.IsNotExist(err) || readCount > 0 && err != nil {
			t.Fatalf("Git read %d occurred on wrong side of wrapper marker: %v", readCount, err)
		}
		if readCount == 1 {
			data, err := os.ReadFile(marker)
			if err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(strings.TrimSpace(string(data)), "\n")
			if len(lines) != 14 || !strings.HasPrefix(lines[8], "arg=") {
				t.Fatalf("wrapper did not record message argument: %q", data)
			}
			message, err := os.ReadFile(strings.TrimPrefix(lines[8], "arg="))
			if err != nil || string(message) != "land goal-a in batch batch\n\nOriginal join order; prefix tree tree.\n" {
				t.Fatalf("wrapper message=%q error=%v", message, err)
			}
		}
		result := reads[readCount].result
		readCount++
		return result, nil
	}
	seams := batchLandSeamsWithRead(repository, "batch", batch.Record{}, "base", "actor", readGit)
	head, err := seams.Commit(batch.Unit{GoalID: "goal-a", Chain: "chain-a", AuthorName: "Owner", AuthorEmail: "owner@example.invalid"}, batch.PrefixReceipt{Tree: "tree"})
	if err != nil {
		t.Fatal(err)
	}
	if head != "after" || readCount != len(reads) {
		t.Fatalf("commit HEAD=%q Git reads=%d, want after and %d", head, readCount, len(reads))
	}
	data, err := os.ReadFile(marker)
	want, canonicalErr := canonicalPath(module)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if err != nil || canonicalErr != nil || len(lines) != 14 || lines[0] != want {
		t.Fatalf("wrapper marker=%q error=%v canonical-error=%v, want cwd %s", data, err, canonicalErr, want)
	}
	messageFile := strings.TrimPrefix(lines[8], "arg=")
	wantLines := []string{want, "arg=--chain", "arg=chain-a", "arg=--goal", "arg=goal-a", "arg=--test-receipt", "arg=" + filepath.Join(module, "artifacts", "agents", "proof-runs", "batch", "batch.json"), "arg=-F", "arg=" + messageFile, "author-name=Owner", "author-email=owner@example.invalid", "committer-name=Owner", "committer-email=owner@example.invalid", "landed-by=actor"}
	if !reflect.DeepEqual(lines, wantLines) || filepath.Dir(messageFile) != module || !strings.HasPrefix(filepath.Base(messageFile), ".batch-commit-message-") {
		t.Fatalf("wrapper invocation=%q, want %q and a temporary message under %s", lines, wantLines, module)
	}
	if _, err := os.Stat(messageFile); !os.IsNotExist(err) {
		t.Fatalf("temporary commit message remains: %v", err)
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
