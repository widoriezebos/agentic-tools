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

func checkTrunkRed(repoRoot string, now time.Time) RoleVerdict {
	if !goal.NewWorld(repoRoot) {
		return roleAlive(RoleTrunkRed, "the bootstrap ledger has no trunk-red register")
	}
	endpoint, err := goal.ResolveEndpoint(repoRoot)
	if err != nil {
		return roleUnknown(RoleTrunkRed, "the trunk-red ledger endpoint is unreadable: "+err.Error(), "repair the goal sync configuration, then run metasystem health")
	}
	projection, err := goal.Project(endpoint, false, now)
	return checkTrunkRedFromProjection(repoRoot, now, projection, err, config.ResolveBatchLanding)
}

func checkTrunkRedFromProjection(repoRoot string, now time.Time, projection goal.Projection, projectionErr error, resolveBatchLanding func(string, string, func() time.Time) (config.BatchLanding, error)) RoleVerdict {
	if projectionErr != nil {
		return roleUnknown(RoleTrunkRed, "the trunk-red ledger is unreadable: "+projectionErr.Error(), "repair or fetch the goal ledger, then run metasystem health")
	}
	cadenceRoot := repoRoot
	if configured, _, configErr := config.Get(config.GetParams{Key: config.BatchRootKey, ConfPath: filepath.Join(repoRoot, "metasystem.conf"), Default: "", DefaultSet: true}); configErr == nil && strings.TrimSpace(configured) != "" {
		cadenceRoot = configured
	}
	remedy := fmt.Sprintf("metasystem gate cadence-tick --root %q", cadenceRoot)
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

	staleBatches, batchErr := staleUnrecordedTrunkRedBatchesWith(repoRoot, now, resolveBatchLanding)
	if batchErr != nil {
		return roleUnknown(RoleTrunkRed, batchErr.Error(), "repair the configured landing batch root, then run metasystem health")
	}

	var open []goal.TrunkRedEntry
	var unowned []string
	for _, entry := range projection.Tree.TrunkRed {
		if entry.Closed != nil {
			continue
		}
		open = append(open, entry)
		if entry.Owner.Machine == "" {
			unowned = append(unowned, entry.ID)
		}
	}
	if len(unowned) > 0 {
		return roleDead(RoleTrunkRed, "open trunk red without an owner: "+strings.Join(unowned, ", "),
			"metasystem goal trunk-red own --id "+unowned[0]+" --goal <goal>")
	}
	if len(staleBatches) > 0 {
		first := staleBatches[0]
		return roleDead(RoleTrunkRed, "held trunk red was not recorded: "+strings.Join(staleBatches, ", "),
			"metasystem landing batch tick; then metasystem goal trunk-red own --id <id> --goal <goal> (first held batch "+first+")")
	}
	if len(open) == 0 {
		return roleAlive(RoleTrunkRed, "no open trunk red")
	}
	oldest, _ := time.Parse(time.RFC3339, open[0].Opened)
	for _, entry := range open[1:] {
		opened, _ := time.Parse(time.RFC3339, entry.Opened)
		if opened.Before(oldest) {
			oldest = opened
		}
	}
	return roleAlive(RoleTrunkRed, fmt.Sprintf("%d open, all owned; oldest %s", len(open), deliveryAge(now, oldest)))
}

func staleUnrecordedTrunkRedBatches(repoRoot string, now time.Time) ([]string, error) {
	return staleUnrecordedTrunkRedBatchesWith(repoRoot, now, config.ResolveBatchLanding)
}

func staleUnrecordedTrunkRedBatchesWith(repoRoot string, now time.Time, resolveBatchLanding func(string, string, func() time.Time) (config.BatchLanding, error)) ([]string, error) {
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
	directory := filepath.Join(settings.Root, "artifacts", "agents", "landing-batches")
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
