package diskstore

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// ulidAlphabet is Crockford's base32.
const ulidAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// NewID mints a record id: a ULID of now's milliseconds and eighty bits of
// entropy, so ids sort by creation.
func NewID(now time.Time, entropy io.Reader) (string, error) {
	if entropy == nil {
		return "", errors.New("a store id needs an entropy source")
	}
	var raw [16]byte
	binary.BigEndian.PutUint64(raw[:8], uint64(now.UnixMilli())<<16)
	if _, err := io.ReadFull(entropy, raw[6:]); err != nil {
		return "", fmt.Errorf("mint a store id: %w", err)
	}
	high, low := binary.BigEndian.Uint64(raw[:8]), binary.BigEndian.Uint64(raw[8:])
	id := make([]byte, 26)
	for index := len(id) - 1; index >= 0; index-- {
		id[index] = ulidAlphabet[low&0x1f]
		low = low>>5 | high<<59
		high >>= 5
	}
	return string(id), nil
}

// ReadGitIdentity reads a linked worktree's identity from its .git file
// ("gitdir: <common>/worktrees/<name>") and that file's device, inode and,
// where the file system has one, inode generation: Linux reuses a freed
// inode number at once, so a .git file replaced at the same path can carry
// the recorded inode, never the recorded generation. It runs no git and
// writes nothing, so registration never dirties the tree.
func ReadGitIdentity(worktree string) (Identity, error) {
	path := filepath.Join(worktree, ".git")
	info, err := os.Lstat(path)
	if err != nil {
		return Identity{}, fmt.Errorf("worktree %s has no .git file: %w", worktree, err)
	}
	if !info.Mode().IsRegular() {
		return Identity{}, fmt.Errorf("worktree %s: .git is not a file (a main checkout is never a registered store)", worktree)
	}
	file, err := os.Open(path)
	if err != nil {
		return Identity{}, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil {
		return Identity{}, err
	}
	device, inode, ok := fileID(info)
	openedDevice, openedInode, openedOK := fileID(opened)
	if !ok || !openedOK {
		return Identity{}, fmt.Errorf("worktree %s: .git has no inode on this platform", worktree)
	}
	if openedDevice != device || openedInode != inode {
		return Identity{}, fmt.Errorf("worktree %s: .git changed while it was read", worktree)
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return Identity{}, err
	}
	line := strings.TrimSpace(string(data))
	gitdir, ok := strings.CutPrefix(line, "gitdir: ")
	if !ok || strings.ContainsAny(gitdir, "\n\r") || gitdir == "" {
		return Identity{}, fmt.Errorf("worktree %s: .git does not name a gitdir", worktree)
	}
	if !filepath.IsAbs(gitdir) {
		gitdir = filepath.Join(worktree, gitdir)
	}
	return Identity{Gitdir: filepath.Clean(gitdir), GitFileDevice: device, GitFileInode: inode,
		GitFileGeneration: fileGeneration(file)}, nil
}

func fileID(info os.FileInfo) (device, inode uint64, ok bool) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, 0, false
	}
	return uint64(stat.Dev), uint64(stat.Ino), true
}

// Revalidate checks that the store at the record's path is still the one
// the record names: the marker names this record, or the .git file still
// names the recorded gitdir with the recorded device and inode. A mismatch
// is an error naming what changed; the caller keeps the store.
func Revalidate(record Record) error {
	info, err := os.Lstat(record.Path)
	if err != nil {
		return fmt.Errorf("store %s: %w", record.Path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("store %s is not a directory", record.Path)
	}
	if record.Identity.Marker {
		found, err := readMarker(record.Path)
		if err != nil {
			return err
		}
		if found.ID != record.ID || found.Path != record.Path {
			return fmt.Errorf("store %s carries the marker of record %s, not %s", record.Path, found.ID, record.ID)
		}
		return sameRoot(record, info)
	}
	current, err := ReadGitIdentity(record.Path)
	if err != nil {
		return err
	}
	if current != record.Identity {
		return fmt.Errorf("worktree %s is not the recorded one (gitdir %s inode %d generation %d, recorded %s inode %d generation %d)",
			record.Path, current.Gitdir, current.GitFileInode, current.GitFileGeneration,
			record.Identity.Gitdir, record.Identity.GitFileInode, record.Identity.GitFileGeneration)
	}
	return nil
}
