package landing

import (
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// A held push binds each goal commit to one positive Goal-Revision. Agents are
// refused without one; a human actor is warned and the push continues.
func TestHeldRequiresAPositiveGoalRevision(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, actor, revision string
		want                  string
	}{
		{name: "agent zero revision", actor: "m9+L1", revision: "0", want: "has no positive Goal-Revision"},
		{name: "agent non-numeric revision", actor: "m9+L1", revision: "three", want: "has no positive Goal-Revision"},
		{name: "human zero revision", actor: "m9+human", revision: "0", want: "has no positive Goal-Revision"},
		{name: "human missing revision", actor: "m9+human", want: "has no Goal-Revision"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			f := newHeldPolicyFixture(t)
			base, commit, tree := f.id(), f.id(), f.id()
			f.opening(base, commit, heldRawRange(heldCommit{commit, base}))
			f.object(commit, base, "revision binding", heldTrailersFor(test.actor, "ship-widget", test.revision, "")...)
			f.normalEndpoint()
			f.tree(base, tree)
			got := f.verdict(base, commit, "origin", "refs/heads/main")
			if !heldHumanActor(test.actor) {
				assertHeldRefusal(t, got, 1, "goal-revision-unbound", commit, "goal item ship-widget "+test.want)
				return
			}
			if got.Outcome != "ok" || got.ExitCode != 0 || got.Refusal != nil || len(got.Warnings) != 1 ||
				!strings.HasPrefix(got.Warnings[0], "held: warning goal-revision-unbound: "+commit) || !strings.HasSuffix(got.Warnings[0], test.want) {
				t.Fatalf("human revision defect was not softened to one warning: %+v", got)
			}
		})
	}
}

// A commit object without a message body cannot be classified, so the push is
// reported unreadable rather than judged.
func TestHeldReportsACommitWithoutAMessageUnreadable(t *testing.T) {
	t.Parallel()
	f := newHeldPolicyFixture(t)
	base, commit := f.id(), f.id()
	f.opening(base, commit, heldRawRange(heldCommit{commit, base}))
	f.expect("object", commit, "", []byte("tree "+strings.Repeat("a", 40)+"\nparent "+base+"\n"), true, goal.Endpoint{})
	got := f.verdict(base, commit, "origin", "refs/heads/main")
	if got.Outcome != "unreadable" || got.ExitCode != 2 || got.Refusal != nil || len(got.Warnings) != 1 || got.Warnings[0] != "held: unreadable: "+commit {
		t.Fatalf("message-less commit was not reported unreadable: %+v", got)
	}
}

// Duplicate Goal-Revision trailers are malformed before actor policy applies,
// so a human actor cannot soften them.
func TestHeldRefusesTwoGoalRevisionTrailersEvenFromAHuman(t *testing.T) {
	t.Parallel()
	f := newHeldPolicyFixture(t)
	base, commit := f.id(), f.id()
	f.opening(base, commit, heldRawRange(heldCommit{commit, base}))
	f.object(commit, base, "two revisions", "Machine: m9+human", "Goal-Item: ship-widget", "Goal-Revision: 3", "Goal-Revision: 4")
	got := f.verdict(base, commit, "origin", "refs/heads/main")
	assertHeldRefusal(t, got, 1, "machine-trailer-malformed", commit, "2 Goal-Revision trailers")
}

// A human commit without a Goal-Item on an ordinary ledger is warned, not
// refused, and the push continues.
func TestHeldWarnsAHumanCommitWithoutAGoalItem(t *testing.T) {
	t.Parallel()
	f := newHeldPolicyFixture(t)
	base, commit, tree := f.id(), f.id(), f.id()
	f.capture(tree)
	f.opening(base, commit, heldRawRange(heldCommit{commit, base}))
	f.object(commit, base, "human without goal", heldTrailersFor("m9+human", "", "", "")...)
	f.normalEndpoint()
	f.tree(base, tree)
	f.file(tree, "plans/goals/backlog.md")
	got := f.verdict(base, commit, "origin", "refs/heads/main")
	if got.Outcome != "ok" || got.ExitCode != 0 || got.Refusal != nil || len(got.Warnings) != 1 ||
		!strings.HasPrefix(got.Warnings[0], "held: warning goal-binding-missing: "+commit+": agent landings are goal work") {
		t.Fatalf("human commit without a Goal-Item was not softened to one warning: %+v", got)
	}
}
