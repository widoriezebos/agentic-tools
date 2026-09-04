package goal

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func riskLocalRoot(t *testing.T, seedID string) string {
	t.Helper()
	root := obligationAuthorityLocalRoot(t, seedID)
	obligationAuthorityGit(t, root, "rm", "-q", "plans/goals/"+seedID+".md")
	obligationAuthorityGit(t, root, "commit", "-qm", "remove local fixture seed")
	obligationAuthorityGit(t, root, "update-ref", LocalLedgerBranch, "HEAD")
	obligationAuthorityGit(t, root, "update-ref", AcceptedRef, "HEAD")
	return root
}

func TestSTR4R1RaiseTransaction(t *testing.T) {
	root := riskLocalRoot(t, "raise-bed")
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\nmetasystem.governance.correlation-policy=A\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	low := RiskRecord{Severity: 1, Novelty: 1, Exposure: 1, Accumulation: 1, Basis: "landed precedent"}
	budget := Budget{ElapsedLimit: "1h", AttemptLimit: 3, ReservedJobMinutesLimit: 360, ActiveJobLimit: 1, ReviewRoundLimit: 0}
	opened := obligationAuthorityVerbReq(root, "01J5X00000000000000000RA00", "mac-a")
	if result, err := OpenRisked(opened, "risk-raise", "Raise rigor without erasing control state.", OriginMain, "Exercise the raise.", low, 0, "", &budget, nil); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("open risk-scored goal: %+v %v", result, err)
	}
	proof := testHumanAuthority(t, root, opened.Now)
	approve := opened
	approve.Actor.Human = "Wido"
	approve.Ulid = "01J5X00000000000000000RA10"
	approve.Now = opened.Now.Add(time.Minute)
	if result, err := Approve(approve, []string{"risk-raise"}, &budget, proof); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("approve risk-scored goal: %+v %v", result, err)
	}
	claim := opened
	claim.Ulid = "01J5X00000000000000000RA20"
	claim.Now = opened.Now.Add(2 * time.Minute)
	claim.ClaimEpoch = 9
	if result, err := Claim(claim, "risk-raise"); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("claim risk-scored goal: %+v %v", result, err)
	}
	obligationReq := claim
	obligationReq.Actor.Human = "Wido"
	obligationReq.Ulid = "01J5X00000000000000000RA30"
	obligationReq.Now = opened.Now.Add(3 * time.Minute)
	if result, err := SetObligation(obligationReq, "risk-raise", testGovernedObligation(ObligationDraft), proof); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("set governed obligation: %+v %v", result, err)
	}
	beforeStop, err := Project(obligationAuthorityEndpoint(root), true, obligationReq.Now)
	if err != nil {
		t.Fatal(err)
	}
	capability := *beforeStop.Tree.Live["risk-raise"].StopCapability
	stop := CloseStopRequest{VerbRequest: VerbRequest{Endpoint: obligationAuthorityEndpoint(root), Actor: Actor{Machine: "mac-a", Lineage: "goal-stop-custodian"}, Ulid: "01J5X00000000000000000RA40", Now: opened.Now.Add(4 * time.Minute), ClaimEpoch: 9}, GoalID: "risk-raise", StopID: "stop-risk-raise-r1-f1", Reason: StopReasonElapsedLimit, Capability: capability}
	if result, err := CloseStop(stop); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("close launch fence: %+v %v", result, err)
	}
	stoppedProjection, err := Project(obligationAuthorityEndpoint(root), true, stop.Now)
	if err != nil {
		t.Fatal(err)
	}
	stopped := stoppedProjection.Tree.Live["risk-raise"]
	beforeFence, err := json.Marshal(stopped.StopFence)
	if err != nil {
		t.Fatal(err)
	}
	beforeObligation, err := json.Marshal(stopped.Obligation)
	if err != nil {
		t.Fatal(err)
	}
	beforeClaim := *stopped.Claimed
	beforeCapability := *stopped.StopCapability
	beforeRevision := stopped.Revision
	beforeBudget := *stopped.Budget
	beforeExceptions := stopped.BudgetExceptions

	high := RiskRecord{Severity: 2, Novelty: 1, Exposure: 1, Accumulation: 1, Basis: "moderate consequence discovered"}
	overrideTier := uint8(3)
	raise := claim
	raise.Ulid = "01J5X00000000000000000RA50"
	raise.Now = opened.Now.Add(5 * time.Minute)
	result, err := Edit(raise, "risk-raise", EditFields{Risk: &high, Tier: &overrideTier, Why: "retain full review", Evidence: "refusal:BUDGET_REFUSED"})
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("raise risk transaction: %+v %v", result, err)
	}
	afterTree, err := loadTree(root, result.Tip)
	if err != nil {
		t.Fatal(err)
	}
	after := afterTree.Live["risk-raise"]
	if after.Revision != beforeRevision+1 || after.Tier != 3 || after.Risk == nil || after.Risk.DerivedTier() != 2 {
		t.Fatalf("raise did not atomically advance risk, tier, and one revision: %+v", after)
	}
	afterFence, err := json.Marshal(after.StopFence)
	if err != nil {
		t.Fatal(err)
	}
	afterObligation, err := json.Marshal(after.Obligation)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(beforeFence, afterFence) || !bytes.Equal(beforeObligation, afterObligation) {
		t.Fatalf("raise changed the standing fence or governed obligation: fence before=%s after=%s obligation before=%s after=%s", beforeFence, afterFence, beforeObligation, afterObligation)
	}
	if after.Claimed.Revision != after.Revision || after.Claimed.AccountingRevision != beforeClaim.AccountingRevision || after.Claimed.At != beforeClaim.At || after.Claimed.Machine != beforeClaim.Machine || after.Claimed.Lineage != beforeClaim.Lineage {
		t.Fatalf("claim rebind changed more than its revision: before=%+v after=%+v", beforeClaim, after.Claimed)
	}
	if after.StopCapability.Revision != after.Revision || after.StopCapability.Generation != after.Revision || after.StopCapability.Machine != beforeCapability.Machine || after.StopCapability.ClaimEpoch != beforeCapability.ClaimEpoch || after.StopCapability.FenceEpoch != beforeCapability.FenceEpoch {
		t.Fatalf("stop capability rebind changed more than revision coordinates: before=%+v after=%+v", beforeCapability, after.StopCapability)
	}
	if after.Budget.ElapsedLimit != beforeBudget.ElapsedLimit || after.Budget.AttemptLimit != beforeBudget.AttemptLimit || after.Budget.ReservedJobMinutesLimit != beforeBudget.ReservedJobMinutesLimit || after.Budget.ActiveJobLimit != beforeBudget.ActiveJobLimit || after.Budget.ReviewRoundLimit != 3 || after.BudgetExceptions != beforeExceptions {
		t.Fatalf("raise changed spend members, failed to lift review rounds, or counted an exception: before=%+v after=%+v exceptions=%d", beforeBudget, after.Budget, after.BudgetExceptions)
	}
	lastReason := after.History[len(after.History)-1].Reason
	if after.Approved == nil || after.Approved.Authority != "raise="+raise.opid() || after.ValidateApprovalRecord() != nil || !strings.Contains(lastReason, "Misclassified: from=1 to=2 evidence=refusal:BUDGET_REFUSED") || !strings.Contains(lastReason, "TierOverride: derived=2 set=3 why=retain full review") {
		t.Fatalf("raise approval/history binding is invalid: approved=%+v history=%+v", after.Approved, after.History[len(after.History)-1])
	}
	withoutLine := *after
	withoutLine.History = append([]HistoryLine(nil), after.History...)
	withoutLine.History[len(withoutLine.History)-1].Reason = ""
	if err := withoutLine.ValidateApprovalRecord(); err == nil {
		t.Fatal("raise authority validated after its Misclassified line was removed")
	}
	lower := RiskRecord{Severity: 1, Novelty: 1, Exposure: 1, Accumulation: 1, Basis: "attempt to reduce claimed rigor"}
	lowerReq := raise
	lowerReq.Ulid = "01J5X00000000000000000RA60"
	lowerReq.Now = raise.Now.Add(time.Minute)
	if lowered, err := Edit(lowerReq, "risk-raise", EditFields{Risk: &lower}); err != nil || lowered.Outcome != OutcomeRejected {
		t.Fatalf("pair lowered risk after claim: %+v %v", lowered, err)
	}

	preserveLow := RiskRecord{Severity: 1, Novelty: 1, Exposure: 1, Accumulation: 1, Basis: "routine but conservatively tiered"}
	preserveOpen := obligationAuthorityVerbReq(root, "01J5X00000000000000000RA70", "mac-b")
	if opened, err := OpenRisked(preserveOpen, "preserve-override", "Preserve an existing override.", OriginMain, "Raise derivation.", preserveLow, 3, "standing full review", &budget, nil); err != nil || opened.Outcome != OutcomeConfirmed {
		t.Fatalf("open preserved override: %+v %v", opened, err)
	}
	preserveApprove := preserveOpen
	preserveApprove.Actor.Human = "Wido"
	preserveApprove.Ulid = "01J5X00000000000000000RA80"
	preserveApprove.Now = raise.Now.Add(3 * time.Minute)
	if approved, err := Approve(preserveApprove, []string{"preserve-override"}, &budget, proof); err != nil || approved.Outcome != OutcomeConfirmed {
		t.Fatalf("approve preserved override: %+v %v", approved, err)
	}
	preserveClaim := preserveOpen
	preserveClaim.Ulid = "01J5X00000000000000000RA90"
	preserveClaim.Now = raise.Now.Add(4 * time.Minute)
	preserveClaim.ClaimEpoch = 10
	if claimed, err := Claim(preserveClaim, "preserve-override"); err != nil || claimed.Outcome != OutcomeConfirmed {
		t.Fatalf("claim preserved override: %+v %v", claimed, err)
	}
	preserveRaised := RiskRecord{Severity: 2, Novelty: 1, Exposure: 1, Accumulation: 1, Basis: "moderate consequence discovered"}
	preserveEdit := preserveClaim
	preserveEdit.Ulid = "01J5X00000000000000000RAA0"
	preserveEdit.Now = raise.Now.Add(5 * time.Minute)
	preservedResult, err := Edit(preserveEdit, "preserve-override", EditFields{Risk: &preserveRaised, Evidence: "refusal:BUDGET_REFUSED"})
	if err != nil || preservedResult.Outcome != OutcomeConfirmed {
		t.Fatalf("raise with omitted tier: %+v %v", preservedResult, err)
	}
	preservedTree, err := loadTree(root, preservedResult.Tip)
	if err != nil || preservedTree.Live["preserve-override"].Tier != 3 {
		t.Fatalf("raise with omitted tier lost the standing override: %+v %v", preservedTree.Live["preserve-override"], err)
	}
}

