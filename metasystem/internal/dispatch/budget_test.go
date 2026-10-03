package dispatch

import (
	"encoding/json"
	"math"
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
		Approved: &goal.ApprovalRecord{Revision: 3},
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

func TestCandidateEpisodeSurvivesClaim(t *testing.T) {
	root := budgetProjectionRoot(t)
	writeBudgetJob(t, root, "episode-one", "episode-one", 3, 1, "completed", budgetJobLife{
		startedAt: "2026-08-28T08:10:00Z", endedAt: "2026-08-28T08:11:00Z", pid: 41,
	})
	writeBudgetJob(t, root, "episode-two", "episode-two", 4, 1, "completed", budgetJobLife{
		startedAt: "2026-08-28T08:12:00Z", endedAt: "2026-08-28T08:13:00Z", pid: 42,
	})
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	file := budgetGoal()
	file.State = goal.StateApproved
	file.Claimed = nil
	file.Approved = &goal.ApprovalRecord{Revision: 3, EpisodeRevision: 3}
	file.Budget.AttemptLimit = 10
	file.Budget.ReservedJobMinutesLimit = 100

	assertSpent := func(label string, projection ConsumptionProjection) {
		t.Helper()
		if projection.Status != BudgetKnown || projection.Attempts != 2 || projection.ReservedJobMinutes != 2 || projection.ObservedJobMinutes != 2 {
			t.Fatalf("%s consumption = %+v, want two attempts and two minutes", label, projection)
		}
	}
	assertSpent("approved", ProjectConsumption(root, file, now))

	file.State = goal.StateClaimed
	file.Claimed = &goal.ClaimRecord{
		Machine: "bed-m1", Lineage: "coordinator", At: file.History[4].At, Revision: 5,
		AccountingRevision: 5, EpisodeAt: file.History[2].At, EpisodeRevision: 3,
	}
	claimed := ProjectBudget(root, file, now)
	if claimed.Status != BudgetKnown || claimed.Attempts != 2 || claimed.ReservedJobMinutes != 2 || claimed.ObservedJobMinutes != 2 {
		t.Fatalf("claimed consumption = %+v, want the approved episode's two attempts", claimed)
	}

	file.State = goal.StateApproved
	file.Claimed = nil
	assertSpent("released", ProjectConsumption(root, file, now))

	file.History = append(file.History, goal.HistoryLine{At: "2026-08-28T10:00:00Z"})
	file.Revision = 6
	file.State = goal.StateClaimed
	file.Claimed = &goal.ClaimRecord{
		Machine: "bed-m1", Lineage: "coordinator", At: file.History[5].At, Revision: 6,
		AccountingRevision: 6, EpisodeAt: file.History[2].At, EpisodeRevision: 3,
	}
	assertSpent("reclaimed", ProjectConsumption(root, file, now))

	file.History = append(file.History, goal.HistoryLine{At: "2026-08-28T10:30:00Z"})
	file.Revision = 7
	file.Claimed.Revision = 7
	file.Claimed.AccountingRevision = 7
	file.Claimed.At = file.History[6].At
	file.Approved.Revision = 7
	assertSpent("risk-raised", ProjectConsumption(root, file, now))

	file.History = append(file.History, goal.HistoryLine{At: "2026-08-28T11:00:00Z"})
	file.Revision = 8
	file.Claimed.Revision = 8
	file.Claimed.AccountingRevision = 8
	file.Claimed.At = file.History[7].At
	file.Approved.Revision = 8
	file.Approved.EpisodeRevision = 8
	reset := ProjectConsumption(root, file, now)
	if reset.Status != BudgetKnown || reset.Attempts != 0 || reset.ReservedJobMinutes != 0 {
		t.Fatalf("set-budget did not start a fresh consumption episode: %+v", reset)
	}
}

func TestConsumptionLensSameForClaimedAndUnclaimed(t *testing.T) {
	root, identity := dispatchProofFixture(t, "consumption-lens")
	now := time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC)
	writeBudgetJob(t, root, "lens-job", "lens-job", 3, 4, "completed", budgetJobLife{
		startedAt: "2026-08-28T08:10:00Z", endedAt: "2026-08-28T08:12:00Z", pid: 51,
	})
	launcher, err := proofrun.CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	_, decision, err := proofrun.ReserveLocked(candidateProofAdmission(proofrun.AdmissionRequest{
		ControlRoot: root, ExecutionRoot: root, GoalID: "bounded", GoalRevision: 3, AccountingRevision: 3,
		ReservedMinutes: 5, Identity: identity, Launcher: launcher, Now: now, AttemptID: "lens-proof",
	}))
	if err != nil {
		t.Fatal(err)
	}
	requireProofReservationNotAdmissionRefused(t, decision)
	store := &run.Store{Root: root, Now: func() time.Time { return now }}
	weightGeneration := uint64(1)
	store.AdmitGoverned = func(run.GovernedAdmissionRequest) (run.GovernedAdmissionResult, error) {
		return run.GovernedAdmissionResult{Attempt: run.GovernedAttempt{
			GoalRevision: 3, ObligationRevision: 6, Recurrence: governance.StandingSharedProcess,
			WeightGeneration: &weightGeneration, ExecutionCostMinutes: 7, AttemptOrdinal: 1, Budget: *budgetGoal().Budget,
			BudgetStartedAt: "2026-08-28T08:00:00Z", Breaker: run.BreakerClosed,
			ExpectedAssumptions: governance.ObligationAssumptions{
				Recurrence: governance.StandingSharedProcess, Platform: "fixture/os", ToolchainIdentity: "fixture-go",
				SurfaceDigest: "fixture-digest", MaxActiveJobs: 1, TimingEnvelopeSeconds: 1800, ObservationSource: "run-terminal-record",
			}, AdmissionDecision: governance.ConsequenceDecision{Apply: true},
		}}, nil
	}
	if _, err := store.Launch(run.Caller{Class: "HUMAN"}, run.LaunchParams{
		Id: "lens-governed", Kind: "suite", Display: "lens governed", Log: "artifacts/lens-governed.log",
		GoalId: "bounded", ObligationRevision: 6,
	}); err != nil {
		t.Fatal(err)
	}
	if err := obligationstate.RecordTerminal(root, "bounded", 3, 9, obligationstate.TerminalAttempt{
		RunID: "lens-durable", Status: run.StatusGreen, StartedAt: "2026-08-28T08:20:00Z",
		EndedAt: "2026-08-28T08:29:00Z", PrunedAt: "2026-08-28T08:30:00Z",
		AttemptOrdinal: 1, ExecutionCostMinutes: 11, ObservedCostMinutes: 9, WeightGeneration: 1, Breaker: run.BreakerClosed,
	}); err != nil {
		t.Fatal(err)
	}

	file := budgetGoal()
	file.Approved = &goal.ApprovalRecord{Revision: 3, EpisodeRevision: 3}
	file.Claimed.Revision = 5
	file.Claimed.AccountingRevision = 5
	file.Claimed.At = file.History[4].At
	file.Budget.AttemptLimit = 20
	file.Budget.ReservedJobMinutesLimit = 200
	unclaimed := *file
	unclaimed.State = goal.StateApproved
	unclaimed.Claimed = nil
	before := ProjectConsumption(root, &unclaimed, now)
	claimed := ProjectBudget(root, file, now)
	if before.Status != BudgetKnown || claimed.Status != BudgetKnown {
		t.Fatalf("lens status: unclaimed=%+v claimed=%+v", before, claimed)
	}
	if before.Attempts != 4 || before.ReservedJobMinutes != 23 || before.ObservedJobMinutes != 11 || before.ProofReservationMinutes != 5 {
		t.Fatalf("four-store consumption = %+v", before)
	}
	if before.Attempts != claimed.Attempts || before.ReservedJobMinutes != claimed.ReservedJobMinutes ||
		before.ObservedJobMinutes != claimed.ObservedJobMinutes || before.ProofReservationMinutes != claimed.ProofReservationMinutes {
		t.Fatalf("claimed and unclaimed consumption differ: unclaimed=%+v claimed=%+v", before, claimed)
	}
	after := ProjectConsumption(root, &unclaimed, now)
	if after.Attempts != before.Attempts || after.ReservedJobMinutes != before.ReservedJobMinutes ||
		after.ObservedJobMinutes != before.ObservedJobMinutes || after.ProofReservationMinutes != before.ProofReservationMinutes {
		t.Fatalf("release changed consumption: before=%+v after=%+v", before, after)
	}
}

