package partner

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	resolver "github.com/widoriezebos/agentic-tools/metasystem/internal/project"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/overview"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/snapshot"
)

// The Partner, as the server's routes see it: one runtime, one host, one
// conversation, one turn at a time, and one stream of events.
//
// Everything a route needs is here, so the routes stay routes: Snapshot
// answers GET, Submit answers POST with 202, 409 or 503, Stop answers the stop
// route, and Subscribe is what the page's one event stream reads from.

// Event is one beat of a turn as the page receives it. It rides the existing
// notification stream under its own event type and with no `id:` field, so
// Last-Event-ID stays the notifications journal's cursor and a reconnect
// replays notifications only.
type Event struct {
	Turn string `json:"turn"`
	// Conversation is which conversation the beat belongs to: a sitting's
	// record, or "" for the human's own (g1-s65 D16). A page shows the beats of
	// the conversation on its screen and no other's.
	Conversation string `json:"conversation"`
	Seq          int    `json:"seq"`
	// Kind is text, activity, look, done, error or stopped.
	Kind string `json:"kind"`
	Text string `json:"text"`
	At   string `json:"at"`
	// Look is one completed read, on a look beat and nowhere else.
	Look *Look `json:"look,omitempty"`
	// Suggestion is words the Partner prepared for a field of the editor this
	// turn's human handed over, on a suggestion beat and nowhere else. It is
	// carried as it is admitted, so the card appears under the answer while the
	// answer is still arriving.
	Suggestion *Suggestion `json:"suggestion,omitempty"`
	// Deposit is one entry the Partner offered the sitting's record, on a
	// deposit beat and nowhere else. It is carried as it is admitted, for the
	// suggestion's reason: the card is on the transcript and on the table while
	// the answer is still arriving.
	Deposit *Deposit `json:"deposit,omitempty"`
	// Present is one thing the Partner put on the review's desk, on a present
	// beat and nowhere else (g1-s65 D5). It is a display suggestion: the page
	// shows it on the desk unless the human stopped the walk's presenting.
	Present *Present `json:"present,omitempty"`
	// Proposal is one act the Partner proposed on one goal, on a proposal beat
	// and nowhere else. It is carried as it is admitted, for the suggestion's
	// reason: the card fills line by line while the answer is still arriving,
	// and its buttons wake when the answer ends.
	Proposal *Proposal `json:"proposal,omitempty"`
}

// The eight event kinds.
const (
	EventText     = "text"
	EventActivity = "activity"
	EventDoing    = "doing"
	EventLook     = "look"
	EventDone     = "done"
	EventError    = "error"
	EventStopped  = "stopped"
	// EventSuggestion is one admitted suggestion. It is a beat of its own
	// rather than part of the answer's text because it is not the answer: the
	// words are the Partner's offer for one field, and the page renders them as
	// a card with Use this beside it.
	EventSuggestion = "suggestion"
	// EventDeposit is one admitted deposit, a beat of its own for the
	// suggestion's reason: it is an offer the human decides about, not a
	// sentence of the answer.
	EventDeposit = "deposit"
	// EventProposal is one admitted or refused action, a beat of its own for the
	// suggestion's reason: it is an act the human decides about, not a sentence
	// of the answer.
	EventProposal = "proposal"
	// EventPresent is one admitted display suggestion for the review's desk.
	EventPresent = "present"
)

// Snapshot is what GET /api/partner answers: everything a page needs to render
// the conversation from cold, including whether a turn is running and what it
// has said so far.
type Snapshot struct {
	Runtime string `json:"runtime"`
	Model   string `json:"model"`
	Human   string `json:"human"`
	Busy    bool   `json:"busy"`
	// Turn is the running turn's id, or empty.
	Turn string `json:"turn"`
	// Partial is what the running turn has said so far, so a reload mid-answer
	// shows the answer rather than an empty box.
	Partial string `json:"partial"`
	// PartialSeq is the sequence of the last event that went into Partial, so
	// the page can join live events to it without a gap or a duplicate.
	PartialSeq int      `json:"partialSeq"`
	Activity   []string `json:"activity"`
	// Doing is what the running turn is at this moment, in one line. It is
	// replaced by the next thing and kept nowhere: what was read is the looked
	// list, and what a human has to be told is the activity.
	Doing string `json:"doing"`
	// Looked is what the running turn has read so far, so a reload mid-answer
	// shows the list rather than starting it over.
	Looked []Look `json:"looked"`
	// Suggestions is what the running turn has offered so far, for the reason
	// Looked is here: a reload in the middle of an answer must show the cards
	// that have already arrived rather than lose them until the turn ends.
	Suggestions []Suggestion `json:"suggestions"`
	// Deposits is what the running turn has offered the sitting's record so
	// far, for the same reason.
	Deposits []Deposit `json:"deposits"`
	// Proposals is the actions the running turn has proposed so far, for the
	// same reason: a reload in the middle of an answer must show the card that
	// is filling rather than lose the lines that have already arrived. Their
	// buttons stay asleep until the answer's terminal beat, because until then
	// there is no message for an outcome to be recorded on.
	Proposals []Proposal `json:"proposals"`
	// Sitting is the sitting this conversation is, or null. It is read from the
	// conversation rather than from the browser's memory of it, so a reload and
	// a second tab agree about which record is under discussion.
	Sitting *Sitting `json:"sitting"`
	// Index is what the conversation can point at: the goals the accepted tip
	// carries and the records the checkout declares. It is read here, with the
	// conversation, because the page has no other reader for it and an answer
	// that names a goal has to be able to link it.
	Index Index `json:"index"`
	// ReadOnly is what makes this runtime read-only, in its own words.
	ReadOnly string `json:"readOnly"`
	// Conversation is which conversation this is: a sitting's record, or ""
	// for the human's own.
	Conversation string    `json:"conversation"`
	Messages     []Message `json:"messages"`
}

// Service is the conversation owner.
//
// One seat, one runtime, one live process — and one conversation per human,
// because the transcript is a human's and the file is named after them. Two
// humans on one seat is deferred, and this is what the deferral costs: the
// live session belongs to whoever spoke last, so a turn from a different human
// ends that session and opens a fresh one, which is then given that human's
// own history exactly as a process loss would be.
type Service struct {
	runtime Runtime
	host    *Host
	open    func(human string) (*Conversation, error)
	facts   Facts
	now     func() time.Time
	// announce says whether a turn is running, for the seat's own record, so
	// `ui status` in another process can print it. A nil announce is a build
	// that keeps no record, which costs a line and nothing else.
	announce func(busy bool)

	mu            sync.Mutex
	conversations map[string]*Conversation
	// opening names the keys a caller is loading right now, with the channel it
	// closes when it is done. It is what makes the FIRST open of one transcript
	// exclusive: a load repairs the file it reads (R-131-ui), so two loaders
	// over one transcript archive it twice and the loser rewrites it from the
	// reading it took before the winner appended anything — losing a message
	// the human had already been told was accepted (Astra F-02). One opener per
	// key; every other caller waits for it and takes what it installed.
	opening map[string]chan struct{}
	// live is which conversation the live session belongs to, by its key, or
	// "" before any turn. The process carries one conversation, so a turn on
	// another one ends the session and opens that conversation's, fresh, with
	// its own history replayed (g1-s65 D16). It generalises what used to be
	// "whoever spoke last": a human is one conversation, and a sitting another.
	live string
	// settle bounds the wait for a stopped turn to be written down, both for
	// the stop route and for the handoff to another conversation (S65-06).
	settle   time.Duration
	current  *turn
	watchers map[int]chan Event
	nextID   int
	// indexedAt is when the live session's map of the project's memory was
	// read. A fresh session is given the map; every later prompt of that
	// session is given this moment instead, so the Partner knows how old its
	// map is without being handed a new one that would look current.
	indexedAt time.Time
	// opened says that a session was started outside a turn — by Admits, which
	// asks the runtime whether a sitting could be opened at all before anything
	// is created for it — and has been given no prompt yet. The turn that
	// follows is therefore still that session's first, and is given how to
	// answer here and the map of the project's memory. Without it, asking
	// whether the runtime is there would silently cost the next turn its
	// instructions.
	opened bool
}

