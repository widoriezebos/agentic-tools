package dispatch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/governance"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/obligationstate"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/run"
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

func budgetProjectionRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

type budgetJobLife struct {
	startedAt string
	endedAt   string
	createdAt string
	pid       int
	provenAt  string
}

func writeBudgetJob(t *testing.T, root, name, operation string, revision, cap uint64, status string, life budgetJobLife) {
	t.Helper()
	record := map[string]any{
		"jobId": name, "operationId": operation, "goalId": "bounded",
		"goalRevision": revision, "capMin": cap, "status": status,
	}
	if life.startedAt != "" {
		record["startedAt"] = life.startedAt
	}
	if life.endedAt != "" {
		record["endedAt"] = life.endedAt
	}
	if life.createdAt != "" {
		record["createdAt"] = life.createdAt
	}
	if life.pid != 0 {
		record["pid"] = life.pid
	}
	if life.provenAt != "" {
		record["ownershipProof"] = map[string]any{"provenAt": life.provenAt, "source": "trusted-launcher"}
	}
	writeJSON(t, filepath.Join(root, "artifacts", "agents", "jobs", name+".json"), record)
}

func writeConsumedBudgetProof(t *testing.T, root, runID string, goalRevision, obligationRevision uint64, consumedAt time.Time) {
	t.Helper()
	if err := obligationstate.RecordTerminal(root, "bounded", goalRevision, obligationRevision, obligationstate.TerminalAttempt{
		RunID: runID, Status: run.StatusGreen, StartedAt: consumedAt.Add(-30 * time.Minute).Format(time.RFC3339),
		EndedAt: consumedAt.Add(-time.Minute).Format(time.RFC3339), PrunedAt: consumedAt.Add(time.Minute).Format(time.RFC3339),
		AttemptOrdinal: 1, ExecutionCostMinutes: 30, ObservedCostMinutes: 29, WeightGeneration: 1, Breaker: run.BreakerClosed,
	}); err != nil {
		t.Fatal(err)
	}
	writeJSON(t, filepath.Join(root, "artifacts", "agents", "validation-weight.json"), map[string]any{
		"schema": 1, "generation": 2,
		"consumedProofs": []any{map[string]any{
			"runId": runID, "goalId": "bounded", "goalRevision": goalRevision, "obligationRevision": obligationRevision,
			"weightGeneration": 1, "consumedAt": consumedAt.Format(time.RFC3339),
			"resetDecision": map[string]any{"apply": true, "wouldRefuse": false}, "dischargeDecision": map[string]any{"apply": true, "wouldRefuse": false},
		}},
	})
}

func raisedEpisodeGoal(revision, inheritedObligationRevision uint64) *goal.GoalFile {
	file := budgetGoal()
	for uint64(len(file.History)) < revision {
		index := len(file.History) - 5
		file.History = append(file.History, goal.HistoryLine{At: time.Date(2026, 8, 28, 9, 40+index*10, 0, 0, time.UTC).Format(time.RFC3339)})
	}
	file.Revision = revision
	file.Claimed.Revision = revision
	file.Claimed.AccountingRevision = revision
	file.Claimed.At = file.History[revision-1].At
	file.Claimed.EpisodeAt = file.History[2].At
	file.Claimed.EpisodeRevision = 3
	file.Claimed.EpisodeObligationRevision = inheritedObligationRevision
	file.Obligation = nil
	return file
}

func dischargedEpisodeGoal() *goal.GoalFile {
	file := budgetGoal()
	file.Claimed.AccountingRevision = 3
	file.Claimed.EpisodeAt = file.Claimed.At
	file.Claimed.EpisodeRevision = 3
	file.Obligation = &goal.GovernedObligation{Revision: 5}
	return file
}

func TestRaiseDoesNotResetBreachClock(t *testing.T) {
	root := budgetProjectionRoot(t)
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.budget.elapsed-grace-percent=0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	file := raisedEpisodeGoal(6, 0)
	projection := ProjectBudget(root, file, time.Date(2026, 8, 28, 13, 0, 0, 0, time.UTC))
	if projection.Status != BudgetKnown || projection.StartedAt.Format(time.RFC3339) != "2026-08-28T08:00:00Z" ||
		projection.Elapsed != 5*time.Hour || projection.ElapsedState != ElapsedBreach {
		t.Fatalf("a budget change reset or weakened the elapsed breach clock: %+v", projection)
	}
}

func TestFiveRaisesCannotOutrunTheBreaker(t *testing.T) {
	root := budgetProjectionRoot(t)
	var priorStart time.Time
	for revision := uint64(4); revision <= 8; revision++ {
		projection := ProjectBudget(root, raisedEpisodeGoal(revision, 0), time.Date(2026, 8, 28, 14, 0, 0, 0, time.UTC))
		if projection.Status != BudgetKnown || projection.StartedAt.Format(time.RFC3339) != "2026-08-28T08:00:00Z" ||
			projection.Elapsed != 6*time.Hour || projection.ElapsedState != ElapsedBreach {
			t.Fatalf("raise at revision %d bought a fresh elapsed period: %+v", revision, projection)
		}
		if !priorStart.IsZero() && !projection.StartedAt.Equal(priorStart) {
			t.Fatalf("raise at revision %d moved the elapsed start from %s to %s", revision, priorStart, projection.StartedAt)
		}
		priorStart = projection.StartedAt
	}
}

func TestRaiseAfterDischargeKeepsThePostDischargeStart(t *testing.T) {
	root := budgetProjectionRoot(t)
	dischargeAt := time.Date(2026, 8, 28, 9, 30, 0, 0, time.UTC)
	writeConsumedBudgetProof(t, root, "green-discharge", 3, 5, dischargeAt)
	before := ProjectBudget(root, dischargedEpisodeGoal(), dischargeAt.Add(5*time.Minute))
	after := ProjectBudget(root, raisedEpisodeGoal(6, 5), time.Date(2026, 8, 28, 10, 30, 0, 0, time.UTC))
	if before.Status != BudgetKnown || after.Status != BudgetKnown || !before.StartedAt.Equal(dischargeAt) || !after.StartedAt.Equal(dischargeAt) ||
		before.WeightEpoch == nil || after.WeightEpoch == nil || *before.WeightEpoch != *after.WeightEpoch || after.Elapsed != time.Hour {
		t.Fatalf("budget change lost the consumed discharge origin: before=%+v after=%+v", before, after)
	}
}

