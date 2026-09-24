package batch

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// The ordered series is valid: A deletes an existing path, B adds it back,
// and C changes an unrelated path. B alone cannot apply to the original base.
func deleteReaddFixture(t *testing.T) (assemblyBed, Store, string, string) {
	t.Helper()
	bed := assemblyFixture(t)
	baseCommit := bedGit(t, bed.root, "rev-parse", "HEAD^")
	bedGit(t, bed.root, "switch", "-q", "--detach", baseCommit)
	path := filepath.Join(bed.root, "a.go")
	must(t, os.Remove(path))
	must(t, os.WriteFile(filepath.Join(bed.root, "b.go"), []byte("package p\nvar B = 2\n"), 0o644))
	patchA := bedGit(t, bed.root, "diff", "--binary", "HEAD", "--", "a.go", "b.go")
	must(t, os.WriteFile(filepath.Join(bed.root, "artifacts/agents/landing-batches/chains/chain-a/diff.patch"), []byte(patchA+"\n"), 0o644))
	bedGit(t, bed.root, "add", "-u", "a.go", "b.go")
	bedGit(t, bed.root, "commit", "-qm", "delete a.go")
	must(t, os.WriteFile(path, []byte("package p\nvar A = 2\n"), 0o644))
	bedGit(t, bed.root, "add", "-N", "a.go")
	patchB := bedGit(t, bed.root, "diff", "--binary", "HEAD", "--", "a.go")
	must(t, os.WriteFile(filepath.Join(bed.root, "artifacts/agents/landing-batches/chains/chain-b/diff.patch"), []byte(patchB+"\n"), 0o644))
	bedGit(t, bed.root, "add", "a.go")
	bedGit(t, bed.root, "commit", "-qm", "re-add a.go")
	addCommit := bedGit(t, bed.root, "rev-parse", "HEAD")
	bed.record.Units = append(bed.record.Units, Unit{GoalID: "goal-c", Chain: "chain-c", Claim: Claim{Machine: "seat", Lineage: "l", Epoch: 1, Revision: 9, AccountingRevision: 7}, State: UnitJoined})
	prefixes, err := assembleUnits(bed.root, bed.base, bed.record.Units)
	must(t, err)
	if _, err := assembleUnits(bed.root, bed.base, bed.record.Units[1:]); err == nil {
		t.Fatal("B unexpectedly applied without A")
	}
	cOnly, err := assembleUnits(bed.root, bed.base, bed.record.Units[2:])
	must(t, err)
	bed.record.State, bed.record.PrefixTrees, bed.record.TipTree = StateDiagnosing, prefixes, prefixes[len(prefixes)-1]
	bed.record.Proof = &Proof{Status: "failed", AttemptID: "native-red"}
	store := NewStore(bed.root, nil)
	must(t, store.Create(bed.record))
	return bed, store, cOnly[0], addCommit
}

func TestGLEBatchPathExistsGitDiagnosticIsDefiniteComposition(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	calls := 0
	readUnmerged := func(gotRoot string) ([]byte, error) {
		if gotRoot != root {
			t.Fatalf("unmerged reader root = %q, want %q", gotRoot, root)
		}
		calls++
		return []byte{}, nil
	}
	command := exec.Command("sh", "-c", "exit 1")
	err := command.Run()
	for _, diagnostic := range []string{"error: a.go: already exists in working directory", "error: a.go: already exists in index"} {
		if !patchCompositionConflictWithUnmerged(root, []byte(diagnostic), err, readUnmerged) {
			t.Fatalf("ordinary add/add diagnostic was not typed: %q", diagnostic)
		}
	}
	for _, diagnostic := range []string{"error: metasystem/testing.json: already exists in index", "error: a.go: patch does not apply", "fatal: a.go: already exists in index", "error: a.go: already exists in index\nfatal: could not read index"} {
		if patchCompositionConflictWithUnmerged(root, []byte(diagnostic), err, readUnmerged) {
			t.Fatalf("non-composition diagnostic was typed: %q", diagnostic)
		}
	}
	if calls != 6 {
		t.Fatalf("unmerged reader called %d times, want 6", calls)
	}
}

