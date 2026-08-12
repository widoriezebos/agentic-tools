// Package atomicfile owns atomic file replacement: writing a file's new
// content so a reader sees either the old bytes or the new ones, never a
// half-written file, and never a truncated original if the write fails.
//
// It exists as one owner rather than a copy per package because the
// guarantee it makes is a DURABILITY contract — go-production-grade's B5
// and B6 harden exactly this operation, and two copies would be two fixes
// that can silently diverge. The current guarantee is stated honestly here:
// the temp file is synced before the rename, and the directory sync that
// makes the rename itself durable is attempted but its error is discarded.
// Phase 4 of that plan replaces the discard with the decided outcome model;
// until it lands, callers may not claim crash-durability of the rename.
package atomicfile

import (
	"os"
	"path/filepath"
)

// WriteText replaces path's content with text. The write lands through a
// temporary file in the same directory (so the rename is atomic on one
// filesystem) which is synced before the rename; a failure before the
// rename leaves the original untouched.
func WriteText(path, text string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.WriteString(text); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	// The directory sync is what makes the RENAME durable; its error is
	// currently discarded (go-production-grade B5, fixed in Phase 4).
	if dir, err := os.Open(filepath.Dir(path)); err == nil {
		_ = dir.Sync()
		_ = dir.Close()
	}
	return nil
}
