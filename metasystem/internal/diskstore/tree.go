package diskstore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// afterTreeEntryRemoved is a test seam called after each unlink.
var afterTreeEntryRemoved = func(string) {}

// RemoveTree removes path entry by entry and stops at the context: every
// removal the engine makes under a lock is bounded per entry and
// cancellable (3.3, DL3B-04). A cut-short removal leaves a partly removed
// tree that the next call finishes; an absent path is success. It never
// follows a symlink: a link is removed as an entry, and a symlinked root is
// refused. RemoveTree proves nothing about ownership; callers remove only
// inside a store's critical section, after its identity checks.
func RemoveTree(ctx context.Context, path string) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || path == string(filepath.Separator) {
		return fmt.Errorf("remove tree: %q is not an absolute, clean, non-root path", path)
	}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("remove tree: %s is a symlink; the store root must be a directory", path)
	}
	if !info.IsDir() {
		return removeEntry(ctx, path)
	}
	return removeDirectory(ctx, path)
}

func removeDirectory(ctx context.Context, directory string) error {
	entries, err := os.ReadDir(directory)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	for _, entry := range entries {
		child := filepath.Join(directory, entry.Name())
		if entry.IsDir() && entry.Type()&os.ModeSymlink == 0 {
			if err := removeDirectory(ctx, child); err != nil {
				return err
			}
			continue
		}
		if err := removeEntry(ctx, child); err != nil {
			return err
		}
	}
	return removeEntry(ctx, directory)
}

func removeEntry(ctx context.Context, path string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("remove tree stopped at %s: %w", path, err)
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	afterTreeEntryRemoved(path)
	return nil
}

const copyChunk = 1 << 20

// CopyTree copies source to a destination that must not exist, in chunks,
// stopping at the context. Symlinks are copied as links and never followed;
// regular files are synced. A cut-short copy leaves a partial destination
// its caller's own registered store owns; nothing existing is overwritten.
func CopyTree(ctx context.Context, source, destination string) error {
	if !filepath.IsAbs(source) || !filepath.IsAbs(destination) {
		return fmt.Errorf("copy tree: %q and %q must be absolute", source, destination)
	}
	if _, err := os.Lstat(destination); err == nil {
		return fmt.Errorf("copy tree: %s exists; a copy never overwrites", destination)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	return copyEntry(ctx, source, destination, info)
}

func copyEntry(ctx context.Context, source, destination string, info os.FileInfo) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("copy tree stopped at %s: %w", source, err)
	}
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		target, err := os.Readlink(source)
		if err != nil {
			return err
		}
		return os.Symlink(target, destination)
	case info.IsDir():
		if err := os.Mkdir(destination, info.Mode().Perm()|0o700); err != nil {
			return err
		}
		entries, err := os.ReadDir(source)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			childInfo, err := entry.Info()
			if err != nil {
				return err
			}
			if err := copyEntry(ctx, filepath.Join(source, entry.Name()), filepath.Join(destination, entry.Name()), childInfo); err != nil {
				return err
			}
		}
		return nil
	case info.Mode().IsRegular():
		return copyFile(ctx, source, destination, info.Mode().Perm())
	default:
		return fmt.Errorf("copy tree: %s is neither a file, a directory nor a symlink", source)
	}
}

func copyFile(ctx context.Context, source, destination string, mode os.FileMode) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	for {
		if err := ctx.Err(); err != nil {
			return errors.Join(fmt.Errorf("copy tree stopped in %s: %w", source, err), out.Close())
		}
		written, err := io.CopyN(out, in, copyChunk)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return errors.Join(err, out.Close())
		}
		if written < copyChunk {
			break
		}
	}
	return errors.Join(out.Sync(), out.Close())
}
