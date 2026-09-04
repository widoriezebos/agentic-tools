package goal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/retrodebt"
)

func testMembers(parent string) []MemberDraft {
	return []MemberDraft{
		{ID: parent + "-one", Intent: "Deliver the first part.", NextStep: "Build part one.", Labels: []string{"part-one"}},
		{ID: parent + "-two", Intent: "Deliver the second part.", NextStep: "Build part two.", Blocked: []string{parent + "-one"}},
	}
}

func mainRatification(parent string, members []MemberDraft) SplitRatification {
	return SplitRatification{Tier: RatifierMain, MainID: "main-1", ClaimEpoch: 1, DraftSHA256: SplitDraftSHA256(parent, members)}
}

func seedGoalNormConfig(t *testing.T, root string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSplitDraftGrammarIsClosedAndCanonical(t *testing.T) {
	draft := []byte("# split parent\n\n## member parent-two\n- Intent: Second.\n- Next step: Go.\n- Labels: z, a, a\n\n## member parent-one\n- Intent: First.\n- Next step: Go.\n")
	members, err := ParseMemberDraft(draft, "parent")
	if err != nil {
		t.Fatal(err)
	}
	rendered := string(RenderMemberDraft("parent", members))
	if strings.Index(rendered, "parent-one") > strings.Index(rendered, "parent-two") || !strings.Contains(rendered, "- Labels: a, z") {
		t.Fatalf("canonical draft did not sort members and labels:\n%s", rendered)
	}
	if _, err := ParseMemberDraft([]byte(strings.Replace(string(draft), "- Labels: z, a, a", "- Budget: 4h", 1)), "parent"); err == nil || !strings.Contains(err.Error(), "unknown or computed") {
		t.Fatalf("computed draft fields must refuse: %v", err)
	}
	if _, err := ParseMemberDraft([]byte("# split parent\n## member only\n- Intent: One.\n- Next step: Go.\n"), "parent"); err == nil || !strings.Contains(err.Error(), "at least two") {
		t.Fatalf("one-member rename must refuse: %v", err)
	}
}

func TestSplitIsAtomicPermanentAndRewritesDependencies(t *testing.T) {
	_, root := oneClone(t)
	seedLedger(t, root)
	if res, err := Open(verbReq(root, "01J5X00000000000000000S100", "mac-a"), "old-blocker", "External prerequisite.", OriginMain, "Finish it."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open blocker: %+v %v", res, err)
	}
	if res, err := Open(verbReq(root, "01J5X00000000000000000S110", "mac-a"), "split-parent", "Large bounded intent.", OriginMain, "Decompose it.", "parent-label"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open parent: %+v %v", res, err)
	}
	blocked := []string{"old-blocker"}
	if res, err := Edit(verbReq(root, "01J5X00000000000000000S120", "mac-a"), "split-parent", EditFields{Blocked: &blocked}); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("block parent: %+v %v", res, err)
	}
	if res, err := SetArc(verbReq(root, "01J5X00000000000000000S130", "mac-a"), "split-parent", "old-arc"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("set old arc: %+v %v", res, err)
	}
	pinParent := verbReq(root, "01J5X00000000000000000S135", "mac-a")
	pinParent.Actor.Human = "wido"
	if res, err := SetPin(pinParent, "split-parent", "mac-a"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("pin parent: %+v %v", res, err)
	}
	if res, err := Open(verbReq(root, "01J5X00000000000000000S140", "mac-a"), "dependent", "Wait for the whole parent.", OriginMain, "Wait."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open dependent: %+v %v", res, err)
	}
	blocked = []string{"split-parent"}
	if res, err := Edit(verbReq(root, "01J5X00000000000000000S150", "mac-a"), "dependent", EditFields{Blocked: &blocked}); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("block dependent: %+v %v", res, err)
	}

	members := testMembers("split-parent")
	request := verbReq(root, "01J5X00000000000000000S160", "mac-a")
	result, err := Split(request, "split-parent", members, mainRatification("split-parent", members), nil)
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("split: %+v %v", result, err)
	}
	tree, err := loadTree(root, result.Tip)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"split-parent-one", "split-parent-two"} {
		member := tree.Live[id]
		if member == nil || member.Arc != "split-parent" || member.Origin != OriginMain || member.Pinned != "mac-a" || !containsString(member.Blocked, "old-blocker") || !containsString(member.Labels, "parent-label") {
			t.Fatalf("member %s did not inherit the parent envelope: %+v", id, member)
		}
	}
	if got := strings.Join(tree.Live["dependent"].Blocked, ","); got != "split-parent-one,split-parent-two" {
		t.Fatalf("inbound dependency did not expand to all members: %s", got)
	}
	parent := tree.Done["split-parent"]
	if parent == nil || parent.Ratified == nil || parent.Ratified.DraftSHA256 != SplitDraftSHA256("split-parent", members) || !strings.Contains(parent.Conclude, "goal:split-parent-one") {
		t.Fatalf("archived parent lacks ratification or pointers: %+v", parent)
	}
	if entry, ok := rootDecomposed(tree.Root, "split-parent"); !ok || entry.Opid != request.opid() {
		t.Fatalf("root decomposition registry did not land with split: %+v", tree.Root.Decomposed)
	}
	debts, err := retrodebt.Open(root)
	if err != nil || len(debts) != 1 || !strings.HasPrefix(debts[0].Source, "old-arc:") {
		t.Fatalf("last old-arc member split did not raise debt: %+v %v", debts, err)
	}

	reopen, err := Reopen(verbReq(root, "01J5X00000000000000000S170", "mac-a"), "split-parent")
	if err != nil || reopen.Outcome != OutcomeRejected || !strings.Contains(reopen.Detail, "never returns") {
		t.Fatalf("decomposed parent reopen did not refuse: %+v %v", reopen, err)
	}
	if pruned, err := Prune(verbReq(root, "01J5X00000000000000000S180", "mac-a"), 0); err != nil || pruned.Outcome != OutcomeConfirmed {
		t.Fatalf("prune: %+v %v", pruned, err)
	}
	recreated, err := Open(verbReq(root, "01J5X00000000000000000S190", "mac-a"), "split-parent", "Illicit resurrection.", OriginMain, "Stop.")
	if err != nil || recreated.Outcome != OutcomeRejected || !strings.Contains(recreated.Detail, "retired") {
		t.Fatalf("prune must not reopen the identifier: %+v %v", recreated, err)
	}
}