func TestSTR4R1FourDowngradesRefused(t *testing.T) {
	root := riskLocalRoot(t, "downgrade-bed")
	budget := Budget{ElapsedLimit: "1h", AttemptLimit: 3, ReservedJobMinutesLimit: 360, ActiveJobLimit: 1, ReviewRoundLimit: 0}
	type downgrade struct {
		id      string
		initial RiskRecord
		tier    uint8
		why     string
		fields  func() EditFields
		check   func(*GoalFile) bool
	}
	cases := []downgrade{
		{id: "lower-score", initial: RiskRecord{Severity: 3, Novelty: 2, Exposure: 1, Accumulation: 1, Basis: "two novelty answers"}, fields: func() EditFields {
			risk := RiskRecord{Severity: 3, Novelty: 1, Exposure: 1, Accumulation: 1, Basis: "one novelty answer"}
			return EditFields{Risk: &risk}
		}, check: func(f *GoalFile) bool { return f.Risk.Novelty == 1 && f.Tier == 3 }},
		{id: "lower-derived", initial: RiskRecord{Severity: 3, Novelty: 1, Exposure: 1, Accumulation: 1, Basis: "severe"}, fields: func() EditFields {
			risk := RiskRecord{Severity: 2, Novelty: 1, Exposure: 1, Accumulation: 1, Basis: "moderate"}
			return EditFields{Risk: &risk}
		}, check: func(f *GoalFile) bool { return f.Risk.Severity == 2 && f.Tier == 2 }},
		{id: "lower-set", initial: RiskRecord{Severity: 1, Novelty: 1, Exposure: 1, Accumulation: 1, Basis: "routine"}, tier: 3, why: "extra review", fields: func() EditFields { tier := uint8(2); return EditFields{Tier: &tier} }, check: func(f *GoalFile) bool { return f.Tier == 2 }},
		{id: "lower-width", initial: RiskRecord{Severity: 2, Novelty: 1, Exposure: 1, Accumulation: 2, Basis: "accumulates"}, fields: func() EditFields {
			risk := RiskRecord{Severity: 2, Novelty: 1, Exposure: 1, Accumulation: 1, Basis: "area only"}
			return EditFields{Risk: &risk}
		}, check: func(f *GoalFile) bool { return f.Risk.GateWidth() == "area" && f.Tier == 2 }},
	}
	openULIDs := []string{"01J5X00000000000000000DB00", "01J5X00000000000000000DB10", "01J5X00000000000000000DB20", "01J5X00000000000000000DB30"}
	for index, test := range cases {
		req := obligationAuthorityVerbReq(root, openULIDs[index], "mac-a")
		if result, err := OpenRisked(req, test.id, "Exercise "+test.id+".", OriginMain, "Edit it.", test.initial, test.tier, test.why, &budget, nil); err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("open %s: %+v %v", test.id, result, err)
		}
	}
	proof := testHumanAuthority(t, root, obligationAuthorityVerbReq(root, openULIDs[0], "mac-a").Now)
	pairULIDs := []string{"01J5X00000000000000000DC00", "01J5X00000000000000000DC10", "01J5X00000000000000000DC20", "01J5X00000000000000000DC30"}
	humanULIDs := []string{"01J5X00000000000000000DD00", "01J5X00000000000000000DD10", "01J5X00000000000000000DD20", "01J5X00000000000000000DD30"}
	for index, test := range cases {
		pair := obligationAuthorityVerbReq(root, pairULIDs[index], "mac-a")
		if result, err := Edit(pair, test.id, test.fields()); err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "human act") {
			t.Errorf("pair downgrade %s = %+v %v", test.id, result, err)
			continue
		}
		human := obligationAuthorityVerbReq(root, humanULIDs[index], "mac-a")
		human.Actor.Human = "Wido"
		fields := test.fields()
		fields.Proof = proof
		result, err := Edit(human, test.id, fields)
		if err != nil || result.Outcome != OutcomeConfirmed {
			t.Errorf("human downgrade %s = %+v %v", test.id, result, err)
			continue
		}
		tree, err := loadTree(root, result.Tip)
		if err != nil || !test.check(tree.Live[test.id]) {
			t.Errorf("human downgrade %s did not land: goal=%+v err=%v", test.id, tree.Live[test.id], err)
		}
	}
}

