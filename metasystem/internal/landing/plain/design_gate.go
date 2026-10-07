package plain

import (
	"fmt"
	"path/filepath"
	"time"
)

// DesignCheck is the design verdict for a hand-in considered for publication.
// The lane keeps it outside the checkout's tracked narrator records.
type DesignCheck struct {
	Goal    string `json:"goal"`
	Commit  string `json:"commit"`
	Verdict string `json:"verdict"`
	Reason  string `json:"reason"`
	At      string `json:"at"`
}

func designGatePath(install string) string { return filepath.Join(Dir(install), "design-gate.jsonl") }

// RecordDesignCheck appends a design verdict to the lane's record folder.
func RecordDesignCheck(install string, check DesignCheck) error {
	return withLock(install, func() error { return appendLine(designGatePath(install), check) })
}

// DesignChecks reads the lane's design verdicts in append order.
func DesignChecks(install string) ([]DesignCheck, error) {
	return readLines[DesignCheck](designGatePath(install))
}

// ReturnDesignRefused returns only the hand-in whose design was checked.
// The queue lock keeps a newer hand-in from inheriting an older design refusal.
func ReturnDesignRefused(install string, check DesignCheck, now time.Time, effects ...ProveSeams) (entry Entry, changed bool, err error) {
	err = withLock(install, func() error {
		latest, ok, readErr := Latest(install, check.Goal)
		if readErr != nil {
			return readErr
		}
		if !ok || latest.SHA != check.Commit {
			return fmt.Errorf("%w: the checked commit of %s is no longer its newest hand-in", ErrNotWaiting, check.Goal)
		}
		entry, changed, readErr = returnLocked(install, check.Goal, check.Reason,
			&Cause{Kind: "own", Goal: check.Goal, SHA: check.Commit, Evidence: check.Reason}, nil, now, effects...)
		return readErr
	})
	return
}
