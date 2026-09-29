package diskstore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// RemoveTree removes path entry by entry and stops at the context: every
// removal the engine makes under a lock is bounded per entry and
// cancellable (3.3, DL3B-04). A cut-short removal leaves a partly removed
// tree that the next call finishes; an absent path is success. It never
// follows a symlink: a link is removed as an entry, and a symlinked root is
// refused. A directory inside the tree that its owner made read-only (Go
// leaves its module cache 0555) gets owner read, write and search added,
// without following a link, before its entries are removed; nothing outside
// the tree is ever changed. RemoveTree proves nothing about ownership; callers remove only
// inside a store's critical section, after its identity checks.
func RemoveTree(ctx context.Context, path string) error {
	return removeTree(ctx, path, nil)
}

// removeTree is RemoveTree with a per-call hook after each unlink (tests).
func removeTree(ctx context.Context, path string, after func(string)) error {
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
		return removeEntry(ctx, path, after)
	}
	return removeDirectory(ctx, path, after)
}

func removeDirectory(ctx context.Context, directory string, after func(string)) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("remove tree stopped at %s: %w", directory, err)
	}
	liftDirectory(directory)
	entries, err := os.ReadDir(directory)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	for _, entry := range entries {
		child := filepath.Join(directory, entry.Name())
		if entry.IsDir() && entry.Type()&os.ModeSymlink == 0 {
			if err := removeDirectory(ctx, child, after); err != nil {
				return err
			}
			continue
		}
		if err := removeEntry(ctx, child, after); err != nil {
			return err
		}
	}
	return removeEntry(ctx, directory, after)
}

// ownerAll is the owner's read, write and search bits a directory needs for
// its entries to be listed and removed.
const ownerAll = unix.S_IRWXU

// liftDirectory adds the owner bits to a directory this process owns that
// lacks them, never following a symlink: fchmodat with AT_SYMLINK_NOFOLLOW
// where the kernel has it, else a descriptor opened with O_NOFOLLOW whose
// device and inode are the ones just seen. Anything else is left as it is,
// and the removal that follows reports what stopped it.
func liftDirectory(path string) {
	var seen unix.Stat_t
	if err := unix.Lstat(path, &seen); err != nil || seen.Mode&unix.S_IFMT != unix.S_IFDIR || int(seen.Uid) != os.Geteuid() ||
		uint32(seen.Mode)&ownerAll == ownerAll {
		return
	}
	mode := uint32(seen.Mode)&0o7777 | ownerAll
	err := unix.Fchmodat(unix.AT_FDCWD, path, mode, unix.AT_SYMLINK_NOFOLLOW)
	if !errors.Is(err, unix.EOPNOTSUPP) && !errors.Is(err, unix.ENOSYS) {
		return
	}
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return
	}
	defer unix.Close(fd)
	var opened unix.Stat_t
	if unix.Fstat(fd, &opened) != nil || opened.Dev != seen.Dev || opened.Ino != seen.Ino {
		return
	}
	_ = unix.Fchmod(fd, mode)
}

func removeEntry(ctx context.Context, path string, after func(string)) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("remove tree stopped at %s: %w", path, err)
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if after != nil {
		after(path)
	}
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
