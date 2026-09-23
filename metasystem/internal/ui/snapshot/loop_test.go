package snapshot

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/boundedexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// fakeTimer hands the test the loop's clock. The test decides when the loop
// looks, so no deadline in this file depends on real time passing.
type fakeTimer struct {
	c chan time.Time
}

func (t *fakeTimer) C() <-chan time.Time { return t.c }
func (t *fakeTimer) Stop()               {}

func (t *fakeTimer) fire() {
	select {
	case t.c <- time.Time{}:
	default:
	}
}

type fakeTimers struct {
	mu        sync.Mutex
	durations []time.Duration
	armed     chan *fakeTimer
}

func newFakeTimers() *fakeTimers {
	return &fakeTimers{armed: make(chan *fakeTimer, 1024)}
}

func (f *fakeTimers) NewTimer(after time.Duration) Timer {
	f.mu.Lock()
	f.durations = append(f.durations, after)
	f.mu.Unlock()
	timer := &fakeTimer{c: make(chan time.Time, 1)}
	f.armed <- timer
	return timer
}

// next blocks until the loop arms its next deadline, which is the one moment
// the loop is provably back at its deadline with no tick in flight.
func (f *fakeTimers) next() *fakeTimer { return <-f.armed }

func (f *fakeTimers) recorded() []time.Duration {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]time.Duration(nil), f.durations...)
}

func (f *fakeTimers) armedSince(mark int) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.durations) - mark
}

type scriptedFetch struct {
	mu        sync.Mutex
	calls     int
	endpoints []goal.Endpoint
	answer    func(call int) (goal.AdvanceResult, error)
	entered   chan struct{}
	held      chan struct{}
}

func (s *scriptedFetch) fetch(endpoint goal.Endpoint) (goal.AdvanceResult, error) {
	s.mu.Lock()
	s.calls++
	call := s.calls
	s.endpoints = append(s.endpoints, endpoint)
	entered, held := s.entered, s.held
	s.mu.Unlock()
	if entered != nil {
		entered <- struct{}{}
	}
	if held != nil {
		<-held
	}
	return s.answer(call)
}

func (s *scriptedFetch) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

func (s *scriptedFetch) lastEndpoint() goal.Endpoint {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.endpoints[len(s.endpoints)-1]
}

func currentAt(tip string) func(int) (goal.AdvanceResult, error) {
	return func(int) (goal.AdvanceResult, error) {
		return goal.AdvanceResult{Tip: tip, Detail: "already at the canonical tip"}, nil
	}
}

func alwaysFailing(message string) func(int) (goal.AdvanceResult, error) {
	return func(int) (goal.AdvanceResult, error) {
		return goal.AdvanceResult{}, fmt.Errorf("%s", message)
	}
}

// fetchState reads the loop's state without observing, so an assertion about
// what a tick left behind is not disturbed by the act of reading it.
func fetchState(h *Holder) FetchState {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.fetch
}

// quiet takes the request signal a setup observation left behind, so it does
// not also re-arm the loop's first deadline.
func quiet(h *Holder) {
	select {
	case <-h.wake:
	default:
	}
}

func runLoop(h *Holder, ctx context.Context, fetch Fetch, timers Timers) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		h.Run(ctx, fetch, timers)
	}()
	return done
}

// drain fires one deadline and returns the one the loop arms after it. It is
// for tests that make no request in between: a request re-arms the loop's
// deadline without ticking, and then tickOnce is the right instrument.
func drain(timers *fakeTimers, timer *fakeTimer) *fakeTimer {
	timer.fire()
	return timers.next()
}

// tickOnce drives the loop through exactly one more advance. A request
// arriving while the loop waits replaces the deadline it holds, so a test
// that also makes requests cannot assume the deadline in its hand is still
// the loop's.
func tickOnce(t *testing.T, timers *fakeTimers, fetch *scriptedFetch, timer *fakeTimer) *fakeTimer {
	t.Helper()
	before := fetch.count()
	for {
		timer.fire()
		timer = timers.next()
		if fetch.count() > before {
			return timer
		}
	}
}

