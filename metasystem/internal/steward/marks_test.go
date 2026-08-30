package steward

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func marksGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

// The ledger mark reads the accepted goals ref, not the retired
// plans/goals.md: goal movement with a still head reads as PROGRESS, and
// the retired file's churn moves nothing (steward-marks-retired-ledger).
func TestCurrentMarksFollowTheAcceptedGoalsRef(t *testing.T) {
	repo := t.TempDir()
	marksGit(t, repo, "init", "-q", "-b", "main")
	marksGit(t, repo, "-c", "user.name=m", "-c", "user.email=m@example.invalid",
		"commit", "-q", "--allow-empty", "-m", "seed")

	before, err := CurrentMarks(repo)
	if err != nil {
		t.Fatal(err)
	}
	if before.OpidDigest != "no-ledger" {
		t.Fatalf("absent accepted ref must keep the sentinel: %q", before.OpidDigest)
	}

	// The retired ledger file no longer speaks for the mark.
	if err := os.MkdirAll(filepath.Join(repo, "plans"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "plans", "goals.md"), []byte("retired churn\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	retired, err := CurrentMarks(repo)
	if err != nil {
		t.Fatal(err)
	}
	if retired.OpidDigest != "no-ledger" {
		t.Fatalf("the retired file moved the mark: %q", retired.OpidDigest)
	}

	// Ledger-only movement: the accepted ref advances, HEAD stays still,
	// and the mark reads PROGRESS.
	marksGit(t, repo, "-c", "user.name=m", "-c", "user.email=m@example.invalid",
		"commit", "-q", "--allow-empty", "-m", "ledger tip one")
	tip := marksGit(t, repo, "rev-parse", "HEAD")
	marksGit(t, repo, "update-ref", "refs/metasystem/goals/accepted", tip[:40])
	marksGit(t, repo, "reset", "-q", "--hard", "HEAD~1")

	first, err := CurrentMarks(repo)
	if err != nil {
		t.Fatal(err)
	}
	if first.OpidDigest == "no-ledger" || first.OpidDigest == retired.OpidDigest {
		t.Fatalf("an accepted ref tip must move the mark: %q", first.OpidDigest)
	}
	if first.HeadOid != before.HeadOid {
		t.Fatalf("the ledger ref advance must not move HEAD's mark: %q vs %q", first.HeadOid, before.HeadOid)
	}

	// A second advance moves the digest again.
	marksGit(t, repo, "-c", "user.name=m", "-c", "user.email=m@example.invalid",
		"commit", "-q", "--allow-empty", "-m", "ledger tip two")
	tip2 := marksGit(t, repo, "rev-parse", "HEAD")
	marksGit(t, repo, "update-ref", "refs/metasystem/goals/accepted", tip2[:40])
	marksGit(t, repo, "reset", "-q", "--hard", "HEAD~1")
	second, err := CurrentMarks(repo)
	if err != nil {
		t.Fatal(err)
	}
	if second.OpidDigest == first.OpidDigest {
		t.Fatal("a second ledger advance must read as fresh progress")
	}
}
