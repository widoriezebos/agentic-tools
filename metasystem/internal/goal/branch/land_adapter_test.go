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
	t.Run("series", func(t *testing.T) {
		f := newLandFixture(t)
		beforeWorktrees := git(t, f.root, "worktree", "list", "--porcelain")
		out := filepath.Join(t.TempDir(), "prepared")
		result, err := branch.PrepareLanding(landRequest(t, f, out))
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
	})
}
