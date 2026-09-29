package diskstore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Syncer is the durability seam of the evidence path (3.5, 3.12 DL4E-05):
// every file the evidence path publishes is written, synced, renamed and
// its directory synced, and a failed sync of any of them is "not durable":
// the caller stops and repeats the step later. Dir nil is fsync(2) of the
// directory; a fixture injects a failure to witness that nothing proceeds
// on an unconfirmed publication.
type Syncer struct {
	Dir func(path string) error
}

// SyncDir syncs one directory.
func (s Syncer) SyncDir(path string) error {
	if s.Dir != nil {
		return s.Dir(path)
	}
	return syncDirectory(path)
}

func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	return errors.Join(directory.Sync(), directory.Close())
}

// PartialSuffix marks a staged file or directory of the evidence path: a
// name ending ".partial-<ulid>" is incomplete work its owner finishes or
// discards, never evidence.
const PartialSuffix = ".partial-"

// IsPartial reports whether a name is a staged, incomplete entry.
func IsPartial(name string) bool { return strings.Contains(name, PartialSuffix) }

// WriteDurable publishes data at path: written to path.partial-<stage>,
// synced, renamed into place, and the directory synced. Any failure is an
// error and says which step; a failed directory sync after the rename is
// still an error (published, not durable), and the caller repeats the step.
func (s Syncer) WriteDurable(path string, data []byte, stage string) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return err
	}
	partial := path + PartialSuffix + stage
	file, err := os.OpenFile(partial, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		_ = os.Remove(partial)
		return err
	}
	if err := errors.Join(file.Sync(), file.Close()); err != nil {
		_ = os.Remove(partial)
		return fmt.Errorf("sync %s: %w", partial, err)
	}
	if err := os.Rename(partial, path); err != nil {
		_ = os.Remove(partial)
		return err
	}
	if err := s.SyncDir(directory); err != nil {
		return fmt.Errorf("%s is published but its directory could not be synced, so it is not durable: %w", path, err)
	}
	return nil
}

// FileDigest is the sha256 of a regular file's bytes and its size, read in
// chunks under the context.
func FileDigest(ctx context.Context, path string) (string, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()
	return readerDigest(ctx, file)
}

func readerDigest(ctx context.Context, reader io.Reader) (string, int64, error) {
	hash := sha256.New()
	size, err := copyChunks(ctx, hash, reader)
	if err != nil {
		return "", size, err
	}
	return hex.EncodeToString(hash.Sum(nil)), size, nil
}

// copyChunks copies in chunks of copyChunk and stops at the context.
func copyChunks(ctx context.Context, to io.Writer, from io.Reader) (int64, error) {
	var total int64
	for {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		written, err := io.CopyN(to, from, copyChunk)
		total += written
		if errors.Is(err, io.EOF) {
			return total, nil
		}
		if err != nil {
			return total, err
		}
	}
}

// validDigest reports a lower-case hex sha256.
func validDigest(digest string) bool {
	if len(digest) != 64 {
		return false
	}
	for _, r := range digest {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f') {
			return false
		}
	}
	return true
}
