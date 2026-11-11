package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The physical Git adapter proves that an adopted result is counted and
// committed, and that unrelated bytes never become the idle builder's work.
func TestIntentStoppedUnitRebuildGitAdapter(t *testing.T) {
	t.Parallel()
	for _, dirt := range []bool{false, true} {
		t.Run(fmt.Sprintf("unrelated-dirt=%t", dirt), func(t *testing.T) {
			t.Parallel()
			c := newConnectionBed(t)
			owners := c.connectionOwners()
			owners.prove = enrolledPersonProver(t, c.root(), c.manager.Now())
			build := func(brief string) (int, intentResult) {
				return c.runJSON(owners, append([]string{"work", "build", c.id, "rebuild", "--brief", c.brief(brief, brief+"\n"), "--lines", "10"}, workCheck...)...)
			}
			c.edits = map[string]string{"kept.txt": "the stopped unit's change\n"}
			code, first := build("first.md")
			if code != 0 {
				t.Fatalf("initial build: %d %+v", code, first)
			}
			run := resultData(t, first)["run"].(string)
			inputs := resultData(t, first)["inputs"].(string)
			request, err := os.ReadFile(filepath.Join(inputs, "request.json"))
			if err != nil {
				t.Fatal(err)
			}
			if code, refused := build("changed.md"); code == 0 || !strings.Contains(resultWords(refused), "UNIT_NAMED_INPUT_CHANGED") {
				t.Fatalf("live name accepted new inputs: %d %+v", code, refused)
			}
			if code, stopped := c.runJSON(owners, "work", "stop", "run:"+run); code != 0 || c.runRecord(run).State != "cancelled" {
				t.Fatalf("stop: %d %+v", code, stopped)
			}
			if dirt {
				if err := os.WriteFile(filepath.Join(c.worktree, "unrelated.txt"), []byte("someone else's change\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			c.edits = nil
			code, second := build("replacement.md")
			fresh := resultData(t, second)["run"].(string)
			r := c.runRecord(fresh)
			if fresh == run || r.StoppedRun != run || len(r.Notes) != 1 || !strings.Contains(r.Notes[0], "this build starts run "+fresh) {
				t.Fatalf("no fresh run: %+v", r)
			}
			if resultData(t, second)["startingResult"] != r.Rounds[0].StartingResult || !strings.Contains(fmt.Sprint(resultData(t, second)["notes"]), "this build starts run "+fresh) {
				t.Fatalf("rebuild decision not reported: %+v", second)
			}
			archive := filepath.Join(c.unitRoot, run)
			retained, err := os.ReadFile(filepath.Join(archive, "released-inputs", "request.json"))
			if err != nil || !bytes.Equal(retained, request) {
				t.Fatalf("old inputs not archived: %q %v", retained, err)
			}
			if _, err := os.Stat(filepath.Join(archive, "released-named.json")); err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(filepath.Join(r.Rounds[0].Directory, "build-before", "worktree.diff"))
			if err != nil {
				t.Fatal(err)
			}
			if dirt {
				if code == 0 || r.Rounds[0].Outcome != "build-gap" || r.Rounds[0].BuildLines != 0 || !bytes.Contains(before, []byte("unrelated.txt")) ||
					!strings.Contains(r.Rounds[0].StartingResult, "unrelated.txt") || strings.Contains(r.Rounds[0].StartingResult, "kept.txt") {
					t.Fatalf("unrelated dirt counted: %d %+v", code, r.Rounds[0])
				}
				return
			}
			if code != 0 || r.Rounds[0].BuildLines != 1 || len(before) != 0 || !strings.Contains(r.Rounds[0].StartingResult, "change (1 lines) is this round's starting result") {
				t.Fatalf("kept result not counted: %d %+v", code, r.Rounds[0])
			}
			if code, reviewed := c.runJSON(owners, "work", "review", "run:"+fresh); reviewed.Outcome != intentInProgress || c.commits != 1 {
				t.Fatalf("retained change not committed: %d %+v", code, reviewed)
			}
			if content := connectionGit(t, c.worktree, "show", "HEAD:kept.txt"); content != "the stopped unit's change" {
				t.Fatalf("wrong committed bytes: %q", content)
			}
		})
	}
}
