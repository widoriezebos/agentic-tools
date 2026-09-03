package goal

import (
	"strings"
	"testing"
)

func TestSTR3MigrationBootstrap01ApprovedAndClaimedLegacyGoals(t *testing.T) {
	_, root := oneClone(t)
	seedLedger(t, root)
	legacyBudget := Budget{ElapsedLimit: "4h", AttemptLimit: 4, ReservedJobMinutesLimit: 240, ActiveJobLimit: 1, ReviewRoundLimit: 3}
	for index, id := range []string{"legacy-approved", "legacy-claimed"} {
		open := verbReq(root, []string{"01J5X00000000000000000MB00", "01J5X00000000000000000MB10"}[index], "mac-a")
		if result, err := Open(open, id, "Migrate "+id+".", OriginMain, "Classify it."); err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("open %s: %+v %v", id, result, err)
		}
		approve := verbReq(root, []string{"01J5X00000000000000000MB20", "01J5X00000000000000000MB30"}[index], "mac-a")
		approveGoalForTest(t, approve, id, legacyBudget)
	}
	claim := verbReq(root, "01J5X00000000000000000MB40", "mac-a")
	if result, err := Claim(claim, "legacy-claimed"); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("claim legacy goal: %+v %v", result, err)
	}

	// Reproduce the approved, tierless, four-member records that exist at
	// migration intake. Their legacy approval digests must parse before the
	// sweep has any chance to rebind them.
	tree, err := loadTree(root, acceptedTip(t, root))
	if err != nil {
		t.Fatal(err)
	}
	legacyChanges := make([]Change, 0, 2)
	for _, id := range []string{"legacy-approved", "legacy-claimed"} {
		file := tree.Live[id]
		file.Tier = 0
		file.Approved.Digest = legacyApprovalDigest(file.Intent, *file.Budget)
		raw := string(RenderFile(file))
		raw = strings.Replace(raw, "- Tier: 3\n", "", 1)
		raw = strings.Replace(raw, " reviewRoundLimit=3", "", 1)
		raw = withFreshIntegrity(raw)
		if parsed, problems := ParseFile([]byte(raw)); parsed == nil || len(problems) != 0 || parsed.Tier != 0 || !parsed.legacyFourBudget {
			t.Fatalf("legacy %s did not bootstrap: parsed=%+v problems=%v", id, parsed, problems)
		}
		legacyChanges = append(legacyChanges, Change{Path: livePath(id), Content: []byte(raw)})
	}
	legacyResult, err := Publish(endpointFor(root), PublishRequest{
		Opid: "legacy-tierless-intake", Machine: "mac-fixture", Lineage: "lin-fixture",
		Intent: testIntentFor("migrate"), Message: "legacy tierless intake",
		Mutate: func(string) ([]Change, error) { return legacyChanges, nil },
	})
	if err != nil || legacyResult.Outcome != OutcomeConfirmed {
		t.Fatalf("publish legacy intake: %+v %v", legacyResult, err)
	}

	draft := []byte("legacy-claimed 2 claimed migration\nlegacy-approved 1 approved migration\n")
	listing, err := PreviewClassificationSweep(endpointFor(root), draft, claim.Now)
	if err != nil || len(listing.Proposals) != 2 || listing.Lines[0] != "legacy-approved 1 approved migration" {
		t.Fatalf("preview did not normalize the legacy listing: %+v %v", listing, err)
	}
	human := verbReq(root, "01J5X00000000000000000MB50", "mac-a")
	human.Actor.Human = "Wido"
	first, err := ClassifyTier(human, listing.Proposals[0], false)
	if err != nil || first.Outcome != OutcomeConfirmed {
		t.Fatalf("classify first legacy goal: %+v %v", first, err)
	}
	human.Ulid = "01J5X00000000000000000MB60"
	last, err := ClassifyTier(human, listing.Proposals[1], true)
	if err != nil || last.Outcome != OutcomeConfirmed {
		t.Fatalf("classify final legacy goal: %+v %v", last, err)
	}
	if err := ValidateCommit(root, last.Tip); err != nil {
		t.Fatalf("post-marker tree validation: %v", err)
	}
	tree, err = loadTree(root, last.Tip)
	if err != nil {
		t.Fatal(err)
	}
	approved, claimed := tree.Live["legacy-approved"], tree.Live["legacy-claimed"]
	if approved.Tier != 1 || approved.Budget.ReviewRoundLimit != 0 || approved.State != StateApproved || approved.ValidateApprovalRecord() != nil {
		t.Fatalf("approved legacy normalization = %+v", approved)
	}
	if claimed.Tier != 2 || claimed.Budget.ReviewRoundLimit != 2 || claimed.State != StateClaimed || claimed.ValidateApprovalRecord() != nil {
		t.Fatalf("claimed legacy normalization = %+v", claimed)
	}
	if tree.Root.TierLaw != human.opid() {
		t.Fatalf("TierLaw marker = %q, want final edit %q", tree.Root.TierLaw, human.opid())
	}
}