func TestRiskOverridesAboveAndBelow(t *testing.T) {
	root := riskLocalRoot(t, "override-bed")
	budget := Budget{ElapsedLimit: "1h", AttemptLimit: 3, ReservedJobMinutesLimit: 360, ActiveJobLimit: 1, ReviewRoundLimit: 0}
	low := RiskRecord{Severity: 1, Novelty: 1, Exposure: 1, Accumulation: 1, Basis: "routine"}
	pair := obligationAuthorityVerbReq(root, "01J5X00000000000000000RB00", "mac-a")
	result, err := OpenRisked(pair, "override-above", "Record a conservative override.", OriginMain, "Review it.", low, 2, "independent review requested", &budget, nil)
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("pair override above: %+v %v", result, err)
	}
	tree, err := loadTree(root, result.Tip)
	if err != nil || !strings.Contains(tree.Live["override-above"].History[0].Reason, "TierOverride: derived=1 set=2 why=independent review requested") {
		t.Fatalf("pair override above was not recorded: %+v %v", tree.Live["override-above"], err)
	}
	high := RiskRecord{Severity: 3, Novelty: 1, Exposure: 1, Accumulation: 1, Basis: "severe"}
	if _, err := OpenRisked(obligationAuthorityVerbReq(root, "01J5X00000000000000000RB10", "mac-a"), "override-below-pair", "Refuse an unsafe override.", OriginMain, "Refuse it.", high, 2, "pair asks lower", &budget, nil); err == nil || !strings.Contains(err.Error(), "human act") {
		t.Fatalf("pair override below = %v", err)
	}
	human := obligationAuthorityVerbReq(root, "01J5X00000000000000000RB20", "mac-a")
	human.Actor.Human = "Wido"
	proof := testHumanAuthority(t, root, human.Now)
	result, err = OpenRisked(human, "override-below-human", "Record the human override.", OriginMain, "Proceed under human authority.", high, 2, "human accepts narrower rigor", &budget, proof)
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("human override below: %+v %v", result, err)
	}
	tree, err = loadTree(root, result.Tip)
	if err != nil || !strings.Contains(tree.Live["override-below-human"].History[0].Reason, "TierOverride: derived=3 set=2 why=human accepts narrower rigor") {
		t.Fatalf("human override below = %+v %v", tree.Live["override-below-human"], err)
	}
}

