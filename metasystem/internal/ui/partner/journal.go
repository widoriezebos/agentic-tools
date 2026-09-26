package partner

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// The wire journal's own writer, and the only thing that renames the file.
//
// Astra's F3 on g1-s54 is what this type answers. The journal is the io.Writer
// the connection journals every frame through, held for the life of one runtime
// process; renaming that path from outside would leave the connection writing
// to the renamed file, because a descriptor follows the inode and not the name.
// So the rotation is the writer's: it closes, renames, and reopens between two
// writes, under the same lock a write takes. Housekeeping asks; it never
// renames the path itself.
//
// One previous is kept. The rename replaces whatever `wire.1.jsonl` held, so
// the previous is kept and the older is gone in the one operation, and the
// journal's whole cost is at most twice the bound.
//
// One writer serves every runtime this host spawns, one after another, so every
// write and every close is bound to the generation that opened the file it
// means. Sol's read of g1-s54 under R-124 is what that answers: Host.Ready may
// find a connection dead and open the next runtime's journal before the old
// endpoint's close callback has run, and an unqualified close would then close
// the descriptor the new runtime writes through — after which every frame would
// report success and land nowhere.
//
// A journal that cannot be opened journals nothing, which is what this store
// did before there was a writer: the wire is evidence of a conversation, not
// the conversation, and a human's turn does not fail because a log file could
// not be made.
type Journal struct {
	path string

	mu   sync.Mutex
	file *os.File
	// generation names the open the current file belongs to. It moves on with
	// every Open and stays put across a rotation, which keeps the runtime that
	// is writing through it.
	generation Generation
	// written is what this file has taken since it was opened. The file is
	// truncated on open, so the count is the size, without a stat per write.
	written int64
}

// Generation names one open of the journal: one runtime process's whole wire,
// from the Open that started it to the Close that ends it.
type Generation uint64

// ErrStaleJournal is what a write or a close from a generation that no longer
// holds the journal answers. It is never a turn's failure: it says the runtime
// that asked has been replaced, so the frame it still carries belongs to no
// file this journal keeps, and the descriptor it would close is not its own.
var ErrStaleJournal = errors.New("this wire journal was reopened by a newer runtime")

// Sink is one generation's own writer, and the only thing that closes it: the
// connection journals through it, and the endpoint's teardown closes it.
type Sink struct {
	journal    *Journal
	generation Generation
}

// NewJournal names the file one runtime's frames are journalled to.
func NewJournal(path string) *Journal { return &Journal{path: path} }

// Path is the journal's own file.
func (j *Journal) Path() string { return j.path }

// Previous is where a rotation puts the file it closes: the same name with `.1`
// before its extension, beside it.
func (j *Journal) Previous() string {
	extension := filepath.Ext(j.path)
	return j.path[:len(j.path)-len(extension)] + ".1" + extension
}

// Open starts one process's journal, truncating what an earlier process left,
// and answers the sink bound to the generation it opened. The truncation is why
// the journal bounds itself across processes and why rotation is only ever
// needed inside one that runs long.
//
// The generation moves on whether or not the open succeeds: the descriptor the
// previous generation wrote through is closed here either way, so nothing that
// generation does afterwards may reach this journal.
func (j *Journal) Open() (*Sink, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.file != nil {
		_ = j.file.Close()
		j.file = nil
	}
	j.generation++
	if err := j.openLocked(true); err != nil {
		return nil, err
	}
	return &Sink{journal: j, generation: j.generation}, nil
}

// Generation is which open this sink belongs to.
func (s *Sink) Generation() Generation { return s.generation }

// Close ends this generation's journal. The writer stays: the next process
// opens it again, and housekeeping may hold it across both. A close from a
// generation that no longer holds the journal closes nothing and says so.
func (s *Sink) Close() error { return s.journal.close(s.generation) }

// Write takes one whole line, as the connection's serialized sink hands it
// over. A closed or unopened journal discards it and says so to nobody: this
// is the posture the store had before rotation existed, and a journal failure
// must not be the reason a turn does not answer.
//
// A write from a generation that no longer holds the journal is discarded and
// reported: the file is a newer runtime's, and an older runtime's frame must
// neither land in it nor be counted against its bound.
func (s *Sink) Write(b []byte) (int, error) { return s.journal.write(s.generation, b) }

func (j *Journal) close(generation Generation) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if generation != j.generation {
		return ErrStaleJournal
	}
	if j.file == nil {
		return nil
	}
	file := j.file
	j.file = nil
	return file.Close()
}

func (j *Journal) write(generation Generation, b []byte) (int, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if generation != j.generation {
		return 0, ErrStaleJournal
	}
	if j.file == nil {
		return len(b), nil
	}
	written, err := j.file.Write(b)
	j.written += int64(written)
	if err != nil {
		return written, err
	}
	return written, nil
}

// Size is what this journal holds, as the writer counts it.
func (j *Journal) Size() int64 {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.written
}

// Rotate keeps the journal under limit bytes, and reports whether it rotated.
//
// It happens between two writes, because a write takes the same lock: the
// connection either wrote its line into the file that is about to be renamed,
// or into the fresh one, and never into a file that was renamed under it. The
// generation is kept, because the runtime writing through it is kept: the sink
// that journalled before the rotation journals on after it.
//
// A limit of zero or less disables the bound, as every bound of this store
// does. A journal no process has open is left alone: the next process truncates
// it, so rotating it would only preserve a dead process's frames as the one
// previous and make the store bigger.
func (j *Journal) Rotate(limit int64) (bool, error) {
	if limit <= 0 {
		return false, nil
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.file == nil || j.written < limit {
		return false, nil
	}
	file := j.file
	j.file = nil
	if err := file.Close(); err != nil {
		// The descriptor is gone either way; reopening is what keeps the
		// runtime's remaining frames journalled.
		_ = j.openLocked(false)
		return false, fmt.Errorf("the Partner's wire journal could not be closed for rotation: %w", err)
	}
	if err := os.Rename(j.path, j.Previous()); err != nil {
		_ = j.openLocked(false)
		return false, fmt.Errorf("the Partner's wire journal could not be rotated: %w", err)
	}
	j.written = 0
	if err := j.openLocked(true); err != nil {
		return true, err
	}
	return true, nil
}

// openLocked opens the journal at its path, truncating or appending to what is
// there. The caller holds the lock.
func (j *Journal) openLocked(truncate bool) error {
	flags := os.O_CREATE | os.O_WRONLY
	if truncate {
		flags |= os.O_TRUNC
	} else {
		flags |= os.O_APPEND
	}
	file, err := os.OpenFile(j.path, flags, 0o600)
	if err != nil {
		return fmt.Errorf("the Partner's wire journal at %s could not be opened: %w", j.path, err)
	}
	j.file = file
	if truncate {
		j.written = 0
		return nil
	}
	if info, err := file.Stat(); err == nil {
		j.written = info.Size()
	}
	return nil
}
