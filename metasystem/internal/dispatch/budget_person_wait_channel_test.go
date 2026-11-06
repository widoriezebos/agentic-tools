package dispatch_test

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testchannel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testprovider"
)

func channelWaitBed(t *testing.T) (string, string, *goal.GoalFile, time.Time) {
	t.Helper()
	root := t.TempDir()
	testchannel.Save(t, filepath.Join(root, "metasystem.conf"), []byte{})
	start := time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC)
	file := &goal.GoalFile{Id: "bounded", State: goal.StateClaimed, Revision: 1,
		Claimed:  &goal.ClaimRecord{Machine: "bed-m1", Lineage: "coordinator", At: start.Format(time.RFC3339), Revision: 1},
		Approved: &goal.ApprovalRecord{Revision: 1}, History: []goal.HistoryLine{{At: start.Format(time.RFC3339)}},
		Budget: &goal.Budget{ElapsedLimit: "8h", AttemptLimit: 2, ReservedJobMinutesLimit: 75, ActiveJobLimit: 1}}
	return root, testprovider.Register(t, root), file, start
}

func saveWaitQuestion(t *testing.T, root string, q channel.Question) {
	t.Helper()
	testchannel.Save(t, filepath.Join(root, "artifacts/agents/channel/questions", q.ID+".json"), q)
}

func TestPersonWaitClosedQuestion(t *testing.T) {
	t.Parallel()
	root, home, file, start := channelWaitBed(t)
	saveWaitQuestion(t, root, channel.Question{ID: "closed", Goal: file.Id, State: "closed", OpenedAt: start.Add(2 * time.Hour), ClosedAt: start.Add(4 * time.Hour)})
	p := dispatch.ProjectBudget(root, file, start.Add(9*time.Hour), home)
	if p.Status != dispatch.BudgetKnown || p.Elapsed != 7*time.Hour {
		t.Fatalf("closed interval lost: %+v", p)
	}
}

func TestPersonWaitReplyIsNotTheAct(t *testing.T) {
	t.Parallel()
	root, home, file, start := channelWaitBed(t)
	stop := &channel.UnitStopQuestion{Loop: "unit-round", Subject: "bounded/U1/run", Attempt: 2, Finding: "read:1", AcceptableActs: []string{"work-revise"}}
	q := channel.Question{ID: "act", Goal: file.Id, State: "answered", OpenedAt: start.Add(2 * time.Hour), Answer: &channel.Answer{At: start.Add(3 * time.Hour), Phase: "recorded"}, UnitStop: stop}
	saveWaitQuestion(t, root, q)
	p := dispatch.ProjectBudget(root, file, start.Add(6*time.Hour), home)
	if p.Status != dispatch.BudgetKnown || p.Elapsed != 2*time.Hour {
		t.Fatalf("reply substituted for act: %+v", p)
	}
	actAt := start.Add(6 * time.Hour)
	if err := channel.RecordUnitStopAct(root, channel.UnitStopAct{ID: "fixture-act", Goal: file.Id, Loop: stop.Loop, Subject: stop.Subject, Attempt: stop.Attempt, Findings: []string{stop.Finding}, Kind: "work-revise", At: actAt}); err != nil {
		t.Fatal(err)
	}
	stored, err := channel.ReadQuestion(root, q.ID)
	if err != nil || stored.State != "closed" || !stored.ClosedAt.Equal(actAt) {
		t.Errorf("act did not close replied question at its time: %+v %v", stored, err)
	}
	p = dispatch.ProjectBudget(root, file, start.Add(9*time.Hour), home)
	if p.Status != dispatch.BudgetKnown || p.Elapsed != 5*time.Hour {
		t.Errorf("act did not resume charging: %+v", p)
	}
	q.UnitStop = nil
	q.ID = "plain"
	saveWaitQuestion(t, root, q)
	// Use another root so the act wait cannot mask the ordinary reply rule.
	plainRoot, plainHome, plainFile, _ := channelWaitBed(t)
	saveWaitQuestion(t, plainRoot, q)
	p = dispatch.ProjectBudget(plainRoot, plainFile, start.Add(9*time.Hour), plainHome)
	if p.Status != dispatch.BudgetKnown || p.Elapsed != 8*time.Hour {
		t.Fatalf("ordinary reply failed to end wait: %+v", p)
	}
}

