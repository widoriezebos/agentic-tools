package goal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
)

func attorneyBed(t *testing.T) string {
	t.Helper()
	root := riskLocalRoot(t, "attorney-bed")
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\nmetasystem.governance.correlation-policy=A\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func attorneyReq(root string, n int, machine string) VerbRequest {
	return obligationAuthorityVerbReq(root, fmt.Sprintf("01J5X00000000000000000PA%02d", n), machine)
}

func expectRefusal(t *testing.T, label string, res PublishResult, err error, needle string) {
	t.Helper()
	if err != nil {
		if strings.Contains(err.Error(), needle) {
			return
		}
		t.Fatalf("%s: error %v lacks %q", label, err, needle)
	}
	if res.Outcome == OutcomeConfirmed || !strings.Contains(res.Detail, needle) {
		t.Fatalf("%s: %+v lacks %q", label, res, needle)
	}
}

// A person records a power of attorney with their own proof; the entry is
// scoped, expires within seven days, coexists with others, round-trips
// through the root record, and revokes.
func TestGrantRecordsAPowerOfAttorneyWithinItsBounds(t *testing.T) {
	root := attorneyBed(t)
	human := attorneyReq(root, 0, "mac-a")
	human.Actor.Human = "Wido"
	proof := testHumanAuthority(t, root, human.Now)
	expires := human.Now.AddDate(0, 0, 5).Format("2006-01-02")
	res, err := Grant(human, proof, []uint8{1}, []string{"set-budget", "approve"}, expires)
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("grant: %+v %v", res, err)
	}
	tree, err := loadTree(root, res.Tip)
	if err != nil {
		t.Fatal(err)
	}
	if len(tree.Root.PowerOfAttorney) != 1 {
		t.Fatalf("one entry: %+v", tree.Root.PowerOfAttorney)
	}
	entry := tree.Root.PowerOfAttorney[0]
	if entry.ID != human.opid() || entry.By != "human:Wido" || renderTiers(entry.Tiers) != "1" || strings.Join(entry.Verbs, ",") != "approve,set-budget" || entry.Expires != expires || entry.Revoked != "" {
		t.Fatalf("the entry carries the grant: %+v", entry)
	}
	if last := tree.Root.History[len(tree.Root.History)-1]; last.Verb != "grant" || last.Actor != "human:Wido" || !strings.Contains(last.Reason, "expires="+expires) {
		t.Fatalf("the grant is on the root history: %+v", last)
	}
	rendered := RenderRoot(tree.Root)
	if !strings.Contains(string(rendered), "\nPowerOfAttorney:\n- "+entry.ID+" by=human:Wido tiers=1 verbs=approve,set-budget since=") {
		t.Fatalf("the root renders its section: %s", rendered)
	}
	parsed, problems := ParseRoot(rendered)
	if len(problems) != 0 || string(RenderRoot(parsed)) != string(rendered) {
		t.Fatalf("the root is a render fixed point: %v", problems)
	}
	if live, why := entry.LiveAt(human.Now.AddDate(0, 0, 5)); !live {
		t.Fatalf("the expiry day still counts: %s", why)
	}
	if live, _ := entry.LiveAt(human.Now.AddDate(0, 0, 6)); live {
		t.Fatal("the day after the expiry does not count")
	}

	// A second entry coexists.
	second := attorneyReq(root, 1, "mac-a")
	second.Actor.Human = "Wido"
	secondRes, err := Grant(second, proof, []uint8{1}, []string{"approve"}, expires)
	if err != nil || secondRes.Outcome != OutcomeConfirmed {
		t.Fatalf("second grant: %+v %v", secondRes, err)
	}
	if tree, err = loadTree(root, secondRes.Tip); err != nil {
		t.Fatal(err)
	}
	if len(tree.Root.PowerOfAttorney) != 2 {
		t.Fatalf("two entries coexist: %+v", tree.Root.PowerOfAttorney)
	}

	// The bounds: seven days, tier 1, the two verbs, a human, their own proof.
	bad := attorneyReq(root, 2, "mac-a")
	bad.Actor.Human = "Wido"
	res, err = Grant(bad, proof, []uint8{1}, []string{"approve"}, human.Now.AddDate(0, 0, 7).Format("2006-01-02"))
	expectRefusal(t, "eight calendar days", res, err, "no entry lives longer than 7 days")
	if res, err := Grant(withUlid(bad, 6), proof, []uint8{1}, []string{"approve"}, human.Now.AddDate(0, 0, 6).Format("2006-01-02")); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("seven days, the expiry day included, is the bound: %+v %v", res, err)
	}
	res, err = Grant(bad, proof, []uint8{1, 2}, []string{"approve"}, expires)
	expectRefusal(t, "tier 2", res, err, "tier 1 in this build")
	res, err = Grant(bad, proof, []uint8{1}, []string{"done"}, expires)
	expectRefusal(t, "verb done", res, err, "covers approve,set-budget only")
	res, err = Grant(attorneyReq(root, 3, "mac-a"), proof, []uint8{1}, []string{"approve"}, expires)
	expectRefusal(t, "agent", res, err, "human-only")
	// A proof that is not the human's own observed authority cannot grant:
	// the relayed-word horizon has passed, so an unobserved relay stands in.
	relayed := humanauthority.Proof{Outcome: humanauthority.OutcomeTemporary, TemporaryHumanWord: "Wido said so", ReviewBy: expires}
	res, err = Grant(bad, &relayed, []uint8{1}, []string{"approve"}, expires)
	expectRefusal(t, "relayed word", res, err, "human")

	// Revoke closes the entry; a second revoke has nothing to do.
	revoke := attorneyReq(root, 4, "mac-a")
	revoke.Actor.Human = "Wido"
	res, err = Revoke(revoke, proof, entry.ID)
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("revoke: %+v %v", res, err)
	}
	if tree, err = loadTree(root, res.Tip); err != nil {
		t.Fatal(err)
	}
	revoked, _ := rootAttorney(tree.Root, entry.ID)
	if revoked.Revoked == "" || revoked.RevokedBy != "human:Wido" {
		t.Fatalf("the entry is revoked: %+v", revoked)
	}
	if live, why := revoked.LiveAt(human.Now); live || !strings.Contains(why, "revoked") {
		t.Fatalf("a revoked entry is not live: %v %s", live, why)
	}
	again := attorneyReq(root, 5, "mac-a")
	again.Actor.Human = "Wido"
	if res, err := Revoke(again, proof, entry.ID); err != nil || res.Outcome == OutcomeConfirmed {
		t.Fatalf("a second revoke has nothing to do: %+v %v", res, err)
	}
}

