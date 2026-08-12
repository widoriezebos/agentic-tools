package dispatch

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// Mirror copies a terminal job's durable evidence — record, log, brief,
// round artifacts, capability snapshot — into the evidence root outside the
// repository, maintaining a per-chain manifest of content digests. Every copy
// is verified by digest after landing. A mirror whose manifest already covers
// every source byte-for-byte does nothing and reports unchanged, so repeated
// reaps of a settled chain cost only reads.
func Mirror(repoRoot, checkout, evidence, rootJob, job, resultPath string) error {
	evidenceResolved := resolvePath(evidence)
	checkoutResolved := resolvePath(checkout)
	if pathWithin(evidenceResolved, checkoutResolved) {
		return fmt.Errorf("evidence.root is inside the repository")
	}
	agents := filepath.Join(repoRoot, "artifacts", "agents")
	recordPath := filepath.Join(agents, "jobs", job+".json")
	record, err := readObject(recordPath)
	if err != nil {
		return fmt.Errorf("cannot read the job record: %v", err)
	}
	round, ok := numString(record["round"])
	if !ok {
		return fmt.Errorf("job record has no numeric round")
	}
	payload := filepath.Join(agents, rootJob)

	// Test hook: a chain marked .mirror-fail-once fails exactly one mirror
	// attempt, so recovery from an interrupted mirror stays provable.
	failOnce := filepath.Join(payload, ".mirror-fail-once")
	if _, err := os.Stat(failOnce); err == nil {
		os.Remove(failOnce)
		os.WriteFile(filepath.Join(payload, ".mirror-failed"), []byte("scripted interruption\n"), 0o644)
		return fmt.Errorf("scripted mirror interruption")
	}

	// Evidence for different checkouts of the same repository must not
	// collide: each checkout mirrors under a segment derived from its path.
	segmentSum := sha256.Sum256([]byte(checkoutResolved))
	segment := hex.EncodeToString(segmentSum[:])[:12]
	destination := filepath.Join(evidenceResolved, "agents", segment, rootJob)
	if err := os.MkdirAll(destination, 0o755); err != nil {
		return err
	}
	manifestPath := filepath.Join(destination, "manifest.json")

	type source struct {
		path     string
		relative string
	}
	sources := []source{{recordPath, filepath.Join("jobs", job+".json")}}
	log := filepath.Join(agents, "jobs", job+".log")
	if fileExists(log) {
		sources = append(sources, source{log, filepath.Join("jobs", job+".log")})
	}
	if roundValue, ok := numFloat(record["round"]); ok && roundValue == 1 && fileExists(filepath.Join(payload, "brief.md")) {
		sources = append(sources, source{filepath.Join(payload, "brief.md"), "brief.md"})
	}
	roundDir := filepath.Join(payload, "rounds", round)
	var roundFiles []string
	filepath.WalkDir(roundDir, func(path string, entry fs.DirEntry, err error) error {
		if err == nil && entry.Type().IsRegular() {
			roundFiles = append(roundFiles, path)
		}
		return nil
	})
	sort.Strings(roundFiles)
	for _, path := range roundFiles {
		relative, err := filepath.Rel(roundDir, path)
		if err != nil {
			continue
		}
		sources = append(sources, source{path, filepath.Join("rounds", round, relative)})
	}
	snapshotField, ok := record["capabilitySnapshot"].(string)
	if !ok {
		return fmt.Errorf("job record has no capability snapshot path")
	}
	snapshot := filepath.Join(repoRoot, snapshotField)
	if fileExists(snapshot) {
		sources = append(sources, source{snapshot, filepath.Join("capabilities", filepath.Base(snapshot))})
	}

	old := map[string]any{}
	manifestExisted := fileExists(manifestPath)
	if manifestExisted {
		if manifest, err := readObject(manifestPath); err == nil {
			if files, ok := manifest["files"].(map[string]any); ok {
				old = files
			}
		}
	}

	// Unchanged means every source is already mirrored byte-for-byte AND the
	// manifest's digests still match what sits at the destination. The record
	// compares by its semantic hash — the record content with its own mirror
	// field blanked — because mirroring stamps the record afterwards and that
	// stamp must not read as drift.
	unchanged := true
	for _, item := range sources {
		entry, ok := old[item.relative].(map[string]any)
		target := filepath.Join(destination, item.relative)
		if !ok || !fileExists(target) {
			unchanged = false
			break
		}
		targetDigest, err := sha256File(target)
		if err != nil || targetDigest != asString(entry["sha256"]) {
			unchanged = false
			break
		}
		if item.path == recordPath {
			semantic, err := semanticRecordHash(item.path)
			if err != nil || asString(entry["sourceStateHash"]) != semantic {
				unchanged = false
				break
			}
		} else {
			sourceDigest, err := sha256File(item.path)
			if err != nil || sourceDigest != asString(entry["sha256"]) {
				unchanged = false
				break
			}
		}
	}

	var manifestHash string
	if unchanged && manifestExisted {
		if manifestHash, err = sha256File(manifestPath); err != nil {
			return err
		}
	} else {
		files := map[string]any{}
		for name, entry := range old {
			files[name] = entry
		}
		for _, item := range sources {
			target := filepath.Join(destination, item.relative)
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			if err := copyFileAtomic(item.path, target); err != nil {
				return err
			}
			sourceDigest, err := sha256File(item.path)
			if err != nil {
				return err
			}
			targetDigest, err := sha256File(target)
			if err != nil {
				return err
			}
			if sourceDigest != targetDigest {
				return fmt.Errorf("mirror verification failed for %s", item.relative)
			}
			info, err := os.Stat(target)
			if err != nil {
				return err
			}
			entry := map[string]any{"sha256": targetDigest, "bytes": info.Size()}
			if item.path == recordPath {
				semantic, err := semanticRecordHash(item.path)
				if err != nil {
					return err
				}
				entry["sourceStateHash"] = semantic
			}
			files[item.relative] = entry
		}
		manifest := map[string]any{
			"rootJob":   rootJob,
			"files":     files,
			"updatedAt": nowISO(),
		}
		if err := writeRecord(manifestPath, manifest); err != nil {
			return err
		}
		if manifestHash, err = sha256File(manifestPath); err != nil {
			return err
		}
	}

	return writeCompactJSON(resultPath, map[string]any{
		"path":       destination,
		"manifest":   manifestHash,
		"unchanged":  unchanged && manifestExisted,
		"mirroredAt": nowISO(),
	})
}

// semanticRecordHash digests a job record with its mirror field blanked: the
// hash of what the record says, independent of when it was last mirrored.
func semanticRecordHash(path string) (string, error) {
	record, err := readObject(path)
	if err != nil {
		return "", err
	}
	record["mirror"] = nil
	sum := sha256.Sum256([]byte(jsonCompact(record) + "\n"))
	return hex.EncodeToString(sum[:]), nil
}

// copyFileAtomic lands a copy through the durable-write owner
// (go-production-grade B5): a reader at the destination never sees a
// half-copied file. Empty anchor until this caller carries the two-outcome
// contract; the owner also syncs the temp before publishing, which the old
// copy here skipped.
func copyFileAtomic(sourcePath, targetPath string) error {
	_, err := atomicfile.CopyFile(sourcePath, targetPath, "")
	return err
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
