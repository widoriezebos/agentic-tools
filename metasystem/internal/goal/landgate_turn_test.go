package goal

// The holder's step on its Stop path (g1-s70 D3, SOL-S70-02): the seat that
// holds a claim takes the step itself, landing a due goal or revising a
// sent-back one through the public command under its own identity, and the
// Stop shows what was done or why it was refused; a taken step never blocks,
// since what stops it (a hold, a moved tip, a missing word) is the human's.

import (
	"strings"
	"testing"
	"time"
)

// holderFixture serves one goal file exactly as given, history and all, at the
// instant *now holds when each Stop reads it, with a step taker that records
// each step and answers it with answer.
func holderFixture(t *testing.T, machine string, f *GoalFile, now *time.Time, answer func(HolderStep) string) (*Store, *[]HolderStep) {
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
	taken := &[]HolderStep{}
	return &Store{Root: root, Now: func() time.Time { return *now },
		projectionDeps: projectionDependencies{source: &projectionSource{endpoint: endpoint, machine: machine}},
		TakeHolderStep: func(step HolderStep) string { *taken = append(*taken, step); return answer(step) }}, taken
}

func holdersLanding(tier uint8) *GoalFile {
	f := tiered("due-goal", tier)
	f.Claimed = &ClaimRecord{Machine: "bed-m1", Lineage: "coordinator", At: "2026-08-20T10:05:00Z"}
	for index := range f.History {
		f.History[index].Actor = "bed-m1+coordinator"
	}
	return f
}

func joined(step HolderStep) string { return "LANDED " + step.Goal + ": joined batch b-1" }

// firstStop passes the Stop on which the goal's own next step blocks once, an
// hour after the Landing record, before any landing is due.
func firstStop(t *testing.T, store *Store, taken *[]HolderStep) {
	t.Helper()
	verdict, err := store.TurnVerdict(ScanResult{}, "holder-session", "", "main-1", TurnVerdictOptions{SeatActor: Actor{Machine: "bed-m1", Lineage: "coordinator"}})
	if err != nil || len(*taken) != 0 || !verdict.ShouldBlock || !strings.Contains(verdict.Display, "the goal file names the next step") {
		t.Fatalf("the first Stop: %+v %v %+v", verdict, err, *taken)
	}
}

func TestTheHolderLandsADueGoalOnItsStopWithNoCommandTyped(t *testing.T) {
	t.Parallel()
	landedAt := time.Date(2026, 8, 20, 10, 6, 0, 0, time.UTC)
	options := TurnVerdictOptions{SeatActor: Actor{Machine: "bed-m1", Lineage: "coordinator"}}

	now := landedAt.Add(time.Hour)
	store, taken := holderFixture(t, "bed-m1", holdersLanding(1), &now, joined)
	firstStop(t, store, taken)
	now = landedAt.Add(5 * time.Hour)
	verdict, err := store.TurnVerdict(ScanResult{}, "holder-session", "", "main-1", options)
	if err != nil {
		t.Fatal(err)
	}
	want := HolderStep{Goal: "due-goal", Why: "eligible under landing.review.auto-after=4h, tier 1 below human-from-tier=2"}
	if len(*taken) != 1 || (*taken)[0] != want {
		t.Fatalf("the due landing was not taken on the Stop: %+v", *taken)
	}
	if verdict.ShouldBlock || !strings.Contains(verdict.Display, "LANDED due-goal: joined batch b-1") || strings.Contains(verdict.Display, "metasystem work land") {
		t.Fatalf("the taken step blocks, or is not shown, or is still advised: %+v", verdict)
	}

	// Above the tier the clock never makes it due.
	late := landedAt.Add(100 * time.Hour)
	above, taken := holderFixture(t, "bed-m1", holdersLanding(2), &late, joined)
	if verdict, err := above.TurnVerdict(ScanResult{}, "holder-session", "", "main-1", options); err != nil || len(*taken) != 0 || strings.Contains(verdict.Display, "LANDED") {
		t.Fatalf("a goal at the tier was landed by the clock: %+v %v", verdict, err)
	}

	// A claim that changed hands before the holder's Stop is not this seat's.
	moved := holdersLanding(1)
	moved.Claimed.Machine = "bed-m2"
	due := landedAt.Add(5 * time.Hour)
	elsewhere, taken := holderFixture(t, "bed-m1", moved, &due, joined)
	if verdict, err := elsewhere.TurnVerdict(ScanResult{}, "holder-session", "", "main-1", options); err != nil || len(*taken) != 0 {
		t.Fatalf("another machine's claim was landed here: %+v %v", verdict, err)
	}
}

