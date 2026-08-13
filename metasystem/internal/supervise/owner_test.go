package supervise

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// fakeWorld implements Checkout, Components, Ledger, and Intents with
// programmable behavior, so each Proof row drives the loop
// deterministically through Cycle().
type fakeWorld struct {
	root, state    FileState
	currency       CurrencyState
	stateNamesSelf bool
	publishErr     error
	published      int

	launchErr      error
	launched       []Held
	observation    Observation
	groupCount     int
	groupCountErr  error
	stopProven     bool
	stopped        []Held
	relaunchedErr  error
	relaunched     []relaunchedRecord
	launchedAppend map[string]error
	appendedLaunch []Held
	exited         []exitRecord
	intentFresh    bool
	nextPid        int64
}

type relaunchedRecord struct {
	generation, retiredThrough int64
}

type exitRecord struct {
	reason           string
	teardownComplete bool
}

func newWorld() *fakeWorld {
	return &fakeWorld{
		root: Present, state: Present, currency: NamesSelf,
		stateNamesSelf: true, observation: Healthy, groupCount: 2,
		stopProven: true, launchedAppend: map[string]error{}, nextPid: 100,
	}
}

func (w *fakeWorld) RootState() FileState          { return w.root }
func (w *fakeWorld) StateFileState() FileState     { return w.state }
func (w *fakeWorld) Currency() CurrencyState       { return w.currency }
func (w *fakeWorld) StateNamesSelf() (bool, error) { return w.stateNamesSelf, nil }
func (w *fakeWorld) PublishState(held []Held) error {
	if w.publishErr != nil {
		return w.publishErr
	}
	w.published++
	return nil
}

func (w *fakeWorld) Launch(component Component, tag string) (identity.Ref, error) {
	if w.launchErr != nil {
		return identity.Ref{}, w.launchErr
	}
	w.nextPid++
	ref := identity.Ref{Pid: w.nextPid, StartedAtSec: 1000}
	w.launched = append(w.launched, Held{Component: component, Tag: tag, Identity: ref})
	return ref, nil
}
func (w *fakeWorld) Observe(Held) Observation       { return w.observation }
func (w *fakeWorld) GroupCount([]Held) (int, error) { return w.groupCount, w.groupCountErr }
func (w *fakeWorld) Stop(held Held) bool {
	w.stopped = append(w.stopped, held)
	return w.stopProven
}

func (w *fakeWorld) AppendRelaunched(generation int64, watcherTag, reaperTag string, retiredThrough int64) error {
	if w.relaunchedErr != nil {
		return w.relaunchedErr
	}
	w.relaunched = append(w.relaunched, relaunchedRecord{generation, retiredThrough})
	return nil
}
func (w *fakeWorld) AppendLaunched(held Held) error {
	if err := w.launchedAppend[held.Tag]; err != nil {
		return err
	}
	w.appendedLaunch = append(w.appendedLaunch, held)
	return nil
}
func (w *fakeWorld) AppendExited(reason, diagnosis string, complete bool) {
	w.exited = append(w.exited, exitRecord{reason, complete})
}
func (w *fakeWorld) LatchShutdown() bool { return w.intentFresh }

func newOwner(world *fakeWorld) *Owner {
	return &Owner{
		Checkout: world, Components: world, Ledger: world, Intents: world,
		BaseInterval: time.Second, Ceiling: 12,
		Breaker:       Breaker{GiveUpAt: 5, BaseInterval: time.Second, BackoffCap: 10 * time.Minute},
		Establishment: Establishment{Deadline: 5},
		TagPrefix:     "test-owner",
		Sleep:         func(time.Duration) {},
	}
}

// establish drives the owner through its first publication.
func establish(t *testing.T, owner *Owner, world *fakeWorld) {
	t.Helper()
	if exit := owner.Cycle(time.Now()); exit != nil {
		t.Fatalf("establishment cycle exited: %+v", exit)
	}
	if world.published != 1 || len(world.relaunched) != 1 {
		t.Fatalf("establishment did not launch and publish: %+v", world)
	}
}

// Proof "Purpose gone": the owner exits within one cycle of the root
// vanishing, tears down its own components, and reports the reason.
func TestPurposeGoneExitsAndTearsDown(t *testing.T) {
	world := newWorld()
	owner := newOwner(world)
	establish(t, owner, world)
	world.root = Absent
	exit := owner.Run()
	if exit.Reason != "purpose-gone" || !exit.TeardownComplete {
		t.Fatalf("wrong exit: %+v", exit)
	}
	if len(world.stopped) < 2 {
		t.Fatalf("teardown did not stop the held set: %v", world.stopped)
	}
	if len(world.exited) != 1 || world.exited[0].reason != "purpose-gone" {
		t.Fatalf("terminal not appended: %v", world.exited)
	}
}

