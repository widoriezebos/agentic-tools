package partner

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/uihome"
)

// The conversation: one per human per checkout, owned here and nowhere else.
//
// Astra's sixth finding is what this file answers: a transcript of unspecified
// messages cannot tell a page whether a question was accepted, whether an
// answer is complete, or whether Retry would submit it twice. So one owner
// decides turn identity, admission, busy state, partial text and the terminal
// outcome, and the page reads all five from it.
//
// It is kept as one JSON line per message, in this account's own directory for
// this workspace, beside a small state file that names the live session and the
// sitting. A line is appended and never rewritten, so a reader that is halfway
// through the file reads a prefix of the truth rather than a torn record.
//
// # Why it is not in the state root
//
// It was, under `artifacts/agents/ui/partner`, and that was Astra's F2 on
// g1-s53: the state root lies inside the checkout on both layouts, the
// Partner's own permission owner grants native reads anywhere inside the
// checkout, and a critic is handed the repository as a read root. The master
// says the transcript is private sitting material in a protected server-local
// store outside the checkout, which examiners never read — and the first
// sitting creates exactly the material an examiner must not read. File mode
// 0600 keeps out other accounts, not a worker running as this one.
//
// So the conversation lives under the account's registry home, in the notepad's
// manner and through the same owner (internal/ui/uihome), and grants_test.go
// proves neither grant reaches it by reading the grants themselves. The formats
// are unchanged: the same JSON line per message, the same small state file
// beside it.
//
// A conversation written before that move is carried into the new place by Carry
// below, on the first open of a workspace's directory. Moving where a store is
// read from without moving what is already in it would have left a human with an
// empty history and the old files still inside the checkout, which is the whole
// of what the move was for.

// Owner is this store's own directory under the account's registry home. It is
// the interface's agent and not the steward's, which is what the name says.
const Owner = "partner"

// Home is the account's registry home, refused where it cannot be resolved as
// an absolute path. A seat whose home cannot be read keeps no conversation at
// all rather than writing one into the checkout it serves.
func Home() (string, error) { return uihome.Home() }

// Directory is where one workspace's conversations live under that home.
func Directory(home, checkout string) string {
	return uihome.Under(home, Owner, checkout)
}

// LegacyRelative is where a checkout kept its Partner conversations before the
// store moved out of it: under the state root, in the agents' own artifacts
// directory, one directory deeper. Carry is the only thing that reads it.
const LegacyRelative = "artifacts/agents/ui/partner"

// Carry brings a conversation written before the store moved out of the checkout
// into the directory the store now keeps it in.
//
// Sol's second finding: the move (D11) changed where the conversation is written
// and read, and nothing brought the one already written. A human who had talked
// to the Partner opened the new build and found an empty history — while the old
// files stayed inside the checkout, which is the one place this move exists to
// keep them out of. An empty transcript is not a smaller problem than a refusal:
// it is the same private material, still readable by a critic, with nothing on
// screen to say so.
//
// What it does, and when:
//
//   - Nothing at all where the old place is absent. On a machine where the files
//     were already carried by hand this is the whole of it, and it must cost
//     nothing to say so.
//   - Nothing where this workspace's new directory already holds a conversation
//     file. The carry is for the FIRST open of that directory; a directory that
//     already holds a transcript, a state file or a wire journal is a store in
//     use, and a carry into it would be this function deciding which of two
//     conversations is the real one.
//   - Otherwise every conversation file of the old place moves — a move, so
//     nothing private is left behind in the checkout — and a name the new place
//     has already taken refuses the whole carry before anything is moved, rather
//     than being written over.
//
// It answers how many files it moved, and an error the caller reports as the
// reason the Partner is not served. Serving an empty history instead would be
// the finding again in a quieter form.
func Carry(directory, was string) (int, error) {
	old, err := os.ReadDir(was)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return 0, nil
		}
		return 0, fmt.Errorf("the Partner's earlier conversation at %s could not be read: %w", was, err)
	}
	taking := []string{}
	for _, entry := range old {
		if entry.IsDir() || !conversationFile(entry.Name()) {
			continue
		}
		taking = append(taking, entry.Name())
	}
	if len(taking) == 0 {
		return 0, nil
	}
	sort.Strings(taking)

	held, err := os.ReadDir(directory)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return 0, fmt.Errorf("the Partner's conversation directory at %s could not be read: %w", directory, err)
	}
	for _, entry := range held {
		if !entry.IsDir() && conversationFile(entry.Name()) {
			return 0, nil
		}
	}
	// Every name is checked before anything moves: a carry that refused halfway
	// would leave one conversation in two places, which is worse than not having
	// started. The names that reach this are names no conversation file of the
	// new place holds, so what takes one is something else standing in the way.
	for _, name := range taking {
		if _, err := os.Lstat(filepath.Join(directory, name)); err == nil {
			return 0, fmt.Errorf(
				"the Partner's earlier conversation cannot be carried out of the checkout: %s already exists at %s, and this will not write over it",
				name, directory)
		} else if !errors.Is(err, fs.ErrNotExist) {
			return 0, fmt.Errorf("%s could not be looked for at %s: %w", name, directory, err)
		}
	}
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return 0, fmt.Errorf("the Partner's conversation directory could not be made: %w", err)
	}
	moved := 0
	for _, name := range taking {
		if err := carryOne(filepath.Join(was, name), filepath.Join(directory, name)); err != nil {
			return moved, err
		}
		moved++
	}
	// The old directory goes where it is empty, so the checkout is left without
	// even the shape of a store it no longer holds. Anything else in it — a file
	// a newer build wrote, a directory — keeps it, and that is not a failure.
	_ = os.Remove(was)
	return moved, nil
}

// conversationFile reports whether one name in a conversation directory is part
// of the conversation: a human's transcript or state file, the unnamed seat's
// pair, or the wire journal.
//
// It is recognised by the extension rather than by a list of names, because two
// of the four carry a human's own handle and this runs before any human has
// asked this server for anything.
func conversationFile(name string) bool {
	switch filepath.Ext(name) {
	case ".json", ".jsonl":
		return true
	}
	return false
}

// carryOne moves one file, and never over another.
//
// A link and an unlink rather than a rename: a rename writes over whatever is at
// the name it is given, and this is carrying private material into a directory
// this process does not own alone. The two places can be on different
// filesystems — the checkout is anywhere and the account's home is under the
// account — so a link that cannot cross falls back to a copy that refuses to
// create a file that exists, and the original goes only once the copy is whole.
func carryOne(from, to string) error {
	if err := os.Link(from, to); err == nil {
		if err := os.Remove(from); err != nil {
			// The copy is whole but the original is still in the checkout, which
			// is the one thing this must not leave behind. So the copy goes and
			// the next open tries again.
			_ = os.Remove(to)
			return fmt.Errorf("%s could not be removed from the checkout after being carried out of it: %w", from, err)
		}
		return nil
	}
	if err := copyOne(from, to); err != nil {
		return err
	}
	if err := os.Remove(from); err != nil {
		_ = os.Remove(to)
		return fmt.Errorf("%s could not be removed from the checkout after being carried out of it: %w", from, err)
	}
	return nil
}

