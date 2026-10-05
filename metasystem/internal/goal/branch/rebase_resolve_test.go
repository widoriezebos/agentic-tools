package branch

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestRebaseResolveRound(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"green", "mixed", "outside", "red", "answer", "stale-answer"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			f := newRebaseFixture(t)
			f.reviewedUnit()
			f.changed, f.replayErr, f.conflict = true, errors.New("conflict"), "metasystem/source\x00"
			dir := t.TempDir()
			f.d.repository.effects.Open = func(string, string, bool) (string, func(), error) { return dir, func() { f.closed++ }, nil }
			if mode == "mixed" {
				f.conflict += "metasystem/gen/out\x00"
				f.contract = rebaseTestContract(t, `[{"paths":["gen/**"],"command":["compile"],"then":["finish"],"cwd":"src"}]`)
				f.d.run = func(argv []string, _ string, _ *os.File, _ func(int64) error) error {
					f.commands = append(f.commands, argv)
					return nil
				}
			}
			if mode == "answer" || mode == "stale-answer" {
				f.judgement = true
				original := "original"
				if mode == "stale-answer" {
					original = "another-blob"
				}
				path := filepath.Join(f.req.Repo, "artifacts/agents/goals/goal-a/conflict.json")
				if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(`{"paths":[{"path":"metasystem/source","original":"`+original+`","main":"main","goal":"goal","resolution":"keep main's"}]}`), 0600); err != nil {
					t.Fatal(err)
				}
			}
			calls, finished := 0, false
			git := f.d.git
			f.d.git = func(repo string, args ...string) ([]byte, error) {
				if len(args) > 6 && args[0] == "ls-tree" {
					return []byte(strings.Join(args[6:], "\x00") + "\x00"), nil
				}
				switch strings.Join(args, " ") {
				case "checkout --conflict=diff3 -- metasystem/source":
					path := filepath.Join(dir, "metasystem/source")
					if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
						t.Fatal(err)
					}
					return nil, os.WriteFile(path, []byte("context\n<<<<<<< ours\nmain insertion\n||||||| base\noriginal text\n=======\ngoal insertion\n>>>>>>> theirs\ncontext\n"), 0600)
				case "diff --no-ext-diff --no-textconv --no-renames --binary HEAD", "diff --no-ext-diff --no-textconv --no-renames --binary HEAD -- . :(exclude,literal)metasystem/source":
					if mode == "outside" && finished {
						return []byte("diff --git a/another b/another\n--- a/another\n+++ b/another\n@@ -1 +1 @@\n-old\n+new\n"), nil
					}
					return nil, nil
				case "diff --name-only --diff-filter=U -z":
					if finished {
						return nil, nil
					}
				}
				return git(repo, args...)
			}
			f.req.Resolve = func(stop RebaseResolution) (string, error) {
				calls++
				if stop.Base != policySecond || stop.Commit != policyMoved || stop.Unit != "u1" || stop.Worktree != dir || !reflect.DeepEqual(stop.Paths, []string{"metasystem/source"}) {
					t.Fatalf("stop %+v", stop)
				}
				for _, want := range []string{"Conflicts to resolve", "resolve exactly these hunks; touch no other path", "<<<<<<< main\nmain insertion\n||||||| base\noriginal text\n=======\ngoal insertion\n>>>>>>> goal"} {
					if !strings.Contains(stop.Conflicts, want) {
						t.Fatalf("missing %q in brief %s", want, stop.Conflicts)
					}
				}
				if mode == "answer" && !strings.Contains(stop.Conflicts, "Written resolution: keep main's") {
					t.Fatal(stop.Conflicts)
				}
				finished = true
				if mode == "red" {
					return "resolve/run.json", errors.New("proof-red")
				}
				return "resolve/run.json", nil
			}
			got, err := rebaseWith(f.req, f.d)
			if mode == "stale-answer" {
				var question *RebaseConflict
				if !errors.As(err, &question) || calls != 0 || len(question.Paths) != 1 {
					t.Fatalf("stale answer %+v %v calls %d", got, err, calls)
				}
			} else if mode == "red" || mode == "outside" {
				if err == nil || !strings.Contains(err.Error(), "resolve/run.json") || calls != 1 {
					t.Fatalf("failure %+v %v calls %d", got, err, calls)
				}
			} else {
				if err != nil || calls != 1 || f.continues != 1 || !reflect.DeepEqual(got.NeedsReview, []string{"u1"}) || f.carries != 0 || !reflect.DeepEqual(got.Resolved, []string{"metasystem/source"}) {
					t.Fatalf("resolved %+v %v calls %d", got, err, calls)
				}
				want := []string{"add -A -- metasystem/source", "continue", "keep", "checkout", "move"}
				if mode == "mixed" {
					want = append([]string{"restore --source=HEAD --staged --worktree -- metasystem/gen/out", "add -A -- metasystem/source", "add -A -- metasystem/gen/out"}, want[1:]...)
					if len(f.commands) != 2 {
						t.Fatalf("regeneration %v", f.commands)
					}
				}
				if !reflect.DeepEqual(f.events, want) {
					t.Fatalf("events %v want %v", f.events, want)
				}
				return
			}
			if f.tip != policyFirst || f.continues != 0 || f.pushes != 0 || !reflect.DeepEqual(f.events, []string{"abort"}) {
				t.Fatalf("aborted %+v", f)
			}
		})
	}
}