// turn is the running turn's state, which the snapshot reads and the events
// are numbered against.
type turn struct {
	id    string
	human string
	key   string
	// where is the conversation this turn was asked in, by its key: a
	// sitting's record, or "" for the human's own. Every beat carries it.
	where string
	// conversation is the transcript this turn writes its answer into, and the
	// sitting a deposit it prepares is admitted against. It is held here rather
	// than by the goroutine that runs the turn because the first sign-in moves
	// it: the seat's conversation becomes the human's while the answer is still
	// arriving, and the answer belongs with the question it followed (Astra
	// A-03). It is read under the service's mutex at the moment it is used, and
	// never captured earlier.
	conversation *Conversation
	// page is the capture this turn was asked with, held to its bounds. It is
	// the one thing that says which editor opening the human handed over and
	// which of its fields they may be written into, so it is what a suggestion
	// is admitted against.
	page Page
	// verdict is the verdict a review's closing turn was asked with, stamped on
	// the outcome it offers (g1-s65 D10), and "" for every other turn.
	verdict string
	// tip is the branch tip the review's record named when its closing turn
	// was asked, stamped on the outcome beside the verdict (g1-s69 D1), so
	// the Outcome is bound to the tip it was drafted for.
	tip      string
	seq      int
	text     strings.Builder
	activity []string
	doing    string
	looked   []Look
	// suggestions is what this turn has offered and the service admitted, in
	// the order they were admitted.
	suggestions []Suggestion
	// deposits is what this turn offered the sitting's record, in the order
	// they were admitted.
	deposits []Deposit
	// proposals is the actions this turn proposed, admitted and refused alike,
	// in the order they arrived. The order is the index the outcome route names
	// a line by, so a refused action holds its place.
	proposals []Proposal
	// cut is how many actions this answer carried past the count it is bounded
	// at. They are not stored line by line — the bound is on what the
	// transcript keeps — so the answer holds one account of them, and this is
	// what it counts (Astra F-06).
	cut int
	// observed is the ledger reading this turn was composed against, held so
	// that an action is admitted against what the Partner was told rather than
	// against a later reading of a moving ledger.
	observed snapshot.Observation
	stopping bool
	// done closes when the turn has been written down and its terminal event
	// published. Stop waits for it, so the snapshot the stop route answers
	// with is the settled one rather than the one the turn was in when the
	// cancellation reached the runtime.
	done chan struct{}
}

// NewService builds the owner. It starts nothing: the runtime's process is
// started by the first turn, and torn down when it has been idle for an hour.
func NewService(runtime Runtime, host *Host, open func(human string) (*Conversation, error), facts Facts, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{
		runtime: runtime, host: host, open: open, facts: facts, now: now,
		conversations: map[string]*Conversation{},
		opening:       map[string]chan struct{}{},
		watchers:      map[int]chan Event{},
		settle:        settleWait,
	}
}

// conversationKey is the one name a conversation is held under: the file name
// the human maps to and, for a sitting's, the record it is about. It is the
// file's identity rather than the spelling a caller used, for the one-opener
// rule's reason below.
func conversationKey(human, sitting string) string {
	if sitting == "" {
		return fileName(human)
	}
	return fileName(human) + "\x00" + sitting
}

// conversation answers one human's transcript, opening it the first time.
//
// It is held under the name the FILE is kept under rather than the handle as it
// was spelled, because that is what decides which transcript this is: a handle
// with a space in it and the same handle with a dash are one file, and two
// objects over one file would be two mutexes over it — which is the race the
// trim's own mutex exists to prevent (g1-s54 F2). Housekeeping reaches a
// transcript nobody has opened this run by the file's name, so the two ways in
// have to meet at one object.
// The first open of one key is also EXCLUSIVE, because a load is no longer a
// read: an oversized transcript is archived and rewritten by the loader that
// finds it (R-131-ui). Two loaders over one file archive it twice, and the one
// that is discarded has already published its own older reading over the file —
// taking out a message the other opener has since accepted (Astra F-02). So one
// caller loads and the others wait for it here, and the object that is installed
// is the one they all take.
func (s *Service) conversation(human string) (*Conversation, error) {
	return s.conversationOf(human, "")
}

// conversationOf is the conversation one room's address names: the record's
// own (g1-s65 D16), or the human's ordinary conversation where the record's
// carries no mark and the ordinary one's mark names that record — a sitting
// begun before D16, which is the one set of sittings not keyed by their record
// (g1-s67 D6). "" is the human's own conversation. This is the one
// place that rule is applied, and every request naming a room goes through it.
//
// The resolution is made on every call and kept nowhere: the fallback is never
// held under the record's key, so once End takes the ordinary conversation's
// mark off, the same address names the record's own conversation again and a
// Start there opens a sitting keyed by its record.
func (s *Service) conversationOf(human, record string) (*Conversation, error) {
	record = strings.TrimSpace(record)
	own, err := s.openedOf(human, record)
	if err != nil || record == "" || own.Sitting() != nil {
		return own, err
	}
	ordinary, err := s.openedOf(human, "")
	if err != nil {
		return nil, err
	}
	if sitting := ordinary.Sitting(); sitting != nil && sitting.Subject.ID == record {
		return ordinary, nil
	}
	return own, nil
}

// openedOf is one human's conversation about one sitting's record, or their own
// where the record is "", exactly as its files hold it (g1-s65 D16). A sitting's
// conversation is opened in the store the human's own lives in, under the same
// one-opener rule.
func (s *Service) openedOf(human, sitting string) (*Conversation, error) {
	sitting = strings.TrimSpace(sitting)
	key := conversationKey(human, sitting)
	for {
		s.mu.Lock()
		if held, known := s.conversations[key]; known {
			s.mu.Unlock()
			return held, nil
		}
		if loading, busy := s.opening[key]; busy {
			s.mu.Unlock()
			// The loader has the file. Waiting costs this caller the load it
			// would have run itself, and the loop then finds what was
			// installed — or, where that load failed, takes the open on.
			<-loading
			continue
		}
		mine := make(chan struct{})
		s.opening[key] = mine
		s.mu.Unlock()

		opened, err := s.openOf(human, sitting)
		s.mu.Lock()
		delete(s.opening, key)
		if err == nil {
			s.conversations[key] = opened
		}
		s.mu.Unlock()
		// After the map is written, so a waiter that wakes finds the
		// conversation rather than going round and loading it again.
		close(mine)
		if err != nil {
			return nil, err
		}
		return opened, nil
	}
}

// openOf opens a conversation for the first time: the caller's own opener for a
// human's, and the store that one lives in for a sitting's.
func (s *Service) openOf(human, sitting string) (*Conversation, error) {
	if sitting == "" {
		return s.open(human)
	}
	own, err := s.openedOf(human, "")
	if err != nil {
		return nil, err
	}
	return OpenConversation(own.directory, human, sitting)
}

