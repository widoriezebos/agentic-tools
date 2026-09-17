package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

func TestGoalListSummaryKeepsLargeHistoriesOutAndOrdersEachBucket(t *testing.T) {
	root := goalListHistoryFixture(t)
	output, code := captureGoalOutput(t, func() int { return runGoalList([]string{"--root", root}) })
	if code != 0 || len(output) > 64*1024 || strings.Contains(output, "history payload") {
		t.Fatalf("summary code=%d bytes=%d includesHistory=%v", code, len(output), strings.Contains(output, "history payload"))
	}
	tip := strings.TrimSpace(goalSyncMutationGit(t, root, "rev-parse", "HEAD"))
	if !strings.HasPrefix(output, "claimed=1 approved=1 queued=5 parked=1 done=1 abandoned=0 tip="+tip+"\n") {
		t.Fatalf("summary lacks bucket counts or accepted tip: %s", output)
	}
	var rows []string
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, " :: ") {
			rows = append(rows, line)
		}
	}
	want := []string{
		"0:0 claimed tier 3 standing-validation pin=- claim=mac-cli :: Run it.",
		"0:0 approved tier 3 approved-one pin=seat-a claim=- :: Check approval.",
		"1:1 queued tier 3 z-first pin=- claim=- :: First step.",
		"1:2 queued tier 3 a-second pin=- claim=- :: " + strings.Repeat("界", 117) + "...",
		"2:1 queued tier 3 b-third pin=- claim=- :: Continue?",
		"0:0 queued tier 3 c-unranked pin=- claim=- :: ",
		"0:0 queued tier 3 d-unranked pin=- claim=- :: Last step!",
		"0:0 parked tier 3 parked-one pin=- claim=- parked=Waiting for input. :: Wait for input.",
	}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("summary rows\ngot: %q\nwant: %q", rows, want)
	}
	withDone, code := captureGoalOutput(t, func() int { return runGoalList([]string{"--root", root, "--done"}) })
	if code != 0 || !strings.HasPrefix(withDone, output) || !strings.HasSuffix(withDone, "0:0 done tier 3 done-one pin=- claim=- :: \n") {
		t.Fatalf("--done did not append the archived bucket: code=%d output=%q", code, withDone)
	}
	filtered, code := captureGoalOutput(t, func() int {
		return runGoalList([]string{"--root", root, "--fetch", "--label", "shared", "--label", "selected"})
	})
	if code != 0 || strings.Count(filtered, " :: ") != 1 || !strings.Contains(filtered, want[2]) || !strings.Contains(filtered, "queued=1") {
		t.Fatalf("summary lost repeated-label filtering or local fetch: code=%d output=%q", code, filtered)
	}
}

