package landing

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

func TestHeldRecheckReadsTheParent(t *testing.T) {
	f := newHeldPolicyFixture(t)
	base, commit, tree := f.id(), f.id(), f.id()
	f.claimed("ship-widget", "m9", "L1", 3)
	f.capture(tree, "plans/goals/ship-widget.md")
	f.opening(base, commit, heldRawRange(heldCommit{commit, base}))
	f.object(commit, base, "held parent", heldTrailersFor("m9+L1", "ship-widget", "3", "")...)
	f.normalEndpoint()
	f.tree(base, tree)
	f.file(tree, "plans/goals/ship-widget.md")
	got := f.verdict(base, commit, "origin", "refs/heads/main")
	if got.Outcome != "ok" || got.ExitCode != 0 || got.Commits != 1 || got.Base != base || got.Commit != commit {
		t.Fatalf("matching parent classified as %+v", got)
	}

	abandoned := newHeldPolicyFixture(t)
	abandonedBase, abandonedCommit, abandonedTree := abandoned.id(), abandoned.id(), abandoned.id()
	abandoned.abandoned("ship-widget")
	abandoned.capture(abandonedTree, "records/goals/ship-widget.md")
	abandoned.opening(abandonedBase, abandonedCommit, heldRawRange(heldCommit{abandonedCommit, abandonedBase}))
	abandoned.object(abandonedCommit, abandonedBase, "held abandoned parent", heldTrailersFor("m9+L1", "ship-widget", "3", "")...)
	abandoned.normalEndpoint()
	abandoned.tree(abandonedBase, abandonedTree)
	abandoned.file(abandonedTree, "plans/goals/ship-widget.md")
	abandoned.file(abandonedTree, "records/goals/ship-widget.md")
	got = abandoned.verdict(abandonedBase, abandonedCommit, "origin", "refs/heads/main")
	assertHeldRefusal(t, got, 1, "goal-item-not-held", abandonedCommit, "goal ship-widget is abandoned at "+shortHeldID(abandonedBase))

	moved := newHeldPolicyFixture(t)
	movedBase, movedCommit, movedTree := moved.id(), moved.id(), moved.id()
	moved.claimed("ship-widget", "m9", "L1", 5)
	moved.capture(movedTree, "plans/goals/ship-widget.md")
	moved.opening(movedBase, movedCommit, heldRawRange(heldCommit{movedCommit, movedBase}))
	moved.object(movedCommit, movedBase, "held moved revision", heldTrailersFor("m9+L1", "ship-widget", "3", "")...)
	moved.normalEndpoint()
	moved.tree(movedBase, movedTree)
	moved.file(movedTree, "plans/goals/ship-widget.md")
	got = moved.verdict(movedBase, movedCommit, "origin", "refs/heads/main")
	assertHeldRefusal(t, got, 1, "goal-revision-moved", movedCommit, "claimed at revision 5")

	human := newHeldPolicyFixture(t)
	humanBase, humanCommit, humanTree := human.id(), human.id(), human.id()
	human.claimed("ship-widget", "m9", "L1", 5)
	human.capture(humanTree, "plans/goals/ship-widget.md")
	human.opening(humanBase, humanCommit, heldRawRange(heldCommit{humanCommit, humanBase}))
	human.object(humanCommit, humanBase, "held human warning", heldTrailersFor("m9+human", "ship-widget", "3", "")...)
	human.normalEndpoint()
	human.tree(humanBase, humanTree)
	human.file(humanTree, "plans/goals/ship-widget.md")
	got = human.verdict(humanBase, humanCommit, "origin", "refs/heads/main")
	if got.ExitCode != 0 || got.Outcome != "ok" || len(got.Warnings) != 1 || !strings.Contains(got.Warnings[0], "held: warning goal-revision-moved: "+humanCommit) {
		t.Fatalf("human revision mismatch was not softened: %+v", got)
	}

	unbound := newHeldPolicyFixture(t)
	unboundBase, unboundCommit, unboundTree := unbound.id(), unbound.id(), unbound.id()
	unbound.claimed("ship-widget", "m9", "L1", 3)
	unbound.capture(unboundTree, "plans/goals/ship-widget.md")
	unbound.opening(unboundBase, unboundCommit, heldRawRange(heldCommit{unboundCommit, unboundBase}))
	unbound.object(unboundCommit, unboundBase, "held unbound revision", heldTrailersFor("m9+L1", "ship-widget", "", "")...)
	unbound.normalEndpoint()
	unbound.tree(unboundBase, unboundTree)
	got = unbound.verdict(unboundBase, unboundCommit, "origin", "refs/heads/main")
	assertHeldRefusal(t, got, 1, "goal-revision-unbound", unboundCommit, "has no Goal-Revision")
}

