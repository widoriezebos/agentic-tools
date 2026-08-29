package steward

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

type healthProbe map[int64]struct {
	exact identity.Exact
	state identity.Liveness
	err   error
}

func (p healthProbe) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	if result, ok := p[pid]; ok {
		return result.exact, result.state, result.err
	}
	return identity.Exact{}, identity.Unknown, errors.New("fixture has no process fact")
}

func TestHealthExitCodeArms(t *testing.T) {
	for _, test := range []struct {
		name  string
		roles []RoleVerdict
		want  int
	}{
		{name: "healthy", roles: []RoleVerdict{{Role: RoleStewardRunner, Status: HealthAlive}}, want: 0},
		{name: "dead outranks unknown", roles: []RoleVerdict{
			{Role: RoleStewardRunner, Status: HealthUnknown},
			{Role: RoleRepoWatcher, Status: HealthDead},
		}, want: 1},
		{name: "unknown", roles: []RoleVerdict{{Role: RoleStewardRunner, Status: HealthUnknown}}, want: 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			verdict := applyHealthObservation("/repo", HealthObservationState{}, test.roles, time.Now())
			if got := verdict.ExitCode(); got != test.want {
				t.Fatalf("exit code = %d, want %d: %+v", got, test.want, verdict)
			}
		})
	}
}

func TestUnknownGraceAlertsOnSecondConsecutiveObservation(t *testing.T) {
	unknown := []RoleVerdict{{Role: RoleCensusFreshness, Status: HealthUnknown, Reason: "census unreadable", Remedy: "repair"}}
	first := applyHealthObservation("/repo", HealthObservationState{}, unknown, time.Now())
	if first.ExitCode() != 2 || first.ShouldAlert {
		t.Fatalf("the first unknown exits 2 without alerting: %+v", first)
	}
	second := applyHealthObservation("/repo", first.State, unknown, time.Now().Add(time.Minute))
	if second.ExitCode() != 2 || !second.ShouldAlert || second.Roles[0].ConsecutiveUnknown != 2 {
		t.Fatalf("the second consecutive unknown alerts: %+v", second)
	}
	reset := applyHealthObservation("/repo", second.State, []RoleVerdict{{Role: RoleCensusFreshness, Status: HealthAlive}}, time.Now().Add(2*time.Minute))
	if reset.State.UnknownCounts[RoleCensusFreshness] != 0 {
		t.Fatalf("an alive observation resets unknown grace: %+v", reset.State)
	}
}

func TestHealthClockRegressionNeverReportsAlive(t *testing.T) {
	now := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	originalObservation := now.Add(time.Minute)
	previous := HealthObservationState{
		Sequence: 4, ObservedAt: originalObservation, UnknownCounts: map[HealthRole]int{}, FailureCounts: map[HealthRole]int{},
	}
	verdict := applyHealthObservation("/repo", previous, []RoleVerdict{{
		Role: RoleStewardRunner, Status: HealthAlive, Reason: "fresh",
	}}, now)
	if verdict.ExitCode() != 2 || verdict.Roles[0].Status != HealthUnknown ||
		!strings.Contains(verdict.Roles[0].Reason, "CLOCK_REGRESSED") || verdict.Roles[0].Remedy == "" {
		t.Fatalf("a backward clock movement is unknown, never alive: %+v", verdict)
	}
	if !verdict.State.ObservedAt.Equal(originalObservation) {
		t.Fatalf("a regressed observation cannot replace the coherent clock anchor: %+v", verdict.State)
	}
	stillRegressed := applyHealthObservation("/repo", verdict.State, []RoleVerdict{{
		Role: RoleStewardRunner, Status: HealthAlive, Reason: "fresh",
	}}, now.Add(time.Second))
	if stillRegressed.Roles[0].Status != HealthUnknown || !stillRegressed.State.ObservedAt.Equal(originalObservation) {
		t.Fatalf("clock regression persists until UTC reaches the original observation: %+v", stillRegressed)
	}
	recovered := applyHealthObservation("/repo", stillRegressed.State, []RoleVerdict{{
		Role: RoleStewardRunner, Status: HealthAlive, Reason: "fresh",
	}}, originalObservation)
	if recovered.Roles[0].Status != HealthAlive || !recovered.State.ObservedAt.Equal(originalObservation) {
		t.Fatalf("an observation at the original timestamp clears the regression: %+v", recovered)
	}
}