func TestSecondRaiseWithNoLiveObligation(t *testing.T) {
	root := budgetProjectionRoot(t)
	dischargeAt := time.Date(2026, 8, 28, 9, 30, 0, 0, time.UTC)
	writeConsumedBudgetProof(t, root, "green-discharge", 3, 5, dischargeAt)
	first := ProjectBudget(root, raisedEpisodeGoal(6, 5), time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC))
	secondFile := raisedEpisodeGoal(7, 5)
	second := ProjectBudget(root, secondFile, time.Date(2026, 8, 28, 10, 30, 0, 0, time.UTC))
	if secondFile.Obligation != nil || secondFile.Claimed.EpisodeObligationRevision != 5 || first.Status != BudgetKnown || second.Status != BudgetKnown ||
		!first.StartedAt.Equal(dischargeAt) || !second.StartedAt.Equal(dischargeAt) || first.WeightEpoch == nil || second.WeightEpoch == nil || *first.WeightEpoch != *second.WeightEpoch {
		t.Fatalf("a second budget change lost the inherited discharge identity: first=%+v second=%+v claim=%+v", first, second, secondFile.Claimed)
	}
}

func TestFreshEpisodeExcludesPriorDischarge(t *testing.T) {
	root := budgetProjectionRoot(t)
	dischargeAt := time.Date(2026, 8, 28, 9, 30, 0, 0, time.UTC)
	writeConsumedBudgetProof(t, root, "prior-episode-discharge", 3, 5, dischargeAt)
	fresh := raisedEpisodeGoal(8, 0)
	fresh.Claimed.EpisodeAt = fresh.History[7].At
	fresh.Claimed.EpisodeRevision = 8
	projection := ProjectBudget(root, fresh, time.Date(2026, 8, 28, 11, 0, 0, 0, time.UTC))
	if projection.Status != BudgetKnown || projection.WeightEpoch != nil || !projection.StartedAt.Equal(time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)) {
		t.Fatalf("a consumed proof crossed into the fresh ownership episode: %+v", projection)
	}
}

func TestSetObligationReturnsTheStartToTheEpisodeOrigin(t *testing.T) {
	root := budgetProjectionRoot(t)
	dischargeAt := time.Date(2026, 8, 28, 9, 30, 0, 0, time.UTC)
	writeConsumedBudgetProof(t, root, "green-discharge", 3, 5, dischargeAt)
	file := dischargedEpisodeGoal()
	file.Obligation = &goal.GovernedObligation{Revision: 7}
	projection := ProjectBudget(root, file, time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC))
	if projection.Status != BudgetKnown || projection.StartedAt.Format(time.RFC3339) != "2026-08-28T08:00:00Z" || projection.WeightEpoch != nil {
		t.Fatalf("a replacement obligation inherited the superseded proof: %+v unknown=%+v", projection, projection.Unknown)
	}
}

func TestRaiseAfterSetObligationKeepsTheSupersession(t *testing.T) {
	root := budgetProjectionRoot(t)
	dischargeAt := time.Date(2026, 8, 28, 9, 30, 0, 0, time.UTC)
	writeConsumedBudgetProof(t, root, "green-discharge", 3, 5, dischargeAt)
	file := raisedEpisodeGoal(8, 7)
	projection := ProjectBudget(root, file, time.Date(2026, 8, 28, 10, 30, 0, 0, time.UTC))
	if projection.Status != BudgetKnown || projection.StartedAt.Format(time.RFC3339) != "2026-08-28T08:00:00Z" || projection.WeightEpoch != nil {
		t.Fatalf("a budget change resurrected a proof superseded by obligation revision 7: %+v", projection)
	}
}

func TestRaiseThenSetObligationThenRaiseStaysAtTheEpisodeOrigin(t *testing.T) {
	root := budgetProjectionRoot(t)
	dischargeAt := time.Date(2026, 8, 28, 9, 30, 0, 0, time.UTC)
	writeConsumedBudgetProof(t, root, "green-discharge", 3, 5, dischargeAt)
	firstRaise := ProjectBudget(root, raisedEpisodeGoal(6, 5), time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC))
	replacement := raisedEpisodeGoal(6, 5)
	replacement.Obligation = &goal.GovernedObligation{Revision: 9}
	afterReplacement := ProjectBudget(root, replacement, time.Date(2026, 8, 28, 10, 10, 0, 0, time.UTC))
	afterSecondRaise := ProjectBudget(root, raisedEpisodeGoal(10, 9), time.Date(2026, 8, 28, 11, 0, 0, 0, time.UTC))
	if firstRaise.Status != BudgetKnown || !firstRaise.StartedAt.Equal(dischargeAt) || afterReplacement.Status != BudgetKnown || afterSecondRaise.Status != BudgetKnown ||
		afterReplacement.StartedAt.Format(time.RFC3339) != "2026-08-28T08:00:00Z" || afterSecondRaise.StartedAt.Format(time.RFC3339) != "2026-08-28T08:00:00Z" ||
		afterReplacement.WeightEpoch != nil || afterSecondRaise.WeightEpoch != nil {
		t.Fatalf("replacement obligation supersession did not survive the next raise: first=%+v replacement=%+v second=%+v", firstRaise, afterReplacement, afterSecondRaise)
	}
}

func TestClockRegressedNamesEpisodeOrigin(t *testing.T) {
	root := budgetProjectionRoot(t)
	projection := ProjectBudget(root, raisedEpisodeGoal(6, 0), time.Date(2026, 8, 28, 7, 59, 0, 0, time.UTC))
	if projection.Status != BudgetUnknown || projection.Unknown == nil || projection.Unknown.Reason != "CLOCK_REGRESSED: the claim episode origin is later than the observation" {
		t.Fatalf("clock regression did not name the episode origin: %+v", projection)
	}
}

