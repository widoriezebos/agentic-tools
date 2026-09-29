package testutil

import (
	"os"
	"strings"
	"syscall"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// A child caught while it exits reads with no executable and no argv, so the
// survivor scan cannot class it as certainly owned; under load the installed
// wait beds' HoldOwnedChildren refused such a child ("fixture child
// ownership is unproven: pid=N exe=\"\" argv=[]", batch 9 and batch 13 VM
// suites). A child that is already dead, a zombie or exiting needs no hold,
// so it is skipped before its ownership is judged; a live child whose
// ownership is unproven is still refused.
func TestHoldOwnedChildrenSkipsAnUnreadableChildThatAlreadyExited(t *testing.T) {
	t.Parallel()
	owner := fixtureTeardownExact(int64(os.Getpid()), 1)
	for _, test := range []struct {
		name      string
		state     identity.Liveness
		mutate    func(*identity.Exact)
		wantError bool
	}{
		{name: "zombie", state: identity.Alive, mutate: func(exact *identity.Exact) { exact.Zombie, exact.Exiting = true, true }},
		{name: "exiting", state: identity.Alive, mutate: func(exact *identity.Exact) { exact.Exiting = true }},
		{name: "dead", state: identity.Dead, mutate: func(*identity.Exact) {}},
		{name: "live", state: identity.Alive, mutate: func(*identity.Exact) {}, wantError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			child := fixtureTeardownExact(600, 3)
			observed := child
			test.mutate(&observed)
			prober := fixtureProbeFunc(func(pid int64) (identity.Exact, identity.Liveness, error) {
				if pid == owner.Pid {
					return owner, identity.Alive, nil
				}
				return observed, test.state, nil
			})
			recorder := &recordingTB{}
			fixture := newProcessFixture(recorder, t.Name(), owner.Ref(), true, prober, func(int, syscall.Signal) error { return nil })
			fixture.scan = func(identity.FixtureKey) ([]identity.FixtureSurvivor, error) {
				return []identity.FixtureSurvivor{{Ref: child.Ref(), Class: identity.FixtureSurvivorUnreadable}}, nil
			}
			err := fixture.HoldOwnedChildren()
			if test.wantError != (err != nil) || err != nil && !strings.Contains(err.Error(), "fixture child ownership is unproven") {
				t.Fatalf("hold = %v, want error %t", err, test.wantError)
			}
			if len(fixture.refs) != 0 {
				t.Fatalf("held %+v", fixture.refs)
			}
			fixture.scan = noFixtureSurvivors
			recorder.cleanups[0]()
		})
	}
}
