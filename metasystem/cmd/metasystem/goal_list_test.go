package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

func TestGoalListSummaryTruncatesAtWholeLinesAndCountsOmittedGoals(t *testing.T) {
	grouped := map[string][]*goal.GoalFile{}
	for i := 999; i >= 0; i-- {
		grouped[goal.StateQueued] = append(grouped[goal.StateQueued], &goal.GoalFile{
			Id: fmt.Sprintf("goal-%04d", i), State: goal.StateQueued, Priority: 1, Sequence: uint64(i + 1), Tier: 2,
			NextStep: strings.Repeat("界", 130),
		})
	}
	output := goalListSummary(grouped, syncedListStates, "accepted-tip", []string{"projection notice"}, false, goal.ApprovalHorizon{})
	lines := strings.Split(strings.TrimSuffix(output, "\n"), "\n")
	printed := len(lines) - 3
	if len(output) > 64*1024 || !utf8.ValidString(output) || printed == 0 || printed >= 1000 {
		t.Fatalf("summary bound failed: bytes=%d rows=%d", len(output), printed)
	}
	for i, line := range lines[2 : len(lines)-1] {
		want := fmt.Sprintf("%d:%d queued tier 2 goal-%04d pin=- claim=- :: %s...", 1, i+1, i, strings.Repeat("界", 117))
		if line != want {
			t.Fatalf("row %d is incomplete or out of order: %q", i, line)
		}
	}
	wantFooter := fmt.Sprintf("... %d more; run with --json > file for the records", 1000-printed)
	if lines[len(lines)-1] != wantFooter {
		t.Fatalf("omission count: got %q want %q", lines[len(lines)-1], wantFooter)
	}
}

type goalListRepositoryFixture struct {
	*obligationCommandFixture
	resolutions int
}

func (f *goalListRepositoryFixture) resolve(root string) (goal.Endpoint, error) {
	f.t.Helper()
	if root != f.root() {
		f.t.Fatalf("endpoint root = %q, want %q", root, f.root())
	}
	f.resolutions++
	return goal.Endpoint{Root: root, Remote: "local", Branch: goal.LocalLedgerBranch, Repository: f.repo}, nil
}

func goalListHistoryFixture(t *testing.T) *goalListRepositoryFixture {
	t.Helper()
	base := newObligationCommandFixture(t)
	root := base.root()
	standingPath := filepath.Join(root, "plans", "goals", "standing-validation.md")
	data, err := os.ReadFile(standingPath)
	if err != nil {
		t.Fatal(err)
	}
	standing, problems := goal.ParseFile(data)
	if len(problems) != 0 {
		t.Fatal(problems)
	}
	approved := commandApprovedPriorityGoal("approved-one", 0, 0, "seat-a")
	approved.NextStep = "Check approval. Then continue."
	parked := commandPriorityGoal("parked-one", goal.StateParked)
	parked.NextStep = "Wait for input. Then resume."
	parked.Parked = &goal.ParkRecord{By: "human:Wido", At: "2026-08-30T09:00:00Z", Because: "Waiting for input."}
	done := commandPriorityGoal("done-one", goal.StateDone)
	done.Conclude = "Finished."
	files := []*goal.GoalFile{standing, approved, parked, done}
	for i, id := range []string{"z-first", "a-second", "b-third", "c-unranked", "d-unranked"} {
		file := commandPriorityGoal(id, goal.StateQueued)
		file.NextStep = []string{"First step. Second step.", strings.Repeat("界", 130), "Continue? Then inspect.", "", "Last step! Then inspect."}[i]
		if i < 2 {
			file.Priority, file.Sequence = 1, uint64(i+1)
		} else if i == 2 {
			file.Priority, file.Sequence = 2, 1
		}
		if i == 0 {
			file.Labels = []string{"selected", "shared"}
		} else if i == 1 {
			file.Labels = []string{"shared"}
		}
		files = append(files, file)
	}
	changes := make([]goal.Change, 0, len(files))
	for _, file := range files {
		for i := 0; i < 64; i++ {
			file.History = append(file.History, goal.HistoryLine{
				At: "2026-09-01T10:00:00Z", Opid: goal.Opid(fmt.Sprintf("%026d", i), "mac-cli", file.Id),
				Verb: "edit", Actor: "mac-cli+m1", Targets: []string{file.Id}, Keep: -1,
				Reason: strings.Repeat("history payload ", 128),
			})
		}
		file.Revision = uint64(len(file.History))
		path := filepath.Join(root, "plans", "goals", file.Id+".md")
		if file.State == goal.StateDone {
			path = filepath.Join(root, "records", "goals", file.Id+".md")
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			t.Fatal(err)
		}
		changes = append(changes, goal.Change{Path: filepath.ToSlash(relative), Content: goal.RenderFile(file)})
	}
	parent := base.repo.accepted
	opid := goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FAC", "mac-cli", "m1")
	tip, err := base.repo.Build(opid, parent, changes, "large history fixture")
	if err != nil {
		t.Fatal(err)
	}
	if outcome, err := base.repo.Publish(parent, tip); err != nil || outcome != goal.CASLanded {
		t.Fatalf("publish history fixture: outcome=%v error=%v", outcome, err)
	}
	if err := base.repo.AcceptedCAS(parent, tip); err != nil {
		t.Fatal(err)
	}
	return &goalListRepositoryFixture{obligationCommandFixture: base}
}

