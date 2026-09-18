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
	terminal := "tty-test"
	if pid == 10 {
		parent = 1
	} else if pid == 1 {
		parent = 0
		terminal = ""
	}
	return humanauthority.Snapshot{
		Exact:      identity.Exact{Pid: pid, StartedAt: time.Unix(pid*10, 0), Argv: []string{"human-shell"}, ArgvKnown: true},
		Executable: "/fixture/human-shell", ExecutableKnown: true,
		OwnerUID: 501, OwnerKnown: true,
		ParentPID: parent, ParentKnown: true, TerminalID: terminal, TerminalKnown: true,
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

func testTerminalAuthority(t *testing.T, root string, now time.Time) *humanauthority.Proof {
	t.Helper()
	directory := filepath.Join(root, "scripts", "agents", "adapters")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\n[ \"$1\" = signature ] && printf '%s\\n' 'match never-a-human-shell'\n"
	if err := os.WriteFile(filepath.Join(directory, "authority-test.sh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	proof, err := humanauthority.ProveTerminal(root, 20, goalAuthorityReader{}, now)
	if err != nil {
		t.Fatal(err)
	}
	return &proof
}

func TestBreachStopFenceAndHumanResumeAreOneWayTransactions(t *testing.T) {
	t.Parallel()
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
		!strings.Contains(reports[len(reports)-1].Detail, "cannot be replayed from journal text") {
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
		fresh.Claimed.EpisodeAt != resume.stamp() || fresh.Claimed.EpisodeRevision != fresh.Claimed.Revision ||
		fresh.Claimed.EpisodeObligationRevision != 0 || fresh.History[len(fresh.History)-1].Verb != "resume" {
		t.Fatalf("resume did not create one fresh revision and tuple: %+v", fresh)
	}
}

func fencedSetBudgetBed(t *testing.T, state StopBatchState) (string, Budget, Budget, *GoalFile, VerbRequest) {
	t.Helper()
	_, root, _ := twoClones(t)
	seedLedger(t, root)
	goalID := "fenced-rebudget"
	if result, err := Open(verbReq(root, "01J5X00000000000000000FB00", "mac-a"), goalID, "Rebudget stopped work atomically.", OriginHuman, "Lift the completed fence."); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("open fenced rebudget goal: %+v %v", result, err)
	}
	budget := testBudget()
	claim := verbReq(root, "01J5X00000000000000000FB10", "mac-a")
	claim.ClaimEpoch = 9
	if result, err := claimApprovedForTest(t, claim, goalID, budget); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("claim fenced rebudget goal: %+v %v", result, err)
	}
	projection, err := Project(endpointFor(root), true, claim.Now)
	if err != nil {
		t.Fatal(err)
	}
	claimed := projection.Tree.Live[goalID]
	stop := CloseStopRequest{
		VerbRequest: VerbRequest{Endpoint: endpointFor(root), Actor: Actor{Machine: "mac-a", Lineage: "goal-stop-custodian"}, Ulid: "01J5X00000000000000000FB20", Now: claim.Now.Add(time.Minute), ClaimEpoch: 9},
		GoalID:      goalID, StopID: "stop-fenced-rebudget-r3-f1", Reason: StopReasonElapsedLimit, Capability: *claimed.StopCapability,
	}
	if result, err := CloseStop(stop); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("close fenced rebudget goal: %+v %v", result, err)
	}
	projection, err = Project(endpointFor(root), true, stop.Now)
	if err != nil {
		t.Fatal(err)
	}
	stopped := projection.Tree.Live[goalID]
	stamp := stop.Now.UTC().Format(time.RFC3339)
	batch := StopBatch{
		StopID: stop.StopID, GoalID: goalID, GoalRevision: stopped.Claimed.Revision,
		FenceEpoch: stopped.StopFence.Epoch, CapabilityGeneration: stopped.StopCapability.Generation,
		Machine: stopped.Claimed.Machine, ClaimEpoch: stopped.StopCapability.ClaimEpoch, Reason: stopped.StopFence.Reason,
		State: state, OpenedAt: stamp, UpdatedAt: stamp, Pass: 1,
	}
	if state == StopBatchComplete {
		batch.CompletedAt = stamp
	}
	if err := WriteStopBatch(root, batch); err != nil {
		t.Fatal(err)
	}
	next := budget
	next.ElapsedLimit = "3h"
	set := verbReq(root, "01J5X00000000000000000FB30", "mac-a")
	set.Now = stop.Now.Add(time.Minute)
	return root, budget, next, stopped, set
}

func TestSetBudgetLiftsCompletedFenceInOneTransaction(t *testing.T) {
	t.Parallel()
	root, _, next, stopped, set := fencedSetBudgetBed(t, StopBatchComplete)
	result, err := setBudgetApprovedForTest(t, set, stopped.Id, next)
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("one-step fenced set-budget: %+v %v", result, err)
	}
	tree, err := loadTree(root, result.Tip)
	if err != nil {
		t.Fatal(err)
	}
	fresh := tree.Live[stopped.Id]
	last := fresh.History[len(fresh.History)-1]
	if fresh.Revision != stopped.Revision+1 || len(fresh.History) != len(stopped.History)+1 ||
		fresh.StopFence != nil || fresh.IsFencedClaim() || fresh.StopCapability == nil || fresh.StopCapability.FenceEpoch != 0 ||
		fresh.Claimed == nil || fresh.Claimed.Revision != fresh.Revision || fresh.Claimed.AccountingRevision != fresh.Revision ||
		*fresh.Budget != next || fresh.Approved == nil || fresh.Approved.EpisodeRevision != fresh.Revision ||
		last.Verb != "set-budget" || last.Resumed != stopped.StopFence.StopID {
		t.Fatalf("one-step set-budget did not atomically lift and rebind the stopped goal: before=%+v after=%+v last=%+v", stopped, fresh, last)
	}
	if problems := ValidateTree(tree); len(problems) != 0 {
		t.Fatalf("rebudgeted tree does not admit the fresh claim: %v", problems)
	}
}

func TestSetBudgetFencedSameTupleRefusesWithoutMutation(t *testing.T) {
	t.Parallel()
	root, budget, _, stopped, set := fencedSetBudgetBed(t, StopBatchComplete)
	before := RenderFile(stopped)
	beforeTip := acceptedTip(t, root)
	result, err := setBudgetApprovedForTest(t, set, stopped.Id, budget)
	if err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "SET_BUDGET_FENCED_SAME_TUPLE") ||
		!strings.Contains(result.Detail, "only goal resume") {
		t.Fatalf("same-tuple fenced set-budget refusal: %+v %v", result, err)
	}
	if acceptedTip(t, root) != beforeTip {
		t.Fatal("same-tuple refusal advanced the accepted ledger")
	}
	afterTree, err := loadTree(root, beforeTip)
	if err != nil {
		t.Fatal(err)
	}
	if after := RenderFile(afterTree.Live[stopped.Id]); string(after) != string(before) {
		t.Fatalf("same-tuple refusal changed goal bytes or digest:\n%s\n---\n%s", before, after)
	}
}

