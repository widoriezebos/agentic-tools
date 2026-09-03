package goal

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestUnapproveWithdrawsApprovedAndClaimedWork(t *testing.T) {
	t.Run("approved work returns to queued", func(t *testing.T) {
		_, root := oneClone(t)
		seedLedger(t, root)
		if result, err := Open(verbReq(root, "01J5X00000000000000000RV00", "mac-a"), "revoke-waiting", "Approval may be withdrawn.", OriginMain, "Wait."); err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("open approval fixture: %+v %v", result, err)
		}
		approveGoalForTest(t, verbReq(root, "01J5X00000000000000000RV10", "mac-a"), "revoke-waiting", testBudget())

		withdraw := verbReq(root, "01J5X00000000000000000RV20", "mac-a")
		withdraw.Actor.Human = "Wido"
		proof := testHumanAuthority(t, root, withdraw.Now)
		result, err := Unapprove(withdraw, "revoke-waiting", "the reviewed work must wait", proof)
		if err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("withdraw approved work: %+v %v", result, err)
		}
		tree, err := loadTree(root, result.Tip)
		if err != nil {
			t.Fatal(err)
		}
		file := tree.Live["revoke-waiting"]
		if file.State != StateQueued || file.Approved != nil || file.Budget != nil || file.NormApproval != nil || file.Parked != nil {
			t.Fatalf("unapproval did not remove the complete execution authority tuple: %+v", file)
		}
		last := file.History[len(file.History)-1]
		if last.Verb != "unapprove" || last.Reason != "the reviewed work must wait" || last.Actor != "human:Wido" {
			t.Fatalf("unapproval history did not retain the human reason: %+v", last)
		}

		retry := verbReq(root, "01J5X00000000000000000RV30", "mac-a")
		retry.Actor.Human = "Wido"
		result, err = Unapprove(retry, "revoke-waiting", "withdraw again", proof)
		if err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "APPROVAL_REQUIRED") || !strings.Contains(result.Detail, "this unapprove is refused") {
			t.Fatalf("a second unapproval did not refuse by name: %+v %v", result, err)
		}
	})

	t.Run("claimed work is parked and its claimant is displaced", func(t *testing.T) {
		_, root := oneClone(t)
		seedLedger(t, root)
		if result, err := Open(verbReq(root, "01J5X00000000000000000RC00", "mac-a"), "revoke-running", "Withdraw a running approval.", OriginMain, "Run."); err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("open approval fixture: %+v %v", result, err)
		}
		claim := verbReq(root, "01J5X00000000000000000RC10", "mac-a")
		if result, err := claimApprovedForTest(t, claim, "revoke-running", testBudget()); err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("claim approval fixture: %+v %v", result, err)
		}

		withdraw := verbReq(root, "01J5X00000000000000000RC20", "mac-b")
		withdraw.Actor.Human = "Wido"
		result, err := Unapprove(withdraw, "revoke-running", "stop admitting new reservations", testHumanAuthority(t, root, withdraw.Now))
		if err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("withdraw claimed work: %+v %v", result, err)
		}
		tree, err := loadTree(root, result.Tip)
		if err != nil {
			t.Fatal(err)
		}
		file := tree.Live["revoke-running"]
		if file.State != StateParked || file.Claimed != nil || file.Approved != nil || file.Budget != nil || file.NormApproval != nil || file.Parked == nil {
			t.Fatalf("unapproval did not park claimed work and clear execution authority: %+v", file)
		}
		const displaced = "mac-a+lin-1@2026-08-20T22:00:00Z"
		if file.Parked.Because != "approval revoked: stop admitting new reservations" || file.Parked.Displaced != displaced {
			t.Fatalf("unapproval did not preserve the revocation reason and displaced pair: %+v", file.Parked)
		}
		last := file.History[len(file.History)-1]
		if last.Reason != "stop admitting new reservations" || last.Displaced != displaced {
			t.Fatalf("claimed unapproval history lost its reason or displaced pair: %+v", last)
		}
	})
}

