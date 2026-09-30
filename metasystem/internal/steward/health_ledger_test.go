package steward

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// One health evaluation projects the goal ledger once: the claimed-goal
// budget, stop-capability, delivery and trunk-red roles share the read that
// each of them made on its own (about 0.5 s apiece on a seat's Stop).
func TestHealthEvaluationProjectsTheLedgerOnce(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	var projections atomic.Int64
	ledger := newHealthLedger(root, now)
	ledger.readWorld = func(string) bool { return true }
	ledger.readEndpoint = func(root string) (goal.Endpoint, error) {
		return goal.Endpoint{Root: root, Remote: "local", Branch: "refs/heads/main"}, nil
	}
	ledger.project = func(goal.Endpoint, bool, time.Time) (goal.Projection, error) {
		projections.Add(1)
		return goal.Projection{Tree: &goal.TreeGoals{Live: map[string]*goal.GoalFile{}}}, nil
	}
	evaluateHealthRolesWithLedger(root, root, now, healthProbe{}, true, measureSpend, ledger)
	if got := projections.Load(); got != 1 {
		t.Fatalf("one health evaluation projected the ledger %d times, want 1", got)
	}
}