func TestSetBudgetFencedIncompleteBatchNamesRemedy(t *testing.T) {
	t.Parallel()
	root, _, next, stopped, set := fencedSetBudgetBed(t, StopBatchOpen)
	before := RenderFile(stopped)
	result, err := setBudgetApprovedForTest(t, set, stopped.Id, next)
	if err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, stopped.StopFence.StopID) ||
		!strings.Contains(result.Detail, "metasystem job stop-batch") {
		t.Fatalf("incomplete stop batch refusal omitted its exact remedy: %+v %v", result, err)
	}
	afterTree, loadErr := loadTree(root, acceptedTip(t, root))
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if after := RenderFile(afterTree.Live[stopped.Id]); string(after) != string(before) {
		t.Fatalf("incomplete-batch refusal changed goal bytes or digest:\n%s\n---\n%s", before, after)
	}
}

func TestSetBudgetFencedOtherClaimNamesConflict(t *testing.T) {
	t.Parallel()
	root, _, next, stopped, set := fencedSetBudgetBed(t, StopBatchComplete)
	otherID := "other-live-claim"
	if result, err := Open(verbReq(root, "01J5X00000000000000000FB40", "mac-a"), otherID, "Occupy the stopped goal's machine.", OriginHuman, "Remain live."); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("open other claim: %+v %v", result, err)
	}
	if result, err := claimApprovedForTest(t, verbReq(root, "01J5X00000000000000000FB50", "mac-a"), otherID, testBudget()); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("claim other goal: %+v %v", result, err)
	}
	beforeTree, err := loadTree(root, acceptedTip(t, root))
	if err != nil {
		t.Fatal(err)
	}
	before := RenderFile(beforeTree.Live[stopped.Id])
	result, err := setBudgetApprovedForTest(t, set, stopped.Id, next)
	if err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, otherID) ||
		!strings.Contains(result.Detail, "already holds live claim") {
		t.Fatalf("other live claim refusal omitted the conflict: %+v %v", result, err)
	}
	afterTree, loadErr := loadTree(root, acceptedTip(t, root))
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if after := RenderFile(afterTree.Live[stopped.Id]); string(after) != string(before) {
		t.Fatalf("other-claim refusal changed the fenced goal:\n%s\n---\n%s", before, after)
	}
}

