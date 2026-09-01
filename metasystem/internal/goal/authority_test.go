package goal

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// The transition-authority fold (review r1, F12/F13): claim is
// agent-only, the pair is the ownership key, human-origin goals are
// human-reserved, edit checks authority, steal cascades, reopen
// adopts the arc's standing state, set-arc composes under the
// membership matrix, prune's closure seeds from keep survivors, and
// arc members remain independently claimable.

func TestClaimIsAgentOnlyAndPairKeyed(t *testing.T) {
	_, a, _ := twoClones(t)
	seedLedger(t, a)
	if res, err := Open(verbReq(a, "01J5X00000000000000000AK00", "mac-a"), "pair-keyed", "Pair semantics.", "main", "Go."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", res, err)
	}

	// Claim under a human name refuses up front.
	humanClaim := verbReq(a, "01J5X00000000000000000AK10", "mac-a")
	humanClaim.Actor.Human = "wido"
	if _, err := Claim(humanClaim, "pair-keyed", testBudget()); err == nil || !strings.Contains(err.Error(), "agent-only") {
		t.Fatalf("humans cannot claim: %v", err)
	}
	if _, err := OpenClaim(humanClaim, "other", "X.", "main", "Go.", testBudget()); err == nil || !strings.Contains(err.Error(), "agent-only") {
		t.Fatalf("humans cannot open --claim: %v", err)
	}
	if _, err := ClaimArc(humanClaim, "pair-keyed", testBudget()); err == nil || !strings.Contains(err.Error(), "agent-only") {
		t.Fatalf("humans cannot claim arcs: %v", err)
	}

	if res, err := Claim(verbReq(a, "01J5X00000000000000000AK20", "mac-a"), "pair-keyed", testBudget()); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("claim: %+v %v", res, err)
	}

	// A second lineage on the SAME machine is a stranger: the pair
	// is the ownership key, and the refusal names the standing
	// lineage instead of pretending idempotence.
	secondLineage := verbReq(a, "01J5X00000000000000000AK30", "mac-a")
	secondLineage.Actor.Lineage = "lin-2"
	res, err := Claim(secondLineage, "pair-keyed", testBudget())
	if err != nil || res.Outcome != OutcomeRejected || !strings.Contains(res.Detail, "lineage lin-1") {
		t.Fatalf("a second lineage refuses by name: %+v %v", res, err)
	}
	secondLineage.Ulid = "01J5X00000000000000000AK35"
	res, err = Release(secondLineage, "pair-keyed")
	if err != nil || res.Outcome != OutcomeRejected || !strings.Contains(res.Detail, "human act") {
		t.Fatalf("a second lineage cannot release the pair's claim: %+v %v", res, err)
	}
	// The pair itself replays idempotent-shaped.
	res, err = Claim(verbReq(a, "01J5X00000000000000000AK40", "mac-a"), "pair-keyed", testBudget())
	if err != nil || res.Outcome != OutcomeAbandoned || !strings.Contains(res.Detail, "already claimed by this pair") {
		t.Fatalf("the pair's re-claim abandons by name: %+v %v", res, err)
	}
}

func TestHumanOriginGoalsAreHumanReserved(t *testing.T) {
	_, a, _ := twoClones(t)
	seedLedger(t, a)
	if res, err := Open(verbReq(a, "01J5X00000000000000000HG00", "mac-a"), "human-owned", "Wido's standing wish.", OriginHuman, "Go."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", res, err)
	}
	// An agent can neither conclude nor park the human's goal.
	res, err := Done(verbReq(a, "01J5X00000000000000000HG10", "mac-a"), "human-owned", "Presumed done.")
	if err != nil || res.Outcome != OutcomeRejected || !strings.Contains(res.Detail, "human act") {
		t.Fatalf("agent done on human-origin refuses: %+v %v", res, err)
	}
	res, err = Park(verbReq(a, "01J5X00000000000000000HG20", "mac-a"), "human-owned", "agent tidy-up")
	if err != nil || res.Outcome != OutcomeRejected || !strings.Contains(res.Detail, "human reservation") {
		t.Fatalf("agent park on human-origin refuses: %+v %v", res, err)
	}
	// The human parks it; the agent cannot lift the pause; the human
	// can.
	humanReq := verbReq(a, "01J5X00000000000000000HG30", "mac-a")
	humanReq.Actor.Human = "wido"
	if res, err := Park(humanReq, "human-owned", "on hold"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("human park: %+v %v", res, err)
	}
	res, err = Unpark(verbReq(a, "01J5X00000000000000000HG40", "mac-a"), "human-owned")
	if err != nil || res.Outcome != OutcomeRejected || !strings.Contains(res.Detail, "human's pause") {
		t.Fatalf("agent unpark of a human's park refuses: %+v %v", res, err)
	}
	humanUnpark := verbReq(a, "01J5X00000000000000000HG50", "mac-a")
	humanUnpark.Actor.Human = "wido"
	if res, err := Unpark(humanUnpark, "human-owned"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("human unpark: %+v %v", res, err)
	}
	humanDone := verbReq(a, "01J5X00000000000000000HG60", "mac-a")
	humanDone.Actor.Human = "wido"
	if res, err := Done(humanDone, "human-owned", "Wido closed it."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("human done: %+v %v", res, err)
	}
}

