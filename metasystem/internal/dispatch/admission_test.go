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

func reviewChainBudgetBed(t *testing.T) string {
	t.Helper()
	root := revisionBindingBed(t, 2)
	path := filepath.Join(root, "plans", "goals", "bounded.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	file, problems := goal.ParseFile(data)
	if len(problems) != 0 {
		t.Fatalf("parse goal fixture: %v", problems)
	}
	file.Budget.AttemptLimit = 20
	file.Budget.ReservedJobMinutesLimit = 1000
	file.Budget.ActiveJobLimit = 10
	file.Budget.ReviewRoundLimit = 2
	if err := os.WriteFile(path, goal.RenderFile(file), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "plans/goals/bounded.md"}, {"commit", "-q", "-m", "review chain budget bed"}, {"update-ref", goal.AcceptedRef, "HEAD"}} {
		command := exec.Command("git", append([]string{"-C", root}, args...)...)
		command.Env = []string{"PATH=" + os.Getenv("PATH"), "LC_ALL=C"}
		if output, runErr := command.CombinedOutput(); runErr != nil {
			t.Fatalf("git %v: %v: %s", args, runErr, output)
		}
	}
	return root
}

func writeCountedCriticRootAtRevision(t *testing.T, root, job, role string, revision uint64) {
	t.Helper()
	writeJSON(t, filepath.Join(root, "artifacts", "agents", "jobs", job+".json"), map[string]any{
		"jobId": job, "operationId": job, "role": role, "parentJob": nil,
		"goalId": "bounded", "goalRevision": revision, "capMin": 1, "status": "completed",
		reviewChainCountedField: true,
	})
}

func writeCountedCriticRoot(t *testing.T, root, job, role string) {
	t.Helper()
	writeCountedCriticRootAtRevision(t, root, job, role, 2)
}

func amendReviewChainBudgetBed(t *testing.T, root, message string, mutate func(*goal.GoalFile)) {
	t.Helper()
	path := filepath.Join(root, "plans", "goals", "bounded.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	file, problems := goal.ParseFile(data)
	if len(problems) != 0 {
		t.Fatalf("parse goal fixture before amendment: %v", problems)
	}
	mutate(file)
	if err := os.WriteFile(path, goal.RenderFile(file), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "plans/goals/bounded.md"}, {"commit", "-q", "-m", message}, {"update-ref", goal.AcceptedRef, "HEAD"}} {
		command := exec.Command("git", append([]string{"-C", root}, args...)...)
		command.Env = []string{"PATH=" + os.Getenv("PATH"), "LC_ALL=C"}
		if output, runErr := command.CombinedOutput(); runErr != nil {
			t.Fatalf("git %v: %v: %s", args, runErr, output)
		}
	}
}

func TestGoalRevisionAdmissionRefusesThirdCodeCritiqueChain(t *testing.T) {
	root := reviewChainBudgetBed(t)
	writeCountedCriticRoot(t, root, "code-one", "code-critic")
	writeCountedCriticRoot(t, root, "code-two", "code-critic")
	now := time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC)

	verdict, err := EvaluateGoalRevisionAdmissionForDispatch(root, "bounded", 2, 1, now, "code-critic", "fresh", HazardMechanical)
	if err != nil || !verdict.Refused() || verdict.Refusal == nil {
		t.Fatalf("third code critique was not refused: verdict=%+v err=%v", verdict, err)
	}
	lines := FormatGoalAdmission(GoalAdmissionVerdict{Refusals: []GoalAdmissionRefusal{*verdict.Refusal}})
	want := "BUDGET_REFUSED: goal bounded revision=2 admission closed: codeCritiques=2/2; reserved observed=0 open-caps=0 limit=1000"
	if len(lines) != 1 || lines[0] != want {
		t.Fatalf("third code critique refusal = %v, want %q", lines, want)
	}
}