func TestAbandonOfABreachStoppedClaimKeepsTheFenceFreesTheQuotaAndEnforcesTheDependencyRule(t *testing.T) {
	t.Parallel()
	_, root, _ := twoClones(t)
	seedLedger(t, root)
	configureAbandonFloorTest(t, strings.Repeat("a", 40))
	floorReq := verbReq(root, "01J5X00000000000000001T000", "mac-a")
	floorReq.Actor.Human = "Wido"
	if result, err := EngineFloor(floorReq, strings.Repeat("a", 40), goalHumanProof(t, root, floorReq.Now)); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("engine floor: %+v %v", result, err)
	}
	if result, err := Open(verbReq(root, "01J5X00000000000000001T010", "mac-a"), "stop-me", "intent", "main", "next"); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("open stop-me: %+v %v", result, err)
	}
	claim := verbReq(root, "01J5X00000000000000001T020", "mac-a")
	claim.ClaimEpoch = 9
	budget := Budget{ElapsedLimit: "1m", AttemptLimit: 2, ReservedJobMinutesLimit: 20, ActiveJobLimit: 1}
	if result, err := claimApprovedForTest(t, claim, "stop-me", budget); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("claim: %+v %v", result, err)
	}
	projection, err := Project(endpointFor(root), true, claim.Now)
	if err != nil {
		t.Fatal(err)
	}
	claimed := projection.Tree.Live["stop-me"]
	closeRequest := CloseStopRequest{
		VerbRequest: VerbRequest{Endpoint: endpointFor(root), Actor: Actor{Machine: "mac-a", Lineage: "goal-stop-custodian"}, Ulid: "01J5X00000000000000001T030", Now: claim.Now.Add(time.Minute), ClaimEpoch: 9},
		GoalID:      "stop-me", StopID: "stop-stop-me-r2-f1", Reason: StopReasonElapsedLimit, Capability: *claimed.StopCapability,
	}
	if result, err := CloseStop(closeRequest); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("close stop: %+v %v", result, err)
	}
	projection, err = Project(endpointFor(root), true, closeRequest.Now)
	if err != nil {
		t.Fatal(err)
	}
	stopped := projection.Tree.Live["stop-me"]
	capabilityBefore, fenceBefore := *stopped.StopCapability, *stopped.StopFence

	wedge := []struct {
		name string
		run  func() (PublishResult, error)
	}{
		{"release", func() (PublishResult, error) {
			return Release(verbReq(root, "01J5X00000000000000001T031", "mac-a"), "stop-me")
		}},
		{"done", func() (PublishResult, error) {
			return Done(verbReq(root, "01J5X00000000000000001T032", "mac-a"), "stop-me", "must not bypass the stop batch")
		}},
		{"park", func() (PublishResult, error) {
			return Park(verbReq(root, "01J5X00000000000000001T033", "mac-a"), "stop-me", "must not orphan the stop batch")
		}},
		{"set-budget", func() (PublishResult, error) {
			request := verbReq(root, "01J5X00000000000000001T034", "mac-a")
			request.Actor.Human = "Wido"
			return SetBudgetApproved(request, "stop-me", budget, goalHumanProof(t, root, request.Now))
		}},
		{"steal", func() (PublishResult, error) {
			request := verbReq(root, "01J5X00000000000000001T035", "mac-b")
			request.Actor.Human = "Wido"
			return Steal(request, "stop-me")
		}},
	}
	for _, operation := range wedge {
		result, operationErr := operation.run()
		if operationErr != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "breach-stopped") || !strings.Contains(result.Detail, "only goal resume") {
			t.Fatalf("%s crossed the breach-stop wedge: %+v %v", operation.name, result, operationErr)
		}
	}

	for index, id := range []string{"dependent", "successor"} {
		if result, err := Open(verbReq(root, []string{"01J5X00000000000000001T040", "01J5X00000000000000001T050"}[index], "mac-b"), id, "intent", "main", "next"); err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("open %s: %+v %v", id, result, err)
		}
	}
	blocked := []string{"stop-me"}
	if result, err := Edit(verbReq(root, "01J5X00000000000000001T060", "mac-b"), "dependent", EditFields{Blocked: &blocked}); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("wire dependent: %+v %v", result, err)
	}
	approveDependent := verbReq(root, "01J5X00000000000000001T065", "mac-b")
	approveDependent.Actor.Human = "Wido"
	if result, err := Approve(approveDependent, []string{"dependent"}, nil, goalHumanProof(t, root, approveDependent.Now)); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("approve dependent: %+v %v", result, err)
	}
	abandonReq := verbReq(root, "01J5X00000000000000001T070", "mac-a")
	abandonReq.Actor.Human = "Wido"
	result, err := Abandon(abandonReq, "stop-me", AbandonSpec{Because: "stopped permanently"}, goalHumanProof(t, root, abandonReq.Now))
	if err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "goal dependent is blocked by stop-me") {
		t.Fatalf("uncovered dependency did not refuse: %+v %v", result, err)
	}
	abandonReq.Ulid = "01J5X00000000000000001T080"
	result, err = Abandon(abandonReq, "stop-me", AbandonSpec{Because: "stopped permanently", Carried: "successor"}, goalHumanProof(t, root, abandonReq.Now))
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("carried abandon: %+v %v", result, err)
	}
	tree, err := loadTree(root, result.Tip)
	if err != nil {
		t.Fatal(err)
	}
	abandoned := tree.Abandoned["stop-me"]
	if abandoned == nil || abandoned.State != StateAbandoned || abandoned.Claimed != nil || abandoned.StopCapability == nil || abandoned.StopFence == nil ||
		*abandoned.StopCapability != capabilityBefore || *abandoned.StopFence != fenceBefore || abandoned.Abandoned == nil || abandoned.Abandoned.StopID != fenceBefore.StopID {
		t.Fatalf("abandon did not preserve the frozen fence and clear the claim: %+v", abandoned)
	}
	if problems := ValidateTree(tree); len(problems) != 0 {
		t.Fatalf("abandoned stopped tree is invalid: %v", problems)
	}
	if tree.Live["stop-me"] != nil {
		t.Fatalf("abandoned goal remained in the live map and could reach budget projection: %+v", tree.Live["stop-me"])
	}
	admissionOutput, admissionErr := goalRevisionAdmissionCLI(root, "stop-me", capabilityBefore.Revision, abandonReq.Now)
	if admissionErr == nil || !strings.Contains(admissionOutput, "not a claimed accepted goal") || strings.Contains(admissionOutput, "BUDGET_") {
		t.Fatalf("dispatch admission did not stop before budget projection for the abandoned goal: output=%q err=%v", admissionOutput, admissionErr)
	}
	if got := tree.Live["dependent"].Blocked; len(got) != 1 || got[0] != "successor" {
		t.Fatalf("dependent was not re-pointed: %v", got)
	}
	frontier, err := Next(Projection{Root: root, Tree: tree, Horizon: projection.Horizon}, "mac-a")
	if err != nil || len(frontier.Blocked) != 1 || frontier.Blocked[0] != "dependent" {
		t.Fatalf("dependent is not blocked on its live successor: %+v %v", frontier, err)
	}

	if result, err := Open(verbReq(root, "01J5X00000000000000001T090", "mac-a"), "fourth", "intent", "main", "next"); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("open fourth: %+v %v", result, err)
	}
	fourthClaim := verbReq(root, "01J5X00000000000000001T100", "mac-a")
	if result, err := claimApprovedForTest(t, fourthClaim, "fourth", testBudget()); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("abandoned claim still consumed the machine quota: %+v %v", result, err)
	}
}