func TestSliceStartIsImmutableAndBlocksSplit(t *testing.T) {
	_, root := oneClone(t)
	seedLedger(t, root)
	seedGoalNormConfig(t, root)
	if res, err := openClaimForTest(t, verbReq(root, "01J5X00000000000000000SA00", "mac-a"), "sliced-parent", "Already slicing.", OriginMain, "Work.", testBudget()); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open claim: %+v %v", res, err)
	}
	mark := verbReq(root, "01J5X00000000000000000SA10", "mac-a")
	result, err := MarkSliced(mark, "sliced-parent")
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("mark sliced: %+v %v", result, err)
	}
	second, err := MarkSliced(verbReq(root, "01J5X00000000000000000SA20", "mac-a"), "sliced-parent")
	if err != nil || second.Outcome != OutcomeAbandoned || !strings.Contains(second.Detail, "already recorded") {
		t.Fatalf("second marker must be a no-write no-op: %+v %v", second, err)
	}
	members := testMembers("sliced-parent")
	split, err := Split(verbReq(root, "01J5X00000000000000000SA30", "mac-a"), "sliced-parent", members, mainRatification("sliced-parent", members), nil)
	if err != nil || split.Outcome != OutcomeRejected || !strings.Contains(split.Detail, "first slice") {
		t.Fatalf("sliced parent must refuse split by its durable coordinates: %+v %v", split, err)
	}
	if released, err := Release(verbReq(root, "01J5X00000000000000000SA40", "mac-a"), "sliced-parent"); err != nil || released.Outcome != OutcomeConfirmed {
		t.Fatalf("release: %+v %v", released, err)
	}
	split, err = Split(verbReq(root, "01J5X00000000000000000SA50", "mac-a"), "sliced-parent", members, mainRatification("sliced-parent", members), nil)
	if err != nil || split.Outcome != OutcomeRejected || !strings.Contains(split.Detail, "first slice") {
		t.Fatalf("release must not erase ever-sliced: %+v %v", split, err)
	}
}

