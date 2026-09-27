package main

import (
	"context"
	"sync"
	"time"

	metarun "github.com/widoriezebos/agentic-tools/metasystem/internal/run"
)

// virtualWaitClock is one test's clock for the real wait owner. Time moves only
// when the wait sleeps, and every deadline the wait derives (for a source read,
// an actionable check or the runtime's wait-delivery call) expires only when
// this clock passes it. Real work inside the wait — an adapter subprocess on a
// loaded machine — therefore never races a wall-clock budget, while the wait
// still reaches its deadline through its own loop.
type virtualWaitClock struct {
	mu     sync.Mutex
	now    time.Time
	boot   time.Duration
	bootID string
	timers map[*virtualDeadline]struct{}
}

func newVirtualWaitClock(start time.Time) *virtualWaitClock {
	return &virtualWaitClock{now: start.UTC(), boot: time.Hour, bootID: "virtual-wait-boot", timers: map[*virtualDeadline]struct{}{}}
}

// apply installs the clock as one unit, as runWaitCommandOnClock requires.
func (c *virtualWaitClock) apply(options *metarun.WaitOptions) {
	options.Now = c.Now
	options.BootClock = c.BootClock
	options.Sleep = c.Sleep
	options.WithTimeout = c.WithTimeout
}

func (c *virtualWaitClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *virtualWaitClock) BootClock() (string, time.Duration, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.bootID, c.boot, nil
}

func (c *virtualWaitClock) Sleep(ctx context.Context, duration time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.mu.Lock()
	if duration > 0 {
		c.now = c.now.Add(duration)
		c.boot += duration
	}
	var expired []*virtualDeadline
	for timer := range c.timers {
		if !c.now.Before(timer.at) {
			expired = append(expired, timer)
			delete(c.timers, timer)
		}
	}
	c.mu.Unlock()
	for _, timer := range expired {
		timer.expire()
	}
	return nil
}

func (c *virtualWaitClock) WithTimeout(parent context.Context, duration time.Duration) (context.Context, context.CancelFunc) {
	base, cancel := context.WithCancel(parent)
	c.mu.Lock()
	timer := &virtualDeadline{Context: base, cancel: cancel, at: c.now.Add(duration)}
	due := duration <= 0
	if !due {
		c.timers[timer] = struct{}{}
	}
	c.mu.Unlock()
	if due {
		timer.expire()
	}
	return timer, func() {
		c.mu.Lock()
		delete(c.timers, timer)
		c.mu.Unlock()
		cancel()
	}
}

// virtualDeadline is a context whose deadline is on the virtual clock: it
// reports context.DeadlineExceeded, as context.WithTimeout would, once the
// clock passes it, and otherwise follows its parent.
type virtualDeadline struct {
	context.Context
	cancel context.CancelFunc
	at     time.Time

	mu      sync.Mutex
	expired bool
}

func (d *virtualDeadline) expire() {
	d.mu.Lock()
	d.expired = true
	d.mu.Unlock()
	d.cancel()
}

func (d *virtualDeadline) Deadline() (time.Time, bool) { return d.at, true }

func (d *virtualDeadline) Err() error {
	d.mu.Lock()
	expired := d.expired
	d.mu.Unlock()
	if expired && d.Context.Err() != nil {
		return context.DeadlineExceeded
	}
	return d.Context.Err()
}
