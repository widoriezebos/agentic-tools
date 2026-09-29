package diskstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
)

// The blob reference check and the per-blob sweep (3.12 "The blob store";
// DL4C-04, DL4D-01, DL4E-04): both run under .blobs/.gc.flock held
// exclusively, taken before any recipe is read, so neither ever sees a
// distiller transaction in flight (the distiller holds it shared). One
// reference at a time:
//   - its recipe exists and names the digest: kept;
//   - its bundle has a removal tombstone whose receipt is committed in the
//     segment's disposals ledger: dropped (the disposer normally dropped it
//     already), with a receipt line;
//   - its bundle has a begun removal tombstone without its receipt (the
//     bundle may be set aside as .disposing-*): kept, unconditionally;
//   - its recipe exists and no longer names the digest: dropped with its
//     stages (an interrupted transaction the distiller redoes);
//   - its bundle is gone with no tombstone, in an armed checkout: dangling,
//     reported from first sight and dropped only after the age floor;
//     elsewhere kept and reported.
// A blob whose refs directory has been empty longer than the grace is
// removed, one blob per step; a stage no reference names is removed.

// BlobReceipt is one line of .blobs/disposals.jsonl.
type BlobReceipt struct {
	Schema   int       `json:"schema"`
	At       time.Time `json:"at"`
	Step     string    `json:"step"`
	Blob     string    `json:"blob"`
	Referrer string    `json:"referrer,omitempty"`
	Reason   string    `json:"reason"`
	By       string    `json:"by"`
}

// BlobCheck is one reference check and sweep.
type BlobCheck struct {
	Blobs BlobStore
	Now   time.Time
	// Grace is evidence.blob-grace-hours; AgeFloor evidence.age-floor-days
	// (how long a dangling reference is reported before it goes).
	Grace, AgeFloor time.Duration
	// Armed reports whether an installation is an armed, readable checkout.
	Armed func(installation string) bool
	By    string
}

// BlobCheckResult says what a check did.
type BlobCheckResult struct {
	Kept       int      `json:"kept"`
	Dropped    []string `json:"dropped,omitempty"`
	Dangling   []string `json:"dangling,omitempty"`
	Removed    []string `json:"removed,omitempty"`
	Pending    string   `json:"pending,omitempty"`
	FreedBytes int64    `json:"freedBytes,omitempty"`
	// Unreadable are blobs whose references could not be read: kept.
	Unreadable []string `json:"unreadable,omitempty"`
}

// ReferenceState is how one reference stands.
type referenceState int

const (
	refKeep referenceState = iota
	refDrop
	refDangling
	// refHold keeps the reference and reports why.
	refHold
	// refUnreachable holds the whole check: the evidence root is not
	// reachable.
	refUnreachable
)

// Run checks every reference and sweeps every unreferenced blob, stopping
// at the context; a held lock is pending.
func (c BlobCheck) Run(ctx context.Context) BlobCheckResult {
	var result BlobCheckResult
	if _, err := os.Stat(c.Blobs.Dir); errors.Is(err, os.ErrNotExist) {
		return result
	}
	release, err := c.Blobs.TryExclusive()
	if err != nil {
		result.Pending = "the blob store is busy (" + err.Error() + "); the check waits for the next pass"
		return result
	}
	defer release()
	digests, err := c.referencedDigests()
	if err != nil {
		result.Pending = "the references cannot be listed: " + err.Error()
		return result
	}
	for _, digest := range digests {
		if ctx.Err() != nil {
			result.Pending = "the pass budget ran out during the reference check"
			return result
		}
		c.checkDigest(digest, &result)
	}
	if result.Pending != "" {
		// A check that could not judge every reference sweeps nothing:
		// a blob it could not see the references of may still be needed.
		return result
	}
	c.sweep(ctx, &result)
	return result
}

func (c BlobCheck) referencedDigests() ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(c.Blobs.Dir, "refs"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var digests []string
	for _, entry := range entries {
		if entry.IsDir() && validDigest(entry.Name()) {
			digests = append(digests, entry.Name())
		}
	}
	return digests, nil
}

