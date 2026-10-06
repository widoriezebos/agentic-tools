package main

import (
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
)

func TestGoalPauseAcceptsChangesRead(t *testing.T) {
	t.Parallel()
	bed := newIntentBed(t, false, nil)
	owners := bed.owners()
	// A --changes read has no earlier build. Parking doesn't validate the read.
	changesRead, main := strings.Repeat("b", 40), strings.Repeat("a", 40)
	origin := main
	read := func(repo string, args ...string) ([]byte, error) {
		if repo == bed.root() {
			switch strings.Join(args, " ") {
			case "rev-list --first-parent " + main + ".." + changesRead:
				return []byte(changesRead + "\n"), nil
			case "show -s --format=%(trailers:only,unfold=true) " + changesRead:
				return []byte("Goal-Read: " + bedGoal + "/seat-change " + strings.Repeat("c", 40) + "\n"), nil
			}
		}
		t.Errorf("unexpected Git read at %q: %q", repo, args)
		return nil, fmt.Errorf("unexpected Git read")
	}
	owners.parkBranchCheck = func(root string, endpoint goal.Endpoint) func(string, string) (string, error) {
		return branch.ParkCheckWithRaw(root, endpoint,
			func(string, string) (string, bool, error) { return changesRead, true, nil },
			func(string, goal.Endpoint) (string, error) { return main, nil },
			func(string, goal.Endpoint, string) (string, bool, error) { return origin, true, nil }, read)
	}
	reason := "wait for Wido's word on the --changes read"
	if code, result := bed.runJSON(owners, "goal", "pause", bedGoal, "--reason", reason, "--lineage", "m1"); code == 0 || bed.goalFile(bedGoal).State == goal.StateParked {
		t.Fatalf("an unpushed branch paused: %d %+v", code, result)
	}
	origin = changesRead
	code, result := bed.runJSON(owners, "goal", "pause", bedGoal, "--reason", reason, "--lineage", "m1")
	file := bed.goalFile(bedGoal)
	if code != 0 || file.State != goal.StateParked || file.Parked == nil || file.Parked.Because != reason {
		t.Fatalf("pause after a changes read = %d %+v; record %s %+v", code, result, file.State, file.Parked)
	}
	if !strings.Contains(file.NextStep, "goal/"+bedGoal+" at "+changesRead+" has no unit") || file.History[len(file.History)-1].Reason != reason {
		t.Fatalf("pause lost its branch tip or reason: %+v", file)
	}
}

func TestGoalPauseChangesReadSummarySweepsWithoutLocalBranch(t *testing.T) {
	t.Parallel()
	bed := newIntentBed(t, false, nil)
	owners := bed.owners()
	main, changesRead := strings.Repeat("a", 40), strings.Repeat("b", 40)
	olderUnit, lastUnit, subject := strings.Repeat("c", 40), strings.Repeat("d", 40), strings.Repeat("e", 40)
	read := func(repo string, args ...string) ([]byte, error) {
		if repo == bed.root() {
			switch strings.Join(args, " ") {
			case "rev-list --first-parent " + main + ".." + changesRead:
				return []byte(changesRead + "\n" + lastUnit + "\n" + olderUnit + "\n"), nil
			case "show -s --format=%(trailers:only,unfold=true) " + changesRead:
				// The seat's read names no earlier build of the same work.
				return []byte("Goal-Read: " + bedGoal + "/seat-change " + subject + "\n"), nil
			case "show -s --format=%(trailers:only,unfold=true) " + lastUnit:
				return []byte("Goal-Unit: " + bedGoal + "/last\n"), nil
			}
		}
		t.Errorf("unexpected Git read at %q: %q", repo, args)
		return nil, fmt.Errorf("unexpected Git read")
	}
	owners.parkBranchCheck = func(root string, endpoint goal.Endpoint) func(string, string) (string, error) {
		return branch.ParkCheckWithRaw(root, endpoint,
			func(string, string) (string, bool, error) { return changesRead, true, nil },
			func(string, goal.Endpoint) (string, error) { return main, nil },
			func(string, goal.Endpoint, string) (string, bool, error) { return changesRead, true, nil }, read)
	}
	code, result := bed.runJSON(owners, "goal", "pause", bedGoal, "--reason", "wait for Wido", "--lineage", "m1")
	file := bed.goalFile(bedGoal)
	if code != 0 || file.State != goal.StateParked {
		t.Fatalf("pause after a changes read = %d %+v; record %+v", code, result, file)
	}
	sweep, err := branch.ShouldSweepWithRaw(bed.root(), bedGoal, file.NextStep,
		func(repo, ref string) (string, bool, error) {
			if repo != bed.root() || ref != "refs/heads/goal/"+bedGoal {
				t.Fatalf("sweep local ref = %q %q", repo, ref)
			}
			return "", false, nil
		}, read)
	if err != nil || !sweep {
		t.Fatalf("sweep without a local branch = %v, %v; archived next step %q", sweep, err, file.NextStep)
	}
	if !strings.Contains(file.NextStep, "goal/"+bedGoal+" at "+changesRead+" is pushed; last unit last commit "+lastUnit) {
		t.Fatalf("pause lost the pushed tip or last unit: %q", file.NextStep)
	}
}

