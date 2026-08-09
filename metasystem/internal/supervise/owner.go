package supervise

import (
	"fmt"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// The owner loop: D-1's check every base interval, D-2's breaker
// around component health, exits that are teardowns (D-1), all
// dependencies behind small consumer-defined interfaces so the loop's
// behavior is provable with fakes and the process-touching edges live
// in cmd wiring.

// Component names the two supervised components.
type Component string

const (
	Watcher Component = "watcher"
	Reaper  Component = "reaper"
)

// Held is what the owner HOLDS IN MEMORY about a component it
// launched: teardown uses exactly this, never a re-read state file
// (SLC-F-001 — the trap that killed successors is the shape this
// type exists to prevent).
type Held struct {
	Component Component
	Tag       string
	Identity  identity.Ref
	// Generation the component belongs to (REG-2 pairing).
	Generation int64
}

// Checkout reads the three D-1 inputs, three-way each.
type Checkout interface {
	RootState() FileState
	StateFileState() FileState
	// Currency reports what the lock's owner file says about self.
	Currency() CurrencyState
	// StateNamesSelf reports whether state.json's owner stanza names
	// this owner (for the republish repair, SLC-R5-016). Only
	// meaningful when StateFileState is Present.
	StateNamesSelf() (bool, error)
	// PublishState atomically writes state.json from held identities;
	// implementations MUST re-read the lock immediately before the
	// rename and abort if it no longer names self (fenced publication,
	// SLC-R4-001).
	PublishState(held []Held) error
}

// Components launches and observes the supervised pair.
type Components interface {
	// Launch starts one component detached and returns its identity.
	Launch(component Component, tag string) (identity.Ref, error)
	// Observe reports one component's three-way health: identity
	// alive + heartbeat fresh = Healthy.
	Observe(held Held) Observation
	// GroupCount counts live members of the held components' process
	// groups (the ceiling's input).
	GroupCount(held []Held) (int, error)
	// Stop signals one held identity's group TERM-then-KILL within
	// the component stop ceiling and reports whether it is PROVEN
	// gone (three-way: false only on definitive survival or unknown).
	Stop(held Held) (proven bool)
}

// Ledger is the owner's slice of the registry: the write-ahead
// appends (REG-2) and the terminal.
type Ledger interface {
	// AppendRelaunched is the GATING write-ahead (SLC-R6-006): if it
	// fails, nothing launches this cycle.
	AppendRelaunched(generation int64, watcherTag, reaperTag string, retiredThrough int64) error
	// AppendLaunched records one component's identity; failures are
	// retried at every observation (SLC-R6-006).
	AppendLaunched(held Held) error
	// AppendExited is best-effort (REG-5): an owner that cannot speak
	// must still die.
	AppendExited(reason, diagnosis string, teardownComplete bool)
}

// Intents reads the shutdown-intent channel (D-1, SLC-R7-009).
type Intents interface {
	// LatchShutdown reports whether a fresh intent names this owner
	// AT EXIT-INITIATION (SLC-R9-004); it consumes the file either
	// way and reports stale or foreign intents without honoring them.
	LatchShutdown() bool
}

// Exit is how the loop ends: reason plus the teardown outcome.
type Exit struct {
	Reason           string
	Diagnosis        string
	TeardownComplete bool
}

// Owner runs the supervision lifecycle for one checkout.
type Owner struct {
	Checkout   Checkout
	Components Components
	Ledger     Ledger
	Intents    Intents

	// Numbers are D-6's decisions.
	BaseInterval  time.Duration
	Ceiling       int
	Breaker       Breaker
	Establishment Establishment

	// Sleep is injectable for tests; nil means time.Sleep.
	Sleep func(time.Duration)

	// TagPrefix mints component tags: <prefix>-<component>-<gen>-<n>.
	TagPrefix string

	held           []Held
	generation     int64
	retiredThrough int64
	published      bool
	launchedRetry  []Held // launched appends still owed (SLC-R6-006)
	relaunchDue    time.Time
	appendFailures int
}

// Run loops until an exit condition fires, then tears down BY HELD
// IDENTITY, appends the terminal, and returns the exit.
func (o *Owner) Run() Exit {
	if o.Sleep == nil {
		o.Sleep = time.Sleep
	}
	for {
		exit := o.Cycle(time.Now())
		if exit != nil {
			complete := o.teardownHeld()
			exit.TeardownComplete = complete
			o.Ledger.AppendExited(exit.Reason, exit.Diagnosis, complete)
			return *exit
		}
		o.Sleep(o.BaseInterval)
	}
}

// Cycle runs ONE base-interval observation: D-1 first, then
// establishment, then the breaker's business. It returns nil to
// continue or the exit to perform. Exposed so tests drive the loop
// deterministically.
func (o *Owner) Cycle(now time.Time) *Exit {
	// D-1: the purpose/currency check runs every cycle, independent
	// of backoff (SLC-R2-008), but only arms after first publication
	// (SLC-R1-004).
	if o.published {
		switch Classify(o.Checkout.RootState(), o.Checkout.Currency(), o.Checkout.StateFileState()) {
		case PurposeGone:
			return o.exitVia("purpose-gone", "checkout root or state token definitively absent")
		case Superseded:
			return o.exitVia("superseded", "the lock names another identity")
		case Blind:
			return nil // log-and-continue: no relaunch on indeterminacy
		}
		// Stale state is REPAIRED, not tolerated (SLC-R5-016).
		if namesSelf, err := o.Checkout.StateNamesSelf(); err == nil && !namesSelf {
			if err := o.Checkout.PublishState(o.held); err != nil {
				// A fenced publication that aborts means we lost
				// currency mid-cycle; the next cycle classifies it.
				return nil
			}
		}
	}

	// Establishment: first publication within the deadline or give up.
	if !o.published {
		if err := o.establish(); err != nil {
			if o.Establishment.Observe(false) {
				return o.exitVia("establishment-failed", err.Error())
			}
			return nil
		}
		o.Establishment.Observe(true)
		o.published = true
		return nil
	}

	// Owed launched appends retry every observation; persistent
	// failure is itself an incrementing observation (SLC-R6-006).
	unrecordable := o.retryOwedAppends()

	// The ceiling is a duration property (SLC-R4-010): overshoot is
	// an incrementing observation and the set stops NOW.
	members, err := o.Components.GroupCount(o.held)
	observation := o.observeComponents()
	if err == nil {
		if stop, _ := CeilingVerdict(members, o.Ceiling); stop {
			// Stop the set NOW: overshoot survives at most one
			// interval (SLC-R4-010), and the observation increments.
			o.stopHeldSet()
			observation = Failing
		}
	}
	if unrecordable && observation == Healthy {
		observation = Failing
	}

	verdict := o.Breaker.Advance(observation)
	switch {
	case verdict.GiveUp:
		return o.exitVia("giving-up", fmt.Sprintf("%d consecutive failing observations", o.Breaker.Consecutive))
	case verdict.SkipRelaunch:
		return nil
	}
	if observation == Failing {
		if o.relaunchDue.IsZero() {
			o.relaunchDue = now.Add(verdict.RelaunchAfter)
		}
		if !now.Before(o.relaunchDue) {
			o.relaunchSet()
			o.relaunchDue = time.Time{}
		}
	}
	return nil
}

// exitVia builds a voluntary exit. The shutdown-intent channel only
// relabels SIGNAL-induced exits (ExitOnSignal); revocation,
// replacement, and the breaker keep their own reasons.
func (o *Owner) exitVia(reason, diagnosis string) *Exit {
	return &Exit{Reason: reason, Diagnosis: diagnosis}
}

// ExitOnSignal is the TERM/INT path: latch the intent FIRST, then
// teardown, then the terminal (D-1).
func (o *Owner) ExitOnSignal() Exit {
	reason := "terminated"
	if o.Intents.LatchShutdown() {
		reason = "shutdown"
	}
	complete := o.teardownHeld()
	o.Ledger.AppendExited(reason, "signal-induced exit", complete)
	return Exit{Reason: reason, TeardownComplete: complete}
}

// establish performs the first launch_set + publication.
func (o *Owner) establish() error {
	if err := o.relaunchSet(); err != nil {
		return err
	}
	if err := o.Checkout.PublishState(o.held); err != nil {
		return fmt.Errorf("first publication: %w", err)
	}
	return nil
}

// relaunchSet is launch_set: stop the old set verifying deaths for
// the watermark (SLC-R8-002), write-ahead relaunched (GATING), then
// launch and record each component as its identity is captured.
func (o *Owner) relaunchSet() error {
	// Stop the current set, verifying deaths; re-verify every
	// unretired generation for the contiguous watermark.
	verified := map[int64]bool{}
	recorded := []int64{}
	var unproven []Held
	byGeneration := map[int64][]Held{}
	for _, held := range o.held {
		byGeneration[held.Generation] = append(byGeneration[held.Generation], held)
	}
	for generation, set := range byGeneration {
		recorded = append(recorded, generation)
		all := true
		for _, held := range set {
			if !o.Components.Stop(held) {
				all = false
				// An unproven identity stays HELD: teardown and the
				// sweep must still be able to find it, and its
				// generation pins the watermark (SLC-R9-003).
				unproven = append(unproven, held)
			}
		}
		verified[generation] = all
	}
	o.retiredThrough = RetireWatermark(o.retiredThrough, verified, recorded)

	next := o.generation + 1
	watcherTag := fmt.Sprintf("%s-watcher-%d", o.TagPrefix, next)
	reaperTag := fmt.Sprintf("%s-reaper-%d", o.TagPrefix, next)
	// WRITE-AHEAD GATES THE LAUNCH (SLC-R6-006): an owner that cannot
	// record intent must not create processes.
	if err := o.Ledger.AppendRelaunched(next, watcherTag, reaperTag, o.retiredThrough); err != nil {
		return fmt.Errorf("write-ahead refused, launching nothing: %w", err)
	}
	o.generation = next
	o.held = unproven
	for component, tag := range map[Component]string{Watcher: watcherTag, Reaper: reaperTag} {
		ref, err := o.Components.Launch(component, tag)
		if err != nil {
			continue // the breaker sees the missing component next cycle
		}
		held := Held{Component: component, Tag: tag, Identity: ref, Generation: next}
		o.held = append(o.held, held)
		if err := o.Ledger.AppendLaunched(held); err != nil {
			o.launchedRetry = append(o.launchedRetry, held)
		}
	}
	return nil
}

func (o *Owner) retryOwedAppends() (persistentFailure bool) {
	var still []Held
	for _, held := range o.launchedRetry {
		if err := o.Ledger.AppendLaunched(held); err != nil {
			still = append(still, held)
		}
	}
	o.launchedRetry = still
	if len(still) > 0 {
		o.appendFailures++
		return true
	}
	o.appendFailures = 0
	return false
}

func (o *Owner) observeComponents() Observation {
	if len(o.currentGenerationHeld()) < 2 {
		return Failing
	}
	worst := Healthy
	for _, held := range o.currentGenerationHeld() {
		switch o.Components.Observe(held) {
		case Failing:
			return Failing
		case Indeterminable:
			worst = Indeterminable
		}
	}
	return worst
}

func (o *Owner) currentGenerationHeld() []Held {
	var current []Held
	for _, held := range o.held {
		if held.Generation == o.generation {
			current = append(current, held)
		}
	}
	return current
}

func (o *Owner) stopHeldSet() {
	for _, held := range o.currentGenerationHeld() {
		o.Components.Stop(held)
	}
}

// teardownHeld stops exactly what this owner launched, by held
// identity, and reports whether EVERY death was proven — the honest
// teardownComplete (SLC-R4-012). It must work with the checkout gone
// (SLC-R7-005): implementations of Components.Stop use held memory
// and system calls only.
func (o *Owner) teardownHeld() (complete bool) {
	complete = true
	for _, held := range o.held {
		if !o.Components.Stop(held) {
			complete = false
		}
	}
	return complete
}
