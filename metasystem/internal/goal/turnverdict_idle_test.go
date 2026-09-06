package goal

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

func budgetedQueuedGoal(id, opened string) *GoalFile {
	budget := testBudget()
	return &GoalFile{
		Id: id, State: StateApproved, Intent: "Claim the shared work", Origin: OriginMain,
		NextStep: "Claim and dispatch it.", OpenedAt: opened, Revision: 1,
		Budget: &budget,
	}
}

type idleFixtureProber map[int64]identity.Exact

func (p idleFixtureProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	if exact, ok := p[pid]; ok {
		return exact, identity.Alive, nil
	}
	return identity.Exact{}, identity.Dead, nil
}

func writeIdleJSON(t *testing.T, path string, value any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func installIdleLiveClaim(t *testing.T, root, lineage string) identity.Prober {
	t.Helper()
	writeIdleJSON(t, filepath.Join(root, "artifacts", "agents", "mains", "main-activity.json"), map[string]any{
		"mainId": "main-activity", "ownerLineage": lineage,
		"pid": 41, "pidStartedAt": 100,
	})
	return idleFixtureProber{41: {Pid: 41, StartedAt: time.Unix(100, 0)}}
}

func TestIdleBacklogBlocksTwiceThenDefersClaimAndPreparesStewardContinuation(t *testing.T) {
	root := servingBed(t, "bed-m1", map[string]*GoalFile{
		"waiting": budgetedQueuedGoal("waiting", "2026-08-23T00:00:00Z"),
	})
	if !NewWorld(root) {
		t.Fatal("serving bed did not create a converted world")
	}
	var prepared, recorded IdleEscalationEvent
	alarmRaised := false
	store := &Store{
		Root: root,
		ClaimIdleGoal: func(IdleEscalationEvent) error {
			t.Fatal("TurnVerdict crossed the external goal Claim boundary")
			return nil
		},
		PrepareIdleContinuation: func(event IdleEscalationEvent) (string, error) {
			prepared = event
			return "intent-seat-idle", nil
		},
		RecordIdleIncident: func(event IdleEscalationEvent) (string, error) {
			recorded = event
			return "alert-seat-idle", nil
		},
		RaiseIdleAlarm: func(IdleEscalationEvent) error {
			alarmRaised = true
			return nil
		},
	}
	options := TurnVerdictOptions{
		SeatActor: Actor{Machine: "bed-m1", Lineage: "seat-lineage"}, SeatClaimEpoch: 7,
	}
	for stop := 1; stop <= 2; stop++ {
		verdict, err := store.TurnVerdict(ScanResult{}, "same-session", "", "main-1", options)
		if err != nil || !verdict.ShouldBlock || !verdict.IdleRefusal || verdict.BlockSource == nil || *verdict.BlockSource != "idle-backlog" ||
			!strings.Contains(verdict.Display, "waiting") ||
			!strings.Contains(verdict.Display, fmt.Sprintf("refusal %d of 3 for this unchanged backlog", stop)) ||
			!strings.Contains(verdict.Display, "at 3 the steward claims if needed and continues the seat's goal") {
			t.Fatalf("unchanged stop %d must block on claimable backlog: %+v %v", stop, verdict, err)
		}
	}
	options.StopHookActive = true
	verdict, err := store.TurnVerdict(ScanResult{}, "same-session", "", "main-1", options)
	if err != nil || verdict.ShouldBlock || verdict.BlockSource != nil {
		t.Fatalf("the third unchanged stop must end after steward handoff: %+v %v", verdict, err)
	}
	for _, text := range []string{
		"selected goal waiting and deferred its claim as bed-m1+seat-lineage to the steward tick",
		"prepared steward continuation intent intent-seat-idle",
		"recorded steward alert episode alert-seat-idle",
		"stop_hook_active=true",
	} {
		if !strings.Contains(verdict.Display, text) {
			t.Fatalf("third-stop display did not name %q: %s", text, verdict.Display)
		}
	}
	if !prepared.ClaimNeeded || prepared.ClaimMade || prepared.GoalID != "waiting" ||
		prepared.ClaimActor != options.SeatActor || prepared.SeatClaimEpoch != 7 {
		t.Fatalf("continuation preparation did not receive the deferred seat claim: %+v", prepared)
	}
	if !recorded.IntentPrepared || recorded.IntentID != "intent-seat-idle" || !recorded.StopHookActive {
		t.Fatalf("incident did not receive the completed handoff and harness flag: %+v", recorded)
	}
	if alarmRaised {
		t.Fatal("a successful steward handoff raised the human idle alarm")
	}
	endpoint, err := ResolveEndpoint(root)
	if err != nil {
		t.Fatal(err)
	}
	projection, err := Project(endpoint, false, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if claim := projection.Tree.Live["waiting"].Claimed; claim != nil {
		t.Fatalf("TurnVerdict performed the steward's deferred claim: %+v", claim)
	}
}

func TestIdleBacklogContinuesThisMachinesHeldClaimWithoutAnotherClaim(t *testing.T) {
	held := budgetedQueuedGoal("held", "2026-08-22T00:00:00Z")
	held.State = StateClaimed
	held.Revision = 2
	held.Claimed = &ClaimRecord{
		Machine: "bed-m1", Lineage: "seat-lineage", At: "2026-08-23T01:00:00Z", Revision: 2,
	}
	root := servingBed(t, "bed-m1", map[string]*GoalFile{
		"held":    held,
		"waiting": budgetedQueuedGoal("waiting", "2026-08-23T00:00:00Z"),
	})
	var prepared IdleEscalationEvent
	store := &Store{
		Root: root,
		ClaimIdleGoal: func(IdleEscalationEvent) error {
			t.Fatal("TurnVerdict tried to claim while this machine already held a goal")
			return nil
		},
		PrepareIdleContinuation: func(event IdleEscalationEvent) (string, error) {
			prepared = event
			return "intent-held", nil
		},
		RecordIdleIncident: func(IdleEscalationEvent) (string, error) { return "alert-held", nil },
	}
	options := TurnVerdictOptions{
		SeatActor: Actor{Machine: "bed-m1", Lineage: "seat-lineage"}, SeatClaimEpoch: 7,
	}
	for stop := 1; stop <= 3; stop++ {
		verdict, err := store.TurnVerdict(ScanResult{}, "held-session", "", "main-1", options)
		if err != nil {
			t.Fatal(err)
		}
		if stop < 3 && !verdict.ShouldBlock {
			t.Fatalf("pre-bound held-claim stop %d did not block: %+v", stop, verdict)
		}
		if stop == 3 && (verdict.ShouldBlock ||
			!strings.Contains(verdict.Display, "selected this machine's held goal held") ||
			!strings.Contains(verdict.Display, "without another claim attempt")) {
			t.Fatalf("the held goal was not handed to the steward at the bound: %+v", verdict)
		}
	}
	if prepared.GoalID != "held" || prepared.ClaimNeeded || prepared.SeatClaimEpoch != 7 {
		t.Fatalf("the intent did not preserve the held-claim invariant: %+v", prepared)
	}
}

func TestMismatchedSessionStopMarkerDoesNotGateIdleEscalation(t *testing.T) {
	now := time.Date(2026, 9, 2, 10, 1, 0, 0, time.UTC)
	root := servingBed(t, "bed-m1", map[string]*GoalFile{
		"waiting": budgetedQueuedGoal("waiting", "2026-08-23T00:00:00Z"),
	})
	prepared := 0
	store := &Store{
		Root: root, Now: func() time.Time { return now },
		Prober: idleFixtureProber{41: {Pid: 41, StartedAt: time.Unix(100, 0)}},
		PrepareIdleContinuation: func(IdleEscalationEvent) (string, error) {
			prepared++
			return "intent-mismatch", nil
		},
		RecordIdleIncident: func(IdleEscalationEvent) (string, error) { return "alert-mismatch", nil },
	}
	sessionStopFixture(t, store, "marker-mismatch", "other-main", 7)
	options := TurnVerdictOptions{
		SeatActor: Actor{Machine: "bed-m1", Lineage: "seat-lineage"}, SeatClaimEpoch: 7,
	}
	var third Verdict
	for stop := 1; stop <= 3; stop++ {
		var err error
		third, err = store.TurnVerdict(ScanResult{}, "marker-mismatch", "", "main-1", options)
		if err != nil {
			t.Fatal(err)
		}
	}
	if third.ShouldBlock || prepared != 1 ||
		!strings.Contains(third.Display, "SESSION STOP authorization") ||
		!strings.Contains(third.Display, "prepared steward continuation intent intent-mismatch") {
		t.Fatalf("a mismatched display-only marker gated the bounded escalation: %+v prepared=%d", third, prepared)
	}
}

func TestIdleEscalationPreservesAnIndependentOpenWorkBlock(t *testing.T) {
	root := servingBed(t, "bed-m1", map[string]*GoalFile{
		"waiting": budgetedQueuedGoal("waiting", "2026-08-23T00:00:00Z"),
	})
	var prepared, incident IdleEscalationEvent
	store := &Store{
		Root: root,
		PrepareIdleContinuation: func(event IdleEscalationEvent) (string, error) {
			prepared = event
			return "intent-open-work", nil
		},
		RecordIdleIncident: func(event IdleEscalationEvent) (string, error) {
			incident = event
			return "alert-open-work", nil
		},
	}
	options := TurnVerdictOptions{
		SeatActor: Actor{Machine: "bed-m1", Lineage: "seat-lineage"}, SeatClaimEpoch: 7,
	}
	for stop := 1; stop <= 2; stop++ {
		if verdict, err := store.TurnVerdict(ScanResult{}, "open-work-bound", "", "main-1", options); err != nil || !verdict.ShouldBlock {
			t.Fatalf("pre-bound stop %d did not block: %+v %v", stop, verdict, err)
		}
	}
	scan := ScanResult{Open: []Item{{Kind: "plan", Id: "unfinished", Detail: "an unfinished implementation remains"}}}
	third, err := store.TurnVerdict(scan, "open-work-bound", "", "main-1", options)
	if err != nil || !third.ShouldBlock || third.BlockSource == nil || *third.BlockSource != "open-work" || third.IdleRefusal ||
		!strings.Contains(third.Display, "OPEN WORK") ||
		!strings.Contains(third.Display, "another turn-verdict branch remains blocking") {
		t.Fatalf("the local escalation swallowed an independent open-work block: %+v %v", third, err)
	}
	if prepared.GoalID != "waiting" || !prepared.ClaimNeeded {
		t.Fatalf("the open-work block suppressed the local intent write: %+v", prepared)
	}
	if incident.IntentID != "intent-open-work" || !incident.IntentPrepared {
		t.Fatalf("the open-work block suppressed the incident write: %+v", incident)
	}
}

func TestIdleBacklogDigestChangeResetsTheRefusalCount(t *testing.T) {
	root := servingBed(t, "bed-m1", map[string]*GoalFile{
		"waiting": budgetedQueuedGoal("waiting", "2026-08-23T00:00:00Z"),
	})
	store := &Store{Root: root}
	options := TurnVerdictOptions{SeatActor: Actor{Machine: "bed-m1", Lineage: "seat-lineage"}, SeatClaimEpoch: 7}
	first, err := store.TurnVerdict(ScanResult{}, "digest-reset", "", "main-1", options)
	if err != nil || !first.ShouldBlock || !strings.Contains(first.Display, "refusal 1 of 3") {
		t.Fatalf("first backlog refusal was not counted: %+v %v", first, err)
	}
	writeIdleJSON(t, filepath.Join(root, "artifacts", "agents", "jobs", "new-reservation.json"), map[string]any{
		"jobId": "new-reservation", "status": "pending", "pid": 99999999, "pidStartedAt": 1,
	})
	second, err := store.TurnVerdict(ScanResult{}, "digest-reset", "", "main-1", options)
	if err != nil || !second.ShouldBlock || !strings.Contains(second.Display, "refusal 1 of 3") {
		t.Fatalf("a changed non-terminal job set did not reset the count: %+v %v", second, err)
	}
}

func TestIdleBacklogFailedIntentStillEndsAndNamesTheFailure(t *testing.T) {
	waiting := budgetedQueuedGoal("waiting", "2026-08-23T00:00:00Z")
	waiting.Pinned = "bed-m1"
	root := servingBed(t, "bed-m1", map[string]*GoalFile{"waiting": waiting})
	prepared := false
	alarmRaised := false
	store := &Store{
		Root: root,
		PrepareIdleContinuation: func(IdleEscalationEvent) (string, error) {
			prepared = true
			return "", errors.New("intent store unavailable")
		},
		RecordIdleIncident: func(IdleEscalationEvent) (string, error) { return "alert-refused", nil },
		RaiseIdleAlarm: func(IdleEscalationEvent) error {
			alarmRaised = true
			return nil
		},
	}
	options := TurnVerdictOptions{SeatActor: Actor{Machine: "bed-m2", Lineage: "seat-lineage"}, SeatClaimEpoch: 7}
	for stop := 1; stop <= 2; stop++ {
		if verdict, err := store.TurnVerdict(ScanResult{}, "claim-refused", "", "main-1", options); err != nil || !verdict.ShouldBlock {
			t.Fatalf("pre-bound stop %d did not block: %+v %v", stop, verdict, err)
		}
	}
	verdict, err := store.TurnVerdict(ScanResult{}, "claim-refused", "", "main-1", options)
	if err != nil || verdict.ShouldBlock || verdict.BlockSource != nil {
		t.Fatalf("a refused claim must still end the turn: %+v %v", verdict, err)
	}
	if !strings.Contains(verdict.Display, "could not prepare a steward continuation") ||
		!strings.Contains(verdict.Display, "intent store unavailable") ||
		!strings.Contains(verdict.Display, "queued the steward's existing human idle alarm") {
		t.Fatalf("the refused intent was not explained: %s", verdict.Display)
	}
	if !prepared {
		t.Fatal("the bounded refusal did not attempt a local continuation intent")
	}
	if !alarmRaised {
		t.Fatal("a refused intent did not raise the steward's human idle alarm")
	}
}

func TestStopHookActivePreservesTheIdleCountWhenDigestReadIsUnavailable(t *testing.T) {
	root := servingBed(t, "bed-m1", map[string]*GoalFile{
		"waiting": budgetedQueuedGoal("waiting", "2026-08-23T00:00:00Z"),
	})
	originalFetch := fetchForProjection
	t.Cleanup(func() { fetchForProjection = originalFetch })
	var incident IdleEscalationEvent
	store := &Store{
		Root: root,
		RecordIdleIncident: func(event IdleEscalationEvent) (string, error) {
			incident = event
			return "alert-unavailable", nil
		},
	}
	options := TurnVerdictOptions{SeatActor: Actor{Machine: "bed-m1", Lineage: "seat-lineage"}, SeatClaimEpoch: 7}
	first, err := store.TurnVerdict(ScanResult{}, "unavailable-repeat", "", "main-1", options)
	if err != nil || !first.ShouldBlock || !strings.Contains(first.Display, "refusal 1 of 3") {
		t.Fatalf("the initial readable refusal was not stored: %+v %v", first, err)
	}
	fetchForProjection = func(Endpoint) (AdvanceResult, error) {
		return AdvanceResult{}, errors.New("canonical ledger unavailable")
	}
	options.StopHookActive = true
	second, err := store.TurnVerdict(ScanResult{}, "unavailable-repeat", "", "main-1", options)
	if err != nil || !second.ShouldBlock || !strings.Contains(second.Display, "refusal 2 of 3") ||
		!strings.Contains(second.Display, "stop_hook_active=true") {
		t.Fatalf("the repeated hook did not preserve and increment uncertainty: %+v %v", second, err)
	}
	third, err := store.TurnVerdict(ScanResult{}, "unavailable-repeat", "", "main-1", options)
	if err != nil || third.ShouldBlock || third.BlockSource != nil ||
		!strings.Contains(third.Display, "could not identify a continuation goal") ||
		!strings.Contains(third.Display, "the turn will end") {
		t.Fatalf("the third uncertain repeat recreated the refusal loop: %+v %v", third, err)
	}
	if incident.Refusal != 3 || !incident.StopHookActive || incident.BacklogDigest != unreadableIdleBacklogDigest {
		t.Fatalf("the uncertainty incident lost the stored repeat evidence: %+v", incident)
	}
}

func TestThreeUnreadableLedgerStopsRecordIncidentRaiseAlarmAndEnd(t *testing.T) {
	root := servingBed(t, "bed-m1", map[string]*GoalFile{
		"waiting": budgetedQueuedGoal("waiting", "2026-08-23T00:00:00Z"),
	})
	originalFetch := fetchForProjection
	t.Cleanup(func() { fetchForProjection = originalFetch })
	fetchForProjection = func(Endpoint) (AdvanceResult, error) {
		return AdvanceResult{}, errors.New("canonical ledger unreadable")
	}
	var incident IdleEscalationEvent
	prepared := false
	alarmRaised := false
	store := &Store{
		Root: root,
		PrepareIdleContinuation: func(IdleEscalationEvent) (string, error) {
			prepared = true
			return "unexpected", nil
		},
		RecordIdleIncident: func(event IdleEscalationEvent) (string, error) {
			incident = event
			return "alert-unreadable", nil
		},
		RaiseIdleAlarm: func(IdleEscalationEvent) error {
			alarmRaised = true
			return nil
		},
	}
	options := TurnVerdictOptions{}
	for stop := 1; stop <= 3; stop++ {
		options.StopHookActive = stop > 1
		verdict, err := store.TurnVerdict(ScanResult{}, "three-unreadable", "", "main-1", options)
		if err != nil {
			t.Fatal(err)
		}
		if stop < 3 && (!verdict.ShouldBlock ||
			!strings.Contains(verdict.Display, fmt.Sprintf("refusal %d of 3 for the unreadable ledger", stop))) {
			t.Fatalf("unreadable stop %d was not counted: %+v", stop, verdict)
		}
		if stop == 3 && (verdict.ShouldBlock ||
			!strings.Contains(verdict.Display, "could not identify a continuation goal") ||
			!strings.Contains(verdict.Display, "the turn will end")) {
			t.Fatalf("the third unreadable stop did not end after its human escalation: %+v", verdict)
		}
	}
	if prepared || !alarmRaised || incident.BacklogDigest != unreadableIdleBacklogDigest || incident.GoalID != "" {
		t.Fatalf("the unreadable-ledger escalation was not local and goal-free: prepared=%t alarm=%t incident=%+v", prepared, alarmRaised, incident)
	}
}

func TestFreshLedgerFailureAndFetchTimeoutBlockTheStop(t *testing.T) {
	root := servingBed(t, "bed-m1", map[string]*GoalFile{
		"waiting": budgetedQueuedGoal("waiting", "2026-08-23T00:00:00Z"),
	})
	originalFetch, originalTimeout := fetchForProjection, freshProjectionTimeout
	t.Cleanup(func() {
		fetchForProjection, freshProjectionTimeout = originalFetch, originalTimeout
	})

	t.Run("fetch failure", func(t *testing.T) {
		fetchForProjection = func(Endpoint) (AdvanceResult, error) {
			return AdvanceResult{}, errors.New("canonical remote unavailable")
		}
		verdict, err := (&Store{Root: root}).TurnVerdict(ScanResult{}, "fetch-failure", "", "main-1")
		if err != nil || !verdict.ShouldBlock || verdict.BlockSource == nil || *verdict.BlockSource != "uncertainty" ||
			!strings.Contains(verdict.Display, "canonical remote unavailable") {
			t.Fatalf("a fresh-ledger failure must return a structured block: %+v %v", verdict, err)
		}
		if _, err := os.Stat(sessionStopRegistryPath(root)); !os.IsNotExist(err) {
			t.Fatalf("an agent-path fetch failure consumed stop authority: %v", err)
		}
	})

	t.Run("fetch timeout", func(t *testing.T) {
		freshProjectionTimeout = 20 * time.Millisecond
		fetchForProjection = func(Endpoint) (AdvanceResult, error) {
			time.Sleep(100 * time.Millisecond)
			return AdvanceResult{}, nil
		}
		started := time.Now()
		verdict, err := (&Store{Root: root}).TurnVerdict(ScanResult{}, "fetch-timeout", "", "main-1")
		if elapsed := time.Since(started); elapsed > time.Second {
			t.Fatalf("the bounded fetch did not release the verdict promptly: %s", elapsed)
		}
		if err != nil || !verdict.ShouldBlock || !strings.Contains(verdict.Display, "timed out") {
			t.Fatalf("a fetch timeout must return a structured block: %+v %v", verdict, err)
		}
	})
}

func TestMissingAcceptedGoalTreeReferenceBlocksAsUncertainty(t *testing.T) {
	root := servingBed(t, "bed-m1", map[string]*GoalFile{
		"waiting": budgetedQueuedGoal("waiting", "2026-08-23T00:00:00Z"),
	})
	if _, err := gitIn(root, "update-ref", "-d", AcceptedRef); err != nil {
		t.Fatal(err)
	}
	verdict, err := (&Store{Root: root}).TurnVerdict(ScanResult{}, "missing-accepted", "", "main-1")
	if err != nil || !verdict.ShouldBlock || verdict.BlockSource == nil || *verdict.BlockSource != "uncertainty" ||
		!strings.Contains(verdict.Display, "accepted reference") {
		t.Fatalf("a missing accepted canonical tree fell back to a quiet legacy world: %+v %v", verdict, err)
	}
}

func TestUnreadableTurnVerdictStateBlocksAsUncertainty(t *testing.T) {
	root := servingBed(t, "bed-m1", nil)
	writeIdleJSON(t, filepath.Join(root, "artifacts", "agents", "turn-verdict-state.json"), map[string]any{
		"schemaVersion": 2,
		"sessions":      map[string]any{},
	})
	verdict, err := (&Store{Root: root}).TurnVerdict(ScanResult{}, "unreadable-state", "", "main-1")
	if err != nil || !verdict.ShouldBlock || verdict.BlockSource == nil || *verdict.BlockSource != "uncertainty" ||
		!strings.Contains(verdict.Display, "turn verdict state") {
		t.Fatalf("unreadable turn verdict state did not fail closed: %+v %v", verdict, err)
	}
}

func TestMissingTemplateStateRootBlocksAsUncertainty(t *testing.T) {
	outer := t.TempDir()
	if err := os.MkdirAll(filepath.Join(outer, "development"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outer, "development", "metasystem-design.md"), []byte("template marker\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	verdict, err := (&Store{Root: outer}).TurnVerdict(ScanResult{}, "missing-template-state", "", "main-1")
	if err != nil || !verdict.ShouldBlock || verdict.BlockSource == nil || *verdict.BlockSource != "uncertainty" ||
		!strings.Contains(verdict.Display, "template installation is missing") {
		t.Fatalf("missing template state root did not fail closed: %+v %v", verdict, err)
	}
}

func TestTemplateCheckoutTurnVerdictUsesTheMetasystemStateRoot(t *testing.T) {
	standalone := servingBed(t, "bed-m1", map[string]*GoalFile{
		"waiting": budgetedQueuedGoal("waiting", "2026-08-23T00:00:00Z"),
	})
	outer := t.TempDir()
	if err := os.MkdirAll(filepath.Join(outer, "development"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outer, "development", "metasystem-design.md"), []byte("template marker\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stateRoot := filepath.Join(outer, "metasystem")
	if err := os.Rename(standalone, stateRoot); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stateRoot, "metasystem.conf"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	verdict, err := (&Store{Root: outer}).TurnVerdict(ScanResult{}, "template-root", "", "main-1")
	if err != nil || !verdict.ShouldBlock || verdict.BlockSource == nil || *verdict.BlockSource != "idle-backlog" ||
		!strings.Contains(verdict.Display, "waiting") {
		t.Fatalf("the template turn verdict did not see the metasystem backlog state: %+v %v", verdict, err)
	}
	if _, err := os.Stat(filepath.Join(stateRoot, "artifacts", "agents", "turn-verdict-state.json")); err != nil {
		t.Fatalf("the verdict did not write into the state root it judged: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outer, "artifacts", "agents", "turn-verdict-state.json")); !os.IsNotExist(err) {
		t.Fatalf("the verdict split state back into the containing checkout: %v", err)
	}
}

func sessionStopFixture(t *testing.T, store *Store, session, main string, epoch int64) SessionStop {
	t.Helper()
	writeIdleJSON(t, sessionStopLeasePath(store.Root), map[string]any{
		"holderMainId": main, "pid": 41, "pidStartedAt": 100, "claimEpoch": epoch,
	})
	writeIdleJSON(t, filepath.Join(store.Root, "artifacts", "agents", "mains", session+"-41.json"), map[string]any{
		"sessionId": session, "mainId": main, "pid": 41, "pidStartedAt": 100,
		"pgid": 41, "runtime": "claude", "instanceTag": "fixture-session-" + session,
		"commandHash": strings.Repeat("a", 64), "announcedAt": "2026-09-02T09:59:00Z",
	})
	proof := *testHumanAuthority(t, store.Root, time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC))
	if prober, ok := store.Prober.(idleFixtureProber); ok {
		prober[20] = identity.Exact{Pid: 20, StartedAt: time.Unix(200, 0)}
	}
	marker, err := store.WriteSessionStop(SessionStop{
		SchemaVersion: 3, SessionId: session, HolderMainId: main, ClaimEpoch: epoch,
		By: "Wido", WrittenAt: "2026-09-02T10:00:00Z", ExpiresAt: "2026-09-02T18:00:00Z",
	}, proof)
	if err != nil {
		t.Fatal(err)
	}
	return marker
}

func TestValidSessionStopBypassesHangingFetchAndSpendsExactlyOnce(t *testing.T) {
	now := time.Date(2026, 9, 2, 10, 1, 0, 0, time.UTC)
	root := servingBed(t, "bed-m1", map[string]*GoalFile{
		"waiting": budgetedQueuedGoal("waiting", "2026-08-23T00:00:00Z"),
	})
	store := &Store{Root: root, Now: func() time.Time { return now }, Prober: idleFixtureProber{
		41: {Pid: 41, StartedAt: time.Unix(100, 0)},
	}}
	marker := sessionStopFixture(t, store, "human-local-only", "main-1", 7)

	originalFetch, originalTimeout := fetchForProjection, freshProjectionTimeout
	remoteStarted := make(chan struct{}, 1)
	releaseRemote := make(chan struct{})
	fetchForProjection = func(Endpoint) (AdvanceResult, error) {
		remoteStarted <- struct{}{}
		<-releaseRemote
		return AdvanceResult{}, errors.New("hanging remote released")
	}
	freshProjectionTimeout = 20 * time.Millisecond
	t.Cleanup(func() {
		close(releaseRemote)
		fetchForProjection, freshProjectionTimeout = originalFetch, originalTimeout
	})

	first, err := store.TurnVerdict(ScanResult{}, marker.SessionId, "", marker.HolderMainId)
	if err != nil || first.ShouldBlock || !strings.Contains(first.Display, "authorized once") {
		t.Fatalf("the attended human marker must authorize a quiet stop without the remote: %+v %v", first, err)
	}
	select {
	case <-remoteStarted:
		t.Fatal("the attended-human path started the fresh-ledger network fetch")
	default:
	}
	registry, err := store.readSessionStopRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if _, consumed := registry.Consumed[marker.AuthorizationId]; !consumed {
		t.Fatal("the allowed human stop did not spend its authorization")
	}
	if _, err := os.Stat(sessionStopPath(root, marker.SessionId)); !os.IsNotExist(err) {
		t.Fatalf("the allowed human stop left its marker available: %v", err)
	}

	fetchForProjection = func(Endpoint) (AdvanceResult, error) {
		return AdvanceResult{}, errors.New("canonical remote unavailable after the one human stop")
	}
	second, err := store.TurnVerdict(ScanResult{}, marker.SessionId, "", marker.HolderMainId)
	if err != nil || !second.ShouldBlock || second.BlockSource == nil || *second.BlockSource != "uncertainty" ||
		strings.Contains(second.Display, "authorized once") {
		t.Fatalf("the spent marker authorized a second stop: %+v %v", second, err)
	}
	registry, err = store.readSessionStopRegistry()
	if err != nil || len(registry.Consumed) != 1 {
		t.Fatalf("the blocked agent path changed the consumed registry: %+v %v", registry, err)
	}
}

func TestFetchDeadlineLeavesNewlyValidSessionStopUnspent(t *testing.T) {
	now := time.Date(2026, 9, 2, 10, 1, 0, 0, time.UTC)
	root := servingBed(t, "bed-m1", map[string]*GoalFile{
		"waiting": budgetedQueuedGoal("waiting", "2026-08-23T00:00:00Z"),
	})
	store := &Store{Root: root, Now: func() time.Time { return now }, Prober: idleFixtureProber{
		41: {Pid: 41, StartedAt: time.Unix(100, 0)},
	}}
	marker := sessionStopFixture(t, store, "deadline-unspent", "main-1", 7)
	markerPath := sessionStopPath(root, marker.SessionId)
	markerBytes, err := os.ReadFile(markerPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(markerPath); err != nil {
		t.Fatal(err)
	}

	originalFetch, originalTimeout := fetchForProjection, freshProjectionTimeout
	markerPublished := make(chan struct{})
	releaseRemote := make(chan struct{})
	fetchForProjection = func(Endpoint) (AdvanceResult, error) {
		if err := os.WriteFile(markerPath, markerBytes, 0o644); err != nil {
			return AdvanceResult{}, err
		}
		close(markerPublished)
		<-releaseRemote
		return AdvanceResult{}, errors.New("hanging remote released")
	}
	freshProjectionTimeout = 20 * time.Millisecond
	t.Cleanup(func() {
		close(releaseRemote)
		fetchForProjection, freshProjectionTimeout = originalFetch, originalTimeout
	})

	aborted, err := store.TurnVerdict(ScanResult{}, marker.SessionId, "", marker.HolderMainId)
	if err != nil || !aborted.ShouldBlock || aborted.BlockSource == nil || *aborted.BlockSource != "uncertainty" ||
		!strings.Contains(aborted.Display, "timed out") || strings.Contains(aborted.Display, "authorized once") {
		t.Fatalf("the fetch-deadline turn did not fail closed: %+v %v", aborted, err)
	}
	select {
	case <-markerPublished:
	default:
		t.Fatal("the deadline fixture did not publish the valid marker during the fetch")
	}
	if _, err := os.Stat(markerPath); err != nil {
		t.Fatalf("the deadline-aborted turn spent the valid marker: %v", err)
	}
	if _, err := os.Stat(sessionStopRegistryPath(root)); !os.IsNotExist(err) {
		t.Fatalf("the deadline-aborted turn changed the consumed registry: %v", err)
	}

	retried, err := store.TurnVerdict(ScanResult{}, marker.SessionId, "", marker.HolderMainId)
	if err != nil || retried.ShouldBlock || !strings.Contains(retried.Display, "authorized once") {
		t.Fatalf("the unspent marker did not authorize the next completed human stop: %+v %v", retried, err)
	}
}

func TestBlockedHumanVerdictLeavesValidSessionStopUnspent(t *testing.T) {
	now := time.Date(2026, 9, 2, 10, 1, 0, 0, time.UTC)
	root := servingBed(t, "bed-m1", map[string]*GoalFile{
		"waiting": budgetedQueuedGoal("waiting", "2026-08-23T00:00:00Z"),
	})
	store := &Store{Root: root, Now: func() time.Time { return now }, Prober: idleFixtureProber{
		41: {Pid: 41, StartedAt: time.Unix(100, 0)},
	}}
	marker := sessionStopFixture(t, store, "blocked-human", "main-1", 7)
	writeIdleJSON(t, filepath.Join(root, "artifacts", "agents", "turn-verdict-state.json"), map[string]any{
		"schemaVersion": 2,
		"sessions":      map[string]any{},
	})

	blocked, err := store.TurnVerdict(ScanResult{}, marker.SessionId, "", marker.HolderMainId)
	if err != nil || !blocked.ShouldBlock || blocked.BlockSource == nil || *blocked.BlockSource != "uncertainty" ||
		!strings.Contains(blocked.Display, "turn verdict state") || strings.Contains(blocked.Display, "authorized once") {
		t.Fatalf("the incomplete local verdict did not fail closed: %+v %v", blocked, err)
	}
	if _, err := os.Stat(sessionStopPath(root, marker.SessionId)); err != nil {
		t.Fatalf("the blocked human verdict spent its valid marker: %v", err)
	}
	if _, err := os.Stat(sessionStopRegistryPath(root)); !os.IsNotExist(err) {
		t.Fatalf("the blocked human verdict changed the consumed registry: %v", err)
	}
}

func TestSessionStopLibraryAndConsumerRequireHumanClassificationProof(t *testing.T) {
	root := servingBed(t, "bed-m1", map[string]*GoalFile{
		"waiting": budgetedQueuedGoal("waiting", "2026-08-23T00:00:00Z"),
	})
	store := &Store{Root: root}
	writeIdleJSON(t, sessionStopLeasePath(root), map[string]any{
		"holderMainId": "main-1", "pid": 41, "pidStartedAt": 100, "claimEpoch": 7,
	})
	if _, err := store.WriteSessionStop(SessionStop{
		SchemaVersion: 3, SessionId: "agent-library", HolderMainId: "main-1", ClaimEpoch: 7,
		By: "Agent", WrittenAt: "2026-09-02T10:00:00Z", ExpiresAt: "2026-09-02T18:00:00Z",
	}, humanauthority.Proof{}); err == nil || !strings.Contains(err.Error(), "human-classification proof") {
		t.Fatalf("the exported library writer accepted an unclassified caller: %v", err)
	}
	if _, err := os.Stat(sessionStopPath(root, "agent-library")); !os.IsNotExist(err) {
		t.Fatalf("the rejected library call wrote a marker: %v", err)
	}
	writeIdleJSON(t, sessionStopPath(root, "agent-library"), SessionStop{
		SchemaVersion: 3, AuthorizationId: strings.Repeat("a", 32),
		SessionId: "agent-library", HolderMainId: "main-1", ClaimEpoch: 7,
		By: "Agent", WrittenAt: "2026-09-02T10:00:00Z", ExpiresAt: "2026-09-02T18:00:00Z",
		Human: SessionStopProcessRef{Pid: 42, PidStartedAt: 200},
	})
	verdict, err := store.TurnVerdict(ScanResult{}, "agent-library", "", "main-1")
	if err != nil || !verdict.ShouldBlock || verdict.BlockSource == nil || *verdict.BlockSource != "uncertainty" ||
		!strings.Contains(verdict.Display, "human-classification proof") {
		t.Fatalf("the consumer accepted a marker lacking its proof token: %+v %v", verdict, err)
	}
}

func TestSessionStopIsHolderBoundSingleUseAndConsumedAcrossQuietTurns(t *testing.T) {
	now := time.Date(2026, 9, 2, 10, 1, 0, 0, time.UTC)
	root := servingBed(t, "bed-m1", map[string]*GoalFile{
		"waiting": budgetedQueuedGoal("waiting", "2026-08-23T00:00:00Z"),
	})
	store := &Store{Root: root, Now: func() time.Time { return now }, Prober: idleFixtureProber{
		41: {Pid: 41, StartedAt: time.Unix(100, 0)},
	}}
	marker := sessionStopFixture(t, store, "session-a", "main-1", 7)
	markerBytes, err := os.ReadFile(sessionStopPath(root, marker.SessionId))
	if err != nil {
		t.Fatal(err)
	}

	authorized, err := store.TurnVerdict(ScanResult{}, "session-a", "", "main-1")
	if err != nil || authorized.ShouldBlock || !strings.Contains(authorized.Display, "authorized once") {
		t.Fatalf("the attended human marker must authorize one quiet stop: %+v %v", authorized, err)
	}
	if err := os.WriteFile(sessionStopPath(root, marker.SessionId), markerBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	replayed, err := store.TurnVerdict(ScanResult{}, "session-a", "", "main-1")
	if err != nil || !replayed.ShouldBlock || !strings.Contains(replayed.Display, "cannot replay") {
		t.Fatalf("restored marker bytes must remain consumed: %+v %v", replayed, err)
	}

	changed := sessionStopFixture(t, store, "session-b", "main-1", 7)
	_ = changed
	writeIdleJSON(t, sessionStopLeasePath(root), map[string]any{
		"holderMainId": "main-2", "pid": 41, "pidStartedAt": 100, "claimEpoch": 8,
	})
	holderChanged, err := store.TurnVerdict(ScanResult{}, "session-b", "", "main-2")
	if err != nil || !holderChanged.ShouldBlock || !strings.Contains(holderChanged.Display, "SESSION STOP authorization") {
		t.Fatalf("a holder change must invalidate the old authorization: %+v %v", holderChanged, err)
	}

	quietRoot := servingBed(t, "bed-m1", nil)
	quietStore := &Store{Root: quietRoot, Now: func() time.Time { return now }, Prober: store.Prober}
	quietMarker := sessionStopFixture(t, quietStore, "session-quiet", "main-1", 9)
	quietBytes, _ := os.ReadFile(sessionStopPath(quietRoot, quietMarker.SessionId))
	if verdict, err := quietStore.TurnVerdict(ScanResult{}, "session-quiet", "", "main-1"); err != nil || verdict.ShouldBlock {
		t.Fatalf("the next quiet turn must consume the marker without inventing a block: %+v %v", verdict, err)
	}
	if err := os.WriteFile(sessionStopPath(quietRoot, quietMarker.SessionId), quietBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, authorized, detail, err := quietStore.consumeSessionStop("session-quiet", "main-1"); err != nil || authorized || !strings.Contains(detail, "cannot replay") {
		t.Fatalf("a marker consumed while work was parked or absent must not survive for later: authorized=%v detail=%q err=%v", authorized, detail, err)
	}
}

func TestSessionStopConsumeOnUseSurvivesAbsentOrFailedSessionEnd(t *testing.T) {
	now := time.Date(2026, 9, 2, 10, 1, 0, 0, time.UTC)
	for _, test := range []struct {
		name       string
		sessionEnd func(*testing.T, *Store, SessionStop)
	}{
		{name: "SessionEnd did not run"},
		{
			name: "SessionEnd failed before it could inspect the marker",
			sessionEnd: func(t *testing.T, store *Store, marker SessionStop) {
				lockPath := filepath.Join(store.Root, "artifacts", "agents", "goal.lock")
				if err := os.Remove(lockPath); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(lockPath, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := store.EndSessionStop(marker.SessionId); err == nil || !strings.Contains(err.Error(), "goal lock cannot be opened") {
					t.Fatalf("the fixture did not force SessionEnd to fail before mutation: %v", err)
				}
				if err := os.Remove(lockPath); err != nil {
					t.Fatal(err)
				}
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := servingBed(t, "bed-m1", map[string]*GoalFile{
				"waiting": budgetedQueuedGoal("waiting", "2026-08-23T00:00:00Z"),
			})
			store := &Store{Root: root, Now: func() time.Time { return now }, Prober: idleFixtureProber{
				41: {Pid: 41, StartedAt: time.Unix(100, 0)},
			}}
			marker := sessionStopFixture(t, store, "same-live-session", "main-1", 7)

			first, err := store.TurnVerdict(ScanResult{}, marker.SessionId, "", marker.HolderMainId)
			if err != nil || first.ShouldBlock || !strings.Contains(first.Display, "authorized once") {
				t.Fatalf("one attended marker must authorize the first quiet stop: %+v %v", first, err)
			}
			registry, err := store.readSessionStopRegistry()
			if err != nil {
				t.Fatal(err)
			}
			if _, consumed := registry.Consumed[marker.AuthorizationId]; !consumed {
				t.Fatal("the quiet-stop verdict returned before the authorization was recorded as consumed")
			}
			if _, err := os.Stat(sessionStopPath(root, marker.SessionId)); !os.IsNotExist(err) {
				t.Fatalf("the quiet-stop verdict left its used marker in place: %v", err)
			}

			if test.sessionEnd != nil {
				test.sessionEnd(t, store, marker)
			}
			second, err := store.TurnVerdict(ScanResult{}, marker.SessionId, "", marker.HolderMainId)
			if err != nil || !second.ShouldBlock || second.BlockSource == nil || *second.BlockSource != "idle-backlog" ||
				strings.Contains(second.Display, "authorized once") {
				t.Fatalf("the same live session stopped twice without a fresh human marker: %+v %v", second, err)
			}
		})
	}
}

func TestSessionStopConsumeErrorBlocksBeforeAuthorization(t *testing.T) {
	now := time.Date(2026, 9, 2, 10, 1, 0, 0, time.UTC)
	root := servingBed(t, "bed-m1", map[string]*GoalFile{
		"waiting": budgetedQueuedGoal("waiting", "2026-08-23T00:00:00Z"),
	})
	store := &Store{Root: root, Now: func() time.Time { return now }, Prober: idleFixtureProber{
		41: {Pid: 41, StartedAt: time.Unix(100, 0)},
	}}
	marker := sessionStopFixture(t, store, "consume-error", "main-1", 7)
	writeIdleJSON(t, sessionStopRegistryPath(root), map[string]any{
		"schemaVersion": 1,
		"consumed":      "unreadable as a consumed registry",
	})

	verdict, err := store.TurnVerdict(ScanResult{}, marker.SessionId, "", marker.HolderMainId)
	if err != nil || !verdict.ShouldBlock || verdict.BlockSource == nil || *verdict.BlockSource != "uncertainty" ||
		!strings.Contains(verdict.Display, "consumed registry unreadable") || strings.Contains(verdict.Display, "authorized once") {
		t.Fatalf("an uncertain consume path authorized a quiet stop: %+v %v", verdict, err)
	}
	if _, err := os.Stat(sessionStopPath(root, marker.SessionId)); err != nil {
		t.Fatalf("the failed consume path did not leave the unused marker available for a later proven consume: %v", err)
	}
}

func TestSessionStopCannotCrossASessionEndWithoutAStopHook(t *testing.T) {
	now := time.Date(2026, 9, 2, 10, 1, 0, 0, time.UTC)
	root := servingBed(t, "bed-m1", map[string]*GoalFile{
		"waiting": budgetedQueuedGoal("waiting", "2026-08-23T00:00:00Z"),
	})
	store := &Store{Root: root, Now: func() time.Time { return now }, Prober: idleFixtureProber{
		41: {Pid: 41, StartedAt: time.Unix(100, 0)},
	}}
	marker := sessionStopFixture(t, store, "session-ended", "main-1", 7)
	announcement := filepath.Join(root, "artifacts", "agents", "mains", "session-ended-41.json")
	if err := os.Remove(announcement); err != nil {
		t.Fatal(err)
	}
	writeIdleJSON(t, announcement, map[string]any{
		"sessionId": "session-ended", "mainId": "main-1", "pid": 41, "pidStartedAt": 100,
		"pgid": 41, "runtime": "claude", "instanceTag": "fixture-session-restarted",
		"commandHash": strings.Repeat("a", 64), "announcedAt": "2026-09-02T10:00:30Z",
	})
	verdict, err := store.TurnVerdict(ScanResult{}, marker.SessionId, "", "main-1")
	if err != nil || !verdict.ShouldBlock || !strings.Contains(verdict.Display, "earlier session lifecycle") {
		t.Fatalf("a marker surviving SessionEnd authorized a later lifecycle: %+v %v", verdict, err)
	}
}

func TestSessionEndDurablySpendsUnusedStopAuthorizationBeforeAnnouncementRetirement(t *testing.T) {
	now := time.Date(2026, 9, 2, 10, 1, 0, 0, time.UTC)
	root := servingBed(t, "bed-m1", map[string]*GoalFile{
		"waiting": budgetedQueuedGoal("waiting", "2026-08-23T00:00:00Z"),
	})
	store := &Store{Root: root, Now: func() time.Time { return now }, Prober: idleFixtureProber{
		41: {Pid: 41, StartedAt: time.Unix(100, 0)},
	}}
	marker := sessionStopFixture(t, store, "session-retirement-failed", "main-1", 7)
	markerPath := sessionStopPath(root, marker.SessionId)
	markerBytes, err := os.ReadFile(markerPath)
	if err != nil {
		t.Fatal(err)
	}
	announcement := filepath.Join(root, "artifacts", "agents", "mains", "session-retirement-failed-41.json")

	if err := store.EndSessionStop(marker.SessionId); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(announcement); err != nil {
		t.Fatalf("the fixture requires announcement retirement to have failed: %v", err)
	}
	if _, err := os.Stat(markerPath); !os.IsNotExist(err) {
		t.Fatalf("SessionEnd left its unused stop authorization in place: %v", err)
	}

	if err := os.WriteFile(markerPath, markerBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	verdict, err := store.TurnVerdict(ScanResult{}, marker.SessionId, "", "main-1")
	if err != nil || !verdict.ShouldBlock || !strings.Contains(verdict.Display, "cannot replay") {
		t.Fatalf("a marker restored after SessionEnd authorized a later stop: %+v %v", verdict, err)
	}
}

func TestClaudeTurnExitRequiresDelegateWorkNotMerelyALiveSeat(t *testing.T) {
	root := servingBed(t, "bed-m1", map[string]*GoalFile{
		"waiting": budgetedQueuedGoal("waiting", "2026-08-23T00:00:00Z"),
	})
	jobs := filepath.Join(root, "artifacts", "agents", "jobs")
	writeIdleJSON(t, filepath.Join(jobs, "delegate.json"), map[string]any{
		"jobId": "delegate", "status": "running", "pid": 99, "pidStartedAt": 1,
	})
	store := &Store{Root: root, Prober: idleFixtureProber{41: {Pid: 41, StartedAt: time.Unix(100, 0)}}}
	stale, _ := store.TurnVerdict(ScanResult{}, "stale-job", "", "main-1")
	if !stale.ShouldBlock {
		t.Fatalf("a stale running record must not suppress the block: %+v", stale)
	}
	writeIdleJSON(t, filepath.Join(jobs, "delegate.json"), map[string]any{
		"jobId": "delegate", "status": "running", "pid": 41, "pidStartedAt": 100,
	})
	live, _ := store.TurnVerdict(ScanResult{}, "live-job", "", "main-1")
	if live.ShouldBlock {
		t.Fatalf("a running job joined to its live process is in flight: %+v", live)
	}
	writeIdleJSON(t, filepath.Join(jobs, "delegate.json"), map[string]any{
		"jobId": "delegate", "status": "pending-setup",
		"creatorLiveness": map[string]any{"pid": 41, "pidStartedAt": 100},
	})
	pendingSetup, _ := store.TurnVerdict(ScanResult{}, "pending-setup", "", "main-1")
	if pendingSetup.ShouldBlock {
		t.Fatalf("pending-setup with a live creator is in flight for both owners: %+v", pendingSetup)
	}

	claimRoot := servingBed(t, "bed-m1", map[string]*GoalFile{
		"waiting": budgetedQueuedGoal("waiting", "2026-08-23T00:00:00Z"),
		"claimed": {
			Id: "claimed", State: StateClaimed, Intent: "Joined claim", Origin: OriginMain,
			NextStep: "Keep working.", OpenedAt: "2026-08-22T00:00:00Z", Revision: 2,
			Claimed: &ClaimRecord{Machine: "bed-m1", Lineage: "coordinator", At: "2026-08-23T01:00:00Z"},
		},
	})
	claimStore := &Store{Root: claimRoot, Prober: installIdleLiveClaim(t, claimRoot, "coordinator")}
	liveClaim, _ := claimStore.TurnVerdict(ScanResult{}, "live-claim", "", "main-1")
	if !liveClaim.ShouldBlock || liveClaim.BlockSource == nil || *liveClaim.BlockSource != "idle-backlog" {
		t.Fatalf("the Claude turn exit treated its still-live seat as active work on the claim: %+v", liveClaim)
	}
}
