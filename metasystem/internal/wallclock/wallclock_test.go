package wallclock

import (
	"testing"
	"time"
)

func TestSystemReadsAndWaitsOnTheWallClock(t *testing.T) {
	t.Parallel()
	var c Clock = System()
	before := time.Now()
	start := c.Now()
	c.Sleep(time.Millisecond)
	if start.Before(before) || !c.Now().After(start) {
		t.Fatalf("the system clock does not follow the wall: start=%v before=%v", start, before)
	}
}