// A file keeps full-record assertions independent of the pipe buffer size.
func captureGoalOutput(t *testing.T, run func() int) (string, int) {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "goal-output")
	if err != nil {
		t.Fatal(err)
	}
	original := os.Stdout
	os.Stdout = file
	defer func() {
		os.Stdout = original
		file.Close()
	}()
	code := run()
	data, err := os.ReadFile(file.Name())
	if err != nil {
		t.Fatal(err)
	}
	return string(data), code
}

// TestGoalListSummaryCarriesMarkersDropsControlsAndMarksCuts: the summary
// keeps the table view's facts (a landing, a park's reason, a relayed
// approval's standing), never prints a control character, and marks a cut.
func TestGoalListSummaryCarriesMarkersDropsControlsAndMarksCuts(t *testing.T) {
	// The temporary goal authority horizon (2026-09-06) has passed by the
	// day this test was written, so the "fresh" relayed approval is read
	// on a day before it; the stale one has a review date before that day.
	horizon := goal.ApprovalHorizon{Now: time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)}
	grouped := map[string][]*goal.GoalFile{
		goal.StateApproved: {
			{Id: "relayed-fresh", State: goal.StateApproved, Priority: 1, Sequence: 1, Tier: 3, NextStep: "Go.",
				Approved: &goal.ApprovalRecord{Authority: goal.ApprovalAuthorityRelayed, ReviewBy: "2026-09-30"}},
			{Id: "relayed-stale", State: goal.StateApproved, Priority: 1, Sequence: 2, Tier: 3, NextStep: "Go.",
				Approved: &goal.ApprovalRecord{Authority: goal.ApprovalAuthorityRelayed, ReviewBy: "2026-09-01"}},
		},
		goal.StateParked: {{Id: "parked-one", State: goal.StateParked, Priority: 2, Sequence: 1, Tier: 3,
			NextStep: "BEFORE\x1b[2K\x07INJECTED rest of the step. Second sentence.", Parked: &goal.ParkRecord{Because: "waits on the human"}}},
		goal.StateClaimed: {{Id: "landing-one", State: goal.StateClaimed, Priority: 1, Sequence: 3, Tier: 3, NextStep: strings.Repeat("x", 200),
			Landing: &goal.LandingRecord{At: "2026-09-12T10:00:00Z"}}},
	}
	output := goalListSummary(grouped, syncedListStates, "tip", nil, false, horizon)
	for _, want := range []string{
		"landing-one pin=- claim=- landing-since=2026-09-12T10:00:00Z :: " + strings.Repeat("x", 117) + "...\n",
		"relayed-fresh pin=- claim=- relayed=review-by:2026-09-30 :: Go.\n",
		"relayed-stale pin=- claim=- relayed=EXPIRED:the_review_date_2026-09-01_has_passed :: Go.\n",
		"parked-one pin=- claim=- parked=waits on the human :: BEFORE[2KINJECTED rest of the step.\n",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("summary lacks %q:\n%s", want, output)
		}
	}
	if strings.ContainsAny(output, "\x1b\x07") {
		t.Fatalf("a control character reached the summary:\n%q", output)
	}
	if !strings.HasPrefix(output, "claimed=1 approved=2 queued=0 parked=1 done=0 tip=tip\n") {
		t.Fatalf("header = %q", strings.SplitN(output, "\n", 2)[0])
	}
}
