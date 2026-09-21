package snapshot

import (
	"context"
	"sync"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

func TestGateReleasesOnlyWhenOpened(t *testing.T) {
	t.Parallel()

	t.Run("a context that ended first releases nothing", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		testutil.Expect(t, "wait", NewGate().Wait(ctx), false)
	})

	t.Run("an open gate releases every waiter", func(t *testing.T) {
		t.Parallel()
		gate := NewGate()
		gate.Open()
		released := make([]bool, 16)
		var waiters sync.WaitGroup
		for index := range released {
			waiters.Add(1)
			go func(index int) {
				defer waiters.Done()
				released[index] = gate.Wait(context.Background())
			}(index)
		}
		waiters.Wait()
		for _, opened := range released {
			if !opened {
				t.Fatalf("an open gate held a waiter")
			}
		}
	})

	t.Run("an open gate does not release a caller that is already stopping", func(t *testing.T) {
		t.Parallel()
		gate := NewGate()
		gate.Open()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		testutil.Expect(t, "wait", gate.Wait(ctx), false)
	})

	t.Run("opening twice at once is not a second close", func(t *testing.T) {
		t.Parallel()
		gate := NewGate()
		var openers sync.WaitGroup
		for i := 0; i < 8; i++ {
			openers.Add(1)
			go func() {
				defer openers.Done()
				gate.Open()
			}()
		}
		openers.Wait()
		testutil.Expect(t, "wait", gate.Wait(context.Background()), true)
	})

	t.Run("a waiter is released the moment the gate opens", func(t *testing.T) {
		t.Parallel()
		gate := NewGate()
		released := make(chan bool, 1)
		go func() { released <- gate.Wait(context.Background()) }()
		gate.Open()
		testutil.Expect(t, "wait", <-released, true)
	})
}

// TestAGateThatNeverOpensPerformsNoFetch is the composed shape the interface
// wiring delegates to: the loop's goroutine is started before the server
// takes the checkout, and a server that is refused the checkout, or whose
// listener fails, cancels the loop's context on the way out. Neither case may
// fetch or advance anything, and neither may leave the goroutine behind.
func TestAGateThatNeverOpensPerformsNoFetch(t *testing.T) {
	t.Parallel()
	b, tip := readableBed(t)
	holder := New(b.root, newClock(fixtureNow).now)
	fetch := &scriptedFetch{answer: currentAt("never reached")}
	gate := NewGate()
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		defer close(done)
		if !gate.Wait(ctx) {
			return
		}
		holder.Run(ctx, fetch.fetch, newFakeTimers())
	}()

	cancel()
	<-done

	testutil.Expect(t, "fetches", fetch.count(), 0)
	testutil.Expect(t, "the accepted ref is untouched", b.metasystemRefs(), goal.AcceptedRef+" "+tip)
}
