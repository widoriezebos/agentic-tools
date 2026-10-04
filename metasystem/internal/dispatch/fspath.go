package dispatch

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"time"
)

// SelectedInstallationEnv carries the caller's registry and settings source
// to the delegate lifecycle and its adapter processes.
const SelectedInstallationEnv = "METASYSTEM_DISPATCH_INSTALLATION"

// ResolveTool names the installation serving root, its checkout, and the
// engine to run. The installation resolver owns the worktree mapping;
// METASYSTEM_BIN explicitly overrides that installation's engine.
func ResolveTool(root string, serving func(string) (string, string), lookupEnv func(string) (string, bool)) (installation, checkout, engine string) {
	installation, checkout = serving(root)
	engine, _ = lookupEnv("METASYSTEM_BIN")
	if engine == "" {
		engine = filepath.Join(installation, "bin", "metasystem")
	}
	return
}

// InLinkedWorktree reports whether the nearest .git entry is a file.
// Only linked worktrees need Git to locate a serving installation.
func InLinkedWorktree(root string) bool {
	dir, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	for {
		if entry, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return entry.Mode().IsRegular()
		}
		if filepath.Dir(dir) == dir {
			return false
		}
		dir = filepath.Dir(dir)
	}
}

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
