// Package plain is the plain landing lane (plain-lane design, revision 3):
// seats hand goal branches to the lane by one line in a queue file, the
// landing agent merges and proves them, and landing push puts a proven HEAD
// on main. Nothing is settled: a line is landed when main contains its sha,
// derived whenever it is read, and the seat concludes its own goal. Its
// threat model is our own agents and operators making mistakes and
// crashing: nothing here defends against an adversary. Every record is a
// JSON line appended under one small file lock, and every act is
// idempotent.
package plain

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
)

// The states of a queued line. Waiting and returned are recorded; landed
// is derived: main contains the line's sha.
const (
	StateWaiting  = "waiting"
	StateLanded   = "landed"
	StateReturned = "returned"
)

// Dir is where the lane keeps its records: in the lane installation.
func Dir(install string) string {
	return filepath.Join(install, "artifacts", "agents", "landing")
}

func queuePath(install string) string   { return filepath.Join(Dir(install), "queue.jsonl") }
func resultsPath(install string) string { return filepath.Join(Dir(install), "results.jsonl") }
func pushesPath(install string) string  { return filepath.Join(Dir(install), "pushes.jsonl") }
func runningPath(install string) string { return filepath.Join(Dir(install), "running.json") }
func lockPath(install string) string    { return filepath.Join(Dir(install), "lane.lock") }

