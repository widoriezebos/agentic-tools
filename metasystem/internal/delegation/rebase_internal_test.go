package delegation

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// scriptedGit answers Git by argv; an unscripted call fails.
type scriptedGit struct {
	answers map[string]scriptedAnswer
	// sequences answer a repeated call in order, the last answer standing.
	sequences map[string][]scriptedAnswer
	calls     []string
}

type scriptedAnswer struct {
	stdout string
	fail   bool
}

func (g *scriptedGit) Run(_ context.Context, _ string, args ...string) ([]byte, []byte, error) {
	key := strings.Join(args, " ")
	g.calls = append(g.calls, key)
	answer, ok := g.answers[key]
	if sequence := g.sequences[key]; len(sequence) > 0 {
		answer, ok = sequence[0], true
		if len(sequence) > 1 {
			g.sequences[key] = sequence[1:]
		}
	}
	if !ok || answer.fail {
		return []byte(answer.stdout), []byte("scripted failure"), &GitExitError{Code: 1, Stderr: "scripted failure"}
	}
	return []byte(answer.stdout), nil, nil
}

func rebaseSession(git *scriptedGit) *session {
	var stderr bytes.Buffer
	return &session{l: &Lifecycle{ports: Ports{Git: git}}, ctx: context.Background(), stderr: &stderr}
}

// A restoration that cannot reset or reapply names each failed step and
// keeps the tagged stash, identified by hash and tag, for manual recovery
// (the retired script's sourced restore_follow_up_rebase_worktree leg).
func TestRebaseRestoreFailureKeepsAndNamesItsTaggedStash(t *testing.T) {
	t.Parallel()
	git := &scriptedGit{answers: map[string]scriptedAnswer{
		"reset --hard -q base-commit": {fail: true},
		"clean -fdq":                  {},
		"stash apply -q stash-hash":   {fail: true},
	}}
	failure := rebaseSession(git).restoreRebaseWorktree("/w", "base-commit", "stash-hash", "restore-tag")
	for _, want := range []string{"git reset --hard to base-commit", "git stash apply stash-hash",
		"stash hash stash-hash with tag restore-tag remains for manual recovery"} {
		if !strings.Contains(failure, want) {
			t.Fatalf("failure %q lacks %q", failure, want)
		}
	}
	for _, call := range git.calls {
		if strings.HasPrefix(call, "stash drop") {
			t.Fatalf("a failed restore dropped its stash: %v", git.calls)
		}
	}
}

// A clean restore drops exactly the stash it made, and says so when the
// stash outlives the drop.
func TestRebaseRestoreDropsItsOwnStash(t *testing.T) {
	t.Parallel()
	listed := "stash@{0}\tstash-hash\tOn agent/job: restore-tag\n"
	answers := map[string]scriptedAnswer{
		"reset --hard -q base-commit": {},
		"clean -fdq":                  {},
		"stash apply -q stash-hash":   {},
		"stash drop -q stash@{0}":     {},
	}
	git := &scriptedGit{answers: answers, sequences: map[string][]scriptedAnswer{
		"stash list --format=%gd%x09%H%x09%gs": {{stdout: listed}, {stdout: ""}},
	}}
	if failure := rebaseSession(git).restoreRebaseWorktree("/w", "base-commit", "stash-hash", "restore-tag"); failure != "" {
		t.Fatalf("a clean restore failed: %s (calls %v)", failure, git.calls)
	}
	stuck := &scriptedGit{answers: answers, sequences: map[string][]scriptedAnswer{
		"stash list --format=%gd%x09%H%x09%gs": {{stdout: listed}},
	}}
	failure := rebaseSession(stuck).restoreRebaseWorktree("/w", "base-commit", "stash-hash", "restore-tag")
	if !strings.Contains(failure, "removing stash hash stash-hash with tag restore-tag failed; it remains for manual recovery") {
		t.Fatalf("a stash that outlives its drop: %q", failure)
	}
}