func (c BlobCheck) checkDigest(digest string, result *BlobCheckResult) {
	refs, err := c.Blobs.Refs(digest)
	if err != nil {
		result.Unreadable = append(result.Unreadable, digest[:12]+": "+err.Error())
		result.Pending = "the references of blob " + digest[:12] + " cannot be read (" + err.Error() + "); every blob is kept and nothing is swept this pass"
		return
	}
	for _, ref := range refs {
		state, reason := c.judge(digest, ref)
		switch state {
		case refHold:
			result.Kept++
			result.Unreadable = append(result.Unreadable, reason)
		case refUnreachable:
			result.Kept++
			result.Pending = reason + "; the check holds and nothing is swept this pass"
		case refKeep:
			result.Kept++
			if ref.DanglingSince != nil {
				ref.DanglingSince = nil
				_ = c.Blobs.WriteRef(digest, ref, fmt.Sprintf("check-%d", c.Now.UnixNano()))
			}
		case refDangling:
			// Reported, never dropped: only a committed removal drops a
			// reference (Round B2-2, R2).
			if ref.DanglingSince == nil {
				since := c.Now.UTC()
				ref.DanglingSince = &since
				_ = c.Blobs.WriteRef(digest, ref, fmt.Sprintf("check-%d", c.Now.UnixNano()))
			}
			result.Dangling = append(result.Dangling, fmt.Sprintf("%s by %s since %s: %s; a person decides", digest[:12], ref.Referrer, ref.DanglingSince.Format("2006-01-02"), reason))
			result.Kept++
		case refDrop:
			c.drop(digest, ref.Referrer, reason, result)
		}
	}
}

// judge decides one reference (Round B2-2, R2): a read error never
// concludes "bundle gone". A bundle directory that exists keeps its
// references whatever its DISTILLED.txt says; an unreadable one is held and
// reported. A segment directory that cannot be read (an unmounted or
// unreadable evidence root) holds the whole check. Only a bundle proven
// absent whose removal a matching tombstone and receipt commit drops them;
// one gone without a committed removal is reported as dangling and kept.
func (c BlobCheck) judge(digest string, ref BlobRef) (referenceState, string) {
	bundle := filepath.Dir(ref.Recipe)
	info, err := os.Lstat(bundle)
	switch {
	case err == nil && info.IsDir():
		_, _, present, readErr := ReadDistilled(bundle)
		if readErr != nil || !present {
			reason := "its DISTILLED.txt is absent"
			if readErr != nil {
				reason = "its DISTILLED.txt cannot be read: " + readErr.Error()
			}
			return refHold, bundle + ": " + reason + "; its references are kept for a person"
		}
		return refKeep, ""
	case err == nil:
		return refHold, bundle + " is not a directory; its references are kept for a person"
	case !errors.Is(err, os.ErrNotExist):
		return refUnreachable, bundle + " cannot be examined (" + err.Error() + ")"
	}
	segmentDir := filepath.Dir(bundle)
	if entries, err := os.ReadDir(segmentDir); err != nil || len(entries) == 0 && !segmentHasRecords(segmentDir) {
		reason := "is empty"
		if err != nil {
			reason = "cannot be read (" + err.Error() + ")"
		}
		return refUnreachable, "the bundle's segment " + segmentDir + " " + reason + ": its evidence root may be unmounted"
	}
	tombstone, tombErr := ReadTombstone(RemovedTombstonePath(bundle))
	if tombErr == nil {
		ledger := filepath.Join(filepath.Dir(filepath.Dir(filepath.Dir(bundle))), "disposals", tombstone.Segment+".jsonl")
		committed, err := ReceiptCommitted(ledger, tombstone.Receipt, tombstone.Item)
		switch {
		case err != nil:
			return refHold, "the disposals ledger " + ledger + " cannot be read: " + err.Error()
		case committed && tombstone.Item == filepath.Base(bundle):
			return refDrop, "its bundle's removal is committed (receipt " + tombstone.Receipt + ")"
		default:
			return refKeep, "" // an uncommitted removal may roll back
		}
	}
	if !errors.Is(tombErr, os.ErrNotExist) {
		return refHold, "its bundle's tombstone cannot be read: " + tombErr.Error()
	}
	return refDangling, "its bundle " + bundle + " is gone and left no tombstone"
}