func TestGoalRevisionAdmissionKeepsCritiqueClassesSeparate(t *testing.T) {
	root := reviewChainBudgetBed(t)
	now := time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC)
	sequence := []struct{ job, role string }{
		{"design-one", "design-critic"}, {"design-two", "design-critic"},
		{"code-one", "code-critic"}, {"code-two", "code-critic"},
	}
	for _, step := range sequence {
		verdict, err := EvaluateGoalRevisionAdmissionForDispatch(root, "bounded", 2, 1, now, step.role, "fresh", HazardMechanical)
		if err != nil || verdict.Refused() {
			t.Fatalf("%s was not admitted before its class reached two chains: verdict=%+v err=%v", step.job, verdict, err)
		}
		writeCountedCriticRoot(t, root, step.job, step.role)
	}
	for _, role := range []string{"design-critic", "code-critic"} {
		verdict, err := EvaluateGoalRevisionAdmissionForDispatch(root, "bounded", 2, 1, now, role, "fresh", HazardMechanical)
		if err != nil || !verdict.Refused() || verdict.Refusal == nil {
			t.Fatalf("third %s chain was admitted: verdict=%+v err=%v", role, verdict, err)
		}
		lines := FormatGoalAdmission(GoalAdmissionVerdict{Refusals: []GoalAdmissionRefusal{*verdict.Refusal}})
		field := "codeCritiques"
		if role == "design-critic" {
			field = "designCritiques"
		}
		want := field + "=2/2; reserved observed=0 open-caps=0 limit=1000"
		if len(lines) != 1 || !strings.HasSuffix(lines[0], want) {
			t.Fatalf("%s refusal lost its separate class count: %v", role, lines)
		}
	}
}

func TestGoalRevisionAdmissionDoesNotChargeFollowUpOrLegacyRootsAsNewChains(t *testing.T) {
	root := reviewChainBudgetBed(t)
	writeCountedCriticRoot(t, root, "code-one", "code-critic")
	writeCountedCriticRoot(t, root, "code-two", "code-critic")
	legacy := filepath.Join(root, "artifacts", "agents", "jobs", "legacy-code.json")
	writeJSON(t, legacy, map[string]any{
		"jobId": "legacy-code", "operationId": "legacy-code", "role": "code-critic", "parentJob": nil,
		"goalId": "bounded", "goalRevision": 2, "capMin": 1, "status": "completed",
	})
	now := time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC)

	verdict, err := EvaluateGoalRevisionAdmissionForDispatch(root, "bounded", 2, 1, now, "code-critic", "follow-up", HazardMechanical)
	if err != nil || verdict.Refused() {
		t.Fatalf("follow-up inside a counted critic root consumed another chain: verdict=%+v err=%v", verdict, err)
	}
}

func TestGoalRevisionAdmissionDoesNotChargeSetupRefusedCriticRoot(t *testing.T) {
	root := reviewChainBudgetBed(t)
	writeJSON(t, filepath.Join(root, "artifacts", "agents", "jobs", "setup-refused.json"), map[string]any{
		"jobId": "setup-refused", "operationId": "setup-refused", "role": "code-critic", "parentJob": nil,
		"goalId": "bounded", "goalRevision": 2, "capMin": 1, "status": "failed", "phase": "setup", "refusalClass": "setup",
		reviewChainCountedField: true,
	})
	now := time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC)
	for _, job := range []string{"code-one", "code-two"} {
		verdict, err := EvaluateGoalRevisionAdmissionForDispatch(root, "bounded", 2, 1, now, "code-critic", "fresh", HazardMechanical)
		if err != nil || verdict.Refused() {
			t.Fatalf("%s was refused after a setup refusal that consumed no budget: verdict=%+v err=%v", job, verdict, err)
		}
		writeCountedCriticRoot(t, root, job, "code-critic")
	}
	verdict, err := EvaluateGoalRevisionAdmissionForDispatch(root, "bounded", 2, 1, now, "code-critic", "fresh", HazardMechanical)
	if err != nil || !verdict.Refused() || verdict.Refusal == nil {
		t.Fatalf("third consumed code critique was not refused: verdict=%+v err=%v", verdict, err)
	}
	lines := FormatGoalAdmission(GoalAdmissionVerdict{Refusals: []GoalAdmissionRefusal{*verdict.Refusal}})
	if len(lines) != 1 || !strings.Contains(lines[0], "codeCritiques=2/2") {
		t.Fatalf("refusal did not name the two consumed code critiques: %v", lines)
	}
}

