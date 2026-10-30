package plain

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// Real Git merges main's records into the proven batch and publishes the
// fold, as landing push does; main's keeper then observes the pushed tree.
func TestAFoldOfGoalPagesKeepsTheBatchsGreen(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"goal pages", "a scoped batch", "a design page", "a page that does not parse", "the folded tree is red", "a refused publication"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b := newBed(t)
			mainCommit, mainTree := b.git(b.checkout, "rev-parse", "HEAD"), b.git(b.checkout, "rev-parse", "HEAD^{tree}")
			sha := b.seat("seat-a", "goal-a")
			b.handIn("seat-a", "goal-a", sha)
			proven := b.merge("goal-a")
			provenTree := b.git(b.checkout, "rev-parse", "HEAD^{tree}")
			var batch Batch
			if err := json.Unmarshal(mainMoveFixture(t, "batch.json"), &batch); err != nil {
				t.Fatal(err)
			}
			batch.Lane.Root, batch.Lane.Install = b.checkout, b.install
			batch.Base, batch.State = b.originMain(), BatchRunning
			batch.Members = []GoalSHA{{Goal: "goal-a", SHA: sha}}
			if err := writeBatch(b.install, &batch); err != nil {
				t.Fatal(err)
			}
			var fixture Result
			if err := json.Unmarshal(mainMoveFixture(t, "results.jsonl"), &fixture); err != nil {
				t.Fatal(err)
			}
			at := bedNow.Format(time.RFC3339)
			// Main's own full check sets its four-hour clock.
			trunk := fixture
			trunk.Trunk, trunk.Commit, trunk.Tree, trunk.FullTree, trunk.At, trunk.FullAt, trunk.Attempt, trunk.Goals = true, mainCommit, mainTree, mainTree, at, at, "main-full", nil
			result := fixture
			result.Trunk, result.Commit, result.Tree, result.FullTree, result.At, result.FullAt, result.Attempt = false, proven, provenTree, provenTree, at, at, "batch-full"
			result.BatchID, result.BatchMembers, result.Goals = batch.ID, batch.Members, batch.Members
			if name == "a scoped batch" {
				result.Scope, result.Base, result.FullTree = "scoped", mainTree, mainTree
			}
			for _, row := range []Result{trunk, result} {
				if err := appendLine(resultsPath(b.install), row); err != nil {
					t.Fatal(err)
				}
			}
			seed := filepath.Join(b.root, "seed")
			var folded []string
			for _, id := range []string{"lane-first-run-fixes", "lane-rehearsal-before-landing"} {
				data := mainMoveFixture(t, id+".md")
				if _, problems := goal.ParseFile(data); len(problems) != 0 {
					t.Fatalf("fixture %s: %v", id, problems)
				}
				path := "metasystem/plans/goals/" + id + ".md"
				switch {
				case name == "a design page":
					path, data = "metasystem/plans/designs/x.md", []byte("A design, "+id+".\n")
				case name == "a page that does not parse":
					text, _, _ := strings.Cut(string(data), "Integrity: ")
					data = []byte(text)
				}
				b.write(filepath.Join(seed, path), string(data))
				b.git(seed, "add", "-A")
				b.git(seed, "commit", "-qm", "goal slice-start "+id)
				folded = append(folded, b.git(seed, "rev-parse", "HEAD"))
			}
			b.git(seed, "push", "--quiet", "origin", "main")
			var foldTree string
			if name == "the folded tree is red" {
				b.git(b.checkout, "fetch", "--quiet", "origin")
				foldTree = b.git(b.checkout, "merge-tree", "--write-tree", proven, "origin/main")
				red := Result{Tree: foldTree, Commit: "red-fold", Result: Red, Scope: "full", At: at, Attempt: "red-fold"}
				if err := appendLine(resultsPath(b.install), red); err != nil {
					t.Fatal(err)
				}
			}
			before, err := Results(b.install)
			if err != nil {
				t.Fatal(err)
			}
			var refusal *Refusal
			switch name {
			case "a design page", "a page that does not parse", "the folded tree is red":
				out, err := PushChecked(b.install, b.checkout, bedNow, nil)
				want := CodeUnproven
				if name == "the folded tree is red" {
					want = CodeRed
				}
				after, readErr := Results(b.install)
				if !errors.As(err, &refusal) || refusal.Code != want || out.Changed || b.originMain() != folded[1] || readErr != nil || len(after) != len(before) {
					t.Fatalf("fold pushed or recorded: %+v %v; rows %d -> %d", out, err, len(before), len(after))
				}
				return
			case "a refused publication":
				refused := errors.New("the publication is refused")
				out, err := PushChecked(b.install, b.checkout, bedNow, func(string, string) error { return refused })
				kept, readErr := Results(b.install)
				if !errors.Is(err, refused) || out.Changed || b.originMain() != folded[1] || readErr != nil || len(kept) != len(before)+1 {
					t.Fatalf("refused publication: %+v %v; rows %d -> %d", out, err, len(before), len(kept))
				}
				out, err = PushChecked(b.install, b.checkout, bedNow, nil)
				after, readErr := Results(b.install)
				if err != nil || !out.Changed || b.originMain() != out.Commit || readErr != nil || len(after) != len(kept)+1 || after[len(after)-1].Attempt != kept[len(kept)-1].Attempt {
					t.Fatalf("the repeat push: %+v %v; rows %d -> %d", out, err, len(kept), len(after))
				}
				return
			}
			out, err := PushChecked(b.install, b.checkout, bedNow, nil)
			if err != nil || !out.Changed || b.originMain() != out.Commit {
				t.Fatalf("goal pages fold: %+v %v", out, err)
			}
			parents := strings.Fields(b.git(b.checkout, "show", "-s", "--format=%P", out.Commit))
			if !reflect.DeepEqual(parents, []string{proven, folded[1]}) {
				t.Fatalf("fold parents=%v", parents)
			}
			push, ok, err := LastPush(b.install)
			if err != nil || !ok || !reflect.DeepEqual(push.FoldedCommits, folded) {
				t.Fatalf("folded push record: %+v %v", push, err)
			}
			// The push closes the proof loop with a marked copy of the newest row.
			after, err := Results(b.install)
			if err != nil || len(after) != len(before)+2 || !after[len(after)-1].LoopClosed {
				t.Fatalf("rows %d -> %d: %v", len(before), len(after), err)
			}
			row := after[len(before)]
			want := Result{Commit: out.Commit, Tree: out.Tree, Result: Green, Attempt: row.Attempt, At: at, Scope: "inherited",
				ScopeReason: "the batch took goal records from main; its green stands",
				Reason:      "inherits green from tree " + Short(provenTree) + "; the batch took goal records from main",
				Base:        provenTree, BaseCommit: proven, Log: result.Log, FullTree: result.FullTree, FullAt: result.FullAt,
				Environment: result.Environment, Goals: result.Goals, BatchID: result.BatchID, BatchMembers: result.BatchMembers}
			if row.Attempt == "" || row.Attempt == result.Attempt || !reflect.DeepEqual(row, want) {
				t.Fatalf("fold row:\n got %+v\nwant %+v", row, want)
			}
			launches := 0
			seams := ProveSeams{Executable: func() (string, error) { return "fixture-engine", nil },
				Incidents: func(string, string, string) ([]goal.TrunkRedEntry, error) { return nil, nil },
				Launch: func(argv []string, _, _ string) (int64, error) {
					if launches++; !slices.Contains(argv, "--trunk") {
						t.Fatalf("not a check of main: %v", argv)
					}
					return 0, errors.New("the test counts the launch")
				}}
			observe := func(after time.Duration) error {
				seams.Now = func() time.Time { return bedNow.Add(after) }
				return RecoverMovedMain(b.install, b.checkout, seams)
			}
			if err := observe(30 * time.Minute); err != nil || launches != 0 {
				t.Fatalf("observation at 30m: launches=%d %v", launches, err)
			}
			err = observe(61 * time.Minute)
			due, dueErr := scopedProofDue(b.install, bedNow.Add(61*time.Minute))
			if name == "a scoped batch" {
				if !due || dueErr != nil || launches != 1 {
					t.Fatalf("a fold of a scoped green: due=%v launches=%d %v %v", due, launches, dueErr, err)
				}
				return
			}
			if err != nil || launches != 0 || due || dueErr != nil {
				t.Fatalf("a fold of a full green expired: launches=%d due=%v %v %v", launches, due, err, dueErr)
			}
			d := proofScope(b.install, b.checkout, Running{BatchID: "cheap", Tree: "candidate"}, seams)
			if strings.HasPrefix(d.ScopeReason, "full proof overdue") {
				t.Fatalf("a cheap batch after the folded push: %+v", d.scopeRecord)
			}
		})
	}
}