func TestHeldRefusesAMalformedMachineTrailerBeforeAnythingElse(t *testing.T) {
	for _, test := range []struct {
		name     string
		trailers []string
		want     string
	}{
		{name: "two Machine trailers", trailers: []string{"Machine: m9+L1", "Machine: forged+human", "Goal-Item: ship-widget", "Goal-Revision: 3"}, want: "2 Machine trailers"},
		{name: "no Machine trailer", trailers: []string{"Goal-Item: ship-widget", "Goal-Revision: 3"}, want: "0 Machine trailers"},
		{name: "two Goal-Item trailers", trailers: []string{"Machine: m9+L1", "Goal-Item: ship-widget", "Goal-Item: other", "Goal-Revision: 3"}, want: "2 Goal-Item trailers"},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newHeldPolicyFixture(t)
			base, commit := f.id(), f.id()
			f.claimed("ship-widget", "m9", "L1", 3)
			f.opening(base, commit, heldRawRange(heldCommit{commit, base}))
			f.object(commit, base, "malformed trailer", test.trailers...)
			got := f.verdict(base, commit, "origin", "refs/heads/main")
			assertHeldRefusal(t, got, 1, "machine-trailer-malformed", commit, test.want)
		})
	}
}

func TestHeldChecksTheGoalEndpoint(t *testing.T) {
	newBed := func(t *testing.T, actor, endpointRemote, endpointBranch string, readTree bool) (*heldPolicyFixture, string, string) {
		f := newHeldPolicyFixture(t)
		base, commit, tree := f.id(), f.id(), f.id()
		f.claimed("ship-widget", "m9", "L1", 3)
		f.capture(tree, "plans/goals/ship-widget.md")
		f.opening(base, commit, heldRawRange(heldCommit{commit, base}))
		f.object(commit, base, "endpoint", heldTrailersFor(actor, "ship-widget", "3", "")...)
		f.endpoint(endpointRemote, endpointBranch)
		if readTree {
			f.tree(base, tree)
			f.file(tree, "plans/goals/ship-widget.md")
		}
		return f, base, commit
	}

	for _, test := range []struct {
		name, remote, ref              string
		endpointRemote, endpointBranch string
	}{
		{name: "other remote", remote: "transport", ref: "refs/heads/main", endpointRemote: "origin", endpointBranch: "refs/heads/main"},
		{name: "other ref", remote: "origin", ref: "refs/heads/paper", endpointRemote: "origin", endpointBranch: "refs/heads/main"},
		{name: "local endpoint", remote: "origin", ref: "refs/heads/main", endpointRemote: "local", endpointBranch: "refs/heads/main"},
	} {
		t.Run(test.name, func(t *testing.T) {
			f, base, commit := newBed(t, "m9+L1", test.endpointRemote, test.endpointBranch, false)
			got := f.verdict(base, commit, test.remote, test.ref)
			assertHeldRefusal(t, got, 1, "endpoint-mismatch", commit, "this landing pushes "+test.remote+" "+test.ref)
		})
	}

	t.Run("configured branch", func(t *testing.T) {
		f, base, commit := newBed(t, "m9+L1", "origin", "refs/heads/other", true)
		got := f.verdict(base, commit, "origin", "refs/heads/other")
		if got.Outcome != "ok" || got.ExitCode != 0 {
			t.Fatalf("configured endpoint classified as %+v", got)
		}
	})

	t.Run("human warning", func(t *testing.T) {
		f, base, commit := newBed(t, "m9+human", "origin", "refs/heads/main", false)
		got := f.verdict(base, commit, "transport", "refs/heads/main")
		if got.ExitCode != 0 || len(got.Warnings) != 1 || !strings.Contains(got.Warnings[0], "held: warning endpoint-mismatch: "+commit) {
			t.Fatalf("human endpoint mismatch was not softened: %+v", got)
		}
	})
}

