package landing

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

func TestHeldRecheckReadsTheParent(t *testing.T) {
	f := newObserveFixture(t)
	writeHeldGoalForPush(f, "ship-widget", "m9", "L1", 3)
	commitHeldSetup(f, "claim ship-widget", "plans/goals/ship-widget.md")
	base := f.git("rev-parse", "HEAD")
	commit := commitHeldFixture(f, "held parent", "m9+L1", "ship-widget", "3", "")
	got := requireHeldVerdict(t, f, base, commit, "origin", "refs/heads/main")
	if got.Outcome != "ok" || got.ExitCode != 0 || got.Commits != 1 || got.Base != base || got.Commit != commit {
		t.Fatalf("matching parent classified as %+v", got)
	}

	abandoned := newObserveFixture(t)
	moveGoalOutOfClaimedState(abandoned, "ship-widget")
	commitHeldSetup(abandoned, "abandon ship-widget", "records/goals/ship-widget.md")
	abandonedBase := abandoned.git("rev-parse", "HEAD")
	abandonedCommit := commitHeldFixture(abandoned, "held abandoned parent", "m9+L1", "ship-widget", "3", "")
	got = requireHeldVerdict(t, abandoned, abandonedBase, abandonedCommit, "origin", "refs/heads/main")
	assertHeldRefusal(t, got, 1, "goal-item-not-held", abandonedCommit, "goal ship-widget is abandoned at "+shortHeldID(abandonedBase))

	moved := newObserveFixture(t)
	writeHeldGoalForPush(moved, "ship-widget", "m9", "L1", 5)
	commitHeldSetup(moved, "claim ship-widget at revision five", "plans/goals/ship-widget.md")
	movedBase := moved.git("rev-parse", "HEAD")
	movedCommit := commitHeldFixture(moved, "held moved revision", "m9+L1", "ship-widget", "3", "")
	got = requireHeldVerdict(t, moved, movedBase, movedCommit, "origin", "refs/heads/main")
	assertHeldRefusal(t, got, 1, "goal-revision-moved", movedCommit, "claimed at revision 5")

	human := newObserveFixture(t)
	writeHeldGoalForPush(human, "ship-widget", "m9", "L1", 5)
	commitHeldSetup(human, "claim ship-widget for human warning", "plans/goals/ship-widget.md")
	humanBase := human.git("rev-parse", "HEAD")
	humanCommit := commitHeldFixture(human, "held human warning", "m9+human", "ship-widget", "3", "")
	got = requireHeldVerdict(t, human, humanBase, humanCommit, "origin", "refs/heads/main")
	if got.ExitCode != 0 || got.Outcome != "ok" || len(got.Warnings) != 1 || !strings.Contains(got.Warnings[0], "held: warning goal-revision-moved: "+humanCommit) {
		t.Fatalf("human revision mismatch was not softened: %+v", got)
	}

	unbound := newObserveFixture(t)
	writeHeldGoalForPush(unbound, "ship-widget", "m9", "L1", 3)
	commitHeldSetup(unbound, "claim ship-widget for unbound", "plans/goals/ship-widget.md")
	unboundBase := unbound.git("rev-parse", "HEAD")
	unboundCommit := commitHeldFixture(unbound, "held unbound revision", "m9+L1", "ship-widget", "", "")
	got = requireHeldVerdict(t, unbound, unboundBase, unboundCommit, "origin", "refs/heads/main")
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
			f := newObserveFixture(t)
			writeHeldGoalForPush(f, "ship-widget", "m9", "L1", 3)
			commitHeldSetup(f, "claim ship-widget", "plans/goals/ship-widget.md")
			base := f.git("rev-parse", "HEAD")
			commit := commitHeldFixtureTrailers(f, "malformed trailer", test.trailers...)
			removeHeldTreeObject(t, f, base)
			got := requireHeldVerdict(t, f, base, commit, "origin", "refs/heads/main")
			assertHeldRefusal(t, got, 1, "machine-trailer-malformed", commit, test.want)
		})
	}
}

func TestHeldChecksTheGoalEndpoint(t *testing.T) {
	newBed := func(t *testing.T, actor string) (*observeFixture, string, string) {
		f := newObserveFixture(t)
		writeHeldGoalForPush(f, "ship-widget", "m9", "L1", 3)
		commitHeldSetup(f, "claim ship-widget", "plans/goals/ship-widget.md")
		base := f.git("rev-parse", "HEAD")
		return f, base, commitHeldFixture(f, "endpoint", actor, "ship-widget", "3", "")
	}

	for _, test := range []struct {
		name, remote, ref string
		configure         func(*observeFixture)
	}{
		{name: "other remote", remote: "transport", ref: "refs/heads/main"},
		{name: "other ref", remote: "origin", ref: "refs/heads/paper"},
		{name: "local endpoint", remote: "origin", ref: "refs/heads/main", configure: func(f *observeFixture) { f.git("config", "goal.sync-remote", "local") }},
	} {
		t.Run(test.name, func(t *testing.T) {
			f, base, commit := newBed(t, "m9+L1")
			if test.configure != nil {
				test.configure(f)
			}
			got := requireHeldVerdict(t, f, base, commit, test.remote, test.ref)
			assertHeldRefusal(t, got, 1, "endpoint-mismatch", commit, "this landing pushes "+test.remote+" "+test.ref)
		})
	}

	t.Run("configured branch", func(t *testing.T) {
		f, base, commit := newBed(t, "m9+L1")
		f.git("config", "goal.sync-branch", "refs/heads/other")
		got := requireHeldVerdict(t, f, base, commit, "origin", "refs/heads/other")
		if got.Outcome != "ok" || got.ExitCode != 0 {
			t.Fatalf("configured endpoint classified as %+v", got)
		}
	})

	t.Run("human warning", func(t *testing.T) {
		f, base, commit := newBed(t, "m9+human")
		got := requireHeldVerdict(t, f, base, commit, "transport", "refs/heads/main")
		if got.ExitCode != 0 || len(got.Warnings) != 1 || !strings.Contains(got.Warnings[0], "held: warning endpoint-mismatch: "+commit) {
			t.Fatalf("human endpoint mismatch was not softened: %+v", got)
		}
	})
}