func TestBudgetProjectionUsesJobRecordsForTheBoundRevision(t *testing.T) {
	root := budgetProjectionRoot(t)
	writeBudgetJob(t, root, "done", "reserve-a", 3, 30, "completed", budgetJobLife{
		startedAt: "2026-08-28T08:10:00Z", endedAt: "2026-08-28T08:19:30Z", pid: 4242,
	})
	writeBudgetJob(t, root, "live", "reserve-b", 3, 45, "running", budgetJobLife{})
	writeBudgetJob(t, root, "old-revision", "reserve-old", 2, 60, "completed", budgetJobLife{})
	writeJSON(t, filepath.Join(root, "artifacts", "agents", "jobs", "unrelated.json"), map[string]any{
		"jobId": "unrelated", "goalId": nil, "status": "completed",
	})

	projection := ProjectBudget(root, budgetGoal(), time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC))
	if projection.Status != BudgetKnown || projection.GoalRevision != 3 || projection.Attempts != 2 ||
		projection.ReservedJobMinutes != 55 || projection.ObservedJobMinutes != 10 || projection.OpenCapMinutes != 45 ||
		projection.ActiveJobs != 1 || projection.Elapsed != 2*time.Hour ||
		projection.ElapsedGracePercent != 50 || projection.ElapsedBreachLimit != 6*time.Hour ||
		projection.ElapsedState != "" || len(projection.Breaches) != 0 {
		t.Fatalf("projection did not use the sole reservation facts: %+v", projection)
	}
}

func TestProjectBudgetWithoutRunStillRejectsDuplicateDurableOwners(t *testing.T) {
	root := budgetProjectionRoot(t)
	file := budgetGoal()
	attempt := obligationstate.TerminalAttempt{
		RunID: "duplicate-run", Status: run.StatusRed,
		StartedAt: "2026-08-28T08:10:00Z", EndedAt: "2026-08-28T08:11:00Z",
		AttemptOrdinal: 1, ExecutionCostMinutes: 1, ObservedCostMinutes: 1,
		Breaker: run.BreakerClosed,
	}
	if err := obligationstate.RecordTerminal(root, "bounded", 3, 1, attempt); err != nil {
		t.Fatal(err)
	}
	if err := obligationstate.RecordTerminal(root, "bounded", 3, 2, attempt); err != nil {
		t.Fatal(err)
	}

	projection := ProjectBudgetWithoutRun(root, file, time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC), "duplicate-run")
	wantRecord := "artifacts/agents/governed-obligations/bounded.g3.o2.json"
	if projection.Status != BudgetUnknown || projection.Unknown == nil || projection.Unknown.Record != wantRecord ||
		!strings.Contains(projection.Unknown.Reason, `runId "duplicate-run" duplicates terminal state in artifacts/agents/governed-obligations/bounded.g3.o1.json`) {
		t.Fatalf("excluded run bypassed durable owner uniqueness: %+v", projection)
	}
}

func TestCompletedJobChargesObservedMinutesNotItsCap(t *testing.T) {
	root := budgetProjectionRoot(t)
	writeBudgetJob(t, root, "completed", "reserve-completed", 3, 120, "completed", budgetJobLife{
		startedAt: "2026-08-28T08:10:00Z", endedAt: "2026-08-28T08:20:00Z", pid: 4242,
	})

	projection := ProjectBudget(root, budgetGoal(), time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC))
	if projection.Status != BudgetKnown || projection.ReservedJobMinutes != 10 || projection.ObservedJobMinutes != 10 ||
		projection.OpenCapMinutes != 0 || projection.ActiveJobs != 0 || projection.Attempts != 1 {
		t.Fatalf("completed job did not settle to its observed runtime: %+v", projection)
	}
}

func TestRunningJobChargesItsCap(t *testing.T) {
	for _, status := range []string{"running", "pending", "pending-setup"} {
		t.Run(status, func(t *testing.T) {
			root := budgetProjectionRoot(t)
			writeBudgetJob(t, root, status, "reserve-"+status, 3, 45, status, budgetJobLife{})
			projection := ProjectBudget(root, budgetGoal(), time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC))
			if projection.Status != BudgetKnown || projection.ReservedJobMinutes != 45 || projection.ObservedJobMinutes != 0 ||
				projection.OpenCapMinutes != 45 || projection.ActiveJobs != 1 || projection.Attempts != 1 {
				t.Fatalf("open %s job did not retain its cap reservation: %+v", status, projection)
			}
		})
	}
}

func TestFailedJobThatEndedSecondsAfterStartChargesOneMinute(t *testing.T) {
	root := budgetProjectionRoot(t)
	writeBudgetJob(t, root, "failed", "reserve-failed", 3, 120, "failed", budgetJobLife{
		startedAt: "2026-08-28T08:10:00Z", endedAt: "2026-08-28T08:10:12Z", pid: 4242,
	})
	projection := ProjectBudget(root, budgetGoal(), time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC))
	if projection.Status != BudgetKnown || projection.ReservedJobMinutes != 1 || projection.ObservedJobMinutes != 1 {
		t.Fatalf("seconds-long failed job did not charge the one-minute floor: %+v", projection)
	}
}

func TestTimeoutAtTheCapChargesTheCapNeverMore(t *testing.T) {
	root := budgetProjectionRoot(t)
	writeBudgetJob(t, root, "timeout", "reserve-timeout", 3, 120, "timeout", budgetJobLife{
		startedAt: "2026-08-28T08:00:00Z", endedAt: "2026-08-28T10:01:30Z", pid: 4242,
	})
	projection := ProjectBudget(root, budgetGoal(), time.Date(2026, 8, 28, 10, 2, 0, 0, time.UTC))
	if projection.Status != BudgetKnown || projection.ReservedJobMinutes != 120 || projection.ObservedJobMinutes != 120 {
		t.Fatalf("timeout charge exceeded or missed its cap: %+v", projection)
	}
}

func TestSettledMinutesRoundUpLikeTheGovernedPath(t *testing.T) {
	start := time.Date(2026, 8, 28, 8, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		seconds int
		want    uint64
	}{
		{seconds: 0, want: 1}, {seconds: 1, want: 1}, {seconds: 60, want: 1},
		{seconds: 61, want: 2}, {seconds: 544, want: 10}, {seconds: 711, want: 12},
		{seconds: 7260, want: 120},
	} {
		if got := settledJobMinutes(start, start.Add(time.Duration(test.seconds)*time.Second), 120); got != test.want {
			t.Errorf("settledJobMinutes for %d seconds = %d, want %d", test.seconds, got, test.want)
		}
	}
}