func TestEditChecksAuthorityAndTheBlockerInvariant(t *testing.T) {
	_, a, b := twoClones(t)
	seedLedger(t, a)
	for _, leg := range []struct{ ulid, id string }{
		{"01J5X00000000000000000EA00", "held"},
		{"01J5X00000000000000000EA10", "loose"},
	} {
		if res, err := Open(verbReq(a, leg.ulid, "mac-a"), leg.id, "Work "+leg.id, "main", "Go."); err != nil || res.Outcome != OutcomeConfirmed {
			t.Fatalf("open %s: %+v %v", leg.id, res, err)
		}
	}
	if res, err := Claim(verbReq(a, "01J5X00000000000000000EA20", "mac-a"), "held", testBudget()); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("claim: %+v %v", res, err)
	}

	// A foreign agent cannot edit the claim.
	intent := "Rewritten by a stranger."
	res, err := Edit(verbReq(b, "01J5X00000000000000000EA30", "mac-b"), "held", EditFields{Intent: &intent})
	if err != nil || res.Outcome != OutcomeRejected || !strings.Contains(res.Detail, "human act") {
		t.Fatalf("foreign agent edit refuses: %+v %v", res, err)
	}
	// The foreign HUMAN can — and the override leaves the
	// displacement signal.
	humanEdit := verbReq(b, "01J5X00000000000000000EA40", "mac-b")
	humanEdit.Actor.Human = "wido"
	res, err = Edit(humanEdit, "held", EditFields{Intent: &intent})
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("human edit: %+v %v", res, err)
	}
	t2, err := loadTree(b, res.Tip)
	if err != nil {
		t.Fatal(err)
	}
	held := t2.Live["held"]
	last := held.History[len(held.History)-1]
	if !strings.HasPrefix(last.Displaced, "mac-a+lin-1@") {
		t.Fatalf("the foreign-human edit records displaced=: %+v", last)
	}
	if held.Claimed == nil || held.Claimed.Machine != "mac-a" {
		t.Fatal("an edit never moves the claim itself")
	}
	// A claimed goal is never blocked — even the human cannot
	// hang a live blocker on it.
	edge := []string{"loose"}
	humanEdge := verbReq(b, "01J5X00000000000000000EA50", "mac-b")
	humanEdge.Actor.Human = "wido"
	res, err = Edit(humanEdge, "held", EditFields{Blocked: &edge})
	if err != nil || res.Outcome != OutcomeRejected || !strings.Contains(res.Detail, "never blocked") {
		t.Fatalf("a live blocker on a claimed goal refuses for every actor: %+v %v", res, err)
	}
}

// arcBed opens two goals, wires them into one arc, and claims it.
// code is two ulid-safe characters keeping the bed's opids unique.
func arcBed(t *testing.T, a, arc, prefix, code string) {
	t.Helper()
	const base = "01J5X00000000000000000"
	for i, id := range []string{prefix + "-one", prefix + "-two"} {
		if res, err := Open(verbReq(a, fmt.Sprintf("%s%s%d0", base, code, i), "mac-a"), id, "Arc "+id, "main", "Go."); err != nil || res.Outcome != OutcomeConfirmed {
			t.Fatalf("open %s: %+v %v", id, res, err)
		}
		if res, err := SetArc(verbReq(a, fmt.Sprintf("%s%s%d1", base, code, i), "mac-a"), id, arc); err != nil || res.Outcome != OutcomeConfirmed {
			t.Fatalf("set-arc %s: %+v %v", id, res, err)
		}
	}
	if res, err := ClaimArc(verbReq(a, base+code+"90", "mac-a"), prefix+"-one", testBudget()); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("claim arc: %+v %v", res, err)
	}
}

