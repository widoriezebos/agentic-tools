package batch

import (
	"fmt"
	"reflect"
	"slices"
	"time"
)

// CostForecast is a historical, read-only estimate. It never grants proof
// admission. Actual native execution still reserves through the test owner.
type CostForecast struct {
	SchemaVersion              int                   `json:"schemaVersion"`
	ObservedAt                 string                `json:"observedAt"`
	Currency                   string                `json:"currency"`
	Binding                    CostForecastBinding   `json:"binding"`
	Requests                   []CostForecastRequest `json:"requests"`
	Groups                     []CostForecastGroup   `json:"groups"`
	Budgets                    []CostForecastBudget  `json:"budgets"`
	KnownMissing               int                   `json:"knownMissing"`
	UnknownMissing             int                   `json:"unknownMissing"`
	Reusable                   int                   `json:"reusable"`
	LiveWaits                  int                   `json:"liveWaits"`
	ObservedWorkEstimateMS     int64                 `json:"observedWorkEstimateMs"`
	UnknownDeclaredWorkMS      int64                 `json:"unknownDeclaredWorkMs"`
	SerialWorkEstimateMS       int64                 `json:"serialWorkEstimateMs"`
	DeclaredReservationMin     uint64                `json:"declaredReservationMin"`
	HeavyCapacity              int                   `json:"heavyCapacity,omitempty"`
	ExclusiveResourceConflicts []string              `json:"exclusiveResourceConflicts,omitempty"`
	MetadataReads              string                `json:"metadataReads"`
}

type CostForecastBinding struct {
	BaseTree       string                `json:"baseTree"`
	PrefixTrees    []string              `json:"prefixTrees"`
	SelectedGroups []string              `json:"selectedGroups"`
	Members        []CostForecastMember  `json:"members"`
	PrefixEpisodes []CostForecastEpisode `json:"prefixEpisodes"`
}

// Each ordered member has an explicit episode slot, including when no episode
// exists. A replaced, cleared, or newly created episode changes the forecast
// context before a claim can cross the handover boundary.
type CostForecastEpisode struct {
	GoalID     string `json:"goalId"`
	Present    bool   `json:"present"`
	DecisionID string `json:"decisionId,omitempty"`
	Token      string `json:"token,omitempty"`
	ExpiresAt  string `json:"expiresAt,omitempty"`
}

type CostForecastMember struct {
	GoalID         string   `json:"goalId"`
	Chain          string   `json:"chain"`
	State          string   `json:"state"`
	Claim          Claim    `json:"claim"`
	SelectedGroups []string `json:"selectedGroups"`
}

type CostForecastRequest struct {
	ID                 string   `json:"id"`
	Kind               string   `json:"kind"`
	Tree               string   `json:"tree"`
	ChargeGoal         string   `json:"chargeGoal"`
	SelectedGroups     []string `json:"selectedGroups"`
	NeedsAttempt       bool     `json:"needsAttempt"`
	AttemptDemand      uint64   `json:"attemptDemand"`
	ReservationMinutes uint64   `json:"reservationMinutes"`
	Reason             string   `json:"reason,omitempty"`
}

type CostForecastGroup struct {
	RequestID           string   `json:"requestId"`
	Tree                string   `json:"tree"`
	ChargeGoal          string   `json:"chargeGoal"`
	GroupID             string   `json:"groupId"`
	ExecutionIdentity   string   `json:"executionIdentity,omitempty"`
	IdentityKnown       bool     `json:"identityKnown"`
	Status              string   `json:"status"`
	Reason              string   `json:"reason,omitempty"`
	CoveredBy           string   `json:"coveredBy,omitempty"`
	FreshEpisode        string   `json:"freshEpisode,omitempty"`
	ObservedDurationMS  *int64   `json:"observedDurationMs,omitempty"`
	DeclaredAllowanceMS int64    `json:"declaredAllowanceMs,omitempty"`
	TargetMS            int64    `json:"targetMs,omitempty"`
	ResourceClass       string   `json:"resourceClass,omitempty"`
	ExclusiveResources  []string `json:"exclusiveResources,omitempty"`
}

