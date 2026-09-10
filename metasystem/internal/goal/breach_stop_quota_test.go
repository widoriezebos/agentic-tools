package goal

import (
	"strings"
	"testing"
	"time"
)

func breachStoppedGoalForTest(id, machine string) *GoalFile {
	budget := testBudget()
	claimedAt := "2026-08-23T01:00:00Z"
	return &GoalFile{
		Id: id, State: StateClaimed, Intent: "Finish the stopped work", Origin: OriginMain,
		NextStep: "Resume only after the human allows it.", OpenedAt: "2026-08-23T00:00:00Z", Revision: 3,
		Budget: &budget,
		Claimed: &ClaimRecord{
			Machine: machine, Lineage: "seat-lineage", At: claimedAt, Revision: 2, AccountingRevision: 2,
		},
		StopCapability: &StopCapability{
			Generation: 2, Revision: 2, Machine: machine, ClaimEpoch: 9, FenceEpoch: 1,
		},
		StopFence: &StopFence{
			StopID: "stop-" + id + "-r2-f1", Revision: 2, Epoch: 1, CapabilityGeneration: 2,
			ClosedAt: "2026-08-23T01:02:00Z", Reason: StopReasonElapsedLimit,
		},
	}
}

func TestBreachStoppedClaimLeavesMachineQuotaOpenButKeepsResumeFence(t *testing.T) {
	_, root, _ := twoClones(t)
	seedLedger(t, root)
	budget := Budget{ElapsedLimit: "1m", AttemptLimit: 2, ReservedJobMinutesLimit: 20, ActiveJobLimit: 1}

	if result, err := Open(verbReq(root, "01J5X00000000000000000T000", "mac-a"), "fenced-a", "Bound the first item.", OriginMain, "Run it."); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("open goal A: %+v %v", result, err)
	}
	claimA := verbReq(root, "01J5X00000000000000000T010", "mac-a")
	claimA.ClaimEpoch = 9
	if result, err := claimApprovedForTest(t, claimA, "fenced-a", budget); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("claim goal A: %+v %v", result, err)
	}
	projection, err := Project(endpointFor(root), true, claimA.Now)
	if err != nil {
		t.Fatal(err)
	}
	goalA := projection.Tree.Live["fenced-a"]
	stop := CloseStopRequest{
		VerbRequest: VerbRequest{
			Endpoint: endpointFor(root), Actor: Actor{Machine: "mac-a", Lineage: "goal-stop-custodian"},
			Ulid: "01J5X00000000000000000T020", Now: claimA.Now.Add(90 * time.Second), ClaimEpoch: 9,
		},
		GoalID: "fenced-a", StopID: "stop-fenced-a-r2-f1", Reason: StopReasonElapsedLimit,
		Capability: *goalA.StopCapability,
	}
	if result, err := CloseStop(stop); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("breach-stop goal A: %+v %v", result, err)
	}

	if result, err := Open(verbReq(root, "01J5X00000000000000000T030", "mac-a"), "working-b", "Bound the next item.", OriginMain, "Run it."); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("open goal B: %+v %v", result, err)
	}
	claimB := verbReq(root, "01J5X00000000000000000T040", "mac-a")
	claimB.ClaimEpoch = 9
	if result, err := claimApprovedForTest(t, claimB, "working-b", budget); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("claim goal B while goal A is fenced: %+v %v", result, err)
	}
	projection, err = Project(endpointFor(root), true, claimB.Now)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateCommit(root, acceptedTip(t, root)); err != nil {
		t.Fatalf("tree with one fenced and one live claim must validate: %v", err)
	}
	goalA = projection.Tree.Live["fenced-a"]
	if goalA == nil || goalA.Claimed == nil || goalA.StopCapability == nil || goalA.StopFence == nil {
		t.Fatalf("claiming goal B changed goal A's stopped binding: %+v", goalA)
	}

	park := verbReq(root, "01J5X00000000000000000T050", "mac-a")
	if result, err := Park(park, "fenced-a", "wait for the human"); err != nil || result.Outcome != OutcomeRejected ||
		!strings.Contains(result.Detail, "only goal resume may clear its launch fence") {
		t.Fatalf("park must preserve goal A's launch fence: %+v %v", result, err)
	}
	done := verbReq(root, "01J5X00000000000000000T060", "mac-a")
	if result, err := Done(done, "fenced-a", "must not bypass the fence"); err != nil || result.Outcome != OutcomeRejected ||
		!strings.Contains(result.Detail, "only goal resume may clear its launch fence") {
		t.Fatalf("done must preserve goal A's launch fence: %+v %v", result, err)
	}

	if result, err := Open(verbReq(root, "01J5X00000000000000000T070", "mac-a"), "third-c", "Bound a third item.", OriginMain, "Run it."); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("open goal C: %+v %v", result, err)
	}
	claimC := verbReq(root, "01J5X00000000000000000T080", "mac-a")
	claimC.ClaimEpoch = 9
	if result, err := claimApprovedForTest(t, claimC, "third-c", budget); err != nil || result.Outcome != OutcomeRejected ||
		!strings.Contains(result.Detail, "quota is one claim per machine") {
		t.Fatalf("goal B must still consume the machine quota: %+v %v", result, err)
	}
}

