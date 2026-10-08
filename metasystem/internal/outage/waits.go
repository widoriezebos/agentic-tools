package outage

import (
	"fmt"
	"sort"
	"time"
)

// Span is an evidenced exclusion from an elapsed clock.
type Span struct{ Start, End time.Time }

// Waiting intersects a provider's history with one dependent clock window.
// An open interval stops at its evidence bound even before expiry is persisted.
func (s Providers) Waiting(runtime string, start, end time.Time) ([]Span, error) {
	if runtime == "" && len(s.Current) > 0 {
		return nil, fmt.Errorf("provider dependency is unknown; restore the retained launch runtime")
	}
	c := s.Current[Provider(runtime)]
	intervals := append([]Interval(nil), c.Intervals...)
	if c.Mark.ConsecutiveFailures > 0 {
		last, err := time.Parse(time.RFC3339Nano, c.Mark.LastAt)
		if err != nil {
			return nil, fmt.Errorf("provider %s observation is unreadable", Provider(runtime))
		}
		bound := last.Add(Horizon)
		if c.Mark.ResetAt != "" {
			reset, err := time.Parse(time.RFC3339Nano, c.Mark.ResetAt)
			if err != nil {
				return nil, fmt.Errorf("provider %s reset is unreadable", Provider(runtime))
			}
			bound = reset.Add(2 * time.Minute)
		}
		intervals = append(intervals, Interval{c.Mark.Since, bound.Format(time.RFC3339Nano)})
	}
	var spans []Span
	for _, interval := range intervals {
		from, fromErr := time.Parse(time.RFC3339Nano, interval.Since)
		to, toErr := time.Parse(time.RFC3339Nano, interval.Until)
		if fromErr != nil || toErr != nil || to.Before(from) {
			return nil, fmt.Errorf("provider %s wait history is unreadable", Provider(runtime))
		}
		if from.Before(start) {
			from = start
		}
		if to.After(end) {
			to = end
		}
		if to.After(from) {
			spans = append(spans, Span{from, to})
		}
	}
	return spans, nil
}

// Paused counts a union once, including waits from other exclusion owners.
func Paused(spans []Span) time.Duration {
	sort.Slice(spans, func(i, j int) bool { return spans[i].Start.Before(spans[j].Start) })
	var total time.Duration
	var until time.Time
	for _, span := range spans {
		from := span.Start
		if from.Before(until) {
			from = until
		}
		if span.End.After(from) {
			total += span.End.Sub(from)
			until = span.End
		}
	}
	return total
}
