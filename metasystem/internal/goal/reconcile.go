package goal

// Reconcile, stage one (BGS-5's second half, R8-06/R9-01/R9-02):
// the PERSISTED MATERIALIZED-EDIT-BASE and the STABLE CAPTURE. The
// engine records the commit whose goal tree it last wrote into this
// checkout; reconcile diffs the edited snapshot against THAT
// recorded base — never HEAD, never "the accepted tree" — so
// consecutive reconciles without a pull each start from the
// previously published tree, HEAD moving mid-session changes
// nothing, and another machine's concurrent edit is in neither
// snapshot. Capture reads every candidate file ONCE into one
// in-memory snapshot; mapping and publication use only snapshot
// bytes, so an editor save mid-session cannot tear the published
// transaction. The delta-to-verb mapping is stage two.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// baseRecordPath stores the materialized-edit-base, machine-local.
func baseRecordPath(repoRoot string) string {
	return filepath.Join(repoRoot, "artifacts", "agents", "goal-base", "materialized.json")
}

// BaseRecord names the commit whose goal tree this checkout's
// files were last materialized from.
type BaseRecord struct {
	Commit     string `json:"commit"`
	WrittenAt  string `json:"writtenAt"`
	RefreshDue bool   `json:"refreshDue"` // a publish is in flight or landed; the refresh has not completed
	// Publishing marks the window where the publish's OUTCOME is not
	// yet known (R2-1): Commit still names the BASE, and completing
	// "from" it would erase the hand edits — resolution goes through
	// the opid's trailer on a fresh capture instead.
	Publishing bool   `json:"publishing,omitempty"`
	Opid       string `json:"opid,omitempty"`
	// Snapshot carries the captured bytes DURABLY (F10): a refresh
	// completing after a crash distinguishes post-capture edits,
	// creations, and deletions exactly as the live session would.
	Snapshot map[string][]byte `json:"snapshot,omitempty"`
}

// ReadBase loads the record; absent is not an error (the fallback
// is HEAD's goal tree, taken by the caller).
func ReadBase(repoRoot string) (BaseRecord, bool, error) {
	data, err := os.ReadFile(baseRecordPath(repoRoot))
	if err != nil {
		if os.IsNotExist(err) {
			return BaseRecord{}, false, nil
		}
		return BaseRecord{}, false, err
	}
	var rec BaseRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return BaseRecord{}, false, fmt.Errorf("the materialized-base record is torn: %w", err)
	}
	return rec, true, nil
}

// WriteBase records the base durably (atomic rename).
func WriteBase(repoRoot string, rec BaseRecord) error {
	dir := filepath.Dir(baseRecordPath(repoRoot))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	tmp := baseRecordPath(repoRoot) + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, baseRecordPath(repoRoot))
}

// Snapshot is one stable capture of the checkout's ledger surface:
// every candidate file's bytes, read once. The index is neither
// read nor written.
type Snapshot struct {
	Files map[string][]byte // repo-relative path -> bytes at capture
}

// CaptureSnapshot reads the working tree's plans/goals/ surface —
// live, done/, and backlog.md — into one in-memory snapshot.
func CaptureSnapshot(repoRoot string) (*Snapshot, error) {
	snap := &Snapshot{Files: map[string][]byte{}}
	base := filepath.Join(repoRoot, "plans", "goals")
	walk := func(dir, prefix string) error {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
			if err != nil {
				return fmt.Errorf("capture cannot read %s: %w", entry.Name(), err)
			}
			snap.Files[prefix+entry.Name()] = data
		}
		return nil
	}
	if err := walk(base, goalsPrefix); err != nil {
		return nil, err
	}
	if err := walk(filepath.Join(base, "done"), goalsPrefix+"done/"); err != nil {
		return nil, err
	}
	return snap, nil
}

