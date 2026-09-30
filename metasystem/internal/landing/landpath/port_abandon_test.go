package landpath

// Ports of the abandonment-route scenarios of scripts/agents/land-fixtures.sh
// (abandonment-route-normal, -retry, -wrapper, -commit-push-range,
// -commit-push-rejected, -stack, -positive, -recertified). The bash bed
// moved the goal out of its claimed state from a peer clone inside a git
// shim and waited on real processes; here the move is one flag the scripted
// Git flips at the same call (the fetch or the push), and the held owner and
// the evaluator read it. Nothing waits, polls or reads a clock.

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
)

const (
	abandonGoal    = "ship-widget"
	abandonLineage = "fixture-lineage"
	abandonActor   = "brain-leg+" + abandonLineage
)

// abandonBed is an agent landing of ship-widget whose goal a peer abandons
// at a scripted git call.
type abandonBed struct {
	*bed
	abandoned bool
	pushes    int
	heldCalls []string
	parks     []ParkRequest
}

func newAbandonBed(t *testing.T) *abandonBed {
	t.Helper()
	a := &abandonBed{bed: newBed(t)}
	a.epoch = epochOf(5)
	a.git.machine = "brain-leg"
	a.git.stagedEmpty = true
	a.git.on("add --", func(GitCall) GitResult { a.git.stagedEmpty = false; return ok("") })
	a.git.on("push", func(call GitCall) GitResult {
		a.pushes++
		return ok("")
	})
	// The held owner reads the parent the landing is rebased on: once the
	// peer abandoned the goal, the landing's commit is no longer held.
	a.owners.Held = func(_, base, commit, _, _ string, stdout, _ io.Writer) int {
		a.heldCalls = append(a.heldCalls, base+" "+commit)
		a.log.add("held base=%s", base)
		if a.abandoned {
			fmt.Fprintf(stdout, "held refused: goal-item-not-held: %s: goal %s is abandoned at %s\n", a.git.head, abandonGoal, base)
			return 1
		}
		fmt.Fprintf(stdout, "held: ok 1 commit(s) above %s\n", base)
		return 0
	}
	a.owners.Park = func(request ParkRequest) (string, error) {
		a.parks = append(a.parks, request)
		a.log.add("park reason=%s", request.Reason)
		return "state=parked\nreason=" + request.Reason, nil
	}
	return a
}

// abandonAt makes the peer's abandonment land at the first git call whose
// arguments start with prefix, before that call answers.
func (a *abandonBed) abandonAt(prefix string, answer func(GitCall) GitResult) {
	a.git.on(prefix, func(call GitCall) GitResult {
		a.abandoned = true
		return answer(call)
	})
}

func (a *abandonBed) landRecord(record string) int {
	a.t.Helper()
	return a.land(LandRequest{Pathspecs: []string{record}, Goal: abandonGoal, GoalSet: true,
		DirectFix: "register-carriage", SkipTransport: true, OwnerLineage: abandonLineage})
}

func (a *abandonBed) expectNoPush() {
	a.t.Helper()
	if a.pushes != 0 || a.log.has("notify") {
		a.t.Fatalf("an abandoned landing reached push: pushes=%d log=%v", a.pushes, a.log.calls)
	}
}

// TestAbandonRouteNormalRefusesAtTheFetchedParent ports
// abandonment-route-normal: the goal is abandoned on origin between the
// commit and the fetch; the landing rebases, the held check at the rebased
// base refuses, and nothing is pushed.
func TestAbandonRouteNormalRefusesAtTheFetchedParent(t *testing.T) {
	t.Parallel()
	a := newAbandonBed(t)
	a.abandonAt("fetch", func(GitCall) GitResult { return ok("") })
	status := a.landRecord("records/misc/abandonment-normal.md")
	a.expect(status, 1, "step: rebase onto origin/main", "step: goal held at the rebased base",
		"held refused: goal-item-not-held:", "goal ship-widget is abandoned at ")
	a.expectNoPush()
	if a.git.commits != 1 || len(a.heldCalls) != 1 || a.heldCalls[0] != "refs/remotes/origin/main HEAD" {
		t.Fatalf("commits=%d held=%v", a.git.commits, a.heldCalls)
	}
}