// A seat approves and set-budgets a tier-1 goal under a live entry as its
// own act, within the box, and is refused outside the entry.
func TestApproveAndSetBudgetUnderPowerOfAttorney(t *testing.T) {
	root := attorneyBed(t)
	human := attorneyReq(root, 10, "mac-a")
	human.Actor.Human = "Wido"
	proof := testHumanAuthority(t, root, human.Now)
	expires := human.Now.AddDate(0, 0, 5).Format("2006-01-02")
	res, err := Grant(human, proof, []uint8{1}, []string{"approve", "set-budget"}, expires)
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("grant: %+v %v", res, err)
	}
	entry, err := ResolveAttorney(root, human.opid(), "approve", human.Now)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	low := RiskRecord{Severity: 1, Novelty: 1, Exposure: 1, Accumulation: 1, Basis: "routine"}
	mid := RiskRecord{Severity: 2, Novelty: 1, Exposure: 1, Accumulation: 1, Basis: "moderate"}
	small := Budget{ElapsedLimit: "1h", AttemptLimit: 3, ReservedJobMinutesLimit: 360, ActiveJobLimit: 1, ReviewRoundLimit: 0}
	for i, open := range []struct {
		id   string
		risk RiskRecord
	}{{"small", low}, {"medium", mid}, {"proven-first", low}} {
		if res, err := OpenRisked(attorneyReq(root, 11+i, "mac-a"), open.id, "Work "+open.id+".", OriginHuman, "Do it.", "", open.risk, 0, "", &small, nil); err != nil || res.Outcome != OutcomeConfirmed {
			t.Fatalf("open %s: %+v %v", open.id, res, err)
		}
	}

	seat := attorneyReq(root, 20, "mac-a")
	seat.Attorney = &entry
	res, err = Approve(seat, []string{"small"}, nil, nil)
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("approve under attorney: %+v %v", res, err)
	}
	tree, err := loadTree(root, res.Tip)
	if err != nil {
		t.Fatal(err)
	}
	f := tree.Live["small"]
	if f.State != StateApproved || f.Approved == nil || f.Approved.Authority != ApprovalAuthorityAttorney || f.Approved.By != "mac-a+lin-1" || f.Approved.ReviewBy != "" {
		t.Fatalf("the approval is the seat's act under attorney: %+v", f.Approved)
	}
	if last := f.History[len(f.History)-1]; last.Verb != "approve" || last.Actor != "mac-a+lin-1" || last.AuthorityOutcome != AuthorityOutcomePowerOfAttorney || last.AuthorityRuling != entry.ID {
		t.Fatalf("the history line names the entry: %+v", last)
	}
	if expired, _ := f.ApprovalExpired(ApprovalHorizon{Now: human.Now.AddDate(0, 0, 30)}); expired {
		t.Fatal("an attorney approval stands after the entry expires")
	}
	// The same approval again has nothing to do; --by beside the entry is refused.
	if res, err := Approve(withUlid(seat, 21), []string{"small"}, nil, nil); err != nil || res.Outcome == OutcomeConfirmed {
		t.Fatalf("a repeated attorney approval has nothing to do: %+v %v", res, err)
	}
	mixed := withUlid(seat, 22)
	mixed.Actor.Human = "Wido"
	res, err = Approve(mixed, []string{"small"}, nil, nil)
	expectRefusal(t, "--by beside --under", res, err, "seat's own")

	// The claim, then set-budget within the box; over the box refuses.
	if res, err := Claim(attorneyReq(root, 23, "mac-a"), "small"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("claim: %+v %v", res, err)
	}
	within := Budget{ElapsedLimit: "1h", AttemptLimit: 2, ReservedJobMinutesLimit: 120, ActiveJobLimit: 1, ReviewRoundLimit: 0}
	res, err = SetBudgetApproved(withUlid(seat, 24), "small", within, nil)
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("set-budget under attorney: %+v %v", res, err)
	}
	if tree, err = loadTree(root, res.Tip); err != nil {
		t.Fatal(err)
	}
	f = tree.Live["small"]
	if f.Approved.Authority != ApprovalAuthorityAttorney || *f.Budget != within || f.History[len(f.History)-1].AuthorityRuling != entry.ID {
		t.Fatalf("the set-budget is the seat's act under attorney: %+v %+v", f.Approved, f.Budget)
	}
	over := Budget{ElapsedLimit: "8h", AttemptLimit: 2, ReservedJobMinutesLimit: 120, ActiveJobLimit: 1, ReviewRoundLimit: 0}
	res, err = SetBudgetApproved(withUlid(seat, 25), "small", over, nil)
	expectRefusal(t, "over the box", res, err, "GOAL_NORM_REFUSED")

	// Outside the entry: a tier-2 goal, a verb the entry lacks, an expired
	// or revoked entry, and a standing proven approval.
	res, err = Approve(withUlid(seat, 26), []string{"medium"}, nil, nil)
	expectRefusal(t, "tier 2", res, err, "covers tier 1 only")
	narrow := attorneyReq(root, 27, "mac-a")
	narrow.Actor.Human = "Wido"
	if res, err := Grant(narrow, proof, []uint8{1}, []string{"approve"}, expires); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("narrow grant: %+v %v", res, err)
	}
	narrowEntry, err := ResolveAttorney(root, narrow.opid(), "approve", human.Now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveAttorney(root, narrow.opid(), "set-budget", human.Now); err == nil || !strings.Contains(err.Error(), "covers approve, not set-budget") {
		t.Fatalf("the entry's verbs bound the act: %v", err)
	}
	narrowSeat := withUlid(seat, 28)
	narrowSeat.Attorney = &narrowEntry
	res, err = SetBudgetApproved(narrowSeat, "small", within, nil)
	expectRefusal(t, "verb outside the entry", res, err, "covers approve, not set-budget")
	late := withUlid(seat, 29)
	late.Now = human.Now.AddDate(0, 0, 6)
	res, err = Approve(late, []string{"medium"}, nil, nil)
	expectRefusal(t, "expired", res, err, "expired "+expires)
	if _, err := ResolveAttorney(root, entry.ID, "approve", late.Now); err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("the command edge refuses an expired entry: %v", err)
	}
	proven := attorneyReq(root, 30, "mac-a")
	proven.Actor.Human = "Wido"
	if res, err := Approve(proven, []string{"proven-first"}, &small, proof); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("proven approval: %+v %v", res, err)
	}
	res, err = Approve(withUlid(seat, 31), []string{"proven-first"}, nil, nil)
	expectRefusal(t, "proven approval stands", res, err, "does not rewrite it")
	revoke := attorneyReq(root, 32, "mac-a")
	revoke.Actor.Human = "Wido"
	if res, err := Revoke(revoke, proof, entry.ID); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("revoke: %+v %v", res, err)
	}
	res, err = Approve(withUlid(seat, 33), []string{"medium"}, nil, nil)
	expectRefusal(t, "revoked", res, err, "revoked")
}

