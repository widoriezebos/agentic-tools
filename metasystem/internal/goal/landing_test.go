package goal

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// landingReq is verbReq with a controllable clock.
func landingReq(root, ulid, machine string, at time.Time) VerbRequest {
	r := verbReq(root, ulid, machine)
	r.Now = at
	return r
}

func TestLandReadyOpensTheSlotBesideAWorkingClaim(t *testing.T) {
	_, a, b := twoClones(t)
	seedLedger(t, a)
	budget := testBudget()
	t0 := time.Date(2026, 9, 12, 8, 0, 0, 0, time.UTC)
	for _, id := range []string{"built-a", "next-b", "third-c"} {
		if res, err := Open(landingReq(a, "01J5X00000000000000000NA0"+strings.ToUpper(id[len(id)-1:]), "mac-a", t0), id, "Work called "+id, OriginMain, "Build it."); err != nil || res.Outcome != OutcomeConfirmed {
			t.Fatalf("open %s: %+v %v", id, res, err)
		}
	}
	if res, err := claimApprovedForTest(t, landingReq(a, "01J5X00000000000000000NA10", "mac-a", t0.Add(time.Minute)), "built-a", budget); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("claim built-a: %+v %v", res, err)
	}
	// Another pair cannot enter the holder's claim into landing; a person
	// has no such act either.
	if res, err := LandReady(landingReq(b, "01J5X00000000000000000NA11", "mac-b", t0.Add(2*time.Minute)), "built-a"); err != nil || res.Outcome != OutcomeRejected || !strings.Contains(res.Detail, "claim holder's own act") {
		t.Fatalf("a foreign land-ready is refused: %+v %v", res, err)
	}
	human := landingReq(a, "01J5X00000000000000000NA12", "mac-a", t0.Add(2*time.Minute))
	human.Actor.Human = "Wido"
	if _, err := LandReady(human, "built-a"); err == nil || !strings.Contains(err.Error(), "takes no --by") {
		t.Fatalf("land-ready under --by is refused at the edge: %v", err)
	}
	// The holder enters landing: the claim stays, the record and the
	// history line are written.
	landAt := t0.Add(3 * time.Hour)
	res, err := LandReady(landingReq(a, "01J5X00000000000000000NA13", "mac-a", landAt), "built-a")
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("land-ready: %+v %v", res, err)
	}
	tree, err := loadTree(a, res.Tip)
	if err != nil {
		t.Fatal(err)
	}
	landing := tree.Live["built-a"]
	if landing.State != StateClaimed || landing.Claimed == nil || landing.Landing == nil || landing.Landing.At != landAt.Format(time.RFC3339) || landing.Landing.Opid != landing.History[len(landing.History)-1].Opid {
		t.Fatalf("land-ready did not write the record with its operation: %+v history=%+v", landing.Landing, landing.History[len(landing.History)-1])
	}
	if last := landing.History[len(landing.History)-1]; last.Verb != "land-ready" {
		t.Fatalf("land-ready did not write its history line: %+v", last)
	}
	if !landing.IsLandingClaim() {
		t.Fatal("a claimed goal with a Landing record is a landing claim")
	}
	// A repeat is nothing to do; the ledger does not move.
	before := acceptedTip(t, a)
	if res, err := LandReady(landingReq(a, "01J5X00000000000000000NA14", "mac-a", landAt.Add(time.Minute)), "built-a"); err != nil || res.Outcome != OutcomeAbandoned || !strings.Contains(res.Detail, "already in landing") {
		t.Fatalf("a repeated land-ready is nothing to do: %+v %v", res, err)
	}
	if acceptedTip(t, a) != before {
		t.Fatal("the repeated land-ready moved the ledger")
	}
	// The machine's one claim is free: the next goal claims beside the
	// landing goal and the tree validates.
	if res, err := claimApprovedForTest(t, landingReq(a, "01J5X00000000000000000NA15", "mac-a", landAt.Add(2*time.Minute)), "next-b", budget); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("claim beside the landing goal: %+v %v", res, err)
	}
	if err := ValidateCommit(a, acceptedTip(t, a)); err != nil {
		t.Fatalf("one landing claim beside one working claim must validate: %v", err)
	}
	// One landing slot per machine.
	if res, err := LandReady(landingReq(a, "01J5X00000000000000000NA16", "mac-a", landAt.Add(3*time.Minute)), "next-b"); err != nil || res.Outcome != OutcomeRejected || !strings.Contains(res.Detail, "one landing slot per machine") {
		t.Fatalf("a second landing slot is refused: %+v %v", res, err)
	}
	// A third working claim is still over the quota.
	if res, err := claimApprovedForTest(t, landingReq(a, "01J5X00000000000000000NA17", "mac-a", landAt.Add(4*time.Minute)), "third-c", budget); err != nil || res.Outcome != OutcomeRejected || !strings.Contains(res.Detail, "quota is one claim per machine") {
		t.Fatalf("the working claim still consumes the quota: %+v %v", res, err)
	}
	// The frontier lists the landing goal apart and continues the working
	// claim; the current goal is the working claim.
	projection, err := Project(endpointFor(a), true, landAt.Add(5*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	frontier, err := Next(projection, "mac-a")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(frontier.Landing, ",") != "built-a" || strings.Join(frontier.Claimed, ",") != "next-b" {
		t.Fatalf("the frontier did not separate the landing claim: %+v", frontier)
	}
	if selection := SelectNext(frontier); selection.Kind != NextSelectionContinue || selection.GoalID != "next-b" {
		t.Fatalf("the working claim is continued first: %+v", selection)
	}
	if current := currentClaimOf(projection.Tree, "mac-a"); current == nil || current.Id != "next-b" {
		t.Fatalf("the current goal is the working claim: %+v", current)
	}
	lines := LandingClaimLines([]*GoalFile{projection.Tree.Live["built-a"]}, landAt.Add(5*time.Minute))
	if len(lines) != 1 || !strings.HasPrefix(lines[0], "LANDING built-a: land-ready since "+landAt.Format(time.RFC3339)) {
		t.Fatalf("the landing line: %v", lines)
	}
	// Past its elapsed box the wait prints as overdue and nothing more.
	overdue := LandingClaimLines([]*GoalFile{projection.Tree.Live["built-a"]}, t0.Add(5*time.Hour))
	if len(overdue) != 1 || !strings.HasPrefix(overdue[0], "LANDING OVERDUE built-a:") {
		t.Fatalf("the overdue landing line: %v", overdue)
	}
	// Release the working claim: the landing goal alone resolves as current
	// and the frontier offers the ready goal.
	if res, err := Release(landingReq(a, "01J5X00000000000000000NA18", "mac-a", landAt.Add(6*time.Minute)), "next-b"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("release: %+v %v", res, err)
	}
	projection, err = Project(endpointFor(a), true, landAt.Add(7*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if current := currentClaimOf(projection.Tree, "mac-a"); current == nil || current.Id != "built-a" {
		t.Fatalf("a landing goal alone resolves as current: %+v", current)
	}
	frontier, err = Next(projection, "mac-a")
	if err != nil {
		t.Fatal(err)
	}
	if selection := SelectNext(frontier); selection.Kind != NextSelectionReady || selection.GoalID != "next-b" {
		t.Fatalf("a landing goal never blocks the next claim: %+v frontier=%+v", selection, frontier)
	}
	// Done archives the goal with the land-ready line and without the slot.
	if res, err := Done(landingReq(a, "01J5X00000000000000000NA19", "mac-a", landAt.Add(8*time.Minute)), "built-a", "Landed."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("done: %+v %v", res, err)
	}
	tree, err = loadTree(a, acceptedTip(t, a))
	if err != nil {
		t.Fatal(err)
	}
	archived := tree.Done["built-a"]
	if archived == nil || archived.Landing != nil || archived.Claimed != nil {
		t.Fatalf("done did not clear the slot with the claim: %+v", archived)
	}
	landReadyLines := 0
	for _, line := range archived.History {
		if line.Verb == "land-ready" {
			landReadyLines++
		}
	}
	if landReadyLines != 1 {
		t.Fatalf("the land-ready line did not survive into the archive: %+v", archived.History)
	}
	// Park and release clear the slot too; the own pair's leave keeps the
	// episode (section 2), so the record travels and the slot does not.
	// third-c was approved by the refused claim above; the claim now fits.
	if res, err := Claim(landingReq(a, "01J5X00000000000000000NA1A", "mac-a", landAt.Add(9*time.Minute)), "third-c"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("claim third-c: %+v %v", res, err)
	}
	if res, err := LandReady(landingReq(a, "01J5X00000000000000000NA1B", "mac-a", landAt.Add(10*time.Minute)), "third-c"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("land-ready third-c: %+v %v", res, err)
	}
	if res, err := Park(landingReq(a, "01J5X00000000000000000NA1C", "mac-a", landAt.Add(11*time.Minute)), "third-c", "wait for the landing window"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("park: %+v %v", res, err)
	}
	tree, err = loadTree(a, acceptedTip(t, a))
	if err != nil {
		t.Fatal(err)
	}
	parked := tree.Live["third-c"]
	if parked.Landing != nil || parked.Claimed != nil || parked.Episode == nil {
		t.Fatalf("park did not clear the slot while keeping the own pair's episode: %+v episode=%+v", parked.Landing, parked.Episode)
	}
}

func TestSamePairReclaimKeepsItsEpisodeAndAnotherPairStartsFresh(t *testing.T) {
	_, a, b := twoClones(t)
	seedLedger(t, a)
	budget := testBudget()
	t0 := time.Date(2026, 9, 12, 8, 0, 0, 0, time.UTC)
	if res, err := Open(landingReq(a, "01J5X00000000000000000NE00", "mac-a", t0), "kept", "Work the pair leaves and returns to.", OriginMain, "Build it."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", res, err)
	}
	claimAt := t0.Add(time.Minute)
	if res, err := claimApprovedForTest(t, landingReq(a, "01J5X00000000000000000NE01", "mac-a", claimAt), "kept", budget); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("claim: %+v %v", res, err)
	}
	tree, err := loadTree(a, acceptedTip(t, a))
	if err != nil {
		t.Fatal(err)
	}
	first := *tree.Live["kept"].Claimed
	// The own pair releases after an hour: the goal keeps the episode.
	releaseAt := claimAt.Add(time.Hour)
	if res, err := Release(landingReq(a, "01J5X00000000000000000NE02", "mac-a", releaseAt), "kept"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("release: %+v %v", res, err)
	}
	tree, err = loadTree(a, acceptedTip(t, a))
	if err != nil {
		t.Fatal(err)
	}
	released := tree.Live["kept"]
	if released.Claimed != nil || released.Episode == nil {
		t.Fatalf("the own pair's release did not keep the episode: %+v", released.Episode)
	}
	if e := released.Episode; e.Machine != "mac-a" || e.Lineage != "lin-1" || e.AccountingRevision != first.AccountingRevision || e.EpisodeAt != first.EpisodeAt || e.EpisodeRevision != first.EpisodeRevision || e.IdleSeconds != 0 || e.Released != releaseAt.Format(time.RFC3339) {
		t.Fatalf("the kept episode contradicts the claim it left: %+v claim=%+v", e, first)
	}
	if err := ValidateCommit(a, acceptedTip(t, a)); err != nil {
		t.Fatalf("an unclaimed goal with a kept episode validates: %v", err)
	}
	// The same pair re-claims two hours later: the accounting facts return
	// and the gap is idle time.
	reclaimAt := releaseAt.Add(2 * time.Hour)
	if res, err := Claim(landingReq(a, "01J5X00000000000000000NE03", "mac-a", reclaimAt), "kept"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("re-claim: %+v %v", res, err)
	}
	tree, err = loadTree(a, acceptedTip(t, a))
	if err != nil {
		t.Fatal(err)
	}
	reclaimed := tree.Live["kept"]
	if reclaimed.Episode != nil || reclaimed.Claimed == nil {
		t.Fatalf("the re-claim did not consume the kept episode: %+v", reclaimed)
	}
	if c := reclaimed.Claimed; c.AccountingRevision != first.AccountingRevision || c.EpisodeAt != first.EpisodeAt || c.EpisodeRevision != first.EpisodeRevision || c.IdleSeconds != 7200 || c.Revision == first.Revision {
		t.Fatalf("the re-claim did not continue the episode with the gap idle: %+v first=%+v", c, first)
	}
	if err := ValidateCommit(a, acceptedTip(t, a)); err != nil {
		t.Fatalf("a re-claimed goal validates: %v", err)
	}
	rendered := string(RenderFile(reclaimed))
	if !strings.Contains(rendered, " idleSeconds=7200") {
		t.Fatalf("idle seconds did not render on the claim: %s", rendered)
	}
	// A human set-budget keeps the idle seconds and the episode.
	if res, err := setBudgetApprovedForTest(t, landingReq(a, "01J5X00000000000000000NE04", "mac-a", reclaimAt.Add(time.Minute)), "kept", Budget{ElapsedLimit: "6h", AttemptLimit: 6, ReservedJobMinutesLimit: 300, ActiveJobLimit: 2}); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("set-budget: %+v %v", res, err)
	}
	tree, err = loadTree(a, acceptedTip(t, a))
	if err != nil {
		t.Fatal(err)
	}
	if c := tree.Live["kept"].Claimed; c.IdleSeconds != 7200 || c.EpisodeAt != first.EpisodeAt {
		t.Fatalf("set-budget dropped the idle seconds or the episode: %+v", c)
	}
	// The own pair releases again; another pair claims and starts fresh.
	secondRelease := reclaimAt.Add(2 * time.Minute)
	if res, err := Release(landingReq(a, "01J5X00000000000000000NE05", "mac-a", secondRelease), "kept"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("second release: %+v %v", res, err)
	}
	tree, err = loadTree(a, acceptedTip(t, a))
	if err != nil {
		t.Fatal(err)
	}
	if e := tree.Live["kept"].Episode; e == nil || e.IdleSeconds != 7200 || e.EpisodeAt != first.EpisodeAt {
		t.Fatalf("the second release did not keep the continued episode: %+v", e)
	}
	if res, err := Claim(landingReq(b, "01J5X00000000000000000NE06", "mac-b", secondRelease.Add(time.Minute)), "kept"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("foreign claim: %+v %v", res, err)
	}
	tree, err = loadTree(b, acceptedTip(t, b))
	if err != nil {
		t.Fatal(err)
	}
	fresh := tree.Live["kept"]
	if fresh.Episode != nil || fresh.Claimed == nil || fresh.Claimed.Machine != "mac-b" || fresh.Claimed.IdleSeconds != 0 || fresh.Claimed.AccountingRevision != fresh.Claimed.Revision || fresh.Claimed.EpisodeAt == first.EpisodeAt {
		t.Fatalf("another pair's claim did not start fresh: %+v", fresh.Claimed)
	}
}

func TestParksDropOrKeepTheEpisodeByWhoParks(t *testing.T) {
	_, a, _ := twoClones(t)
	seedLedger(t, a)
	budget := testBudget()
	t0 := time.Date(2026, 9, 12, 8, 0, 0, 0, time.UTC)
	if res, err := Open(landingReq(a, "01J5X00000000000000000NP00", "mac-a", t0), "paused", "Work the pair pauses.", OriginMain, "Build it."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", res, err)
	}
	if res, err := claimApprovedForTest(t, landingReq(a, "01J5X00000000000000000NP01", "mac-a", t0.Add(time.Minute)), "paused", budget); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("claim: %+v %v", res, err)
	}
	tree, err := loadTree(a, acceptedTip(t, a))
	if err != nil {
		t.Fatal(err)
	}
	first := *tree.Live["paused"].Claimed
	// An own-pair park keeps the episode; unpark keeps it; the same pair's
	// claim continues it with the pause idle.
	parkAt := t0.Add(31 * time.Minute)
	if res, err := Park(landingReq(a, "01J5X00000000000000000NP02", "mac-a", parkAt), "paused", "waiting on a fixture"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("park: %+v %v", res, err)
	}
	tree, err = loadTree(a, acceptedTip(t, a))
	if err != nil {
		t.Fatal(err)
	}
	if e := tree.Live["paused"].Episode; e == nil || e.Released != parkAt.Format(time.RFC3339) || e.EpisodeAt != first.EpisodeAt {
		t.Fatalf("an own-pair park did not keep the episode: %+v", e)
	}
	if err := ValidateCommit(a, acceptedTip(t, a)); err != nil {
		t.Fatalf("a parked goal with a kept episode validates: %v", err)
	}
	if res, err := Unpark(landingReq(a, "01J5X00000000000000000NP03", "mac-a", parkAt.Add(10*time.Minute)), "paused"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("unpark: %+v %v", res, err)
	}
	tree, err = loadTree(a, acceptedTip(t, a))
	if err != nil {
		t.Fatal(err)
	}
	if tree.Live["paused"].Episode == nil {
		t.Fatal("unpark dropped the kept episode")
	}
	if res, err := Claim(landingReq(a, "01J5X00000000000000000NP04", "mac-a", parkAt.Add(30*time.Minute)), "paused"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("re-claim after the pause: %+v %v", res, err)
	}
	tree, err = loadTree(a, acceptedTip(t, a))
	if err != nil {
		t.Fatal(err)
	}
	if c := tree.Live["paused"].Claimed; c.IdleSeconds != 1800 || c.AccountingRevision != first.AccountingRevision || c.EpisodeAt != first.EpisodeAt {
		t.Fatalf("park-and-reclaim did not continue the box: %+v", c)
	}
	// A person's park starts the box afresh: the episode is gone.
	human := landingReq(a, "01J5X00000000000000000NP05", "mac-a", parkAt.Add(40*time.Minute))
	human.Actor.Human = "Wido"
	if res, err := Park(human, "paused", "the person pauses it"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("human park: %+v %v", res, err)
	}
	tree, err = loadTree(a, acceptedTip(t, a))
	if err != nil {
		t.Fatal(err)
	}
	if tree.Live["paused"].Episode != nil {
		t.Fatalf("a person's park kept the episode: %+v", tree.Live["paused"].Episode)
	}
	// A kept episode on an approved goal is dropped by the person's
	// unapprove and by approve with a tuple.
	unparkHuman := landingReq(a, "01J5X00000000000000000NP06", "mac-a", parkAt.Add(41*time.Minute))
	unparkHuman.Actor.Human = "Wido"
	if res, err := Unpark(unparkHuman, "paused"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("human unpark: %+v %v", res, err)
	}
	if res, err := Claim(landingReq(a, "01J5X00000000000000000NP07", "mac-a", parkAt.Add(42*time.Minute)), "paused"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("claim: %+v %v", res, err)
	}
	if res, err := Release(landingReq(a, "01J5X00000000000000000NP08", "mac-a", parkAt.Add(50*time.Minute)), "paused"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("release: %+v %v", res, err)
	}
	tree, err = loadTree(a, acceptedTip(t, a))
	if err != nil {
		t.Fatal(err)
	}
	if tree.Live["paused"].Episode == nil {
		t.Fatal("the release did not keep the episode")
	}
	withdraw := landingReq(a, "01J5X00000000000000000NP09", "mac-a", parkAt.Add(51*time.Minute))
	withdraw.Actor.Human = "Wido"
	if res, err := Unapprove(withdraw, "paused", "rethink the box", testHumanAuthority(t, a, withdraw.Now)); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("unapprove: %+v %v", res, err)
	}
	tree, err = loadTree(a, acceptedTip(t, a))
	if err != nil {
		t.Fatal(err)
	}
	if tree.Live["paused"].Episode != nil {
		t.Fatalf("unapprove kept the episode: %+v", tree.Live["paused"].Episode)
	}
}

