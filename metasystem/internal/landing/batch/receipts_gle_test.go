package batch

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestGLEBatchReceiptDecisionChangesWithPolicyAuthorityAndTree(t *testing.T) {
	t.Parallel()
	units := []Unit{{GoalID: "goal-a", Chain: "chain-a", SelectedGroups: []string{"a"}}}
	sealed := map[string]Claim{"goal-a": {Revision: 2, AccountingRevision: 3}}
	decision := PrefixDecision{Groups: []string{"a"}, PolicyContext: "protected-base=base-a; contract=contract-a"}
	first, err := PrefixDecisionID("base-a", "tree-a", units, sealed, decision)
	if err != nil {
		t.Fatal(err)
	}
	same, err := PrefixDecisionID("base-a", "tree-a", slices.Clone(units), sealed, decision)
	if err != nil || first != same {
		t.Fatalf("identical decision changed: %s %s %v", first, same, err)
	}
	for _, change := range []struct {
		base, tree string
		units      []Unit
		sealed     map[string]Claim
		decision   PrefixDecision
	}{
		{"base-b", "tree-a", units, sealed, decision},
		{"base-a", "tree-b", units, sealed, decision},
		{"base-a", "tree-a", units, map[string]Claim{"goal-a": {Revision: 3, AccountingRevision: 3}}, decision},
		{"base-a", "tree-a", units, sealed, PrefixDecision{Groups: []string{"a", "b"}, PolicyContext: decision.PolicyContext}},
		{"base-a", "tree-a", units, sealed, PrefixDecision{Groups: []string{"a"}, PolicyContext: "protected-base=base-b; contract=contract-a"}},
	} {
		changed, err := PrefixDecisionID(change.base, change.tree, change.units, change.sealed, change.decision)
		if err != nil || changed == first {
			t.Fatalf("changed decision reused receipt: %s %v", changed, err)
		}
	}
}

func TestGLEBatchReceiptReplansAndVerifiesBeforeReuse(t *testing.T) {
	t.Parallel()
	_, store := prefixReceiptBed(t)
	prepared := load(t, store)
	prepared.Seal = map[string]Claim{"goal-a": {Revision: 2}}
	must(t, store.Update(testBatchID, func(record *Record) error { record.Seal = prepared.Seal; return nil }))
	decision := PrefixDecision{Groups: []string{"a"}, PolicyContext: "policy-a"}
	executions, verifications := 0, 0
	seams := PrefixReceiptSeams{
		Plan: func(_ []Unit, _ string) (PrefixDecision, error) { return decision, nil },
		Execute: func(_, _ string, groups []string) (PrefixRunResult, error) {
			executions++
			if !slices.Equal(groups, decision.Groups) {
				t.Fatalf("executed %v, wanted %v", groups, decision.Groups)
			}
			return PrefixRunResult{AttemptID: "attempt", Executed: []string{"a"}}, nil
		},
		Verify: func(_ Unit, _ string, _ PrefixDecision) error { verifications++; return nil },
	}
	must(t, ComposePrefixReceipts(store, testBatchID, "owner", time.Unix(3, 0), seams))
	must(t, ComposePrefixReceipts(store, testBatchID, "owner", time.Unix(4, 0), seams))
	if executions != 1 || verifications != 2 {
		t.Fatalf("same decision executions=%d verifications=%d", executions, verifications)
	}
	decision.PolicyContext = "policy-b"
	must(t, ComposePrefixReceipts(store, testBatchID, "owner", time.Unix(5, 0), seams))
	if executions != 2 || verifications != 3 {
		t.Fatalf("changed policy executions=%d verifications=%d", executions, verifications)
	}
}

func TestGLEBatchCachedReceiptCannotOutliveMemberFence(t *testing.T) {
	t.Parallel()
	_, store := prefixReceiptBed(t)
	prepared := load(t, store)
	prepared.Seal = map[string]Claim{"goal-a": {Revision: 2, AccountingRevision: 1}}
	must(t, store.Update(testBatchID, func(record *Record) error { record.Seal = prepared.Seal; return nil }))
	executions := 0
	seams := PrefixReceiptSeams{
		Plan: func([]Unit, string) (PrefixDecision, error) {
			return PrefixDecision{Groups: []string{"a"}, PolicyContext: "policy"}, nil
		},
		Execute: func(string, string, []string) (PrefixRunResult, error) {
			executions++
			return PrefixRunResult{AttemptID: "green"}, nil
		},
		Verify:    func(Unit, string, PrefixDecision) error { return nil },
		Authorize: func(Unit, Claim) error { return nil },
	}
	must(t, ComposePrefixReceipts(store, testBatchID, "owner", time.Unix(3, 0), seams))
	seams.Authorize = func(Unit, Claim) error { return &PrefixFencedRefusal{Reason: "CANDIDATE_GOAL_REFUSED state=fenced"} }
	must(t, ComposePrefixReceipts(store, testBatchID, "owner", time.Unix(4, 0), seams))
	record := load(t, store)
	if executions != 1 || record.State != StateOpen || record.Units[0].State != UnitReturnPending || record.Units[0].Outcome != UnitEjected {
		t.Fatalf("cached green escaped live fence: executions=%d state=%s unit=%+v", executions, record.State, record.Units[0])
	}
}