// copyOne writes one file into a name nothing holds, at the mode this store
// keeps: one account's own, and no other's.
func copyOne(from, to string) error {
	source, err := os.Open(from)
	if err != nil {
		return fmt.Errorf("%s could not be read: %w", from, err)
	}
	defer func() { _ = source.Close() }()
	target, err := os.OpenFile(to, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("%s could not be written: %w", to, err)
	}
	if _, err := io.Copy(target, source); err != nil {
		_ = target.Close()
		_ = os.Remove(to)
		return fmt.Errorf("%s could not be carried to %s: %w", from, to, err)
	}
	if err := target.Close(); err != nil {
		_ = os.Remove(to)
		return fmt.Errorf("%s could not be written: %w", to, err)
	}
	return nil
}

// The two roles a message can have.
const (
	RoleHuman   = "human"
	RolePartner = "partner"
)

// Page is the capture: what the page was showing at the moment a question was
// sent, as the page itself knows it.
//
// One capture answers four things that used to be answered separately and
// could therefore disagree: what the sheet shows before the question goes,
// what the question carries, what the message keeps, and where the message's
// own chip takes a human back to weeks later. The page composes it once, when
// Send is pressed; navigating afterwards cannot retarget a question already
// sent, and nothing recomposes it.
type Page struct {
	Section string `json:"section"`
	Path    string `json:"path"`
	Tab     string `json:"tab,omitempty"`
	// View is which reading of the section was open, where the section has
	// more than one: the board or the list.
	View string `json:"view,omitempty"`
	// Tip and ObservedAt are the reading the page rendered from. They are the
	// page's, not this server's: the server composes from its own reading a
	// moment later, and where the two differ the block and the stamp say both
	// rather than certifying the newer one as what the human saw.
	Tip        string `json:"tip,omitempty"`
	ObservedAt string `json:"observedAt,omitempty"`
	// Window is how far back the Done lane reached, as the page spells it.
	Window string `json:"window,omitempty"`
	// Subject is the identity of what the page is about: a goal id, or a
	// document id. Kind says which.
	Kind    string `json:"kind,omitempty"`
	Subject string `json:"subject,omitempty"`
	Title   string `json:"title,omitempty"`
	// Revision is the revision the page displayed: a document's revision, or
	// empty where the subject's revision is the ledger's accepted tip, which
	// the server reads for itself.
	Revision string `json:"revision,omitempty"`
	// Filters is what the page was narrowed to, as the page spells it.
	Filters []string `json:"filters,omitempty"`
	// Lanes is the board as the page is showing it: one entry per lane on
	// screen, in the board's own order, with the goals the page's filters and
	// its Done window left in it. Only the page knows this — the filters, the
	// ordering and the window are the browser's — and only the server knows
	// what each of those goals is, so the page names them and the server
	// reads them.
	Lanes []Lane `json:"lanes,omitempty"`
	// Records is what the open project tab is listing, by the key the page
	// lists it under: a record's checkout-relative path, or a question's id.
	Records []string `json:"records,omitempty"`
	// Fleet is the fleet as the page was showing it: the machines on screen
	// with their standings and flags, and where the presence copy came from.
	// It travels for the board's reason — only the page knows what was on
	// screen — and it is bounded, because a fleet is a list and a capture is
	// not a listing tool.
	Fleet *FleetCapture `json:"fleet,omitempty"`
	// Label is the line the drawer showed, which is what the human read.
	Label string `json:"label,omitempty"`
	// Quote is the passage a human selected, whole, with where it came from:
	// the document's id and the revision it was read at, or the page. Only
	// saved text travels; an unsaved edit never does.
	Quote         string `json:"quote,omitempty"`
	QuoteFrom     string `json:"quoteFrom,omitempty"`
	QuoteRevision string `json:"quoteRevision,omitempty"`
	// QuoteAnchor is the heading the passage sits under, which is what a
	// message chip returns to when the passage itself has moved.
	QuoteAnchor string `json:"quoteAnchor,omitempty"`
	// Sheet is the sheet the human had open over the work area when they
	// asked, by the name on its head. It says where they were standing and
	// nothing more: a message chip returns to the page with the sheet named
	// and does not reopen it.
	Sheet string `json:"sheet,omitempty"`
	// Draft is a sheet's fields as the human had them at the moment they
	// offered it, and only because they offered it. It is context.go's, which
	// is where it is read into the block and where what it is read as — a
	// draft, not a record — is said. Nothing a human is still typing travels
	// without that act.
	Draft *Draft `json:"draft,omitempty"`
	// Return is the address this capture takes a human back to, composed by
	// the page that made it: the path with the view, filters, window, tab and
	// subject it was showing. The page owns its own address grammar, so the
	// server keeps the string rather than reassembling one.
	Return string `json:"return,omitempty"`
	// Stickies is the human's own notepad as the page was showing it: what the
	// panel showed while it stood open, and otherwise the stickies about the
	// thing on the page.
	//
	// Astra's F2 is why the second half is there. A note about no subject —
	// "the Fleet page's wording is off" — is on no page at all, so a capture
	// that only ever carried a page's own stickies would never carry it, and
	// J4's own example would fail. Opening the panel and asking is how a human
	// reaches every one of them.
	//
	// They travel because they are nowhere else: the notepad is outside every
	// checkout precisely so that no seat reads it, so a Partner that was not
	// told them could not find them and must not try.
	Stickies []Sticky `json:"stickies,omitempty"`
	// StickiesOpen is how many are open, whether or not any of them travel. It
	// is always carried, so "you have four open stickies, none about this
	// page" is an answer the Partner can give.
	StickiesOpen int `json:"stickiesOpen,omitempty"`
	// StickiesCut is how many the page was showing that this capture does not
	// carry, because it arrived past the bound Bound holds it to. It is
	// counted rather than dropped in silence, so the block says what is
	// missing instead of offering a truncated notepad as a whole one.
	StickiesCut int `json:"stickiesCut,omitempty"`
}