func TestResumeWaitsUntilMachinesOtherLiveClaimIsReleased(t *testing.T) {
	_, root, _ := twoClones(t)
	seedLedger(t, root)
	budget := Budget{ElapsedLimit: "1m", AttemptLimit: 2, ReservedJobMinutesLimit: 20, ActiveJobLimit: 1}

	if result, err := Open(verbReq(root, "01J5X00000000000000000V000", "mac-a"), "fenced-a", "Bound the stopped item.", OriginMain, "Run it."); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("open stopped goal: %+v %v", result, err)
	}
	claimA := verbReq(root, "01J5X00000000000000000V010", "mac-a")
	claimA.ClaimEpoch = 9
	if result, err := claimApprovedForTest(t, claimA, "fenced-a", budget); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("claim stopped goal: %+v %v", result, err)
	}
	projection, err := Project(endpointFor(root), true, claimA.Now)
	if err != nil {
		t.Fatal(err)
	}
	goalA := projection.Tree.Live["fenced-a"]
	stop := CloseStopRequest{
		VerbRequest: VerbRequest{
			Endpoint: endpointFor(root), Actor: Actor{Machine: "mac-a", Lineage: "goal-stop-custodian"},
			Ulid: "01J5X00000000000000000V020", Now: claimA.Now.Add(90 * time.Second), ClaimEpoch: 9,
		},
		GoalID: "fenced-a", StopID: "stop-fenced-a-r2-f1", Reason: StopReasonElapsedLimit,
		Capability: *goalA.StopCapability,
	}
	if result, err := CloseStop(stop); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("breach-stop goal A: %+v %v", result, err)
	}
	projection, err = Project(endpointFor(root), true, stop.Now)
	if err != nil {
		t.Fatal(err)
	}
	stopped := projection.Tree.Live["fenced-a"]
	resumeAt := stop.Now.Add(time.Minute)
	stamp := resumeAt.UTC().Format(time.RFC3339)
	if err := WriteStopBatch(root, StopBatch{
		StopID: stop.StopID, GoalID: stopped.Id, GoalRevision: stopped.Claimed.Revision,
		FenceEpoch: stopped.StopFence.Epoch, CapabilityGeneration: stopped.StopCapability.Generation,
		Machine: "mac-a", ClaimEpoch: 9, Reason: StopReasonElapsedLimit, State: StopBatchComplete,
		OpenedAt: stop.Now.UTC().Format(time.RFC3339), UpdatedAt: stamp, CompletedAt: stamp, Pass: 1,
	}); err != nil {
		t.Fatal(err)
	}

	if result, err := Open(verbReq(root, "01J5X00000000000000000V030", "mac-a"), "working-b", "Bound the live item.", OriginMain, "Run it."); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("open live goal: %+v %v", result, err)
	}
	claimB := verbReq(root, "01J5X00000000000000000V040", "mac-a")
	claimB.ClaimEpoch = 9
	if result, err := claimApprovedForTest(t, claimB, "working-b", budget); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("claim live goal: %+v %v", result, err)
	}

	resume := ResumeRequest{
		VerbRequest: VerbRequest{
			Endpoint: endpointFor(root), Actor: Actor{Machine: "mac-a", Lineage: "human-shell", Human: "wido"},
			Ulid: "01J5X00000000000000000V050", Now: resumeAt, ClaimEpoch: 9,
		},
		GoalID: "fenced-a", Budget: budget,
	}
	resume.Authority = testHumanAuthority(t, root, resume.Now)
	wantRefusal := "goal resume fenced-a refused: machine mac-a already holds live claim working-b; conclude, park or release working-b first, then resume fenced-a"
	if result, err := Resume(resume); err != nil || result.Outcome != OutcomeRejected || result.Detail != wantRefusal {
		t.Fatalf("resume with another live claim mismatch: %+v %v", result, err)
	}
	projection, err = Project(endpointFor(root), true, resume.Now)
	if err != nil {
		t.Fatal(err)
	}
	stillStopped := projection.Tree.Live["fenced-a"]
	if stillStopped.StopFence == nil || stillStopped.StopFence.StopID != stop.StopID || stillStopped.Claimed == nil {
		t.Fatalf("refused resume changed the stopped claim: %+v", stillStopped)
	}

	if result, err := Release(verbReq(root, "01J5X00000000000000000V060", "mac-a"), "working-b"); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("release live goal: %+v %v", result, err)
	}
	resume.Ulid = "01J5X00000000000000000V070"
	if result, err := Resume(resume); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("resume after release: %+v %v", result, err)
	}
	projection, err = Project(endpointFor(root), true, resume.Now)
	if err != nil {
		t.Fatal(err)
	}
	resumed := projection.Tree.Live["fenced-a"]
	if resumed.StopFence != nil || resumed.Claimed == nil || resumed.Claimed.Machine != "mac-a" {
		t.Fatalf("resume after release did not reopen goal A: %+v", resumed)
	}
}

