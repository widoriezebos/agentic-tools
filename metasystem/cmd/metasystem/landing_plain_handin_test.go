package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

// plainLaneBed is a delivery bed whose lane is registered at /landing: its
// installation is a folder of the test, where the hand-in's queue.jsonl
// lands.
func plainLaneBed(t *testing.T, sources ...string) (*deliveryBed, *landingOwners, string) {
	t.Helper()
	b := newDeliveryBed(t)
	owners := &landingOwners{status: readBranch(2, sources...)}
	owners.install(b)
	return b, owners, laneOnBed(t, b)
}

// gitlessLaneBed is plainLaneBed without Git or its stubbed landing gate.
func gitlessLaneBed(t *testing.T, amend func(*goal.GoalFile), sources ...string) (*deliveryBed, *landingOwners, string) {
	t.Helper()
	b := gitlessDeliveryBed(t, amend)
	owners := &landingOwners{status: readBranch(2, sources...)}
	owners.install(b)
	gitNeverCalled(t)
	// Main contains a commit only when it is main's tip, and the seat is
	// named by the bed's machine.
	b.owners.containedIn = func(_, main string) func(string) (bool, error) {
		return func(sha string) (bool, error) { return sha == main, nil }
	}
	b.owners.laneSeat = func(string) string { return "mac-cli" }
	return b, owners, laneOnBed(t, b)
}

// gitNeverCalled fails the test at its end when the run's Git shim recorded
// a call. The run without Git names the shim's marker file in
// METASYSTEM_TEST_GIT_MARKER; other runs check nothing.
func gitNeverCalled(t *testing.T) {
	t.Helper()
	marker := os.Getenv("METASYSTEM_TEST_GIT_MARKER")
	if marker == "" {
		return
	}
	t.Cleanup(func() {
		if calls, err := os.ReadFile(marker); !os.IsNotExist(err) {
			t.Errorf("Git was called: %q %v", calls, err)
		}
	})
}

// laneOnBed registers the bed's lane at /landing and answers its
// installation, a folder of the test. The bed's commands run as the session
// holding the goal (mac-cli+m1), and the mark after a hand-in is taken
// without touching the ledger.
func laneOnBed(t *testing.T, b *deliveryBed) string {
	t.Helper()
	b.lineage = "m1"
	b.owners.laneRoot = func(string, time.Time) (string, bool, error) { return "/landing", true, nil }
	install := filepath.Join(t.TempDir(), "lane", "metasystem")
	b.owners.laneInstall = func(root string) (string, error) {
		if root != "/landing" {
			t.Errorf("the lane installation was asked for %q", root)
		}
		return install, nil
	}
	b.owners.landMark = func(*intentInvocation, string) intentResult {
		return intentResult{Outcome: intentConfirmed, Summary: "the bed's mark"}
	}
	return install
}