func TestSetBudgetKeepsTheLandingSlotAndUnapproveDropsIt(t *testing.T) {
	_, a, _ := twoClones(t)
	seedLedger(t, a)
	budget := testBudget()
	t0 := time.Date(2026, 9, 12, 8, 0, 0, 0, time.UTC)
	if res, err := Open(landingReq(a, "01J5X00000000000000000NS00", "mac-a", t0), "slotted", "Built work.", OriginMain, "Land it."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", res, err)
	}
	if res, err := claimApprovedForTest(t, landingReq(a, "01J5X00000000000000000NS01", "mac-a", t0.Add(time.Minute)), "slotted", budget); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("claim: %+v %v", res, err)
	}
	if res, err := LandReady(landingReq(a, "01J5X00000000000000000NS02", "mac-a", t0.Add(time.Hour)), "slotted"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("land-ready: %+v %v", res, err)
	}
	if res, err := setBudgetApprovedForTest(t, landingReq(a, "01J5X00000000000000000NS03", "mac-a", t0.Add(61*time.Minute)), "slotted", Budget{ElapsedLimit: "6h", AttemptLimit: 6, ReservedJobMinutesLimit: 300, ActiveJobLimit: 2}); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("set-budget: %+v %v", res, err)
	}
	tree, err := loadTree(a, acceptedTip(t, a))
	if err != nil {
		t.Fatal(err)
	}
	if f := tree.Live["slotted"]; f.Landing == nil || !f.IsLandingClaim() {
		t.Fatalf("set-budget dropped the landing slot: %+v", f.Landing)
	}
	withdraw := landingReq(a, "01J5X00000000000000000NS04", "mac-a", t0.Add(62*time.Minute))
	withdraw.Actor.Human = "Wido"
	if res, err := Unapprove(withdraw, "slotted", "not this week", testHumanAuthority(t, a, withdraw.Now)); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("unapprove: %+v %v", res, err)
	}
	tree, err = loadTree(a, acceptedTip(t, a))
	if err != nil {
		t.Fatal(err)
	}
	if f := tree.Live["slotted"]; f.State != StateParked || f.Landing != nil || f.Episode != nil {
		t.Fatalf("unapprove did not drop the slot and the episode with the claim: state=%s landing=%+v episode=%+v", f.State, f.Landing, f.Episode)
	}
}

