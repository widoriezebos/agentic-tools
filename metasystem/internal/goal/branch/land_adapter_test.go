package branch_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
)

func TestPrepareLandingGitAdapter(t *testing.T) {
	t.Parallel()
	t.Run("series", func(t *testing.T) {
		f := newLandFixture(t)
		beforeWorktrees := git(t, f.root, "worktree", "list", "--porcelain")
		out := filepath.Join(t.TempDir(), "prepared")
		request := landRequest(t, f, out)
		request.Repo = filepath.Join(f.root, "metasystem")
		result, err := branch.PrepareLanding(request)
		if err != nil {
			t.Fatal(err)
		}
		if result.Endpoint != f.base || result.Attempt != "attempt-deep" || result.LastUnit != "u3" || result.ProofNumber != 1 {
			t.Fatalf("landing result = %+v", result)
		}
		if remote := git(t, f.origin, "rev-parse", "refs/heads/landing/goal-a"); remote != result.Landing {
			t.Fatalf("landing remote = %s; want %s", remote, result.Landing)
		}
		if got := git(t, f.root, "worktree", "list", "--porcelain"); got != beforeWorktrees {
			t.Fatalf("scratch worktree remains:\n%s", got)
		}
		commits := strings.Fields(git(t, f.root, "rev-list", "--reverse", f.base+".."+result.Landing))
		if len(commits) != 3 {
			t.Fatalf("landing parent chain = %v", commits)
		}
		parent := f.base
		for index, commit := range commits {
			if got := git(t, f.root, "rev-parse", commit+"^"); got != parent {
				t.Fatalf("landing commit %d parent = %s; want %s", index, got, parent)
			}
			unit := []string{"u1", "u2", "u3"}[index]
			patch, err := os.ReadFile(filepath.Join(out, unit+".patch"))
			if err != nil {
				t.Fatal(err)
			}
			physical, err := exec.Command("git", "-C", f.root, "diff", "--binary", "--full-index", parent, commit).Output()
			if err != nil || !bytes.Equal(patch, physical) {
				t.Fatalf("landing %s patch differs from committed diff: %v", unit, err)
			}
			parent = commit
			identity := git(t, f.root, "show", "-s", "--format=%an <%ae>|%cn <%ce>|%aI|%cI", commit)
			if identity != "Wido Approver <wido@example.invalid>|Wido Approver <wido@example.invalid>|2026-09-17T10:00:00Z|2026-09-17T10:00:00Z" {
				t.Fatalf("landing commit %d identity and date = %s", index, identity)
			}
			message := git(t, f.root, "show", "-s", "--format=%B", commit)
			artifactMessage, err := os.ReadFile(filepath.Join(out, unit+".message"))
			if err != nil || strings.TrimSpace(string(artifactMessage)) != message {
				t.Fatalf("landing %s message differs from commit: %v", unit, err)
			}
			for _, trailer := range []string{"Goal-Unit: goal-a/u", "Goal-Digest: ", "Goal-Source: " + f.units[index], "Landed-By: seat-a"} {
				if !strings.Contains(message, trailer) {
					t.Fatalf("landing commit %d lacks %q:\n%s", index, trailer, message)
				}
			}
			if (strings.Count(message, "Goal-Last: goal-a") == 1) != (index == 2) {
				t.Fatalf("landing commit %d last trailer:\n%s", index, message)
			}
			if index == 2 {
				for _, fold := range []string{"Goal-Fold: metasystem/plans/between.md", "Goal-Fold: metasystem/plans/tail.md"} {
					if !strings.Contains(message, fold) {
						t.Fatalf("last landing commit lacks %s:\n%s", fold, message)
					}
				}
			}
		}
		if got := git(t, f.root, "rev-parse", result.Landing+"^{tree}"); got != result.Candidate {
			t.Fatalf("candidate tree = %s; want %s", got, result.Candidate)
		}
		projected, err := landing.ProjectWorkspaceTree(filepath.Join(f.root, "metasystem"), result.Candidate)
		if err != nil || projected != f.projected {
			t.Fatalf("projected tree = %s, error %v; want %s", projected, err, f.projected)
		}
		verified, err := branch.VerifyLandedSeries(f.root, result.Landing)
		if err != nil || len(verified) != 3 {
			t.Fatalf("verified landing series = %+v, error %v", verified, err)
		}
		for index, item := range verified {
			if item.Commit != commits[index] || item.Goal != "goal-a" || item.Units != []string{"u1", "u2", "u3"}[index] || item.Actual != item.Expected {
				t.Fatalf("verified landing %d = %+v", index, item)
			}
		}
		receipts, err := exec.Command("git", "-C", f.root, "show", result.Landing+":metasystem/memory/receipts.log").Output()
		if err != nil {
			t.Fatal(err)
		}
		row := "1789639200|2026-09-17T10:00:00Z|RECEIPT|type=implement|outcome=shipped|skills=none|verify=clean|corrections=0|stop_loss=no|delegate=none|goal=goal-a|built_by=coordinator|last_unit=u3|proof=attempt-deep|critique_waived=none|waiver_stream=none|note=landing proof attempt-deep\n"
		wantReceipts := "1|1970-01-01T00:00:00Z|RECEIPT|type=seed|outcome=shipped\n" + strings.Repeat(row, 3)
		if !bytes.Equal(receipts, []byte(wantReceipts)) {
			t.Fatalf("landing receipt bytes:\ngot  %q\nwant %q", receipts, wantReceipts)
		}
		draft, err := os.ReadFile(filepath.Join(out, "record-draft"))
		if err != nil {
			t.Fatal(err)
		}
		for _, value := range []string{"endpoint=" + f.base, "candidate=" + result.Candidate, "landing=" + result.Landing, "attempt=attempt-deep"} {
			if !strings.Contains(string(draft), value) {
				t.Fatalf("record draft lacks %s: %s", value, draft)
			}
		}
		t.Run("retry_identity_transfer", func(t *testing.T) {
			if result.RetryIdentity == "" {
				t.Fatal("prepared landing has no retry identity")
			}
			git(t, f.root, "push", "-q", "origin", ":refs/heads/landing/goal-a")
			proof := branch.LandingProof{Number: 1, Endpoint: result.Endpoint, Candidate: result.Candidate,
				Landing: result.Landing, RetryIdentity: result.RetryIdentity, Attempt: result.Attempt,
				Verdict: "red", Groups: []string{"deep"}}
			recordPath := "metasystem/records/misc/goal-a-landing.md"
			recordBytes := []byte(branch.RenderLandingProof(proof) + "\n")
			if err := os.MkdirAll(filepath.Join(f.root, "metasystem/records/misc"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(f.root, recordPath), recordBytes, 0o644); err != nil {
				t.Fatal(err)
			}
			git(t, f.root, "add", recordPath)
			if _, err := branch.CommitStaged(branch.CommitRequest{Repo: f.root, Remote: "origin", EndpointTip: f.base,
				GoalID: "goal-a", OpID: "transfer-red-record", Kind: branch.Plan, CheckClaim: claimAllowed}); err != nil {
				t.Fatal(err)
			}
			if _, err := branch.Push(pushRequest(f.branchFixture, "transfer-red-push")); err != nil {
				t.Fatal(err)
			}

			clone := filepath.Join(t.TempDir(), "clone-b")
			git(t, filepath.Dir(clone), "clone", "-q", "--no-local", f.origin, clone)
			git(t, clone, "config", "user.name", "Clone B")
			git(t, clone, "config", "user.email", "clone-b@example.invalid")
			git(t, clone, "config", "goal.human.Wido", "Wido Approver <wido@example.invalid>")
			freshTip := git(t, clone, "rev-parse", "origin/goal/goal-a")
			if freshTip != git(t, f.root, "rev-parse", "refs/heads/goal/goal-a") {
				t.Fatal("clone B did not receive the goal tip")
			}
			for _, commit := range append([]string{f.base}, f.units...) {
				git(t, clone, "cat-file", "-e", commit+"^{commit}")
			}
			transferred, err := exec.Command("git", "-C", clone, "show", freshTip+":"+recordPath).Output()
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(transferred, recordBytes) {
				t.Fatalf("transferred record bytes = %q; want %q", transferred, recordBytes)
			}
			parsed, err := branch.ParseLandingRecord(transferred)
			if err != nil || len(parsed) != 1 || parsed[0].RetryIdentity != result.RetryIdentity {
				t.Fatalf("transferred retry identity = %+v, err=%v", parsed, err)
			}
			for _, object := range []string{result.Landing + "^{commit}", result.Candidate + "^{tree}"} {
				if err := exec.Command("git", "-C", clone, "cat-file", "-e", object).Run(); err == nil {
					t.Fatalf("old landing object %s is available in clone B", object)
				}
			}
			if err := exec.Command("git", "-C", clone, "show-ref", "--verify", "--quiet", "refs/remotes/origin/landing/goal-a").Run(); err == nil {
				t.Fatal("old landing ref is available in clone B")
			}
			fresh := landFixture{branchFixture: &branchFixture{root: clone, origin: f.origin, base: f.base},
				units: append([]string(nil), f.units...), tip: freshTip}
			fresh.projected, err = landing.ProjectWorkspaceTree(filepath.Join(clone, "metasystem"), git(t, clone, "rev-parse", freshTip+"^{tree}"))
			if err != nil {
				t.Fatal(err)
			}
			fresh.receipt = filepath.Join(t.TempDir(), "retry.json")
			writeLandingReceipt(t, fresh.receipt, fresh.projected, "clone-b-retry")
			out := filepath.Join(t.TempDir(), "retry")
			before := git(t, clone, "worktree", "list", "--porcelain")
			_, err = branch.PrepareLanding(landRequest(t, fresh, out))
			requireLandCode(t, err, branch.LandRetryCode)
			requireAbsent(t, out)
			if after := git(t, clone, "worktree", "list", "--porcelain"); after != before {
				t.Fatalf("clone B scratch worktree remains:\n%s", after)
			}
			if state := git(t, clone, "status", "--short"); state != "" {
				t.Fatalf("clone B source changed: %s", state)
			}
			if err := exec.Command("git", "-C", f.origin, "show-ref", "--verify", "--quiet", "refs/heads/landing/goal-a").Run(); err == nil {
				t.Fatal("retry published a landing ref")
			}
		})
	})
}
