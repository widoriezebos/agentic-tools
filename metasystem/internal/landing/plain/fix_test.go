package plain

import (
	"github.com/widoriezebos/agentic-tools/metasystem/internal/conflict"
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
