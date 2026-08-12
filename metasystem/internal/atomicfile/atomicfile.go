// Package atomicfile owns atomic, durable file replacement.
//
// It exists as one owner rather than a copy per package because what it
// promises is a DURABILITY contract: the guarantee has to be implemented
// once, or two copies become two fixes that silently diverge
// (go-production-grade B5).
//
// # The outcome model
//
// A write has exactly two outcomes, and the signature carries them:
//
//   - PRE-PUBLICATION FAILURE — the directory chain, the temp file, its
//     sync, or the rename failed. No new bytes are at the target and the
//     prior content is untouched. Returns (false, err); the caller fails.
//   - COMMITTED, DURABILITY UNKNOWN — the rename succeeded, so the new
//     content IS the file, but the directory sync that makes that rename
//     survive a crash failed. Returns (false, nil): committed, with doubt
//     attached. A committed transition is never reported as a failure —
//     that divergence is what this model exists to prevent — and there is
//     no retry, because after a failed fsync the kernel may clear the error
//     state and a later success would prove nothing.
//
// Success is (true, nil): committed and durable.
//
// # Why the directory chain is synced first
//
// A new directory's own entry lives in its PARENT, so syncing only the
// target directory says nothing about whether the directory itself survives
// a crash. The chain is therefore synced unconditionally from the target's
// directory up to and INCLUDING a caller-named anchor that is guaranteed to
// pre-exist, before the temp file is written. Unconditional because
// conditioning on "did this call create something" has a retry hole: a
// failed chain sync leaves the directories visible, so a retry sees them
// pre-existing, skips the chain, and could report durable over an unproven
// chain.
package atomicfile

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// syncDir syncs one directory. It is a variable so fault-injection tests can
// fail a chosen sync without touching the filesystem's real behavior.
var syncDir = func(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	if err := dir.Sync(); err != nil {
		dir.Close()
		return err
	}
	return dir.Close()
}

// WriteText replaces path's content with text.
//
// anchor names a directory guaranteed to pre-exist — the repository checkout
// for in-repo writers, or the parent of a directory the writer may itself
// create. Every directory from path's own up to and including anchor is made
// durable before publication. When anchor is empty, only the target's own
// directory is synced (the pre-B5 behavior, for callers not yet converted).
//
// See the package doc for the outcome model behind (durable, err).
func WriteText(path, text, anchor string) (durable bool, err error) {
	target := filepath.Dir(path)
	if err := os.MkdirAll(target, 0o755); err != nil {
		return false, err
	}
	// Pre-publication: make the directory chain durable. A failure here is
	// a plain error — nothing has been published.
	for _, dir := range chain(target, anchor) {
		if err := syncDir(dir); err != nil {
			return false, fmt.Errorf("atomicfile: cannot make the directory chain durable at %s: %w", dir, err)
		}
	}
	tmp, err := os.CreateTemp(target, filepath.Base(path)+".*.tmp")
	if err != nil {
		return false, err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.WriteString(text); err != nil {
		tmp.Close()
		return false, err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return false, err
	}
	if err := tmp.Close(); err != nil {
		return false, err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return false, err
	}
	// PUBLISHED. From here the transition is committed; only its durability
	// is in question, and no error may be returned for it.
	if err := syncDir(target); err != nil {
		return false, nil
	}
	return true, nil
}

// chain lists the directories to sync, from dir upward through anchor
// inclusive. An empty or unrelated anchor yields just dir, and the walk is
// bounded by the filesystem root so a bad anchor can never loop.
func chain(dir, anchor string) []string {
	if anchor == "" {
		return []string{dir}
	}
	anchorResolved := filepath.Clean(anchor)
	current := filepath.Clean(dir)
	var dirs []string
	for {
		dirs = append(dirs, current)
		if current == anchorResolved {
			return dirs
		}
		parent := filepath.Dir(current)
		if parent == current {
			// Reached the root without meeting the anchor: the anchor is
			// not an ancestor, so sync only what we walked below it.
			if !strings.HasPrefix(anchorResolved, string(filepath.Separator)) {
				return dirs
			}
			return dirs
		}
		current = parent
	}
}

// CopyFile publishes a copy of sourcePath at targetPath under the same
// outcome model as WriteText: the copy lands through a synced temp file in
// the target's directory, the chain through anchor is made durable before
// publication, and a post-publication directory-sync failure is committed
// with doubt — (false, nil) — never an error.
func CopyFile(sourcePath, targetPath, anchor string) (durable bool, err error) {
	source, err := os.Open(sourcePath)
	if err != nil {
		return false, err
	}
	defer source.Close()
	target := filepath.Dir(targetPath)
	if err := os.MkdirAll(target, 0o755); err != nil {
		return false, err
	}
	for _, dir := range chain(target, anchor) {
		if err := syncDir(dir); err != nil {
			return false, fmt.Errorf("atomicfile: cannot make the directory chain durable at %s: %w", dir, err)
		}
	}
	temp, err := os.CreateTemp(target, filepath.Base(targetPath)+".*.tmp")
	if err != nil {
		return false, err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if _, err := io.Copy(temp, source); err != nil {
		temp.Close()
		return false, err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return false, err
	}
	if err := temp.Close(); err != nil {
		return false, err
	}
	if err := os.Rename(tempName, targetPath); err != nil {
		return false, err
	}
	if err := syncDir(target); err != nil {
		return false, nil
	}
	return true, nil
}

// WriteVolatile replaces path's content atomically WITHOUT any durability
// barrier: readers see old bytes or new bytes, never a torn file, and
// nothing is fsynced. This is for EPHEMERAL liveness signals — heartbeats,
// freshness stamps — that are rewritten every interval and carry no value
// across a crash: a rebooted machine re-arms and re-stamps. Routing those
// through WriteText's barriers taxes the hottest write path with
// full-flushes (F_FULLFSYNC on darwin) for durability nobody reads, which
// destabilized the suite's timing-scaled fixtures when it was tried
// (go-production-grade B5, the recorded classification).
func WriteVolatile(path, text string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.WriteString(text); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