func TestLandingAndEpisodeRecordsRoundTripAndValidate(t *testing.T) {
	file := episodeGolden()
	file.Claimed.IdleSeconds = 90
	file.Landing = &LandingRecord{At: "2026-08-20T03:00:00Z", Opid: "01J5X0000000000000000000C5-mac-studio-1a2b3c4d"}
	rendered := RenderFile(file)
	parsed, problems := ParseFile(rendered)
	if len(problems) != 0 || parsed.Claimed == nil || *parsed.Claimed != *file.Claimed || parsed.Landing == nil || *parsed.Landing != *file.Landing || string(RenderFile(parsed)) != string(rendered) {
		t.Fatalf("landing claim did not round-trip: claim=%+v landing=%+v problems=%v\n%s", parsed.Claimed, parsed.Landing, problems, rendered)
	}
	if !strings.Contains(string(rendered), "- Landing: at=2026-08-20T03:00:00Z opid=01J5X0000000000000000000C5-mac-studio-1a2b3c4d\n") || !strings.Contains(string(rendered), " idleSeconds=90\n") {
		t.Fatalf("the grammar: %s", rendered)
	}
	// A landing record on a goal that is not claimed refuses.
	unclaimed := episodeGolden()
	unclaimed.State = StateQueued
	unclaimed.Claimed = nil
	unclaimed.StopCapability = nil
	unclaimed.Landing = &LandingRecord{At: "2026-08-20T03:00:00Z", Opid: "01J5X0000000000000000000C5-mac-studio-1a2b3c4d"}
	if _, problems := ParseFile(RenderFile(unclaimed)); !problemsContain(problems, "Landing record on a goal that is not claimed") {
		t.Fatalf("landing on an unclaimed goal did not refuse: %v", problems)
	}
	// An episode record lives only on an unclaimed live goal.
	kept := episodeGolden()
	kept.State = StateQueued
	kept.Claimed = nil
	kept.StopCapability = nil
	kept.Episode = &EpisodeRecord{Machine: "mac-studio", Lineage: "lin-1", AccountingRevision: 5, EpisodeAt: "2026-08-20T01:00:00Z", EpisodeRevision: 2, EpisodeObligationRevision: 4, IdleSeconds: 30, Released: "2026-08-20T04:00:00Z"}
	rendered = RenderFile(kept)
	parsed, problems = ParseFile(rendered)
	if len(problems) != 0 || parsed.Episode == nil || *parsed.Episode != *kept.Episode || string(RenderFile(parsed)) != string(rendered) {
		t.Fatalf("episode record did not round-trip: %+v problems=%v\n%s", parsed.Episode, problems, rendered)
	}
	claimedWithEpisode := episodeGolden()
	claimedWithEpisode.Episode = kept.Episode
	if _, problems := ParseFile(RenderFile(claimedWithEpisode)); !problemsContain(problems, "Episode record on a claimed goal") {
		t.Fatalf("episode on a claimed goal did not refuse: %v", problems)
	}
	backwards := *kept
	backwardsEpisode := *kept.Episode
	backwardsEpisode.Released = "2026-08-20T00:30:00Z"
	backwards.Episode = &backwardsEpisode
	if _, problems := ParseFile(RenderFile(&backwards)); !problemsContain(problems, "precedes episodeAt") {
		t.Fatalf("a release before the episode start did not refuse: %v", problems)
	}
	// Two landing claims on one machine refuse at the tree.
	one := vGoal("landing-one", StateClaimed)
	two := vGoal("landing-two", StateClaimed)
	for _, f := range []*GoalFile{one, two} {
		f.Landing = &LandingRecord{At: "2026-08-20T10:06:00Z", Opid: "01J5X0000000000000000000B1-mac-a-1a2b3c4d"}
	}
	tree := &TreeGoals{Root: vRoot(), Live: map[string]*GoalFile{one.Id: one, two.Id: two}, Done: map[string]*GoalFile{}}
	found := false
	for _, problem := range ValidateTree(tree) {
		if strings.Contains(string(problem), "one landing slot per machine") {
			found = true
		}
	}
	if !found {
		t.Fatalf("two landing slots on one machine did not refuse: %v", ValidateTree(tree))
	}
	// One landing claim beside one working claim on one machine is lawful.
	working := vGoal("working", StateClaimed)
	tree = &TreeGoals{Root: vRoot(), Live: map[string]*GoalFile{one.Id: one, working.Id: working}, Done: map[string]*GoalFile{}}
	for _, problem := range ValidateTree(tree) {
		if strings.Contains(string(problem), "quota") || strings.Contains(string(problem), "landing slot") {
			t.Fatalf("a landing claim beside a working claim refused: %v", problem)
		}
	}
}

