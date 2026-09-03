package goal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

type goalAuthorityReader struct{}

func (goalAuthorityReader) Read(pid int64) (humanauthority.Snapshot, error) {
	parent := int64(10)
	if pid == 10 {
		parent = 1
	}
	return humanauthority.Snapshot{
		Exact:      identity.Exact{Pid: pid, StartedAt: time.Unix(pid*10, 0), Argv: []string{"human-shell"}, ArgvKnown: true},
		Executable: "/fixture/human-shell", ExecutableKnown: true,
		ParentPID: parent, ParentKnown: true, TerminalID: "tty-test", TerminalKnown: true,
	}, nil
}

func (goalAuthorityReader) SessionLeader(int64) (int64, error) { return 10, nil }

func testHumanAuthority(t *testing.T, root string, now time.Time) *humanauthority.Proof {
	t.Helper()
	directory := filepath.Join(root, "scripts", "agents", "adapters")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\n[ \"$1\" = signature ] && printf '%s\\n' 'match never-a-human-shell'\n"
	if err := os.WriteFile(filepath.Join(directory, "authority-test.sh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	reader := goalAuthorityReader{}
	if _, err := humanauthority.Enroll(root, 20, reader, now); err != nil {
		t.Fatal(err)
	}
	proof, err := humanauthority.Prove(root, 20, reader, now)
	if err != nil {
		t.Fatal(err)
	}
	return &proof
}

func TestBreachStopFenceAndHumanResumeAreOneWayTransactions(t *testing.T) {
	_, root, _ := twoClones(t)
	seedLedger(t, root)
	if res, err := Open(verbReq(root, "01J5X00000000000000000S000", "mac-a"), "stop-me", "Bound this work.", "main", "Run it."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", res, err)
	}
	claim := verbReq(root, "01J5X00000000000000000S010", "mac-a")
	claim.ClaimEpoch = 9
	approvedBudget := Budget{ElapsedLimit: "1m", AttemptLimit: 2, ReservedJobMinutesLimit: 20, ActiveJobLimit: 1}
	if res, err := claimApprovedForTest(t, claim, "stop-me", approvedBudget); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("claim: %+v %v", res, err)
	}
	p, err := Project(endpointFor(root), true, claim.Now)
	if err != nil {
		t.Fatal(err)
	}
	file := p.Tree.Live["stop-me"]
	stop := CloseStopRequest{
		VerbRequest: VerbRequest{
			Endpoint: endpointFor(root), Actor: Actor{Machine: "mac-a", Lineage: "goal-stop-custodian"},
			Ulid: "01J5X00000000000000000S020", Now: claim.Now.Add(90 * time.Second), ClaimEpoch: 9,
		},
		GoalID: "stop-me", StopID: "stop-stop-me-r2-f1", Reason: StopReasonElapsedLimit,
		Capability: *file.StopCapability,
	}
	if res, err := CloseStop(stop); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("close stop: %+v %v", res, err)
	}
	// The same operation and a fresh custodian retry are both harmless.
	if res, err := CloseStop(stop); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("same-op retry: %+v %v", res, err)
	}
	retry := stop
	retry.Ulid = "01J5X00000000000000000S021"
	if res, err := CloseStop(retry); err != nil || res.Outcome != OutcomeAbandoned {
		t.Fatalf("fresh retry must rediscover the fence: %+v %v", res, err)
	}
	p, err = Project(endpointFor(root), true, stop.Now)
	if err != nil {
		t.Fatal(err)
	}
	stopped := p.Tree.Live["stop-me"]
	if stopped.StopFence == nil || stopped.StopFence.StopID != stop.StopID || stopped.StopCapability.FenceEpoch != 1 ||
		stopped.History[len(stopped.History)-1].Verb != "breach-stop" {
		t.Fatalf("accepted stop fence/history missing: %+v", stopped)
	}
	resumeJournalOpid := Opid("01J5X00000000000000000S023", "mac-a", "human-shell")
	strandEntryAt(t, root, resumeJournalOpid, "mac-a", PhaseCreated, Intent{
		Verb: "resume", Targets: []string{"stop-me"}, Args: mergeIntentArgs(
			map[string]string{"by": "wido"},
			budgetIntentArgs(Budget{ElapsedLimit: "2h", AttemptLimit: 4, ReservedJobMinutesLimit: 80, ActiveJobLimit: 2}),
		),
	})
	reports, err := Recover(endpointFor(root))
	if err != nil {
		t.Fatal(err)
	}
	p, err = Project(endpointFor(root), true, stop.Now)
	if err != nil {
		t.Fatal(err)
	}
	journalResume := p.Tree.Live["stop-me"]
	entry, err := ReadEntry(root, resumeJournalOpid)
	if err != nil || entry.Outcome != OutcomeRejected || journalResume.StopFence == nil || len(reports) == 0 ||
		!strings.Contains(reports[len(reports)-1].Detail, "enrolled terminal") {
		t.Fatalf("dead-owner resume journal crossed the human boundary: goal=%+v entry=%+v reports=%+v err=%v", journalResume, entry, reports, err)
	}
	park := verbReq(root, "01J5X00000000000000000S025", "mac-a")
	park.Actor.Human = "wido"
	if res, err := Park(park, "stop-me", "do not orphan the stop batch"); err != nil || res.Outcome != OutcomeRejected {
		t.Fatalf("ordinary park cleared a stopped claim: %+v %v", res, err)
	}
	done := verbReq(root, "01J5X00000000000000000S026", "mac-a")
	if res, err := Done(done, "stop-me", "must not bypass the stop batch"); err != nil || res.Outcome != OutcomeRejected ||
		!strings.Contains(res.Detail, "only goal resume may clear its launch fence") {
		t.Fatalf("ordinary done cleared a stopped claim: %+v %v", res, err)
	}
	p, err = Project(endpointFor(root), true, done.Now)
	if err != nil {
		t.Fatal(err)
	}
	stillStopped := p.Tree.Live["stop-me"]
	if stillStopped == nil || stillStopped.StopFence == nil || stillStopped.StopFence.StopID != stop.StopID {
		t.Fatalf("the refused done must preserve the stopped authority: %+v", stillStopped)
	}

	resume := ResumeRequest{
		VerbRequest: VerbRequest{
			Endpoint: endpointFor(root), Actor: Actor{Machine: "mac-a", Lineage: "human-shell", Human: "wido"},
			Ulid: "01J5X00000000000000000S030", Now: stop.Now.Add(time.Minute),
		},
		GoalID: "stop-me",
		Budget: approvedBudget,
	}
	resume.Authority = testHumanAuthority(t, root, resume.Now)
	if res, err := Resume(resume); err != nil || res.Outcome != OutcomeRejected {
		t.Fatalf("resume before COMPLETE must refuse: %+v %v", res, err)
	}
	stamp := resume.Now.UTC().Format(time.RFC3339)
	batch := StopBatch{
		StopID: stop.StopID, GoalID: "stop-me", GoalRevision: stopped.Claimed.Revision,
		FenceEpoch: 1, CapabilityGeneration: stopped.StopCapability.Generation,
		Machine: "mac-a", ClaimEpoch: 9, Reason: StopReasonElapsedLimit,
		State: StopBatchComplete, OpenedAt: stop.Now.UTC().Format(time.RFC3339), UpdatedAt: stamp,
		CompletedAt: stamp, Pass: 1,
	}
	if err := WriteStopBatch(root, batch); err != nil {
		t.Fatal(err)
	}
	wrongCapability := *stopped.StopCapability
	wrongCapability.ClaimEpoch++
	if err := VerifyStopBatchComplete(root, "stop-me", wrongCapability, *stopped.StopFence); err == nil {
		t.Fatal("resume verification accepted the wrong capability tuple")
	}
	resume.Ulid = "01J5X00000000000000000S031"
	if res, err := Resume(resume); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("complete-batch resume: %+v %v", res, err)
	}
	p, err = Project(endpointFor(root), true, resume.Now)
	if err != nil {
		t.Fatal(err)
	}
	fresh := p.Tree.Live["stop-me"]
	if fresh.StopFence != nil || fresh.StopCapability == nil || fresh.StopCapability.FenceEpoch != 0 ||
		fresh.StopCapability.Revision != fresh.Claimed.Revision || fresh.Budget.ElapsedLimit != "1m" ||
		fresh.History[len(fresh.History)-1].Verb != "resume" {
		t.Fatalf("resume did not create one fresh revision and tuple: %+v", fresh)
	}
}

