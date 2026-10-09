package dispatch

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// CloseGoalReviewChains records a confirmed person's conclusion on each
// still-open critic root. Findings and reads retain their original evidence.
func CloseGoalReviewChains(repoRoot, goalID, reason string) error {
	_, err := goalReviewChains(repoRoot, goalID, reason, true)
	return err
}

// GoalReviewCleanupPending reads the same critic roots that conclusion closes.
func GoalReviewCleanupPending(repoRoot, goalID string) (bool, error) {
	return goalReviewChains(repoRoot, goalID, "", false)
}

func goalReviewChains(repoRoot, goalID, reason string, close bool) (bool, error) {
	pending := false
	state, scanErr := readCritiqueStateAt(filepath.Join(repoRoot, "artifacts", "agents"))
	var failures []error
	failures = append(failures, scanErr)
	entries, _ := os.ReadDir(filepath.Join(repoRoot, "artifacts", "agents", "jobs"))
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".json") && state.records[strings.TrimSuffix(entry.Name(), ".json")] == nil {
			failures = append(failures, fmt.Errorf("review record %s has no readable identity", entry.Name()))
		}
	}
	for id, record := range state.records {
		role := asString(record["role"])
		if asString(record["goalId"]) == goalID && state.chainRoot(id) == "" {
			failures = append(failures, fmt.Errorf("review %s has unreadable ancestry", id))
		}
		if state.chainRoot(id) != id || asString(record["goalId"]) != goalID || role != "code-critic" && role != "design-critic" && role != "warden" {
			continue
		}
		if closed, _ := record["chainClosed"].(bool); !closed {
			pending = true
		}
		if !close {
			continue
		}
		_, err := withFindingRegisterLock(repoRoot, func() (string, error) {
			err := withRecordSessionLock(repoRoot, id, func(path string, transaction *SessionIndexTransaction) error {
				root, err := readObject(path)
				if err != nil {
					return err
				}
				if asString(root["goalId"]) != goalID {
					return nil
				}
				if closed, _ := root["chainClosed"].(bool); closed {
					return transaction.syncRecord(id, root)
				}
				root["chainClosed"], root["chainCloseReason"] = true, reason
				delete(root, closureField)
				if err := writeRecord(path, root); err != nil {
					return err
				}
				return transaction.syncRecord(id, root)
			})
			return "", err
		})
		if err != nil {
			failures = append(failures, fmt.Errorf("review %s: %w", id, err))
		}
	}
	return pending, errors.Join(failures...)
}