func TestStealCascadesAcrossTheArc(t *testing.T) {
	_, a, b := twoClones(t)
	seedLedger(t, a)
	arcBed(t, a, "steal-arc", "st", "SC")

	steal := verbReq(b, "01J5X00000000000000000SC00", "mac-b")
	steal.Actor.Human = "wido"
	res, err := Steal(steal, "st-one")
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("steal: %+v %v", res, err)
	}
	t2, err := loadTree(b, res.Tip)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"st-one", "st-two"} {
		m := t2.Live[id]
		if m.State != StateClaimed || m.Claimed == nil || m.Claimed.Machine != "mac-b" || m.Claimed.Lineage != "lin-1" {
			t.Fatalf("the steal reassigns EVERY live member (the claim binds the arc): %s %+v", id, m.Claimed)
		}
		last := m.History[len(m.History)-1]
		if last.Verb != "steal" || !strings.HasPrefix(last.Displaced, "mac-a+lin-1@") {
			t.Fatalf("every touched line carries the displaced marker: %s %+v", id, last)
		}
	}
}

func TestReopenAdoptsTheArcState(t *testing.T) {
	_, a, b := twoClones(t)
	seedLedger(t, a)
	arcBed(t, a, "adopt-arc", "ra", "RA")
	if res, err := Done(verbReq(a, "01J5X00000000000000000RA80", "mac-a"), "ra-two", "First pass shipped."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("done: %+v %v", res, err)
	}

	// A foreign claim no longer owns the arc: an outside agent reopens
	// the member queued instead of injecting it into that claim.
	res, err := Reopen(verbReq(b, "01J5X00000000000000000RA81", "mac-b"), "ra-two")
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("stranger reopen into a mixed arc: %+v %v", res, err)
	}
	foreignTree, loadErr := loadTree(b, res.Tip)
	if loadErr != nil || foreignTree.Live["ra-two"].State != StateQueued {
		t.Fatalf("stranger must reopen queued without adopting a foreign claim: %+v %v", foreignTree.Live["ra-two"], loadErr)
	}
	if res, err := Done(verbReq(b, "01J5X00000000000000000RA87", "mac-b"), "ra-two", "Queued reopen checked."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("archive the queued reopen: %+v %v", res, err)
	}
	// The claimant's reopen rejoins CLAIMED under the standing pair.
	res, err = Reopen(verbReq(a, "01J5X00000000000000000RA82", "mac-a"), "ra-two")
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("claimant reopen: %+v %v", res, err)
	}
	t2, err := loadTree(a, res.Tip)
	if err != nil {
		t.Fatal(err)
	}
	back := t2.Live["ra-two"]
	if back.State != StateClaimed || back.Claimed == nil || back.Claimed.Machine != "mac-a" || back.Claimed.Lineage != "lin-1" {
		t.Fatalf("the member rejoins under the standing claimant: %+v", back)
	}

	// A parked arc adopts human-only: park the arc, conclude a
	// member (human), reopen it — the agent refuses, the human lands
	// it parked WITH the arc's record.
	if res, err := Done(verbReq(a, "01J5X00000000000000000RA83", "mac-a"), "ra-two", "Second pass shipped."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("done again: %+v %v", res, err)
	}
	if res, err := ParkArc(verbReq(a, "01J5X00000000000000000RA84", "mac-a"), "ra-one", "waiting on vendor"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("park arc: %+v %v", res, err)
	}
	res, err = Reopen(verbReq(a, "01J5X00000000000000000RA85", "mac-a"), "ra-two")
	if err != nil || res.Outcome != OutcomeRejected || !strings.Contains(res.Detail, "human act") {
		t.Fatalf("agent reopen into a parked arc refuses: %+v %v", res, err)
	}
	humanReopen := verbReq(a, "01J5X00000000000000000RA86", "mac-a")
	humanReopen.Actor.Human = "wido"
	res, err = Reopen(humanReopen, "ra-two")
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("human reopen: %+v %v", res, err)
	}
	t3, err := loadTree(a, res.Tip)
	if err != nil {
		t.Fatal(err)
	}
	rejoined := t3.Live["ra-two"]
	if rejoined.State != StateParked || rejoined.Parked == nil || rejoined.Parked.Because != "waiting on vendor" {
		t.Fatalf("the member rejoins parked with the arc's record: %+v", rejoined)
	}
}