func TestHeldRequiresAGoalBindingFromANonHumanActor(t *testing.T) {
	t.Run("ordinary ledger", func(t *testing.T) {
		f := newHeldPolicyFixture(t)
		base, commit, tree := f.id(), f.id(), f.id()
		f.capture(tree)
		f.opening(base, commit, heldRawRange(heldCommit{commit, base}))
		f.object(commit, base, "missing goal", heldTrailersFor("m9+L1", "", "", "")...)
		f.normalEndpoint()
		f.tree(base, tree)
		f.file(tree, "plans/goals/backlog.md")
		got := f.verdict(base, commit, "origin", "refs/heads/main")
		assertHeldRefusal(t, got, 1, "goal-binding-missing", commit, "agent landings are goal work")
	})

	t.Run("Goal-free ledger", func(t *testing.T) {
		f := newHeldPolicyFixture(t)
		base, commit, tree := f.id(), f.id(), f.id()
		f.rootFile(true)
		f.capture(tree, "plans/goals/backlog.md")
		f.opening(base, commit, heldRawRange(heldCommit{commit, base}))
		f.object(commit, base, "goal free", heldTrailersFor("m9+L1", "", "", "")...)
		f.normalEndpoint()
		f.tree(base, tree)
		f.file(tree, "plans/goals/backlog.md")
		got := f.verdict(base, commit, "origin", "refs/heads/main")
		if got.Outcome != "goal-free" || got.ExitCode != 0 {
			t.Fatalf("Goal-free parent classified as %+v", got)
		}
	})

	for _, missingGoal := range []bool{false, true} {
		name, goalID, wantCode := "different Goal-Item", "other-goal", "goal-binding-mismatch"
		if missingGoal {
			name, goalID, wantCode = "missing Goal-Item", "", "goal-binding-missing"
		}
		t.Run(name+" under readable chain", func(t *testing.T) {
			f := newHeldPolicyFixture(t)
			base, commit := f.id(), f.id()
			f.rootFile(true)
			f.claimed("other-goal", "m9", "L1", 3)
			f.chain("goal-chain", "ship-widget")
			f.opening(base, commit, heldRawRange(heldCommit{commit, base}))
			f.object(commit, base, "chain binding", heldTrailersFor("m9+L1", goalID, "3", "chain=goal-chain change=fixture")...)
			f.normalEndpoint()
			got := f.verdict(base, commit, "origin", "refs/heads/main")
			assertHeldRefusal(t, got, 1, wantCode, commit, "chain goal-chain")
		})
	}

	t.Run("unreadable chain record leaves ordinary checks", func(t *testing.T) {
		f := newHeldPolicyFixture(t)
		base, commit, tree := f.id(), f.id(), f.id()
		f.capture(tree)
		f.opening(base, commit, heldRawRange(heldCommit{commit, base}))
		f.object(commit, base, "unreadable chain", heldTrailersFor("m9+L1", "", "", "chain=missing-chain change=fixture")...)
		f.normalEndpoint()
		f.tree(base, tree)
		f.file(tree, "plans/goals/backlog.md")
		got := f.verdict(base, commit, "origin", "refs/heads/main")
		assertHeldRefusal(t, got, 1, "goal-binding-missing", commit, "agent landings are goal work")
	})
}

