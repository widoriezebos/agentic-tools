package goal

// The verdict on the ledger (g1-s69 D1, D2, §8): goal review writes one history
// line with the tip the record's own head names, lands the record in the same
// commit, publishes create-only, and a send-back leaves the Landing record as
// it stands so a holder with a second claim is still a valid ledger.

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
)

var reviewNow = time.Date(2026, 9, 29, 9, 0, 0, 0, time.UTC)

const (
	reviewedTip = "9c1f0a2b3c4d5e6f708192a3b4c5d6e7f8091a2b"
	movedTip    = "4d2e7b1c3d4e5f60718293a4b5c6d7e8f9a0b1c2"
	reviewPath  = "plans/reviews/review-of-under-review.md"
	landingOpid = "01J5X0000000000000000000M1-mac-a-1a2b3c4d"
)

func reviewUlid(index, step int) string {
	return fmt.Sprintf("01J5X0000000000000000RV%d%02d", index, step)
}

// underReview is a goal waiting to land: claimed by mac-a+lin-1, with the
// land-ready line its Landing record names.
func underReview(id string) *GoalFile {
	f := vGoal(id, StateClaimed)
	f.History = append(f.History, HistoryLine{
		At: "2026-08-20T10:06:00Z", Opid: landingOpid, Verb: "land-ready", Actor: "mac-a+lin-1", Targets: []string{id}, Keep: -1,
	})
	f.Revision = uint64(len(f.History))
	f.Landing = &LandingRecord{At: "2026-08-20T10:06:00Z", Opid: landingOpid}
	return f
}

func reviewRecord(goals, verdict, reviewedAt string) []byte {
	return []byte("# Review of " + goals + "\n\n- Kind: review\n- Id: 01M3MP8CZYPATTR0382JS6HMFA\n- Status: draft\n" +
		"- Goals: " + goals + "\n- Reviewed: " + reviewedTip + " (the tip of goal/" + goals + ")\n\n" +
		"## Facts\n\n## Findings\n\n- The owner reads the wrong tree · 2026-09-29 · Wido — Anchor: internal/owner.go:60 — Answer: fix\n\n" +
		"## Decisions\n\n## Open questions\n\n## Drawings\n\n## Outcome\n\n" +
		"Verdict: " + verdict + "\n\nReviewed at: " + reviewedAt + "\n\nExamined: internal/owner.go:60\n\n" +
		"- Recorded from the sitting · 2026-09-29 · Wido [d:deposit:t3#0]\n")
}

func reviewBed(t *testing.T, live ...*GoalFile) Endpoint {
	t.Helper()
	endpoint, _ := fakeGoalEndpoint(t, live...)
	return endpoint
}

// reviewer is the act layer's request: this human, under a session proof.
func reviewer(t *testing.T, endpoint Endpoint, index, step int) (VerbRequest, *humanauthority.Proof) {
	t.Helper()
	request := verbReqFor(endpoint, reviewUlid(index, step), "mac-ui")
	request.Actor.Human = "Wido"
	request.Actor.Lineage = "browser-session"
	request.Now = reviewNow
	proof := sessionProofForTest(t, endpoint.Root, request.Now)
	request.Authority = proof
	return request, proof
}

func clearAct() ReviewAct {
	return ReviewAct{Record: reviewPath, Content: reviewRecord("under-review", "clear to land", reviewedTip), Verdict: VerdictClearToLand}
}

