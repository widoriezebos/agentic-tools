package overview

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
)

// The last visit.
//
// A visit is a span of looking, not a click. A human opens the page, reads it,
// opens a goal, comes back, presses Refresh — that is one visit, and every one
// of those reads has to be told the same thing about what changed. So what the
// page compares against is not "the last time you loaded this": it is the end
// of the visit BEFORE this one, which does not move while a visit is going on.
// Pressing Refresh during a visit therefore never narrows the window, which is
// the whole reason this file exists instead of a single timestamp.
//
// What separates two visits is a gap. A read more than thirty minutes after
// the last one begins a new visit; anything sooner continues the one in
// progress. Thirty minutes is a coffee and a meeting, and it is deliberately
// not tunable: a rule a human cannot see the value of is a rule they cannot
// reason about when the page surprises them.
//
// The file lives beside the interface's other lifecycle state, one row per
// human, per checkout — the file is under the checkout's state root, so the
// checkout is in the path rather than in a key. It is preference state and
// nothing more: losing it changes the comparison window on the next read and
// nothing about the work. That is why a file this build cannot read is
// treated as a first visit rather than as a refusal: the alternative is a page
// that will not render because a cache is corrupt.
//
// The human is a handle, which is the master's "per human" as this build can
// spell it. Two handles that name one person are two rows, and a seat that
// names nobody is the row under the empty handle — the seat itself. An encoded
// identity is the honest key and is deferred; the cost of the handle is a
// window, never an act.

// visitGap is how long a page can go unread before the next read is a new
// visit.
const visitGap = 30 * time.Minute

// firstWindow is how far back a first visit looks. There is no previous visit
// to end, and a page that compared against the beginning of time would tell a
// human that everything ever recorded changed while they were away.
const firstWindow = 24 * time.Hour

// visitsSchema is the shape of the file this build writes and reads.
const visitsSchema = 1

// Visits is the whole file: one row per human.
type Visits struct {
	SchemaVersion int               `json:"schemaVersion"`
	Humans        map[string]Marker `json:"humans"`
}

// Marker is one human's last-visit marker: the visit in progress, and the end
// of the one before it. It is the master's "last-visit marker kept per human
// in server-local preference state", written out as the three instants the
// rule above needs.
type Marker struct {
	// Began is when the visit in progress started.
	Began string `json:"began"`
	// Seen is when that human last read the page, which is what the gap is
	// measured from.
	Seen string `json:"seen"`
	// Previous is when the visit before this one ended, which is the instant
	// the page compares against. It is empty while the first visit is still
	// in progress.
	Previous string `json:"previous"`
}

// visiting serializes this process's rewrites of the file, as the lifecycle
// slice serializes its own and for the same reason: one server owns one state
// root for its whole life, so a mutex is the whole of the exclusion needed.
var visiting sync.Mutex

// interfaceDir is the interface's lifecycle directory beneath the state root:
// the directory that holds server.json and sessions.json, and now the marker
// beside them, because it is the same kind of thing — what one run of the
// server hands the next about a human.
//
// It is spelled here rather than imported from the lifecycle slice for the
// reason the project slice gives for its own Roots: that package's tests
// import the server, the server imports this one, and taking a name from it
// would close an import cycle. The one fact shared is a directory name, and
// the lifecycle slice owns it.
func interfaceDir(stateRoot string) string {
	return filepath.Join(stateRoot, "artifacts", "agents", "ui")
}

// VisitsPath is the file this package keeps the marker in.
func VisitsPath(stateRoot string) string {
	return filepath.Join(interfaceDir(stateRoot), "visits.json")
}

