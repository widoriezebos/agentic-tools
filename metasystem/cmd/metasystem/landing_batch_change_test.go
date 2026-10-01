package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batchowner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/adapter"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/textui"
)

const laneChangeCommit = "abcdef0123456789abcdef0123456789abcdef01"

func laneChangeUnit() batch.Unit {
	unit := batch.NewChangeUnit(batch.ChangeMember{Commit: laneChangeCommit, Parent: strings.Repeat("9", 40), AskedBy: "m1e+human", Subject: "record: notes"},
		"/seats/m1e/metasystem", "m1e", "human", []string{"records/notes.md"}, nil)
	unit.State = batch.UnitJoined
	return unit
}

func laneGoalUnit() batch.Unit {
	return batch.Unit{GoalID: "goal-a", Chain: "chain-a", State: batch.UnitJoined, SelectedGroups: []string{"app-a"},
		Claim: batch.Claim{Machine: "landing", Lineage: "owner", Epoch: 1, Revision: 7, AccountingRevision: 5}}
}

// TestBatchChangeForecastChargesTheGoalMember (U11b): the cost check treats a
// change as one member of the series with no budget of its own: no prefix
// request for it, the tip request charged to the goal member, and no budget
// row for it.
func TestBatchChangeForecastChargesTheGoalMember(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	change := laneChangeUnit()
	candidate := batch.Record{Schema: 1, BatchID: "01j5x00000000000000000ba78", State: batch.StateOpen, BaseTree: "base-tree", TipTree: "tip-tree",
		PrefixTrees: []string{"goal-tree", "tip-tree"}, Units: []batch.Unit{laneGoalUnit(), change}}
	var decided, charged, budgets []string
	forecast, err := batchowner.ForecastBatchCostWith(root, candidate, nil, time.Unix(10, 0),
		func(_ string, units []batch.Unit, tree string) (batch.PrefixDecision, error) {
			decided = append(decided, units[len(units)-1].GoalID+"@"+tree)
			return batch.PrefixDecision{Groups: []string{"app-a"}}, nil
		},
		func(_ string, selection testrun.CostSelection, _ uint64) (testrun.CostEvidence, error) {
			charged = append(charged, selection.ID+"="+selection.GoalID)
			return testrun.CostEvidence{Request: batch.CostForecastRequest{ID: selection.ID, Kind: selection.Kind, Tree: selection.Tree, ChargeGoal: selection.GoalID}}, nil
		},
		func(_ string, unit batch.Unit, _ *batch.Unit, _ time.Time) (batchowner.BatchCostBudgetProjection, error) {
			budgets = append(budgets, unit.GoalID)
			return batchowner.BatchCostBudgetProjection{}, errors.New("budget read for " + unit.GoalID)
		})
	if err == nil || err.Error() != "budget read for goal-a" {
		t.Fatalf("forecast err=%v", err)
	}
	if !slices.Equal(decided, []string{change.GoalID + "@tip-tree", "goal-a@goal-tree"}) ||
		!slices.Equal(charged, []string{"tip:" + change.GoalID + "=goal-a", "prefix:goal-a=goal-a"}) || !slices.Equal(budgets, []string{"goal-a"}) {
		t.Fatalf("decided=%v charged=%v budgets=%v forecast=%+v", decided, charged, budgets, forecast)
	}
}

// TestBatchChangePrefixDecisionPlansGoalMembersOnly (U11b): a prefix's
// decision is its goal members' plans on the prefix tree, which holds every
// change before it.
func TestBatchChangePrefixDecisionPlansGoalMembersOnly(t *testing.T) {
	t.Parallel()
	change := laneChangeUnit()
	var plans []string
	decision, err := batchowner.PlanPrefixDecisionWith("", []batch.Unit{laneGoalUnit(), change}, "tip-tree",
		func(_, goalID, tree string, _ testpolicy.Mode, _ []string) (testrun.PlanOutput, error) {
			plans = append(plans, goalID)
			return testrun.PlanOutput{CandidateTree: tree, ContractDigest: "c", PolicyBaseCommit: "p",
				Plan: testpolicy.Plan{SelectedGroups: []string{"app-a"}, RequiredGroups: []string{"app-a"}}}, nil
		})
	if err != nil || !slices.Equal(plans, []string{"goal-a"}) || !slices.Equal(decision.Groups, []string{"app-a"}) {
		t.Fatalf("decision=%+v plans=%v err=%v", decision, plans, err)
	}
	// A prefix of changes alone is planned on the lane's account; a lane that
	// cannot be named plans nothing (fail closed).
	if _, err := batchowner.PlanPrefixDecisionWith(t.TempDir(), []batch.Unit{change}, "tip-tree", nil); err == nil || !strings.Contains(refusalDetail(err), "LANE_ACCOUNT_UNRESOLVED") {
		t.Fatalf("a change-only prefix without a lane: %v", err)
	}
}