func TestResumeAllowsAnotherLiveClaimInTheSameArc(t *testing.T) {
	_, root, _ := twoClones(t)
	seedLedger(t, root)
	arcBed(t, root, "resume-together", "resume-arc", "RA")

	projection, err := Project(endpointFor(root), true, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	stoppedMember := projection.Tree.Live["resume-arc-one"]
	stop := CloseStopRequest{
		VerbRequest: VerbRequest{
			Endpoint: endpointFor(root), Actor: Actor{Machine: "mac-a", Lineage: "goal-stop-custodian"},
			Ulid: "01J5X00000000000000000W010", Now: time.Date(2026, 8, 20, 22, 1, 0, 0, time.UTC), ClaimEpoch: 1,
		},
		GoalID: "resume-arc-one", StopID: "stop-resume-arc-one-r2-f1", Reason: StopReasonElapsedLimit,
		Capability: *stoppedMember.StopCapability,
	}
	if result, err := CloseStop(stop); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("breach-stop arc member: %+v %v", result, err)
	}
	projection, err = Project(endpointFor(root), true, stop.Now)
	if err != nil {
		t.Fatal(err)
	}
	stoppedMember = projection.Tree.Live["resume-arc-one"]
	resumeAt := stop.Now.Add(time.Minute)
	stamp := resumeAt.UTC().Format(time.RFC3339)
	if err := WriteStopBatch(root, StopBatch{
		StopID: stop.StopID, GoalID: stoppedMember.Id, GoalRevision: stoppedMember.Claimed.Revision,
		FenceEpoch: stoppedMember.StopFence.Epoch, CapabilityGeneration: stoppedMember.StopCapability.Generation,
		Machine: "mac-a", ClaimEpoch: 1, Reason: StopReasonElapsedLimit, State: StopBatchComplete,
		OpenedAt: stop.Now.UTC().Format(time.RFC3339), UpdatedAt: stamp, CompletedAt: stamp, Pass: 1,
	}); err != nil {
		t.Fatal(err)
	}

	resume := ResumeRequest{
		VerbRequest: VerbRequest{
			Endpoint: endpointFor(root), Actor: Actor{Machine: "mac-a", Lineage: "human-shell", Human: "wido"},
			Ulid: "01J5X00000000000000000W020", Now: resumeAt, ClaimEpoch: 1,
		},
		GoalID: stoppedMember.Id, Budget: testBudget(),
	}
	resume.Authority = testHumanAuthority(t, root, resume.Now)
	if result, err := Resume(resume); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("resume stopped member beside its live arc sibling: %+v %v", result, err)
	}
	projection, err = Project(endpointFor(root), true, resume.Now)
	if err != nil {
		t.Fatal(err)
	}
	resumed := projection.Tree.Live["resume-arc-one"]
	sibling := projection.Tree.Live["resume-arc-two"]
	if resumed.StopFence != nil || resumed.Claimed == nil || sibling.Claimed == nil ||
		resumed.Claimed.Machine != "mac-a" || sibling.Claimed.Machine != "mac-a" ||
		resumed.Arc == "" || resumed.Arc != sibling.Arc {
		t.Fatalf("resume did not preserve the machine's one live arc: resumed=%+v sibling=%+v", resumed, sibling)
	}
}

func TestNextSeparatesFencedClaimAndSelectsReadyWork(t *testing.T) {
	root := t.TempDir()
	seedGoalNormConfig(t, root)
	fenced := breachStoppedGoalForTest("fenced-a", "mac-a")
	fenced.Priority, fenced.Sequence = 1, 1
	ready := nextPriorityGoal("working-b", 1, 2, "")
	projection := Projection{Root: root, Tree: &TreeGoals{Live: map[string]*GoalFile{
		fenced.Id: fenced,
		ready.Id:  ready,
	}, Done: map[string]*GoalFile{}}}

	frontier, err := Next(projection, "mac-a")
	if err != nil {
		t.Fatal(err)
	}
	selection := SelectNext(frontier)
	if strings.Join(frontier.Fenced, ",") != "fenced-a" || len(frontier.Claimed) != 0 ||
		strings.Join(frontier.Ready, ",") != "working-b" || selection.Kind != NextSelectionReady || selection.GoalID != "working-b" {
		t.Fatalf("fenced claim displaced ready work: selection=%+v frontier=%+v", selection, frontier)
	}
}

