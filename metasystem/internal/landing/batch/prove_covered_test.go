package batch

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

// coveredProofGroups returns a native group and a group whose every expected
// test passed in that native group of the same proof.
func coveredProofGroups(sourceID, coveredID, sourceStatus string) (proofrun.GroupResult, proofrun.GroupResult) {
	exit := 0
	if sourceStatus == "failed" {
		exit = 1
	}
	context := strings.Repeat("c", 64)
	expected := proofrun.NativeTestIdentity{Classname: "fixture/" + sourceID, Name: "TestShared", Status: "expected"}
	observed := proofrun.NativeTestIdentity{Report: "go-test-json", Classname: expected.Classname, Name: expected.Name, Status: "passed"}
	source := proofrun.GroupResult{ID: sourceID, Status: sourceStatus, NativeLaunched: true, NativeExitStatus: &exit, CollectionComplete: sourceStatus == "passed",
		NativeContext: context, Expected: []proofrun.NativeTestIdentity{expected}, Observed: []proofrun.NativeTestIdentity{observed}}
	if sourceStatus == "reused" {
		source.NativeLaunched, source.NativeExitStatus, source.ReuseAttempt, source.CollectionComplete = false, nil, "other-attempt", true
	}
	covered := proofrun.GroupResult{ID: coveredID, Status: "passed", CollectionComplete: true, NativeContext: context,
		Expected: []proofrun.NativeTestIdentity{expected}, Observed: []proofrun.NativeTestIdentity{observed},
		CoveredByGroups: []string{sourceID}, CoveredTests: []proofrun.NativeTestIdentity{observed}}
	return source, covered
}

// A group whose tests passed natively in another group of the same green
// proof passes that proof and clears its own trunk-red entry. Coverage from a
// failed or borrowed source clears nothing, and only native launches count as
// executions.
func TestGreenTipProofClearsCoveredGroupOnlyFromNativePassingSource(t *testing.T) {
	t.Parallel()
	store := NewStore(t.TempDir(), nil)
	must(t, store.Create(Record{Schema: 1, BatchID: testBatchID, State: StateProving, Proof: &Proof{Status: "planned"}}))
	source, covered := coveredProofGroups("source", "covered", "passed")
	red, coveredByRed := coveredProofGroups("red", "covered-by-red", "failed")
	borrowed, coveredByBorrowed := coveredProofGroups("borrowed", "covered-by-borrowed", "reused")
	result := proofrun.TestResult{AttemptID: "tip-green", BaseCommit: "next-commit", CandidateTree: "next-tree",
		Delivery: proofrun.DeliveryJudgment{Sufficient: true},
		Groups:   []proofrun.GroupResult{source, covered, red, coveredByRed, borrowed, coveredByBorrowed}}
	must(t, FinishProof(store, testBatchID, "owner", result, nil, time.Unix(11, 0)))
	proof := load(t, store).Proof
	if !slices.Equal(proof.Passed, []string{"source", "covered"}) || !slices.Equal(proof.Executions, []string{"source", "red"}) {
		t.Fatalf("covered proof passed=%v executions=%v", proof.Passed, proof.Executions)
	}
	record := ownerRecord(testBatchID, StateLanding, time.Unix(1, 0))
	record.Proof = proof
	bed := newOwnerBed(t, record, time.Unix(12, 0))
	ledger := &clearingLedger{open: []OpenEntry{
		{ID: "covered", Group: "covered", LastBaseCommit: "old-commit"},
		{ID: "covered-by-red", Group: "covered-by-red", LastBaseCommit: "old-commit"},
		{ID: "covered-by-borrowed", Group: "covered-by-borrowed", LastBaseCommit: "old-commit"},
	}}
	calls := 0
	bed.store = bed.store.WithLedgerOwner(ledger)
	bed.owner.store = bed.store
	bed.owner.mint = func() (string, error) { calls++; return fmt.Sprintf("clear-%d", calls), nil }
	bed.owner.descendsFrom = func(string, string) (bool, error) { return true, nil }
	must(t, bed.owner.Tick(testBatchID))
	if len(ledger.cleared) != 1 || ledger.cleared[0].ref.ID != "covered" || ledger.cleared[0].green.AttemptID != "tip-green" || bed.launches != 0 {
		t.Fatalf("covered clears=%+v launches=%d", ledger.cleared, bed.launches)
	}
}
