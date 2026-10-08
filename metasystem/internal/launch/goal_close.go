package launch

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
)

// CloseGoalReviewChains closes local review records after a confirmed
// conclusion, without changing any round's read, material count or decision.
func (runner *UnitRunner) CloseGoalReviewChains(goalID, reason string, worktrees []string) error {
	_, err := runner.goalReviewChains(goalID, reason, worktrees, true)
	return err
}

// GoalReviewCleanupPending reads the same unit and stop records that conclusion closes.
func (runner *UnitRunner) GoalReviewCleanupPending(goalID string, worktrees []string) (bool, error) {
	return runner.goalReviewChains(goalID, "", worktrees, false)
}

func (runner *UnitRunner) goalReviewChains(goalID, reason string, worktrees []string, close bool) (bool, error) {
	pending := false
	unitRoot := runner.root()
	entries, err := os.ReadDir(unitRoot)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var failures []error
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		id := entry.Name()
		path := filepath.Join(unitRoot, id, "run.json")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			continue
		}
		record, err := runner.read(id)
		if err == nil && (record.ID != id || record.Goal == "") {
			err = fmt.Errorf("unit record %s has no matching run and goal", path)
		}
		if err != nil {
			failures = append(failures, err)
			continue
		}
		if record.Goal != goalID || !slices.Contains(worktrees, record.Worktree) {
			continue
		}
		inspect := func() error {
			if err == nil && record.Goal == goalID && slices.Contains(worktrees, record.Worktree) {
				if record.ReviewCloseReason == "" {
					pending = true
					if close {
						record.ReviewCloseReason = reason
						err = writeUnitJSON(path, record, unitRoot)
					}
				}
				if err == nil {
					for _, round := range record.Rounds {
						stopPath := filepath.Join(round.Directory, "stop-register.json")
						if _, statErr := os.Stat(stopPath); os.IsNotExist(statErr) {
							continue
						}
						entry := map[string]any{}
						data, readErr := os.ReadFile(stopPath)
						if readErr == nil {
							readErr = json.Unmarshal(data, &entry)
						}
						if readErr == nil && entry["status"] != "open" && entry["status"] != "closed" && entry["status"] != "cleared" {
							readErr = fmt.Errorf("stop record %s has no readable closure state", stopPath)
						}
						if readErr != nil {
							failures = append(failures, readErr)
							continue
						}
						if entry["status"] == "open" {
							pending = true
							if close {
								entry["status"], entry["reason"] = "closed", record.ReviewCloseReason
								failures = append(failures, writeUnitJSON(stopPath, entry, unitRoot))
							}
						}
					}
				}
			}
			return err
		}
		if close {
			lock, lockErr := runner.lock(id)
			if lockErr != nil {
				failures = append(failures, lockErr)
				continue
			}
			record, err = runner.read(id)
			failures = append(failures, inspect())
			releaseUnitLock(lock)
		} else {
			failures = append(failures, inspect())
		}
	}
	return pending, errors.Join(failures...)
}