func publishGoalFixtures(t *testing.T, root string, files ...*GoalFile) {
	t.Helper()
	changes := make([]Change, 0, len(files))
	for _, file := range files {
		changes = append(changes, Change{Path: livePath(file.Id), Content: RenderFile(file)})
	}
	result, err := Publish(endpointFor(root), PublishRequest{
		Opid: "approval-fixture-" + files[0].Id, Machine: "mac-fixture", Lineage: "lin-fixture",
		Intent: testIntentFor("migrate"), Message: "seed approval fixtures",
		Mutate: func(string) ([]Change, error) { return changes, nil },
	})
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("publish approval fixtures: %+v %v", result, err)
	}
}

func legacyClaimedFixture(id string, budget Budget) *GoalFile {
	file := vGoal(id, StateClaimed)
	file.Budget = &budget
	file.Claimed.At = file.History[0].At
	file.Claimed.Revision = 1
	return file
}

func TestAgentCannotApprove(t *testing.T) {
	_, root := oneClone(t)
	seedLedger(t, root)
	if result, err := Open(verbReq(root, "01J5X00000000000000000AP00", "mac-a"), "needs-human", "Only a human admits execution.", OriginMain, "Wait."); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", result, err)
	}
	request := verbReq(root, "01J5X00000000000000000AP10", "mac-a")
	if _, err := Approve(request, []string{"needs-human"}, ptrBudget(testBudget()), testHumanAuthority(t, root, request.Now)); err == nil || !strings.Contains(err.Error(), "human-only") {
		t.Fatalf("an agent-shaped caller approved execution: %v", err)
	}
	request.Actor.Human = "Wido"
	if _, err := Approve(request, []string{"needs-human"}, ptrBudget(testBudget()), nil); err == nil || !strings.Contains(err.Error(), "freshly observed") {
		t.Fatalf("a --by name without boundary proof approved execution: %v", err)
	}
}

func ptrBudget(budget Budget) *Budget { return &budget }

func TestApprovedGoalClaimsAndPayloadEditsInvalidateBinding(t *testing.T) {
	_, root := oneClone(t)
	seedLedger(t, root)
	if result, err := Open(verbReq(root, "01J5X00000000000000000AB00", "mac-a"), "bound-work", "The reviewed intent.", OriginMain, "Run it."); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", result, err)
	}
	request := verbReq(root, "01J5X00000000000000000AB10", "mac-a")
	approveGoalForTest(t, request, "bound-work", testBudget())
	projection, err := Project(endpointFor(root), false, request.Now)
	if err != nil {
		t.Fatal(err)
	}
	approved := projection.Tree.Live["bound-work"]
	intentChanged := *approved
	intentChanged.Intent = "An intent the human never reviewed."
	if err := intentChanged.ValidateApprovalRecord(); err == nil || !strings.Contains(err.Error(), "digest") {
		t.Fatalf("an intent edit preserved the approval binding: %v", err)
	}
	budgetChanged := *approved
	changedBudget := *approved.Budget
	changedBudget.AttemptLimit++
	budgetChanged.Budget = &changedBudget
	if err := budgetChanged.ValidateApprovalRecord(); err == nil || !strings.Contains(err.Error(), "digest") {
		t.Fatalf("a budget edit preserved the approval binding: %v", err)
	}
	newIntent := "Still not authorized."
	if result, err := Edit(verbReq(root, "01J5X00000000000000000AB20", "mac-a"), "bound-work", EditFields{Intent: &newIntent}); err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "approved this intent") {
		t.Fatalf("the executable intent edit did not refuse: %+v %v", result, err)
	}
	if result, err := Claim(verbReq(root, "01J5X00000000000000000AB30", "mac-a"), "bound-work"); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("the approved goal did not claim: %+v %v", result, err)
	}
}