func TestSetArcComposesMovesUnderTheMatrix(t *testing.T) {
	_, a, _ := twoClones(t)
	seedLedger(t, a)

	// A claimed standalone goal cannot join an arc: release first.
	// (This leg runs before the bed claims the arc — the quota
	// admits one claim per machine.)
	if res, err := OpenClaim(verbReq(a, "01J5X00000000000000000MX00", "mac-a"), "solo-held", "Solo.", "main", "Go.", testBudget()); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open --claim: %+v %v", res, err)
	}
	res, err := SetArc(verbReq(a, "01J5X00000000000000000MX10", "mac-a"), "solo-held", "move-src")
	if err != nil || res.Outcome != OutcomeRejected || !strings.Contains(res.Detail, "release first") {
		t.Fatalf("a claimed standalone refuses to join: %+v %v", res, err)
	}
	if res, err := Release(verbReq(a, "01J5X00000000000000000MX15", "mac-a"), "solo-held"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("release: %+v %v", res, err)
	}
	if res, err := Done(verbReq(a, "01J5X00000000000000000MX16", "mac-a"), "solo-held", "Out of the way."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("done: %+v %v", res, err)
	}
	arcBed(t, a, "move-src", "mv", "MV")

	// The claimant moves a member out of its claimed arc into a
	// fresh arc: released as it detaches, lands QUEUED
	// (auto-claim fires only when the DESTINATION is claimed).
	res, err = SetArc(verbReq(a, "01J5X00000000000000000MX20", "mac-a"), "mv-two", "move-dst")
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("move between arcs: %+v %v", res, err)
	}
	t2, err := loadTree(a, res.Tip)
	if err != nil {
		t.Fatal(err)
	}
	moved := t2.Live["mv-two"]
	if moved.Arc != "move-dst" || moved.State != StateQueued || moved.Claimed != nil {
		t.Fatalf("the move lands queued in the destination: %+v", moved)
	}

	// A parked destination is human-only, and the join adopts the
	// arc's park record.
	if res, err := ParkArc(verbReq(a, "01J5X00000000000000000MX30", "mac-a"), "mv-one", "arc on hold"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("park the source arc: %+v %v", res, err)
	}
	if res, err := Open(verbReq(a, "01J5X00000000000000000MX40", "mac-a"), "joiner", "Wants in.", "main", "Go."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open joiner: %+v %v", res, err)
	}
	res, err = SetArc(verbReq(a, "01J5X00000000000000000MX50", "mac-a"), "joiner", "move-src")
	if err != nil || res.Outcome != OutcomeRejected || !strings.Contains(res.Detail, "human act") {
		t.Fatalf("an agent cannot edit a parked arc's membership: %+v %v", res, err)
	}
	humanJoin := verbReq(a, "01J5X00000000000000000MX60", "mac-a")
	humanJoin.Actor.Human = "wido"
	res, err = SetArc(humanJoin, "joiner", "move-src")
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("human join into the parked arc: %+v %v", res, err)
	}
	t3, err := loadTree(a, res.Tip)
	if err != nil {
		t.Fatal(err)
	}
	joined := t3.Live["joiner"]
	if joined.State != StateParked || joined.Parked == nil || joined.Parked.Because != "arc on hold" {
		t.Fatalf("the joiner parks with the arc's record: %+v", joined)
	}
}

func TestPruneRetainsKeepSurvivorsBlockers(t *testing.T) {
	_, a, _ := twoClones(t)
	seedLedger(t, a)
	mk := func(ulid, id string, blocked []string) {
		if res, err := Open(verbReq(a, ulid, "mac-a"), id, "Work "+id, "main", "Go."); err != nil || res.Outcome != OutcomeConfirmed {
			t.Fatalf("open %s: %+v %v", id, res, err)
		}
		if blocked != nil {
			if res, err := Edit(verbReq(a, ulid[:len(ulid)-1]+"E", "mac-a"), id, EditFields{Blocked: &blocked}); err != nil || res.Outcome != OutcomeConfirmed {
				t.Fatalf("edge %s: %+v %v", id, res, err)
			}
		}
		if res, err := Done(verbReq(a, ulid[:len(ulid)-1]+"D", "mac-a"), id, "Done "+id); err != nil || res.Outcome != OutcomeConfirmed {
			t.Fatalf("done %s: %+v %v", id, res, err)
		}
	}
	// Same OpenedAt (the bed's fixed clock): the id breaks the tie,
	// so z-newest IS the keep-count survivor. Its blocker has no
	// live edge — before the fix it died and the edge dangled.
	mk("01J5X00000000000000000PK00", "a-buried-blocker", nil)
	mk("01J5X00000000000000000PK10", "m-loose", nil)
	mk("01J5X00000000000000000PK20", "z-newest", []string{"a-buried-blocker"})

	res, err := Prune(verbReq(a, "01J5X00000000000000000PK30", "mac-a"), 1)
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("prune: %+v %v", res, err)
	}
	t2, err := loadTree(a, res.Tip)
	if err != nil {
		t.Fatal(err)
	}
	if _, kept := t2.Done["z-newest"]; !kept {
		t.Fatal("the keep-count survivor stays")
	}
	if _, kept := t2.Done["a-buried-blocker"]; !kept {
		t.Fatal("the survivor's own blocker is retained WITH it (closure from keep survivors)")
	}
	if _, gone := t2.Done["m-loose"]; gone {
		t.Fatal("outside the closed set, the rest die")
	}
}