func TestFailureBreakerProjectsEveryObservationAndResets(t *testing.T) {
	dead := []RoleVerdict{{Role: RoleStewardRunner, Status: HealthDead, Reason: "stale", Remedy: "restart"}}
	state := HealthObservationState{}
	for observation := 1; observation <= 5; observation++ {
		verdict := applyHealthObservation("/repo", state, dead, time.Unix(int64(observation), 0))
		role := verdict.Roles[0]
		if role.ConsecutiveFailures != observation || verdict.State.FailureCounts[RoleStewardRunner] != observation {
			t.Fatalf("failure observation %d was not persisted and projected: %+v", observation, verdict)
		}
		if observation < 5 && (verdict.ShouldAlert || role.FailureEscalation != "AUTO_HEAL_ELIGIBLE") {
			t.Fatalf("failure observation %d remains below the breaker: %+v", observation, verdict)
		}
		if observation == 5 && (!verdict.ShouldAlert || role.FailureEscalation != "AUTO_HEAL_ENDED") {
			t.Fatalf("failure five ends auto-heal and alerts: %+v", verdict)
		}
		state = verdict.State
	}
	unknown := applyHealthObservation("/repo", state, []RoleVerdict{{Role: RoleStewardRunner, Status: HealthUnknown}}, time.Unix(6, 0))
	if unknown.State.FailureCounts[RoleStewardRunner] != 5 {
		t.Fatalf("unknown neither increments nor resets the failure breaker: %+v", unknown.State)
	}
	alive := applyHealthObservation("/repo", unknown.State, []RoleVerdict{{Role: RoleStewardRunner, Status: HealthAlive}}, time.Unix(7, 0))
	if alive.State.FailureCounts[RoleStewardRunner] != 0 {
		t.Fatalf("one alive observation resets the failure breaker: %+v", alive.State)
	}
}

func TestHealingFlapEscalatesAcrossAliveResets(t *testing.T) {
	now := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	dead := []RoleVerdict{{Role: RoleStewardRunner, Status: HealthDead, Reason: "recorded runner pid 91 is dead", Remedy: "restart"}}
	alive := []RoleVerdict{{Role: RoleStewardRunner, Status: HealthAlive, Reason: "fresh"}}
	state := HealthObservationState{}
	for episode := 1; episode <= healthFlapLimit; episode++ {
		failed := applyHealthObservation("/repo", state, dead, now.Add(time.Duration(episode*2)*time.Minute))
		if episode < healthFlapLimit && (failed.ShouldAlert || failed.Roles[0].FailureEscalation != AutoHealEligible) {
			t.Fatalf("failure episode %d remains silent while healing is eligible: %+v", episode, failed)
		}
		if episode == healthFlapLimit && (!failed.ShouldAlert || failed.Roles[0].FailureEscalation != HealingFlapping || failed.Roles[0].ConsecutiveFailures != 1) {
			t.Fatalf("the third heal-and-fail episode must escalate across healthy resets: %+v", failed)
		}
		state = failed.State
		if episode < healthFlapLimit {
			recovered := applyHealthObservation("/repo", state, alive, now.Add(time.Duration(episode*2+1)*time.Minute))
			if recovered.State.FailureCounts[RoleStewardRunner] != 0 || len(recovered.State.FailureEpisodes[RoleStewardRunner]) != episode {
				t.Fatalf("healthy resets the consecutive breaker but retains flap history: %+v", recovered.State)
			}
			state = recovered.State
		}
	}
}

func TestNoLawfulRemedyEscalatesOnFirstObservation(t *testing.T) {
	dead := []RoleVerdict{{Role: RoleHookFreshness, Status: HealthDead, Reason: "the hook did not emit", Remedy: "inspect"}}
	verdict := applyHealthObservation("/repo", HealthObservationState{}, dead, time.Now())
	if !verdict.ShouldAlert || verdict.Roles[0].FailureEscalation != NoLawfulRemedy {
		t.Fatalf("a failure class without an automatic remedy must escalate immediately: %+v", verdict)
	}
}

func TestFindingDigestUsesStableRoleIdentity(t *testing.T) {
	first := healthFindingDigest([]RoleVerdict{{Role: RoleNarratorFreshness, Status: HealthDead, Reason: "lastSuccess is stale at 3m0s"}})
	second := healthFindingDigest([]RoleVerdict{{Role: RoleNarratorFreshness, Status: HealthDead, Reason: "lastSuccess is stale at 4m0s"}})
	if first != second {
		t.Fatalf("time-varying reason text must not mint another finding identity: %s != %s", first, second)
	}
}

