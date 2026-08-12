package dispatch

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Filesystem-path facts the dispatch decisions rest on: where a path really
// is once symlinks resolve, whether it sits inside a boundary, the record
// timestamp grammar, and the content digest that proves a mirrored file is
// the file it claims to be.

// resolvePath returns the absolute, symlink-free form of a path. A path that
// does not (yet) exist resolves through its deepest existing ancestor, so a
// destination that will be created still compares against the real directory
// it will land in.
func resolvePath(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	suffix := ""
	current := path
	for {
		if real, err := filepath.EvalSymlinks(current); err == nil {
			return filepath.Join(real, suffix)
		}
		parent := filepath.Dir(current)
		if parent == current {
			return filepath.Clean(path)
		}
		suffix = filepath.Join(filepath.Base(current), suffix)
		current = parent
	}
}

// pathWithin reports whether path sits at or below root, comparing whole
// path segments so /a/bc never counts as inside /a/b.
func pathWithin(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, "../"))
}

// parseRecordTime parses the timezone-qualified timestamps job records carry
// (whole-second or fractional, Z or offset).
func parseRecordTime(value string) (time.Time, error) {
	return time.Parse(time.RFC3339, value)
}

// sha256File streams a file through SHA-256 and returns the hex digest.
func sha256File(path string) (string, error) {
	handle, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer handle.Close()
	sum := sha256.New()
	if _, err := io.Copy(sum, handle); err != nil {
		return "", err
	}
	return hex.EncodeToString(sum.Sum(nil)), nil
}
