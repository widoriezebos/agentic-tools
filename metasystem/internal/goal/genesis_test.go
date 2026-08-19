package goal

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const goalFreeLedger = "# Goals\n\n## Goal-free: declared 2026-08-15T12:00:00Z by human over abc\n"
const goalLedger = "# Goals\n\n## Current goal: solo — One goal\n- Origin: main\n- Next step: Do.\n"

func gitOK(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func wantShaped(t *testing.T, root string, ledger []byte, want bool, reasonPart string) {
	t.Helper()
	shaped, reason, err := AdoptionShaped(root, ledger)
	if err != nil {
		t.Fatalf("AdoptionShaped: %v", err)
	}
	if shaped != want {
		t.Fatalf("shaped=%v want %v (reason %q)", shaped, want, reason)
	}
	if !want && !strings.Contains(reason, reasonPart) {
		t.Fatalf("reason %q does not name %q", reason, reasonPart)
	}
}

// The shape a non-holder may baseline: goal-free bytes (or none) on a
// root that is not a git work tree, or one whose history carries no
// ledger yet — unborn HEAD, or commits without the ledger.
func TestAdoptionShapedOutsideAndBeforeHistory(t *testing.T) {
	plain := t.TempDir()
	wantShaped(t, plain, []byte(goalFreeLedger), true, "")
	wantShaped(t, plain, nil, true, "")
	wantShaped(t, plain, []byte(goalLedger), false, "already carries goals")
	wantShaped(t, plain, []byte("# Goals\n\n## Nonsense"), false, "malformed")

	unborn := t.TempDir()
	gitOK(t, unborn, "init", "-q")
	wantShaped(t, unborn, []byte(goalFreeLedger), true, "")

	committed := t.TempDir()
	gitOK(t, committed, "init", "-q")
	writeFile(t, filepath.Join(committed, "README.md"), "hi\n")
	gitOK(t, committed, "add", ".")
	gitOK(t, committed, "commit", "-qm", "no ledger")
	wantShaped(t, committed, []byte(goalFreeLedger), true, "")
}

// A ledger the live branch tracks was deleted, not never written: the
// shape refuses it for a non-holder, whatever the bytes say now. The
// question is asked against the root's own prefix, so a nested checkout
// is judged by its own plans/goals.md, not the toplevel's.
func TestAdoptionShapedRefusesTrackedLedgerAtRootPrefix(t *testing.T) {
	top := t.TempDir()
	gitOK(t, top, "init", "-q")
	writeFile(t, filepath.Join(top, "plans", "goals.md"), goalFreeLedger)
	writeFile(t, filepath.Join(top, "nested", "README.md"), "nested\n")
	gitOK(t, top, "add", ".")
	gitOK(t, top, "commit", "-qm", "toplevel ledger only")

	wantShaped(t, top, []byte(goalFreeLedger), false, "committed history")
	// The nested root's history has no ledger of its own.
	nested := filepath.Join(top, "nested")
	wantShaped(t, nested, []byte(goalFreeLedger), true, "")

	writeFile(t, filepath.Join(nested, "plans", "goals.md"), goalFreeLedger)
	gitOK(t, top, "add", ".")
	gitOK(t, top, "commit", "-qm", "nested ledger")
	wantShaped(t, nested, []byte(goalFreeLedger), false, "committed history")
	// Deleting the tracked file from the work tree changes nothing: the
	// guard reads HEAD, not the directory.
	if err := os.Remove(filepath.Join(nested, "plans", "goals.md")); err != nil {
		t.Fatal(err)
	}
	wantShaped(t, nested, nil, false, "committed history")
}

// A probe that cannot run git refuses rather than authorizes.
func TestAdoptionShapedFailsClosedWithoutGit(t *testing.T) {
	root := t.TempDir()
	t.Setenv("PATH", t.TempDir())
	shaped, _, err := AdoptionShaped(root, []byte(goalFreeLedger))
	if err == nil || shaped {
		t.Fatalf("want a probe error and not shaped; got shaped=%v err=%v", shaped, err)
	}
}

// The probe answers about the checkout containing root, whatever the
// caller exported: a GIT_DIR pointing at some other repository — set
// deliberately, or inherited from a git hook or rebase subprocess —
// must not redirect the guard.
func TestAdoptionShapedIgnoresGitSteeringEnv(t *testing.T) {
	tracked := t.TempDir()
	gitOK(t, tracked, "init", "-q")
	writeFile(t, filepath.Join(tracked, "plans", "goals.md"), goalFreeLedger)
	gitOK(t, tracked, "add", ".")
	gitOK(t, tracked, "commit", "-qm", "ledger committed")

	empty := t.TempDir()
	gitOK(t, empty, "init", "-q")
	t.Setenv("GIT_DIR", filepath.Join(empty, ".git"))
	t.Setenv("GIT_WORK_TREE", empty)
	wantShaped(t, tracked, []byte(goalFreeLedger), false, "committed history")

	plain := t.TempDir()
	t.Setenv("GIT_DIR", filepath.Join(tracked, ".git"))
	t.Setenv("GIT_WORK_TREE", tracked)
	wantShaped(t, plain, []byte(goalFreeLedger), true, "")
}