func TestRelayedResumeIsBoundOncePerGoalPerRuling(t *testing.T) {
	_, root, _ := twoClones(t)
	seedLedger(t, root)
	if result, err := Open(verbReq(root, "01J5X00000000000000000S100", "mac-a"), "one-relayed-resume", "Bound this work.", "main", "Run it."); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", result, err)
	}
	claim := verbReq(root, "01J5X00000000000000000S110", "mac-a")
	claim.ClaimEpoch = 9
	if result, err := claimApprovedForTest(t, claim, "one-relayed-resume", Budget{ElapsedLimit: "1m", AttemptLimit: 2, ReservedJobMinutesLimit: 20, ActiveJobLimit: 1}); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("claim: %+v %v", result, err)
	}

	closeAndComplete := func(ulid, stopID string, now time.Time) {
		t.Helper()
		projection, err := Project(endpointFor(root), true, now)
		if err != nil {
			t.Fatal(err)
		}
		file := projection.Tree.Live["one-relayed-resume"]
		request := CloseStopRequest{
			VerbRequest: VerbRequest{
				Endpoint: endpointFor(root), Actor: Actor{Machine: "mac-a", Lineage: "goal-stop-custodian"},
				Ulid: ulid, Now: now, ClaimEpoch: 9,
			},
			GoalID: "one-relayed-resume", StopID: stopID, Reason: StopReasonElapsedLimit,
			Capability: *file.StopCapability,
		}
		if result, err := CloseStop(request); err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("close stop: %+v %v", result, err)
		}
		projection, err = Project(endpointFor(root), true, now)
		if err != nil {
			t.Fatal(err)
		}
		stopped := projection.Tree.Live["one-relayed-resume"]
		stamp := now.UTC().Format(time.RFC3339)
		batch := StopBatch{
			StopID: stopID, GoalID: "one-relayed-resume", GoalRevision: stopped.Claimed.Revision,
			FenceEpoch: stopped.StopFence.Epoch, CapabilityGeneration: stopped.StopCapability.Generation,
			Machine: "mac-a", ClaimEpoch: 9, Reason: StopReasonElapsedLimit,
			State: StopBatchComplete, OpenedAt: stamp, UpdatedAt: stamp, CompletedAt: stamp, Pass: 1,
		}
		if err := WriteStopBatch(root, batch); err != nil {
			t.Fatal(err)
		}
	}

	firstAt := claim.Now.Add(time.Minute)
	closeAndComplete("01J5X00000000000000000S120", "stop-one-relayed-resume-r2-f1", firstAt)
	firstProof := testTemporaryGoalProof(t, root, "Wido authorizes first resume", "2026-09-06")
	first := ResumeRequest{
		VerbRequest: VerbRequest{
			Endpoint: endpointFor(root), Actor: Actor{Machine: "mac-a", Lineage: "human-shell", Human: "Wido"},
			Ulid: "01J5X00000000000000000S130", Now: firstAt, ClaimEpoch: 9,
		},
		GoalID: "one-relayed-resume", Budget: Budget{ElapsedLimit: "1m", AttemptLimit: 2, ReservedJobMinutesLimit: 20, ActiveJobLimit: 1},
		Authority: &firstProof,
	}
	if result, err := Resume(first); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("first relayed resume did not confirm: %+v %v", result, err)
	}

	secondAt := firstAt.Add(time.Minute)
	closeAndComplete("01J5X00000000000000000S140", "stop-one-relayed-resume-r4-f1", secondAt)
	secondProof := testTemporaryGoalProof(t, root, "Wido authorizes second resume", "2026-09-06")
	second := first
	second.Ulid = "01J5X00000000000000000S150"
	second.Now = secondAt
	second.Authority = &secondProof
	result, err := Resume(second)
	want := `goal one-relayed-resume already used relayed resume authority on 2026-08-20T22:01:00Z with recorded word "Wido authorizes first resume"; a further resume needs freshly observed enrolled-terminal authority`
	if err != nil || result.Outcome != OutcomeRejected || result.Detail != want {
		t.Fatalf("second relayed resume refusal mismatch: result=%+v err=%v", result, err)
	}
}