// Adopt gives a human the conversation an unnamed seat was having.
//
// Until somebody signs in, a seat that knows nobody is its own human and what
// is said goes into the seat's transcript. The first sign-in names it, and
// every read and every write after that is about the NAME's conversation — so
// an action proposed a moment earlier, which the human is pressing Apply on
// through the sign-in sheet, would be looked for in a transcript that never
// held it, and the reload would take the card away (Astra A-03).
//
// So the seat's messages move to the human, and the seat is left with none: it
// is one conversation that gained a name, not two. A human who already has a
// transcript keeps it and takes the seat's messages after it, because those
// are the ones just spoken. Both transcripts are written under the mutex their
// own appends take, the seat's first, so no turn writes into either halfway
// through the move.
//
// Three things move, because a conversation is all three. The messages are the
// transcript. The sitting is the mark beside it, and a human who opened one
// before signing in is still sitting on that record afterwards. And a turn
// still running is rebound to the human under the service's own mutex, so the
// answer it has not written down yet — and the actions it proposed in it —
// land where the question already is rather than in the seat just emptied.
func (s *Service) Adopt(seat, human string) error {
	if fileName(seat) == fileName(human) {
		return nil
	}
	was, err := s.conversation(seat)
	if err != nil {
		return err
	}
	mine, err := s.conversation(human)
	if err != nil {
		return err
	}
	if err := s.adoptOne(was, mine, human); err != nil {
		return err
	}
	// And every sitting the seat stands in, each its own conversation (g1-s65
	// D16): a review started before signing in is the human's review after it.
	records, err := sittingsOn(was.directory, seat)
	if err != nil {
		return err
	}
	for _, record := range records {
		fromSeat, err := s.openedOf(seat, record)
		if err != nil {
			return err
		}
		toHuman, err := s.openedOf(human, record)
		if err != nil {
			return err
		}
		if err := s.adoptOne(fromSeat, toHuman, human); err != nil {
			return err
		}
	}
	return nil
}

// adoptOne moves one conversation's messages, its mark and its running turn to
// the human's conversation of the same key.
func (s *Service) adoptOne(was, mine *Conversation, human string) error {
	was.mu.Lock()
	defer was.mu.Unlock()
	if len(was.messages) == 0 && was.sitting == nil && !s.running(was) {
		return nil
	}
	mine.mu.Lock()
	defer mine.mu.Unlock()
	if len(was.messages) > 0 {
		moved := append(append([]Message{}, mine.messages...), was.messages...)
		if err := writeTranscript(mine.transcript, moved); err != nil {
			return err
		}
		mine.messages = moved
		// The seat's file is emptied only once the human's holds the messages, so a
		// failure between the two leaves them where they were rather than nowhere.
		if err := writeTranscript(was.transcript, nil); err != nil {
			return err
		}
		was.messages = nil
	}
	if err := s.adoptSitting(was, mine); err != nil {
		return err
	}
	s.mu.Lock()
	if s.current != nil && s.current.conversation == was {
		s.current.conversation = mine
		// And whose turn it is, or the page that has just been named would read
		// a snapshot with no running turn on it and take the arriving answer off
		// the screen until it ended.
		s.current.human = human
	}
	s.mu.Unlock()
	return nil
}

// running says whether the turn in flight is this conversation's, which is one
// of the three reasons there is something to adopt.
//
// An empty transcript is not proof that there is nothing: a turn is admitted,
// and bound to its conversation, a moment before its question is written down,
// and a sign-in that landed in that moment would leave the turn behind.
func (s *Service) running(conversation *Conversation) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.current != nil && s.current.conversation == conversation
}

// adoptSitting moves the seat's sitting to the human, with both conversations
// held.
//
// The mark is not in the transcript, it is in the state file beside it, so a
// move that carried the messages alone left the human sitting on nothing while
// the emptied seat went on claiming the record. A sitting the human already had
// is replaced: what they are adopting is the conversation they are having now,
// and one conversation is one sitting.
//
// The human's file is written first, as the messages are, so a failure between
// the two leaves the sitting where it was rather than nowhere.
func (s *Service) adoptSitting(was, mine *Conversation) error {
	if was.sitting == nil {
		return nil
	}
	moved, previous := was.sitting, mine.sitting
	mine.sitting = moved
	if err := mine.writeStateHeld(s.now()); err != nil {
		mine.sitting = previous
		return err
	}
	was.sitting = nil
	if err := was.writeStateHeld(s.now()); err != nil {
		was.sitting = moved
		return err
	}
	return nil
}

// Trim keeps this seat's transcripts within the bounds, and reports how many
// messages went.
//
// humans names the store's own files, so a transcript that grew under an
// earlier run of this seat is bounded at start, before anybody has spoken —
// otherwise housekeeping's first sweep would find no conversation open and do
// nothing. Every one of them is trimmed through the object that owns it, this
// service's own, because the trim is the conversation's operation under the
// mutex its appends take.
//
// A transcript that cannot be opened or cannot be trimmed does not stop the
// rest: the other files are still over their bounds, and one unreadable file is
// a reason to say so rather than to leave the store growing.
func (s *Service) Trim(bounds TrimBounds, humans []string) (int, error) {
	var trouble []error
	for _, human := range humans {
		if _, err := s.conversation(human); err != nil {
			trouble = append(trouble, err)
		}
	}
	s.mu.Lock()
	held := make([]*Conversation, 0, len(s.conversations))
	for _, conversation := range s.conversations {
		held = append(held, conversation)
	}
	s.mu.Unlock()
	cut := 0
	now := s.now()
	for _, conversation := range held {
		removed, err := conversation.Trim(bounds, now)
		cut += removed
		if err != nil {
			trouble = append(trouble, err)
		}
	}
	return cut, errors.Join(trouble...)
}

// Runtime is what this seat admitted.
func (s *Service) Runtime() Runtime { return s.runtime }

// SeesOverview gives the conversation the landing page as the server composes
// it. It is set rather than passed in at construction because composing that
// page needs the steward's journal and the seat's own standing as well as the
// two readers this service is built with, and those belong to the interface
// server rather than to the Partner.
func (s *Service) SeesOverview(read func() (overview.Page, error)) {
	s.mu.Lock()
	s.facts.Overview = read
	s.mu.Unlock()
}

// reading is the readers as they stand, copied under the lock because one of
// them is set after construction.
func (s *Service) reading() Facts {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.facts
}

// Announce sets what this service tells the seat's record when a turn starts
// and ends, and says the idle state at once.
func (s *Service) Announce(say func(busy bool)) {
	s.mu.Lock()
	s.announce = say
	s.mu.Unlock()
	if say != nil {
		say(false)
	}
}

// said tells the seat's record what changed, outside the lock.
func (s *Service) said(busy bool) {
	s.mu.Lock()
	say := s.announce
	s.mu.Unlock()
	if say != nil {
		say(busy)
	}
}

// Busy reports whether a turn is running, which `ui status` prints.
func (s *Service) Busy() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.current != nil
}

// Snapshot answers the read route for one human's own conversation.
func (s *Service) Snapshot(human string, limit int) (Snapshot, error) {
	return s.SnapshotIn(human, "", limit)
}

// SnapshotIn answers the read route for one conversation: a sitting's, named by
// its record, or the human's own where the record is "".
func (s *Service) SnapshotIn(human, sitting string, limit int) (Snapshot, error) {
	conversation, err := s.conversationOf(human, sitting)
	if err != nil {
		return Snapshot{}, err
	}
	s.mu.Lock()
	running := s.current
	// The answer names the address it was asked for, which is the room's record
	// even where the room's conversation is the ordinary one, so the page files
	// it under the room it asked from.
	answer := Snapshot{
		Runtime: s.runtime.Name, Model: s.runtime.Model, Human: human,
		ReadOnly: s.runtime.ReadOnly, Conversation: strings.TrimSpace(sitting),
	}
	if running != nil && running.conversation == conversation {
		answer.Busy = true
		answer.Turn = running.id
		answer.Partial = running.text.String()
		answer.PartialSeq = running.seq
		answer.Activity = append([]string{}, running.activity...)
		answer.Doing = running.doing
		answer.Looked = append([]Look{}, running.looked...)
		answer.Suggestions = append([]Suggestion{}, running.suggestions...)
		answer.Deposits = append([]Deposit{}, running.deposits...)
		answer.Proposals = append([]Proposal{}, running.proposals...)
	}
	s.mu.Unlock()
	answer.Sitting = conversation.Sitting()
	answer.Messages = conversation.Messages(limit)
	answer.Index = IndexOf(s.reading())
	return answer, nil
}

