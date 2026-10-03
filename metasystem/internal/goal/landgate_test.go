package goal

// The landing gate (g1-s70 D2, D3, D4, §8): the gate reads the history alone —
// the human's word at the current tip, a standing sitting, the tier — and the
// clock starts at the Landing record, restarts at every human act after it,
// never runs under a hold and never at or above the tier.

import (
	"errors"
	"strings"
	"testing"
	"time"
)

var gateSettings = GateSettings{HumanFromTier: 2, AutoAfter: 4 * time.Hour, AutoAfterText: "4h"}

func tiered(id string, tier uint8) *GoalFile {
	f := underReview(id)
	f.Tier = tier
	return f
}

func humanLine(f *GoalFile, at, opid, verb, reason string) {
	f.History = append(f.History, HistoryLine{At: at, Opid: opid, Verb: verb, Actor: "human:Wido", Targets: []string{f.Id}, Keep: -1, Reason: reason})
	f.Revision = uint64(len(f.History))
}

func gateCode(err error) string {
	var refusal *GateRefusal
	if errors.As(err, &refusal) {
		return refusal.Code
	}
	return ""
}

func TestGateRefusesAtAndAboveTheTierWithoutTheHumansWord(t *testing.T) {
	t.Parallel()
	for _, tier := range []uint8{2, 3, 0} {
		_, err := Gate(tiered("g", tier), reviewedTip, gateSettings)
		if gateCode(err) != GateWaitsForHuman || !strings.Contains(err.Error(), "waits for a person") {
			t.Errorf("tier %d without the word: %v", tier, err)
		}
	}
	under, err := Gate(tiered("g", 1), reviewedTip, gateSettings)
	if err != nil || under != "landing.review.auto-after=4h, tier 1 below human-from-tier=2" {
		t.Fatalf("below the tier the landing proceeds: %q %v", under, err)
	}
	// A threshold of 1 makes every tier wait.
	one := gateSettings
	one.HumanFromTier = 1
	if _, err := Gate(tiered("g", 1), reviewedTip, one); gateCode(err) != GateWaitsForHuman {
		t.Fatalf("a threshold of 1 let tier 1 through: %v", err)
	}
}

func TestGateProceedsWithTheWordAtTheCurrentTipAndRefusesAnotherTip(t *testing.T) {
	t.Parallel()
	cleared := tiered("g", 2)
	humanLine(cleared, "2026-08-20T11:00:00Z", "01J5X0000000000000000000R1-mac-ui-1a2b3c4d", "review",
		"reviewed verdict=clear-to-land tip="+reviewedTip+" record="+reviewPath+" by=Wido")
	under, err := Gate(cleared, reviewedTip, gateSettings)
	if err != nil || !strings.Contains(under, "reviewed verdict=clear-to-land by=Wido") {
		t.Fatalf("a clear-to-land at the tip: %q %v", under, err)
	}
	if _, err := Gate(cleared, movedTip, gateSettings); gateCode(err) != GateWaitsForHuman || !strings.Contains(err.Error(), "a moved tip needs the word again") {
		t.Fatalf("a verdict at another tip let the landing through: %v", err)
	}
	if _, err := Gate(cleared, "", gateSettings); gateCode(err) != GateWaitsForHuman {
		t.Fatalf("a landing with no tip passed on a word: %v", err)
	}

	decided := tiered("g", 3)
	humanLine(decided, "2026-08-20T11:00:00Z", "01J5X0000000000000000000R2-mac-ui-1a2b3c4d", LandWithoutSittingVerb,
		"landed-without-sitting tip="+reviewedTip+" by=Wido because=one-line doc fix, read the diff on the card")
	if under, err := Gate(decided, reviewedTip, gateSettings); err != nil || !strings.Contains(under, "landed-without-sitting by=Wido") {
		t.Fatalf("a land-without-sitting at the tip: %q %v", under, err)
	}
	if _, err := Gate(decided, movedTip, gateSettings); gateCode(err) != GateWaitsForHuman {
		t.Fatalf("a decision at another tip let the landing through: %v", err)
	}

	// A send-back after the clearance is the newest word.
	humanLine(cleared, "2026-08-20T12:00:00Z", "01J5X0000000000000000000R3-mac-ui-1a2b3c4d", "review",
		"reviewed verdict=send-back tip="+reviewedTip+" record="+reviewPath+" by=Wido brief="+BriefPathFor(reviewPath))
	if _, err := Gate(cleared, reviewedTip, gateSettings); gateCode(err) != GateWaitsForHuman || !strings.Contains(err.Error(), "send-back") {
		t.Fatalf("a send-back newer than the clearance let it land: %v", err)
	}

	// An exception recorded earlier is not the word.
	excepted := tiered("g", 2)
	excepted.History = append(excepted.History, HistoryLine{At: "2026-08-20T11:00:00Z", Opid: "01J5X0000000000000000000R4-mac-a-1a2b3c4d",
		Verb: "carry", Actor: "human:Wido", Targets: []string{"g"}, Keep: -1, Reason: "open"})
	if _, err := Gate(excepted, reviewedTip, gateSettings); gateCode(err) != GateWaitsForHuman {
		t.Fatalf("an exception passed for the word: %v", err)
	}
}