func TestHeldRequiresAGoalBindingFromANonHumanActor(t *testing.T) {
	t.Run("ordinary ledger", func(t *testing.T) {
		f := newObserveFixture(t)
		base := f.git("rev-parse", "HEAD")
		commit := commitHeldFixture(f, "missing goal", "m9+L1", "", "", "")
		got := requireHeldVerdict(t, f, base, commit, "origin", "refs/heads/main")
		assertHeldRefusal(t, got, 1, "goal-binding-missing", commit, "agent landings are goal work")
	})

	t.Run("Goal-free ledger", func(t *testing.T) {
		f := newObserveFixture(t)
		writeHeldRoot(f, true)
		commitHeldSetup(f, "declare Goal-free", "plans/goals/backlog.md")
		base := f.git("rev-parse", "HEAD")
		commit := commitHeldFixture(f, "goal free", "m9+L1", "", "", "")
		got := requireHeldVerdict(t, f, base, commit, "origin", "refs/heads/main")
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
			f := newObserveFixture(t)
			writeHeldRoot(f, true)
			writeHeldGoalForPush(f, "other-goal", "m9", "L1", 3)
			commitHeldSetup(f, "seed chain binding", "plans/goals")
			f.writeChainRecord("goal-chain", map[string]any{"jobId": "goal-chain", "parentJob": nil, "goalId": "ship-widget", "goalRevision": 3})
			base := f.git("rev-parse", "HEAD")
			commit := commitHeldFixture(f, "chain binding", "m9+L1", goalID, "3", "chain=goal-chain change=fixture")
			got := requireHeldVerdict(t, f, base, commit, "origin", "refs/heads/main")
			assertHeldRefusal(t, got, 1, wantCode, commit, "chain goal-chain")
		})
	}

	t.Run("unreadable chain record leaves ordinary checks", func(t *testing.T) {
		f := newObserveFixture(t)
		base := f.git("rev-parse", "HEAD")
		commit := commitHeldFixture(f, "unreadable chain", "m9+L1", "", "", "chain=missing-chain change=fixture")
		got := requireHeldVerdict(t, f, base, commit, "origin", "refs/heads/main")
		assertHeldRefusal(t, got, 1, "goal-binding-missing", commit, "agent landings are goal work")
	})
}

