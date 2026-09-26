package branch

import (
	"errors"
	"fmt"
	"testing"
)

// The park check's answers once the branch is known to be pushed, and the
// sweep policy over a caller's raw local-ref reader. Both run over the
// scripted status facts of status_policy_test.go; no repository is read.

// A pushed branch whose range cannot be validated refuses the park with the
// range's own error rather than answering a summary it could not read, and a
// pushed branch carrying no unit says exactly that.
func TestParkOfAPushedBranchAnswersItsRangeOrItsEmptiness(t *testing.T) {
	t.Parallel()

	repo := t.TempDir()
	ref := goalBranchRef("goal-a")
	unreadable := errors.New("range is not ancestry-linked")
	f := newStatusFacts(t,
		tipFact(repo, ref, statusTip, true, nil),
		rangeFact(repo, statusBase, statusTip, "goal-a", nil, unreadable),
		tipFact(repo, ref, statusTip, true, nil),
		rangeFact(repo, statusBase, statusTip, "goal-a", []Commit{}, nil),
	)
	deps := f.dependencies()
	pushed := func() (string, string, bool, error) { return statusBase, statusTip, true, nil }

	state, err := checkParkBranch(repo, "goal-a", "continue", pushed, deps)
	if !errors.Is(err, unreadable) || state.Branch {
		t.Fatalf("a pushed branch with an unreadable range parked as %+v, %v; want the range error", state, err)
	}
	state, err = checkParkBranch(repo, "goal-a", "continue", pushed, deps)
	want := fmt.Sprintf("goal/goal-a at %s has no unit", statusTip)
	if err != nil || !state.Branch || state.Summary != want {
		t.Fatalf("a pushed branch with no unit parked as %+v, %v; want %q", state, err, want)
	}
	f.assertCalls(t, "local-tip", "range", "local-tip", "range")
}

// The sweep policy over a caller's local-ref reader sweeps a present branch,
// keeps an absent one whose next step names no unit, and refuses to decide
// when the local ref cannot be read.
func TestSweepPolicyOverACallersLocalRefReader(t *testing.T) {
	t.Parallel()

	unreadable := errors.New("packed-refs is locked")
	for name, c := range map[string]struct {
		present bool
		err     error
		sweep   bool
	}{
		"a present local branch":  {present: true, sweep: true},
		"an absent local branch":  {present: false, sweep: false},
		"an unreadable local ref": {err: unreadable},
	} {
		asked := ""
		sweep, err := ShouldSweepWithLocalTip("/repo", "goal-a", "ordinary next step", func(repo, ref string) (string, bool, error) {
			asked = repo + " " + ref
			return statusTip, c.present, c.err
		})
		if asked != "/repo "+goalBranchRef("goal-a") {
			t.Errorf("%s: the reader was asked %q; want the goal branch ref", name, asked)
		}
		if !errors.Is(err, c.err) || sweep != c.sweep {
			t.Errorf("%s: sweep %v err %v; want %v, %v", name, sweep, err, c.sweep, c.err)
		}
	}
}
