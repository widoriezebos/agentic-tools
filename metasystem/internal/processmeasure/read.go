package processmeasure

import (
	"slices"
	"time"
)

type Tokens struct{ Input, CacheRead, CacheCreation, Output int64 }
type Interval struct{ Start, End time.Time }
type Step struct {
	ID, Kind, Pending, Start, End, Collected string
	Argv                                     []string
	Terminal                                 bool
	Usage                                    *Tokens
}
type Input struct {
	Now             time.Time
	Run             string
	Steps           []Step
	Revisions       []int
	FullArgv        []string
	EstimateMinutes *float64
	Questions       []Interval
	Unknown         []string
}
type Measures struct {
	ObservedAt       time.Time
	Run              string
	Sources          []string
	Hours            map[string]*float64
	EstimateMinutes  *float64
	SuiteMinutes     *float64
	ElapsedFinish    *time.Time
	PersonLowerBound bool
	WorkLowerBound   bool
	Corrections      int
	Tokens           *Tokens
	ReportedTokens   Tokens
	UsageKnown       int
	UsageExpected    int
	Unknown          []string
}

func Read(in Input) Measures {
	m := Measures{ObservedAt: in.Now, Run: in.Run, EstimateMinutes: in.EstimateMinutes, Hours: map[string]*float64{}, Unknown: slices.Clone(in.Unknown)}
	seen := map[string]bool{}
	bad := map[string]bool{"person": len(in.Unknown) > 0}
	values := map[string]float64{"build": 0, "attest": 0, "read": 0, "correction": 0, "pending": 0, "collection": 0, "person": 0, "suite": 0}
	bad["suite"] = len(in.FullArgv) == 0
	revisions := slices.Clone(in.Revisions)
	slices.Sort(revisions)
	m.Corrections = len(slices.Compact(revisions))
	for _, step := range in.Steps {
		if seen[step.ID] || step.ID == "" {
			continue
		}
		seen[step.ID] = true
		m.Sources = append(m.Sources, step.ID)
		start, _ := time.Parse(time.RFC3339Nano, step.Start)
		end, _ := time.Parse(time.RFC3339Nano, step.End)
		if step.End == "" && !step.Terminal {
			end, m.WorkLowerBound = in.Now, true
		}
		hours, valid := duration(Interval{start, end}, in.Now)
		values[step.Kind] += hours
		bad[step.Kind] = bad[step.Kind] || !valid
		if len(in.FullArgv) > 0 && slices.Equal(step.Argv, in.FullArgv) {
			values["suite"] += hours * 60
			bad["suite"] = bad["suite"] || !valid
		}
		pending, _ := time.Parse(time.RFC3339Nano, step.Pending)
		collected, _ := time.Parse(time.RFC3339Nano, step.Collected)
		if !step.Terminal && start.IsZero() {
			start = in.Now
		}
		for kind, interval := range map[string]Interval{"pending": {pending, start}, "collection": {end, collected}} {
			if kind == "collection" && !step.Terminal {
				continue
			}
			if kind == "collection" && step.Collected == "" {
				interval.End = in.Now
			}
			hours, valid := duration(interval, in.Now)
			values[kind] += hours
			bad[kind] = bad[kind] || !valid
		}
		if step.Kind == "attest" {
			continue
		}
		m.UsageExpected++
		if step.Usage != nil {
			m.UsageKnown++
			m.ReportedTokens.Input += step.Usage.Input
			m.ReportedTokens.CacheRead += step.Usage.CacheRead
			m.ReportedTokens.CacheCreation += step.Usage.CacheCreation
			m.ReportedTokens.Output += step.Usage.Output
		}
	}
	questions := slices.Clone(in.Questions)
	for index := range questions {
		if questions[index].End.IsZero() {
			questions[index].End, m.PersonLowerBound = in.Now, true
		}
		if _, valid := duration(questions[index], in.Now); !valid {
			bad["person"] = true
		}
	}
	slices.SortFunc(questions, func(a, b Interval) int { return a.Start.Compare(b.Start) })
	var merged Interval
	for _, interval := range questions {
		if merged.End.Before(interval.Start) {
			hours, _ := duration(merged, in.Now)
			values["person"] += hours
			merged = interval
		} else if interval.End.After(merged.End) {
			merged.End = interval.End
		}
	}
	hours, _ := duration(merged, in.Now)
	values["person"] += hours
	for kind, value := range values {
		if bad[kind] {
			m.Unknown = append(m.Unknown, kind+" time unavailable")
		} else {
			m.Hours[kind] = &value
		}
	}
	m.SuiteMinutes = m.Hours["suite"]
	delete(m.Hours, "suite")
	if m.UsageExpected > 0 && m.UsageKnown == m.UsageExpected {
		m.Tokens = &m.ReportedTokens
	}
	m.Unknown = append(m.Unknown, "elapsed finish unavailable", "nested checks unavailable", "fix units unavailable", "load waits unavailable")
	slices.Sort(m.Unknown)
	return m
}

func duration(interval Interval, now time.Time) (float64, bool) {
	if interval.Start.IsZero() || interval.End.IsZero() || interval.End.Before(interval.Start) || interval.End.After(now) {
		return 0, false
	}
	return interval.End.Sub(interval.Start).Hours(), true
}
