package missionrunner

import (
	"path/filepath"
	"sort"
	"strings"
)

// Job selection for the runner's reap loop and end-of-mission chain close.
// This package only decides WHICH jobs need action; the runner invokes the
// dispatch tooling on them, keeping process reaping with the code that owns
// job lifecycles.

// jobRecord is one job-record file stamped for a mission.
type jobRecord struct {
	path string
	doc  map[string]any
}

// missionJobs lists the job records stamped for a mission, in path order so
// every downstream decision is deterministic. Unreadable records are skipped:
// they cannot prove mission membership.
func missionJobs(root, mission string) []jobRecord {
	paths, _ := filepath.Glob(filepath.Join(jobsDirPath(root), "*.json"))
	sort.Strings(paths)
	records := []jobRecord{}
	for _, path := range paths {
		doc, err := readJSONDoc(path)
		if err != nil {
			continue
		}
		if stamped, _ := doc["mission"].(string); stamped == mission {
			records = append(records, jobRecord{path: path, doc: doc})
		}
	}
	return records
}

// ActiveJobs lists the mission's jobs that are not yet terminal — the set the
// runner must keep reaping until it drains empty. A record without a readable
// status counts as active. Jobs are identified by their recorded jobId,
// falling back to the record's file stem.
func ActiveJobs(root, mission string) []string {
	active := []string{}
	for _, record := range missionJobs(root, mission) {
		status, _ := record.doc["status"].(string)
		if TerminalJobStatuses[status] {
			continue
		}
		id, ok := record.doc["jobId"].(string)
		if !ok {
			id = strings.TrimSuffix(filepath.Base(record.path), ".json")
		}
		active = append(active, id)
	}
	sort.Strings(active)
	return active
}

// CloseableChains lists the root jobs of this mission's delegation chains
// where every job in the chain is terminal and the root is not already
// closed — the chains the runner must reap once more and then close. A job
// whose parent walk hits a cycle or leaves the mission's records belongs to
// no chain and neither closes nor blocks one.
func CloseableChains(root, mission string) []string {
	byID := map[string]map[string]any{}
	for _, record := range missionJobs(root, mission) {
		if id, ok := record.doc["jobId"].(string); ok {
			byID[id] = record.doc
		}
	}
	chains := map[string][]map[string]any{}
	for _, id := range sortedKeys(byID) {
		doc := byID[id]
		current := doc
		seen := map[string]bool{}
		for current != nil && current["parentJob"] != nil {
			parent, ok := current["parentJob"].(string)
			if !ok || seen[parent] || byID[parent] == nil {
				current = nil
				break
			}
			seen[parent] = true
			current = byID[parent]
		}
		if current == nil {
			continue
		}
		if rootID, ok := current["jobId"].(string); ok {
			chains[rootID] = append(chains[rootID], doc)
		}
	}
	closeable := []string{}
	for _, rootID := range sortedKeys(chains) {
		allTerminal := true
		for _, doc := range chains[rootID] {
			status, _ := doc["status"].(string)
			if !TerminalJobStatuses[status] {
				allTerminal = false
				break
			}
		}
		if !allTerminal {
			continue
		}
		if closed, _ := byID[rootID]["chainClosed"].(bool); closed {
			continue
		}
		closeable = append(closeable, rootID)
	}
	return closeable
}
