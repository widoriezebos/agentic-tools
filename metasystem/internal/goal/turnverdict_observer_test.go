package goal

import (
	"context"
	"testing"

	metarun "github.com/widoriezebos/agentic-tools/metasystem/internal/run"
)

func (fixture *pendingWaitVerdictFixture) expectLedgerObservation(t *testing.T, expected int, row metarun.Waiter, reply metarun.SourceObservation) {
	t.Helper()
	calls := 0
	fixture.store.verdictDeps.observeLedger = func(ctx context.Context, root string, selector metarun.WaitSelector, target metarun.WaiterTarget, lastTip, lineage string) (metarun.SourceObservation, error) {
		t.Helper()
		calls++
		if calls > expected {
			t.Fatalf("unexpected ledger observation call %d, expected %d", calls, expected)
		}
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("ledger observation has no context deadline")
		}
		if root != fixture.root || selector != row.Selector || target != row.Target || lastTip != row.LastCheckedTip || lineage != row.OwnerLineage {
			t.Fatalf("ledger observation arguments: root=%q selector=%+v target=%+v lastTip=%q lineage=%q; want root=%q selector=%+v target=%+v lastTip=%q lineage=%q",
				root, selector, target, lastTip, lineage, fixture.root, row.Selector, row.Target, row.LastCheckedTip, row.OwnerLineage)
		}
		return reply, nil
	}
	t.Cleanup(func() {
		if calls != expected {
			t.Errorf("ledger observation calls = %d, want %d", calls, expected)
		}
	})
}