func TestRunTicksAtOnceAndSettlesOnTheIdleCadence(t *testing.T) {
	t.Parallel()
	b, tip := readableBed(t)
	holder := New(b.root, newClock(fixtureNow).now)
	timers := newFakeTimers()
	fetch := &scriptedFetch{answer: currentAt(tip)}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := runLoop(holder, ctx, fetch.fetch, timers)

	first := timers.next()
	testutil.Expect(t, "the first deadline", timers.recorded(), []time.Duration{0})
	drain(timers, first)

	state := fetchState(holder)
	testutil.Expect(t, "outcome", state.Outcome, OutcomeCurrent)
	testutil.Expect(t, "tip", state.Tip, tip)
	testutil.Expect(t, "detail", state.Detail, "already at the canonical tip")
	testutil.Expect(t, "started at", state.StartedAt, fixtureNow)
	testutil.Expect(t, "finished at", state.FinishedAt, fixtureNow)
	testutil.Expect(t, "when the look landed", state.SucceededAt, fixtureNow)
	testutil.Expect(t, "the tip it landed on", state.SucceededTip, tip)
	testutil.Expect(t, "failures", state.Failures, 0)
	testutil.Expect(t, "cadence", state.Cadence, CadenceIdle)
	testutil.Expect(t, "next at", state.NextAt, fixtureNow.Add(idleInterval))
	testutil.Expect(t, "the second deadline", timers.recorded()[1], idleInterval)
	testutil.Expect(t, "the endpoint was resolved from the checkout", fetch.lastEndpoint().Root, b.root)

	cancel()
	<-done
}

// TestEveryTickResolvesTheEndpointAgain: git configuration changes
// independently of anything the loop holds, so a tick that reused a resolved
// endpoint would keep fetching from a remote the checkout no longer names.
func TestEveryTickResolvesTheEndpointAgain(t *testing.T) {
	t.Parallel()
	b, tip := readableBed(t)
	holder := New(b.root, newClock(fixtureNow).now)
	timers := newFakeTimers()
	fetch := &scriptedFetch{answer: currentAt(tip)}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := runLoop(holder, ctx, fetch.fetch, timers)

	timer := drain(timers, timers.next())
	testutil.Expect(t, "the branch the first tick read", fetch.lastEndpoint().Branch, "refs/heads/main")

	b.git("config", "goal.sync-branch", "refs/heads/canonical")
	drain(timers, timer)
	testutil.Expect(t, "the branch the second tick read", fetch.lastEndpoint().Branch, "refs/heads/canonical")

	cancel()
	<-done
}

// TestObserveSetsTheCadenceAndNothingElse is the whole of a request's effect
// on the loop: it says a browser is reading, which shortens the wait, and it
// starts nothing.
func TestObserveSetsTheCadenceAndNothingElse(t *testing.T) {
	t.Parallel()
	b, tip := readableBed(t)
	at := newClock(fixtureNow)
	holder := New(b.root, at.now)
	timers := newFakeTimers()
	fetch := &scriptedFetch{answer: currentAt(tip)}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := runLoop(holder, ctx, fetch.fetch, timers)

	drain(timers, timers.next())
	testutil.Require(t, "the idle deadline", timers.recorded(), []time.Duration{0, idleInterval})

	// A request pulls the idle deadline in to the connected interval and
	// starts nothing.
	holder.Observe()
	connectedTimer := timers.next()
	testutil.Expect(t, "the deadline a request pulled in", timers.recorded()[2], connectedInterval)
	testutil.Expect(t, "the loop did not tick for the request", fetch.count(), 1)

	// While requests keep arriving, every tick stays on the connected cadence.
	after := drain(timers, connectedTimer)
	testutil.Expect(t, "the cadence while a browser reads", fetchState(holder).Cadence, CadenceConnected)
	testutil.Expect(t, "the connected deadline", timers.recorded()[3], connectedInterval)

	// Once nobody has read for longer than the window, it returns to idle.
	at.advance(connectedWindow + time.Second)
	drain(timers, after)
	testutil.Expect(t, "the cadence once nobody reads", fetchState(holder).Cadence, CadenceIdle)
	testutil.Expect(t, "the idle deadline again", timers.recorded()[4], idleInterval)

	cancel()
	<-done
}

