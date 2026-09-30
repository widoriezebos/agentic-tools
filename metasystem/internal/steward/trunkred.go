package steward

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

const cadenceHealthInterval = 6 * time.Hour

type trunkRedBatchRecord struct {
	BatchID  string `json:"batchId"`
	State    string `json:"state"`
	TrunkRed *struct {
		Opid    string            `json:"opid"`
		Entries []json.RawMessage `json:"entries"`
	} `json:"trunkRed"`
	History []struct {
		At     string `json:"at"`
		Verb   string `json:"verb"`
		Detail string `json:"detail"`
	} `json:"history"`
}

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
	return checkTrunkRedFromProjection(repoRoot, now, ledger.projection, ledger.projectionErr, config.ResolveBatchLanding, LandingLaneRoot)
}

func checkTrunkRedFromProjection(repoRoot string, now time.Time, projection goal.Projection, projectionErr error, resolveBatchLanding func(string, string, func() time.Time) (config.BatchLanding, error), laneRoot func(string, time.Time) (string, bool, error)) RoleVerdict {
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
	// The landing owner runs the deep validation cadence in its own loop, so
	// a missing, overdue or red cadence is repaired by that owner running.
	remedy := fmt.Sprintf("the landing owner runs the deep validation cadence; keep it running with metasystem system start --repo %q", cadenceRoot)
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

	staleBatches, batchErr := staleUnrecordedTrunkRedBatchesWith(repoRoot, now, resolveBatchLanding, laneRoot)
	if batchErr != nil {
		return roleUnknown(RoleTrunkRed, batchErr.Error(), "repair the configured landing batch root, then run metasystem system check")
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
	if len(staleBatches) > 0 {
		first := staleBatches[0]
		return roleDead(RoleTrunkRed, "held trunk red was not recorded: "+strings.Join(staleBatches, ", "),
			"the landing owner records it on its next pass (keep it running with metasystem system start); then metasystem incident claim <id> --goal <goal> (first held batch "+first+")")
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

func staleUnrecordedTrunkRedBatchesWith(repoRoot string, now time.Time, resolveBatchLanding func(string, string, func() time.Time) (config.BatchLanding, error), laneRoot func(string, time.Time) (string, bool, error)) ([]string, error) {
	if laneRoot != nil {
		root, configured, err := laneRoot(repoRoot, now)
		if err != nil {
			return nil, fmt.Errorf("the trunk-red batch root is unreadable: %w", err)
		}
		if !configured {
			return nil, nil
		}
		return staleUnrecordedTrunkRedBatchesIn(root, now)
	}
	conf := filepath.Join(repoRoot, "metasystem.conf")
	configured, _, err := config.Get(config.GetParams{Key: config.BatchRootKey, ConfPath: conf, Default: "", DefaultSet: true})
	if err != nil {
		return nil, fmt.Errorf("the trunk-red batch-root configuration is unreadable: %w", err)
	}
	if strings.TrimSpace(configured) == "" {
		return nil, nil
	}
	settings, err := resolveBatchLanding(conf, repoRoot, func() time.Time { return now })
	if err != nil {
		return nil, fmt.Errorf("the trunk-red batch root is unreadable: %w", err)
	}
	return staleUnrecordedTrunkRedBatchesIn(settings.Root, now)
}

// staleUnrecordedTrunkRedBatchesIn reads one landing checkout's held batches.
func staleUnrecordedTrunkRedBatchesIn(root string, now time.Time) ([]string, error) {
	directory := filepath.Join(root, "artifacts", "agents", "landing-batches")
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, fmt.Errorf("the trunk-red batch root is unreadable at %s: %w", directory, err)
	}
	var stale []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(directory, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("the trunk-red batch root is unreadable at %s: %w", path, err)
		}
		var record trunkRedBatchRecord
		if err := json.Unmarshal(data, &record); err != nil {
			return nil, fmt.Errorf("the trunk-red batch root has an unreadable record at %s: %w", path, err)
		}
		if record.State != "held-trunk-red" || record.TrunkRed == nil || len(record.TrunkRed.Entries) != 0 {
			continue
		}
		var heldAt time.Time
		for _, history := range record.History {
			if history.Verb != "trunk-red-hold" {
				continue
			}
			candidate, err := time.Parse(time.RFC3339Nano, history.At)
			if err != nil {
				return nil, fmt.Errorf("the trunk-red batch root has an unreadable hold time at %s: %w", path, err)
			}
			if heldAt.IsZero() || candidate.After(heldAt) {
				heldAt = candidate
			}
		}
		if !heldAt.IsZero() && now.Sub(heldAt) > goal.DefaultPublishDeadline {
			batchID := record.BatchID
			if batchID == "" {
				batchID = strings.TrimSuffix(entry.Name(), ".json")
			}
			stale = append(stale, fmt.Sprintf("batch %s opid %s", batchID, record.TrunkRed.Opid))
		}
	}
	sort.Strings(stale)
	return stale, nil
}
