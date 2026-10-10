package plain

import (
	"bytes"
	"errors"

	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/conflict"
)

func TestFixRecordAdvanceRequiresFirstParentAndSameBatch(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"same commit", "main refresh", "second parent only", "different batch", "different members", "trunk", "unreadable ancestry"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			install := t.TempDir()
			fix := Fix{Attempt: "red", Round: 1, Parent: "batch-commit", Goal: "goal-a", Units: []string{"unit-a"}, Job: "build", Commit: "repair", State: "reviewing"}
			previous := Running{Attempt: fix.Attempt, Commit: fix.Parent, Checkpoint: true, BatchID: "batch", BatchMembers: []GoalSHA{{Goal: fix.Goal, SHA: "unit"}}}
			if err := WriteFix(install, &fix); err != nil {
				t.Fatal(err)
			}
			if err := withLock(install, func() error { return writeRunning(install, previous) }); err != nil {
				t.Fatal(err)
			}
			next := previous
			next.Attempt, next.Commit = "green", "refresh"
			chain := "refresh\nrepair\nbatch-commit\n"
			switch name {
			case "same commit":
				next.Commit = fix.Commit
			case "second parent only":
				chain = "refresh\nmain\n"
			case "different batch":
				next.BatchID = "another"
			case "different members":
				next.BatchMembers = []GoalSHA{{Goal: fix.Goal, SHA: "other-unit"}}
			case "trunk":
				next.Trunk = true
			}
			seams := ProveSeams{Git: func(dir string, args ...string) (string, error) {
				if dir != install || strings.Join(args, " ") != "rev-list --first-parent refresh" {
					t.Fatalf("unexpected ancestry query in %s: %v", dir, args)
				}
				if name == "unreadable ancestry" {
					return "", errors.New("ancestry unavailable")
				}
				return chain, nil
			}}
			err := withLock(install, func() error { return advanceFix(install, install, next, seams) })
			old, readErr := FixForAttempt(install, fix.Attempt)
			followed, followErr := FixForAttempt(install, next.Attempt)
			if readErr != nil || followErr != nil || old == nil {
				t.Fatalf("records old=%+v new=%+v: %v %v", old, followed, readErr, followErr)
			}
			if name == "unreadable ancestry" {
				if err == nil || old.State != "reviewing" || followed != nil {
					t.Fatalf("failed ancestry changed repair: old=%+v new=%+v err=%v", old, followed, err)
				}
				return
			}
			if err != nil || old.State != "closed" {
				t.Fatalf("old repair %+v: %v", old, err)
			}
			if name == "same commit" || name == "main refresh" {
				if followed == nil || followed.Commit != fix.Commit || followed.Parent != fix.Parent || followed.State != "reviewing" || followed.Round != 1 {
					t.Fatalf("repair did not follow: %+v", followed)
				}
			} else if followed != nil {
				t.Fatalf("unrelated attempt inherited repair: %+v", followed)
			}
		})
	}
}

func TestFixRecordStaleReadersDoNotWrite(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	fix := Fix{Attempt: "older", Goal: "goal-a", Units: []string{"unit-a"}, Job: "build", Commit: "repair", State: "reviewing"}
	if err := WriteFix(b.install, &fix); err != nil {
		t.Fatal(err)
	}
	b.lines("running.json", Running{Attempt: "current", Checkpoint: true, BatchMembers: []GoalSHA{{Goal: fix.Goal, SHA: "unit"}}})
	path := filepath.Join(Dir(b.install), "fixes", fix.Attempt+".json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if active, err := ActiveFix(b.install); err != nil || active != nil {
		t.Fatalf("stale active fix %+v: %v", active, err)
	}
	status := b.read(ProveSeams{}, b.git(nil, nil))
	if status.RunningFix != nil {
		t.Fatalf("status shows stale fix: %+v", status.RunningFix)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("reader changed fix record: %s %v", after, err)
	}
	if err := withLock(b.install, func() error { return closeFix(b.install, fix.Goal) }); err != nil {
		t.Fatal(err)
	}
	closed, err := FixForAttempt(b.install, fix.Attempt)
	if err != nil || closed == nil || closed.State != "closed" {
		t.Fatalf("locked close %+v: %v", closed, err)
	}
}

func TestReadStatusLaneFixHeadline(t *testing.T) {
	t.Parallel()
	for _, state := range []string{"running", "reviewing", "done"} {
		t.Run(state, func(t *testing.T) {
			t.Parallel()
			b := newStatusBed(t)
			if err := os.MkdirAll(filepath.Join(Dir(b.install), "fixes"), 0o755); err != nil {
				t.Fatal(err)
			}
			b.lines("fixes/attempt.json", Fix{Goal: "goal-a", Units: []string{"unit-a", "unit-b"}, Job: "build-1", Read: "read-1", Commit: "0123456789abcdef", State: state})
			status := b.read(ProveSeams{}, laneGit{main: func() (string, error) { return "main", nil }, contains: func(string, string) (bool, error) { return false, nil }})
			want := "Fixing unit-a, unit-b of goal-a on 0123456789ab (fix round 1)"
			if state == "done" {
				if status.RunningFix != nil || strings.HasPrefix(status.ProofHeadline, "Fixing") {
					t.Fatalf("completed repair still runs: %+v", status)
				}
			} else if status.RunningFix == nil || status.ProofHeadline != want || status.Summary != want {
				t.Fatalf("headline %q summary %q fix %+v, want %q", status.ProofHeadline, status.Summary, status.RunningFix, want)
			}
		})
	}
}