func TestGateRefusesUnderAHoldAtEveryTierFromTheHistoryAlone(t *testing.T) {
	t.Parallel()
	for _, tier := range []uint8{1, 2, 3} {
		f := tiered("g", tier)
		humanLine(f, "2026-08-20T11:00:00Z", "01J5X0000000000000000000H1-mac-ui-1a2b3c4d", "review", SittingReason(true, reviewPath, "Wido"))
		if tier >= 2 {
			humanLine(f, "2026-08-20T11:01:00Z", "01J5X0000000000000000000H2-mac-ui-1a2b3c4d", LandWithoutSittingVerb,
				"landed-without-sitting tip="+reviewedTip+" by=Ann because=fine")
		}
		if _, err := Gate(f, reviewedTip, gateSettings); gateCode(err) != GateHeldBySitting || !strings.Contains(err.Error(), "Wido's review sitting") {
			t.Errorf("tier %d under a hold: %v", tier, err)
		}
	}
	f := tiered("g", 1)
	humanLine(f, "2026-08-20T11:00:00Z", "01J5X0000000000000000000H1-mac-ui-1a2b3c4d", "review", SittingReason(true, reviewPath, "Wido"))
	humanLine(f, "2026-08-20T11:30:00Z", "01J5X0000000000000000000H3-mac-ui-1a2b3c4d", "review", SittingReason(false, reviewPath, "Wido"))
	if _, err := Gate(f, reviewedTip, gateSettings); err != nil {
		t.Fatalf("a released hold still holds: %v", err)
	}
	// The same human's verdict releases in the same line.
	v := tiered("g", 2)
	humanLine(v, "2026-08-20T11:00:00Z", "01J5X0000000000000000000H1-mac-ui-1a2b3c4d", "review", SittingReason(true, reviewPath, "Wido"))
	humanLine(v, "2026-08-20T11:30:00Z", "01J5X0000000000000000000H4-mac-ui-1a2b3c4d", "review",
		"reviewed verdict=clear-to-land tip="+reviewedTip+" record="+reviewPath+" by=Wido")
	if holds := HoldsOf(v); len(holds) != 0 {
		t.Fatalf("the verdict did not release its human's hold: %+v", holds)
	}
	if _, err := Gate(v, reviewedTip, gateSettings); err != nil {
		t.Fatalf("clear to land after the sitting: %v", err)
	}
	// Another human's verdict does not release Wido's hold.
	other := tiered("g", 1)
	humanLine(other, "2026-08-20T11:00:00Z", "01J5X0000000000000000000H1-mac-ui-1a2b3c4d", "review", SittingReason(true, reviewPath, "Wido"))
	other.History = append(other.History, HistoryLine{At: "2026-08-20T11:10:00Z", Opid: "01J5X0000000000000000000H5-mac-ui-1a2b3c4d", Verb: "review", Actor: "human:Ann",
		Targets: []string{"g"}, Keep: -1, Reason: "review-sitting released record=" + reviewPath + " by=Ann"})
	if holds := HoldsOf(other); len(holds) != 1 || holds[0].By != "Wido" {
		t.Fatalf("another human released Wido's hold: %+v", holds)
	}
}

