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
	for _, mode := range []string{"replay", "conflict", "generated", "generated-rename", "resolve"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
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
			write(primary, "metasystem/metasystem.conf", "testing.contract=testing.json\n")
			write(primary, "metasystem/testing.json", rebaseTestContract(t, `[{"paths":["gen/**"],"command":["cp","src.txt","gen/out.txt"]}]`))
			write(primary, "metasystem/src.txt", "base\n")
			write(primary, "metasystem/gen/out.txt", "base\n")
			base := commit(primary, "metasystem/code.txt", "base\n", "base")
			git(primary, "push", "-qu", "origin", "main")
			git(primary, "worktree", "add", "-qb", "goal/goal-a", work)
			plan := commit(work, "metasystem/plans/design.md", "a plan\n", "plan\n\nGoal-Plan: goal-a")
			if strings.HasPrefix(mode, "generated") {
				write(work, "metasystem/gen/out.txt", "goal\n")
				if mode == "generated-rename" {
					git(work, "mv", "metasystem/code.txt", "metasystem/a-much-longer-name.txt")
				}
			}
			unitPath, unitBody := "metasystem/code.txt", "unit\n"
			if mode == "generated-rename" {
				unitPath, unitBody = "metasystem/a-much-longer-name.txt", "base\n"
			}
			if mode == "resolve" {
				unitBody = "goal insertion\nbase\n"
			}
			unit := commit(work, unitPath, unitBody, "unit\n\nGoal-Unit: goal-a/u1")
			record := "metasystem/records/reads/goal-a/" + unit + ".json"
			old := commit(work, record, `{"testsChanged":[]}`+"\n", "read\n\nGoal-Read: goal-a/u1 "+unit)
			git(work, "push", "-q", "origin", "goal/goal-a")
			path, body := "metasystem/other.txt", "main\n"
			if mode == "conflict" {
				path, body = "metasystem/code.txt", "main\n"
			}
			if mode == "resolve" {
				write(primary, "metasystem/code.txt", "main insertion\nbase\n")
			}
			if strings.HasPrefix(mode, "generated") {
				write(primary, "metasystem/gen/out.txt", "main\n")
				write(primary, "metasystem/src.txt", "merged sources\n")
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
			if mode == "resolve" {
				req.Resolve = func(stop RebaseResolution) (string, error) {
					if stop.Base != git(stop.Worktree, "rev-parse", "HEAD") || !strings.Contains(stop.Conflicts, "<<<<<<< main\nmain insertion\n||||||| base\n=======\ngoal insertion\n>>>>>>> goal") {
						t.Fatalf("resolve %+v", stop)
					}
					write(stop.Worktree, "metasystem/code.txt", "main insertion\ngoal insertion\nbase\n")
					return "resolve/run.json", nil
				}
			}
			got, err := rebaseWith(req, d)
			if mode == "conflict" {
				var refusal *RebaseConflict
				if !errors.As(err, &refusal) || refusal.Code != RebaseJudgementCode || !strings.Contains(err.Error(), "metasystem/code.txt") {
					t.Fatalf("conflict %v", err)
				}
				versions := refusal.Paths[0]
				if versions.FirstLine != 1 || versions.LastLine != 1 || versions.Original != git(work, "rev-parse", base+":metasystem/code.txt") || versions.Main != git(work, "rev-parse", main+":metasystem/code.txt") || versions.Goal != git(work, "rev-parse", unit+":metasystem/code.txt") || versions.MainCommit != main {
					t.Fatalf("versions %+v", versions)
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
			if strings.HasPrefix(mode, "generated") {
				if data, err := os.ReadFile(filepath.Join(work, "metasystem/gen/out.txt")); err != nil || string(data) != "merged sources\n" {
					t.Fatalf("generated content %s %v", data, err)
				}
				if !reflect.DeepEqual(got.Carried, []string{"u1"}) || !reflect.DeepEqual(got.Regenerated, []string{"metasystem/gen/out.txt"}) {
					t.Fatalf("review or regeneration %+v", got)
				}
			}
			if mode == "resolve" && (!reflect.DeepEqual(got.NeedsReview, []string{"u1"}) || len(got.Carried) != 0) {
				t.Fatalf("resolved read %+v", got)
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
