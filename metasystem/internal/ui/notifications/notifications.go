// Package notifications reads the steward's notification journal for the
// interface: the history a page opens with, and the lines that arrive after it.
//
// The journal is append-only, one JSON object per line, in the order the
// steward attempted delivery. That is the whole contract this package depends
// on, and it is what makes both readings cheap: the history is the tail of the
// file, and what is new is whatever lies past the offset the reader last
// consumed. Nothing here writes to the journal, locks it, or asks the steward
// anything — the steward does not know it is being read.
//
// A line that is not complete is not a line. A reader can catch an appender
// mid-write on a filesystem that does not make the write atomic, and the
// symptom is a final fragment with no newline after it; a fragment is skipped
// and read again on the next pass, when it is whole. A complete line that does
// not parse is skipped for good: one corrupt record must not end the history
// that surrounds it.
package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"time"
)

// Notice is one delivery attempt as the interface serves it.
//
// Every field is written, including the two the journal omits when it has
// nothing to say: a browser reading this shape never has to tell an absent
// field from an empty one.
type Notice struct {
	ID        string `json:"id"`
	At        string `json:"at"`
	Message   string `json:"message"`
	Source    string `json:"source"`
	Ref       string `json:"ref"`
	Delivered bool   `json:"delivered"`
	Error     string `json:"error"`
}

// The page sizes the routes offer. A caller that asks for more than the
// ceiling gets the ceiling: the history is for reading, not for downloading.
const (
	DefaultLimit = 200
	MaxLimit     = 1000
)

// Page is the newest limit notices, newest first.
//
// before, when it names an id, narrows the page to what was recorded strictly
// before that line — which is the "Load older" button, and the reason the
// whole file is walked rather than its tail alone: the second page is older
// than the tail the first one came from.
//
// A journal that does not exist is an empty page and not an error. The steward
// creates the file with its first delivery, and an installation that has never
// notified anybody has a history of nothing, which is an answer.
func Page(path string, limit int, before string) ([]Notice, error) {
	if limit <= 0 {
		limit = DefaultLimit
	}
	if limit > MaxLimit {
		limit = MaxLimit
	}
	data, err := readJournal(path)
	if err != nil {
		return nil, err
	}
	// A ring of the last `limit` matching notices: the file is walked once,
	// oldest line to newest, and only the page that will be answered is held.
	ring := make([]Notice, 0, limit)
	reached := before == ""
	forEachNotice(data, func(notice Notice) {
		if reached && before != "" {
			// Past the named line: everything here is newer than this page.
			return
		}
		if notice.ID == before {
			reached = true
			return
		}
		if len(ring) == limit {
			copy(ring, ring[1:])
			ring = ring[:limit-1]
		}
		ring = append(ring, notice)
	})
	// A `before` naming a line this journal does not carry pages nothing
	// rather than paging the whole history again.
	if !reached {
		return []Notice{}, nil
	}
	page := make([]Notice, 0, len(ring))
	for index := len(ring) - 1; index >= 0; index-- {
		page = append(page, ring[index])
	}
	return page, nil
}

// Follower reads what the journal gains, from a stated starting point.
//
// It holds a byte offset rather than an id, because the journal's order is the
// file's order: resuming is continuing to read, not searching. The offset only
// ever sits after a complete line.
type Follower struct {
	path   string
	offset int64
}