func TestTheClockStartsAtTheLandingAndRestartsAtEveryHumanAct(t *testing.T) {
	t.Parallel()
	landedAt := time.Date(2026, 8, 20, 10, 6, 0, 0, time.UTC)
	f := tiered("g", 1)
	read := ReadGate(f, gateSettings, landedAt.Add(3*time.Hour))
	if !read.AutoLandsAt.Equal(landedAt.Add(4*time.Hour)) || read.Eligible {
		t.Fatalf("the clock from At: %+v", read)
	}
	if read = ReadGate(f, gateSettings, landedAt.Add(4*time.Hour)); !read.Eligible {
		t.Fatalf("the grace time passed and the goal is not eligible: %+v", read)
	}
	// A priority change by the human restarts it.
	humanLine(f, "2026-08-20T12:00:00Z", "01J5X0000000000000000000C1-mac-ui-1a2b3c4d", "set-priority", "")
	read = ReadGate(f, gateSettings, landedAt.Add(4*time.Hour))
	if read.Eligible || !read.AutoLandsAt.Equal(time.Date(2026, 8, 20, 16, 0, 0, 0, time.UTC)) {
		t.Fatalf("a human act did not restart the clock: %+v", read)
	}
	// The seat's own lines do not.
	f.History = append(f.History, HistoryLine{At: "2026-08-20T13:00:00Z", Opid: "01J5X0000000000000000000C2-mac-a-1a2b3c4d", Verb: "send-back", Actor: "mac-a+lin-1", Targets: []string{"g"}, Keep: -1})
	if read = ReadGate(f, gateSettings, landedAt); !read.AutoLandsAt.Equal(time.Date(2026, 8, 20, 16, 0, 0, 0, time.UTC)) {
		t.Fatalf("a seat's line restarted the clock: %+v", read)
	}
	// Under a hold there is no clock; the release restarts it.
	humanLine(f, "2026-08-20T14:00:00Z", "01J5X0000000000000000000C3-mac-ui-1a2b3c4d", "review", SittingReason(true, reviewPath, "Wido"))
	if read = ReadGate(f, gateSettings, landedAt.Add(24*time.Hour)); !read.AutoLandsAt.IsZero() || read.Eligible || len(read.HeldBy) != 1 {
		t.Fatalf("the clock ran under a hold: %+v", read)
	}
	humanLine(f, "2026-08-20T15:00:00Z", "01J5X0000000000000000000C4-mac-ui-1a2b3c4d", "review", SittingReason(false, reviewPath, "Wido"))
	if read = ReadGate(f, gateSettings, landedAt); !read.AutoLandsAt.Equal(time.Date(2026, 8, 20, 19, 0, 0, 0, time.UTC)) {
		t.Fatalf("the release did not restart the clock: %+v", read)
	}
	// Never at or above the tier.
	if read = ReadGate(tiered("g", 2), gateSettings, landedAt.Add(100*time.Hour)); !read.AutoLandsAt.IsZero() || read.Eligible || !read.WaitsForHuman {
		t.Fatalf("the clock ran at the tier: %+v", read)
	}
}

