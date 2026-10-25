package goal

import (
	"strings"
	"testing"
)

func holderConclusionGoal() *GoalFile {
	file := underReview("under-review")
	file.Origin = OriginHuman
	return file
}

func holderConclusionRequest(endpoint Endpoint, step int) VerbRequest {
	request := verbReqFor(endpoint, reviewUlid(7, step), "mac-a")
	request.Now = reviewNow
	return request
}

func TestHolderConcludesAHumanGoalOnceItsLandingIsRecorded(t *testing.T) {
	t.Parallel()
	endpoint := reviewBed(t, holderConclusionGoal())
	if result, err := Done(holderConclusionRequest(endpoint, 1), "under-review", "Delivered."); err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "metasystem work land under-review") {
		t.Fatalf("conclusion before landing: %+v %v", result, err)
	}
	if result, err := RecordLanded(holderConclusionRequest(endpoint, 2), "under-review", "the lane"); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("record landing: %+v %v", result, err)
	}
	if result, err := Done(holderConclusionRequest(endpoint, 3), "under-review", "Delivered."); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("holder concludes: %+v %v", result, err)
	}
	file := archivedGoal(t, endpoint, "under-review")
	last := file.History[len(file.History)-1]
	if last.Verb != "done" || last.Actor != "mac-a+lin-1" {
		t.Fatalf("holder's conclusion: %+v", last)
	}
}

func TestAnotherPairStillNeedsAPerson(t *testing.T) {
	t.Parallel()
	endpoint := reviewBed(t, holderConclusionGoal())
	if result, err := RecordLanded(holderConclusionRequest(endpoint, 1), "under-review", "the lane"); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("record landing: %+v %v", result, err)
	}
	other := holderConclusionRequest(endpoint, 2)
	other.Actor.Lineage = "other-session"
	if result, err := Done(other, "under-review", "Delivered."); err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "another's work") {
		t.Fatalf("another lineage concluded: %+v %v", result, err)
	}
}

func TestHolderConclusionKeepsTheGoalChecks(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"read item", "review obligation", "parked"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			file := holderConclusionGoal()
			file.History = append(file.History, HistoryLine{At: reviewNow.Format("2006-01-02T15:04:05Z07:00"), Opid: Opid(reviewUlid(7, 1), "mac-a", "lin-1"), Verb: LandedVerb, Actor: "mac-a+lin-1", Targets: []string{file.Id}, Keep: -1, Reason: "landed under the lane"})
			file.Revision++
			want, wantCode := "", ""
			switch name {
			case "read item":
				file.ReadItems = []ReadItem{{ID: "critic-a-1", Read: "critic-a", State: ReadItemOpen, Text: "Explain the fallback.", AddedAt: "2026-09-29T09:00:00Z"}}
				want, wantCode = "open review notes", "GOAL_DONE_READ_ITEMS_OPEN"
			case "review obligation":
				file.ReviewObligations = []ReviewObligation{{Finding: "F-1", Chain: "critic-a", Artifact: "owner.go", Test: "prove the fallback", State: "open"}}
				want = "open review obligation"
			case "parked":
				file.State, file.Claimed, file.Landing = StateParked, nil, nil
				file.Parked = &ParkRecord{By: "human:Wido", At: "2026-09-29T09:00:00Z", Because: "paused"}
				want = "parked"
			}
			endpoint := reviewBed(t, file)
			if result, err := Done(holderConclusionRequest(endpoint, 2), file.Id, "Delivered."); err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, want) || wantCode != "" && result.Code != wantCode {
				t.Fatalf("%s allowed conclusion: %+v %v", name, result, err)
			}
		})
	}
}

func TestALandingBeforeTheStandingLandingDoesNotCount(t *testing.T) {
	t.Parallel()
	file := holderConclusionGoal()
	file.History = append(file.History,
		HistoryLine{At: reviewNow.Format("2006-01-02T15:04:05Z07:00"), Opid: Opid(reviewUlid(7, 1), "mac-a", "lin-1"), Verb: LandedVerb, Actor: "mac-a+lin-1", Targets: []string{file.Id}, Keep: -1, Reason: "landed under the lane"},
		HistoryLine{At: reviewNow.Format("2006-01-02T15:04:05Z07:00"), Opid: Opid(reviewUlid(7, 2), "mac-a", "lin-1"), Verb: "land-ready", Actor: "mac-a+lin-1", Targets: []string{file.Id}, Keep: -1})
	file.Revision = uint64(len(file.History))
	file.Landing = &LandingRecord{At: file.History[len(file.History)-1].At, Opid: file.History[len(file.History)-1].Opid}
	endpoint := reviewBed(t, file)
	if result, err := Done(holderConclusionRequest(endpoint, 3), file.Id, "Delivered."); err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "metasystem work land") {
		t.Fatalf("old landing allowed conclusion: %+v %v", result, err)
	}
}

func TestHolderConclusionStepNeedsARecordedLandingAndNoSendBack(t *testing.T) {
	t.Parallel()
	for _, state := range []string{"unlanded", "landed", "sent back"} {
		t.Run(state, func(t *testing.T) {
			t.Parallel()
			file := holderConclusionGoal()
			if state != "unlanded" {
				file.History = append(file.History, HistoryLine{At: reviewNow.Format("2006-01-02T15:04:05Z07:00"), Opid: Opid(reviewUlid(7, 1), "mac-a", "lin-1"), Verb: LandedVerb, Actor: "mac-a+lin-1", Targets: []string{file.Id}, Keep: -1, Reason: "landed under the lane"})
				file.Revision++
			}
			if state == "sent back" {
				humanLine(file, reviewNow.Format("2006-01-02T15:04:05Z07:00"), reviewUlid(7, 2)+"-mac-ui-human", "review",
					"reviewed verdict=send-back tip="+reviewedTip+" record="+reviewPath+" by=Wido brief="+BriefPathFor(reviewPath))
			}
			steps := HolderStepsDue([]*GoalFile{file}, gateSettings, reviewNow)
			if state == "unlanded" {
				if len(steps) != 0 {
					t.Fatalf("unlanded goal has a due step: %+v", steps)
				}
				return
			}
			if len(steps) != 1 || steps[0].Conclude != (state == "landed") || steps[0].Revise != (state == "sent back") {
				t.Fatalf("%s: %+v", state, steps)
			}
		})
	}
}
