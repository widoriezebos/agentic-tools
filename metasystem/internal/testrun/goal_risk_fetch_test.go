package testrun

import (
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testgoal"
)

// laggingGoalLedger is a ledger whose local accepted ref still points at the
// genesis commit while the shared ledger already carries the goal's open
// commit: the lane checkout whose repo watcher stopped fetching.
func laggingGoalLedger(t *testing.T, id string, now time.Time) goal.Endpoint {
	t.Helper()
	root := t.TempDir()
	backlog := goal.RenderRoot(&goal.RootRecord{Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "1", SyncMode: goal.SyncLocal, Revision: 1})
	genesis := "0000000000000000000000000000000000000001"
	repository := testgoal.New(map[string][]byte{"plans/goals/backlog.md": backlog}, now.Add(-time.Hour), genesis)
	risk := &goal.RiskRecord{Severity: 2, Novelty: 3, Exposure: 2, Accumulation: 1, Basis: "Opened after the lane last fetched."}
	budget := goal.Budget{ElapsedLimit: "4h", AttemptLimit: 8, ReservedJobMinutesLimit: 480, ActiveJobLimit: 2, ReviewRoundLimit: 3}
	openedAt, approvedAt := now.Add(-30*time.Minute).Format(time.RFC3339), now.Add(-29*time.Minute).Format(time.RFC3339)
	openOpid := goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FAV", "mac-cli", id)
	approvalOpid := goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FAW", "mac-cli", id)
	file := &goal.GoalFile{Id: id, State: goal.StateApproved, Tier: 3, Risk: risk, Intent: "Prove " + id + ".", Origin: goal.OriginMain, NextStep: "Prove it.", OpenedAt: openedAt, Revision: 2, Budget: &budget,
		Approved: &goal.ApprovalRecord{By: "human:Wido", At: approvedAt, Revision: 2, EpisodeRevision: 2, Opid: approvalOpid, Authority: goal.ApprovalAuthorityProven},
		History:  []goal.HistoryLine{{At: openedAt, Opid: openOpid, Verb: "open", Actor: "mac-cli+" + id, Targets: []string{id}, Keep: -1}, {At: approvedAt, Opid: approvalOpid, Verb: "approve", Actor: "human:Wido", Targets: []string{id}, Keep: -1}}}
	file.Approved.Digest = goal.ApprovalDigest(file.Intent, file.Tier, budget, risk)
	if _, err := repository.Capture(openOpid); err != nil {
		t.Fatal(err)
	}
	opened, err := repository.Build(openOpid, genesis, []goal.Change{{Path: "plans/goals/" + id + ".md", Content: goal.RenderFile(file)}}, "goal open "+id)
	if err != nil {
		t.Fatal(err)
	}
	if outcome, err := repository.Publish(genesis, opened); err != nil || outcome != goal.CASLanded {
		t.Fatalf("publish the open commit: %v %v", outcome, err)
	}
	if err := repository.Release(openOpid); err != nil {
		t.Fatal(err)
	}
	if accepted, _, _ := repository.Accepted(); accepted != genesis {
		t.Fatalf("fixture accepted ref moved to %s; it must lag the shared ledger", accepted)
	}
	return goal.Endpoint{Root: root, Remote: goal.SyncLocal, Branch: goal.LocalLedgerBranch, Repository: repository, ProjectionDeadline: func(wait time.Duration) <-chan time.Time {
		if wait != 4*time.Second {
			t.Errorf("risk projection deadline=%s; want 4s", wait)
		}
		return make(chan time.Time)
	}}
}

func TestGoalRiskFetchesOnceWhenTheAcceptedLedgerLagsTheGoal(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	id := "fleet-card-follows-the-simple-lane"
	endpoint := laggingGoalLedger(t, id, now)
	deadlines := 0
	clock := endpoint.ProjectionDeadline
	endpoint.ProjectionDeadline = func(wait time.Duration) <-chan time.Time { deadlines++; return clock(wait) }
	risk, revision, err := GoalRiskAt(endpoint, id, now)
	if deadlines != 1 {
		t.Fatalf("risk projection used %d fixture deadlines; want 1", deadlines)
	}
	if err != nil {
		t.Fatalf("planning for a goal opened after the last fetch refused: %v", err)
	}
	if revision != 2 || risk.Severity != 2 || risk.Novelty != 3 || risk.Exposure != 2 || risk.Accumulation != 1 {
		t.Fatalf("risk=%+v revision=%d", risk, revision)
	}
	if accepted, _, _ := endpoint.Repository.Accepted(); accepted == "0000000000000000000000000000000000000001" {
		t.Fatal("the accepted ledger did not advance to the shared tip")
	}
}

func TestGoalRiskRefusesAGoalTheSharedLedgerDoesNotHold(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	endpoint := laggingGoalLedger(t, "fleet-card-follows-the-simple-lane", now)
	deadlines := 0
	clock := endpoint.ProjectionDeadline
	endpoint.ProjectionDeadline = func(wait time.Duration) <-chan time.Time { deadlines++; return clock(wait) }
	_, _, err := GoalRiskAt(endpoint, "no-such-goal", now)
	if deadlines != 1 {
		t.Fatalf("risk refusal used %d fixture deadlines; want 1", deadlines)
	}
	if err == nil {
		t.Fatal("a goal in no ledger was planned")
	}
	lines := strings.Split(err.Error(), "\n")
	if len(lines) != 2 || !strings.Contains(lines[0], "no-such-goal") || !strings.HasPrefix(lines[1], "run: ") {
		t.Fatalf("refusal is not two plain lines: %q", err.Error())
	}
}
