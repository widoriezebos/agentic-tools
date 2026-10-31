package steward

import (
	"context"
	"slices"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/textui"
)

// SituationPoll is how often a wait reads the shared page: a file read while
// the tick keeps the page fresh.
const SituationPoll = 30 * time.Second

// WaitSituation returns when page's wake digest differs from start's or one of
// start's deadlines passes, at most bound: it checks once, then after each of
// ceil(bound/poll) sleeps, whatever the clock does, so an event in the last
// interval is seen. A sleep ends early at the next deadline, and a deadline
// passed reads the page fresh, so it shows what the deadline changed. A read
// or sleep error returns at once: unknown is never "nothing changed".
func WaitSituation(ctx context.Context, page func(at time.Time, fresh time.Duration) (Composition, error), start Composition,
	now func() time.Time, sleep func(context.Context, time.Duration) error, bound, poll time.Duration) (Composition, string, error) {
	end, last := now().Add(bound), start
	for check := int64(0); check <= int64((bound+poll-1)/poll); check++ {
		t := now()
		if check > 0 {
			pause := min(poll, end.Sub(t))
			if next := slices.IndexFunc(start.Deadlines, t.Before); next >= 0 {
				pause = min(pause, start.Deadlines[next].Sub(t))
			}
			if err := sleep(ctx, max(pause, 0)); err != nil {
				return last, "", err
			}
			t = now()
		}
		if passed := slices.IndexFunc(start.Deadlines, func(d time.Time) bool { return !d.After(t) }); passed >= 0 {
			c, err := page(t, 0)
			return c, "deadline " + textui.Env{Now: t, Zone: t.Location()}.Time(start.Deadlines[passed]), err
		}
		c, err := page(t, SituationFresh)
		if err != nil {
			return last, "", err
		}
		if last = c; c.Digest != start.Digest {
			return c, "changed: " + firstChange(start.Wake, c.Wake), nil
		}
	}
	return last, "nothing changed in " + textui.Duration(bound), nil
}

// firstChange is the first wake line the new page adds, else the first it
// drops.
func firstChange(was, is []string) string {
	for _, pair := range [][2][]string{{is, was}, {was, is}} {
		for _, line := range pair[0] {
			if !slices.Contains(pair[1], line) {
				return line
			}
		}
	}
	return ""
}