func TestHumanCanRatifyAMainOriginSplitWithFreshProof(t *testing.T) {
	_, root := oneClone(t)
	seedLedger(t, root)
	if res, err := Open(verbReq(root, "01J5X00000000000000000SH00", "mac-a"), "human-ratifies-main", "Main-origin intent.", OriginMain, "Split it."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", res, err)
	}
	members := testMembers("human-ratifies-main")
	request := verbReq(root, "01J5X00000000000000000SH10", "mac-a")
	request.Actor.Human = "wido"
	proof := testHumanAuthority(t, root, request.Now)
	ratification := SplitRatification{Tier: RatifierHuman, By: "wido", DraftSHA256: SplitDraftSHA256("human-ratifies-main", members)}
	result, err := Split(request, "human-ratifies-main", members, ratification, proof)
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("fresh human proof did not ratify main-origin split: %+v %v", result, err)
	}
	tree, err := loadTree(root, result.Tip)
	if err != nil || tree.Done["human-ratifies-main"].Ratified.Tier != RatifierHuman || tree.Done["human-ratifies-main"].Ratified.By != "wido" {
		t.Fatalf("human ratification token did not publish: %+v %v", tree.Done["human-ratifies-main"], err)
	}
}

func TestSplitPreconditionsRefuseByNameAndHumanOriginInherits(t *testing.T) {
	t.Run("foreign claim", func(t *testing.T) {
		_, a, b := twoClones(t)
		seedLedger(t, a)
		if res, err := openClaimForTest(t, verbReq(a, "01J5X00000000000000000PF00", "mac-a"), "foreign-parent", "Foreign claim.", OriginMain, "Work.", testBudget()); err != nil || res.Outcome != OutcomeConfirmed {
			t.Fatalf("open claim: %+v %v", res, err)
		}
		members := testMembers("foreign-parent")
		result, err := Split(verbReq(b, "01J5X00000000000000000PF10", "mac-b"), "foreign-parent", members, mainRatification("foreign-parent", members), nil)
		if err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "park or steal") {
			t.Fatalf("foreign claim did not refuse toward the authority transition: %+v %v", result, err)
		}
	})

	for _, archived := range []bool{false, true} {
		name := "live"
		if archived {
			name = "archived"
		}
		t.Run(name+" arc collision", func(t *testing.T) {
			_, root := oneClone(t)
			seedLedger(t, root)
			if res, err := Open(verbReq(root, "01J5X00000000000000000PC00", "mac-a"), "collision-parent", "Collision parent.", OriginMain, "Split."); err != nil || res.Outcome != OutcomeConfirmed {
				t.Fatalf("open parent: %+v %v", res, err)
			}
			if res, err := Open(verbReq(root, "01J5X00000000000000000PC10", "mac-a"), "arc-bearer", "Existing arc bearer.", OriginMain, "Wait."); err != nil || res.Outcome != OutcomeConfirmed {
				t.Fatalf("open bearer: %+v %v", res, err)
			}
			if res, err := SetArc(verbReq(root, "01J5X00000000000000000PC20", "mac-a"), "arc-bearer", "collision-parent"); err != nil || res.Outcome != OutcomeConfirmed {
				t.Fatalf("set collision arc: %+v %v", res, err)
			}
			if archived {
				if res, err := Done(verbReq(root, "01J5X00000000000000000PC30", "mac-a"), "arc-bearer", "Archived bearer."); err != nil || res.Outcome != OutcomeConfirmed {
					t.Fatalf("archive bearer: %+v %v", res, err)
				}
			}
			members := testMembers("collision-parent")
			result, err := Split(verbReq(root, "01J5X00000000000000000PC40", "mac-a"), "collision-parent", members, mainRatification("collision-parent", members), nil)
			if err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "already in use by arc-bearer") {
				t.Fatalf("%s arc collision did not refuse by bearer: %+v %v", name, result, err)
			}
		})
	}

	t.Run("human origin requires proof and propagates", func(t *testing.T) {
		_, root := oneClone(t)
		seedLedger(t, root)
		if res, err := Open(verbReq(root, "01J5X00000000000000000PH00", "mac-a"), "human-parent", "Human origin.", OriginHuman, "Split."); err != nil || res.Outcome != OutcomeConfirmed {
			t.Fatalf("open: %+v %v", res, err)
		}
		members := testMembers("human-parent")
		human := verbReq(root, "01J5X00000000000000000PH10", "mac-a")
		human.Actor.Human = "wido"
		ratification := SplitRatification{Tier: RatifierHuman, By: "wido", DraftSHA256: SplitDraftSHA256("human-parent", members)}
		refused, err := Split(human, "human-parent", members, ratification, nil)
		if err != nil || refused.Outcome != OutcomeRejected || !strings.Contains(refused.Detail, "fresh enrolled-terminal proof") {
			t.Fatalf("human-origin split without proof did not refuse: %+v %v", refused, err)
		}
		human.Ulid = "01J5X00000000000000000PH20"
		result, err := Split(human, "human-parent", members, ratification, testHumanAuthority(t, root, human.Now))
		if err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("proven human split: %+v %v", result, err)
		}
		tree, err := loadTree(root, result.Tip)
		if err != nil || tree.Live["human-parent-one"].Origin != OriginHuman || tree.Live["human-parent-two"].Origin != OriginHuman {
			t.Fatalf("human origin did not propagate: %+v %v", tree, err)
		}
	})
}

