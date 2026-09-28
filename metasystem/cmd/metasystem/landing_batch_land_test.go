package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
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
	for _, dir := range []string{filepath.Join(module, "scripts", "agents"), filepath.Join(repository, "development")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for path, content := range map[string]string{
		filepath.Join(module, "go.mod"):                                  "module fixture\n",
		filepath.Join(module, "metasystem.conf"):                         "",
		filepath.Join(repository, "development", "metasystem-design.md"): "template\n",
	} {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	seams := batchLandSeamsWithRead(repository, "batch", batch.Record{}, "base", "actor", gitOutput, plantedBatchCommit)
	if err := seams.AppendReceipt(batch.Unit{GoalID: "goal-a"}, batch.PrefixReceipt{Tree: "tree"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(module, "memory", "receipts.log"))
	if err != nil {
		t.Fatalf("the receipt must land in the module's own ledger: %v", err)
	}
	line := string(data)
	for _, want := range []string{"|RECEIPT|type=implement|outcome=shipped|", "|goal=goal-a|", "|built_by=coordinator|", "|note=batch batch prefix tree"} {
		if !strings.Contains(line, want) {
			t.Fatalf("receipt line %q lacks %q", line, want)
		}
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
	seams := batchLandSeamsWithRead(repository, "batch", batch.Record{}, "base", "actor", readGit, plantedBatchCommit)
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
	if !reflect.DeepEqual(lines, wantLines) || strings.HasPrefix(messageFile, module+string(filepath.Separator)) || !strings.HasPrefix(filepath.Base(messageFile), "metasystem-batch-commit-message-") {
		t.Fatalf("wrapper invocation=%q, want %q and a temporary message outside the work tree %s", lines, wantLines, module)
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

// R6 (a): batch A landed a records commit while B waited to land. B's landing
// run meets the moved base at its one push: nothing B's proof selected moved,
// so the series rebases once, the retained verifier runs once on the rebased
// tip, and B lands on the proof it has, with no new proof attempt.
func TestFirstGreenLandsOthersRebaseWhenNoInputMoved(t *testing.T) {
	t.Parallel()
	root, id := t.TempDir(), "01j5x00000000000000000ba22"
	claim := batch.Claim{Machine: "landing", Lineage: "owner", Epoch: 1, Revision: 1, AccountingRevision: 1}
	record := batch.Record{Schema: 1, BatchID: id, State: batch.StateLanding, BaseTree: "base-1", PrefixTrees: []string{"tip-1"}, TipTree: "tip-1",
		Units: []batch.Unit{{GoalID: "goal-b", Chain: "chain-b", Claim: claim, State: batch.UnitJoined}},
		Proof: &batch.Proof{Status: "green", AttemptID: "tip-proof", BaseCommit: "commit-1", SelectedGroups: []string{"unit-standard"},
			InputManifests: map[string][]string{"unit-standard": {"internal/**"}}},
		History: []batch.HistoryEntry{{At: time.Unix(1, 0).UTC().Format(time.RFC3339Nano), To: batch.StateLanding, Actor: "owner"}}}
	store := batch.NewStore(root, nil).WithReassembly(func(string, []batch.Unit) ([]string, error) { return []string{"tip-2"}, nil },
		func(string, string) error { return nil }, nil)
	if err := store.Create(record); err != nil {
		t.Fatal(err)
	}
	var calls []string
	var verified [][]string
	edges := movedBaseEdges{
		readGit: func(_ string, args ...string) (string, error) {
			switch strings.Join(args, " ") {
			case "rev-parse commit-2^{tree}":
				return "base-2", nil
			case "rev-parse HEAD":
				return "rebased-tip", nil
			case "log --first-parent --reverse --format=%T commit-2..rebased-tip":
				return "rebased-tree", nil
			}
			return "", errors.New("unstubbed git " + strings.Join(args, " "))
		},
		onEndpoint: func(string, string, string) (bool, error) { return false, nil },
		paths: func(_, from, to string) ([]string, string, error) {
			calls = append(calls, "paths "+from+".."+to)
			return []string{"records/2026-09-28.md"}, "", nil
		},
		advance:   func(string) error { calls = append(calls, "advance"); return nil },
		held:      func(_, base, commit, _, _ string) error { calls = append(calls, "held "+base+" "+commit); return nil },
		verify:    func(_ string, _ batch.Record, trees []string) error { verified = append(verified, trees); return nil },
		authorize: func(string, batch.Store, batch.Record, string, time.Time) error { return nil },
		now:       func(string) (time.Time, error) { return time.Unix(3, 0), nil },
		publish:   func(_, _, expected, tip string) error { calls = append(calls, "publish "+expected+" "+tip); return nil },
		push:      func(_, _, base, tip string) error { calls = append(calls, "push "+base+" "+tip); return nil },
		fetch: func(string) (string, string, error) {
			return "", "", errors.New("no fetch after a push that succeeded")
		},
	}
	stalePushes := 0
	seams := batch.LandSeams{Prepare: func(string) error { return nil }, Apply: func(batch.Unit) error { return nil },
		AppendReceipt: func(batch.Unit, batch.PrefixReceipt) error { return nil },
		Commit:        func(batch.Unit, batch.PrefixReceipt) (string, error) { return "old-tip", nil },
		Held:          func(string, string) error { return nil }, PublishBranch: func(string, string) error { return nil },
		Push: func(string, string) error {
			stalePushes++
			return &batch.EndpointPushError{StaleLease: true, Cause: errors.New("stale info")}
		},
		Origin: func() (string, error) { return "commit-2", nil }, OriginTree: func(string) (string, error) { return "base-2", nil },
		Abandon: func(string, string) error { return nil }, SeriesOnOrigin: func(string, string) (bool, error) { return false, nil },
		LeaseBase: "commit-1",
		RecoverPush: func(origin, baseTree, tip string) (batch.PushRecovery, error) {
			return recoverMovedBatchPushWith(root, id, record, "owner", "commit-1", origin, baseTree, tip, edges)
		},
	}
	if err := batch.LandSeries(store, id, "owner", time.Unix(2, 0), seams); err != nil {
		t.Fatal(err)
	}
	landed, err := store.Load(id)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"paths base-1..base-2", "advance", "held commit-2 rebased-tip", "publish old-tip rebased-tip", "push commit-2 rebased-tip"}
	if stalePushes != 1 || !slices.Equal(calls, want) || len(verified) != 1 || !slices.Equal(verified[0], []string{"rebased-tree"}) {
		t.Fatalf("rebase at the tick: stale pushes=%d calls=%q verified=%q", stalePushes, calls, verified)
	}
	if landed.State != batch.StateLanding || landed.Landing == nil || !landed.Landing.PushComplete || landed.Landing.PushedTip != "rebased-tip" ||
		landed.Proof.AttemptID != "tip-proof" || landed.Proof.Status != "green" {
		t.Fatalf("B did not land on its proof: %+v landing=%+v", landed, landed.Landing)
	}
}

// Two batch proofs on bases T1 < T2 re-arm the one control root: never at
// once, and never back from T2 to T1 (the tip proof runs in its own detached
// worktree, so a root already past the batch base stays where it is).
func TestConcurrentRearmsNeverOverlapNorMoveTheControlRootBack(t *testing.T) {
	t.Parallel()
	var (
		mu               sync.Mutex
		head             = "T0"
		active, overlaps int
		moves            []string
	)
	order := map[string]int{"T0": 0, "T1": 1, "T2": 2}
	entered, proceed := make(chan struct{}), make(chan struct{})
	step := func(name string) {
		mu.Lock()
		active++
		if active > 1 {
			overlaps++
		}
		mu.Unlock()
		if batchRearmMutex.TryLock() {
			batchRearmMutex.Unlock()
			mu.Lock()
			overlaps++
			mu.Unlock()
		}
		if name == "fast-forward T2" {
			close(entered)
			<-proceed
		}
		mu.Lock()
		active--
		mu.Unlock()
	}
	edges := batchRearmEdges{
		head:       func(string) (string, string, error) { mu.Lock(); defer mu.Unlock(); return head, "tree-" + head, nil },
		baseCommit: func(_, tree string) (string, error) { return strings.TrimPrefix(tree, "tree-"), nil },
		descends:   func(_, descendant, ancestor string) (bool, error) { return order[descendant] > order[ancestor], nil },
		fastForward: func(_ context.Context, _, commit string) error {
			step("fast-forward " + commit)
			mu.Lock()
			moves, head = append(moves, head+"->"+commit), commit
			mu.Unlock()
			return nil
		},
		rebuild: func(context.Context, string) error { step("rebuild"); return nil },
		up:      func(context.Context, string, string) (upOutcome, error) { step("up"); return upOutcome{}, nil },
	}
	errs := make(chan error, 2)
	go func() { errs <- rearmBatchBaseWith("root", "tree-T2", edges) }()
	<-entered
	go func() { errs <- rearmBatchBaseWith("root", "tree-T1", edges) }()
	close(proceed)
	for range 2 {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	if overlaps != 0 || !slices.Equal(moves, []string{"T0->T2"}) || head != "T2" {
		t.Fatalf("rearms overlapped %d time(s) or moved the control root back: moves=%q head=%s", overlaps, moves, head)
	}
}