func TestHandEditsOfLandingAndEpisodeFollowTheirVerbs(t *testing.T) {
	base := episodeGolden()
	base.Landing = &LandingRecord{At: "2026-08-20T03:00:00Z", Opid: "01J5X0000000000000000000C5-mac-studio-1a2b3c4d"}
	// Adding, changing or dropping the slot by hand refuses.
	edited := *base
	edited.Landing = nil
	if _, err := mapOneChange("plans/goals/episode.md", base, &edited); err == nil || !strings.Contains(err.Error(), "Landing is a generated field") {
		t.Fatalf("a hand-dropped landing slot did not refuse: %v", err)
	}
	moved := *base
	moved.Landing = &LandingRecord{At: "2026-08-20T03:30:00Z", Opid: base.Landing.Opid}
	if _, err := mapOneChange("plans/goals/episode.md", base, &moved); err == nil || !strings.Contains(err.Error(), "Landing is a generated field") {
		t.Fatalf("a hand-moved landing slot did not refuse: %v", err)
	}
	// A hand park of the claimed goal drops it with the claim line.
	parked := *base
	parked.State = StateParked
	parked.Claimed = nil
	parked.StopCapability = nil
	parked.Landing = nil
	parked.Parked = &ParkRecord{By: "human:wido", At: "2026-08-20T04:00:00Z", Because: "wait"}
	rows, err := mapOneChange("plans/goals/episode.md", base, &parked)
	if err != nil || len(rows) == 0 || rows[0].Verb != "park" {
		t.Fatalf("a hand park did not map with the slot dropped: %+v %v", rows, err)
	}
	// A hand done may drop it too (the replay clears the claim binding); a
	// hand done that keeps the line is the ordinary unchanged copy.
	done := *base
	done.State = StateDone
	done.Conclude = "Landed by hand."
	done.Landing = nil
	if rows, err := mapOneChange("plans/goals/episode.md", base, &done); err != nil || len(rows) == 0 || rows[0].Verb != "done" {
		t.Fatalf("a hand done dropping the slot did not map: %+v %v", rows, err)
	}
	doneKept := *base
	doneKept.State = StateDone
	doneKept.Conclude = "Landed by hand."
	if rows, err := mapOneChange("plans/goals/episode.md", base, &doneKept); err != nil || len(rows) == 0 || rows[0].Verb != "done" {
		t.Fatalf("a hand done keeping the slot line did not map: %+v %v", rows, err)
	}
	if _, problems := ParseFile(RenderFile(&doneKept)); problemsContain(problems, "Landing record on a goal that is not claimed") {
		t.Fatalf("a hand done's unchanged Landing line was refused at parse: %v", problems)
	}
	// An episode record is written by the verbs alone.
	kept := vGoal("kept", StateApproved)
	kept.Approved = nil
	written := *kept
	written.Episode = &EpisodeRecord{Machine: "mac-a", Lineage: "lin-1", AccountingRevision: 1, EpisodeAt: "2026-08-20T10:00:00Z", EpisodeRevision: 1, IdleSeconds: 0, Released: "2026-08-20T11:00:00Z"}
	if _, err := mapOneChange("plans/goals/kept.md", kept, &written); err == nil || !strings.Contains(err.Error(), "Episode is a generated field") {
		t.Fatalf("a hand-written episode did not refuse: %v", err)
	}
	// A hand park of an unclaimed goal may drop it: a person's park starts
	// the box afresh, which is the mapped verb's own effect.
	withEpisode := vGoal("kept", StateQueued)
	withEpisode.Episode = written.Episode
	handParked := *withEpisode
	handParked.State = StateParked
	handParked.Episode = nil
	handParked.Parked = &ParkRecord{By: "human:wido", At: "2026-08-20T12:00:00Z", Because: "later"}
	if rows, err := mapOneChange("plans/goals/kept.md", withEpisode, &handParked); err != nil || len(rows) == 0 || rows[0].Verb != "park" {
		t.Fatalf("a hand park dropping the kept episode did not map: %+v %v", rows, err)
	}
	dropped := *withEpisode
	dropped.Episode = nil
	if _, err := mapOneChange("plans/goals/kept.md", withEpisode, &dropped); err == nil || !strings.Contains(err.Error(), "Episode is a generated field") {
		t.Fatalf("a hand-dropped episode without a park did not refuse: %v", err)
	}
}