func TestSplitDebtFailureStaysPushedAndRecoveryCompletesIt(t *testing.T) {
	_, root := oneClone(t)
	seedLedger(t, root)
	if res, err := Open(verbReq(root, "01J5X00000000000000000SD00", "mac-a"), "debt-parent", "Exercise debt recovery.", OriginMain, "Split it."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", res, err)
	}
	if res, err := SetArc(verbReq(root, "01J5X00000000000000000SD10", "mac-a"), "debt-parent", "old-debt-arc"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("set arc: %+v %v", res, err)
	}
	// Registers resolve below memory/. A file at that directory is a
	// reversible fixture fault that makes the post-confirm debt raise fail.
	if err := os.WriteFile(filepath.Join(root, "memory"), []byte("fixture fault\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	members := testMembers("debt-parent")
	request := verbReq(root, "01J5X00000000000000000SD20", "mac-a")
	result, splitErr := Split(request, "debt-parent", members, mainRatification("debt-parent", members), nil)
	if splitErr == nil || result.Outcome != "" || !strings.Contains(splitErr.Error(), "old-arc retro debt") {
		t.Fatalf("debt failure did not leave confirmation unresolved: %+v %v", result, splitErr)
	}
	entry, err := ReadEntry(root, request.opid())
	if err != nil || entry.Phase != PhasePushed {
		t.Fatalf("split terminalized before its required debt: %+v %v", entry, err)
	}
	if err := os.Remove(filepath.Join(root, "memory")); err != nil {
		t.Fatal(err)
	}
	reports, err := Recover(endpointFor(root))
	if err != nil {
		t.Fatal(err)
	}
	entry, err = ReadEntry(root, request.opid())
	if err != nil || entry.Outcome != OutcomeConfirmed {
		t.Fatalf("recovery did not terminalize after the debt landed: %+v %v reports=%+v", entry, err, reports)
	}
	debts, err := retrodebt.Open(root)
	if err != nil || len(debts) != 1 || !strings.HasPrefix(debts[0].Source, "old-debt-arc:") {
		t.Fatalf("recovery did not raise exactly one debt: %+v %v", debts, err)
	}
}

func TestDeadAbsentSliceStartAbandonsWithoutMarkingGoal(t *testing.T) {
	_, root := oneClone(t)
	seedLedger(t, root)
	seedGoalNormConfig(t, root)
	if res, err := openClaimForTest(t, verbReq(root, "01J5X00000000000000000SR00", "mac-a"), "recover-slice", "Recover safely.", OriginMain, "Work.", testBudget()); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open claim: %+v %v", res, err)
	}
	opid := Opid("01J5X00000000000000000SR10", "mac-a", "lin-1")
	entry, err := CreateEntry(root, opid, "mac-a", "lin-1", Intent{Verb: "slice-start", Targets: []string{"recover-slice"}})
	if err != nil {
		t.Fatal(err)
	}
	entry.Owner.Pid = 99999999
	entry.Owner.PidStartedAt = 1
	entry.Owner.StartTicks = 0
	entry.Owner.BootID = ""
	if err := writeEntry(root, entry); err != nil {
		t.Fatal(err)
	}
	reports, err := Recover(endpointFor(root))
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 1 || !strings.Contains(reports[0].Detail, "abandoned without marking") {
		t.Fatalf("slice-start recovery took the wrong terminal path: %+v", reports)
	}
	terminal, err := ReadEntry(root, opid)
	if err != nil || terminal.Outcome != OutcomeAbandoned {
		t.Fatalf("slice-start recovery did not terminalize abandoned: %+v %v", terminal, err)
	}
	projection, err := Project(endpointFor(root), false, verbReq(root, "01J5X00000000000000000SR20", "mac-a").Now)
	if err != nil || projection.Tree.Live["recover-slice"].Sliced != nil {
		t.Fatalf("absent slice-start recovery falsely marked the goal: %+v %v", projection.Tree.Live["recover-slice"], err)
	}
}

func TestGoalNormRefusesAndPublishesStrictApproval(t *testing.T) {
	_, root := oneClone(t)
	seedLedger(t, root)
	seedGoalNormConfig(t, root)
	if res, err := Open(verbReq(root, "01J5X00000000000000000GN00", "mac-a"), "large-goal", "Large goal.", OriginMain, "Split it."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", res, err)
	}
	over := Budget{ElapsedLimit: "1h", AttemptLimit: 1, ReservedJobMinutesLimit: 1201, ActiveJobLimit: 1, ReviewRoundLimit: 3}
	request := verbReq(root, "01J5X00000000000000000GN10", "mac-a")
	request.Actor.Human = "wido"
	refused, err := Approve(request, []string{"large-goal"}, &over, testHumanAuthority(t, root, request.Now))
	if err != nil || refused.Outcome != OutcomeRejected || !strings.Contains(refused.Detail, "GOAL_NORM_REFUSED") || !strings.Contains(refused.Detail, "split it into an arc of members within the box") {
		t.Fatalf("over-norm claim did not exercise the typed split remedy: %+v %v", refused, err)
	}
	if err := os.MkdirAll(filepath.Join(root, "memory"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "memory", "rulings.md"), []byte("| R-25b | goal=large-goal minutes=1600 reviewRounds=3 goalRevision=1 |\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	approved := verbReq(root, "01J5X00000000000000000GN20", "mac-a")
	approved.Actor.Human = "wido"
	approved.ApprovedRef = "R-25b"
	result, err := Approve(approved, []string{"large-goal"}, &over, testHumanAuthority(t, root, approved.Now))
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("strict approval did not admit set-budget: %+v %v", result, err)
	}
	tree, err := loadTree(root, result.Tip)
	if err != nil || tree.Live["large-goal"].NormApproval == nil || tree.Live["large-goal"].NormApproval.Minutes != 1600 {
		t.Fatalf("approved admission did not publish its proof: %+v %v", tree.Live["large-goal"], err)
	}
	within := testBudget()
	withinReq := verbReq(root, "01J5X00000000000000000GN30", "mac-a")
	withinReq.Actor.Human = "wido"
	result, err = Approve(withinReq, []string{"large-goal"}, &within, testHumanAuthority(t, root, withinReq.Now))
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("within-norm replacement: %+v %v", result, err)
	}
	tree, _ = loadTree(root, result.Tip)
	if tree.Live["large-goal"].NormApproval != nil {
		t.Fatal("within-norm replacement must clear stale approval evidence")
	}
	if res, err := Open(verbReq(root, "01J5X00000000000000000GN40", "mac-a"), "at-norm", "Exactly bounded.", OriginMain, "Proceed."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open at-norm: %+v %v", res, err)
	}
	atNorm := Budget{ElapsedLimit: "1h", AttemptLimit: 1, ReservedJobMinutesLimit: 1200, ActiveJobLimit: 1, ReviewRoundLimit: 3}
	atNormReq := verbReq(root, "01J5X00000000000000000GN50", "mac-a")
	atNormReq.Actor.Human = "wido"
	if res, err := Approve(atNormReq, []string{"at-norm"}, &atNorm, testHumanAuthority(t, root, atNormReq.Now)); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("the exact norm boundary must pass without approval: %+v %v", res, err)
	}
}

