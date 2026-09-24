package batch

import "testing"

type policyBed struct {
	root, base, moved string
	record            Record
}

func policyFixture(t *testing.T) policyBed {
	t.Helper()
	base, moved := "base-tree", "moved-tree"
	return policyBed{root: t.TempDir(), base: base, moved: moved, record: Record{
		Schema: 1, BatchID: testBatchID, State: StateOpen, TipTree: base,
		Units: []Unit{
			{GoalID: "goal-a", Chain: "chain-a", Claim: Claim{Machine: "seat", Lineage: "l", Epoch: 1, Revision: 7, AccountingRevision: 5}, State: UnitJoined},
			{GoalID: "goal-b", Chain: "chain-b", Claim: Claim{Machine: "seat", Lineage: "l", Epoch: 1, Revision: 8, AccountingRevision: 6}, State: UnitJoined},
		},
		batchRecordFields: batchRecordFields{BaseTree: base},
	}}
}
