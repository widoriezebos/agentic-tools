package plain

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

// Real Git is needed here to prove ancestry, detached proof trees and publication.
func newFixTopologyGitBed(t *testing.T) (*bed, ProveSeams, string) {
	t.Helper()
	b := newBed(t)
	sha := b.seat("seat-a", "goal-a")
	b.handIn("seat-a", "goal-a", sha)
	parent := b.merge("goal-a")
	batch := &Batch{ID: "batch", Base: b.originMain(), State: BatchRunning, Lane: lane.Record{Root: b.checkout, Install: b.install, CustodyEpoch: 1}, Members: []GoalSHA{{Goal: "goal-a", SHA: sha}}}
	if err := writeBatch(b.install, batch); err != nil {
		t.Fatal(err)
	}
	attempt := 0
	seams := ProveSeams{Now: func() time.Time { return bedNow }, NewID: func() string {
		attempt++
		return fmt.Sprintf("proof-%d", attempt)
	}, Command: func(cmd *exec.Cmd) error {
		var commit string
		for _, env := range cmd.Env {
			if strings.HasPrefix(env, "LANDING_COMMIT=") {
				commit = strings.TrimPrefix(env, "LANDING_COMMIT=")
			}
		}
		if commit == parent {
			fmt.Fprint(cmd.Stdout, "LANDING-FAILED\tpackage-a\tTestBroken\nLANDING-CHECKED\t1\n")
			return errors.New("red")
		}
		fmt.Fprint(cmd.Stdout, "LANDING-CHECKED\t0\n")
		return nil
	}}
	result, err := Run(b.install, b.checkout, "fixture", "", io.Discard, seams)
	if err != nil || result.Result != Red || result.Cause == nil || result.Cause.Kind != "own" || result.Cause.Goal != "goal-a" {
		t.Fatalf("initial red %+v: %v", result, err)
	}
	return b, seams, parent
}

func recordTopologyFix(t *testing.T, b *bed, parent, message string) Fix {
	t.Helper()
	running, recorded, alive, err := ReadRunning(b.install, ProveSeams{})
	if err != nil || !recorded || alive || running.Commit != parent {
		t.Fatalf("red checkpoint %+v recorded=%v alive=%v: %v", running, recorded, alive, err)
	}
	if hold, err := ProofHold(b.install, ProveSeams{Alive: func(Running) bool { return true }}); err != nil || hold != "" {
		t.Fatalf("red still holds a live proof: %q %v", hold, err)
	}
	if ReadRunningProof(b.install, ProveSeams{}) != nil {
		t.Fatal("red checkpoint is shown as a dead proof")
	}
	b.write(filepath.Join(b.checkout, "goal-a.txt"), "repaired\n")
	b.git(b.checkout, "add", "goal-a.txt")
	b.git(b.checkout, "commit", "--quiet", "-m", "repair", "-m", message)
	fix := Fix{Attempt: running.Attempt, Round: 1, Parent: parent, Goal: "goal-a", Units: []string{"package-a"}, Job: "build", Read: "read", Commit: b.git(b.checkout, "rev-parse", "HEAD"), State: "reviewing"}
	if err := WriteFix(b.install, &fix); err != nil {
		t.Fatal(err)
	}
	return fix
}

