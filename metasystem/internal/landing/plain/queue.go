// Package plain is the plain landing lane (plain-lane design, revision 3;
// first landed through itself on 2026-10-02, at night; checked for pause):
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
	"slices"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/conflict"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
)

// The states of a queued line. Waiting and returned are recorded; landed
// is derived: main contains the line's sha.
const (
	StateWaiting  = "waiting"
	StateLanded   = "landed"
	StateReturned = "returned"
	// StateSuperseded is a waiting line a newer hand-in of its goal
	// replaced: the lane no longer has work for it.
	StateSuperseded = "superseded"
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
	Previous     *Entry        `json:"-"`
	ExpectEmpty  bool          `json:"-"`
	ReturnOrigin *ReturnOrigin `json:"return-origin,omitempty"`
	goal.AreaSnapshot
	Exception *Exception       `json:"exception,omitempty"`
	Fix       string           `json:"fix,omitempty"`
	DrainBy   string           `json:"drainBy,omitempty"`
	WholeBy   string           `json:"wholeBy,omitempty"`
	Again     bool             `json:"-"`
	Goal      string           `json:"goal"`
	Branch    string           `json:"branch,omitempty"`
	SHA       string           `json:"sha"`
	Seat      string           `json:"seat,omitempty"`
	Records   bool             `json:"records,omitempty"`
	At        string           `json:"at"`
	Outcome   string           `json:"outcome,omitempty"`
	Reason    string           `json:"reason,omitempty"`
	Cause     *Cause           `json:"cause,omitempty"`
	After     []GoalSHA        `json:"after,omitempty"`
	Held      bool             `json:"held,omitempty"`
	Conflict  *conflict.Return `json:"conflict,omitempty"`
	// Delivered is the hand-in's one plain sentence of what it delivers,
	// written by the agent that did the work; the channel posts it when
	// the work reaches main.
	Delivered string       `json:"delivered,omitempty"`
	Units     []UnitRounds `json:"units,omitempty"`
}

// UnitRounds records a named unit's attempts and the proof and read used at hand-in.
type UnitRounds struct {
	Unit      string         `json:"unit"`
	Counted   int            `json:"counted"`
	Machinery map[string]int `json:"machinery,omitempty"`
	Proof     []string       `json:"proof"`
	Read      string         `json:"read"`
}

// Entry is one hand-in and what became of it.
type Entry struct {
	ReturnOrigin *ReturnOrigin `json:"return-origin,omitempty"`
	goal.AreaSnapshot
	DrainBy   string           `json:"drainBy,omitempty"`
	Exception *Exception       `json:"exception,omitempty"`
	Fix       string           `json:"fix,omitempty"`
	Goal      string           `json:"goal"`
	Branch    string           `json:"branch"`
	SHA       string           `json:"sha"`
	Seat      string           `json:"seat"`
	Records   bool             `json:"records,omitempty"`
	At        string           `json:"at"`
	State     string           `json:"state"`
	Reason    string           `json:"reason,omitempty"`
	Cause     *Cause           `json:"cause,omitempty"`
	After     []GoalSHA        `json:"after,omitempty"`
	Held      bool             `json:"held,omitempty"`
	Conflict  *conflict.Return `json:"conflict,omitempty"`
	// Delivered is the hand-in's plain sentence of what it delivers.
	Delivered string       `json:"delivered,omitempty"`
	Units     []UnitRounds `json:"units,omitempty"`
	// ReturnedAt is when it was returned.
	ReturnedAt string `json:"returned_at,omitempty"`
	// LandedAt is when a landed hand-in reached main: the time of the push
	// that brought its commit, read from pushes.jsonl (LandingTimes); empty
	// where no push of the lane's last day brought it.
	LandedAt string `json:"landed_at,omitempty"`
	Landing  *Clock `json:"landing,omitempty"`
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
// is empty. A line that does not decode (a crash mid-append) is skipped, so
// the keeper and the verbs never stop on a partial append.
func readLines[T any](path string) ([]T, error) {
	out, _, err := countedLines[T](path)
	return out, err
}

// countedLines is readLines that also counts the lines it skipped because
// they do not decode, for the lane's status to say (fix round 4); a blank
// line holds no record and is not counted.
func countedLines[T any](path string) ([]T, int, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, 0, nil
	}
	if err != nil {
		return nil, 0, err
	}
	var out []T
	skipped := 0
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		var value T
		switch {
		case json.Unmarshal(scanner.Bytes(), &value) == nil:
			out = append(out, value)
		case len(bytes.TrimSpace(scanner.Bytes())) > 0:
			skipped++
		}
	}
	return out, skipped, scanner.Err()
}

