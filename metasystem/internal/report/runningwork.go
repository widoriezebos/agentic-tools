package report

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// The turn-end "is anything still working?" answer (relocated from
// supervision-hook.sh): live
// delegate jobs read from decoded records instead of a raw status grep,
// mission runners found by argv tokens instead of pgrep-plus-sed, and gate
// runs. This is the surface the README's unsupervised-runs failure class
// hangs off — a false "nothing running" while a mission burns.

// runningJobDetail names one live delegate: "role jobId [status, runtime]".
func runningJobDetail(path string) (string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	var record struct {
		JobId   string `json:"jobId"`
		Role    string `json:"role"`
		Runtime string `json:"runtime"`
		Status  string `json:"status"`
	}
	if json.Unmarshal(data, &record) != nil {
		return "", false
	}
	if record.Status != "pending" && record.Status != "running" {
		return "", false
	}
	if record.JobId == "" {
		record.JobId = strings.TrimSuffix(filepath.Base(path), ".json")
	}
	if record.Role == "" {
		record.Role = "?"
	}
	if record.Runtime == "" {
		record.Runtime = "?"
	}
	return fmt.Sprintf("%s %s [%s, %s]", record.Role, record.JobId, record.Status, record.Runtime), true
}

// missionRootBase extracts the basename of a runner argv's --root value.
func missionRootBase(argv string) string {
	tokens := strings.Fields(argv)
	for i, token := range tokens {
		if token == "--root" && i+1 < len(tokens) {
			return filepath.Base(tokens[i+1])
		}
	}
	return ""
}

// RunningWorkClause composes the active clause of the turn-end sentence —
// empty when nothing is running. The wording is the hook's historical one:
// "N helper agent(s): role id [status, runtime]; ...", ", and a mission
// still going in a, b", ", and the test gates". The facts come from
// the checkout-scoped scanner: argv matching
// answers for the whole machine, so another checkout's mission could
// swing this checkout's sentence.
func RunningWorkClause(repo string) string {
	scan := Scan(repo)
	var details, missions []string
	gates := false
	seen := map[string]bool{}
	for _, item := range scan.Busy {
		switch item.Kind {
		case "job":
			details = append(details, item.Detail)
		case "mission":
			if !seen[item.Id] {
				seen[item.Id] = true
				missions = append(missions, item.Id)
			}
		case "gate":
			gates = true
		}
	}
	sort.Strings(details)
	sort.Strings(missions)
	return composeRunningClause(details, missions, gates)
}

// composeRunningClause is the pure wording: pieces joined ", and ", empty
// when idle. Split out because the process-scan half cannot be pinned in a
// unit test that itself runs under a live gate.
func composeRunningClause(details, missions []string, gates bool) string {
	var pieces []string
	if len(details) > 0 {
		pieces = append(pieces, fmt.Sprintf("%d helper agent(s): %s", len(details), strings.Join(details, "; ")))
	}
	if len(missions) > 0 {
		pieces = append(pieces, "a mission still going in "+strings.Join(missions, ", "))
	}
	if gates {
		pieces = append(pieces, "the test gates")
	}
	return strings.Join(pieces, ", and ")
}