func TestValidatorAllowsIndependentArcMembers(t *testing.T) {
	// An arc is grouping only: members may be in different states.
	one := vGoal("mix-one", StateQueued)
	one.Arc = "the-arc"
	two := vGoal("mix-two", StateParked)
	two.Arc = "the-arc"
	problems := ValidateTree(&TreeGoals{Root: vRoot(), Live: map[string]*GoalFile{
		"mix-one": one, "mix-two": two,
	}, Done: map[string]*GoalFile{}})
	for _, problem := range problems {
		if strings.Contains(string(problem), "arc the-arc") {
			t.Fatalf("mixed member states are lawful: %v", problems)
		}
	}

	// Two claimant pairs in one arc are likewise independent.
	ca := vGoal("pair-one", StateClaimed)
	ca.Arc = "held-arc"
	cb := vGoal("pair-two", StateClaimed)
	cb.Arc = "held-arc"
	cb.Claimed = &ClaimRecord{Machine: "mac-b", Lineage: "lin-9", At: "2026-08-20T10:06:00Z"}
	problems = ValidateTree(&TreeGoals{Root: vRoot(), Live: map[string]*GoalFile{
		"pair-one": ca, "pair-two": cb,
	}, Done: map[string]*GoalFile{}})
	for _, problem := range problems {
		if strings.Contains(string(problem), "two claimant pairs") {
			t.Fatalf("independent claimant pairs are lawful: %v", problems)
		}
	}

	// A uniform claimed arc remains lawful too.
	cc := vGoal("uni-one", StateClaimed)
	cc.Arc = "clean-arc"
	cd := vGoal("uni-two", StateClaimed)
	cd.Arc = "clean-arc"
	problems = ValidateTree(&TreeGoals{Root: vRoot(), Live: map[string]*GoalFile{
		"uni-one": cc, "uni-two": cd,
	}, Done: map[string]*GoalFile{}})
	for _, p := range problems {
		if strings.Contains(string(p), "arc") {
			t.Fatalf("a uniform arc is lawful: %v", problems)
		}
	}
}