func TestGoalReviewWritesTheVerdictLineWithTheHeadsTipAndLandsTheRecord(t *testing.T) {
	t.Parallel()
	endpoint := reviewBed(t, underReview("under-review"))
	request, proof := reviewer(t, endpoint, 1, 1)
	result, err := Review(request, "under-review", clearAct(), proof)
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("clear to land: %+v %v", result, err)
	}
	tree, err := loadTreeFor(endpoint, result.Tip)
	if err != nil {
		t.Fatal(err)
	}
	f := tree.Live["under-review"]
	last := f.History[len(f.History)-1]
	want := "reviewed verdict=clear-to-land tip=" + reviewedTip + " record=" + reviewPath + " by=Wido"
	if last.Verb != "review" || last.Actor != "human:Wido" || last.Reason != want {
		t.Fatalf("the verdict line = %+v, want reason %q", last, want)
	}
	assertSessionLine(t, last)
	if f.Landing == nil || f.Landing.Opid != landingOpid {
		t.Fatalf("clear to land moved the Landing record: %+v", f.Landing)
	}
	files, err := readCommitFiles(endpoint, result.Tip, reviewPath)
	if err != nil || string(files[reviewPath]) != string(clearAct().Content) {
		t.Fatalf("the record did not land with the ledger commit: %q %v", files[reviewPath], err)
	}
	if parsed, problems := ParseFile(RenderFile(f)); parsed == nil || len(problems) != 0 {
		t.Fatalf("the goal does not read back clean: %v", problems)
	}
	read := VerdictsOf(f)
	if read.Latest == nil || read.Latest.Tip != reviewedTip || read.SentBack {
		t.Fatalf("the verdict does not read back: %+v", read)
	}
}

func TestGoalReviewRefusesARecordThatIsNotThisGoalsReview(t *testing.T) {
	t.Parallel()
	endpoint := reviewBed(t, underReview("under-review"), underReview("other-goal"))
	rows := []struct {
		name string
		act  ReviewAct
		id   string
		want string
	}{
		{"a review of A offered for B", clearAct(), "other-goal", "is a review of under-review, not of other-goal"},
		{"an Outcome without the verdict", ReviewAct{Record: reviewPath, Content: reviewRecord("under-review", "send back", reviewedTip), Verdict: VerdictClearToLand}, "under-review", `does not open with "Verdict: clear to land"`},
		{"not in its home", ReviewAct{Record: "plans/designs/review-of-under-review.md", Content: clearAct().Content, Verdict: VerdictClearToLand}, "under-review", "is not a review record in its home"},
		{"a parent step out of the home", ReviewAct{Record: "plans/reviews/../goals/x.md", Content: clearAct().Content, Verdict: VerdictClearToLand}, "under-review", "is not a review record in its home"},
		{"an Outcome drafted for another tip", ReviewAct{Record: reviewPath, Content: reviewRecord("under-review", "clear to land", movedTip), Verdict: VerdictClearToLand}, "under-review", "the branch was retipped since; press End again"},
		{"an Outcome with no Reviewed at", ReviewAct{Record: reviewPath, Content: reviewRecord("under-review", "clear to land", ""), Verdict: VerdictClearToLand}, "under-review", "drafted for no tip"},
		{"no verdict", ReviewAct{Record: reviewPath, Content: clearAct().Content, Verdict: "no-verdict"}, "under-review", "records clear-to-land or send-back"},
		{"a send-back with no brief", ReviewAct{Record: reviewPath, Content: reviewRecord("under-review", "send back", reviewedTip), Verdict: VerdictSendBack}, "under-review", "mark at least one finding fix"},
	}
	for index, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			request, proof := reviewer(t, endpoint, 2, index+1)
			result, err := Review(request, row.id, row.act, proof)
			if err == nil || !strings.Contains(err.Error(), row.want) {
				t.Fatalf("refusal = %v (%+v), want %q", err, result, row.want)
			}
		})
	}
	request, _ := reviewer(t, endpoint, 2, 20)
	if _, err := Review(request, "under-review", clearAct(), nil); err == nil || !strings.Contains(err.Error(), "signed-in browser session") {
		t.Fatalf("no proof: %v", err)
	}
	request.Actor.Human = ""
	if _, err := Review(request, "under-review", clearAct(), sessionProofForTest(t, endpoint.Root, reviewNow)); err == nil || !strings.Contains(err.Error(), "human act") {
		t.Fatalf("no human: %v", err)
	}
}