func TestProofBearingSetBudgetRatifiesMatchingLegacyClaim(t *testing.T) {
	_, root := oneClone(t)
	seedLedger(t, root)
	budget := testBudget()
	publishGoalFixtures(t, root, legacyClaimedFixture("legacy-budget", budget))

	request := verbReq(root, "01J5X00000000000000000SB00", "mac-a")
	request.Actor.Human = "Wido"
	result, err := SetBudgetApproved(request, "legacy-budget", budget, testHumanAuthority(t, root, request.Now))
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("proof-bearing set-budget did not ratify an otherwise identical legacy tuple: %+v %v", result, err)
	}
	tree, err := loadTree(root, result.Tip)
	if err != nil {
		t.Fatal(err)
	}
	file := tree.Live["legacy-budget"]
	if file == nil || file.Approved == nil || file.Claimed == nil || file.Claimed.Revision != file.Revision {
		t.Fatalf("set-budget did not bind approval and the fresh claimed revision: %+v", file)
	}
	if tree.Root.ApprovalGate == nil || tree.Root.ApprovalGate.Opid != request.opid() {
		t.Fatalf("set-budget did not arm the approval gate in its transaction: %+v", tree.Root.ApprovalGate)
	}
}

func TestApproveBatchRefusesClaimedMember(t *testing.T) {
	_, root := oneClone(t)
	seedLedger(t, root)
	for index, id := range []string{"batch-claimed", "batch-waiting"} {
		ulid := []string{"01J5X00000000000000000BA00", "01J5X00000000000000000BA10"}[index]
		if result, err := Open(verbReq(root, ulid, "mac-a"), id, "Batch member "+id+".", OriginMain, "Wait."); err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("open %s: %+v %v", id, result, err)
		}
	}
	claimRequest := verbReq(root, "01J5X00000000000000000BA20", "mac-a")
	approveGoalForTest(t, claimRequest, "batch-claimed", testBudget())
	if result, err := Claim(claimRequest, "batch-claimed"); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("claim batch fixture: %+v %v", result, err)
	}

	human := verbReq(root, "01J5X00000000000000000BA30", "mac-a")
	human.Actor.Human = "Wido"
	result, err := Approve(human, []string{"batch-claimed", "batch-waiting"}, ptrBudget(testBudget()), testHumanAuthority(t, root, human.Now))
	if err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "repeated --id") {
		t.Fatalf("approval batch did not refuse its claimed member atomically: %+v %v", result, err)
	}
}

