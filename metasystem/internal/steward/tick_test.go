package steward

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

type fakeCensus struct {
	workers Workers
	err     error
}

func (f fakeCensus) Workers(string) (Workers, error) { return f.workers, f.err }

func TestKilledWatcherIsRoutedToItsOwnerWithinOneTick(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "artifacts", "agents", "supervision")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	state := map[string]any{
		"generation": 4,
		"components": map[string]any{
			"watcher": map[string]any{"pid": 44001, "pidStartedAt": 100, "instanceTag": "owner-watcher-4"},
		},
	}
	data, _ := json.Marshal(state)
	if err := os.WriteFile(filepath.Join(directory, "state.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	health := HealthVerdict{Roles: []RoleVerdict{{
		Role: RoleRepoWatcher, Status: HealthDead, Reason: "recorded pid 44001 is dead",
	}}}
	requestedAt := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	ended := health
	ended.Roles = append([]RoleVerdict(nil), health.Roles...)
	ended.Roles[0].FailureEscalation = AutoHealEnded
	if err := requestWatcherRepair(root, health, requestedAt); err != nil {
		t.Fatal(err)
	}
	requestData, err := os.ReadFile(filepath.Join(directory, "watcher-restart-request.json"))
	if err != nil {
		t.Fatal(err)
	}
	var request struct {
		Generation  int64     `json:"generation"`
		Pid         int64     `json:"pid"`
		Completed   bool      `json:"completed"`
		RequestedAt time.Time `json:"requestedAt"`
	}
	if err := json.Unmarshal(requestData, &request); err != nil {
		t.Fatal(err)
	}
	if request.Generation != 4 || request.Pid != 44001 || request.Completed || !request.RequestedAt.Equal(requestedAt) {
		t.Fatalf("the tick must request only the exact enrolled watcher generation: %+v", request)
	}
	if err := requestWatcherRepair(root, ended, requestedAt.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	requestData, err = os.ReadFile(filepath.Join(directory, "watcher-restart-request.json"))
	if err != nil || json.Unmarshal(requestData, &request) != nil || !request.Completed {
		t.Fatalf("failure five must retire an earlier pending watcher repair: %+v %v", request, err)
	}
}

func scanBreachStopsInRoleHealthBed(t *testing.T, bed *roleHealthProjectionBed) []BreachStopReport {
	t.Helper()
	reads := 0
	repository := roleHealthRepository{bed: bed}
	reports := runBreachStopCustodianWithScanner(bed.root, bed.now,
		func(root string, now time.Time) ([]dispatch.StopRoute, error) {
			if root != bed.root || !now.Equal(bed.now) {
				t.Fatalf("stop scan root/time = %q/%s, want %q/%s", root, now, bed.root, bed.now)
			}
			return dispatch.FindBreachStopsWithRawReads(root, now,
				func(got string) bool {
					if got != bed.root {
						t.Fatalf("accepted-tree read root = %q, want %q", got, bed.root)
					}
					reads++
					_, accepted, err := repository.Accepted()
					if err != nil {
						t.Fatal(err)
					}
					return accepted
				},
				func(got string) (goal.Endpoint, error) {
					if got != bed.root {
						t.Fatalf("endpoint read root = %q, want %q", got, bed.root)
					}
					reads++
					return goal.Endpoint{Root: bed.root, Remote: "local", Branch: goal.LocalLedgerBranch, Repository: repository}, nil
				})
		})
	if reads != 2 {
		t.Fatalf("stop scan raw reads = %d, want 2", reads)
	}
	return reports
}