// Visit records that this human is looking, and answers the window the page
// reads "what changed" over.
//
// The three answers are the rule above: a read more than visitGap after the
// last one begins a new visit and compares against the end of the last one; a
// read inside a visit compares against whatever that visit already compares
// against; and a human this file has never seen is a first visit, which
// compares against a day back and says so.
//
// The error is returned BESIDE a usable window, never instead of one. Losing
// or failing to write this file changes the comparison window and nothing
// else, so a caller that cannot record the visit still has a page to render;
// the error is there so it can say so rather than pretend it wrote.
func Visit(root, human string, now time.Time) (since time.Time, first bool, err error) {
	visiting.Lock()
	defer visiting.Unlock()

	at := now.UTC()
	held, readErr := readVisits(root)
	row, known := held.Humans[human]
	next := advance(row, known, at)
	since, first = windowOf(next, at)

	held.SchemaVersion = visitsSchema
	if held.Humans == nil {
		held.Humans = map[string]Marker{}
	}
	held.Humans[human] = next
	if writeErr := writeVisits(root, held); writeErr != nil {
		return since, first, writeErr
	}
	return since, first, readErr
}

// advance is the visit this read leaves behind.
func advance(row Marker, known bool, now time.Time) Marker {
	stamped := stamp(now)
	seen, dated := instant(row.Seen)
	if !known || !dated {
		// Nobody has been seen here, or the row says nothing this build can
		// read. Either way this read is the beginning of a visit with no
		// visit before it.
		return Marker{Began: stamped, Seen: stamped}
	}
	if now.Sub(seen) > visitGap {
		// The gap ends the visit that was in progress. What it ended at is
		// the last read, not now: nobody was looking in between.
		return Marker{Began: stamped, Seen: stamped, Previous: row.Seen}
	}
	// Still looking. Only the last-seen instant moves, so the window this
	// visit compares against stays exactly where it was.
	began := row.Began
	if began == "" {
		began = stamped
	}
	return Marker{Began: began, Seen: stamped, Previous: row.Previous}
}

// windowOf is what a visit compares against: the end of the visit before it,
// or a day back from the moment this one began.
//
// The day is measured from the visit's own beginning rather than from now, so
// that a human who reads the page again ten minutes into their first visit is
// shown the same window they were shown at the start of it.
func windowOf(visit Marker, now time.Time) (time.Time, bool) {
	if previous, dated := instant(visit.Previous); dated {
		return previous, false
	}
	if began, dated := instant(visit.Began); dated {
		return began.Add(-firstWindow), true
	}
	return now.Add(-firstWindow), true
}

// readVisits is the file, or an empty set of visits where there is none.
//
// A file that cannot be read or parsed, and a file written under a schema this
// build does not know, are all "no visits recorded": the rows are preference
// state, the worst case is one wide window on one read, and refusing to render
// the page over them would be the only outcome a human could not work around.
// The reason is returned all the same, so the caller can say the marker was
// not read rather than silently claim a first visit.
func readVisits(root string) (Visits, error) {
	data, err := os.ReadFile(VisitsPath(root))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Visits{SchemaVersion: visitsSchema, Humans: map[string]Marker{}}, nil
		}
		return Visits{Humans: map[string]Marker{}}, err
	}
	var held Visits
	if err := json.Unmarshal(data, &held); err != nil {
		return Visits{Humans: map[string]Marker{}}, err
	}
	if held.SchemaVersion != visitsSchema {
		return Visits{Humans: map[string]Marker{}},
			errors.New(VisitsPath(root) + " is written under a schema this build does not read")
	}
	if held.Humans == nil {
		held.Humans = map[string]Marker{}
	}
	return held, nil
}

// writeVisits replaces the file atomically, so a reader never sees half a set
// of rows and a crash leaves the previous file rather than a truncated one.
func writeVisits(root string, held Visits) error {
	if err := os.MkdirAll(interfaceDir(root), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(held)
	if err != nil {
		return err
	}
	// Durability is not asserted. A marker whose last write did not reach the
	// platter costs one wide window on the next read, which is the same cost
	// as losing the file, and paying for a full flush on every page load to
	// avoid it would be the more expensive mistake.
	_, err = atomicfile.WriteText(VisitsPath(root), string(data)+"\n", root)
	return err
}