func TestGoalRevisionAdmissionKeepsCritiquesAcrossRiskRaiseAndResetsOnFreshClaim(t *testing.T) {
	root := reviewChainBudgetBed(t)
	writeCountedCriticRoot(t, root, "code-one", "code-critic")
	writeCountedCriticRoot(t, root, "code-two", "code-critic")
	amendReviewChainBudgetBed(t, root, "risk raise", func(file *goal.GoalFile) {
		file.Claimed.Revision = 3
		file.History[2].Reason = "Misclassified: from=1 to=3 evidence=refusal:BUDGET_REFUSED"
		file.StopCapability.Generation = 3
		file.StopCapability.Revision = 3
		file.Budget.ReviewRoundLimit = 3
	})
	now := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	projection := ProjectBudget(root, loadReviewChainGoal(t, root), now)
	if projection.Status != BudgetKnown || projection.CodeCritiques != 2 || projection.Limits.ReviewRoundLimit != 3 {
		t.Fatalf("risk raise did not preserve two code critiques inside the raised box: %+v", projection)
	}
	verdict, err := EvaluateGoalRevisionAdmissionForDispatch(root, "bounded", 3, 1, now, "code-critic", "fresh", HazardMechanical)
	if err != nil || verdict.Refused() {
		t.Fatalf("third code critique was not admitted after the box rose to three: verdict=%+v err=%v", verdict, err)
	}
	writeCountedCriticRootAtRevision(t, root, "code-three", "code-critic", 3)
	verdict, err = EvaluateGoalRevisionAdmissionForDispatch(root, "bounded", 3, 1, now, "code-critic", "fresh", HazardMechanical)
	if err != nil || !verdict.Refused() || verdict.Refusal == nil {
		t.Fatalf("fourth code critique was not refused after the raise: verdict=%+v err=%v", verdict, err)
	}
	lines := FormatGoalAdmission(GoalAdmissionVerdict{Refusals: []GoalAdmissionRefusal{*verdict.Refusal}})
	if len(lines) != 1 || !strings.Contains(lines[0], "codeCritiques=3/3") {
		t.Fatalf("raised-box refusal lost the prior chains: %v", lines)
	}
	amendReviewChainBudgetBed(t, root, "fresh claim", func(file *goal.GoalFile) {
		file.Claimed.Revision = 4
		file.Claimed.AccountingRevision = 4
		file.Claimed.At = file.History[3].At
		file.StopCapability.Generation = 4
		file.StopCapability.Revision = 4
	})
	projection = ProjectBudget(root, loadReviewChainGoal(t, root), now.Add(time.Hour))
	if projection.Status != BudgetKnown || projection.CodeCritiques != 0 || projection.Limits.ReviewRoundLimit != 3 {
		t.Fatalf("fresh claim did not restart the critique count at zero: %+v", projection)
	}
	verdict, err = EvaluateGoalRevisionAdmissionForDispatch(root, "bounded", 4, 1, now.Add(time.Hour), "code-critic", "fresh", HazardMechanical)
	if err != nil || verdict.Refused() {
		t.Fatalf("fresh claim did not restart critique accounting at zero: verdict=%+v err=%v", verdict, err)
	}
}

func TestBudgetProjectionCountsFollowUpAsAttemptButNotCriticChain(t *testing.T) {
	root := reviewChainBudgetBed(t)
	writeCountedCriticRoot(t, root, "code-one", "code-critic")
	writeJSON(t, filepath.Join(root, "artifacts", "agents", "jobs", "code-one-r2.json"), map[string]any{
		"jobId": "code-one-r2", "operationId": "code-one-r2", "role": "code-critic", "parentJob": "code-one",
		"goalId": "bounded", "goalRevision": 2, "capMin": 1, "status": "completed",
	})
	projection := ProjectBudget(root, loadReviewChainGoal(t, root), time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC))
	if projection.Status != BudgetKnown || projection.Attempts != 2 || projection.CodeCritiques != 1 {
		t.Fatalf("follow-up did not remain inside one critic chain: %+v", projection)
	}
}

func TestBudgetProjectionRejectsCountedMarkerOnFollowUp(t *testing.T) {
	root := reviewChainBudgetBed(t)
	writeJSON(t, filepath.Join(root, "artifacts", "agents", "jobs", "code-one-r2.json"), map[string]any{
		"jobId": "code-one-r2", "operationId": "code-one-r2", "role": "code-critic", "parentJob": "code-one",
		"goalId": "bounded", "goalRevision": 2, "capMin": 1, "status": "completed", reviewChainCountedField: true,
	})
	projection := ProjectBudget(root, loadReviewChainGoal(t, root), time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC))
	if projection.Status != BudgetUnknown || projection.Unknown == nil || !strings.Contains(projection.Unknown.Reason, "does not name a design-critic or code-critic chain root") {
		t.Fatalf("counted marker on a follow-up did not fail closed: %+v", projection)
	}
}

func loadReviewChainGoal(t *testing.T, root string) *goal.GoalFile {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "plans", "goals", "bounded.md"))
	if err != nil {
		t.Fatal(err)
	}
	file, problems := goal.ParseFile(data)
	if len(problems) != 0 {
		t.Fatalf("parse accepted goal fixture: %v", problems)
	}
	return file
}