func withUlid(r VerbRequest, n int) VerbRequest {
	r.Ulid = fmt.Sprintf("01J5X00000000000000000PA%02d", n)
	r.Now = r.Now.Add(time.Duration(n) * time.Second)
	return r
}

// An entry outside R-95-m1e's bounds stays readable on a landed ledger but
// is never honoured, so a hand edit of the section grants nothing.
func TestAPowerOfAttorneyOutsideItsBoundsIsNeverLive(t *testing.T) {
	now := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
	sound := PowerOfAttorneyEntry{ID: Opid("01J5X00000000000000000PB00", "mac-a", "lin-1"), By: "human:Wido", Tiers: []uint8{1},
		Verbs: []string{"approve"}, Since: "2026-09-12T09:00:00Z", Expires: "2026-09-18"}
	if live, why := sound.LiveAt(now); !live {
		t.Fatalf("a sound entry is live: %s", why)
	}
	for _, bent := range []struct {
		label string
		edit  func(e *PowerOfAttorneyEntry)
	}{
		{"tier 3", func(e *PowerOfAttorneyEntry) { e.Tiers = []uint8{1, 3} }},
		{"verb done", func(e *PowerOfAttorneyEntry) { e.Verbs = []string{"approve", "done"} }},
		{"a year", func(e *PowerOfAttorneyEntry) { e.Expires = "2027-09-12" }},
		{"eight days", func(e *PowerOfAttorneyEntry) { e.Expires = "2026-09-19" }},
		{"revoked before the grant", func(e *PowerOfAttorneyEntry) { e.Revoked, e.RevokedBy = "2026-09-11T09:00:00Z", "human:Wido" }},
	} {
		entry := sound
		entry.Tiers = append([]uint8(nil), sound.Tiers...)
		entry.Verbs = append([]string(nil), sound.Verbs...)
		bent.edit(&entry)
		if live, why := entry.LiveAt(now); live || !strings.Contains(why, "outside its bounds") && !strings.Contains(why, "precedes") {
			t.Fatalf("%s: live=%v why=%q", bent.label, live, why)
		}
		rendered := RenderRoot(&RootRecord{Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "1", SyncMode: SyncLocal, Revision: 1, PowerOfAttorney: []PowerOfAttorneyEntry{entry}})
		if _, problems := ParseRoot(rendered); len(problems) != 0 {
			t.Fatalf("%s: a bent entry still parses (the ledger stays readable): %v", bent.label, problems)
		}
	}
}