func TestGLEBatchDeleteReaddDependentReturnsWithIndependentSurvivor(t *testing.T) {
	t.Parallel()
	bed, store, cOnly, patches := dependentPolicyStore(t, "delete-readd")
	strictReassembly(t, &store,
		expectedReassembly{kind: "assemble", base: bed.base, goals: []string{"goal-b", "goal-c"}, chains: []string{"chain-b", "chain-c"},
			err:    &assemblyConflict{GoalID: "goal-b", Cause: refuseBatch("BATCH_JOIN_CONFLICT", "unit goal-b does not apply: a.go already exists in index")},
			onCall: patches("chain-b", "chain-c")},
		expectedReassembly{kind: "assemble", base: bed.base, goals: []string{"goal-c"}, chains: []string{"chain-c"}, prefixes: []string{cOnly}, onCall: patches("chain-c")})
	if err := ReassembleSurvivorsWithReturns(store, testBatchID, "owner", time.Unix(3, 0),
		[]ReturnDecision{{GoalID: "goal-a", Outcome: UnitEjected, Reason: "A fenced"}}); err != nil {
		t.Fatal(err)
	}
	record := load(t, store)
	if record.State != StateOpen || record.Proof != nil || record.TipTree != cOnly || !slices.Equal(record.PrefixTrees, []string{cOnly}) ||
		record.Units[0].State != UnitReturnPending || record.Units[1].State != UnitReturnPending ||
		record.Units[0].Outcome != UnitEjected || record.Units[1].Outcome != UnitEjected ||
		record.Units[2].State != UnitJoined || !strings.Contains(record.Units[1].Failure, "cannot apply after returning goal-a") {
		t.Fatalf("delete/re-add dependency did not close to C: %+v", record)
	}
	var returns []string
	for _, entry := range record.History {
		if entry.Verb == "return-request" {
			returns = append(returns, strings.Fields(entry.Detail)[0])
		}
	}
	if !slices.Equal(returns, []string{"goal-a", "goal-b"}) {
		t.Fatalf("delete/re-add return order=%v", returns)
	}
}

func TestGLEBatchBranchAddOverExistingPathIsTypedCompositionConflict(t *testing.T) {
	t.Parallel()
	bed, _, _, addCommit := deleteReaddFixture(t)
	_, err := AssembleBranchMembers(bed.root, bed.base, []BranchMember{{GoalID: "goal-b", Builds: []BranchBuild{{Commit: addCommit, Digest: "unused-before-apply"}}}})
	var conflict *assemblyConflict
	if !errors.As(err, &conflict) || conflict.GoalID != "goal-b" {
		t.Fatalf("branch add over existing a.go was not a typed B conflict: %v", err)
	}
}

func movedDeleteReaddFixture(t *testing.T) (assemblyBed, Store, string) {
	t.Helper()
	bed, store, _, _ := deleteReaddFixture(t)
	baseCommit := bedGit(t, bed.root, "rev-parse", "HEAD~2")
	bedGit(t, bed.root, "switch", "-q", "--detach", baseCommit)
	must(t, os.WriteFile(filepath.Join(bed.root, "b.go"), []byte("package p\nvar B = 9\n"), 0o644))
	bedGit(t, bed.root, "add", "b.go")
	bedGit(t, bed.root, "commit", "-qm", "move base across A")
	newBase := bedGit(t, bed.root, "rev-parse", "HEAD^{tree}")
	must(t, store.Update(testBatchID, func(record *Record) error {
		record.State = StateLanding
		record.Proof.Status = "green"
		return nil
	}))
	return bed, store, newBase
}

