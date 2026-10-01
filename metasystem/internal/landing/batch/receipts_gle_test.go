package batch

import (
	"slices"
	"testing"
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
