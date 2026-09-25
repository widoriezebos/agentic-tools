package fleet

// A launch record that changed is a cause of the `fleet` event.
//
// The page reads /api/fleet and re-reads on that one event, and it holds no
// timer: a launch takes minutes and rewrites its record after every step, and
// a page that only learned of it on the presence fetcher's cadence would
// show a clone finishing a minute after it did.
//
// So the server watches the records the way it watches presence: on the poll
// tick it already has, reading a fingerprint of the directory — the names,
// sizes and modification times of the records in it — and announcing only
// when that fingerprint differs from the last one. It is a read of a
// directory this server owns and not a fetch; nothing here decides anything
// about a launch, and nothing here writes.

import (
	"context"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

// LaunchWatch announces the fleet event when this host's launch records
// change.
type LaunchWatch struct {
	// Fingerprint is what the records look like now. Two readings that differ
	// are a change; two that agree are not.
	Fingerprint func() string
	// Announce is the one `fleet` event every open stream carries.
	Announce func()

	read bool
	last string
}

// Consider reads the fingerprint once and announces if it moved. It reports
// whether it announced, so a test drives the rule without a stream.
//
// The first reading announces nothing: a server that started with a launch
// already on disk has changed nothing, and the page reads that record on
// mount anyway.
func (w *LaunchWatch) Consider() bool {
	if w == nil || w.Fingerprint == nil {
		return false
	}
	now := w.Fingerprint()
	moved := w.read && now != w.last
	w.read, w.last = true, now
	if moved && w.Announce != nil {
		w.Announce()
	}
	return moved
}

// Run considers on every tick until the context ends. The tick is the
// caller's, so the server passes its own ticker and a test passes a channel.
func (w *LaunchWatch) Run(ctx context.Context, tick <-chan time.Time) {
	for {
		select {
		case <-ctx.Done():
			return
		case _, open := <-tick:
			if !open {
				return
			}
			w.Consider()
		}
	}
}

// FingerprintOf is one reading of a directory of records: every entry's name,
// size and modification time, in name order.
//
// It is deliberately not a hash of the contents. A record is rewritten
// atomically after every step, so the file's size and time move with it, and
// reading a directory listing costs one system call however long a launch
// runs. A directory that cannot be read fingerprints as empty, which is what
// a host with no launches reads as: neither is a change to announce.
func FingerprintOf(directory string) string {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return ""
	}
	lines := make([]string, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		lines = append(lines, entry.Name()+":"+strconv.FormatInt(info.Size(), 10)+":"+
			strconv.FormatInt(info.ModTime().UnixNano(), 10))
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}
