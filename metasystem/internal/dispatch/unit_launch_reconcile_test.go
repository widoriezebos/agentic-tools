package dispatch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

func TestReconcileRepairsAForeignTerminalRecord(t *testing.T) {
	t.Parallel()
	for _, status := range []string{"cancelled", "failed", "owned"} {
		t.Run(status, func(t *testing.T) {
			t.Parallel()
			root, file := budgetProjectionRoot(t), budgetGoal()
			file.StopCapability = &goal.StopCapability{Generation: 3, Revision: 3, Machine: file.Claimed.Machine, ClaimEpoch: 1}
			start := time.Date(2026, 8, 28, 8, 10, 0, 0, time.UTC)
			end := start.Add(time.Minute)
			if err := ReserveUnitLaunch(root, "execution", "unit", file, 1, 5, start, false); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, "artifacts/agents/jobs/execution.json")
			record, err := readObject(path)
			if err != nil {
				t.Fatal(err)
			}
			record["status"], record["endedAt"] = status, end.Format(time.RFC3339Nano)
			if status == "failed" {
				record["phase"], record["error"] = "claim-sweep", "stale-claim-epoch"
			} else {
				record["phase"] = "cancelled"
			}
			if status == "owned" {
				record["status"], record["unitExecuted"], record["startedAt"] = "failed", false, start.Format(time.RFC3339Nano)
			}
			if err := writeRecord(path, record); err != nil {
				t.Fatal(err)
			}
			before, _ := os.ReadFile(path)
			for repeat := 0; repeat < 2; repeat++ {
				err := ReconcileUnitLaunch(root, "execution", "unit", file.Id, 3, "completed", "", start.Format(time.RFC3339Nano), end.Format(time.RFC3339Nano), true)
				if status == "owned" {
					if err == nil || !strings.Contains(err.Error(), "contradicts its terminal reservation") {
						t.Fatalf("a conflicting owner outcome was accepted: %v", err)
					}
					after, _ := os.ReadFile(path)
					if string(after) != string(before) {
						t.Fatal("a conflicting collection changed the reservation")
					}
					continue
				}
				if err != nil {
					t.Fatal(err)
				}
				settled, err := readObject(path)
				if err != nil || settled["status"] != "completed" || settled["startedAt"] != start.Format(time.RFC3339Nano) || settled["endedAt"] != end.Format(time.RFC3339Nano) || settled["unitExecuted"] != true || settled["phase"] != record["phase"] || settled["error"] != record["error"] {
					t.Fatalf("collection lost outcome or history: %v, %v", settled, err)
				}
				projection := ProjectBudget(root, file, end)
				if projection.Status != BudgetKnown || projection.Attempts != 1 || projection.ReservedJobMinutes != 1 {
					t.Fatalf("repaired outcome did not charge the attempt: %+v", projection)
				}
			}
		})
	}
}
