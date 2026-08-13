// Package evidence implements the durable-evidence collector: the second half
// of the standing rule that raw run evidence under gitignored artifacts/ is
// mirrored to the durable evidence root before it counts as disposable — and
// disposable evidence eventually gets disposed of. artifacts/ holds LIVE
// state; history belongs to the evidence root, outside the repository.
//
// Safety: a chain is collected only when every job of it is terminal, the
// chain is explicitly closed, and every file under its payload directory is
// accounted for by the mirror's manifest. Anything else is refused loudly,
// never skipped silently.
package evidence

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
)

// now is the wall clock, overridable in tests so grace windows and archive
// ages are deterministic.
var now = time.Now

// terminalStatuses are the job states that mean a round can never run again.
var terminalStatuses = map[string]bool{
	"completed": true,
	"failed":    true,
	"timeout":   true,
	"cancelled": true,
}

// reservedDirs are the artifacts/agents entries that are infrastructure, not
// chain payloads, and are never candidates for collection.
var reservedDirs = map[string]bool{
	"jobs":         true,
	"worktrees":    true,
	"record-locks": true,
	"supervision":  true,
	"mains":        true,
	"capabilities": true,
	"missions":     true,
}

// spineDirs stay even when empty: the census and watcher read these every
// interval, and pruning an empty jobs/ once silenced the census entirely (no
// directory, no verdict, no arming). Only per-job ephemera collapse.
var spineDirs = map[string]bool{
	"jobs":         true,
	"capabilities": true,
	"mains":        true,
	"record-locks": true,
	"supervision":  true,
}

var (
	roundSuffixRe   = regexp.MustCompile(`-r[0-9]+$`)
	archiveStampRe  = regexp.MustCompile(`^events-(\d{8}T\d{6}Z)`)
	tempLockMaxAge  = time.Hour
	archiveKeepDays = 14
)

// manifestTimeLayout is the format the mirror stamps updatedAt with.
const manifestTimeLayout = "2006-01-02T15:04:05Z"

type keptChain struct {
	chain  string
	reason string
}

// GC runs one collection pass over a checkout's agent artifacts: collect
// closed terminal chains verified against the mirror manifest, prune terminal
// job records whose mirror is past the grace window, sweep heartbeat, lock,
// temp, and superseded-snapshot residue, prune empty non-spine directories,
// and copy-then-age flight-recorder archives. It refuses to run without an
// absolute evidence root: with nowhere durable to check against, nothing here
// is safe to delete.
func GC(checkoutRoot, evidenceRoot string, graceSeconds float64, out io.Writer) error {
	if !filepath.IsAbs(evidenceRoot) {
		return fmt.Errorf("refusing to collect: the durable evidence root must be an absolute path, got %q", evidenceRoot)
	}
	agents := filepath.Join(checkoutRoot, "artifacts", "agents")
	jobsDir := filepath.Join(agents, "jobs")

	collected, kept, err := collectChains(checkoutRoot, agents, jobsDir, evidenceRoot)
	if err != nil {
		return err
	}
	if err := pruneMirroredRecords(checkoutRoot, jobsDir, evidenceRoot, graceSeconds); err != nil {
		return err
	}
	residue, err := sweepResidue(agents, jobsDir)
	if err != nil {
		return err
	}
	residue += pruneEmptyDirs(agents)
	if err := collectEventArchives(checkoutRoot, evidenceRoot, out); err != nil {
		return err
	}

	for _, chain := range collected {
		fmt.Fprintf(out, "collected %s\n", chain)
	}
	fmt.Fprintf(out, "residue removed: %d heartbeat, lock, temp and superseded-snapshot entries\n", residue)
	for _, keep := range kept {
		fmt.Fprintf(out, "kept      %s: %s\n", keep.chain, keep.reason)
	}
	fmt.Fprintf(out, "evidence-gc: %d collected, %d kept\n", len(collected), len(kept))
	return nil
}

