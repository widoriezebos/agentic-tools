package batch

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// The indexed b.go change in A conflicts when the base changes b.go to a
// different value. B adds a.go only after A removes the original path.
func deleteReaddPolicyPatches() map[string][]byte {
	return map[string][]byte{
		"chain-a": []byte("diff --git a/a.go b/a.go\ndeleted file mode 100644\nindex b02d6da8c10240ec8ca3507ccd9fd2d021fb57d6..0000000000000000000000000000000000000000\n--- a/a.go\n+++ /dev/null\n@@ -1,2 +0,0 @@\n-package p\n-var A = 0\ndiff --git a/b.go b/b.go\nindex a664d74a82a1b31916bf41ec8cc7625cef777817..2e4ba9f0892c14e0e2eb20bc3282e11876d9b3c1 100644\n--- a/b.go\n+++ b/b.go\n@@ -1,2 +1,2 @@\n package p\n-var B = 0\n+var B = 2\n"),
		"chain-b": []byte("diff --git a/a.go b/a.go\nnew file mode 100644\nindex 0000000000000000000000000000000000000000..941c9d9e2568af8f54a3d260d309c67c09493ce3\n--- /dev/null\n+++ b/a.go\n@@ -0,0 +1,2 @@\n+package p\n+var A = 2\n"),
		"chain-c": []byte("diff --git a/c.go b/c.go\nindex f2f0e20bf5704db09a490ad89688704970bd1859..22eda802d8236e80e3ce1901c8126cc4d4422061 100644\n--- a/c.go\n+++ b/c.go\n@@ -1,2 +1,2 @@\n package p\n-var C = 0\n+var C = 1\n"),
	}
}

// dependentPolicyStore keeps patch bytes on disk while Store owns the return
// decisions. Each declared repository reply checks the patch files it uses.
func dependentPolicyStore(t *testing.T, form string) (assemblyBed, Store, string, func(...string) func()) {
	t.Helper()
	fixture := policyFixture(t)
	bed := assemblyBed(fixture)
	bed.record.Units = append(bed.record.Units, Unit{GoalID: "goal-c", Chain: "chain-c",
		Claim: Claim{Machine: "seat", Lineage: "l", Epoch: 1, Revision: 9, AccountingRevision: 7}, State: UnitJoined})
	patches := map[string][]byte{
		"chain-c": []byte("diff --git a/c.go b/c.go\n--- a/c.go\n+++ b/c.go\n@@ -1 +1 @@\n-old C\n+new C\n"),
	}
	switch form {
	case "bridge":
		patches["chain-a"] = []byte("diff --git a/bridge.txt b/bridge.txt\nnew file mode 100644\n--- /dev/null\n+++ b/bridge.txt\n@@ -0,0 +1 @@\n+created by A\n")
		patches["chain-b"] = []byte("diff --git a/bridge.txt b/bridge.txt\n--- a/bridge.txt\n+++ b/bridge.txt\n@@ -1 +1 @@\n-created by A\n+changed by B\n")
		bed.record.Units[0].SelectedGroups = []string{"bridge-check"}
	case "delete-readd":
		patches = deleteReaddPolicyPatches()
	default:
		t.Fatalf("unknown dependent patch form %q", form)
	}
	for chain, patch := range patches {
		path := filepath.Join(bed.root, "artifacts/agents/landing-batches/chains", chain, "diff.patch")
		must(t, os.MkdirAll(filepath.Dir(path), 0o755))
		must(t, os.WriteFile(path, patch, 0o644))
	}
	bed.record.PrefixTrees = []string{testCommit(201), testCommit(202), testCommit(203)}
	bed.record.TipTree, bed.record.State = bed.record.PrefixTrees[2], StateDiagnosing
	bed.record.Proof = &Proof{Status: "failed", AttemptID: "tip-red"}
	store := NewStore(bed.root, nil)
	must(t, store.Create(bed.record))
	checkPatches := func(chains ...string) func() {
		return func() {
			for _, chain := range chains {
				path := filepath.Join(bed.root, "artifacts/agents/landing-batches/chains", chain, "diff.patch")
				got, err := os.ReadFile(path)
				must(t, err)
				if !bytes.Equal(got, patches[chain]) {
					t.Fatalf("%s %s patch bytes changed before repository effect", form, chain)
				}
			}
		}
	}
	return bed, store, testCommit(204), checkPatches
}

