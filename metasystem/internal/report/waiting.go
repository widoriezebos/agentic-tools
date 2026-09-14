package report

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/run"
)

// WaitingLines returns the restart commands for one proven owner lineage.
// Enumeration launches nothing; a malformed version-2 row remains a visible
// read failure instead of disappearing from orientation.
func WaitingLines(root, ownerLineage string) ([]string, error) {
	rows, failures := run.PendingWaiters(root, ownerLineage)
	lines := make([]string, 0, len(rows)+len(failures))
	for _, row := range rows {
		lines = append(lines, waitingLine(row))
	}
	for _, failure := range failures {
		lines = append(lines, "WAITING unreadable waiter row: "+failure.Error())
	}
	return lines, nil
}

// CurrentWaitingLines resolves the checkout's current holder before exposing
// predecessor rows; another seat's rows are never offered for takeover.
func CurrentWaitingLines(root string) ([]string, error) {
	// An installation with no checkout lease holds no seat, so it owns no
	// waiter rows: orientation there is silence, not a read failure. A
	// template or local-only installation reaches its Stop this way.
	if _, statErr := os.Stat(filepath.Join(root, "artifacts", "agents", "mains", "worktree-lease.json")); os.IsNotExist(statErr) {
		return nil, nil
	}
	holder, err := lease.CurrentHolder(root)
	if err != nil {
		return nil, err
	}
	lineages := []string{holder.OwnerLineage}
	data, readErr := os.ReadFile(filepath.Join(root, "artifacts", "agents", "mains", "worktree-lease.json"))
	if readErr == nil {
		var current lease.Lease
		if json.Unmarshal(data, &current) == nil {
			for _, predecessor := range connectedTakeoverPredecessors(current, holder.MainId, holder.ClaimEpoch) {
				lineage, exists, lineageErr := announcedOwnerLineage(root, predecessor)
				if lineageErr != nil {
					return nil, lineageErr
				}
				if exists {
					lineages = append(lineages, lineage)
				}
			}
		}
	}
	rows, failures := run.PendingWaitersForLineages(root, lineages)
	lines := make([]string, 0, len(rows)+len(failures))
	for _, row := range rows {
		lines = append(lines, waitingLine(row))
	}
	for _, failure := range failures {
		lines = append(lines, "WAITING unreadable waiter row: "+failure.Error())
	}
	return lines, nil
}

func waitingLine(row run.Waiter) string {
	return fmt.Sprintf("WAITING %s %s until %s: %s", row.Kind, row.TargetID, row.Deadline, run.WaitResumeCommand(row))
}

// SucceededWaitOwner proves that candidateMain is connected to the current
// holder by the lease's recorded takeover chain, then returns the logical
// lineage recorded by that predecessor's announcement.
func SucceededWaitOwner(root, holderMain string, claimEpoch int64, candidateMain string) (string, bool, error) {
	data, err := os.ReadFile(filepath.Join(root, "artifacts", "agents", "mains", "worktree-lease.json"))
	if err != nil {
		return "", false, err
	}
	var current lease.Lease
	if err := json.Unmarshal(data, &current); err != nil {
		return "", false, fmt.Errorf("checkout lease schema is invalid")
	}
	for _, predecessor := range connectedTakeoverPredecessors(current, holderMain, claimEpoch) {
		if predecessor == candidateMain {
			return announcedOwnerLineage(root, candidateMain)
		}
	}
	return "", false, nil
}

func connectedTakeoverPredecessors(current lease.Lease, holderMain string, claimEpoch int64) []string {
	if current.HolderMainId != holderMain || current.ClaimEpoch != claimEpoch {
		return nil
	}
	next := holderMain
	nextEpoch := claimEpoch
	var predecessors []string
	for i := len(current.Takeovers) - 1; i >= 0; i-- {
		takeover := current.Takeovers[i]
		if takeover.Reason != "holder-death" || takeover.ToMainId != next || takeover.ClaimEpoch != nextEpoch {
			continue
		}
		predecessors = append(predecessors, takeover.FromMainId)
		next = takeover.FromMainId
		nextEpoch--
	}
	return predecessors
}

func announcedOwnerLineage(root, mainID string) (string, bool, error) {
	paths, err := filepath.Glob(filepath.Join(root, "artifacts", "agents", "mains", "*.json"))
	if err != nil {
		return "", false, err
	}
	sort.Strings(paths)
	for _, path := range paths {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return "", false, fmt.Errorf("main announcement %s is unreadable: %w", filepath.Base(path), readErr)
		}
		var announcement struct {
			MainID       string `json:"mainId"`
			OwnerLineage string `json:"ownerLineage"`
		}
		if json.Unmarshal(data, &announcement) != nil || announcement.MainID != mainID {
			continue
		}
		if announcement.OwnerLineage == "" {
			announcement.OwnerLineage = announcement.MainID
		}
		return announcement.OwnerLineage, true, nil
	}
	return "", false, nil
}
