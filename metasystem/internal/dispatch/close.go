package dispatch

import (
	"fmt"
	"path/filepath"
)

// CloseCheck decides whether a chain may be closed: every record terminal,
// the root mirrored, the durable manifest covering every job record, and —
// because the diff is the deliverable — every implementer round's diff.patch
// mirrored at its current content. Closing a chain asserts its evidence is
// durable; this is the proof.
func CloseCheck(repoRoot, root string) error {
	jobsDir := filepath.Join(repoRoot, "artifacts", "agents", "jobs")
	members, err := chainMembers(jobsDir, root)
	if err != nil {
		return err
	}
	if len(members) == 0 {
		return fmt.Errorf("cannot close a chain with a non-terminal record")
	}
	for _, member := range members {
		if !terminalStatuses[asString(member.record["status"])] {
			return fmt.Errorf("cannot close a chain with a non-terminal record")
		}
	}
	rootRecord, err := readObject(filepath.Join(jobsDir, root+".json"))
	if err != nil {
		return fmt.Errorf("cannot close an unmirrored chain")
	}
	mirror, ok := rootRecord["mirror"].(map[string]any)
	if !ok {
		return fmt.Errorf("cannot close an unmirrored chain")
	}
	manifest, err := readObject(filepath.Join(asString(mirror["path"]), "manifest.json"))
	if err != nil {
		return fmt.Errorf("cannot close a chain without its durable manifest")
	}
	files, _ := manifest["files"].(map[string]any)
	for _, member := range members {
		job := asString(member.record["jobId"])
		round, ok := numString(member.record["round"])
		if !ok {
			return fmt.Errorf("manifest does not cover job record %s", job)
		}
		if _, present := files["jobs/"+job+".json"]; !present {
			return fmt.Errorf("manifest does not cover job record %s", job)
		}
		if asString(member.record["role"]) != "implementer" {
			continue
		}
		relative := "rounds/" + round + "/diff.patch"
		source := filepath.Join(repoRoot, "artifacts", "agents", root, "rounds", round, "diff.patch")
		entry, entryOK := files[relative].(map[string]any)
		if !entryOK || !fileExists(source) {
			return fmt.Errorf("implementer diff.patch is not mirrored for %s", job)
		}
		digest, err := sha256File(source)
		if err != nil || asString(entry["sha256"]) != digest {
			return fmt.Errorf("manifest has a stale implementer diff.patch for %s", job)
		}
	}
	return nil
}