// Bound holds a capture to what this server will carry and keep.
//
// The stickies are the one part of a capture that is read from nowhere else.
// The notepad lives outside every checkout precisely so no seat reaches it, so
// what the browser sends is all there is — and what the browser sends is
// written down: a turn's message keeps the page it was asked from, in this
// checkout's state root, for as long as the conversation lasts. A capture that
// arrived carrying a whole notepad would therefore put a whole notepad of
// private reminders inside the checkout the notepad is kept out of, and the
// block's own bound — applied later, over what is already stored — would not
// have stopped it.
//
// The page caps what it sends to the same number. This is the boundary that
// does not take the page's word for it, and it is the same number on purpose:
// two bounds that could differ are two bounds that eventually do.
//
// It truncates rather than refuses. The question is the human's, and losing it
// because their notepad is long is the wrong half to throw away; what was cut
// is counted, and the block says so in the line it already has for the bound.
func (p Page) Bound() Page {
	if over := len(p.Stickies) - maxStickiesCarried; over > 0 {
		kept := make([]Sticky, maxStickiesCarried)
		copy(kept, p.Stickies)
		p.Stickies = kept
		p.StickiesCut += over
	}
	return p
}

// Sticky is one of the human's own reminders as the page was showing it: what
// it says, and what it is about.
//
// It carries no instants and no id. A sticky in a capture is something the
// human wrote to themselves and is looking at; the Partner neither acts on one
// nor dates one, and a capture is not a copy of the notepad.
type Sticky struct {
	Text string `json:"text"`
	// About is what it is about, as the page spells it: "goal g1-s45", or a
	// document's own path.
	About []string `json:"about,omitempty"`
	// Done says this one has been struck off. The panel shows done stickies
	// behind a disclosure, so one only travels when a human had it open.
	Done bool `json:"done,omitempty"`
}

// FleetCapture is the Fleet page as it was on screen: where the presence copy
// came from, the machines shown, and the goals the page said need a human.
//
// It is the page's own reading and never a later one. The Partner may call
// the fleet tool afterwards and get a different answer, a minute newer; the
// block says which is which rather than certifying the newer one as what the
// human was looking at.
type FleetCapture struct {
	// Source and FetchedAt are the copy's provenance as the page showed it.
	Source    string `json:"source,omitempty"`
	FetchedAt string `json:"fetchedAt,omitempty"`
	// Problem is the copy's own trouble, in the page's words.
	Problem  string         `json:"problem,omitempty"`
	Machines []FleetMachine `json:"machines,omitempty"`
	NeedsYou []string       `json:"needsYou,omitempty"`
	Total    int            `json:"total,omitempty"`
	// Launches is the launch cards the page was showing, as it showed them.
	// The human's authorization is not among their fields and never travels:
	// the sheet's word is the one thing on the Fleet page this capture does
	// not carry.
	Launches []FleetLaunch `json:"launches,omitempty"`
}

// FleetLaunch is one launch card as it was displayed: what it is making, how
// it ended, and where it stopped, in the page's own words.
type FleetLaunch struct {
	Machine     string `json:"machine"`
	Outcome     string `json:"outcome"`
	Destination string `json:"destination,omitempty"`
	// Step is the step the card was showing as the current one, and Words the
	// owner's own sentence under a failed one.
	Step  string `json:"step,omitempty"`
	Words string `json:"words,omitempty"`
}

// FleetMachine is one row of that table as it was displayed.
type FleetMachine struct {
	Machine  string `json:"machine"`
	Standing string `json:"standing"`
	// Seen is the age words the row showed, in the browser's own clock.
	Seen string `json:"seen,omitempty"`
	// Phase is the sentence the Running column showed: what the machine is
	// doing, how long the job has run and the cap it reserved.
	Phase string `json:"phase,omitempty"`
	// Open says the human had this row's disclosure open. It is what they
	// were looking at, which is the whole of what this capture is for.
	Open  bool     `json:"open,omitempty"`
	Flag  string   `json:"flag,omitempty"`
	Holds []string `json:"holds,omitempty"`
}

// Lane is one column of the board as the page is showing it.
type Lane struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	// Total is how many goals the lane holds after the page's filters, which
	// is what its header counts; Goals names the first of them.
	Total int      `json:"total"`
	Goals []string `json:"goals,omitempty"`
}

// Message is one line of the transcript.
type Message struct {
	ID   string `json:"id"`
	Turn string `json:"turn"`
	Role string `json:"role"`
	Text string `json:"text"`
	At   string `json:"at"`
	// Outcome is complete, stopped, failed or refused, on a Partner's message
	// only. A human's message has none: it was accepted or it was not.
	Outcome string `json:"outcome,omitempty"`
	// Detail is the runtime's own words where the turn did not complete.
	Detail string `json:"detail,omitempty"`
	// Activity is what the Partner did on the way, including every refusal.
	Activity []string `json:"activity,omitempty"`
	// Looked is what this answer was read from, in the order it was read: the
	// page the human was looking at first, as its own entry, and then every
	// tool call with its completion. It is on a Partner's message only.
	Looked []Look `json:"looked,omitempty"`
	// Suggestions is what this answer offered the human for the fields of the
	// editor they handed over, in the order they were admitted. They are kept
	// with the answer rather than applied anywhere: the card a human presses Use
	// this on is rendered from here, and what a field holds is the human's.
	Suggestions []Suggestion `json:"suggestions,omitempty"`
	// Deposits is what this answer offered the sitting's record: facts with
	// their anchors, decisions with the reason the Partner heard, open
	// questions with their consequences. They are offers and nothing else —
	// Record it is the human's press — so they are kept with the answer for the
	// same reason the suggestions are.
	Deposits []Deposit `json:"deposits,omitempty"`
	// Proposals is the acts this answer proposed on goals, admitted and refused
	// alike, each with the state it now stands in.
	//
	// They are kept here rather than in a store of their own because this is the
	// record of what happened: an approve applied twice is two approval records,
	// so the one thing the page must never do is forget that a line was applied.
	// A reload reads this and shows applied as applied, and a line left at
	// `applying` as having been in flight. The state is written by the outcome
	// route, which rewrites this message in place and touches nothing else in
	// the transcript.
	Proposals []Proposal `json:"proposals,omitempty"`
	// Key is the client-minted turn key, on a human's message only. It is
	// what makes a retry after a lost answer the same turn rather than a
	// second one, and it is kept in the file so a restart cannot forget it.
	Key string `json:"key,omitempty"`
	// Interface marks a question this interface submitted on the human's
	// behalf rather than one they typed: the sitting's opening turn, and
	// nothing else today.
	//
	// It is on a human's message, and it is in the file. A provenance only the
	// live page knew would be a provenance a reload quietly turns into the
	// human's own words — and the one turn nobody typed is exactly the one a
	// human must be able to tell apart weeks later. The transcript renders it
	// from here, so persistence and replay carry it (g1-s53 D3).
	Interface bool `json:"interface,omitempty"`
	// Page is where the human was, on a human's message only.
	Page *Page `json:"page,omitempty"`
	// Trimmed marks the one line a trim leaves at the head of a transcript:
	// the note that says what is gone and when it went. It is in the file
	// because a later trim has to recognise it — a note about messages that
	// are gone is itself the first thing to cut, and without the mark the
	// head of the file would collect one note per sweep.
	Trimmed bool `json:"trimmed,omitempty"`
}

