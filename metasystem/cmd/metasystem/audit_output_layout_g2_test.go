package main

import (
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/realpath"
)

// G2's layout goldens (output-style.md §6.7, §6.8): goal list and goal show
// on a small ledger of every state.
func g2LayoutCases() []layoutCase {
	return []layoutCase{
		{name: "goal-list", args: []string{"goal", "list"}, bed: goalLayoutBed},
		{name: "goal-list-all", args: []string{"goal", "list", "--all"}, bed: goalLayoutBed},
		{name: "goal-list-refusal", args: []string{"goal", "list", "--label", "Not A Label"}, bed: goalLayoutBed},
		{name: "goal-list-tiers", args: []string{"goal", "list", "--tiers"}, bed: goalLayoutBed},
		{name: "goal-list-ready", args: []string{"goal", "list", "--ready"}, bed: goalLayoutBed},
		{name: "goal-show", args: []string{"goal", "show", bedGoal}, bed: goalLayoutBed},
		{name: "goal-show-stopped", args: []string{"goal", "show", bedGoal}, bed: goalStoppedLayoutBed},
		{name: "goal-show-refusal", args: []string{"goal", "show", "no-such-goal"}, bed: goalLayoutBed},
		{name: "goal-pause", args: []string{"goal", "pause", bedGoal, "--reason", "wait for the vendor's 1.2 release", "--lineage", "m1"}, bed: goalLayoutBed},
		{name: "goal-pause-again", args: []string{"goal", "pause", "wait-for-the-vendor", "--reason", "again", "--lineage", "m1"}, bed: goalLayoutBed},
		{name: "goal-pause-refusal", args: []string{"goal", "pause", "no-such-goal", "--reason", "later", "--lineage", "m1"}, bed: goalLayoutBed},
		{name: "incident-list", args: []string{"incident", "list"}, bed: incidentLayoutBed},
		{name: "incident-list-empty", args: []string{"incident", "list"}, bed: goalStoppedLayoutBed},
		{name: "incident-list-refusal", args: []string{"incident", "list"}, bed: outsideLayoutBed},
	}
}

// goalLayoutBed is a ledger with a goal in each state: two claimed, two
// approved, three queued (two at priority 1, one pinned) and one parked.
func goalLayoutBed(t *testing.T) layoutBed {
	b := newIntentBed(t, false, func(file *goal.GoalFile) {
		file.Priority, file.Sequence = 1, 1
		file.NextStep = "Push 17 landed the batch lane; confirm the ledger tip after the next sync. Then close the goal."
	})
	template := *b.goalFile(bedGoal)
	// Claimed and approved goals copy the bed's claimed goal, whose claim,
	// approval and budget records agree with its history.
	like := func(id string, tier, priority uint8, sequence uint64, next string, amend func(*goal.GoalFile)) {
		file := template
		file.Id, file.Tier, file.Priority, file.Sequence, file.NextStep = id, tier, priority, sequence, next
		file.History = append([]goal.HistoryLine(nil), template.History...)
		for index := range file.History {
			file.History[index].Targets = []string{id}
		}
		amend(&file)
		b.addGoal(&file)
	}
	add := func(id, state string, tier, priority uint8, sequence uint64, next string, amend func(*goal.GoalFile)) {
		file := queuedIntentGoal(id, tier)
		file.State, file.Priority, file.Sequence, file.NextStep = state, priority, sequence, next
		if amend != nil {
			amend(file)
		}
		b.addGoal(file)
	}
	like("machinery-runs-unattended-on-codex", 3, 3, 1, "Wido rejected the first result: every public command must sit on one grammar.", func(file *goal.GoalFile) {
		claim := *template.Claimed
		claim.Machine = "ui"
		file.Claimed = &claim
	})
	approved := func(file *goal.GoalFile) { file.State, file.Claimed = goal.StateApproved, nil }
	like("test-environment-edges-are-closed", template.Tier, 1, 2, "Free, tier 1.", approved)
	like("ask-what-happened-follow-ups", template.Tier, 0, 0, "Keep the waiting trouble on the room's page until a person answers it.", approved)
	add("one-steward-per-checkout-on-its-own-root", goal.StateQueued, 3, 1, 3, "Find how the second steward started.", func(file *goal.GoalFile) {
		file.Pinned = "m1e"
	})
	add("receipt-writer-follows-the-worktree", goal.StateQueued, 2, 1, 4, "Locate the root resolution.", nil)
	add("fleet-doctor-repairs-what-stops-other-seats", goal.StateQueued, 3, 2, 1, "Design first.", nil)
	add("wait-for-the-vendor", goal.StateParked, 2, 0, 0, "Upgrade once 1.2 ships.", func(file *goal.GoalFile) {
		file.Parked = &goal.ParkRecord{By: "human:wido", At: "2026-09-28T10:00:00Z", Because: "wait for the vendor's 1.2 release"}
	})
	owners := b.owners()
	root := realpath.Resolve(b.root())
	return layoutBed{owners: owners, cwd: b.root(), replace: layoutPaths(b.root(), root, "/Users/wido/GitHub/agentic-tools-m1e")}
}

