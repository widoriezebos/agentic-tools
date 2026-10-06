package branch

import (
	"errors"
	"fmt"
	"testing"
)

// The park check's answers once the branch is known to be pushed, and the
// sweep policy over a caller's raw local-ref reader. Both run over the
// scripted status facts of status_policy_test.go; no repository is read.

// Parking names the last unit without validating the range or its reads.
func TestParkOfAPushedBranchAnswersItsRangeOrItsEmptiness(t *testing.T) {
	t.Parallel()

	repo := t.TempDir()
	ref := goalBranchRef("goal-a")
	f := newStatusFacts(t,
		tipFact(repo, ref, statusTip, true, nil),
		statusGitFact(repo, statusR1+"\n"+statusU2+"\n"+statusU1, nil, "rev-list", "--first-parent", statusBase+".."+statusTip),
		kindFact(repo, statusR1, "goal-a", KindInfo{Kind: Read}, nil),
		kindFact(repo, statusU2, "goal-a", KindInfo{Kind: Unit, Unit: "u2"}, nil),
		tipFact(repo, ref, statusTip, true, nil),
		statusGitFact(repo, "", nil, "rev-list", "--first-parent", statusBase+".."+statusTip),
	)
	deps := f.dependencies()
	pushed := func() (string, string, bool, error) { return statusBase, statusTip, true, nil }

	state, err := checkParkBranch(repo, "goal-a", "continue", pushed, deps)
	want := fmt.Sprintf("goal/goal-a at %s is pushed; last unit u2 commit %s", statusTip, statusU2)
	if err != nil || !state.Branch || state.Summary != want {
		t.Fatalf("a pushed branch parked as %+v, %v; want %q", state, err, want)
	}
	state, err = checkParkBranch(repo, "goal-a", "continue", pushed, deps)
	want = fmt.Sprintf("goal/goal-a at %s has no unit", statusTip)
	if err != nil || !state.Branch || state.Summary != want {
		t.Fatalf("a pushed branch with no unit parked as %+v, %v; want %q", state, err, want)
	}
	f.assertCalls(t, "local-tip", "git", "kind", "kind", "local-tip", "git")
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
