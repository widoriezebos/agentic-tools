package seat

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// NewGit resolves the ledger endpoint and returns the presence transport for
// this checkout. Presence rides the ledger remote and nothing else: one
// mechanism for seats on the same host and across hosts.
func NewGit(root string) (Git, error) {
	endpoint, err := goal.ResolveEndpoint(root)
	if err != nil {
		return Git{}, err
	}
	return Git{Root: root, Remote: endpoint.Remote, Local: endpoint.LocalMode()}, nil
}

// Claims joins the machines the ledger names in a Claimed: line at one
// captured accepted tip to the goals they hold. The second return explains
// an accepted tip that could not be read, in which case the reader reports
// standings from the refs alone and raises no flag.
func Claims(root string) (map[string][]string, string) {
	tip, exists, err := goal.AcceptedLedgerTip(root)
	if err != nil {
		return nil, "the accepted ledger tip is unreadable: " + err.Error()
	}
	if !exists {
		return map[string][]string{}, ""
	}
	projection, err := goal.ProjectAt(root, tip)
	if err != nil {
		return nil, "the accepted ledger tree is unreadable: " + err.Error()
	}
	claims := map[string][]string{}
	if projection.Tree == nil {
		return claims, ""
	}
	for id, file := range projection.Tree.Live {
		if file == nil || file.Claimed == nil || file.Claimed.Machine == "" {
			continue
		}
		claims[file.Claimed.Machine] = append(claims[file.Claimed.Machine], id)
	}
	for machine := range claims {
		sort.Strings(claims[machine])
	}
	return claims, ""
}

// Machine reads this checkout's enrolled nickname. A checkout without one
// performs no ledger act and publishes no presence.
func Machine(root string) (string, bool) {
	name, err := goal.ResolveMachine(root)
	if err != nil {
		return "", false
	}
	return name, true
}

// ReadJobs reads this machine's delegate job records. An unreadable record
// names itself rather than being silently dropped, because the chain it
// might have carried is the panel's running column.
func ReadJobs(root string) JobSet {
	dir := filepath.Join(root, "artifacts", "agents", "jobs")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return JobSet{}
		}
		return JobSet{Unreadable: []string{"artifacts/agents/jobs: " + err.Error()}}
	}
	jobs := JobSet{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		logical := filepath.ToSlash(filepath.Join("artifacts", "agents", "jobs", entry.Name()))
		data, readErr := os.ReadFile(filepath.Join(dir, entry.Name()))
		if readErr != nil {
			jobs.Unreadable = append(jobs.Unreadable, logical+": "+readErr.Error())
			continue
		}
		var record map[string]any
		if err := json.Unmarshal(data, &record); err != nil {
			jobs.Unreadable = append(jobs.Unreadable, logical+": the job record is not readable JSON")
			continue
		}
		jobs.Records = append(jobs.Records, JobRecord{
			Root:      jobText(record, "parentJob", "jobId"),
			Job:       jobText(record, "jobId"),
			Role:      jobText(record, "role"),
			Goal:      jobText(record, "goalId"),
			Round:     jobNumber(record, "round"),
			StartedAt: jobText(record, "startedAt"),
			CreatedAt: jobText(record, "createdAt"),
			Status:    jobText(record, "status"),
		})
	}
	return jobs
}

// jobText reads the first of keys that carries a string.
func jobText(record map[string]any, keys ...string) string {
	for _, key := range keys {
		if text, ok := record[key].(string); ok && text != "" {
			return text
		}
	}
	return ""
}

func jobNumber(record map[string]any, key string) int {
	switch value := record[key].(type) {
	case float64:
		return int(value)
	case json.Number:
		var parsed int
		if _, err := fmt.Sscanf(value.String(), "%d", &parsed); err == nil {
			return parsed
		}
	}
	return 0
}