// quietSyncFlags parses a goal verb's flags as its owner does; the
// refusal it would print goes nowhere, since only whether it parsed is
// asserted here.
func quietSyncFlags(name string, args []string) (*syncFlags, bool) {
	return syncRequestDependencies{stdout: io.Discard, stderr: io.Discard}.parseSyncFlags(name, args)
}

// --blocks and --blocked-by belong to goal open alone: every other verb
// refuses them at the flag edge, and open carries both to the verb. Each is
// repeatable and each takes a comma-separated line, because a human naming
// three goals should not have to learn which of the two spellings this
// command prefers.
func TestOnlyGoalOpenTakesBlocks(t *testing.T) {
	if _, ok := quietSyncFlags("park", []string{"--root", t.TempDir(), "--id", "x", "--because", "y", "--blocks", "z"}); ok {
		t.Fatal("goal park accepted --blocks")
	}
	if _, ok := quietSyncFlags("park", []string{"--root", t.TempDir(), "--id", "x", "--because", "y", "--blocked-by", "z"}); ok {
		t.Fatal("goal park accepted --blocked-by")
	}
	f, ok := quietSyncFlags("open", []string{
		"--root", t.TempDir(), "--id", "x", "--blocks", "z,y", "--blocks", "w", "--blocked-by", "a,b",
	})
	if !ok || strings.Join(f.blocks, "|") != "z,y|w" || strings.Join(f.blockedBy, "|") != "a,b" {
		t.Fatalf("goal open carries both directions: ok=%v blocks=%v blockedBy=%v", ok, f.blocks, f.blockedBy)
	}
}

// --blocker belongs to the two edge verbs, and to nothing else.
func TestOnlyBlockAndUnblockTakeTheBlockerFlag(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, name := range []string{"open", "park", "unpark"} {
		if _, ok := quietSyncFlags(name, []string{"--root", root, "--id", "x", "--because", "y", "--blocker", "g"}); ok {
			t.Fatalf("goal %s accepted --blocker", name)
		}
	}
	for _, name := range []string{"block", "unblock"} {
		f, ok := quietSyncFlags(name, []string{"--root", root, "--id", "x", "--blocker", "g"})
		if !ok || f.id != "x" || f.blocker != "g" {
			t.Fatalf("goal %s carries --id and --blocker: ok=%v %+v", name, ok, f)
		}
	}
}

// --under belongs to the attorney verbs (approve, set-budget and unpark,
// the last with --verified); the grant flags to grant.
func TestUnderBelongsToTheAttorneyVerbs(t *testing.T) {
	root := t.TempDir()
	if _, ok := quietSyncFlags("park", []string{"--root", root, "--id", "x", "--because", "y", "--under", "e"}); ok {
		t.Fatal("goal park accepted --under")
	}
	u, ok := quietSyncFlags("unpark", []string{"--root", root, "--id", "x", "--under", "e", "--verified", "the vendor shipped 1.2"})
	if !ok || u.under != "e" || u.verified != "the vendor shipped 1.2" {
		t.Fatalf("goal unpark carries --under and --verified (R-105-m1e): ok=%v %+v", ok, u)
	}
	if _, ok := quietSyncFlags("open", []string{"--root", root, "--id", "x", "--tiers", "1"}); ok {
		t.Fatal("goal open accepted --tiers")
	}
	f, ok := quietSyncFlags("set-budget", []string{"--root", root, "--id", "x", "--under", "e"})
	if !ok || f.under != "e" {
		t.Fatalf("goal set-budget carries --under: ok=%v under=%q", ok, f.under)
	}
	g, ok := quietSyncFlags("grant", []string{"--root", root, "--by", "Wido", "--tiers", "1", "--verbs", "approve", "--expires", "2026-09-19"})
	if !ok || g.tiers != "1" || g.verbs != "approve" || g.expires != "2026-09-19" {
		t.Fatalf("goal grant carries its flags: ok=%v %+v", ok, g)
	}
}
