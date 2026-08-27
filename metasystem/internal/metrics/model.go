// Package metrics owns the actionable metric family: source discovery,
// period scoping, threshold judgments, computation, rendering, and report
// publication. Callers provide only the checkout and requested report scope.
package metrics

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Options is one report request after command-line parsing.
type Options struct {
	Root      string
	PeriodEnd string
	Since     string
	GoalID    string
	Now       func() time.Time
}

func (o Options) now() time.Time {
	if o.Now != nil {
		return o.Now().UTC().Truncate(time.Second)
	}
	return time.Now().UTC().Truncate(time.Second)
}

// Period is the one instant and event window every computation shares.
type Period struct {
	Start        time.Time
	End          time.Time
	Instant      time.Time
	CalendarWeek bool
	Week         string
}

func resolvePeriod(opts Options) (Period, error) {
	now := opts.now()
	parse := func(name, value string) (time.Time, error) {
		stamp, err := time.Parse(time.RFC3339, value)
		if err != nil {
			return time.Time{}, fmt.Errorf("metrics report: %s must be ISO 8601: %q", name, value)
		}
		return stamp.UTC().Truncate(time.Second), nil
	}

	if opts.GoalID != "" {
		instant := now
		if opts.PeriodEnd != "" {
			var err error
			instant, err = parse("--period-end", opts.PeriodEnd)
			if err != nil {
				return Period{}, err
			}
		}
		if opts.Since != "" {
			return Period{}, fmt.Errorf("metrics report: --since does not apply to a whole-lifecycle goal report")
		}
		return Period{Instant: instant}, nil
	}

	var start, end time.Time
	if opts.PeriodEnd == "" && opts.Since == "" {
		weekday := (int(now.Weekday()) + 6) % 7
		end = time.Date(now.Year(), now.Month(), now.Day()-weekday, 0, 0, 0, 0, time.UTC)
		start = end.AddDate(0, 0, -7)
	} else {
		end = now
		if opts.PeriodEnd != "" {
			var err error
			end, err = parse("--period-end", opts.PeriodEnd)
			if err != nil {
				return Period{}, err
			}
		}
		start = end.AddDate(0, 0, -7)
		if opts.Since != "" {
			var err error
			start, err = parse("--since", opts.Since)
			if err != nil {
				return Period{}, err
			}
		}
	}
	if !start.Before(end) {
		return Period{}, fmt.Errorf("metrics report: --since must be before --period-end")
	}
	calendar := start.Weekday() == time.Monday && start.Hour() == 0 && start.Minute() == 0 &&
		start.Second() == 0 && start.Nanosecond() == 0 && end.Equal(start.AddDate(0, 0, 7))
	week := ""
	if calendar {
		year, number := start.ISOWeek()
		week = fmt.Sprintf("%04d-W%02d", year, number)
	}
	return Period{Start: start, End: end, Instant: end, CalendarWeek: calendar, Week: week}, nil
}

func (p Period) contains(stamp time.Time) bool {
	return !stamp.Before(p.Start) && stamp.Before(p.End)
}

// Coverage uses one fixed vocabulary for every record source.
type Coverage struct {
	Source   string
	Found    int
	Usable   int
	Rejected int
	Missing  int
	Extra    string
	Details  []string
}

func (c Coverage) String() string {
	line := fmt.Sprintf("source=%s found=%d usable=%d rejected=%d missing=%d", c.Source, c.Found, c.Usable, c.Rejected, c.Missing)
	if c.Extra != "" {
		line += " " + c.Extra
	}
	return line
}

type detail struct {
	Text        string
	MachineOnly bool
}

type metricRow struct {
	Key        string
	Name       string
	Scope      string
	Value      string
	Coverage   []Coverage
	Thresholds []string
	Action     string
	Owner      string
	Details    []detail
}

type sourceIdentity struct {
	AcceptedTip string
	MainTip     string
	ReceiptBlob string
}

func (s sourceIdentity) String() string {
	return fmt.Sprintf("fleet-synced-as-of accepted_ref=%s main_tip=%s receipts_blob=%s", emptyAs(s.AcceptedTip, "missing"), emptyAs(s.MainTip, "missing"), emptyAs(s.ReceiptBlob, "missing"))
}

func emptyAs(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func sortedStrings(values map[string]bool) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func joinedDetails(coverages ...Coverage) []detail {
	var result []detail
	for _, coverage := range coverages {
		for _, item := range coverage.Details {
			result = append(result, detail{Text: "rejected " + coverage.Source + " " + item, MachineOnly: true})
		}
	}
	return result
}

func cleanLine(value string) string {
	return strings.NewReplacer("\r", " ", "\n", " ").Replace(value)
}

func reportStamp(stamp time.Time) string { return stamp.UTC().Format("20060102T150405Z") }

func detailedPeriodPath(root string, period Period) string {
	return filepath.Join(root, "artifacts", "agents", "metrics", "period-"+reportStamp(period.Start)+"-"+reportStamp(period.End)+".md")
}