func TestGoalReviewPublishesCreateOnly(t *testing.T) {
	t.Parallel()
	endpoint := reviewBed(t, underReview("under-review"))
	request, proof := reviewer(t, endpoint, 3, 1)
	if result, err := Review(request, "under-review", clearAct(), proof); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("first verdict: %+v %v", result, err)
	}
	// The same act again: the same bytes are a replay and nothing is written.
	again, proof := reviewer(t, endpoint, 3, 2)
	result, err := Review(again, "under-review", clearAct(), proof)
	if err != nil || !result.Unchanged || !strings.Contains(result.Detail, "already carries this verdict") {
		t.Fatalf("the replay is not a repeat: %+v %v", result, err)
	}
	// Another review's words at the same path are refused.
	other := clearAct()
	other.Content = []byte(strings.Replace(string(other.Content), "Examined: internal/owner.go:60", "Examined: something else", 1))
	third, proof := reviewer(t, endpoint, 3, 3)
	result, err = Review(third, "under-review", other, proof)
	if err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "never overwritten") {
		t.Fatalf("different bytes at the path: %+v %v", result, err)
	}
}

// The rebuild on a moved tip decides again: a competitor that published other
// words at the record's path between capture and push is refused on the retry.
func TestGoalReviewRefusesOtherBytesPublishedOnAMovedTip(t *testing.T) {
	t.Parallel()
	endpoint, client := fakeGoalEndpoint(t, underReview("under-review"))
	request, proof := reviewer(t, endpoint, 4, 1)
	act := clearAct()
	head, err := act.check("under-review")
	if err != nil {
		t.Fatal(err)
	}
	publish := reviewRequest(request, "under-review", act, head, proof)
	publish.BeforePush = func(attempt int) error {
		if attempt != 1 {
			return nil
		}
		parent, err := client.Capture("competitor")
		if err != nil {
			return err
		}
		commit, err := client.Build("competitor", parent, []Change{{Path: reviewPath, Content: []byte("# Another checkout's first review\n")}}, "competitor")
		if err != nil {
			return err
		}
		_, err = client.Publish(parent, commit)
		return err
	}
	result, err := Publish(endpoint, publish)
	if err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "never overwritten") {
		t.Fatalf("a moved tip's different bytes: %+v %v", result, err)
	}
	files, _ := readCommitFiles(endpoint, client.store.canonical, reviewPath)
	if string(files[reviewPath]) != "# Another checkout's first review\n" {
		t.Fatalf("the competitor's words were overwritten: %q", files[reviewPath])
	}
}

func sendBackAct() ReviewAct {
	return ReviewAct{
		Record: reviewPath, Content: reviewRecord("under-review", "send back", reviewedTip), Verdict: VerdictSendBack,
		Brief: []byte("# Correction brief\n\n## Fix these findings from " + reviewPath + " at 9c1f0a2\n\n1. The owner reads the wrong tree (internal/owner.go:60)\n"),
	}
}

// A holder that lawfully claimed a second goal while the first waits to land:
// clearing the Landing would put it over the one-claim quota, so the send-back
// leaves the Landing as it stands and the ledger stays valid (Astra S69-05).
func TestGoalReviewSendBackPublishesTheBriefAndLeavesTheLanding(t *testing.T) {
	t.Parallel()
	second := vGoal("second-claim", StateClaimed)
	endpoint := reviewBed(t, underReview("under-review"), second)
	request, proof := reviewer(t, endpoint, 5, 1)
	result, err := Review(request, "under-review", sendBackAct(), proof)
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("send back with a second claim held: %+v %v", result, err)
	}
	tree, err := loadTreeFor(endpoint, result.Tip)
	if err != nil {
		t.Fatal(err)
	}
	f := tree.Live["under-review"]
	if f.Landing == nil || f.Landing.Opid != landingOpid || f.State != StateClaimed {
		t.Fatalf("the send-back touched the Landing record: %+v", f.Landing)
	}
	if problems := ValidateTree(tree); len(problems) != 0 {
		t.Fatalf("the ledger after the send-back is not valid: %v", problems)
	}
	brief := BriefPathFor(reviewPath)
	files, err := readCommitFiles(endpoint, result.Tip, brief)
	if err != nil || string(files[brief]) != string(sendBackAct().Brief) {
		t.Fatalf("the brief did not land beside the record: %q %v", files[brief], err)
	}
	last := f.History[len(f.History)-1]
	if !strings.HasSuffix(last.Reason, " brief="+brief) || !strings.Contains(last.Reason, "verdict=send-back") {
		t.Fatalf("the send-back line does not carry its brief: %q", last.Reason)
	}
	sent, standing := SentBackOf(f)
	if !standing || sent.Brief != brief || sent.Opid != last.Opid {
		t.Fatalf("the send-back does not read as standing: %+v %v", sent, standing)
	}
	published, err := ReadPublished(endpoint, brief)
	if err != nil || string(published) != string(sendBackAct().Brief) {
		t.Fatalf("the holder cannot read the published brief: %q %v", published, err)
	}
}