func TestNeverLaunchedTerminalRecordChargesZero(t *testing.T) {
	t.Run("pending setup husk", func(t *testing.T) {
		root := budgetProjectionRoot(t)
		writeBudgetJob(t, root, "husk", "reserve-husk", 3, 120, "cancelled", budgetJobLife{
			createdAt: "2026-08-28T08:05:00Z", endedAt: "2026-08-28T08:12:00Z",
		})
		projection := ProjectBudget(root, budgetGoal(), time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC))
		if projection.Status != BudgetKnown || projection.ReservedJobMinutes != 0 || projection.ObservedJobMinutes != 0 || projection.Attempts != 1 {
			t.Fatalf("never-launched setup husk consumed minutes: %+v", projection)
		}
	})
	t.Run("pending launch failure", func(t *testing.T) {
		root := budgetProjectionRoot(t)
		writeBudgetJob(t, root, "failed", "reserve-failed", 3, 120, "failed", budgetJobLife{
			startedAt: "2026-08-28T08:10:00Z", endedAt: "2026-08-28T08:20:00Z",
		})
		projection := ProjectBudget(root, budgetGoal(), time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC))
		if projection.Status != BudgetKnown || projection.ReservedJobMinutes != 0 || projection.ObservedJobMinutes != 0 || projection.Attempts != 1 {
			t.Fatalf("never-launched pending record consumed minutes: %+v", projection)
		}
	})
	t.Run("post discharge uses createdAt", func(t *testing.T) {
		root := budgetProjectionRoot(t)
		file := budgetGoal()
		file.Obligation = &goal.GovernedObligation{Revision: 6}
		zero := uint64(0)
		if err := obligationstate.RecordTerminal(root, "bounded", 3, 6, obligationstate.TerminalAttempt{
			RunID: "green-proof", Status: run.StatusGreen,
			StartedAt: "2026-08-28T09:00:00Z", EndedAt: "2026-08-28T09:25:00Z", PrunedAt: "2026-08-28T09:31:00Z",
			AttemptOrdinal: 1, ExecutionCostMinutes: 30, ObservedCostMinutes: 25,
			WeightGeneration: 1, BudgetEpoch: &zero, Breaker: run.BreakerClosed,
		}); err != nil {
			t.Fatal(err)
		}
		writeJSON(t, filepath.Join(root, "artifacts", "agents", "validation-weight.json"), map[string]any{
			"schema": 1, "generation": 2,
			"consumedProofs": []any{map[string]any{
				"runId": "green-proof", "goalId": "bounded", "goalRevision": 3, "obligationRevision": 6,
				"weightGeneration": 1, "consumedAt": "2026-08-28T09:30:00Z",
				"resetDecision":     map[string]any{"apply": true, "wouldRefuse": false},
				"dischargeDecision": map[string]any{"apply": true, "wouldRefuse": false},
			}},
		})
		writeBudgetJob(t, root, "after", "reserve-after", 3, 120, "cancelled", budgetJobLife{
			createdAt: "2026-08-28T09:45:00Z", endedAt: "2026-08-28T09:46:00Z",
		})
		writeBudgetJob(t, root, "before", "reserve-before", 3, 120, "cancelled", budgetJobLife{
			createdAt: "2026-08-28T09:00:00Z", endedAt: "2026-08-28T09:01:00Z",
		})
		projection := ProjectBudget(root, file, time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC))
		if projection.Status != BudgetKnown || projection.Attempts != 1 || projection.ReservedJobMinutes != 0 {
			t.Fatalf("post-discharge husks were not filtered by createdAt: %+v", projection)
		}
		writeBudgetJob(t, root, "stampless", "reserve-stampless", 3, 120, "cancelled", budgetJobLife{endedAt: "2026-08-28T09:46:00Z"})
		projection = ProjectBudget(root, file, time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC))
		if projection.Status != BudgetUnknown || projection.Unknown == nil || projection.Unknown.Record != "artifacts/agents/jobs/stampless.json" ||
			!strings.Contains(projection.Unknown.Reason, "startedAt or createdAt") {
			t.Fatalf("stampless post-discharge husk did not fail closed: %+v", projection)
		}
	})
}

func TestLaunchedTerminalRecordWithoutReadableTimestampsIsUnknown(t *testing.T) {
	for _, test := range []struct {
		name string
		life budgetJobLife
		want string
	}{
		{name: "missing start", life: budgetJobLife{endedAt: "2026-08-28T08:20:00Z", pid: 4242}, want: "startedAt"},
		{name: "missing end", life: budgetJobLife{startedAt: "2026-08-28T08:10:00Z", pid: 4242}, want: "endedAt"},
		{name: "unreadable end", life: budgetJobLife{startedAt: "2026-08-28T08:10:00Z", endedAt: "yesterday", pid: 4242}, want: "endedAt"},
		{name: "unreadable ownership proof", life: budgetJobLife{startedAt: "2026-08-28T08:10:00Z", provenAt: "yesterday", endedAt: "2026-08-28T08:20:00Z", pid: 4242}, want: "ownershipProof.provenAt"},
		{name: "clock regressed", life: budgetJobLife{startedAt: "2026-08-28T08:10:00Z", endedAt: "2026-08-28T08:09:00Z", pid: 4242}, want: "CLOCK_REGRESSED"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := budgetProjectionRoot(t)
			writeBudgetJob(t, root, "bad-time", "reserve-bad-time", 3, 120, "completed", test.life)
			projection := ProjectBudget(root, budgetGoal(), time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC))
			if projection.Status != BudgetUnknown || projection.Unknown == nil ||
				projection.Unknown.Record != "artifacts/agents/jobs/bad-time.json" || !strings.Contains(projection.Unknown.Reason, test.want) {
				t.Fatalf("bad terminal timestamp did not fail closed with %q: %+v", test.want, projection)
			}
		})
	}
}

