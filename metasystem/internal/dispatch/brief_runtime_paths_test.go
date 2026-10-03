package dispatch

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// nestedSeat is the m1f shape: a repository whose top holds plans/ and an
// installation folder metasystem/ that ignores its artifacts/, with a goal
// worktree checked out beside it.
type nestedSeat struct {
	top, install, worktree, worktreeInstall string
}

func newNestedSeat(t *testing.T) nestedSeat {
	t.Helper()
	parent := t.TempDir()
	resolved, err := filepath.EvalSymlinks(parent)
	if err != nil {
		t.Fatal(err)
	}
	seat := nestedSeat{top: filepath.Join(resolved, "seat")}
	seat.install = filepath.Join(seat.top, "metasystem")
	seat.worktree = filepath.Join(resolved, "seat-goal")
	seat.worktreeInstall = filepath.Join(seat.worktree, "metasystem")
	for name, content := range map[string]string{
		"plans/README.md":                 "top plans\n",
		"metasystem/.gitignore":           "artifacts/\nplans/goals/\n",
		"metasystem/metasystem.conf":      "\n",
		"metasystem/plans/README.md":      "installation plans\n",
		"metasystem/internal/a/a.go":      "package a\n",
		"metasystem/records/README.md":    "records\n",
		"metasystem/artifacts/agents/.no": "ignored\n",
	} {
		writeSeatFile(t, filepath.Join(seat.top, filepath.FromSlash(name)), content)
	}
	run := func(dir string, args ...string) {
		t.Helper()
		command := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=seat", "-c", "user.email=seat@example.invalid", "-c", "commit.gpgsign=false"}, args...)...)
		command.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
	}
	run(seat.top, "init", "-q", "-b", "main")
	run(seat.top, "add", "-A")
	run(seat.top, "commit", "-q", "-m", "base")
	run(seat.top, "worktree", "add", "-q", "-b", "goal/g", seat.worktree)
	return seat
}

func writeSeatFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// admitAtGoalWorktree runs the delegate's real admission for a critic
// dispatched from the goal worktree's installation: base tree and disk are
// the goal worktree's repository top.
func (seat nestedSeat) admitAtGoalWorktree(t *testing.T, text string) error {
	t.Helper()
	brief := filepath.Join(t.TempDir(), "brief.md")
	writeSeatFile(t, brief, text)
	_, err := ReadBriefAdmissionAtRoot(brief, seat.worktreeInstall, seat.worktree, seat.worktree, false)
	return err
}

func wantMissingPaths(t *testing.T, name string, err error, want ...string) {
	t.Helper()
	var refusal *BriefAuthorityRefusal
	if !errors.As(err, &refusal) || !reflect.DeepEqual(refusal.MissingPaths, want) {
		t.Fatalf("%s: admission = %v, want missing %v", name, err, want)
	}
	if first, remedy, _ := strings.Cut(err.Error(), "\n"); !strings.Contains(first, want[0]) || remedy == "" {
		t.Fatalf("%s: the refusal must name the path and a next step: %q", name, err.Error())
	}
}

// TestBriefAdmissionReadsRuntimePathsAtTheInstallation: a git-ignored path
// can never be in a tree, so admission looks for it on the disk the
// critic gets, at the cited path from the repository top or under the
// installation folder, as a brief written from the installation names it.
// A runtime path present at neither still refuses.
func TestBriefAdmissionReadsRuntimePathsAtTheInstallation(t *testing.T) {
	t.Parallel()
	seat := newNestedSeat(t)
	writeSeatFile(t, filepath.Join(seat.worktreeInstall, "artifacts", "agents", "context", "tool-gate.jsonl"), "{}\n")
	writeSeatFile(t, filepath.Join(seat.worktreeInstall, "plans", "goals", "g.md"), "goal notes\n")
	for _, cited := range []string{"artifacts/agents/context", "metasystem/artifacts/agents/context", "metasystem/plans/goals/g.md"} {
		if err := seat.admitAtGoalWorktree(t, "Working Mode: implement\n\nThe rows are under `"+cited+"`.\n"); err != nil {
			t.Fatalf("a runtime path the critic's disk holds was refused (%s): %v", cited, err)
		}
	}
	err := seat.admitAtGoalWorktree(t, "Working Mode: implement\n\nThe rows are under `artifacts/agents/absent`.\n")
	wantMissingPaths(t, "runtime path present nowhere", err, "artifacts/agents/absent")
	err = seat.admitAtGoalWorktree(t, "Working Mode: implement\n\nSee `metasystem/plans/goals/absent.md`.\n")
	wantMissingPaths(t, "ignored path present nowhere", err, "metasystem/plans/goals/absent.md")
}
