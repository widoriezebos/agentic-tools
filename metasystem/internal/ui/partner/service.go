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

	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/overview"
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
	Seq  int    `json:"seq"`
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
	ReadOnly string    `json:"readOnly"`
	Messages []Message `json:"messages"`
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
	spokeLast     string
	current       *turn
	watchers      map[int]chan Event
	nextID        int
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
	// page is the capture this turn was asked with, held to its bounds. It is
	// the one thing that says which editor opening the human handed over and
	// which of its fields they may be written into, so it is what a suggestion
	// is admitted against.
	page     Page
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
		watchers:      map[int]chan Event{},
	}
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
func (s *Service) conversation(human string) (*Conversation, error) {
	key := fileName(human)
	s.mu.Lock()
	held, known := s.conversations[key]
	s.mu.Unlock()
	if known {
		return held, nil
	}
	opened, err := s.open(human)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	if again, raced := s.conversations[key]; raced {
		opened = again
	} else {
		s.conversations[key] = opened
	}
	s.mu.Unlock()
	return opened, nil
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

// Snapshot answers the read route for one human.
func (s *Service) Snapshot(human string, limit int) (Snapshot, error) {
	conversation, err := s.conversation(human)
	if err != nil {
		return Snapshot{}, err
	}
	s.mu.Lock()
	running := s.current
	answer := Snapshot{
		Runtime: s.runtime.Name, Model: s.runtime.Model, Human: human,
		ReadOnly: s.runtime.ReadOnly,
	}
	if running != nil && running.human == human {
		answer.Busy = true
		answer.Turn = running.id
		answer.Partial = running.text.String()
		answer.PartialSeq = running.seq
		answer.Activity = append([]string{}, running.activity...)
		answer.Doing = running.doing
		answer.Looked = append([]Look{}, running.looked...)
		answer.Suggestions = append([]Suggestion{}, running.suggestions...)
		answer.Deposits = append([]Deposit{}, running.deposits...)
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

// Submit admits one turn.
//
// The same key twice is the same turn once, so a retry after a lost answer
// never submits twice. A send while busy is refused and the draft stays. A
// runtime that cannot start refuses here, before the turn exists, so the
// human's question stays in the composer and the route answers 503 with the
// runtime's own words.
func (s *Service) Submit(ctx context.Context, human, key, text string, page Page) (string, error) {
	return s.submit(ctx, human, key, text, page, false)
}

// submit is Submit with the one thing only this package may decide: whether the
// question is the human's own or one this interface asked on their behalf.
//
// Nothing outside this package can set that mark, and nothing outside it should
// be able to: a browser that could claim a question was the interface's could
// dress up a question the human typed as one they did not.
func (s *Service) submit(ctx context.Context, human, key, text string, page Page, byInterface bool) (string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", errors.New("a turn needs a question")
	}
	// The capture is held to its bounds before anything is done with it, and
	// above all before it is written into the transcript: what a message keeps
	// is what this server was prepared to keep.
	page = page.Bound()
	conversation, err := s.conversation(human)
	if err != nil {
		return "", err
	}
	if existing, known := conversation.TurnFor(key); known {
		return existing, nil
	}
	s.mu.Lock()
	if s.current != nil {
		s.mu.Unlock()
		return "", ErrBusy
	}
	// A turn from somebody other than whoever spoke last ends the live
	// session: the process carries one conversation, and it is not this one.
	changed := s.spokeLast != "" && s.spokeLast != human
	s.mu.Unlock()
	if changed {
		s.host.Close()
	}

	// Starting the process is part of admitting the turn, not part of running
	// it: a runtime that is not installed or not signed in must refuse the
	// send rather than accept a turn it cannot take.
	fresh, err := s.host.Ready(ctx)
	if err != nil {
		return "", err
	}

	// The live session's id is recorded beside the transcript, so a build that
	// learns to load sessions natively has it without guessing.
	conversation.RecordSession(s.host.Session(), s.now())

	s.mu.Lock()
	if s.current != nil {
		s.mu.Unlock()
		return "", ErrBusy
	}
	// A session opened outside a turn has been given no prompt, so this turn is
	// its first however Ready answered just now.
	if s.opened {
		fresh = true
		s.opened = false
	}
	id := mintTurn()
	running := &turn{id: id, human: human, key: key, page: page, done: make(chan struct{})}
	s.current = running
	s.spokeLast = human
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
	if err := conversation.Append(asked); err != nil {
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

	prompt := ComposeOpening(seen, page, human, opening) + "\n\n"
	if given > 0 {
		prompt += history + "\n\n"
		s.record(running, Event{Kind: EventActivity, Text: freshLine(given)})
	}
	prompt += "The human asks:\n" + text

	s.said(true)
	go s.run(running, conversation, prompt)
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
	if purpose != PurposeShapeIntent && purpose != PurposeShapeDesign {
		return fmt.Errorf(
			"a sitting is for %q or %q; review and learning sittings are not in this build",
			PurposeShapeIntent, PurposeShapeDesign)
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
	conversation, err := s.conversation(human)
	if err != nil {
		return Sitting{}, err
	}
	previous := conversation.Sitting()
	sitting := Sitting{Subject: subject, Purpose: purpose, StartedAt: s.now().UTC().Format(time.RFC3339)}
	if err := conversation.Sit(sitting, s.now()); err != nil {
		return Sitting{}, err
	}
	if _, err := s.submit(ctx, human, "", OpeningRequest(sitting), page, true); err != nil {
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
	conversation, err := s.conversation(human)
	if err != nil {
		return err
	}
	return conversation.Rise(s.now())
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
func (s *Service) admitDeposit(running *turn, conversation *Conversation, prepared Deposit) {
	sitting := conversation.Sitting()
	if sitting == nil {
		prepared.Offered = false
		prepared.NotOffered = noSitting
		s.record(running, Event{Kind: EventDeposit, Deposit: &prepared})
		return
	}
	prepared.Subject = sitting.Subject
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
func (s *Service) run(running *turn, conversation *Conversation, prompt string) {
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
				s.admitDeposit(running, conversation, *update.Deposit)
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
	s.mu.Unlock()

	answered := Message{ID: mintTurn(), Turn: running.id, Role: RolePartner, Text: text,
		At: s.now().UTC().Format(time.RFC3339), Outcome: result.Outcome,
		Detail: result.Detail, Activity: activity, Looked: looked,
		Suggestions: suggestions, Deposits: deposits}
	_ = conversation.Append(answered)

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

// record numbers one event against its turn, folds it into the turn's own
// state so a reload can read it, and publishes it.
func (s *Service) record(running *turn, event Event) {
	s.mu.Lock()
	running.seq++
	event.Turn = running.id
	event.Seq = running.seq
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
	}
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
// page is about to be told the turn ended.
func (s *Service) Stop(ctx context.Context, id string) error {
	s.mu.Lock()
	running := s.current
	if running == nil || running.id != id {
		s.mu.Unlock()
		return nil
	}
	running.stopping = true
	s.mu.Unlock()
	if err := s.host.Stop(ctx); err != nil {
		return err
	}
	// The route answers with the snapshot, so the turn has to be written down
	// before it does; otherwise the page is handed a running turn whose
	// terminal beat it has already seen.
	select {
	case <-running.done:
	case <-ctx.Done():
	case <-time.After(settleWait):
	}
	return nil
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