func makeFencedAbandonedGoal(t *testing.T, root, goalID, ulidPrefix string) (*GoalFile, time.Time) {
	t.Helper()
	if result, err := Open(verbReq(root, ulidPrefix+"0", "mac-a"), goalID, "intent", "main", "next"); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("open %s: %+v %v", goalID, result, err)
	}
	claim := verbReq(root, ulidPrefix+"1", "mac-a")
	claim.ClaimEpoch = 19
	if result, err := claimApprovedForTest(t, claim, goalID, testBudget()); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("claim %s: %+v %v", goalID, result, err)
	}
	projection, err := Project(endpointFor(root), true, claim.Now)
	if err != nil {
		t.Fatal(err)
	}
	claimed := projection.Tree.Live[goalID]
	closedAt := claim.Now.Add(time.Minute)
	closeRequest := CloseStopRequest{
		VerbRequest: VerbRequest{Endpoint: endpointFor(root), Actor: Actor{Machine: "mac-a", Lineage: "goal-stop-custodian"}, Ulid: ulidPrefix + "2", Now: closedAt, ClaimEpoch: 19},
		GoalID:      goalID, StopID: "stop-" + goalID + "-r2-f1", Reason: StopReasonElapsedLimit, Capability: *claimed.StopCapability,
	}
	if result, err := CloseStop(closeRequest); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("close stop %s: %+v %v", goalID, result, err)
	}
	abandon := verbReq(root, ulidPrefix+"3", "mac-a")
	abandon.Now = closedAt.Add(time.Minute)
	abandon.Actor.Human = "Wido"
	if result, err := Abandon(abandon, goalID, AbandonSpec{Because: "the stopped work will not resume"}, goalHumanProof(t, root, abandon.Now)); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("abandon %s: %+v %v", goalID, result, err)
	}
	projection, err = Project(endpointFor(root), true, abandon.Now)
	if err != nil {
		t.Fatal(err)
	}
	return projection.Tree.Abandoned[goalID], abandon.Now
}

