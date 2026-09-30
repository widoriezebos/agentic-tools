package landing

import "testing"

// held accepts the lane's own commits only when landing begin listed them
// (design r10 K4, K-c): a listed Lane-Integration commit needs no goal
// binding, a listed Lane-Resolved replay keeps its member's binding (the
// goal must still be held at the bound revision) though the lane authored
// it, and a Lane-* commit the series did not list is refused, in the
// lane's check and in a seat's, so no one forges a lane commit past held.
func TestHeldAcceptsOnlyListedLaneCommits(t *testing.T) {
	t.Parallel()
	f := newObserveFixture(t)
	writeHeldGoalForPush(f, "ship-widget", "m9", "L1", 3)
	commitHeldSetup(f, "claim ship-widget", "plans/goals/ship-widget.md")
	base := f.git("rev-parse", "HEAD")
	resolved := commitHeldFixtureTrailers(f, "member replay with the lane's resolution",
		"Machine: landing+landing-lane", "Goal-Item: ship-widget", "Goal-Revision: 3", "Lane-Resolved: ship-widget")
	integration := commitHeldFixtureTrailers(f, "seam fix", "Machine: landing+landing-lane", "Lane-Integration: batch-1")

	listed, err := HeldLane(f.root, base, integration, "origin", "refs/heads/main", []string{resolved, integration})
	if err != nil || listed.Outcome != "ok" || listed.ExitCode != 0 || listed.Commits != 2 {
		t.Fatalf("the listed lane series = %+v %v; want ok", listed, err)
	}
	unlisted, err := HeldLane(f.root, base, integration, "origin", "refs/heads/main", []string{resolved})
	if err != nil {
		t.Fatal(err)
	}
	assertHeldRefusal(t, unlisted, 1, "lane-commit-unlisted", integration, "Lane-Integration")
	seat := requireHeldVerdict(t, f, base, resolved, "origin", "refs/heads/main")
	assertHeldRefusal(t, seat, 1, "lane-commit-unlisted", resolved, "Lane-Resolved")

	// A resolved replay whose member no longer holds its goal is refused
	// like the member's own commit.
	moved := newObserveFixture(t)
	writeHeldGoalForPush(moved, "ship-widget", "m9", "L1", 5)
	commitHeldSetup(moved, "claim ship-widget", "plans/goals/ship-widget.md")
	movedBase := moved.git("rev-parse", "HEAD")
	stale := commitHeldFixtureTrailers(moved, "stale replay",
		"Machine: landing+landing-lane", "Goal-Item: ship-widget", "Goal-Revision: 3", "Lane-Resolved: ship-widget")
	verdict, err := HeldLane(moved.root, movedBase, stale, "origin", "refs/heads/main", []string{stale})
	if err != nil {
		t.Fatal(err)
	}
	assertHeldRefusal(t, verdict, 1, "goal-revision-moved", stale, "claimed at revision 5")
}