func TestRecoveryReplaysADeadOwnersLandReady(t *testing.T) {
	_, a, _ := twoClones(t)
	seedLedger(t, a)
	if res, err := Open(verbReq(a, "01J5X00000000000000000NR00", "mac-a"), "built", "Built work of a dead owner.", OriginMain, "Land it."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", res, err)
	}
	if res, err := claimApprovedForTest(t, verbReq(a, "01J5X00000000000000000NR01", "mac-a"), "built", testBudget()); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("claim: %+v %v", res, err)
	}
	opid := Opid("01J5X00000000000000000NR02", "mac-a", "lin-1")
	strandEntry(t, a, opid, PhaseCreated, Intent{Verb: "land-ready", Targets: []string{"built"}})
	if _, err := Recover(endpointFor(a)); err != nil {
		t.Fatal(err)
	}
	p, err := Project(endpointFor(a), true, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	built := p.Tree.Live["built"]
	if built == nil || built.Landing == nil || built.Landing.Opid != opid || built.History[len(built.History)-1].Opid != opid {
		t.Fatalf("recovery did not replay land-ready under the original opid: %+v", built)
	}
}

func TestResumeIsNotBlockedByALandingClaim(t *testing.T) {
	_, root, _ := twoClones(t)
	seedLedger(t, root)
	budget := Budget{ElapsedLimit: "1m", AttemptLimit: 2, ReservedJobMinutesLimit: 20, ActiveJobLimit: 1}
	if result, err := Open(verbReq(root, "01J5X00000000000000000NX00", "mac-a"), "fenced-a", "Bound the stopped item.", OriginMain, "Run it."); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("open stopped goal: %+v %v", result, err)
	}
	claimA := verbReq(root, "01J5X00000000000000000NX01", "mac-a")
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
			Ulid: "01J5X00000000000000000NX02", Now: claimA.Now.Add(90 * time.Second), ClaimEpoch: 9,
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
	// The machine's other claim is in landing: it does not block the resume.
	if result, err := Open(verbReq(root, "01J5X00000000000000000NX03", "mac-a"), "landing-b", "Built work waiting to land.", OriginMain, "Land it."); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("open landing goal: %+v %v", result, err)
	}
	claimB := verbReq(root, "01J5X00000000000000000NX04", "mac-a")
	claimB.ClaimEpoch = 9
	if result, err := claimApprovedForTest(t, claimB, "landing-b", budget); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("claim landing goal: %+v %v", result, err)
	}
	landReady := verbReq(root, "01J5X00000000000000000NX05", "mac-a")
	landReady.Now = claimB.Now.Add(time.Second)
	if result, err := LandReady(landReady, "landing-b"); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("land-ready: %+v %v", result, err)
	}
	resume := ResumeRequest{
		VerbRequest: VerbRequest{
			Endpoint: endpointFor(root), Actor: Actor{Machine: "mac-a", Lineage: "human-shell", Human: "wido"},
			Ulid: "01J5X00000000000000000NX06", Now: resumeAt, ClaimEpoch: 9,
		},
		GoalID: "fenced-a", Budget: budget,
	}
	resume.Authority = testHumanAuthority(t, root, resume.Now)
	if result, err := Resume(resume); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("resume beside a landing claim: %+v %v", result, err)
	}
	if err := ValidateCommit(root, acceptedTip(t, root)); err != nil {
		t.Fatalf("a resumed working claim beside a landing claim validates: %v", err)
	}
}

