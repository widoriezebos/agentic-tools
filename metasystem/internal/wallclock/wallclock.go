// Package wallclock owns the lifecycle clock port — wall readings and waits —
// that the delegate lifecycle and the adapter supervisor both inject, so
// tests can drive time without sleeping. seat/launch's clock has a different
// shape (After) and is not this port.
package wallclock

import "time"

// Clock is the time every deadline, poll and stamp reads.
type Clock interface {
	Now() time.Time
	Sleep(d time.Duration)
}

type system struct{}

func (system) Now() time.Time        { return time.Now() }
func (system) Sleep(d time.Duration) { time.Sleep(d) }

// System is the real wall clock.
func System() Clock { return system{} }