// BaseTip resolves the diffing base: the recorded materialized
// commit when one exists, HEAD's goal tree otherwise (the
// no-record fallback the design names).
func BaseTip(repoRoot string) (string, error) {
	rec, exists, err := ReadBase(repoRoot)
	if err != nil {
		return "", err
	}
	if exists {
		if rec.RefreshDue {
			return "", fmt.Errorf("a published reconcile's refresh is pending; goal reconcile --refresh-only completes it before any new session")
		}
		// The recorded commit must still resolve — the anchor ref
		// below keeps it reachable through gc.
		if _, err := gitIn(repoRoot, "cat-file", "-e", rec.Commit+"^{commit}"); err != nil {
			return "", fmt.Errorf("the materialized base %s is gone; a pull or checkout rewrites it: %w", short(rec.Commit), err)
		}
		return rec.Commit, nil
	}
	out, err := gitIn(repoRoot, "rev-parse", "--verify", "HEAD")
	if err != nil {
		return "", fmt.Errorf("no materialized base and no HEAD: %w", err)
	}
	return strings.TrimSpace(out), nil
}

// baseAnchorRef keeps the materialized base reachable through
// rewind repair and git gc (R10-M02).
const baseAnchorRef = "refs/metasystem/goals/materialized-base"

// MaintainBase advances the materialized base when the checkout's
// goal files exactly match HEAD's goal tree and the record lags —
// the ordinary pull/checkout path (F9): no hook needed, the next
// session's read does the bookkeeping.
func MaintainBase(repoRoot string) {
	rec, exists, err := ReadBase(repoRoot)
	if err != nil || (exists && rec.RefreshDue) {
		return
	}
	headOut, err := gitIn(repoRoot, "rev-parse", "--verify", "HEAD")
	if err != nil {
		return
	}
	head := strings.TrimSpace(headOut)
	if exists && rec.Commit == head {
		return
	}
	headFiles, err := ReadCommitGoals(repoRoot, head)
	if err != nil {
		return
	}
	snap, err := CaptureSnapshot(repoRoot)
	if err != nil || len(snap.Files) != len(headFiles) {
		return
	}
	for p, content := range headFiles {
		if string(snap.Files[p]) != string(content) {
			return
		}
	}
	_ = RecordMaterialized(repoRoot, head)
}

// RecordMaterialized stamps the base after a refresh or a checkout
// update of goal paths, anchoring the commit against gc.
func RecordMaterialized(repoRoot, commit string) error {
	if _, err := gitIn(repoRoot, "update-ref", baseAnchorRef, commit); err != nil {
		return err
	}
	return WriteBase(repoRoot, BaseRecord{Commit: commit, WrittenAt: nowISO8601()})
}

// SnapshotDelta is one file's difference against the base tree.
type SnapshotDelta struct {
	Path string
	Kind string // added | removed | changed
}

// DiffAgainstBase names every ledger path whose snapshot bytes
// differ from the base commit's tree.
func DiffAgainstBase(repoRoot string, baseCommit string, snap *Snapshot) ([]SnapshotDelta, error) {
	baseFiles, err := ReadCommitGoals(repoRoot, baseCommit)
	if err != nil {
		return nil, err
	}
	var deltas []SnapshotDelta
	for _, p := range sortedKeys(snap.Files) {
		baseData, existed := baseFiles[p]
		if !existed {
			deltas = append(deltas, SnapshotDelta{Path: p, Kind: "added"})
			continue
		}
		if string(baseData) != string(snap.Files[p]) {
			deltas = append(deltas, SnapshotDelta{Path: p, Kind: "changed"})
		}
	}
	for _, p := range sortedKeys(baseFiles) {
		if _, present := snap.Files[p]; !present {
			deltas = append(deltas, SnapshotDelta{Path: p, Kind: "removed"})
		}
	}
	sort.Slice(deltas, func(i, j int) bool { return deltas[i].Path < deltas[j].Path })
	return deltas, nil
}

