package validate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// CodeCritiqueClaim verifies a receipt's code-critique claim: among the
// delegate entries (runtime:model:job-id triples) there must be a
// top-level code-critic chain whose reviews field names one of the
// implementer delegate jobs. Unreadable or mismatched job records are
// skipped, never trusted.
func CodeCritiqueClaim(root string, delegates []string) bool {
	records := delegateJobRecords(root, delegates)
	implementers := map[string]bool{}
	for _, record := range records {
		if record["role"] == "implementer" {
			if id, ok := record["jobId"].(string); ok {
				implementers[id] = true
			}
		}
	}
	for _, record := range records {
		if record["role"] != "code-critic" || record["parentJob"] != nil {
			continue
		}
		if reviews, ok := record["reviews"].(string); ok && implementers[reviews] {
			return true
		}
	}
	return false
}

// WaiverFacts resolves an implementer delegate's critique-waiver facts:
// the waiver class from the first implementer job record carrying a
// critiqueWaived claim, and the mission stream read from that chain's
// root brief. Both values are "none" when no delegate carries a waiver.
func WaiverFacts(root string, delegates []string) (class, stream string) {
	jobs := filepath.Join(root, "artifacts", "agents", "jobs")
	for _, triple := range delegates {
		jobID := delegateJobID(triple)
		record, ok := readJobRecord(jobs, jobID)
		if !ok || record["jobId"] != jobID || record["role"] != "implementer" {
			continue
		}
		claim, ok := record["critiqueWaived"].(map[string]any)
		if !ok {
			continue
		}
		waiverClass, ok := claim["class"].(string)
		if !ok {
			continue
		}

		// Walk the parentJob chain to the root job; a cycle, a bad
		// link, or an unreadable parent ends the walk where it stands.
		current := record
		seen := map[string]bool{}
		for current["parentJob"] != nil {
			parent, ok := current["parentJob"].(string)
			if !ok || seen[parent] {
				break
			}
			seen[parent] = true
			next, ok := readJobRecord(jobs, parent)
			if !ok {
				break
			}
			current = next
		}
		rootJob := jobID
		if id, ok := current["jobId"].(string); ok {
			rootJob = id
		}

		stream := "standalone"
		brief := filepath.Join(root, "artifacts", "agents", rootJob, "brief.md")
		if data, err := os.ReadFile(brief); err == nil {
			for _, line := range splitLines(string(data)) {
				if !strings.HasPrefix(line, "Mission Stream:") {
					continue
				}
				_, value, _ := strings.Cut(line, ":")
				if trimmed := strings.TrimSpace(value); trimmed != "" {
					stream = trimmed
				}
				break
			}
		}
		return waiverClass, stream
	}
	return "none", "none"
}

// delegateJobID extracts the job id from a runtime:model:job-id triple:
// everything after the last colon.
func delegateJobID(triple string) string {
	if colon := strings.LastIndex(triple, ":"); colon >= 0 {
		return triple[colon+1:]
	}
	return triple
}

// delegateJobRecords reads each delegate triple's job record, keeping
// only objects whose jobId matches the file they were read from.
func delegateJobRecords(root string, delegates []string) []map[string]any {
	jobs := filepath.Join(root, "artifacts", "agents", "jobs")
	var records []map[string]any
	for _, triple := range delegates {
		jobID := delegateJobID(triple)
		record, ok := readJobRecord(jobs, jobID)
		if ok && record["jobId"] == jobID {
			records = append(records, record)
		}
	}
	return records
}

func readJobRecord(jobsDir, jobID string) (map[string]any, bool) {
	data, err := os.ReadFile(filepath.Join(jobsDir, jobID+".json"))
	if err != nil {
		return nil, false
	}
	var parsed any
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, false
	}
	record, ok := parsed.(map[string]any)
	return record, ok
}
