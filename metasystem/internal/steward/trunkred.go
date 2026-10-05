package steward

import (
	"fmt"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

func checkTrunkRedWith(repoRoot string, now time.Time, ledger *healthLedger) RoleVerdict {
	if !ledger.read().newWorld {
		return roleAlive(RoleTrunkRed, "the bootstrap ledger has no trunk-red register")
	}
	if err := ledger.endpointErr; err != nil {
		return roleUnknown(RoleTrunkRed, "the trunk-red ledger endpoint is unreadable: "+err.Error(), "repair the goal sync configuration, then run metasystem system check")
	}
	return checkTrunkRedFromProjection(repoRoot, now, ledger.projection, ledger.projectionErr)
}

func checkTrunkRedFromProjection(repoRoot string, now time.Time, projection goal.Projection, projectionErr error) RoleVerdict {
	if projectionErr != nil {
		return roleUnknown(RoleTrunkRed, "the trunk-red ledger is unreadable: "+projectionErr.Error(), "repair or fetch the goal ledger, then run metasystem system check")
	}

	var open []goal.TrunkRedEntry
	var unowned []string
	tracked := 0
	for _, entry := range projection.Tree.TrunkRed {
		if entry.Closed != nil {
			continue
		}
		if entry.EntryClass() != goal.TrunkRedClassTrunkRed {
			// Flake, hang and quality entries are tracked defects, not reds on
			// main; they never hold a landing and do not make this role dead.
			tracked++
			continue
		}
		open = append(open, entry)
		if entry.Owner.Machine == "" {
			unowned = append(unowned, entry.ID)
		}
	}
	if len(unowned) > 0 {
		return roleDead(RoleTrunkRed, "open trunk red without an owner: "+strings.Join(unowned, ", "),
			"metasystem incident claim "+unowned[0]+" --goal <goal>")
	}
	defects := ""
	if tracked > 0 {
		defects = fmt.Sprintf("; %d flake or hang entr%s tracked separately (metasystem incident list)", tracked, map[bool]string{true: "y", false: "ies"}[tracked == 1])
	}
	if len(open) == 0 {
		return roleAlive(RoleTrunkRed, "no open trunk red"+defects)
	}
	oldest, _ := time.Parse(time.RFC3339, open[0].Opened)
	for _, entry := range open[1:] {
		opened, _ := time.Parse(time.RFC3339, entry.Opened)
		if opened.Before(oldest) {
			oldest = opened
		}
	}
	return roleAlive(RoleTrunkRed, fmt.Sprintf("%d open, all owned; oldest %s", len(open), deliveryAge(now, oldest))+defects)
}
