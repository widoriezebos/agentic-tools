package processmeasure

import (
	"fmt"
	"slices"
	"time"
)

type Tokens struct{ Input, CacheRead, CacheCreation, Output int64 }
type Interval struct{ Start, End time.Time }
type Step struct {
	ID, Kind, Pending, Start, End, Collected string
	Act                                      string
	Parent, Coverage, Outcome                string
	Argv                                     []string
	FullArgv                                 []string
	Terminal                                 bool
	Usage                                    *Tokens
}
type Input struct {
	Unit, PublishedAt string
	Current, GoalOpen bool
	Runs              []Input

	Now             time.Time
	Run             string
	Steps           []Step
	Revisions       []int
	FullArgv        []string
	CheckMinutes    *float64
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

	ElapsedHours      *float64
	ElapsedLowerBound bool
	Children          []Step
}

func Read(in Input) Measures {
	suiteUnknown := len(in.FullArgv) == 0 && !slices.ContainsFunc(in.Steps, func(step Step) bool { return len(step.FullArgv) > 0 })
	if len(in.Runs) > 0 {
		suiteUnknown = false
		latest := map[string]Input{}
		starts := map[string]time.Time{}
		revisions := map[string]bool{}
		for _, run := range in.Runs {
			var start time.Time
			for _, step := range run.Steps {
				if step.Coverage == "" {
					step.FullArgv = run.FullArgv
				}
				in.Steps = append(in.Steps, step)
				at, _ := time.Parse(time.RFC3339Nano, step.Start)
				if !at.IsZero() && (start.IsZero() || at.Before(start)) {
					start = at
				}
			}
			previous, found := latest[run.Unit]
			if !found || run.Current || (!previous.Current && start.After(starts[run.Unit])) {
				latest[run.Unit], starts[run.Unit] = run, start
			}
			for _, revision := range run.Revisions {
				revisions[fmt.Sprintf("%s/%d", run.Run, revision)] = true
			}
			in.Questions = append(in.Questions, run.Questions...)
			in.Unknown = append(in.Unknown, run.Unknown...)
			suiteUnknown = suiteUnknown || len(run.FullArgv) == 0
		}
		for range revisions {
			in.Revisions = append(in.Revisions, len(in.Revisions))
		}
		estimate, known, published := 0.0, true, true
		var finish time.Time
		for _, run := range latest {
			if run.EstimateMinutes == nil {
				known = false
			} else {
				estimate += *run.EstimateMinutes
			}
			at, err := time.Parse(time.RFC3339Nano, run.PublishedAt)
			published = published && err == nil && !at.Before(starts[run.Unit]) && !at.After(in.Now)
			if at.After(finish) {
				finish = at
			}
		}
		if known {
			in.EstimateMinutes = &estimate
		}
		if published {
			in.PublishedAt = finish.Format(time.RFC3339Nano)
		}
	}

	m := Measures{ObservedAt: in.Now, Run: in.Run, EstimateMinutes: in.EstimateMinutes, Hours: map[string]*float64{}, Unknown: slices.Clone(in.Unknown)}
	seen := map[string]bool{}
	bad := map[string]bool{"person": len(in.Unknown) > 0}
	values := map[string]float64{"build": 0, "attest": 0, "read": 0, "correction": 0, "pending": 0, "collection": 0, "person": 0, "suite": 0}
	unknownInclusion := false
	bad["suite"] = suiteUnknown
	var firstBuild time.Time
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
		if (step.Kind == "build" || step.Kind == "correction") && !start.IsZero() && (firstBuild.IsZero() || start.Before(firstBuild)) {
			firstBuild = start
		}
		hours, valid := duration(Interval{start, end}, in.Now)
		if (step.Kind == "build" || step.Kind == "correction") && !slices.ContainsFunc(in.Steps, func(child Step) bool { return child.Parent == step.ID }) {
			m.Unknown = append(m.Unknown, "nested checks unavailable: "+step.ID)
		}
		if step.Kind == "build" || step.Kind == "correction" || (step.Kind == "attest" && step.Coverage == "") {
			hours -= nestedHours(step.ID, Interval{start, end}, in.Steps, in.Now)
		}
		values[step.Kind] += hours
		bad[step.Kind] = bad[step.Kind] || !valid
		if len(step.FullArgv) > 0 && slices.Equal(step.Argv, step.FullArgv) {
			values["suite"] += hours * 60
			bad["suite"] = bad["suite"] || !valid
		}
		if step.Coverage != "" {
			bad["suite"] = bad["suite"] || len(step.FullArgv) == 0
			m.Children = append(m.Children, step)
			if step.Parent == "" {
				m.Unknown = append(m.Unknown, "check parent unavailable: "+step.ID)
			}
			if step.Usage == nil {
				m.Unknown = append(m.Unknown, "check usage unavailable: "+step.ID)
			}
			if step.Usage != nil && step.Coverage == "unknown" {
				unknownInclusion = true
				m.Unknown = append(m.Unknown, "check token inclusion unavailable: "+step.ID)
			}
			if step.Coverage != "separate" {
				continue
			}
			step.Pending, step.Collected = step.Start, step.End
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
		if step.Kind == "attest" && step.Coverage == "" {
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
	if !unknownInclusion && m.UsageExpected > 0 && m.UsageKnown == m.UsageExpected {
		m.Tokens = &m.ReportedTokens
	}
	finish, err := time.Parse(time.RFC3339Nano, in.PublishedAt)
	if !in.GoalOpen && err == nil && !firstBuild.IsZero() && !finish.Before(firstBuild) && !finish.After(in.Now) {
		m.ElapsedFinish = &finish
	}
	end := in.Now
	if m.ElapsedFinish != nil {
		end = *m.ElapsedFinish
	} else {
		m.Unknown = append(m.Unknown, "elapsed finish unavailable")
		m.ElapsedLowerBound = true
	}
	if hours, valid := duration(Interval{firstBuild, end}, in.Now); valid {
		m.ElapsedHours = &hours
	}
	m.Unknown = append(m.Unknown, "fix units unavailable", "load waits unavailable")
	slices.Sort(m.Unknown)
	return m
}

// Nested worker intervals overlap; only their union belongs outside parent work.
func nestedHours(parent string, window Interval, steps []Step, now time.Time) float64 {
	var intervals []Interval
	seen := map[string]bool{}
	for _, step := range steps {
		if step.Parent != parent || seen[step.ID] {
			continue
		}
		seen[step.ID] = true
		start, _ := time.Parse(time.RFC3339Nano, step.Start)
		end, _ := time.Parse(time.RFC3339Nano, step.End)
		if step.End == "" && !step.Terminal {
			end = now
		}
		if _, valid := duration(Interval{start, end}, now); !valid {
			continue
		}
		if start.Before(window.Start) {
			start = window.Start
		}
		if end.After(window.End) {
			end = window.End
		}
		if end.After(start) {
			intervals = append(intervals, Interval{start, end})
		}
	}
	slices.SortFunc(intervals, func(a, b Interval) int { return a.Start.Compare(b.Start) })
	var merged Interval
	hours := 0.0
	for _, interval := range intervals {
		if merged.End.Before(interval.Start) {
			value, _ := duration(merged, now)
			hours += value
			merged = interval
		} else if interval.End.After(merged.End) {
			merged.End = interval.End
		}
	}
	value, _ := duration(merged, now)
	return hours + value
}

func duration(interval Interval, now time.Time) (float64, bool) {
	if interval.Start.IsZero() || interval.End.IsZero() || interval.End.Before(interval.Start) || interval.End.After(now) {
		return 0, false
	}
	return interval.End.Sub(interval.Start).Hours(), true
}
