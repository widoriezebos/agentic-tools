// Package stickies owns one human's notepad: the reminders they jot while
// they work, kept per human, outside every checkout, and never a record.
//
// # Why it is not in the state root
//
// The visit marker beside it keeps preference state in the checkout's state
// root, which on both layouts lies inside the checkout. That is right for a
// marker and wrong for a note: the Project Partner's permission owner grants
// native reads anywhere inside the checkout, and a critic is handed the
// repository as a read root, so a note written there would be readable by a
// seat from the moment it was written. A sticky is the human's own working
// material — "ask Sol about the retry", "the launch sheet's wording is off" —
// and the master keeps personal working material out of the project's records
// until the working-material owner exists.
//
// So the file lives under the account's registry home: the one directory the
// kit already resolves that is outside every checkout. Neither grant reaches
// it, and the build proves both by reading them (see stickies_grants_test.go)
// rather than by asserting it in a comment.
//
// # What it is
//
// One file per workspace, `stickies.json`, keyed inside by human, rewritten
// whole under one mutex through the kit's atomic writer. One server owns one
// registry home for its whole life, so a mutex is the whole of the exclusion
// needed; the atomic replace is what keeps a reader from seeing half a file
// and a crash from leaving a truncated one.
//
// Every act answers the WHOLE list, because every act changes what the panel
// shows: a create that answered only the created sticky would leave the page
// to guess at the order and the counts, and two tabs would drift apart.
package stickies

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/project"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/registry"
)

// SchemaVersion is the shape this build writes and reads.
const SchemaVersion = 1

// The two kinds a sticky can be about. A goal is a ledger id; a record is a
// document path as the reader names it. Anything else is refused, because a
// kind this build does not know is a chip the page could not render and a
// link it could not follow.
const (
	KindGoal   = "goal"
	KindRecord = "record"
)

// The two bounds. They are here rather than in the route because they are
// facts about the notepad and not about HTTP: the same bounds hold whoever
// asks.
const (
	// MaxText is the longest one sticky runs. A reminder is a line or a
	// paragraph; past this it is a document, and the checkout is where
	// documents go.
	MaxText = 2000
	// MaxStickies is how many one human keeps. It is a bound on a notepad,
	// not on a filing system: past five hundred the panel is no longer
	// something a human reads at a glance, which is the whole of what it is
	// for.
	MaxStickies = 500
)

// Sticky is one note: what it says, what it is about, and its three instants.
//
// DoneAt is the empty string while a sticky is open, rather than a pointer or
// an omitted field, so that "open" is one comparison everywhere — in the
// order, in the counts, in the payload the page reads — and there is no third
// state for a reader to get wrong.
type Sticky struct {
	ID    string  `json:"id"`
	Text  string  `json:"text"`
	About []About `json:"about"`
	// CreatedAt is what open stickies are ordered by, UpdatedAt what the card
	// says was last touched, DoneAt what done stickies are ordered by.
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
	DoneAt    string `json:"doneAt"`
}

// About is one thing a sticky is about: which kind, and which one.
type About struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

// Counts is what the header's badge and the panel's disclosure read.
type Counts struct {
	Open int `json:"open"`
	Done int `json:"done"`
}

// List is the whole answer every route gives: who it is for, every sticky in
// the order the panel shows them, and the two counts.
type List struct {
	SchemaVersion int      `json:"schemaVersion"`
	Human         string   `json:"human"`
	Stickies      []Sticky `json:"stickies"`
	Counts        Counts   `json:"counts"`
}

// stored is the file: the schema, and one row list per human.
//
// Keyed by human rather than one file per human, because a human is a handle
// and a handle is not a safe file name; and because the whole file is rewritten
// under one lock anyway, so one file is one write rather than a directory to
// keep consistent. A handle that names nobody — the seat itself — is the row
// under the empty key, exactly as the visit marker's is.
type stored struct {
	SchemaVersion int                 `json:"schemaVersion"`
	Humans        map[string][]Sticky `json:"humans"`
}

// RefusalKind is why an act was refused, in the shapes a caller can act on.
type RefusalKind string