// Entries are the queue's hand-ins, oldest first, each with its recorded
// state: waiting, returned, or superseded by a newer hand-in of its goal
// (Landed derives landed).
func Entries(install string) ([]Entry, error) {
	lines, err := readLines[Line](queuePath(install))
	if err != nil {
		return nil, err
	}
	return entriesOf(lines), nil
}

// entriesOf are the hand-ins the queue's lines record, as Entries says.
func entriesOf(lines []Line) []Entry {
	entries := []Entry{}
	index := map[string]int{}
	for _, line := range lines {
		key := line.Goal + "@" + line.SHA
		if line.Outcome == "" {
			if at, seen := index[key]; seen && entries[at].State != StateReturned {
				// A repeat adds its delivery sentence or explicit
				// exception to the waiting line.
				if line.Exception != nil {
					entries[at].Exception = line.Exception
				}
				if line.Fix != "" {
					entries[at].Fix = line.Fix
				}
				if line.Delivered != "" {
					entries[at].Delivered = line.Delivered
				}
				continue
			}
			if line.Goal != "" && line.SHA != "" {
				// A new hand-in of a goal supersedes its older waiting line.
				for at := range entries {
					if entries[at].Goal == line.Goal && entries[at].State == StateWaiting {
						entries[at].State = StateSuperseded
					}
				}
				index[key] = len(entries)
				entries = append(entries, Entry{AreaSnapshot: line.AreaSnapshot, DrainBy: line.DrainBy, Exception: line.Exception, Fix: line.Fix, Goal: line.Goal, Branch: line.Branch, SHA: line.SHA, Seat: line.Seat, Records: line.Records, At: line.At, State: StateWaiting, Delivered: line.Delivered, Units: line.Units})
			}
			continue
		}
		at, seen := index[key]
		if !seen || entries[at].State != StateWaiting {
			continue
		}
		entries[at].State, entries[at].Reason = line.Outcome, line.Reason
		if line.Outcome == StateReturned {
			entries[at].ReturnedAt = line.At
			entries[at].ReturnOrigin = line.ReturnOrigin
		}
		entries[at].After, entries[at].Held = line.After, line.Held
		entries[at].Conflict, entries[at].Cause = line.Conflict, line.Cause
	}
	return entries
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
// sha is a repeat: the existing entry is returned with added false, and
// nothing is appended unless the repeat brings a new delivery sentence,
// exception or fix incident for a waiting line. Again re-queues a returned
// line; a waiting repeat writes only new delivery information.
func HandIn(install string, line Line, person ...string) (Entry, bool, error) {
	return handIn(install, line, person)
}

// HandInChecked rechecks an exception at the queue write, retaining the verified caller's drain authority.
func HandInChecked(install string, line Line, person string, seams ProveSeams) (Entry, bool, error) {
	return handIn(install, line, []string{person}, seams)
}

func handIn(install string, line Line, person []string, effects ...ProveSeams) (entry Entry, added bool, err error) {
	if line.Goal == "" || line.SHA == "" {
		return Entry{}, false, errors.New("a hand-in names its goal and its commit")
	}
	line.Outcome, line.Reason, line.Conflict, line.Cause, line.After, line.Held = "", "", nil, nil, nil, false
	err = withLock(install, func() error {
		readEntries := Entries
		if len(effects) > 0 && line.Exception != nil {
			readEntries = AreaEntries
		}
		entries, err := readEntries(install)
		if err != nil {
			return err
		}
		if len(effects) > 0 && line.Exception != nil {
			current, known, err := Latest(install, line.Goal)
			if err != nil {
				return err
			}
			if line.ExpectEmpty && known || line.Previous != nil && (!known || current.SHA != line.Previous.SHA || current.State != line.Previous.State) {
				return fmt.Errorf("the queued target changed; repeat the exception for the current hand-in")
			}
			seams := effects[0]
			if seams.Lane == nil {
				return fmt.Errorf("the exception's destination cannot be checked")
			}
			registered, err := seams.Lane()
			if err != nil {
				return err
			}
			if line.Exception.Person == nil || line.Exception.Person.Destination != registered {
				return fmt.Errorf("the exception's lane registration changed; repeat the act")
			}
			if !line.Exception.BindingUnknown {
				if line.Exception.Incidents == nil {
					return fmt.Errorf("the exception has no bound incident set")
				}
				main, err := checkoutGit(registered.Root, seams).main()
				if err != nil {
					return err
				}
				incidents, err := seams.incidents(install, registered.Root, main)
				if err != nil {
					return fmt.Errorf("the incident set changed or became unreadable; repeat the exception: %w", err)
				}
				if !slices.Equal(line.Exception.Incidents.Open, openIncidents(incidents)) {
					return fmt.Errorf("the incident set changed; repeat the exception for the current incidents")
				}
			}
		}
		line.DrainBy = ""
		for index := len(entries) - 1; index >= 0; index-- {
			existing := entries[index]
			if existing.Goal == line.Goal && existing.SHA == line.SHA {
				if line.Again && existing.State == StateReturned {
					break
				}
				entry = existing
				if existing.State != StateWaiting || (line.Delivered == "" || line.Delivered == existing.Delivered) && (line.Exception == nil || sameException(line.Exception, existing.Exception)) && (line.Fix == "" || line.Fix == existing.Fix) {
					return nil
				}
				// Delivery information belongs to the waiting line; it
				// does not create another hand-in.
				if err := appendLine(queuePath(install), line); err != nil {
					return err
				}
				if line.Delivered != "" {
					entry.Delivered = line.Delivered
				}
				if line.Exception != nil {
					entry.Exception = line.Exception
				}
				if line.Fix != "" {
					entry.Fix = line.Fix
				}
				return nil
			}
		}
		drain, drainErr := ReadDrain(install)
		if drain != nil || drainErr != nil {
			if len(person) == 0 || person[0] == "" {
				// An agent is refused while admission is closed whatever the
				// queue holds; the refusal's remedy (work land G once landing
				// status shows admission open) is the one that works.
				return &AdmissionClosed{Drain: drain, Unreadable: drainErr}
			}
			line.DrainBy = person[0]
			if drain != nil && drain.State == DrainHeld {
				drain.State = DrainDraining
				if err := writeDrain(install, *drain); err != nil {
					return err
				}
			}
		}
		if line.Delivered == "" {
			// A goal handed in again after a return, without a sentence,
			// keeps the returned line's; a landed or waiting line's
			// sentence was for other work. A new sentence replaces it.
			for index := len(entries) - 1; index >= 0; index-- {
				if entries[index].Goal == line.Goal {
					if entries[index].State == StateReturned {
						line.Delivered = entries[index].Delivered
					}
					break
				}
			}
		}
		if err := appendLine(queuePath(install), line); err != nil {
			return err
		}
		if err := closeGoalStopsLocked(install, line.Goal, "superseded by hand-in "+line.Goal, time.Now()); err != nil {
			return err
		}
		entry, added = Entry{AreaSnapshot: line.AreaSnapshot, DrainBy: line.DrainBy, Exception: line.Exception, Fix: line.Fix, Goal: line.Goal, Branch: line.Branch, SHA: line.SHA, Seat: line.Seat, Records: line.Records, At: line.At, State: StateWaiting, Delivered: line.Delivered, Units: line.Units}, true
		return nil
	})
	return entry, added, err
}

// Say keeps a sentence of what it delivers with the waiting hand-in of
// goal at sha; with no such line, or the same sentence, nothing changes.
func Say(install, goal, sha, delivered string) error {
	return withLock(install, func() error {
		entries, err := Entries(install)
		if err != nil {
			return err
		}
		for _, existing := range entries {
			if existing.Goal == goal && existing.SHA == sha && existing.State == StateWaiting && existing.Delivered != delivered && delivered != "" {
				return appendLine(queuePath(install), Line{Goal: goal, SHA: sha, At: time.Now().UTC().Format(time.RFC3339), Delivered: delivered})
			}
		}
		return nil
	})
}

// ErrNotWaiting is a return of a goal with no waiting hand-in.
var ErrNotWaiting = errors.New("no hand-in of this goal waits in the lane")

func returnLocked(install, goal, reason string, cause *Cause, detail *conflict.Return, now time.Time, effects ...ProveSeams) (Entry, bool, error) {
	latest, ok, err := Latest(install, goal)
	switch {
	case err != nil:
		return Entry{}, false, err
	case !ok:
		return Entry{}, false, fmt.Errorf("%w: %s was never handed in", ErrNotWaiting, goal)
	case latest.State == StateReturned:
		return latest, false, closeFix(install, goal)
	case latest.State != StateWaiting:
		return latest, false, fmt.Errorf("%w: %s already %s", ErrNotWaiting, goal, latest.State)
	}
	seams := ProveSeams{}
	if len(effects) > 0 {
		seams = effects[0]
	}
	origin, err := checkReturnLocked(install, latest, cause, detail, now, seams)
	if err != nil {
		return latest, false, err
	}
	if fix, err := currentFix(install, seams); err == nil && fix != nil && fix.Goal == goal {
		reason += fmt.Sprintf("; fix round %d, job %s, read %s", fix.Round, fix.Job, fix.Read)
	}
	at := now.UTC().Format(time.RFC3339)
	if err := appendLine(queuePath(install), Line{ReturnOrigin: &origin, Goal: goal, SHA: latest.SHA, At: at, Outcome: StateReturned, Reason: reason, Cause: cause, Conflict: detail}); err != nil {
		return latest, false, err
	}
	latest.ReturnOrigin = &origin
	latest.State, latest.Reason, latest.ReturnedAt, latest.Conflict, latest.Cause = StateReturned, reason, at, detail, cause
	if err := closeGoalStopsLocked(install, goal, "return "+goal, now, latest.SHA); err != nil {
		return latest, true, err
	}
	return latest, true, closeFix(install, goal)
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
	for at := range out {
		if len(out[at].After) == 0 || out[at].State != StateWaiting {
			continue
		}
		out[at].Held = false
		for _, before := range out[at].After {
			for _, candidate := range out {
				if candidate.Goal == before.Goal && candidate.SHA == before.SHA && candidate.State == StateWaiting {
					out[at].Held = true
				}
			}
		}
	}
	return out, errors.Join(problems...)
}

// Pending are the hand-ins the lane still has work for: neither returned
// nor contained in origin's main as the lane checkout last fetched it. It
// is the signal a keeper wakes the landing agent on.
func Pending(install, checkout string, effects ...ProveSeams) ([]Entry, error) {
	seams := ProveSeams{}
	if len(effects) > 0 {
		seams = effects[0]
	}
	return pending(install, checkout, seams)
}

func pending(install, checkout string, seams ProveSeams) ([]Entry, error) {
	return pendingQueue(install, checkout, seams, false)
}

func pendingQueue(install, checkout string, seams ProveSeams, refresh bool) ([]Entry, error) {
	waiting, err := Waiting(install)
	if err != nil || len(waiting) == 0 {
		return []Entry{}, err
	}
	main, err := seams.git(checkout, "rev-parse", "--verify", "--quiet", "refs/remotes/origin/main^{commit}")
	if err != nil {
		main = ""
	}
	contains := func(sha string) (bool, error) {
		if main == "" {
			return false, nil
		}
		return checkoutGit(checkout, seams).contains(main, sha)
	}
	derived, err := Landed(waiting, contains)
	if err != nil {
		return nil, err
	}
	if main != "" {
		incidents, err := seams.incidents(install, checkout, main)
		if err != nil {
			return nil, err
		}
		held := HoldEntries(derived, incidents, seams)
		if refresh && incidentOnlyHolds(derived, held) {
			if err := seams.fetchMain(checkout); err != nil {
				return nil, err
			}
			return pendingQueue(install, checkout, seams, false)
		}
		derived = held
	}
	pending := []Entry{}
	for _, entry := range derived {
		if entry.State == StateWaiting && !entry.Held {
			pending = append(pending, entry)
		}
	}
	return pending, err
}

// AreaEntries preserves queue corruption as unknown admission advice. Ordinary
// queue readers still tolerate interrupted appends for lane recovery.
func AreaEntries(install string) ([]Entry, error) {
	lines, skipped, err := countedLines[Line](queuePath(install))
	if err != nil {
		return nil, err
	}
	if skipped != 0 {
		return nil, fmt.Errorf("%s has %d unreadable queue lines", queuePath(install), skipped)
	}
	return entriesOf(lines), nil
}