func stopBatchForAbandoned(t *testing.T, file *GoalFile, state StopBatchState, now time.Time) StopBatch {
	t.Helper()
	stamp := now.UTC().Format(time.RFC3339)
	batch := StopBatch{
		StopID: file.StopFence.StopID, GoalID: file.Id, GoalRevision: file.StopCapability.Revision,
		FenceEpoch: file.StopFence.Epoch, CapabilityGeneration: file.StopCapability.Generation,
		Machine: file.StopCapability.Machine, ClaimEpoch: file.StopCapability.ClaimEpoch,
		Reason: file.StopFence.Reason, State: state, OpenedAt: stamp, UpdatedAt: stamp, Pass: 1,
	}
	if state == StopBatchComplete {
		batch.CompletedAt = stamp
	}
	return batch
}

func TestReopenFromAbandonedRequiresTheStopBatchComplete(t *testing.T) {
	t.Parallel()
	_, root := oneClone(t)
	seedLedger(t, root)
	configureAbandonFloorTest(t, strings.Repeat("a", 40))
	recordAbandonFloorTest(t, root, "01J5X000000000000000001S00")
	abandoned, now := makeFencedAbandonedGoal(t, root, "fenced-reopen", "01J5X000000000000000001S1")
	if abandoned == nil || abandoned.StopCapability == nil || abandoned.StopFence == nil {
		t.Fatalf("fixture did not retain the frozen fence: %+v", abandoned)
	}
	revisionBefore := abandoned.Revision
	reopen := verbReq(root, "01J5X000000000000000001S20", "mac-a")
	reopen.Now = now.Add(time.Minute)
	reopen.Actor.Human = "Wido"
	proof := goalHumanProof(t, root, reopen.Now)
	result, err := ReopenAbandoned(reopen, "fenced-reopen", proof)
	if err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "cannot prove stop batch "+abandoned.StopFence.StopID+" complete") {
		t.Fatalf("missing batch refusal: %+v %v", result, err)
	}
	projection, _ := Project(endpointFor(root), true, reopen.Now)
	if projection.Tree.Abandoned["fenced-reopen"].Revision != revisionBefore {
		t.Fatal("missing batch refusal changed the record")
	}

	if err := WriteStopBatch(root, stopBatchForAbandoned(t, abandoned, StopBatchOpen, reopen.Now)); err != nil {
		t.Fatal(err)
	}
	reopen.Ulid = "01J5X000000000000000001S30"
	result, err = ReopenAbandoned(reopen, "fenced-reopen", proof)
	if err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "not COMPLETE") {
		t.Fatalf("open batch refusal: %+v %v", result, err)
	}
	projection, _ = Project(endpointFor(root), true, reopen.Now)
	if projection.Tree.Abandoned["fenced-reopen"].Revision != revisionBefore {
		t.Fatal("non-complete batch refusal changed the record")
	}

	if err := WriteStopBatch(root, stopBatchForAbandoned(t, abandoned, StopBatchComplete, reopen.Now)); err != nil {
		t.Fatal(err)
	}
	reopen.Ulid = "01J5X000000000000000001S40"
	result, err = ReopenAbandoned(reopen, "fenced-reopen", proof)
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("complete batch reopen: %+v %v", result, err)
	}
	tree, err := loadTree(root, result.Tip)
	if err != nil {
		t.Fatal(err)
	}
	file := tree.Live["fenced-reopen"]
	var verbs []string
	for _, line := range file.History {
		if line.Verb == "breach-stop" || line.Verb == "abandon" || line.Verb == "reopen" {
			verbs = append(verbs, line.Verb)
		}
	}
	if strings.Join(verbs, ",") != "breach-stop,abandon,reopen" || file.History[len(file.History)-2].StopID != abandoned.StopFence.StopID {
		t.Fatalf("stop, abandon, and reopen history did not survive in order: %+v", file.History)
	}
}

