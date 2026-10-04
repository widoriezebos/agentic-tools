package main

import (
	"encoding/json"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot/stateroottest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

// TestCriticStoreGitIntegration: a critic's records stay in the installation
// that dispatched it, a goal worktree or the primary checkout that serves it,
// and the review verbs find them from either side (branch.CriticStore).
// accept-risk and close run at the primary read a chain the goal worktree
// recorded; revise run from the goal worktree reads a return the primary
// recorded. Real Git: the store is found through the repository's worktrees.
func TestCriticStoreGitIntegration(t *testing.T) {
	t.Parallel()
	top := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		command := exec.Command("git", append([]string{"-C", top, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid"}, args...)...)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
	}
	git("init", "-q")
	git("commit", "--allow-empty", "-q", "-m", "base")
	worktree := filepath.Join(t.TempDir(), "goal-worktree")
	git("worktree", "add", "-q", "-b", "goal/g", worktree)
	resolve := func(path string) string {
		t.Helper()
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			t.Fatal(err)
		}
		return resolved
	}
	top, worktree = resolve(top), resolve(worktree)
	primary, goalInstall := filepath.Join(top, "metasystem"), filepath.Join(worktree, "metasystem")
	for _, install := range []string{primary, goalInstall} {
		if err := os.MkdirAll(install, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(install, "metasystem.conf"), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write := func(path string, data []byte) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	job := func(install string, record map[string]any) {
		t.Helper()
		data, err := json.Marshal(record)
		if err != nil {
			t.Fatal(err)
		}
		write(filepath.Join(install, "artifacts", "agents", "jobs", record["jobId"].(string)+".json"), data)
	}

	// accept-risk at the primary: the review's chain is the goal worktree's.
	job(goalInstall, map[string]any{"jobId": "crit1", "role": "code-critic", "status": "completed"})
	job(goalInstall, map[string]any{"jobId": "crit1-r2", "role": "code-critic", "status": "completed", "parentJob": "crit1"})
	atPrimary := &intentInvocation{stateRoot: primary, layout: stateroot.Layout{GitRoot: top, InstallationRoot: stateroottest.Installation(t, primary)}, cwd: primary,
		input: intentInput{values: map[string][]string{}}}
	if root, problem := atPrimary.reviewRoot("g", "crit1-r2"); problem != nil || root != "crit1" {
		t.Fatalf("accept-risk at the primary did not read the goal worktree's review: root %q, %+v", root, problem)
	}

	// Close at the primary: the chain the goal worktree recorded is read there.
	job(goalInstall, map[string]any{"jobId": "crit2", "role": "builder", "chainClosed": true})
	if closed := atPrimary.closeChain("crit2"); closed.Outcome != intentUnchanged || !strings.Contains(closed.Summary, "already closed") {
		t.Fatalf("close at the primary did not read the goal worktree's chain: %+v", closed)
	}

	// revise from the goal worktree: the examination ran at the primary.
	job(primary, map[string]any{"jobId": "crit3", "role": "code-critic", "status": "completed"})
	returnPath := filepath.Join(primary, "artifacts", "agents", "crit3", "rounds", "1", "return.json")
	write(returnPath, []byte(`{"jobId":"crit3","round":1,"verdict":"clean","findings":[]}`+"\n"))
	digest, _, err := reviewReturnDigest(returnPath)
	if err != nil {
		t.Fatal(err)
	}
	subject := strings.Repeat("d", 40)
	decided := filepath.Join(goalInstall, "decided.md")
	write(decided, []byte(reviewBinding{Goal: "g", Work: "main", Attempt: 1, Subject: subject, Examination: "crit3", Round: 1, Return: digest}.line()+"\n\n"+
		deliveryDispositionsHeader))
	fromWorktree := &intentInvocation{stateRoot: goalInstall, layout: stateroot.Layout{GitRoot: worktree, InstallationRoot: stateroottest.Installation(t, goalInstall)}, cwd: goalInstall,
		input: intentInput{values: map[string][]string{"dispositions": {decided}}}}
	work := launch.NamedWork{Unit: "main", Record: &launch.UnitRunRecord{Worktree: worktree,
		Subjects: []launch.UnitSubject{{Round: 1, Commit: subject, Examination: "crit3", ExaminationRound: 1}}}}
	if document, problem := fromWorktree.reviseDecisions("g", work, 0); problem != nil || !strings.Contains(string(document), "examination crit3 round 1") {
		t.Fatalf("revise from the goal worktree did not read the primary's return: %+v", problem)
	}
}
