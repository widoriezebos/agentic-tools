// Package counselor computes advisory drift signals from durable records and
// renders their evidence limits beside the observations. It owns no mutation
// path and makes no consequence decision.
package counselor

import (
	"sort"
	"time"
)

// Options identifies the checkout whose current durable records form a brief.
type Options struct {
	Root string
	Now  func() time.Time
}

func (o Options) now() time.Time {
	if o.Now != nil {
		return o.Now().UTC().Truncate(time.Second)
	}
	return time.Now().UTC().Truncate(time.Second)
}

// Window is a half-open period: Start is included and End is excluded.
type Window struct {
	Start time.Time
	End   time.Time
}

func (w Window) contains(stamp time.Time) bool {
	return !stamp.Before(w.Start) && stamp.Before(w.End)
}

// Limitation carries an evidence boundary and the durable enrichment that
// would remove it. It is data so every renderer must preserve the honesty.
type Limitation struct {
	Name       string
	Detail     string
	Enrichment string
}

// OutcomeCounts is the closed terminal vocabulary carried by run records.
type OutcomeCounts struct {
	Green        int
	Red          int
	EndedUnknown int
	LaunchFailed int
}

func (c *OutcomeCounts) add(outcome RunOutcome) {
	switch outcome {
	case OutcomeGreen:
		c.Green++
	case OutcomeRed:
		c.Red++
	case OutcomeEndedUnknown:
		c.EndedUnknown++
	case OutcomeLaunchFailed:
		c.LaunchFailed++
	}
}

// RunTotals separates governed proof spend from ordinary retained run spend.
type RunTotals struct {
	Runs     int
	Duration time.Duration
	Outcomes OutcomeCounts
}

// LandingTotals is the Git outcome carried by one time window.
type LandingTotals struct {
	Commits     int
	Files       int
	Insertions  int
	BinaryFiles int
}

// SpendVsOutcomeWindow compares recorded run spend and outcomes with what
// landed in the same time window. It makes no causal or quality judgment.
type SpendVsOutcomeWindow struct {
	Window   Window
	Governed RunTotals
	Tracked  RunTotals
	Landings LandingTotals
}

// SpendVsOutcomeTrend is the typed signal plus the limits on its evidence.
type SpendVsOutcomeTrend struct {
	Windows     []SpendVsOutcomeWindow
	Limitations []Limitation
}

// GoalVerbCounts is the exact requested mapping of durable history verbs.
type GoalVerbCounts struct {
	Open   int
	Edit   int
	Claim  int
	Budget int
	Done   int
}

// ActivityRatio carries numerator and denominator even when division is not
// defined. Value is nil when no product outcome was recorded.
type ActivityRatio struct {
	ProcessActivities int
	ProductOutcomes   int
	Value             *float64
}

// ProcessVsProductWindow is one auditable activity comparison.
type ProcessVsProductWindow struct {
	Window               Window
	GoalEvents           GoalVerbCounts
	RecordOnlyLandings   int
	GoalCarrierLandings  int
	ProductLandings      int
	UnclassifiedLandings int
	Ratio                ActivityRatio
}

// ProcessVsProductTrend is the typed activity signal plus its evidence limits.
type ProcessVsProductTrend struct {
	Windows            []ProcessVsProductWindow
	ClassificationRule string
	Limitations        []Limitation
}

// RegisterEntryKind names the durable accepted-risk register entry family.
type RegisterEntryKind string

const (
	RegisterAcceptedRisk      RegisterEntryKind = "accepted-risk"
	RegisterNearMiss          RegisterEntryKind = "near-miss"
	RegisterMisclassification RegisterEntryKind = "misclassification"
)

// RegisterCitation points one specimen fact at a durable record.
type RegisterCitation struct {
	Kind   string
	Target string
	Detail string
}

// RegisterSpecimenFact is one observed fact retained with its evidence.
type RegisterSpecimenFact struct {
	Fact      string
	Citations []RegisterCitation
}

// RegisterReviewLink names the durable review surface that should consume the
// entry. The counselor only reports the link; the sitting owns consumption.
type RegisterReviewLink struct {
	Kind   string
	Target string
	Detail string
}

