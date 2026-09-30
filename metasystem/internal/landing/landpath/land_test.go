package landpath

import (
	"fmt"
	"io"
	"strings"
	"testing"
)

// TestLandPathspecsStageCommitRebaseProvePushTransport is the ordinary
// landing: named paths are staged, the receipt line is judged, the commit
// boundary records the change, and the landing rebases onto origin, re-checks
// held goals and proof, pushes once and mirrors transport, in that order.
func TestLandPathspecsStageCommitRebaseProvePushTransport(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.git.stagedEmpty = true
	b.git.on("add --", func(GitCall) GitResult { b.git.stagedEmpty = false; return ok("") })
	b.expect(b.land(LandRequest{Pathspecs: []string{"a.go"}, Goal: "g1", GoalSet: true}), 0)
	want := []string{"brain-fence", "drift empty=false", "receipt-line goal=g1", "verify tree=t1 goal=g1", "observe",
		"drift empty=true", "advance refs/remotes/origin/main", "held base=refs/remotes/origin/main", "verify tree=t1 goal=g1",
		"notify g1 c1", "transport main"}
	position := 0
	for _, call := range b.log.calls {
		if position < len(want) && strings.HasPrefix(call, want[position]) {
			position++
		}
	}
	if position != len(want) {
		t.Fatalf("owner order: reached %d of %v in %v", position, want, b.log.calls)
	}
	if len(b.git.called("add -- a.go")) != 1 || len(b.git.called("push --porcelain origin refs/heads/main:refs/heads/main")) != 1 {
		t.Fatalf("git: %v", b.git.calls)
	}
	for _, step := range []string{"step: verify checks\n  ok", "step: stage caller paths", "step: receipt line for the landing",
		"step: commit", "step: push origin (attempt 1 of 3)", "step: sync transport"} {
		if !strings.Contains(b.stdout.String(), step) {
			t.Fatalf("stdout lacks %q:\n%s", step, b.stdout.String())
		}
	}
}

// TestLandRetriesAMovingOriginTwiceThenFails: a rejected push because origin
// moved is fetched, rebased, held-checked and re-proved before each retry; a
// third rejection fails the step with its retained log.
func TestLandRetriesAMovingOriginTwiceThenFails(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	pushes := 0
	b.git.on("push --porcelain", func(GitCall) GitResult {
		pushes++
		if pushes < 3 {
			return failed(1, " ! [rejected] main -> main (fetch first)\n")
		}
		return ok("")
	})
	b.expect(b.land(LandRequest{StagedOnly: true}), 0, "origin moved during push; fetching and rebasing before retry 2 of 3",
		"origin moved during push; fetching and rebasing before retry 3 of 3")
	if pushes != 3 || b.log.count("advance") != 3 || b.log.count("held") != 3 {
		t.Fatalf("pushes=%d log=%v", pushes, b.log.calls)
	}
	b = newBed(t)
	b.git.on("push --porcelain", func(GitCall) GitResult { return failed(1, " ! [rejected] main -> main (non-fast-forward)\n") })
	b.expect(b.land(LandRequest{StagedOnly: true}), 1, "step failed: push origin (attempt 3 of 3) (exit 1)", "output retained: land.log")
	if b.log.has("transport") || b.log.has("notify") {
		t.Fatalf("a failed push went on: %v", b.log.calls)
	}
	b = newBed(t)
	b.git.on("push --porcelain", func(GitCall) GitResult { return failed(1, "remote: permission denied\n") })
	b.expect(b.land(LandRequest{StagedOnly: true}), 1, "step failed: push origin (attempt 1 of 3) (exit 1)", "remote: permission denied")
}