// TestBatchChangeAuthorityRechecksItsGoal (U11b): a goal-less change needs no
// ledger; a change made in G's name leaves the batch when G's claim is no
// longer the asker's at the committed revision.
func TestBatchChangeAuthorityRechecksItsGoal(t *testing.T) {
	t.Parallel()
	change := laneChangeUnit()
	record := batch.Record{BatchID: "01j5x00000000000000000ba80"}
	if err := batchowner.AuthorizeBatchMemberInProjection("", time.Unix(10, 0), record, change, goal.Projection{}); err != nil {
		t.Fatalf("goal-less change: %v", err)
	}
	named := change
	named.Change = &batch.ChangeMember{Commit: laneChangeCommit, AskedBy: "m1e+seat", Goal: "goal-g", GoalRevision: 4}
	moved := goal.Projection{Tree: &goal.TreeGoals{Live: map[string]*goal.GoalFile{"goal-g": {Id: "goal-g", State: goal.StateClaimed,
		Claimed: &goal.ClaimRecord{Machine: "m1b", Lineage: "other", Revision: 5}}}}}
	err := batchowner.AuthorizeBatchMemberInProjection("", time.Unix(10, 0), record, named, moved)
	var revision *batch.PrefixRevisionRefusal
	if !errors.As(err, &revision) || !strings.Contains(err.Error(), "goal-g") {
		t.Fatalf("moved claim: %v", err)
	}
	if err := batchowner.AuthorizeBatchMemberInProjection("", time.Unix(10, 0), record, named, goal.Projection{}); err == nil {
		t.Fatal("an unreadable ledger authorized a change in a goal's name")
	}
	// A person's change in G's name holds no claim of G (held only warns on
	// a person's commit): its authority is G's landing gate alone.
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	person := named
	person.Change = &batch.ChangeMember{Commit: laneChangeCommit, AskedBy: "m1e+human", Goal: "goal-g"}
	moved.Tree.Live["goal-g"].Tier = 1
	if err := batchowner.AuthorizeChangeWith(root, person, moved, noBranchTip); err != nil {
		t.Fatalf("a person's change in a goal's name below the human tier: %v", err)
	}
	moved.Tree.Live["goal-g"].Tier = 4
	if err := batchowner.AuthorizeChangeWith(root, person, moved, noBranchTip); !errors.As(err, &revision) || !strings.Contains(err.Error(), "waits for a person") {
		t.Fatalf("a person's change past its goal's gate: %v", err)
	}
}

