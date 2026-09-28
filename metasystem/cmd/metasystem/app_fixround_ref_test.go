package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/applaunch"
)

// A goal's candidate is origin's tip: a local goal branch that lags runs
// nothing old, and a moved origin tip replaces the run.
func TestAppGoalRunsOriginsTipBeforeALaggingLocalBranch(t *testing.T) {
	address := appFreePort(t)
	bed := newAppBed(t, appHTTPContract(appFixtureApp(t), address))
	bed.withEvidenceRoot()
	bed.git("branch", "goal/g1")
	originCommit := func(name string) string {
		t.Helper()
		if err := os.WriteFile(filepath.Join(bed.root, name), []byte(name+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		bed.git("checkout", "--quiet", "-B", "origin-side")
		bed.git("add", "-A")
		bed.git("commit", "--quiet", "-m", "origin moves "+name)
		out, err := gitIn(bed.root, "rev-parse", "HEAD")
		if err != nil {
			t.Fatal(err)
		}
		bed.git("update-ref", "refs/remotes/origin/goal/g1", strings.TrimSpace(out))
		bed.git("checkout", "--quiet", "main")
		return strings.TrimSpace(out)
	}
	first := originCommit("first.txt")
	t.Cleanup(func() { bed.run("app", "stop", "--goal", "g1", "--clean") })
	if code, out := bed.run("app", "start", "--goal", "g1"); code != 0 {
		t.Fatalf("app start --goal g1: %d\n%s", code, out)
	}
	key := applaunch.KeyFor("goal/g1")
	record, err := applaunch.ReadRecord(bed.installation, key)
	if err != nil {
		t.Fatal(err)
	}
	if record.Commit != first {
		t.Fatalf("the goal's candidate is origin's tip %s, not the lagging local branch: ran %s", first, record.Commit)
	}
	if _, out := bed.run("app", "status", "--at", "goal/g1"); !strings.Contains(out, "origin/goal/g1") {
		t.Fatalf("status says which ref the commit was resolved from:\n%s", out)
	}
	second := originCommit("second.txt")
	if code, out := bed.run("app", "start", "--at", "goal/g1"); code != 0 || strings.Contains(out, "already running") {
		t.Fatalf("a moved origin tip replaces the run: %d\n%s", code, out)
	}
	if record, err = applaunch.ReadRecord(bed.installation, key); err != nil || record.Commit != second {
		t.Fatalf("the replaced run is at origin's new tip %s: %+v %v", second, record, err)
	}
}