func TestLaneFixTopologyRealGitProofAndPush(t *testing.T) {
	t.Parallel()
	b, seams, parent := newFixTopologyGitBed(t)
	fix := recordTopologyFix(t, b, parent, "Goal-Unit: goal-a/lane-fix-1")
	batch, err := CheckBatch(b.install, b.checkout, fix.Commit, "", false, seams)
	if err != nil || batch == nil || !slices.Equal(batch.Members, []GoalSHA{{Goal: "goal-a", SHA: b.git(b.checkout, "rev-parse", "origin/goal/goal-a")}}) {
		t.Fatalf("repair admission %+v: %v", batch, err)
	}
	result, err := Run(b.install, b.checkout, "fixture", "", io.Discard, seams)
	if err != nil || result.Result != Green || result.Commit != fix.Commit {
		t.Fatalf("fixed proof %+v: %v", result, err)
	}
	if _, err := os.Stat(runningPath(b.install)); !os.IsNotExist(err) {
		t.Fatalf("green kept checkpoint: %v", err)
	}
	outcome, err := PushChecked(b.install, b.checkout, bedNow, nil, seams)
	if err != nil || !outcome.Changed || b.originMain() != fix.Commit {
		t.Fatalf("push %+v: %v", outcome, err)
	}
	push, ok, err := LastPush(b.install)
	if err != nil || !ok || len(push.Fixes) != 1 || push.Fixes[0].Commit != fix.Commit || !slices.Equal(push.BatchMembers, batch.Members) {
		t.Fatalf("push accounting %+v: %v", push, err)
	}
	entries, err := Landed(mustFixEntries(t, b.install), ContainedIn(b.checkout, b.originMain()))
	if err != nil {
		t.Fatal(err)
	}
	entries, err = LandingTimes(entries, []Pushed{push}, bedNow.Add(-time.Hour), func(old, head string) ([]string, error) {
		return strings.Fields(b.git(b.checkout, "rev-list", old+".."+head)), nil
	})
	if err != nil || entries[0].Fix != fix.Commit || entries[0].Reason != "landed with lane fix 1" {
		t.Fatalf("landing %+v: %v", entries, err)
	}
	if active, err := ActiveFix(b.install); err != nil || active != nil {
		t.Fatalf("active %+v: %v", active, err)
	}
}

func mustFixEntries(t *testing.T, install string) []Entry {
	t.Helper()
	entries, err := Entries(install)
	if err != nil {
		t.Fatal(err)
	}
	return entries
}

func TestLaneFixTopologyRealGitMainRefreshProofAndPush(t *testing.T) {
	t.Parallel()
	for _, stage := range []string{"red checkpoint", "green proof", "settled green"} {
		t.Run(stage, func(t *testing.T) {
			t.Parallel()
			b, seams, parent := newFixTopologyGitBed(t)
			fix := recordTopologyFix(t, b, parent, "Goal-Unit: goal-a/lane-fix-1")
			if stage != "red checkpoint" {
				result, err := Run(b.install, b.checkout, "fixture", "", io.Discard, seams)
				if err != nil || result.Result != Green {
					t.Fatalf("repair proof %+v: %v", result, err)
				}
			}
			seed := filepath.Join(b.root, "seed")
			b.write(filepath.Join(seed, "metasystem", "plans", "goals.md"), "ledger advanced\n")
			b.git(seed, "add", "metasystem/plans/goals.md")
			b.git(seed, "commit", "--quiet", "-m", "advance goal ledger")
			b.git(seed, "push", "--quiet", "origin", "main")
			b.git(b.checkout, "fetch", "--quiet", "origin")
			b.git(b.checkout, "merge", "--quiet", "--no-ff", "--no-edit", "origin/main")
			head := b.git(b.checkout, "rev-parse", "HEAD")
			var result Result
			var err error
			if stage == "settled green" {
				var found bool
				result, found, err = Settled(b.install, b.checkout, seams)
				if !found {
					t.Fatalf("refreshed green was not inherited: %+v %v", result, err)
				}
			} else {
				result, err = Run(b.install, b.checkout, "fixture", "", io.Discard, seams)
			}
			if err != nil || result.Result != Green || result.Commit != head {
				t.Fatalf("refreshed repair proof %+v: %v", result, err)
			}
			if result.Attempt == fix.Attempt {
				t.Fatal("main refresh reused the red proof's attempt")
			}
			outcome, err := PushChecked(b.install, b.checkout, bedNow, nil, seams)
			if err != nil || !outcome.Changed || b.originMain() != head {
				t.Fatalf("refreshed repair push %+v: %v", outcome, err)
			}
			push, ok, err := LastPush(b.install)
			if err != nil || !ok || len(push.Fixes) != 1 || push.Fixes[0].Commit != fix.Commit || push.Fixes[0].Parent != parent || push.Fixes[0].Attempt != result.Attempt || push.Fixes[0].Round != 1 || !slices.Equal(push.BatchMembers, result.BatchMembers) {
				t.Fatalf("refreshed repair accounting %+v: %v", push, err)
			}
			for _, attempt := range []string{fix.Attempt, result.Attempt} {
				closed, err := FixForAttempt(b.install, attempt)
				if err != nil || closed == nil || closed.State != "closed" {
					t.Fatalf("repair attempt %s: %+v %v", attempt, closed, err)
				}
			}
		})
	}
}

