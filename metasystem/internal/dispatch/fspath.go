package dispatch

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"time"
)

// Filesystem-path facts the dispatch decisions rest on: where a path really
// is once symlinks resolve, whether it sits inside a boundary, the record
// timestamp grammar, and the content digest that proves a mirrored file is
// the file it claims to be.

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