func TestReadStatusResolvingMergeNamesPaths(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	fix := &Fix{Goal: "goal-a", Units: []string{"lane-merge-1"}, Commit: "batch", Tip: "tip", State: "resolving", Attempt: "merge-attempt", Paths: []conflict.Path{{Path: "code.go", Class: conflict.Builder}, {Path: "bundle.js", Class: conflict.Generated}}}
	if err := WriteFix(b.install, fix); err != nil {
		t.Fatal(err)
	}
	status := b.read(ProveSeams{Git: func(string, ...string) (string, error) { return "tip", nil }}, laneGit{main: func() (string, error) { return "main", nil }, contains: func(string, string) (bool, error) { return false, nil }})
	if status.Summary != "Resolving 2 conflicts of goal-a (code.go, bundle.js)" || status.RunningFix == nil {
		t.Fatalf("status=%+v", status)
	}
}

func TestReadStatusLaneFixUnreadable(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	if err := os.MkdirAll(filepath.Join(Dir(b.install), "fixes"), 0o755); err != nil {
		t.Fatal(err)
	}
	b.lines("fixes/attempt.json", Fix{State: "running"})
	status := b.read(ProveSeams{}, laneGit{main: func() (string, error) { return "main", nil }, contains: func(string, string) (bool, error) { return false, nil }})
	if status.RunningFix != nil || !strings.Contains(strings.Join(status.Problems, " "), "incomplete lane fix") {
		t.Fatalf("damaged repair disappeared: %+v", status)
	}
}

func TestFixRecordClosesOnReturnAndPush(t *testing.T) {
	t.Parallel()
	for _, outcome := range []string{"return", "push"} {
		t.Run(outcome, func(t *testing.T) {
			t.Parallel()
			b := newBed(t)
			sha := b.seat("seat-a", "goal-a")
			b.handIn("seat-a", "goal-a", sha)
			head := b.merge("goal-a")
			fix := Fix{Attempt: "attempt", Round: 1, Goal: "goal-a", Units: []string{"unit-a"}, Job: "build", Commit: head, State: "reviewing"}
			if err := WriteFix(b.install, &fix); err != nil {
				t.Fatal(err)
			}
			if outcome == "return" {
				_, changed, err := ReturnDesignRefused(b.install, DesignCheck{Goal: "goal-a", Commit: sha, Reason: "accepted design changed"}, bedNow)
				if err != nil || !changed {
					t.Fatalf("return changed=%v: %v", changed, err)
				}
			} else {
				if result := b.prove(b.greenScript); result.Result != Green {
					t.Fatalf("proof %+v", result)
				}
				if _, err := b.push(); err != nil {
					t.Fatal(err)
				}
			}
			// A repeated outcome finishes a fix record whose earlier close failed.
			if err := WriteFix(b.install, &fix); err != nil {
				t.Fatal(err)
			}
			if outcome == "return" {
				if _, changed, err := ReturnDesignRefused(b.install, DesignCheck{Goal: "goal-a", Commit: sha, Reason: "accepted design changed"}, bedNow); err != nil || changed {
					t.Fatalf("repeated return changed=%v: %v", changed, err)
				}
			} else if _, err := b.push(); err != nil {
				t.Fatal(err)
			}
			if active, err := ActiveFix(b.install); err != nil || active != nil {
				t.Fatalf("fix still active %+v: %v", active, err)
			}
			data, err := os.ReadFile(filepath.Join(Dir(b.install), "fixes", "attempt.json"))
			if err != nil || !strings.Contains(string(data), `"state":"closed"`) {
				t.Fatalf("closure %s: %v", data, err)
			}
		})
	}
}

func TestFixBuildingHeadlineUsesBatchParent(t *testing.T) {
	t.Parallel()
	install := t.TempDir()
	fix := Fix{Attempt: "attempt", Round: 1, Goal: "goal-a", Units: []string{"unit-a"}, Job: "build", Parent: "0123456789abcdef", State: "building"}
	if err := WriteFix(install, &fix); err != nil {
		t.Fatal(err)
	}
	active, err := ActiveFix(install)
	if err != nil || active == nil {
		t.Fatalf("building fix %+v: %v", active, err)
	}
	if headline := fixHeadline(active); headline != "Fixing unit-a of goal-a on 0123456789ab (fix round 1)" {
		t.Fatalf("building headline %q", headline)
	}
}

func TestFixRecordDamagedOtherGoalDoesNotBlockReturnOrPush(t *testing.T) {
	t.Parallel()
	for _, outcome := range []string{"return", "push"} {
		t.Run(outcome, func(t *testing.T) {
			t.Parallel()
			b := newBed(t)
			sha := b.seat("seat-a", "goal-a")
			b.handIn("seat-a", "goal-a", sha)
			head := b.merge("goal-a")
			fix := Fix{Attempt: "attempt", Round: 1, Goal: "goal-a", Units: []string{"unit-a"}, Job: "build", Commit: head, State: "reviewing"}
			if err := WriteFix(b.install, &fix); err != nil {
				t.Fatal(err)
			}
			b.write(filepath.Join(Dir(b.install), "fixes", "other.json"), `{"goal":"other","state":"building"}`)
			if outcome == "push" {
				b.prove(b.greenScript)
				if _, err := b.push(); err != nil {
					t.Fatal(err)
				}
			} else {
				if _, _, err := ReturnDesignRefused(b.install, DesignCheck{Goal: "goal-a", Commit: sha, Reason: "design changed"}, bedNow); err != nil {
					t.Fatal(err)
				}
			}
			closed, err := FixForAttempt(b.install, fix.Attempt)
			if err != nil || closed.State != "closed" {
				t.Fatalf("fix %+v: %v", closed, err)
			}
		})
	}
}