// See composes what the Partner would be given for one capture, without
// sending anything. It is the sheet's own answer, through the composer the
// turn uses, so the two cannot be two readings of the same page.
func (s *Service) See(page Page, now time.Time) Seen {
	return See(s.reading(), page.Bound(), now.UTC())
}

// Busy is the refusal a second send gets while a turn runs.
var ErrBusy = errors.New("the Partner is answering; wait for it to finish or stop it")

// ErrUnsettled is the refusal a turn on another conversation gets when the
// running turn it stopped has not been written down within the wait (S65-06).
// Nothing of the new turn has started: its conversation is untouched and no
// session was opened for it.
var ErrUnsettled = errors.New("the previous room's answer has not settled; try again in a moment")

// Submit admits one turn.
//
// The same key twice is the same turn once, so a retry after a lost answer
// never submits twice. A send while busy is refused and the draft stays. A
// runtime that cannot start refuses here, before the turn exists, so the
// human's question stays in the composer and the route answers 503 with the
// runtime's own words.
func (s *Service) Submit(ctx context.Context, human, key, text string, page Page) (string, error) {
	return s.submit(ctx, human, "", key, text, page, false)
}

// SubmitIn admits one turn in one conversation: a sitting's, named by its
// record, or the human's own where the record is "".
func (s *Service) SubmitIn(ctx context.Context, human, sitting, key, text string, page Page) (string, error) {
	return s.submit(ctx, human, sitting, key, text, page, false)
}

// submit is Submit with the one thing only this package may decide: whether the
// question is the human's own or one this interface asked on their behalf.
//
// Nothing outside this package can set that mark, and nothing outside it should
// be able to: a browser that could claim a question was the interface's could
// dress up a question the human typed as one they did not.
func (s *Service) submit(ctx context.Context, human, sitting, key, text string, page Page, byInterface bool) (string, error) {
	return s.submitClosing(ctx, human, sitting, key, text, page, byInterface, "")
}

// submitClosing is submit for a turn that carries the verdict a review's
// closing was asked with, so the outcome it offers carries it too.
func (s *Service) submitClosing(ctx context.Context, human, sitting, key, text string, page Page, byInterface bool, verdict string) (string, error) {
	return s.submitClosingAt(ctx, human, sitting, key, text, page, byInterface, verdict, "")
}

// submitClosingAt is submitClosing with the tip a review's record names.
func (s *Service) submitClosingAt(ctx context.Context, human, sitting, key, text string, page Page, byInterface bool, verdict, tip string) (string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", errors.New("a turn needs a question")
	}
	// The capture is held to its bounds before anything is done with it, and
	// above all before it is written into the transcript: what a message keeps
	// is what this server was prepared to keep.
	page = page.Bound()
	conversation, err := s.conversationOf(human, sitting)
	if err != nil {
		return "", err
	}
	if existing, known := conversation.TurnFor(key); known {
		return existing, nil
	}
	where := conversationKey(human, conversation.key)
	s.mu.Lock()
	// One running turn, and it stays one. A turn in THIS conversation is busy,
	// as it always was. A turn in another conversation is the handoff (Astra
	// S65-06): it is stopped through the service's own Stop, bound to its own
	// conversation through its terminal write, and only once it has been
	// written down does this turn go on — or, where it does not settle within
	// the wait, this one is refused in words and nothing of it starts.
	for s.current != nil {
		running := s.current
		if running.conversation == conversation {
			s.mu.Unlock()
			return "", ErrBusy
		}
		running.stopping = true
		s.mu.Unlock()
		if !s.stopAndSettle(ctx, running) {
			return "", ErrUnsettled
		}
		s.mu.Lock()
	}
	// A turn in another conversation than the live session's ends that
	// session: the process carries one conversation, and it is not this one.
	changed := s.live != "" && s.live != where
	// The turn is bound to its conversation HERE, before the startup wait
	// below, because a sign-in that lands inside that wait has to be able to
	// find it. Starting a runtime is a process spawn, an initialize and a
	// session/new; for all of that, the question exists and is written nowhere,
	// so a turn bound afterwards left Adopt looking at an empty seat with
	// nothing running, deciding there was nothing to adopt, and startup then
	// bound the turn to the seat the human had just left — the question, the
	// answer and every action the answer proposed stayed there (Astra A-03,
	// second confirmation read). A startup that refuses releases it again,
	// below, so a runtime that is not installed leaves no turn behind.
	id := mintTurn()
	// The turn's beats name the address it was asked from, the room's record
	// even where that room's conversation is the ordinary one, so the page files
	// them under the room they were asked from.
	running := &turn{id: id, human: human, key: key, page: page, verdict: verdict, tip: tip, where: strings.TrimSpace(sitting),
		conversation: conversation, done: make(chan struct{})}
	s.current = running
	s.mu.Unlock()
	if changed {
		s.host.Close()
	}

	// Starting the process is part of admitting the turn, not part of running
	// it: a runtime that is not installed or not signed in must refuse the
	// send rather than accept a turn it cannot take.
	fresh, err := s.host.Ready(ctx)
	if err != nil {
		s.mu.Lock()
		s.current = nil
		s.mu.Unlock()
		return "", err
	}

	// Which conversation this turn is in is the TURN's answer from here on: a
	// sign-in during the wait above moved it, and the one captured before the
	// wait is the emptied seat. The session id, the history and the proposals
	// block all belong to the conversation the turn is in now.
	s.mu.Lock()
	conversation = running.conversation
	s.mu.Unlock()

	// The live session's id is recorded beside the transcript, so a build that
	// learns to load sessions natively has it without guessing.
	conversation.RecordSession(s.host.Session(), s.now())

	s.mu.Lock()
	// A session opened outside a turn has been given no prompt, so this turn is
	// its first however Ready answered just now.
	if s.opened {
		fresh = true
		s.opened = false
	}
	s.live = where
	s.mu.Unlock()

	// The history is what this conversation held BEFORE this question, and it
	// is read before the question is written down: a turn must not be given
	// itself back as its own context.
	history, given := "", 0
	if fresh {
		history, given = conversation.History()
	}

	now := s.now().UTC()
	asked := Message{ID: mintTurn(), Turn: id, Role: RoleHuman, Text: text,
		At: now.Format(time.RFC3339), Key: key, Page: &page, Interface: byInterface}
	if err := s.appendMessage(running, asked); err != nil {
		s.mu.Lock()
		s.current = nil
		s.mu.Unlock()
		return "", err
	}

	// The page is composed once, and the same composition is three things: what
	// the turn is given, what the answer's stamp names, and the first entry in
	// what this answer was read from. A snapshot recomposed for the stamp would
	// be a second reading of a moving ledger.
	seen := See(s.reading(), page, now)
	// The reading that composition made is held on the turn, so an action the
	// Partner proposes is admitted against the ledger the Partner was told
	// about rather than against a second reading taken while it answered.
	s.mu.Lock()
	running.observed = seen.Observed
	s.mu.Unlock()
	s.record(running, Event{Kind: EventLook, Look: lookedAtPage(seen)})

	// The first prompt of a session carries how to answer here and a map of
	// the project's memory; a later prompt of the same session carries the
	// moment that map was read. A session that was lost and reopened is a
	// first prompt again, and is given both again, read afresh.
	opening := ""
	if fresh {
		composed, index := Opening(s.reading(), now)
		opening = composed
		s.mu.Lock()
		s.indexedAt = index.At
		s.mu.Unlock()
	} else {
		s.mu.Lock()
		indexedAt := s.indexedAt
		s.mu.Unlock()
		if !indexedAt.IsZero() {
			opening = Returning(indexedAt)
		}
	}

	// What happened to the actions the last two answers proposed, from the
	// states the messages record: the Partner builds on what the human actually
	// did rather than on what it prepared.
	//
	// The whole kept transcript is read for it, not a window of recent messages:
	// the block picks the last two answers that proposed anything, and ten
	// ordinary exchanges are twenty messages, so a window that size dropped a
	// proposal the human was still applying from the Decisions inbox (Astra
	// B-03). What the prompt carries is bounded by that selection of two, here
	// as before.
	prompt := ComposeOpening(seen, page, human, opening, proposalsBlock(conversation.Messages(0))) + "\n\n"
	if given > 0 {
		prompt += history + "\n\n"
		s.record(running, Event{Kind: EventActivity, Text: freshLine(given)})
	}
	prompt += "The human asks:\n" + text

	s.said(true)
	go s.run(running, prompt)
	return id, nil
}