func TestSTR4R1FiveMemberExceptions(t *testing.T) {
	root := riskLocalRoot(t, "exception-bed")
	low := RiskRecord{Severity: 3, Novelty: 1, Exposure: 1, Accumulation: 1, Basis: "severe"}
	box := Budget{ElapsedLimit: "8h", AttemptLimit: 10, ReservedJobMinutesLimit: 1200, ActiveJobLimit: 1, ReviewRoundLimit: 3}
	opened := obligationAuthorityVerbReq(root, "01J5X00000000000000000EX00", "mac-a")
	if result, err := OpenRisked(opened, "budget-exceptions", "Count every over-box member.", OriginMain, "Raise two members.", low, 0, "", &box, nil); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", result, err)
	}
	if result, err := claimApprovedForTest(t, obligationAuthorityVerbReq(root, "01J5X00000000000000000EX10", "mac-a"), "budget-exceptions", box); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("claim: %+v %v", result, err)
	}
	equal := box
	equal.ElapsedLimit = "1d"
	if result, err := setBudgetApprovedForTest(t, obligationAuthorityVerbReq(root, "01J5X00000000000000000EX20", "mac-a"), "budget-exceptions", equal); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("equal elapsed duration: %+v %v", result, err)
	}
	tree, err := loadTree(root, acceptedTip(t, root))
	if err != nil || tree.Live["budget-exceptions"].BudgetExceptions != 0 {
		t.Fatalf("equal one-day and eight-hour limits counted as an exception: %+v %v", tree.Live["budget-exceptions"], err)
	}
	elapsed := box
	elapsed.ElapsedLimit = "1d2h"
	if result, err := setBudgetApprovedForTest(t, obligationAuthorityVerbReq(root, "01J5X00000000000000000EX25", "mac-a"), "budget-exceptions", elapsed); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("over elapsed duration: %+v %v", result, err)
	}
	tree, err = loadTree(root, acceptedTip(t, root))
	if err != nil || tree.Live["budget-exceptions"].BudgetExceptions != 1 {
		t.Fatalf("one-day-two-hour exception count = %+v %v", tree.Live["budget-exceptions"], err)
	}
	active := box
	active.ActiveJobLimit = 2
	if result, err := setBudgetApprovedForTest(t, obligationAuthorityVerbReq(root, "01J5X00000000000000000EX30", "mac-a"), "budget-exceptions", active); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("active-job exception: %+v %v", result, err)
	}
	tree, err = loadTree(root, acceptedTip(t, root))
	if err != nil || tree.Live["budget-exceptions"].BudgetExceptions != 2 {
		t.Fatalf("active-job exception count = %+v %v", tree.Live["budget-exceptions"], err)
	}
}

