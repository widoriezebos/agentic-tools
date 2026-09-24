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

func attorneyBed(t *testing.T) Endpoint {
	t.Helper()
	endpoint := riskLocalEndpoint(t)
	if err := os.WriteFile(filepath.Join(endpoint.Root, "metasystem.conf"), []byte("metasystem.runtimes=fake\nmetasystem.governance.correlation-policy=A\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return endpoint
}

func attorneyReq(endpoint Endpoint, n int, machine string) VerbRequest {
	return verbReqFor(endpoint, fmt.Sprintf("01J5X00000000000000000PA%02d", n), machine)
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
	t.Parallel()
	endpoint := attorneyBed(t)
	root := endpoint.Root
	human := attorneyReq(endpoint, 0, "mac-a")
	human.Actor.Human = "Wido"
	proof := testHumanAuthority(t, root, human.Now)
	expires := human.Now.AddDate(0, 0, 5).Format("2006-01-02")
	res, err := Grant(human, proof, []uint8{1}, []string{"set-budget", "approve"}, expires)
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("grant: %+v %v", res, err)
	}
	tree, err := loadTreeFor(endpoint, res.Tip)
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
	second := attorneyReq(endpoint, 1, "mac-a")
	second.Actor.Human = "Wido"
	secondRes, err := Grant(second, proof, []uint8{1}, []string{"approve"}, expires)
	if err != nil || secondRes.Outcome != OutcomeConfirmed {
		t.Fatalf("second grant: %+v %v", secondRes, err)
	}
	if tree, err = loadTreeFor(endpoint, secondRes.Tip); err != nil {
		t.Fatal(err)
	}
	if len(tree.Root.PowerOfAttorney) != 2 {
		t.Fatalf("two entries coexist: %+v", tree.Root.PowerOfAttorney)
	}

	// The bounds: seven days, tier 1, the two verbs, a human, their own proof.
	bad := attorneyReq(endpoint, 2, "mac-a")
	bad.Actor.Human = "Wido"
	res, err = Grant(bad, proof, []uint8{1}, []string{"approve"}, human.Now.AddDate(0, 0, 7).Format("2006-01-02"))
	expectRefusal(t, "eight calendar days", res, err, "no entry lives longer than 7 days")
	if res, err := Grant(withUlid(bad, 6), proof, []uint8{1}, []string{"approve"}, human.Now.AddDate(0, 0, 6).Format("2006-01-02")); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("seven days, the expiry day included, is the bound: %+v %v", res, err)
	}
	res, err = Grant(bad, proof, []uint8{1, 3}, []string{"approve"}, expires)
	expectRefusal(t, "tier 3", res, err, "tiers 1 and 2 only")
	if res, err := Grant(withUlid(bad, 7), proof, []uint8{1, 2}, []string{"approve"}, expires); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("tiers 1 and 2 are the delegable tiers: %+v %v", res, err)
	}
	res, err = Grant(bad, proof, []uint8{1}, []string{"done"}, expires)
	expectRefusal(t, "verb done", res, err, "covers approve,set-budget,unpark only")
	res, err = Grant(attorneyReq(endpoint, 3, "mac-a"), proof, []uint8{1}, []string{"approve"}, expires)
	expectRefusal(t, "agent", res, err, "human-only")
	// A proof that is not the human's own observed authority cannot grant:
	// the relayed-word horizon has passed, so an unobserved relay stands in.
	relayed := humanauthority.Proof{Outcome: humanauthority.OutcomeTemporary, TemporaryHumanWord: "Wido said so", ReviewBy: expires}
	res, err = Grant(bad, &relayed, []uint8{1}, []string{"approve"}, expires)
	expectRefusal(t, "relayed word", res, err, "human")

	// Revoke closes the entry; a second revoke has nothing to do.
	revoke := attorneyReq(endpoint, 4, "mac-a")
	revoke.Actor.Human = "Wido"
	res, err = Revoke(revoke, proof, entry.ID)
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("revoke: %+v %v", res, err)
	}
	if tree, err = loadTreeFor(endpoint, res.Tip); err != nil {
		t.Fatal(err)
	}
	revoked, _ := rootAttorney(tree.Root, entry.ID)
	if revoked.Revoked == "" || revoked.RevokedBy != "human:Wido" {
		t.Fatalf("the entry is revoked: %+v", revoked)
	}
	if live, why := revoked.LiveAt(human.Now); live || !strings.Contains(why, "revoked") {
		t.Fatalf("a revoked entry is not live: %v %s", live, why)
	}
	again := attorneyReq(endpoint, 5, "mac-a")
	again.Actor.Human = "Wido"
	if res, err := Revoke(again, proof, entry.ID); err != nil || res.Outcome == OutcomeConfirmed {
		t.Fatalf("a second revoke has nothing to do: %+v %v", res, err)
	}
}

