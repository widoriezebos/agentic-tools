package main

import (
	"fmt"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"time"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func prepareProspectiveBatchCost(root string, record batch.Record, incoming batch.Unit, at time.Time,
	plan func(string, string, string) (testpolicy.Plan, error),
	assemble func(string, string, []batch.Unit) ([]string, error)) (batch.Unit, batch.CostForecast, error) {
	unitTrees, err := assemble(root, record.BaseTree, []batch.Unit{incoming})
	if err != nil || len(unitTrees) != 1 {
		return batch.Unit{}, batch.CostForecast{}, fmt.Errorf("BATCH_COST_INPUT_MOVED: incoming unit tree: %w", err)
	}
	selection, err := batch.PlanJoinedUnit(root, record.BaseTree, incoming, unitTrees[0], plan)
	if err != nil {
		return batch.Unit{}, batch.CostForecast{}, err
	}
	incoming.SelectedGroups = slices.Clone(selection.SelectedGroups)
	incoming.Admission = &batch.JoinAdmission{Tree: unitTrees[0], Status: "pending"}
	units := make([]batch.Unit, 0, len(record.Units)+1)
	for _, unit := range record.Units {
		if unit.State == batch.UnitJoined {
			units = append(units, unit)
		}
	}
	units = append(units, incoming)
	prefixes, err := assemble(root, record.BaseTree, units)
	if err != nil || len(prefixes) != len(units) {
		return batch.Unit{}, batch.CostForecast{}, fmt.Errorf("BATCH_COST_INPUT_MOVED: prospective series: %w", err)
	}
	candidate := record
	candidate.Units = units
	candidate.PrefixTrees, candidate.TipTree = prefixes, prefixes[len(prefixes)-1]
	forecast, err := forecastBatchCost(root, candidate, &incoming, at)
	return incoming, forecast, err
}

type batchCostSelectionRun func(string, costSelection, uint64) (costSelectionEvidence, error)

type batchCostBudgetProjection struct {
	Budget       dispatchcore.BudgetProjection
	LandingClaim bool
}

// forecastBatchCost prepares the exact final tip, then earlier cumulative
// prefixes. The pending join admission is a separate earlier-in-time proof
// request; it is conservatively counted even if the later tip shares inputs.
func forecastBatchCost(root string, candidate batch.Record, incoming *batch.Unit, at time.Time) (batch.CostForecast, error) {
	return forecastBatchCostWith(root, candidate, incoming, at, productionPrefixDecision, forecastTestingSelection, batchBudgetProjection)
}

func forecastBatchCostWith(root string, candidate batch.Record, incoming *batch.Unit, at time.Time,
	decision func(string, []batch.Unit, string) (batch.PrefixDecision, error),
	run batchCostSelectionRun, budget func(string, batch.Unit, *batch.Unit, time.Time) (batchCostBudgetProjection, error)) (batch.CostForecast, error) {
	units := make([]batch.Unit, 0, len(candidate.Units))
	for _, unit := range candidate.Units {
		if unit.State == batch.UnitJoined || unit.State == batch.UnitJoining {
			units = append(units, unit)
		}
	}
	if len(units) == 0 || len(candidate.PrefixTrees) != len(units) || candidate.TipTree != candidate.PrefixTrees[len(units)-1] {
		return batch.CostForecast{}, fmt.Errorf("BATCH_COST_INPUT_MOVED: incomplete cumulative prefix series")
	}
	capMinutes, err := proofCostCap(root)
	if err != nil {
		return batch.CostForecast{}, err
	}
	capacity, err := proofrun.ResolveAdmissionCap(filepath.Join(batch.ModuleRoot(root), "metasystem.conf"), runtime.NumCPU())
	if err != nil {
		return batch.CostForecast{}, err
	}
	forecast := batch.CostForecast{SchemaVersion: 1, ObservedAt: at.UTC().Format(time.RFC3339Nano),
		Currency: "snapshot-not-revalidated", Binding: batch.CostBinding(candidate, units, candidate.PrefixTrees),
		HeavyCapacity: capacity.Max, MetadataReads: "retained-policy, engine/tool identity, input, and attempt metadata; no native build or test"}
	var requests []costSelection
	order := []int{len(units) - 1}
	for index := 0; index < len(units)-1; index++ {
		order = append(order, index)
	}
	for _, index := range order {
		unit := units[index]
		kind, id := "prefix", "prefix:"+unit.GoalID
		if index == len(units)-1 {
			kind, id = "tip", "tip:"+unit.GoalID
		}
		planned, err := decision(root, units[:index+1], candidate.PrefixTrees[index])
		if err != nil {
			return batch.CostForecast{}, err
		}
		selection := costSelectionForPrefix(id, kind, candidate.PrefixTrees[index], unit.GoalID, planned.Groups)
		if episode, ok := candidate.PrefixEpisodes[unit.GoalID]; ok && planned.FreshRequired {
			decisionID, idErr := batch.PrefixDecisionID(candidate.BaseTree, selection.Tree, units[:index+1], candidate.Seal, planned)
			if idErr != nil {
				return batch.CostForecast{}, idErr
			}
			if episode.DecisionID == decisionID {
				selection.FreshEpisode, selection.FreshExpiresAt = episode.Token, episode.ExpiresAt
			}
		}
		requests = append(requests, selection)
	}
	if incoming != nil && incoming.Admission != nil && incoming.Admission.Tree != "" {
		requests = append(requests, costSelection{ID: "join-admission:" + incoming.GoalID, Kind: "join-admission",
			Tree: incoming.Admission.Tree, GoalID: incoming.GoalID, Admission: true,
			FreshEpisode: incoming.Admission.FreshEpisode, FreshExpiresAt: incoming.Admission.FreshExpiresAt})
	}
	seen := map[string]string{}
	resourceOwners := map[string]string{}
	conflicts := map[string]bool{}
	for _, selection := range requests {
		evidence, err := run(root, selection, capMinutes)
		if err != nil {
			return batch.CostForecast{}, fmt.Errorf("forecast %s: %w", selection.ID, err)
		}
		request := evidence.Request
		if request.ID != selection.ID || request.Tree != selection.Tree || request.ChargeGoal != selection.GoalID {
			return batch.CostForecast{}, fmt.Errorf("BATCH_COST_INPUT_MOVED: selection %s returned a different request", selection.ID)
		}
		for _, row := range evidence.Groups {
			if row.RequestID != selection.ID || row.Tree != selection.Tree || row.ChargeGoal != selection.GoalID {
				return batch.CostForecast{}, fmt.Errorf("BATCH_COST_INPUT_MOVED: %s returned an unbound group", selection.ID)
			}
			if row.Status == "reusable" {
				forecast.Reusable++
				forecast.Groups = append(forecast.Groups, row)
				continue
			}
			// The actual admission runs before tip proof. Never claim its future
			// success is already available merely because tip was examined first.
			key := ""
			if row.IdentityKnown && row.FreshEpisode != "fresh-uncreated" && row.Reason != "fresh-episode-not-created" && selection.Kind != "join-admission" {
				key = row.GroupID + "\x00" + row.ExecutionIdentity + "\x00" + row.FreshEpisode
			}
			if previous := seen[key]; key != "" && previous != "" {
				row.CoveredBy, row.Status = previous, "covered-planned"
				row.DeclaredAllowanceMS = 0
				forecast.Groups = append(forecast.Groups, row)
				continue
			}
			if key != "" {
				seen[key] = selection.ID
			}
			request.NeedsAttempt = true
			if row.IdentityKnown {
				forecast.KnownMissing++
			} else {
				forecast.UnknownMissing++
			}
			if row.Status == "live-wait" {
				forecast.LiveWaits++
			}
			if row.ObservedDurationMS != nil {
				forecast.ObservedWorkEstimateMS += *row.ObservedDurationMS
			} else {
				forecast.UnknownDeclaredWorkMS += row.DeclaredAllowanceMS
			}
			for _, resource := range row.ExclusiveResources {
				if owner := resourceOwners[resource]; owner != "" && owner != selection.ID+":"+row.GroupID {
					conflicts[resource] = true
				}
				resourceOwners[resource] = selection.ID + ":" + row.GroupID
			}
			forecast.Groups = append(forecast.Groups, row)
		}
		if request.NeedsAttempt {
			request.AttemptDemand, request.ReservationMinutes = 1, capMinutes
			forecast.DeclaredReservationMin += capMinutes
		} else {
			request.Reason = "all requirements reusable or covered by planned tip"
		}
		forecast.Requests = append(forecast.Requests, request)
	}
	forecast.SerialWorkEstimateMS = forecast.ObservedWorkEstimateMS + forecast.UnknownDeclaredWorkMS
	for name := range conflicts {
		forecast.ExclusiveResourceConflicts = append(forecast.ExclusiveResourceConflicts, name)
	}
	sort.Strings(forecast.ExclusiveResourceConflicts)
	for _, unit := range units {
		budgetView, err := budget(root, unit, incoming, at)
		if err != nil {
			return batch.CostForecast{}, err
		}
		projection := budgetView.Budget
		row := batch.CostForecastBudget{GoalID: unit.GoalID, Status: string(projection.Status), Fits: projection.Status == dispatchcore.BudgetKnown}
		for _, request := range forecast.Requests {
			if request.ChargeGoal == unit.GoalID {
				row.AttemptDemand += request.AttemptDemand
				row.ReservationMinutes += request.ReservationMinutes
			}
		}
		if projection.Status == dispatchcore.BudgetKnown {
			row.AttemptsLeft = saturatingLeft(projection.Limits.AttemptLimit, projection.Attempts)
			row.ReservedMinutesLeft = saturatingLeft(projection.Limits.ReservedJobMinutesLimit, projection.ReservedJobMinutes)
			switch {
			// The dispatch admission owner suspends elapsed admission for a
			// land-ready claim. Preserve that authority rule in this snapshot.
			case !budgetView.LandingClaim && (projection.ElapsedState != "" || projection.Elapsed >= projection.Limits.ElapsedDuration()):
				row.Fits, row.Reason = false, "elapsed-admission-closed"
			case row.AttemptDemand > row.AttemptsLeft:
				row.Fits, row.Reason = false, "attempt-headroom"
			case row.ReservationMinutes > row.ReservedMinutesLeft:
				row.Fits, row.Reason = false, "reserved-minute-headroom"
			}
		} else if projection.Unknown != nil {
			row.Reason = projection.Unknown.Record + ": " + projection.Unknown.Reason
		} else {
			row.Reason = "budget projection is unknown"
		}
		forecast.Budgets = append(forecast.Budgets, row)
	}
	return forecast, nil
}

func saturatingLeft(limit, used uint64) uint64 {
	if limit <= used {
		return 0
	}
	return limit - used
}

func batchBudgetProjection(root string, unit batch.Unit, incoming *batch.Unit, at time.Time) (batchCostBudgetProjection, error) {
	if incoming != nil && unit.GoalID == incoming.GoalID {
		binding, err := dispatchcore.ResolveGoalBinding(incoming.SeatRoot, incoming.GoalID, at)
		if err != nil {
			return batchCostBudgetProjection{}, err
		}
		if binding.Revision != incoming.Claim.Revision || binding.File == nil || binding.File.Claimed == nil ||
			binding.File.Claimed.AccountingRevision != incoming.Claim.AccountingRevision {
			return batchCostBudgetProjection{}, fmt.Errorf("BATCH_COST_INPUT_MOVED: incoming claim changed before handover")
		}
		return batchCostBudgetProjection{Budget: dispatchcore.ProjectBudget(incoming.SeatRoot, binding.File, at), LandingClaim: binding.File.IsLandingClaim()}, nil
	}
	controlRoot := batch.ModuleRoot(root)
	endpoint, err := goal.ResolveEndpoint(controlRoot)
	if err != nil {
		return batchCostBudgetProjection{}, err
	}
	projection, err := goal.Project(endpoint, true, at)
	if err != nil {
		return batchCostBudgetProjection{}, err
	}
	if projection.Tree == nil || projection.Tree.Live[unit.GoalID] == nil {
		return batchCostBudgetProjection{}, fmt.Errorf("BATCH_COST_AUTHORITY_REFUSED: member %s is absent from accepted ledger", unit.GoalID)
	}
	file := projection.Tree.Live[unit.GoalID]
	if file.State != goal.StateClaimed || file.Claimed == nil || file.Claimed.Revision != unit.Claim.Revision ||
		file.Claimed.AccountingRevision != unit.Claim.AccountingRevision || file.IsFencedClaim() {
		return batchCostBudgetProjection{}, fmt.Errorf("BATCH_COST_AUTHORITY_REFUSED: member %s claim changed or fenced", unit.GoalID)
	}
	return batchCostBudgetProjection{Budget: dispatchcore.ProjectBudget(controlRoot, file, at), LandingClaim: file.IsLandingClaim()}, nil
}

func forecastCostRefusal(forecast batch.CostForecast) error {
	var reasons []string
	for _, budget := range forecast.Budgets {
		if !budget.Fits {
			reasons = append(reasons, fmt.Sprintf("%s %s demand=%d attempts/%d reserved-minutes remaining=%d/%d", budget.GoalID,
				budget.Reason, budget.AttemptDemand, budget.ReservationMinutes, budget.AttemptsLeft, budget.ReservedMinutesLeft))
		}
	}
	if len(reasons) == 0 {
		return nil
	}
	return fmt.Errorf("BATCH_COST_HEADROOM_REFUSED: %s", strings.Join(reasons, "; "))
}
