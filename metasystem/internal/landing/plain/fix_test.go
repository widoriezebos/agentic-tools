package plain

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
			if err := WriteFix(b.install, fix); err != nil {
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
			if err := WriteFix(b.install, fix); err != nil {
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
	if err := WriteFix(install, fix); err != nil {
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