// A seat approves and set-budgets a tier-1 goal under a live entry as its
// own act, within the box, and is refused outside the entry.
func TestApproveAndSetBudgetUnderPowerOfAttorney(t *testing.T) {
	t.Parallel()
	endpoint := attorneyBed(t)
	root := endpoint.Root
	human := attorneyReq(endpoint, 10, "mac-a")
	human.Actor.Human = "Wido"
	proof := testHumanAuthority(t, root, human.Now)
	expires := human.Now.AddDate(0, 0, 5).Format("2006-01-02")
	res, err := Grant(human, proof, []uint8{1}, []string{"approve", "set-budget"}, expires)
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("grant: %+v %v", res, err)
	}
	entry, err := resolveAttorneyForEndpoint(endpoint, human.opid(), "approve", human.Now)
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
		if res, err := OpenRisked(attorneyReq(endpoint, 11+i, "mac-a"), open.id, "Work "+open.id+".", OriginHuman, "Do it.", "", open.risk, 0, "", &small, nil); err != nil || res.Outcome != OutcomeConfirmed {
			t.Fatalf("open %s: %+v %v", open.id, res, err)
		}
	}

	seat := attorneyReq(endpoint, 20, "mac-a")
	seat.Attorney = &entry
	res, err = Approve(seat, []string{"small"}, nil, nil)
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("approve under attorney: %+v %v", res, err)
	}
	tree, err := loadTreeFor(endpoint, res.Tip)
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
	holder := attorneyReq(endpoint, 23, "mac-b")
	holder.ClaimEpoch = 6
	if res, err := Claim(holder, "small"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("claim: %+v %v", res, err)
	}
	within := Budget{ElapsedLimit: "1h", AttemptLimit: 2, ReservedJobMinutesLimit: 120, ActiveJobLimit: 1, ReviewRoundLimit: 0}
	seat.ClaimEpoch = 2
	res, err = SetBudgetApproved(withUlid(seat, 24), "small", within, nil)
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("set-budget under attorney: %+v %v", res, err)
	}
	if tree, err = loadTreeFor(endpoint, res.Tip); err != nil {
		t.Fatal(err)
	}
	f = tree.Live["small"]
	if f.Approved.Authority != ApprovalAuthorityAttorney || *f.Budget != within || f.History[len(f.History)-1].AuthorityRuling != entry.ID || f.StopCapability == nil || f.StopCapability.ClaimEpoch != 6 {
		t.Fatalf("the set-budget is the seat's act under attorney: %+v %+v", f.Approved, f.Budget)
	}
	over := Budget{ElapsedLimit: "8h", AttemptLimit: 2, ReservedJobMinutesLimit: 120, ActiveJobLimit: 1, ReviewRoundLimit: 0}
	res, err = SetBudgetApproved(withUlid(seat, 25), "small", over, nil)
	expectRefusal(t, "over the box", res, err, "GOAL_NORM_REFUSED")

	// Outside the entry: a tier-2 goal, a verb the entry lacks, an expired
	// or revoked entry, and a standing proven approval.
	res, err = Approve(withUlid(seat, 26), []string{"medium"}, nil, nil)
	expectRefusal(t, "tier 2", res, err, "covers tier 1 only")
	narrow := attorneyReq(endpoint, 27, "mac-a")
	narrow.Actor.Human = "Wido"
	if res, err := Grant(narrow, proof, []uint8{1}, []string{"approve"}, expires); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("narrow grant: %+v %v", res, err)
	}
	narrowEntry, err := resolveAttorneyForEndpoint(endpoint, narrow.opid(), "approve", human.Now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := resolveAttorneyForEndpoint(endpoint, narrow.opid(), "set-budget", human.Now); err == nil || !strings.Contains(err.Error(), "covers approve, not set-budget") {
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
	if _, err := resolveAttorneyForEndpoint(endpoint, entry.ID, "approve", late.Now); err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("the command edge refuses an expired entry: %v", err)
	}
	proven := attorneyReq(endpoint, 30, "mac-a")
	proven.Actor.Human = "Wido"
	if res, err := Approve(proven, []string{"proven-first"}, &small, proof); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("proven approval: %+v %v", res, err)
	}
	res, err = Approve(withUlid(seat, 31), []string{"proven-first"}, nil, nil)
	expectRefusal(t, "proven approval stands", res, err, "does not rewrite it")
	revoke := attorneyReq(endpoint, 32, "mac-a")
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
	t.Parallel()
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
		{"tier 3", func(e *PowerOfAttorneyEntry) { e.Tiers = []uint8{2, 3} }},
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
	t.Parallel()
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
	endpoint := attorneyBed(t)
	root := endpoint.Root
	human := attorneyReq(endpoint, 40, "mac-a")
	human.Actor.Human = "Wido"
	proof := testHumanAuthority(t, root, human.Now)
	small := Budget{ElapsedLimit: "1h", AttemptLimit: 3, ReservedJobMinutesLimit: 360, ActiveJobLimit: 1, ReviewRoundLimit: 0}
	if res, err := Grant(human, proof, []uint8{1}, []string{"approve", "set-budget"}, human.Now.AddDate(0, 0, 3).Format("2006-01-02")); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("grant: %+v %v", res, err)
	}
	entry, err := resolveAttorneyForEndpoint(endpoint, human.opid(), "set-budget", human.Now)
	if err != nil {
		t.Fatal(err)
	}
	low := RiskRecord{Severity: 1, Novelty: 1, Exposure: 1, Accumulation: 1, Basis: "routine"}
	if res, err := OpenRisked(attorneyReq(endpoint, 41, "mac-a"), "kept", "Kept by the person.", OriginHuman, "Do it.", "", low, 0, "", &small, nil); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", res, err)
	}
	if res, err := claimApprovedForTest(t, attorneyReq(endpoint, 42, "mac-a"), "kept", small); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("claim: %+v %v", res, err)
	}
	seat := attorneyReq(endpoint, 43, "mac-a")
	seat.Attorney = &entry
	res, err := SetBudgetApproved(seat, "kept", Budget{ElapsedLimit: "1h", AttemptLimit: 2, ReservedJobMinutesLimit: 120, ActiveJobLimit: 1, ReviewRoundLimit: 0}, nil)
	expectRefusal(t, "set-budget over a proven approval", res, err, "does not rewrite it")
}