func TestHeldChecksEveryCommitIntroducedByPush(t *testing.T) {
	f := newObserveFixture(t)
	moveGoalOutOfClaimedState(f, "ship-widget")
	writeHeldGoalForPush(f, "ship-gadget", "m9", "L1", 4)
	commitHeldSetup(f, "seed moved and held goals", "records/goals/ship-widget.md", "plans/goals/ship-gadget.md")
	base := f.git("rev-parse", "HEAD")
	lower := commitHeldFixture(f, "lower moved goal", "m9+L1", "ship-widget", "3", "")
	upper := commitHeldFixture(f, "upper held goal", "m9+L1", "ship-gadget", "4", "")
	got := requireHeldVerdict(t, f, base, upper, "origin", "refs/heads/main")
	assertHeldRefusal(t, got, 1, "goal-item-not-held", lower, "goal ship-widget is abandoned")
	got = requireHeldVerdict(t, f, lower, upper, "origin", "refs/heads/main")
	if got.Outcome != "ok" || got.Commits != 1 {
		t.Fatalf("tip-only range classified as %+v", got)
	}

	human := newObserveFixture(t)
	moveGoalOutOfClaimedState(human, "ship-widget")
	writeHeldGoalForPush(human, "ship-gadget", "m9", "L1", 4)
	commitHeldSetup(human, "seed warning stack", "records/goals/ship-widget.md", "plans/goals/ship-gadget.md")
	humanBase := human.git("rev-parse", "HEAD")
	humanLower := commitHeldFixture(human, "lower human warning", "m9+human", "ship-widget", "3", "")
	humanUpper := commitHeldFixture(human, "upper held goal", "m9+L1", "ship-gadget", "4", "")
	got = requireHeldVerdict(t, human, humanBase, humanUpper, "origin", "refs/heads/main")
	if got.ExitCode != 0 || got.Commits != 2 || len(got.Warnings) != 1 || !strings.Contains(got.Warnings[0], humanLower) {
		t.Fatalf("human warning did not continue through the range: %+v", got)
	}

	t.Run("non-linear ranges", func(t *testing.T) {
		merge := newObserveFixture(t)
		writeHeldGoalForPush(merge, "ship-widget", "m9", "L1", 3)
		commitHeldSetup(merge, "seed merge bed", "plans/goals/ship-widget.md")
		mergeBase := merge.git("rev-parse", "HEAD")
		first := commitHeldFixture(merge, "first parent landing", "m9+L1", "ship-widget", "3", "")
		tree := merge.git("rev-parse", "HEAD^{tree}")
		side := merge.git("commit-tree", tree, "-p", mergeBase, "-m", "side")
		mergeCommit := merge.git("commit-tree", tree, "-p", first, "-p", side, "-m", "merge")
		got := requireHeldVerdict(t, merge, mergeBase, mergeCommit, "origin", "refs/heads/main")
		assertHeldRefusal(t, got, 2, "range-not-linear", mergeCommit, "is a merge")
		got = requireHeldVerdict(t, merge, side, first, "origin", "refs/heads/main")
		assertHeldRefusal(t, got, 2, "range-not-linear", first, "is not a first-parent ancestor")
	})

	got = requireHeldVerdict(t, f, upper, upper, "origin", "refs/heads/main")
	if got.Outcome != "nothing-to-push" || got.ExitCode != 0 || got.Commits != 0 {
		t.Fatalf("empty range classified as %+v", got)
	}

	pass := newObserveFixture(t)
	writeHeldGoalForPush(pass, "ship-widget", "m9", "L1", 3)
	writeHeldGoalForPush(pass, "ship-gadget", "m9", "L1", 4)
	commitHeldSetup(pass, "seed two held goals", "plans/goals")
	passBase := pass.git("rev-parse", "HEAD")
	commitHeldFixture(pass, "first held goal", "m9+L1", "ship-widget", "3", "")
	passUpper := commitHeldFixture(pass, "second held goal", "m9+L1", "ship-gadget", "4", "")
	got = requireHeldVerdict(t, pass, passBase, passUpper, "origin", "refs/heads/main")
	if got.Outcome != "ok" || got.ExitCode != 0 || got.Commits != 2 {
		t.Fatalf("two-commit held range classified as %+v", got)
	}

	t.Run("Goal-free lower commit does not hide an abandoned goal above it", func(t *testing.T) {
		stack := newObserveFixture(t)
		writeHeldRoot(stack, true)
		moveGoalOutOfClaimedState(stack, "ship-widget")
		commitHeldSetup(stack, "seed Goal-free abandoned stack", "plans/goals/backlog.md", "records/goals/ship-widget.md")
		stackBase := stack.git("rev-parse", "HEAD")
		commitHeldFixture(stack, "lower Goal-free commit", "m9+L1", "", "", "")
		stackUpper := commitHeldFixture(stack, "upper abandoned goal", "m9+L1", "ship-widget", "3", "")

		got := requireHeldVerdict(t, stack, stackBase, stackUpper, "origin", "refs/heads/main")
		assertHeldRefusal(t, got, 1, "goal-item-not-held", stackUpper, "goal ship-widget is abandoned")
	})

	t.Run("Goal-free lower commit does not hide malformed Machine trailers above it", func(t *testing.T) {
		stack := newObserveFixture(t)
		writeHeldRoot(stack, true)
		commitHeldSetup(stack, "seed Goal-free malformed stack", "plans/goals/backlog.md")
		stackBase := stack.git("rev-parse", "HEAD")
		commitHeldFixture(stack, "lower Goal-free commit", "m9+L1", "", "", "")
		stackUpper := commitHeldFixtureTrailers(stack, "upper malformed actor", "Machine: m9+L1", "Machine: forged+human")

		got := requireHeldVerdict(t, stack, stackBase, stackUpper, "origin", "refs/heads/main")
		assertHeldRefusal(t, got, 1, "machine-trailer-malformed", stackUpper, "2 Machine trailers")
	})

	t.Run("a later held commit earns an ok range outcome", func(t *testing.T) {
		stack := newObserveFixture(t)
		writeHeldRoot(stack, true)
		commitHeldSetup(stack, "seed Goal-free range", "plans/goals/backlog.md")
		stackBase := stack.git("rev-parse", "HEAD")

		writeHeldRoot(stack, false)
		writeHeldGoalForPush(stack, "ship-widget", "m9", "L1", 3)
		stack.git("add", "plans/goals/backlog.md", "plans/goals/ship-widget.md")
		stack.git("commit", "-qm", "lower Goal-free ledger transition", "--trailer", "Machine: m9+L1")
		stackUpper := commitHeldFixture(stack, "upper held goal", "m9+L1", "ship-widget", "3", "")

		got := requireHeldVerdict(t, stack, stackBase, stackUpper, "origin", "refs/heads/main")
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
	if err := os.WriteFile(wrapper, []byte(script), 0o755); err != nil {
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
