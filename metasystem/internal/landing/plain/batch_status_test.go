package plain

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

func TestLandingBatchStatusProvingNeverIdle(t *testing.T) {
	t.Parallel()
	for _, known := range []bool{false, true} {
		t.Run(map[bool]string{false: "counting", true: "planned"}[known], func(t *testing.T) {
			t.Parallel()
			b := newStatusBed(t)
			b.view.Summary = "idle; its agent starts within one tick"
			data := []byte("partial plan")
			if known {
				var err error
				data, err = os.ReadFile("testdata/batch-proof-progress.log")
				if err != nil {
					t.Fatal(err)
				}
				// Repeated reports and an incomplete line never add a unit.
				data = append(data, []byte("landing package metasystem/cmd/metasystem 14 ok 144755\nlanding package partial")...)
			}
			log := filepath.Join(t.TempDir(), "proof.log")
			if err := os.WriteFile(log, data, 0600); err != nil {
				t.Fatal(err)
			}
			b.lines("running.json", Running{Commit: "main", Tree: "tree", Attempt: "proof", Pid: 42, Since: bedNow.Format(time.RFC3339Nano), Log: log})
			question, err := channel.Ask(channel.AskRequest{RepoRoot: b.install, About: "lane", Kind: "other", Machine: "fixture", Facts: []string{"A pending lane question"}, Now: bedNow})
			if err != nil {
				t.Fatal(err)
			}
			status := b.read(ProveSeams{Now: func() time.Time { return bedNow.Add(7 * time.Minute) }, Alive: func(Running) bool { return true }, Machine: func(string) (string, error) { return "fixture", nil }}, b.git(nil, nil))
			want := "proving (0 of counting units, 7.0 min); the lane proves main; the agent waits for the keeper; nothing to do"
			if known {
				want = "proving (4 of 233 units, 7.0 min); the lane proves main; the agent waits for the keeper; nothing to do"
			}
			if status.Summary != LaneQuestionHeadline(question) || status.ProofHeadline != want || strings.Contains(status.Summary, "idle") {
				t.Fatalf("summary %q proof %q; want %q", status.Summary, status.ProofHeadline, want)
			}
			encoded, err := json.Marshal(status)
			if err != nil {
				t.Fatal(err)
			}
			if known && (status.RunningProof.UnitsTotal == nil || *status.RunningProof.UnitsTotal != 233 || status.RunningProof.UnitsDone != 4) || !known && status.RunningProof.UnitsTotal != nil {
				t.Fatalf("count state: %s", encoded)
			}
		})
	}
}

func TestLandingBatchProvingPreservesSummaryPriorityAndDepth(t *testing.T) {
	t.Parallel()
	for _, problem := range []bool{false, true} {
		t.Run(map[bool]string{false: "proof", true: "hand-in problem"}[problem], func(t *testing.T) {
			t.Parallel()
			b := newStatusBed(t)
			b.record.CustodyEpoch = 1
			b.lines("batch.json", Batch{ID: "batch", Base: "base", Lane: b.record, State: BatchRunning, DepthClass: "full", Members: []GoalSHA{{Goal: "a", SHA: "sha-a"}}})
			b.lines("running.json", Running{BatchID: "batch", Commit: "main", Tree: "tree", Attempt: "proof", Pid: 42, Since: bedNow.Format(time.RFC3339Nano)})
			if err := os.WriteFile(filepath.Join(b.install, "metasystem.conf"), []byte("proof.trunk-every=1h\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if problem {
				b.view.Wake = &lane.Wake{Unread: []string{"hand-in a cannot be read"}}
			}
			status := b.read(ProveSeams{Now: func() time.Time { return bedNow }, Alive: func(Running) bool { return true }, Git: func(_ string, args ...string) (string, error) {
				if strings.Join(args, " ") != "rev-parse --verify HEAD^{commit}" {
					t.Fatalf("unexpected Git read: %v", args)
				}
				return "main", nil
			}}, b.git(nil, nil))
			want := status.ProofHeadline
			if problem {
				want = "hand-in a cannot be read; run: metasystem landing resume"
			}

			if status.Summary != want {
				t.Fatalf("summary %q; want %q", status.Summary, want)
			}
		})
	}
}