func TestOverNormApprovalComposesWithClaimAndSteal(t *testing.T) {
	_, a, b := twoClones(t)
	seedLedger(t, a)
	if res, err := Open(verbReq(a, "01J5X00000000000000000NS00", "mac-a"), "over-steal", "Approved exception.", OriginMain, "Work."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", res, err)
	}
	if err := os.MkdirAll(filepath.Join(a, "memory"), 0o755); err != nil {
		t.Fatal(err)
	}
	over := Budget{ElapsedLimit: "1h", AttemptLimit: 1, ReservedJobMinutesLimit: 1500, ActiveJobLimit: 1, ReviewRoundLimit: 3}
	if err := os.WriteFile(filepath.Join(a, "memory", "rulings.md"), []byte("| R-301 | goal=over-steal minutes=1500 reviewRounds=3 goalRevision=1 |\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	set := verbReq(a, "01J5X00000000000000000NS10", "mac-a")
	set.Actor.Human = "wido"
	set.ApprovedRef = "R-301"
	if res, err := Approve(set, []string{"over-steal"}, &over, testHumanAuthority(t, a, set.Now)); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("approved over-norm set-budget: %+v %v", res, err)
	}
	claim := verbReq(a, "01J5X00000000000000000NS30", "mac-a")
	if res, err := Claim(claim, "over-steal"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("approved stored claim: %+v %v", res, err)
	}
	steal := verbReq(b, "01J5X00000000000000000NS40", "mac-b")
	steal.Actor.Human = "wido"
	if res, err := Steal(steal, "over-steal"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("the bound over-norm approval did not compose with steal: %+v %v", res, err)
	}
}

func TestGoalNormApprovalGrammarIsDistinctAndUnambiguous(t *testing.T) {
	minutes, rounds, revision, ok := StrictApprovalQuadruple("approve goal=large-goal minutes=1600 reviewRounds=3 goalRevision=4", "large-goal")
	if !ok || minutes != 1600 || rounds != 3 || revision != 4 {
		t.Fatalf("strict goal approval did not parse: minutes=%d rounds=%d revision=%d ok=%v", minutes, rounds, revision, ok)
	}
	if _, _, _, ok := StrictApprovalQuadruple("goal=large-goal capMin=1600 goalRevision=4", "large-goal"); ok {
		t.Fatal("a slice-cap approval silently doubled as a goal-norm approval")
	}
	if _, _, _, ok := StrictApprovalQuadruple("goal=large-goal minutes=1600 reviewRounds=3 goalRevision=4 goal=large-goal minutes=1700 reviewRounds=3 goalRevision=4", "large-goal"); ok {
		t.Fatal("two distinct approval triples were accepted as one human word")
	}
	if _, _, _, ok := StrictApprovalQuadruple("goal=large-goal minutes=1600 reviewRounds=3 goalRevision=4 goal=large-goal minutes=1600 reviewRounds=3 goalRevision=4", "large-goal"); ok {
		t.Fatal("two byte-identical approval occurrences were deduplicated instead of refused")
	}
}

func TestSplitAndSliceStartRaceHasExactlyOneWinner(t *testing.T) {
	_, a, b := twoClones(t)
	seedLedger(t, a)
	if res, err := openClaimForTest(t, verbReq(a, "01J5X00000000000000000RC00", "mac-a"), "race-parent", "Race the boundary.", OriginMain, "Choose one.", testBudget()); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open claimed parent: %+v %v", res, err)
	}
	members := testMembers("race-parent")
	splitVerb := verbReq(a, "01J5X00000000000000000RC10", "mac-a")
	req, err := splitRequest(splitVerb, "race-parent", members, mainRatification("race-parent", members), nil)
	if err != nil {
		t.Fatal(err)
	}
	markerRan := false
	req.BeforePush = func(attempt int) error {
		if markerRan {
			return nil
		}
		markerRan = true
		marked, markErr := MarkSliced(verbReq(b, "01J5X00000000000000000RC20", "mac-a"), "race-parent")
		if markErr != nil || marked.Outcome != OutcomeConfirmed {
			t.Fatalf("slice-start competitor: %+v %v", marked, markErr)
		}
		return nil
	}
	result, err := Publish(endpointFor(a), req)
	if err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "first slice") {
		t.Fatalf("split did not lose by the durable sliced fact: %+v %v", result, err)
	}
	advanced, err := FetchAdvance(endpointFor(a))
	if err != nil {
		t.Fatal(err)
	}
	tree, err := loadTree(a, advanced.Tip)
	if err != nil || tree.Live["race-parent"].Sliced == nil || tree.Done["race-parent"] != nil || tree.Live["race-parent-one"] != nil {
		t.Fatalf("exactly slice-start won and no split bytes landed: %+v %v", tree, err)
	}
}