func TestReservedJobMinutesIsSumOfNamedComponents(t *testing.T) {
	writeDelegated := func(t *testing.T, root string) {
		t.Helper()
		writeBudgetJob(t, root, "completed", "reserve-completed", 3, 120, "completed", budgetJobLife{
			startedAt: "2026-08-28T08:10:00Z", endedAt: "2026-08-28T08:20:00Z", pid: 4242,
		})
		writeBudgetJob(t, root, "running", "reserve-running", 3, 45, "running", budgetJobLife{})
		writeBudgetJob(t, root, "never-launched", "reserve-never-launched", 3, 120, "failed", budgetJobLife{
			startedAt: "2026-08-28T08:30:00Z", endedAt: "2026-08-28T08:31:00Z",
		})
	}
	writeGoverned := func(t *testing.T, root string) {
		t.Helper()
		if err := obligationstate.RecordTerminal(root, "bounded", 3, 6, obligationstate.TerminalAttempt{
			RunID: "settled-run", Status: run.StatusGreen,
			StartedAt: "2026-08-28T08:10:00Z", EndedAt: "2026-08-28T08:35:00Z", PrunedAt: "2026-08-28T08:40:00Z",
			AttemptOrdinal: 1, ExecutionCostMinutes: 30, ObservedCostMinutes: 25,
			WeightGeneration: 1, Breaker: run.BreakerClosed,
		}); err != nil {
			t.Fatal(err)
		}
		now := time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC)
		weightGeneration := uint64(1)
		store := &run.Store{Root: root, Now: func() time.Time { return now }}
		store.AdmitGoverned = func(run.GovernedAdmissionRequest) (run.GovernedAdmissionResult, error) {
			return run.GovernedAdmissionResult{Attempt: run.GovernedAttempt{
				GoalRevision: 3, ObligationRevision: 6, Recurrence: governance.StandingSharedProcess,
				WeightGeneration: &weightGeneration, ExecutionCostMinutes: 30, AttemptOrdinal: 2,
				Budget: *budgetGoal().Budget, BudgetStartedAt: "2026-08-28T08:00:00Z",
				ExpectedAssumptions: governance.ObligationAssumptions{
					Recurrence: governance.StandingSharedProcess, Platform: "fixture/os", ToolchainIdentity: "fixture-go",
					SurfaceDigest: "fixture-digest", MaxActiveJobs: 1, TimingEnvelopeSeconds: 1800, ObservationSource: "run-terminal-record",
				},
				AdmissionDecision: governance.ConsequenceDecision{Apply: true}, Breaker: run.BreakerClosed,
			}}, nil
		}
		if _, err := store.Launch(run.Caller{Class: "HUMAN"}, run.LaunchParams{
			Id: "live-run", Kind: "suite", Display: "live governed run", Log: "artifacts/live-run.log",
			GoalId: "bounded", ObligationRevision: 6,
		}); err != nil {
			t.Fatal(err)
		}
	}
	writeProofs := func(t *testing.T, root string, terminalIdentity proofrun.ProofIdentity) {
		t.Helper()
		liveIdentity, err := proofrun.BuildProofIdentity(root, filepath.Join(root, "metasystem.conf"), "full", "sum-live-proof",
			[]string{"gate"}, behaviorsurface.SupportedVersion)
		if err != nil {
			t.Fatal(err)
		}
		launcher, err := proofrun.CurrentProcessIdentity(nil)
		if err != nil {
			t.Fatal(err)
		}
		terminalStarted := time.Date(2026, 8, 28, 8, 0, 0, 0, time.UTC)
		terminal, _, err := proofrun.ReserveLocked(proofrun.AdmissionRequest{
			ControlRoot: root, ExecutionRoot: root, GoalID: "bounded", GoalRevision: 3, AccountingRevision: 3,
			ReservedMinutes: 20, Identity: terminalIdentity, Launcher: launcher, Now: terminalStarted, AttemptID: "sum-terminal-proof",
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := proofrun.FinalizeAttempt(root, terminal.AttemptID, proofrun.TerminalFailed, 1, "controlled proof failure", nil,
			terminalStarted.Add(5*time.Minute)); err != nil {
			t.Fatal(err)
		}
		if _, _, err := proofrun.ReserveLocked(proofrun.AdmissionRequest{
			ControlRoot: root, ExecutionRoot: root, GoalID: "bounded", GoalRevision: 3, AccountingRevision: 3,
			ReservedMinutes: 15, Identity: liveIdentity, Launcher: launcher,
			Now: time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC), AttemptID: "sum-live-proof",
		}); err != nil {
			t.Fatal(err)
		}
	}

	for _, test := range []struct {
		name                        string
		delegated, governed, proofs bool
		observed, open, proof       uint64
		reserved, attempts          uint64
	}{
		{name: "delegated", delegated: true, observed: 10, open: 45, reserved: 55, attempts: 3},
		{name: "governed", governed: true, observed: 25, open: 30, reserved: 55, attempts: 2},
		{name: "mixed", delegated: true, governed: true, observed: 35, open: 75, reserved: 110, attempts: 5},
		// The terminal proof reserved 20 minutes and ran 5: it is charged
		// what it used; the live one is charged its 15-minute reservation.
		{name: "proof reservations", proofs: true, observed: 10, proof: 20, reserved: 30, attempts: 3},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := ""
			var terminalIdentity proofrun.ProofIdentity
			if test.proofs {
				root, terminalIdentity = dispatchProofFixture(t, "sum-terminal-proof")
			} else {
				root = budgetProjectionRoot(t)
			}
			if test.delegated {
				writeDelegated(t, root)
			}
			if test.governed {
				writeGoverned(t, root)
			}
			if test.proofs {
				writeBudgetJob(t, root, "completed", "reserve-completed", 3, 120, "completed", budgetJobLife{
					startedAt: "2026-08-28T08:10:00Z", endedAt: "2026-08-28T08:20:00Z", pid: 4242,
				})
				writeProofs(t, root, terminalIdentity)
			}
			projection := ProjectBudget(root, budgetGoal(), time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC))
			if projection.Status != BudgetKnown || projection.ObservedJobMinutes != test.observed || projection.OpenCapMinutes != test.open ||
				projection.ProofReservationMinutes != test.proof ||
				projection.ReservedJobMinutes != test.reserved || projection.Attempts != test.attempts ||
				projection.ReservedJobMinutes != projection.ObservedJobMinutes+projection.OpenCapMinutes+projection.ProofReservationMinutes {
				t.Fatalf("reserved-minute components do not add up: %+v", projection)
			}
			if test.name == "governed" && projection.ActiveJobs != 1 {
				t.Fatalf("live governed run counted as %d active jobs, want 1: %+v", projection.ActiveJobs, projection)
			}
			if test.proofs {
				lines := FormatGoalAdmission(GoalAdmissionVerdict{Refusals: []GoalAdmissionRefusal{{
					GoalID: "bounded", GoalRevision: 3, Breaches: budgetAdmissionBreaches(projection),
					Reserved: reservedMinutesEvidence(projection),
				}}})
				want := "BUDGET_REFUSED: goal bounded revision=3 admission closed: attemptLimit used=3 limit=2, activeJobLimit used=1 limit=1; reserved observed=10 open-caps=0 proof=20 limit=75; rule=setup-refusal-release: terminal records with phase=setup and refusalClass=setup count as neither attempts nor reserved job minutes"
				if len(lines) != 1 || lines[0] != want {
					t.Fatalf("proof reservation refusal line = %v, want %q", lines, want)
				}
			}
		})
	}
}

