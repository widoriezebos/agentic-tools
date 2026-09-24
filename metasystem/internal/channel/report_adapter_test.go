package channel

import (
	"bytes"
	"os/exec"
	"testing"
	"time"
)

func TestLandingHistoryGitAdapterHonorsWindowsAndGoalItemFormat(t *testing.T) {
	root := t.TempDir()
	runGit := func(env []string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(testGitEnv(), env...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	runGit(nil, "init", "-q", "-b", "main")
	commit := func(at, subject, id string) {
		t.Helper()
		runGit([]string{
			"GIT_AUTHOR_NAME=report-adapter", "GIT_AUTHOR_EMAIL=report@example.invalid",
			"GIT_COMMITTER_NAME=report-adapter", "GIT_COMMITTER_EMAIL=report@example.invalid",
			"GIT_AUTHOR_DATE=" + at, "GIT_COMMITTER_DATE=" + at,
		}, "commit", "-q", "--allow-empty", "-m", subject, "-m", "Goal-Item: "+id)
	}
	commit("2026-09-04T10:00:00Z", "Earlier subject", "first-goal")
	commit("2026-09-04T11:00:00Z", "Later subject", "second-goal")
	runGit(nil, "update-ref", "refs/remotes/origin/main", "HEAD")

	for _, test := range []struct {
		start time.Time
		want  string
	}{
		{time.Date(2026, 9, 4, 9, 0, 0, 0, time.UTC), "Later subject\x00second-goal\n\nEarlier subject\x00first-goal\n\n"},
		{time.Date(2026, 9, 4, 10, 30, 0, 0, time.UTC), "Later subject\x00second-goal\n\n"},
	} {
		got, err := defaultLandingLogBytes(root, test.start)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, []byte(test.want)) {
			t.Fatalf("landing bytes for %s:\n got %q\nwant %q", test.start.Format(time.RFC3339), got, test.want)
		}
	}
}