func TestGLEBatchMovedBaseClosesTwoDependentConflictsBeforeReopen(t *testing.T) {
	t.Parallel()
	bed, store, _, patches := dependentPolicyStore(t, "delete-readd")
	newBase, cOnly := bed.moved, testCommit(205)
	must(t, store.Update(testBatchID, func(record *Record) error {
		record.State, record.Proof.Status = StateLanding, "green"
		return nil
	}))
	strictReassembly(t, &store,
		expectedReassembly{kind: "assemble", base: newBase, goals: []string{"goal-a", "goal-b", "goal-c"}, chains: []string{"chain-a", "chain-b", "chain-c"},
			err:    &assemblyConflict{GoalID: "goal-a", Cause: refuseBatch("BATCH_JOIN_CONFLICT", "unit goal-a does not apply on moved base: b.go changed")},
			onCall: patches("chain-a", "chain-b", "chain-c")},
		expectedReassembly{kind: "assemble", base: newBase, goals: []string{"goal-b", "goal-c"}, chains: []string{"chain-b", "chain-c"},
			err:    &assemblyConflict{GoalID: "goal-b", Cause: refuseBatch("BATCH_JOIN_CONFLICT", "unit goal-b does not apply: a.go already exists in index")},
			onCall: patches("chain-b", "chain-c")},
		expectedReassembly{kind: "assemble", base: newBase, goals: []string{"goal-c"}, chains: []string{"chain-c"}, prefixes: []string{cOnly}, onCall: patches("chain-c")})
	if err := ReopenMovedTrunk(store, testBatchID, newBase, "owner", time.Unix(4, 0)); err != nil {
		t.Fatal(err)
	}
	record := load(t, store)
	if record.State != StateOpen || record.BaseTree != newBase || record.TipTree != cOnly || !slices.Equal(record.PrefixTrees, []string{cOnly}) || record.Proof != nil ||
		record.Units[0].State != UnitReturnPending || record.Units[1].State != UnitReturnPending || record.Units[2].State != UnitJoined ||
		!strings.Contains(record.Units[0].Failure, "cannot apply on moved base") ||
		!strings.Contains(record.Units[1].Failure, "cannot apply after returning goal-a") {
		t.Fatalf("moved-base dependent closure did not retain C: %+v", record)
	}
	var returns []string
	for _, entry := range record.History {
		if entry.Verb == "return-request" {
			returns = append(returns, strings.Fields(entry.Detail)[0])
		}
	}
	if !slices.Equal(returns, []string{"goal-a", "goal-b"}) {
		t.Fatalf("moved-base return order=%v", returns)
	}
}

