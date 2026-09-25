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
		job := JobRecord{
			Parent:    jobText(record, "parentJob"),
			Job:       jobText(record, "jobId"),
			Role:      jobText(record, "role"),
			Goal:      jobText(record, "goalId"),
			Round:     jobNumber(record, "round"),
			StartedAt: jobText(record, "startedAt"),
			CreatedAt: jobText(record, "createdAt"),
			Status:    jobText(record, "status"),
			// What the records already carry and this reader dropped: when a
			// job ended, the minutes it reserved, the deadline those minutes
			// are enforced against, and the round limit a critic chain's root
			// froze. None of them is a new fact; each was simply not read.
			EndedAt:          jobText(record, "endedAt"),
			CapMinutes:       jobNestedNumber(record, "capRequest", "minutes"),
			CapDeadline:      jobText(record, "capDeadline"),
			ReviewRoundLimit: jobOptionalNumber(record, "reviewRoundLimit"),
		}
		// A record that parses but cannot say which job it is, or what state
		// it is in, cannot be reasoned about: it is unreadable in the only
		// sense that matters here, and it names itself rather than being
		// dropped from the scan.
		if job.Job == "" {
			jobs.Unreadable = append(jobs.Unreadable, logical+": the job record names no jobId")
			continue
		}
		if job.Status == "" {
			jobs.Unreadable = append(jobs.Unreadable, logical+": job "+job.Job+" names no status")
			continue
		}
		jobs.Records = append(jobs.Records, job)
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

// jobOptionalNumber tells a key the record does not carry from one carrying
// zero. A round limit of none and a round limit of nought are different
// things, and only the first of them is null.
func jobOptionalNumber(record map[string]any, key string) *int {
	value, present := record[key]
	if !present {
		return nil
	}
	number, ok := value.(float64)
	if !ok {
		return nil
	}
	held := int(number)
	return &held
}

// jobNestedNumber reads one number out of one nested object, which is where
// the reserved cap lives.
func jobNestedNumber(record map[string]any, key, field string) *int {
	nested, ok := record[key].(map[string]any)
	if !ok {
		return nil
	}
	return jobOptionalNumber(nested, field)
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