func TestUnapprovedExecutionPathsRefuseApprovalRequired(t *testing.T) {
	t.Run("claim", func(t *testing.T) {
		_, root := oneClone(t)
		seedLedger(t, root)
		if result, err := Open(verbReq(root, "01J5X00000000000000000KC00", "mac-a"), "unapproved-claim", "Await review.", OriginMain, "Wait."); err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("open: %+v %v", result, err)
		}
		result, err := Claim(verbReq(root, "01J5X00000000000000000KC10", "mac-a"), "unapproved-claim")
		if err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "APPROVAL_REQUIRED") {
			t.Fatalf("unapproved claim did not fail closed: %+v %v", result, err)
		}
	})

	t.Run("steal", func(t *testing.T) {
		_, root := oneClone(t)
		seedLedger(t, root)
		publishGoalFixtures(t, root, legacyClaimedFixture("unapproved-steal", testBudget()))
		request := verbReq(root, "01J5X00000000000000000KS10", "mac-b")
		request.Actor.Human = "Wido"
		result, err := Steal(request, "unapproved-steal")
		if err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "APPROVAL_REQUIRED") {
			t.Fatalf("unapproved steal did not fail closed: %+v %v", result, err)
		}
	})

	t.Run("recover", func(t *testing.T) {
		_, root := oneClone(t)
		seedLedger(t, root)
		if result, err := Open(verbReq(root, "01J5X00000000000000000KR00", "mac-a"), "unapproved-recovery", "Await review.", OriginMain, "Wait."); err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("open: %+v %v", result, err)
		}
		opid := Opid("01J5X00000000000000000KR10", "mac-a", "lin-1")
		strandEntry(t, root, opid, PhaseCreated, Intent{Verb: "claim", Targets: []string{"unapproved-recovery"}, Args: map[string]string{"claimEpoch": "1"}})
		if _, err := Recover(endpointFor(root)); err != nil {
			t.Fatal(err)
		}
		entry, err := ReadEntry(root, opid)
		if err != nil || entry.Outcome != OutcomeRejected || !strings.Contains(entry.Evidence, "APPROVAL_REQUIRED") {
			t.Fatalf("recovery reconstructed approval: %+v %v", entry, err)
		}
	})

	t.Run("reconcile", func(t *testing.T) {
		base := vGoal("unapproved-reconcile", StateQueued)
		edited := *base
		edited.State = StateClaimed
		edited.Claimed = &ClaimRecord{Machine: "mac-a", Lineage: "lin-1", At: base.OpenedAt, Revision: 1}
		if _, err := mapOneChange(livePath(base.Id), base, &edited); err == nil || !strings.Contains(err.Error(), "APPROVAL_REQUIRED") {
			t.Fatalf("reconcile admitted an unapproved claimed state: %v", err)
		}
	})

	t.Run("resume", func(t *testing.T) {
		_, root := oneClone(t)
		seedLedger(t, root)
		budget := testBudget()
		file := legacyClaimedFixture("unapproved-resume", budget)
		file.Revision = 2
		file.StopCapability = &StopCapability{Generation: 1, Revision: 1, Machine: "mac-a", ClaimEpoch: 1, FenceEpoch: 1}
		file.StopFence = &StopFence{StopID: "stop-unapproved-resume", Revision: 1, Epoch: 1, CapabilityGeneration: 1, ClosedAt: "2026-08-20T10:02:00Z", Reason: StopReasonElapsedLimit}
		file.History = append(file.History, HistoryLine{At: file.StopFence.ClosedAt, Opid: Opid("01J5X00000000000000000KZ00", "mac-a", "lin-1"), Verb: "breach-stop", Actor: "mac-a+lin-1", Targets: []string{file.Id}, Keep: -1})
		publishGoalFixtures(t, root, file)
		if err := WriteStopBatch(root, StopBatch{
			StopID: file.StopFence.StopID, GoalID: file.Id, GoalRevision: 1, FenceEpoch: 1, CapabilityGeneration: 1,
			Machine: "mac-a", ClaimEpoch: 1, Reason: StopReasonElapsedLimit, State: StopBatchComplete,
			OpenedAt: file.StopFence.ClosedAt, UpdatedAt: file.StopFence.ClosedAt, CompletedAt: file.StopFence.ClosedAt, Pass: 1,
		}); err != nil {
			t.Fatal(err)
		}
		request := verbReq(root, "01J5X00000000000000000KZ10", "mac-a")
		request.Actor.Human = "Wido"
		result, err := Resume(ResumeRequest{VerbRequest: request, GoalID: file.Id, Budget: budget, Authority: testHumanAuthority(t, root, request.Now)})
		if err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "APPROVAL_REQUIRED") {
			t.Fatalf("unapproved resume did not fail closed: %+v %v", result, err)
		}
	})
}