const (
	// RefusalBad is a request that says something this notepad cannot mean:
	// no text at all, an about of an unknown kind, an about naming nothing.
	RefusalBad RefusalKind = "bad"
	// RefusalAbsent is an id this human has no sticky under.
	RefusalAbsent RefusalKind = "absent"
	// RefusalBounds is a well-formed act past one of the two bounds.
	RefusalBounds RefusalKind = "bounds"
)

// Refusal is what this package says no with. The message is the whole of what
// a human is shown, so it is written as a sentence to a person rather than as
// a code to a machine.
type Refusal struct {
	Kind    RefusalKind
	Message string
}

func (r *Refusal) Error() string { return r.Message }

func refuse(kind RefusalKind, message string) *Refusal {
	return &Refusal{Kind: kind, Message: message}
}

// Home is the account's registry home: the directory the kit already resolves
// for seat registration, outside every checkout.
//
// It is derived from the registry's own selected path rather than resolved a
// second time here, so that the fixture seam the registry offers — one
// environment variable naming a run-scoped home — moves this file too. A test
// that redirected one and not the other would be a test writing into the real
// account's home, which is the one thing a test of this file must never do.
func Home() (string, error) {
	selected, err := registry.DefaultPath()
	if err != nil {
		return "", err
	}
	return filepath.Dir(selected), nil
}

// Path is the file one workspace's stickies live in.
//
// The directory is keyed by the workspace rather than named after it: a
// checkout path is not a path segment, two checkouts can share a last name,
// and a name derived by hand would collide or escape. So the key is the
// checkout's own last name, which is what a human recognises, with a short
// digest of the whole absolute path after it, which is what makes it that
// checkout and no other.
func Path(home, checkout string) string {
	return filepath.Join(home, "ui", "stickies", workspaceKey(checkout), "stickies.json")
}

// workspaceKey is the one directory segment a checkout is kept under.
func workspaceKey(checkout string) string {
	resolved := checkout
	if absolute, err := filepath.Abs(checkout); err == nil {
		resolved = absolute
	}
	resolved = filepath.Clean(resolved)
	digest := sha256.Sum256([]byte(resolved))
	name := filepath.Base(resolved)
	// A base that is not a name at all — the root, a relative dot — leaves the
	// digest to say which workspace this is on its own.
	if name == "." || name == string(filepath.Separator) || name == ".." {
		return hex.EncodeToString(digest[:6])
	}
	return safeName(name) + "-" + hex.EncodeToString(digest[:6])
}

// safeName is a checkout's last name with everything that is not a plain
// letter, digit, dot, dash or underscore replaced, so the segment is a name on
// every filesystem this runs on.
func safeName(name string) string {
	var built strings.Builder
	for _, letter := range name {
		switch {
		case letter >= 'a' && letter <= 'z',
			letter >= 'A' && letter <= 'Z',
			letter >= '0' && letter <= '9',
			letter == '.' || letter == '-' || letter == '_':
			built.WriteRune(letter)
		default:
			built.WriteByte('-')
		}
	}
	return built.String()
}

// Store is one workspace's notepad.
type Store struct {
	path string
	// anchor is the directory atomicfile makes durable up to: the one above
	// the registry home, which pre-exists every run, because everything below
	// it this store may create itself.
	anchor string
	// now and mint are the clock and the id source, taken here so a test can
	// say when a sticky was made and what it was called without a wall-clock
	// wait and without a seed flag.
	now  func() time.Time
	mint func() (string, error)
	// writing serializes this process's rewrites of the file, as the visit
	// marker's does and for the same reason: one server owns one home for its
	// whole life.
	writing sync.Mutex
}

// New is the notepad of one workspace under one registry home.
func New(home, checkout string, now func() time.Time) *Store {
	if now == nil {
		now = time.Now
	}
	return &Store{
		path:   Path(home, checkout),
		anchor: filepath.Dir(home),
		now:    now,
		mint:   project.NewID,
	}
}

// File is where this store writes, which the fixture prints and the tests read.
func (s *Store) File() string { return s.path }

