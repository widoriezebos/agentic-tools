package testenv

import (
	"fmt"
	"time"
)

// AwaitPause paces Await's observations. It spaces two looks at a fact so a
// wait does not spin; it never decides whether the wait succeeds.
const AwaitPause = 10 * time.Millisecond

// awaitReserve is the part of the test binary's deadline Await leaves when it
// gives up, so the failure is the test's own, naming what it awaited, and the
// test's cleanups still run before go test's timeout panic.
const awaitReserve = 30 * time.Second

// AwaitTB is the part of testing.TB Await reports through; a *testing.T
// also hands it the test binary's deadline (Deadline).
type AwaitTB interface {
	Helper()
	Fatalf(format string, args ...any)
}

// Await observes until observed reports true, pausing AwaitPause between two
// observations. It is the wait for an event that sends no signal a test can
// block on: a file another process writes, a process leaving the kernel's
// table, a record reaching a state. It never fails because the event took
// longer: a loaded or slower host makes the test slower, never red. Its only
// bound is the test binary's own deadline (go test -timeout, read through
// t.Deadline): when less than awaitReserve of it is left, Await fails naming
// what it awaited. A t without a deadline waits for the event alone.
func Await(t AwaitTB, what string, observed func() bool) {
	t.Helper()
	AwaitOr(t, what, observed, nil)
}

// AwaitOr is Await whose failure also carries what giveUp gathers when the
// wait gives up, such as a hung process's goroutine dump; giveUp runs once,
// only then.
func AwaitOr(t AwaitTB, what string, observed func() bool, giveUp func() string) {
	t.Helper()
	await(t, what, observed, awaitPause, deadlineRemaining(t), giveUp)
}

// AwaitError waits like Await but returns a deadline error instead of failing
// the test, so worker goroutines can finish their cleanup and return to callers.
func AwaitError(t interface{ Deadline() (time.Time, bool) }, what string, observed func() bool) error {
	return awaitError(what, observed, awaitPause, deadlineRemaining(t), nil)
}

// DeadlineRemaining reports the time left before the test binary's deadline,
// so an owned test subprocess can share its parent's remaining timeout.
func DeadlineRemaining(t interface{ Deadline() (time.Time, bool) }) (time.Duration, bool) {
	return deadlineRemaining(t)()
}

func awaitPause() { time.Sleep(AwaitPause) }

func deadlineRemaining(t any) func() (time.Duration, bool) {
	remaining := func() (time.Duration, bool) { return 0, false }
	if bounded, ok := t.(interface{ Deadline() (time.Time, bool) }); ok {
		if deadline, ok := bounded.Deadline(); ok {
			remaining = func() (time.Duration, bool) { return time.Until(deadline), true }
		}
	}
	return remaining
}

func await(t AwaitTB, what string, observed func() bool, pause func(), remaining func() (time.Duration, bool), giveUp func() string) {
	t.Helper()
	if err := awaitError(what, observed, pause, remaining, giveUp); err != nil {
		t.Fatalf("%v", err)
	}
}

func awaitError(what string, observed func() bool, pause func(), remaining func() (time.Duration, bool), giveUp func() string) error {
	for !observed() {
		if left, bounded := remaining(); bounded && left <= awaitReserve {
			report := ""
			if giveUp != nil {
				report = "; " + giveUp()
			}
			return fmt.Errorf("still awaiting %s with %s left before the test binary's deadline%s", what, left.Round(time.Millisecond), report)
		}
		pause()
	}
	return nil
}
