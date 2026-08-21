package goal

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// AdoptionShaped reports whether ledgerBytes, baselined into root, would
// state no more than adoption states: the ledger is parse-legal and
// goal-free, and the checkout's live branch carries no goal ledger. This
// is the whole of what a caller who is neither the human nor the root's
// lease holder may turn into a first baseline — a control plane that
// says "no intent here", on a root whose history has none — and nothing
// else. A ledger with goals is an initialized project missing its
// baseline, which only its holder restores; a ledger that HEAD tracks
// was deleted, not never written, and a deletion-then-reconcile must not
// become a ledger rewrite that a merge carries.
//
// The verb layer computes it before authorization so the authority
// matrix can decide on it; the store recomputes it under its lock so a
// ledger that changed while the caller waited is judged as it is now.
// A nil ledger (no goals.md) is vacuously goal-free: with no bytes to
// baseline the store has nothing to adopt and says so itself.
//
// The returned reason is the refusal text when shaped is false; err is a
// probe failure (git unreadable), which the caller treats as not shaped —
// a probe that cannot read refuses rather than authorizes.
func AdoptionShaped(root string, ledgerBytes []byte) (shaped bool, reason string, err error) {
	if ledgerBytes != nil {
		parsed, problems := Parse(ledgerBytes)
		if len(problems) > 0 {
			return false, fmt.Sprintf("the ledger is malformed: %s", problems[0]), nil
		}
		if parsed.HasGoals() {
			return false, "the ledger already carries goals but has no accepted baseline; only the lease holder may re-baseline an initialized project (a deleted goals-accepted.json is restored, not re-adopted)", nil
		}
	}
	tracked, err := headTracksLedger(root)
	if err != nil {
		return false, "", err
	}
	if tracked {
		return false, "the checkout's committed history carries a goal ledger; a deleted ledger or baseline is restored by the lease holder through `goal reconcile` or git, never re-adopted", nil
	}
	return true, "", nil
}

// headTracksLedger reports whether the checkout containing root tracks
// plans/goals.md at HEAD, resolved against root's own prefix (git does
// that: a nested checkout and a toplevel one ask the same question). Not
// a git work tree, or a work tree with no commit yet, tracks nothing.
// Any other git failure is an error: the guard fails closed.
func headTracksLedger(root string) (bool, error) {
	out, err := gitIn(root, "rev-parse", "--is-inside-work-tree")
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "not a git repository") {
			return false, nil
		}
		return false, err
	}
	if strings.TrimSpace(out) != "true" {
		// Inside a .git directory itself, or a bare repository: no work
		// tree, so no tracked ledger at a work-tree path.
		return false, nil
	}
	if _, err := gitIn(root, "rev-parse", "--verify", "-q", "HEAD^{commit}"); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == 1 {
			return false, nil // unborn HEAD: nothing committed yet
		}
		return false, err
	}
	out, err = gitIn(root, "ls-tree", "--name-only", "HEAD", "--", "plans/goals.md")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

// gitIn runs one git command with root as its working directory and
// returns stdout; a failure carries git's stderr so the caller can tell
// "not a repository" from a broken probe. The repository-steering git
// environment is stripped: the guard's whole point is that the checkout
// judged is the one CONTAINING root, and an inherited GIT_DIR (exported
// inside every git hook, and by rebase/bisect subprocesses) or its
// siblings would let the caller — deliberately or accidentally — point
// the probe at a repository of its choosing.
func gitIn(root string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	cmd.Env = environWithoutGitSteering()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return "", &gitError{ExitError: exit, stderr: strings.TrimSpace(stderr.String()), args: args}
		}
		return "", fmt.Errorf("git %s: %v", strings.Join(args, " "), err)
	}
	return stdout.String(), nil
}

// gitError is an exec.ExitError that also speaks git's stderr, so errors.As
// still finds the exit code and the message names the refusal.
type gitError struct {
	*exec.ExitError
	stderr string
	args   []string
}

func (e *gitError) Error() string {
	return fmt.Sprintf("git %s: %s", strings.Join(e.args, " "), e.stderr)
}

func (e *gitError) Unwrap() error { return e.ExitError }

// environWithoutGitSteering is the process environment minus every
// variable that redirects git away from the probed directory.
func environWithoutGitSteering() []string {
	steering := map[string]bool{
		"GIT_DIR": true, "GIT_WORK_TREE": true, "GIT_COMMON_DIR": true,
		"GIT_INDEX_FILE": true, "GIT_CEILING_DIRECTORIES": true,
		"GIT_OBJECT_DIRECTORY": true, "GIT_ALTERNATE_OBJECT_DIRECTORIES": true,
		"GIT_CONFIG": true, "GIT_CONFIG_PARAMETERS": true,
		"GIT_CONFIG_COUNT": true, "GIT_CONFIG_GLOBAL": true,
		"GIT_CONFIG_SYSTEM": true, "GIT_CONFIG_NOSYSTEM": true,
	}
	var out []string
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if steering[name] || strings.HasPrefix(name, "GIT_CONFIG_KEY_") || strings.HasPrefix(name, "GIT_CONFIG_VALUE_") {
			continue
		}
		out = append(out, entry)
	}
	return out
}