func TestOwnPairLeavesKeepTheEpisodeOnEveryPath(t *testing.T) {
	_, a, _ := twoClones(t)
	seedLedger(t, a)
	budget := testBudget()
	risk := RiskRecord{Severity: 1, Novelty: 1, Exposure: 1, Accumulation: 1, Basis: "fixture"}
	t0 := time.Date(2026, 9, 12, 8, 0, 0, 0, time.UTC)
	// Release, park, unpark, claim by the seat alone: the park of the
	// unclaimed goal keeps the pair's own record.
	if res, err := Open(landingReq(a, "01J5X00000000000000000NK00", "mac-a", t0), "kept-g", "Work the seat leaves twice.", OriginMain, "Build it."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", res, err)
	}
	if res, err := claimApprovedForTest(t, landingReq(a, "01J5X00000000000000000NK01", "mac-a", t0.Add(time.Minute)), "kept-g", budget); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("claim: %+v %v", res, err)
	}
	tree, err := loadTree(a, acceptedTip(t, a))
	if err != nil {
		t.Fatal(err)
	}
	first := *tree.Live["kept-g"].Claimed
	if res, err := Release(landingReq(a, "01J5X00000000000000000NK02", "mac-a", t0.Add(time.Hour)), "kept-g"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("release: %+v %v", res, err)
	}
	if res, err := Park(landingReq(a, "01J5X00000000000000000NK03", "mac-a", t0.Add(2*time.Hour)), "kept-g", "wait a while"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("park of the released goal: %+v %v", res, err)
	}
	tree, err = loadTree(a, acceptedTip(t, a))
	if err != nil {
		t.Fatal(err)
	}
	if e := tree.Live["kept-g"].Episode; e == nil || e.Released != t0.Add(time.Hour).Format(time.RFC3339) {
		t.Fatalf("the seat's park of its released goal dropped the kept episode: %+v", e)
	}
	if res, err := Unpark(landingReq(a, "01J5X00000000000000000NK04", "mac-a", t0.Add(3*time.Hour)), "kept-g"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("unpark: %+v %v", res, err)
	}
	if res, err := Claim(landingReq(a, "01J5X00000000000000000NK05", "mac-a", t0.Add(4*time.Hour)), "kept-g"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("re-claim: %+v %v", res, err)
	}
	tree, err = loadTree(a, acceptedTip(t, a))
	if err != nil {
		t.Fatal(err)
	}
	if c := tree.Live["kept-g"].Claimed; c.AccountingRevision != first.AccountingRevision || c.EpisodeAt != first.EpisodeAt || c.IdleSeconds != 3*3600 {
		t.Fatalf("release-park-unpark-claim reset the box: %+v first=%+v", c, first)
	}
	if res, err := Release(landingReq(a, "01J5X00000000000000000NK06", "mac-a", t0.Add(5*time.Hour)), "kept-g"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("release: %+v %v", res, err)
	}
	// The park a seat's --blocks open records keeps the episode; the return
	// keeps it; the same pair's claim continues it.
	if res, err := Open(landingReq(a, "01J5X00000000000000000NK10", "mac-a", t0.Add(6*time.Hour)), "held-h", "Work a blocker parks.", OriginMain, "Build it."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open held-h: %+v %v", res, err)
	}
	if res, err := claimApprovedForTest(t, landingReq(a, "01J5X00000000000000000NK11", "mac-a", t0.Add(6*time.Hour+time.Minute)), "held-h", budget); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("claim held-h: %+v %v", res, err)
	}
	tree, err = loadTree(a, acceptedTip(t, a))
	if err != nil {
		t.Fatal(err)
	}
	heldFirst := *tree.Live["held-h"].Claimed
	blockAt := t0.Add(7 * time.Hour)
	if res, err := OpenRisked(landingReq(a, "01J5X00000000000000000NK12", "mac-a", blockAt), "fix-h", "The defect that blocks held-h.", OriginMain, "Fix it.", "held-h", risk, 0, "", &budget, nil); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open --blocks: %+v %v", res, err)
	}
	tree, err = loadTree(a, acceptedTip(t, a))
	if err != nil {
		t.Fatal(err)
	}
	if held := tree.Live["held-h"]; held.State != StateParked || held.Episode == nil || held.Episode.Released != blockAt.Format(time.RFC3339) {
		t.Fatalf("the blocker park dropped the seat's episode: state=%s episode=%+v", held.State, held.Episode)
	}
	if res, err := claimApprovedForTest(t, landingReq(a, "01J5X00000000000000000NK13", "mac-a", blockAt.Add(time.Minute)), "fix-h", budget); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("claim the blocker: %+v %v", res, err)
	}
	if res, err := Done(landingReq(a, "01J5X00000000000000000NK14", "mac-a", blockAt.Add(30*time.Minute)), "fix-h", "Fixed."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("done the blocker: %+v %v", res, err)
	}
	if res, err := Claim(landingReq(a, "01J5X00000000000000000NK15", "mac-a", blockAt.Add(time.Hour)), "held-h"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("re-claim the returned goal: %+v %v", res, err)
	}
	tree, err = loadTree(a, acceptedTip(t, a))
	if err != nil {
		t.Fatal(err)
	}
	if c := tree.Live["held-h"].Claimed; c.AccountingRevision != heldFirst.AccountingRevision || c.EpisodeAt != heldFirst.EpisodeAt || c.IdleSeconds != 3600 {
		t.Fatalf("the blocker park and return reset the box: %+v first=%+v", c, heldFirst)
	}
	if res, err := Release(landingReq(a, "01J5X00000000000000000NK16", "mac-a", blockAt.Add(2*time.Hour)), "held-h"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("release held-h: %+v %v", res, err)
	}
	// The arc cascades keep every member's episode; a single claim of a
	// member continues it and the arc release keeps it again.
	arcBed(t, a, "keep-arc", "ka", "NA")
	tree, err = loadTree(a, acceptedTip(t, a))
	if err != nil {
		t.Fatal(err)
	}
	oneFirst := *tree.Live["ka-one"].Claimed
	arcPark := verbReq(a, "01J5X00000000000000000NK20", "mac-a")
	if res, err := ParkArc(arcPark, "ka-one", "pause the arc"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("park --arc: %+v %v", res, err)
	}
	tree, err = loadTree(a, acceptedTip(t, a))
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"ka-one", "ka-two"} {
		if e := tree.Live[id].Episode; e == nil || e.Machine != "mac-a" {
			t.Fatalf("park --arc dropped %s's episode: %+v", id, e)
		}
	}
	if res, err := UnparkArc(verbReq(a, "01J5X00000000000000000NK21", "mac-a"), "ka-one"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("unpark --arc: %+v %v", res, err)
	}
	reclaimOne := verbReq(a, "01J5X00000000000000000NK22", "mac-a")
	reclaimOne.Now = arcPark.Now.Add(30 * time.Minute)
	if res, err := Claim(reclaimOne, "ka-one"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("claim one member: %+v %v", res, err)
	}
	tree, err = loadTree(a, acceptedTip(t, a))
	if err != nil {
		t.Fatal(err)
	}
	if c := tree.Live["ka-one"].Claimed; c.AccountingRevision != oneFirst.AccountingRevision || c.IdleSeconds != 1800 {
		t.Fatalf("park --arc and a member's claim reset the box: %+v first=%+v", c, oneFirst)
	}
	arcRelease := verbReq(a, "01J5X00000000000000000NK23", "mac-a")
	arcRelease.Now = reclaimOne.Now.Add(time.Hour)
	if res, err := ReleaseArc(arcRelease, "ka-one"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("release --arc: %+v %v", res, err)
	}
	tree, err = loadTree(a, acceptedTip(t, a))
	if err != nil {
		t.Fatal(err)
	}
	if e := tree.Live["ka-one"].Episode; e == nil || e.IdleSeconds != 1800 || e.Released != arcRelease.stamp() {
		t.Fatalf("release --arc dropped the continued episode: %+v", e)
	}
}