// Proof "False-death supersession": the lock naming another identity
// is a voluntary, teardown-first exit.
func TestSupersededExit(t *testing.T) {
	world := newWorld()
	owner := newOwner(world)
	establish(t, owner, world)
	world.currency = NamesOther
	exit := owner.Run()
	if exit.Reason != "superseded" {
		t.Fatalf("wrong exit: %+v", exit)
	}
}

// Proof "Indeterminacy": chmod-000 state keeps the owner running,
// relaunching nothing, counter unmoved.
func TestBlindCycleAttemptsNothing(t *testing.T) {
	world := newWorld()
	owner := newOwner(world)
	establish(t, owner, world)
	world.state = Indeterminate
	launchesBefore := len(world.launched)
	for i := 0; i < 10; i++ {
		if exit := owner.Cycle(time.Now()); exit != nil {
			t.Fatalf("blind owner exited: %+v", exit)
		}
	}
	if len(world.launched) != launchesBefore {
		t.Fatal("a blind owner relaunched")
	}
	if owner.Breaker.Consecutive != 0 {
		t.Fatal("indeterminacy moved the breaker")
	}
}

// Proof "Establishment bounded": first publication impossible → exit
// establishment-failed at the deadline.
func TestEstablishmentFailure(t *testing.T) {
	world := newWorld()
	world.publishErr = errors.New("disk full")
	owner := newOwner(world)
	var exit *Exit
	for i := 0; i < 6 && exit == nil; i++ {
		exit = owner.Cycle(time.Now())
	}
	if exit == nil || exit.Reason != "establishment-failed" {
		t.Fatalf("no establishment failure: %+v", exit)
	}
}

// Proof "Breaker on the REAL shape": components dying every cycle trip
// the counter at N; teardown precedes the terminal and the diagnosis
// rides it.
func TestGivingUpAtN(t *testing.T) {
	world := newWorld()
	owner := newOwner(world)
	establish(t, owner, world)
	world.observation = Failing
	exit := owner.Run()
	if exit.Reason != "giving-up" {
		t.Fatalf("wrong exit: %+v", exit)
	}
	if len(world.exited) != 1 || world.exited[0].reason != "giving-up" {
		t.Fatalf("terminal wrong: %v", world.exited)
	}
}

// Proof "Unrecordable set" (SLC-R6-006): a failed relaunched append
// launches NOTHING.
func TestWriteAheadGatesLaunch(t *testing.T) {
	world := newWorld()
	world.relaunchedErr = errors.New("registry unwritable")
	owner := newOwner(world)
	if exit := owner.Cycle(time.Now()); exit != nil {
		t.Fatalf("early exit: %+v", exit)
	}
	if len(world.launched) != 0 {
		t.Fatal("an owner that cannot record intent created processes")
	}
}

// Proof "Ceiling under forking" (SLC-R4-010): overshoot stops the set
// on THIS observation and counts against the breaker.
func TestCeilingStopsTheSet(t *testing.T) {
	world := newWorld()
	owner := newOwner(world)
	establish(t, owner, world)
	world.groupCount = 13
	stopsBefore := len(world.stopped)
	if exit := owner.Cycle(time.Now()); exit != nil {
		t.Fatalf("early exit: %+v", exit)
	}
	if len(world.stopped) <= stopsBefore {
		t.Fatal("overshoot did not stop the set")
	}
	if owner.Breaker.Consecutive != 1 {
		t.Fatalf("overshoot did not increment the breaker: %d", owner.Breaker.Consecutive)
	}
}

// SLC-R6-006: a failed launched append is retried each observation and
// persistent failure increments the breaker.
func TestLaunchedAppendRetries(t *testing.T) {
	world := newWorld()
	world.launchedAppend["test-owner-watcher-1"] = errors.New("append refused")
	owner := newOwner(world)
	establish(t, owner, world)
	if exit := owner.Cycle(time.Now()); exit != nil {
		t.Fatalf("early exit: %+v", exit)
	}
	if owner.Breaker.Consecutive != 1 {
		t.Fatalf("persistent append failure did not increment: %d", owner.Breaker.Consecutive)
	}
	delete(world.launchedAppend, "test-owner-watcher-1")
	if exit := owner.Cycle(time.Now()); exit != nil {
		t.Fatalf("early exit: %+v", exit)
	}
	found := false
	for _, held := range world.appendedLaunch {
		if held.Tag == "test-owner-watcher-1" {
			found = true
		}
	}
	if !found {
		t.Fatal("owed launched append was not retried to success")
	}
}