func TestMixedArcCascadesMoveOnlyEligibleMembers(t *testing.T) {
	_, a, b := twoClones(t)
	seedLedger(t, a)
	for index, id := range []string{"mixed-one", "mixed-two", "mixed-parked"} {
		ulid := []string{"01J5X00000000000000000MC00", "01J5X00000000000000000MC10", "01J5X00000000000000000MC20"}[index]
		if res, err := Open(verbReq(a, ulid, "mac-a"), id, "Mixed member.", OriginMain, "Work independently."); err != nil || res.Outcome != OutcomeConfirmed {
			t.Fatalf("open %s: %+v %v", id, res, err)
		}
		if res, err := SetArc(verbReq(a, ulid[:len(ulid)-1]+"1", "mac-a"), id, "mixed-cascade"); err != nil || res.Outcome != OutcomeConfirmed {
			t.Fatalf("set arc %s: %+v %v", id, res, err)
		}
	}
	if res, err := Claim(verbReq(a, "01J5X00000000000000000MC30", "mac-a"), "mixed-one", testBudget()); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("claim A: %+v %v", res, err)
	}
	if res, err := Claim(verbReq(b, "01J5X00000000000000000MC40", "mac-b"), "mixed-two", testBudget()); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("claim B: %+v %v", res, err)
	}
	park := verbReq(a, "01J5X00000000000000000MC50", "mac-a")
	if res, err := Park(park, "mixed-parked", "not a mover"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("park budgetless sibling: %+v %v", res, err)
	}
	pin := verbReq(a, "01J5X00000000000000000MC60", "mac-a")
	pin.Actor.Human = "wido"
	if res, err := SetPin(pin, "mixed-parked", "mac-z"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("pin unrelated sibling: %+v %v", res, err)
	}

	// Releasing A skips B's independent claim instead of refusing the
	// operation on a sibling A does not own.
	if res, err := ReleaseArc(verbReq(a, "01J5X00000000000000000MC70", "mac-a"), "mixed-one"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("mixed release: %+v %v", res, err)
	}
	tree, err := loadTree(a, mustGit(t, a, "rev-parse", "origin/main"))
	if err != nil || tree.Live["mixed-one"].State != StateQueued || tree.Live["mixed-two"].Claimed == nil || tree.Live["mixed-two"].Claimed.Machine != "mac-b" {
		t.Fatalf("release moved a foreign sibling: %+v %v", tree, err)
	}
	if res, err := Claim(verbReq(a, "01J5X00000000000000000MC80", "mac-a"), "mixed-one", testBudget()); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("reclaim A: %+v %v", res, err)
	}

	// Steal follows B's pair only. The independently claimed A member and
	// the pinned, parked, budgetless sibling cannot veto or move.
	steal := verbReq(a, "01J5X00000000000000000MC90", "mac-c")
	steal.Actor.Human = "wido"
	if res, err := Steal(steal, "mixed-two"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("mover-scoped steal: %+v %v", res, err)
	}
	tree, err = loadTree(a, mustGit(t, a, "rev-parse", "origin/main"))
	if err != nil || tree.Live["mixed-one"].Claimed.Machine != "mac-a" || tree.Live["mixed-two"].Claimed.Machine != "mac-c" || tree.Live["mixed-parked"].State != StateParked {
		t.Fatalf("steal crossed an independent-member boundary: %+v %v", tree, err)
	}

	// A human park may displace both independent pairs. Every member keeps
	// its actual marker, so the acknowledgment fold sees two pairs.
	humanPark := verbReq(a, "01J5X00000000000000000MCA0", "mac-h")
	humanPark.Actor.Human = "wido"
	if res, err := ParkArc(humanPark, "mixed-one", "whole planning group pause"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("multi-pair park: %+v %v", res, err)
	}
	tree, err = loadTree(a, mustGit(t, a, "rev-parse", "origin/main"))
	if err != nil {
		t.Fatal(err)
	}
	markers := map[string]bool{}
	for _, id := range []string{"mixed-one", "mixed-two"} {
		if tree.Live[id].Parked == nil || tree.Live[id].Parked.Displaced == "" {
			t.Fatalf("displaced pair missing on %s: %+v", id, tree.Live[id])
		}
		markers[tree.Live[id].Parked.Displaced] = true
	}
	if len(markers) != 2 {
		t.Fatalf("distinct claimant pairs collapsed into one displacement marker: %v", markers)
	}

	humanUnpark := verbReq(a, "01J5X00000000000000000MCB0", "mac-h")
	humanUnpark.Actor.Human = "wido"
	if res, err := UnparkArc(humanUnpark, "mixed-one"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("human unpark: %+v %v", res, err)
	}
	if res, err := Park(verbReq(a, "01J5X00000000000000000MCC0", "mac-a"), "mixed-parked", "remain parked"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("repark pinned sibling: %+v %v", res, err)
	}
	if res, err := Claim(verbReq(a, "01J5X00000000000000000MCD0", "mac-a"), "mixed-one", testBudget()); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("claim anchor: %+v %v", res, err)
	}
	// Asking through the parked member still sweeps the queued remainder.
	if res, err := ClaimArc(verbReq(a, "01J5X00000000000000000MCE0", "mac-a"), "mixed-parked", testBudget()); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("claim cascade through parked member: %+v %v", res, err)
	}
	tree, err = loadTree(a, mustGit(t, a, "rev-parse", "origin/main"))
	if err != nil || tree.Live["mixed-two"].State != StateClaimed || tree.Live["mixed-two"].Claimed.Machine != "mac-a" || tree.Live["mixed-parked"].State != StateParked {
		t.Fatalf("claim cascade did not sweep only the queued remainder: %+v %v", tree, err)
	}
}

