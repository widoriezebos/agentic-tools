package steward

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

const cadenceHealthInterval = 6 * time.Hour

// LandingLaneRoot resolves the landing lane a checkout lands through (U12:
// its own landing.batch-root against the host's one lane, never
// registering). The command layer sets it; nil reads the checkout's own
// setting, as before the host lane.
var LandingLaneRoot func(repoRoot string, now time.Time) (root string, configured bool, err error)

func checkTrunkRedWith(repoRoot string, now time.Time, ledger *healthLedger) RoleVerdict {
	if !ledger.read().newWorld {
		return roleAlive(RoleTrunkRed, "the bootstrap ledger has no trunk-red register")
	}
	if err := ledger.endpointErr; err != nil {
		return roleUnknown(RoleTrunkRed, "the trunk-red ledger endpoint is unreadable: "+err.Error(), "repair the goal sync configuration, then run metasystem system check")
	}
	return checkTrunkRedFromProjection(repoRoot, now, ledger.projection, ledger.projectionErr, LandingLaneRoot)
}

func checkTrunkRedFromProjection(repoRoot string, now time.Time, projection goal.Projection, projectionErr error, laneRoot func(string, time.Time) (string, bool, error)) RoleVerdict {
	if projectionErr != nil {
		return roleUnknown(RoleTrunkRed, "the trunk-red ledger is unreadable: "+projectionErr.Error(), "repair or fetch the goal ledger, then run metasystem system check")
	}
	cadenceRoot := repoRoot
	if laneRoot != nil {
		if root, configured, laneErr := laneRoot(repoRoot, now); laneErr == nil && configured {
			cadenceRoot = root
		}
	} else if configured, _, configErr := config.Get(config.GetParams{Key: config.BatchRootKey, ConfPath: filepath.Join(repoRoot, "metasystem.conf"), Default: "", DefaultSet: true}); configErr == nil && strings.TrimSpace(configured) != "" {
		cadenceRoot = configured
	}
	// The landing lane's agent runs the deep validation cadence (landing
	// validate) when it is due, so a missing, overdue or red cadence is
	// repaired by the lane running.
	remedy := fmt.Sprintf("the landing lane runs the deep validation cadence when it is due; keep its checkout running with metasystem system start --repo %q", cadenceRoot)
	if projection.Tree.Cadence == nil {
		return roleDead(RoleTrunkRed, "no deep validation cadence status is recorded", remedy)
	}
	cadence := projection.Tree.Cadence
	window, cadenceErr := time.Parse(time.RFC3339, cadence.ForcedWindowStart)
	if cadenceErr != nil {
		return roleUnknown(RoleTrunkRed, "the cadence window is unreadable: "+cadenceErr.Error(), remedy)
	}
	if !now.UTC().Before(window.Add(cadenceHealthInterval + time.Minute)) {
		return roleDead(RoleTrunkRed, fmt.Sprintf("deep validation cadence is overdue at trunk %s tree %s", cadence.TrunkCommit, cadence.TrunkTree), remedy)
	}
	if !cadence.Green() {
		return roleDead(RoleTrunkRed, fmt.Sprintf("deep validation cadence is non-green at trunk %s tree %s", cadence.TrunkCommit, cadence.TrunkTree), remedy)
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
