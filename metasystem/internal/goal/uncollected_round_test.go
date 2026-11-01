package goal

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestAHeldUncollectedRoundBlocksTheStop(t *testing.T) {
	t.Parallel()
	for _, row := range []string{"held", "other lineage", "unheld", "unreadable", "unresolved", "nil reader"} {
		t.Run(row, func(t *testing.T) {
			t.Parallel()
			files := map[string]*GoalFile{}
			for _, id := range []string{"held", "other"} {
				lineage := "seat-lineage"
				if id == "other" {
					lineage = "another-seat"
				}
				files[id] = &GoalFile{Id: id, State: StateClaimed, Intent: "Finish it", Origin: "main", OpenedAt: "2026-08-23T00:00:00Z", Revision: 2, Claimed: &ClaimRecord{Machine: "bed-m1", Lineage: lineage, At: "2026-08-23T01:00:00Z"}}
			}
			store, _, _ := fakeServingFixture(t, "bed-m1", files)
			store.Now = func() time.Time { return time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC) }
			store.ResolveIdleSeat = func() (Actor, int64, error) {
				if row == "unresolved" {
					return Actor{}, 0, errors.New("identity unavailable")
				}
				return questionSeat.SeatActor, 7, nil
			}
			calls := 0
			store.UncollectedRuns = func(goals []string) ([]UncollectedRun, error) {
				calls++
				if !slices.Equal(goals, []string{"held"}) {
					t.Fatalf("queried another seat's claim: %v", goals)
				}
				if row == "unreadable" {
					return []UncollectedRun{{Run: "must-ignore"}}, errors.New("store unreadable")
				}
				id := "held"
				if row == "other lineage" {
					id = "other"
				}
				if row == "unheld" {
					id = "foreign"
				}
				if !slices.Contains(goals, id) {
					return nil, nil
				}
				return []UncollectedRun{{Goal: id, Run: "run-1", LaunchID: "launch-1", Step: "build"}}, nil
			}
			if row == "nil reader" {
				store.UncollectedRuns = nil
			}
			store.OpenQuestions = openQuestions(OpenQuestion{About: "machine", Machine: "bed-m1", Lineage: "seat-lineage", ID: "waiting"})
			for stop := 0; stop < 2; stop++ {
				v, err := store.TurnVerdict(ScanResult{}, "collect-session", "", "main-1", questionSeat)
				if err != nil {
					t.Fatal(err)
				}
				blocked := v.BlockSource != nil && *v.BlockSource == "unwatched-work"
				if blocked != (row == "held" && stop == 0) || v.ShouldBlock != (row == "held" && stop == 0) {
					t.Fatalf("stop %d: %+v", stop, v)
				}
				if blocked && (!v.ShouldBlock || !strings.Contains(v.Display, "metasystem work wait run:run-1 --timeout 30m")) {
					t.Fatalf("no collection command: %+v", v)
				}
				if (row == "unreadable" || row == "unresolved" || row == "nil reader") && !strings.Contains(strings.Join(v.Diagnostics, "\n"), "uncollected rounds:") {
					t.Fatalf("no diagnostic: %+v", v)
				}
			}
			if row == "unresolved" && calls != 0 {
				t.Fatal("queried without a seat identity")
			}
		})
	}
}
