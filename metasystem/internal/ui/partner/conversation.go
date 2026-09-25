package partner

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// The conversation: one per human per checkout, owned here and nowhere else.
//
// Astra's sixth finding is what this file answers: a transcript of unspecified
// messages cannot tell a page whether a question was accepted, whether an
// answer is complete, or whether Retry would submit it twice. So one owner
// decides turn identity, admission, busy state, partial text and the terminal
// outcome, and the page reads all five from it.
//
// It is kept as one JSON line per message, under the state root's artifacts,
// beside a small state file that names the live session. A line is appended
// and never rewritten, so a reader that is halfway through the file reads a
// prefix of the truth rather than a torn record.

// Relative is where a checkout keeps its Partner conversations, under the
// state root. It is the agents' own artifacts directory, one directory deeper,
// because this is the interface's agent and not the steward's.
const Relative = "artifacts/agents/ui/partner"

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
	// Key is the client-minted turn key, on a human's message only. It is
	// what makes a retry after a lost answer the same turn rather than a
	// second one, and it is kept in the file so a restart cannot forget it.
	Key string `json:"key,omitempty"`
	// Page is where the human was, on a human's message only.
	Page *Page `json:"page,omitempty"`
}

// Conversation is the transcript and the state beside it.
type Conversation struct {
	transcript string
	state      string

	mu       sync.Mutex
	messages []Message
	session  string
}

// stateFile is what the small file holds: the live session's id, so a build
// that learns to load sessions natively has it, and the moment it was written.
type stateFile struct {
	Human     string `json:"human"`
	Session   string `json:"session"`
	UpdatedAt string `json:"updatedAt"`
}

// OpenConversation reads one human's conversation under a state root, creating
// the directory if it is not there. A file that cannot be read is a refusal
// rather than an empty conversation: a transcript that silently starts over is
// a transcript nobody can trust.
func OpenConversation(stateRoot, human string) (*Conversation, error) {
	name := fileName(human)
	directory := filepath.Join(stateRoot, filepath.FromSlash(Relative))
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return nil, fmt.Errorf("the Partner's conversation directory could not be made: %w", err)
	}
	conversation := &Conversation{
		transcript: filepath.Join(directory, name+".jsonl"),
		state:      filepath.Join(directory, name+".json"),
	}
	if err := conversation.load(); err != nil {
		return nil, err
	}
	return conversation, nil
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
	reader.Buffer(make([]byte, 0, 64*1024), maxMessageBytes+1024)
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
		return fmt.Errorf("the Partner's transcript could not be read: %w", err)
	}
	return c.readState()
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
	return nil
}

// maxMessageBytes bounds one stored message. A model that answers with a
// megabyte is a model whose answer is kept to the bound and said to be.
const maxMessageBytes = 256 << 10

// Append writes one message and keeps it.
func (c *Conversation) Append(message Message) error {
	if len(message.Text) > maxMessageBytes {
		message.Text = message.Text[:maxMessageBytes] + "\n\n[the rest of this answer was longer than the transcript keeps]"
	}
	body, err := json.Marshal(message)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
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
	body, err := json.Marshal(stateFile{
		Session:   session,
		UpdatedAt: now.UTC().Format(time.RFC3339),
	})
	if err != nil {
		return
	}
	_ = os.WriteFile(c.state, append(body, '\n'), 0o600)
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
