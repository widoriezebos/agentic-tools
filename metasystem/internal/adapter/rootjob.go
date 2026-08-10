package adapter

import (
	"fmt"
	"path/filepath"
)

// RootJobID walks a job's parentJob chain to the root of its lineage: the first
// job with no parent. A job whose parentJob is null or absent is its own root.
// A chain that ever revisits a job it already stepped through is cyclic and
// cannot have a root, so the walk refuses rather than looping forever.
func RootJobID(jobsDir, job string) (string, error) {
	seen := map[string]bool{}
	for {
		if seen[job] {
			return "", fmt.Errorf("cyclic job chain")
		}
		seen[job] = true

		record, err := readObject(filepath.Join(jobsDir, job+".json"))
		if err != nil {
			return "", err
		}
		parent, present := record["parentJob"]
		if !present || parent == nil {
			return job, nil
		}
		next, ok := parent.(string)
		if !ok {
			return "", fmt.Errorf("job %s has a non-string parentJob", job)
		}
		job = next
	}
}