// goalStoppedLayoutBed is the bed's goal stopped by its elapsed budget.
func goalStoppedLayoutBed(t *testing.T) layoutBed {
	b := newIntentBed(t, true, nil)
	root := realpath.Resolve(b.root())
	return layoutBed{owners: b.owners(), cwd: b.root(), replace: layoutPaths(b.root(), root, "/Users/wido/GitHub/agentic-tools-m1e")}
}

// incidentLayoutBed is a register with one unowned red on main, one owned
// by its fix goal, and a pending flake seen only on a batch tip.
func incidentLayoutBed(t *testing.T) layoutBed {
	b := newIntentBed(t, false, nil)
	stamp := layoutNow.Add(-3 * time.Hour).Format(time.RFC3339)
	sighting := goal.TrunkRedSighting{Attempt: "attempt-1", Batch: "batch-1", BaseCommit: "base-1", SeenAt: stamp,
		Opid: goal.Opid("01J5X0000000000000000000X1", "mac-cli", "m1")}
	register := goal.RenderTrunkRed([]goal.TrunkRedEntry{
		{ID: "tr-fast-main000001", Identity: "tr-fast-main000001", Group: "fast", Status: "failed", Failures: []goal.TrunkRedFailure{},
			Sightings: []goal.TrunkRedSighting{sighting}, Holds: []string{"batch-1"}, Opened: stamp},
		{ID: "tr-deep-main000003", Identity: "tr-deep-main000003", Group: "deep", Status: "failed", Failures: []goal.TrunkRedFailure{},
			Sightings: []goal.TrunkRedSighting{sighting}, Owner: goal.TrunkRedOwner{Machine: "mac-other", Since: stamp, How: "joiner"},
			FixGoal: bedGoal, Opened: stamp},
		{ID: "tr-fast-tip0000002", Identity: "tr-fast-tip0000002", Class: goal.TrunkRedClassPendingFlake, Group: "fast", Status: "failed",
			Failures: []goal.TrunkRedFailure{}, Sightings: []goal.TrunkRedSighting{sighting}, Opened: stamp},
	})
	parent := b.repo.accepted
	tip, err := b.repo.Build(goal.Opid("01J5X0000000000000000000X2", "mac-cli", "m1"), parent,
		[]goal.Change{{Path: "plans/goals/trunk-red.json", Content: register}}, "incident layout fixture")
	if err != nil {
		t.Fatal(err)
	}
	if outcome, err := b.repo.Publish(parent, tip); err != nil || outcome != goal.CASLanded {
		t.Fatalf("publish incident fixture: %v %v", outcome, err)
	}
	if err := b.repo.AcceptedCAS(parent, tip); err != nil {
		t.Fatal(err)
	}
	root := realpath.Resolve(b.root())
	return layoutBed{owners: b.owners(), cwd: b.root(), replace: layoutPaths(b.root(), root, "/Users/wido/GitHub/agentic-tools-m1e")}
}
