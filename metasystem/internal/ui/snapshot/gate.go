package snapshot

import (
	"context"
	"sync"
)

// Gate holds the freshness loop until this process owns the checkout.
//
// Exclusivity over a state root is acquired while the server starts, not
// before: a second server for the same checkout is refused, and a loop that
// had already run its first tick would by then have fetched and advanced a
// ref on behalf of a server that never served. The loop therefore waits here
// and is released only from the point where ownership is established.
//
// Open is idempotent and safe from any goroutine. Wait reports whether the
// gate opened; a context that ends first releases nothing, so a gate that
// never opens performs no fetch at all.
type Gate struct {
	opened chan struct{}
	once   sync.Once
}

func NewGate() *Gate {
	return &Gate{opened: make(chan struct{})}
}

// Open releases every waiter, now and later.
func (g *Gate) Open() {
	g.once.Do(func() { close(g.opened) })
}

// Wait blocks until the gate opens or the context ends. Cancellation is
// checked before anything else, so a caller that is already stopping is never
// released by a gate that happens to be open.
func (g *Gate) Wait(ctx context.Context) bool {
	if ctx.Err() != nil {
		return false
	}
	select {
	case <-g.opened:
		return true
	case <-ctx.Done():
		return false
	}
}