func TestMixedArcJoinUsesOwnPairOrNewestAllParkedRecord(t *testing.T) {
	t.Run("all parked copies newest", func(t *testing.T) {
		_, root := oneClone(t)
		seedLedger(t, root)
		for index, id := range []string{"park-old", "park-new", "park-join"} {
			ulid := []string{"01J5X00000000000000000MJ00", "01J5X00000000000000000MJ10", "01J5X00000000000000000MJ20"}[index]
			if res, err := Open(verbReq(root, ulid, "mac-a"), id, "Park join.", OriginMain, "Wait."); err != nil || res.Outcome != OutcomeConfirmed {
				t.Fatalf("open %s: %+v %v", id, res, err)
			}
		}
		for index, id := range []string{"park-old", "park-new"} {
			ulid := []string{"01J5X00000000000000000MJ30", "01J5X00000000000000000MJ40"}[index]
			if res, err := SetArc(verbReq(root, ulid, "mac-a"), id, "park-destination"); err != nil || res.Outcome != OutcomeConfirmed {
				t.Fatalf("set arc %s: %+v %v", id, res, err)
			}
		}
		old := verbReq(root, "01J5X00000000000000000MJ50", "mac-a")
		old.Actor.Human = "wido"
		old.Now = old.Now.Add(-time.Hour)
		if res, err := Park(old, "park-old", "older word"); err != nil || res.Outcome != OutcomeConfirmed {
			t.Fatalf("old park: %+v %v", res, err)
		}
		newest := verbReq(root, "01J5X00000000000000000MJ60", "mac-a")
		newest.Actor.Human = "wido"
		if res, err := Park(newest, "park-new", "newest word"); err != nil || res.Outcome != OutcomeConfirmed {
			t.Fatalf("new park: %+v %v", res, err)
		}
		join := verbReq(root, "01J5X00000000000000000MJ70", "mac-a")
		join.Actor.Human = "wido"
		res, err := SetArc(join, "park-join", "park-destination")
		if err != nil || res.Outcome != OutcomeConfirmed {
			t.Fatalf("all-parked join: %+v %v", res, err)
		}
		tree, err := loadTree(root, res.Tip)
		joined := tree.Live["park-join"]
		if err != nil || joined.State != StateParked || joined.Parked == nil || joined.Parked.Because != "newest word" || joined.Parked.Displaced != "" {
			t.Fatalf("join did not copy the newest record without displacement: %+v %v", joined, err)
		}
	})

	t.Run("reconcile matches own-pair verb", func(t *testing.T) {
		_, a, b := twoClones(t)
		seedLedger(t, a)
		for index, id := range []string{"join-own", "join-foreign", "join-candidate"} {
			ulid := []string{"01J5X00000000000000000MR00", "01J5X00000000000000000MR10", "01J5X00000000000000000MR20"}[index]
			if res, err := Open(verbReq(a, ulid, "mac-a"), id, "Reconcile join.", OriginMain, "Join."); err != nil || res.Outcome != OutcomeConfirmed {
				t.Fatalf("open %s: %+v %v", id, res, err)
			}
		}
		if res, err := SetBudget(verbReq(a, "01J5X00000000000000000MR25", "mac-a"), "join-candidate", testBudget()); err != nil || res.Outcome != OutcomeConfirmed {
			t.Fatalf("budget candidate: %+v %v", res, err)
		}
		for index, id := range []string{"join-own", "join-foreign"} {
			ulid := []string{"01J5X00000000000000000MR30", "01J5X00000000000000000MR40"}[index]
			if res, err := SetArc(verbReq(a, ulid, "mac-a"), id, "join-destination"); err != nil || res.Outcome != OutcomeConfirmed {
				t.Fatalf("set arc %s: %+v %v", id, res, err)
			}
		}
		if res, err := Claim(verbReq(a, "01J5X00000000000000000MR50", "mac-a"), "join-own", testBudget()); err != nil || res.Outcome != OutcomeConfirmed {
			t.Fatalf("claim own: %+v %v", res, err)
		}
		if res, err := Claim(verbReq(b, "01J5X00000000000000000MR60", "mac-b"), "join-foreign", testBudget()); err != nil || res.Outcome != OutcomeConfirmed {
			t.Fatalf("claim foreign: %+v %v", res, err)
		}
		advanced, err := FetchAdvance(endpointFor(a))
		if err != nil {
			t.Fatal(err)
		}
		handTree, err := loadTree(a, advanced.Tip)
		if err != nil {
			t.Fatal(err)
		}
		hand := verbReq(a, "01J5X00000000000000000MR70", "mac-a")
		hand.Actor.Human = "wido"
		if _, err := applyRow(handTree, hand, MappedVerb{Verb: "set-arc", Id: "join-candidate", Arc: "join-destination", BaseState: StateQueued}, newReplaySession()); err != nil {
			t.Fatalf("reconcile application: %v", err)
		}
		verbResult, err := SetArc(verbReq(a, "01J5X00000000000000000MR80", "mac-a"), "join-candidate", "join-destination")
		if err != nil || verbResult.Outcome != OutcomeConfirmed {
			t.Fatalf("verb join: %+v %v", verbResult, err)
		}
		verbTree, err := loadTree(a, verbResult.Tip)
		handJoined, verbJoined := handTree.Live["join-candidate"], verbTree.Live["join-candidate"]
		if err != nil || handJoined.State != verbJoined.State || handJoined.Claimed == nil || verbJoined.Claimed == nil ||
			handJoined.Claimed.Machine != verbJoined.Claimed.Machine || handJoined.Claimed.Lineage != verbJoined.Claimed.Lineage {
			t.Fatalf("reconcile and verb diverged: hand=%+v verb=%+v err=%v", handJoined, verbJoined, err)
		}
	})
}