func TestLaneFixTopologyRealGitRefusals(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"no record", "wrong round", "non-member", "off batch", "stale attempt", "extra paragraph", "closed record", "older checkpoint", "returned member"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b, seams, parent := newFixTopologyGitBed(t)
			message := "Goal-Unit: goal-a/lane-fix-1"
			if name == "wrong round" {
				message = "Goal-Unit: goal-a/lane-fix-2"
			}
			if name == "non-member" {
				message = "Goal-Unit: other/lane-fix-1"
			}
			if name == "extra paragraph" {
				message += "\nextra"
			}
			fix := recordTopologyFix(t, b, parent, message)
			switch name {
			case "no record":
				if err := os.Remove(filepath.Join(Dir(b.install), "fixes", fix.Attempt+".json")); err != nil {
					t.Fatal(err)
				}
			case "off batch":
				fix.Parent = b.originMain()
			case "stale attempt":
				if err := os.Remove(filepath.Join(Dir(b.install), "fixes", fix.Attempt+".json")); err != nil {
					t.Fatal(err)
				}
				fix.Attempt = "older"
			case "closed record":
				fix.State = "closed"
			case "older checkpoint":
				last, _, err := LastResult(b.install)
				if err != nil {
					t.Fatal(err)
				}
				last.Attempt = "newer"
				if err := appendLine(resultsPath(b.install), last); err != nil {
					t.Fatal(err)
				}
			case "returned member":
				if _, _, err := ReturnProven(b.install, "goal-a", "own", "broken", false, "", bedNow); err != nil {
					t.Fatal(err)
				}
			}
			if name != "no record" {
				if err := WriteFix(b.install, &fix); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := CheckBatch(b.install, b.checkout, fix.Commit, "", false, seams); err == nil {
				t.Fatal("unauthorized repair admitted")
			}
		})
	}
}

func TestLaneFixTopologyRealGitRedReplayAndReturn(t *testing.T) {
	t.Parallel()
	for _, gate := range []bool{false, true} {
		t.Run(fmt.Sprint(gate), func(t *testing.T) {
			t.Parallel()
			b, seams, parent := newFixTopologyGitBed(t)
			fix := recordTopologyFix(t, b, parent, "Goal-Unit: goal-a/lane-fix-1")
			seams.Gate = gate
			seams.Command = func(cmd *exec.Cmd) error {
				if strings.Contains(cmd.Args[len(cmd.Args)-1], " test groups fast-static-build") {
					fmt.Fprint(cmd.Stdout, "landing group fast-static-build green 1\nLANDING-CHECKED\t0\n")
					return nil
				}

				for _, env := range cmd.Env {
					if env == "LANDING_COMMIT="+fix.Commit {
						fmt.Fprint(cmd.Stdout, "LANDING-FAILED\tpackage-a\tTestBroken\nLANDING-CHECKED\t1\n")
						return errors.New("still red")
					}
				}
				fmt.Fprint(cmd.Stdout, "LANDING-CHECKED\t0\n")
				return nil
			}
			result, err := Run(b.install, b.checkout, "fixture", "", io.Discard, seams)
			if err != nil || result.Cause == nil || result.Cause.Kind != "own" || result.Cause.Goal != fix.Goal || result.Cause.SHA != mustFixEntries(t, b.install)[0].SHA || result.Commit != fix.Commit {
				t.Fatalf("repair attribution %+v: %v", result, err)
			}
			running, recorded, alive, err := ReadRunning(b.install, seams)
			active, fixErr := ActiveFix(b.install)
			if err != nil || fixErr != nil || !recorded || alive || active == nil || active.Attempt != running.Attempt || running.Commit != fix.Commit || running.Gate != gate {
				t.Fatalf("checkpoint %+v active %+v: %v %v", running, active, err, fixErr)
			}
			_, _, _, err = checkState(b.install, b.checkout, "", seams)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(runningPath(b.install)); err != nil {
				t.Fatalf("checkState discarded red: %v", err)
			}
			entry, changed, err := ReturnProven(b.install, "goal-a", "own", "still broken", false, "", bedNow)
			if err != nil || !changed || !strings.Contains(entry.Reason, "fix round 1") {
				t.Fatalf("return %+v changed=%v: %v", entry, changed, err)
			}
			if _, err := os.Stat(runningPath(b.install)); !os.IsNotExist(err) {
				t.Fatalf("return kept checkpoint: %v", err)
			}
		})
	}
}