// TestAbandonRouteRetryRechecksTheNewlyFetchedParent ports
// abandonment-route-retry: the abandonment reaches origin while the first
// push is in flight, so the push is rejected as a moving origin; the retry
// fetches, rebases and re-runs held, which refuses before a second push.
func TestAbandonRouteRetryRechecksTheNewlyFetchedParent(t *testing.T) {
	t.Parallel()
	a := newAbandonBed(t)
	a.abandonAt("push", func(GitCall) GitResult {
		a.pushes++
		return failed(1, "To origin\n ! [rejected] main -> main (fetch first)\n")
	})
	status := a.landRecord("records/misc/abandonment-retry.md")
	a.expect(status, 1, "step: push origin (attempt 1 of 3)", "step: fetch origin after push attempt 1",
		"step: rebase onto origin/main after push attempt 1", "step: goal held at the rebased base",
		"held refused: goal-item-not-held:", "goal ship-widget is abandoned at ")
	if a.pushes != 1 || strings.Contains(a.stdout.String(), "push origin (attempt 2 of 3)") {
		t.Fatalf("the retry pushed again: pushes=%d\n%s", a.pushes, a.stdout.String())
	}
	if len(a.heldCalls) != 2 {
		t.Fatalf("held ran %d times, want the pre-push check and the retry check", len(a.heldCalls))
	}
}

// TestAbandonRouteWrapperRefusals ports abandonment-route-wrapper: an agent
// commit refuses a typed Machine trailer and an empty owner lineage before
// any effect, and a goal-less agent landing is refused with the evaluator's
// goal-binding-missing cause.
func TestAbandonRouteWrapperRefusals(t *testing.T) {
	t.Parallel()
	a := newAbandonBed(t)
	a.writeMessage("x\n\nMachine: forged+human\n")
	a.expect(a.commit(CommitRequest{OwnerLineage: abandonLineage}), 2, "the commit message types a line the landing adds itself (Machine:)")
	if len(a.git.called("commit")) != 0 || a.log.has("token") {
		t.Fatalf("a typed Machine trailer reached an effect: %v %v", a.git.calls, a.log.calls)
	}

	a = newAbandonBed(t)
	a.expect(a.commit(CommitRequest{}), 2,
		"this agent shell doesn't say which session it is, so nothing was committed\nneeded first: export METASYSTEM_OWNER_LINEAGE in the session's shell, then repeat this command\n")
	if len(a.git.called("commit")) != 0 {
		t.Fatal("an agent commit without lineage was recorded")
	}

	a = newAbandonBed(t)
	var observed ObserveRequest
	live := a.owners.Live
	a.owners.Live = func() Judge {
		judge := live()
		judge.Observe = func(request ObserveRequest) (landing.Observation, int) {
			observed = request
			return landing.Observation{Mode: "refuse", RefusesAgent: true, Code: "goal-binding-missing",
				Provenance: "none change=x", VerdictTrailer: "would-refuse code=goal-binding-missing"}, 0
		}
		return judge
	}
	status := a.land(LandRequest{Pathspecs: []string{"records/misc/abandonment-wrapper.md"}, DirectFix: "register-carriage",
		SkipTransport: true, OwnerLineage: abandonLineage})
	a.expect(status, 1, "this change names no goal, and landings here need one")
	if observed.Goal != "" || observed.DirectFix != "register-carriage" || observed.Actor != abandonActor || len(a.git.called("commit")) != 0 {
		t.Fatalf("goal-less landing observed %+v, commits %v", observed, a.git.called("commit"))
	}
	a.expectNoPush()
}

