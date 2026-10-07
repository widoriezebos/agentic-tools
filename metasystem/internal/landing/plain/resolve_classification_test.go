package plain

import (
	"errors"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

func TestResolveKilledRegenerationRetriesOnceAndNeverReplaysOrReturns(t *testing.T) {
	t.Parallel()
	b := newResolveFixture(t)
	runs := 0
	b.seams.Run = func([]string, string, *os.File, func(int64) error) error {
		runs++
		command := exec.Command("/bin/sleep", "60")
		if err := command.Start(); err != nil {
			t.Fatal(err)
		}
		if err := command.Process.Kill(); err != nil {
			t.Fatal(err)
		}
		return command.Wait()
	}
	for attempt := 1; attempt <= 2; attempt++ {
		out, err := b.resolve()
		if err == nil || out.Exit < 128 || out.Cause.Kind != "environment" || out.Cause.Name != "lost-process" || out.Held != (attempt == 2) || out.Entry.State != StateWaiting || runs != attempt {
			t.Fatalf("attempt %d: %+v runs=%d err=%v", attempt, out, runs, err)
		}
		if _, err := os.Stat(resolveBegunPath(b.install)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("aborted run left a begun merge: %v", err)
		}
	}
	rows, err := readLines[Regeneration](regeneratePath(b.install))
	if err != nil || len(rows) != 2 {
		t.Fatalf("resolution records=%v err=%v", rows, err)
	}
	out, err := b.resolve()
	if err != nil || !out.Held || runs != 2 {
		t.Fatalf("third run: %+v runs=%d err=%v", out, runs, err)
	}
}

func TestResolveBaselineCleansOnlySetsItRan(t *testing.T) {
	t.Parallel()
	b := newResolveFixture(t)
	b.paths += "metasystem/unused/conflict\x00"
	b.tracked += "metasystem/unused/conflict\x00"
	runs, aborted := 0, false
	originalGit := b.seams.Git
	b.seams.Git = func(dir string, args ...string) (string, error) {
		if strings.Join(args, " ") == "merge --abort" {
			aborted = true
		}
		return originalGit(dir, args...)
	}
	b.seams.Run = func([]string, string, *os.File, func(int64) error) error {
		runs++
		if runs == 2 && !aborted {
			t.Fatal("baseline ran before abort")
		}
		if aborted {
			b.untracked = "metasystem/out/new\x00metasystem/unused/preexisting\x00metasystem/unrelated\x00"
		}
		return exec.Command("/usr/bin/false").Run()
	}
	out, err := b.resolve()
	if err == nil || !out.Held || out.Cause.Kind != "unclassified" || runs != 2 || out.Entry.State != StateWaiting {
		t.Fatalf("baseline failure=%+v runs=%d err=%v", out, runs, err)
	}
	var replayWrites [][]string
	for at, write := range b.writes {
		if reflect.DeepEqual(write, []string{"merge", "--abort"}) {
			replayWrites = b.writes[at+1:]
			break
		}
	}
	want := [][]string{
		{"clean", "-f", "--", "metasystem/out/new"},
		{"restore", "--source=HEAD", "--staged", "--worktree", "--", "metasystem/out/conflict", "metasystem/out/other", "metasystem/out/new"},
	}
	if !reflect.DeepEqual(replayWrites, want) {
		t.Fatalf("replay touched an unrun set or unrelated file: %v", replayWrites)
	}
}

func TestConflictDependencyHoldTracksExactHandInAndPendingSkipsHeld(t *testing.T) {
	t.Parallel()
	for _, outcome := range []string{"waiting", "landed", "returned", "superseded"} {
		t.Run(outcome, func(t *testing.T) {
			t.Parallel()
			install := t.TempDir()
			for _, goal := range []string{"A", "B", "environment"} {
				if _, _, err := HandIn(install, Line{Goal: goal, SHA: goal}); err != nil {
					t.Fatal(err)
				}
			}
			out := ResolveOutcome{Regeneration: Regeneration{Reason: "conflicts with goal A, which is in the same batch"}, Held: true}
			entry, _, _ := Latest(install, "B")
			if err := withLock(install, func() error {
				return resolveWaitingLocked(install, entry, []GoalSHA{{Goal: "A", SHA: "A"}}, false, &out)
			}); err != nil {
				t.Fatal(err)
			}
			entry, _, _ = Latest(install, "environment")
			out.Cause = &Cause{Kind: "environment", Name: "lost-process"}
			if err := withLock(install, func() error { return resolveWaitingLocked(install, entry, nil, true, &out) }); err != nil {
				t.Fatal(err)
			}
			if outcome == "returned" {
				if _, _, err := ReturnProven(install, "A", "unclassified", "own", true, "fixture", bedNow, ProveSeams{Person: &ActProvenance{Kind: "return", Person: "fixture"}}); err != nil {
					t.Fatal(err)
				}
			}
			if outcome == "superseded" {
				if _, _, err := HandIn(install, Line{Goal: "A", SHA: "new-A"}); err != nil {
					t.Fatal(err)
				}
			}
			entries, err := Entries(install)
			if err != nil {
				t.Fatal(err)
			}
			derived, err := Landed(entries, func(sha string) (bool, error) { return outcome == "landed" && sha == "A", nil })
			if err != nil || derived[1].Held != (outcome == "waiting") || derived[1].State != StateWaiting || !derived[2].Held {
				t.Fatalf("hold after %s: %+v %v", outcome, derived, err)
			}
			// An unavailable main cannot wake a permanent hold. Batch dependencies
			// remain waiting until their landing can be established.
			pending, err := pending(install, "checkout", ProveSeams{Git: func(string, ...string) (string, error) { return "", errors.New("main unavailable") }})
			if err != nil {
				t.Fatal(err)
			}
			for _, candidate := range pending {
				if candidate.Goal == "environment" || candidate.Goal == "B" && (outcome == "waiting" || outcome == "landed") {
					t.Fatalf("held entry woke the lane: %+v", pending)
				}
			}
		})
	}
}

func TestResolveBatchDependencyRecordsMergedSupersededCommit(t *testing.T) {
	t.Parallel()
	b := newResolveFixture(t)
	b.paths = "metasystem/src/list.go\x00"
	for _, sha := range []string{"merged-A", "new-A"} {
		if _, _, err := HandIn(b.install, Line{Goal: "A", SHA: sha}); err != nil {
			t.Fatal(err)
		}
	}
	notAncestor := exec.Command("/usr/bin/false").Run()
	base := b.seams.Git
	b.seams.Git = func(dir string, args ...string) (string, error) {
		switch strings.Join(args, " ") {
		case "rev-parse --verify HEAD^{commit}":
			return "batch", nil
		case "merge-tree --write-tree main-sha goal-sha":
			return "clean-tree", nil
		}
		if args[0] == "cat-file" {
			return "", nil
		}
		if args[0] == "merge-base" {
			if args[2] == "merged-A" && args[3] == "batch" {
				return "", nil
			}
			return "", notAncestor
		}
		return base(dir, args...)
	}
	out, err := b.resolve()
	want := []GoalSHA{{Goal: "A", SHA: "merged-A"}}
	if err != nil || !out.Held || out.Entry == nil || out.Entry.State != StateWaiting || !reflect.DeepEqual(out.Entry.After, want) {
		t.Fatalf("merged superseded dependency: %+v err=%v", out, err)
	}
}
