package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

// plainLaneBed is a delivery bed whose lane is registered: its installation
// is a folder of the test, where the hand-in's queue.jsonl lands.
func plainLaneBed(t *testing.T, sources ...string) (*deliveryBed, *landingOwners, string) {
	t.Helper()
	b := newDeliveryBed(t)
	owners := &landingOwners{configured: true, status: readBranch(2, sources...)}
	owners.install(b)
	install := filepath.Join(t.TempDir(), "lane", "metasystem")
	b.owners.laneInstall = func(root string) (string, error) {
		if root != "/landing" {
			t.Errorf("the lane installation was asked for %q", root)
		}
		return install, nil
	}
	return b, owners, install
}

// Plain lane step 1: with a lane registered, work land G passes the seat's
// gates, appends one line {goal, branch, sha, seat, at} to the lane's
// queue.jsonl and says "handed to the lane"; a repeat at the same sha is
// success and appends nothing; work land G then shows the line's state:
// waiting, returned with its reason, landed. A new sha after a return
// hands in again. No batch is joined.
func TestWorkLandHandsInToThePlainLane(t *testing.T) {
	t.Parallel()
	b, owners, install := plainLaneBed(t, "critic-root", "critic-root")
	code, result := b.do("work", "land", "standing-validation")
	expectOutcome(t, "hand-in", code, result, intentConfirmed)
	if !strings.Contains(result.Summary, "handed to the lane") || len(owners.joins) != 0 {
		t.Fatalf("hand-in: %+v joins=%v", result, owners.joins)
	}
	data, err := os.ReadFile(filepath.Join(install, "artifacts", "agents", "landing", "queue.jsonl"))
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
	if data, _ := os.ReadFile(filepath.Join(install, "artifacts", "agents", "landing", "queue.jsonl")); strings.Count(string(data), "\n") != 1 {
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

	// The seat fixed it: a new sha hands in again.
	owners.status.BranchTip = strings.Repeat("3", 40)
	code, result = b.do("work", "land", "standing-validation")
	expectOutcome(t, "hand-in again", code, result, intentConfirmed)
	if _, err := plain.Settle(install, "main", func(string, string) (bool, error) { return true, nil },
		func(plain.Entry, string) (string, error) { return "done", nil }, time.Now()); err != nil {
		t.Fatal(err)
	}
	code, result = b.do("work", "land", "standing-validation")
	expectOutcome(t, "landed", code, result, intentUnchanged)
	if !strings.Contains(result.Summary, "landed") {
		t.Fatalf("the seat sees the landing: %+v", result)
	}
}

// The seat's gates stay at hand-in: a unit without a critic's read is
// refused there and nothing is queued.
func TestWorkLandKeepsTheReadGateBeforeTheHandIn(t *testing.T) {
	t.Parallel()
	b, _, install := plainLaneBed(t, "critic-root", "reader-record")
	code, result := b.do("work", "land", "standing-validation")
	expectOutcome(t, "unread unit", code, result, intentRefused)
	if entries, _ := plain.Entries(install); len(entries) != 0 {
		t.Fatalf("a refused hand-in queued: %+v", entries)
	}
}

// Plain lane step 4: the settlement concludes a landed goal in process on
// the lane installation's ledger, as the pair that holds its claim; a
// repeat is success.
func TestPlainLaneDoneConcludesTheGoalAsItsHolder(t *testing.T) {
	t.Parallel()
	root := syncedClaimedGoalFixture(t)
	done := plainLaneDone(root)
	note, err := done(plain.Entry{Goal: "standing-validation", SHA: strings.Repeat("a", 40)}, strings.Repeat("b", 40))
	if err != nil || note != "done" {
		t.Fatalf("done: %q %v", note, err)
	}
	endpoint, err := goal.ResolveEndpoint(root)
	if err != nil {
		t.Fatal(err)
	}
	projection, err := goal.Project(endpoint, true, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	file, archived := projection.Tree.Archived("standing-validation")
	if !archived || file.State != goal.StateDone || !strings.Contains(file.Conclude, "through the landing lane") {
		t.Fatalf("the goal is done: %+v %v", file, archived)
	}
	if note, err := done(plain.Entry{Goal: "standing-validation"}, strings.Repeat("b", 40)); err != nil || note != "done" {
		t.Fatalf("a repeat: %q %v", note, err)
	}
}
