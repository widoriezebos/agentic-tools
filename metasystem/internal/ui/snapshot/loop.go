package snapshot

import (
	"context"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// The freshness loop carries this clone's accepted ref forward so that
// another machine's publication reaches a reader without anyone typing a
// command. It is the server's, never a request's: a request answers from the
// ref as it stands and moves nothing, while the ref moves only here, on a
// schedule, under the engine's own acceptance gates.

const (
	// connectedInterval is how often the loop looks while a browser is
	// reading; idleInterval is how often it looks when nobody is.
	connectedInterval = 5 * time.Second
	idleInterval      = 30 * time.Second
	// connectedWindow is how long after a request a browser still counts as
	// connected.
	connectedWindow = 30 * time.Second
	// backoffCap bounds the wait after repeated failures, so a loop that has
	// been failing all night still notices the minute the cause is fixed.
	backoffCap = 5 * time.Minute
	// maxBackoffDoublings bounds the arithmetic itself, not just its result.
	maxBackoffDoublings = 10

	// FetchBudget is how long the loop's one network operation may take
	// before its process group is killed and the tick ends as a failure. A
	// loop whose whole purpose is freshness stops being fresh the moment a
	// hung transport holds it, with no way back but a kill.
	FetchBudget = 60 * time.Second
)

// Cadence names which interval the current deadline was computed from.
const (
	CadenceConnected = "connected"
	CadenceIdle      = "idle"
)

// Outcome is what the loop's last tick did.
type Outcome string

const (
	// OutcomeNever is a loop that has not completed a tick yet.
	OutcomeNever   Outcome = "never"
	OutcomeRunning Outcome = "running"
	// OutcomeAdvanced is a tick that validated a new canonical tip and moved
	// the accepted ref onto it.
	OutcomeAdvanced Outcome = "advanced"
	// OutcomeCurrent is a tick that found the canonical branch at the tip
	// this clone already accepted.
	OutcomeCurrent Outcome = "current"
	// OutcomeFailed is a tick the transport, a gate, or the validator ended.
	OutcomeFailed Outcome = "failed"
)

// FetchState is what the loop last did and when it will look again. Every
// observation carries it, so a reader can always tell a stale tip from a
// broken one.
type FetchState struct {
	Outcome    Outcome
	StartedAt  time.Time
	FinishedAt time.Time
	Tip        string
	Detail     string
	Message    string
	Failures   int
	Cadence    string
	// NextAt is when the next tick is due. It is zero while no loop is
	// running, which says the server is stopping rather than promising a
	// fetch nobody will make.
	NextAt time.Time
}

// Fetch is one pass of the engine's read-side advance.
type Fetch func(goal.Endpoint) (goal.AdvanceResult, error)

// Timer and Timers are the loop's clock. Production passes WallTimers; a test
// passes its own and decides when the loop looks.
type Timer interface {
	C() <-chan time.Time
	Stop()
}

type Timers interface {
	NewTimer(time.Duration) Timer
}

type WallTimers struct{}

type wallTimer struct {
	c    <-chan time.Time
	stop func()
}

func (WallTimers) NewTimer(after time.Duration) Timer {
	timer := time.NewTimer(after)
	return wallTimer{c: timer.C, stop: func() { timer.Stop() }}
}

func (t wallTimer) C() <-chan time.Time { return t.c }
func (t wallTimer) Stop()               { t.stop() }

// Run carries the accepted ref forward until the context ends. It returns
// once no tick is in flight, so a pass that is mid-advance finishes its
// compare-and-swap and cleans up its per-operation ref before the process
// exits.
//
// Cancellation is checked, never merely selected for: a timer that comes
// ready in the same instant as cancellation could otherwise win a uniform
// choice and start one more fetch on the way out.
func (h *Holder) Run(ctx context.Context, fetch Fetch, timers Timers) {
	h.start()
	for {
		if ctx.Err() != nil {
			h.stop()
			return
		}
		timer := timers.NewTimer(h.until())
		select {
		case <-ctx.Done():
			timer.Stop()
			h.stop()
			return
		case <-h.wake:
			// A request shortened the cadence. Re-arm against the new
			// deadline; nothing else about a request reaches the loop.
			timer.Stop()
		case <-timer.C():
			timer.Stop()
			if ctx.Err() != nil {
				h.stop()
				return
			}
			h.Advance(fetch)
		}
	}
}

// Advance runs one pass of the read-side advance now and waits for it.
//
// It exists for the one caller that cannot wait for a cadence: a human act
// published through this server has landed on the canonical branch, and the
// board must not move the card until this clone's accepted ref carries it.
// The loop's own ticks go through here too, so the invariant that only one
// advance is ever in flight survives a request arriving mid-tick.
func (h *Holder) Advance(fetch Fetch) {
	h.advancing.Lock()
	defer h.advancing.Unlock()
	h.tick(fetch)
}

// tick runs one advance inline, so a second can never start while one is in
// flight. The mutex is never held across git.
func (h *Holder) tick(fetch Fetch) {
	h.mu.Lock()
	h.fetch.Outcome = OutcomeRunning
	h.fetch.StartedAt = h.now().UTC()
	h.fetch.FinishedAt = time.Time{}
	h.mu.Unlock()

	// The endpoint is resolved every tick because git configuration changes
	// independently of anything this loop holds.
	var result goal.AdvanceResult
	endpoint, err := goal.ResolveEndpoint(h.root)
	if err == nil {
		result, err = fetch(endpoint)
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	finished := h.now().UTC()
	h.fetch.FinishedAt = finished
	switch {
	case err != nil:
		// A failed tick found nothing, so it reports nothing but its cause.
		h.fetch.Outcome = OutcomeFailed
		h.fetch.Tip, h.fetch.Detail = "", ""
		h.fetch.Message = err.Error()
		h.fetch.Failures++
	case result.Advanced:
		h.fetch.Outcome = OutcomeAdvanced
		h.fetch.Tip, h.fetch.Detail, h.fetch.Message = result.Tip, result.Detail, ""
		h.fetch.Failures = 0
	default:
		h.fetch.Outcome = OutcomeCurrent
		h.fetch.Tip, h.fetch.Detail, h.fetch.Message = result.Tip, result.Detail, ""
		h.fetch.Failures = 0
	}
	h.armLocked(finished)
}

// start makes the first tick due at once, so a clone with no accepted ref
// heals within seconds of the server coming up rather than within a cadence.
func (h *Holder) start() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.stopped = false
	h.next = h.now().UTC()
	h.fetch.NextAt = h.next
	if h.fetch.Outcome == "" {
		h.fetch.Outcome = OutcomeNever
	}
	h.fetch.Cadence = cadenceOf(connected(h.next, h.lastObserve))
}

func (h *Holder) stop() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.stopped = true
	h.next = time.Time{}
	h.fetch.NextAt = time.Time{}
}

