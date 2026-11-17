package branch

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestReplayConfigurationEquivalence(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, config string
		want         bool
	}{
		{"shared", "local\x00merge.demo.driver\nshared\x00", true},
		{"same override", "local\x00merge.demo.driver\nshared\x00worktree\x00merge.demo.driver\nshared\x00", true},
		{"different override", "local\x00merge.demo.driver\nshared\x00worktree\x00merge.demo.driver\nother\x00", false},
		{"command override", "local\x00merge.demo.driver\nshared\x00worktree\x00merge.demo.driver\nother\x00command\x00merge.demo.driver\nengine\x00", true},
		{"conditional include", "local\x00includeif.onbranch:goal/*.path\nbranch-config\x00", false},
		{"relative attributes", "local\x00core.attributesfile\nattributes\x00", false},
		{"unknown scope", "unknown\x00merge.demo.driver\nshared\x00", false},
		{"malformed", "local\x00", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := replayConfigEquivalent([]byte(tc.config)); got != tc.want {
				t.Fatalf("equivalent = %t", got)
			}
		})
	}
}

func TestReplayAcceptsOnlyTheCapturedLinearSequence(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"accepted", "unavailable", "wrong ref", "wrong old", "extra update", "dropped", "parent", "author", "message", "signed", "merge", "range refusal"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			base, old, next := strings.Repeat("a", 40), strings.Repeat("b", 40), strings.Repeat("c", 40)
			req := RebaseRequest{Repo: "repo", GoalID: "goal-a", EndpointTip: base}
			opens := 0
			d := rebaseDependencies{repository: commitRepository{
				facts: commitFacts{Range: func(string, string, string, string) ([]Commit, error) {
					if mode == "range refusal" {
						return nil, errors.New(mode)
					}
					return nil, nil
				}},
				effects: commitEffects{Open: func(string, string, bool) (string, func(), error) {
					opens++
					return "", nil, errors.New("scratch selected")
				}},
			}}
			d.git = func(_ string, args ...string) ([]byte, error) {
				switch args[0] {
				case "config":
					return []byte("command\x00merge.test.driver\nengine\x00"), nil
				case "rev-list":
					if strings.HasSuffix(args[len(args)-1], old) {
						return []byte(old), nil
					}
					if mode == "dropped" {
						return nil, nil
					}
					return []byte(next), nil
				case "cat-file":
					parent, author, message, extra := base, "author A <a@example.invalid> 123 +0130", "unit\n\nGoal-Unit: goal-a/u1\n", ""
					if args[2] == old {
						if mode == "signed" {
							extra = "gpgsig fake\n signature\n"
						}
						if mode == "merge" {
							extra = "parent " + next + "\n"
						}
					} else {
						if mode == "parent" {
							parent = old
						}
						if mode == "author" {
							author += " altered"
						}
						if mode == "message" {
							message += "altered"
						}
					}
					return []byte("tree " + next + "\nparent " + parent + "\n" + author + "\n" + extra + "\n" + message), nil
				case "rev-parse":
					return []byte(base), nil
				case "log", "diff", "ls-files":
					return nil, nil
				case "-c":
					if !reflect.DeepEqual(args, []string{"-c", "replay.refAction=print", "replay", "--onto", base, base + "..refs/heads/goal/goal-a"}) {
						t.Fatalf("replay argv %q", args)
					}
					if mode == "unavailable" {
						return nil, errors.New(mode)
					}
					ref, prior := goalBranchRef(req.GoalID), old
					if mode == "wrong ref" {
						ref = "refs/heads/main"
					}
					if mode == "wrong old" {
						prior = base
					}
					out := "update " + ref + " " + next + " " + prior + "\n"
					if mode == "extra update" {
						out += out
					}
					return []byte(out), nil
				}
				t.Fatalf("unexpected git %q", args)
				return nil, nil
			}
			got, _, err := replayRebase(req, old, d)
			if mode == "accepted" {
				if err != nil || got != next || opens != 0 {
					t.Fatalf("result %s %v, opens %d", got, err, opens)
				}
			} else if err == nil || opens != 1 {
				t.Fatalf("unsafe replay accepted: %s %v, opens %d", got, err, opens)
			}
		})
	}
}

