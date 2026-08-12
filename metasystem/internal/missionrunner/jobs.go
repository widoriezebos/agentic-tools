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
	// A record normally names its mission, but a pending-setup husk dies
	// before the stamp is written — while the mission's fence reservation
	// already names the job and holds its concurrency slot. The reservation
	// keys are therefore part of this mission's job set: without them a
	// crashed setup is invisible to the drain and the slot leaks forever.
	reserved := reservedJobIDs(root, mission)
	paths, _ := filepath.Glob(filepath.Join(jobsDirPath(root), "*.json"))
	sort.Strings(paths)
	records := []jobRecord{}
	for _, path := range paths {
		doc, err := readJSONDoc(path)
		if err != nil {
			continue
		}
		stamped, _ := doc["mission"].(string)
		id, _ := doc["jobId"].(string)
		if stamped == mission || (stamped == "" && reserved[id]) {
			records = append(records, jobRecord{path: path, doc: doc})
		}
	}
	return records
}

// reservedJobIDs reads the mission's fence reservations, whose keys name
// every job the mission has ever reserved a slot for.
func reservedJobIDs(root, mission string) map[string]bool {
	ids := map[string]bool{}
	doc, err := readJSONDoc(filepath.Join(root, "artifacts", "agents", "missions", mission, "fences.json"))
	if err != nil {
		return ids
	}
	reservations, _ := doc["reservations"].(map[string]any)
	for job := range reservations {
		ids[job] = true
	}
	return ids
}

// activeJobRecords lists the mission's non-terminal job records — the live
// set the drain reaps, times its deadline over, and names as survivors when
// it stalls. A record without a readable status counts as active.
func activeJobRecords(root, mission string) []jobRecord {
	records := []jobRecord{}
	for _, record := range missionJobs(root, mission) {
		status, _ := record.doc["status"].(string)
		if TerminalJobStatuses[status] {
			continue
		}
		records = append(records, record)
	}
	return records
}

// jobRecordID identifies a job record: its recorded jobId, falling back to
// the record's file stem.
func jobRecordID(record jobRecord) string {
	if id, ok := record.doc["jobId"].(string); ok && id != "" {
		return id
	}
	return strings.TrimSuffix(filepath.Base(record.path), ".json")
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
