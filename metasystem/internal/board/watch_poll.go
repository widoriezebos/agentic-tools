package board

import "time"

// PollWatch is the bounded poll: every tick is one "read again". The bridge
// compares each read with the last, so a tick with nothing changed is no
// event to its subscribers.
func PollWatch(ticks <-chan time.Time) *Watcher {
	watcher := newWatcher(false)
	go func() {
		for {
			select {
			case <-watcher.stop:
				return
			case _, open := <-ticks:
				if !open {
					return
				}
				watcher.signal()
			}
		}
	}()
	return watcher
}
