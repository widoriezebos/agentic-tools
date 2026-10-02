package main

import (
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

// seatClaimBed is the intent bed with its held goal released to approved and,
// when next is set, a second approved goal: two ready goals for two machines.
func seatClaimBed(t *testing.T, next bool) *intentBed {
	t.Helper()
	bed := newIntentBed(t, false, nil)
	// This test process holds the checkout, so a claim binds its epoch.
	announceProofFixtureHolder(t, bed.root())
	released := bed.goalFile(bedGoal)
	if next {
		other := *released
		other.Id, other.State, other.Claimed = "next-ready", goal.StateApproved, nil
		other.History = slices.Clone(other.History)
		for index := range other.History {
			other.History[index].Targets = []string{other.Id}
		}
		bed.addGoal(&other)
	}
	released.State, released.Claimed = goal.StateApproved, nil
	bed.addGoal(released)
	return bed
}

// otherMachine is the bed's owners acting as another machine.
func otherMachine(bed *intentBed, machine string) intentOwners {
	owners := bed.owners()
	owners.dependencies.machine = func(string) (string, error) { return machine, nil }
	return owners
}

// claimTakenFirst has machine mac-other claim the bed's goal, as another
// seat would before this one's claim of the goal its brief names.
func claimTakenFirst(t *testing.T, bed *intentBed) {
	t.Helper()
	bed.lineage = "a1"
	code, result := bed.runJSON(otherMachine(bed, "mac-other"), "goal", "claim", bedGoal)
	if code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("machine mac-other's claim = %d %+v", code, result)
	}
	if file := bed.goalFile(bedGoal); file.State != goal.StateClaimed || file.Claimed == nil || file.Claimed.Machine != "mac-other" {
		t.Fatalf("the goal is not mac-other's: %+v", file.Claimed)
	}
}

// TestSeatClaimOfATakenGoalTakesTheNextReady: a steward seat's claim of the
// goal its brief names, which another machine took first, claims this
// machine's next ready goal instead, exactly what a claim without a goal
// picks, and says so on line 1 with the next command on line 2.
func TestSeatClaimOfATakenGoalTakesTheNextReady(t *testing.T) {
	t.Parallel()
	bed := seatClaimBed(t, true)
	claimTakenFirst(t, bed)
	bed.lineage = launch.SeatOwnerLineage
	code, result := bed.runJSON(bed.owners(), "goal", "claim", bedGoal)
	next := bed.goalFile("next-ready")
	if code != 0 || result.Outcome != intentConfirmed || next.State != goal.StateClaimed || next.Claimed == nil ||
		next.Claimed.Machine != "mac-cli" || next.Claimed.Lineage != launch.SeatOwnerLineage {
		t.Fatalf("the seat's claim of a taken goal = %d %+v; next-ready %+v", code, result, next.Claimed)
	}
	if want := bedGoal + " was taken by mac-other; claimed next-ready instead"; strings.Split(result.Summary, "\n")[0] != want {
		t.Fatalf("line 1 = %q, want %q", result.Summary, want)
	}
	if result.Next == nil || !slices.Equal(result.Next.Argv, []string{"metasystem", "goal", "show", "next-ready"}) {
		t.Fatalf("line 2 is not the next command for the claimed goal: %+v", result.Next)
	}
	if len(result.Targets) != 1 || result.Targets[0].ID != "next-ready" {
		t.Fatalf("the result names %+v, not the claimed goal", result.Targets)
	}
	if taken := bed.goalFile(bedGoal); taken.Claimed == nil || taken.Claimed.Machine != "mac-other" {
		t.Fatalf("the taken goal changed hands: %+v", taken.Claimed)
	}
}

// TestSeatClaimOfATakenGoalWithNothingReady: with an empty frontier the seat
// is told nothing is claimable, as a claim without a goal tells it.
func TestSeatClaimOfATakenGoalWithNothingReady(t *testing.T) {
	t.Parallel()
	bed := seatClaimBed(t, false)
	claimTakenFirst(t, bed)
	bed.lineage = launch.SeatOwnerLineage
	before := bed.publications()
	code, result := bed.runJSON(bed.owners(), "goal", "claim", bedGoal)
	bed.expectNoEffect(before, []string{"goal", "claim", bedGoal}, code, result)
	if want := bedGoal + " was taken by mac-other; no goal is ready for mac-cli; nothing was claimed"; result.Summary != want {
		t.Fatalf("summary = %q, want %q", result.Summary, want)
	}
	if result.Next == nil || !slices.Equal(result.Next.Argv, []string{"metasystem", "goal", "list"}) {
		t.Fatalf("line 2 = %+v", result.Next)
	}
}

// TestNamedClaimOfATakenGoalIsStillRefused: any other session names its goal
// deliberately, so a goal another machine holds is refused as before and
// nothing else is claimed.
func TestNamedClaimOfATakenGoalIsStillRefused(t *testing.T) {
	t.Parallel()
	bed := seatClaimBed(t, true)
	claimTakenFirst(t, bed)
	// A person at the terminal (no session lineage) and another session.
	for _, lineage := range []string{"", "m1"} {
		bed.lineage = lineage
		code, result := bed.runJSON(bed.owners(), "goal", "claim", bedGoal)
		if code == 0 || result.Outcome == intentConfirmed || strings.Contains(result.Summary, "instead") {
			t.Fatalf("a named claim of a taken goal by %q = %d %+v", lineage, code, result)
		}
	}
	if next := bed.goalFile("next-ready"); next.State != goal.StateApproved || next.Claimed != nil {
		t.Fatalf("a refused named claim claimed another goal: %+v", next)
	}
	if taken := bed.goalFile(bedGoal); taken.Claimed == nil || taken.Claimed.Machine != "mac-other" {
		t.Fatalf("the taken goal changed hands: %+v", taken.Claimed)
	}
}