func TestGLEBatchNativeDependentChainPatchAdapter(t *testing.T) {
	t.Parallel()
	bridge, _, _ := dependentSplitStore(t)
	_, err := assembleUnits(bridge.root, bridge.base, bridge.record.Units[1:2])
	var conflict *assemblyConflict
	if !errors.As(err, &conflict) || conflict.GoalID != "goal-b" {
		t.Fatalf("bridge B-only patch was not a typed B conflict: %v", err)
	}

	deleted := assemblyFixture(t)
	for chain, patch := range deleteReaddPolicyPatches() {
		path := filepath.Join(deleted.root, "artifacts/agents/landing-batches/chains", chain, "diff.patch")
		must(t, os.WriteFile(path, patch, 0o644))
	}
	if _, err := assembleUnits(deleted.root, deleted.base, deleted.record.Units); err != nil {
		t.Fatalf("declared delete/re-add patches did not assemble on their base: %v", err)
	}
	_, err = assembleUnits(deleted.root, deleted.base, deleted.record.Units[1:2])
	conflict = nil
	if !errors.As(err, &conflict) || conflict.GoalID != "goal-b" {
		t.Fatalf("delete/re-add B-only patch was not a typed B conflict: %v", err)
	}
	baseCommit := bedGit(t, deleted.root, "rev-parse", "HEAD^")
	bedGit(t, deleted.root, "switch", "-q", "--detach", baseCommit)
	must(t, os.WriteFile(filepath.Join(deleted.root, "b.go"), []byte("package p\nvar B = 9\n"), 0o644))
	bedGit(t, deleted.root, "add", "b.go")
	bedGit(t, deleted.root, "commit", "-qm", "move base across A")
	newBase := bedGit(t, deleted.root, "rev-parse", "HEAD^{tree}")
	for _, check := range []struct {
		units []Unit
		goal  string
	}{{deleted.record.Units, "goal-a"}, {deleted.record.Units[1:], "goal-b"}} {
		_, err := assembleUnits(deleted.root, newBase, check.units)
		conflict = nil
		if !errors.As(err, &conflict) || conflict.GoalID != check.goal {
			t.Fatalf("moved-base %s conflict was not typed in join order: %v", check.goal, err)
		}
		if check.goal == "goal-a" && !strings.Contains(err.Error(), "b.go") {
			t.Fatalf("moved-base A conflict did not name changed b.go: %v", err)
		}
	}
	path := filepath.Join(bridge.root, "artifacts/agents/landing-batches/chains/chain-c/diff.patch")
	must(t, os.Remove(path))
	_, err = assembleUnits(bridge.root, bridge.base, bridge.record.Units[2:])
	var readError *os.PathError
	conflict = nil
	if !errors.As(err, &readError) || readError.Op != "open" || readError.Path != path || !errors.Is(err, os.ErrNotExist) || errors.As(err, &conflict) {
		t.Fatalf("missing C patch was not an untyped os.ReadFile failure: %v", err)
	}
}

func TestGLEBatchMovedBaseRecordCASProtectsPrivateBranch(t *testing.T) {
	t.Parallel()
	bed, store, newBase := movedDeleteReaddFixture(t)
	origin := filepath.Join(t.TempDir(), "origin.git")
	if output, err := exec.Command("git", "init", "-q", "--bare", origin).CombinedOutput(); err != nil {
		t.Fatalf("create origin: %v: %s", err, output)
	}
	bedGit(t, bed.root, "remote", "add", "origin", origin)
	bedGit(t, bed.root, "push", "-q", "origin", "HEAD:refs/heads/main")
	privateTip := bedGit(t, bed.root, "rev-parse", "HEAD")
	if err := PublishLandingBranch(bed.root, testBatchID, "", privateTip); err != nil {
		t.Fatal(err)
	}
	must(t, store.Update(testBatchID, func(record *Record) error {
		record.Landing = &LandingProgress{Base: record.BaseTree, BranchTip: privateTip, CandidateTip: privateTip}
		return nil
	}))
	flock := store.seams.flock
	mutated := false
	store.seams.flock = func(fd, op int) error {
		if op == unix.LOCK_EX && !mutated {
			mutated = true
			must(t, NewStore(bed.root, nil).Update(testBatchID, func(record *Record) error {
				record.Transition(StateLanding, time.Unix(4, 0), "concurrent", "other", "new owner fact")
				return nil
			}))
		}
		return flock(fd, op)
	}
	err := ReopenMovedTrunk(store, testBatchID, newBase, "owner", time.Unix(5, 0))
	if err == nil || !strings.Contains(err.Error(), "BATCH_REASSEMBLE_MOVED") || !mutated {
		t.Fatalf("concurrent record movement was not refused: error=%v mutated=%t", err, mutated)
	}
	record := load(t, store)
	if record.State != StateLanding || record.BaseTree != bed.base || len(record.History) == len(bed.record.History) || record.Landing == nil || record.Landing.CandidateTip != privateTip {
		t.Fatalf("moved-base reopen overwrote newer batch: %+v", record)
	}
	remoteTip, _, present, err := remoteLandingBranchTipAndTree(bed.root, testBatchID)
	if err != nil || !present || remoteTip != privateTip {
		t.Fatalf("moved-base CAS touched private branch: tip=%s present=%t err=%v want=%s", remoteTip, present, err, privateTip)
	}
}