// An act under attorney binds only where no approval stands or where the
// standing one is itself an attorney act: a person's proven, relayed or
// channel approval is never rewritten by approve or set-budget.
func TestAnAttorneyActNeverRewritesAPersonsApproval(t *testing.T) {
	for _, authority := range []string{ApprovalAuthorityProven, ApprovalAuthorityRelayed, ApprovalAuthorityChannel} {
		f := &GoalFile{Id: "held", Approved: &ApprovalRecord{Authority: authority}}
		if err := attorneyMayRebind(f, "entry"); err == nil || !strings.Contains(err.Error(), authority) {
			t.Fatalf("%s approval is not rewritten: %v", authority, err)
		}
	}
	if err := attorneyMayRebind(&GoalFile{Id: "fresh"}, "entry"); err != nil {
		t.Fatalf("no standing approval binds: %v", err)
	}
	if err := attorneyMayRebind(&GoalFile{Id: "own", Approved: &ApprovalRecord{Authority: ApprovalAuthorityAttorney}}, "entry"); err != nil {
		t.Fatalf("an attorney approval rebinds: %v", err)
	}
	// Through the verb: a person's proven approval on a claimed goal refuses
	// an attorney set-budget, not only an attorney approve.
	root := attorneyBed(t)
	human := attorneyReq(root, 40, "mac-a")
	human.Actor.Human = "Wido"
	proof := testHumanAuthority(t, root, human.Now)
	small := Budget{ElapsedLimit: "1h", AttemptLimit: 3, ReservedJobMinutesLimit: 360, ActiveJobLimit: 1, ReviewRoundLimit: 0}
	if res, err := Grant(human, proof, []uint8{1}, []string{"approve", "set-budget"}, human.Now.AddDate(0, 0, 3).Format("2006-01-02")); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("grant: %+v %v", res, err)
	}
	entry, err := ResolveAttorney(root, human.opid(), "set-budget", human.Now)
	if err != nil {
		t.Fatal(err)
	}
	low := RiskRecord{Severity: 1, Novelty: 1, Exposure: 1, Accumulation: 1, Basis: "routine"}
	if res, err := OpenRisked(attorneyReq(root, 41, "mac-a"), "kept", "Kept by the person.", OriginHuman, "Do it.", "", low, 0, "", &small, nil); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", res, err)
	}
	if res, err := claimApprovedForTest(t, attorneyReq(root, 42, "mac-a"), "kept", small); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("claim: %+v %v", res, err)
	}
	seat := attorneyReq(root, 43, "mac-a")
	seat.Attorney = &entry
	res, err := SetBudgetApproved(seat, "kept", Budget{ElapsedLimit: "1h", AttemptLimit: 2, ReservedJobMinutesLimit: 120, ActiveJobLimit: 1, ReviewRoundLimit: 0}, nil)
	expectRefusal(t, "set-budget over a proven approval", res, err, "does not rewrite it")
}
