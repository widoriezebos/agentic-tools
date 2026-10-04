package plain

import (
	"path/filepath"
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