func dependentSplitStore(t *testing.T) (assemblyBed, Store, string) {
	t.Helper()
	bed := assemblyFixture(t)
	baseCommit := bedGit(t, bed.root, "rev-parse", "HEAD^")
	bedGit(t, bed.root, "switch", "-q", "--detach", baseCommit)
	bridge := filepath.Join(bed.root, "bridge.txt")
	must(t, os.WriteFile(bridge, []byte("created by A\n"), 0o644))
	bedGit(t, bed.root, "add", "-N", "bridge.txt")
	patchA := bedGit(t, bed.root, "diff", "--binary", "HEAD", "--", "bridge.txt")
	must(t, os.WriteFile(filepath.Join(bed.root, "artifacts/agents/landing-batches/chains/chain-a/diff.patch"), []byte(patchA+"\n"), 0o644))
	bedGit(t, bed.root, "add", "bridge.txt")
	bedGit(t, bed.root, "commit", "-qm", "create bridge")
	must(t, os.WriteFile(bridge, []byte("changed by B\n"), 0o644))
	patchB := bedGit(t, bed.root, "diff", "--binary", "HEAD", "--", "bridge.txt")
	must(t, os.WriteFile(filepath.Join(bed.root, "artifacts/agents/landing-batches/chains/chain-b/diff.patch"), []byte(patchB+"\n"), 0o644))

	bed.record.Units[0].SelectedGroups = []string{"bridge-check"}
	bed.record.Units = append(bed.record.Units, Unit{GoalID: "goal-c", Chain: "chain-c", Claim: Claim{Machine: "seat", Lineage: "l", Epoch: 1, Revision: 9, AccountingRevision: 7}, State: UnitJoined})
	tip, err := assembleUnits(bed.root, bed.base, bed.record.Units)
	must(t, err)
	if _, err := assembleUnits(bed.root, bed.base, bed.record.Units[1:]); err == nil {
		t.Fatal("dependent B patch unexpectedly applied without A")
	}
	cOnly, err := assembleUnits(bed.root, bed.base, bed.record.Units[2:])
	must(t, err)
	bed.record.TipTree = tip[len(tip)-1]
	bed.record.PrefixTrees = tip
	bed.record.State = StateDiagnosing
	bed.record.Proof = &Proof{Status: "failed", AttemptID: "tip-red"}
	store := NewStore(bed.root, nil)
	must(t, store.Create(bed.record))
	return bed, store, cOnly[0]
}

func TestGLEBatchSplitBranchPatchConflictNamesOnlyInapplicableMember(t *testing.T) {
	t.Parallel()
	bed := newGoalBranchBed(t)
	branchGit(t, bed.root, "switch", "-q", "--detach", bed.base)
	branchWrite(t, bed.root, "metasystem/bridge.txt", "created by A\n")
	branchGit(t, bed.root, "add", "metasystem/bridge.txt")
	branchGit(t, bed.root, "commit", "-qm", "create bridge")
	branchWrite(t, bed.root, "metasystem/bridge.txt", "changed by B\n")
	branchGit(t, bed.root, "add", "metasystem/bridge.txt")
	branchGit(t, bed.root, "commit", "-qm", "modify bridge")
	build := BranchBuild{Commit: branchGit(t, bed.root, "rev-parse", "HEAD"), Digest: "unused-before-apply"}
	baseTree := branchGit(t, bed.root, "rev-parse", bed.base+"^{tree}")
	_, err := AssembleBranchMembers(bed.root, baseTree, []BranchMember{{GoalID: "goal-b", Builds: []BranchBuild{build}}})
	var conflict *assemblyConflict
	if !errors.As(err, &conflict) || conflict.GoalID != "goal-b" {
		t.Fatalf("dependent branch patch did not name B: %v", err)
	}
}

