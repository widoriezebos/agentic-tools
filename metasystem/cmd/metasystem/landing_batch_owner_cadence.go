package main

import "sync"

// batchOwnerCadence owns every cadence tick admitted by one landing-owner
// component. A stop closes admission before waiting, so cleanup cannot race a
// tick that is still using the component's checkout.
type batchOwnerCadence struct {
	mu       sync.Mutex
	stopped  bool
	stopping chan struct{}
	done     chan struct{}
	doneOnce sync.Once
	ticks    sync.WaitGroup
}

func newBatchOwnerCadence() *batchOwnerCadence {
	return &batchOwnerCadence{stopping: make(chan struct{}), done: make(chan struct{})}
}

func (cadence *batchOwnerCadence) start(tick func()) bool {
	cadence.mu.Lock()
	defer cadence.mu.Unlock()
	if cadence.stopped {
		return false
	}
	cadence.ticks.Add(1)
	batchOwnerCadenceStart(func() {
		defer cadence.ticks.Done()
		tick()
	})
	return true
}

func (cadence *batchOwnerCadence) requestStop() {
	cadence.mu.Lock()
	defer cadence.mu.Unlock()
	if cadence.stopped {
		return
	}
	cadence.stopped = true
	close(cadence.stopping)
}

func (cadence *batchOwnerCadence) stop() {
	cadence.requestStop()
	cadence.ticks.Wait()
	cadence.doneOnce.Do(func() { close(cadence.done) })
	<-cadence.done
}