// A landing is due to its holder when the clock or the human's word says it is
// worth taking; the gate the landing meets then decides whether it lands, so a
// held goal with a word is taken and refused, its refusal shown (SOL-S70-02).
func TestLandingIsDueToTheHolderWhenTheClockOrTheWordSaysSo(t *testing.T) {
	t.Parallel()
	landedAt := time.Date(2026, 8, 20, 10, 6, 0, 0, time.UTC)
	f := tiered("g", 1)
	if due, _ := LandingDue(f, gateSettings, landedAt.Add(time.Hour)); due {
		t.Fatal("due before the grace time")
	}
	due, why := LandingDue(f, gateSettings, landedAt.Add(4*time.Hour))
	if !due || why != "eligible under landing.review.auto-after=4h, tier 1 below human-from-tier=2" {
		t.Fatalf("due after the grace time: %v %q", due, why)
	}
	f.History = append(f.History, HistoryLine{At: "2026-08-20T15:00:00Z", Opid: "01J5X0000000000000000000D1-mac-a-1a2b3c4d", Verb: LandedVerb, Actor: "mac-a+lin-1",
		Targets: []string{"g"}, Keep: -1, Reason: "landed under " + why})
	if due, _ := LandingDue(f, gateSettings, landedAt.Add(8*time.Hour)); due {
		t.Fatal("a landed goal is due again")
	}

	above := tiered("g", 2)
	if due, _ := LandingDue(above, gateSettings, landedAt.Add(100*time.Hour)); due {
		t.Fatal("a goal at the tier with no word is due")
	}
	humanLine(above, "2026-08-20T11:00:00Z", "01J5X0000000000000000000D2-mac-ui-1a2b3c4d", "review",
		"reviewed verdict=clear-to-land tip="+reviewedTip+" record="+reviewPath+" by=Wido")
	if due, why := LandingDue(above, gateSettings, landedAt); !due || !strings.Contains(why, "cleared to land by Wido") {
		t.Fatalf("a cleared goal is not due: %v %q", due, why)
	}
	humanLine(above, "2026-08-20T11:10:00Z", "01J5X0000000000000000000D3-mac-ui-1a2b3c4d", "review", SittingReason(true, reviewPath, "Wido"))
	if due, _ := LandingDue(above, gateSettings, landedAt); !due {
		t.Fatal("a held goal with the word is not taken, so its refusal is never shown")
	}
	held := tiered("g", 1)
	humanLine(held, "2026-08-20T11:10:00Z", "01J5X0000000000000000000D4-mac-ui-1a2b3c4d", "review", SittingReason(true, reviewPath, "Wido"))
	if due, _ := LandingDue(held, gateSettings, landedAt.Add(100*time.Hour)); due {
		t.Fatal("the clock ran under a hold below the tier")
	}
	notLanding := vGoal("h", StateClaimed)
	notLanding.Tier = 1
	if due, _ := LandingDue(notLanding, gateSettings, landedAt.Add(100*time.Hour)); due {
		t.Fatal("a claim with no Landing record is due")
	}
}