// TestChangeJoinGitAdapterFetchesThePinnedChangeAndJoins (U11b): the lane
// checkout fetches the seat's pinned commit (Git is the claim), reads the
// asker and subject from the commit the boundary made, and joins it as a
// change member of a new open batch; a repeat changes nothing; a commit with
// no Machine trailer was not made through the commit boundary and is refused.
func TestChangeJoinGitAdapterFetchesThePinnedChangeAndJoins(t *testing.T) {
	t.Parallel()
	seat, lane := t.TempDir(), t.TempDir()
	run := func(dir string, args ...string) string {
		t.Helper()
		output, err := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=Wido", "-c", "user.email=wido@example.com"}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
		return strings.TrimSpace(string(output))
	}
	run(seat, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(seat, "notes.md"), []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(seat, "add", "notes.md")
	run(seat, "commit", "-qm", "base")
	base := run(seat, "rev-parse", "HEAD")
	run(lane, "clone", "-q", seat, ".")
	if err := os.WriteFile(filepath.Join(seat, "notes.md"), []byte("one\ntwo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(seat, "commit", "-qam", "record: notes\n\nMachine: m1e+human\nLanding-Provenance-Verdict: would-refuse code=missing-declaration")
	commit := run(seat, "rev-parse", "HEAD")
	run(seat, "update-ref", batchowner.ChangePinRef(commit), commit)
	dependencies := batchowner.ProductionChangeJoinDependencies()
	dependencies.Base = func(root string) (string, error) { return run(root, "rev-parse", "HEAD^{tree}"), nil }
	dependencies.Mint = func() (string, error) { return "01j5x00000000000000000ba82", nil }
	dependencies.ProtectedTests = func(string, string, string) error { return nil }
	dependencies.Closure = func(string, string, string) *adapter.Closure { return nil }
	request := batchowner.ChangeJoinRequest{SeatRoot: seat, LandingRoot: lane, Commit: commit, At: time.Unix(10, 0)}
	record, err := batchowner.ExecuteChangeJoin(request, dependencies)
	if err != nil {
		t.Fatal(err)
	}
	if record.BatchID != "01j5x00000000000000000ba82" || record.State != batch.StateOpen || len(record.Units) != 1 {
		t.Fatalf("record=%+v", record)
	}
	unit := record.Units[0]
	if unit.GoalID != batch.ChangeID(commit) || unit.State != batch.UnitJoined || unit.Claim.Machine != "m1e" || unit.Claim.Lineage != "human" ||
		unit.Change.AskedBy != "m1e+human" || unit.Change.Subject != "record: notes" || unit.Change.Parent != base || !slices.Equal(unit.ChangedPaths, []string{"notes.md"}) {
		t.Fatalf("change unit=%+v change=%+v", unit, unit.Change)
	}
	if fetched := run(lane, "rev-parse", batchowner.ChangePinRef(commit)); fetched != commit {
		t.Fatalf("lane pin=%s", fetched)
	}
	again, err := batchowner.ExecuteChangeJoin(request, dependencies)
	if err != nil || len(again.History) != len(record.History) {
		t.Fatalf("repeat err=%v history %d -> %d", err, len(record.History), len(again.History))
	}
	// A change stacked on a change the lane holds joins; one whose parent is
	// neither on origin/main nor the lane's is refused, naming it (N-3).
	if err := os.WriteFile(filepath.Join(seat, "notes.md"), []byte("one\ntwo\nthree\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(seat, "commit", "-qam", "record: more notes\n\nMachine: m1e+human")
	stacked := run(seat, "rev-parse", "HEAD")
	run(seat, "update-ref", batchowner.ChangePinRef(stacked), stacked)
	if _, err := batchowner.ExecuteChangeJoin(batchowner.ChangeJoinRequest{SeatRoot: seat, LandingRoot: lane, Commit: stacked, At: time.Unix(12, 0)}, dependencies); err != nil {
		t.Fatalf("a change on a live change: %v", err)
	}
	run(seat, "checkout", "-q", "-b", "stray", base)
	run(seat, "commit", "-q", "--allow-empty", "-m", "stray local commit")
	stray := run(seat, "rev-parse", "HEAD")
	run(seat, "commit", "-q", "--allow-empty", "-m", "record: on a stray parent\n\nMachine: m1e+human")
	orphan := run(seat, "rev-parse", "HEAD")
	run(seat, "update-ref", batchowner.ChangePinRef(orphan), orphan)
	if _, err := batchowner.ExecuteChangeJoin(batchowner.ChangeJoinRequest{SeatRoot: seat, LandingRoot: lane, Commit: orphan, At: time.Unix(13, 0)}, dependencies); err == nil ||
		!strings.Contains(err.Error(), "BATCH_CHANGE_PARENT_UNKNOWN") || !strings.Contains(err.Error(), stray) {
		t.Fatalf("a change on a stray parent: %v", err)
	}
	run(seat, "checkout", "-q", "main")
	run(seat, "commit", "-q", "--allow-empty", "-m", "made by hand")
	bare := run(seat, "rev-parse", "HEAD")
	run(seat, "update-ref", batchowner.ChangePinRef(bare), bare)
	if _, err := batchowner.ExecuteChangeJoin(batchowner.ChangeJoinRequest{SeatRoot: seat, LandingRoot: lane, Commit: bare, At: time.Unix(11, 0)}, dependencies); err == nil ||
		!strings.Contains(err.Error(), "BATCH_CHANGE_UNREADABLE") || !strings.Contains(err.Error(), "names no machine and session") {
		t.Fatalf("a commit without a Machine trailer joined: %v", err)
	}
}

// TestLandingStatusVerbosePrintsReturnedChanges (U11b): landing status
// --verbose prints a change that left the current batch, its outcome, the
// asker's seat and the reason, next to the members.
func TestLandingStatusVerbosePrintsReturnedChanges(t *testing.T) {
	t.Parallel()
	view := lane.View{Batch: &lane.BatchView{ID: "b1", State: lane.BatchCollecting, Members: []lane.Member{{Goal: "change:abcdef012345", Seat: "m1e"}},
		Returned: []lane.Returned{{Goal: "change:1234567890ab", Seat: "ui", Outcome: batch.UnitEjected, Reason: "EJECTED from landing batch b1: TestNotes failed"}}},
		Spend: &lane.Spend{Account: "lane:0123456789ab", Attempts: 3, ReservedMinutes: 120}}
	root := "/lanes/landing"
	view.Root, view.Owner = &root, lane.OwnerView{State: lane.OwnerRunning}
	lines := oneSpaced(landingStatusPage(view, true))
	if !strings.Contains(lines, "spend 3 attempts · 120 reserved minutes · lane:0123456789ab, charged to no goal") {
		t.Fatalf("verbose status names no lane spend:\n%s", lines)
	}
	if !strings.Contains(lines, "change:abcdef012345 from m1e") || !strings.Contains(lines, "ejected change:1234567890ab from ui: EJECTED from landing batch b1: TestNotes failed") {
		t.Fatalf("verbose status:\n%s", lines)
	}
}

// TestChangeGateRecheckRereadsTheGoalBranchTip (U11b N-6): the gate of a
// change made in G's name is read again just before the push at G's branch
// tip as it is now, not at the tip recorded when the change joined.
func TestChangeGateRecheckRereadsTheGoalBranchTip(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	const current = "3333333333333333333333333333333333333333"
	file := &goal.GoalFile{Id: "goal-g", State: goal.StateClaimed, Tier: 4, Claimed: &goal.ClaimRecord{Machine: "m1b", Lineage: "other", Revision: 5}}
	humanWord(file, "01ARZ3NDEKTSV4RRFFQ69G5FW1", "review", "reviewed verdict=clear-to-land tip="+current+" record="+reviewBedRecord+" by=Wido")
	projection := goal.Projection{Tree: &goal.TreeGoals{Live: map[string]*goal.GoalFile{"goal-g": file}}}
	person := laneChangeUnit()
	person.Change = &batch.ChangeMember{Commit: laneChangeCommit, AskedBy: "m1e+human", Goal: "goal-g", GateTip: "1111111111111111111111111111111111111111"}
	var asked []string
	tip := func(_, goalID string) (string, error) { asked = append(asked, goalID); return current, nil }
	if err := batchowner.AuthorizeChangeWith(root, person, projection, tip); err != nil || len(asked) != 1 {
		t.Fatalf("gate at the re-read tip: err=%v asked=%v", err, asked)
	}
	unreadable := func(string, string) (string, error) { return "", errors.New("fetch goal/goal-g: network down") }
	if err := batchowner.AuthorizeChangeWith(root, person, projection, unreadable); err == nil {
		t.Fatal("an unreadable goal branch tip authorized the change")
	}
}

func noBranchTip(string, string) (string, error) { return "", nil }

// TestHeldRefusalKindsForTheLanding (U11b N-b): held's refusal of the series
// or the lane's configuration is a series refusal (the batch holds); one
// naming a member's commit ejects that member.
func TestHeldRefusalKindsForTheLanding(t *testing.T) {
	t.Parallel()
	cause := errors.New("held refused")
	for code, series := range map[string]bool{"endpoint-mismatch": true, "range-not-linear": true, "goal-item-not-held": false, "machine-trailer-malformed": false} {
		err := heldRefusalError(landing.HeldVerdict{Outcome: "refused", Refusal: &landing.HeldRefusal{Code: code, Commit: "c1"}}, cause)
		var held *batch.HeldSeriesRefusal
		var member *batch.HeldCommitRefusal
		if series != errors.As(err, &held) || series == errors.As(err, &member) {
			t.Fatalf("%s: %T %v", code, err, err)
		}
	}
}

// landingStatusPage is landing status's page of view, at the full width.
func landingStatusPage(view lane.View, verbose bool) string {
	page := textui.New(textui.Env{Width: textui.MaxWidth, Now: time.Now(), Zone: time.UTC, Verbose: verbose})
	(&intentInvocation{}).landingStatusView(view, false)(page)
	return page.String()
}