func TestTwoBarsForChangesSpecimenSettlesToObservedMinutes(t *testing.T) {
	records := []struct {
		name                   string
		revision               uint64
		started, proven, ended string
	}{
		{name: "r26-a", revision: 26, started: "2026-09-02T11:48:39Z", proven: "2026-09-02T11:48:40Z", ended: "2026-09-02T11:48:51Z"},
		{name: "r26-b", revision: 26, started: "2026-09-02T11:34:35Z", proven: "2026-09-02T11:34:36Z", ended: "2026-09-02T11:46:27Z"},
		{name: "r26-c", revision: 26, started: "2026-09-02T16:01:59Z", proven: "2026-09-02T16:02:00Z", ended: "2026-09-02T16:15:41Z"},
		{name: "r26-d", revision: 26, started: "2026-09-02T11:52:22Z", proven: "2026-09-02T11:52:23Z", ended: "2026-09-02T11:52:34Z"},
		{name: "r26-e", revision: 26, started: "2026-09-02T15:49:32Z", proven: "2026-09-02T15:49:33Z", ended: "2026-09-02T15:58:37Z"},
		{name: "r26-f", revision: 26, started: "2026-09-02T16:18:10Z", proven: "2026-09-02T16:18:11Z", ended: "2026-09-02T16:29:46Z"},
		{name: "r28-a", revision: 28, started: "2026-09-02T16:35:27Z", proven: "2026-09-02T16:35:28Z", ended: "2026-09-02T16:46:05Z"},
		{name: "r28-b", revision: 28, started: "2026-09-02T16:48:04Z", proven: "2026-09-02T16:48:05Z", ended: "2026-09-02T16:56:49Z"},
	}
	for _, test := range []struct {
		revision uint64
		want     uint64
	}{{revision: 26, want: 50}, {revision: 28, want: 20}} {
		root := budgetProjectionRoot(t)
		for _, record := range records {
			if record.revision != test.revision {
				continue
			}
			writeBudgetJob(t, root, record.name, "reserve-"+record.name, record.revision, 120, "completed", budgetJobLife{
				startedAt: record.started, endedAt: record.ended, pid: 4242, provenAt: record.proven,
			})
		}
		file := budgetGoal()
		file.Revision = 28
		file.History = make([]goal.HistoryLine, 28)
		for index := range file.History {
			file.History[index].At = file.Claimed.At
		}
		file.Budget.AttemptLimit = 20
		file.Budget.ReservedJobMinutesLimit = 2000
		file.Budget.ActiveJobLimit = 20
		file.Claimed.Revision = test.revision
		file.Claimed.AccountingRevision = test.revision
		projection := ProjectBudget(root, file, time.Date(2026, 9, 2, 18, 0, 0, 0, time.UTC))
		if projection.Status != BudgetKnown || projection.ReservedJobMinutes != test.want || projection.ObservedJobMinutes != test.want || projection.OpenCapMinutes != 0 {
			t.Fatalf("specimen revision %d settled to %+v, want %d observed minutes", test.revision, projection, test.want)
		}
	}
}

func TestObservedMinutesRunFromTheOwnershipStamp(t *testing.T) {
	for _, test := range []struct {
		name       string
		life       budgetJobLife
		pidStarted bool
		want       uint64
	}{
		{name: "ownership proof", life: budgetJobLife{startedAt: "2026-08-28T08:10:00Z", provenAt: "2026-08-28T08:12:00Z", endedAt: "2026-08-28T08:22:00Z", pid: 4242}, want: 10},
		{name: "absent proof falls back", life: budgetJobLife{startedAt: "2026-08-28T08:10:00Z", endedAt: "2026-08-28T08:20:00Z", pid: 4242}, want: 10},
		{name: "empty proof falls back", life: budgetJobLife{startedAt: "2026-08-28T08:10:00Z", endedAt: "2026-08-28T08:20:00Z", pid: 4242}, want: 10},
		{name: "pid start is ignored", life: budgetJobLife{startedAt: "2026-08-28T08:10:00Z", provenAt: "2026-08-28T08:12:00Z", endedAt: "2026-08-28T08:22:00Z", pid: 4242}, pidStarted: true, want: 10},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := budgetProjectionRoot(t)
			writeBudgetJob(t, root, "ownership", "reserve-ownership", 3, 120, "completed", test.life)
			path := filepath.Join(root, "artifacts", "agents", "jobs", "ownership.json")
			if test.name == "empty proof falls back" || test.pidStarted {
				record, err := readObject(path)
				if err != nil {
					t.Fatal(err)
				}
				if test.name == "empty proof falls back" {
					record["ownershipProof"] = map[string]any{"provenAt": "", "source": "trusted-launcher"}
				}
				if test.pidStarted {
					record["pidStartedAt"] = time.Date(2026, 8, 28, 7, 0, 0, 0, time.UTC).Unix()
				}
				writeJSON(t, path, record)
			}
			projection := ProjectBudget(root, budgetGoal(), time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC))
			if projection.Status != BudgetKnown || projection.ReservedJobMinutes != test.want {
				t.Fatalf("ownership-stamp settlement = %+v, want %d minutes", projection, test.want)
			}
		})
	}
}

