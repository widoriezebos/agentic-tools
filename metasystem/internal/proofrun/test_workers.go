package proofrun

import (
	"context"
	"fmt"
	"sync"
)

type testWorkerPoolContextKey struct{}

type testWorkerPool struct {
	mu        sync.Mutex
	capacity  int
	available int
	changed   chan struct{}
	waiters   int
}

// withTestWorkerPool gives one test attempt a finite adapter-worker pool.
// Nested coordinators preserve the parent's pool so every group and native
// leaf in an attempt accounts against the same allowance.
func withTestWorkerPool(ctx context.Context, workers int) context.Context {
	if _, ok := ctx.Value(testWorkerPoolContextKey{}).(*testWorkerPool); ok {
		return ctx
	}
	if workers < 1 {
		workers = 1
	}
	pool := &testWorkerPool{capacity: workers, available: workers, changed: make(chan struct{})}
	return context.WithValue(ctx, testWorkerPoolContextKey{}, pool)
}

// acquireTestWorkers atomically reserves workers from the attempt pool. It
// never retains a partial grant while waiting. The returned release owns that
// grant and is safe to call more than once.
func acquireTestWorkers(ctx context.Context, workers int) (release func(), err error) {
	pool, err := testWorkerPoolForRequest(ctx, workers)
	if err != nil {
		return nil, err
	}

	pool.mu.Lock()
	for pool.available < workers {
		if err := ctx.Err(); err != nil {
			pool.mu.Unlock()
			return nil, err
		}
		changed := pool.changed
		pool.waiters++
		pool.mu.Unlock()
		select {
		case <-ctx.Done():
			pool.mu.Lock()
			pool.waiters--
			pool.mu.Unlock()
			return nil, ctx.Err()
		case <-changed:
			pool.mu.Lock()
			pool.waiters--
		}
	}
	if err := ctx.Err(); err != nil {
		pool.mu.Unlock()
		return nil, err
	}
	pool.available -= workers
	pool.mu.Unlock()
	return testWorkerRelease(pool, workers), nil
}

// tryAcquireTestWorkers reserves the complete request when it fits now. A
// failed attempt registers one pending request and returns the pool change
// that makes retrying useful. The caller owns stopWaiting until it either
// retries or abandons the request.
func tryAcquireTestWorkers(ctx context.Context, workers int) (release func(), changed <-chan struct{}, stopWaiting func(), acquired bool, err error) {
	pool, err := testWorkerPoolForRequest(ctx, workers)
	if err != nil {
		return nil, nil, nil, false, err
	}
	pool.mu.Lock()
	if err := ctx.Err(); err != nil {
		pool.mu.Unlock()
		return nil, nil, nil, false, err
	}
	if pool.available >= workers {
		pool.available -= workers
		pool.mu.Unlock()
		return testWorkerRelease(pool, workers), nil, nil, true, nil
	}
	changed = pool.changed
	pool.waiters++
	pool.mu.Unlock()
	var once sync.Once
	stopWaiting = func() {
		once.Do(func() {
			pool.mu.Lock()
			pool.waiters--
			pool.mu.Unlock()
		})
	}
	return nil, changed, stopWaiting, false, nil
}

func testWorkerPoolForRequest(ctx context.Context, workers int) (*testWorkerPool, error) {
	pool, ok := ctx.Value(testWorkerPoolContextKey{}).(*testWorkerPool)
	if !ok || pool == nil {
		return nil, fmt.Errorf("test worker pool is absent from context")
	}
	if workers < 1 {
		return nil, fmt.Errorf("test worker request must be positive: %d", workers)
	}
	if workers > pool.capacity {
		return nil, fmt.Errorf("test worker request %d exceeds pool capacity %d", workers, pool.capacity)
	}
	return pool, nil
}

func testWorkerRelease(pool *testWorkerPool, workers int) func() {
	var once sync.Once
	return func() {
		once.Do(func() {
			pool.mu.Lock()
			pool.available += workers
			close(pool.changed)
			pool.changed = make(chan struct{})
			pool.mu.Unlock()
		})
	}
}
