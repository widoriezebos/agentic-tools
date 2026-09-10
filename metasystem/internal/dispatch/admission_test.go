package dispatch

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/run"
)

func admissionBudgetBed(t *testing.T, attemptLimit, reservedLimit, activeLimit uint64) string {
	t.Helper()
	root := revisionBindingBed(t, 2)
	path := filepath.Join(root, "plans", "goals", "bounded.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	file, problems := goal.ParseFile(data)
	if len(problems) != 0 {
		t.Fatalf("admission fixture goal did not parse: %v", problems)
	}
	file.Claimed.Revision = 3
	file.Claimed.At = file.History[2].At
	file.Claimed.AccountingRevision = 3
	file.StopCapability.Generation = 3
	file.StopCapability.Revision = 3
	file.Budget.AttemptLimit = attemptLimit
	file.Budget.ReservedJobMinutesLimit = reservedLimit
	file.Budget.ActiveJobLimit = activeLimit
	if err := os.WriteFile(path, goal.RenderFile(file), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "plans/goals/bounded.md"}, {"commit", "-q", "-m", "admission budget fixture"}, {"update-ref", goal.AcceptedRef, "HEAD"}} {
		if output, runErr := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); runErr != nil {
			t.Fatalf("git %v: %v: %s", args, runErr, output)
		}
	}
	return root
}