func (h *Holder) until() time.Duration {
	h.mu.Lock()
	defer h.mu.Unlock()
	wait := h.next.Sub(h.now().UTC())
	if wait < 0 {
		return 0
	}
	return wait
}

// armLocked sets the deadline after a tick that ended at the given instant.
func (h *Holder) armLocked(at time.Time) {
	if h.stopped {
		return
	}
	wait, cadence := nextDue(connected(at, h.lastObserve), h.fetch.Failures)
	h.next = at.Add(wait)
	h.fetch.Cadence = cadence
	h.fetch.NextAt = h.next
}

// markConnectedLocked records that a browser is reading and pulls the loop's
// deadline in to the connected cadence when that is sooner. A request can
// shorten the wait to the connected interval but never below the backoff's
// wait, and never to "now": it sets the cadence and starts nothing.
func (h *Holder) markConnectedLocked(now time.Time) {
	h.lastObserve = now
	if h.stopped {
		return
	}
	wait, cadence := nextDue(true, h.fetch.Failures)
	candidate := now.Add(wait)
	if h.next.IsZero() || candidate.Before(h.next) {
		h.next = candidate
		h.fetch.Cadence = cadence
	}
	h.fetch.NextAt = h.next
	select {
	case h.wake <- struct{}{}:
	default:
	}
}

// nextDue is how long to wait before looking again, and which interval that
// wait came from. Each consecutive failure doubles the wait up to the cap, so
// an unreachable remote is not asked every five seconds all night.
func nextDue(connected bool, failures int) (time.Duration, string) {
	wait := idleInterval
	if connected {
		wait = connectedInterval
	}
	doublings := failures
	if doublings > maxBackoffDoublings {
		doublings = maxBackoffDoublings
	}
	for i := 0; i < doublings; i++ {
		wait *= 2
		if wait >= backoffCap {
			return backoffCap, cadenceOf(connected)
		}
	}
	return wait, cadenceOf(connected)
}

func connected(at, lastObserve time.Time) bool {
	return !lastObserve.IsZero() && at.Sub(lastObserve) <= connectedWindow
}

func cadenceOf(connected bool) string {
	if connected {
		return CadenceConnected
	}
	return CadenceIdle
}