func TestReopenFromAbandonedIsBoundToTheClaimantCheckoutAndCarriedRecovers(t *testing.T) {
	t.Parallel()
	_, rootA, rootB := twoClones(t)
	seedLedger(t, rootA)
	configureAbandonFloorTest(t, strings.Repeat("a", 40))
	recordAbandonFloorTest(t, rootA, "01J5X000000000000000001T00")
	for index, id := range []string{"successor-one", "successor-two"} {
		ulid := []string{"01J5X000000000000000001T10", "01J5X000000000000000001T20"}[index]
		if result, err := Open(verbReq(rootA, ulid, "mac-a"), id, "intent", "main", "next"); err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("open %s: %+v %v", id, result, err)
		}
	}
	abandoned, now := makeFencedAbandonedGoal(t, rootA, "lost-checkout-goal", "01J5X000000000000000001T3")
	if err := WriteStopBatch(rootA, stopBatchForAbandoned(t, abandoned, StopBatchComplete, now)); err != nil {
		t.Fatal(err)
	}

	reopenB := verbReq(rootB, "01J5X000000000000000001T40", "mac-b")
	reopenB.Now = now.Add(time.Minute)
	reopenB.Actor.Human = "Wido"
	proofB := goalHumanProof(t, rootB, reopenB.Now)
	result, err := ReopenAbandoned(reopenB, "lost-checkout-goal", proofB)
	if err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "cannot prove stop batch "+abandoned.StopFence.StopID+" complete") {
		t.Fatalf("foreign checkout reopen did not refuse on its local batch: %+v %v", result, err)
	}

	carry := verbReq(rootB, "01J5X000000000000000001T50", "mac-b")
	carry.Now = reopenB.Now.Add(time.Minute)
	carry.Actor.Human = "Wido"
	result, err = CarryAbandoned(carry, "lost-checkout-goal", "successor-one", proofB)
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("first carry: %+v %v", result, err)
	}
	tree, err := loadTree(rootB, result.Tip)
	if err != nil {
		t.Fatal(err)
	}
	carried := tree.Abandoned["lost-checkout-goal"]
	if carried.Abandoned.Carried != "successor-one" || carried.History[len(carried.History)-1].Verb != "carry" || carried.History[len(carried.History)-1].Carried != "successor-one" {
		t.Fatalf("first carry did not bind its event: %+v", carried)
	}
	if _, problems := ParseFile(RenderFile(carried)); len(problems) != 0 {
		t.Fatalf("first carried archive no longer parses: %v", problems)
	}

	carry.Ulid = "01J5X000000000000000001T60"
	result, err = CarryAbandoned(carry, "lost-checkout-goal", "successor-two", proofB)
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("second carry: %+v %v", result, err)
	}
	tree, _ = loadTree(rootB, result.Tip)
	carried = tree.Abandoned["lost-checkout-goal"]
	if carried.Abandoned.Carried != "successor-two" || carried.History[len(carried.History)-2].Carried != "successor-one" || carried.History[len(carried.History)-1].Carried != "successor-two" {
		t.Fatalf("second carry did not replace the field and retain both events: %+v", carried.History)
	}
	if _, problems := ParseFile(RenderFile(carried)); len(problems) != 0 {
		t.Fatalf("twice-carried archive no longer parses: %v", problems)
	}

	withoutHuman := carry
	withoutHuman.Ulid = "01J5X000000000000000001T70"
	withoutHuman.Actor.Human = ""
	if _, err := CarryAbandoned(withoutHuman, "lost-checkout-goal", "successor-one", proofB); err == nil || err.Error() != "carry is a human act and names its human (--by)" {
		t.Fatalf("carry without human refusal = %v", err)
	}
	if _, err := CarryAbandoned(carry, "lost-checkout-goal", "successor-one", nil); err == nil || err.Error() != "carry requires freshly observed enrolled-terminal human authority" {
		t.Fatalf("carry without proof refusal = %v", err)
	}
	revisionBeforeRefusals := carried.Revision
	for index, test := range []struct{ id, successor, want string }{
		{"successor-one", "successor-two", "goal successor-one is live; carry records a successor on an abandoned goal only"},
		{"lost-checkout-goal", "missing-successor", "carried must name a live successor"},
	} {
		request := carry
		request.Ulid = []string{"01J5X000000000000000001T80", "01J5X000000000000000001T90"}[index]
		result, err := CarryAbandoned(request, test.id, test.successor, proofB)
		if err != nil || result.Outcome != OutcomeRejected || result.Detail != test.want {
			t.Fatalf("carry refusal %d: %+v %v", index, result, err)
		}
	}
	self := carry
	self.Ulid = "01J5X000000000000000001TA0"
	if _, err := CarryAbandoned(self, "lost-checkout-goal", "lost-checkout-goal", proofB); err == nil || err.Error() != "carried must name a live successor" {
		t.Fatalf("carry to self refusal = %v", err)
	}
	projectionB, err := Project(endpointFor(rootB), true, carry.Now)
	if err != nil {
		t.Fatal(err)
	}
	if projectionB.Tree.Abandoned["lost-checkout-goal"].Revision != revisionBeforeRefusals {
		t.Fatal("a refused carry changed the abandoned record")
	}

	reopenA := verbReq(rootA, "01J5X000000000000000001TB0", "mac-a")
	reopenA.Now = carry.Now.Add(time.Minute)
	reopenA.Actor.Human = "Wido"
	result, err = ReopenAbandoned(reopenA, "lost-checkout-goal", goalHumanProof(t, rootA, reopenA.Now))
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("claimant checkout reopen: %+v %v", result, err)
	}
	tree, err = loadTree(rootA, result.Tip)
	if err != nil {
		t.Fatal(err)
	}
	reopened := tree.Live["lost-checkout-goal"]
	if reopened == nil || len(reopened.History) < 5 || reopened.History[len(reopened.History)-3].Carried != "successor-one" || reopened.History[len(reopened.History)-2].Carried != "successor-two" || reopened.History[len(reopened.History)-1].Verb != "reopen" {
		t.Fatalf("reopened goal lost its carry history: %+v", reopened)
	}
	if problems := ValidateTree(tree); len(problems) != 0 {
		t.Fatalf("reopened carried tree does not validate: %v", problems)
	}
	if _, problems := ParseFile(RenderFile(reopened)); len(problems) != 0 {
		t.Fatalf("reopened carried record does not parse: %v", problems)
	}
}

func TestRelayedResumeIsBoundOncePerGoalPerRuling(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	_, err := Resume(ResumeRequest{
		VerbRequest: VerbRequest{Actor: Actor{Human: "argv-is-not-authority"}},
		Budget:      Budget{ElapsedLimit: "1h", AttemptLimit: 1, ReservedJobMinutesLimit: 1, ActiveJobLimit: 1},
	})
	if err == nil {
		t.Fatal("a human-shaped string passed without an ancestry proof")
	}
}

func TestResumeRefusesInvalidFreshBudgetWithHumanAuthority(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
