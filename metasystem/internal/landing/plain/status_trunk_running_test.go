package plain

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

func TestReadStatusTrunkRedWaitsForRunningTrunkProof(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	b.lines("results.jsonl", Result{Trunk: true, Commit: "main", Result: Red, Failed: []FailedUnit{{Unit: "fixture"}}})
	b.lines("running.json", Running{Trunk: true, Commit: "main", Tree: "trunk-tree", Attempt: "trunk-1", Since: bedNow.Format(time.RFC3339)})
	seams := ProveSeams{
		Alive: func(Running) bool { return true },
		Now:   func() time.Time { return bedNow },
		Incidents: func(string, string, string) ([]goal.TrunkRedEntry, error) {
			return []goal.TrunkRedEntry{{ID: "red-1", Identity: "red-1"}}, nil
		},
	}
	want := "main main red; its trunk proof runs since " + bedNow.Local().Format("15:04") + " (attempt trunk-1); wait"
	status := readStatus(b.home, b.record, b.view, seams, b.git(nil, nil))
	if status.ProofHeadline != want || status.Summary != want || len(status.PendingActions) != 0 || len(status.Problems) != 0 {
		t.Fatalf("running trunk proof: %+v; want %q and no person act", status, want)
	}
	t.Logf("running headline: %s", status.Summary)
	if err := os.Remove(filepath.Join(Dir(b.install), "running.json")); err != nil {
		t.Fatal(err)
	}
	status = readStatus(b.home, b.record, b.view, seams, b.git(nil, nil))
	want = "main main proven red: fixture; incident red-1; hot-fix, then metasystem landing prove --trunk"
	if status.ProofHeadline != want || status.Summary != want {
		t.Fatalf("no running trunk proof: %+v; want %q", status, want)
	}
	t.Logf("idle headline: %s", status.Summary)
}