// SLC-R5-016: a state file naming another owner is repaired by
// republication within the cycle, without a relaunch.
func TestStateRepair(t *testing.T) {
	world := newWorld()
	owner := newOwner(world)
	establish(t, owner, world)
	world.stateNamesSelf = false
	launchesBefore := len(world.launched)
	if exit := owner.Cycle(time.Now()); exit != nil {
		t.Fatalf("early exit: %+v", exit)
	}
	if world.published != 2 {
		t.Fatalf("state was not republished: %d", world.published)
	}
	if len(world.launched) != launchesBefore {
		t.Fatal("repair relaunched instead of republishing")
	}
}

// Proof "Shutdown attribution" (SLC-R9-004): the intent is latched at
// exit initiation — fresh intent means shutdown, none means terminated.
func TestSignalExitLatchesIntent(t *testing.T) {
	for _, fresh := range []bool{true, false} {
		world := newWorld()
		world.intentFresh = fresh
		owner := newOwner(world)
		establish(t, owner, world)
		exit := owner.ExitOnSignal()
		want := "terminated"
		if fresh {
			want = "shutdown"
		}
		if exit.Reason != want {
			t.Fatalf("intent fresh=%v: got %q want %q", fresh, exit.Reason, want)
		}
		if len(world.stopped) < 2 {
			t.Fatal("signal exit skipped teardown")
		}
	}
}

// SLC-R8-002/SLC-R9-003: each relaunch verifies the old set and the
// watermark rides the relaunched record; an unproven stop pins it.
func TestWatermarkAdvancesOnVerifiedStops(t *testing.T) {
	world := newWorld()
	owner := newOwner(world)
	establish(t, owner, world)
	world.observation = Failing
	// Drive one relaunch (k=1 relaunches immediately).
	if exit := owner.Cycle(time.Now()); exit != nil {
		t.Fatalf("early exit: %+v", exit)
	}
	if len(world.relaunched) != 2 {
		t.Fatalf("no relaunch recorded: %+v", world.relaunched)
	}
	if got := world.relaunched[1]; got.generation != 2 || got.retiredThrough != 1 {
		t.Fatalf("verified stop did not advance the watermark: %+v", got)
	}

	// Now make stops unprovable: the next relaunch pins the watermark.
	world.stopProven = false
	owner.Breaker.Consecutive = 0
	if exit := owner.Cycle(time.Now()); exit != nil {
		t.Fatalf("early exit: %+v", exit)
	}
	last := world.relaunched[len(world.relaunched)-1]
	if last.retiredThrough != 1 {
		t.Fatalf("unproven stop advanced the watermark: %+v", last)
	}
}

// The teardownComplete bit is honest: an unprovable stop reports
// incomplete teardown on the terminal (SLC-R4-012).
func TestHonestTeardownComplete(t *testing.T) {
	world := newWorld()
	owner := newOwner(world)
	establish(t, owner, world)
	world.stopProven = false
	world.root = Absent
	exit := owner.Run()
	if exit.TeardownComplete {
		t.Fatal("teardownComplete lied about unproven stops")
	}
	if world.exited[0].teardownComplete {
		t.Fatal("the terminal hid survivors")
	}
}

func TestTagsCarryGeneration(t *testing.T) {
	world := newWorld()
	owner := newOwner(world)
	establish(t, owner, world)
	for _, held := range world.launched {
		want := fmt.Sprintf("test-owner-%s-1", held.Component)
		if held.Tag != want {
			t.Fatalf("tag %q, want %q", held.Tag, want)
		}
	}
}

// dispatch-supervise-6: an uncountable group is not a healthy one — the
// count error maps to Indeterminable, which HOLDS the breaker where it is
// (no reset, no increment: unknown never authorizes anything) instead of
// being silently ignored and read as Healthy, which would reset the
// breaker while a forking set outgrows its ceiling unseen.
func TestCeilingCountErrorIsIndeterminable(t *testing.T) {
	world := newWorld()
	owner := newOwner(world)
	establish(t, owner, world)
	owner.Breaker.Consecutive = 2
	world.groupCountErr = errors.New("process table denied")
	if exit := owner.Cycle(time.Now()); exit != nil {
		t.Fatalf("early exit: %+v", exit)
	}
	if owner.Breaker.Consecutive != 2 {
		t.Fatalf("an indeterminable count must hold the breaker, not reset or advance it: %d", owner.Breaker.Consecutive)
	}
}
