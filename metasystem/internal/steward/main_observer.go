package steward

import (
	"fmt"
	"io"
	"time"
)

// The runner flock outlives this watcher. Its timer and the cycle's keeper
// serialize lane work; the tick, notifications and engine follow run independently.
func (k *laneKeeping) watchMain(pass func(func() bool) error, stopping, followWaiting func() bool, clock *HandoffClock, log io.Writer) func() {
	quit, done := make(chan struct{}), make(chan struct{})
	held := func() bool {
		select {
		case <-quit:
			return true
		default:
		}
		return stopping() || followWaiting != nil && followWaiting()
	}
	last := ""
	step := func() {
		k.mu.Lock()
		defer k.mu.Unlock()
		select {
		case <-quit:
			return
		default:
		}
		if stopping() {
			return
		}
		line := ""
		if err := pass(held); err != nil {
			line = err.Error()
		}
		if line != "" && line != last {
			fmt.Fprintln(log, line)
		}
		last = line
	}
	newTicker := func(d time.Duration) (<-chan time.Time, func()) {
		ticker := time.NewTicker(d)
		return ticker.C, ticker.Stop
	}
	if clock != nil && clock.Ticker != nil {
		newTicker = clock.Ticker
	}
	ticks, stop := newTicker(laneRecheck)
	step()
	go func() {
		defer close(done)
		defer stop()
		for {
			select {
			case <-quit:
				return
			case <-ticks:
				step()
			}
		}
	}()
	return func() { close(quit); <-done }
}