// The holder's answer: needs-work names the candidates, the human's repeat
// with --work is a send-back of its own, and an attempt line answers it once.
func TestSendBackAnswersAreTheHoldersAndAnswerOnce(t *testing.T) {
	t.Parallel()
	endpoint := reviewBed(t, underReview("under-review"))
	request, proof := reviewer(t, endpoint, 6, 1)
	sent, err := Review(request, "under-review", sendBackAct(), proof)
	if err != nil || sent.Outcome != OutcomeConfirmed {
		t.Fatalf("send back: %+v %v", sent, err)
	}
	tree, _ := loadTreeFor(endpoint, sent.Tip)
	review, _ := SentBackOf(tree.Live["under-review"])

	holder := verbReqFor(endpoint, reviewUlid(6, 2), "mac-a")
	holder.Now = reviewNow
	if _, err := AnswerSendBack(holder, "under-review", SendBackAnswer{Review: review.Opid, Candidates: []string{"discovery", "writer"}}); err != nil {
		t.Fatalf("needs-work: %v", err)
	}
	tree, _ = loadTreeFor(endpoint, acceptedTipForEndpoint(t, endpoint))
	read := VerdictsOf(tree.Live["under-review"])
	if read.Answer == nil || strings.Join(read.Answer.Candidates, ",") != "discovery,writer" {
		t.Fatalf("the needs-work answer does not read back: %+v", read.Answer)
	}
	if _, standing := SentBackOf(tree.Live["under-review"]); standing {
		t.Fatal("an answered send-back still reads as the holder's to act on")
	}

	named := sendBackAct()
	named.Work = "writer"
	again, proof := reviewer(t, endpoint, 6, 3)
	if result, err := Review(again, "under-review", named, proof); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("the send-back naming its work: %+v %v", result, err)
	}
	tree, _ = loadTreeFor(endpoint, acceptedTipForEndpoint(t, endpoint))
	second, standing := SentBackOf(tree.Live["under-review"])
	if !standing || second.Work != "writer" {
		t.Fatalf("the named send-back does not stand: %+v %v", second, standing)
	}

	holder.Ulid = reviewUlid(6, 4)
	answer := SendBackAnswer{Review: second.Opid, Attempt: 3, Work: "writer"}
	if result, err := AnswerSendBack(holder, "under-review", answer); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("the attempt answer: %+v %v", result, err)
	}
	holder.Ulid = reviewUlid(6, 5)
	if result, err := AnswerSendBack(holder, "under-review", answer); err != nil || !result.Unchanged {
		t.Fatalf("a second answer to one send-back is not a repeat: %+v %v", result, err)
	}
	stranger := verbReqFor(endpoint, reviewUlid(6, 6), "mac-b")
	if result, _ := AnswerSendBack(stranger, "under-review", answer); result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "not this seat's claim") {
		t.Fatalf("another seat answered the holder's send-back: %+v", result)
	}
	human := holder
	human.Actor.Human = "Wido"
	if _, err := AnswerSendBack(human, "under-review", answer); err == nil {
		t.Fatal("a human answered the holder's send-back")
	}
}

// A later land-ready puts the goal back in Review with no verdict standing.
func TestAVerdictOlderThanTheLandingSaysNothing(t *testing.T) {
	t.Parallel()
	f := underReview("under-review")
	f.History = append(f.History, HistoryLine{
		At: "2026-08-20T11:00:00Z", Opid: "01J5X0000000000000000000R1-mac-ui-1a2b3c4d", Verb: "review", Actor: "human:Wido", Targets: []string{"under-review"}, Keep: -1,
		Reason: "reviewed verdict=send-back tip=" + reviewedTip + " record=" + reviewPath + " by=Wido brief=" + BriefPathFor(reviewPath),
	})
	if !VerdictsOf(f).SentBack {
		t.Fatal("a send-back newer than the Landing does not read as sent back")
	}
	later := "01J5X0000000000000000000M2-mac-a-1a2b3c4d"
	f.History = append(f.History, HistoryLine{At: "2026-08-20T12:00:00Z", Opid: later, Verb: "land-ready", Actor: "mac-a+lin-1", Targets: []string{"under-review"}, Keep: -1})
	f.Landing = &LandingRecord{At: "2026-08-20T12:00:00Z", Opid: later}
	if read := VerdictsOf(f); read.SentBack || read.Latest != nil {
		t.Fatalf("a verdict older than the Landing still stands: %+v", read)
	}
}

