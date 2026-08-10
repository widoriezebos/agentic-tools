package dispatch

import (
	"fmt"
	"path/filepath"
	"sort"
)

// A job chain is a lineage of records linked by parentJob: the root has no
// parent, every follow-up names its predecessor. Chain scans here are
// deliberately tolerant — an unreadable record or ancestor drops that record
// from the result instead of failing the whole scan, because chains are
// enumerated while other writers are mid-flight.

// chainMember pairs a record with the file it was read from.
type chainMember struct {
	path   string
	record map[string]any
}

// chainMembers returns every readable job record whose ancestry resolves to
// root. A record whose parent walk breaks (missing ancestor, cycle,
// non-string parent) belongs to no chain and is skipped.
func chainMembers(jobsDir, root string) ([]chainMember, error) {
	paths, err := filepath.Glob(filepath.Join(jobsDir, "*.json"))
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	var members []chainMember
	for _, path := range paths {
		record, err := readObject(path)
		if err != nil {
			continue
		}
		current := record
		seen := map[string]bool{}
		for {
			parent, present := current["parentJob"]
			if !present || parent == nil {
				break
			}
			name, ok := parent.(string)
			if !ok || seen[name] {
				break
			}
			seen[name] = true
			next, err := readObject(filepath.Join(jobsDir, name+".json"))
			if err != nil {
				break
			}
			current = next
		}
		if asString(current["jobId"]) == root {
			members = append(members, chainMember{path: path, record: record})
		}
	}
	return members, nil
}

// LatestChainRecord returns the path of the highest-round record in root's
// chain. A record without a numeric round counts as round zero. No chain
// member at all is a silent refusal (exit 1) — the caller distinguishes "no
// such chain" from real errors.
func LatestChainRecord(jobsDir, root string) (string, error) {
	members, err := chainMembers(jobsDir, root)
	if err != nil {
		return "", err
	}
	best := ""
	bestRound := 0.0
	for _, member := range members {
		round := 0.0
		if f, ok := numFloat(member.record["round"]); ok {
			round = f
		}
		if best == "" || round > bestRound {
			best, bestRound = member.path, round
		}
	}
	if best == "" {
		return "", silentRefusal(1)
	}
	return best, nil
}

// ChainMemberStatuses lists "jobId|status" for every member of root's chain,
// optionally restricted to terminal records. Members without a job id are
// skipped — a record that cannot even name itself has nothing to report.
func ChainMemberStatuses(jobsDir, root string, terminalOnly bool) ([]string, error) {
	members, err := chainMembers(jobsDir, root)
	if err != nil {
		return nil, err
	}
	var lines []string
	for _, member := range members {
		id := asString(member.record["jobId"])
		status := asString(member.record["status"])
		if id == "" {
			continue
		}
		if terminalOnly && !terminalStatuses[status] {
			continue
		}
		lines = append(lines, fmt.Sprintf("%s|%s", id, status))
	}
	return lines, nil
}
