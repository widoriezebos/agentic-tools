package diskstore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/realpath"
	"golang.org/x/sys/unix"
)

// The host's one evidence blob store (3.5, 3.12; Wido 2026-09-28): found by
// convention at $HOME/metasystem-evidence/.blobs, never by a key, shared by
// every evidence root of the host. A blob is <sha256> holding the raw bytes;
// a reference refs/<sha256>/<referrer> is written and synced before any
// recipe depends on the blob, so a blob is never the only copy of a file
// without a recipe line naming it. The distiller holds .gc.flock shared for
// each transaction; the reference check and the per-blob sweep (U5h-3) take
// it exclusively, so neither ever sees a transaction in flight (DL4D-01).

// BlobStoreName is the store's directory under the evidence parent.
const BlobStoreName = ".blobs"

// EvidenceParent is $HOME/metasystem-evidence: the parent of every default
// evidence root and of the blob store.
func EvidenceParent(home string) string { return filepath.Join(home, "metasystem-evidence") }

// BlobStoreDir is the host's blob store under the user's home directory.
func BlobStoreDir(home string) string { return filepath.Join(EvidenceParent(home), BlobStoreName) }

// Segment is the twelve hex digits of a checkout path's sha256 after
// symlink resolution: the segment name the chain mirror, the bundle move and
// the events collector share (3.12 "Checkout identity"). dispatch's
// CheckoutSegment derives from this one function.
func Segment(path string) string {
	sum := sha256.Sum256([]byte(realpath.Resolve(path)))
	return hex.EncodeToString(sum[:])[:12]
}

// BlobStore is one host blob store.
type BlobStore struct {
	Dir  string
	Sync Syncer
}

// BlobRef is the content of one reference: what recipe depends on the blob.
type BlobRef struct {
	Schema       string `json:"schema"`
	Referrer     string `json:"referrer"`
	Recipe       string `json:"recipe"`
	Installation string `json:"installation,omitempty"`
	Segment      string `json:"segment,omitempty"`
	// DanglingSince is when the reference check first found its bundle
	// gone with no tombstone.
	DanglingSince *time.Time `json:"danglingSince,omitempty"`
}

// BlobRefSchema names the reference format.
const BlobRefSchema = "metasystem.blob-ref/1"

// Path is a published blob's path.
func (b BlobStore) Path(digest string) string { return filepath.Join(b.Dir, digest) }

func (b BlobStore) refDir(digest string) string { return filepath.Join(b.Dir, "refs", digest) }

// RefPath is one reference's path.
func (b BlobStore) RefPath(digest, referrer string) string {
	return filepath.Join(b.refDir(digest), referrer)
}

func (b BlobStore) lockPath() string { return filepath.Join(b.Dir, ".gc.flock") }

// ErrBlobStoreBusy is a held .gc.flock: a reference check or sweep is in
// its step, or (for an exclusive taker) a distiller transaction is live.
var ErrBlobStoreBusy = errors.New("the blob store's .gc.flock is held")

// TryShared takes .gc.flock LOCK_SH without waiting, creating the store.
func (b BlobStore) TryShared() (release func(), err error) { return b.try(unix.LOCK_SH) }

// TryExclusive takes .gc.flock LOCK_EX without waiting.
func (b BlobStore) TryExclusive() (release func(), err error) { return b.try(unix.LOCK_EX) }

func (b BlobStore) try(how int) (func(), error) {
	if b.Dir == "" || !filepath.IsAbs(b.Dir) {
		return nil, fmt.Errorf("the blob store must be an absolute directory, got %q", b.Dir)
	}
	if err := os.MkdirAll(b.Dir, 0o755); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(b.lockPath(), os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, err
	}
	if err := unix.Flock(int(file.Fd()), how|unix.LOCK_NB); err != nil {
		_ = file.Close()
		if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
			return nil, ErrBlobStoreBusy
		}
		return nil, err
	}
	return func() { _ = unlockAndClose(file) }, nil
}

// Verify reports whether the published blob exists and hashes to its name.
// A present blob whose bytes differ is an error: nothing may depend on it.
func (b BlobStore) Verify(ctx context.Context, digest string) (bool, error) {
	got, _, err := FileDigest(ctx, b.Path(digest))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if got != digest {
		return false, fmt.Errorf("blob %s holds bytes hashing to %s", digest, got)
	}
	return true, nil
}

// stage copies source into the store as <digest>.partial-<stage>, synced,
// and verifies the copy hashes to digest.
func (b BlobStore) stage(ctx context.Context, source, digest, stage string) (string, error) {
	partial := b.Path(digest) + PartialSuffix + stage
	if err := os.MkdirAll(b.Dir, 0o755); err != nil {
		return "", err
	}
	if err := copyFileNew(ctx, source, partial, 0o444); err != nil {
		_ = os.Remove(partial)
		return "", err
	}
	got, _, err := FileDigest(ctx, partial)
	if err != nil || got != digest {
		_ = os.Remove(partial)
		if err == nil {
			err = fmt.Errorf("the staged blob hashes to %s, not %s", got, digest)
		}
		return "", err
	}
	return partial, nil
}

// publish renames a verified stage to the blob's name; an existing blob is
// verified and reused, and the stage dropped.
func (b BlobStore) publish(ctx context.Context, digest, partial string) error {
	present, err := b.Verify(ctx, digest)
	if err != nil {
		return err
	}
	if present {
		return os.Remove(partial)
	}
	if err := os.Rename(partial, b.Path(digest)); err != nil {
		return err
	}
	return b.Sync.SyncDir(b.Dir)
}

// WriteRef publishes one reference durably: the file, its directory and the
// refs directory are synced before it returns.
func (b BlobStore) WriteRef(digest string, ref BlobRef, stage string) error {
	if !validDigest(digest) || ref.Referrer == "" || filepath.Base(ref.Referrer) != ref.Referrer {
		return fmt.Errorf("a blob reference needs a digest and a plain referrer name")
	}
	ref.Schema = BlobRefSchema
	data, err := json.Marshal(ref)
	if err != nil {
		return err
	}
	path := b.RefPath(digest, ref.Referrer)
	if existing, err := os.ReadFile(path); err == nil && string(existing) == string(data)+"\n" {
		return nil
	}
	if err := b.Sync.WriteDurable(path, append(data, '\n'), stage); err != nil {
		return err
	}
	return b.Sync.SyncDir(filepath.Dir(b.refDir(digest)))
}

// Refs lists a blob's references by referrer name.
func (b BlobStore) Refs(digest string) ([]BlobRef, error) {
	entries, err := os.ReadDir(b.refDir(digest))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var refs []BlobRef
	for _, entry := range entries {
		if IsPartial(entry.Name()) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(b.refDir(digest), entry.Name()))
		if err != nil {
			return nil, err
		}
		var ref BlobRef
		if err := json.Unmarshal(data, &ref); err != nil {
			return nil, fmt.Errorf("reference %s/%s is unreadable: %w", digest, entry.Name(), err)
		}
		refs = append(refs, ref)
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i].Referrer < refs[j].Referrer })
	return refs, nil
}

// copyFileNew copies source to a destination that must not exist, in
// chunks under the context, and syncs it.
func copyFileNew(ctx context.Context, source, destination string, mode os.FileMode) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if _, err := copyChunks(ctx, out, in); err != nil {
		return errors.Join(err, out.Close())
	}
	return errors.Join(out.Sync(), out.Close())
}
