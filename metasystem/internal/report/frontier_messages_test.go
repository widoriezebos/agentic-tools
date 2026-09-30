package report

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// EM-33: a missing or non-numeric --score says which, with an example of
// the command, instead of "record requires a numeric --score".
func TestFrontierScoreRefusalsSayWhatIsWrong(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	file := filepath.Join(repo, "plans", "frontier")
	noGit := frontierGitScript(t, nil)
	for _, test := range []struct {
		name string
		run  func() *FrontierError
		want string
	}{
		{"record without a score", func() *FrontierError {
			_, ferr := frontierRecordWithGit(FrontierOptions{File: file, Repo: repo, Env: noEnv, Eval: "e"}, noGit)
			return ferr
		}, "experiment record needs --score, the evaluation's number, e.g. metasystem experiment record --score 0.82 --eval COMMAND --artifact PATH; nothing was recorded"},
		{"record with a word", func() *FrontierError {
			_, ferr := frontierRecordWithGit(FrontierOptions{File: file, Repo: repo, Env: noEnv, Score: "abc", Eval: "e"}, noGit)
			return ferr
		}, "--score abc is not a number, e.g. --score 0.82; nothing was recorded"},
		{"challenge without a score", func() *FrontierError {
			_, ferr := FrontierChallenge(FrontierOptions{File: file, Env: noEnv})
			return ferr
		}, "experiment challenge needs --score, the candidate's number, e.g. metasystem experiment challenge --score 0.85 --eval COMMAND --artifact PATH; nothing was compared"},
		{"challenge with a word", func() *FrontierError {
			_, ferr := FrontierChallenge(FrontierOptions{File: file, Env: noEnv, Score: "abc"})
			return ferr
		}, "--score abc is not a number, e.g. --score 0.85; nothing was compared"},
	} {
		ferr := test.run()
		if ferr == nil || ferr.Code != 2 || ferr.Message != test.want {
			t.Errorf("%s: got %+v, want code 2 and %q", test.name, ferr, test.want)
		}
	}
}

// EM-20: experiment status outside any repository read "no frontier
// recorded at plans/frontier" and exited 0, as if it had looked in a
// project. Outside a repository it refuses; inside one an absent frontier
// is still reported, and a present one is read without asking Git.
func TestFrontierStatusOutsideARepositoryRefuses(t *testing.T) {
	t.Parallel()
	outside := t.TempDir()
	file := filepath.Join(outside, "plans", "frontier")
	outsideGit := frontierGitScript(t, []frontierGitStep{{outside, []string{"rev-parse", "--is-inside-work-tree"}, "", errors.New("outside repository")}})
	lines, ferr := frontierStatusWithGit(FrontierOptions{File: file, Repo: outside}, outsideGit)
	want := outside + " is not inside a Git repository, so there is no frontier to read\nrun: cd <the repository> && metasystem experiment status"
	if ferr == nil || ferr.Code != 2 || ferr.Message != want || len(lines) != 0 {
		t.Fatalf("status outside a repository = %v %+v, want code 2 and %q", lines, ferr, want)
	}

	repo := t.TempDir()
	absent := filepath.Join(repo, "plans", "frontier")
	insideGit := frontierGitScript(t, []frontierGitStep{{repo, []string{"rev-parse", "--is-inside-work-tree"}, "true", nil}})
	lines, ferr = frontierStatusWithGit(FrontierOptions{File: absent, Repo: repo}, insideGit)
	if ferr != nil || len(lines) != 1 || lines[0] != "no frontier recorded at "+absent {
		t.Fatalf("absent frontier in a repository = %v %+v", lines, ferr)
	}

	present := filepath.Join(t.TempDir(), "frontier")
	if err := os.WriteFile(present, []byte("score=80\ndirection=min\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	lines, ferr = frontierStatusWithGit(FrontierOptions{File: present, Repo: repo}, frontierGitScript(t, nil))
	if ferr != nil || len(lines) != 1 || lines[0] != "score=80\ndirection=min" {
		t.Fatalf("present frontier = %v %+v", lines, ferr)
	}
}

// SOL-EM-02: a frontier path that exists but cannot be read as a file (a
// directory, say) is not an absent frontier; it refuses and names the path.
func TestFrontierStatusUnreadableIsNotAbsent(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	insideGit := frontierGitScript(t, []frontierGitStep{{repo, []string{"rev-parse", "--is-inside-work-tree"}, "true", nil}})
	lines, ferr := frontierStatusWithGit(FrontierOptions{File: repo, Repo: repo}, insideGit)
	if ferr == nil || ferr.Code != 1 || !strings.Contains(ferr.Message, "cannot read the frontier at "+repo) || len(lines) != 0 {
		t.Fatalf("a directory as the frontier = %v %+v, want code 1 naming it", lines, ferr)
	}
}
