package plain

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// readParents exercises the public checkout adapter and rejects extra Git work.
func (b *statusBed) readParents(roots []string, listing string, listErr error) (Status, int) {
	b.t.Helper()
	calls, listings := 0, 0
	seams := ProveSeams{Now: func() time.Time { return bedNow }, Incidents: func(string, string, string) ([]goal.TrunkRedEntry, error) { return nil, nil }}
	seams.Git = func(_ string, args ...string) (string, error) {
		calls++
		switch strings.Join(args, " ") {
		case "rev-parse --verify --quiet refs/remotes/origin/main^{commit}":
			return "main", nil
		case "rev-parse --verify HEAD^{commit}":
			return "head", nil
		// Pending policy requests retain their existing checks of current work.
		case "cat-file -e main^{commit}", "cat-file -e sha-z2^{commit}", "cat-file -e base^{commit}", "cat-file -e external^{commit}", "merge-base --is-ancestor sha-z2 main", "merge-base --is-ancestor base main", "merge-base --is-ancestor external main":
			return "", nil
		case "cat-file -e replacement^{commit}", "cat-file -e new^{commit}", "cat-file -e outside^{commit}", "cat-file -e sha-a2^{commit}":
			return "", errors.New("fixture commit absent from main")
		}
		want := append([]string{"rev-list", "--parents", "--ignore-missing"}, roots...)
		if strings.Join(args, " ") != strings.Join(want, " ") || listings != 0 {
			b.t.Fatalf("unexpected Git call: %v; want one %v", args, want)
		}
		listings++
		return listing, listErr
	}
	status := ReadStatus(b.home, b.record, b.view, seams)
	if listings != 1 {
		b.t.Fatalf("parent listings = %d, want one", listings)
	}
	return status, calls
}

func TestReadStatusParentGraphPreservesPushRanges(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"merge", "non-ancestor", "force reset", "missing endpoint", "listing error"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			b := newStatusBed(t)
			old, pushed := bedNow.Add(-96*time.Hour).Format(time.RFC3339), bedNow.Add(-120*time.Hour).Format(time.RFC3339)
			b.lines("queue.jsonl", Line{Goal: "old", SHA: "landed", At: old}, Line{Goal: "old", SHA: "new", At: old}, Line{Goal: "shared", SHA: "base", At: old}, Line{Goal: "outside", SHA: "outside", At: old}, Line{Goal: "external", SHA: "external", At: old})
			pushes := []any{Pushed{Old: "from", Commit: "to", At: pushed}}
			roots := []string{"main", "to", "from"}
			listing := "main to external\nto first side\nfirst base\nside landed\nlanded base\nfrom base\nbase\nexternal\n"
			if kind == "non-ancestor" {
				listing = strings.Replace(listing, "from base\n", "from outside\noutside base\n", 1)
			}
			if kind == "force reset" {
				listing = "main landed external\nto discarded\ndiscarded landed\nlanded base\nfrom base\nbase\nexternal\n"
			}
			if kind == "missing endpoint" {
				pushes = append([]any{Pushed{Old: "absent", Commit: "to", At: pushed}}, pushes...)
				roots = []string{"main", "to", "absent", "from"}
			}
			b.lines("pushes.jsonl", pushes...)
			listErr := map[string]error{"listing error": errors.New("parent listing failed")}[kind]
			status, _ := b.readParents(roots, listing, listErr)
			wantProblem := map[string]string{"missing endpoint": "push to: commit absent is absent", "listing error": "parent listing failed"}[kind]
			if problem := strings.Join(status.Problems, " "); (wantProblem == "" && problem != "") || !strings.Contains(problem, wantProblem) {
				t.Fatalf("problems=%v; want %q", status.Problems, wantProblem)
			}
			if listErr != nil {
				return
			}
			got := map[string]Entry{}
			for _, entry := range status.Queue {
				got[entry.SHA] = entry
			}
			if got["landed"].State != StateLanded || got["landed"].LandedAt != pushed {
				t.Fatalf("old superseded landing: %+v", got["landed"])
			}
			if got["base"].LandedAt != "" || got["external"].LandedAt != "" || got["outside"].State != StateWaiting {
				t.Fatalf("containment or range subtraction: %+v", got)
			}
		})
	}
}

func TestReadStatusHistoricalGitCostIsConstant(t *testing.T) {
	t.Parallel()
	for _, size := range []struct{ entries, pushes int }{{1, 1}, {40, 1}, {1, 40}} {
		t.Run(fmt.Sprintf("entries-%d-pushes-%d", size.entries, size.pushes), func(t *testing.T) {
			t.Parallel()
			b := newStatusBed(t)
			old := bedNow.Add(-72 * time.Hour).Format(time.RFC3339)
			var lines, pushes []any
			listing, roots := "from\n", []string{"main"}
			mainParents := "main"
			for i := 0; i < size.entries; i++ {
				for _, sha := range []string{"external", "landed", "unlanded"} {
					commit := fmt.Sprintf("%s-%d", sha, i)
					lines = append(lines, Line{Goal: sha, SHA: commit, At: old})
					if sha != "unlanded" {
						mainParents += " " + commit
						listing += commit + " from\n"
					}
				}
			}
			for _, g := range []string{"external", "landed", "unlanded"} {
				lines = append(lines, Line{Goal: g, SHA: "replacement", At: bedNow.Format(time.RFC3339)})
			}
			for i := 0; i < size.pushes; i++ {
				tip := fmt.Sprintf("push-%d", i)
				roots = append(roots, tip)
				if i == 0 {
					roots = append(roots, "from")
				}
				listing += tip + " from\n"
				pushes = append(pushes, Pushed{Old: "from", Commit: tip, At: old})
			}
			listing += mainParents + "\n"
			b.lines("queue.jsonl", lines...)
			b.lines("pushes.jsonl", pushes...)
			status, calls := b.readParents(roots, listing, nil)
			if calls != 10 || len(status.Problems) > 0 {
				t.Fatalf("Git calls=%d, want 3 ancestry/gate calls plus 7 unchanged current-work policy calls; problems=%v", calls, status.Problems)
			}
			current, history := QueueHistory(status.Queue)
			if len(current) != 3 || len(history) != 3*size.entries {
				t.Fatalf("current=%d history=%d", len(current), len(history))
			}
		})
	}
}
