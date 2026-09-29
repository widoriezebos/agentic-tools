package batchowner

import "sync"

// BatchOwnerCadence owns every cadence tick admitted by one landing-owner
// component. A stop closes admission before waiting, so cleanup cannot race a
// tick that is still using the component's checkout.
type BatchOwnerCadence struct {
	mu       sync.Mutex
	stopped  bool
	Stopping chan struct{}
	Done     chan struct{}
	doneOnce sync.Once
	ticks    sync.WaitGroup
}

func NewBatchOwnerCadence() *BatchOwnerCadence {
	return &BatchOwnerCadence{Stopping: make(chan struct{}), Done: make(chan struct{})}
}

func (cadence *BatchOwnerCadence) start(tick func()) bool {
	cadence.mu.Lock()
	defer cadence.mu.Unlock()
	if cadence.stopped {
		return false
	}
	cadence.ticks.Add(1)
	BatchOwnerCadenceStart(func() {
		defer cadence.ticks.Done()
		tick()
	})
	return true
}

func (cadence *BatchOwnerCadence) requestStop() {
	cadence.mu.Lock()
	defer cadence.mu.Unlock()
	if cadence.stopped {
		return
	}
	cadence.stopped = true
	close(cadence.Stopping)
}

func (cadence *BatchOwnerCadence) Stop() {
	cadence.requestStop()
	cadence.ticks.Wait()
	cadence.doneOnce.Do(func() { close(cadence.Done) })
	<-cadence.Done
}