func TestSittingHoldAndReleaseAreTheHumansLinesOnTheLedger(t *testing.T) {
	t.Parallel()
	endpoint := reviewBed(t, tiered("under-review", 1))
	request, proof := reviewer(t, endpoint, 7, 1)
	held, err := Sitting(request, "under-review", reviewPath, true, proof)
	if err != nil || held.Outcome != OutcomeConfirmed {
		t.Fatalf("hold: %+v %v", held, err)
	}
	tree, _ := loadTreeFor(endpoint, held.Tip)
	f := tree.Live["under-review"]
	last := f.History[len(f.History)-1]
	if last.Verb != "review" || last.Actor != "human:Wido" || last.Reason != "review-sitting opened record="+reviewPath+" by=Wido" {
		t.Fatalf("the hold line = %+v", last)
	}
	if parsed, problems := ParseFile(RenderFile(f)); parsed == nil || len(problems) != 0 {
		t.Fatalf("the held goal does not read back clean: %v", problems)
	}
	if _, err := Gate(f, reviewedTip, gateSettings); gateCode(err) != GateHeldBySitting {
		t.Fatalf("the ledger's hold does not hold: %v", err)
	}
	again, proof := reviewer(t, endpoint, 7, 2)
	if repeat, err := Sitting(again, "under-review", reviewPath, true, proof); err != nil || !repeat.Unchanged {
		t.Fatalf("a second hold is not a repeat: %+v %v", repeat, err)
	}
	release, proof := reviewer(t, endpoint, 7, 3)
	if released, err := Sitting(release, "under-review", reviewPath, false, proof); err != nil || released.Outcome != OutcomeConfirmed {
		t.Fatalf("release: %+v %v", released, err)
	}
	tree, _ = loadTreeFor(endpoint, acceptedTipForEndpoint(t, endpoint))
	if holds := HoldsOf(tree.Live["under-review"]); len(holds) != 0 {
		t.Fatalf("the release left a hold: %+v", holds)
	}
	release2, proof := reviewer(t, endpoint, 7, 4)
	if repeat, err := Sitting(release2, "under-review", reviewPath, false, proof); err != nil || !repeat.Unchanged {
		t.Fatalf("a second release is not a repeat: %+v %v", repeat, err)
	}
	seat := verbReqFor(endpoint, reviewUlid(7, 5), "mac-a")
	if _, err := Sitting(seat, "under-review", reviewPath, true, proof); err == nil {
		t.Fatal("a seat opened a sitting")
	}
	human, proof := reviewer(t, endpoint, 7, 6)
	if _, err := Sitting(human, "under-review", "plans/designs/x.md", true, proof); err == nil {
		t.Fatal("a sitting named a record outside the review home")
	}
}

func TestLandWithoutSittingIsTheHumansDecisionWithItsReasonAtTheTip(t *testing.T) {
	t.Parallel()
	endpoint := reviewBed(t, tiered("under-review", 2))
	request, proof := reviewer(t, endpoint, 8, 1)
	if _, err := LandWithoutSitting(request, "under-review", reviewedTip, "  ", proof); err == nil || !strings.Contains(err.Error(), "carries your reason") {
		t.Fatalf("a decision without a reason: %v", err)
	}
	if _, err := LandWithoutSitting(request, "under-review", "", "fine", proof); err == nil {
		t.Fatal("a decision with no tip")
	}
	if _, err := LandWithoutSitting(request, "under-review", reviewedTip, "two\nlines", proof); err == nil {
		t.Fatal("a reason of two lines")
	}
	decided, err := LandWithoutSitting(request, "under-review", reviewedTip, "one-line doc fix, read the diff on the card", proof)
	if err != nil || decided.Outcome != OutcomeConfirmed {
		t.Fatalf("the decision: %+v %v", decided, err)
	}
	tree, _ := loadTreeFor(endpoint, decided.Tip)
	f := tree.Live["under-review"]
	last := f.History[len(f.History)-1]
	want := "landed-without-sitting tip=" + reviewedTip + " by=Wido because=one-line doc fix, read the diff on the card"
	if last.Verb != LandWithoutSittingVerb || last.Actor != "human:Wido" || last.Reason != want {
		t.Fatalf("the decision line = %+v, want %q", last, want)
	}
	assertSessionLine(t, last)
	if parsed, problems := ParseFile(RenderFile(f)); parsed == nil || len(problems) != 0 {
		t.Fatalf("the decided goal does not read back clean: %v", problems)
	}
	if _, err := Gate(f, reviewedTip, gateSettings); err != nil {
		t.Fatalf("the decision does not open the gate at its tip: %v", err)
	}
	again, proof := reviewer(t, endpoint, 8, 2)
	if repeat, err := LandWithoutSitting(again, "under-review", reviewedTip, "same", proof); err != nil || !repeat.Unchanged {
		t.Fatalf("a second decision at the tip is not a repeat: %+v %v", repeat, err)
	}
	seat := verbReqFor(endpoint, reviewUlid(8, 3), "mac-a")
	if _, err := LandWithoutSitting(seat, "under-review", reviewedTip, "fine", proof); err == nil {
		t.Fatal("a seat decided to land without a sitting")
	}
}