func TestClaimedGoalWithoutStructuredBudgetIsDead(t *testing.T) {
	root := convertedBed(t, "bed-m1", map[string]*goal.GoalFile{
		"hungry-goal": {
			Id: "hungry-goal", State: goal.StateClaimed, Intent: "Keep work bounded", Origin: goal.OriginMain,
			NextStep: "Finish whenever it is ready.", OpenedAt: "2026-08-23T00:00:00Z", Revision: 2,
			Claimed: &goal.ClaimRecord{Machine: "bed-m1", Lineage: "coordinator", At: "2026-08-23T01:00:00Z"},
			History: bedHistory("hungry-goal", "claim"),
		},
	})
	role := checkClaimedGoalBudgets(root, time.Now())
	if role.Status != HealthDead || !strings.Contains(role.Reason, "BUDGET_MISSING record=plans/goals/hungry-goal.md") ||
		!strings.Contains(role.Remedy, "goal set-budget --root . --id hungry-goal") {
		t.Fatalf("the budgetless claimed goal and its current remedy must be named: %+v", role)
	}
}

func structuredHealthGoal() *goal.GoalFile {
	history := bedHistory("bounded-goal", "open")
	history[0].At = "2026-08-28T07:00:00Z"
	history = append(history,
		goal.HistoryLine{
			At: "2026-08-28T08:00:00Z", Opid: "01ARZ3NDEKTSV4RRFFQ69G5FAW-bed-m1-00000001",
			Verb: "claim", Actor: "bed-m1+coordinator", Targets: []string{"bounded-goal"}, Keep: -1,
		},
		goal.HistoryLine{
			At: "2026-08-28T08:30:00Z", Opid: "01ARZ3NDEKTSV4RRFFQ69G5FAX-bed-m1-00000002",
			Verb: "edit", Actor: "bed-m1+coordinator", Targets: []string{"bounded-goal"}, Keep: -1,
		},
	)
	return &goal.GoalFile{
		Id: "bounded-goal", State: goal.StateClaimed, Intent: "Keep work bounded", Origin: goal.OriginMain,
		NextStep: "Finish the bounded slice.", OpenedAt: "2026-08-28T08:00:00Z", Revision: 3,
		Budget: &goal.Budget{
			ElapsedLimit: "4h", AttemptLimit: 2, ReservedJobMinutesLimit: 60, ActiveJobLimit: 1,
		},
		Claimed: &goal.ClaimRecord{
			Machine: "bed-m1", Lineage: "coordinator", At: "2026-08-28T08:00:00Z", Revision: 2,
		},
		History: history,
	}
}

func writeHealthJob(t *testing.T, root, name, body string) {
	t.Helper()
	directory := filepath.Join(root, "artifacts", "agents", "jobs")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, name+".json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestClaimedGoalStructuredBudgetHealthEvidence(t *testing.T) {
	now := time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC)

	t.Run("within budget", func(t *testing.T) {
		root := convertedBed(t, "bed-m1", map[string]*goal.GoalFile{"bounded-goal": structuredHealthGoal()})
		role := checkClaimedGoalBudgets(root, now)
		if role.Status != HealthAlive || !strings.Contains(role.Reason, "attempts=0/2") {
			t.Fatalf("known structured budget was not judged: %+v", role)
		}
	})

	t.Run("breach", func(t *testing.T) {
		root := convertedBed(t, "bed-m1", map[string]*goal.GoalFile{"bounded-goal": structuredHealthGoal()})
		writeHealthJob(t, root, "one", `{"jobId":"one","operationId":"reserve-one","goalId":"bounded-goal","goalRevision":2,"capMin":40,"status":"running"}`)
		writeHealthJob(t, root, "two", `{"jobId":"two","operationId":"reserve-two","goalId":"bounded-goal","goalRevision":2,"capMin":40,"status":"pending"}`)
		role := checkClaimedGoalBudgets(root, now)
		if role.Status != HealthDead || !strings.Contains(role.Reason, "reservedJobMinutesLimit") ||
			!strings.Contains(role.Reason, "activeJobLimit") || !strings.Contains(role.Remedy, "steward tick") {
			t.Fatalf("structured breaches did not route to breach-stop healing: %+v", role)
		}
	})

	t.Run("budget unknown", func(t *testing.T) {
		root := convertedBed(t, "bed-m1", map[string]*goal.GoalFile{"bounded-goal": structuredHealthGoal()})
		writeHealthJob(t, root, "revisionless", `{"jobId":"revisionless","operationId":"reserve-one","goalId":"bounded-goal","capMin":20,"status":"running"}`)
		role := checkClaimedGoalBudgets(root, now)
		if role.Status != HealthUnknown || !strings.Contains(role.Reason, "BUDGET_UNKNOWN") ||
			!strings.Contains(role.Reason, "artifacts/agents/jobs/revisionless.json") {
			t.Fatalf("unknown spending did not name its exact record: %+v", role)
		}
	})
}

