package act

// What a request leaves behind when it does not return.
//
// The two holds an act takes — its registration and this clone's one lock — are
// taken inside request() and released by the caller's own `defer done()`. That
// defer is installed only after request() RETURNS, so the whole of the assembly
// between the two is a window with no cleanup at all: an assembly reader that
// panics strands the registration and the lock until the process restarts, and
// every act on that goal, and every publication and recovery on that clone, is
// refused or blocked from then on (Astra E-04).

import (
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// panicking is a reading that fails the way nothing here expects: not with an
// error the caller can return, but by unwinding through it.
const panicking = "this reading unwound instead of answering"

// nothingIsHeld says whether this clone's ledger is free: the one lock can be
// taken, and no act stands registered.
func nothingIsHeld(t *testing.T, bed *ledgerBed) (lock bool, registered int) {
	t.Helper()
	held := ownerOf(bed.root)
	if held.publications.TryLock() {
		held.publications.Unlock()
		lock = true
	}
	held.running.Lock()
	defer held.running.Unlock()
	return lock, len(held.acts)
}

func TestAnAssemblyThatUnwindsLeavesNoHoldAndNoLock(t *testing.T) {
	t.Parallel()
	bed := ledger(t)
	openGoal(t, bed, "ui-unwound")
	authority := sessionFor(t, bed)
	// The endpoint reader is the one an act reaches first after it has taken
	// both holds, which is the middle of the window.
	authority.reads.endpoint = func(string) (goal.Endpoint, error) { panic(panicking) }

	func() {
		defer func() {
			if recovered := recover(); recovered != panicking {
				t.Fatalf("the reading did not unwind through the request: %v", recovered)
			}
		}()
		_ = authority.Park("ui-unwound", "waiting for the review")
	}()

	lock, registered := nothingIsHeld(t, bed)
	testutil.Expect(t, "this clone's one lock is free again", lock, true)
	testutil.Expect(t, "and no act stands registered", registered, 0)

	// Which is the whole of what it means: the goal is actable again.
	sound := sessionFor(t, bed)
	if err := sound.Park("ui-unwound", "waiting for the review"); err != nil {
		t.Fatalf("the press after the unwound one: %v", err)
	}
	testutil.Expect(t, "the pause the human made stands", readGoal(t, bed, "ui-unwound").State, goal.StateParked)
}