func TestGoalListJSONKeepsItsShapeAndRequiresHistoryFlag(t *testing.T) {
	root := goalListHistoryFixture(t)
	endpoint, err := goal.ResolveEndpoint(root)
	if err != nil {
		t.Fatal(err)
	}
	projection, err := goal.Project(endpoint, false, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	for _, flags := range [][]string{{"--json"}, {"--json", "--pretty"}, {"--json", "--history"}} {
		output, code := captureGoalOutput(t, func() int { return runGoalList(append([]string{"--root", root}, flags...)) })
		var envelope map[string]json.RawMessage
		if code != 0 || json.Unmarshal([]byte(output), &envelope) != nil {
			t.Fatalf("%v did not emit JSON: code=%d bytes=%d", flags, code, len(output))
		}
		if len(envelope) != 12 {
			t.Fatalf("JSON envelope keys changed: %v", reflect.ValueOf(envelope).MapKeys())
		}
		for _, key := range []string{"root", "world", "tip", "banners", "open", "queued", "approved", "claimed", "parked", "done", "abandoned", "trunkRed"} {
			if _, ok := envelope[key]; !ok {
				t.Fatalf("JSON envelope lost %s", key)
			}
		}
		withHistory := flags[len(flags)-1] == "--history"
		counts := map[string]int{"open": 8, "queued": 5, "approved": 1, "claimed": 1, "parked": 1, "done": 1, "abandoned": 0}
		for bucket, count := range counts {
			var records []map[string]json.RawMessage
			if err := json.Unmarshal(envelope[bucket], &records); err != nil || len(records) != count {
				t.Fatalf("%s count=%d want=%d error=%v", bucket, len(records), count, err)
			}
			for _, record := range records {
				var id string
				if err := json.Unmarshal(record["Id"], &id); err != nil {
					t.Fatal(err)
				}
				file := projection.Tree.Live[id]
				if bucket == "done" {
					file = projection.Tree.Done[id]
				}
				if bucket == "abandoned" {
					file = projection.Tree.Abandoned[id]
				}
				data, err := json.Marshal(file)
				if err != nil {
					t.Fatal(err)
				}
				var want map[string]json.RawMessage
				if err := json.Unmarshal(data, &want); err != nil {
					t.Fatal(err)
				}
				if !withHistory {
					want["History"] = json.RawMessage("[]")
				}
				compactRecord, _ := json.Marshal(record)
				compactWant, _ := json.Marshal(want)
				if string(compactRecord) != string(compactWant) {
					t.Fatalf("%v changed fields or history for %s in %s", flags, id, bucket)
				}
			}
		}
		if flags[len(flags)-1] == "--pretty" && !strings.Contains(output, "\n  \"open\": [\n") {
			t.Fatal("--pretty did not indent JSON")
		}
	}
}

func TestGoalShowKeepsPageFieldsAndRequiresHistoryFlag(t *testing.T) {
	root := goalListHistoryFixture(t)
	for _, id := range []string{"standing-validation", "done-one"} {
		args := []string{"--root", root, "--id", id}
		plain, plainCode := captureGoalOutput(t, func() int { return runGoalShow(args) })
		full, fullCode := captureGoalOutput(t, func() int { return runGoalShow(append(args, "--history")) })
		var plainPage, fullPage map[string]json.RawMessage
		if plainCode != 0 || fullCode != 0 || json.Unmarshal([]byte(plain), &plainPage) != nil || json.Unmarshal([]byte(full), &fullPage) != nil {
			t.Fatalf("goal show failed for %s: default=%d history=%d", id, plainCode, fullCode)
		}
		var plainGoal, fullGoal map[string]json.RawMessage
		if json.Unmarshal(plainPage["goal"], &plainGoal) != nil || json.Unmarshal(fullPage["goal"], &fullGoal) != nil {
			t.Fatal("goal show lost the goal record")
		}
		if string(plainGoal["History"]) != "[]" || !strings.Contains(string(fullGoal["History"]), "history payload") {
			t.Fatal("goal show did not make history opt-in")
		}
		fullGoal["History"] = json.RawMessage("[]")
		if !reflect.DeepEqual(plainGoal, fullGoal) {
			t.Fatal("goal show changed fields besides History")
		}
		delete(plainPage, "goal")
		delete(fullPage, "goal")
		if len(plainPage) != 4 || !reflect.DeepEqual(plainPage, fullPage) {
			t.Fatal("goal show changed the page envelope")
		}
	}
}

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

func TestGoalListLegacyJSONRetainsAdoptionFacts(t *testing.T) {
	root := t.TempDir()
	writeLedger(t, root, "# Goals\n\n## Current goal: solo \u2014 One goal\n- Origin: main\n- Next step: Continue. Then inspect.\n")
	output, code := captureGoalOutput(t, func() int { return runGoalList([]string{"--root", root, "--json"}) })
	var records map[string]json.RawMessage
	if code != 0 || json.Unmarshal([]byte(output), &records) != nil {
		t.Fatalf("legacy JSON failed: code=%d output=%q", code, output)
	}
	for _, field := range []string{"baselinePresent", "baselineMatches"} {
		if string(records[field]) != "false" {
			t.Fatalf("adoption lost %s: %s", field, records[field])
		}
	}
	if string(records["world"]) != `"legacy"` || !strings.Contains(string(records["current"]), `"Id":"solo"`) {
		t.Fatalf("legacy JSON lost its record shape: %s", output)
	}
	summary, code := captureGoalOutput(t, func() int { return runGoalList([]string{"--root", root}) })
	if code != 0 || !strings.Contains(summary, "current=1") || !strings.HasSuffix(summary, "0:0 current tier 0 solo pin=- claim=- :: Continue.\n") {
		t.Fatalf("legacy summary lost the current goal: code=%d output=%q", code, summary)
	}
}

func goalListHistoryFixture(t *testing.T) string {
	t.Helper()
	root := syncedClaimedGoalFixture(t)
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
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, goal.RenderFile(file), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	goalSyncMutationGit(t, root, "add", "plans/goals", "records/goals")
	goalSyncMutationGit(t, root, "commit", "-q", "-m", "large history fixture")
	goalSyncMutationGit(t, root, "update-ref", goal.LocalLedgerBranch, "HEAD")
	goalSyncMutationGit(t, root, "update-ref", goal.AcceptedRef, "HEAD")
	return root
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

func TestGoalListRefusesRecordFlagsOnTheSummary(t *testing.T) {
	root := goalListHistoryFixture(t)
	for _, args := range [][]string{{"--root", root, "--history"}, {"--root", root, "--pretty"}} {
		output, code := captureGoalOutput(t, func() int { return runGoalList(args) })
		if code != 2 || strings.Contains(output, " :: ") {
			t.Fatalf("%v printed a summary (code %d) instead of refusing: %q", args, code, output)
		}
	}
}