func TestUnapproveRequiresCompleteHumanAuthority(t *testing.T) {
	_, root := oneClone(t)
	request := verbReq(root, "01J5X00000000000000000RA00", "mac-a")
	if _, err := Unapprove(request, "missing", "because", nil); err == nil || !strings.Contains(err.Error(), "human-only") {
		t.Fatalf("agent-shaped unapproval did not refuse before publication: %v", err)
	}
	request.Actor.Human = "Wido"
	if _, err := Unapprove(request, "missing", "", nil); err == nil || !strings.Contains(err.Error(), "requires --by and --because") {
		t.Fatalf("reasonless unapproval did not refuse before publication: %v", err)
	}
	if _, err := Unapprove(request, "missing", "because", nil); err == nil || !strings.Contains(err.Error(), "freshly observed") {
		t.Fatalf("unproved unapproval did not refuse before publication: %v", err)
	}
}

func TestApproveRefusalsAndProvenReratification(t *testing.T) {
	_, root := oneClone(t)
	seedLedger(t, root)
	if result, err := Open(verbReq(root, "01J5X00000000000000000RR00", "mac-a"), "ratify-once", "Ratify one exact payload.", OriginMain, "Wait."); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("open approval fixture: %+v %v", result, err)
	}

	human := verbReq(root, "01J5X00000000000000000RR10", "mac-a")
	human.Actor.Human = "Wido"
	proof := testHumanAuthority(t, root, human.Now)
	if _, err := Approve(human, nil, ptrBudget(testBudget()), proof); err == nil || !strings.Contains(err.Error(), "at least one goal id") {
		t.Fatalf("targetless approval did not refuse: %v", err)
	}
	if _, err := Approve(human, []string{"ratify-once"}, &Budget{}, proof); err == nil || !strings.Contains(err.Error(), "invalid approval budget") {
		t.Fatalf("incomplete approval budget did not refuse: %v", err)
	}

	human.Ulid = "01J5X00000000000000000RR20"
	result, err := Approve(human, []string{"absent-goal"}, ptrBudget(testBudget()), proof)
	if err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "is not live") {
		t.Fatalf("approval rewrote an absent goal: %+v %v", result, err)
	}
	human.Ulid = "01J5X00000000000000000RR30"
	result, err = Approve(human, []string{"ratify-once"}, nil, proof)
	if err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "requires one complete budget tuple") {
		t.Fatalf("first approval reused a nonexistent tuple: %+v %v", result, err)
	}

	human.Ulid = "01J5X00000000000000000RR40"
	result, err = Approve(human, []string{"ratify-once"}, ptrBudget(testBudget()), proof)
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("approve exact tuple: %+v %v", result, err)
	}
	human.Ulid = "01J5X00000000000000000RR50"
	result, err = Approve(human, []string{"ratify-once"}, nil, proof)
	if err != nil || result.Outcome != OutcomeAbandoned || !strings.Contains(result.Detail, "same proven approval") {
		t.Fatalf("identical proven approval was not an explicit no-op: %+v %v", result, err)
	}

	if result, err = Claim(verbReq(root, "01J5X00000000000000000RR60", "mac-a"), "ratify-once"); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("claim approved fixture: %+v %v", result, err)
	}
	human.Ulid = "01J5X00000000000000000RR70"
	result, err = Approve(human, []string{"ratify-once"}, ptrBudget(testBudget()), proof)
	if err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "tuple changes through goal set-budget") {
		t.Fatalf("approve changed a claimed tuple instead of naming set-budget: %+v %v", result, err)
	}
}

func TestApprovalRequiredNamesEveryClaimProducingPath(t *testing.T) {
	file := vGoal("not-approved", StateQueued)
	for _, verb := range []string{"claim", "arc claim", "steal", "set-arc claim", "reconcile set-arc claim", "resume"} {
		t.Run(verb, func(t *testing.T) {
			if _, err := requireApprovedForClaim(t.TempDir(), &TreeGoals{Root: vRoot()}, file, time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), verb); err == nil || !strings.Contains(err.Error(), "APPROVAL_REQUIRED") || !strings.Contains(err.Error(), "this "+verb+" is refused") {
				t.Fatalf("%s did not carry its approval refusal and remedy: %v", verb, err)
			}
		})
	}
}