func TestApprovedOverNormWithoutCoveringNormApprovalRefusesExecution(t *testing.T) {
	over := Budget{ElapsedLimit: "1h", AttemptLimit: 1, ReservedJobMinutesLimit: 2400, ActiveJobLimit: 1}
	assertNormRefused := func(t *testing.T, result PublishResult, err error) {
		t.Helper()
		if err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "GOAL_NORM_REFUSED") {
			t.Fatalf("approved over-norm execution did not refuse without its covering norm approval: %+v %v", result, err)
		}
	}

	t.Run("claim", func(t *testing.T) {
		_, root := oneClone(t)
		seedLedger(t, root)
		seedGoalNormConfig(t, root)
		file := approvedGoalFixture(vGoal("over-norm-claim", StateQueued), over)
		publishGoalFixtures(t, root, file)

		result, err := Claim(verbReq(root, "01J5X00000000000000000NC10", "mac-a"), file.Id)
		assertNormRefused(t, result, err)
	})

	t.Run("steal", func(t *testing.T) {
		_, root := oneClone(t)
		seedLedger(t, root)
		seedGoalNormConfig(t, root)
		file := approvedGoalFixture(legacyClaimedFixture("over-norm-steal", over), over)
		file.State = StateClaimed
		publishGoalFixtures(t, root, file)

		request := verbReq(root, "01J5X00000000000000000NS10", "mac-b")
		request.Actor.Human = "Wido"
		result, err := Steal(request, file.Id)
		assertNormRefused(t, result, err)
	})

	t.Run("resume", func(t *testing.T) {
		_, root := oneClone(t)
		seedLedger(t, root)
		seedGoalNormConfig(t, root)
		file := approvedGoalFixture(legacyClaimedFixture("over-norm-resume", over), over)
		file.State = StateClaimed
		file.StopCapability = &StopCapability{Generation: 1, Revision: 1, Machine: "mac-a", ClaimEpoch: 1, FenceEpoch: 1}
		file.StopFence = &StopFence{StopID: "stop-over-norm-resume", Revision: 1, Epoch: 1, CapabilityGeneration: 1, ClosedAt: "2026-08-20T10:02:00Z", Reason: StopReasonElapsedLimit}
		file.Revision++
		file.History = append(file.History, HistoryLine{At: file.StopFence.ClosedAt, Opid: Opid("01J5X00000000000000000NR00", "mac-a", "lin-1"), Verb: "breach-stop", Actor: "mac-a+lin-1", Targets: []string{file.Id}, Keep: -1})
		publishGoalFixtures(t, root, file)
		if err := WriteStopBatch(root, StopBatch{
			StopID: file.StopFence.StopID, GoalID: file.Id, GoalRevision: 1, FenceEpoch: 1, CapabilityGeneration: 1,
			Machine: "mac-a", ClaimEpoch: 1, Reason: StopReasonElapsedLimit, State: StopBatchComplete,
			OpenedAt: file.StopFence.ClosedAt, UpdatedAt: file.StopFence.ClosedAt, CompletedAt: file.StopFence.ClosedAt, Pass: 1,
		}); err != nil {
			t.Fatal(err)
		}

		request := verbReq(root, "01J5X00000000000000000NR10", "mac-a")
		request.Actor.Human = "Wido"
		result, err := Resume(ResumeRequest{VerbRequest: request, GoalID: file.Id, Budget: over, Authority: testHumanAuthority(t, root, request.Now)})
		assertNormRefused(t, result, err)
	})
}

