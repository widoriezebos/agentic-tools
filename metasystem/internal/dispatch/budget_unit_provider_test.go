package dispatch

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/outage"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testprovider"
)

func TestProviderClockRequiresLocalStartOnlyWithModelWork(t *testing.T) {
	t.Parallel()
	for _, model := range []bool{false, true} {
		name := "local accounting"
		if model {
			name = "model dependency"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			home := testprovider.Register(t, t.TempDir())
			start := time.Date(2026, 8, 28, 8, 0, 0, 0, time.UTC)
			dependencies := []clockDependency{{runtime: "local"}}
			if model {
				dependencies = append(dependencies, clockDependency{at: start, until: start.Add(time.Minute), runtime: "claude-headless"})
			}
			spans, err := providerWaits(home, "bounded", start, start.Add(time.Hour), dependencies)
			if model {
				if err == nil || !strings.Contains(err.Error(), "local work dependency time") {
					t.Fatalf("unknown local start allowed model pause subtraction: spans=%v err=%v", spans, err)
				}
			} else if err != nil || len(spans) != 0 {
				t.Fatalf("local accounting required a provider start: spans=%v err=%v", spans, err)
			}
		})
	}
}

func TestBudgetUnitReservationUsesPhysicalLaunchProvider(t *testing.T) {
	t.Parallel()
	root := budgetProjectionRoot(t)
	home := testprovider.Register(t, root)
	file := budgetGoal()
	file.StopCapability = &goal.StopCapability{Generation: 3, Revision: 3, Machine: file.Claimed.Machine, ClaimEpoch: 1}
	start := time.Date(2026, 8, 28, 8, 10, 0, 0, time.UTC)
	end, now := start.Add(time.Minute), start.Add(5*time.Minute)
	if err := ReserveUnitLaunch(root, "execution", "unit", file, 1, 5, start); err != nil {
		t.Fatal(err)
	}
	if err := ReconcileUnitLaunch(root, "execution", "unit", file.Id, file.Claimed.Revision, "failed", "environment", start.Format(time.RFC3339Nano), end.Format(time.RFC3339Nano), true); err != nil {
		t.Fatal(err)
	}
	store := launch.Store{Root: filepath.Join(home, "launch")}
	if err := store.Create(launch.Record{ID: "execution", Goal: file.Id, Kind: "build", Adapter: "claude-headless", State: launch.Failed,
		Cause: outage.ProviderLimit, StartedAt: start.Format(time.RFC3339Nano), FinishedAt: end.Format(time.RFC3339Nano)}); err != nil {
		t.Fatal(err)
	}
	if _, err := outage.Observe(home, "claude", "fixture-model", "overloaded", "529", "fixture", end); err != nil {
		t.Fatal(err)
	}
	projection := ProjectBudget(root, file, now, home)
	if projection.Status != BudgetKnown || projection.Wait != 4*time.Minute || projection.Elapsed != 11*time.Minute {
		t.Fatalf("physical provider pause was duplicated or lost: %+v", projection)
	}
	if projection.Attempts != 0 || projection.ReservedJobMinutes != 0 {
		t.Fatalf("environment failure consumed a unit attempt or charged capacity: %+v", projection)
	}
}
