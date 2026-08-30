package steward

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/run"
)

// refreshGovernedObligations is the steward-tick evaluator. It re-observes
// live assumptions and repairs only a debt write already demanded by a
// terminal exhausted record.
func refreshGovernedObligations(repoRoot string, now time.Time) []string {
	store := &run.Store{Root: repoRoot}
	var failures []string
	for _, path := range run.RecordFiles(repoRoot) {
		id := strings.TrimSuffix(filepath.Base(path), ".json")
		record, err := store.Read(id)
		if err != nil || record == nil {
			failures = append(failures, id+": unreadable")
			continue
		}
		if record.Governed == nil {
			continue
		}
		if run.Terminal(record.Status) {
			if record.Governed.Exhausted && !record.Governed.RetroDebtRaised {
				if err := store.RepairGovernedDebt(id); err != nil {
					failures = append(failures, id+": retro debt repair failed: "+err.Error())
				}
			}
			continue
		}
		observation := dispatch.ObserveGovernedRun(repoRoot, record, now)
		if err := store.RecordGovernedObservation(id, observation); err != nil {
			failures = append(failures, id+": observation write failed: "+err.Error())
		}
	}
	sort.Strings(failures)
	return failures
}

func checkGovernedObligations(repoRoot string) RoleVerdict {
	store := &run.Store{Root: repoRoot}
	var failures []string
	failures = append(failures, directValidationWindowFailures(repoRoot)...)
	seen := 0
	for _, path := range run.RecordFiles(repoRoot) {
		id := strings.TrimSuffix(filepath.Base(path), ".json")
		record, err := store.Read(id)
		if err != nil || record == nil {
			failures = append(failures, id+" observation unavailable")
			continue
		}
		if record.Governed == nil {
			if record.Kind == "suite" && record.Display == "weight-triggered direct validation" {
				failures = append(failures, id+" is a recurring ungoverned execution with a governing-effect attempt")
			}
			continue
		}
		seen++
		g := record.Governed
		if g.Observation == nil || g.Observation.AssumptionState == run.AssumptionUnavailable {
			failures = append(failures, id+" assumption observation unavailable")
		} else if g.Observation.AssumptionState == run.AssumptionDrift {
			failures = append(failures, id+" ASSUMPTION_DRIFT fields="+strings.Join(g.Observation.DriftedFields, ","))
		}
		if g.Exhausted || g.Breaker == run.BreakerExhausted || g.Breaker == run.BreakerAssumption {
			failures = append(failures, fmt.Sprintf("%s breaker=%s exhausted=%t", id, g.Breaker, g.Exhausted))
		}
		if g.Exhausted && !g.RetroDebtRaised {
			failures = append(failures, id+" exhausted without its terminalization-owned retro debt")
		}
	}
	if len(failures) > 0 {
		sort.Strings(failures)
		role := roleDead(RoleGovernedObligations, strings.Join(failures, "; "),
			"Wido chooses one: reduce, redesign, retire, or extend with a fresh complete tuple and obligation revision")
		role.NoAutomaticRemedy = true
		return role
	}
	if seen == 0 {
		return roleAlive(RoleGovernedObligations, "there are no governed obligation attempts")
	}
	return roleAlive(RoleGovernedObligations, fmt.Sprintf("%d governed obligation attempt(s) have matching typed evidence", seen))
}