// collectChains removes every chain payload whose history is already durable,
// and reports why each remaining chain stays. Job records (jobs/*.json)
// always stay here: they are the registry.
func collectChains(checkoutRoot, agents, jobsDir, evidenceRoot string) ([]string, []keptChain, error) {
	entries, err := os.ReadDir(agents)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	var collected []string
	var kept []keptChain
	for _, entry := range entries {
		if !entry.IsDir() || reservedDirs[entry.Name()] {
			continue
		}
		chain := entry.Name()
		chainDir := filepath.Join(agents, chain)
		records, err := chainRecords(jobsDir, chain)
		if err != nil {
			return nil, nil, err
		}
		if len(records) == 0 {
			kept = append(kept, keptChain{chain, "no job records; not this tool's to judge"})
			continue
		}
		if anyLive(records) {
			kept = append(kept, keptChain{chain, "a round is still live"})
			continue
		}
		// An OPEN chain is working state even when every round is terminal:
		// the orchestrator is adjudicating between rounds, and the collector
		// once ate an active critique chain mid-conversation because this
		// check was missing. Only an explicitly closed chain is history.
		root := rootRecord(records, chain)
		if root == nil || root["chainClosed"] != true {
			kept = append(kept, keptChain{chain, "chain not closed; working state"})
			continue
		}
		manifestPath := chainManifestPath(evidenceRoot, checkoutRoot, chain, mirroredPath(root))
		if !fileExists(manifestPath) {
			kept = append(kept, keptChain{chain, "no mirror manifest"})
			continue
		}
		files, err := manifestFiles(manifestPath)
		if err != nil {
			return nil, nil, err
		}
		unaccounted, err := unaccountedFiles(chainDir, files)
		if err != nil {
			return nil, nil, err
		}
		if len(unaccounted) > 0 {
			sample := unaccounted
			if len(sample) > 3 {
				sample = sample[:3]
			}
			kept = append(kept, keptChain{chain, "mirror does not account for: " + strings.Join(sample, ", ")})
			continue
		}
		if err := os.RemoveAll(chainDir); err != nil {
			return nil, nil, err
		}
		if err := removeMirroredLogs(jobsDir, chain, files); err != nil {
			return nil, nil, err
		}
		collected = append(collected, chain)
	}
	return collected, kept, nil
}

// chainRecords parses every job record belonging to a chain: the root record
// and its -rN rounds, nothing that merely shares a prefix.
func chainRecords(jobsDir, chain string) ([]map[string]any, error) {
	member := regexp.MustCompile("^" + regexp.QuoteMeta(chain) + `(-r[0-9]+)?$`)
	matches, _ := filepath.Glob(filepath.Join(jobsDir, chain+"*.json"))
	var records []map[string]any
	for _, match := range matches {
		stem := strings.TrimSuffix(filepath.Base(match), ".json")
		if !member.MatchString(stem) {
			continue
		}
		record, err := readJSONObject(match)
		if err != nil {
			return nil, fmt.Errorf("cannot read job record %s: %w", match, err)
		}
		records = append(records, record)
	}
	return records, nil
}

func anyLive(records []map[string]any) bool {
	for _, record := range records {
		status, _ := record["status"].(string)
		if !terminalStatuses[status] {
			return true
		}
	}
	return false
}

func rootRecord(records []map[string]any, chain string) map[string]any {
	for _, record := range records {
		if record["jobId"] == chain {
			return record
		}
	}
	return nil
}

// mirroredPath is the directory the root record says its evidence was
// mirrored to, or empty when the record carries no mirror stamp.
func mirroredPath(record map[string]any) string {
	mirror, ok := record["mirror"].(map[string]any)
	if !ok {
		return ""
	}
	path, _ := mirror["path"].(string)
	return path
}

// chainManifestPath locates a chain's mirror manifest. The record's own
// mirror stamp wins when it names one of the known layouts; otherwise the
// first existing candidate is used, and when none exists the primary
// candidate stands in so the caller reports a missing manifest.
func chainManifestPath(evidenceRoot, checkoutRoot, chain, mirrored string) string {
	candidates := manifestCandidates(evidenceRoot, checkoutRoot, chain)
	if mirrored != "" {
		for _, candidate := range candidates {
			if filepath.Clean(filepath.Dir(candidate)) == filepath.Clean(mirrored) {
				return candidate
			}
		}
	}
	for _, candidate := range candidates {
		if fileExists(candidate) {
			return candidate
		}
	}
	return candidates[0]
}