func TestParkedEverSlicedParentStillRefusesSplit(t *testing.T) {
	_, root := oneClone(t)
	seedLedger(t, root)
	if res, err := openClaimForTest(t, verbReq(root, "01J5X00000000000000000PS00", "mac-a"), "parked-sliced", "Started work.", OriginMain, "Pause.", testBudget()); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open claim: %+v %v", res, err)
	}
	if res, err := MarkSliced(verbReq(root, "01J5X00000000000000000PS10", "mac-a"), "parked-sliced"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("mark sliced: %+v %v", res, err)
	}
	human := verbReq(root, "01J5X00000000000000000PS20", "mac-a")
	human.Actor.Human = "wido"
	if res, err := Park(human, "parked-sliced", "operator pause"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("park: %+v %v", res, err)
	}
	members := testMembers("parked-sliced")
	splitHuman := verbReq(root, "01J5X00000000000000000PS30", "mac-a")
	splitHuman.Actor.Human = "wido"
	result, err := Split(splitHuman, "parked-sliced", members, mainRatification("parked-sliced", members), nil)
	if err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "first slice") {
		t.Fatalf("parked state bypassed the immutable sliced refusal: %+v %v", result, err)
	}
}

func TestGoalNormApprovalHistoryStalenessAndAtRestCoverage(t *testing.T) {
	_, root := oneClone(t)
	seedLedger(t, root)
	for index, id := range []string{"history-large", "approval-carrier", "stale-large"} {
		ulid := []string{"01J5X00000000000000000NH00", "01J5X00000000000000000NH10", "01J5X00000000000000000NH20"}[index]
		if res, err := Open(verbReq(root, ulid, "mac-a"), id, "Approval fixture.", OriginMain, "Continue."); err != nil || res.Outcome != OutcomeConfirmed {
			t.Fatalf("open %s: %+v %v", id, res, err)
		}
	}
	human := verbReq(root, "01J5X00000000000000000NH30", "mac-a")
	human.Actor.Human = "wido"
	reason := "goal=history-large minutes=1600 reviewRounds=3 goalRevision=1"
	if res, err := Park(human, "approval-carrier", reason); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("record human history approval: %+v %v", res, err)
	}
	over := Budget{ElapsedLimit: "1h", AttemptLimit: 1, ReservedJobMinutesLimit: 1500, ActiveJobLimit: 1, ReviewRoundLimit: 3}
	approved := verbReq(root, "01J5X00000000000000000NH40", "mac-a")
	approved.Actor.Human = "wido"
	approved.ApprovedRef = human.opid()
	if res, err := Approve(approved, []string{"history-large"}, &over, testHumanAuthority(t, root, approved.Now)); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("history-line approval channel did not admit the tuple: %+v %v", res, err)
	}
	if err := os.MkdirAll(filepath.Join(root, "memory"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "memory", "rulings.md"), []byte("| R-251 | goal=stale-large minutes=1600 reviewRounds=3 goalRevision=1 |\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	next := "Advance the revision."
	if res, err := Edit(verbReq(root, "01J5X00000000000000000NH50", "mac-a"), "stale-large", EditFields{NextStep: &next}); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("advance stale goal: %+v %v", res, err)
	}
	stale := verbReq(root, "01J5X00000000000000000NH60", "mac-a")
	stale.Actor.Human = "wido"
	stale.ApprovedRef = "R-251"
	result, err := Approve(stale, []string{"stale-large"}, &over, testHumanAuthority(t, root, stale.Now))
	if err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "not current revision 2") {
		t.Fatalf("stale approval did not refuse by both revisions: %+v %v", result, err)
	}

	invalid := vGoal("invalid-approval", StateQueued)
	invalid.Budget = &Budget{ElapsedLimit: "1h", AttemptLimit: 1, ReservedJobMinutesLimit: 1500, ActiveJobLimit: 1, ReviewRoundLimit: 3}
	invalid.NormApproval = &GoalNormApprovalClaim{ApprovedRef: "R-25b", Minutes: 1499, ReviewRounds: 3, GoalRevision: invalid.Revision}
	problems := ValidateTree(&TreeGoals{Root: vRoot(), Live: map[string]*GoalFile{"invalid-approval": invalid}, Done: map[string]*GoalFile{}})
	if !problemsContain(problems, "NormApproval proves 1499 minutes but Budget reserves 1500 minutes") {
		t.Fatalf("at-rest approval coverage was not enforced: %v", problems)
	}
}