// TestAbandonRouteCommitPushRefusesARangeAboveTheFetchedOrigin ports
// abandonment-route-commit-push-range: commit --push fetches origin (where
// the abandonment now is) and the held check over the range refuses; the
// commit stands locally with its Goal-Item and git push never runs.
func TestAbandonRouteCommitPushRefusesARangeAboveTheFetchedOrigin(t *testing.T) {
	t.Parallel()
	a := newAbandonBed(t)
	a.abandonAt("fetch", func(GitCall) GitResult { return ok("") })
	a.owners.Held = func(_, base, commit, _, _ string, stdout, _ io.Writer) int {
		a.heldCalls = append(a.heldCalls, base+" "+commit)
		if !a.abandoned {
			t.Fatal("held ran before the fetch")
		}
		fmt.Fprintf(stdout, "held refused: range-not-linear: %s: %s is not a first-parent ancestor of %s\n", a.git.head, base, a.git.head)
		return 2
	}
	status := a.commit(CommitRequest{HeldEpoch: "5", Push: true, Goal: abandonGoal, GoalSet: true,
		DirectFix: "register-carriage", OwnerLineage: abandonLineage})
	a.expect(status, 1, "held refused: range-not-linear:")
	if a.pushes != 0 {
		t.Fatalf("commit --push ran git push after held refused: %d", a.pushes)
	}
	if a.git.commits != 1 || countExact(a.git.message, "Goal-Item: "+abandonGoal) != 1 {
		t.Fatalf("the local commit lost its Goal-Item:\n%s", a.git.message)
	}
	if len(a.heldCalls) != 1 || a.heldCalls[0] != "refs/remotes/origin/main HEAD" {
		t.Fatalf("held range %v", a.heldCalls)
	}
}

// TestAbandonRouteCommitPushRejectedAfterHeld ports
// abandonment-route-commit-push-rejected: held passes, a peer advances
// origin during the push, and the rejection is reported by name with the
// commit standing locally.
func TestAbandonRouteCommitPushRejectedAfterHeld(t *testing.T) {
	t.Parallel()
	a := newAbandonBed(t)
	a.git.on("push", func(GitCall) GitResult {
		a.pushes++
		a.log.add("push")
		return failed(1, "To origin\n ! [rejected]        main -> main (fetch first)\n")
	})
	status := a.commit(CommitRequest{HeldEpoch: "5", Push: true, Goal: abandonGoal, GoalSet: true,
		DirectFix: "register-carriage", OwnerLineage: abandonLineage})
	a.expect(status, 1, "held: ok 1 commit(s)", "[rejected]", "committed, but origin refused the push")
	held, pushed := -1, -1
	for i, call := range a.log.calls {
		switch {
		case strings.HasPrefix(call, "held") && held < 0:
			held = i
		case call == "push" && pushed < 0:
			pushed = i
		}
	}
	if held < 0 || pushed < 0 || held > pushed || a.pushes != 1 {
		t.Fatalf("held must run before the one push: %v", a.log.calls)
	}
}

// TestAbandonRouteStackChecksEveryCommitAboveOrigin ports
// abandonment-route-stack: a lawful lower commit of the now-abandoned goal
// sits under the landing of another held goal. The landing hands held the
// whole range from origin to HEAD, and held's refusal naming the lower
// commit stops it before push.
func TestAbandonRouteStackChecksEveryCommitAboveOrigin(t *testing.T) {
	t.Parallel()
	a := newAbandonBed(t)
	a.git.head = "lower1"
	a.abandoned = true
	a.owners.Held = func(_, base, commit, _, _ string, stdout, _ io.Writer) int {
		a.heldCalls = append(a.heldCalls, base+" "+commit)
		if base == "refs/remotes/origin/main" && commit == "HEAD" {
			fmt.Fprintf(stdout, "held refused: goal-item-not-held: lower1: goal %s is abandoned at origin1\n", abandonGoal)
			return 1
		}
		fmt.Fprintln(stdout, "held: ok 1 commit(s) above "+base)
		return 0
	}
	status := a.land(LandRequest{Pathspecs: []string{"records/misc/abandonment-stack-l2.md"}, Goal: "ship-gadget", GoalSet: true,
		DirectFix: "register-carriage", SkipTransport: true, OwnerLineage: abandonLineage})
	a.expect(status, 1, "held refused: goal-item-not-held: lower1: goal ship-widget is abandoned at ")
	a.expectNoPush()
	if countExact(a.git.message, "Goal-Item: ship-gadget") != 1 {
		t.Fatalf("the upper commit was not the gadget landing:\n%s", a.git.message)
	}
}

