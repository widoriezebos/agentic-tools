package plain

import (
	"errors"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

func TestResolveConflictReturnLimitHoldsWaitingEntry(t *testing.T) {
	t.Parallel()
	b := newResolveFixture(t)
	if err := os.Remove(queuePath(b.install)); err != nil {
		t.Fatal(err)
	}
	b.seams.Proof.Policy = func(key string) (PolicyValue, error) {
		if key != "landing.on-red" {
			t.Fatalf("unexpected policy %q", key)
		}
		return PolicyValue{Value: "auto"}, nil
	}
	for index, sha := range []string{"first-sha", "second-sha"} {
		if _, _, err := HandIn(b.install, Line{Goal: "goal", SHA: sha}); err != nil {
			t.Fatal(err)
		}
		failures := []string{"first-test", "second-test"}
		if index == 1 {
			failures = []string{"third-test"}
		}
		if err := withLock(b.install, func() error {
			entry, changed, err := returnLocked(b.install, "goal", "own test failure", &Cause{Kind: "own", Goal: "goal", SHA: sha, Tests: failures}, nil, bedNow, b.seams.Proof)
			if err == nil && (!changed || entry.State != StateReturned || entry.ReturnOrigin == nil || entry.ReturnOrigin.Person != nil) {
				t.Fatalf("automatic return=%+v changed=%v", entry, changed)
			}
			return err
		}); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := HandIn(b.install, Line{Goal: "goal", SHA: "goal-sha"}); err != nil {
		t.Fatal(err)
	}
	runs, aborted := 0, false
	git := b.seams.Git
	b.seams.Git = func(dir string, args ...string) (string, error) {
		if strings.Join(args, " ") == "merge --abort" {
			aborted = true
		}
		return git(dir, args...)
	}
	b.seams.Run = func([]string, string, *os.File, func(int64) error) error {
		runs++
		if !aborted {
			return exec.Command("/usr/bin/false").Run()
		}
		return nil
	}
	out, err := b.resolve()
	var refusal *Refusal
	if !errors.As(err, &refusal) || refusal.Code != "LANE_RETURN_PERSON" || !strings.Contains(refusal.Reason, "two automatic returns") {
		t.Fatalf("return limit was not enforced: out=%+v err=%v", out, err)
	}
	if out.Outcome != "held" || !out.Held || out.Exit != 0 || out.Entry == nil || out.Entry.State != StateWaiting || !out.Entry.Held || out.Cause == nil || out.Cause.Kind != "own" || runs != 0 {
		t.Errorf("refused conflict return=%+v runs=%d", out, runs)
	}
	entry, ok, err := Latest(b.install, "goal")
	if err != nil || !ok || entry.SHA != "goal-sha" || entry.State != StateWaiting || !entry.Held {
		t.Errorf("queue did not retain the hold: entry=%+v found=%v err=%v", entry, ok, err)
	}
	second, err := b.resolve()
	if err != nil || second.Outcome != "held" || !second.Held || runs != 0 {
		t.Fatalf("held conflict ran again: out=%+v runs=%d err=%v", second, runs, err)
	}
}

func TestResolveReturnsAllGeneratedSetsWithoutReplayOrCleanup(t *testing.T) {
	t.Parallel()
	b := newResolveFixture(t)
	b.paths += "metasystem/unused/conflict\x00"
	out, err := b.resolve()
	if err != nil || out.Held || out.Outcome != "returned" || out.Entry == nil || out.Entry.State != StateReturned || len(out.Command) != 0 || len(out.Conflict.Paths) != 2 || !strings.Contains(out.Reason, "metasystem/out/conflict") || !strings.Contains(out.Reason, "metasystem/unused/conflict") {
		t.Fatalf("return=%+v err=%v", out, err)
	}
	if !reflect.DeepEqual(b.writes, [][]string{{"merge", "--abort"}}) {
		t.Fatalf("return changed outputs: %v", b.writes)
	}
	b.record(out)
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