func TestExecutionGateRejectsIntentAndBudgetDigestEdits(t *testing.T) {
	_, root := oneClone(t)
	seedGoalNormConfig(t, root)
	approved := approvedGoalFixture(vGoal("digest-bound", StateQueued), testBudget())
	tree := &TreeGoals{Root: vRoot(), Live: map[string]*GoalFile{approved.Id: approved}}

	intentEdit := *approved
	intentEdit.Intent = "Intent changed after approval."
	if _, err := requireApprovedForClaim(root, tree, &intentEdit, time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), "claim"); err == nil || !strings.Contains(err.Error(), "APPROVAL_REQUIRED") || !strings.Contains(err.Error(), "digest") {
		t.Fatalf("an intent edit retained execution approval: %v", err)
	}

	budgetEdit := *approved
	changedBudget := *approved.Budget
	changedBudget.AttemptLimit++
	budgetEdit.Budget = &changedBudget
	if _, err := requireApprovedForClaim(root, tree, &budgetEdit, time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), "resume"); err == nil || !strings.Contains(err.Error(), "APPROVAL_REQUIRED") || !strings.Contains(err.Error(), "digest") {
		t.Fatalf("a budget edit retained execution approval: %v", err)
	}
}

func TestApprovalRecordRejectsEveryIncompleteBindingClass(t *testing.T) {
	if err := (*GoalFile)(nil).ValidateApprovalRecord(); err != nil {
		t.Fatalf("an absent approval record should need no validation: %v", err)
	}
	tests := []struct {
		name string
		edit func(*GoalFile)
		want string
	}{
		{"non-human actor", func(file *GoalFile) { file.Approved.By = "mac-a+lin-1" }, "does not name a human"},
		{"event coordinates", func(file *GoalFile) { file.Approved.At = "not-a-time" }, "incomplete or out-of-range"},
		{"history event", func(file *GoalFile) { file.Approved.Opid = "different-operation" }, "does not bind"},
		{"proven relay facts", func(file *GoalFile) { file.Approved.ReviewBy = "2026-09-06" }, "proven authority cannot carry"},
		{"relayed history facts", func(file *GoalFile) {
			file.Approved.Authority = ApprovalAuthorityRelayed
			file.Approved.ReviewBy = "2026-09-06"
		}, "relayed authority does not match"},
		{"unknown authority", func(file *GoalFile) { file.Approved.Authority = "delegated" }, "not proven|relayed"},
		{"missing budget", func(file *GoalFile) { file.Budget = nil }, "requires a complete Budget"},
		{"changed digest", func(file *GoalFile) { file.Intent = "Changed after approval." }, "digest does not match"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			file := approvedGoalFixture(vGoal("approval-binding", StateQueued), testBudget())
			test.edit(file)
			if err := file.ValidateApprovalRecord(); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("invalid approval binding did not refuse with %q: %v", test.want, err)
			}
		})
	}
}

func TestRelayedSweepExpiresAtReviewDate(t *testing.T) {
	_, root := oneClone(t)
	seedLedger(t, root)
	seedGoalNormConfig(t, root)
	budget := testBudget()
	waiting := vGoal("relay-sweep", StateQueued)
	waiting.Budget = &budget
	publishGoalFixtures(t, root, waiting)

	human := verbReq(root, "01J5X00000000000000000RS00", "mac-a")
	human.Now = time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	human.Actor.Human = "Wido"
	proof := testTemporaryGoalProof(t, root, "Wido approves this sweep", "2026-09-02")
	listing, err := PreviewApprovalSweep(endpointFor(root), human.Now)
	if err != nil {
		t.Fatal(err)
	}
	result, err := ApproveSweep(human, listing.Digest, &proof)
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("relayed sweep: %+v %v", result, err)
	}
	tree, err := loadTree(root, result.Tip)
	if err != nil {
		t.Fatal(err)
	}
	file := tree.Live[waiting.Id]
	if file.State != StateApproved || file.Approved == nil || file.Approved.Authority != ApprovalAuthorityRelayed || file.Approved.ReviewBy != "2026-09-02" {
		t.Fatalf("relayed sweep did not record its temporary authority: %+v", file)
	}

	claim := verbReq(root, "01J5X00000000000000000RS10", "mac-a")
	claim.Now = time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
	result, err = Claim(claim, waiting.Id)
	if err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "APPROVAL_EXPIRED") || !strings.Contains(result.Detail, "review date 2026-09-02 has passed") {
		t.Fatalf("the relayed word admitted work after its review date: %+v %v", result, err)
	}

	second, err := PreviewApprovalSweep(endpointFor(root), claim.Now)
	if err != nil {
		t.Fatal(err)
	}
	human.Ulid = "01J5X00000000000000000RS20"
	human.Now = claim.Now
	result, err = ApproveSweep(human, second.Digest, &proof)
	if err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "relayed sweep refuses after any approval exists") {
		t.Fatalf("a second relayed sweep did not refuse the standing approval: %+v %v", result, err)
	}
}