// TestOneTickAtATime proves the loop cannot overlap itself: a tick runs
// inline, so neither a fired deadline nor a flood of requests can start a
// second advance while one is in flight.
func TestOneTickAtATime(t *testing.T) {
	t.Parallel()
	b, tip := readableBed(t)
	holder := New(b.root, newClock(fixtureNow).now)
	timers := newFakeTimers()
	fetch := &scriptedFetch{
		answer:  currentAt(tip),
		entered: make(chan struct{}, 1),
		held:    make(chan struct{}),
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := runLoop(holder, ctx, fetch.fetch, timers)

	first := timers.next()
	first.fire()
	<-fetch.entered

	first.fire()
	for i := 0; i < 10; i++ {
		holder.Observe()
	}
	testutil.Expect(t, "fetches while one is in flight", fetch.count(), 1)
	running := fetchState(holder)
	testutil.Expect(t, "outcome while in flight", running.Outcome, OutcomeRunning)
	testutil.Expect(t, "started at", running.StartedAt, fixtureNow)
	testutil.Expect(t, "finished at", running.FinishedAt, time.Time{})

	close(fetch.held)
	timers.next()
	settled := fetchState(holder)
	testutil.Expect(t, "outcome after release", settled.Outcome, OutcomeCurrent)
	testutil.Expect(t, "the requests during the tick set the cadence", settled.Cadence, CadenceConnected)
	testutil.Expect(t, "the deadline after the tick", settled.NextAt, fixtureNow.Add(connectedInterval))
	testutil.Expect(t, "fetches after release", fetch.count(), 1)

	cancel()
	<-done
}

func TestFailingTicksBackOffAndOneSuccessResets(t *testing.T) {
	t.Parallel()

	t.Run("while a browser is reading", func(t *testing.T) {
		t.Parallel()
		b, tip := readableBed(t)
		holder := New(b.root, newClock(fixtureNow).now)
		timers := newFakeTimers()
		fetch := &scriptedFetch{answer: func(call int) (goal.AdvanceResult, error) {
			if call == 1 || call == 9 {
				return goal.AdvanceResult{Tip: tip, Detail: "already at the canonical tip"}, nil
			}
			return goal.AdvanceResult{}, fmt.Errorf("git fetch: the remote does not answer")
		}}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		holder.Observe()
		quiet(holder)
		done := runLoop(holder, ctx, fetch.fetch, timers)

		timer := timers.next()
		for i := 0; i < 9; i++ {
			timer = drain(timers, timer)
		}

		testutil.Expect(t, "the deadlines", timers.recorded()[1:], []time.Duration{
			connectedInterval, 10 * time.Second, 20 * time.Second, 40 * time.Second,
			80 * time.Second, 160 * time.Second, backoffCap, backoffCap, connectedInterval,
		})
		testutil.Expect(t, "failures after the reset", fetchState(holder).Failures, 0)

		cancel()
		<-done
	})

	t.Run("while nobody is reading", func(t *testing.T) {
		t.Parallel()
		b, _ := readableBed(t)
		holder := New(b.root, newClock(fixtureNow).now)
		timers := newFakeTimers()
		fetch := &scriptedFetch{answer: alwaysFailing("git fetch: the remote does not answer")}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := runLoop(holder, ctx, fetch.fetch, timers)

		timer := timers.next()
		for i := 0; i < 5; i++ {
			timer = drain(timers, timer)
		}

		testutil.Expect(t, "the deadlines", timers.recorded()[1:], []time.Duration{
			60 * time.Second, 120 * time.Second, 240 * time.Second, backoffCap, backoffCap,
		})
		state := fetchState(holder)
		testutil.Expect(t, "the failed message", state.Message, "git fetch: the remote does not answer")
		testutil.Expect(t, "a failed tick reports no tip", state.Tip, "")

		cancel()
		<-done
	})

	t.Run("a request never shortens a wait below the backoff", func(t *testing.T) {
		t.Parallel()
		b, _ := readableBed(t)
		holder := New(b.root, newClock(fixtureNow).now)
		timers := newFakeTimers()
		fetch := &scriptedFetch{answer: alwaysFailing("git fetch: the remote does not answer")}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		holder.Observe()
		quiet(holder)
		done := runLoop(holder, ctx, fetch.fetch, timers)

		timer := timers.next()
		for i := 0; i < 3; i++ {
			timer = drain(timers, timer)
		}
		testutil.Require(t, "the wait after three failures", timers.recorded()[3], 40*time.Second)

		mark := len(timers.recorded())
		holder.Observe()
		timers.next()
		testutil.Expect(t, "the wait a request asked for", timers.recorded()[mark], 40*time.Second)
		testutil.Expect(t, "the loop did not tick for the request", fetch.count(), 3)

		cancel()
		<-done
	})
}

// The last look that landed outlives the looks after it. A reader asking how
// fresh this clone is is asking when it last heard from the canonical branch,
// and a failure that erased that instant would make an hour of failures
// indistinguishable from a clone that has never fetched at all.
func TestTheLastLookThatLandedOutlivesTheOnesAfterIt(t *testing.T) {
	t.Parallel()

	b, tip := readableBed(t)
	at := newClock(fixtureNow)
	holder := New(b.root, at.now)
	timers := newFakeTimers()
	fetch := &scriptedFetch{answer: func(call int) (goal.AdvanceResult, error) {
		if call == 1 {
			return goal.AdvanceResult{Tip: tip, Detail: "already at the canonical tip"}, nil
		}
		return goal.AdvanceResult{}, fmt.Errorf("git fetch: the remote does not answer")
	}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := runLoop(holder, ctx, fetch.fetch, timers)

	timer := timers.next()
	timer = drain(timers, timer)
	landed := fetchState(holder)
	testutil.Require(t, "the first look landed", landed.Outcome, OutcomeCurrent)

	drain(timers, timer)
	failed := fetchState(holder)

	testutil.Expect(t, "the outcome after the failure", failed.Outcome, OutcomeFailed)
	testutil.Expect(t, "the failed tick reports no tip", failed.Tip, "")
	testutil.Expect(t, "when the last look landed", failed.SucceededAt, landed.SucceededAt)
	testutil.Expect(t, "the tip it landed on", failed.SucceededTip, tip)

	cancel()
	<-done
}

func TestNextDue(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		connected bool
		failures  int
		wait      time.Duration
		cadence   string
	}{
		{"a browser is reading", true, 0, 5 * time.Second, CadenceConnected},
		{"nobody is reading", false, 0, 30 * time.Second, CadenceIdle},
		{"one failure while reading", true, 1, 10 * time.Second, CadenceConnected},
		{"five failures while reading", true, 5, 160 * time.Second, CadenceConnected},
		{"six failures while reading", true, 6, backoffCap, CadenceConnected},
		{"a hundred failures while reading", true, 100, backoffCap, CadenceConnected},
		{"one failure while idle", false, 1, 60 * time.Second, CadenceIdle},
		{"three failures while idle", false, 3, 240 * time.Second, CadenceIdle},
		{"four failures while idle", false, 4, backoffCap, CadenceIdle},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			wait, cadence := nextDue(testCase.connected, testCase.failures)
			testutil.Expect(t, "wait", wait, testCase.wait)
			testutil.Expect(t, "cadence", cadence, testCase.cadence)
		})
	}
}