// R-105-m1e: under an entry naming unpark the seat lifts a park a person
// recorded on a tier-1 goal and says what it verified; a blocker park, a
// tier-2 goal, a missing reason, an entry without the verb and a bare
// unpark refuse; recovery closes an interrupted attorney unpark by name.
func TestUnparkUnderPowerOfAttorney(t *testing.T) {
	t.Parallel()
	endpoint := attorneyBed(t)
	root := endpoint.Root
	human := attorneyReq(endpoint, 40, "mac-a")
	human.Actor.Human = "Wido"
	proof := testHumanAuthority(t, root, human.Now)
	human.Authority = proof
	expires := human.Now.AddDate(0, 0, 5).Format("2006-01-02")
	lifting, err := Grant(human, proof, []uint8{1, 2}, []string{"unpark"}, expires)
	if err != nil || lifting.Outcome != OutcomeConfirmed {
		t.Fatalf("grant unpark: %+v %v", lifting, err)
	}
	liftEntry, err := resolveAttorneyForEndpoint(endpoint, human.opid(), "unpark", human.Now)
	if err != nil {
		t.Fatalf("resolve unpark entry: %v", err)
	}
	approving := withUlid(human, 41)
	approveOnly, err := Grant(approving, proof, []uint8{1}, []string{"approve"}, expires)
	if err != nil || approveOnly.Outcome != OutcomeConfirmed {
		t.Fatalf("grant approve: %+v %v", approveOnly, err)
	}
	approveEntry, err := resolveAttorneyForEndpoint(endpoint, approving.opid(), "approve", approving.Now)
	if err != nil {
		t.Fatalf("resolve approve entry: %v", err)
	}
	low := RiskRecord{Severity: 1, Novelty: 1, Exposure: 1, Accumulation: 1, Basis: "routine"}
	mid := RiskRecord{Severity: 2, Novelty: 1, Exposure: 1, Accumulation: 1, Basis: "moderate"}
	small := Budget{ElapsedLimit: "1h", AttemptLimit: 3, ReservedJobMinutesLimit: 360, ActiveJobLimit: 1, ReviewRoundLimit: 0}
	for i, open := range []struct {
		id   string
		risk RiskRecord
	}{{"paused-one", low}, {"paused-two", mid}, {"held-one", low}, {"revoked-one", low}} {
		if res, err := OpenRisked(attorneyReq(endpoint, 70+i, "mac-a"), open.id, "Work "+open.id+".", OriginHuman, "Do it.", "", open.risk, 0, "", &small, nil); err != nil || res.Outcome != OutcomeConfirmed {
			t.Fatalf("open %s: %+v %v", open.id, res, err)
		}
	}
	// A person parks the tier-1 goal.
	park := withUlid(human, 45)
	if res, err := Park(park, "paused-one", "wait for the vendor's 1.2 release"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("human park: %+v %v", res, err)
	}
	seat := attorneyReq(endpoint, 46, "mac-a")
	// A bare seat unpark still refuses; an entry without the verb refuses;
	// an empty reason refuses at the edge.
	expectRefusal(t, "bare unpark", mustPublish(Unpark(seat, "paused-one")), nil, "lifting a human's pause is a human act")
	wrong := withUlid(seat, 47)
	wrong.Attorney = &approveEntry
	expectRefusal(t, "entry without unpark", mustPublish(UnparkUnderAttorney(wrong, "paused-one", "1.2 shipped")), nil, "covers approve, not unpark")
	lifter := withUlid(seat, 48)
	lifter.Attorney = &liftEntry
	if _, err := UnparkUnderAttorney(lifter, "paused-one", "  "); err == nil || !strings.Contains(err.Error(), "--verified") {
		t.Fatalf("an empty verified reason was accepted: %v", err)
	}
	res, err := UnparkUnderAttorney(lifter, "paused-one", "1.2 is on the vendor's page")
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("unpark under attorney: %+v %v", res, err)
	}
	tree, err := loadTreeFor(endpoint, res.Tip)
	if err != nil {
		t.Fatal(err)
	}
	lifted := tree.Live["paused-one"]
	last := lifted.History[len(lifted.History)-1]
	if lifted.State != StateQueued || lifted.Parked != nil || last.Verb != "unpark" || last.Actor != "mac-a+lin-1" ||
		last.AuthorityOutcome != AuthorityOutcomePowerOfAttorney || last.AuthorityRuling != liftEntry.ID || last.Reason != "verified: 1.2 is on the vendor's page" {
		t.Fatalf("the attorney unpark did not record its act: state=%s parked=%+v last=%+v", lifted.State, lifted.Parked, last)
	}
	// The line round-trips through the file grammar.
	if parsed, problems := ParseFile(RenderFile(lifted)); len(problems) != 0 || parsed.History[len(parsed.History)-1].Reason != last.Reason {
		t.Fatalf("the attorney unpark line did not round-trip: %v", problems)
	}
	// A tier-2 person park refuses whatever the entry covers.
	if res, err := Park(withUlid(human, 49), "paused-two", "the person pauses a tier-2 goal"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("human park tier 2: %+v %v", res, err)
	}
	expectRefusal(t, "tier-2 unpark", mustPublish(UnparkUnderAttorney(withUlid(lifter, 50), "paused-two", "it holds")), nil, "tier-1 goals only")
	// An entry that does not cover tier 1 lifts no tier-1 park either.
	narrowing := withUlid(human, 62)
	narrow, err := Grant(narrowing, proof, []uint8{2}, []string{"unpark"}, expires)
	if err != nil || narrow.Outcome != OutcomeConfirmed {
		t.Fatalf("grant a tier-2 unpark entry: %+v %v", narrow, err)
	}
	narrowEntry, err := resolveAttorneyForEndpoint(endpoint, narrowing.opid(), "unpark", narrowing.Now)
	if err != nil {
		t.Fatalf("resolve the tier-2 entry: %v", err)
	}
	if res, err := Park(withUlid(human, 63), "paused-one", "paused for the narrow entry"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("human park for the narrow entry: %+v %v", res, err)
	}
	narrowLifter := withUlid(seat, 64)
	narrowLifter.Attorney = &narrowEntry
	expectRefusal(t, "tier-2-only entry", mustPublish(UnparkUnderAttorney(narrowLifter, "paused-one", "it holds")), nil, "covers tier 2 only")
	// A person's park that gained a blocker edge afterwards stays until the
	// blocker is done: the goal could not be worked anyway.
	if res, err := OpenRisked(withUlid(human, 65), "fix-paused", "The defect that blocks paused-one.", OriginHuman, "Fix.", "paused-one", low, 0, "", &small, nil); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("human open --blocks against a parked goal: %+v %v", res, err)
	}
	expectRefusal(t, "park with an unfinished blocker edge", mustPublish(UnparkUnderAttorney(withUlid(lifter, 66), "paused-one", "the condition passed")), nil, "which is not done")
	if res, err := Done(withUlid(human, 67), "fix-paused", "fixed"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("done the blocker: %+v %v", res, err)
	}
	if res, err := UnparkUnderAttorney(withUlid(lifter, 68), "paused-one", "the condition passed"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("unpark after the blocker is done: %+v %v", res, err)
	}
	// A goal that is not parked refuses, and a claimed goal keeps its fence.
	approveGoalForTest(t, attorneyReq(endpoint, 51, "mac-a"), "held-one", small)
	claimed, err := Claim(attorneyReq(endpoint, 52, "mac-a"), "held-one")
	if err != nil || claimed.Outcome != OutcomeConfirmed {
		t.Fatalf("claim held-one: %+v %v", claimed, err)
	}
	expectRefusal(t, "claimed goal", mustPublish(UnparkUnderAttorney(withUlid(lifter, 56), "held-one", "nothing to lift")), nil, "is claimed, not parked")
	before, err := loadTreeFor(endpoint, claimed.Tip)
	if err != nil {
		t.Fatal(err)
	}
	after, err := Project(human.Endpoint, false, human.Now)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(after.Tree.Live["held-one"].Claimed) != fmt.Sprint(before.Live["held-one"].Claimed) {
		t.Fatalf("a refused attorney unpark touched the fence: %+v", after.Tree.Live["held-one"].Claimed)
	}
	// The park unapprove records when it displaces a claim is a person's
	// park too, and lifts back to queued.
	approveGoalForTest(t, attorneyReq(endpoint, 57, "mac-a"), "revoked-one", small)
	if res, err := Claim(attorneyReq(endpoint, 60, "mac-b"), "revoked-one"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("claim revoked-one: %+v %v", res, err)
	}
	if res, err := Unapprove(withUlid(human, 58), "revoked-one", "the scope moved", proof); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("unapprove: %+v %v", res, err)
	}
	revoked, err := UnparkUnderAttorney(withUlid(lifter, 59), "revoked-one", "the scope is settled again")
	if err != nil || revoked.Outcome != OutcomeConfirmed {
		t.Fatalf("unpark an unapprove park under attorney: %+v %v", revoked, err)
	}
	if tree, err := loadTreeFor(endpoint, revoked.Tip); err != nil || tree.Live["revoked-one"].State != StateQueued || tree.Live["revoked-one"].Parked != nil {
		t.Fatalf("the unapprove park did not lift to queued: %v %+v", err, tree.Live["revoked-one"])
	}
	// A blocker park is never lifted this way.
	if res, err := OpenRisked(attorneyReq(endpoint, 53, "mac-a"), "fix-held", "The defect that blocks held-one.", OriginMain, "Fix.", "held-one", low, 0, "", &small, nil); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open --blocks: %+v %v", res, err)
	}
	expectRefusal(t, "blocker park", mustPublish(UnparkUnderAttorney(withUlid(lifter, 54), "held-one", "the blocker is done")), nil, "returns by itself")
	// Recovery closes an interrupted attorney unpark by name instead of
	// replaying it as a plain agent unpark.
	if res, err := Park(withUlid(human, 55), "paused-one", "paused again"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("human park again: %+v %v", res, err)
	}
	opid := Opid("01J5X00000000000000000PA61", "mac-a", "lin-1")
	strandEntryAt(t, root, opid, "mac-a", PhaseCreated, Intent{Verb: "unpark", Targets: []string{"paused-one"}, Args: map[string]string{"under": liftEntry.ID, "verified": "1.2 shipped"}})
	reports, err := Recover(human.Endpoint)
	if err != nil {
		t.Fatal(err)
	}
	closed := false
	for _, report := range reports {
		if report.Opid == opid && strings.Contains(report.Detail, "cannot be replayed") {
			closed = true
		}
	}
	if !closed {
		t.Fatalf("recovery replayed or ignored the attorney unpark: %+v", reports)
	}
	p, err := Project(human.Endpoint, false, human.Now)
	if err != nil {
		t.Fatal(err)
	}
	if p.Tree.Live["paused-one"].State != StateParked {
		t.Fatal("recovery lifted a person's park without the live act")
	}
}

func mustPublish(res PublishResult, err error) PublishResult {
	if err != nil {
		return PublishResult{Outcome: OutcomeRejected, Detail: err.Error()}
	}
	return res
}
