package main

// goal review (g1-s69 D1): the same verdict from the same record, recorded
// again, is success with no record; and the public refusals name only public
// forms.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

const (
	reviewBedTip    = "9c1f0a2b3c4d5e6f708192a3b4c5d6e7f8091a2b"
	reviewBedRecord = "plans/reviews/review-of-standing-validation.md"
)

// waitingToLandBed puts the bed's claimed goal in the Review lane: the
// land-ready line and the Landing record it names.
func waitingToLandBed(file *goal.GoalFile) {
	opid := goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FRV", "mac-cli", "m1")
	file.History = append(file.History, goal.HistoryLine{At: "2026-09-01T09:30:00Z", Opid: opid, Verb: "land-ready",
		Actor: "mac-cli+m1", Targets: []string{file.Id}, Keep: -1})
	file.Revision++
	file.Landing = &goal.LandingRecord{At: "2026-09-01T09:30:00Z", Opid: opid}
}

func writeReviewRecord(t *testing.T, root, verdict string) string {
	t.Helper()
	return writeReviewRecordAnswered(t, root, verdict, "fix — waits for Send back")
}

// writeReviewRecordAnswered writes the bed's review record with one finding
// carrying the answer given, as the room's recorder writes it.
func writeReviewRecordAnswered(t *testing.T, root, verdict, answer string) string {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(reviewBedRecord))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	record := "# Review of " + bedGoal + "\n\n- Kind: review\n- Id: 01M3MP8CZYPATTR0382JS6HMFB\n- Status: draft\n- Goals: " + bedGoal +
		"\n- Reviewed: " + reviewBedTip + " (the tip of goal/" + bedGoal + ")\n\n## Findings\n\n" +
		"- 2026-09-29 · Wido · The owner reads the wrong tree [d:deposit:t2#0]\n  - Anchor: internal/owner.go:60\n  - Answer: " + answer +
		"\n\n## Outcome\n\nVerdict: " + verdict +
		"\n\nReviewed at: " + reviewBedTip + "\n\nExamined: the change index\n"
	if err := os.WriteFile(path, []byte(record), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func init() {
	registerIdempotency("goal review", idemStateful, "the same verdict from the same record at the same tip is success with no record",
		func(t *testing.T) {
			bed := newIntentBed(t, false, waitingToLandBed)
			record := writeReviewRecord(t, bed.root(), "clear to land")
			args := []string{"goal", "review", bedGoal, "--record", record, "--verdict", "clear-to-land"}
			code, first := bed.runJSON(bed.terminalOwners(), args...)
			if code != 0 || first.Outcome != intentConfirmed {
				t.Fatalf("the first verdict = %d %+v", code, first)
			}
			bed.expectRepeatRecordsNothing("already carries this verdict", args...)
		})
}

func TestIntentGoalReviewRecordsTheVerdictAndRefusesInPublicWords(t *testing.T) {
	t.Parallel()
	bed := newIntentBed(t, false, waitingToLandBed)
	record := writeReviewRecord(t, bed.root(), "send back")
	rows := []struct {
		args []string
		want string
	}{
		{[]string{"goal", "review", bedGoal, "--verdict", "clear-to-land"}, "a review needs its record file and its verdict"},
		{[]string{"goal", "review", bedGoal, "--record", record, "--verdict", "no-verdict"}, "the verdict is clear-to-land or send-back, not"},
		{[]string{"goal", "review", bedGoal, "--record", record, "--verdict", "send-back"}, "a send-back needs its brief"},
		{[]string{"goal", "review", bedGoal, "--record", record, "--verdict", "clear-to-land"}, `doesn't start with "Verdict: clear to land"`},
		{[]string{"goal", "review", bedGoal, "--record", filepath.Join(bed.root(), "plans", "goals", bedGoal+".md"), "--verdict", "clear-to-land"}, "is not a review record in its home"},
	}
	for _, row := range rows {
		code, result := bed.runJSON(bed.terminalOwners(), row.args...)
		said := result.Summary + " " + result.Decision
		if code == 0 || !strings.Contains(said, row.want) {
			t.Errorf("%v = %d %q; want %q", row.args, code, said, row.want)
		}
	}
	brief := filepath.Join(bed.root(), "fix.md")
	if err := os.WriteFile(brief, []byte("# Correction brief\n\n1. Read the reviewed tree.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A send-back whose record answers no finding fix is refused in words on
	// its first call (Sol SOL-S69-02).
	writeReviewRecordAnswered(t, bed.root(), "send back", "left open")
	if code, result := bed.runJSON(bed.terminalOwners(), "goal", "review", bedGoal, "--record", record, "--verdict", "send-back", "--brief", brief); code == 0 ||
		!strings.Contains(result.Summary+" "+result.Decision, "has no finding marked fix") {
		t.Fatalf("a send-back with no finding answered fix = %d %+v", code, result)
	}
	writeReviewRecord(t, bed.root(), "send back")
	code, result := bed.runJSON(bed.terminalOwners(), "goal", "review", bedGoal, "--record", record, "--verdict", "send-back", "--brief", brief)
	if code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("send back = %d %+v", code, result)
	}
	endpoint, err := bed.dependencies().endpoint(bed.root())
	if err != nil {
		t.Fatal(err)
	}
	published, err := goal.ReadPublished(endpoint, goal.BriefPathFor(reviewBedRecord))
	if err != nil || !strings.Contains(string(published), "Read the reviewed tree.") {
		t.Fatalf("the brief was not published beside the record: %q %v", published, err)
	}
}
