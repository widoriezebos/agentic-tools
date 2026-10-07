package boundedexec

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestTotalDeadlineIncludesProcessReaping(t *testing.T) {
	t.Parallel()
	fixture := newPipeHeldCommand(t, "IFS= read -r _ <&3")
	bound := FixedBound(3*time.Second, "fresh ledger allowance")
	now := time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)
	bound.TotalDeadline = now.Add(2 * time.Second)
	var requested []time.Duration
	fired := make(chan time.Time, 1)
	fired <- time.Time{}
	err := runContext(context.Background(), fixture.command, bound, "ledger transport", func(wait time.Duration) <-chan time.Time {
		requested = append(requested, wait)
		if len(requested) == 1 {
			now = now.Add(1500 * time.Millisecond)
			return fired
		}
		return make(chan time.Time)
	}, func() time.Time { return now })
	if !errors.Is(err, ErrTimedOut) || len(requested) != 2 || requested[0] != 2*time.Second || requested[1] != 500*time.Millisecond {
		t.Fatalf("process execution or reaping exceeds the total allowance: waits=%v error=%v", requested, err)
	}
}