// Conversation is the transcript and the state beside it.
type Conversation struct {
	transcript string
	state      string
	// directory is the store this conversation was opened in, and key the
	// sitting's record it is kept under, or "" for the human's own
	// conversation. A sitting's conversation lives beside the human's, in the
	// same store (g1-s65 D16), and the service reaches the store through here.
	directory string
	key       string

	mu       sync.Mutex
	messages []Message
	session  string
	sitting  *Sitting
	// publish writes the state file; nil is publishState. A test sets it to
	// hold one write back while another runs.
	publish func(path string, held stateFile) error
}

// Subject is what a sitting is about: one record of this project, by the kind
// it is addressed under and its id.
//
// A record is addressed by its checkout-relative path, because that is what the
// document reader serves it under and what the edit route writes it by; the
// record's own ulid addresses its status and nothing here. So kind is "record"
// and id is that path (g1-s53 §5).
type Subject struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
	// Title is the record's own title as the page showed it, so the chip and
	// the table can name the subject without a second read.
	Title string `json:"title,omitempty"`
}

// Sitting is the working conversation a human opened on one record.
//
// It is not a second store and it holds no working material: the record is the
// memory. What a sitting is, here, is the mark that says this conversation is
// one — which record it is about, what it is for, and when it began — so that
// every turn carries it, a deposit can be admitted against it, and a human who
// closed the browser comes back to the same sitting (g1-s53 D1).
type Sitting struct {
	Subject Subject `json:"subject"`
	// Purpose is what this sitting is for, from the three this build admits.
	Purpose   string `json:"purpose"`
	StartedAt string `json:"startedAt"`
	// Room is the review room's working state, kept on the mark so a human who
	// steps out comes back to the same desk, the same face and the same
	// unfinished words (g1-s65 D9). It is private sitting material, like the
	// transcript beside it, and it never touches the record.
	Room *Room `json:"room,omitempty"`
}

// Room is what the review room keeps between visits: the desk's strip and the
// item on it, which face of the desk pane is up, and the human's unfinished
// words — every unrecorded card's edited text and clause, and the fields of an
// open Decide sheet, keyed by deposit id (Astra S65-03).
//
// The desk and the drafts are the page's own shapes, kept whole and bounded: the
// server stores them and reads nothing out of them, because what an item on a
// desk is belongs to the page that draws it.
type Room struct {
	Desk   json.RawMessage `json:"desk,omitempty"`
	Face   string          `json:"face,omitempty"`
	Drafts json.RawMessage `json:"drafts,omitempty"`
	// At is when the room was last kept, which is when the human last touched
	// it: the door line's "you stepped out 2h ago".
	At string `json:"at,omitempty"`
	// Seq is the page's number for this keep, rising with every keep it sends.
	// The mark takes only a keep numbered above the one it holds, so a request
	// that set out earlier and arrives later cannot put older words back.
	Seq int64 `json:"seq,omitempty"`
}

// maxRoomBytes bounds what one room keeps. A desk strip and a handful of
// unfinished cards are a few kilobytes; a room past this is a page sending
// something that is not a room.
const maxRoomBytes = 128 << 10

// The purposes this build admits. The learning sitting has moves of its own —
// the withheld diagnosis — and is not here, so it is not offered (g1-s53 §4).
// The review is the paper's review sitting (g1-s65): the human examines a
// goal's built work before it lands, with a colleague who was not in the room
// that shaped it.
const (
	PurposeShapeIntent = "shape intent"
	PurposeShapeDesign = "shape a design"
	PurposeReview      = "review"
)

// SubjectRecord is the one kind of subject a sitting has today.
const SubjectRecord = "record"

// stateFile is what the small file holds: the live session's id, so a build
// that learns to load sessions natively has it, the sitting this conversation
// is, and the moment it was written.
type stateFile struct {
	Human     string   `json:"human"`
	Session   string   `json:"session"`
	Sitting   *Sitting `json:"sitting,omitempty"`
	UpdatedAt string   `json:"updatedAt"`
}

// OpenConversation reads one human's conversation in one directory, creating
// the directory if it is not there. A file that cannot be read is a refusal
// rather than an empty conversation: a transcript that silently starts over is
// a transcript nobody can trust.
//
// The directory is the caller's: the server's is Directory(Home(), checkout),
// and the walkthrough's is one beside its own fixture checkout, so no fixture
// writes into a human's actual conversations.
//
// A sitting names its record as the key beside the human's (g1-s65 D16): the
// sitting's conversation is opened instead of the human's own, in the same
// store, under a file of its own.
func OpenConversation(directory, human string, sitting ...string) (*Conversation, error) {
	name := fileName(human)
	key := ""
	if len(sitting) > 0 {
		key = strings.TrimSpace(sitting[0])
	}
	transcript := filepath.Join(directory, name+".jsonl")
	state := filepath.Join(directory, name+".json")
	if key != "" {
		transcript, state = sittingFiles(directory, human, key)
	}
	if err := os.MkdirAll(filepath.Dir(transcript), 0o700); err != nil {
		return nil, fmt.Errorf("the Partner's conversation directory could not be made: %w", err)
	}
	conversation := &Conversation{transcript: transcript, state: state, directory: directory, key: key}
	if err := conversation.load(); err != nil {
		return nil, err
	}
	return conversation, nil
}

// sittingsDirectory is where a store keeps its sittings' conversations: one
// directory per human beneath it, so the human's own files stay what
// housekeeping and the carry recognise at the top of the store.
const sittingsDirectory = "sittings"

// sittingFiles is the transcript and the state file of one human's
// conversation about one record. The record's path is readable in the name and
// a digest of it follows, because a path flattened into a file name can meet
// another path flattened the same way, and two sittings must never share a file.
func sittingFiles(directory, human, record string) (string, string) {
	sum := sha256.Sum256([]byte(record))
	readable := fileName(record)
	if len(readable) > 80 {
		readable = readable[len(readable)-80:]
	}
	name := readable + "-" + hex.EncodeToString(sum[:8])
	under := filepath.Join(directory, sittingsDirectory, fileName(human))
	return filepath.Join(under, name+".jsonl"), filepath.Join(under, name+".json")
}

// fileName is the one name a human maps to. Anything that is not a plain
// letter, digit, dash or underscore becomes a dash, so a handle with a space
// or a slash in it names a file in this directory and never a path out of it.
func fileName(human string) string {
	human = strings.TrimSpace(human)
	if human == "" {
		return "seat"
	}
	var built strings.Builder
	for _, letter := range human {
		switch {
		case letter >= 'a' && letter <= 'z', letter >= 'A' && letter <= 'Z',
			letter >= '0' && letter <= '9', letter == '-', letter == '_':
			built.WriteRune(letter)
		default:
			built.WriteRune('-')
		}
	}
	name := built.String()
	if name == "" {
		return "seat"
	}
	return name
}

