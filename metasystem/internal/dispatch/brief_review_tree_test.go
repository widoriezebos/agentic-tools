package dispatch

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gitIn runs one Git command in dir with the seat's identity and no user
// configuration, and returns its trimmed output.
func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=seat", "-c", "user.email=seat@example.invalid", "-c", "commit.gpgsign=false"}, args...)...)
	command.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}

// TestReviewBriefAdmissionMatrix is the m1f shape of a critic's brief
// admission: the installation lives under metasystem/, main holds the
// accepted design, and the goal branch's unit commit adds a file main does
// not have. The critic is dispatched from the primary checkout (HEAD main)
// to review the unit commit. Each row is one citation; a tracked path is
// admitted when the tree the critic reads holds it, at the cited path or
// under the installation folder; a path in neither tree nor on disk still
// refuses, naming the path and a next step.
func TestReviewBriefAdmissionMatrix(t *testing.T) {
	t.Parallel()
	seat := newNestedSeat(t)
	writeSeatFile(t, filepath.Join(seat.install, "plans", "designs", "accepted.md"), "the accepted design\n")
	gitIn(t, seat.top, "add", "-A")
	gitIn(t, seat.top, "commit", "-q", "-m", "design on main")
	gitIn(t, seat.worktree, "merge", "-q", "--ff-only", "main")
	writeSeatFile(t, filepath.Join(seat.worktreeInstall, "internal", "root", "root.go"), "package root\n")
	gitIn(t, seat.worktree, "add", "-A")
	gitIn(t, seat.worktree, "commit", "-q", "-m", "unit adds internal/root")
	unit := gitIn(t, seat.worktree, "rev-parse", "HEAD")
	reviews := "commit:" + unit

	admit := func(cited, reviews string) error {
		t.Helper()
		brief := filepath.Join(t.TempDir(), "brief.md")
		writeSeatFile(t, brief, "Working Mode: implement\n\nRead `"+cited+"`.\n")
		_, err := ReadReviewBriefAdmission(brief, seat.install, seat.top, seat.top, reviews)
		return err
	}
	for _, row := range []struct {
		name, cited, reviews string
		missing              string
	}{
		{name: "tracked path cited from the installation", cited: "plans/designs/accepted.md", reviews: reviews},
		{name: "tracked path cited from the repository top", cited: "metasystem/plans/designs/accepted.md", reviews: reviews},
		{name: "file only the unit commit holds", cited: "metasystem/internal/root/root.go", reviews: reviews},
		{name: "file only the unit commit holds, cited from the installation", cited: "internal/root/root.go", reviews: reviews},
		{name: "file the reviewed commit lacks and main lacks", cited: "metasystem/internal/root/absent.go", reviews: reviews, missing: "metasystem/internal/root/absent.go"},
		{name: "installation path present in neither tree", cited: "plans/designs/absent.md", reviews: reviews, missing: "plans/designs/absent.md"},
		{name: "unit file with no reviewed commit named", cited: "metasystem/internal/root/root.go", missing: "metasystem/internal/root/root.go"},
	} {
		err := admit(row.cited, row.reviews)
		if row.missing == "" {
			if err != nil {
				t.Errorf("%s: %s was refused: %v", row.name, row.cited, err)
			}
			continue
		}
		var refusal *BriefAuthorityRefusal
		if !errors.As(err, &refusal) || len(refusal.MissingPaths) != 1 || refusal.MissingPaths[0] != row.missing {
			t.Errorf("%s: admission = %v, want %s refused", row.name, err, row.missing)
			continue
		}
		if first, remedy, _ := strings.Cut(err.Error(), "\n"); !strings.Contains(first, row.missing) || remedy == "" {
			t.Errorf("%s: the refusal must name the path and a next step: %q", row.name, err.Error())
		}
	}
	// A reviewed commit the repository does not hold is never read as
	// HEAD: admission refuses to judge.
	if err := admit("plans/designs/accepted.md", "commit:"+strings.Repeat("0", 40)); err == nil {
		t.Errorf("an unknown reviewed commit was admitted")
	}
}
