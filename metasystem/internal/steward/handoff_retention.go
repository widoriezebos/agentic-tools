package steward

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/output"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/usage"
)

var (
	syncHandoffDir            = syncDirectoryForHandoff
	beforeHandoffPruneInspect = func() {}
	beforeHandoffPruneRemove  = func(string) {}
	pruneCallSessions         = usage.PruneCallSessions
)

func syncDirectoryForHandoff(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	if err := directory.Sync(); err != nil {
		directory.Close()
		return err
	}
	return directory.Close()
}

func ensureCanonicalDirectory(root, path string) error {
	relative, inside := relativePathInside(root, path)
	if !inside {
		return fmt.Errorf("handoff directory is outside its state root: %s", path)
	}
	current := root
	for _, component := range splitRelativePath(relative) {
		next := filepath.Join(current, component)
		info, err := os.Lstat(next)
		if os.IsNotExist(err) {
			if err := os.Mkdir(next, 0o755); err != nil {
				return err
			}
			if err := syncHandoffDir(current); err != nil {
				return err
			}
			info, err = os.Lstat(next)
		}
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("handoff directory is redirected or nonregular: %s", next)
		}
		current = next
	}
	return syncHandoffDir(current)
}

func splitRelativePath(path string) []string {
	if path == "." || path == "" {
		return nil
	}
	var reversed []string
	for path != "." && path != "" {
		directory, base := filepath.Split(path)
		if base != "" {
			reversed = append(reversed, base)
		}
		path = filepath.Clean(directory)
	}
	parts := make([]string, len(reversed))
	for index := range reversed {
		parts[len(reversed)-1-index] = reversed[index]
	}
	return parts
}

func handoffNonceInUse(root, nonce string) (bool, error) {
	paths := []string{
		HandoffDir(root, nonce), filepath.Join(intentsDir(root), nonce+".json"),
		filepath.Join(consumedDir(root), nonce+".json"), filepath.Join(cancelledDir(root), nonce+".json"),
		BriefPath(root, nonce), filepath.Join(pendingDir(root), handoffNoticeNonce(nonce)+".json"),
	}
	for _, path := range paths {
		if _, err := os.Lstat(path); err == nil {
			return true, nil
		} else if !os.IsNotExist(err) {
			return false, err
		}
	}
	return false, nil
}

func addOwnedHandoffReference(owned, directories map[string]bool, dir string, reference HandoffReference) error {
	if reference.Status == "missing" {
		return nil
	}
	relative, inside := relativePathInside(dir, reference.OpenPath)
	if !inside || relative == "." {
		return fmt.Errorf("handoff reference is outside its immutable directory: %s", reference.OpenPath)
	}
	owned[relative] = true
	for parent := filepath.Dir(relative); parent != "."; parent = filepath.Dir(parent) {
		directories[parent] = true
	}
	return nil
}

func completeHandoffState(root, nonce string) (time.Time, error) {
	dir := HandoffDir(root, nonce)
	path := filepath.Join(dir, "state.json")
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return time.Time{}, fmt.Errorf("handoff state is not a regular file: %s", path)
	}
	if info.Size() >= int64(output.MaxInlineBytes) {
		return time.Time{}, fmt.Errorf("handoff state is too large: %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return time.Time{}, err
	}
	var state HandoffState
	if err := decodeStrictHandoffJSON(data, &state); err != nil {
		return time.Time{}, fmt.Errorf("decode handoff state %s: %w", path, err)
	}
	binding := HandoffBinding{StatePath: path, StateDigest: testableDigest(data), Runtime: state.Seat.Runtime,
		Session: state.Seat.NormalizedSession, MainId: state.Seat.MainID, Predecessor: state.Seat.Identity,
		PredecessorTag: state.Seat.Tag, PredecessorJob: state.Seat.JobID, RecordedAt: state.WrittenAt}
	if _, err := verifyBoundHandoffState(root, nonce, state.HeldGoal.ID, binding); err != nil {
		return time.Time{}, err
	}

	owned := map[string]bool{"state.json": true}
	directories := map[string]bool{".": true}
	manifest := HandoffManifest{}
	if state.Manifest != nil {
		owned["manifest.json"] = true
		manifestData, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
		if err != nil || decodeStrictHandoffJSON(manifestData, &manifest) != nil {
			return time.Time{}, fmt.Errorf("handoff manifest is unreadable: %s", filepath.Join(dir, "manifest.json"))
		}
	}
	jobs := append(append([]HandoffOpenJob(nil), state.OpenJobs...), manifest.OpenJobs...)
	for _, job := range jobs {
		if err := addOwnedHandoffReference(owned, directories, dir, job.Record); err != nil {
			return time.Time{}, err
		}
	}
	references := append(append([]HandoffReference(nil), state.NextStep.References...), state.Scratch...)
	references = append(references, manifest.Scratch...)
	for _, reference := range references {
		if err := addOwnedHandoffReference(owned, directories, dir, reference); err != nil {
			return time.Time{}, err
		}
	}
	if err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if !directories[relative] {
				return fmt.Errorf("handoff tree has unexpected directory %s", path)
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 || !entry.Type().IsRegular() {
			return fmt.Errorf("handoff tree has redirected or nonregular member %s", path)
		}
		if !owned[relative] {
			return fmt.Errorf("handoff tree has unexpected member %s", path)
		}
		return nil
	}); err != nil {
		return time.Time{}, err
	}
	return info.ModTime(), nil
}