// TestARequestMovesNoRef is the boundary this whole arrangement exists to
// keep: reading the backlog answers from the accepted ref as it stands and
// moves nothing, whatever a page does.
func TestARequestMovesNoRef(t *testing.T) {
	t.Parallel()

	t.Run("a request never starts a tick", func(t *testing.T) {
		t.Parallel()
		b, tip := readableBed(t)
		holder := New(b.root, newClock(fixtureNow).now)
		timers := newFakeTimers()
		fetch := &scriptedFetch{answer: currentAt(tip)}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := runLoop(holder, ctx, fetch.fetch, timers)

		timers.next()
		for i := 0; i < 100; i++ {
			holder.Observe()
		}
		testutil.Expect(t, "fetches", fetch.count(), 0)

		cancel()
		<-done
	})

	t.Run("with no loop at all, the repository is untouched", func(t *testing.T) {
		t.Parallel()
		b, _ := readableBed(t)
		holder := New(b.root, newClock(fixtureNow).now)
		refsBefore := b.metasystemRefs()
		headBefore := b.git("rev-parse", "HEAD")
		statusBefore := b.git("status", "--porcelain")

		for i := 0; i < 50; i++ {
			if observation := holder.Observe(); observation.State != StateRead {
				t.Fatalf("observation %d answered %s", i, observation.State)
			}
		}

		testutil.Expect(t, "the metasystem refs", b.metasystemRefs(), refsBefore)
		testutil.Expect(t, "HEAD", b.git("rev-parse", "HEAD"), headBefore)
		testutil.Expect(t, "the working tree", b.git("status", "--porcelain"), statusBefore)
		testutil.Expect(t, "no per-operation ref was ever created",
			strings.Contains(refsBefore, "refs/metasystem/goals/fetch/"), false)
	})
}

