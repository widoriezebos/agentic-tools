package steward

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

type manualRearmClock struct {
	mu      sync.Mutex
	now     time.Time
	timers  []chan time.Time
	created chan chan time.Time
}

func newManualRearmClock() *manualRearmClock {
	return &manualRearmClock{now: time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC), created: make(chan chan time.Time, 256)}
}

func (clock *manualRearmClock) Now() time.Time {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	return clock.now
}

func (clock *manualRearmClock) After(time.Duration) <-chan time.Time {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	timer := make(chan time.Time, 1)
	clock.timers = append(clock.timers, timer)
	select {
	case clock.created <- timer:
	default:
	}
	return timer
}

func (clock *manualRearmClock) advance(duration time.Duration) {
	clock.mu.Lock()
	clock.now = clock.now.Add(duration)
	clock.mu.Unlock()
}

func waitForRearmTimer(clock *manualRearmClock, count int) chan time.Time {
	return <-clock.created
}

func TestWitnessResolverStallsOnlyWhenAStepIsSilent(t *testing.T) {
	clock := newManualRearmClock()
	permit := make(chan struct{})
	progressed := make(chan struct{})
	finish := make(chan struct{})
	result := make(chan error, 1)
	go func() {
		result <- RunRearmStep(context.Background(), clock, 20*time.Second, "digest", func(_ context.Context, progress func()) error {
			for {
				select {
				case <-permit:
					progress()
					progressed <- struct{}{}
				case <-finish:
					return nil
				}
			}
		})
	}()
	current := waitForRearmTimer(clock, 1)
	for index := 0; index < 18; index++ {
		clock.advance(5 * time.Second)
		permit <- struct{}{}
		<-progressed
		current <- clock.Now()
		select {
		case current = <-clock.created:
		case err := <-result:
			t.Fatalf("progressing step stopped after %d seconds: %v", (index+1)*5, err)
		}
		select {
		case err := <-result:
			t.Fatalf("progressing step stopped after %d seconds: %v", (index+1)*5, err)
		default:
		}
	}
	close(finish)
	if err := <-result; err != nil {
		t.Fatalf("progressing 90-second step failed: %v", err)
	}

	silentClock := newManualRearmClock()
	silent := make(chan error, 1)
	go func() {
		silent <- RunRearmStep(context.Background(), silentClock, 20*time.Second, "digest", func(ctx context.Context, _ func()) error {
			<-ctx.Done()
			return ctx.Err()
		})
	}()
	silentTimer := waitForRearmTimer(silentClock, 1)
	silentClock.advance(21 * time.Second)
	silentTimer <- silentClock.Now()
	if err := <-silent; err == nil || !errors.Is(err, ErrJudgmentStalled) || !strings.Contains(err.Error(), "digest exceeded the configured 20-second bound") {
		t.Fatalf("silent step did not name its stall: %v", err)
	}

	sequenceClock := newManualRearmClock()
	for index := 0; index < 5; index++ {
		sequenceClock.advance(15 * time.Second)
		if err := RunRearmStep(context.Background(), sequenceClock, 20*time.Second, "step", func(context.Context, func()) error { return nil }); err != nil {
			t.Fatalf("step %d inherited a total deadline: %v", index+1, err)
		}
	}
}