func TestFencedClaimStaysVisibleWithoutBecomingCurrentOrIdleContinuation(t *testing.T) {
	fenced := breachStoppedGoalForTest("fenced-a", "bed-m1")
	ready := budgetedQueuedGoal("working-b", "2026-08-23T00:03:00Z")
	root := servingBed(t, "bed-m1", map[string]*GoalFile{
		fenced.Id: fenced,
		ready.Id:  ready,
	})
	store := &Store{Root: root}
	facts, status, statusLine := store.convertedGoalFacts()
	if facts != nil || status != "queued-only" || statusLine != "" {
		t.Fatalf("fenced claim became current work: facts=%+v status=%q detail=%q", facts, status, statusLine)
	}

	var prepared IdleEscalationEvent
	store.PrepareIdleContinuation = func(event IdleEscalationEvent) (string, error) {
		prepared = event
		return "intent-working-b", nil
	}
	store.RecordIdleIncident = func(IdleEscalationEvent) (string, error) { return "alert-working-b", nil }
	options := TurnVerdictOptions{
		SeatActor: Actor{Machine: "bed-m1", Lineage: "seat-lineage"}, SeatClaimEpoch: 9,
	}
	var third Verdict
	for stop := 1; stop <= 3; stop++ {
		var err error
		third, err = store.TurnVerdict(ScanResult{}, "fenced-session", "", "main-1", options)
		if err != nil {
			t.Fatal(err)
		}
	}
	wantFence := "FENCED fenced-a: breach-stopped by stop-fenced-a-r2-f1 (ELAPSED_LIMIT); only goal resume, a human act, clears it; the queue is open"
	if third.Goal != nil || !strings.Contains(third.Display, "no current goal; the queue holds working-b") ||
		!strings.Contains(third.Display, wantFence) {
		t.Fatalf("turn verdict hid or continued the fenced goal: %+v", third)
	}
	if prepared.GoalID != "working-b" || !prepared.ClaimNeeded || prepared.SeatClaimEpoch != 9 ||
		!strings.Contains(third.Display, "selected goal working-b and deferred its claim") ||
		strings.Contains(third.Display, "selected this machine's held goal fenced-a") {
		t.Fatalf("idle continuation did not choose ready work: event=%+v verdict=%+v", prepared, third)
	}
}

func TestTurnVerdictShowsLiveCurrentGoalBesideFencedClaim(t *testing.T) {
	fenced := breachStoppedGoalForTest("fenced-a", "bed-m1")
	fenced.Priority, fenced.Sequence = 1, 1
	live := &GoalFile{
		Id: "working-b", State: StateClaimed, Intent: "Carry the live work", Origin: OriginMain,
		NextStep: "Continue working B.", OpenedAt: "2026-08-23T00:03:00Z", Revision: 2,
		Priority: 1, Sequence: 2,
		Claimed: &ClaimRecord{
			Machine: "bed-m1", Lineage: "seat-lineage", At: "2026-08-23T01:03:00Z",
			Revision: 2, AccountingRevision: 2,
		},
	}
	root := servingBed(t, "bed-m1", map[string]*GoalFile{
		fenced.Id: fenced,
		live.Id:   live,
	})
	store := &Store{Root: root}
	options := TurnVerdictOptions{SeatActor: Actor{Machine: "bed-m1", Lineage: "seat-lineage"}, SeatClaimEpoch: 9}
	if _, err := store.TurnVerdict(ScanResult{}, "live-and-fenced-session", "", "main-1", options); err != nil {
		t.Fatal(err)
	}
	verdict, err := store.TurnVerdict(ScanResult{}, "live-and-fenced-session", "", "main-1", options)
	if err != nil {
		t.Fatal(err)
	}
	wantCurrent := "NOTHING LEFT TO WORK ON; the current goal is working-b (Continue working B.)"
	wantFence := "FENCED fenced-a: breach-stopped by stop-fenced-a-r2-f1 (ELAPSED_LIMIT); only goal resume, a human act, clears it; the queue is open"
	if verdict.Goal == nil || verdict.Goal.Id != "working-b" || !strings.Contains(verdict.Display, wantCurrent) || !strings.Contains(verdict.Display, wantFence) {
		t.Fatalf("live current goal or fenced claim missing from verdict: %+v", verdict)
	}
}