func TestResumeRequiresHumanAuthority(t *testing.T) {
	_, err := Resume(ResumeRequest{
		VerbRequest: VerbRequest{Actor: Actor{Human: "argv-is-not-authority"}},
		Budget:      Budget{ElapsedLimit: "1h", AttemptLimit: 1, ReservedJobMinutesLimit: 1, ActiveJobLimit: 1},
	})
	if err == nil {
		t.Fatal("a human-shaped string passed without an ancestry proof")
	}
}

func TestResumeRefusesInvalidFreshBudgetWithHumanAuthority(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	_, err := Resume(ResumeRequest{
		VerbRequest: VerbRequest{
			Endpoint: Endpoint{Root: root},
			Actor:    Actor{Machine: "mac-a", Lineage: "human-shell", Human: "wido"},
			Now:      now,
		},
		Budget:    Budget{AttemptLimit: 1, ReservedJobMinutesLimit: 1, ActiveJobLimit: 1},
		Authority: testHumanAuthority(t, root, now),
	})
	if err == nil || !strings.Contains(err.Error(), "invalid fresh budget") || !strings.Contains(err.Error(), "elapsedLimit") {
		t.Fatalf("resume did not refuse the invalid fresh budget by field name: %v", err)
	}
}

func TestStopBatchRefusesContradictionsAndCompleteIsAbsorbing(t *testing.T) {
	stamp := "2026-08-29T12:00:00Z"
	complete := StopBatch{
		StopID: "stop-bounded-r2-f1", GoalID: "bounded", GoalRevision: 2,
		FenceEpoch: 1, CapabilityGeneration: 3, Machine: "mac-a", ClaimEpoch: 4,
		Reason: StopReasonElapsedLimit, State: StopBatchComplete,
		OpenedAt: stamp, UpdatedAt: stamp, CompletedAt: stamp, Pass: 1,
	}
	root := t.TempDir()
	if err := WriteStopBatch(root, complete); err != nil {
		t.Fatalf("write complete batch: %v", err)
	}
	settled, err := ReadStopBatch(root, complete.StopID)
	if err != nil || settled.State != StopBatchComplete {
		t.Fatalf("read complete batch: %+v %v", settled, err)
	}
	if err := WriteStopBatch(root, settled); err != nil {
		t.Fatalf("identical complete batch must be idempotent: %v", err)
	}
	changed := settled
	changed.Pass++
	if err := WriteStopBatch(root, changed); err == nil || !strings.Contains(err.Error(), "COMPLETE and immutable") {
		t.Fatalf("complete batch accepted changed evidence: %v", err)
	}

	tests := []struct {
		name   string
		change func(*StopBatch)
	}{
		{name: "authority", change: func(batch *StopBatch) { batch.CapabilityGeneration = 0 }},
		{name: "reason", change: func(batch *StopBatch) { batch.Reason = "manual" }},
		{name: "state", change: func(batch *StopBatch) { batch.State = "FINISHED" }},
		{name: "timestamps", change: func(batch *StopBatch) { batch.UpdatedAt = "not-a-time" }},
		{name: "pending completion", change: func(batch *StopBatch) { batch.Pending = []string{"job-1"} }},
		{name: "non-complete completion time", change: func(batch *StopBatch) {
			batch.State = StopBatchOpen
		}},
		{name: "observed generation", change: func(batch *StopBatch) {
			batch.Observed = []StopJob{{JobID: "job-1"}}
		}},
		{name: "cancellation outcome", change: func(batch *StopBatch) {
			batch.CancelOutcomes = []StopOutcome{{JobID: "job-1"}}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			batch := complete
			test.change(&batch)
			batch.StopID += "-" + strings.ReplaceAll(test.name, " ", "-")
			if err := WriteStopBatch(t.TempDir(), batch); err == nil {
				t.Fatal("contradictory stop batch was accepted")
			}
		})
	}
}