func TestHeldChecksEveryCommitIntroducedByPush(t *testing.T) {
	f := newHeldPolicyFixture(t)
	base, lower, upper, tree := f.id(), f.id(), f.id(), f.id()
	f.abandoned("ship-widget")
	f.claimed("ship-gadget", "m9", "L1", 4)
	f.capture(tree, "records/goals/ship-widget.md", "plans/goals/ship-gadget.md")
	f.opening(base, upper, heldRawRange(heldCommit{lower, base}, heldCommit{upper, lower}))
	f.object(lower, base, "lower moved goal", heldTrailersFor("m9+L1", "ship-widget", "3", "")...)
	f.normalEndpoint()
	f.tree(base, tree)
	f.file(tree, "plans/goals/ship-widget.md")
	f.file(tree, "records/goals/ship-widget.md")
	got := f.verdict(base, upper, "origin", "refs/heads/main")
	assertHeldRefusal(t, got, 1, "goal-item-not-held", lower, "goal ship-widget is abandoned")
	f.opening(lower, upper, heldRawRange(heldCommit{upper, lower}))
	f.goalCommit(upper, lower, tree, "upper held goal", "m9+L1", "ship-gadget", "4")
	got = f.verdict(lower, upper, "origin", "refs/heads/main")
	if got.Outcome != "ok" || got.Commits != 1 {
		t.Fatalf("tip-only range classified as %+v", got)
	}

	human := newHeldPolicyFixture(t)
	humanBase, humanLower, humanUpper, humanTree := human.id(), human.id(), human.id(), human.id()
	human.abandoned("ship-widget")
	human.claimed("ship-gadget", "m9", "L1", 4)
	human.capture(humanTree, "records/goals/ship-widget.md", "plans/goals/ship-gadget.md")
	human.opening(humanBase, humanUpper, heldRawRange(heldCommit{humanLower, humanBase}, heldCommit{humanUpper, humanLower}))
	human.object(humanLower, humanBase, "lower human warning", heldTrailersFor("m9+human", "ship-widget", "3", "")...)
	human.normalEndpoint()
	human.tree(humanBase, humanTree)
	human.file(humanTree, "plans/goals/ship-widget.md")
	human.file(humanTree, "records/goals/ship-widget.md")
	human.goalCommit(humanUpper, humanLower, humanTree, "upper held goal", "m9+L1", "ship-gadget", "4")
	got = human.verdict(humanBase, humanUpper, "origin", "refs/heads/main")
	if got.ExitCode != 0 || got.Commits != 2 || len(got.Warnings) != 1 || !strings.Contains(got.Warnings[0], humanLower) {
		t.Fatalf("human warning did not continue through the range: %+v", got)
	}

	t.Run("non-linear ranges", func(t *testing.T) {
		merge := newHeldPolicyFixture(t)
		mergeBase, first, side, mergeCommit := merge.id(), merge.id(), merge.id(), merge.id()
		merge.opening(mergeBase, mergeCommit, first+" "+mergeBase+"\n"+mergeCommit+" "+first+" "+side+"\n")
		got := merge.verdict(mergeBase, mergeCommit, "origin", "refs/heads/main")
		assertHeldRefusal(t, got, 2, "range-not-linear", mergeCommit, "is a merge")
		merge.opening(side, first, heldRawRange(heldCommit{first, mergeBase}))
		got = merge.verdict(side, first, "origin", "refs/heads/main")
		assertHeldRefusal(t, got, 2, "range-not-linear", first, "is not a first-parent ancestor")
	})

	f.opening(upper, upper, "")
	got = f.verdict(upper, upper, "origin", "refs/heads/main")
	if got.Outcome != "nothing-to-push" || got.ExitCode != 0 || got.Commits != 0 {
		t.Fatalf("empty range classified as %+v", got)
	}

	pass := newHeldPolicyFixture(t)
	passBase, passLower, passUpper, passTree := pass.id(), pass.id(), pass.id(), pass.id()
	pass.claimed("ship-widget", "m9", "L1", 3)
	pass.claimed("ship-gadget", "m9", "L1", 4)
	pass.capture(passTree, "plans/goals/ship-widget.md", "plans/goals/ship-gadget.md")
	pass.opening(passBase, passUpper, heldRawRange(heldCommit{passLower, passBase}, heldCommit{passUpper, passLower}))
	pass.goalCommit(passLower, passBase, passTree, "first held goal", "m9+L1", "ship-widget", "3")
	pass.goalCommit(passUpper, passLower, passTree, "second held goal", "m9+L1", "ship-gadget", "4")
	got = pass.verdict(passBase, passUpper, "origin", "refs/heads/main")
	if got.Outcome != "ok" || got.ExitCode != 0 || got.Commits != 2 {
		t.Fatalf("two-commit held range classified as %+v", got)
	}

	t.Run("Goal-free lower commit does not hide an abandoned goal above it", func(t *testing.T) {
		stack := newHeldPolicyFixture(t)
		stackBase, stackLower, stackUpper, stackTree := stack.id(), stack.id(), stack.id(), stack.id()
		stack.rootFile(true)
		stack.abandoned("ship-widget")
		stack.capture(stackTree, "plans/goals/backlog.md", "records/goals/ship-widget.md")
		stack.opening(stackBase, stackUpper, heldRawRange(heldCommit{stackLower, stackBase}, heldCommit{stackUpper, stackLower}))
		stack.freeCommit(stackLower, stackBase, stackTree, "lower Goal-free commit")
		stack.object(stackUpper, stackLower, "upper abandoned goal", heldTrailersFor("m9+L1", "ship-widget", "3", "")...)
		stack.normalEndpoint()
		stack.tree(stackLower, stackTree)
		stack.file(stackTree, "plans/goals/ship-widget.md")
		stack.file(stackTree, "records/goals/ship-widget.md")
		got := stack.verdict(stackBase, stackUpper, "origin", "refs/heads/main")
		assertHeldRefusal(t, got, 1, "goal-item-not-held", stackUpper, "goal ship-widget is abandoned")
	})

	t.Run("Goal-free lower commit does not hide malformed Machine trailers above it", func(t *testing.T) {
		stack := newHeldPolicyFixture(t)
		stackBase, stackLower, stackUpper, stackTree := stack.id(), stack.id(), stack.id(), stack.id()
		stack.rootFile(true)
		stack.capture(stackTree, "plans/goals/backlog.md")
		stack.opening(stackBase, stackUpper, heldRawRange(heldCommit{stackLower, stackBase}, heldCommit{stackUpper, stackLower}))
		stack.freeCommit(stackLower, stackBase, stackTree, "lower Goal-free commit")
		stack.object(stackUpper, stackLower, "upper malformed actor", "Machine: m9+L1", "Machine: forged+human")
		got := stack.verdict(stackBase, stackUpper, "origin", "refs/heads/main")
		assertHeldRefusal(t, got, 1, "machine-trailer-malformed", stackUpper, "2 Machine trailers")
	})

	t.Run("a later held commit earns an ok range outcome", func(t *testing.T) {
		stack := newHeldPolicyFixture(t)
		stackBase, stackLower, stackUpper, baseTree, lowerTree := stack.id(), stack.id(), stack.id(), stack.id(), stack.id()
		stack.rootFile(true)
		stack.capture(baseTree, "plans/goals/backlog.md")
		stack.rootFile(false)
		stack.claimed("ship-widget", "m9", "L1", 3)
		stack.capture(lowerTree, "plans/goals/backlog.md", "plans/goals/ship-widget.md")
		stack.opening(stackBase, stackUpper, heldRawRange(heldCommit{stackLower, stackBase}, heldCommit{stackUpper, stackLower}))
		stack.freeCommit(stackLower, stackBase, baseTree, "lower Goal-free ledger transition")
		stack.goalCommit(stackUpper, stackLower, lowerTree, "upper held goal", "m9+L1", "ship-widget", "3")
		got := stack.verdict(stackBase, stackUpper, "origin", "refs/heads/main")
		if got.Outcome != "ok" || got.ExitCode != 0 || got.Commits != 2 {
			t.Fatalf("Goal-free lower commit left the range outcome stale: %+v", got)
		}
	})
}