func (c *Conversation) load() error {
	file, err := os.Open(c.transcript)
	if err != nil {
		if os.IsNotExist(err) {
			return c.readState()
		}
		return fmt.Errorf("the Partner's transcript could not be read: %w", err)
	}
	defer file.Close()
	reader := bufio.NewScanner(file)
	reader.Buffer(make([]byte, 0, 64*1024), maxLineBytes)
	for reader.Scan() {
		line := strings.TrimSpace(reader.Text())
		if line == "" {
			continue
		}
		var message Message
		if err := json.Unmarshal([]byte(line), &message); err != nil {
			// A line a newer build wrote, or a line torn by a crash. Dropping
			// it costs one message; refusing the whole file would cost the
			// conversation.
			continue
		}
		c.messages = append(c.messages, message)
	}
	if err := reader.Err(); err != nil {
		if errors.Is(err, bufio.ErrTooLong) {
			// A line past the reader's bound used to fail the whole load, which
			// took the conversation page and the Decisions page down on every
			// restart (Astra B-01's residual). R-131-ui: the original is kept
			// whole, the transcript is rewritten to what fits, and the human is
			// told once.
			return c.archiveAndFit()
		}
		return fmt.Errorf("the Partner's transcript could not be read: %w", err)
	}
	return c.readState()
}

// archivedExtension is what an archived transcript is named with. It is neither
// .json nor .jsonl on purpose: conversationFile and Humans recognise a
// conversation by its extension, and an archive is a copy and not one.
const archivedExtension = ".archived"

// archiveAndFit is Wido's R-131-ui, and it runs only where the scan failed for
// a line past the reader's bound: "If the partner file is too large, we should
// archive the original and then just truncate so that it fits."
//
// A conversation the reader cannot load because of its size is not an error a
// human meets. So the original is kept whole beside it, the transcript is
// rewritten to the messages that fit, and the last message says that this
// happened. Summarizing what was cut is later.
//
// The archive is written BEFORE anything is rewritten, and its failure is
// returned: a truncation over an archive that was not written is the one way
// this loses a conversation instead of saving one.
//
// load has no clock — OpenConversation takes none — so the archive's stamp and
// the notice's are time.Now().UTC().
func (c *Conversation) archiveAndFit() error {
	now := time.Now().UTC()
	archive, err := archiveTranscript(c.transcript, now)
	if err != nil {
		return err
	}
	kept, setAside, err := messagesThatFit(c.transcript)
	if err != nil {
		return err
	}
	kept = append(kept, unreadableNotice(filepath.Base(archive), setAside, now))
	if err := writeTranscript(c.transcript, kept); err != nil {
		return err
	}
	// The cached messages are the rewritten file's, not the prefix the scanner
	// read before it stopped.
	c.messages = kept
	return c.readState()
}

// archiveTranscript keeps the original whole beside itself, under its own name
// with the moment it was archived: Wido.jsonl.20260928T143052Z.archived.
//
// It streams rather than reads the file into memory, because the file being too
// large to read is the whole reason it is here. And it never writes over
// anything: the name is opened O_EXCL, and a name something already holds takes
// the next distinct one, so a second archive in the same second is a second file
// rather than the first one lost.
func archiveTranscript(path string, now time.Time) (string, error) {
	source, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("the Partner's transcript could not be read: %w", err)
	}
	defer func() { _ = source.Close() }()
	stamp := now.UTC().Format("20060102T150405Z")
	for attempt := 0; attempt < 100; attempt++ {
		name := fmt.Sprintf("%s.%s%s", path, stamp, archivedExtension)
		if attempt > 0 {
			name = fmt.Sprintf("%s.%s-%d%s", path, stamp, attempt, archivedExtension)
		}
		target, err := os.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("the Partner's transcript could not be archived at %s: %w", name, err)
		}
		if _, err := io.Copy(target, source); err != nil {
			_ = target.Close()
			return "", fmt.Errorf("the Partner's transcript could not be archived at %s: %w", name, err)
		}
		if err := target.Close(); err != nil {
			return "", fmt.Errorf("the Partner's transcript could not be archived at %s: %w", name, err)
		}
		return name, nil
	}
	return "", fmt.Errorf("the Partner's transcript could not be archived beside %s: every name of this moment is taken", path)
}

// messagesThatFit reads the whole file again, keeping every message the reader's
// own bound admits and counting each line that passes it, in the file's order.
//
// A bufio.Scanner cannot do this: it stops at the FIRST line over its bound, so
// every message after the one oversized line would be lost — and those messages
// are exactly what this is for. So a bufio.Reader and ReadLine, which hands one
// line back in pieces: a line under the bound is accumulated, and a line that
// passes it is DRAINED rather than accumulated, so an enormous line costs this
// bounded memory and nothing more.
//
// The bound is the scanner's own, exactly. A Scanner can return a
// newline-terminated token of at most maxLineBytes - 1 bytes, because the
// delimiter has to fit in the same buffer; so a line of maxLineBytes or more is
// one the next load could not read, and keeping it would archive the file again
// on every open.
func messagesThatFit(path string) ([]Message, int, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, 0, fmt.Errorf("the Partner's transcript could not be read: %w", err)
	}
	defer func() { _ = file.Close() }()
	reader := bufio.NewReaderSize(file, 64*1024)
	kept := []Message{}
	setAside := 0
	for {
		line, over, err := readLineWithin(reader)
		if errors.Is(err, io.EOF) {
			return kept, setAside, nil
		}
		if err != nil {
			return nil, 0, fmt.Errorf("the Partner's transcript could not be read: %w", err)
		}
		if over {
			setAside++
			continue
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var message Message
		if err := json.Unmarshal([]byte(line), &message); err != nil {
			// Dropped exactly as load drops one, and for load's own reason: a
			// line this build cannot read costs one message, not the file.
			continue
		}
		kept = append(kept, message)
	}
}

// readLineWithin reads one line and says whether it passed the bound. A line
// under it comes back whole; one that passes it is read to its end a piece at a
// time and thrown away, so its size is never this process's.
func readLineWithin(reader *bufio.Reader) (string, bool, error) {
	var built strings.Builder
	over := false
	for {
		// ReadLine answers a line or an error and never both, so a piece here is
		// a piece of the line this call is reading.
		piece, more, err := reader.ReadLine()
		if err != nil {
			// A final line with no newline after it — the last thing a writer
			// that went away had written. ReadLine hands its pieces back and
			// reports the end of the file on the NEXT call, so what has been
			// accumulated is a whole line and is returned once before the EOF is
			// passed on; the call after this one reports it (Astra F-07). A line
			// past the bound accumulates nothing, so this is the fitting line
			// alone.
			if errors.Is(err, io.EOF) && built.Len() > 0 {
				return built.String(), false, nil
			}
			return "", false, err
		}
		if !over && built.Len()+len(piece) >= maxLineBytes {
			over = true
			built.Reset()
		}
		if !over {
			built.Write(piece)
		}
		if !more {
			return built.String(), over, nil
		}
	}
}