// RegisterEntry is one accepted risk or near miss from the append-only
// register. Every entry is classed so repeated specimens can aggregate.
type RegisterEntry struct {
	ID               string
	RecordedAt       time.Time
	Kind             RegisterEntryKind
	Class            string
	Title            string
	AcceptanceStatus string
	AcceptanceReason string
	SpecimenFacts    []RegisterSpecimenFact
	ReviewLinks      []RegisterReviewLink
}

// RegisterClassSummary is the auditable aggregate for one exact class string.
type RegisterClassSummary struct {
	Class         string
	AcceptedRisks int
	NearMisses    int
	EntryIDs      []string
}

// AcceptedRiskRegister is the counselor's read-only view of the durable
// register and its named evidence limits.
type AcceptedRiskRegister struct {
	Source       string
	CountingRule string
	Entries      []RegisterEntry
	Classes      []RegisterClassSummary
	Limitations  []Limitation
}

// Brief is the counselor's complete read-only observation at one instant.
type Brief struct {
	GeneratedAt          time.Time
	WindowRule           string
	SpendVsOutcome       SpendVsOutcomeTrend
	ProcessVsProduct     ProcessVsProductTrend
	AcceptedRiskRegister AcceptedRiskRegister
}

// RunKind identifies which durable owner supplied one completed run.
type RunKind string

const (
	RunGoverned RunKind = "governed"
	RunTracked  RunKind = "tracked"
)

// RunOutcome mirrors the terminal run-record vocabulary.
type RunOutcome string

const (
	OutcomeGreen        RunOutcome = "green"
	OutcomeRed          RunOutcome = "red"
	OutcomeEndedUnknown RunOutcome = "ended-unknown"
	OutcomeLaunchFailed RunOutcome = "launch-failed"
)

// RunObservation is one deduplicated completed run carried by durable state.
type RunObservation struct {
	ID          string
	Kind        RunKind
	CompletedAt time.Time
	Duration    time.Duration
	Outcome     RunOutcome
}

// LandingObservation is one current-branch commit and its numstat facts.
type LandingObservation struct {
	Commit          string
	CompletedAt     time.Time
	Files           int
	Insertions      int
	BinaryFiles     int
	Paths           []string
	GoalOperationID string
}

// GoalVerbClass is the requested closed projection over open history verbs.
type GoalVerbClass string

const (
	GoalOpen   GoalVerbClass = "open"
	GoalEdit   GoalVerbClass = "edit"
	GoalClaim  GoalVerbClass = "claim"
	GoalBudget GoalVerbClass = "budget"
	GoalDone   GoalVerbClass = "done"
)

// GoalEventObservation is one unique goal-ledger operation.
type GoalEventObservation struct {
	OperationID string
	At          time.Time
	Class       GoalVerbClass
}

// RecordSet is the durable evidence projection consumed by the signal core.
// Tests use this boundary to prove the computations without depending on the
// machine's Git history or state directories.
type RecordSet struct {
	Runs                []RunObservation
	Landings            []LandingObservation
	GoalEvents          []GoalEventObservation
	RegisterEntries     []RegisterEntry
	SpendLimitations    []Limitation
	ActivityLimitations []Limitation
	RegisterLimitations []Limitation
}

func completedWeekStart(stamp time.Time) time.Time {
	stamp = stamp.UTC()
	daysSinceMonday := (int(stamp.Weekday()) + 6) % 7
	return time.Date(stamp.Year(), stamp.Month(), stamp.Day()-daysSinceMonday, 0, 0, 0, 0, time.UTC)
}

func observedWindows(records RecordSet, now time.Time) []Window {
	latestEnd := completedWeekStart(now)
	starts := map[time.Time]bool{latestEnd.AddDate(0, 0, -7): true}
	add := func(stamp time.Time) {
		start := completedWeekStart(stamp)
		if start.Before(latestEnd) {
			starts[start] = true
		}
	}
	for _, record := range records.Runs {
		add(record.CompletedAt)
	}
	for _, record := range records.Landings {
		add(record.CompletedAt)
	}
	for _, record := range records.GoalEvents {
		add(record.At)
	}
	ordered := make([]time.Time, 0, len(starts))
	for start := range starts {
		ordered = append(ordered, start)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Before(ordered[j]) })
	windows := make([]Window, 0, len(ordered))
	for _, start := range ordered {
		windows = append(windows, Window{Start: start, End: start.AddDate(0, 0, 7)})
	}
	return windows
}