func TestConsumptionLensUsesCandidateProofIdentity(t *testing.T) {
	root, identity := dispatchProofFixture(t, "candidate-consumption")
	launcher, err := proofrun.CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	_, decision, err := proofrun.ReserveLocked(dispatchFixtureProofAdmission(proofrun.WithTestHostLoadSampler(proofrun.AdmissionRequest{
		ControlRoot: root, ExecutionRoot: root, GoalID: "authority", GoalRevision: 5, AccountingRevision: 5,
		CandidateGoalID: "candidate", CandidateRevision: 3, ReservedMinutes: 5,
		Identity: identity, Launcher: launcher, Now: time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC), AttemptID: "candidate-charge",
	}, "0")))
	if err != nil {
		t.Fatal(err)
	}
	requireProofReservationNotAdmissionRefused(t, decision)
	file := budgetGoal()
	file.Id = "candidate"
	file.State = goal.StateApproved
	file.Revision = 3
	file.Claimed = nil
	file.Approved = &goal.ApprovalRecord{Revision: 3, EpisodeRevision: 3}
	file.Budget.AttemptLimit = 10
	file.Budget.ReservedJobMinutesLimit = 100
	projection := ProjectConsumption(root, file, time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC))
	if projection.Status != BudgetKnown || projection.Attempts != 1 || projection.ReservedJobMinutes != 5 || projection.ProofReservationMinutes != 5 {
		t.Fatalf("candidate proof identity was not charged to the candidate: %+v", projection)
	}
}