type CostForecastBudget struct {
	GoalID              string `json:"goalId"`
	Status              string `json:"status"`
	AttemptsLeft        uint64 `json:"attemptsLeft,omitempty"`
	ReservedMinutesLeft uint64 `json:"reservedMinutesLeft,omitempty"`
	AttemptDemand       uint64 `json:"attemptDemand"`
	ReservationMinutes  uint64 `json:"reservationMinutes"`
	Fits                bool   `json:"fits"`
	Reason              string `json:"reason,omitempty"`
}

func CostBinding(record Record, units []Unit, prefixes []string) CostForecastBinding {
	binding := CostForecastBinding{BaseTree: record.BaseTree, PrefixTrees: slices.Clone(prefixes), SelectedGroups: slices.Clone(record.SelectedGroups)}
	for _, unit := range units {
		binding.Members = append(binding.Members, CostForecastMember{GoalID: unit.GoalID, Chain: unit.Chain,
			State: unit.State, Claim: unit.Claim, SelectedGroups: slices.Clone(unit.SelectedGroups)})
		binding.PrefixEpisodes = append(binding.PrefixEpisodes, costForecastEpisode(record, unit.GoalID))
	}
	return binding
}

func costForecastEpisode(record Record, goalID string) CostForecastEpisode {
	episode, present := record.PrefixEpisodes[goalID]
	return CostForecastEpisode{GoalID: goalID, Present: present, DecisionID: episode.DecisionID,
		Token: episode.Token, ExpiresAt: episode.ExpiresAt}
}

func (forecast CostForecast) Matches(record Record) bool {
	units := joinedUnits(record.Units)
	return reflect.DeepEqual(forecast.Binding, CostBinding(record, units, record.PrefixTrees))
}

func (forecast CostForecast) ObservedTime() (time.Time, error) {
	return time.Parse(time.RFC3339Nano, forecast.ObservedAt)
}

// CloseAdmissionForCost closes only a still-matching, nonempty open batch.
// An over-budget first member is refused before an open record or claim
// handover is created.
func CloseAdmissionForCost(store Store, batchID, actor string, at time.Time, forecast CostForecast) error {
	return store.Update(batchID, func(record *Record) error {
		if err := joinRefusal(*record); err != nil {
			return err
		}
		members := joinedUnits(record.Units)
		if len(forecast.Binding.Members) != len(members)+1 || len(forecast.Binding.PrefixTrees) != len(members)+1 ||
			len(forecast.Binding.PrefixEpisodes) != len(forecast.Binding.Members) ||
			!slices.Equal(record.PrefixTrees, forecast.Binding.PrefixTrees[:len(members)]) ||
			!slices.Equal(record.SelectedGroups, forecast.Binding.SelectedGroups) ||
			!slices.EqualFunc(CostBinding(*record, members, record.PrefixTrees).Members, forecast.Binding.Members[:len(members)], func(left, right CostForecastMember) bool { return reflect.DeepEqual(left, right) }) ||
			record.BaseTree != forecast.Binding.BaseTree {
			return fmt.Errorf("BATCH_COST_INPUT_MOVED: batch changed before cost closure")
		}
		for _, bound := range forecast.Binding.PrefixEpisodes {
			if costForecastEpisode(*record, bound.GoalID) != bound {
				return fmt.Errorf("BATCH_COST_INPUT_MOVED: prefix episode changed before cost closure")
			}
		}
		if len(members) == 0 {
			return nil
		}
		record.ClosedReason = "budget-cost"
		record.History = append(record.History, HistoryEntry{At: at.UTC().Format(time.RFC3339Nano), Verb: "close",
			From: record.State, To: record.State, Actor: actor, Detail: "budget-cost"})
		return nil
	})
}