// TestLandValidatesItsDeclarations covers every refusal the landing makes
// before its first step.
func TestLandValidatesItsDeclarations(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		request LandRequest
		text    string
	}{
		{"staged and paths", LandRequest{StagedOnly: true, Pathspecs: []string{"a"}}, "--staged lands what is staged, so it takes no --path"},
		{"neither", LandRequest{}, "nothing to land: no --path and no --staged"},
		{"tests and receipt", LandRequest{StagedOnly: true, Tests: "true", TestReceipt: "r", DirectFix: "tier-1"}, "--tests and --test-receipt don't go together"},
		{"receipt alone", LandRequest{StagedOnly: true, TestReceipt: "r"}, "--test-receipt goes with --chain or --direct-fix tier-1"},
		{"carried without goal", LandRequest{Carried: "op"}, "an exception landing names its goal"},
		{"recert without chain", LandRequest{StagedOnly: true, Recertification: "r"}, "--recertification goes with --chain"},
		{"recert with other fix", LandRequest{StagedOnly: true, Chain: "j1", Recertification: "r", DirectFix: "exact-revert"}, "--recertification combines only with --direct-fix register-carriage"},
		{"recert without receipt", LandRequest{StagedOnly: true, Chain: "j1", Recertification: "r"}, "a recertified landing needs a fresh test receipt"},
		{"tier-1 incomplete", LandRequest{StagedOnly: true, DirectFix: "tier-1", Goal: "g", GoalSet: true}, "--direct-fix tier-1 needs a goal, --root-job, and --test-receipt or --tests"},
		{"root job outside tier-1", LandRequest{StagedOnly: true, RootJob: "j"}, "--root-job and --tests go only with --direct-fix tier-1"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b := newBed(t)
			b.expect(b.land(c.request), 2, c.text)
			if strings.Contains(b.stdout.String(), "step: ") {
				t.Fatalf("a refused landing ran a step:\n%s", b.stdout.String())
			}
		})
	}
	b := newBed(t)
	b.owners.JobGateWidth = func(string, string) string { return "full" }
	b.expect(b.land(LandRequest{StagedOnly: true, Chain: "j1"}), 2, "chain j1 needs its full test run's receipt, so nothing was landed\nrun: metasystem test run")
	b = newBed(t)
	b.expect(b.land(LandRequest{StagedOnly: true, MessageFile: "/nonexistent"}), 2, "the commit message file can't be read, so nothing was landed", "message file: /nonexistent")
	b = newBed(t)
	b.owners.BrainFence = func(string, string) (string, error) { return "land refused: this checkout is declared the brain", nil }
	b.expect(b.land(LandRequest{StagedOnly: true}), 2, "declared the brain")
}

// TestLandStepFailureSpillsTheLogAndStops: a failed step prints its last
// forty lines, names its retained log, and stops the landing with its status.
func TestLandStepFailureSpillsTheLogAndStops(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.owners.Advance = func(_, _ string, stdout, _ io.Writer) int {
		for i := 1; i <= 50; i++ {
			fmt.Fprintf(stdout, "advance line %d\n", i)
		}
		return 7
	}
	b.expect(b.land(LandRequest{StagedOnly: true}), 7, "step failed: rebase onto origin/main (exit 7)", "advance line 50", "output retained: land.log")
	if strings.Contains(b.stderr.String(), "advance line 10\n") {
		t.Fatalf("more than forty lines printed:\n%s", b.stderr.String())
	}
	b = newBed(t)
	b.owners.OutputSpill = func(string, string, string, []byte) (string, error) { return "", fmt.Errorf("disk full") }
	b.owners.Advance = func(string, string, io.Writer, io.Writer) int { return 2 }
	b.expect(b.land(LandRequest{StagedOnly: true}), 2, "full step log not retained: disk full")
}

// TestLandCommitOnlyStopsAfterTheCommit (U11b): a change bound for the
// landing lane runs the seat's steps up to the commit (checks, staging, the
// receipt line, the commit boundary and its proof, the clean tree) and
// nothing after it: no fetch, rebase, push or transport, which the lane owns.
func TestLandCommitOnlyStopsAfterTheCommit(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.git.stagedEmpty = true
	b.git.on("add --", func(GitCall) GitResult { b.git.stagedEmpty = false; return ok("") })
	b.expect(b.land(LandRequest{Pathspecs: []string{"a.go"}, CommitOnly: true}), 0)
	for _, call := range []string{"advance", "held", "transport", "notify"} {
		if b.log.has(call) {
			t.Fatalf("a lane-bound change ran %s: %v", call, b.log.calls)
		}
	}
	if len(b.git.called("push")) != 0 || len(b.git.called("fetch")) != 0 || !b.log.has("drift empty=true") || !strings.Contains(b.stdout.String(), "step: commit") {
		t.Fatalf("git=%v log=%v stdout=%s", b.git.calls, b.log.calls, b.stdout.String())
	}
}