func TestOldEpochProofSkipsPrunedGovernedOwnerValidation(t *testing.T) {
	root, identity := dispatchProofFixture(t, "old-epoch-owner")
	file := budgetGoal()
	file.Obligation = &goal.GovernedObligation{Revision: 6}
	writeConsumedBudgetProof(t, root, "new-epoch-proof", 3, 6, time.Date(2026, 8, 28, 9, 30, 0, 0, time.UTC))
	launcher, err := proofrun.CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	oldEpoch := uint64(0)
	started := time.Date(2026, 8, 28, 9, 40, 0, 0, time.UTC)
	deadline := started.Add(5 * time.Minute).Format(time.RFC3339Nano)
	_, decision, err := proofrun.ReserveLocked(candidateProofAdmission(proofrun.AdmissionRequest{
		ControlRoot: root, ExecutionRoot: root, GoalID: "bounded", GoalRevision: 3, AccountingRevision: 3,
		BudgetEpoch: &oldEpoch, ReservedMinutes: 5, Identity: identity, Launcher: launcher, Now: started,
		AttemptID: "old-epoch-owner", ReservationOwner: &proofrun.ReservationOwner{
			ControlRoot: root, RunID: "pruned-old-owner", RunGeneration: 1, LaunchNonce: "old-epoch-nonce",
			GoalRevision: 3, ObligationRevision: 6, AttemptOrdinal: 1, BudgetEpoch: &oldEpoch, Deadline: deadline,
		},
	}))
	if err != nil {
		t.Fatal(err)
	}
	requireProofReservationNotAdmissionRefused(t, decision)
	projection := ProjectBudget(root, file, time.Date(2026, 8, 28, 9, 41, 0, 0, time.UTC))
	if projection.Status != BudgetKnown || projection.Attempts != 0 || projection.ReservedJobMinutes != 0 {
		t.Fatalf("old epoch proof validated its pruned owner instead of being ignored: %+v", projection)
	}
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
	writeConsumedProof(t, root, "bounded", runID, goalRevision, obligationRevision, consumedAt.Add(-30*time.Minute), consumedAt)
}

