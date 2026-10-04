package branch

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestRebaseGitAdapter observes replay, checkout installation, atomic publication and abort cleanup.
func TestRebaseGitAdapter(t *testing.T) {
	t.Parallel()
	for _, conflicts := range []bool{false, true} {
		t.Run(map[bool]string{false: "replay", true: "conflict"}[conflicts], func(t *testing.T) {
			dir := t.TempDir()
			remote := filepath.Join(dir, "origin.git")
			primary := filepath.Join(dir, "primary")
			work := filepath.Join(dir, "goal")
			git := func(repo string, args ...string) string {
				t.Helper()
				out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput()
				if err != nil {
					t.Fatalf("git %v: %v: %s", args, err, out)
				}
				return strings.TrimSpace(string(out))
			}
			write := func(repo, path, body string) {
				t.Helper()
				path = filepath.Join(repo, path)
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(body), 0644); err != nil {
					t.Fatal(err)
				}
			}
			commit := func(repo, path, body, message string) string {
				write(repo, path, body)
				git(repo, "add", "-A")
				git(repo, "commit", "-qm", message)
				return git(repo, "rev-parse", "HEAD")
			}
			git(dir, "init", "-q", "--bare", remote)
			git(remote, "symbolic-ref", "HEAD", "refs/heads/main")
			git(dir, "clone", "-q", remote, primary)
			git(primary, "config", "user.name", "fixture")
			git(primary, "config", "user.email", "fixture@example.invalid")
			commit(primary, "metasystem/code.txt", "base\n", "base")
			git(primary, "push", "-qu", "origin", "main")
			git(primary, "worktree", "add", "-qb", "goal/goal-a", work)
			plan := commit(work, "metasystem/plans/design.md", "a plan\n", "plan\n\nGoal-Plan: goal-a")
			unit := commit(work, "metasystem/code.txt", "unit\n", "unit\n\nGoal-Unit: goal-a/u1")
			record := "metasystem/records/reads/goal-a/" + unit + ".json"
			old := commit(work, record, `{"testsChanged":[]}`+"\n", "read\n\nGoal-Read: goal-a/u1 "+unit)
			git(work, "push", "-q", "origin", "goal/goal-a")
			path, body := "metasystem/other.txt", "main\n"
			if conflicts {
				path, body = "metasystem/code.txt", "main\n"
			}
			main := commit(primary, path, body, "main moved")
			git(primary, "push", "-q", "origin", "main")
			write(work, "scratch.txt", "untracked\n")
			before := []string{git(work, "rev-parse", "HEAD"), git(work, "status", "--porcelain"), git(work, "diff"), git(work, "worktree", "list", "--porcelain"), git(work, "for-each-ref", "--format=%(refname) %(objectname)", "refs/metasystem/goals/before/"), git(remote, "show-ref")}
			d := gitRebaseDependencies()
			d.gate = func(ReadGateRequest) (GateObservation, error) {
				return GateObservation{RunID: "adapter-gate", Tree: main}, nil
			}
			d.commitRead = func(req CommitReadRequest) (string, Attestation, error) {
				if req.Carry != unit {
					t.Fatalf("carry = %s, want %s", req.Carry, unit)
				}
				data, err := os.ReadFile(filepath.Join(req.Repo, "records", "reads", "goal-a", unit+".json"))
				if err != nil || string(data) != `{"testsChanged":[]}`+"\n" {
					t.Fatalf("old review was not replayed: %s %v", data, err)
				}
				return git(req.Repo, "rev-parse", "HEAD"), Attestation{}, nil
			}
			req := RebaseRequest{Repo: filepath.Join(work, "metasystem"), Remote: "origin", GoalID: "goal-a", EndpointTip: main, CheckClaim: func() error { return nil }}
			got, err := rebaseWith(req, d)
			if conflicts {
				var refusal *OpError
				if !errors.As(err, &refusal) || refusal.Code != RebaseConflictCode || !strings.Contains(err.Error(), "metasystem/code.txt") {
					t.Fatalf("conflict %v", err)
				}
				after := []string{git(work, "rev-parse", "HEAD"), git(work, "status", "--porcelain"), git(work, "diff"), git(work, "worktree", "list", "--porcelain"), git(work, "for-each-ref", "--format=%(refname) %(objectname)", "refs/metasystem/goals/before/"), git(remote, "show-ref")}
				if !reflect.DeepEqual(before, after) {
					t.Fatalf("conflict changed state\nbefore %q\nafter %q", before, after)
				}
				data, err := os.ReadFile(filepath.Join(work, "metasystem/code.txt"))
				if err != nil || string(data) != "unit\n" {
					t.Fatalf("checkout %s %v", data, err)
				}
				return
			}
			if err != nil || got.State != "rebased" || got.OldTip != old {
				t.Fatalf("rebase %+v %v", got, err)
			}
			commits, err := ValidateRange(req.Repo, main, got.NewTip, "goal-a")
			if err != nil || len(commits) != 3 {
				t.Fatalf("replayed commits %+v %v", commits, err)
			}
			if commits[0].Kind != Plan || commits[1].Kind != Unit || commits[2].Kind != Read || commits[0].ID == plan || commits[1].ID == unit || commits[2].ID == old {
				t.Fatalf("replay %+v", commits)
			}
			read, err := KindOf(req.Repo, commits[2].ID, "goal-a")
			if err != nil || read.CommitID != unit {
				t.Fatalf("read trailer %+v %v", read, err)
			}
			kept := "refs/metasystem/goals/before/goal-a/" + old
			for _, repo := range []string{work, remote} {
				if git(repo, "rev-parse", policyGoalRef) != got.NewTip || git(repo, "rev-parse", kept) != old {
					t.Fatal("branch or kept tip was not published")
				}
			}
			if git(work, "rev-parse", commits[0].ID+"^") != main || git(work, "rev-parse", "HEAD") != got.NewTip {
				t.Fatal("main or checkout tip differs")
			}
			if data, err := os.ReadFile(filepath.Join(work, "metasystem/other.txt")); err != nil || string(data) != "main\n" {
				t.Fatalf("worktree files %s %v", data, err)
			}
			if git(work, "worktree", "list", "--porcelain") != before[3] && strings.Count(git(work, "worktree", "list", "--porcelain"), "worktree ") != 2 {
				t.Fatal("scratch worktree left registered")
			}
		})
	}
}