func TestPersonWaitHoldRetiresWhenConditionClears(t *testing.T) {
	t.Parallel()
	for _, subject := range []string{"bounded", "seat:bed-m1"} {
		t.Run(subject, func(t *testing.T) {
			t.Parallel()
			root, home, file, start := channelWaitBed(t)
			q := channel.Question{ID: "hold", About: "machine", State: "closed", Contract: true, OpenedAt: start.Add(2 * time.Hour), Answer: &channel.Answer{At: start.Add(3 * time.Hour)}, ClosedAt: start.Add(3 * time.Hour), Decision: &channel.Decision{Kind: "steward-hold", Subject: subject}}
			saveWaitQuestion(t, root, q)
			p := dispatch.ProjectBudget(root, file, start.Add(5*time.Hour), home)
			want := 2 * time.Hour
			if subject != file.Id {
				want = 5 * time.Hour
			}
			if p.Status != dispatch.BudgetKnown || p.Elapsed != want {
				t.Fatalf("reply ended hold or seat subject paused goal: %+v", p)
			}
			if err := channel.Retire(root, q.ID, "condition cleared", start.Add(6*time.Hour)); err != nil {
				t.Fatal(err)
			}
			p = dispatch.ProjectBudget(root, file, start.Add(9*time.Hour), home)
			want = 5 * time.Hour
			if subject != file.Id {
				want = 9 * time.Hour
			}
			if p.Status != dispatch.BudgetKnown || p.Elapsed != want {
				t.Fatalf("retired hold duration: %+v", p)
			}
		})
	}
}

func TestPersonWaitUncollectedRound(t *testing.T) {
	t.Parallel()
	root, home, file, start := channelWaitBed(t)
	q := channel.Question{ID: "round", About: "machine", State: "open", OpenedAt: start.Add(2 * time.Hour), Decision: &channel.Decision{Kind: "uncollected-round", Subject: file.Id}}
	saveWaitQuestion(t, root, q)
	if err := channel.Retire(root, q.ID, "round collected", start.Add(5*time.Hour)); err != nil {
		t.Fatal(err)
	}
	p := dispatch.ProjectBudget(root, file, start.Add(9*time.Hour), home)
	if p.Status != dispatch.BudgetKnown || p.Elapsed != 6*time.Hour {
		t.Fatalf("collection wait lost: %+v", p)
	}
}

func TestPersonWaitUnboundedQuestionCounts(t *testing.T) {
	t.Parallel()
	for _, damage := range []string{"closed", "unreadable", "missing id", "missing openedAt", "missing state"} {
		t.Run(damage, func(t *testing.T) {
			t.Parallel()
			root, home, file, start := channelWaitBed(t)
			q := channel.Question{ID: "damaged", Goal: file.Id, State: "closed", OpenedAt: start.Add(2 * time.Hour)}
			switch damage {
			case "missing id":
				q.ID = ""
			case "missing openedAt":
				q.OpenedAt = time.Time{}
			case "missing state":
				q.State = ""
			}
			path := filepath.Join(root, "artifacts/agents/channel/questions/damaged.json")
			testchannel.Save(t, path, q)
			if damage == "unreadable" {
				testchannel.Save(t, path, []byte("{"))
			}
			p := dispatch.ProjectBudget(root, file, start.Add(9*time.Hour), home)
			if p.Status != dispatch.BudgetKnown || p.Elapsed != 9*time.Hour || len(p.Problems) != 1 || !strings.Contains(p.Problems[0], "damaged") {
				t.Fatalf("bad record hid elapsed or its problem: %+v", p)
			}
		})
	}
}

func TestPersonWaitOtherQuestions(t *testing.T) {
	t.Parallel()
	root, home, file, start := channelWaitBed(t)
	saveWaitQuestion(t, root, channel.Question{ID: "other", Goal: "another", State: "open", OpenedAt: start.Add(2 * time.Hour)})
	saveWaitQuestion(t, root, channel.Question{ID: "lane", About: "lane", State: "open", OpenedAt: start.Add(2 * time.Hour)})
	p := dispatch.ProjectBudget(root, file, start.Add(9*time.Hour), home)
	if p.Status != dispatch.BudgetKnown || p.Elapsed != 9*time.Hour {
		t.Fatalf("unrelated question paused goal: %+v", p)
	}
}

func TestPersonWaitRecoveryNeedsActButProcessReplyEndsWait(t *testing.T) {
	t.Parallel()
	for _, act := range []bool{false, true} {
		t.Run(map[bool]string{false: "process", true: "recovery"}[act], func(t *testing.T) {
			t.Parallel()
			root, home, file, start := channelWaitBed(t)
			q := channel.Question{ID: "kind", Goal: file.Id, State: "answered", OpenedAt: start.Add(2 * time.Hour), Answer: &channel.Answer{At: start.Add(3 * time.Hour), Phase: "recorded"}, ProcessAct: "continue"}
			if act {
				q.Recovery = &channel.RecoverySubject{}
				q.ProcessAct = ""
			}
			saveWaitQuestion(t, root, q)
			p := dispatch.ProjectBudget(root, file, start.Add(6*time.Hour), home)
			want := 5 * time.Hour
			if act {
				want = 2 * time.Hour
			}
			if p.Status != dispatch.BudgetKnown || p.Elapsed != want {
				t.Fatalf("act kind used wrong reply rule: %+v", p)
			}
		})
	}
}
