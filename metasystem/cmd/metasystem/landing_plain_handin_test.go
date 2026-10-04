package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/textui"
)

func TestLandingStatusSaysRecords(t *testing.T) {
	t.Parallel()
	install := t.TempDir()
	var line plain.Line
	if err := json.Unmarshal([]byte(`{"goal":"design","branch":"goal/design","sha":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","seat":"ui","records":true}`), &line); err != nil {
		t.Fatal(err)
	}
	if _, _, err := plain.HandIn(install, line); err != nil {
		t.Fatal(err)
	}
	for _, state := range []string{plain.StateWaiting, plain.StateLanded, plain.StateReturned} {
		if state == plain.StateReturned {
			if _, _, err := plain.Return(install, "design", "records check failed", time.Time{}); err != nil {
				t.Fatal(err)
			}
		}
		entries, err := plain.Entries(install)
		if err != nil {
			t.Fatal(err)
		}
		entries, err = plain.Landed(entries, func(string) (bool, error) { return state == plain.StateLanded, nil })
		if err != nil {
			t.Fatal(err)
		}
		entries = append(entries, plain.Entry{Goal: "code", Branch: "goal/code", SHA: line.SHA, Seat: "ui", State: state})
		page := textui.New(textui.Env{Verbose: true})
		withPlainLane(func(*textui.Page) {}, landingStatusData{View: lane.View{Root: &install}, Queue: entries})(page)
		words := oneSpaced(page.String())
		for _, want := range []string{"goal/design records at bbbbbbbbbbbb from ui · " + state, "goal/code at bbbbbbbbbbbb from ui · " + state} {
			if !strings.Contains(words, want) {
				t.Fatalf("status %s lacks %q: %s", state, want, words)
			}
		}
		if state == plain.StateReturned && !strings.Contains(words, "returned: records check failed") {
			t.Fatal(words)
		}
	}
}

// plainLaneBed is a delivery bed whose lane is registered at /landing: its
// installation is a folder of the test, where the hand-in's queue.jsonl
// lands.
func plainLaneBed(t *testing.T, sources ...string) (*deliveryBed, *landingOwners, string) {
	t.Helper()
	return plainLaneBedWith(t, false, sources...)
}

func plainLaneBedWith(t *testing.T, withoutGit bool, sources ...string) (*deliveryBed, *landingOwners, string) {
	t.Helper()
	b := newDeliveryBedWith(t, nil, withoutGit)
	b.owners.landingGate = func(*intentInvocation, string, string) (string, error) { return "the bed's landing", nil }
	b.owners.recordLanded = func(*intentInvocation, string) error { return nil }
	owners := &landingOwners{status: readBranch(2, sources...)}
	owners.install(b)
	b.owners.laneRoot = func(string, time.Time) (string, bool, error) { return "/landing", true, nil }
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