func TestGLEBatchFreshEpisodeSurvivesRestartAndRenewsOnExpiryOrBaseMove(t *testing.T) {
	t.Parallel()
	_, store := prefixReceiptBed(t)
	prepared := load(t, store)
	prepared.Seal = map[string]Claim{"goal-a": {Revision: 2, AccountingRevision: 1}}
	must(t, store.Update(testBatchID, func(record *Record) error { record.Seal = prepared.Seal; return nil }))
	decision := PrefixDecision{Groups: []string{"fresh"}, PolicyContext: "policy", FreshRequired: true, FreshMaxAgeMS: 1000}
	var observed []string
	seams := PrefixReceiptSeams{
		Plan: func([]Unit, string) (PrefixDecision, error) { return decision, nil },
		ExecuteDecision: func(_ string, _ string, planned PrefixDecision) (PrefixRunResult, error) {
			observed = append(observed, planned.FreshEpisode)
			if len(planned.FreshEpisode) != 64 || planned.FreshExpiresAt == "" {
				t.Fatalf("fresh decision lacked durable expiry: %+v", planned)
			}
			return PrefixRunResult{AttemptID: "fresh"}, nil
		},
		Verify: func(Unit, string, PrefixDecision) error { return nil },
	}
	must(t, ComposePrefixReceipts(store, testBatchID, "owner", time.Unix(3, 0), seams))
	first := load(t, store).PrefixEpisodes["goal-a"]
	must(t, ComposePrefixReceipts(store, testBatchID, "owner", time.Unix(3, 500000000), seams))
	if len(observed) != 1 || load(t, store).PrefixEpisodes["goal-a"] != first {
		t.Fatal("unchanged restart replaced its fresh episode")
	}
	must(t, ComposePrefixReceipts(store, testBatchID, "owner", time.Unix(5, 0), seams))
	if len(observed) != 2 || observed[1] == observed[0] {
		t.Fatal("expired pending episode reused old observation")
	}
	must(t, store.Update(testBatchID, func(record *Record) error { record.BaseTree = "moved-base"; return nil }))
	must(t, ComposePrefixReceipts(store, testBatchID, "owner", time.Unix(5, 500000000), seams))
	if len(observed) != 3 || observed[2] == observed[1] {
		t.Fatal("moved destination base retained stale episode")
	}
}

func TestGLEBatchFencedQueuedMemberReturnsAndReassemblesSurvivor(t *testing.T) {
	t.Parallel()
	_, store := prefixReceiptBed(t)
	err := ComposePrefixReceipts(store, testBatchID, "owner", time.Unix(3, 0), PrefixReceiptSeams{Execute: func(string, string, []string) (PrefixRunResult, error) {
		return PrefixRunResult{}, &PrefixFencedRefusal{Reason: "CANDIDATE_GOAL_REFUSED state=fenced"}
	}})
	if err != nil {
		t.Fatal(err)
	}
	record := load(t, store)
	if record.State != StateOpen || record.Units[0].State != UnitReturnPending || record.Units[0].Outcome != UnitEjected || record.Units[1].State != UnitJoined || len(record.PrefixTrees) != 1 {
		t.Fatalf("fenced member stalled survivors: state=%s units=%+v prefixes=%v", record.State, record.Units, record.PrefixTrees)
	}
	if !strings.Contains(record.Units[0].Failure, "state=fenced") {
		t.Fatal("fence reason was lost")
	}
}

func TestGLEBatchTipGreenCannotHideRedEarlierPrefix(t *testing.T) {
	t.Parallel()
	_, store := prefixReceiptBed(t)
	err := ComposePrefixReceipts(store, testBatchID, "owner", time.Unix(3, 0), PrefixReceiptSeams{
		Execute: func(_, _ string, _ []string) (PrefixRunResult, error) {
			return PrefixRunResult{AttemptID: "red-prefix", Red: []RedGroup{{ID: "earlier-only", Status: "failed", InputManifest: []string{"a.txt"}}}}, nil
		},
	})
	var red *PrefixRedError
	if !errors.As(err, &red) || red.GoalID != "goal-a" {
		t.Fatalf("green tip hid red prefix: %v", err)
	}
	record := load(t, store)
	if record.State != StateDiagnosing || record.Proof.Status != "prefix-red" || record.Proof.PrefixGoal != "goal-a" || len(record.Receipts) != 0 {
		t.Fatalf("red prefix admitted landing: state=%s proof=%+v receipts=%v", record.State, record.Proof, record.Receipts)
	}
}
