package adapter

import (
	"context"
	"errors"
	"time"
)

var ErrStartGateExpired = errors.New("start gate deadline expired")

type StartGateDependencies struct {
	Now    func() time.Time
	After  func(time.Duration) <-chan time.Time
	Events <-chan struct{}
	Exists func() bool
}

type StartGateResult struct {
	Opened    bool
	ElapsedMS int64
}

// WaitStartGate waits for the runner-owned gate event. The timer decides only
// after one last existence check, so an already-published gate wins at the
// boundary even if its notification and the timer become ready together.
func WaitStartGate(ctx context.Context, timeout time.Duration, deps StartGateDependencies) (StartGateResult, error) {
	started := deps.Now()
	result := func(open bool) StartGateResult {
		elapsed := deps.Now().Sub(started)
		if elapsed < 0 {
			elapsed = 0
		}
		return StartGateResult{Opened: open, ElapsedMS: elapsed.Milliseconds()}
	}
	if deps.Exists() {
		return result(true), nil
	}
	timer := deps.After(timeout)
	for {
		select {
		case <-ctx.Done():
			return result(false), ctx.Err()
		case <-deps.Events:
			if deps.Exists() {
				return result(true), nil
			}
		case <-timer:
			if deps.Exists() {
				return result(true), nil
			}
			return result(false), ErrStartGateExpired
		}
	}
}