func (c *Conversation) readState() error {
	body, err := os.ReadFile(c.state)
	if err != nil {
		return nil
	}
	var held stateFile
	if err := json.Unmarshal(body, &held); err != nil {
		return nil
	}
	c.session = held.Session
	c.sitting = held.Sitting
	return nil
}

// maxMessageBytes bounds one stored message. A model that answers with a
// megabyte is a model whose answer is kept to the bound and said to be.
const maxMessageBytes = 256 << 10

// maxLineBytes is the longest line this store reads back, and it is the writer's
// own bound rather than a guess.
//
// It was maxMessageBytes plus a kilobyte, which is less than the text bound alone
// can cost: JSON spends six bytes on one `<` and on one control character, so an
// answer of those — or a plain answer carrying one proposal — wrote a line the
// reader then refused. And bufio refuses THE WHOLE FILE for one long line, which
// is the opposite of this file's own rule that a line it cannot read costs one
// message: the transcript failed to open on every restart afterwards, and the
// Decisions page, which reads the same transcript, failed with it (Astra B-01).
//
// So the bound is the text bound as JSON can spell it, six times over, and the
// rest is for what a message carries beside its text — its stamps, its activity,
// and its proposals, suggestions and deposits, every field of which is bounded
// where it is admitted.
const maxLineBytes = 8 * maxMessageBytes

// Append writes one message and keeps it.
func (c *Conversation) Append(message Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.appendHeld(message)
}

// appendHeld is Append with this conversation's own lock already held.
//
// It is apart for the one caller that has to decide WHICH conversation it is
// writing into while holding that lock: a turn's answer, whose conversation a
// first sign-in can move out from under it (Service.appendAnswer).
func (c *Conversation) appendHeld(message Message) error {
	if len(message.Text) > maxMessageBytes {
		message.Text = message.Text[:maxMessageBytes] + "\n\n[the rest of this answer was longer than the transcript keeps]"
	}
	body, err := json.Marshal(message)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(c.transcript, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("the Partner's transcript could not be written: %w", err)
	}
	defer file.Close()
	if _, err := file.Write(append(body, '\n')); err != nil {
		return fmt.Errorf("the Partner's transcript could not be written: %w", err)
	}
	c.messages = append(c.messages, message)
	return nil
}

// Messages answers the last n messages, oldest first. A limit of zero or less
// answers them all.
func (c *Conversation) Messages(limit int) []Message {
	c.mu.Lock()
	defer c.mu.Unlock()
	held := c.messages
	if limit > 0 && len(held) > limit {
		held = held[len(held)-limit:]
	}
	answer := make([]Message, len(held))
	copy(answer, held)
	return answer
}

// keptMessages is the floor a trim never cuts below, whatever the bounds say:
// the last two hundred messages of a conversation. It is why the bounds are
// retention targets and not disk ceilings — two hundred kept messages can
// exceed two megabytes on their own (g1-s54 D4).
const keptMessages = 200

// TrimBounds is what a transcript is kept to. Zero disables that bound, as
// every bound of this store does.
type TrimBounds struct {
	// Bytes is the size the file may reach before its head is cut.
	Bytes int64
	// Age is how old the oldest message may be.
	Age time.Duration
}

// Trim cuts this transcript's head where it has passed its bounds, and reports
// how many messages went.
//
// It is the conversation's OWN operation, under the mutex Append takes, and it
// replaces the file and the cached messages together. That is Astra's F2 on
// g1-s54: a read-trim-replace from outside could drop an append that had
// already been accepted, and a trim that only rewrote the file would leave the
// served transcript showing messages the file no longer holds.
//
// Three things bound what it may cut. The last two hundred messages always
// stay. The cut falls only where the turn changes, so a question is never left
// without its answer nor an answer without its question. And a trailing turn
// that has no terminal outcome yet is a turn still being answered, so it is
// kept whole even where that means cutting less than the bound asks — the
// bounds are retention targets, and a day passes between sweeps.
//
// What is cut is gone. The first remaining message says when it went.
func (c *Conversation) Trim(bounds TrimBounds, now time.Time) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.messages) <= keptMessages {
		return 0, nil
	}
	oversized := false
	if bounds.Bytes > 0 {
		info, err := os.Stat(c.transcript)
		if err != nil && !os.IsNotExist(err) {
			return 0, fmt.Errorf("the Partner's transcript could not be measured: %w", err)
		}
		oversized = err == nil && info.Size() > bounds.Bytes
	}
	stale := 0
	if bounds.Age > 0 {
		stale = olderThan(c.messages, now.UTC().Add(-bounds.Age))
	}
	if !oversized && stale == 0 {
		return 0, nil
	}
	ceiling := len(c.messages) - keptMessages
	if start, unfinished := trailingTurn(c.messages); unfinished && start < ceiling {
		ceiling = start
	}
	if ceiling <= 0 {
		return 0, nil
	}
	wanted := stale
	if oversized {
		// The file is over its size, and nothing smaller than the floor is
		// worth writing twice: the sweep cuts as far down as the floor and the
		// turn boundaries allow.
		wanted = ceiling
	}
	if wanted > ceiling {
		wanted = ceiling
	}
	cut := turnBoundary(c.messages, wanted, ceiling)
	if cut <= 0 || !holdsAMessage(c.messages[:cut]) {
		// A cut that would take nothing but an earlier trim's own notice is not
		// a trim: it would rewrite the file every sweep and date the notice
		// today for messages that went months ago. The floor is what stops the
		// bound here, and saying so once is enough.
		return 0, nil
	}
	kept := make([]Message, 0, len(c.messages)-cut+1)
	kept = append(kept, trimNotice(now))
	kept = append(kept, c.messages[cut:]...)
	if err := writeTranscript(c.transcript, kept); err != nil {
		return 0, err
	}
	c.messages = kept
	return cut, nil
}

// holdsAMessage reports whether a run of messages is more than trim notices.
func holdsAMessage(messages []Message) bool {
	for _, message := range messages {
		if !message.Trimmed {
			return true
		}
	}
	return false
}

// olderThan is how many messages at the head of a transcript are older than a
// moment, counting a trim notice as neither old nor a reason to stop: the note
// is about messages that are gone, so it goes with the next cut rather than
// standing at the head forever and stopping every age sweep after the first.
//
// A stamp this build cannot read stops the count. A message whose date is
// unreadable is a message whose age is unknown, and the unknown is kept.
func olderThan(messages []Message, cutoff time.Time) int {
	older := 0
	for at, message := range messages {
		if message.Trimmed {
			continue
		}
		stamp, err := time.Parse(time.RFC3339, message.At)
		if err != nil || !stamp.Before(cutoff) {
			break
		}
		older = at + 1
	}
	return older
}