func TestApprovalExpiryUsesReviewDateAndFinalHorizon(t *testing.T) {
	relayed := &GoalFile{Approved: &ApprovalRecord{Authority: ApprovalAuthorityRelayed, ReviewBy: "2026-09-02"}}
	if expired, why := relayed.ApprovalExpired(ApprovalHorizon{Now: time.Date(2026, 9, 3, 0, 0, 0, 0, time.UTC)}); !expired || !strings.Contains(why, "review date 2026-09-02 has passed") {
		t.Fatalf("review-date expiry was not deterministic: expired=%v why=%q", expired, why)
	}
	relayed.Approved.ReviewBy = "malformed"
	if expired, why := relayed.ApprovalExpired(ApprovalHorizon{Now: time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)}); !expired || !strings.Contains(why, "temporary authority horizon 2026-09-06 has passed") {
		t.Fatalf("the final relay horizon did not fail closed: expired=%v why=%q", expired, why)
	}
}

func TestSweepListingBindsIntentBudgetAndDisplayedApproval(t *testing.T) {
	budget := testBudget()
	approved := approvedGoalFixture(vGoal("listed-approved", StateQueued), budget)
	approved.NormApproval = &GoalNormApprovalClaim{ApprovedRef: "R-400", Minutes: 1600, GoalRevision: approved.Revision}
	invalid := vGoal("listed-invalid", StateQueued)
	invalid.Budget = &Budget{}
	tree := &TreeGoals{Root: vRoot(), Live: map[string]*GoalFile{approved.Id: approved, invalid.Id: invalid}}

	original := sweepListing(tree)
	if len(original.Skipped) != 1 || original.Skipped[0] != invalid.Id {
		t.Fatalf("listing did not name exactly the invalid-budget goal as skipped: %+v", original)
	}
	lines := strings.Join(original.Lines, "\n")
	if !strings.Contains(lines, `intent="Do the thing called listed-approved"`) || !strings.Contains(lines, "normApproval=R-400/1600/2") || !strings.Contains(lines, "authority=proven") {
		t.Fatalf("listing hid intent, norm approval, or authority from the human: %s", lines)
	}

	approved.Intent = "A different intent."
	intentChanged := sweepListing(tree)
	if intentChanged.Digest == original.Digest {
		t.Fatalf("the sweep digest did not bind the displayed intent: %s", original.Digest)
	}
	approved.Intent = "Do the thing called listed-approved"
	changedBudget := *approved.Budget
	changedBudget.AttemptLimit++
	approved.Budget = &changedBudget
	budgetChanged := sweepListing(tree)
	if budgetChanged.Digest == original.Digest {
		t.Fatalf("the sweep digest did not bind the displayed budget: %s", original.Digest)
	}
}

func TestProvenSweepWithNoEligibleChangesIsExplicitNoOp(t *testing.T) {
	_, root := oneClone(t)
	seedLedger(t, root)
	if result, err := Open(verbReq(root, "01J5X00000000000000000PN00", "mac-a"), "already-proven", "Already approved.", OriginMain, "Wait."); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("open approval fixture: %+v %v", result, err)
	}
	request := verbReq(root, "01J5X00000000000000000PN10", "mac-a")
	approveGoalForTest(t, request, "already-proven", testBudget())
	listing, err := PreviewApprovalSweep(endpointFor(root), request.Now)
	if err != nil {
		t.Fatal(err)
	}
	human := verbReq(root, "01J5X00000000000000000PN20", "mac-a")
	human.Actor.Human = "Wido"
	result, err := ApproveSweep(human, listing.Digest, testHumanAuthority(t, root, human.Now))
	if err != nil || result.Outcome != OutcomeAbandoned || !strings.Contains(result.Detail, "no absent or relayed approval") {
		t.Fatalf("a proven no-change sweep was not an explicit no-op: %+v %v", result, err)
	}
	if _, err := ApproveSweep(human, "not-a-digest", testHumanAuthority(t, root, human.Now)); err == nil || !strings.Contains(err.Error(), "listing sha256") {
		t.Fatalf("a malformed sweep confirmation reached publication: %v", err)
	}
}