// lookedAtPage is the page itself, as the first entry in what an answer was
// read from. It is listed separately from everything the Partner went on to
// read, because it is the one reading it did not choose.
func lookedAtPage(seen Seen) *Look {
	outcome := LookRead
	if seen.Total > 0 && seen.Supplied < seen.Total {
		outcome = LookPartial
	}
	source := seen.Source
	if seen.Displayed != "" {
		source += "; the page had rendered from " + seen.Displayed
	}
	excerpt := seen.Block
	if len(excerpt) > maxExcerpt {
		excerpt = excerpt[:maxExcerpt] + "…"
	}
	return &Look{
		Page:    true,
		What:    "The page you were looking at — " + seen.Label,
		Source:  source,
		Outcome: outcome,
		Excerpt: excerpt,
	}
}

// admit decides whether one prepared suggestion is offered to the human.
//
// This is the one place that can decide it, because this is the one place that
// holds the capture. The tool server has no capture and the host has no turn;
// what the human handed over — which editor opening, and which of its fields
// may be written into — is in the capture this turn was asked with, and nothing
// else in this process knows it.
//
// A suggestion for a field the human did not hand over is not offered, and the
// refusal is recorded as the suggestion it was, with its reason, rather than
// dropped to an activity line the drawer does not show. A human who asked for a
// better wording and got no proposal has to be able to read why, where they
// read the answer. An empty field is admitted like any other — a next step
// nobody has written yet is exactly the field a human asks for words for —
// because the writable names travel whether or not there is anything in them.
//
// A refused one is never stamped with an opening. It is not something to use,
// and an opening on it is the one thing that could let a page offer Use this
// for words nothing may write.
func (s *Service) admit(running *turn, prepared Suggestion) {
	s.mu.Lock()
	draft := running.page.Draft
	s.mu.Unlock()
	if draft == nil || draft.Opening == "" ||
		draft.Sheet != prepared.Editor || !draft.Writes(prepared.Field) {
		prepared.Offered = false
		prepared.Reason = notOffered(draft, prepared.Field)
		s.record(running, Event{Kind: EventSuggestion, Suggestion: &prepared})
		return
	}
	prepared.Opening = draft.Opening
	prepared.Offered = true
	s.record(running, Event{Kind: EventSuggestion, Suggestion: &prepared})
}

// notOffered is why one suggestion was not offered, in the words a human reads.
//
// Two reasons, because there are two things a human would do about it. A draft
// that never reached this turn at all — no sheet handed over, or the chip's
// take-back pressed — is answered with the act that hands it over again. A
// draft that did reach it, for a field it does not open, is answered by naming
// that field: the sheet decides which of its fields the Partner may write for,
// and this one is not among them.
func notOffered(draft *Draft, field string) string {
	if draft == nil || strings.TrimSpace(draft.Sheet) == "" {
		return "the draft was left out; press Ask about this to hand it over again"
	}
	said := strings.TrimSpace(field)
	if said == "" {
		said = "An unnamed field"
	}
	return said + " is not open for proposals"
}

/* --------------------------------------------------------- the sitting -- */

// ErrNoSitting is what a deposit is refused with where no sitting is open, in
// the words a human reads on the card.
const noSitting = "no sitting is open, so there is no record to offer this to; " +
	"start a sitting from the record you are working on"

// admitsPurpose is the one place that says which sittings this build offers.
//
// It is a function of its own because two callers need the same answer at two
// moments: Sit, which will not open a sitting this build has no moves for, and
// Admits, which is asked BEFORE a draft record is created for one.
func admitsPurpose(purpose string) error {
	if purpose != PurposeShapeIntent && purpose != PurposeShapeDesign && purpose != PurposeReview {
		return fmt.Errorf(
			"a sitting is for %q, %q or a %q; learning sittings are not in this build",
			PurposeShapeIntent, PurposeShapeDesign, PurposeReview)
	}
	return nil
}

// subjectKinds is what a sitting on any other kind of record is refused with, in
// the words a human reads. It is a constant because two places say it: the
// refusal itself, and the test that proves the refusal is the one given.
const subjectKinds = "a sitting is about an intent or a design record"

// reviewKind is what a review sitting on any other kind of record is refused
// with: a review's subject is the review record the server created for it at
// Start (g1-s65 D2), so every rule of the sitting holds unchanged.
const reviewKind = "a review sitting is about a review record"

// admitsSubject says whether a sitting may be about this record (g1-s53 D1).
//
// A sitting shapes intent or shapes a design, and its subject is the record it
// shapes: the four piles it writes — Facts, Proposals, Decisions, Open questions
// — belong in one of those two kinds and nowhere else. A doctrine record, a
// recorded decision, or a plain file of the checkout is not a thing there is a
// sitting for. The record's page offers Start on nothing else, but the page is
// not the gate: the route is reachable without it, and a subject arriving from
// anywhere at all is judged here.
//
// The kind is the record's own declared head, read through the same document
// reader the pages are composed from. A file declaring no head, or a head naming
// no kind, is not a record of any kind and is refused for exactly that.
//
// A build with no document reader is not asked: it has no Project section and no
// record page, so there is no Start to press and no head to read. What the reader
// itself refuses is passed on as it is — a subject this checkout cannot read is a
// sitting with nothing to record into, which is the route's own reason for
// creating a draft before opening one.
func (s *Service) admitsSubject(subject Subject, purpose string) error {
	if s.facts.Document == nil {
		return nil
	}
	document, err := s.facts.Document(subject.ID)
	if err != nil {
		return fmt.Errorf("cannot read %s, so a sitting cannot be opened on it: %w", subject.ID, err)
	}
	kind := ""
	if document.Record != nil {
		kind = strings.TrimSpace(document.Record.Kind)
	}
	switch {
	case purpose == PurposeReview && kind != resolver.KindReview:
		if kind == "" {
			return fmt.Errorf("%s, and %s declares no kind", reviewKind, subject.ID)
		}
		return fmt.Errorf("%s, and %s is a %s record", reviewKind, subject.ID, kind)
	case purpose == PurposeReview:
		return nil
	case kind == "":
		return fmt.Errorf("%s, and %s declares neither", subjectKinds, subject.ID)
	case kind != resolver.KindIntent && kind != resolver.KindDesign:
		return fmt.Errorf("%s, and %s is a %s record", subjectKinds, subject.ID, kind)
	}
	return nil
}