// trailingTurn is where the last turn begins, and whether it is unfinished: a
// turn is finished when an answer of it carries a terminal outcome, and until
// then it is a question this server is still answering.
func trailingTurn(messages []Message) (int, bool) {
	if len(messages) == 0 {
		return 0, false
	}
	turn := messages[len(messages)-1].Turn
	start := len(messages) - 1
	for start > 0 && messages[start-1].Turn == turn {
		start--
	}
	for _, message := range messages[start:] {
		if message.Role == RolePartner && message.Outcome != "" {
			return start, false
		}
	}
	return start, true
}

// turnBoundary is the index a cut may fall on: the first one at or after wanted
// where the turn changes, and never past ceiling. Where the range holds no
// boundary, the last one below it is taken instead — half a turn is never cut
// to meet a retention target, and cutting less is the right half to give up.
func turnBoundary(messages []Message, wanted, ceiling int) int {
	if wanted < 1 {
		wanted = 1
	}
	for at := wanted; at <= ceiling; at++ {
		if messages[at].Turn != messages[at-1].Turn {
			return at
		}
	}
	below := 0
	for at := 1; at < wanted && at <= ceiling; at++ {
		if messages[at].Turn != messages[at-1].Turn {
			below = at
		}
	}
	return below
}

// trimNotice is the line a trimmed transcript keeps at its head. It is written
// in the Partner's column because those are the two roles a message has, and
// marked so the next trim cuts it with the rest.
func trimNotice(now time.Time) Message {
	stamp := now.UTC()
	return Message{
		ID:      mintTurn(),
		Turn:    mintTurn(),
		Role:    RolePartner,
		Outcome: OutcomeComplete,
		Text:    "Earlier messages were trimmed on " + stamp.Format("2 January 2006") + ".",
		At:      stamp.Format(time.RFC3339),
		Trimmed: true,
	}
}

// unreadableNotice is the line a rewritten transcript keeps, and it is at the
// END of the file rather than at the head: the drawer shows the newest message
// first and Messages(limit) answers the LAST n, so a notice at the head is one
// the human would not be shown at all (R-131-ui).
//
// It is shaped like trimNotice and marked as a trim's own for trimNotice's
// reason: what it says is that messages are gone, which is the first thing a
// later sweep should cut.
func unreadableNotice(archive string, setAside int, now time.Time) Message {
	stamp := now.UTC()
	gone := fmt.Sprintf("%d messages were set aside", setAside)
	if setAside == 1 {
		gone = "one message was set aside"
	}
	return Message{
		ID:      mintTurn(),
		Turn:    mintTurn(),
		Role:    RolePartner,
		Outcome: OutcomeComplete,
		Text: "This conversation was too large to open, so it was rewritten to what fits. " +
			"The original is kept whole beside it as " + archive + ", and " + gone + ".",
		At:      stamp.Format(time.RFC3339),
		Trimmed: true,
	}
}

// writeTranscript replaces the whole file, in one publication: a reader halfway
// through the old file reads the old file whole rather than a torn mixture, and
// the append that takes the lock next opens the file this left.
func writeTranscript(path string, messages []Message) error {
	var built strings.Builder
	for _, message := range messages {
		body, err := json.Marshal(message)
		if err != nil {
			return err
		}
		built.Write(body)
		built.WriteByte('\n')
	}
	if _, err := atomicfile.WriteText(path, built.String(), ""); err != nil {
		return fmt.Errorf("the Partner's trimmed transcript could not be written: %w", err)
	}
	return nil
}

// Humans names every human this workspace's directory holds a conversation for,
// by the name the file is kept under.
//
// Housekeeping needs it because a transcript that grew under an earlier run of
// this seat is bounded at start, before anybody has spoken: the conversations
// the service holds are the ones somebody opened, and at start there are none.
// The wire journal and the one previous it keeps are not conversations.
func Humans(directory string) ([]string, error) {
	held, err := os.ReadDir(directory)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("the Partner's conversation directory at %s could not be read: %w", directory, err)
	}
	names := []string{}
	for _, entry := range held {
		name := entry.Name()
		if entry.IsDir() || filepath.Ext(name) != ".jsonl" {
			continue
		}
		if name == wireJournal || name == wireJournalPrevious {
			continue
		}
		names = append(names, strings.TrimSuffix(name, ".jsonl"))
	}
	sort.Strings(names)
	return names, nil
}

// The two names the wire journal takes in a conversation directory.
const (
	wireJournal         = "wire.jsonl"
	wireJournalPrevious = "wire.1.jsonl"
)

// TurnFor reports the turn one client key already minted, so the same send
// twice is the same turn once.
func (c *Conversation) TurnFor(key string) (string, bool) {
	if key == "" {
		return "", false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for at := len(c.messages) - 1; at >= 0; at-- {
		if c.messages[at].Key == key {
			return c.messages[at].Turn, true
		}
	}
	return "", false
}

// Session is the live session's id as it was last recorded.
func (c *Conversation) Session() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.session
}

// RecordSession writes the live session's id beside the transcript. It is
// preference-shaped state: losing it costs a build that loads sessions
// natively one resume, and costs this build nothing.
func (c *Conversation) RecordSession(session string, now time.Time) {
	c.mu.Lock()
	c.session = session
	c.mu.Unlock()
	_ = c.writeState(now)
}

// Sitting is the sitting this conversation is, or nil.
func (c *Conversation) Sitting() *Sitting {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.sitting == nil {
		return nil
	}
	held := *c.sitting
	return &held
}

// Sit marks this conversation with a sitting, and Rise takes the mark off.
//
// Unlike the session id, this one is not preference-shaped: a sitting that was
// lost is a human whose deposits would be refused and whose table would be
// empty on the record they are working on. So the write's failure is returned,
// and the caller refuses the act rather than reporting a sitting that is only
// in this process's memory.
func (c *Conversation) Sit(sitting Sitting, now time.Time) error {
	c.mu.Lock()
	previous := c.sitting
	c.sitting = &sitting
	c.mu.Unlock()
	if err := c.writeState(now); err != nil {
		c.mu.Lock()
		c.sitting = previous
		c.mu.Unlock()
		return err
	}
	return nil
}

// Rise ends the sitting. The record it was about keeps everything that was
// recorded in it, which is the whole of what a sitting leaves behind.
func (c *Conversation) Rise(now time.Time) error {
	c.mu.Lock()
	previous := c.sitting
	c.sitting = nil
	c.mu.Unlock()
	if err := c.writeState(now); err != nil {
		c.mu.Lock()
		c.sitting = previous
		c.mu.Unlock()
		return err
	}
	return nil
}