// List is this human's notepad as it stands.
func (s *Store) List(human string) (List, error) {
	s.writing.Lock()
	defer s.writing.Unlock()
	held, err := s.read()
	if err != nil {
		return List{}, err
	}
	return answer(human, held.Humans[human]), nil
}

// Add writes one new sticky and answers the whole list.
func (s *Store) Add(human, text string, about []About) (List, error) {
	return s.change(human, func(rows []Sticky) ([]Sticky, error) {
		body, refusal := admissibleText(text)
		if refusal != nil {
			return nil, refusal
		}
		named, refusal := admissibleAbout(about)
		if refusal != nil {
			return nil, refusal
		}
		if len(rows) >= MaxStickies {
			return nil, refuse(RefusalBounds, fmt.Sprintf(
				"this notepad holds %d stickies, which is as many as it keeps; remove one before writing another",
				MaxStickies))
		}
		id, err := s.mint()
		if err != nil {
			return nil, err
		}
		stamped := s.stamp()
		return append(rows, Sticky{
			ID: id, Text: body, About: named,
			CreatedAt: stamped, UpdatedAt: stamped,
		}), nil
	})
}

// Edit changes one sticky and answers the whole list. A field nobody sent is
// a field nobody changed: the three pointers are how "leave the text alone"
// and "make the text empty" stay two different requests.
func (s *Store) Edit(human, id string, text *string, about *[]About, done *bool) (List, error) {
	return s.change(human, func(rows []Sticky) ([]Sticky, error) {
		at := indexOf(rows, id)
		if at < 0 {
			return nil, absent(id)
		}
		row := rows[at]
		if text != nil {
			body, refusal := admissibleText(*text)
			if refusal != nil {
				return nil, refusal
			}
			row.Text = body
		}
		if about != nil {
			named, refusal := admissibleAbout(*about)
			if refusal != nil {
				return nil, refusal
			}
			row.About = named
		}
		stamped := s.stamp()
		if done != nil {
			// Marking a sticky done that is already done leaves the instant it
			// was struck off where it was: the human struck it off then, not
			// now, and moving it would reorder the done list under them.
			if *done && row.DoneAt == "" {
				row.DoneAt = stamped
			}
			if !*done {
				row.DoneAt = ""
			}
		}
		row.UpdatedAt = stamped
		rows[at] = row
		return rows, nil
	})
}

// Remove drops one sticky and answers the whole list.
func (s *Store) Remove(human, id string) (List, error) {
	return s.change(human, func(rows []Sticky) ([]Sticky, error) {
		at := indexOf(rows, id)
		if at < 0 {
			return nil, absent(id)
		}
		return append(rows[:at:at], rows[at+1:]...), nil
	})
}

func absent(id string) *Refusal {
	return refuse(RefusalAbsent, "no sticky of yours is called "+id)
}

// change is the whole of one act: read, apply, write, answer. Every act goes
// through it, so there is one lock, one read, one write and one answer rather
// than four spellings of them.
func (s *Store) change(human string, apply func([]Sticky) ([]Sticky, error)) (List, error) {
	s.writing.Lock()
	defer s.writing.Unlock()
	held, err := s.read()
	if err != nil {
		return List{}, err
	}
	rows, err := apply(append([]Sticky(nil), held.Humans[human]...))
	if err != nil {
		return List{}, err
	}
	if held.Humans == nil {
		held.Humans = map[string][]Sticky{}
	}
	held.Humans[human] = rows
	held.SchemaVersion = SchemaVersion
	if err := s.write(held); err != nil {
		return List{}, err
	}
	return answer(human, rows), nil
}

func (s *Store) stamp() string { return s.now().UTC().Format(time.RFC3339) }

// admissibleText is the text a sticky may carry, or why it may not.
func admissibleText(text string) (string, *Refusal) {
	body := strings.TrimSpace(text)
	if body == "" {
		return "", refuse(RefusalBad, "a sticky says something; this one is empty")
	}
	// The bound is on characters rather than bytes, because 2000 is what the
	// human was told and a human counts letters.
	if length := len([]rune(body)); length > MaxText {
		return "", refuse(RefusalBounds, fmt.Sprintf(
			"a sticky carries at most %d characters; this one carries %d", MaxText, length))
	}
	return body, nil
}

