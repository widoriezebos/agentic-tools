package goal

import (
	"errors"
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

func wantShapedWithEnvironment(t *testing.T, root string, ledger []byte, environment []string, want bool, reasonPart string) {
	t.Helper()
	shaped, reason, err := adoptionShapedWithEnvironment(root, ledger, environment)
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

// Goal-free bytes (or none) are adoption-shaped when the checkout's
// history has no ledger. Parse refusals do not need a history probe.
func TestAdoptionShapedOutsideAndBeforeHistory(t *testing.T) {
	t.Parallel()
	check := func(root string, ledger []byte, tracked bool, probeErr error, want bool, reasonPart string, wantCalls int) {
		t.Helper()
		calls := 0
		shaped, reason, err := adoptionShapedWithProbe(root, ledger, func(actualRoot string) (bool, error) {
			if actualRoot != root {
				t.Fatalf("probe root=%q want %q", actualRoot, root)
			}
			calls++
			if calls > 1 {
				t.Fatal("history probed more than once")
			}
			return tracked, probeErr
		})
		if calls != wantCalls {
			t.Fatalf("history probes=%d want %d", calls, wantCalls)
		}
		if probeErr != nil {
			if shaped || reason != "" || !errors.Is(err, probeErr) {
				t.Fatalf("probe error tuple=(%v, %q, %v)", shaped, reason, err)
			}
			return
		}
		if err != nil || shaped != want || (!want && !strings.Contains(reason, reasonPart)) {
			t.Fatalf("shape tuple=(%v, %q, %v), want shaped=%v reason containing %q", shaped, reason, err, want, reasonPart)
		}
	}
	plain := t.TempDir()
	check(plain, []byte(goalFreeLedger), false, nil, true, "", 1)
	check(plain, nil, false, nil, true, "", 1)
	check(plain, []byte(goalLedger), false, nil, false, "already carries goals", 0)
	check(plain, []byte("# Goals\n\n## Nonsense"), false, nil, false, "malformed", 0)
	check(plain, []byte(goalFreeLedger), false, errors.New("history unavailable"), false, "", 1)

	unborn := t.TempDir()
	check(unborn, []byte(goalFreeLedger), false, nil, true, "", 1)

	committed := t.TempDir()
	writeFile(t, filepath.Join(committed, "README.md"), "hi\n")
	check(committed, []byte(goalFreeLedger), false, nil, true, "", 1)
}

// A tracked ledger is refused at the root's own prefix, even when its
// worktree file has been deleted.
func TestAdoptionShapedRefusesTrackedLedgerAtRootPrefix(t *testing.T) {
	t.Parallel()
	check := func(root string, ledger []byte, tracked, want bool, reasonPart string) {
		t.Helper()
		calls := 0
		shaped, reason, err := adoptionShapedWithProbe(root, ledger, func(actualRoot string) (bool, error) {
			if actualRoot != root {
				t.Fatalf("probe root=%q want %q", actualRoot, root)
			}
			calls++
			return tracked, nil
		})
		if calls != 1 || err != nil || shaped != want || (!want && !strings.Contains(reason, reasonPart)) {
			t.Fatalf("shape tuple=(%v, %q, %v), probes=%d; want shaped=%v reason containing %q and one probe", shaped, reason, err, calls, want, reasonPart)
		}
	}
	top := t.TempDir()
	writeFile(t, filepath.Join(top, "plans", "goals.md"), goalFreeLedger)
	writeFile(t, filepath.Join(top, "nested", "README.md"), "nested\n")

	check(top, []byte(goalFreeLedger), true, false, "committed history")
	// The nested root has an independent untracked history fact.
	nested := filepath.Join(top, "nested")
	check(nested, []byte(goalFreeLedger), false, true, "")

	writeFile(t, filepath.Join(nested, "plans", "goals.md"), goalFreeLedger)
	check(nested, []byte(goalFreeLedger), true, false, "committed history")
	// Deleting the worktree file leaves the tracked history fact intact.
	if err := os.Remove(filepath.Join(nested, "plans", "goals.md")); err != nil {
		t.Fatal(err)
	}
	check(nested, nil, true, false, "committed history")
}

// A probe that cannot run git refuses rather than authorizes.
func TestAdoptionShapedFailsClosedWithoutGit(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	environment := testEnvironment(os.Environ(), "PATH="+t.TempDir())
	shaped, _, err := adoptionShapedWithEnvironment(root, []byte(goalFreeLedger), environment)
	if err == nil || shaped {
		t.Fatalf("want a probe error and not shaped; got shaped=%v err=%v", shaped, err)
	}
}

// The probe answers about the checkout containing root, whatever the
// caller exported: a GIT_DIR pointing at some other repository — set
// deliberately, or inherited from a git hook or rebase subprocess —
// must not redirect the guard.
func TestGitAdapterAdoptionIgnoresGitSteeringEnv(t *testing.T) {
	t.Parallel()
	tracked := t.TempDir()
	gitOK(t, tracked, "init", "-q")
	writeFile(t, filepath.Join(tracked, "plans", "goals.md"), goalFreeLedger)
	gitOK(t, tracked, "add", ".")
	gitOK(t, tracked, "commit", "-qm", "ledger committed")

	empty := t.TempDir()
	gitOK(t, empty, "init", "-q")
	environment := testEnvironment(os.Environ(), "GIT_DIR="+filepath.Join(empty, ".git"), "GIT_WORK_TREE="+empty)
	wantShapedWithEnvironment(t, tracked, []byte(goalFreeLedger), environment, false, "committed history")

	plain := t.TempDir()
	environment = testEnvironment(os.Environ(), "GIT_DIR="+filepath.Join(tracked, ".git"), "GIT_WORK_TREE="+tracked)
	wantShapedWithEnvironment(t, plain, []byte(goalFreeLedger), environment, true, "")
}