func TestSTR4R1ShapeFreeDerivation(t *testing.T) {
	low := RiskRecord{Severity: 1, Novelty: 1, Exposure: 1, Accumulation: 1, Basis: "same files, routine consequence"}
	high := low
	high.Severity = 3
	if got := low.DerivedTier(); got != 1 {
		t.Fatalf("scores 1,1,1,1 derive tier %d, want 1", got)
	}
	if got := high.DerivedTier(); got != 3 {
		t.Fatalf("scores 3,1,1,1 derive tier %d, want 3", got)
	}
	accumulating := low
	accumulating.Accumulation = 2
	if accumulating.DerivedTier() != 2 || accumulating.GateWidth() != "full" {
		t.Fatalf("accumulation two = tier %d width %s, want tier 2 full", accumulating.DerivedTier(), accumulating.GateWidth())
	}
}

func TestSTR4R1NilRiskDigest(t *testing.T) {
	budget := Budget{ElapsedLimit: "1h", AttemptLimit: 1, ReservedJobMinutesLimit: 5, ActiveJobLimit: 1, ReviewRoundLimit: 0}
	legacy := ApprovalDigest("unchanged intent", 1, budget)
	if got := ApprovalDigest("unchanged intent", 1, budget, nil); got != legacy {
		t.Fatalf("nil Risk changed the approval digest: got %s want %s", got, legacy)
	}
	risk := &RiskRecord{Severity: 1, Novelty: 1, Exposure: 1, Accumulation: 1, Basis: "routine"}
	if got := ApprovalDigest("unchanged intent", 1, budget, risk); got == legacy {
		t.Fatal("a present Risk record contributed no approval-digest bytes")
	}
}

