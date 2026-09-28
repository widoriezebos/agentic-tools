package report

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Recording the frontier that already stands is success and writes nothing
// (R-129-ui), even while the uncommitted frontier file dirties the worktree;
// the same score at another commit is not a repeat and keeps its guard.
func TestFrontierRecordRepeatIsUnchanged(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	file := filepath.Join(repo, "plans", "frontier")
	sha := strings.Repeat("e", 40)
	now := time.Unix(1_000_000, 0)
	clock := func() time.Time { return now }
	steps := frontierCleanReads(repo, sha)
	steps = append(steps,
		frontierGitStep{repo, []string{"rev-parse", "--is-inside-work-tree"}, "true", nil},
		frontierGitStep{repo, []string{"rev-parse", "HEAD"}, sha, nil},
		// After the frontier commit HEAD moved: the same score is not a
		// repeat, and the ordinary guard refuses it.
		frontierGitStep{repo, []string{"rev-parse", "--is-inside-work-tree"}, "true", nil},
		frontierGitStep{repo, []string{"rev-parse", "HEAD"}, strings.Repeat("f", 40), nil},
		frontierGitStep{repo, []string{"status", "--porcelain"}, "", nil},
	)
	gitRead := frontierGitScript(t, steps)
	opts := FrontierOptions{File: file, Repo: repo, Env: noEnv, Now: clock, Score: "80", MinDelta: "1", MaxAge: "60", Eval: "declared eval", Artifact: "runs/1.json"}
	if _, ferr := frontierRecordWithGit(opts, gitRead); ferr != nil {
		t.Fatalf("record: %v", ferr)
	}
	recorded, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(10 * time.Minute)
	lines, ferr := frontierRecordWithGit(opts, gitRead)
	if ferr != nil || len(lines) != 1 || !strings.Contains(lines[0], "already records score 80") {
		t.Fatalf("repeat record = %v %v", lines, ferr)
	}
	if again, err := os.ReadFile(file); err != nil || string(again) != string(recorded) {
		t.Fatalf("a repeated record rewrote the frontier: %q %v", again, err)
	}
	if _, ferr := frontierRecordWithGit(opts, gitRead); ferr == nil || ferr.Code != 1 || !strings.Contains(ferr.Message, "does not beat") {
		t.Fatalf("the same score at another commit = %v, want the regression guard", ferr)
	}
}