func TestGLEBatchSplitReturnsInapplicableDependentAndKeepsIndependentMember(t *testing.T) {
	t.Parallel()
	bed, store, cOnly, patches := dependentPolicyStore(t, "bridge")
	conflict := &assemblyConflict{GoalID: "goal-b", Cause: refuseBatch("BATCH_JOIN_CONFLICT", "unit goal-b does not apply: bridge.txt does not exist in index")}
	strictReassembly(t, &store,
		expectedReassembly{kind: "assemble", base: bed.base, goals: []string{"goal-b", "goal-c"}, chains: []string{"chain-b", "chain-c"}, err: conflict, onCall: patches("chain-b", "chain-c")},
		expectedReassembly{kind: "assemble", base: bed.base, goals: []string{"goal-c"}, chains: []string{"chain-c"}, prefixes: []string{cOnly}, onCall: patches("chain-c")})

	runs := 0
	diagnose := func() error {
		return DiagnoseRed(store, testBatchID, "owner", []RedGroup{{ID: "bridge-check", Status: "failed"}}, "", time.Unix(2, 0), RedSeams{
			Run: func(request DiagnosticRequest) (DiagnosticResult, error) {
				runs++
				if request.Tree != bed.base || !request.NeverReuse {
					t.Fatalf("base diagnosis request=%+v", request)
				}
				return DiagnosticResult{AttemptID: "base-green"}, nil
			},
		})
	}
	if err := diagnose(); err != nil {
		t.Fatalf("dependent split failed before survivor reassembly: %v; record=%+v", err, load(t, store))
	}
	reassembled := load(t, store)
	if reassembled.State != StateOpen || reassembled.Proof != nil || reassembled.Units[0].State != UnitReturnPending ||
		reassembled.Units[1].State != UnitReturnPending || reassembled.Units[2].State != UnitJoined || reassembled.TipTree != cOnly ||
		!slices.Equal(reassembled.PrefixTrees, []string{cOnly}) ||
		!strings.Contains(reassembled.Units[0].Failure, "bridge-check") ||
		!strings.Contains(reassembled.Units[1].Failure, "cannot apply after returning goal-a") {
		t.Fatalf("dependent closure did not retain only C: %+v", reassembled)
	}
	var returns []string
	for _, entry := range reassembled.History {
		if entry.Verb == "return-request" {
			returns = append(returns, strings.Fields(entry.Detail)[0])
		}
	}
	if !slices.Equal(returns, []string{"goal-a", "goal-b"}) {
		t.Fatalf("dependent return order=%v", returns)
	}
	if err := diagnose(); err == nil || runs != 1 {
		t.Fatalf("reassembled batch repeated old native diagnosis: error=%v runs=%d", err, runs)
	}
}