func TestClaimedArcToForeignClaimedArcLandsQueuedOnBothSurfaces(t *testing.T) {
	_, a, b := twoClones(t)
	seedLedger(t, a)
	arcBed(t, a, "trade-src", "ts", "TS")
	// A second claimed arc under ANOTHER machine (the quota admits
	// one claim per machine, so the destination claimant differs).
	for i, id := range []string{"td-one", "td-two"} {
		ulid := fmt.Sprintf("01J5X00000000000000000TD%d0", i)
		if res, err := Open(verbReq(b, ulid, "mac-b"), id, "Dest "+id, "main", "Go."); err != nil || res.Outcome != OutcomeConfirmed {
			t.Fatalf("open %s: %+v %v", id, res, err)
		}
		if res, err := SetArc(verbReq(b, ulid[:len(ulid)-1]+"1", "mac-b"), id, "trade-dst"); err != nil || res.Outcome != OutcomeConfirmed {
			t.Fatalf("set-arc %s: %+v %v", id, res, err)
		}
	}
	claimRes, err := ClaimArc(verbReq(b, "01J5X00000000000000000TD90", "mac-b"), "td-one", testBudget())
	if err != nil || claimRes.Outcome != OutcomeConfirmed {
		t.Fatalf("claim dest arc: %+v %v", claimRes, err)
	}
	// A caller that owns the destination claim but would displace the source
	// claimant still hits the two-pair refusal on both executable surfaces.
	destinationOwner := verbReq(b, "01J5X00000000000000000TD94", "mac-b")
	destinationOwner.Actor.Human = "wido"
	trade, tradeErr := SetArc(destinationOwner, "ts-one", "trade-dst")
	if tradeErr != nil || trade.Outcome != OutcomeRejected || !strings.Contains(trade.Detail, "release it first") {
		t.Fatalf("two-pair verb trade did not refuse: %+v %v", trade, tradeErr)
	}
	tradeTree, err := loadTree(b, claimRes.Tip)
	if err != nil {
		t.Fatal(err)
	}
	if _, handErr := applyRow(tradeTree, destinationOwner, MappedVerb{
		Verb: "set-arc", Id: "ts-one", Arc: "trade-dst", BaseArc: "trade-src", BaseState: StateClaimed,
	}, newReplaySession()); handErr == nil || !strings.Contains(handErr.Error(), "release it first") {
		t.Fatalf("two-pair hand trade did not refuse: %v", handErr)
	}
	// With no caller-owned claim in the destination, the mixed-arc rule
	// releases the source member and lands it queued. A foreign destination
	// claim is neither inherited nor displaced.
	humanMove := verbReq(a, "01J5X00000000000000000TD95", "mac-a")
	humanMove.Actor.Human = "wido"
	res, err := SetArc(humanMove, "ts-two", "trade-dst")
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("foreign claimed destination did not take the queued fallback: %+v %v", res, err)
	}
	verbTree, err := loadTree(a, res.Tip)
	if err != nil || verbTree.Live["ts-two"].State != StateQueued || verbTree.Live["ts-two"].Claimed != nil {
		t.Fatalf("verb fallback inherited a foreign claim: %+v %v", verbTree.Live["ts-two"], err)
	}
	// Reconcile uses the identical queued fallback on the pre-move tip.
	tree, err := loadTree(b, claimRes.Tip)
	if err != nil {
		t.Fatal(err)
	}
	hand := verbReq(b, "01J5X00000000000000000TD97", "mac-a")
	hand.Actor.Human = "wido"
	_, handErr := applyRow(tree, hand, MappedVerb{
		Verb: "set-arc", Id: "ts-two", Arc: "trade-dst",
		BaseArc: "trade-src", BaseState: StateClaimed,
	}, newReplaySession())
	if handErr != nil || tree.Live["ts-two"].State != StateQueued || tree.Live["ts-two"].Claimed != nil {
		t.Fatalf("hand replay diverged from queued fallback: %+v %v", tree.Live["ts-two"], handErr)
	}
}

func TestParkThenDetachComposesInOneHandSession(t *testing.T) {
	_, a, _ := twoClones(t)
	seedLedger(t, a)
	arcBed(t, a, "pd-arc", "pd", "PD")
	p, err := Project(endpointFor(a), true, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	tree, err := loadTree(a, p.Tip)
	if err != nil {
		t.Fatal(err)
	}
	hand := verbReq(a, "01J5X00000000000000000PD90", "mac-a")
	hand.Actor.Human = "wido"
	session := newReplaySession()
	// The park moves the state; the detach that rides the SAME hand
	// edit was mapped against the PRE-park state — the session's own
	// move must compose, not conflict.
	if _, err := applyRow(tree, hand, MappedVerb{Verb: "park", Id: "pd-one", Because: "pausing", BaseState: StateClaimed}, session); err != nil {
		t.Fatalf("park: %v", err)
	}
	if _, err := applyRow(tree, hand, MappedVerb{Verb: "detach", Id: "pd-one", BaseArc: "pd-arc", BaseState: StateClaimed}, session); err != nil {
		t.Fatalf("the same session's park+detach composes: %v", err)
	}
	f := tree.Live["pd-one"]
	if f.Arc != "" || f.State != StateParked {
		t.Fatalf("the composition landed both effects: arc=%q state=%s", f.Arc, f.State)
	}
}