func liveAndConsumedHandoffNonces(root string) (map[string]bool, error) {
	protected := map[string]bool{}
	live, err := LiveIntents(root)
	if err != nil {
		return nil, err
	}
	active, err := ConsumedActive(root)
	if err != nil {
		return nil, err
	}
	for _, intent := range append(live, active...) {
		protected[intent.Nonce] = true
	}
	return protected, nil
}

// PruneContext delegates usage-pair retirement to its owner, then removes
// complete inactive handoff directories under steward arbitration.
func PruneContext(stateRoot string, olderThan time.Duration, now time.Time) (ContextPruneResult, error) {
	result := ContextPruneResult{}
	if olderThan <= 0 {
		return result, fmt.Errorf("context prune older-than must be positive")
	}
	if now.IsZero() {
		return result, fmt.Errorf("context prune requires one nonzero clock observation")
	}
	root, err := handoffCanonicalRoot(stateRoot)
	if err != nil {
		return result, err
	}
	cutoff := now.Add(-olderThan)
	usageCutoff := cutoff
	usageFloor := time.Now().UTC().Add(-usage.CallRetentionWindow - time.Nanosecond)
	if usageCutoff.After(usageFloor) {
		usageCutoff = usageFloor
	}
	result.CallSessions, err = pruneCallSessions(root, usageCutoff)
	if err != nil {
		return result, err
	}
	arbitration, err := AcquireArbitration(root)
	if err != nil {
		return result, err
	}
	defer arbitration.Release()
	beforeHandoffPruneInspect()
	protected, err := liveAndConsumedHandoffNonces(root)
	if err != nil {
		return result, err
	}
	parent := filepath.Join(root, "artifacts", "agents", "context", "handoffs")
	entries, err := os.ReadDir(parent)
	if os.IsNotExist(err) {
		return result, nil
	}
	if err != nil {
		return result, err
	}
	var inspectionErrors []error
	for _, entry := range entries {
		nonce := entry.Name()
		if !handoffNoncePattern.MatchString(nonce) {
			inspectionErrors = append(inspectionErrors, fmt.Errorf("handoff directory has malformed nonce %q", nonce))
			continue
		}
		path := filepath.Join(parent, nonce)
		info, err := os.Lstat(path)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			inspectionErrors = append(inspectionErrors, fmt.Errorf("handoff path is not a regular nonsymlink directory: %s", path))
			continue
		}
		if protected[nonce] {
			continue
		}
		modTime, err := completeHandoffState(root, nonce)
		if err != nil {
			inspectionErrors = append(inspectionErrors, fmt.Errorf("keep incomplete handoff %s: %w", nonce, err))
			continue
		}
		if !modTime.Before(cutoff) {
			continue
		}
		beforeHandoffPruneRemove(path)
		checkedModTime, err := completeHandoffState(root, nonce)
		if err != nil || !checkedModTime.Equal(modTime) {
			if err == nil {
				err = fmt.Errorf("handoff state timestamp changed before removal")
			}
			return result, errors.Join(fmt.Errorf("handoff %s changed before removal: %w", nonce, err), errors.Join(inspectionErrors...))
		}
		if err := os.RemoveAll(path); err != nil {
			return result, errors.Join(err, errors.Join(inspectionErrors...))
		}
		result.Handoffs = append(result.Handoffs, path)
		sort.Strings(result.Handoffs)
		if err := syncHandoffDir(parent); err != nil {
			return result, errors.Join(err, errors.Join(inspectionErrors...))
		}
	}
	return result, errors.Join(inspectionErrors...)
}