func TestBreachStopHealthHealsBeforeNotifyAndEscalatesIndeterminate(t *testing.T) {
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	file := structuredHealthGoal()
	file.StopCapability = &goal.StopCapability{Generation: 2, Revision: 2, Machine: "bed-m1", ClaimEpoch: 7, FenceEpoch: 1}
	file.StopFence = &goal.StopFence{
		StopID: "stop-bounded-goal-r2-f1", Revision: 2, Epoch: 1, CapabilityGeneration: 2,
		ClosedAt: now.Add(-time.Minute).Format(time.RFC3339), Reason: goal.StopReasonElapsedLimit,
	}
	root := convertedBed(t, "bed-m1", map[string]*goal.GoalFile{"bounded-goal": file})
	stamp := now.Format(time.RFC3339)
	batch := goal.StopBatch{
		StopID: file.StopFence.StopID, GoalID: file.Id, GoalRevision: 2, FenceEpoch: 1,
		CapabilityGeneration: 2, Machine: "bed-m1", ClaimEpoch: 7, Reason: goal.StopReasonElapsedLimit,
		State: goal.StopBatchComplete, OpenedAt: stamp, UpdatedAt: stamp, CompletedAt: stamp, Pass: 1,
	}
	if err := goal.WriteStopBatch(root, batch); err != nil {
		t.Fatal(err)
	}
	if role := checkClaimedGoalBudgets(root, now); role.Status != HealthAlive || !strings.Contains(role.Reason, "BREACH_STOP_COMPLETE") {
		t.Fatalf("complete machinery stop must be health-alive history: %+v", role)
	}

	batch.State = goal.StopBatchIndeterminate
	batch.CompletedAt = ""
	batch.Failure = "custody cannot be proven"
	// COMPLETE is absorbing, so use a second root for the failure episode.
	failureRoot := convertedBed(t, "bed-m1", map[string]*goal.GoalFile{"bounded-goal": file})
	if err := goal.WriteStopBatch(failureRoot, batch); err != nil {
		t.Fatal(err)
	}
	role := checkClaimedGoalBudgets(failureRoot, now)
	verdict := applyHealthObservation(failureRoot, HealthObservationState{}, []RoleVerdict{role}, now)
	if role.Status != HealthDead || !strings.Contains(role.Reason, "INDETERMINATE") ||
		!verdict.ShouldAlert || verdict.Roles[0].FailureEscalation != NoLawfulRemedy {
		t.Fatalf("indeterminate custody must keep the fence and alert as no-lawful-remedy: role=%+v verdict=%+v", role, verdict)
	}
}

func TestClaimedGoalRemedyIsJudgedPerGoal(t *testing.T) {
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	breach := structuredHealthGoal()
	breach.Id = "a-breach"
	for index := range breach.History {
		breach.History[index].Targets = []string{breach.Id}
	}
	indeterminate := structuredHealthGoal()
	indeterminate.Id = "z-indeterminate"
	for index := range indeterminate.History {
		indeterminate.History[index].Targets = []string{indeterminate.Id}
	}
	indeterminate.StopCapability = &goal.StopCapability{
		Generation: 2, Revision: 2, Machine: "bed-m1", ClaimEpoch: 7, FenceEpoch: 1,
	}
	indeterminate.StopFence = &goal.StopFence{
		StopID: "stop-z-indeterminate-r2-f1", Revision: 2, Epoch: 1, CapabilityGeneration: 2,
		ClosedAt: now.Add(-time.Minute).Format(time.RFC3339), Reason: goal.StopReasonElapsedLimit,
	}
	root := convertedBed(t, "bed-m1", map[string]*goal.GoalFile{
		breach.Id: breach, indeterminate.Id: indeterminate,
	})
	writeHealthJob(t, root, "over-limit", `{"jobId":"over-limit","operationId":"reserve-over-limit","goalId":"a-breach","goalRevision":2,"capMin":80,"status":"running"}`)
	stamp := now.Format(time.RFC3339)
	batch := goal.StopBatch{
		StopID: indeterminate.StopFence.StopID, GoalID: indeterminate.Id, GoalRevision: 2,
		FenceEpoch: 1, CapabilityGeneration: 2, Machine: "bed-m1", ClaimEpoch: 7,
		Reason: goal.StopReasonElapsedLimit, State: goal.StopBatchIndeterminate,
		OpenedAt: stamp, UpdatedAt: stamp, Failure: "custody cannot be proven",
	}
	if err := goal.WriteStopBatch(root, batch); err != nil {
		t.Fatal(err)
	}
	role := checkClaimedGoalBudgets(root, now)
	verdict := applyHealthObservation(root, HealthObservationState{}, []RoleVerdict{role}, now)
	if !strings.Contains(role.Reason, "a-breach revision=2 BREACH") ||
		!strings.Contains(role.Reason, "z-indeterminate revision=2 BREACH_STOP_INDETERMINATE") ||
		!role.NoAutomaticRemedy || !strings.Contains(role.Remedy, "keep the launch fence closed") ||
		!verdict.ShouldAlert || verdict.Roles[0].FailureEscalation != NoLawfulRemedy {
		t.Fatalf("one healable goal must not mask another goal's indeterminate custody: role=%+v verdict=%+v", role, verdict)
	}
}