// TestAbandonRoutePositiveControlLands ports abandonment-route-positive:
// with the goal still claimed, the same landing passes held, pushes once and
// records exactly one Machine, Goal-Item and Goal-Revision trailer.
func TestAbandonRoutePositiveControlLands(t *testing.T) {
	t.Parallel()
	a := newAbandonBed(t)
	a.observed = landing.Observation{Mode: "observe", Code: "register-carriage", Provenance: "direct-fix class=register-carriage change=abc",
		VerdictTrailer: "pass bar=b", GoalRevision: 2}
	status := a.landRecord("records/misc/abandonment-positive.md")
	a.expect(status, 0, "held: ok 1 commit(s) above ")
	if a.pushes != 1 {
		t.Fatalf("pushes=%d", a.pushes)
	}
	for _, line := range []string{"Machine: " + abandonActor, "Goal-Item: " + abandonGoal, "Goal-Revision: 2"} {
		if countExact(a.git.message, line) != 1 {
			t.Fatalf("message lacks exactly one %q:\n%s", line, a.git.message)
		}
	}
	if countLines(a.git.message, `(?m)^Goal-Revision:`) != 1 || countLines(a.git.message, `(?m)^Machine:`) != 1 {
		t.Fatalf("duplicate trailers:\n%s", a.git.message)
	}
}

// abandonRecertified prepares a recertified landing of the chain whose
// target is the bed's HEAD: the recertification record and the candidate
// receipt are real files the landing reads.
func (a *abandonBed) abandonRecertified() LandRequest {
	a.t.Helper()
	write := func(name, text string) string {
		path := filepath.Join(a.root, name)
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			a.t.Fatal(err)
		}
		return path
	}
	write("record.json", `{"targetCommit":"h0","sourceAnchorRef":"refs/metasystem/recert/source","mergedAnchorRef":"refs/metasystem/recert/merged"}`)
	receipt := write("receipt.json", `{"tree":"t1"}`)
	a.git.on("rev-parse HEAD^1", func(GitCall) GitResult { return ok("h0\n") })
	return LandRequest{Pathspecs: []string{"records/misc/abandonment-recertified.md"}, Chain: "abandonment-recertified-root",
		Goal: abandonGoal, GoalSet: true, Recertification: "record.json", TestReceipt: receipt, SkipTransport: true,
		OwnerLineage: abandonLineage}
}

// TestAbandonRouteRecertifiedHeldBeforePushAndParks ports
// abandonment-route-recertified. Raced: held passes at the frozen target
// before the single push, the abandonment reaches origin during that push,
// and the rejection parks the attempt as target-moved after exactly one
// push. Moved: with the goal already abandoned, the evaluator refuses the
// recertified commit as goal-item-not-held and the attempt is parked under
// that cause before any push.
func TestAbandonRouteRecertifiedHeldBeforePushAndParks(t *testing.T) {
	t.Parallel()
	t.Run("raced", func(t *testing.T) {
		a := newAbandonBed(t)
		request := a.abandonRecertified()
		a.abandonAt("push", func(GitCall) GitResult {
			a.pushes++
			return failed(1, "To origin\n ! [rejected] main -> main (fetch first)\n")
		})
		status := a.land(request)
		a.expect(status, 1, "held: ok 1 commit(s) above h0", "[rejected]", "PARKED", "chain-recertification-target-moved")
		out := a.stdout.String()
		held := strings.Index(out, "step: goal held at the rebased base")
		push := strings.Index(out, "step: push recertified commit to origin (single attempt)")
		if held < 0 || push < 0 || held > push || a.pushes != 1 {
			t.Fatalf("held must precede the single push: pushes=%d\n%s", a.pushes, out)
		}
		if len(a.parks) != 1 || a.parks[0].Reason != "chain-recertification-target-moved" || a.parks[0].Target != "h0" ||
			a.parks[0].CandidateCommit != "c1" || strings.Join(a.parks[0].RecoveryRefs, " ") != "refs/metasystem/recert/source refs/metasystem/recert/merged" {
			t.Fatalf("parks %+v", a.parks)
		}
	})
	t.Run("moved", func(t *testing.T) {
		a := newAbandonBed(t)
		request := a.abandonRecertified()
		a.abandoned = true
		a.observed = landing.Observation{Mode: "refuse", RefusesAgent: true, Code: "goal-item-not-held",
			Provenance: "chain=abandonment-recertified-root change=abc", VerdictTrailer: "would-refuse code=goal-item-not-held"}
		status := a.land(request)
		a.expect(status, 1, "PARKED", "goal-item-not-held")
		a.expectNoPush()
		if len(a.parks) != 1 || a.parks[0].Reason != "goal-item-not-held" || len(a.git.called("commit")) != 0 {
			t.Fatalf("parks %+v commits %v", a.parks, a.git.called("commit"))
		}
	})
}