// Plain lane step 1: with a lane registered, work land G passes the seat's
// gates, appends one line {goal, branch, sha, seat, at} to the lane's
// queue.jsonl and says "handed to the lane"; a repeat at the same sha is
// success and appends nothing; work land G then shows the line's state:
// waiting, returned with its reason, and landed once main contains its sha
// (derived, nothing recorded), naming goal done. A new sha after a return
// hands in again. Nothing is proved or pushed by the seat.
func TestWorkLandHandsInToThePlainLane(t *testing.T) {
	t.Parallel()
	b, owners, install := plainLaneBed(t, "critic-root", "critic-root")
	code, result := b.do("work", "land", "standing-validation")
	expectOutcome(t, "hand-in", code, result, intentConfirmed)
	if !strings.Contains(result.Summary, "handed to the lane") || len(owners.pushes) != 0 || owners.candidates != 0 {
		t.Fatalf("hand-in: %+v pushes=%v candidates=%d", result, owners.pushes, owners.candidates)
	}
	queue := filepath.Join(install, "artifacts", "agents", "landing", "queue.jsonl")
	data, err := os.ReadFile(queue)
	if err != nil || strings.Count(string(data), "\n") != 1 {
		t.Fatalf("one queue line: %q %v", data, err)
	}
	entries, err := plain.Entries(install)
	if err != nil || len(entries) != 1 {
		t.Fatalf("entries: %+v %v", entries, err)
	}
	if got := entries[0]; got.Goal != "standing-validation" || got.Branch != "goal/standing-validation" || got.SHA != strings.Repeat("2", 40) || got.Seat == "" || got.At == "" {
		t.Fatalf("the line: %+v", got)
	}

	code, result = b.do("work", "land", "standing-validation")
	expectOutcome(t, "repeat", code, result, intentUnchanged)
	if !strings.Contains(result.Summary, "waiting") {
		t.Fatalf("a repeat shows the waiting line: %+v", result)
	}
	if data, _ := os.ReadFile(queue); strings.Count(string(data), "\n") != 1 {
		t.Fatalf("a repeat appends nothing: %q", data)
	}

	if _, _, err := plain.Return(install, "standing-validation", "app-standard fails since it joined", time.Now()); err != nil {
		t.Fatal(err)
	}
	code, result = b.do("work", "land", "standing-validation")
	expectOutcome(t, "returned", code, result, intentRefused)
	if !strings.Contains(result.Summary, "returned: app-standard fails since it joined") {
		t.Fatalf("the seat sees the return: %+v", result)
	}

	// The seat fixed it: a new sha hands in again. It is a real commit of
	// the seat's repository, and once main holds it the line reads landed.
	git := func(args ...string) string {
		out, err := exec.Command("git", append([]string{"-C", b.root(), "-c", "user.name=seat", "-c", "user.email=seat@example.invalid", "-c", "commit.gpgsign=false"}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("commit", "--quiet", "--allow-empty", "-m", "the fix")
	fixed := git("rev-parse", "HEAD")
	owners.status.BranchTip = fixed
	code, result = b.do("work", "land", "standing-validation")
	expectOutcome(t, "hand-in again", code, result, intentConfirmed)
	code, result = b.do("work", "land", "standing-validation")
	expectOutcome(t, "waiting again", code, result, intentUnchanged)
	if !strings.Contains(result.Summary, "waiting") {
		t.Fatalf("not yet in main: %+v", result)
	}
	owners.status.EndpointTip = fixed
	code, result = b.do("work", "land", "standing-validation")
	expectOutcome(t, "landed", code, result, intentUnchanged)
	if !strings.Contains(result.Summary, "landed on main") || result.Next == nil || !strings.Contains(strings.Join(result.Next.Argv, " "), "goal done standing-validation") {
		t.Fatalf("the seat sees the landing and concludes the goal itself: %+v", result)
	}
	if data, _ := os.ReadFile(queue); strings.Count(string(data), "\n") != 3 {
		t.Fatalf("landing records nothing: %q", data)
	}
}

// The seat's gates stay at hand-in: a unit without a clean read is refused
// there and nothing is queued.
func TestWorkLandKeepsTheReadGateBeforeTheHandIn(t *testing.T) {
	t.Parallel()
	b, _, install := plainLaneBed(t, "critic-root")
	b.owners.branchState = func(string, string) (intentBranchState, error) { return readBranch(1, "critic-root"), nil }
	code, result := b.do("work", "land", "standing-validation")
	expectOutcome(t, "unread unit", code, result, intentRefused)
	if !strings.Contains(result.Summary, "no clean read") {
		t.Fatalf("the read gate refuses: %+v", result)
	}
	if entries, _ := plain.Entries(install); len(entries) != 0 {
		t.Fatalf("a refused hand-in queued: %+v", entries)
	}
}

// A hand-in of a goal below the tier writes its queue line, then marks the
// goal waiting to land. A mark that fails after the line leaves the line, and
// the repeat at the same commit reads it waiting and marks the goal.
func TestWorkLandWritesTheQueueLineBeforeTheMark(t *testing.T) {
	t.Parallel()
	b, _, install := gitlessLaneBed(t, func(file *goal.GoalFile) { retier(file, 1) }, "critic-root", "critic-root")
	var linesAtMark []int
	b.owners.landMark = func(*intentInvocation, string) intentResult {
		entries, _ := plain.Entries(install)
		linesAtMark = append(linesAtMark, len(entries))
		if len(linesAtMark) == 1 {
			return intentResult{Outcome: intentFailed, Summary: "the goal ledger can't be reached"}
		}
		return intentResult{Outcome: intentConfirmed, Summary: "the bed's mark"}
	}
	code, result := b.do("work", "land", bedGoal)
	expectOutcome(t, "a mark that fails", code, result, intentPartial)
	if !strings.Contains(result.Summary, "handed to the lane") || !strings.Contains(result.Summary, "not marked as waiting to land") {
		t.Fatalf("the hand-in does not say its line stands unmarked: %+v", result)
	}
	if entries, _ := plain.Entries(install); len(entries) != 1 {
		t.Fatalf("a failed mark took the line away: %+v", entries)
	}
	code, result = b.do("work", "land", bedGoal)
	expectOutcome(t, "the repeat", code, result, intentUnchanged)
	if !strings.Contains(result.Summary, "waiting in the landing lane") || len(linesAtMark) != 2 || linesAtMark[0] != 1 || linesAtMark[1] != 1 {
		t.Fatalf("each mark follows the line, and the repeat marks: lines at each mark %v: %+v", linesAtMark, result)
	}
}

// breachStoppedBed puts the bed's claimed goal under its holder's stop
// fence.
func breachStoppedBed(file *goal.GoalFile) {
	closedAt := "2026-09-01T09:00:00Z"
	file.Revision++
	file.StopCapability = &goal.StopCapability{Generation: 2, Revision: 2, Machine: "mac-cli", ClaimEpoch: 1, FenceEpoch: 1}
	file.StopFence = &goal.StopFence{StopID: "stop-standing-validation-r2-f1", Revision: 2, Epoch: 1, CapabilityGeneration: 2, ClosedAt: closedAt, Reason: goal.StopReasonElapsedLimit}
	file.History = append(file.History, goal.HistoryLine{At: closedAt, Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FAC", "mac-cli", "m1"),
		Verb: "breach-stop", Actor: "mac-cli+m1", Targets: []string{file.Id}, Keep: -1})
}

// A goal below the tier under a stop fence could not be marked, so it is
// refused before the line and nothing is handed in.
func TestAStoppedGoalHandsNothingIn(t *testing.T) {
	t.Parallel()
	b, _, install := gitlessLaneBed(t, func(file *goal.GoalFile) { breachStoppedBed(file); retier(file, 1) }, "critic-root", "critic-root")
	code, result := b.do("work", "land", bedGoal)
	expectOutcome(t, "a stopped goal", code, result, intentRefused)
	if !strings.Contains(result.Summary, "breach-stopped") || !strings.Contains(result.Summary, "nothing was handed in") {
		t.Fatalf("the stop fence does not refuse the hand-in: %+v", result)
	}
	if entries, _ := plain.Entries(install); len(entries) != 0 {
		t.Fatalf("a stopped goal was handed in: %+v", entries)
	}
}

// A goal handed in earlier and not marked reads waiting at the same commit
// and is marked then; a goal the ledger shows marked is left as it is.
func TestAWaitingRepeatMarksTheGoal(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		label string
		amend func(*goal.GoalFile)
		marks int
	}{{"not marked", nil, 1}, {"already marked", waitingToLandBed, 0}} {
		b, owners, install := gitlessLaneBed(t, row.amend, "critic-root", "critic-root")
		if _, _, err := plain.HandIn(install, plain.Line{Goal: bedGoal, Branch: "goal/" + bedGoal, SHA: owners.status.BranchTip, At: "2026-09-25T11:00:00Z"}); err != nil {
			t.Fatal(err)
		}
		marks := 0
		b.owners.landMark = func(*intentInvocation, string) intentResult {
			marks++
			return intentResult{Outcome: intentConfirmed, Summary: "the bed's mark"}
		}
		code, result := b.do("work", "land", bedGoal)
		expectOutcome(t, row.label, code, result, intentUnchanged)
		if !strings.Contains(result.Summary, "waiting in the landing lane") || marks != row.marks {
			t.Fatalf("%s: %d marks, want %d: %+v", row.label, marks, row.marks, result)
		}
	}
}

// A goal whose line main holds before a repeat reads it is marked then, as a
// waiting goal is, and still reads landed; a mark that fails says so and names
// the repeat. A goal marked already, or one the mark would refuse, reads
// landed with no mark taken.
func TestALandedRepeatMarksTheGoal(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		label string
		amend func(*goal.GoalFile)
		fails bool
		marks int
	}{{"not marked", nil, false, 1}, {"already marked", waitingToLandBed, false, 0}, {"stopped", breachStoppedBed, false, 0}, {"a mark that fails", nil, true, 1}} {
		b, owners, install := gitlessLaneBed(t, row.amend, "critic-root", "critic-root")
		owners.status.EndpointTip = owners.status.BranchTip
		if _, _, err := plain.HandIn(install, plain.Line{Goal: bedGoal, Branch: "goal/" + bedGoal, SHA: owners.status.BranchTip, At: "2026-09-25T11:00:00Z"}); err != nil {
			t.Fatal(err)
		}
		marks, outcome, next := 0, intentUnchanged, "goal done "+bedGoal
		if row.fails {
			outcome, next = intentPartial, "work land "+bedGoal
		}
		b.owners.landMark = func(*intentInvocation, string) intentResult {
			marks++
			if row.fails {
				return intentResult{Outcome: intentFailed, Summary: "the goal ledger can't be reached"}
			}
			return intentResult{Outcome: intentConfirmed, Summary: "the bed's mark"}
		}
		code, result := b.do("work", "land", bedGoal)
		expectOutcome(t, row.label, code, result, outcome)
		if !strings.Contains(result.Summary, "landed on main") || strings.Contains(result.Summary, "not marked") != row.fails || marks != row.marks ||
			result.Next == nil || !strings.Contains(strings.Join(result.Next.Argv, " "), next) {
			t.Fatalf("%s: %d marks, want %d: %+v", row.label, marks, row.marks, result)
		}
	}
}