// Admits says whether a sitting for this purpose could be opened at all, and
// creates nothing.
//
// Sol's third finding: the route created the draft record a sitting was started
// on before anything had judged the purpose or asked the runtime whether it could
// take a turn. A purpose this build does not offer, or a runtime that is not
// installed, therefore left a record in the project that nobody asked for, that
// no sitting names, and that the human was never told about. So the two things
// that can be asked without writing anything are asked here first.
//
// Admitting the runtime starts its process, which is what makes the answer worth
// having: "not installed" and "not signed in" are only knowable by asking it. The
// session it opens is remembered as one nothing has been said in yet, so the turn
// that follows is still that session's first and is given how to answer here and
// the map of the project's memory.
func (s *Service) Admits(ctx context.Context, purpose string) error {
	if err := admitsPurpose(strings.TrimSpace(purpose)); err != nil {
		return err
	}
	fresh, err := s.host.Ready(ctx)
	if err != nil {
		return err
	}
	if fresh {
		s.mu.Lock()
		s.opened = true
		s.mu.Unlock()
	}
	return nil
}

// Sit opens a sitting on one record and asks the Partner the opening question.
//
// The order is the whole of what this owes a human. The purpose and the subject
// are judged first, because a sitting on nothing is not a sitting. The mark goes
// on the conversation next, so the opening turn is already a turn of this
// sitting and a deposit it prepares is admitted against this subject. The
// opening turn goes last — and the turn's own admission is what refuses a
// runtime that cannot start, BEFORE anything is appended to the transcript, so a
// seat with no Partner answers with the runtime's own words and the transcript
// is untouched. A refused turn takes the mark off again: a sitting whose first
// question never went is not a sitting a human should come back to (g1-s53 D3).
func (s *Service) Sit(ctx context.Context, human string, subject Subject, purpose string, page Page) (Sitting, error) {
	purpose = strings.TrimSpace(purpose)
	subject.Kind = strings.TrimSpace(subject.Kind)
	subject.ID = strings.TrimSpace(subject.ID)
	subject.Title = strings.TrimSpace(subject.Title)
	if err := admitsPurpose(purpose); err != nil {
		return Sitting{}, err
	}
	switch {
	case subject.Kind != SubjectRecord:
		return Sitting{}, errors.New("a sitting is about one record of this project, and nothing else yet")
	case subject.ID == "":
		return Sitting{}, errors.New("a sitting about a record says which one, by its path in this checkout")
	}
	if err := s.admitsSubject(subject, purpose); err != nil {
		return Sitting{}, err
	}
	// A sitting is a conversation (g1-s65 D16): the mark goes on the sitting's
	// own conversation, beside the human's, and the opening turn is asked there.
	conversation, err := s.conversationOf(human, subject.ID)
	if err != nil {
		return Sitting{}, err
	}
	previous := conversation.Sitting()
	sitting := Sitting{Subject: subject, Purpose: purpose, StartedAt: s.now().UTC().Format(time.RFC3339)}
	if err := conversation.Sit(sitting, s.now()); err != nil {
		return Sitting{}, err
	}
	if _, err := s.submit(ctx, human, subject.ID, "", OpeningRequest(sitting), page, true); err != nil {
		s.unsit(conversation, previous)
		return Sitting{}, err
	}
	return sitting, nil
}

// unsit puts the conversation back the way it was after a refused opening turn.
// A write that fails here has nothing left to say: the sitting is already
// refused, and the mark in this process's memory is gone either way.
func (s *Service) unsit(conversation *Conversation, previous *Sitting) {
	if previous == nil {
		_ = conversation.Rise(s.now())
		return
	}
	_ = conversation.Sit(*previous, s.now())
}

// Rise ends the sitting on this human's conversation. What was recorded is in
// the record, which is the whole of what a sitting leaves behind.
func (s *Service) Rise(human string) error {
	return s.RiseIn(human, "")
}

// RiseIn ends the sitting on one conversation. The conversation stays, and so
// does everything recorded.
func (s *Service) RiseIn(human, sitting string) error {
	conversation, err := s.conversationOf(human, sitting)
	if err != nil {
		return err
	}
	return conversation.Rise(s.now())
}

// Sitting is the sitting standing on one human's conversation, or nil.
//
// It is here so that a reader outside this package — the project payload's
// Sittings list, which has to say whether a sitting stands on a record now —
// can ask without taking a whole snapshot of the conversation for one field.
func (s *Service) Sitting(human string) (*Sitting, error) {
	return s.SittingIn(human, "")
}

// SittingIn is the sitting standing on one conversation, or nil.
func (s *Service) SittingIn(human, sitting string) (*Sitting, error) {
	conversation, err := s.conversationOf(human, sitting)
	if err != nil {
		return nil, err
	}
	return conversation.Sitting(), nil
}

// noSittingToClose is what End is refused with where no sitting stands.
const noSittingToClose = "no sitting is open, so there is nothing to close"

// Closing asks the Partner for the closing deposit, and leaves the sitting
// standing (g1-s55 D2).
//
// It is not called Close, because Close on this service is what ends the
// runtime's process, and one word cannot be both the end of a conversation's
// process and the drafting of a sitting's result.
//
// The mark stays on deliberately: the outcome this turn offers is admitted
// against the sitting's own subject, exactly as every other deposit is, and a
// conversation whose mark had already been taken off would offer it against
// nothing. So ending a sitting is two acts — this one, which drafts, and Rise,
// which the page reaches after the human has recorded the outcome or has said
// they are leaving without it.
func (s *Service) Closing(ctx context.Context, human string, page Page) (Sitting, error) {
	return s.ClosingIn(ctx, human, "", "", page)
}

// ClosingIn asks for the closing deposit on one conversation's sitting. A
// review's close carries the verdict the human chose on the End sheet (g1-s65
// D10), which the drafted Outcome opens with.
func (s *Service) ClosingIn(ctx context.Context, human, where, verdict string, page Page) (Sitting, error) {
	return s.ClosingAt(ctx, human, where, verdict, "", page)
}

// ClosingAt is ClosingIn for a review whose record names the tip it reviews:
// the outcome the closing turn offers carries that tip beside the verdict
// (g1-s69 D1), and the recorder writes it into the Outcome as its Reviewed at
// line.
func (s *Service) ClosingAt(ctx context.Context, human, where, verdict, tip string, page Page) (Sitting, error) {
	conversation, err := s.conversationOf(human, where)
	if err != nil {
		return Sitting{}, err
	}
	sitting := conversation.Sitting()
	if sitting == nil {
		return Sitting{}, errors.New(noSittingToClose)
	}
	request, chosen := ClosingRequest(*sitting), ""
	if sitting.Purpose == PurposeReview {
		said, err := ReviewClosingRequest(*sitting, verdict)
		if err != nil {
			return Sitting{}, err
		}
		request, chosen = said, verdict
	} else {
		tip = ""
	}
	if _, err := s.submitClosingAt(ctx, human, where, "", request, page, true, chosen, tip); err != nil {
		return Sitting{}, err
	}
	return *sitting, nil
}

// Resume asks the opening question again where a standing sitting has outlived
// the Partner's session, and reports whether it did (g1-s55 D3).
//
// This is the server's one freshness decision, and it owns the opening turn for
// Start and for resume alike. It is here rather than in a browser effect for the
// reason the mark on the turn itself exists: an effect that submitted a turn
// would submit one per tab, per reload and per remount, and the interface would
// be asking questions in a human's name that nobody can count. The decision is
// one function, in the process that knows whether a session is live.
//
// A session that is still alive remembers this sitting, so nothing is asked: the
// transcript is on the screen and the table is the record's. A turn already
// running is the resume that a second tab has just asked for, or the human's own
// question, and either way one more would be refused as busy.
func (s *Service) Resume(ctx context.Context, human string) (bool, error) {
	return s.ResumeIn(ctx, human, "")
}

