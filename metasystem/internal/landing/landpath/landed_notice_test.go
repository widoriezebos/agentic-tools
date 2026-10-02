package landpath

import (
	"errors"
	"strings"
	"testing"
)

// Decision 7 of the blocked-agent-asks-the-human design: every route of
// the landing path that pushes to main tells the channel once, with the
// pushed commit and the goal it landed in the name of: the staged form,
// the recertified form and the carried (exception) form. A landing that
// never pushed tells nothing, and a failed telling stops nothing.

type landedCalls struct{ calls []string }

func (l *landedCalls) owner(err error) func(root, goal, commit string) error {
	return func(_, goal, commit string) error {
		l.calls = append(l.calls, goal+"@"+commit)
		return err
	}
}

func TestStagedLandingTellsTheChannelOnce(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	driverPathMode(b)
	var told landedCalls
	b.owners.Landed = told.owner(nil)
	b.expect(b.land(LandRequest{Pathspecs: []string{"payload.txt"}, Goal: "fx", GoalSet: true, SkipTransport: true}), 0)
	if len(told.calls) != 1 || told.calls[0] != "fx@"+b.git.head {
		t.Fatalf("told %v; want fx@%s once", told.calls, b.git.head)
	}

	refused := newBed(t)
	driverPathMode(refused)
	gateAnswers(refused, errors.New(gateHeld))
	var none landedCalls
	refused.owners.Landed = none.owner(nil)
	refused.expect(refused.land(LandRequest{Pathspecs: []string{"payload.txt"}, Goal: "fx", GoalSet: true, SkipTransport: true}), 1, gateHeld)
	if len(none.calls) != 0 {
		t.Fatalf("a landing that never pushed told %v", none.calls)
	}

	failing := newBed(t)
	driverPathMode(failing)
	var failed landedCalls
	failing.owners.Landed = failed.owner(errors.New("send failed: unreachable"))
	failing.expect(failing.land(LandRequest{Pathspecs: []string{"payload.txt"}, SkipTransport: true}), 0)
	if len(failed.calls) != 1 || failed.calls[0] != "@"+failing.git.head {
		t.Fatalf("told %v; want the goal-less landing once", failed.calls)
	}
}

func TestRecertifiedLandingTellsTheChannelOnce(t *testing.T) {
	t.Parallel()
	a := newAbandonBed(t)
	request := a.abandonRecertified()
	var told landedCalls
	a.owners.Landed = told.owner(nil)
	if status := a.land(request); status != 0 {
		t.Fatalf("recertified landing = %d\n%s%s", status, a.stdout.String(), a.stderr.String())
	}
	if a.pushes != 1 || len(told.calls) != 1 || told.calls[0] != abandonGoal+"@"+a.git.head {
		t.Fatalf("pushes=%d told %v; want %s@%s once", a.pushes, told.calls, abandonGoal, a.git.head)
	}

	parked := newAbandonBed(t)
	request = parked.abandonRecertified()
	parked.abandonAt("push", func(GitCall) GitResult {
		parked.pushes++
		return failed(1, "To origin\n ! [rejected] main -> main (fetch first)\n")
	})
	var none landedCalls
	parked.owners.Landed = none.owner(nil)
	if status := parked.land(request); status == 0 || len(none.calls) != 0 {
		t.Fatalf("a rejected recertified push = %d told %v", status, none.calls)
	}
}

func TestCarriedLandingTellsTheChannelOnce(t *testing.T) {
	t.Parallel()
	c := newCarriedBed(t)
	var told landedCalls
	c.owners.Landed = told.owner(nil)
	status, killed := c.landCarried(carriedRequest())
	if killed != "" {
		t.Fatalf("killed at %s", killed)
	}
	c.expect(status, 0)
	if c.pushes() != 1 || len(told.calls) != 1 || !strings.HasSuffix(told.calls[0], "@"+c.git.head) {
		t.Fatalf("pushes=%d told %v; want the carried commit %s once", c.pushes(), told.calls, c.git.head)
	}

	held := newCarriedBed(t)
	gateAnswers(held.bed, errors.New(gateHeld))
	var none landedCalls
	held.owners.Landed = none.owner(nil)
	status, _ = held.landCarried(carriedRequest())
	held.expect(status, 1, gateHeld)
	if len(none.calls) != 0 {
		t.Fatalf("a carried landing that never pushed told %v", none.calls)
	}
}