func TestEveryBudgetRefusalNamesObservedAndOpenCaps(t *testing.T) {
	t.Run("attempt-only refusal", func(t *testing.T) {
		root := admissionBudgetBed(t, 1, 10000, 10)
		writeBudgetJob(t, root, "settled", "reserve-settled", 3, 120, "completed", budgetJobLife{
			startedAt: "2026-08-28T09:40:00Z", endedAt: "2026-08-28T09:41:00Z", pid: 4242,
		})
		verdict, err := EvaluateGoalRevisionAdmission(root, "bounded", 3, 120, time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC))
		if err != nil || verdict.Refusal == nil {
			t.Fatalf("attempt boundary was not refused: %+v %v", verdict, err)
		}
		wantReserved := &ReservedMinutesEvidence{Observed: 1, OpenCaps: 0, Limit: 10000}
		if verdict.Refusal.Reserved == nil || *verdict.Refusal.Reserved != *wantReserved {
			t.Fatalf("attempt refusal reserved evidence = %+v, want %+v", verdict.Refusal.Reserved, wantReserved)
		}
		lines := FormatGoalAdmission(GoalAdmissionVerdict{Refusals: []GoalAdmissionRefusal{*verdict.Refusal}})
		want := "BUDGET_REFUSED: goal bounded revision=3 admission closed: attemptLimit used=1 limit=1; reserved observed=1 open-caps=0 limit=10000; rule=setup-refusal-release: terminal records with phase=setup and refusalClass=setup count as neither attempts nor reserved job minutes"
		if len(lines) != 1 || lines[0] != want {
			t.Fatalf("attempt refusal line = %q, want %q", lines, want)
		}
	})

	t.Run("reserved-minute refusal", func(t *testing.T) {
		root := admissionBudgetBed(t, 10, 240, 10)
		writeBudgetJob(t, root, "settled", "reserve-settled", 3, 120, "completed", budgetJobLife{
			startedAt: "2026-08-28T09:35:00Z", endedAt: "2026-08-28T10:25:00Z", pid: 4242,
		})
		writeBudgetJob(t, root, "running", "reserve-running", 3, 120, "running", budgetJobLife{})
		verdict, err := EvaluateGoalRevisionAdmission(root, "bounded", 3, 120, time.Date(2026, 8, 28, 11, 0, 0, 0, time.UTC))
		if err != nil || verdict.Refusal == nil {
			t.Fatalf("reserved-minute proposal was not refused: %+v %v", verdict, err)
		}
		wantReserved := &ReservedMinutesEvidence{Observed: 50, OpenCaps: 120, Limit: 240}
		if verdict.Refusal.Reserved == nil || *verdict.Refusal.Reserved != *wantReserved {
			t.Fatalf("reserved-minute refusal evidence = %+v, want %+v", verdict.Refusal.Reserved, wantReserved)
		}
		lines := FormatGoalAdmission(GoalAdmissionVerdict{Refusals: []GoalAdmissionRefusal{*verdict.Refusal}})
		want := "BUDGET_REFUSED: goal bounded revision=3 admission closed: reservedJobMinutesLimit used=170+120 proposed limit=240; reserved observed=50 open-caps=120 limit=240; rule=setup-refusal-release: terminal records with phase=setup and refusalClass=setup count as neither attempts nor reserved job minutes"
		if len(lines) != 1 || lines[0] != want {
			t.Fatalf("reserved-minute refusal line = %q, want %q", lines, want)
		}
	})

	t.Run("multiple breach fields retain their comma", func(t *testing.T) {
		lines := FormatGoalAdmission(GoalAdmissionVerdict{Refusals: []GoalAdmissionRefusal{{
			GoalID: "bounded", GoalRevision: 3,
			Breaches: []BudgetBreach{
				budgetIntegerBreach("attemptLimit", 2, 2),
				budgetIntegerBreach("activeJobLimit", 1, 1),
			},
			Reserved: &ReservedMinutesEvidence{Observed: 1, OpenCaps: 30, Limit: 10000},
		}}})
		want := "BUDGET_REFUSED: goal bounded revision=3 admission closed: attemptLimit used=2 limit=2, activeJobLimit used=1 limit=1; reserved observed=1 open-caps=30 limit=10000; rule=setup-refusal-release: terminal records with phase=setup and refusalClass=setup count as neither attempts nor reserved job minutes"
		if len(lines) != 1 || lines[0] != want {
			t.Fatalf("multi-limit refusal line = %q, want %q", lines, want)
		}
	})

	t.Run("unknown projection invents no reserved evidence", func(t *testing.T) {
		root := admissionBudgetBed(t, 10, 240, 10)
		writeJSON(t, filepath.Join(root, "artifacts", "agents", "jobs", "revisionless.json"), map[string]any{
			"jobId": "revisionless", "operationId": "reserve-revisionless", "goalId": "bounded", "capMin": 120, "status": "running",
		})
		verdict, err := EvaluateGoalRevisionAdmission(root, "bounded", 3, 120, time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC))
		if err != nil || verdict.Refusal == nil || verdict.Refusal.Unknown == nil || verdict.Refusal.Reserved != nil {
			t.Fatalf("unknown projection did not remain evidence-free: %+v %v", verdict, err)
		}
		lines := FormatGoalAdmission(GoalAdmissionVerdict{Refusals: []GoalAdmissionRefusal{*verdict.Refusal}})
		if len(lines) != 1 || strings.Contains(lines[0], "reserved observed=") || strings.Contains(lines[0], "rule=setup-refusal-release") {
			t.Fatalf("unknown refusal invented reserved evidence or a release rule: %v", lines)
		}
	})

	t.Run("governed refusal", func(t *testing.T) {
		root := revisionBindingBed(t, 2)
		obligationRevision := installEnforcedObligation(t, root, 5)
		writeBudgetJob(t, root, "running", "reserve-running", 2, 60, "running", budgetJobLife{})
		_, err := EvaluateGovernedRunAdmission(root, run.GovernedAdmissionRequest{
			GoalID: "bounded", ObligationRevision: obligationRevision, StandingShared: true,
		}, time.Date(2026, 8, 28, 10, 30, 0, 0, time.UTC))
		if err == nil || !strings.Contains(err.Error(), "; reserved observed=0 open-caps=60 limit=60") ||
			strings.Contains(err.Error(), "rule=setup-refusal-release") {
			t.Fatalf("governed refusal did not carry shared reserved evidence: %v", err)
		}
	})
}