// manifestCandidates lists where THIS checkout's chain manifest can live:
// under the checkout's own segment (the layout mirror.go writes), or
// directly under the evidence agents root (the legacy unsegmented layout).
// Never another checkout's segment: mirror.go segments precisely because
// two checkouts' chains may collide under a shared evidence root, and a
// glob across segments let checkout B's GC read checkout A's manifest and
// delete B's only records (review foundations-1).
func manifestCandidates(evidenceRoot, checkoutRoot, chain string) []string {
	segment := dispatch.CheckoutSegment(checkoutRoot)
	return []string{
		filepath.Join(evidenceRoot, "agents", segment, chain, "manifest.json"),
		filepath.Join(evidenceRoot, "agents", chain, "manifest.json"),
	}
}

// manifestFiles reads the manifest's files map: relative path to entry.
func manifestFiles(manifestPath string) (map[string]any, error) {
	manifest, err := readJSONObject(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read mirror manifest %s: %w", manifestPath, err)
	}
	files, ok := manifest["files"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("mirror manifest %s has no files map", manifestPath)
	}
	return files, nil
}

// unaccountedFiles lists payload files the manifest does not account for. A
// file is accounted when the manifest lists it and its content digest
// matches; entries under jobs/ are accounted by presence alone, because job
// records keep changing after mirroring (the mirror stamp itself lands in the
// record). Anything the manifest cannot vouch for keeps the whole chain.
func unaccountedFiles(chainDir string, files map[string]any) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(chainDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type().IsRegular() || isSymlinkToFile(path, entry) {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	var unaccounted []string
	for _, path := range paths {
		relative, err := filepath.Rel(chainDir, path)
		if err != nil {
			return nil, err
		}
		relative = filepath.ToSlash(relative)
		entry, ok := files[relative].(map[string]any)
		if !ok {
			unaccounted = append(unaccounted, relative)
			continue
		}
		if strings.HasPrefix(relative, "jobs/") {
			continue
		}
		digest, err := sha256File(path)
		if err != nil {
			return nil, err
		}
		if sha, _ := entry["sha256"].(string); sha != digest {
			unaccounted = append(unaccounted, relative)
		}
	}
	return unaccounted, nil
}

func isSymlinkToFile(path string, entry fs.DirEntry) bool {
	if entry.Type()&fs.ModeSymlink == 0 {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

// removeMirroredLogs deletes a collected chain's job logs, but only those the
// manifest lists byte-for-byte: a log the mirror cannot vouch for stays.
func removeMirroredLogs(jobsDir, chain string, files map[string]any) error {
	matches, _ := filepath.Glob(filepath.Join(jobsDir, chain+"*.log"))
	for _, log := range matches {
		entry, ok := files["jobs/"+filepath.Base(log)].(map[string]any)
		if !ok {
			continue
		}
		digest, err := sha256File(log)
		if err != nil {
			return err
		}
		if sha, _ := entry["sha256"].(string); sha == digest {
			if err := os.Remove(log); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	}
	return nil
}

// pruneMirroredRecords removes terminal job records whose chain payload is
// already collected and whose mirror holds the record past the grace window.
// Job records are the registry while work is recent: the staleness check
// reads them for its chain window and the census joins custody through them.
// Past that window, a terminal chain's records serve only history, and
// history is the mirror, which already holds every record file.
func pruneMirroredRecords(checkoutRoot, jobsDir, evidenceRoot string, graceSeconds float64) error {
	agents := filepath.Join(checkoutRoot, "artifacts", "agents")
	matches, _ := filepath.Glob(filepath.Join(jobsDir, "*.json"))
	for _, recordPath := range matches {
		record, err := readJSONObject(recordPath)
		if err != nil {
			continue
		}
		status, _ := record["status"].(string)
		if !terminalStatuses[status] {
			continue
		}
		stem := strings.TrimSuffix(filepath.Base(recordPath), ".json")
		rootChain := roundSuffixRe.ReplaceAllString(stem, "")
		if _, err := os.Stat(filepath.Join(agents, rootChain)); err == nil {
			continue // chain payload not collected yet; records stay with it
		}
		// The record's own mirror stamp is honored here exactly as
		// collectChains honors it (review foundations-1).
		manifestPath := chainManifestPath(evidenceRoot, checkoutRoot, rootChain, mirroredPath(record))
		if !fileExists(manifestPath) {
			continue
		}
		files, err := manifestFiles(manifestPath)
		if err != nil {
			return err
		}
		entry, listed := files["jobs/"+filepath.Base(recordPath)]
		if !listed || entry == nil {
			continue
		}
		// The mirror is only history once it carries the record's CURRENT
		// state (review codex-1). chainClosed/runnerClosed are CAS-patched
		// into the root record AFTER the mirror-on-terminal pass, and a
		// post-close re-mirror can fail, leaving a stale manifest; pruning
		// on presence and age alone would then delete the only copy of the
		// closure state. Equality of the semantic hash is the mandatory
		// proof; anything less retains the record.
		entryMap, ok := entry.(map[string]any)
		if !ok {
			return fmt.Errorf("mirror manifest %s entry for %s is not an object", manifestPath, filepath.Base(recordPath))
		}
		mirroredHash, _ := entryMap["sourceStateHash"].(string)
		if mirroredHash == "" {
			continue // manifest predates the hash contract: currency unprovable, record stays
		}
		currentHash, err := dispatch.SemanticRecordHash(recordPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue // record vanished under us; nothing left to prune
			}
			return fmt.Errorf("cannot hash job record %s: %w", recordPath, err)
		}
		if currentHash != mirroredHash {
			continue // mirror is stale relative to the record; deleting would lose state
		}
		manifest, err := readJSONObject(manifestPath)
		if err != nil {
			return fmt.Errorf("cannot read mirror manifest %s: %w", manifestPath, err)
		}
		updatedAt, _ := manifest["updatedAt"].(string)
		mirroredAt, err := time.Parse(manifestTimeLayout, updatedAt)
		if err != nil {
			continue
		}
		if now().Sub(mirroredAt).Seconds() <= graceSeconds {
			continue
		}
		if err := os.Remove(recordPath); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

// sweepResidue clears the three residue classes that accumulate per terminal
// job and that nobody else cleans — heartbeats, lock files and lifecycle
// directories, and the mktemp leftovers of interrupted operations — plus
// capability snapshots superseded within their own identity.
func sweepResidue(agents, jobsDir string) (int, error) {
	residue := 0
	count, err := sweepHeartbeats(filepath.Join(agents, "hb"), jobsDir)
	if err != nil {
		return residue, err
	}
	residue += count
	count, err = sweepRecordLocks(filepath.Join(agents, "record-locks"), jobsDir)
	if err != nil {
		return residue, err
	}
	residue += count
	count, err = sweepSupersededSnapshots(filepath.Join(agents, "capabilities"))
	if err != nil {
		return residue, err
	}
	return residue + count, nil
}

func jobStatus(jobsDir, job string) string {
	record, err := readJSONObject(filepath.Join(jobsDir, job+".json"))
	if err != nil {
		return ""
	}
	status, _ := record["status"].(string)
	return status
}

func sweepHeartbeats(hbDir, jobsDir string) (int, error) {
	entries, err := os.ReadDir(hbDir)
	if err != nil {
		return 0, nil // no heartbeat directory, nothing to sweep
	}
	removed := 0
	for _, entry := range entries {
		job := strings.TrimSuffix(strings.TrimSuffix(entry.Name(), ".start"), ".waiting")
		if !terminalStatuses[jobStatus(jobsDir, job)] {
			continue
		}
		if err := os.Remove(filepath.Join(hbDir, entry.Name())); err != nil && !os.IsNotExist(err) {
			return removed, err
		}
		removed++
	}
	return removed, nil
}

func sweepRecordLocks(locksDir, jobsDir string) (int, error) {
	entries, err := os.ReadDir(locksDir)
	if err != nil {
		return 0, nil // no lock directory, nothing to sweep
	}
	removed := 0
	for _, entry := range entries {
		name := entry.Name()
		path := filepath.Join(locksDir, name)
		if strings.HasSuffix(name, ".lock") || strings.HasSuffix(name, ".lifecycle.d") {
			job := strings.TrimSuffix(strings.TrimSuffix(name, ".lock"), ".lifecycle.d")
			if !terminalStatuses[jobStatus(jobsDir, job)] {
				continue
			}
			// Two collectors can run at once (the stop hook fires one per turn
			// end); a peer having removed an entry first is success, not an
			// error.
			var removeErr error
			if entry.IsDir() {
				removeErr = os.RemoveAll(path)
			} else {
				removeErr = os.Remove(path)
			}
			if removeErr != nil {
				if os.IsNotExist(removeErr) {
					continue
				}
				return removed, removeErr
			}
			removed++
			continue
		}
		if !entry.Type().IsRegular() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if now().Sub(info.ModTime()) <= tempLockMaxAge {
			continue
		}
		if err := os.Remove(path); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return removed, err
		}
		removed++
	}
	return removed, nil
}

// sweepSupersededSnapshots keeps only the newest capability snapshot per
// identity. Newest means BY CAPTURE, not by filename: the config hash
// precedes the date in the name, so a lexicographic sort once deleted the
// current snapshot and kept a stale one, and the next dispatch refused. The
// identity is runtime+version+CONFIG HASH, which is what dispatch matches on:
// grouping by runtime+version alone treated snapshots for different configs
// as superseding each other, so a config change silently deleted the very
// snapshot the next dispatch needed and every dispatch then refused.
func sweepSupersededSnapshots(capsDir string) (int, error) {
	matches, _ := filepath.Glob(filepath.Join(capsDir, "*.json"))
	groups := map[string][]string{}
	var identities []string
	for _, snapshot := range matches {
		identity := rsplitTwo(filepath.Base(snapshot))[0]
		if _, seen := groups[identity]; !seen {
			identities = append(identities, identity)
		}
		groups[identity] = append(groups[identity], snapshot)
	}
	removed := 0
	for _, identity := range identities {
		snapshots := groups[identity]
		sort.SliceStable(snapshots, func(i, j int) bool {
			di, si := captureKey(snapshots[i])
			dj, sj := captureKey(snapshots[j])
			if di != dj {
				return di < dj
			}
			return si < sj
		})
		for _, snapshot := range snapshots[:len(snapshots)-1] {
			if err := os.Remove(snapshot); err != nil && !os.IsNotExist(err) {
				return removed, err
			}
			removed++
		}
	}
	return removed, nil
}

// captureKey is the date and sequence the snapshot writer stamps at the end
// of the file name, or empty strings when the name does not carry them.
func captureKey(snapshot string) (string, string) {
	stem := strings.TrimSuffix(filepath.Base(snapshot), ".json")
	parts := rsplitTwo(stem)
	if len(parts) != 3 {
		return "", ""
	}
	return parts[1], parts[2]
}

// rsplitTwo splits at the last two hyphens, yielding up to three parts; the
// first part is whatever precedes them.
func rsplitTwo(s string) []string {
	parts := []string{s}
	for range 2 {
		i := strings.LastIndex(parts[0], "-")
		if i < 0 {
			break
		}
		parts = append([]string{parts[0][:i], parts[0][i+1:]}, parts[1:]...)
	}
	return parts
}

// pruneEmptyDirs collapses empty directories bottom-up, so nested empties go
// in one pass. Empty directories are confusion, not placeholders: every
// writer here mkdir-ps before writing, so a directory with nothing in it
// carries no information and comes back the moment it is needed. The spine
// stays even when empty, and the supervision tree is never touched.
func pruneEmptyDirs(agents string) int {
	var directories []string
	filepath.WalkDir(agents, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || path == agents || !entry.IsDir() {
			return nil
		}
		directories = append(directories, path)
		return nil
	})
	sort.Strings(directories)
	removed := 0
	for i := len(directories) - 1; i >= 0; i-- {
		directory := directories[i]
		if spineDirs[filepath.Base(directory)] || underSupervision(agents, directory) {
			continue
		}
		if err := os.Remove(directory); err == nil { // only succeeds when empty
			removed++
		}
	}
	return removed
}

func underSupervision(agents, directory string) bool {
	relative, err := filepath.Rel(agents, directory)
	if err != nil {
		return false
	}
	for _, part := range strings.Split(filepath.ToSlash(relative), "/") {
		if part == "supervision" {
			return true
		}
	}
	return false
}

// collectEventArchives ages out flight-recorder archives. Copy-then-keep is
// the norm: a local archive is deleted ONLY when BOTH hold — a verified
// durable copy exists AND the filename age is at least archiveKeepDays. Age
// comes from the filename timestamp, never mtime, and verified means
// BYTE-IDENTICAL, not same-sized: a corrupt or foreign same-name file must
// never license deleting evidence.
func collectEventArchives(checkoutRoot, evidenceRoot string, out io.Writer) error {
	archiveDir := filepath.Join(checkoutRoot, "artifacts", "agents", "events-archive")
	if info, err := os.Stat(archiveDir); err != nil || !info.IsDir() {
		return nil
	}
	// Durable events segment by the SAME checkout hash mirrors use
	// (review codex-6): a basename key let two same-name checkouts share
	// one durable directory. Legacy-ownership rule: basename directories
	// from before this change are plain history — never written again,
	// never deleted; a still-local archive whose only durable copy lives
	// in a legacy dir is simply copied once more into the segment.
	eventsRoot := filepath.Join(evidenceRoot, "events", dispatch.CheckoutSegment(checkoutRoot))
	matches, _ := filepath.Glob(filepath.Join(archiveDir, "events-*.jsonl"))
	for _, archive := range matches {
		name := filepath.Base(archive)
		stamp := archiveStampRe.FindStringSubmatch(name)
		if stamp == nil {
			continue
		}
		durable := filepath.Join(eventsRoot, name)
		copied := durableCopyVerified(archive, durable, eventsRoot)
		capturedAt, err := time.Parse("20060102T150405Z", stamp[1])
		if err != nil {
			continue
		}
		ageDays := int(now().UTC().Sub(capturedAt).Hours() / 24)
		if copied && ageDays >= archiveKeepDays {
			if err := os.Remove(archive); err != nil && !os.IsNotExist(err) {
				return err
			}
			fmt.Fprintf(out, "collected events archive %s\n", name)
		}
	}
	return nil
}

// durableCopyVerified ensures the durable copy matches the local archive
// byte-for-byte, copying (or replacing a divergent same-name file) when it
// does not, and reports whether the verified copy exists afterwards. Any
// failure keeps the local archive for a retry on the next pass.
func durableCopyVerified(archive, durable, eventsRoot string) bool {
	localDigest, err := sha256File(archive)
	if err != nil {
		return false
	}
	if digest, err := sha256File(durable); err != nil || digest != localDigest {
		if err := os.MkdirAll(eventsRoot, 0o755); err != nil {
			return false
		}
		if err := copyFilePreservingTime(archive, durable); err != nil {
			return false
		}
	}
	digest, err := sha256File(durable)
	return err == nil && digest == localDigest
}

// copyFilePreservingTime lands a copy via a temp file and rename, carrying
// the source's modification time, so a reader at the destination never sees a
// half-copied file.
func copyFilePreservingTime(sourcePath, targetPath string) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()
	info, err := source.Stat()
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(targetPath), filepath.Base(targetPath)+".*.tmp")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if _, err := io.Copy(temp, source); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	// NOT delegated to the durable-write owner (B5, recorded
	// classification): the mtime is set on the TEMP so the file publishes
	// with its preserved time atomically — a crash can never leave a
	// wrong-time file visible, which the GC's time-based reasoning needs.
	os.Chtimes(tempName, info.ModTime(), info.ModTime())
	return os.Rename(tempName, targetPath)
}

func readJSONObject(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, err
	}
	return value, nil
}

func sha256File(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}