func TestSTR2P2A01AccountingRevisionPreservesRaisedSpendAndSetBudgetResetsIt(t *testing.T) {
	root := budgetProjectionRoot(t)
	file := budgetGoal()
	file.Claimed.Revision = 5
	file.Claimed.AccountingRevision = 3
	file.Claimed.EpisodeAt = file.History[2].At
	file.Claimed.EpisodeRevision = 3
	file.Budget.ReviewRoundLimit = 3
	file.History[4].Reason = "Misclassified: from=1 to=3 evidence=refusal:BUDGET_REFUSED"
	writeBudgetJob(t, root, "root-before-raise-one", "reserve-before-one", 3, 20, "completed", budgetJobLife{
		startedAt: "2026-08-28T08:10:00Z", endedAt: "2026-08-28T08:30:00Z", pid: 4242,
	})
	writeBudgetJob(t, root, "root-before-raise-two", "reserve-before-two", 4, 30, "running", budgetJobLife{})
	for _, job := range []string{"root-before-raise-one", "root-before-raise-two"} {
		path := filepath.Join(root, "artifacts", "agents", "jobs", job+".json")
		record, err := readObject(path)
		if err != nil {
			t.Fatal(err)
		}
		record["role"] = "code-critic"
		record["parentJob"] = nil
		record[reviewChainCountedField] = true
		writeJSON(t, path, record)
	}

	projection := ProjectBudget(root, file, time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC))
	if projection.Status != BudgetKnown || projection.Attempts != 2 || projection.ReservedJobMinutes != 50 || projection.ActiveJobs != 1 || projection.CodeCritiques != 2 {
		t.Fatalf("risk raise erased spend from the accounting interval: %+v", projection)
	}
	episodeOrigin, err := time.Parse(time.RFC3339, file.Claimed.EpisodeAt)
	if err != nil {
		t.Fatal(err)
	}
	if !projection.StartedAt.Equal(episodeOrigin) {
		t.Fatalf("risk raise changed the elapsed origin while retaining spend: %+v", projection)
	}
	file.Claimed.AccountingRevision = file.Claimed.Revision
	reset := ProjectBudget(root, file, time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC))
	if reset.Status != BudgetKnown || reset.Attempts != 0 || reset.ReservedJobMinutes != 0 || reset.ActiveJobs != 0 || reset.CodeCritiques != 0 {
		t.Fatalf("human set-budget boundary did not reset the tally: %+v", reset)
	}
	if !reset.StartedAt.Equal(projection.StartedAt) {
		t.Fatalf("accounting reset also reset the elapsed origin: before=%s after=%s", projection.StartedAt, reset.StartedAt)
	}
}

func TestUnconsumedDischargeJSONCannotResetTheBudgetProjection(t *testing.T) {
	root := budgetProjectionRoot(t)
	file := budgetGoal()
	file.Obligation = &goal.GovernedObligation{Revision: 6}
	writeJSON(t, filepath.Join(root, "artifacts", "agents", "validation-weight.json"), map[string]any{
		"schema": 1, "generation": 2, "accumulated": 0, "landings": 0, "sinceUtc": "2026-08-28T09:30:00Z", "lastCommit": "landed",
		"lastDecision": map[string]any{"runId": "green-proof", "goalId": "bounded", "obligationRevision": 6,
			"decidedAt": "2026-08-28T09:30:00Z", "applied": true,
			"resetDecision":     map[string]any{"apply": true, "wouldRefuse": false, "reason": "authorized"},
			"dischargeDecision": map[string]any{"apply": true, "wouldRefuse": false, "reason": "authorized"}},
	})
	for _, job := range []struct {
		id, operation, started, status string
		cap                            int
	}{
		{id: "before", operation: "before", started: "2026-08-28T09:00:00Z", status: "completed", cap: 30},
		{id: "after", operation: "after", started: "2026-08-28T09:45:00Z", status: "running", cap: 20},
	} {
		writeJSON(t, filepath.Join(root, "artifacts", "agents", "jobs", job.id+".json"), map[string]any{
			"jobId": job.id, "operationId": job.operation, "goalId": "bounded", "goalRevision": 3,
			"capMin": job.cap, "status": job.status, "startedAt": job.started,
		})
	}
	projection := ProjectBudget(root, file, time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC))
	if projection.Status != BudgetUnknown || projection.Unknown == nil ||
		!strings.Contains(projection.Unknown.Reason, "consumed") {
		t.Fatalf("forged discharge JSON reset the projection without consuming a proof: %+v", projection)
	}
}

