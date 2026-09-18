package goal

import (
	"sync"
	"time"
)

type attentionTimer interface {
	C() <-chan time.Time
	Stop()
}

type attentionTimerSource interface {
	NewTimer(time.Duration) attentionTimer
	NewTicker(time.Duration) attentionTimer
}

type wallAttentionTimerSource struct{}

type wallAttentionTimer struct {
	c    <-chan time.Time
	stop func()
}

func (wallAttentionTimerSource) NewTimer(after time.Duration) attentionTimer {
	timer := time.NewTimer(after)
	return wallAttentionTimer{
		c: timer.C,
		stop: func() {
			timer.Stop()
		},
	}
}

func (wallAttentionTimerSource) NewTicker(every time.Duration) attentionTimer {
	ticker := time.NewTicker(every)
	return wallAttentionTimer{
		c: ticker.C,
		stop: func() {
			ticker.Stop()
		},
	}
}

func (t wallAttentionTimer) C() <-chan time.Time {
	return t.c
}

func (t wallAttentionTimer) Stop() {
	t.stop()
}

var captureTipTimers = struct {
	sync.RWMutex
	source attentionTimerSource
}{source: wallAttentionTimerSource{}}

func currentCaptureTipTimerSource() attentionTimerSource {
	captureTipTimers.RLock()
	defer captureTipTimers.RUnlock()
	return captureTipTimers.source
}

func replaceCaptureTipTimerSource(source attentionTimerSource) attentionTimerSource {
	captureTipTimers.Lock()
	defer captureTipTimers.Unlock()
	previous := captureTipTimers.source
	captureTipTimers.source = source
	return previous
}