func TestClaimedGoalMissingOrMalformedBudgetNamesSetBudget(t *testing.T) {
	now := time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name   string
		budget *goal.Budget
	}{
		{name: "missing"},
		{name: "malformed", budget: &goal.Budget{ElapsedLimit: "0h", AttemptLimit: 2, ReservedJobMinutesLimit: 60, ActiveJobLimit: 1}},
	} {
		t.Run(test.name, func(t *testing.T) {
			file := structuredHealthGoal()
			file.Budget = test.budget
			root := convertedBed(t, "bed-m1", map[string]*goal.GoalFile{"bounded-goal": file})
			role := checkClaimedGoalBudgets(root, now)
			if role.Status != HealthDead || !strings.Contains(role.Reason, "bounded-goal") ||
				!strings.Contains(role.Remedy, "goal set-budget") {
				t.Fatalf("%s tuple did not produce the typed remedy: %+v", test.name, role)
			}
		})
	}
}

func TestNonterminalJobWithProvablyDeadProcessIsNamed(t *testing.T) {
	root := t.TempDir()
	jobs := filepath.Join(root, "artifacts", "agents", "jobs")
	if err := os.MkdirAll(jobs, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jobs, "dead-job.json"), []byte(`{
  "jobId":"dead-job",
  "status":"running",
  "pid":44001,
  "pidStartedAt":100
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	role := checkNonterminalJobs(root, healthProbe{44001: {state: identity.Dead}})
	if role.Status != HealthDead || !strings.Contains(role.Reason, "dead-job") ||
		!strings.Contains(role.Remedy, "dispatch.sh") || !strings.Contains(role.Remedy, "reap") {
		t.Fatalf("the dead-process job and its current remedy must be named: %+v", role)
	}
}

func TestSessionAnnouncementUsesCurrentSessionSchema(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "artifacts", "agents", "mains")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "session-one.json"), []byte(`{
  "sessionId":"session-one",
  "pid":44004,
  "pidStartedAt":100
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	probe := healthProbe{44004: {exact: identity.Exact{Pid: 44004, StartedAt: time.Unix(100, 0)}, state: identity.Alive}}
	role := checkSessionMain(root, probe)
	if role.Status != HealthAlive || !strings.Contains(role.Reason, "session-one") {
		t.Fatalf("the current sessionId announcement schema must prove an alive main: %+v", role)
	}
}

func TestGenerationBoundComponentSuccess(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	process := identity.Ref{Pid: 44002, StartedAtSec: 100, StartTicks: 99, BootID: "boot-a"}
	attempt, err := beginComponentAttempt(root, "steward-tick", 7, process, now)
	if err != nil {
		t.Fatal(err)
	}
	if !attempt.LastSuccess.IsZero() || !attempt.LastCompletion.IsZero() || attempt.AttemptSeq != 1 {
		t.Fatalf("an attempt alone advances no success or completion: %+v", attempt)
	}
	ok, err := completeComponentAttempt(root, "steward-tick", 7, attempt.AttemptSeq, ComponentOK, "PASS_COMPLETE", "durable-results", now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if !ok.LastSuccess.Equal(now.Add(time.Second)) || !ok.LastCompletion.Equal(now.Add(time.Second)) {
		t.Fatalf("an OK completion advances completion and success: %+v", ok)
	}
	sameGeneration, err := beginComponentAttempt(root, "steward-tick", 7, process, now.Add(2*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	failedSameGeneration, err := completeComponentAttempt(root, "steward-tick", 7, sameGeneration.AttemptSeq,
		ComponentError, "HEALTH_FAILED", "health read failed", now.Add(3*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if !failedSameGeneration.LastSuccess.Equal(ok.LastSuccess) || !failedSameGeneration.LastCompletion.Equal(now.Add(3*time.Second)) {
		t.Fatalf("an ERROR completion preserves the prior success and advances only completion: %+v", failedSameGeneration)
	}

	next, err := beginComponentAttempt(root, "steward-tick", 8, process, now.Add(4*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if next.Generation != 8 || !next.LastSuccess.IsZero() {
		t.Fatalf("a new generation cannot inherit old success: %+v", next)
	}
	beforeCompletion := componentFreshness(root, "steward-tick", RoleStewardRunner, 8, time.Minute, now.Add(5*time.Second), "repair", nil, "fresh")
	if beforeCompletion.Status != HealthDead || !strings.Contains(beforeCompletion.Reason, "no successful completion") {
		t.Fatalf("a live process and an attempt alone cannot satisfy freshness: %+v", beforeCompletion)
	}
	failed, err := completeComponentAttempt(root, "steward-tick", 8, next.AttemptSeq, ComponentError, "HEALTH_FAILED", "read failed", now.Add(5*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if failed.LastCompletion.IsZero() || !failed.LastSuccess.IsZero() {
		t.Fatalf("an ERROR completion advances only completion: %+v", failed)
	}
}

func TestHookEmissionAdvancesOnlyTheExactTurnSuccess(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 28, 11, 0, 0, 0, time.UTC)
	process := identity.Ref{Pid: 45001, StartedAtSec: 100, StartTicks: 99, BootID: "boot-a"}

	first, err := BeginHookAttempt(root, process, "turn-g", now)
	if err != nil {
		t.Fatal(err)
	}
	if role := checkHookFreshness(root, now.Add(time.Millisecond)); role.Status != HealthDead || !strings.Contains(role.Reason, "attempt") {
		t.Fatalf("an attempt without completion is service-dead: %+v", role)
	}
	if role := checkHookFreshnessAt(root, now.Add(time.Millisecond), true); role.Status == HealthDead {
		t.Fatalf("the current hook line must treat its own in-flight attempt as pending: %+v", role)
	}
	payload := `{"systemMessage":"HEALTH unknown — hook-freshness=unknown"}`
	line := "HEALTH unknown — hook-freshness=unknown"
	completed, err := CompleteHookAttempt(root, first.Generation, first.AttemptSeq,
		ComponentOK, "EMITTED", line, payload, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if completed.SuccessAttemptSeq != first.AttemptSeq || !completed.LastSuccess.Equal(now.Add(time.Second)) {
		t.Fatalf("OK/EMITTED must advance this attempt's success: %+v", completed)
	}
	if role := checkHookFreshness(root, now.Add(2*time.Second)); role.Status != HealthAlive {
		t.Fatalf("the completed turn must be healthy: %+v", role)
	}

	second, err := BeginHookAttempt(root, process, "turn-g-next", now.Add(3*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if role := checkHookFreshnessAt(root, now.Add(3500*time.Millisecond), true); role.Status != HealthAlive || !strings.Contains(role.Reason, "prior generation") {
		t.Fatalf("the current line must judge the previous completed turn: %+v", role)
	}
	failed, err := CompleteHookAttempt(root, second.Generation, second.AttemptSeq,
		ComponentError, "EMIT_FAILED", line, "write failed", now.Add(4*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if failed.LastSuccess.Equal(failed.LastCompletion) || failed.SuccessAttemptSeq == failed.AttemptSeq {
		t.Fatalf("an emission failure advances completion, not success: %+v", failed)
	}
	if role := checkHookFreshness(root, now.Add(5*time.Second)); role.Status != HealthDead {
		t.Fatalf("the failed current turn cannot inherit prior success: %+v", role)
	}
	retry, err := BeginHookAttempt(root, process, "turn-g-next", now.Add(6*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if retry.Generation != second.Generation || retry.AttemptSeq <= second.AttemptSeq {
		t.Fatalf("a retry stays in the turn generation and advances its attempt: first=%+v retry=%+v", second, retry)
	}
	if _, err := CompleteHookAttempt(root, retry.Generation, retry.AttemptSeq,
		ComponentOK, "DISPLAYED", line, payload, now.Add(7*time.Second)); err == nil {
		t.Fatal("the hook must never claim client display")
	}
	if _, err := CompleteHookAttempt(root, retry.Generation, retry.AttemptSeq,
		ComponentOK, "EMITTED", line, payload, now.Add(8*time.Second)); err != nil {
		t.Fatal(err)
	}
	if role := checkHookFreshness(root, now.Add(9*time.Second)); role.Status != HealthAlive {
		t.Fatalf("a successful retry restores hook health: %+v", role)
	}
}

func TestNextHookTurnRetainsInterruptedAttemptAsFailedHistory(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 28, 11, 30, 0, 0, time.UTC)
	process := identity.Ref{Pid: 45002, StartedAtSec: 101}
	interrupted, err := BeginHookAttempt(root, process, "turn-killed", now)
	if err != nil {
		t.Fatal(err)
	}
	next, err := BeginHookAttempt(root, process, "turn-after-kill", now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if next.Generation == interrupted.Generation || len(next.AttemptHistory) != 1 {
		t.Fatalf("the successor must retain exactly one terminal fact for the interrupted turn: before=%+v after=%+v", interrupted, next)
	}
	failed := next.AttemptHistory[0]
	if failed.Generation != interrupted.Generation || failed.AttemptSeq != interrupted.AttemptSeq || failed.Result != ComponentError || failed.Outcome != "INTERRUPTED_BY_NEXT_TURN" {
		t.Fatalf("the killed attempt was not closed as failed history: %+v", failed)
	}
	loaded, err := loadComponentEvidence(ComponentEvidencePath(root, "supervision-hook"))
	if err != nil || len(loaded.AttemptHistory) != 1 || loaded.AttemptHistory[0].Outcome != "INTERRUPTED_BY_NEXT_TURN" {
		t.Fatalf("the interrupted turn is not durable beside its successor: %+v %v", loaded, err)
	}
}

func TestHookPreviewDoesNotAdvanceTheTickOwnedObservation(t *testing.T) {
	root := t.TempDir()
	verdict := PreviewHealth(root, time.Now(), healthProbe{})
	if len(verdict.Roles) != len(healthRoleOrder) {
		t.Fatalf("hook preview omitted health roles: %+v", verdict.Roles)
	}
	if _, err := os.Stat(HealthRecordPath(root)); !os.IsNotExist(err) {
		t.Fatalf("hook preview must not advance the durable health breaker: %v", err)
	}
}

func TestFreshnessEqualityIsStale(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	window := 10 * time.Minute
	process := identity.Ref{Pid: 44003, StartedAtSec: 100}
	attempt, err := beginComponentAttempt(root, "narrator", 7, process, now.Add(-window-time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := completeComponentAttempt(root, "narrator", 7, attempt.AttemptSeq,
		ComponentOK, "EMITTED", "line", now.Add(-window)); err != nil {
		t.Fatal(err)
	}
	component := componentFreshness(root, "narrator", RoleNarratorFreshness, 7, window, now, "repair", nil, "fresh")
	if component.Status != HealthDead || !strings.Contains(component.Reason, "stale") {
		t.Fatalf("component age equal to its window is stale: %+v", component)
	}

	if err := os.MkdirAll(filepath.Join(root, "artifacts", "agents", "supervision"), 0o755); err != nil {
		t.Fatal(err)
	}
	census := `{"verdict":"SUCCESS","generation":7,"intervalSec":60,"lastSuccess":"` + now.Add(-2*time.Minute).Format(time.RFC3339Nano) + `"}`
	if err := os.WriteFile(filepath.Join(root, "artifacts", "agents", "supervision", "last-census.json"), []byte(census), 0o644); err != nil {
		t.Fatal(err)
	}
	censusRole := checkCensusFreshness(root, now, map[string]any{"generation": 7}, nil)
	if censusRole.Status != HealthDead || !strings.Contains(censusRole.Reason, "stale") {
		t.Fatalf("census age equal to two recorded producer intervals is stale: %+v", censusRole)
	}
	census = `{"verdict":"SUCCESS","generation":7,"intervalSec":60,"lastSuccess":"` + now.Add(-2*time.Minute+time.Second).Format(time.RFC3339Nano) + `"}`
	if err := os.WriteFile(filepath.Join(root, "artifacts", "agents", "supervision", "last-census.json"), []byte(census), 0o644); err != nil {
		t.Fatal(err)
	}
	censusRole = checkCensusFreshness(root, now, map[string]any{"generation": 7}, nil)
	if censusRole.Status != HealthAlive {
		t.Fatalf("census age below two recorded producer intervals is fresh: %+v", censusRole)
	}
}

func TestRunnerSuccessMustBelongToTheResidentIdentity(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	resident := identity.Ref{Pid: 44100, StartedAtSec: 100, StartTicks: 900, BootID: "boot-a"}
	manual := identity.Ref{Pid: 44101, StartedAtSec: 101, StartTicks: 901, BootID: "boot-a"}
	if err := os.MkdirAll(filepath.Dir(RepoIdentityPath(root)), 0o755); err != nil {
		t.Fatal(err)
	}
	repoIdentity, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := MintIdentity(RepoIdentityPath(root), InstallIdentity{
		RepoIdentity: repoIdentity, Generation: 3, InstallPath: "/fixture/metasystem", MintedAt: now.Format(time.RFC3339),
	}); err != nil {
		t.Fatal(err)
	}
	if err := writeJSONAtomic(runnerRecordPath(root), RunnerRecord{
		Pid: resident.Pid, PidStartedAt: resident.StartedAtSec, StartTicks: resident.StartTicks, BootID: resident.BootID,
	}); err != nil {
		t.Fatal(err)
	}
	attempt, err := beginComponentAttempt(root, "steward-tick", 3, manual, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := completeComponentAttempt(root, "steward-tick", 3, attempt.AttemptSeq, ComponentOK, "PASS_COMPLETE", "manual", now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	probe := healthProbe{resident.Pid: {
		exact: identity.Exact{Pid: resident.Pid, StartedAt: time.Unix(resident.StartedAtSec, 0), StartTicks: resident.StartTicks, BootID: resident.BootID},
		state: identity.Alive,
	}}
	wrongWriter := checkStewardRunner(root, now.Add(2*time.Second), probe)
	if wrongWriter.Status != HealthDead || !strings.Contains(wrongWriter.Reason, "not resident runner") {
		t.Fatalf("a manual tick cannot satisfy resident-runner freshness: %+v", wrongWriter)
	}
	residentAttempt, err := beginComponentAttempt(root, "steward-tick", 3, resident, now.Add(3*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	stillManual := checkStewardRunner(root, now.Add(4*time.Second), probe)
	if stillManual.Status != HealthDead {
		t.Fatalf("a resident attempt cannot adopt the manual tick's prior success: %+v", stillManual)
	}
	if _, err := completeComponentAttempt(root, "steward-tick", 3, residentAttempt.AttemptSeq, ComponentOK, "PASS_COMPLETE", "resident", now.Add(5*time.Second)); err != nil {
		t.Fatal(err)
	}
	current := checkStewardRunner(root, now.Add(6*time.Second), probe)
	if current.Status != HealthAlive {
		t.Fatalf("the resident runner's own completion satisfies freshness: %+v", current)
	}
}

func TestOKCompletionRemainsPendingWhenPromotionDurabilityIsUnknown(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	process := identity.Ref{Pid: 44200, StartedAtSec: 100}
	first, err := beginComponentAttempt(root, "narrator", 1, process, now)
	if err != nil {
		t.Fatal(err)
	}
	firstOK, err := completeComponentAttempt(root, "narrator", 1, first.AttemptSeq, ComponentOK, "EMITTED", "first", now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	second, err := beginComponentAttempt(root, "narrator", 1, process, now.Add(2*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	originalWriter := componentEvidenceWriter
	defer func() { componentEvidenceWriter = originalWriter }()
	writes := 0
	componentEvidenceWriter = func(path, text, anchor string) (bool, error) {
		writes++
		durable, writeErr := originalWriter(path, text, anchor)
		if writes == 2 && writeErr == nil {
			return false, nil
		}
		return durable, writeErr
	}
	if _, err := completeComponentAttempt(root, "narrator", 1, second.AttemptSeq, ComponentOK, "EMITTED", "second", now.Add(3*time.Second)); err == nil {
		t.Fatal("unknown promotion durability must fail loudly")
	}
	record, err := loadComponentEvidence(ComponentEvidencePath(root, "narrator"))
	if err != nil {
		t.Fatal(err)
	}
	if record.Result == ComponentOK || record.Outcome != "DURABILITY_PENDING" || !record.LastSuccess.Equal(firstOK.LastSuccess) {
		t.Fatalf("the visible completion must remain pending and preserve the prior success: %+v", record)
	}
	role := componentFreshness(root, "narrator", RoleNarratorFreshness, 1, time.Minute, now.Add(4*time.Second), "repair", nil, "fresh")
	if role.Status != HealthUnknown || !strings.Contains(role.Reason, "durability") {
		t.Fatalf("health cannot accept a durability-pending completion: %+v", role)
	}
}

func TestForwardClockMovementExpiresComponentFreshness(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	process := identity.Ref{Pid: 44300, StartedAtSec: 100}
	attempt, err := beginComponentAttempt(root, "narrator", 1, process, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := completeComponentAttempt(root, "narrator", 1, attempt.AttemptSeq, ComponentOK, "EMITTED", "line", now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	role := componentFreshness(root, "narrator", RoleNarratorFreshness, 1, 2*time.Minute, now.Add(10*time.Minute), "repair", nil, "fresh")
	if role.Status != HealthDead || !strings.Contains(role.Reason, "stale") {
		t.Fatalf("a forward clock movement expires freshness immediately: %+v", role)
	}
}