func TestPublishedSetupRetainsAttemptAndReservedMinutes(t *testing.T) {
	root := budgetProjectionRoot(t)
	stage := t.TempDir()
	capFile := writeJSON(t, filepath.Join(stage, "cap.json"), map[string]any{
		"capMin": 30, "capDeadline": "2026-08-28T10:00:00Z",
		"source": map[string]any{"rule": "fixture", "origin": "fixture", "truncatedBy": nil},
	})
	setup := filepath.Join(stage, "setup.json")
	if err := BuildSetup(root, setup, "reserved", "implementer", "", "main-1", "5", "bounded", 3, 3, capFile, "", ""); err != nil {
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

func TestSetupRefusalsReleaseAttemptAndMinuteReservations(t *testing.T) {
	root := budgetProjectionRoot(t)
	file := budgetGoal()
	file.Budget.AttemptLimit = 3
	file.Budget.ReservedJobMinutesLimit = 120
	for _, name := range []string{"setup-refused-one", "setup-refused-two"} {
		writeJSON(t, filepath.Join(root, "artifacts", "agents", "jobs", name+".json"), map[string]any{
			"jobId": name, "operationId": name, "goalId": "bounded", "goalRevision": 3,
			"capMin": 30, "status": "failed", "phase": "setup", "refusalClass": "setup",
		})
	}
	writeBudgetJob(t, root, "completed", "completed", 3, 30, "completed", budgetJobLife{
		startedAt: "2026-08-28T08:00:00Z", endedAt: "2026-08-28T08:30:00Z", pid: 4242,
	})

	projection := ProjectBudget(root, file, time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC))
	if projection.Status != BudgetKnown || projection.Attempts != 1 || projection.ReservedJobMinutes != 30 {
		t.Fatalf("two setup refusals and one completion did not leave two of three attempts free: %+v", projection)
	}

	writeJSON(t, filepath.Join(root, "artifacts", "agents", "jobs", "protocol-error.json"), map[string]any{
		"jobId": "protocol-error", "operationId": "protocol-error", "goalId": "bounded", "goalRevision": 3,
		"capMin": 30, "status": "failed", "phase": "validation",
		"pid": 4242, "startedAt": "2026-08-28T08:30:00Z", "endedAt": "2026-08-28T09:00:00Z",
		"protocolError": map[string]any{"key": "invalid-return", "violation": "malformed implementer return"},
	})
	projection = ProjectBudget(root, file, time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC))
	if projection.Status != BudgetKnown || projection.Attempts != 2 || projection.ReservedJobMinutes != 60 {
		t.Fatalf("a protocol error after the agent started did not consume its reservation: %+v", projection)
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
				writeBudgetJob(t, root, "first", "reserve-same", 3, 10, "completed", budgetJobLife{})
				writeBudgetJob(t, root, "second", "reserve-same", 3, 10, "running", budgetJobLife{})
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
			root := budgetProjectionRoot(t)
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
	root := budgetProjectionRoot(t)
	writeBudgetJob(t, root, "one", "reserve-one", 3, 50, "running", budgetJobLife{})
	writeBudgetJob(t, root, "two", "reserve-two", 3, 50, "pending", budgetJobLife{})
	writeBudgetJob(t, root, "three", "reserve-three", 3, 10, "completed", budgetJobLife{})

	projection := ProjectBudget(root, budgetGoal(), time.Date(2026, 8, 28, 14, 0, 0, 0, time.UTC))
	if projection.Status != BudgetKnown || projection.ElapsedState != ElapsedBreach || len(projection.Breaches) != 4 {
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

func TestBudgetAdmissionRefusalNamesSetupRefusalReleaseRule(t *testing.T) {
	lines := FormatGoalAdmission(GoalAdmissionVerdict{Refusals: []GoalAdmissionRefusal{{
		GoalID: "bounded", GoalRevision: 3,
		Breaches: []BudgetBreach{budgetIntegerBreach("attemptLimit", 3, 3)},
	}}})
	if len(lines) != 1 || !strings.Contains(lines[0], "rule=setup-refusal-release") ||
		!strings.Contains(lines[0], "count as neither attempts nor reserved job minutes") {
		t.Fatalf("attempt refusal did not explain the setup-refusal release rule: %v", lines)
	}
}

func TestIdleSecondsComeOffTheElapsedClockAfterTheDischargeStart(t *testing.T) {
	root := budgetProjectionRoot(t)
	// The episode began at 08:00, a consumed discharge at 08:30 advanced the
	// start, the pair was away for half an hour and re-claimed at 09:00: at
	// 10:30 the clock reads two hours less the idle half hour.
	dischargeAt := time.Date(2026, 8, 28, 8, 30, 0, 0, time.UTC)
	writeConsumedBudgetProof(t, root, "green-discharge", 3, 5, dischargeAt)
	file := reclaimedBudgetGoal()
	file.Claimed.IdleSeconds = 1800
	file.Obligation = &goal.GovernedObligation{Revision: 5}
	projection := ProjectBudget(root, file, time.Date(2026, 8, 28, 10, 30, 0, 0, time.UTC))
	if projection.Status != BudgetKnown || !projection.StartedAt.Equal(dischargeAt) || projection.Elapsed != 90*time.Minute {
		t.Fatalf("idle seconds did not compose with the discharge-advanced start: %+v", projection)
	}
	// Without a discharge the idle seconds come off the episode origin.
	plain := raisedEpisodeGoal(6, 0)
	plain.Claimed.IdleSeconds = 3600
	projection = ProjectBudget(budgetProjectionRoot(t), plain, time.Date(2026, 8, 28, 13, 0, 0, 0, time.UTC))
	if projection.Status != BudgetKnown || projection.Elapsed != 4*time.Hour || projection.ElapsedState != AdmissionClosedElapsed {
		t.Fatalf("idle seconds did not come off the episode clock: %+v", projection)
	}
	// The clock never reads below zero.
	plain.Claimed.IdleSeconds = 24 * 3600
	projection = ProjectBudget(budgetProjectionRoot(t), plain, time.Date(2026, 8, 28, 13, 0, 0, 0, time.UTC))
	if projection.Status != BudgetKnown || projection.Elapsed != 0 {
		t.Fatalf("idle seconds drove the clock negative: %+v", projection)
	}
}

// reclaimedBudgetGoal is the bounded goal after an own-pair release at 08:20
// and the same pair's re-claim at 09:00 (revision 5): the box still counts
// from revision 3 and the forty idle minutes come off the clock.
func reclaimedBudgetGoal() *goal.GoalFile {
	file := budgetGoal()
	file.Claimed.At = file.History[4].At
	file.Claimed.Revision = 5
	file.Claimed.AccountingRevision = 3
	file.Claimed.EpisodeAt = file.History[2].At
	file.Claimed.EpisodeRevision = 3
	file.Claimed.IdleSeconds = 40 * 60
	return file
}

func TestReclaimedGoalCountsThePreReleaseAttemptAndKeepsTheDischargeStart(t *testing.T) {
	root := budgetProjectionRoot(t)
	// One attempt ran before the release, under the accounting revision.
	writeBudgetJob(t, root, "before-release", "reserve-before", 3, 30, "completed", budgetJobLife{
		startedAt: "2026-08-28T08:05:00Z", endedAt: "2026-08-28T08:15:00Z", pid: 4242,
	})
	plain := ProjectBudget(root, reclaimedBudgetGoal(), time.Date(2026, 8, 28, 11, 0, 0, 0, time.UTC))
	if plain.Status != BudgetKnown || plain.Attempts != 1 || plain.StartedAt.Format(time.RFC3339) != "2026-08-28T08:00:00Z" || plain.Elapsed != 2*time.Hour+20*time.Minute {
		t.Fatalf("the re-claimed goal lost its earlier attempt or its idle gap: %+v", plain)
	}
	// A discharge consumed in the earlier hold advances the start (and, as
	// today, resets the attempts before it); the gap, which follows it,
	// still comes off.
	dischargeAt := time.Date(2026, 8, 28, 8, 10, 0, 0, time.UTC)
	writeConsumedBudgetProof(t, root, "early-discharge", 3, 5, dischargeAt)
	discharged := reclaimedBudgetGoal()
	discharged.Obligation = &goal.GovernedObligation{Revision: 5}
	projection := ProjectBudget(root, discharged, time.Date(2026, 8, 28, 11, 0, 0, 0, time.UTC))
	if projection.Status != BudgetKnown || !projection.StartedAt.Equal(dischargeAt) || projection.Elapsed != 2*time.Hour+10*time.Minute {
		t.Fatalf("the discharge-advanced start and the idle gap did not compose: %+v", projection)
	}
}

func TestIdleSecondsDoNotApplyToAStartInsideTheCurrentHold(t *testing.T) {
	root := budgetProjectionRoot(t)
	// The discharge is consumed after the re-claim at 09:00: the window it
	// starts holds no gap, so nothing comes off.
	dischargeAt := time.Date(2026, 8, 28, 9, 30, 0, 0, time.UTC)
	writeConsumedBudgetProof(t, root, "late-discharge", 3, 5, dischargeAt)
	file := reclaimedBudgetGoal()
	file.Obligation = &goal.GovernedObligation{Revision: 5}
	projection := ProjectBudget(root, file, time.Date(2026, 8, 28, 11, 0, 0, 0, time.UTC))
	if projection.Status != BudgetKnown || !projection.StartedAt.Equal(dischargeAt) || projection.Elapsed != 90*time.Minute {
		t.Fatalf("idle seconds were subtracted from a window that held no gap: %+v", projection)
	}
}