func TestLandReadyRefusesAFencedClaimAndAFencedLandingClaimKeepsItsSlot(t *testing.T) {
	_, root, _ := twoClones(t)
	seedLedger(t, root)
	budget := Budget{ElapsedLimit: "1m", AttemptLimit: 2, ReservedJobMinutesLimit: 20, ActiveJobLimit: 1}
	fence := func(id, ulidClaim, ulidStop string, at time.Time) time.Time {
		t.Helper()
		claim := verbReq(root, ulidClaim, "mac-a")
		claim.Now = at
		claim.ClaimEpoch = 9
		if res, err := Claim(claim, id); err != nil || res.Outcome != OutcomeConfirmed {
			t.Fatalf("claim %s: %+v %v", id, res, err)
		}
		return at
	}
	stop := func(id, ulid string, at time.Time) {
		t.Helper()
		projection, err := Project(endpointFor(root), true, at)
		if err != nil {
			t.Fatal(err)
		}
		f := projection.Tree.Live[id]
		request := CloseStopRequest{
			VerbRequest: VerbRequest{
				Endpoint: endpointFor(root), Actor: Actor{Machine: "mac-a", Lineage: "goal-stop-custodian"},
				Ulid: ulid, Now: at, ClaimEpoch: 9,
			},
			GoalID: id, StopID: "stop-" + id + "-r" + fmt.Sprint(f.Claimed.Revision) + "-f1", Reason: StopReasonElapsedLimit,
			Capability: *f.StopCapability,
		}
		if res, err := CloseStop(request); err != nil || res.Outcome != OutcomeConfirmed {
			t.Fatalf("breach-stop %s: %+v %v", id, res, err)
		}
	}
	t0 := time.Date(2026, 9, 12, 8, 0, 0, 0, time.UTC)
	for _, id := range []string{"fenced-a", "landing-c", "next-d"} {
		if res, err := Open(landingReq(root, "01J5X00000000000000000NF0"+strings.ToUpper(id[len(id)-1:]), "mac-a", t0), id, "Work called "+id, OriginMain, "Run it."); err != nil || res.Outcome != OutcomeConfirmed {
			t.Fatalf("open %s: %+v %v", id, res, err)
		}
		approveGoalForTest(t, landingReq(root, "01J5X00000000000000000NF1"+strings.ToUpper(id[len(id)-1:]), "mac-a", t0), id, budget)
	}
	// land-ready refuses a fenced claim.
	fence("fenced-a", "01J5X00000000000000000NF20", "", t0.Add(time.Minute))
	stop("fenced-a", "01J5X00000000000000000NF21", t0.Add(3*time.Minute))
	if res, err := LandReady(landingReq(root, "01J5X00000000000000000NF22", "mac-a", t0.Add(4*time.Minute)), "fenced-a"); err != nil || res.Outcome != OutcomeRejected || !strings.Contains(res.Detail, "breach-stopped") {
		t.Fatalf("land-ready of a fenced claim was not refused: %+v %v", res, err)
	}
	// A landing claim that is then fenced still holds the machine's one slot.
	fence("landing-c", "01J5X00000000000000000NF30", "", t0.Add(5*time.Minute))
	if res, err := LandReady(landingReq(root, "01J5X00000000000000000NF31", "mac-a", t0.Add(6*time.Minute)), "landing-c"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("land-ready landing-c: %+v %v", res, err)
	}
	stop("landing-c", "01J5X00000000000000000NF32", t0.Add(8*time.Minute))
	fence("next-d", "01J5X00000000000000000NF40", "", t0.Add(9*time.Minute))
	if res, err := LandReady(landingReq(root, "01J5X00000000000000000NF41", "mac-a", t0.Add(10*time.Minute)), "next-d"); err != nil || res.Outcome != OutcomeRejected || !strings.Contains(res.Detail, "one landing slot per machine") {
		t.Fatalf("a fenced landing claim gave up its slot: %+v %v", res, err)
	}
	// The tree says the same: a fenced landing claim and a landing claim on
	// one machine are two slots.
	fenced := breachStoppedGoalForTest("fenced-l", "mac-a")
	fenced.Landing = &LandingRecord{At: "2026-08-23T01:01:00Z", Opid: "01J5X0000000000000000000B1-mac-a-1a2b3c4d"}
	landing := vGoal("landing-l", StateClaimed)
	landing.Landing = &LandingRecord{At: "2026-08-20T10:06:00Z", Opid: "01J5X0000000000000000000B1-mac-a-1a2b3c4d"}
	tree := &TreeGoals{Root: vRoot(), Live: map[string]*GoalFile{fenced.Id: fenced, landing.Id: landing}, Done: map[string]*GoalFile{}}
	found := false
	for _, problem := range ValidateTree(tree) {
		if strings.Contains(string(problem), "one landing slot per machine") {
			found = true
		}
	}
	if !found {
		t.Fatalf("a fenced landing claim beside a landing claim did not refuse: %v", ValidateTree(tree))
	}
}

func TestReleaseStealAndSetArcClearTheLandingSlot(t *testing.T) {
	_, a, b := twoClones(t)
	seedLedger(t, a)
	budget := testBudget()
	t0 := time.Date(2026, 9, 12, 8, 0, 0, 0, time.UTC)
	enter := func(id, ulidOpen, ulidClaim, ulidLand string, at time.Time) {
		t.Helper()
		if res, err := Open(landingReq(a, ulidOpen, "mac-a", at), id, "Built work "+id, OriginMain, "Land it."); err != nil || res.Outcome != OutcomeConfirmed {
			t.Fatalf("open %s: %+v %v", id, res, err)
		}
		if res, err := claimApprovedForTest(t, landingReq(a, ulidClaim, "mac-a", at.Add(time.Minute)), id, budget); err != nil || res.Outcome != OutcomeConfirmed {
			t.Fatalf("claim %s: %+v %v", id, res, err)
		}
		if res, err := LandReady(landingReq(a, ulidLand, "mac-a", at.Add(2*time.Minute)), id); err != nil || res.Outcome != OutcomeConfirmed {
			t.Fatalf("land-ready %s: %+v %v", id, res, err)
		}
	}
	// Release clears the slot and keeps the episode.
	enter("built-e", "01J5X00000000000000000NR00", "01J5X00000000000000000NR01", "01J5X00000000000000000NR02", t0)
	if res, err := Release(landingReq(a, "01J5X00000000000000000NR03", "mac-a", t0.Add(3*time.Minute)), "built-e"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("release: %+v %v", res, err)
	}
	tree, err := loadTree(a, acceptedTip(t, a))
	if err != nil {
		t.Fatal(err)
	}
	if f := tree.Live["built-e"]; f.Landing != nil || f.Episode == nil {
		t.Fatalf("release did not clear the slot and keep the episode: landing=%+v episode=%+v", f.Landing, f.Episode)
	}
	// A steal clears the slot with the fresh owner's bind.
	enter("built-f", "01J5X00000000000000000NR10", "01J5X00000000000000000NR11", "01J5X00000000000000000NR12", t0.Add(10*time.Minute))
	steal := landingReq(b, "01J5X00000000000000000NR13", "mac-b", t0.Add(13*time.Minute))
	steal.Actor.Human = "Wido"
	if res, err := Steal(steal, "built-f"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("steal: %+v %v", res, err)
	}
	tree, err = loadTree(b, acceptedTip(t, b))
	if err != nil {
		t.Fatal(err)
	}
	if f := tree.Live["built-f"]; f.Landing != nil || f.Claimed == nil || f.Claimed.Machine != "mac-b" || f.Episode != nil {
		t.Fatalf("steal did not clear the slot: landing=%+v claimed=%+v", f.Landing, f.Claimed)
	}
	// set-arc releases the source claim as it moves an arc member to another
	// arc: the slot goes with the claim.
	if res, err := Open(landingReq(a, "01J5X00000000000000000NR20", "mac-a", t0.Add(20*time.Minute)), "built-g", "Built work built-g", OriginMain, "Land it."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open built-g: %+v %v", res, err)
	}
	if res, err := SetArc(landingReq(a, "01J5X00000000000000000NR24", "mac-a", t0.Add(20*time.Minute+30*time.Second)), "built-g", "source-arc"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("set-arc into the source arc: %+v %v", res, err)
	}
	if res, err := claimApprovedForTest(t, landingReq(a, "01J5X00000000000000000NR21", "mac-a", t0.Add(21*time.Minute)), "built-g", budget); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("claim built-g: %+v %v", res, err)
	}
	if res, err := LandReady(landingReq(a, "01J5X00000000000000000NR22", "mac-a", t0.Add(22*time.Minute)), "built-g"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("land-ready built-g: %+v %v", res, err)
	}
	if res, err := SetArc(landingReq(a, "01J5X00000000000000000NR23", "mac-a", t0.Add(23*time.Minute)), "built-g", "moved-arc"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("set-arc: %+v %v", res, err)
	}
	tree, err = loadTree(a, acceptedTip(t, a))
	if err != nil {
		t.Fatal(err)
	}
	if f := tree.Live["built-g"]; f.Landing != nil || f.Arc != "moved-arc" {
		t.Fatalf("set-arc did not drop the slot with the source claim: landing=%+v arc=%q state=%s", f.Landing, f.Arc, f.State)
	}
}