// Adapter integration: compare every replayed tree and raw author/message
// against scratch rebase, and observe that object computation leaves refs,
// HEAD, the index and the caller's checkout unchanged.
func TestReplayWithoutCheckoutGitAdapter(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"ordinary", "testing driver", "newly empty", "reverted attributes", "outside attributes", "worktree config", "conditional config", "signed"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			repo := newMergeDriverRepo(t, "metasystem/testing.json")
			git := func(args ...string) string { return mergeDriverGit(t, repo, args...) }
			commit := func(message string) string {
				git("add", "-A")
				git("commit", "-qm", message)
				return git("rev-parse", "HEAD")
			}
			base := git("rev-parse", "HEAD")
			git("checkout", "-qb", "goal/goal-a")
			firstPath, firstMessage := "metasystem/one.txt", "first\n\nGoal-Unit: goal-a/u1"
			if mode == "newly empty" {
				firstPath, firstMessage = "metasystem/plans/first.md", "plan\n\nGoal-Plan: goal-a"
			}
			mergeDriverWrite(t, repo, firstPath, "one\n")
			first := commit(firstMessage)
			if mode == "reverted attributes" {
				mergeDriverWrite(t, repo, ".gitattributes", "*.txt merge=union\n")
				commit("attributes\n\nGoal-Unit: goal-a/u2")
				mergeDriverWrite(t, repo, ".gitattributes", "metasystem/testing.json merge=metasystem-testing\n")
			}
			if mode == "testing driver" {
				writeMergeDriverContract(t, repo, "metasystem/testing.json", mergeDriverContractWithAddition(mergeDriverContractFixture(), "goal", 1200))
			}
			mergeDriverWrite(t, repo, "metasystem/two.txt", "two\n")
			old := commit("second\n\nGoal-Unit: goal-a/u3")
			if mode == "signed" {
				raw := git("cat-file", "commit", old)
				raw = strings.Replace(raw, "\n\n", "\ngpgsig fake\n signature\n\n", 1) + "\n"
				command := exec.Command("git", "-C", repo, "hash-object", "-t", "commit", "-w", "--stdin")
				command.Stdin = strings.NewReader(raw)
				out, err := command.Output()
				if err != nil {
					t.Fatal(err)
				}
				old = strings.TrimSpace(string(out))
				git("reset", "--hard", old)
			}
			git("checkout", "-q", "main")
			if mode == "testing driver" {
				writeMergeDriverContract(t, repo, "metasystem/testing.json", mergeDriverContractWithAddition(mergeDriverContractFixture(), "endpoint", 1300))
			}
			if mode == "newly empty" {
				git("cherry-pick", "--no-commit", first)
			}
			if mode == "outside attributes" {
				mergeDriverWrite(t, repo, "outside/.gitattributes", "*.txt merge=union\n")
			}
			mergeDriverWrite(t, repo, "metasystem/main.txt", "main\n")
			endpoint := commit("main advanced")
			git("checkout", "-q", "goal/goal-a")
			if mode == "worktree config" {
				git("config", "extensions.worktreeConfig", "true")
				git("config", "--worktree", "merge.demo.driver", "false")
			}
			if mode == "conditional config" {
				include := filepath.Join(t.TempDir(), "config")
				if err := os.WriteFile(include, []byte("[merge \"demo\"]\n driver = false\n"), 0600); err != nil {
					t.Fatal(err)
				}
				git("config", "includeIf.onbranch:goal/*.path", include)
			}
			req := RebaseRequest{Repo: filepath.Join(repo, "metasystem"), GoalID: "goal-a", EndpointTip: endpoint}
			snapshot := func() []string {
				return []string{git("show-ref"), git("rev-parse", "HEAD"), git("write-tree"), git("status", "--porcelain"), git("diff"), git("worktree", "list", "--porcelain")}
			}
			before := snapshot()
			d := gitRebaseDependencies()
			opens := 0
			open := d.repository.effects.Open
			d.repository.effects.Open = func(repo, tip string, amend bool) (string, func(), error) { opens++; return open(repo, tip, amend) }
			next, _, err := replayRebase(req, old, d)
			if err != nil {
				t.Fatal(err)
			}
			wantOpens := 1
			if mode == "ordinary" || mode == "testing driver" {
				wantOpens = 0
			}
			if opens != wantOpens {
				t.Fatalf("scratch adds %d, want %d", opens, wantOpens)
			}
			if after := snapshot(); !reflect.DeepEqual(before, after) {
				t.Fatalf("computation mutated caller: before %q after %q", before, after)
			}
			// Force the existing scratch path over the same captured input.
			originalGit := d.git
			d.git = func(repo string, args ...string) ([]byte, error) {
				if args[0] == "config" {
					return nil, errors.New("force scratch comparison")
				}
				return originalGit(repo, args...)
			}
			scratch, _, err := replayRebase(req, old, d)
			if err != nil {
				t.Fatal(err)
			}
			a, ok := replayCommits(req.Repo, endpoint, next, originalGit)
			if !ok {
				t.Fatal("invalid replay")
			}
			b, ok := replayCommits(req.Repo, endpoint, scratch, originalGit)
			if !ok || len(a) != len(b) {
				t.Fatal("sequence length differs")
			}
			parentA, parentB := endpoint, endpoint
			for i := range a {
				if a[i].parent != parentA || b[i].parent != parentB || a[i].author != b[i].author || !bytes.Equal(a[i].message, b[i].message) || git("rev-parse", a[i].id+"^{tree}") != git("rev-parse", b[i].id+"^{tree}") {
					t.Fatalf("commit %d differs", i)
				}
				parentA, parentB = a[i].id, b[i].id
			}
			if mode == "testing driver" && git("show", next+":metasystem/testing.json") != git("show", scratch+":metasystem/testing.json") {
				t.Fatal("testing driver output differs")
			}
			t.Logf("base %s: %s used %d scratch adds; %d commits match scratch", base, mode, wantOpens, len(a))
		})
	}
}