func TestTheLandedLineIsTheHoldersAndWrittenOnce(t *testing.T) {
	t.Parallel()
	endpoint := reviewBed(t, tiered("under-review", 1))
	holder := verbReqFor(endpoint, reviewUlid(9, 1), "mac-a")
	holder.Now = reviewNow
	under := "landing.review.auto-after=4h, tier 1 below human-from-tier=2"
	result, err := RecordLanded(holder, "under-review", under)
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("landed: %+v %v", result, err)
	}
	tree, _ := loadTreeFor(endpoint, result.Tip)
	f := tree.Live["under-review"]
	last := f.History[len(f.History)-1]
	if last.Verb != LandedVerb || last.Reason != "landed under "+under {
		t.Fatalf("the landed line = %+v", last)
	}
	if parsed, problems := ParseFile(RenderFile(f)); parsed == nil || len(problems) != 0 {
		t.Fatalf("the landed goal does not read back clean: %v", problems)
	}
	holder.Ulid = reviewUlid(9, 2)
	if repeat, err := RecordLanded(holder, "under-review", under); err != nil || !repeat.Unchanged {
		t.Fatalf("a second landed line is not a repeat: %+v %v", repeat, err)
	}
	stranger := verbReqFor(endpoint, reviewUlid(9, 3), "mac-b")
	if result, _ := RecordLanded(stranger, "under-review", under); result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "not this seat's claim") {
		t.Fatalf("another seat recorded the holder's landing: %+v", result)
	}
	human := holder
	human.Actor.Human = "Wido"
	if _, err := RecordLanded(human, "under-review", under); err == nil {
		t.Fatal("a human wrote the holder's landed line")
	}
}

func TestGateHistoryGrammar(t *testing.T) {
	t.Parallel()
	rows := []struct {
		line string
		want string
	}{
		{"- 2026-08-20T11:00:00Z 01J5X0000000000000000000R1-mac-ui-1a2b3c4d land-without-sitting actor=mac-a+lin-1 targets=g reason=landed-without-sitting tip=" + reviewedTip + " by=Wido because=fine", "human's act"},
		{"- 2026-08-20T11:00:00Z 01J5X0000000000000000000R1-mac-ui-1a2b3c4d land-without-sitting actor=human:Wido targets=g reason=landed-without-sitting tip=" + reviewedTip + " by=Wido", "human's reason"},
		{"- 2026-08-20T11:00:00Z 01J5X0000000000000000000R1-mac-ui-1a2b3c4d land-without-sitting actor=human:Wido targets=g reason=landed-without-sitting tip=abc by=Wido because=fine", "names the tip"},
		{"- 2026-08-20T11:00:00Z 01J5X0000000000000000000R1-mac-ui-1a2b3c4d landed actor=human:Wido targets=g reason=landed under x", "holder's line"},
		{"- 2026-08-20T11:00:00Z 01J5X0000000000000000000R1-mac-a-1a2b3c4d landed actor=mac-a+lin-1 targets=g reason=done", "names what it landed under"},
		{"- 2026-08-20T11:00:00Z 01J5X0000000000000000000R1-mac-ui-1a2b3c4d review actor=human:Wido targets=g reason=review-sitting opened record=plans/x.md by=Wido", "review history"},
	}
	for _, row := range rows {
		if _, err := ParseHistoryLine(row.line); err == nil || !strings.Contains(err.Error(), row.want) {
			t.Errorf("%s\n  refusal = %v, want %q", row.line, err, row.want)
		}
	}
	for _, good := range []string{
		"- 2026-08-20T11:00:00Z 01J5X0000000000000000000R1-mac-ui-1a2b3c4d review actor=human:Wido targets=g reason=review-sitting opened record=" + reviewPath + " by=Wido",
		"- 2026-08-20T11:00:00Z 01J5X0000000000000000000R1-mac-ui-1a2b3c4d land-without-sitting actor=human:Wido targets=g reason=landed-without-sitting tip=" + reviewedTip + " by=Wido because=one-line fix",
	} {
		h, err := ParseHistoryLine(good)
		if err != nil || RenderHistoryLine(h) != good {
			t.Fatalf("a good line does not round-trip: %v\n%s", err, RenderHistoryLine(h))
		}
	}
}