// writeConsumedProof records a green proof of goalID that started at
// startedAt and whose discharge was consumed at consumedAt.
func writeConsumedProof(t *testing.T, root, goalID, runID string, goalRevision, obligationRevision uint64, startedAt, consumedAt time.Time) {
	t.Helper()
	if err := obligationstate.RecordTerminal(root, goalID, goalRevision, obligationRevision, obligationstate.TerminalAttempt{
		RunID: runID, Status: run.StatusGreen, StartedAt: startedAt.Format(time.RFC3339),
		EndedAt: consumedAt.Add(-time.Minute).Format(time.RFC3339), PrunedAt: consumedAt.Add(time.Minute).Format(time.RFC3339),
		AttemptOrdinal: 1, ExecutionCostMinutes: 30, ObservedCostMinutes: 29, WeightGeneration: 1, Breaker: run.BreakerClosed,
	}); err != nil {
		t.Fatal(err)
	}
	writeJSON(t, filepath.Join(root, "artifacts", "agents", "validation-weight.json"), map[string]any{
		"schema": 1, "generation": 2,
		"consumedProofs": []any{map[string]any{
			"runId": runID, "goalId": goalID, "goalRevision": goalRevision, "obligationRevision": obligationRevision,
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
		terminal, decision, err := proofrun.ReserveLocked(candidateProofAdmission(proofrun.AdmissionRequest{
			ControlRoot: root, ExecutionRoot: root, GoalID: "bounded", GoalRevision: 3, AccountingRevision: 3,
			ReservedMinutes: 20, Identity: terminalIdentity, Launcher: launcher, Now: terminalStarted, AttemptID: "sum-terminal-proof",
		}))

		if err != nil {
			t.Fatal(err)
		}
		requireProofReservationNotAdmissionRefused(t, decision)
		if _, err := proofrun.FinalizeAttempt(root, terminal.AttemptID, proofrun.TerminalFailed, 1, "controlled proof failure", nil,
			terminalStarted.Add(5*time.Minute)); err != nil {
			t.Fatal(err)
		}
		_, decision, err = proofrun.ReserveLocked(candidateProofAdmission(proofrun.AdmissionRequest{
			ControlRoot: root, ExecutionRoot: root, GoalID: "bounded", GoalRevision: 3, AccountingRevision: 3,
			ReservedMinutes: 15, Identity: liveIdentity, Launcher: launcher,
			Now: time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC), AttemptID: "sum-live-proof",
		}))
		if err != nil {
			t.Fatal(err)
		}
		requireProofReservationNotAdmissionRefused(t, decision)
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
				want := "BUDGET_REFUSED: goal bounded revision=3 admission closed: attemptLimit used=3 limit=2, activeJobLimit used=1 limit=1; reserved observed=10 open-caps=0 proof=20 limit=75; rule=setup-refusal-release: terminal records with phase=setup and refusalClass=setup count as neither attempts nor reserved job minutes; " + budgetRaiseRemedy("bounded")
				if len(lines) != 1 || lines[0] != want {
					t.Fatalf("proof reservation refusal line = %v, want %q", lines, want)
				}
			}
		})
	}
}