func TestGoalNormCheckCoversWithinAndOverNormRemedies(t *testing.T) {
	_, root := oneClone(t)
	seedGoalNormConfig(t, root)
	within := Budget{ReservedJobMinutesLimit: 1440}
	if err := requireWithinGoalNorm(root, within, "within", ""); err != nil {
		t.Fatalf("the exact goal norm refused: %v", err)
	}
	over := Budget{ReservedJobMinutesLimit: 1441}
	if err := requireWithinGoalNorm(root, over, "over", "cannot rejoin"); err == nil || !strings.Contains(err.Error(), "over cannot rejoin with an over-norm tuple") {
		t.Fatalf("the contextual norm remedy was not preserved: %v", err)
	}
	if err := requireWithinGoalNorm(root, over, "over", ""); err == nil || !strings.Contains(err.Error(), "GOAL_NORM_REFUSED") || !strings.Contains(err.Error(), "goal split") {
		t.Fatalf("the ordinary over-norm refusal lost its split remedy: %v", err)
	}
	if err := os.WriteFile(root+"/metasystem.conf", []byte("metasystem.budget.goal-norm-job-minutes=many\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := requireWithinGoalNorm(root, within, "invalid-config", ""); err == nil || !strings.Contains(err.Error(), "must be a positive integer") {
		t.Fatalf("a malformed goal norm was treated as a usable execution bound: %v", err)
	}
}

func TestOverNormApprovalRefusesWithoutAndPassesWithCoveringToken(t *testing.T) {
	_, root := oneClone(t)
	seedLedger(t, root)
	seedGoalNormConfig(t, root)
	if result, err := Open(verbReq(root, "01J5X00000000000000000NR00", "mac-a"), "covered-over-norm", "Run one approved exception.", OriginMain, "Wait for approval."); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("open over-norm fixture: %+v %v", result, err)
	}
	over := Budget{ElapsedLimit: "1h", AttemptLimit: 1, ReservedJobMinutesLimit: 1500, ActiveJobLimit: 1}
	human := verbReq(root, "01J5X00000000000000000NR10", "mac-a")
	human.Actor.Human = "Wido"
	proof := testHumanAuthority(t, root, human.Now)
	result, err := Approve(human, []string{"covered-over-norm"}, &over, proof)
	if err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "GOAL_NORM_REFUSED") {
		t.Fatalf("over-norm approval passed without a covering token: %+v %v", result, err)
	}

	if err := os.MkdirAll(root+"/memory", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(root+"/memory/rulings.md", []byte("| R-401 | goal=covered-over-norm minutes=1500 goalRevision=1 |\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	human.Ulid = "01J5X00000000000000000NR20"
	human.ApprovedRef = "R-401"
	result, err = Approve(human, []string{"covered-over-norm"}, &over, proof)
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("covering token did not admit the over-norm approval: %+v %v", result, err)
	}
	tree, err := loadTree(root, result.Tip)
	if err != nil {
		t.Fatal(err)
	}
	claim := tree.Live["covered-over-norm"].NormApproval
	if claim == nil || claim.ApprovedRef != "R-401" || claim.Minutes != 1500 || claim.GoalRevision != 1 {
		t.Fatalf("approval did not store the exact covering token: %+v", claim)
	}
	result, err = Claim(verbReq(root, "01J5X00000000000000000NR30", "mac-a"), "covered-over-norm")
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("stored covering token did not admit execution: %+v %v", result, err)
	}
}

func TestFleetEnrollmentValidationAndIdempotence(t *testing.T) {
	_, root := oneClone(t)
	seedLedger(t, root)
	request := verbReq(root, "01J5X00000000000000000EN00", "mac-a")
	if _, err := RecordFleetEnrollment(request, 0); err == nil || !strings.Contains(err.Error(), "positive generation") {
		t.Fatalf("zero fleet generation did not refuse: %v", err)
	}
	result, err := RecordFleetEnrollment(request, 4)
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("record first fleet enrollment: %+v %v", result, err)
	}
	request.Ulid = "01J5X00000000000000000EN10"
	result, err = RecordFleetEnrollment(request, 5)
	if err != nil || result.Outcome != OutcomeAbandoned || !strings.Contains(result.Detail, "already recorded") {
		t.Fatalf("a later enrollment replaced the fleet's first cutoff: %+v %v", result, err)
	}
}