func TestReviewHistoryGrammar(t *testing.T) {
	t.Parallel()
	rows := []struct {
		line string
		want string
	}{
		{"- 2026-08-20T11:00:00Z 01J5X0000000000000000000R1-mac-ui-1a2b3c4d review actor=mac-a+lin-1 targets=g reason=reviewed verdict=clear-to-land tip=" + reviewedTip + " record=" + reviewPath + " by=Wido", "human's act"},
		{"- 2026-08-20T11:00:00Z 01J5X0000000000000000000R1-mac-ui-1a2b3c4d review actor=human:Wido targets=g reason=reviewed verdict=maybe tip=" + reviewedTip + " record=" + reviewPath + " by=Wido", "clear-to-land or send-back"},
		{"- 2026-08-20T11:00:00Z 01J5X0000000000000000000R1-mac-ui-1a2b3c4d review actor=human:Wido targets=g reason=reviewed verdict=send-back tip=" + reviewedTip + " record=" + reviewPath + " by=Wido", "names its brief"},
		{"- 2026-08-20T11:00:00Z 01J5X0000000000000000000R1-mac-ui-1a2b3c4d review actor=human:Wido targets=g reason=reviewed verdict=clear-to-land tip=abc record=" + reviewPath + " by=Wido", "full commit id"},
		{"- 2026-08-20T11:00:00Z 01J5X0000000000000000000R2-mac-a-1a2b3c4d send-back actor=human:Wido targets=g reason=send-back attempt=2 review=01J5X0000000000000000000R1-mac-ui-1a2b3c4d", "holder's answer"},
		{"- 2026-08-20T11:00:00Z 01J5X0000000000000000000R2-mac-a-1a2b3c4d send-back actor=mac-a+lin-1 targets=g reason=send-back needs-work candidates=one review=01J5X0000000000000000000R1-mac-ui-1a2b3c4d", "two or more candidates"},
	}
	for _, row := range rows {
		if _, err := ParseHistoryLine(row.line); err == nil || !strings.Contains(err.Error(), row.want) {
			t.Errorf("%s\n  refusal = %v, want %q", row.line, err, row.want)
		}
	}
	good := "- 2026-08-20T11:00:00Z 01J5X0000000000000000000R2-mac-a-1a2b3c4d send-back actor=mac-a+lin-1 targets=g reason=send-back attempt=3 review=01J5X0000000000000000000R1-mac-ui-1a2b3c4d work=writer"
	h, err := ParseHistoryLine(good)
	if err != nil || RenderHistoryLine(h) != good {
		t.Fatalf("a good answer line does not round-trip: %v\n%s", err, RenderHistoryLine(h))
	}
}

func TestReadReviewRecordReadsTheHeadAndTheOutcome(t *testing.T) {
	t.Parallel()
	head := ReadReviewRecord(reviewRecord("under-review", "clear to land", reviewedTip))
	if strings.Join(head.Goals, ",") != "under-review" || head.Tip != reviewedTip || head.Verdict != "clear to land" || head.ReviewedAt != reviewedTip {
		t.Fatalf("head = %+v", head)
	}
	// A Reviewed line inside a section is somebody's words, not the head.
	words := []byte("# R\n\n- Goals: g\n- Reviewed: none found; write them here\n\n## Facts\n\n- Reviewed: " + reviewedTip + " (the tip of goal/g)\n")
	if head := ReadReviewRecord(words); head.Tip != "" {
		t.Fatalf("a section's line was read as the head: %+v", head)
	}
}