func TestBudgetProjectionRefusesObservedProofAccountingOverflow(t *testing.T) {
	root, proofIdentity := dispatchProofFixture(t, "overflowed-terminal-proof")
	writeBudgetJob(t, root, "one-observed-minute", "one-observed-minute", 3, 1, "completed", budgetJobLife{
		startedAt: "2026-08-28T08:00:00Z", endedAt: "2026-08-28T08:01:00Z", pid: 4242,
	})
	launcher, err := proofrun.CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	started := time.Date(2026, 8, 28, 8, 2, 0, 0, time.UTC)
	attempt, decision, err := proofrun.ReserveLocked(candidateProofAdmission(proofrun.AdmissionRequest{
		ControlRoot: root, ExecutionRoot: root, GoalID: "bounded", GoalRevision: 3, AccountingRevision: 3,
		ReservedMinutes: 1, Identity: proofIdentity, Launcher: launcher, Now: started, AttemptID: "overflowed-terminal-proof",
	}))

	if err != nil {
		t.Fatal(err)
	}
	requireProofReservationNotAdmissionRefused(t, decision)
	terminal, err := proofrun.FinalizeAttempt(root, attempt.AttemptID, proofrun.TerminalFailed, 1, "controlled failure", nil, started.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	terminal.ObservedMinutes = math.MaxUint64
	encoded, err := json.MarshalIndent(terminal, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path, err := proofrun.AttemptPath(root, terminal.AttemptID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(encoded, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}

	projection := ProjectBudget(root, budgetGoal(), time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC))
	if projection.Status != BudgetUnknown || projection.Unknown == nil || projection.Unknown.Reason != "proof-attempt accounting overflowed" {
		t.Fatalf("overflowed observed proof accounting = %+v", projection)
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
	file.Approved.Revision = file.Claimed.Revision
	file.Approved.EpisodeRevision = file.Claimed.Revision
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
	reads := acceptedAbsentGoalReads(t, root, 1)
	stage := t.TempDir()
	capFile := writeJSON(t, filepath.Join(stage, "cap.json"), map[string]any{
		"capMin": 30, "capDeadline": "2026-08-28T10:00:00Z",
		"source": map[string]any{"rule": "fixture", "origin": "fixture", "truncatedBy": nil},
	})
	setup := filepath.Join(stage, "setup.json")
	if err := buildSetupWithGoalReads(root, setup, "reserved", "implementer", "", "main-1", "5", "bounded", 3, 3, capFile, "", "", reads); err != nil {
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

// markedBudgetGoal is the bounded goal, claimed at 08:00 with a four-hour
// box, marked as waiting to land at 09:00.
func markedBudgetGoal() *goal.GoalFile {
	file := budgetGoal()
	file.Landing = &goal.LandingRecord{At: "2026-08-28T09:00:00Z"}
	return file
}

// launchBudgetProof records that a reserved proof attempt launched its suite
// at the given time, as the suite launcher does: the run's process record,
// and its key on the attempt.
func launchBudgetProof(t *testing.T, root, attemptID string, at time.Time) {
	t.Helper()
	launcher, err := proofrun.CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	record := proofrun.Record{Suite: "testing", Root: root, ControlRoot: root, AttemptID: attemptID, LaunchID: "launch-1",
		Launcher: launcher, SuiteProcess: proofrun.ProcessIdentity{Pid: 1002, Pgid: 1002, PidStartedAt: at.Unix()},
		Watchdog: proofrun.ProcessIdentity{Pid: 1003, PidStartedAt: at.Unix()}, Status: proofrun.StatusRunning}
	path, err := proofrun.ProcessRecordPath(root, attemptID, record.LaunchID)
	if err != nil {
		t.Fatal(err)
	}
	writeJSON(t, path, record)
	if err := proofrun.UpdateAttemptProcesses(root, attemptID, launcher.Ref(), []string{record.Key()}); err != nil {
		t.Fatal(err)
	}
}

// reserveBudgetProof reserves a proof attempt of the bounded goal at the
// given time, in a fresh root it returns.
func reserveBudgetProof(t *testing.T, attemptID string, at time.Time) string {
	t.Helper()
	root, proofIdentity := dispatchProofFixture(t, attemptID)
	launcher, err := proofrun.CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	_, decision, err := proofrun.ReserveLocked(candidateProofAdmission(proofrun.AdmissionRequest{
		ControlRoot: root, ExecutionRoot: root, GoalID: "bounded", GoalRevision: 3, AccountingRevision: 3,
		ReservedMinutes: 1, Identity: proofIdentity, Launcher: launcher, Now: at, AttemptID: attemptID,
	}))
	if err != nil {
		t.Fatal(err)
	}
	requireProofReservationNotAdmissionRefused(t, decision)
	return root
}

func TestTheGoalsOwnJobEndsTheWait(t *testing.T) {
	t.Parallel()
	// A job of its own starting at 10:00, after the mark, ends the wait at
	// its start: the hour at the mark plus the time since 10:00.
	root := budgetProjectionRoot(t)
	writeBudgetJob(t, root, "after-mark", "reserve-after-mark", 3, 30, "completed", budgetJobLife{
		startedAt: "2026-08-28T10:00:00Z", endedAt: "2026-08-28T10:10:00Z", pid: 4242,
	})
	for _, observed := range []struct {
		now     time.Time
		elapsed time.Duration
	}{
		{time.Date(2026, 8, 28, 11, 0, 0, 0, time.UTC), 2 * time.Hour},
		{time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC), 3 * time.Hour},
	} {
		projection := ProjectBudget(root, markedBudgetGoal(), observed.now)
		if projection.Status != BudgetKnown || projection.Wait != time.Hour || projection.Elapsed != observed.elapsed {
			t.Fatalf("at %s the job after the mark did not end the wait at its start: %+v", observed.now.Format(time.RFC3339), projection)
		}
	}
	// A job still running at the mark leaves no wait at all; so does one
	// that started before the mark and ended after it.
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	for _, job := range []struct {
		status string
		life   budgetJobLife
	}{
		{"running", budgetJobLife{startedAt: "2026-08-28T08:30:00Z"}},
		{"completed", budgetJobLife{startedAt: "2026-08-28T08:30:00Z", endedAt: "2026-08-28T09:30:00Z", pid: 4242}},
	} {
		root := budgetProjectionRoot(t)
		writeBudgetJob(t, root, "across-mark", "reserve-across-mark", 3, 30, job.status, job.life)
		projection := ProjectBudget(root, markedBudgetGoal(), now)
		if projection.Status != BudgetKnown || projection.Wait != 0 || projection.Elapsed != 4*time.Hour {
			t.Fatalf("a %s job across the mark left a wait: %+v", job.status, projection)
		}
	}
	// A proof attempt of its own reserved at 10:30 does not end the wait until
	// it launches; launched at 10:45, it ends the wait at its launch.
	root = reserveBudgetProof(t, "wait-ending-proof", time.Date(2026, 8, 28, 10, 30, 0, 0, time.UTC))
	projection := ProjectBudget(root, markedBudgetGoal(), now)
	if projection.Status != BudgetKnown || projection.Wait != 3*time.Hour || projection.Elapsed != time.Hour {
		t.Fatalf("the proof's reservation alone ended the wait: %+v", projection)
	}
	launchBudgetProof(t, root, "wait-ending-proof", time.Date(2026, 8, 28, 10, 45, 0, 0, time.UTC))
	projection = ProjectBudget(root, markedBudgetGoal(), now)
	if projection.Status != BudgetKnown || projection.Wait != 105*time.Minute || projection.Elapsed != 135*time.Minute {
		t.Fatalf("the proof after the mark did not end the wait at its launch: %+v", projection)
	}
}

func TestIdleCountsForAClaimNeverReleased(t *testing.T) {
	t.Parallel()
	// The budget starts exactly at the claim; the idle half hour recorded on
	// it still comes off the clock.
	file := budgetGoal()
	file.Claimed.IdleSeconds = 1800
	projection := ProjectBudget(budgetProjectionRoot(t), file, time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC))
	if projection.Status != BudgetKnown || !projection.StartedAt.Equal(time.Date(2026, 8, 28, 8, 0, 0, 0, time.UTC)) ||
		projection.Elapsed != 90*time.Minute {
		t.Fatalf("the idle of a claim never released was dropped: %+v", projection)
	}
}

func TestLandingOverdueAgreesWithTheProjection(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name    string
		afterAt string
		now     time.Time
		overdue bool
	}{
		{"only waited, past the box since the mark", "", time.Date(2026, 8, 28, 14, 0, 0, 0, time.UTC), false},
		{"worked after the mark, inside the box", "2026-08-28T10:00:00Z", time.Date(2026, 8, 28, 12, 30, 0, 0, time.UTC), false},
		{"worked after the mark, past the box", "2026-08-28T10:00:00Z", time.Date(2026, 8, 28, 14, 0, 0, 0, time.UTC), true},
	} {
		t.Run(row.name, func(t *testing.T) {
			root := budgetProjectionRoot(t)
			if row.afterAt != "" {
				writeBudgetJob(t, root, "after-mark", "reserve-after-mark", 3, 30, "completed", budgetJobLife{
					startedAt: row.afterAt, endedAt: "2026-08-28T10:10:00Z", pid: 4242,
				})
			}
			file := markedBudgetGoal()
			projection := ProjectBudget(root, file, row.now)
			if projection.Status != BudgetKnown || (projection.ElapsedState != "") != row.overdue {
				t.Fatalf("projection = %+v, want past the box %v", projection, row.overdue)
			}
			lines := goal.LandingClaimLines([]*goal.GoalFile{file}, row.now, LandingOverdue(root))
			if len(lines) != 1 || strings.HasPrefix(lines[0], "LANDING OVERDUE ") != row.overdue {
				t.Fatalf("the landing line %v disagrees with the projection (overdue %v)", lines, row.overdue)
			}
		})
	}
}

func TestTheLandingLineAgreesWithTheProjectionAfterADischarge(t *testing.T) {
	t.Parallel()
	// The episode began at 08:00 and the goal was marked at 09:29:30, after
	// its discharging proof ended; the discharge at 09:30 moved the budget's
	// start, and its own job at 10:00 ended the wait. Five hours after the
	// episode began, the projection still holds it inside its four-hour box.
	root := budgetProjectionRoot(t)
	writeConsumedBudgetProof(t, root, "green-discharge", 3, 5, time.Date(2026, 8, 28, 9, 30, 0, 0, time.UTC))
	writeBudgetJob(t, root, "after-mark", "reserve-after-mark", 3, 30, "completed", budgetJobLife{
		startedAt: "2026-08-28T10:00:00Z", endedAt: "2026-08-28T10:10:00Z", pid: 4242,
	})
	file := dischargedEpisodeGoal()
	file.Landing = &goal.LandingRecord{At: "2026-08-28T09:29:30Z"}
	for _, row := range []struct {
		now     time.Time
		elapsed time.Duration
		overdue bool
	}{
		{time.Date(2026, 8, 28, 13, 0, 0, 0, time.UTC), 3 * time.Hour, false},
		{time.Date(2026, 8, 28, 14, 0, 0, 0, time.UTC), 4 * time.Hour, true},
	} {
		projection := ProjectBudget(root, file, row.now)
		if projection.Status != BudgetKnown || projection.Elapsed != row.elapsed || WaitSpan(root, file, row.now) != 30*time.Minute {
			t.Fatalf("at %s the projection is not the reproduction's: %+v", row.now.Format(time.RFC3339), projection)
		}
		lines := goal.LandingClaimLines([]*goal.GoalFile{file}, row.now, LandingOverdue(root))
		if len(lines) != 1 || strings.HasPrefix(lines[0], "LANDING OVERDUE ") != row.overdue {
			t.Fatalf("at %s the landing line %v disagrees with the projected elapsed %s", row.now.Format(time.RFC3339), lines, projection.Elapsed)
		}
	}
}

func TestAnUnreadableProjectionMakesNoOverdueClaim(t *testing.T) {
	t.Parallel()
	// Marked at 09:00 with no job or proof of its own since, the goal is six
	// hours into its episode at 14:00, past its four-hour box. Only a known
	// projection, which takes off the wait, prints it as overdue: a goal whose
	// own job from 10:00 ended a one-hour wait is five hours in.
	now, file := time.Date(2026, 8, 28, 14, 0, 0, 0, time.UTC), markedBudgetGoal()
	unreadable, known := budgetProjectionRoot(t), budgetProjectionRoot(t)
	writeBudgetJob(t, unreadable, "bad-time", "reserve-bad-time", 3, 120, "completed", budgetJobLife{startedAt: "2026-08-28T08:10:00Z", endedAt: "yesterday", pid: 4242})
	writeBudgetJob(t, known, "after-mark", "reserve-after-mark", 3, 30, "completed", budgetJobLife{startedAt: "2026-08-28T10:00:00Z", endedAt: "2026-08-28T10:10:00Z", pid: 4242})
	if ProjectBudget(unreadable, file, now).Status != BudgetUnknown || ProjectBudget(known, file, now).Elapsed != 5*time.Hour {
		t.Fatal("the fixtures are not an unreadable projection and a known one past the box")
	}
	waiting := "LANDING bounded: land-ready since 2026-08-28T09:00:00Z; the queue is open"
	for name, row := range map[string]struct {
		read func(*goal.GoalFile, time.Time) (bool, bool)
		want string
	}{"no reader": {nil, waiting}, "unreadable": {LandingOverdue(unreadable), waiting}, "known": {LandingOverdue(known), "LANDING OVERDUE bounded: land-ready since 2026-08-28T09:00:00Z and past its elapsed box; land it; the queue is open"}} {
		if lines := goal.LandingClaimLines([]*goal.GoalFile{file}, now, row.read); len(lines) != 1 || lines[0] != row.want {
			t.Fatalf("%s: the landing line %v, want %q", name, lines, row.want)
		}
	}
}

// waitedGoal is a seat's goal claimed at 08:00 under a four-hour box and
// marked as waiting to land at 09:00, with no job or proof of its own since.
func waitedGoal(t *testing.T) *gcliBudgetBed {
	t.Helper()
	claimAt := time.Date(2026, 8, 28, 8, 0, 0, 0, time.UTC)
	bed := newGCLIBudgetBed(t, claimAt)
	result, err := goal.Open(bed.request(claimAt, false), "waited", "Work that waits to land.", goal.OriginMain, "Land it.")
	bed.confirm("open", result, err)
	risk := goal.RiskRecord{Severity: 3, Novelty: 3, Exposure: 1, Accumulation: 1, Basis: "The fixture waits to land."}
	result, err = goal.Edit(bed.request(claimAt, false), "waited", goal.EditFields{Risk: &risk})
	bed.confirm("risk", result, err)
	budget := bed.budget("4h", 4, 240, 2, 3)
	result, err = goal.Approve(bed.request(claimAt, true), []string{"waited"}, &budget, bed.proof)
	bed.confirm("approve", result, err)
	result, err = goal.Claim(bed.request(claimAt, false), "waited")
	bed.confirm("claim", result, err)
	result, err = goal.LandReady(bed.request(claimAt.Add(time.Hour), false), "waited")
	bed.confirm("land-ready", result, err)
	return bed
}

// waitedFile is the goal as the accepted ledger holds it.
func (bed *gcliBudgetBed) waitedFile(at time.Time) *goal.GoalFile {
	bed.t.Helper()
	projection, err := goal.Project(bed.endpoint(), false, at)
	if err != nil {
		bed.t.Fatal(err)
	}
	return projection.Tree.Live["waited"]
}

// claimAgainKeepsTheWait claims the goal again at 11:30 and checks that the
// two-hour wait and the half-hour gap are idle and off the clock at noon.
func (bed *gcliBudgetBed) claimAgainKeepsTheWait() {
	bed.t.Helper()
	result, err := goal.Claim(bed.request(time.Date(2026, 8, 28, 11, 30, 0, 0, time.UTC), false), "waited")
	bed.confirm("claim again", result, err)
	noon := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	file := bed.waitedFile(noon)
	if projection := ProjectBudget(bed.root, file, noon); file.Claimed.IdleSeconds != 9000 || projection.Status != BudgetKnown || projection.Elapsed != 90*time.Minute {
		bed.t.Fatalf("the wait and the gap are not idle: claim=%+v projection=%+v", file.Claimed, projection)
	}
}

func TestAReleasedWaitIsCreditedAtTheNextClaim(t *testing.T) {
	t.Parallel()
	bed := waitedGoal(t)
	releaseAt := time.Date(2026, 8, 28, 11, 0, 0, 0, time.UTC)
	release := bed.request(releaseAt, false)
	release.Waits = map[string]goal.Wait{"waited": OpenWait(bed.root, bed.waitedFile(releaseAt), releaseAt)}
	result, err := goal.ReleaseWithReason(release, "waited", "the lane is slow")
	bed.confirm("release", result, err)
	bed.claimAgainKeepsTheWait()
}

func TestAParkedWaitIsCreditedAtTheNextClaim(t *testing.T) {
	t.Parallel()
	bed := waitedGoal(t)
	parkAt := time.Date(2026, 8, 28, 11, 0, 0, 0, time.UTC)
	park := bed.request(parkAt, false)
	park.Waits = map[string]goal.Wait{"waited": OpenWait(bed.root, bed.waitedFile(parkAt), parkAt)}
	park.ParkBranchCheck = func(string, string) (string, error) { return "", nil }
	result, err := goal.Park(park, "waited", "the lane is slow")
	bed.confirm("park", result, err)
	result, err = goal.Unpark(bed.request(parkAt.Add(10*time.Minute), false), "waited")
	bed.confirm("unpark", result, err)
	bed.claimAgainKeepsTheWait()
}

func TestAReleaseAfterADischargeCreditsOnlyTheSubtractedWait(t *testing.T) {
	t.Parallel()
	// Marked at 09:00, the goal's own proof started at 10:00 and its discharge
	// was consumed at 11:00, moving the budget's start past the whole wait, so
	// the projection takes none of it off. Released at noon and claimed again
	// by its pair at 12:30, the goal keeps only the unheld half hour as idle
	// and has the hour of elapsed it had at the release.
	bed := waitedGoal(t)
	at := func(hour, minute int) time.Time { return time.Date(2026, 8, 28, hour, minute, 0, 0, time.UTC) }
	// The projection reads the discharged obligation from the goal it is
	// handed; the ledger's goal carries none.
	discharged := func(now time.Time) *goal.GoalFile {
		file := bed.waitedFile(now)
		file.Obligation = &goal.GovernedObligation{Revision: 5}
		return file
	}
	writeConsumedProof(t, bed.root, "waited", "green-discharge", bed.waitedFile(at(12, 0)).Claimed.EpisodeRevision, 5, at(10, 0), at(11, 0))
	before := ProjectBudget(bed.root, discharged(at(12, 0)), at(12, 0))
	if before.Status != BudgetKnown || !before.StartedAt.Equal(at(11, 0)) || before.Wait != 0 || before.Elapsed != time.Hour {
		t.Fatalf("the discharge did not move the budget's start past the wait: %+v", before)
	}
	release := bed.request(at(12, 0), false)
	release.Waits = map[string]goal.Wait{"waited": OpenWait(bed.root, discharged(at(12, 0)), at(12, 0))}
	result, err := goal.ReleaseWithReason(release, "waited", "the lane is slow")
	bed.confirm("release", result, err)
	result, err = goal.Claim(bed.request(at(12, 30), false), "waited")
	bed.confirm("claim again", result, err)
	after := ProjectBudget(bed.root, discharged(at(12, 30)), at(12, 30))
	if claim := bed.waitedFile(at(12, 30)).Claimed; claim.IdleSeconds != 1800 || after.Status != BudgetKnown || after.Elapsed != before.Elapsed {
		t.Fatalf("the release credited a wait the projection never took off: claim=%+v projection=%+v", claim, after)
	}
}