// TestTheLoopHealsAClone drives the real bounded advance end to end with no
// network: a clone with no accepted ref creates one from the canonical tip it
// validated, and does so again if the ref is removed underneath it.
func TestTheLoopHealsAClone(t *testing.T) {
	t.Parallel()
	b, tip := singleMachineBed(t)
	holder := New(b.root, newClock(fixtureNow).now)
	timers := newFakeTimers()
	counted := &scriptedFetch{answer: nil}
	advance := func(endpoint goal.Endpoint) (goal.AdvanceResult, error) {
		counted.mu.Lock()
		counted.calls++
		counted.mu.Unlock()
		return goal.FetchAdvanceBounded(endpoint, FetchBudget)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	before := holder.Observe()
	quiet(holder)
	testutil.Require(t, "the state before the first tick", before.State, StateAbsent)
	testutil.Require(t, "the outcome before the first tick", before.Fetch.Outcome, OutcomeNever)

	headBefore := b.git("rev-parse", "HEAD")
	statusBefore := b.git("status", "--porcelain")
	branchesBefore := b.git("for-each-ref", "--format=%(refname) %(objectname)", "refs/heads/")

	done := runLoop(holder, ctx, advance, timers)
	timer := tickOnce(t, timers, counted, timers.next())

	testutil.Expect(t, "the first tick", fetchState(holder).Outcome, OutcomeAdvanced)
	testutil.Expect(t, "the accepted tip", fetchState(holder).Tip, tip)
	healed := holder.Observe()
	testutil.Expect(t, "the state after the first tick", healed.State, StateRead)
	testutil.Expect(t, "the observed tip", healed.Tip, tip)
	testutil.Expect(t, "HEAD", b.git("rev-parse", "HEAD"), headBefore)
	testutil.Expect(t, "the working tree", b.git("status", "--porcelain"), statusBefore)
	testutil.Expect(t, "the branches", b.git("for-each-ref", "--format=%(refname) %(objectname)", "refs/heads/"), branchesBefore)
	testutil.Expect(t, "no per-operation ref is left behind", b.metasystemRefs(), goal.AcceptedRef+" "+tip)

	// The ref removed underneath a running server heals on the next tick.
	b.git("update-ref", "-d", goal.AcceptedRef)
	testutil.Expect(t, "the state once the ref is gone", holder.Observe().State, StateAbsent)
	tickOnce(t, timers, counted, timer)
	testutil.Expect(t, "the state after the healing tick", holder.Observe().State, StateRead)
	testutil.Expect(t, "the refs after healing", b.metasystemRefs(), goal.AcceptedRef+" "+tip)

	cancel()
	<-done
}

// TestTheLoopRefusesARefTheEngineWouldNotHaveMade: a ref pointing at a commit
// with no ledger was set by something other than the engine, and neither the
// loop nor the deliberate repair can move it.
func TestTheLoopRefusesARefTheEngineWouldNotHaveMade(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.write("README.md", []byte("no ledger here\n"))
	ledgerless := b.commit("before the ledger")
	b.writeRoot(goal.SyncLocal)
	b.writeGoal(bedGoal("alpha", goal.StateQueued))
	ledger := b.commit("the ledger")
	b.git("update-ref", goal.LocalLedgerBranch, ledger)
	b.accept(ledgerless)
	holder := New(b.root, newClock(fixtureNow).now)
	timers := newFakeTimers()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	testutil.Require(t, "the state before the tick", holder.Observe().State, StateNoLedger)
	quiet(holder)
	refsBefore := b.metasystemRefs()

	done := runLoop(holder, ctx, boundedAdvance, timers)
	drain(timers, timers.next())

	state := fetchState(holder)
	testutil.Expect(t, "the tick", state.Outcome, OutcomeFailed)
	testutil.Expect(t, "the refusal is the engine's",
		strings.Contains(state.Message, "the accepted tree's identity cannot be read"), true)
	testutil.Expect(t, "failures", state.Failures, 1)
	testutil.Expect(t, "the refs are untouched", b.metasystemRefs(), refsBefore)

	cancel()
	<-done
}

// TestTheLoopMeetsARefFileGitCannotRead records what git does when asked to
// create a ref whose loose file is already there and unreadable. Either the
// creation succeeds and the next observation reads, or it refuses and the
// tick fails with git's words; no third pair is acceptable.
func TestTheLoopMeetsARefFileGitCannotRead(t *testing.T) {
	t.Parallel()
	b, _ := singleMachineBed(t)
	writeGarbageRef(t, b)
	holder := New(b.root, newClock(fixtureNow).now)
	timers := newFakeTimers()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	testutil.Require(t, "the state before the tick", holder.Observe().State, StateBroken)
	quiet(holder)

	done := runLoop(holder, ctx, boundedAdvance, timers)
	drain(timers, timers.next())

	state := fetchState(holder)
	observation := holder.Observe()
	switch {
	case observation.State == StateRead && state.Outcome == OutcomeAdvanced:
		t.Logf("git created the accepted ref over the unreadable file; the ledger reads again")
	case observation.State == StateBroken && state.Outcome == OutcomeFailed && state.Message != "":
		t.Logf("git refused to create the accepted ref over the unreadable file: %s", state.Message)
	default:
		t.Fatalf("an unreadable ref file gave state %q with outcome %q and message %q",
			observation.State, state.Outcome, state.Message)
	}

	cancel()
	<-done
}

// TestTheLoopRefusesAContradictedSyncMode: the ledger says single-machine and
// the configuration says remote, so every tick refuses at the same gate with
// the same words until the configuration is fixed.
func TestTheLoopRefusesAContradictedSyncMode(t *testing.T) {
	t.Parallel()
	b, _ := singleMachineBed(t)
	b.git("remote", "add", "origin", b.root)
	b.git("config", "goal.sync-remote", "origin")
	holder := New(b.root, newClock(fixtureNow).now)
	timers := newFakeTimers()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	testutil.Require(t, "the observation", holder.Observe().State, StateAbsent)
	quiet(holder)

	done := runLoop(holder, ctx, boundedAdvance, timers)
	drain(timers, timers.next())

	state := fetchState(holder)
	testutil.Expect(t, "the tick", state.Outcome, OutcomeFailed)
	testutil.Expect(t, "the refusal is the engine's",
		strings.Contains(state.Message, "sync-mode mismatch refused"), true)
	testutil.Expect(t, "failures", state.Failures, 1)
	testutil.Expect(t, "no ref was created", b.metasystemRefs(), "")

	cancel()
	<-done
}

// TestABoundedFetchThatTimesOutKeepsTheLoopAlive is why the advance is
// bounded: a tick that ran out of time ends as an ordinary failure, carries
// the timeout's own words, and the loop looks again on its backoff rather
// than staying "running" until someone kills the server.
func TestABoundedFetchThatTimesOutKeepsTheLoopAlive(t *testing.T) {
	t.Parallel()
	b, tip := readableBed(t)
	holder := New(b.root, newClock(fixtureNow).now)
	timers := newFakeTimers()
	timedOut := fmt.Errorf("git fetch: fresh canonical ledger fetch %w after %s (raise it with exec.network-timeout-sec) ()",
		boundedexec.ErrTimedOut, FetchBudget)
	fetch := &scriptedFetch{answer: func(call int) (goal.AdvanceResult, error) {
		if call == 1 {
			return goal.AdvanceResult{}, timedOut
		}
		return goal.AdvanceResult{Tip: tip, Detail: "already at the canonical tip"}, nil
	}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := runLoop(holder, ctx, fetch.fetch, timers)

	timer := drain(timers, timers.next())
	expired := fetchState(holder)
	testutil.Expect(t, "the outcome of a fetch that ran out of time", expired.Outcome, OutcomeFailed)
	testutil.Expect(t, "the message is the timeout's", expired.Message, timedOut.Error())
	testutil.Expect(t, "failures", expired.Failures, 1)
	testutil.Expect(t, "the backoff deadline", expired.NextAt, fixtureNow.Add(2*idleInterval))
	testutil.Expect(t, "the recorded backoff", timers.recorded()[1], 2*idleInterval)

	// The loop is alive: the next tick runs and clears the failure.
	drain(timers, timer)
	settled := fetchState(holder)
	testutil.Expect(t, "the outcome after the next tick", settled.Outcome, OutcomeCurrent)
	testutil.Expect(t, "failures after a success", settled.Failures, 0)

	cancel()
	<-done
}

// TestCancellationOutranksAReadyDeadline: Go's select picks uniformly among
// ready cases, so a deadline that comes ready in the same instant as
// cancellation could win it and start one last fetch on the way out.
// Cancellation is therefore checked, never selected for.
func TestCancellationOutranksAReadyDeadline(t *testing.T) {
	t.Parallel()

	t.Run("a loop cancelled before it starts never looks", func(t *testing.T) {
		t.Parallel()
		b, tip := readableBed(t)
		holder := New(b.root, newClock(fixtureNow).now)
		fetch := &scriptedFetch{answer: currentAt(tip)}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		holder.Run(ctx, fetch.fetch, firedTimers{})

		testutil.Expect(t, "fetches", fetch.count(), 0)
		testutil.Expect(t, "the next deadline", fetchState(holder).NextAt, time.Time{})
	})

	t.Run("a deadline ready in the same instant as cancellation never ticks", func(t *testing.T) {
		t.Parallel()
		b, tip := readableBed(t)
		for attempt := 0; attempt < 100; attempt++ {
			holder := New(b.root, newClock(fixtureNow).now)
			fetch := &scriptedFetch{answer: currentAt(tip)}
			ctx, cancel := context.WithCancel(context.Background())
			// The deadline reaches the loop already come, and the context
			// ends in the same act, so both cases of the loop's choice are
			// ready together.
			holder.Run(ctx, fetch.fetch, firedTimers{cancel: cancel})
			if fetch.count() != 0 {
				t.Fatalf("attempt %d started a fetch after cancellation", attempt)
			}
			if next := fetchState(holder).NextAt; !next.IsZero() {
				t.Fatalf("attempt %d left a deadline behind: %s", attempt, next)
			}
			cancel()
		}
	})
}

// firedTimers hands the loop a deadline that has already come, and optionally
// ends the loop's context in the same act.
type firedTimers struct {
	cancel context.CancelFunc
}

func (f firedTimers) NewTimer(time.Duration) Timer {
	timer := &fakeTimer{c: make(chan time.Time, 1)}
	timer.c <- time.Time{}
	if f.cancel != nil {
		f.cancel()
	}
	return timer
}

// TestRunReturnsOnlyAfterTheTickInFlight: a pass that is mid-advance must
// finish its compare-and-swap and clean up its per-operation ref before the
// process exits.
func TestRunReturnsOnlyAfterTheTickInFlight(t *testing.T) {
	t.Parallel()
	b, tip := readableBed(t)
	holder := New(b.root, newClock(fixtureNow).now)
	timers := newFakeTimers()
	fetch := &scriptedFetch{
		answer:  currentAt(tip),
		entered: make(chan struct{}, 1),
		held:    make(chan struct{}),
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := runLoop(holder, ctx, fetch.fetch, timers)

	timers.next().fire()
	<-fetch.entered

	cancel()
	select {
	case <-done:
		t.Fatalf("the loop returned while a tick was in flight")
	default:
	}

	mark := len(timers.recorded())
	close(fetch.held)
	<-done

	testutil.Expect(t, "no deadline is armed after the loop returns", timers.armedSince(mark), 0)
	testutil.Expect(t, "the next deadline", holder.Observe().Fetch.NextAt, time.Time{})
	testutil.Expect(t, "fetches", fetch.count(), 1)
}

func TestTheLoopAndItsReadersShareStateSafely(t *testing.T) {
	t.Parallel()
	b, tip := readableBed(t)
	holder := New(b.root, newClock(fixtureNow).now)
	timers := newFakeTimers()
	fetch := &scriptedFetch{answer: currentAt(tip)}
	ctx, cancel := context.WithCancel(context.Background())
	done := runLoop(holder, ctx, fetch.fetch, timers)

	var readers sync.WaitGroup
	for i := 0; i < 8; i++ {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for j := 0; j < 10; j++ {
				if state := holder.Observe().State; state != StateRead {
					t.Errorf("a reader saw %s while the loop ran", state)
					return
				}
			}
		}()
	}
	timer := timers.next()
	for i := 0; i < 5; i++ {
		timer = tickOnce(t, timers, fetch, timer)
	}
	readers.Wait()

	cancel()
	<-done
	testutil.Expect(t, "the loop kept ticking", fetch.count(), 5)
}

// TestEveryLedgerStateCarriesTheLoopsState: what the loop last found is most
// worth saying exactly when the ledger cannot be read.
func TestEveryLedgerStateCarriesTheLoopsState(t *testing.T) {
	t.Parallel()
	b, _ := singleMachineBed(t)
	holder := New(b.root, newClock(fixtureNow).now)
	timers := newFakeTimers()
	fetch := &scriptedFetch{answer: alwaysFailing("git fetch: the remote does not answer")}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := runLoop(holder, ctx, fetch.fetch, timers)
	drain(timers, timers.next())

	observation := holder.Observe()
	testutil.Expect(t, "state", observation.State, StateAbsent)
	testutil.Expect(t, "the loop's outcome reaches an unreadable ledger", observation.Fetch.Outcome, OutcomeFailed)
	testutil.Expect(t, "the loop's message", observation.Fetch.Message, "git fetch: the remote does not answer")

	cancel()
	<-done
}

// boundedAdvance is what the interface's wiring hands the loop.
func boundedAdvance(endpoint goal.Endpoint) (goal.AdvanceResult, error) {
	return goal.FetchAdvanceBounded(endpoint, FetchBudget)
}

// singleMachineBed is a converted checkout whose ledger commit sits on the
// dedicated single-machine branch and which has never accepted anything --
// the world a fresh clone walks into.
func singleMachineBed(t *testing.T) (*bed, string) {
	t.Helper()
	b := newBed(t)
	b.writeRoot(goal.SyncLocal)
	b.writeGoal(bedGoal("alpha", goal.StateQueued))
	tip := b.commit("the ledger")
	b.git("update-ref", goal.LocalLedgerBranch, tip)
	return b, tip
}