func TestBreachStopCustodianReportsIndeterminateFailureAndCommandOutcome(t *testing.T) {
	now := time.Date(2026, 8, 29, 13, 0, 0, 0, time.UTC)
	file := structuredHealthGoal()
	file.StopCapability = &goal.StopCapability{Generation: 2, Revision: 2, Machine: "bed-m1", ClaimEpoch: 7, FenceEpoch: 1}
	file.StopFence = &goal.StopFence{
		StopID: "stop-bounded-goal-r2-f1", Revision: 2, Epoch: 1, CapabilityGeneration: 2,
		ClosedAt: now.Add(-time.Minute).Format(time.RFC3339), Reason: goal.StopReasonElapsedLimit,
	}
	bed := newRoleHealthProjectionBed(t, now, map[string]*goal.GoalFile{"bounded-goal": file}, nil)
	stamp := now.Format(time.RFC3339)
	batch := goal.StopBatch{
		StopID: file.StopFence.StopID, GoalID: file.Id, GoalRevision: 2, FenceEpoch: 1,
		CapabilityGeneration: 2, Machine: "bed-m1", ClaimEpoch: 7, Reason: goal.StopReasonElapsedLimit,
		State: goal.StopBatchIndeterminate, Failure: "custody cannot be proven", OpenedAt: stamp, UpdatedAt: stamp, Pass: 1,
	}
	if err := goal.WriteStopBatch(bed.root, batch); err != nil {
		t.Fatal(err)
	}
	reports := scanBreachStopsInRoleHealthBed(t, bed)
	if len(reports) != 1 || reports[0].State != "INDETERMINATE" || reports[0].Detail != batch.Failure {
		t.Fatalf("indeterminate stop was not routed to escalation: %+v", reports)
	}

	commandBed := newRoleHealthProjectionBed(t, now, map[string]*goal.GoalFile{"bounded-goal": file}, nil)
	batch.State = goal.StopBatchOpen
	batch.Failure = ""
	if err := goal.WriteStopBatch(commandBed.root, batch); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(commandBed.root, "scripts", "agents", "dispatch.sh")
	if err := os.MkdirAll(filepath.Dir(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(script, []byte("#!/bin/sh\nprintf 'stop failed\\n'\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	reports = scanBreachStopsInRoleHealthBed(t, commandBed)
	if len(reports) != 1 || reports[0].State != "FAILED" || reports[0].Detail != "stop failed" {
		t.Fatalf("failed stop command was not reported: %+v", reports)
	}
	if err := testexec.WriteFile(script, []byte("#!/bin/sh\nprintf 'stop complete\\n'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	reports = scanBreachStopsInRoleHealthBed(t, commandBed)
	if len(reports) != 1 || reports[0].State != "COMPLETE" || reports[0].Detail != "stop complete" {
		t.Fatalf("successful stop command was not reported: %+v", reports)
	}
}

func TestCorruptGraceTickEscalatesWithoutCancellingLawfulWork(t *testing.T) {
	now := time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC)
	file := structuredHealthGoal()
	original := goal.RenderFile(file)
	bed := newRoleHealthProjectionBed(t, now, map[string]*goal.GoalFile{"bounded-goal": file}, nil)
	if err := os.WriteFile(filepath.Join(bed.root, "metasystem.conf"),
		[]byte("metasystem.budget.elapsed-grace-percent=broken\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	reports := scanBreachStopsInRoleHealthBed(t, bed)
	if len(reports) != 1 || reports[0].State != "INDETERMINATE" ||
		!strings.Contains(reports[0].Detail, "BUDGET_UNKNOWN") {
		t.Fatalf("the next stop scan did not surface corrupt grace: %+v", reports)
	}
	projection, err := bed.project()
	if err != nil {
		t.Fatal(err)
	}
	role := checkClaimedGoalBudgetsFromProjection(bed.root, now, projection, true, nil, nil)
	verdict := applyHealthObservation(bed.root, HealthObservationState{}, []RoleVerdict{role}, now)
	if role.Status != HealthDead || !role.NoAutomaticRemedy || !verdict.ShouldAlert ||
		verdict.Roles[0].FailureEscalation != NoLawfulRemedy {
		t.Fatalf("corrupt grace did not escalate immediately under Ruling L: role=%+v verdict=%+v", role, verdict)
	}
	fresh, err := bed.project()
	if err != nil {
		t.Fatal(err)
	}
	goalFile := fresh.Tree.Live["bounded-goal"]
	if goalFile == nil || goalFile.StopFence != nil {
		t.Fatalf("indeterminate grace fenced or cancelled lawful work: %+v", goalFile)
	}
	actual, err := os.ReadFile(filepath.Join(bed.root, "plans", "goals", "bounded-goal.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(actual, original) {
		t.Fatal("indeterminate grace changed the accepted goal file")
	}
}

func TestDegradedTickQueuesTheIncidentOrSurfacesQueueFailure(t *testing.T) {
	root := t.TempDir()
	result, err := degradedTick(root, "evidence store is torn")
	if err != nil || result.Decision.Verdict != VerdictDegraded || result.Decision.Action != ActNotify {
		t.Fatalf("degraded tick did not return its notify verdict: result=%+v err=%v", result, err)
	}
	pending, err := PendingNotifications(root)
	if err != nil || len(pending) != 1 || !strings.Contains(pending[0].Message, "evidence store is torn") {
		t.Fatalf("degraded incident was not queued: pending=%+v err=%v", pending, err)
	}
	blocked := filepath.Join(t.TempDir(), "blocked")
	if err := os.WriteFile(blocked, []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err = degradedTick(blocked, "repository is unreadable")
	if err == nil || result.Decision.Verdict != VerdictDegraded || !strings.Contains(err.Error(), "could not queue") {
		t.Fatalf("queue failure hid the degraded verdict: result=%+v err=%v", result, err)
	}
}

func TestQuietTicksAgeIntoLiveIdleNotification(t *testing.T) {
	bed := newDecisionTickRepository(t)
	census := fakeCensus{workers: Workers{Live: 1, CensusComplete: true}}
	r := bed.tickN(TickConfig{StaleTicks: 3}, census, 3)
	if r.Decision.Verdict != VerdictHealthy {
		t.Fatalf("inside the threshold a live worker is healthy: %+v", r.Decision)
	}
	r = bed.tickN(TickConfig{StaleTicks: 3}, census, 1)
	if r.Decision.Verdict != VerdictStalledIdle || r.Decision.Action != ActNotify {
		t.Fatalf("past the threshold a live-idle worker is notified, never displaced: %+v", r.Decision)
	}
}

func TestCommitResetsTheAging(t *testing.T) {
	bed := newDecisionTickRepository(t)
	census := fakeCensus{workers: Workers{Live: 1, CensusComplete: true}}
	bed.tickN(TickConfig{StaleTicks: 2}, census, 2)
	bed.declareProgress()
	r := bed.tickN(TickConfig{StaleTicks: 2}, census, 1)
	if r.Evidence.TicksSinceAdvance != 0 || r.Decision.Verdict != VerdictHealthy {
		t.Fatalf("a commit is progress: %+v", r)
	}
}

func TestProvenDeathRevivesRegardlessOfFreshEvidence(t *testing.T) {
	bed := newDecisionTickRepository(t)
	census := fakeCensus{workers: Workers{CensusComplete: true}}
	r := bed.tickN(TickConfig{}, census, 1)
	if r.Decision.Verdict != VerdictStalledDead || r.Decision.Action != ActRevive {
		t.Fatalf("dead seconds after a fresh commit is still dead: %+v", r.Decision)
	}
}

func TestUnreadableCensusNeverSpawns(t *testing.T) {
	bed := newDecisionTickRepository(t)
	census := fakeCensus{err: os.ErrPermission}
	r := bed.tickN(TickConfig{}, census, 1)
	if r.Decision.Verdict != VerdictUnknown || r.Decision.Action != ActNotify {
		t.Fatalf("an unreadable census cannot prove death: %+v", r.Decision)
	}
}

func TestOpenIntentSuppressesASecondRevival(t *testing.T) {
	bed := newDecisionTickRepository(t)
	it := testIntent("live-one")
	if err := MintIntent(bed.root, it); err != nil {
		t.Fatal(err)
	}
	census := fakeCensus{workers: Workers{CensusComplete: true}}
	r := bed.tickN(TickConfig{}, census, 1)
	if r.Decision.Action != ActNotify {
		t.Fatalf("an open continuation suppresses dispatch: %+v", r.Decision)
	}
}

func TestGoalFreeRepositoryTicksQuietly(t *testing.T) {
	bed := newDecisionTickRepository(t)
	bed.declareGoalFree()
	r := bed.tickN(TickConfig{}, fakeCensus{}, 1)
	if r.Decision.Verdict != VerdictNoWork || r.Decision.Action != ActNone {
		t.Fatalf("goal-free needs nothing: %+v", r.Decision)
	}
}

func TestNotifyVerdictsReachTheQueue(t *testing.T) {
	bed := newDecisionTickRepository(t)
	live := fakeCensus{workers: Workers{Live: 1, CensusComplete: true}}
	bed.tickN(TickConfig{StaleTicks: 1}, live, 2) // ages past the threshold
	pending, err := PendingNotifications(bed.root)
	if err != nil || len(pending) == 0 {
		t.Fatalf("a live-idle verdict is an incident the operator hears about: %v %v", pending, err)
	}
	found := false
	for _, n := range pending {
		if strings.Contains(n.Message, "stalled-idle") {
			found = true
		}
	}
	if !found {
		t.Fatalf("the incident names its verdict: %v", pending)
	}
	// The standing condition holds ONE pending message per verdict.
	before := len(pending)
	bed.tickN(TickConfig{StaleTicks: 1}, live, 3)
	after, _ := PendingNotifications(bed.root)
	if len(after) != before {
		t.Fatalf("a repeating verdict overwrites its one message: %d -> %d", before, len(after))
	}
}

func TestRevivalPreparationStaysSilentAndLaunches(t *testing.T) {
	t.Parallel()
	revival := newRevivalFixture(t, 1, 1)
	root := revival.root
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.steward.notify-command=exit 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := PrepareIntent(root, filepath.Join(root, "memory", "receipts.log"), testIntent("dg-1")); err != nil {
		t.Fatal(err)
	}
	if pending, err := PendingNotifications(root); err != nil || len(pending) != 0 {
		t.Fatalf("a prepared automatic repair must remain silent: %v %v", pending, err)
	}
	launched := 0
	out, err := revival.complete(TickConfig{}, deadCensus(), "dg-1", func(Intent) error { launched++; return nil })
	if err != nil || !out.Launched || launched != 1 {
		t.Fatalf("the gate-complete intent launches exactly once: %+v %d %v", out, launched, err)
	}
}

func TestReviveVerdictStaysSilentBeforeHealing(t *testing.T) {
	bed := newDecisionTickRepository(t)
	census := fakeCensus{workers: Workers{CensusComplete: true}}
	r := bed.tickN(TickConfig{}, census, 1)
	if r.Decision.Action != ActRevive {
		t.Fatalf("this world revives: %+v", r.Decision)
	}
	pending, err := PendingNotifications(bed.root)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range pending {
		if strings.Contains(n.Message, "stalled-dead") {
			t.Fatalf("a recoverable revive verdict alerted before healing: %v", pending)
		}
	}
}
