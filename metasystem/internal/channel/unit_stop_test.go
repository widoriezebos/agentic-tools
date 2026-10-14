package channel

import (
	"context"
	"sync"
	"testing"
	"time"
)

func unitStopAskFixture(root, finding string) AskRequest {
	return AskRequest{RepoRoot: root, Goal: "g", Kind: "stop", Machine: "m", Facts: []string{"unresolved finding " + finding}, Now: time.Unix(10, 0), UnitStop: &UnitStopQuestion{Loop: "unit-round", Subject: "g/u/run", Attempt: 2, Finding: finding, Review: "read", Needs: "metasystem work revise g --work u --reason 'correct the finding' --by Wido", AcceptableActs: []string{"work-revise", "goal-accept-risk", "goal-done"}}}
}

func TestUnitStopActKeepsGoalFreeDesignAndBoundDrop(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"goal-free design", "goal-free unit", "drop without attempt", "drop without findings", "bound drop"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			request := unitStopAskFixture(root, "read:1")
			request.UnitStop.AcceptableActs = []string{"design-accept", "work-drop"}
			if scenario == "goal-free design" {
				request.Goal, request.About, request.Kind, request.UnitStop.Loop = "", "machine", "other", "design-round"
			}
			q, err := Ask(request)
			if err != nil {
				t.Fatal(err)
			}
			act := UnitStopAct{ID: "act", Goal: request.Goal, Loop: request.UnitStop.Loop, Subject: request.UnitStop.Subject, Attempt: 2, Findings: []string{"read:1"}, Kind: "work-drop", At: time.Unix(11, 0), UnitClosed: true}
			valid := scenario == "goal-free design" || scenario == "bound drop"
			switch scenario {
			case "goal-free design":
				act.Kind, act.Attempt, act.Findings = "design-accept", 0, nil
			case "goal-free unit":
				act.Goal = ""
			case "drop without attempt":
				act.Attempt = 0
			case "drop without findings":
				act.Findings = nil
			}
			if err := RecordUnitStopAct(root, act); (err == nil) != valid {
				t.Fatalf("act validation: valid=%t err=%v act=%+v", valid, err, act)
			}
			stored, err := ReadQuestion(root, q.ID)
			want := "open"
			if valid {
				want = "closed"
			}
			if err != nil || stored.State != want {
				t.Fatalf("question state=%s want=%s err=%v", stored.State, want, err)
			}
		})
	}
}

func TestUnitStopActClosesOnlyItsFindingAndSubject(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	first, err := Ask(unitStopAskFixture(root, "read:1"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := Ask(unitStopAskFixture(root, "read:2"))
	if err != nil {
		t.Fatal(err)
	}
	act := UnitStopAct{ID: "act", Goal: "g", Loop: "unit-round", Subject: "g/another/run", Attempt: 2, Findings: []string{"read:1"}, Kind: "goal-accept-risk", Reason: "local exposure", At: time.Unix(11, 0)}
	if err := RecordUnitStopAct(root, act); err != nil {
		t.Fatal(err)
	}
	if q, _ := ReadQuestion(root, first.ID); q.State != "open" {
		t.Fatalf("another subject closed the ask: %+v", q)
	}
	act.ID, act.Subject, act.Kind = "prepared", "g/u/run", "prepared-drop"
	if err := RecordUnitStopAct(root, act); err != nil {
		t.Fatal(err)
	}
	if q, _ := ReadQuestion(root, first.ID); q.State != "open" {
		t.Fatalf("a prepared drop closed the ask: %+v", q)
	}
	act.ID, act.Kind = "success", "goal-accept-risk"
	if err := RecordUnitStopAct(root, act); err != nil {
		t.Fatal(err)
	}
	if q, _ := ReadQuestion(root, first.ID); q.State != "closed" {
		t.Fatalf("matching act did not close the ask: %+v", q)
	}
	if q, _ := ReadQuestion(root, second.ID); q.State != "open" {
		t.Fatalf("accepting one risk closed another finding: %+v", q)
	}
	if err := CloseGoalUnitStopQuestions(root, "g", "finished explicitly", time.Unix(12, 0)); err != nil {
		t.Fatal(err)
	}
	if q, _ := ReadQuestion(root, second.ID); q.State != "closed" {
		t.Fatalf("goal conclusion left its ask open: %+v", q)
	}
	request := unitStopAskFixture(root, "later-read:1")
	request.UnitStop.Attempt++
	later, err := Ask(request)
	if err != nil || later.State != "open" {
		t.Fatalf("old closure consumed the later attempt: %+v %v", later, err)
	}
}

func TestUnitStopTextAnswerNeverClosesItsQuestion(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	q, err := Ask(unitStopAskFixture(root, "read:1"))
	if err != nil {
		t.Fatal(err)
	}
	q.Answer = &Answer{Text: "please continue", Phase: "recorded"}
	if err := writeJSON(questionPath(root, q.ID), q); err != nil {
		t.Fatal(err)
	}
	if err := advanceAnswerWithEndpoint(context.Background(), PollConfig{RepoRoot: root}, &q, nil); err != nil {
		t.Fatal(err)
	}
	stored, err := ReadQuestion(root, q.ID)
	if err != nil || stored.State != "open" || q.Answer.Phase != "recorded" {
		t.Fatalf("text answer consumed a stop: %+v %v", stored, err)
	}
}

func TestUnitStopQuestionKeyJoinsConcurrentCollection(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	var wg sync.WaitGroup
	ids := make([]string, 8)
	errors := make([]error, len(ids))
	for i := range ids {
		wg.Add(1)
		go func() {
			defer wg.Done()
			q, err := Ask(unitStopAskFixture(root, "read:1"))
			ids[i], errors[i] = q.ID, err
		}()
	}
	wg.Wait()
	for i := range ids {
		if errors[i] != nil || ids[i] != ids[0] {
			t.Fatalf("duplicate ask: ids=%v errors=%v", ids, errors)
		}
	}
	request := unitStopAskFixture(root, "read:1")
	request.UnitStop.Attempt++
	q, err := Ask(request)
	if err != nil || q.ID == ids[0] {
		t.Fatalf("different attempt joined the old ask: %+v %v", q, err)
	}
}
