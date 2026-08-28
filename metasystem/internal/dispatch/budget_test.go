package dispatch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

func budgetGoal() *goal.GoalFile {
	return &goal.GoalFile{
		Id: "bounded", State: goal.StateClaimed, Revision: 5,
		Claimed: &goal.ClaimRecord{
			Machine: "bed-m1", Lineage: "coordinator", At: "2026-08-28T08:00:00Z", Revision: 3,
		},
		Budget: &goal.Budget{
			ElapsedLimit: "4h", AttemptLimit: 2, ReservedJobMinutesLimit: 75, ActiveJobLimit: 1,
		},
		History: []goal.HistoryLine{
			{At: "2026-08-28T06:00:00Z"},
			{At: "2026-08-28T07:00:00Z"},
			{At: "2026-08-28T08:00:00Z"},
			{At: "2026-08-28T08:30:00Z"},
			{At: "2026-08-28T09:00:00Z"},
		},
	}
}

func writeBudgetJob(t *testing.T, root, name, operation string, revision, cap uint64, status string) {
	t.Helper()
	writeJSON(t, filepath.Join(root, "artifacts", "agents", "jobs", name+".json"), map[string]any{
		"jobId": name, "operationId": operation, "goalId": "bounded",
		"goalRevision": revision, "capMin": cap, "status": status,
	})
}

func TestBudgetProjectionUsesJobRecordsForTheBoundRevision(t *testing.T) {
	root := t.TempDir()
	writeBudgetJob(t, root, "done", "reserve-a", 3, 30, "completed")
	writeBudgetJob(t, root, "live", "reserve-b", 3, 45, "running")
	writeBudgetJob(t, root, "old-revision", "reserve-old", 2, 60, "completed")
	writeJSON(t, filepath.Join(root, "artifacts", "agents", "jobs", "unrelated.json"), map[string]any{
		"jobId": "unrelated", "goalId": nil, "status": "completed",
	})

	projection := ProjectBudget(root, budgetGoal(), time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC))
	if projection.Status != BudgetKnown || projection.GoalRevision != 3 || projection.Attempts != 2 ||
		projection.ReservedJobMinutes != 75 || projection.ActiveJobs != 1 || projection.Elapsed != 2*time.Hour ||
		len(projection.Breaches) != 0 {
		t.Fatalf("projection did not use the sole reservation facts: %+v", projection)
	}
}

func TestPublishedSetupRetainsAttemptAndReservedMinutes(t *testing.T) {
	root := sandbox(t)
	stage := t.TempDir()
	capFile := writeJSON(t, filepath.Join(stage, "cap.json"), map[string]any{
		"capMin": 30, "capDeadline": "2026-08-28T10:00:00Z",
		"source": map[string]any{"rule": "fixture", "origin": "fixture", "truncatedBy": nil},
	})
	setup := filepath.Join(stage, "setup.json")
	if err := BuildSetup(setup, "reserved", "implementer", "", "main-1", "5", "bounded", 3, capFile); err != nil {
		t.Fatal(err)
	}
	if err := RecordCreate(root, "reserved", setup); err != nil {
		t.Fatal(err)
	}

	projection := ProjectBudget(root, budgetGoal(), time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC))
	if projection.Status != BudgetKnown || projection.Attempts != 1 || projection.ReservedJobMinutes != 30 || projection.ActiveJobs != 1 {
		t.Fatalf("a crash after setup publication lost reservation spend: %+v", projection)
	}
}

func TestBudgetProjectionReportsExactUnknownRecord(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(root string)
		want   string
	}{
		{
			name: "revisionless",
			mutate: func(root string) {
				writeJSON(t, filepath.Join(root, "artifacts", "agents", "jobs", "lost.json"), map[string]any{
					"jobId": "lost", "operationId": "reserve-lost", "goalId": "bounded", "capMin": 10, "status": "running",
				})
			},
			want: "artifacts/agents/jobs/lost.json",
		},
		{
			name: "duplicate operation",
			mutate: func(root string) {
				writeBudgetJob(t, root, "first", "reserve-same", 3, 10, "completed")
				writeBudgetJob(t, root, "second", "reserve-same", 3, 10, "running")
			},
			want: "artifacts/agents/jobs/second.json",
		},
		{
			name: "contradictory unbound revision",
			mutate: func(root string) {
				writeJSON(t, filepath.Join(root, "artifacts", "agents", "jobs", "contradictory.json"), map[string]any{
					"jobId": "contradictory", "operationId": "reserve-contradictory", "goalId": nil,
					"goalRevision": 3, "capMin": 10, "status": "running",
				})
			},
			want: "artifacts/agents/jobs/contradictory.json",
		},
		{
			name: "unreadable",
			mutate: func(root string) {
				path := filepath.Join(root, "artifacts", "agents", "jobs", "broken.json")
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				// Duplicate keys are unreadable under the authoritative wire grammar.
				data := []byte("{\"goalId\":\"bounded\",\"goalId\":\"bounded\"}\n")
				if err := os.WriteFile(path, data, 0o644); err != nil {
					t.Fatal(err)
				}
			},
			want: "artifacts/agents/jobs/broken.json",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			test.mutate(root)
			projection := ProjectBudget(root, budgetGoal(), time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC))
			if projection.Status != BudgetUnknown || projection.Unknown == nil || projection.Unknown.Code != BudgetUnknown ||
				projection.Unknown.Record != test.want {
				t.Fatalf("unknown evidence did not name the exact record: %+v", projection)
			}
		})
	}
}

func TestBudgetProjectionSurfacesBreachesWithoutEnforcement(t *testing.T) {
	root := t.TempDir()
	writeBudgetJob(t, root, "one", "reserve-one", 3, 50, "running")
	writeBudgetJob(t, root, "two", "reserve-two", 3, 50, "pending")
	writeBudgetJob(t, root, "three", "reserve-three", 3, 10, "completed")

	projection := ProjectBudget(root, budgetGoal(), time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC))
	if projection.Status != BudgetKnown || len(projection.Breaches) != 4 {
		t.Fatalf("all four breaches are health evidence, not a projection refusal: %+v", projection)
	}
	var fields []string
	for _, breach := range projection.Breaches {
		fields = append(fields, breach.Field)
	}
	if strings.Join(fields, ",") != "elapsedLimit,attemptLimit,reservedJobMinutesLimit,activeJobLimit" {
		t.Fatalf("breach fields = %v", fields)
	}
}

func TestBudgetAdmissionClosesAtEveryCurrentEqualityBoundary(t *testing.T) {
	projection := BudgetProjection{
		Status: BudgetKnown,
		Limits: goal.Budget{
			ElapsedLimit: "4h", AttemptLimit: 2, ReservedJobMinutesLimit: 75, ActiveJobLimit: 1,
		},
		Elapsed: 4 * time.Hour, Attempts: 2, ReservedJobMinutes: 75, ActiveJobs: 1,
	}
	breaches := budgetAdmissionBreaches(projection)
	var fields []string
	for _, breach := range breaches {
		fields = append(fields, breach.Field)
	}
	if strings.Join(fields, ",") != "elapsedLimit,attemptLimit,reservedJobMinutesLimit,activeJobLimit" {
		t.Fatalf("admission equality boundaries = %v", fields)
	}
}
