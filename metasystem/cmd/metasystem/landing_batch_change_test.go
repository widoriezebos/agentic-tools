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
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/adapter"
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

// TestBatchChangeProofIsChargedToTheGoalMember (U11b): a sealed batch whose
// last member is a change plans and launches its tip proof charged to the
// last goal member, at that member's sealed revisions, on the tip tree that
// holds the change; the change is never planned as a goal.
func TestBatchChangeProofIsChargedToTheGoalMember(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const id = "01j5x00000000000000000ba77"
	record := batch.Record{Schema: 1, BatchID: id, State: batch.StateSealed, BaseTree: "base-tree", TipTree: "tip-tree",
		PrefixTrees: []string{"goal-tree", "tip-tree"}, SelectedGroups: []string{"app-a"},
		Units: []batch.Unit{laneGoalUnit(), laneChangeUnit()},
		Seal:  map[string]batch.Claim{"goal-a": {Revision: 7, AccountingRevision: 5}, batch.ChangeID(laneChangeCommit): {}}}
	store := batch.NewStore(root, nil)
	if err := store.Create(record); err != nil {
		t.Fatal(err)
	}
	var planned []string
	var launched []batchProofLaunch
	dependencies := batchProofDependencies{
		rearm:    func(string, string) error { return nil },
		attempts: func(string) ([]proofrun.Attempt, error) { return nil, nil },
		plan: func(_, goalID, tree string, _ testpolicy.Mode) (testpolicy.Plan, error) {
			planned = append(planned, goalID+"@"+tree)
			return testpolicy.Plan{SelectedGroups: []string{"app-a"}, ExecutedMode: testpolicy.ModeStandard, RequiredMode: testpolicy.ModeStandard}, nil
		},
		launch: func(request batchProofLaunch) (proofrun.TestResult, error) {
			launched = append(launched, request)
			return proofrun.TestResult{AttemptID: "tip-attempt", CandidateTree: request.Tree, Delivery: proofrun.DeliveryJudgment{Sufficient: true}}, nil
		},
	}
	if err := executeBatchProof(root, id, "landing+owner", "window", "token", proofrun.LoadSample{}, time.Unix(10, 0), dependencies); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(planned, []string{"goal-a@tip-tree"}) || len(launched) != 1 {
		t.Fatalf("planned=%v launched=%+v", planned, launched)
	}
	if got := launched[0]; got.GoalID != "goal-a" || got.GoalRevision != 7 || got.AccountingRevision != 5 || got.Tree != "tip-tree" {
		t.Fatalf("tip proof launch=%+v", got)
	}
	if landed, err := store.Load(id); err != nil || landed.State != batch.StateLanding {
		t.Fatalf("after proof: state=%s err=%v", landed.State, err)
	}
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
	forecast, err := forecastBatchCostWith(root, candidate, nil, time.Unix(10, 0),
		func(_ string, units []batch.Unit, tree string) (batch.PrefixDecision, error) {
			decided = append(decided, units[len(units)-1].GoalID+"@"+tree)
			return batch.PrefixDecision{Groups: []string{"app-a"}}, nil
		},
		func(_ string, selection costSelection, _ uint64) (costSelectionEvidence, error) {
			charged = append(charged, selection.ID+"="+selection.GoalID)
			return costSelectionEvidence{Request: batch.CostForecastRequest{ID: selection.ID, Kind: selection.Kind, Tree: selection.Tree, ChargeGoal: selection.GoalID}}, nil
		},
		func(_ string, unit batch.Unit, _ *batch.Unit, _ time.Time) (batchCostBudgetProjection, error) {
			budgets = append(budgets, unit.GoalID)
			return batchCostBudgetProjection{}, errors.New("budget read for " + unit.GoalID)
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
// change before it; a series verified at landing checks the change's tip
// with the charge member and asks no receipt of a change.
func TestBatchChangePrefixDecisionPlansGoalMembersOnly(t *testing.T) {
	t.Parallel()
	change := laneChangeUnit()
	var plans []string
	decision, err := planPrefixDecisionWith("", []batch.Unit{laneGoalUnit(), change}, "tip-tree",
		func(_, goalID, tree string, _ testpolicy.Mode, _ []string) (testingPlanOutput, error) {
			plans = append(plans, goalID)
			return testingPlanOutput{CandidateTree: tree, ContractDigest: "c", PolicyBaseCommit: "p",
				Plan: testpolicy.Plan{SelectedGroups: []string{"app-a"}, RequiredGroups: []string{"app-a"}}}, nil
		})
	if err != nil || !slices.Equal(plans, []string{"goal-a"}) || !slices.Equal(decision.Groups, []string{"app-a"}) {
		t.Fatalf("decision=%+v plans=%v err=%v", decision, plans, err)
	}
	if _, err := planPrefixDecisionWith("", []batch.Unit{change}, "tip-tree", nil); err == nil || !strings.Contains(err.Error(), "BATCH_PREFIX_PLAN_REFUSED") {
		t.Fatalf("a change-only prefix planned: %v", err)
	}
	record := batch.Record{BatchID: "01j5x00000000000000000ba79", BaseTree: "base-tree", TipTree: "tip-tree", SelectedGroups: []string{"app-a"},
		Units:    []batch.Unit{change, laneGoalUnit(), change},
		Receipts: map[string]batch.PrefixReceipt{}, Seal: map[string]batch.Claim{"goal-a": {Revision: 7, AccountingRevision: 5}, change.GoalID: {}}}
	record.Units[2].GoalID, record.Units[2].Chain, record.Units[2].Change = "change:111111111111", "change:111111111111", &batch.ChangeMember{Commit: "111111111111" + strings.Repeat("0", 28)}
	decide := func(_ string, units []batch.Unit, tree string) (batch.PrefixDecision, error) {
		if _, ok := batch.ChargeMember(units); !ok {
			return batch.PrefixDecision{}, errors.New("planned a prefix of changes only at " + tree)
		}
		return batch.PrefixDecision{Groups: []string{"app-a"}}, nil
	}
	var verified []string
	verify := func(_ string, unit batch.Unit, tree string, _ batch.PrefixDecision) error {
		verified = append(verified, unit.GoalID+"@"+tree)
		return nil
	}
	record.Receipts["goal-a"] = batch.PrefixReceipt{GoalID: "goal-a", Tree: "goal-tree"}
	id, err := batch.PrefixDecisionID(record.BaseTree, "goal-tree", record.Units[:2], record.Seal, batch.PrefixDecision{Groups: []string{"app-a"}})
	if err != nil {
		t.Fatal(err)
	}
	receipt := record.Receipts["goal-a"]
	receipt.DecisionID = id
	record.Receipts["goal-a"] = receipt
	if err := verifyBatchSeriesWith("", record, []string{"change-tree", "goal-tree", "tip-tree"}, decide, verify); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(verified, []string{"goal-a@goal-tree", "goal-a@tip-tree"}) {
		t.Fatalf("verified=%v", verified)
	}
}

// TestBatchChangeAuthorityRechecksItsGoal (U11b): a goal-less change needs no
// ledger; a change made in G's name leaves the batch when G's claim is no
// longer the asker's at the committed revision.
func TestBatchChangeAuthorityRechecksItsGoal(t *testing.T) {
	t.Parallel()
	change := laneChangeUnit()
	record := batch.Record{BatchID: "01j5x00000000000000000ba80"}
	if err := authorizeBatchMemberInProjection("", time.Unix(10, 0), record, change, goal.Projection{}); err != nil {
		t.Fatalf("goal-less change: %v", err)
	}
	named := change
	named.Change = &batch.ChangeMember{Commit: laneChangeCommit, AskedBy: "m1e+seat", Goal: "goal-g", GoalRevision: 4}
	moved := goal.Projection{Tree: &goal.TreeGoals{Live: map[string]*goal.GoalFile{"goal-g": {Id: "goal-g", State: goal.StateClaimed,
		Claimed: &goal.ClaimRecord{Machine: "m1b", Lineage: "other", Revision: 5}}}}}
	err := authorizeBatchMemberInProjection("", time.Unix(10, 0), record, named, moved)
	var revision *batch.PrefixRevisionRefusal
	if !errors.As(err, &revision) || !strings.Contains(err.Error(), "goal-g") {
		t.Fatalf("moved claim: %v", err)
	}
	if err := authorizeBatchMemberInProjection("", time.Unix(10, 0), record, named, goal.Projection{}); err == nil {
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
	if err := authorizeBatchMemberInProjection(root, time.Unix(10, 0), record, person, moved); err != nil {
		t.Fatalf("a person's change in a goal's name below the human tier: %v", err)
	}
	moved.Tree.Live["goal-g"].Tier = 4
	if err := authorizeBatchMemberInProjection(root, time.Unix(10, 0), record, person, moved); !errors.As(err, &revision) || !strings.Contains(err.Error(), "waits for a person") {
		t.Fatalf("a person's change past its goal's gate: %v", err)
	}
}

// TestBatchChangeLandingSeamsReplayAndFindTheTrailer (U11b): the landing
// owner's seams replay a change and find it on origin by its Landing-Change
// trailer.
func TestBatchChangeLandingSeamsReplayAndFindTheTrailer(t *testing.T) {
	t.Parallel()
	change := laneChangeUnit()
	seams := batchLandSeamsWithRead(t.TempDir(), "01j5x00000000000000000ba81", batch.Record{}, "base", "landing+owner", gitOutput, batchCommitBoundary)
	if seams.ReplayChange == nil {
		t.Fatal("the landing seams replay no change")
	}
	log := "c0ffee\x00land goal-a\n\nGoal-Source: x\n\x00beef\x00record: notes\n\nMachine: m1e+human\nLanding-Change: " + change.GoalID + "\n\x00"
	recovery := batchRecoverySeamsWithGit(t.TempDir(), batch.NewStore(t.TempDir(), nil), "01j5x00000000000000000ba81", time.Unix(10, 0),
		func(string, ...string) (string, error) { return log, nil })
	commit, found, err := recovery.OriginChange(change)
	if err != nil || !found || commit != "beef" {
		t.Fatalf("origin change=%q found=%v err=%v", commit, found, err)
	}
	other := change
	other.GoalID = "change:000000000000"
	if _, found, err := recovery.OriginChange(other); err != nil || found {
		t.Fatalf("another change found=%v err=%v", found, err)
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
	run(seat, "update-ref", changePinRef(commit), commit)
	ensured := 0
	dependencies := productionChangeJoinDependencies()
	dependencies.base = func(root string) (string, error) { return run(root, "rev-parse", "HEAD^{tree}"), nil }
	dependencies.mint = func() (string, error) { return "01j5x00000000000000000ba82", nil }
	dependencies.protectedTests = func(string, string, string) error { return nil }
	dependencies.closure = func(string, string, string) *adapter.Closure { return nil }
	dependencies.ensure = func(string) error { ensured++; return nil }
	request := changeJoinRequest{SeatRoot: seat, LandingRoot: lane, Commit: commit, At: time.Unix(10, 0)}
	record, err := executeChangeJoin(request, dependencies)
	if err != nil {
		t.Fatal(err)
	}
	if record.BatchID != "01j5x00000000000000000ba82" || record.State != batch.StateOpen || len(record.Units) != 1 || ensured != 1 {
		t.Fatalf("record=%+v ensured=%d", record, ensured)
	}
	unit := record.Units[0]
	if unit.GoalID != batch.ChangeID(commit) || unit.State != batch.UnitJoined || unit.Claim.Machine != "m1e" || unit.Claim.Lineage != "human" ||
		unit.Change.AskedBy != "m1e+human" || unit.Change.Subject != "record: notes" || unit.Change.Parent != base || !slices.Equal(unit.ChangedPaths, []string{"notes.md"}) {
		t.Fatalf("change unit=%+v change=%+v", unit, unit.Change)
	}
	if fetched := run(lane, "rev-parse", changePinRef(commit)); fetched != commit {
		t.Fatalf("lane pin=%s", fetched)
	}
	again, err := executeChangeJoin(request, dependencies)
	if err != nil || len(again.History) != len(record.History) {
		t.Fatalf("repeat err=%v history %d -> %d", err, len(record.History), len(again.History))
	}
	run(seat, "commit", "-q", "--allow-empty", "-m", "made by hand")
	bare := run(seat, "rev-parse", "HEAD")
	run(seat, "update-ref", changePinRef(bare), bare)
	if _, err := executeChangeJoin(changeJoinRequest{SeatRoot: seat, LandingRoot: lane, Commit: bare, At: time.Unix(11, 0)}, dependencies); err == nil ||
		!strings.Contains(err.Error(), "BATCH_CHANGE_UNREADABLE") || !strings.Contains(err.Error(), "commit boundary") {
		t.Fatalf("a commit without a Machine trailer joined: %v", err)
	}
}

// TestLandingStatusVerbosePrintsReturnedChanges (U11b): landing status
// --verbose prints a change that left the current batch, its outcome, the
// asker's seat and the reason, next to the members.
func TestLandingStatusVerbosePrintsReturnedChanges(t *testing.T) {
	t.Parallel()
	view := lane.View{Batch: &lane.BatchView{ID: "b1", State: lane.BatchCollecting, Members: []lane.Member{{Goal: "change:abcdef012345", Seat: "m1e"}},
		Returned: []lane.Returned{{Goal: "change:1234567890ab", Seat: "ui", Outcome: batch.UnitEjected, Reason: "EJECTED from landing batch b1: TestNotes failed"}}}}
	lines := strings.Join(landingViewDetail(view), "\n")
	if !strings.Contains(lines, "  member change:abcdef012345 from m1e") || !strings.Contains(lines, "  ejected change:1234567890ab from ui: EJECTED from landing batch b1: TestNotes failed") {
		t.Fatalf("verbose status:\n%s", lines)
	}
}