// ResumeIn is Resume for one conversation. A live session that belongs to
// another conversation does not remember this sitting, so it is resumed as a
// session that ended would be — unless a turn is running anywhere, because a
// resume must never be the thing that stops somebody's answer.
func (s *Service) ResumeIn(ctx context.Context, human, where string) (bool, error) {
	conversation, err := s.conversationOf(human, where)
	if err != nil {
		return false, err
	}
	s.mu.Lock()
	own := s.live == conversationKey(human, conversation.key)
	s.mu.Unlock()
	if conversation.Sitting() == nil || (own && s.host.Alive()) || s.Busy() {
		return false, nil
	}
	sitting := conversation.Sitting()
	if _, err := s.submit(ctx, human, where, "", ResumingRequest(*sitting), Page{}, true); err != nil {
		return false, err
	}
	return true, nil
}

// OpeningRequest is the one question this interface asks on a human's behalf.
//
// It is fixed, and it is here rather than in a browser, so that what the
// interface says in a human's name is one sentence a reader can find. It asks
// for what the records hold and it forbids the weighing: the paper's warning is
// that a preparation which quietly supplies the values is the approval habit in
// conversational form, and the first turn of a sitting is exactly where that
// would happen.
func OpeningRequest(sitting Sitting) string {
	if sitting.Purpose == PurposeReview {
		return ReviewOpeningRequest(sitting)
	}
	return "Open this sitting on " + sitting.Subject.ID + ", whose purpose is to " + sitting.Purpose + ".\n\n" +
		"Bring what the records already hold about it, and nothing else: the standing human rulings that touch it, " +
		"the decisions this project has recorded about it, the open questions on it, the intent and design records " +
		"that name it, and what its own four sections — Facts, Proposals, Decisions, Open questions — already carry. " +
		"Read them with your tools and anchor every claim about what this application does today in the application " +
		"itself.\n\n" +
		"Weigh nothing. Do not recommend, do not rank and do not say which option you would pick. " +
		"Where two recorded wishes conflict, say that they conflict and say where each is written down. " +
		"Where nothing is recorded, say that nothing is recorded rather than filling the gap.\n\n" +
		"As facts, decisions and open questions come up, offer each one with the deposit tool; the human records them."
}

// ClosingRequest is the second question this interface asks on a human's
// behalf: the closing deposit, when they press End the sitting (g1-s55 D2).
//
// It is fixed and it is here for the opening request's reason — what the
// interface says in a human's name is one sentence a reader can find — and it
// forbids the weighing for the same reason too. The close is where a
// preparation that quietly supplied the values would do the most damage: it is
// the draft that goes into the record as the sitting's result.
//
// It asks for one deposit and names its kind, because the card the human then
// presses Record it on is the record's Outcome section, and a close that
// offered four cards would be four writes of one thing.
func ClosingRequest(sitting Sitting) string {
	return "Close this sitting on " + sitting.Subject.ID + ". Draft its closing deposit and offer it with the " +
		"deposit tool as one deposit of kind outcome, and offer nothing else.\n\n" +
		"It carries, in this order: the outcome as decided; the constraints it must hold to; the open questions " +
		"with the consequence of leaving each one open; and what the table holds — the record's own four sections, " +
		"Facts, Proposals, Decisions, Open questions, as they now stand. Draft it from those sections and from this " +
		"conversation, and from nothing else: a closing deposit is what was decided here, not what you would " +
		"have decided.\n\n" +
		"Weigh nothing and settle nothing. Where the sitting left something open, say it is open and say what " +
		"follows from that. Where nothing was decided about something, say that nothing was.\n\n" +
		"The human reads it, edits it as they like, and presses Record it; it becomes the record's Outcome " +
		"section then and not before."
}

// ResumingRequest is the opening request again, said as a resuming (g1-s55 D3).
//
// The words the Partner is given have to say which of the two this is, because
// the two are not the same question: opening a sitting is the first turn of a
// conversation, and resuming one is a session that has ended being asked to
// pick up a sitting that has not. The records are read again either way — the
// sitting's memory is the record, and this is what that is for.
func ResumingRequest(sitting Sitting) string {
	return "Resuming this sitting: your session ended, so nothing of it is in your memory, " +
		"and the record is what it left behind.\n\n" + OpeningRequest(sitting)
}

// admitDeposit decides whether one prepared deposit is offered to the human.
//
// This is the one place that can decide it, because this is the one place that
// knows whether there is a sitting. The tool server has no conversation and the
// host has no turn; which record this human is sitting on is on the conversation
// this turn belongs to, and nothing else in this process knows it.
//
// A deposit prepared with no sitting open is not offered, and the refusal is
// recorded as the deposit it was, with its reason, rather than dropped to an
// activity line the drawer does not show — a Partner that deposited into nothing
// has to be visible where the human reads the answer. A refused one is never
// stamped with a subject, because a subject on it is the one thing that could
// let a page offer Record it for words no record is waiting for.
func (s *Service) admitDeposit(running *turn, prepared Deposit) {
	s.mu.Lock()
	conversation := running.conversation
	s.mu.Unlock()
	sitting := conversation.Sitting()
	if sitting == nil {
		prepared.Offered = false
		prepared.NotOffered = noSitting
		s.record(running, Event{Kind: EventDeposit, Deposit: &prepared})
		return
	}
	// A finding is the review's own pile (g1-s65 D8), and the piles are a
	// property of the record's kind: a finding offered to a sitting that shapes
	// a design has nowhere to go, and says so where the card would be.
	if prepared.Kind == DepositFinding && sitting.Purpose != PurposeReview {
		prepared.Offered = false
		prepared.NotOffered = noFindingHere
		s.record(running, Event{Kind: EventDeposit, Deposit: &prepared})
		return
	}
	prepared.Subject = sitting.Subject
	// The verdict is the service's to write, never the Partner's: it is the
	// one the closing turn was asked with, and only on a review's outcome.
	prepared.Verdict, prepared.Tip = "", ""
	if prepared.Kind == DepositOutcome && sitting.Purpose == PurposeReview {
		prepared.Verdict = running.verdict
		prepared.Tip = running.tip
	}
	prepared.Offered = true
	s.record(running, Event{Kind: EventDeposit, Deposit: &prepared})
}

// freshLine says what a fresh session was given, so a human can see why the
// Partner might have forgotten something.
func freshLine(given int) string {
	return "The Partner's session had ended, so a fresh one was opened and given the last " +
		strconv.Itoa(given) + " messages of this conversation."
}