func TestSweepBindsListedIntentAndPreservesClaimedWork(t *testing.T) {
	_, root := oneClone(t)
	seedLedger(t, root)
	budget := testBudget()
	waiting := vGoal("sweep-waiting", StateQueued)
	waiting.Budget = &budget
	running := legacyClaimedFixture("sweep-running", budget)
	publishGoalFixtures(t, root, waiting, running)

	first, err := PreviewApprovalSweep(endpointFor(root), verbReq(root, "01J5X00000000000000000SW00", "mac-a").Now)
	if err != nil || len(first.Lines) != 2 || !strings.Contains(strings.Join(first.Lines, "\n"), `intent="Do the thing called sweep-waiting"`) {
		t.Fatalf("the sweep preview did not show exact intents: %+v %v", first, err)
	}
	changedIntent := "Changed after the human read the listing."
	if result, err := Edit(verbReq(root, "01J5X00000000000000000SW10", "mac-a"), waiting.Id, EditFields{Intent: &changedIntent}); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("pre-sweep intent edit: %+v %v", result, err)
	}
	human := verbReq(root, "01J5X00000000000000000SW20", "mac-a")
	human.Actor.Human = "Wido"
	proof := testHumanAuthority(t, root, human.Now)
	result, err := ApproveSweep(human, first.Digest, proof)
	if err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "SWEEP_LISTING_CHANGED") {
		t.Fatalf("the stale seen-intent digest approved a changed intent: %+v %v", result, err)
	}
	second, err := PreviewApprovalSweep(endpointFor(root), human.Now)
	if err != nil {
		t.Fatal(err)
	}
	human.Ulid = "01J5X00000000000000000SW30"
	result, err = ApproveSweep(human, second.Digest, proof)
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("confirmed sweep: %+v %v", result, err)
	}
	tree, err := loadTree(root, result.Tip)
	if err != nil {
		t.Fatal(err)
	}
	if tree.Live[waiting.Id].State != StateApproved || tree.Live[waiting.Id].Approved == nil {
		t.Fatalf("the listed waiting goal was not approved: %+v", tree.Live[waiting.Id])
	}
	if tree.Live[running.Id].State != StateClaimed || tree.Live[running.Id].Claimed == nil || tree.Live[running.Id].Approved == nil {
		t.Fatalf("the sweep broke or failed to grandfather claimed work: %+v", tree.Live[running.Id])
	}
	if tree.Root.ApprovalGate == nil || tree.Root.ApprovalGate.Opid != human.opid() {
		t.Fatalf("the sweep did not arm the fleet gate in the same transaction: %+v", tree.Root.ApprovalGate)
	}
}

func TestApprovalSweepDoesNotApproveOverNormGoalWithoutNormApproval(t *testing.T) {
	_, root := oneClone(t)
	seedLedger(t, root)
	seedGoalNormConfig(t, root)
	withinBudget := testBudget()
	overBudget := Budget{ElapsedLimit: "1h", AttemptLimit: 1, ReservedJobMinutesLimit: 2400, ActiveJobLimit: 1}
	within := vGoal("sweep-within-norm", StateQueued)
	within.Budget = &withinBudget
	over := vGoal("sweep-over-norm", StateQueued)
	over.Budget = &overBudget
	publishGoalFixtures(t, root, within, over)

	request := verbReq(root, "01J5X00000000000000000SN00", "mac-a")
	listing, err := PreviewApprovalSweep(endpointFor(root), request.Now)
	if err != nil {
		t.Fatal(err)
	}
	request.Actor.Human = "Wido"
	result, err := ApproveSweep(request, listing.Digest, testHumanAuthority(t, root, request.Now))
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("sweep with one within-norm goal: %+v %v", result, err)
	}
	tree, err := loadTree(root, result.Tip)
	if err != nil {
		t.Fatal(err)
	}
	if got := tree.Live[within.Id]; got.State != StateApproved || got.Approved == nil {
		t.Fatalf("sweep did not approve the within-norm goal: %+v", got)
	}
	if got := tree.Live[over.Id]; got.State != StateQueued || got.Approved != nil {
		t.Fatalf("sweep made an over-norm goal claimable without a norm approval: %+v", got)
	}
}