func TestAPersonsBudgetActsAndDoneDropTheKeptEpisode(t *testing.T) {
	_, a, _ := twoClones(t)
	seedLedger(t, a)
	budget := testBudget()
	t0 := time.Date(2026, 9, 12, 8, 0, 0, 0, time.UTC)
	if res, err := Open(landingReq(a, "01J5X00000000000000000ND00", "mac-a", t0), "boxed", "Work whose box a person resets.", OriginMain, "Build it."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", res, err)
	}
	leave := func(ulidClaim, ulidRelease string, at time.Time) {
		t.Helper()
		if res, err := Claim(landingReq(a, ulidClaim, "mac-a", at), "boxed"); err != nil || res.Outcome != OutcomeConfirmed {
			t.Fatalf("claim: %+v %v", res, err)
		}
		if res, err := Release(landingReq(a, ulidRelease, "mac-a", at.Add(time.Minute)), "boxed"); err != nil || res.Outcome != OutcomeConfirmed {
			t.Fatalf("release: %+v %v", res, err)
		}
		tree, err := loadTree(a, acceptedTip(t, a))
		if err != nil {
			t.Fatal(err)
		}
		if tree.Live["boxed"].Episode == nil {
			t.Fatal("the own pair's release kept no episode")
		}
	}
	episodeGone := func(what string) {
		t.Helper()
		tree, err := loadTree(a, acceptedTip(t, a))
		if err != nil {
			t.Fatal(err)
		}
		if f := tree.Live["boxed"]; f != nil && f.Episode != nil {
			t.Fatalf("%s kept the episode: %+v", what, f.Episode)
		}
	}
	approveGoalForTest(t, landingReq(a, "01J5X00000000000000000ND01", "mac-a", t0), "boxed", budget)
	// set-budget acts on claimed work only, where no kept record lives; on
	// the unclaimed goal it points at approve, and the record stays.
	leave("01J5X00000000000000000ND02", "01J5X00000000000000000ND03", t0.Add(time.Minute))
	if res, err := setBudgetApprovedForTest(t, landingReq(a, "01J5X00000000000000000ND04", "mac-a", t0.Add(3*time.Minute)), "boxed", Budget{ElapsedLimit: "6h", AttemptLimit: 6, ReservedJobMinutesLimit: 300, ActiveJobLimit: 2}); err != nil || res.Outcome != OutcomeRejected || !strings.Contains(res.Detail, "goal approve") {
		t.Fatalf("set-budget on unclaimed work: %+v %v", res, err)
	}
	// A person's approve with a tuple.
	leave("01J5X00000000000000000ND05", "01J5X00000000000000000ND06", t0.Add(4*time.Minute))
	human := landingReq(a, "01J5X00000000000000000ND07", "mac-a", t0.Add(6*time.Minute))
	human.Actor.Human = "Wido"
	tuple := Budget{ElapsedLimit: "8h", AttemptLimit: 8, ReservedJobMinutesLimit: 400, ActiveJobLimit: 2}
	if res, err := Approve(human, []string{"boxed"}, &tuple, testHumanAuthority(t, a, human.Now)); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("approve with a tuple: %+v %v", res, err)
	}
	episodeGone("approve with a tuple")
	// Done archives without it.
	leave("01J5X00000000000000000ND08", "01J5X00000000000000000ND09", t0.Add(7*time.Minute))
	if res, err := Done(landingReq(a, "01J5X00000000000000000ND0A", "mac-a", t0.Add(9*time.Minute)), "boxed", "Concluded."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("done: %+v %v", res, err)
	}
	tree, err := loadTree(a, acceptedTip(t, a))
	if err != nil {
		t.Fatal(err)
	}
	if archived := tree.Done["boxed"]; archived == nil || archived.Episode != nil {
		t.Fatalf("done carried the episode into the archive: %+v", archived)
	}
}

func TestLeaveAndResumeEpisodeCarryTheObligationRevision(t *testing.T) {
	f := episodeGolden()
	f.Obligation = &GovernedObligation{Revision: 4}
	leaveEpisode(f, "2026-08-20T03:00:00Z")
	if f.Episode == nil || f.Episode.EpisodeObligationRevision != 4 || f.Episode.AccountingRevision != 5 || f.Episode.EpisodeRevision != 2 {
		t.Fatalf("the kept episode lost the obligation revision: %+v", f.Episode)
	}
	kept := *f.Episode
	// The same pair's claim at revision 6, two hours later.
	f.History = append(f.History, HistoryLine{At: "2026-08-20T05:00:00Z", Opid: "01J5X0000000000000000000C6-mac-studio-1a2b3c4d", Verb: "claim", Actor: "mac-studio+lin-1", Targets: []string{f.Id}, Keep: -1})
	f.Revision = 6
	f.Obligation = nil
	f.Claimed = newClaimRecord(kept.Machine, kept.Lineage, "2026-08-20T05:00:00Z", 6)
	f.Claimed.EpisodeAt, f.Claimed.EpisodeRevision = "2026-08-20T05:00:00Z", 6
	if err := resumeEpisode(f, kept, "2026-08-20T05:00:00Z"); err != nil {
		t.Fatal(err)
	}
	if c := f.Claimed; c.EpisodeObligationRevision != 4 || c.AccountingRevision != 5 || c.EpisodeRevision != 2 || c.IdleSeconds != 7200 || f.Episode != nil {
		t.Fatalf("the re-claim did not restore the obligation revision with the gap idle: %+v", c)
	}
	// A regressed clock refuses the continuation instead of inventing time.
	if err := resumeEpisode(f, kept, "2026-08-20T02:00:00Z"); err == nil || !strings.Contains(err.Error(), "CLOCK_REGRESSED") {
		t.Fatalf("a re-claim before the release did not refuse: %v", err)
	}
	// A leave whose binding the history cannot vouch for keeps nothing.
	unvouched := episodeGolden()
	unvouched.Claimed.EpisodeAt = "2026-08-20T00:59:00Z"
	leaveEpisode(unvouched, "2026-08-20T03:00:00Z")
	if unvouched.Episode != nil {
		t.Fatalf("an unvouched episode binding was kept: %+v", unvouched.Episode)
	}
}
