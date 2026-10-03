package steward

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/brain"
)

func TestStewardLeavesACoordinatorCheckoutAlone(t *testing.T) {
	t.Parallel()
	for _, role := range []string{"", brain.Partner} {
		for _, workers := range []Workers{{CensusComplete: true}, {Live: 1, LiveSeatMains: 1, CensusComplete: true}} {
			b := newSeatBed(t, seatReadyGoal("ready", "Build it."))
			ledger := b.projection(b.now).Tree.Root.Identity
			record := brain.Record{Schema: brain.Schema, Role: role, Ledger: ledger,
				Machine: seatBedMachine, DeclaredBy: "Wido", DeclaredAt: "2026-10-03T19:00:00Z"}
			data, _ := json.Marshal(record)
			if err := os.MkdirAll(filepath.Dir(brain.Path(b.root)), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(brain.Path(b.root), data, 0o644); err != nil {
				t.Fatal(err)
			}
			dependencies := b.dependencies()
			dependencies.LedgerIdentity = func(string) string { return ledger }
			for tick := 0; tick < 7; tick++ {
				result, err := decideTickWithDependencies(b.root, TickConfig{Now: b.now}, fakeCensus{workers: workers}, Evidence{}, Marks{}, dependencies)
				if err != nil || result.Decision.Action != ActNone || result.Seat != nil {
					t.Fatalf("role=%q tick=%d: %+v %v", role, tick, result, err)
				}
			}
			if pending, err := PendingNotifications(b.root); err != nil || len(pending) != 0 || len(b.launcher.starts) != 0 {
				t.Fatalf("role=%q: notifications=%+v starts=%+v err=%v", role, pending, b.launcher.starts, err)
			}
			if err := os.Remove(brain.Path(b.root)); err != nil {
				t.Fatal(err)
			}
			result, err := decideTickWithDependencies(b.root, TickConfig{Now: b.now}, fakeCensus{workers: Workers{CensusComplete: true}}, Evidence{}, Marks{}, dependencies)
			if err != nil || result.Decision.Action != ActRevive || result.Seat == nil {
				t.Fatalf("undeclared checkout must still select a seat: %+v %v", result, err)
			}
		}
	}
}
