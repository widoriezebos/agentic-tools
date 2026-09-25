package partner

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
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
}

// The seven event kinds.
const (
	EventText     = "text"
	EventActivity = "activity"
	EventDoing    = "doing"
	EventLook     = "look"
	EventDone     = "done"
	EventError    = "error"
	EventStopped  = "stopped"
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
}

// turn is the running turn's state, which the snapshot reads and the events
// are numbered against.
type turn struct {
	id       string
	human    string
	key      string
	seq      int
	text     strings.Builder
	activity []string
	doing    string
	looked   []Look
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
func (s *Service) conversation(human string) (*Conversation, error) {
	s.mu.Lock()
	held, known := s.conversations[human]
	s.mu.Unlock()
	if known {
		return held, nil
	}
	opened, err := s.open(human)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	if again, raced := s.conversations[human]; raced {
		opened = again
	} else {
		s.conversations[human] = opened
	}
	s.mu.Unlock()
	return opened, nil
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
	}
	s.mu.Unlock()
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
	id := mintTurn()
	running := &turn{id: id, human: human, key: key, done: make(chan struct{})}
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
		At: now.Format(time.RFC3339), Key: key, Page: &page}
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
	s.mu.Unlock()

	answered := Message{ID: mintTurn(), Turn: running.id, Role: RolePartner, Text: text,
		At: s.now().UTC().Format(time.RFC3339), Outcome: result.Outcome,
		Detail: result.Detail, Activity: activity, Looked: looked}
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