func TestHeldRejectsAMovedBaseWithoutWalkingHistory(t *testing.T) {
	f := newObserveFixture(t)
	tree := f.git("rev-parse", "HEAD^{tree}")
	tip := f.git("rev-parse", "HEAD")
	for i := 0; i < 96; i++ {
		tip = f.git("commit-tree", tree, "-p", tip, "-m", fmt.Sprintf("deep history %d", i))
	}
	movedBase := f.git("commit-tree", tree, "-p", tip, "-m", "origin moved")

	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	wrapperDir := t.TempDir()
	callLog := filepath.Join(wrapperDir, "calls")
	wrapper := filepath.Join(wrapperDir, "git")
	script := "#!/bin/sh\nprintf 'git\\n' >> \"$HELD_GIT_CALLS\"\nexec \"$HELD_REAL_GIT\" \"$@\"\n"
	if err := testexec.WriteFile(wrapper, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HELD_GIT_CALLS", callLog)
	t.Setenv("HELD_REAL_GIT", realGit)
	t.Setenv("PATH", wrapperDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	started := time.Now()
	got := requireHeldVerdict(t, f, movedBase, tip, "origin", "refs/heads/main")
	elapsed := time.Since(started)
	assertHeldRefusal(t, got, 2, "range-not-linear", tip, movedBase+" is not a first-parent ancestor of "+tip)
	calls, err := os.ReadFile(callLog)
	if err != nil {
		t.Fatal(err)
	}
	callCount := len(strings.Fields(string(calls)))
	if callCount != 3 {
		t.Fatalf("moved-base check used %d Git calls; want two resolutions and one range query", callCount)
	}
	t.Logf("moved-base held check over 96 historical commits used %d Git calls in %s", callCount, elapsed)
}

func TestHeldDefaultAdapterReadsConcreteParentLedger(t *testing.T) {
	f := newObserveFixture(t)
	writeHeldGoalForPush(f, "ship-widget", "m9", "L1", 3)
	commitHeldSetup(f, "claim ship-widget", "plans/goals/ship-widget.md")
	base := f.git("rev-parse", "HEAD")
	commit := commitHeldFixture(f, "held parent", "m9+L1", "ship-widget", "3", "")
	got := requireHeldVerdict(t, f, base, commit, "origin", "refs/heads/main")
	if got.Outcome != "ok" || got.ExitCode != 0 || got.Commits != 1 || got.Base != base || got.Commit != commit {
		t.Fatalf("concrete parent ledger classified as %+v", got)
	}
}

func requireHeldVerdict(t *testing.T, f *observeFixture, base, commit, remote, ref string) HeldVerdict {
	t.Helper()
	got, err := Held(f.root, base, commit, remote, ref)
	if err != nil {
		t.Fatalf("Held(%s..%s) returned a plumbing error: %v", base, commit, err)
	}
	return got
}

func assertHeldRefusal(t *testing.T, got HeldVerdict, exit int, code, commit, detail string) {
	t.Helper()
	if got.Outcome != "refused" || got.ExitCode != exit || got.Refusal == nil || got.Refusal.Code != code ||
		got.Refusal.Commit != commit || !strings.Contains(got.Refusal.Detail, detail) {
		t.Fatalf("held refusal does not match %s/%s: verdict=%+v refusal=%+v", code, detail, got, got.Refusal)
	}
}

func writeHeldGoalForPush(f *observeFixture, id, machine, lineage string, revision uint64) {
	f.t.Helper()
	f.writeHeldGoalAtRevision(id, machine, lineage, revision)
}

// moveGoalOutOfClaimedState archives the fixture as abandoned, matching the
// ledger transition whose parent-state recheck this bed exercises.
func moveGoalOutOfClaimedState(f *observeFixture, id string) {
	f.t.Helper()
	_ = os.Remove(filepath.Join(f.root, "plans", "goals", id+".md"))
	const (
		at      = "2026-09-03T08:00:00Z"
		opid    = "01ARZ3NDEKTSV4RRFFQ69G5FAW-m9-00000001"
		actor   = "human:Wido"
		because = "fixture moves the goal out of the claimed state"
	)
	record := goal.RenderFile(&goal.GoalFile{
		Id: id, State: goal.StateAbandoned, Intent: "Fixture ownership.", Origin: goal.OriginMain,
		NextStep: "none",
		OpenedAt: at, Revision: 1,
		Abandoned: &goal.AbandonRecord{By: actor, At: at, Revision: 1, Opid: opid, Because: because},
		History:   []goal.HistoryLine{{At: at, Opid: opid, Verb: "abandon", Actor: actor, Targets: []string{id}, Reason: because, Keep: -1}},
	})
	if _, problems := goal.ParseFile(record); len(problems) != 0 {
		f.t.Fatalf("abandoned goal fixture is invalid: %v", problems)
	}
	f.writeBytes(filepath.Join("records", "goals", id+".md"), record)
}

func writeHeldRoot(f *observeFixture, free bool) {
	f.t.Helper()
	record := &goal.RootRecord{Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "1", SyncMode: goal.SyncRemote, Revision: 1}
	if free {
		record.Free = &goal.FreeRecord{Declared: "2026-09-03T08:00:00Z", Origin: "human", Digest: strings.Repeat("a", 64)}
	}
	f.writeBytes("plans/goals/backlog.md", goal.RenderRoot(record))
}

func commitHeldSetup(f *observeFixture, subject string, paths ...string) {
	f.t.Helper()
	args := append([]string{"add"}, paths...)
	f.git(args...)
	f.git("commit", "-qm", subject)
}

func commitHeldFixture(f *observeFixture, subject, actor, goalID, revision, provenance string) string {
	f.t.Helper()
	trailers := []string{"Machine: " + actor}
	if goalID != "" {
		trailers = append(trailers, "Goal-Item: "+goalID)
	}
	if revision != "" {
		trailers = append(trailers, "Goal-Revision: "+revision)
	}
	if provenance != "" {
		trailers = append(trailers, "Landing-Provenance: "+provenance)
	}
	return commitHeldFixtureTrailers(f, subject, trailers...)
}

func commitHeldFixtureTrailers(f *observeFixture, subject string, trailers ...string) string {
	f.t.Helper()
	path := fmt.Sprintf("records/misc/%s-%d.md", strings.ReplaceAll(subject, " ", "-"), len(strings.Fields(f.git("rev-list", "--all")))+1)
	f.write(path, subject+"\n")
	f.git("add", path)
	args := []string{"commit", "-qm", subject}
	for _, trailer := range trailers {
		args = append(args, "--trailer", trailer)
	}
	f.git(args...)
	return f.git("rev-parse", "HEAD")
}

func removeHeldTreeObject(t *testing.T, f *observeFixture, commit string) {
	t.Helper()
	tree := f.git("rev-parse", commit+"^{tree}")
	gitDir := f.git("rev-parse", "--git-dir")
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(f.root, gitDir)
	}
	if err := os.Remove(filepath.Join(gitDir, "objects", tree[:2], tree[2:])); err != nil {
		t.Fatalf("remove fixture parent tree: %v", err)
	}
}

func shortHeldID(id string) string {
	if len(id) <= 12 {
		return id
	}
	return id[:12]
}

type heldFactCall struct {
	kind, root, first, second string
	data                      []byte
	present                   bool
	endpoint                  goal.Endpoint
}

type heldPolicyFixture struct {
	t       *testing.T
	root    string
	nextID  uint64
	calls   []heldFactCall
	trees   map[string]map[string][]byte
	visited int
}

func newHeldPolicyFixture(t *testing.T) *heldPolicyFixture {
	t.Helper()
	f := &heldPolicyFixture{t: t, root: t.TempDir(), trees: make(map[string]map[string][]byte)}
	t.Cleanup(func() {
		if f.visited != len(f.calls) {
			t.Errorf("held facts left %d ordered calls unused; next: %+v", len(f.calls)-f.visited, f.calls[f.visited])
		}
	})
	return f
}

func (f *heldPolicyFixture) id() string {
	f.nextID++
	if f.nextID >= 1<<48 {
		f.t.Fatal("held fixture exhausted unique ID prefixes")
	}
	return fmt.Sprintf("%012x%028x", f.nextID, f.nextID)
}

func (f *heldPolicyFixture) expect(kind, first, second string, data []byte, present bool, endpoint goal.Endpoint) {
	f.t.Helper()
	f.calls = append(f.calls, heldFactCall{kind: kind, root: f.root, first: first, second: second,
		data: append([]byte(nil), data...), present: present, endpoint: endpoint})
}

func (f *heldPolicyFixture) take(kind, root, first, second string) heldFactCall {
	f.t.Helper()
	if f.visited >= len(f.calls) {
		f.t.Fatalf("unexpected or repeated held fact call: %s %q %q %q", kind, root, first, second)
	}
	want := f.calls[f.visited]
	if want.kind != kind || want.root != root || want.first != first || want.second != second {
		f.t.Fatalf("held fact call %d: got %s %q %q %q, want %+v", f.visited, kind, root, first, second, want)
	}
	f.visited++
	return want
}

func (f *heldPolicyFixture) facts() heldFacts {
	return heldFacts{
		resolveCommit: func(root, revision string) (string, error) {
			return string(f.take("resolve", root, revision, "").data), nil
		},
		rangeBytes: func(root, base, tip string) ([]byte, error) {
			return append([]byte(nil), f.take("range", root, base, tip).data...), nil
		},
		commitObject: func(root, commit string) ([]byte, error) {
			return append([]byte(nil), f.take("object", root, commit, "").data...), nil
		},
		endpoint: func(root string) (goal.Endpoint, error) {
			return f.take("endpoint", root, "", "").endpoint, nil
		},
		parentTree: func(root, parent string) (string, error) {
			return string(f.take("tree", root, parent, "").data), nil
		},
		fileAt: func(root, tree, path string) ([]byte, bool, error) {
			call := f.take("file", root, tree, path)
			return append([]byte(nil), call.data...), call.present, nil
		},
	}
}

func (f *heldPolicyFixture) opening(base, tip, rawRange string) {
	f.expect("resolve", base, "", []byte(base), true, goal.Endpoint{})
	f.expect("resolve", tip, "", []byte(tip), true, goal.Endpoint{})
	if base != tip {
		f.expect("range", base, tip, []byte(rawRange), true, goal.Endpoint{})
	}
}

func heldRawRange(entries ...heldCommit) string {
	var lines []string
	for _, entry := range entries {
		lines = append(lines, entry.commit+" "+entry.parent)
	}
	return strings.Join(lines, "\n") + "\n"
}

func (f *heldPolicyFixture) object(commit, parent, subject string, trailers ...string) {
	message := subject + "\n\n" + strings.Join(trailers, "\n") + "\n"
	raw := "tree " + strings.Repeat("a", 40) + "\nparent " + parent + "\nauthor fixture\n\n" + message
	f.expect("object", commit, "", []byte(raw), true, goal.Endpoint{})
}

func heldTrailersFor(actor, goalID, revision, provenance string) []string {
	lines := []string{"Machine: " + actor}
	if goalID != "" {
		lines = append(lines, "Goal-Item: "+goalID)
	}
	if revision != "" {
		lines = append(lines, "Goal-Revision: "+revision)
	}
	if provenance != "" {
		lines = append(lines, "Landing-Provenance: "+provenance)
	}
	return lines
}

func (f *heldPolicyFixture) endpoint(remote, branch string) {
	f.expect("endpoint", "", "", nil, true, goal.Endpoint{Root: f.root, Remote: remote, Branch: branch})
}

func (f *heldPolicyFixture) normalEndpoint() { f.endpoint("origin", "refs/heads/main") }

func (f *heldPolicyFixture) goalCommit(commit, parent, tree, subject, actor, goalID, revision string) {
	f.object(commit, parent, subject, heldTrailersFor(actor, goalID, revision, "")...)
	f.normalEndpoint()
	f.tree(parent, tree)
	f.file(tree, "plans/goals/"+goalID+".md")
}

func (f *heldPolicyFixture) freeCommit(commit, parent, tree, subject string) {
	f.object(commit, parent, subject, heldTrailersFor("m9+L1", "", "", "")...)
	f.normalEndpoint()
	f.tree(parent, tree)
	f.file(tree, "plans/goals/backlog.md")
}

func (f *heldPolicyFixture) tree(parent, tree string) {
	f.expect("tree", parent, "", []byte(tree), true, goal.Endpoint{})
}

func (f *heldPolicyFixture) write(path string, data []byte) {
	f.t.Helper()
	abs := filepath.Join(f.root, path)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(abs, data, 0o644); err != nil {
		f.t.Fatal(err)
	}
}

func (f *heldPolicyFixture) capture(tree string, paths ...string) {
	f.t.Helper()
	files := make(map[string][]byte)
	for _, path := range paths {
		data, err := os.ReadFile(filepath.Join(f.root, path))
		if err != nil {
			f.t.Fatal(err)
		}
		files[path] = append([]byte(nil), data...)
	}
	f.trees[tree] = files
}

func (f *heldPolicyFixture) file(tree, path string) {
	f.t.Helper()
	files, ok := f.trees[tree]
	if !ok {
		f.t.Fatalf("tree %s was not captured", tree)
	}
	data, present := files[path]
	f.expect("file", tree, path, data, present, goal.Endpoint{})
}

func (f *heldPolicyFixture) verdict(base, tip, remote, ref string) HeldVerdict {
	f.t.Helper()
	got, err := heldWithFacts(f.root, base, tip, remote, ref, f.facts())
	if err != nil {
		f.t.Fatalf("heldWithFacts(%s..%s): %v", base, tip, err)
	}
	return got
}

func (f *heldPolicyFixture) claimed(id, machine, lineage string, revision uint64) {
	f.t.Helper()
	history := make([]goal.HistoryLine, revision)
	for index := range history {
		at := fmt.Sprintf("2026-09-03T08:%02d:00Z", index)
		history[index] = goal.HistoryLine{At: at,
			Opid: fmt.Sprintf("01ARZ3NDEKTSV4RRFFQ69G5FAW-%s-%08x", machine, index+1),
			Verb: "edit", Actor: machine + "+" + lineage, Targets: []string{id}, Keep: -1}
	}
	history[revision-1].Verb = "claim"
	record := goal.RenderFile(&goal.GoalFile{Id: id, State: goal.StateClaimed,
		Intent: "Fixture ownership.", Origin: goal.OriginMain, NextStep: "Exercise landing binding.",
		OpenedAt: "2026-09-03T08:00:00Z", Revision: revision,
		Claimed: &goal.ClaimRecord{Machine: machine, Lineage: lineage, At: history[revision-1].At,
			Revision: revision, AccountingRevision: revision}, History: history})
	if _, problems := goal.ParseFile(record); len(problems) != 0 {
		f.t.Fatalf("invalid claimed goal: %v", problems)
	}
	f.write("plans/goals/"+id+".md", record)
}

func (f *heldPolicyFixture) abandoned(id string) {
	f.t.Helper()
	const at = "2026-09-03T08:00:00Z"
	const opid = "01ARZ3NDEKTSV4RRFFQ69G5FAW-m9-00000001"
	const actor = "human:Wido"
	const because = "fixture moves the goal out of the claimed state"
	record := goal.RenderFile(&goal.GoalFile{Id: id, State: goal.StateAbandoned,
		Intent: "Fixture ownership.", Origin: goal.OriginMain, NextStep: "none", OpenedAt: at, Revision: 1,
		Abandoned: &goal.AbandonRecord{By: actor, At: at, Revision: 1, Opid: opid, Because: because},
		History: []goal.HistoryLine{{At: at, Opid: opid, Verb: "abandon", Actor: actor,
			Targets: []string{id}, Reason: because, Keep: -1}}})
	if _, problems := goal.ParseFile(record); len(problems) != 0 {
		f.t.Fatalf("invalid abandoned goal: %v", problems)
	}
	f.write("records/goals/"+id+".md", record)
}

func (f *heldPolicyFixture) rootFile(free bool) {
	record := &goal.RootRecord{Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "1", SyncMode: goal.SyncRemote, Revision: 1}
	if free {
		record.Free = &goal.FreeRecord{Declared: "2026-09-03T08:00:00Z", Origin: "human", Digest: strings.Repeat("a", 64)}
	}
	f.write("plans/goals/backlog.md", goal.RenderRoot(record))
}

func (f *heldPolicyFixture) chain(chain, goalID string) {
	data, err := json.Marshal(map[string]any{"jobId": chain, "parentJob": nil, "goalId": goalID, "goalRevision": 3})
	if err != nil {
		f.t.Fatal(err)
	}
	f.write("artifacts/agents/jobs/"+chain+".json", append(data, '\n'))
}