// sittingsOn is every record one human has a standing sitting's conversation
// about in this store, read from the marks on the state files beside the
// transcripts. A state file that is there but cannot be read or parsed is
// refused by name rather than skipped: a sign-in that skipped it would report
// the seat's sittings moved and leave that one behind.
func sittingsOn(directory, human string) ([]string, error) {
	under := filepath.Join(directory, sittingsDirectory, fileName(human))
	held, err := os.ReadDir(under)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("the Partner's sittings at %s could not be read: %w", under, err)
	}
	records := []string{}
	for _, entry := range held {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(under, entry.Name())
		body, err := os.ReadFile(path)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return nil, fmt.Errorf("the Partner's sitting state at %s could not be read: %w", path, err)
		}
		var state stateFile
		if err := json.Unmarshal(body, &state); err != nil {
			return nil, fmt.Errorf("the Partner's sitting state at %s could not be read: %w", path, err)
		}
		if state.Sitting == nil {
			continue
		}
		if record := strings.TrimSpace(state.Sitting.Subject.ID); record != "" {
			records = append(records, record)
		}
	}
	sort.Strings(records)
	return records, nil
}

// Keep writes the room's working state onto the sitting mark (g1-s65 D9). It is
// the mark's own write, whole, so the room and the session id are one file. A
// keep whose sequence is not above the mark's is a late or repeated request, and
// is ignored.
func (c *Conversation) Keep(room Room, now time.Time) error {
	if len(room.Desk)+len(room.Drafts)+len(room.Face) > maxRoomBytes {
		return fmt.Errorf("the room carries more than %d bytes, and a room is a desk and some unfinished words",
			maxRoomBytes)
	}
	c.mu.Lock()
	if c.sitting == nil {
		c.mu.Unlock()
		return errors.New("no sitting is open on this conversation, so there is no room to keep")
	}
	// A keep numbered at or below the one the mark holds set out before it, or
	// is the same keep again: the newer words stand, and there is nothing to say.
	held := int64(0)
	if c.sitting.Room != nil {
		held = c.sitting.Room.Seq
	}
	if room.Seq <= held {
		c.mu.Unlock()
		return nil
	}
	// The lock is held through the write, so the file is published in the order
	// the keeps were admitted: a keep admitted earlier cannot write its older
	// room over a later one's.
	previous := c.sitting
	kept := *c.sitting
	room.At = now.UTC().Format(time.RFC3339)
	kept.Room = &room
	c.sitting = &kept
	defer c.mu.Unlock()
	if err := c.writeStateHeld(now); err != nil {
		c.sitting = previous
		return err
	}
	return nil
}

// Key is the record this conversation is a sitting's about, or "" for a
// human's own.
func (c *Conversation) Key() string { return c.key }

// writeState replaces the small file beside the transcript, whole, from the
// state as it stands. It is whole rather than a field at a time because the two
// things it holds are written by different acts, and a write that carried only
// its own field would drop the other's.
//
// The state is captured and published under the lock, so a state captured
// earlier is never the one left on disk after a later one.
func (c *Conversation) writeState(now time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.writeStateHeld(now)
}

// writeStateHeld is writeState with this conversation's own lock already held,
// for the sitting's move to a name at the first sign-in (Service.Adopt), which
// holds both conversations while it moves them.
func (c *Conversation) writeStateHeld(now time.Time) error {
	publish := publishState
	if c.publish != nil {
		publish = c.publish
	}
	return publish(c.state, stateFile{
		Session: c.session, Sitting: c.sitting, UpdatedAt: now.UTC().Format(time.RFC3339)})
}

// publishState writes one state file, whole. The state is written beside the
// file and put over it, so a reader not holding the conversation's lock — a
// sign-in looking for the seat's sittings — finds the old state or the new one
// and never a file cut short. The temporary ends in .tmp, which no reader of the
// store counts, so one left by a crash is never taken for a state.
func publishState(path string, held stateFile) error {
	body, err := json.Marshal(held)
	if err != nil {
		return err
	}
	failed := func(err error) error {
		return fmt.Errorf("the Partner's conversation state could not be written: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return failed(err)
	}
	_, err = temporary.Write(append(body, '\n'))
	if closed := temporary.Close(); err == nil {
		err = closed
	}
	if err == nil {
		err = os.Rename(temporary.Name(), path)
	}
	if err != nil {
		_ = os.Remove(temporary.Name())
		return failed(err)
	}
	return nil
}

// The recovery block's two bounds, from the design: the last twenty messages,
// and eight thousand characters of them.
const (
	recoveryMessages = 20
	recoveryBytes    = 8000
)

// History composes the block a fresh session is given after a process loss,
// and reports how many messages went into it.
//
// It is the last twenty messages, newest kept: each one with its speaker and,
// for a human's, the page it was asked from, so an earlier "this goal" still
// names the goal it meant. The bound is on the block, not on the message, and
// what does not fit is dropped from the oldest end — the recent exchange is
// what a follow-up depends on.
func (c *Conversation) History() (string, int) {
	messages := c.Messages(recoveryMessages)
	if len(messages) == 0 {
		return "", 0
	}
	lines := make([]string, 0, len(messages))
	total := 0
	given := 0
	for at := len(messages) - 1; at >= 0; at-- {
		line := historyLine(messages[at])
		if total+len(line) > recoveryBytes {
			break
		}
		total += len(line)
		given++
		lines = append(lines, line)
	}
	if given == 0 {
		return "", 0
	}
	// The lines were collected newest first; the block reads oldest first.
	for left, right := 0, len(lines)-1; left < right; left, right = left+1, right-1 {
		lines[left], lines[right] = lines[right], lines[left]
	}
	return "Conversation so far\n" + strings.Join(lines, "\n"), given
}

func historyLine(message Message) string {
	who := "The human"
	if message.Role == RolePartner {
		who = "You"
	}
	where := ""
	if message.Page != nil {
		where = " (asked from " + pageLine(*message.Page) + ")"
	}
	return "- " + who + where + ": " + oneLine(message.Text) + "\n"
}

// pageLine says where a human was, in one clause.
func pageLine(page Page) string {
	parts := []string{}
	if page.Section != "" {
		parts = append(parts, page.Section)
	}
	if page.Title != "" {
		parts = append(parts, page.Title)
	} else if page.Subject != "" {
		parts = append(parts, page.Subject)
	}
	if len(parts) == 0 {
		return "this workspace"
	}
	return strings.Join(parts, " · ")
}

// oneLine flattens a message for the history block: newlines become spaces, so
// one message is one line and the block's arithmetic is the block's.
func oneLine(text string) string {
	return strings.Join(strings.Fields(text), " ")
}
