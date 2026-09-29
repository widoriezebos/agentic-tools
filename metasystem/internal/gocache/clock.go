package gocache

import "time"

// SystemClock is the wall clock: the one place in this package that reads
// time (disk-lifetimes rule A9). Every trimmer function takes now from its
// caller; the steward passes its own clock.
func SystemClock() time.Time { return time.Now() }
