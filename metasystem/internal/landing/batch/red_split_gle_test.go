package batch

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

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
	bed, store, cOnly := dependentSplitStore(t)

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
		!strings.Contains(reassembled.Units[0].Failure, "bridge-check") || !strings.Contains(reassembled.Units[1].Failure, "cannot apply") {
		t.Fatalf("dependent closure did not retain only C: %+v", reassembled)
	}
	if err := diagnose(); err == nil || runs != 1 {
		t.Fatalf("reassembled batch repeated old native diagnosis: error=%v runs=%d", err, runs)
	}
}

func TestGLEBatchSplitHoldsUnreadableSurvivorWithoutReturningMember(t *testing.T) {
	t.Parallel()
	bed, store, _ := dependentSplitStore(t)
	must(t, os.Remove(filepath.Join(bed.root, "artifacts/agents/landing-batches/chains/chain-c/diff.patch")))
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
		!strings.Contains(held.Proof.Failure, "survivor composition unclassified") {
		t.Fatalf("unreadable survivor caused a guessed return: %+v", held)
	}
}

func TestGLEBatchSplitFencedOrBudgetReturnClosesDependentPatches(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"fenced", "budget"} {
		t.Run(kind, func(t *testing.T) {
			_, store, cOnly := dependentSplitStore(t)
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
			if record.State != StateOpen || record.TipTree != cOnly || record.Units[0].State != UnitReturnPending ||
				record.Units[0].Outcome != outcome || record.Units[1].State != UnitReturnPending ||
				record.Units[1].Outcome != UnitEjected || record.Units[2].State != UnitJoined {
				t.Fatalf("%s return left dependent B or lost C: %+v", kind, record)
			}
		})
	}
}
