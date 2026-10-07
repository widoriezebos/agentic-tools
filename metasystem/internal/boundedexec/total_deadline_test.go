package boundedexec

import (
	"errors"
	"testing"
	"time"
)

func TestTotalDeadlineIncludesProcessReaping(t *testing.T) {
	t.Parallel()
	fixture := newPipeHeldCommand(t, "IFS= read -r _ <&3")
	bound := FixedBound(3*time.Second, "fresh ledger allowance")
	bound.TotalDeadline = time.Now().Add(2 * time.Second)
	var requested []time.Duration
	fired := make(chan time.Time, 1)
	fired <- time.Time{}
	err := fixture.runWithDeadline(bound, "ledger transport", func(wait time.Duration) <-chan time.Time {
		requested = append(requested, wait)
		if len(requested) == 1 {
			return fired
		}
		return make(chan time.Time)
	})
	if !errors.Is(err, ErrTimedOut) || len(requested) != 2 || requested[0] > 2*time.Second || requested[1] > 2*time.Second {
		t.Fatalf("process execution or reaping exceeds the total allowance: waits=%v error=%v", requested, err)
	}
}