// segmentHasRecords reports a segment directory holding only tombstones
// or sidecars (every bundle removed): it is reachable.
func segmentHasRecords(directory string) bool {
	entries, err := os.ReadDir(directory)
	return err == nil && len(entries) > 0
}

func (c BlobCheck) drop(digest, referrer, reason string, result *BlobCheckResult) {
	if err := os.Remove(c.Blobs.RefPath(digest, referrer)); err != nil && !errors.Is(err, os.ErrNotExist) {
		result.Pending = err.Error()
		return
	}
	// The refs directory's mtime is when its last reference went: the
	// sweep's grace counts from it, on the pass's clock.
	_ = os.Chtimes(c.Blobs.refDir(digest), c.Now, c.Now)
	_ = c.Blobs.Sync.SyncDir(c.Blobs.refDir(digest))
	result.Dropped = append(result.Dropped, digest[:12]+" by "+referrer)
	_ = c.receipt(BlobReceipt{Step: "drop-reference", Blob: digest, Referrer: referrer, Reason: reason})
}

func (c BlobCheck) receipt(line BlobReceipt) error {
	line.Schema, line.At, line.By = 1, c.Now.UTC(), c.By
	data, err := json.Marshal(line)
	if err != nil {
		return err
	}
	path := filepath.Join(c.Blobs.Dir, "disposals.jsonl")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := file.Write(append(data, '\n')); err != nil {
		_ = file.Close()
		return err
	}
	return errors.Join(file.Sync(), file.Close())
}

// sweep removes each blob whose references have been gone longer than the
// grace (the refs directory's own mtime, set by the last drop), and each
// stage no reference names.
func (c BlobCheck) sweep(ctx context.Context, result *BlobCheckResult) {
	entries, err := os.ReadDir(c.Blobs.Dir)
	if err != nil {
		return
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		if ctx.Err() != nil {
			result.Pending = "the pass budget ran out during the blob sweep"
			return
		}
		name := entry.Name()
		digest, _, staged := strings.Cut(name, PartialSuffix)
		if !validDigest(digest) || entry.IsDir() {
			continue
		}
		refDir := c.Blobs.refDir(digest)
		refs, err := c.Blobs.Refs(digest)
		if err != nil {
			// Fail closed: references that cannot be read keep the blob.
			result.Unreadable = append(result.Unreadable, digest[:12]+": "+err.Error())
			continue
		}
		if len(refs) > 0 {
			continue
		}
		if staged {
			// A stage with no reference is incomplete work.
			if os.Remove(filepath.Join(c.Blobs.Dir, name)) == nil {
				result.Removed = append(result.Removed, name)
			}
			continue
		}
		emptySince := time.Time{}
		if info, err := os.Stat(refDir); err == nil {
			emptySince = info.ModTime()
		} else if info, err := entry.Info(); err == nil {
			emptySince = info.ModTime()
		}
		if c.Now.Sub(emptySince) < c.Grace {
			continue
		}
		bytes, _, _ := Measure(ctx, filepath.Join(c.Blobs.Dir, name))
		if err := os.Remove(filepath.Join(c.Blobs.Dir, name)); err != nil {
			continue
		}
		_ = os.Remove(refDir)
		result.Removed = append(result.Removed, digest[:12])
		result.FreedBytes += bytes
		_ = c.receipt(BlobReceipt{Step: "remove-blob", Blob: digest, Reason: "no reference for longer than " + config.DiskEvidenceBlobGraceKey})
	}
}
