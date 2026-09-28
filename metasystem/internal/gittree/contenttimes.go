package gittree

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

// Go's test cache keys a file a test opened or stat'ed on its path, size,
// mode and modification time, never its bytes, and refuses a file younger
// than two seconds. A fresh checkout stamps every file "now", so no cached
// result would ever match again. ContentTimes stamps every checked-out
// entry with a time derived from its Git object id instead: a blob's time
// from the blob id, a directory's from its tree id (which covers the whole
// subtree). Equal times then imply equal content, a stronger guarantee than
// Go's own mtime proxy, and every time lies decades in the past.
var contentTimeEpoch = time.Date(1990, time.January, 1, 0, 0, 0, 0, time.UTC)

const contentTimeSpan = uint64(30 * 365 * 24 * time.Hour)

// ContentTime is the modification time stamped for a Git object id.
func ContentTime(objectID string) (time.Time, error) {
	raw, err := hex.DecodeString(objectID)
	if err != nil || len(raw) < 8 {
		return time.Time{}, fmt.Errorf("gittree content time: %q is not an object id", objectID)
	}
	return contentTimeEpoch.Add(time.Duration(binary.BigEndian.Uint64(raw[:8]) % contentTimeSpan)), nil
}

// ContentTimes stamps the checked-out candidate with content-derived times
// and refreshes the index so Git's stat cache agrees with the new times.
func (d *DetachedWorktree) ContentTimes() error {
	worktree := d.control.at(d.top)
	listing, err := worktree.git(nil, "ls-tree", "-r", "-t", "-z", "--full-tree", "HEAD")
	if err != nil {
		return fmt.Errorf("gittree content times: list: %w", err)
	}
	type directory struct {
		path string
		when time.Time
	}
	var directories []directory
	for _, record := range bytes.Split(listing, []byte{0}) {
		meta, path, ok := strings.Cut(string(record), "\t")
		fields := strings.Fields(meta)
		if !ok || len(fields) != 3 {
			continue
		}
		when, err := ContentTime(fields[2])
		if err != nil {
			return err
		}
		absolute := filepath.Join(d.top, filepath.FromSlash(path))
		switch fields[0] {
		case "040000":
			directories = append(directories, directory{absolute, when})
		case "120000":
			value := unix.NsecToTimeval(when.UnixNano())
			err = unix.Lutimes(absolute, []unix.Timeval{value, value})
		case "100644", "100755":
			err = os.Chtimes(absolute, when, when)
		}
		if err != nil {
			return fmt.Errorf("gittree content times: %s: %w", path, err)
		}
	}
	root, err := worktree.gitLine(nil, "rev-parse", "HEAD^{tree}")
	if err != nil {
		return fmt.Errorf("gittree content times: root tree: %w", err)
	}
	rootTime, err := ContentTime(root)
	if err != nil {
		return err
	}
	directories = append(directories, directory{d.top, rootTime})
	// Deepest first, although utimes on a child never touches its parent.
	sort.SliceStable(directories, func(i, j int) bool { return len(directories[i].path) > len(directories[j].path) })
	for _, entry := range directories {
		if err := os.Chtimes(entry.path, entry.when, entry.when); err != nil {
			return fmt.Errorf("gittree content times: %s: %w", entry.path, err)
		}
	}
	if _, err := worktree.git(nil, "update-index", "-q", "--refresh"); err != nil {
		return fmt.Errorf("gittree content times: refresh index: %w", err)
	}
	return nil
}