// admissibleAbout is what a sticky may be about, or why it may not.
func admissibleAbout(about []About) ([]About, *Refusal) {
	named := make([]About, 0, len(about))
	seen := map[About]bool{}
	for _, one := range about {
		kind := strings.TrimSpace(one.Kind)
		id := strings.TrimSpace(one.ID)
		if kind != KindGoal && kind != KindRecord {
			return nil, refuse(RefusalBad,
				"a sticky is about a goal or a record, not a "+quoteOrNothing(one.Kind))
		}
		if id == "" {
			return nil, refuse(RefusalBad, "a sticky about a "+kind+" says which one")
		}
		one = About{Kind: kind, ID: id}
		// The same thing named twice is one chip, not two: a human who pressed
		// the page's chip and then picked the same goal meant one of them.
		if seen[one] {
			continue
		}
		seen[one] = true
		named = append(named, one)
	}
	return named, nil
}

func quoteOrNothing(kind string) string {
	if strings.TrimSpace(kind) == "" {
		return "thing with no kind at all"
	}
	return `"` + kind + `"`
}

func indexOf(rows []Sticky, id string) int {
	for at, row := range rows {
		if row.ID == id {
			return at
		}
	}
	return -1
}

// answer is the order the panel shows and the counts the badge reads: open
// first, newest first; then done, most recently struck off first.
//
// The order is decided here rather than in the browser so that the panel, the
// goal page's block and the Partner's capture cannot come to three different
// opinions about what "first" means.
func answer(human string, rows []Sticky) List {
	open := make([]Sticky, 0, len(rows))
	done := make([]Sticky, 0, len(rows))
	for _, row := range rows {
		if row.DoneAt == "" {
			open = append(open, row)
			continue
		}
		done = append(done, row)
	}
	// Ties are broken by id, which is a ULID and so sorts in the order the
	// stickies were minted: two written in the same second still have one
	// order, and it is the same order on every read.
	sort.SliceStable(open, func(a, b int) bool { return later(open[a].CreatedAt, open[a].ID, open[b].CreatedAt, open[b].ID) })
	sort.SliceStable(done, func(a, b int) bool { return later(done[a].DoneAt, done[a].ID, done[b].DoneAt, done[b].ID) })
	list := List{
		SchemaVersion: SchemaVersion,
		Human:         human,
		Stickies:      append(open, done...),
		Counts:        Counts{Open: len(open), Done: len(done)},
	}
	if list.Stickies == nil {
		list.Stickies = []Sticky{}
	}
	return list
}

func later(leftAt, leftID, rightAt, rightID string) bool {
	if leftAt != rightAt {
		return leftAt > rightAt
	}
	return leftID > rightID
}

// read is the file, or an empty notepad where there is none.
//
// A missing file is no stickies, which is what a notepad nobody has written in
// is. Everything else is an error and is said: these are a human's own notes,
// and a build that answered "you have none" over a file it could not parse
// would be a build that quietly lost them.
func (s *Store) read() (stored, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return stored{SchemaVersion: SchemaVersion, Humans: map[string][]Sticky{}}, nil
		}
		return stored{}, err
	}
	var held stored
	if err := json.Unmarshal(data, &held); err != nil {
		return stored{}, fmt.Errorf("%s could not be read: %w", s.path, err)
	}
	if held.SchemaVersion != SchemaVersion {
		return stored{}, errors.New(s.path + " is written under a schema this build does not read")
	}
	if held.Humans == nil {
		held.Humans = map[string][]Sticky{}
	}
	return held, nil
}

// write replaces the file atomically, up the directory chain to the one
// directory above the registry home. Durability is asserted here, unlike the
// visit marker's: losing a marker costs one wide window, and losing this costs
// a human the reminders they wrote down.
func (s *Store) write(held stored) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(held)
	if err != nil {
		return err
	}
	if _, err := atomicfile.WriteText(s.path, string(data)+"\n", s.anchor); err != nil {
		return err
	}
	return nil
}