func TestARefusedStepIsShownAtEveryStopAndNeverBlocks(t *testing.T) {
	t.Parallel()
	refused := func(step HolderStep) string {
		return "LANDING REFUSED " + step.Goal + " [LANDING_HELD_BY_SITTING]: goal due-goal is held by Wido's review sitting; it lands once the sitting ends, which releases it: metasystem goal review due-goal --release --record plans/reviews/review-of-due-goal.md"
	}
	now := time.Date(2026, 8, 20, 11, 6, 0, 0, time.UTC)
	store, taken := holderFixture(t, "bed-m1", holdersLanding(1), &now, refused)
	firstStop(t, store, taken)
	now = time.Date(2026, 8, 20, 16, 0, 0, 0, time.UTC)
	options := TurnVerdictOptions{SeatActor: Actor{Machine: "bed-m1", Lineage: "coordinator"}}
	for stop := 1; stop <= 2; stop++ {
		verdict, err := store.TurnVerdict(ScanResult{}, "holder-session", "", "main-1", options)
		if err != nil || verdict.ShouldBlock || !strings.Contains(verdict.Display, "LANDING REFUSED due-goal [LANDING_HELD_BY_SITTING]") {
			t.Fatalf("stop %d: the refusal blocks or is not shown: %+v %v", stop, verdict, err)
		}
		if len(*taken) != stop {
			t.Fatalf("stop %d: the step was taken %d times, want once per Stop while it stands", stop, len(*taken))
		}
	}
}

// A send-back is taken on the Stop; its revision's answer on the goal is what
// makes it once (the second Stop over the ledger is the command layer's test,
// cmd/metasystem/holder_step_test.go).
func TestTheHolderRevisesASentBackGoalOnItsStop(t *testing.T) {
	t.Parallel()
	f := holdersLanding(2)
	humanLine(f, "2026-08-20T11:00:00Z", "01J5X0000000000000000000S1-mac-ui-1a2b3c4d", "review",
		"reviewed verdict=send-back tip="+reviewedTip+" record="+reviewPath+" by=Wido brief="+BriefPathFor(reviewPath))
	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	store, taken := holderFixture(t, "bed-m1", f, &now, func(step HolderStep) string {
		return "REVISION STARTED " + step.Goal + ": attempt 3 is recorded on the goal"
	})
	options := TurnVerdictOptions{SeatActor: Actor{Machine: "bed-m1", Lineage: "coordinator"}}
	verdict, err := store.TurnVerdict(ScanResult{}, "holder-session", "", "main-1", options)
	if err != nil {
		t.Fatal(err)
	}
	if len(*taken) != 1 || !(*taken)[0].Revise || (*taken)[0].Goal != "due-goal" || (*taken)[0].Why != "sent back by Wido at 9c1f0a2b3c4d" {
		t.Fatalf("the send-back's revision was not taken on the Stop: %+v", *taken)
	}
	if !strings.Contains(verdict.Display, "REVISION STARTED due-goal: attempt 3 is recorded on the goal") || strings.Contains(verdict.Display, "metasystem work revise") {
		t.Fatalf("the taken revision is not shown, or is still advised: %+v", verdict)
	}
	// The first Stop blocked for the goal's own next step alone; the next one
	// does not block for the revision.
	if again, err := store.TurnVerdict(ScanResult{}, "holder-session", "", "main-1", options); err != nil || again.ShouldBlock {
		t.Fatalf("the taken revision blocks the Stop: %+v %v", again, err)
	}
}