func TestReconcileRefusesGeneratedScopeBoundaryFields(t *testing.T) {
	base := vGoal("boundary-fields", StateQueued)
	tests := []struct {
		name string
		edit func(*GoalFile)
		want string
	}{
		{"norm approval", func(file *GoalFile) {
			file.NormApproval = &GoalNormApprovalClaim{ApprovedRef: "R-25b", Minutes: 2000, GoalRevision: 1}
		}, "NormApproval"},
		{"sliced", func(file *GoalFile) {
			file.Sliced = &SlicedRecord{Machine: "mac-a", Lineage: "lin-1", Revision: 1, At: "2026-08-20T10:00:00Z"}
		}, "Sliced"},
		{"ratified", func(file *GoalFile) {
			file.Ratified = &SplitRatification{Tier: RatifierMain, MainID: "main-1", ClaimEpoch: 1, DraftSHA256: strings.Repeat("a", 64)}
		}, "Ratified"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			edited := *base
			test.edit(&edited)
			if _, err := mapOneChange("plans/goals/boundary-fields.md", base, &edited); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("generated field did not refuse by name: %v", err)
			}
		})
	}
}

func TestValidatorRefusesADecomposedParentMadeLiveAgain(t *testing.T) {
	root := vRoot()
	root.Decomposed = []DecomposedEntry{{Id: "retired-parent", Opid: Opid("01J5X00000000000000000SV00", "mac-a", "lin-1"), At: "2026-08-20T10:00:00Z"}}
	problems := ValidateTree(&TreeGoals{Root: root, Live: map[string]*GoalFile{"retired-parent": vGoal("retired-parent", StateQueued)}, Done: map[string]*GoalFile{}})
	if !problemsContain(problems, "decomposed parent retired-parent is live again") {
		t.Fatalf("decomposition registry did not enforce permanent retirement: %v", problems)
	}
}