// Refresh writes the published tree's ledger files back into the
// checkout — but a file edited AFTER capture is left untouched and
// NAMED (re-run reconcile for it): the comparison is against the
// captured snapshot, never blind overwrite. The base record is
// written before the refresh and the refresh is idempotently
// re-runnable when publication succeeded but the refresh died.
func Refresh(repoRoot, publishedCommit string, snap *Snapshot) (skipped []string, err error) {
	if err := WriteBase(repoRoot, BaseRecord{
		Commit: publishedCommit, WrittenAt: nowISO8601(), RefreshDue: true,
		Snapshot: snap.Files,
	}); err != nil {
		return nil, err
	}
	if _, err := gitIn(repoRoot, "update-ref", baseAnchorRef, publishedCommit); err != nil {
		return nil, err
	}
	published, err := ReadCommitGoals(repoRoot, publishedCommit)
	if err != nil {
		return nil, err
	}
	for _, p := range sortedKeys(published) {
		abs := filepath.Join(repoRoot, filepath.FromSlash(p))
		current, readErr := os.ReadFile(abs)
		captured, wasCaptured := snap.Files[p]
		if readErr == nil && wasCaptured && string(current) != string(captured) {
			// Edited since capture: preserved and named.
			skipped = append(skipped, p)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			return skipped, err
		}
		if err := os.WriteFile(abs, published[p], 0o644); err != nil {
			return skipped, err
		}
	}
	// Captured files the published tree no longer carries die with
	// the same edited-since-capture protection.
	for _, p := range sortedKeys(snap.Files) {
		if _, still := published[p]; still {
			continue
		}
		abs := filepath.Join(repoRoot, filepath.FromSlash(p))
		current, readErr := os.ReadFile(abs)
		if readErr == nil && string(current) != string(snap.Files[p]) {
			skipped = append(skipped, p)
			continue
		}
		_ = os.Remove(abs)
	}
	return skipped, WriteBase(repoRoot, BaseRecord{Commit: publishedCommit, WrittenAt: nowISO8601()})
}

// RefreshOnly completes a died refresh from the durably recorded
// publish (R10-M01): publication succeeded, the base record says
// refreshDue, and the snapshot protection degrades to "current
// bytes differ from the published tree are preserved and named"
// (the captured snapshot died with the process).
func RefreshOnly(repoRoot string) (skipped []string, err error) {
	rec, exists, err := ReadBase(repoRoot)
	if err != nil {
		return nil, err
	}
	if !exists || !rec.RefreshDue {
		return nil, fmt.Errorf("no refresh is pending; ordinary reconcile owns the next session")
	}
	if rec.Snapshot == nil {
		return nil, fmt.Errorf("the pending record carries no snapshot; this refresh predates the durable capture and completes by hand")
	}
	if rec.Publishing {
		// The crash fell inside the publish window (R2-1): whether
		// the commit landed is a fact about the canonical branch, and
		// the opid's trailer answers it. Landed → refresh onto the
		// tip that carries it. Never landed → the hand edits are
		// still the worktree's truth; clear the pending flag and say
		// so — completing "from" the old base would erase them.
		e, resolveErr := ResolveEndpoint(repoRoot)
		if resolveErr != nil {
			return nil, resolveErr
		}
		nonce, nonceErr := readNonce()
		if nonceErr != nil {
			return nil, nonceErr
		}
		tip, capErr := CaptureTip(e, nonce)
		CleanupRefs(e, nonce)
		if capErr != nil {
			return nil, fmt.Errorf("the crashed publish cannot be resolved offline; retry with the remote reachable: %w", capErr)
		}
		present, trErr := TrailerPresent(e, tip, rec.Opid)
		if trErr != nil {
			return nil, trErr
		}
		if !present {
			if err := WriteBase(repoRoot, BaseRecord{Commit: rec.Commit, WrittenAt: nowISO8601()}); err != nil {
				return nil, err
			}
			return nil, fmt.Errorf("the crashed reconcile never published; the hand edits are untouched in the worktree — re-run goal reconcile")
		}
		// The tip the completion materializes must pass the SAME
		// boundary the read side enforces: a
		// malformed descendant that followed the reconcile commit
		// must not become this checkout's files just because our
		// trailer sits below it.
		if valErr := ValidateCommit(repoRoot, tip); valErr != nil {
			return nil, fmt.Errorf("the reconcile published, but the canonical tip does not validate; repair the branch, then re-run --refresh-only: %w", valErr)
		}
		return Refresh(repoRoot, tip, &Snapshot{Files: rec.Snapshot})
	}
	// The DURABLE snapshot restores the live session's exact
	// protection (F10): the completion runs the same refresh the
	// crash interrupted, post-capture edits, creations, and
	// deletions all distinguished.
	return Refresh(repoRoot, rec.Commit, &Snapshot{Files: rec.Snapshot})
}

func nowISO8601() string {
	return time.Now().UTC().Format(time.RFC3339)
}