// Open starts a follower.
//
// after names the last notice the caller already has. Empty means "whatever
// comes next": the follower starts at the end of the journal and the caller
// receives nothing for what is already written. An id the journal carries
// means the caller is behind, and the backlog is returned at once, oldest
// first, so the order the caller sees is the order the steward wrote.
//
// An id this journal does not carry starts at the end too. It is a browser
// resuming against a journal that was replaced or an id it invented, and
// replaying the whole history at it would be worse than starting fresh.
func Open(path string, after string) (*Follower, []Notice, error) {
	data, err := readJournal(path)
	if err != nil {
		return nil, nil, err
	}
	complete := int64(completeLength(data))
	if after == "" {
		return &Follower{path: path, offset: complete}, []Notice{}, nil
	}
	behind := []Notice{}
	resumed := false
	forEachNotice(data, func(notice Notice) {
		if resumed {
			behind = append(behind, notice)
			return
		}
		if notice.ID == after {
			resumed = true
		}
	})
	if !resumed {
		return &Follower{path: path, offset: complete}, []Notice{}, nil
	}
	return &Follower{path: path, offset: complete}, behind, nil
}

// Next is every complete line appended since the last call, oldest first.
//
// A journal that shrank was replaced rather than appended to — the file was
// rotated or removed — so the follower starts again from its beginning rather
// than reading from an offset that now means something else.
func (f *Follower) Next() ([]Notice, error) {
	info, err := os.Stat(f.path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			f.offset = 0
			return []Notice{}, nil
		}
		return nil, err
	}
	size := info.Size()
	if size < f.offset {
		f.offset = 0
	}
	if size == f.offset {
		return []Notice{}, nil
	}
	file, err := os.Open(f.path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			f.offset = 0
			return []Notice{}, nil
		}
		return nil, err
	}
	defer file.Close()
	if _, err := file.Seek(f.offset, io.SeekStart); err != nil {
		return nil, err
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	arrived := []Notice{}
	forEachNotice(data, func(notice Notice) { arrived = append(arrived, notice) })
	// Only complete lines are consumed: a fragment at the end is an append in
	// flight, and it is read again whole on the next pass.
	f.offset += int64(completeLength(data))
	return arrived, nil
}

// Follow calls arrived for every batch the journal gains, until the context
// ends. It polls the file's size on the given interval, which is the whole
// mechanism: there is no watcher to leak, no inotify equivalent to be missing
// on a platform, and a journal that nobody appends to costs one stat a tick.
//
// An arrived that fails ends the follow with its error, because the caller it
// is writing to is gone.
func Follow(ctx context.Context, follower *Follower, every time.Duration, arrived func([]Notice) error) error {
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			notices, err := follower.Next()
			if err != nil {
				return err
			}
			if len(notices) == 0 {
				continue
			}
			if err := arrived(notices); err != nil {
				return err
			}
		}
	}
}

// readJournal is the whole file, or nothing where there is no journal yet.
func readJournal(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	return data, nil
}

// completeLength is how much of this data is complete lines: everything up to
// and including the last newline.
func completeLength(data []byte) int {
	at := bytes.LastIndexByte(data, '\n')
	if at < 0 {
		return 0
	}
	return at + 1
}

// forEachLine walks the complete lines. A trailing fragment is not a line and
// is not walked.
func forEachLine(data []byte, each func(line []byte)) {
	start := 0
	for start < len(data) {
		at := bytes.IndexByte(data[start:], '\n')
		if at < 0 {
			return
		}
		each(data[start : start+at])
		start = start + at + 1
	}
}

// forEachNotice walks the complete lines that parse. A line that does not is
// skipped: one corrupt record must not end the history around it.
func forEachNotice(data []byte, each func(Notice)) {
	forEachLine(data, func(line []byte) {
		if notice, ok := decode(line); ok {
			each(notice)
		}
	})
}

// decode reads one line. A record with no id is not a notice: the id is what
// the page sorts by, what the stream resumes from, and what unread is counted
// against, and a row without one cannot take part in any of it.
func decode(line []byte) (Notice, bool) {
	trimmed := bytes.TrimSpace(line)
	if len(trimmed) == 0 {
		return Notice{}, false
	}
	var notice Notice
	if err := json.Unmarshal(trimmed, &notice); err != nil {
		return Notice{}, false
	}
	if notice.ID == "" {
		return Notice{}, false
	}
	return notice, true
}