// run drives one turn and writes its outcome into the transcript.
func (s *Service) run(running *turn, prompt string) {
	result, err := s.host.Prompt(context.Background(), prompt, func(update Update) {
		switch update.Kind {
		case UpdateText:
			s.record(running, Event{Kind: EventText, Text: update.Text})
		case UpdateActivity:
			s.record(running, Event{Kind: EventActivity, Text: update.Text})
		case UpdateDoing:
			s.record(running, Event{Kind: EventDoing, Text: update.Text})
		case UpdateLook:
			if update.Look != nil {
				s.record(running, Event{Kind: EventLook, Look: update.Look})
			}
			// The call that prepared words is accounted for as a look like any
			// other, and then the words are admitted or refused against the
			// capture this turn was asked with.
			if update.Suggestion != nil {
				s.admit(running, *update.Suggestion)
			}
			// The same, against the sitting this conversation is.
			if update.Deposit != nil {
				s.admitDeposit(running, *update.Deposit)
			}
			// And the same, against the ledger reading this turn was composed
			// from: an act on a goal nothing carries is an act that would be
			// refused, and the human is told so on the card.
			if update.Action != nil {
				s.admitProposal(running, *update.Action)
			}
			// And the same, against the review this conversation is.
			if update.Present != nil {
				s.admitPresent(running, *update.Present)
			}
		}
	})
	if err != nil {
		result = Result{Outcome: OutcomeFailed, Detail: err.Error()}
	}

	s.mu.Lock()
	stopping := running.stopping
	s.mu.Unlock()
	if stopping && result.Outcome == OutcomeComplete {
		// The answer landed inside the cancellation's settlement: the human
		// asked it to stop and it finished anyway, which is a complete answer
		// and is kept as one.
		stopping = false
	}
	if stopping && result.Outcome != OutcomeStopped {
		result.Outcome = OutcomeStopped
	}

	s.mu.Lock()
	text := running.text.String()
	activity := append([]string{}, running.activity...)
	looked := append([]Look{}, running.looked...)
	suggestions := append([]Suggestion{}, running.suggestions...)
	deposits := append([]Deposit{}, running.deposits...)
	proposals := append([]Proposal{}, running.proposals...)
	s.mu.Unlock()

	answered := Message{ID: mintTurn(), Turn: running.id, Role: RolePartner, Text: text,
		At: s.now().UTC().Format(time.RFC3339), Outcome: result.Outcome,
		Detail: result.Detail, Activity: activity, Looked: looked,
		Suggestions: suggestions, Deposits: deposits, Proposals: proposals}
	_ = s.appendMessage(running, answered)

	// Nothing is running BEFORE the terminal beat goes out, deliberately. A
	// page that reads the conversation after that beat must be told the turn
	// has ended; a page that reads it before is told the turn is running, and
	// the beat that follows settles it. The other order leaves a window in
	// which a snapshot resurrects a turn no beat will ever end.
	s.mu.Lock()
	if s.current == running {
		s.current = nil
	}
	s.mu.Unlock()

	kind := EventDone
	switch result.Outcome {
	case OutcomeStopped:
		kind = EventStopped
	case OutcomeFailed, OutcomeRefused:
		kind = EventError
	}
	s.record(running, Event{Kind: kind, Text: result.Detail})
	s.said(false)
	close(running.done)
}

// appendMessage writes one of a turn's two messages — the question it was
// admitted with, then the answer — into the conversation the turn is bound to
// at the moment of the write.
//
// The binding is taken under the conversation's own lock and checked under the
// service's, because a first sign-in moves it while the answer is being
// composed: Adopt holds both transcripts for the whole move and rebinds the
// turn inside them, so a write that read the binding a moment earlier would
// append the answer to the seat the move has just emptied.
//
// Either order is right, and the check is what makes it one or the other rather
// than both. The answer that gets the seat first is written before the move and
// travels with it; the answer that arrives after finds the human's conversation
// and lands where its question already is.
func (s *Service) appendMessage(running *turn, message Message) error {
	for {
		s.mu.Lock()
		conversation := running.conversation
		s.mu.Unlock()
		conversation.mu.Lock()
		s.mu.Lock()
		moved := running.conversation != conversation
		s.mu.Unlock()
		if moved {
			conversation.mu.Unlock()
			continue
		}
		err := conversation.appendHeld(message)
		conversation.mu.Unlock()
		return err
	}
}

// record numbers one event against its turn, folds it into the turn's own
// state so a reload can read it, and publishes it.
func (s *Service) record(running *turn, event Event) {
	s.mu.Lock()
	running.seq++
	event.Turn = running.id
	event.Seq = running.seq
	event.Conversation = running.where
	event.At = s.now().UTC().Format(time.RFC3339)
	switch event.Kind {
	case EventText:
		running.text.WriteString(event.Text)
	case EventActivity:
		running.activity = append(running.activity, event.Text)
	case EventDoing:
		running.doing = event.Text
	case EventLook:
		if event.Look != nil {
			running.looked = append(running.looked, *event.Look)
		}
	case EventSuggestion:
		if event.Suggestion != nil {
			running.suggestions = append(running.suggestions, *event.Suggestion)
		}
	case EventDeposit:
		if event.Deposit != nil {
			running.deposits = append(running.deposits, *event.Deposit)
		}
	case EventProposal:
		if event.Proposal != nil {
			running.proposals = append(running.proposals, *event.Proposal)
		}
	}
	s.mu.Unlock()
	s.publish(event)
}

// publish hands one event to every open watcher.
//
// It is apart from record because not every beat belongs to a running turn. The
// outcome of a proposed action is written long after the answer ended — it is a
// human pressing Apply on a card, or on a row of the inbox — and the pages that
// have that card open have to be told, or the second tab would go on offering an
// act the first one has already made (g1-s60 D5).
func (s *Service) publish(event Event) {
	s.mu.Lock()
	watchers := make([]chan Event, 0, len(s.watchers))
	for _, watcher := range s.watchers {
		watchers = append(watchers, watcher)
	}
	s.mu.Unlock()
	for _, watcher := range watchers {
		select {
		case watcher <- event:
		default:
			// A page that is not reading its stream is a page that will
			// re-read the snapshot when it reconnects, which is exactly the
			// recovery this design already carries. Blocking here would stop
			// the turn for everyone.
		}
	}
}

// Stop cancels the running turn, if the id names it. A stop for a turn that
// has already ended is not an error: the page and the server raced, and the
// page is about to be told the turn ended. A turn the runtime does not write
// down within the settle wait answers ErrUnsettled.
func (s *Service) Stop(ctx context.Context, id string) error {
	s.mu.Lock()
	running := s.current
	if running == nil || running.id != id {
		s.mu.Unlock()
		return nil
	}
	running.stopping = true
	s.mu.Unlock()
	// The route answers with the snapshot, so the turn has to be written down
	// before it does; otherwise the page is handed a running turn whose
	// terminal beat it has already seen. A turn not written down within the one
	// settle wait is said, never a success: Step out stays in the room on it
	// rather than leaving an answer running behind the human (Sol SOL-A-06).
	if !s.stopAndSettle(ctx, running) {
		return ErrUnsettled
	}
	return nil
}

// stopAndSettle stops one running turn through the runtime's cancellation and
// reports whether it was written down within the wait. It is the handoff's half
// of Stop (S65-06): the turn stays bound to its own conversation, run writes it
// down there as stopped, and only a settled turn lets another conversation's
// session open.
func (s *Service) stopAndSettle(ctx context.Context, running *turn) bool {
	s.mu.Lock()
	wait := s.settle
	s.mu.Unlock()
	// The whole handoff is bounded by the one wait: the cancellation reaching
	// the runtime and the turn's terminal write both. A runtime that does not
	// answer within it is the refusal, never a longer wait.
	bounded, cancel := context.WithTimeout(ctx, wait)
	defer cancel()
	if err := s.host.Stop(bounded); err != nil {
		return false
	}
	return s.settled(bounded, running)
}

// settled waits, bounded, for one turn's terminal write.
func (s *Service) settled(ctx context.Context, running *turn) bool {
	s.mu.Lock()
	wait := s.settle
	s.mu.Unlock()
	select {
	case <-running.done:
		return true
	default:
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-running.done:
		return true
	case <-ctx.Done():
		return false
	case <-timer.C:
		return false
	}
}

// settleWait bounds the wait for a stopped turn to be written down. Past it
// the route answers with what it can see, and the stream settles the page.
const settleWait = 30 * time.Second

// Subscribe opens one watcher on the event stream and returns it with the way
// to close it. The channel is buffered because the stream's writer and the
// turn's pump run at different speeds and the pump must never wait.
func (s *Service) Subscribe() (<-chan Event, func()) {
	events := make(chan Event, 256)
	s.mu.Lock()
	s.nextID++
	id := s.nextID
	s.watchers[id] = events
	s.mu.Unlock()
	return events, func() {
		s.mu.Lock()
		delete(s.watchers, id)
		s.mu.Unlock()
	}
}

// Close ends the runtime's process. The conversation stays where it is.
func (s *Service) Close() { s.host.Close() }

// mintTurn is one identifier: sixteen random bytes, base64 without padding,
// which sorts by nothing and collides with nothing.
func mintTurn() string {
	var value [16]byte
	_, _ = rand.Read(value[:])
	return base64.RawURLEncoding.EncodeToString(value[:])
}
