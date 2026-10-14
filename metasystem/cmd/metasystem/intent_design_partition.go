package main

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

// partitionDesignFold transfers uncertain scope conservatively. A partial
// remainder requires complete ownership and dependency declarations.
func partitionDesignFold(fold *goal.DesignExit, units []launch.UnitSize, mappings map[string]designFoldMapping, capacity int) error {
	sections, scope, transfer := designFoldSections(fold.Page), map[string]goal.DesignUnitScope{}, map[string]bool{}
	for _, unit := range launch.ParseUnitSizes(fold.Page) {
		if unit.Production == nil || *unit.Production > 250 {
			transfer[unit.Name] = true
		}
	}
	for _, m := range mappings {
		if m.Unresolved {
			transfer[m.Unit] = true
		}
	}
	for _, row := range fold.Scope {
		if _, exists := scope[row.Unit]; exists || len(row.Decisions) == 0 || row.DependsOn == nil || !slices.ContainsFunc(units, func(u launch.UnitSize) bool { return u.Name == row.Unit }) {
			return fmt.Errorf("unit %s has an incomplete or duplicate scope mapping", row.Unit)
		}
		for _, decision := range row.Decisions {
			if sections[decision] == "" || decision == "## Units" || decision == "## Acceptance items" || strings.HasPrefix(decision, "# ") {
				return fmt.Errorf("unit %s names an absent or shared record section", row.Unit)
			}
		}
		for _, dependency := range row.DependsOn {
			if !slices.ContainsFunc(units, func(u launch.UnitSize) bool { return u.Name == dependency }) {
				return fmt.Errorf("unit %s has an unknown dependency %s", row.Unit, dependency)
			}
		}
		owned, depth := false, 0
		for _, line := range strings.Split(fold.Page, "\n") {
			if !strings.HasPrefix(line, "#") {
				continue
			}
			rank := len(line) - len(strings.TrimLeft(line, "#"))
			if owned && rank <= depth {
				owned = false
			}
			if slices.Contains(row.Decisions, line) {
				owned, depth = true, rank
			}
			if owned && !slices.Contains(row.Decisions, line) {
				row.Decisions = append(row.Decisions, line)
			}
		}
		scope[row.Unit] = row
	}
	if fold.AuthorAttempt > 0 && len(scope) != len(units) {
		return fmt.Errorf("the final author did not map every unit's Decision ownership and dependencies; its proposal is retained")
	}
	for _, line := range strings.Split(fold.AuthorPrior, "\n") {
		if raw, ok := strings.CutPrefix(line, "Scope item: "); ok {
			var prior goal.DesignUnitScope
			if err := json.Unmarshal([]byte(raw), &prior); err != nil {
				return err
			}
			for _, dependency := range prior.DependsOn {
				if !slices.Contains(scope[prior.Unit].DependsOn, dependency) {
					return fmt.Errorf("the revision cuts %s's dependency on %s", prior.Unit, dependency)
				}
			}
			for _, decision := range prior.Decisions {
				if !slices.Contains(scope[prior.Unit].Decisions, decision) {
					return fmt.Errorf("the revision removes %s's Decision ownership for %s", prior.Unit, decision)
				}
			}
		}
	}
	for i := len(units) - 1; i >= 0 && len(units)-len(transfer) > capacity; i-- {
		transfer[units[i].Name] = true
	}
	for _, m := range mappings {
		if len(scope) == len(units) && !slices.Contains(scope[m.Unit].Decisions, m.Decision) {
			return fmt.Errorf("finding %s is outside its unit's declared Decision ownership", m.Finding)
		}
	}
	if len(transfer) == 0 {
		_, err := launch.CheckDesignSize(fold.Page)
		return err
	}
	if len(scope) != len(units) {
		for _, unit := range units {
			transfer[unit.Name] = true
		}
	}
	removed := map[string]bool{}
	for changed := true; changed; {
		changed = false
		for unit := range transfer {
			for _, decision := range scope[unit].Decisions {
				removed[decision] = true
			}
		}
		for unit, row := range scope {
			if transfer[unit] {
				continue
			}
			dependent := slices.ContainsFunc(row.DependsOn, func(d string) bool { return transfer[d] }) || slices.ContainsFunc(row.Decisions, func(d string) bool { return removed[d] })
			if dependent {
				transfer[unit], changed = true, true
			}
		}
	}
	for _, unit := range units {
		if transfer[unit.Name] {
			fold.TransferUnits = append(fold.TransferUnits, unit.Name)
		}
	}
	var retained []goal.ReviewObligation
	for _, item := range fold.Obligations {
		if transfer[item.DesignItem.Unit] {
			fold.TransferObligations = append(fold.TransferObligations, item)
		} else {
			retained = append(retained, item)
		}
	}
	fold.Obligations = retained
	if len(transfer) == len(units) {
		return nil
	}
	var result strings.Builder
	heading, skip, fenced := "", false, false
	for _, line := range strings.SplitAfter(fold.Page, "\n") {
		if strings.HasPrefix(line, "```") || strings.HasPrefix(line, "~~~") {
			fenced = !fenced
		}
		if !fenced && strings.HasPrefix(line, "#") {
			heading = strings.TrimSuffix(line, "\n")
			skip = removed[heading]
		}
		if skip {
			continue
		}
		if heading == "## Units" && strings.HasPrefix(strings.TrimSpace(line), "|") {
			cells := strings.Split(line, "|")
			if len(cells) > 2 && transfer[strings.TrimSpace(cells[1])] {
				continue
			}
		}
		if raw, ok := strings.CutPrefix(line, "Fold item: "); ok {
			var m designFoldMapping
			if err := json.Unmarshal([]byte(raw), &m); err != nil {
				return err
			}
			if transfer[m.Unit] {
				continue
			}
		}
		if raw, ok := strings.CutPrefix(line, "Scope item: "); ok {
			var row goal.DesignUnitScope
			if err := json.Unmarshal([]byte(raw), &row); err != nil {
				return err
			}
			if transfer[row.Unit] {
				continue
			}
		}
		result.WriteString(line)
	}
	fold.Page = result.String()
	retainedUnits, err := launch.CheckDesignSize(fold.Page)
	if err != nil || len(retainedUnits) != len(units)-len(transfer) || len(retainedUnits) > capacity {
		return fmt.Errorf("the remainder is still too large or loses unit scope: %v", err)
	}
	return nil
}