// Line is one line of queue.jsonl: a seat's hand-in (no Outcome), or the
// "returned" outcome of the hand-in with the same goal and sha.
type Line struct {
	Goal    string `json:"goal"`
	Branch  string `json:"branch,omitempty"`
	SHA     string `json:"sha"`
	Seat    string `json:"seat,omitempty"`
	At      string `json:"at"`
	Outcome string `json:"outcome,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

// Entry is one hand-in and what became of it.
type Entry struct {
	Goal   string `json:"goal"`
	Branch string `json:"branch"`
	SHA    string `json:"sha"`
	Seat   string `json:"seat"`
	At     string `json:"at"`
	State  string `json:"state"`
	Reason string `json:"reason,omitempty"`
	// ReturnedAt is when it was returned.
	ReturnedAt string `json:"returned_at,omitempty"`
}

// withLock runs fn under the lane's file lock, creating its folder.
func withLock(install string, fn func() error) error {
	if err := os.MkdirAll(Dir(install), 0o755); err != nil {
		return err
	}
	held, err := lock.File(lockPath(install), 0o600, lock.Exclusive)
	if err != nil {
		return err
	}
	defer held.Release()
	return fn()
}

// appendLine appends one JSON line in a single write.
func appendLine(path string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(append(data, '\n'))
	return errors.Join(writeErr, file.Close())
}

// readLines decodes every line of a JSON-lines file into T; a missing file
// is empty. A line that does not decode (a crash mid-append) is skipped.
func readLines[T any](path string) ([]T, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []T
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		var value T
		if json.Unmarshal(scanner.Bytes(), &value) == nil {
			out = append(out, value)
		}
	}
	return out, scanner.Err()
}

// Entries are the queue's hand-ins, oldest first, each with its recorded
// state: waiting or returned (Landed derives landed).
func Entries(install string) ([]Entry, error) {
	lines, err := readLines[Line](queuePath(install))
	if err != nil {
		return nil, err
	}
	entries := []Entry{}
	index := map[string]int{}
	for _, line := range lines {
		key := line.Goal + "@" + line.SHA
		if line.Outcome == "" {
			if _, seen := index[key]; !seen && line.Goal != "" && line.SHA != "" {
				index[key] = len(entries)
				entries = append(entries, Entry{Goal: line.Goal, Branch: line.Branch, SHA: line.SHA, Seat: line.Seat, At: line.At, State: StateWaiting})
			}
			continue
		}
		at, seen := index[key]
		if !seen || entries[at].State != StateWaiting {
			continue
		}
		entries[at].State, entries[at].Reason, entries[at].ReturnedAt = line.Outcome, line.Reason, line.At
	}
	return entries, nil
}

// Waiting are the hand-ins not returned, landed or not.
func Waiting(install string) ([]Entry, error) {
	entries, err := Entries(install)
	waiting := []Entry{}
	for _, entry := range entries {
		if entry.State == StateWaiting {
			waiting = append(waiting, entry)
		}
	}
	return waiting, err
}

// Latest is the newest hand-in of a goal.
func Latest(install, goal string) (Entry, bool, error) {
	entries, err := Entries(install)
	if err != nil {
		return Entry{}, false, err
	}
	for index := len(entries) - 1; index >= 0; index-- {
		if entries[index].Goal == goal {
			return entries[index], true, nil
		}
	}
	return Entry{}, false, nil
}

// HandIn appends a seat's hand-in. A hand-in of the same goal at the same
// sha is a repeat: nothing is appended and the existing entry is returned
// with added false.
func HandIn(install string, line Line) (entry Entry, added bool, err error) {
	if line.Goal == "" || line.SHA == "" {
		return Entry{}, false, errors.New("a hand-in names its goal and its commit")
	}
	line.Outcome, line.Reason = "", ""
	err = withLock(install, func() error {
		entries, err := Entries(install)
		if err != nil {
			return err
		}
		for _, existing := range entries {
			if existing.Goal == line.Goal && existing.SHA == line.SHA {
				entry = existing
				return nil
			}
		}
		if err := appendLine(queuePath(install), line); err != nil {
			return err
		}
		entry, added = Entry{Goal: line.Goal, Branch: line.Branch, SHA: line.SHA, Seat: line.Seat, At: line.At, State: StateWaiting}, true
		return nil
	})
	return entry, added, err
}

// ErrNotWaiting is a return of a goal with no waiting hand-in.
var ErrNotWaiting = errors.New("no hand-in of this goal waits in the lane")

// Return appends a "returned" outcome for the goal's waiting hand-in. A
// goal whose newest hand-in is already returned is a repeat (changed
// false); a goal with nothing waiting is ErrNotWaiting.
func Return(install, goal, reason string, now time.Time) (entry Entry, changed bool, err error) {
	err = withLock(install, func() error {
		latest, ok, err := Latest(install, goal)
		switch {
		case err != nil:
			return err
		case !ok:
			return fmt.Errorf("%w: %s was never handed in", ErrNotWaiting, goal)
		case latest.State == StateReturned:
			entry = latest
			return nil
		case latest.State != StateWaiting:
			return fmt.Errorf("%w: %s already %s", ErrNotWaiting, goal, latest.State)
		}
		at := now.UTC().Format(time.RFC3339)
		if err := appendLine(queuePath(install), Line{Goal: goal, SHA: latest.SHA, At: at, Outcome: StateReturned, Reason: reason}); err != nil {
			return err
		}
		latest.State, latest.Reason, latest.ReturnedAt = StateReturned, reason, at
		entry, changed = latest, true
		return nil
	})
	return entry, changed, err
}

// Landed derives each waiting entry's landing: one whose sha main
// contains is landed. contains reads it; an error leaves the entry
// waiting and is returned with the rest still derived.
func Landed(entries []Entry, contains func(sha string) (bool, error)) ([]Entry, error) {
	out := make([]Entry, 0, len(entries))
	var problems []error
	for _, entry := range entries {
		if entry.State == StateWaiting {
			inside, err := contains(entry.SHA)
			if err != nil {
				problems = append(problems, fmt.Errorf("%s: %w", entry.Goal, err))
			} else if inside {
				entry.State = StateLanded
			}
		}
		out = append(out, entry)
	}
	return out, errors.Join(problems...)
}

// Pending are the hand-ins the lane still has work for: neither returned
// nor contained in origin's main as the lane checkout last fetched it. It
// is the signal a keeper wakes the landing agent on.
func Pending(install, checkout string) ([]Entry, error) {
	waiting, err := Waiting(install)
	if err != nil || len(waiting) == 0 {
		return []Entry{}, err
	}
	main, err := Git(checkout, "rev-parse", "--verify", "--quiet", "refs/remotes/origin/main^{commit}")
	if err != nil {
		return waiting, nil
	}
	derived, err := Landed(waiting, ContainedIn(checkout, main))
	pending := []Entry{}
	for _, entry := range derived {
		if entry.State == StateWaiting {
			pending = append(pending, entry)
		}
	}
	return pending, err
}
