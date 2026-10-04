package conflict

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestMain(m *testing.M) { os.Exit(testenv.Main(m)) }

func TestLinesClassifiesOriginalLineChanges(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, main, goal, class, resolution string }{
		{"overlap", "@@ -2,2 +2 @@\n-old\n-old\n+main", "@@ -3 +3 @@\n-old\n+goal", Judgement, ""},
		{"deletion overlap", "@@ -2,2 +1,0 @@\n-old\n-old", "@@ -3 +3 @@\n-old\n+goal", Judgement, ""},
		{"different original lines", "@@ -2 +2 @@\n-old\n+main", "@@ -3 +3 @@\n-old\n+goal", Builder, ""},
		{"same insertion", "@@ -2,0 +3 @@\n+main", "@@ -2,0 +3 @@\n+goal", Builder, "keep both, main's lines then the goal's"},
		{"different insertions", "@@ -2,0 +3 @@\n+main", "@@ -4,0 +5 @@\n+goal", Builder, ""},
		{"insert beside edit", "@@ -2,0 +3 @@\n+main", "@@ -2 +2 @@\n-old\n+goal", Builder, ""},
		{"later overlap", "@@ -1 +1 @@\n-a\n+b\n@@ -9 +9 @@\n-x\n+y", "@@ -9 +9 @@\n-x\n+z", Judgement, ""},
		{"binary", "Binary files a/blob and b/blob differ", "", Judgement, ""},
		{"binary patch", "", "GIT binary patch", Judgement, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			class, resolution, err := Lines(test.main, test.goal)
			if err != nil || class != test.class || resolution != test.resolution {
				t.Fatalf("class=%q resolution=%q err=%v; want %q %q", class, resolution, err, test.class, test.resolution)
			}
		})
	}
	if _, _, err := Lines("@@ broken @@", ""); err == nil {
		t.Fatal("unreadable original lines accepted")
	}
}

func TestClassifyUsesIndexStagesAndDeclaredPatterns(t *testing.T) {
	t.Parallel()
	sets := []testpolicy.Generated{{Paths: []string{"target/openapi/**"}}}
	for _, test := range []struct{ name, stages, class, resolution string }{
		{"metasystem/target/openapi/client.java", "", Generated, ""},
		{"add", "2 3", Judgement, ""},
		{"delete", "1 3", Judgement, ""},
		{"insert", "1 2 3", Builder, "keep both, main's lines then the goal's"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			git := func(args ...string) (string, error) {
				switch args[0] {
				case "ls-files":
					var index strings.Builder
					for _, stage := range strings.Fields(test.stages) {
						fmt.Fprintf(&index, "100644 blob%s %s\t%s\x00", stage, stage, test.name)
					}
					return index.String(), nil
				case "diff":
					return "@@ -2,0 +3 @@\n+insert\n", nil
				default:
					t.Fatalf("unexpected Git call %v", args)
					return "", nil
				}
			}
			got, err := Classify(git, []string{test.name}, func(name string) bool { return GeneratedBy(sets, "metasystem/", name) })
			want := []Path{{Path: test.name, Class: test.class, Resolution: test.resolution}}
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("paths=%v err=%v; want %v", got, err, want)
			}
		})
	}
	if GeneratedBy(sets, "metasystem/", "target/openapi/client.java") || GeneratedBy(nil, "", "target/openapi/client.java") {
		t.Fatal("an undeclared path was generated")
	}
}

func TestClassifyPropagatesUnreadableStagesAndDiffs(t *testing.T) {
	t.Parallel()
	for _, failure := range []string{"ls-files", "diff", "malformed"} {
		t.Run(failure, func(t *testing.T) {
			t.Parallel()
			git := func(args ...string) (string, error) {
				if args[0] == failure {
					return "", errors.New("read failed")
				}
				if failure == "malformed" {
					return "invalid\x00", nil
				}
				return "100644 base 1\tsrc\x00100644 main 2\tsrc\x00100644 goal 3\tsrc\x00", nil
			}
			if _, err := Classify(git, []string{"src"}, func(string) bool { return false }); err == nil {
				t.Fatal("unreadable index or diff accepted")
			}
		})
	}
}

// Git's actual index stages and zero-context blob diffs determine the class;
// a stub cannot prove how a real three-way merge supplies those two inputs.
func TestGitAdapterClassifiesRealThreeWayMerge(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	git := func(args ...string) (string, error) {
		command := exec.Command("/usr/bin/git", append([]string{"-C", root, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "-c", "commit.gpgsign=false"}, args...)...)
		out, err := command.CombinedOutput()
		return strings.TrimSuffix(string(out), "\n"), err
	}
	mustGit := func(args ...string) {
		t.Helper()
		if out, err := git(args...); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	write := func(name, data string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, name), []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mustGit("init", "--quiet", "-b", "main")
	for _, name := range []string{"insert", "overlap", "delete"} {
		write(name, "one\ntwo\nthree\n")
	}
	write("binary", "\x00base")
	mustGit("add", ".")
	mustGit("commit", "--quiet", "-m", "base")
	mustGit("checkout", "--quiet", "-b", "goal")
	write("insert", "one\ngoal\ntwo\nthree\n")
	write("overlap", "one\ngoal\nthree\n")
	write("delete", "one\ngoal\nthree\n")
	write("add", "goal\n")
	write("binary", "\x00goal")
	mustGit("add", ".")
	mustGit("commit", "--quiet", "-m", "goal")
	mustGit("checkout", "--quiet", "main")
	write("insert", "one\nmain\ntwo\nthree\n")
	write("overlap", "one\nmain\nthree\n")
	if err := os.Remove(filepath.Join(root, "delete")); err != nil {
		t.Fatal(err)
	}
	write("add", "main\n")
	write("binary", "\x00main")
	mustGit("add", "-A")
	mustGit("commit", "--quiet", "-m", "main")
	if out, err := git("merge", "--no-commit", "goal"); err == nil {
		t.Fatalf("expected a conflicted merge: %s", out)
	}
	listed, err := git("diff", "--name-only", "--diff-filter=U", "-z")
	if err != nil {
		t.Fatal(err)
	}
	paths := strings.FieldsFunc(listed, func(r rune) bool { return r == 0 })
	if len(paths) != 5 {
		t.Fatalf("unmerged paths=%v; want five", paths)
	}
	got, err := Classify(git, paths, func(string) bool { return false })
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range got {
		class, resolution := Judgement, ""
		if item.Path == "insert" {
			class, resolution = Builder, "keep both, main's lines then the goal's"
		}
		if item.Class != class || item.Resolution != resolution {
			t.Fatalf("classification=%+v; want %s %q", item, class, resolution)
		}
	}
}