func TestFleetEnrollmentExpiresRelayedClaimAndStealEverywhere(t *testing.T) {
	_, a, b := twoClones(t)
	seedLedger(t, a)
	for index, id := range []string{"relay-running", "relay-waiting"} {
		ulid := []string{"01J5X00000000000000000FE00", "01J5X00000000000000000FE10"}[index]
		if result, err := Open(verbReq(a, ulid, "mac-a"), id, "Temporary approval for "+id+".", OriginMain, "Run."); err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("open %s: %+v %v", id, result, err)
		}
	}
	human := verbReq(a, "01J5X00000000000000000FE20", "mac-a")
	human.Actor.Human = "Wido"
	proof := testTemporaryGoalProof(t, a, "Wido authorizes these two goals", "2026-09-06")
	if result, err := Approve(human, []string{"relay-running", "relay-waiting"}, ptrBudget(testBudget()), &proof); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("relayed approval: %+v %v", result, err)
	}
	if result, err := Claim(verbReq(a, "01J5X00000000000000000FE30", "mac-a"), "relay-running"); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("relayed approval did not initially admit claim: %+v %v", result, err)
	}
	if _, err := FetchAdvance(endpointFor(b)); err != nil {
		t.Fatal(err)
	}
	enrollment := verbReq(b, "01J5X00000000000000000FE40", "mac-b")
	enrollment.Actor.Human = "Wido"
	if result, err := RecordFleetEnrollment(enrollment, 1); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("fleet enrollment: %+v %v", result, err)
	}
	if _, err := FetchAdvance(endpointFor(a)); err != nil {
		t.Fatal(err)
	}
	claim, err := Claim(verbReq(a, "01J5X00000000000000000FE50", "mac-a"), "relay-waiting")
	if err != nil || claim.Outcome != OutcomeRejected || !strings.Contains(claim.Detail, "APPROVAL_EXPIRED") || !strings.Contains(claim.Detail, "fleet's first terminal") {
		t.Fatalf("another machine's enrollment did not expire claim: %+v %v", claim, err)
	}
	steal := verbReq(b, "01J5X00000000000000000FE60", "mac-b")
	steal.Actor.Human = "Wido"
	stolen, err := Steal(steal, "relay-running")
	if err != nil || stolen.Outcome != OutcomeRejected || !strings.Contains(stolen.Detail, "APPROVAL_EXPIRED") {
		t.Fatalf("expired approval created a fresh stolen revision: %+v %v", stolen, err)
	}
}

func TestApprovedToAllParkedArcIsLegalOnVerbAndReconcile(t *testing.T) {
	_, root := oneClone(t)
	seedLedger(t, root)
	for index, id := range []string{"parked-one", "parked-two", "approved-joiner"} {
		ulid := []string{"01J5X00000000000000000PA00", "01J5X00000000000000000PA10", "01J5X00000000000000000PA20"}[index]
		if result, err := Open(verbReq(root, ulid, "mac-a"), id, "Arc member "+id+".", OriginMain, "Wait."); err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("open %s: %+v %v", id, result, err)
		}
	}
	for index, id := range []string{"parked-one", "parked-two"} {
		ulid := []string{"01J5X00000000000000000PA30", "01J5X00000000000000000PA40"}[index]
		if result, err := SetArc(verbReq(root, ulid, "mac-a"), id, "all-parked"); err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("set arc %s: %+v %v", id, result, err)
		}
	}
	if result, err := Park(verbReq(root, "01J5X00000000000000000PA50", "mac-a"), "parked-one", "arc paused"); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("park one: %+v %v", result, err)
	}
	if result, err := Park(verbReq(root, "01J5X00000000000000000PA60", "mac-a"), "parked-two", "arc paused"); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("park two: %+v %v", result, err)
	}
	approveGoalForTest(t, verbReq(root, "01J5X00000000000000000PA70", "mac-a"), "approved-joiner", testBudget())

	tree, err := loadTree(root, acceptedTip(t, root))
	if err != nil {
		t.Fatal(err)
	}
	reconcile := verbReq(root, "01J5X00000000000000000PA80", "mac-a")
	reconcile.Actor.Human = "Wido"
	if _, err := applyRow(tree, reconcile, MappedVerb{Verb: "set-arc", Id: "approved-joiner", Arc: "all-parked", BaseState: StateApproved}, newReplaySession()); err != nil {
		t.Fatalf("reconcile all-parked arc transition refused: %v", err)
	}
	if joined := tree.Live["approved-joiner"]; joined.State != StateParked || joined.Approved == nil || joined.Parked == nil {
		t.Fatalf("reconcile orphaned the approved all-parked join: %+v", joined)
	}

	normal := verbReq(root, "01J5X00000000000000000PA90", "mac-a")
	normal.Actor.Human = "Wido"
	result, err := SetArc(normal, "approved-joiner", "all-parked")
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("normal all-parked arc transition refused: %+v %v", result, err)
	}
	landed, err := loadTree(root, result.Tip)
	if err != nil {
		t.Fatal(err)
	}
	if joined := landed.Live["approved-joiner"]; joined.State != StateParked || joined.Approved == nil || joined.Parked == nil {
		t.Fatalf("normal transition orphaned the approved join: %+v", joined)
	}
	if problems := ValidateTree(landed); len(problems) != 0 {
		t.Fatalf("all-parked transition produced an invalid tree: %v", problems)
	}
}