func TestSTR4R1SweepBackfill(t *testing.T) {
	tree := &TreeGoals{Root: &RootRecord{}, Live: map[string]*GoalFile{
		"tierless": {Id: "tierless", Tier: 0},
		"tiered":   {Id: "tiered", Tier: 3},
	}}
	listing, err := classificationListing(tree, []byte("tierless 2,1,1,1 new surface\ntiered 1,1,1,1 cautious incumbent\n"))
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(listing.Lines, "\n")
	if !strings.Contains(joined, "tierless 2,1,1,1 tier=2 new surface") {
		t.Fatalf("tierless derivation absent from listing: %q", joined)
	}
	if !strings.Contains(joined, "tiered 1,1,1,1 tier=3 HUMAN-DECISION derived=1 cautious incumbent") {
		t.Fatalf("lower tiered derivation was not listed as a human decision: %q", joined)
	}
	var tieredProposal *ClassificationProposal
	for i := range listing.Proposals {
		if listing.Proposals[i].ID == "tiered" {
			tieredProposal = &listing.Proposals[i]
		}
	}
	if tieredProposal == nil || tieredProposal.Tier != 3 {
		t.Fatalf("confirmation proposal lowered incumbent tier: %+v", tieredProposal)
	}
	if _, err := classificationListing(tree, []byte("tierless 2 bare tier\ntiered 1,1,1,1 basis\n")); err == nil || !strings.Contains(err.Error(), "must be <goal-id> <severity>,<novelty>,<exposure>,<accumulation> <basis>") {
		t.Fatalf("bare tier row refusal = %v", err)
	}

	root := obligationAuthorityLocalRoot(t, "sweep-confirm")
	path := filepath.Join(root, "plans", "goals", "sweep-confirm.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	file, problems := ParseFile(data)
	if len(problems) != 0 {
		t.Fatalf("parse confirmation fixture: %v", problems)
	}
	file.Tier = 3
	if err := os.WriteFile(path, RenderFile(file), 0o644); err != nil {
		t.Fatal(err)
	}
	obligationAuthorityGit(t, root, "add", "plans/goals/sweep-confirm.md")
	obligationAuthorityGit(t, root, "commit", "-qm", "tiered sweep fixture")
	obligationAuthorityGit(t, root, "update-ref", LocalLedgerBranch, "HEAD")
	obligationAuthorityGit(t, root, "update-ref", AcceptedRef, "HEAD")
	confirmedListing, err := PreviewClassificationSweep(obligationAuthorityEndpoint(root), []byte("sweep-confirm 1,1,1,1 cautious incumbent\n"), time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC))
	if err != nil || len(confirmedListing.Proposals) != 1 {
		t.Fatalf("preview confirmation fixture: %+v %v", confirmedListing, err)
	}
	req := obligationAuthorityVerbReq(root, "01J5X00000000000000000SW00", "mac-a")
	req.Actor.Human = "Wido"
	result, err := ClassifyTier(req, confirmedListing.Proposals[0], true)
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("confirm classification: %+v %v", result, err)
	}
	written, err := loadTree(root, result.Tip)
	if err != nil || written.Live["sweep-confirm"].Tier != 3 || written.Live["sweep-confirm"].Risk == nil || written.Live["sweep-confirm"].Risk.DerivedTier() != 1 {
		t.Fatalf("confirm lowered the incumbent tier in written state: goal=%+v err=%v", written.Live["sweep-confirm"], err)
	}
}

func TestRiskRecordRendersAboveTierAndRoundTrips(t *testing.T) {
	risk := &RiskRecord{Severity: 2, Novelty: 1, Exposure: 3, Accumulation: 2, Basis: "quoted basis"}
	file := &GoalFile{Id: "risk-render", State: StateQueued, Tier: 3, Risk: risk, Intent: "intent", Origin: OriginMain, NextStep: "next", OpenedAt: "2026-09-04T10:00:00Z", Revision: 1, History: []HistoryLine{{At: "2026-09-04T10:00:00Z", Opid: "01J5X0000000000000000000C0-mac-a-1a2b3c4d", Verb: "open", Actor: "mac-a+lineage", Targets: []string{"risk-render"}, Keep: -1}}}
	rendered := string(RenderFile(file))
	if strings.Index(rendered, "- Risk:") > strings.Index(rendered, "- Tier:") {
		t.Fatalf("Risk rendered below Tier:\n%s", rendered)
	}
	parsed, problems := ParseFile([]byte(rendered))
	if len(problems) != 0 {
		t.Fatalf("round-trip problems: %v", problems)
	}
	if parsed.Risk == nil || *parsed.Risk != *risk {
		t.Fatalf("round-trip Risk = %+v, want %+v", parsed.Risk, risk)
	}
}