func TestLandedUnderNamesTheSettingOrTheWord(t *testing.T) {
	t.Parallel()
	if under := LandedUnder(tiered("g", 1), gateSettings); under != "landing.review.auto-after=4h, tier 1 below human-from-tier=2" {
		t.Fatalf("below the tier: %q", under)
	}
	f := tiered("g", 2)
	humanLine(f, "2026-08-20T11:00:00Z", "01J5X0000000000000000000L1-mac-ui-1a2b3c4d", "review",
		"reviewed verdict=clear-to-land tip="+reviewedTip+" record="+reviewPath+" by=Wido")
	if under := LandedUnder(f, gateSettings); !strings.HasPrefix(under, "reviewed verdict=clear-to-land by=Wido tip=9c1f0a2") {
		t.Fatalf("at the tier: %q", under)
	}
}

func TestReviewRecordPathIsInTheReviewHome(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for given, want := range map[string]string{
		"plans/reviews/review-of-g.md":              "plans/reviews/review-of-g.md",
		root + "/plans/reviews/review-of-g.md":      "plans/reviews/review-of-g.md",
		"plans/reviews/../reviews/review-of-g.md":   "plans/reviews/review-of-g.md",
		"plans/designs/g.md":                        "",
		"plans/reviews/review-of-g.brief.md":        "",
		"":                                          "",
		"../elsewhere/plans/reviews/review-of-g.md": "",
	} {
		got, err := ReviewRecordPath(root, given)
		if got != want || (want == "") != (err != nil) {
			t.Errorf("%q: %q %v, want %q", given, got, err, want)
		}
	}
}

// A tier-1 goal stores zero review rounds in its box (R-54-m1), so its units
// land without a read; a budget with rounds, or none recorded, needs them.
func TestReadsWaivedReadsTheReviewRoundLimit(t *testing.T) {
	t.Parallel()
	tierOne := &GoalFile{Tier: 1, Budget: &Budget{ReviewRoundLimit: 0}}
	tierTwo := &GoalFile{Tier: 2, Budget: &Budget{ReviewRoundLimit: 2}}
	if !ReadsWaived(tierOne) || !ReadsWaived(&GoalFile{Tier: 2, Budget: &Budget{ReviewRoundLimit: 0}}) {
		t.Fatal("a zero review-round budget must waive the reads")
	}
	if ReadsWaived(tierTwo) || ReadsWaived(&GoalFile{Tier: 2}) || ReadsWaived(nil) {
		t.Fatal("a budget with review rounds, or none recorded, needs its reads")
	}
}

// No critic may read a tier-1 goal, so its units land unread whatever its box
// says: a goal approved at tier 2 and lowered to tier 1 keeps a box with
// review rounds, and needing a read there would leave it no way to land.
func TestReadsWaivedForEveryTierOneGoal(t *testing.T) {
	t.Parallel()
	for _, file := range []*GoalFile{{Tier: 1, Budget: &Budget{ReviewRoundLimit: 20}}, {Tier: 1}} {
		if !ReadsWaived(file) {
			t.Fatalf("tier-1 goal with budget %+v needs a read no critic may give", file.Budget)
		}
	}
}
