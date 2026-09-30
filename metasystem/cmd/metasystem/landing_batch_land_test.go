package main

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batchowner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testrun"
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
		filepath.Join(module, "metasystem.conf"):                         "metasystem.template=true\n",
		filepath.Join(repository, "development", "metasystem-design.md"): "template\n",
	} {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	seams := batchowner.BatchLandSeamsWithRead(repository, "batch", batch.Record{}, "base", "actor", batchowner.GitOutput, plantedBatchCommit)
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
	if err := os.MkdirAll(module, 0o755); err != nil {
		t.Fatal(err)
	}
	if marker, err := os.OpenFile(filepath.Join(module, "metasystem.conf"), os.O_CREATE|os.O_WRONLY, 0o644); err != nil {
		t.Fatal(err)
	} else {
		marker.Close()
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
	if err := os.MkdirAll(filepath.Dir(wrapper), 0o755); err != nil {
		t.Fatal(err)
	}
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
	seams := batchowner.BatchLandSeamsWithRead(repository, "batch", batch.Record{}, "base", "actor", readGit, plantedBatchCommit)
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
	if err := batchowner.FinishBatchLanding(root, store, batchID, "owner", time.Unix(5, 0)); err != nil {
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
	result, err := batchowner.ExecuteBatchPrefixReceiptWithDependencies(root, "batch", record, "goal-a", tree, batch.PrefixDecision{Groups: []string{"group-a"}}, dependencies)
	if err == nil || len(result.Red) != 0 {
		t.Fatalf("infrastructure result=%+v error=%v", result, err)
	}
}

func TestBatchProofInputsMovedIgnoresSiblingEnginePaths(t *testing.T) {
	record := batch.Record{Proof: &batch.Proof{SelectedGroups: []string{"docs"}, InputManifests: map[string][]string{"docs": {"metasystem/docs/**"}}}}
	for _, outside := range []string{"internal/other/x.go", "cmd/metasystem/main.go", "go.mod"} {
		if batch.DecideMovedBase(record, []string{outside}, "metasystem").Reopen {
			t.Fatalf("sibling path %q was mapped into the installation engine", outside)
		}
	}
}

func TestPrefixGroupExecutionListsCachedPassesApart(t *testing.T) {
	t.Parallel()
	cached := proofrun.GroupResult{Status: "passed", NativeLaunched: true,
		Execution: []proofrun.PackageExecution{{Package: "p", Mode: proofrun.PackageGoTestCache}}}
	mixed := proofrun.GroupResult{Status: "passed", NativeLaunched: true,
		Execution: []proofrun.PackageExecution{{Package: "p", Mode: proofrun.PackageGoTestCache}, {Package: "q", Mode: proofrun.PackageExecuted}}}
	legacy := proofrun.GroupResult{Status: "passed", NativeLaunched: true}
	for _, row := range []struct {
		group    proofrun.GroupResult
		reusable bool
		want     string
	}{{cached, false, "cached"}, {mixed, false, "executed"}, {legacy, false, "executed"}, {cached, true, ""}, {proofrun.GroupResult{}, false, ""}} {
		if got := batchowner.PrefixGroupExecution(row.group, row.reusable); got != row.want {
			t.Errorf("prefixGroupExecution(%+v, %t) = %q, want %q", row.group.Execution, row.reusable, got, row.want)
		}
	}
}

// Two batch proofs on bases T1 < T2 re-arm the one control root: never at
// once, and never back from T2 to T1 (the tip proof runs in its own detached
// worktree, so a root already past the batch base stays where it is).
func TestConcurrentRearmsNeverOverlapNorMoveTheControlRootBack(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
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
		if !batchowner.LaneCheckoutFor(root).Held() {
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
	edges := batchowner.BatchRearmEdges{
		Head:       func(string) (string, string, error) { mu.Lock(); defer mu.Unlock(); return head, "tree-" + head, nil },
		BaseCommit: func(_, tree string) (string, error) { return strings.TrimPrefix(tree, "tree-"), nil },
		Descends:   func(_, descendant, ancestor string) (bool, error) { return order[descendant] > order[ancestor], nil },
		FastForward: func(_ context.Context, _, commit string) error {
			step("fast-forward " + commit)
			mu.Lock()
			moves, head = append(moves, head+"->"+commit), commit
			mu.Unlock()
			return nil
		},
		Rebuild: func(context.Context, string) error { step("rebuild"); return nil },
		Up: func(context.Context, string, string) (testrun.UpOutcome, error) {
			step("up")
			return testrun.UpOutcome{}, nil
		},
	}
	errs := make(chan error, 2)
	go func() { errs <- batchowner.RearmBatchBaseWith(root, "tree-T2", edges) }()
	<-entered
	go func() { errs <- batchowner.RearmBatchBaseWith(root, "tree-T1", edges) }()
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

// TestLandingsAndRearmsTakeTurnsOnTheLaneCheckout: two green batches landing
// at once and a proof start's re-arm all move the one lane checkout, so while
// batch A's landing holds it, batch B's production landing and the re-arm
// wait; neither runs over A's unlanded tree (B1).
func TestLandingsAndRearmsTakeTurnsOnTheLaneCheckout(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	checkout := batchowner.LaneCheckoutFor(root)
	var inside atomic.Int32
	var landedDuringA, rearmedDuringA atomic.Bool
	entered, release := make(chan struct{}), make(chan struct{})
	first := make(chan error, 1)
	go func() {
		first <- batchowner.WithLaneCheckout(root, func() error {
			inside.Add(1)
			close(entered)
			<-release
			inside.Add(-1)
			return nil
		})
	}()
	<-entered
	landed, rearmed := make(chan error, 1), make(chan error, 1)
	go func() {
		err := batchowner.ExecuteBatchLanding(root, "batch-b", "owner", time.Unix(0, 0))
		landedDuringA.Store(inside.Load() != 0)
		landed <- err
	}()
	edges := batchowner.BatchRearmEdges{
		Head: func(string) (string, string, error) {
			rearmedDuringA.Store(inside.Load() != 0)
			return "T1", "tree-T1", nil
		},
		Rebuild: func(context.Context, string) error { return nil },
		Up:      func(context.Context, string, string) (testrun.UpOutcome, error) { return testrun.UpOutcome{}, nil },
	}
	go func() { rearmed <- batchowner.RearmBatchBaseWith(root, "tree-T1", edges) }()
	for checkout.Waiting.Load() != 2 && !landedDuringA.Load() && !rearmedDuringA.Load() {
		runtime.Gosched()
	}
	close(release)
	if err := <-first; err != nil {
		t.Fatal(err)
	}
	<-landed // batch-b has no record in this bed; only the order is witnessed
	if err := <-rearmed; err != nil {
		t.Fatal(err)
	}
	if landedDuringA.Load() || rearmedDuringA.Load() {
		t.Fatalf("the lane checkout was moved while batch A's landing held it: landing=%v rearm=%v", landedDuringA.Load(), rearmedDuringA.Load())
	}
	if checkout.Held() {
		t.Fatal("the lane checkout stayed held after every step finished")
	}
}
