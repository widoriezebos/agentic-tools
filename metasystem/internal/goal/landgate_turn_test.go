package goal

// The holder's step on its turn (g1-s70 D3, g1-s69 SOL-S69-01): the seat that
// holds a claim is told, once, to land a due goal or to revise a sent-back
// one, under its own identity; the command it runs meets the gate then.

import (
	"strings"
	"testing"
	"time"
)

// holderFixture serves one goal file exactly as given, history and all.
func holderFixture(t *testing.T, machine string, f *GoalFile, now time.Time) *Store {
	t.Helper()
	root := t.TempDir()
	commits := newFakeGoalStore()
	seed := commits.commits[commits.canonical]
	files := servingFixtureFiles(machine, nil)
	files[goalsPrefix+f.Id+".md"] = RenderFile(f)
	seed.files = copyFakeFiles(files)
	commits.commits[commits.canonical] = seed
	client := commits.client()
	client.accepted = commits.canonical
	endpoint := Endpoint{Root: root, Remote: "local", Branch: LocalLedgerBranch, Repository: client}
	return &Store{Root: root, Now: func() time.Time { return now },
		projectionDeps: projectionDependencies{source: &projectionSource{endpoint: endpoint, machine: machine}}}
}

func holdersLanding(tier uint8) *GoalFile {
	f := tiered("due-goal", tier)
	f.Claimed = &ClaimRecord{Machine: "bed-m1", Lineage: "coordinator", At: "2026-08-20T10:05:00Z"}
	for index := range f.History {
		f.History[index].Actor = "bed-m1+coordinator"
	}
	return f
}

func TestTheHolderIsToldOnceToLandAnEligibleGoalOnItsTurn(t *testing.T) {
	t.Parallel()
	landedAt := time.Date(2026, 8, 20, 10, 6, 0, 0, time.UTC)
	options := TurnVerdictOptions{SeatActor: Actor{Machine: "bed-m1", Lineage: "coordinator"}}

	early := holderFixture(t, "bed-m1", holdersLanding(1), landedAt.Add(time.Hour))
	verdict, err := early.TurnVerdict(ScanResult{}, "holder-session", "", "main-1", options)
	if err != nil || strings.Contains(verdict.Display, "LANDING DUE") {
		t.Fatalf("a goal inside its grace time is due: %+v %v", verdict, err)
	}

	store := holderFixture(t, "bed-m1", holdersLanding(1), landedAt.Add(5*time.Hour))
	first, err := store.TurnVerdict(ScanResult{}, "holder-session", "", "main-1", options)
	if err != nil {
		t.Fatal(err)
	}
	want := "LANDING DUE due-goal, eligible under landing.review.auto-after=4h, tier 1 below human-from-tier=2: land it now: metasystem work land due-goal"
	if !first.ShouldBlock || !strings.Contains(first.Display, want) {
		t.Fatalf("the eligible goal does not block the holder's turn with its landing: %+v", first)
	}
	second, err := store.TurnVerdict(ScanResult{}, "holder-session", "", "main-1", options)
	if err != nil || second.ShouldBlock || !strings.Contains(second.Display, "LANDING DUE due-goal") || strings.Contains(second.Display, "next step") {
		t.Fatalf("the repeat blocks again instead of showing the line: %+v %v", second, err)
	}

	// Above the tier the clock never makes it due.
	above := holderFixture(t, "bed-m1", holdersLanding(2), landedAt.Add(100*time.Hour))
	if verdict, err := above.TurnVerdict(ScanResult{}, "holder-session", "", "main-1", options); err != nil || strings.Contains(verdict.Display, "LANDING DUE") {
		t.Fatalf("a goal at the tier became due by the clock: %+v %v", verdict, err)
	}

	// A claim that changed hands before the holder's turn is not this seat's.
	moved := holdersLanding(1)
	moved.Claimed.Machine = "bed-m2"
	elsewhere := holderFixture(t, "bed-m1", moved, landedAt.Add(5*time.Hour))
	if verdict, err := elsewhere.TurnVerdict(ScanResult{}, "holder-session", "", "main-1", options); err != nil || strings.Contains(verdict.Display, "LANDING DUE") {
		t.Fatalf("another machine's claim is due here: %+v %v", verdict, err)
	}
}

func TestTheHolderIsToldToReviseASentBackGoal(t *testing.T) {
	t.Parallel()
	f := holdersLanding(2)
	humanLine(f, "2026-08-20T11:00:00Z", "01J5X0000000000000000000S1-mac-ui-1a2b3c4d", "review",
		"reviewed verdict=send-back tip="+reviewedTip+" record="+reviewPath+" by=Wido brief="+BriefPathFor(reviewPath))
	store := holderFixture(t, "bed-m1", f, time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC))
	verdict, err := store.TurnVerdict(ScanResult{}, "holder-session", "", "main-1", TurnVerdictOptions{SeatActor: Actor{Machine: "bed-m1", Lineage: "coordinator"}})
	if err != nil {
		t.Fatal(err)
	}
	if !verdict.ShouldBlock || !strings.Contains(verdict.Display, "SENT BACK due-goal by Wido at 9c1f0a2b3c4d: revise it from the published brief now: metasystem work revise due-goal") {
		t.Fatalf("the send-back does not reach the holder's turn: %+v", verdict)
	}
}