func TestGLEBatchSplitHoldsUnreadableSurvivorWithoutReturningMember(t *testing.T) {
	t.Parallel()
	bed, store, _, patches := dependentPolicyStore(t, "bridge")
	path := filepath.Join(bed.root, "artifacts/agents/landing-batches/chains/chain-c/diff.patch")
	must(t, os.Remove(path))
	_, unreadable := os.ReadFile(path)
	var readError *os.PathError
	var typed *assemblyConflict
	if !errors.As(unreadable, &readError) || readError.Op != "open" || readError.Path != path || !os.IsNotExist(unreadable) || errors.As(unreadable, &typed) {
		t.Fatalf("missing C patch did not produce an os.ReadFile error: %v", unreadable)
	}
	strictReassembly(t, &store,
		expectedReassembly{kind: "assemble", base: bed.base, goals: []string{"goal-b", "goal-c"}, chains: []string{"chain-b", "chain-c"},
			err:    &assemblyConflict{GoalID: "goal-b", Cause: refuseBatch("BATCH_JOIN_CONFLICT", "unit goal-b does not apply: bridge.txt does not exist in index")},
			onCall: patches("chain-b")},
		expectedReassembly{kind: "assemble", base: bed.base, goals: []string{"goal-c"}, chains: []string{"chain-c"}, err: unreadable,
			onCall: func() {
				_, err := os.ReadFile(path)
				if !os.IsNotExist(err) {
					t.Fatalf("missing C patch changed before repository read: %v", err)
				}
			}},
	)
	err := DiagnoseRed(store, testBatchID, "owner", []RedGroup{{ID: "bridge-check", Status: "failed"}}, "", time.Unix(2, 0), RedSeams{
		Run: func(request DiagnosticRequest) (DiagnosticResult, error) {
			if request.Tree != bed.base || !request.NeverReuse {
				t.Fatalf("base diagnosis request=%+v", request)
			}
			return DiagnosticResult{AttemptID: "base-green"}, nil
		},
	})
	must(t, err)
	held := load(t, store)
	if held.State != StateHeldUnclassified || held.Proof == nil || held.Proof.Status != "held-unclassified" ||
		held.Units[0].State != UnitJoined || held.Units[1].State != UnitJoined || held.Units[2].State != UnitJoined ||
		!strings.Contains(held.Proof.Failure, "survivor composition unclassified") || !strings.Contains(held.Proof.Failure, path) {
		t.Fatalf("unreadable survivor caused a guessed return: %+v", held)
	}
}

func TestGLEBatchSplitFencedOrBudgetReturnClosesDependentPatches(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"fenced", "budget"} {
		t.Run(kind, func(t *testing.T) {
			bed, store, cOnly, patches := dependentPolicyStore(t, "bridge")
			strictReassembly(t, &store,
				expectedReassembly{kind: "assemble", base: bed.base, goals: []string{"goal-b", "goal-c"}, chains: []string{"chain-b", "chain-c"},
					err:    &assemblyConflict{GoalID: "goal-b", Cause: refuseBatch("BATCH_JOIN_CONFLICT", "unit goal-b does not apply: bridge.txt does not exist in index")},
					onCall: patches("chain-b", "chain-c")},
				expectedReassembly{kind: "assemble", base: bed.base, goals: []string{"goal-c"}, chains: []string{"chain-c"}, prefixes: []string{cOnly}, onCall: patches("chain-c")})
			var err error
			if kind == "budget" {
				must(t, store.Update(testBatchID, func(record *Record) error {
					record.State, record.Proof.Status = StateProving, "planned"
					return nil
				}))
				err = WithdrawBudgetMember(store, testBatchID, "goal-a", "owner", "budget exhausted", time.Unix(3, 0))
			} else {
				err = ReassembleSurvivorsWithReturns(store, testBatchID, "owner", time.Unix(3, 0),
					[]ReturnDecision{{GoalID: "goal-a", Outcome: UnitEjected, Reason: "live exact fence"}})
			}
			must(t, err)
			record := load(t, store)
			outcome := UnitEjected
			if kind == "budget" {
				outcome = UnitWithdrawnBudget
			}
			if record.State != StateOpen || record.Proof != nil || record.TipTree != cOnly || !slices.Equal(record.PrefixTrees, []string{cOnly}) || record.Units[0].State != UnitReturnPending ||
				record.Units[0].Outcome != outcome || record.Units[1].State != UnitReturnPending ||
				record.Units[1].Outcome != UnitEjected || !strings.Contains(record.Units[1].Failure, "cannot apply after returning goal-a") || record.Units[2].State != UnitJoined {
				t.Fatalf("%s return left dependent B or lost C: %+v", kind, record)
			}
			var returns []string
			for _, entry := range record.History {
				if entry.Verb == "return-request" {
					returns = append(returns, strings.Fields(entry.Detail)[0])
				}
			}
			if !slices.Equal(returns, []string{"goal-a", "goal-b"}) {
				t.Fatalf("%s return order=%v", kind, returns)
			}
		})
	}
}
