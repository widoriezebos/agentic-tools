package branch

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func rebaseAnswerFixture(t *testing.T, repo string) *rebaseFixture {
	t.Helper()
	f := newRebaseFixture(t)
	f.req.Repo = repo
	f.reviewedUnit()
	f.changed, f.replayErr, f.conflict, f.judgement = true, errors.New("conflict"), "metasystem/source\x00", true
	dir := t.TempDir()
	f.d.repository.effects.Open = func(string, string, bool) (string, func(), error) { return dir, func() { f.closed++ }, nil }
	git := f.d.git
	f.d.git = func(repo string, args ...string) ([]byte, error) {
		if strings.Join(args, " ") == "checkout --conflict=diff3 -- metasystem/source" {
			return nil, errors.New("read versions from index stages")
		}
		if len(args) == 2 && args[0] == "show" {
			return []byte(args[1] + "\n"), nil
		}
		return git(repo, args...)
	}
	return f
}

func writeRebaseQuestion(t *testing.T, repo string) string {
	t.Helper()
	path := filepath.Join(repo, "artifacts/agents/goals/goal-a/conflict.json")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"base":"stopped-base","main":"main-tip","paths":[{"path":"metasystem/source","original":"original","main":"main","goal":"goal","class":"judgement","resolution":"","question":"q1","impact":"main drops the goal's change; the goal undoes main; a third is read again"}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRebaseAnswerSurvivesCheckoutChange(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	path := writeRebaseQuestion(t, repo)
	first := rebaseAnswerFixture(t, repo)
	first.req.Answer = func(id string) (string, error) { return map[string]string{"q1": "keep main's"}[id], nil }
	first.req.Resolve = func(RebaseResolution) (string, error) { return "round.json", errors.New("interrupted") }
	if _, err := rebaseWith(first.req, first.d); err == nil {
		t.Fatal("first checkout did not stop at the interrupted round")
	}
	data, err := os.ReadFile(path)
	var saved struct {
		Base, Main string
		Paths      []map[string]string
	}
	if err != nil || json.Unmarshal(data, &saved) != nil || len(saved.Paths) != 1 || saved.Paths[0]["resolution"] != "keep main's" || saved.Paths[0]["impact"] != "main drops the goal's change; the goal undoes main; a third is read again" || saved.Base != "stopped-base" || saved.Main != "main-tip" {
		t.Fatalf("answer and impact were not retained: %s, %v", data, err)
	}
	second := rebaseAnswerFixture(t, repo)
	second.req.Answer = func(string) (string, error) {
		t.Fatal("read another checkout's question store")
		return "", os.ErrNotExist
	}
	second.req.Resolve = func(stop RebaseResolution) (string, error) {
		if !strings.Contains(stop.Conflicts, "Written resolution: keep main's") || stop.Base != policySecond {
			t.Fatalf("resolve round: %+v", stop)
		}
		second.conflict = ""
		return "round.json", nil
	}
	got, err := rebaseWith(second.req, second.d)
	if err != nil || second.continues != 1 || !reflect.DeepEqual(got.Resolved, []string{"metasystem/source"}) {
		t.Fatalf("second checkout: %+v, %v", got, err)
	}
}

func TestRebaseMissingQuestionAsksAgain(t *testing.T) {
	t.Parallel()
	f := rebaseAnswerFixture(t, t.TempDir())
	writeRebaseQuestion(t, f.req.Repo)
	f.req.Answer = func(string) (string, error) {
		return "", &os.PathError{Op: "open", Path: "other-checkout/questions/q1.json", Err: os.ErrNotExist}
	}
	f.req.Resolve = func(RebaseResolution) (string, error) { t.Fatal("resolved without an answer"); return "", nil }
	_, err := rebaseWith(f.req, f.d)
	var question *RebaseConflict
	if !errors.As(err, &question) || len(question.Paths) != 1 || question.Paths[0].Path != "metasystem/source" || !strings.Contains(err.Error(), "run: metasystem work status goal-a") || strings.Contains(err.Error(), "no such file") || !reflect.DeepEqual(f.events, []string{"abort"}) {
		t.Fatalf("missing question did not ask again: %v", err)
	}
}

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
