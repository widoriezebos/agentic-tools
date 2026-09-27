package partner

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/backlog"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/snapshot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/uitools"
)

// What happens to an action between the Partner preparing it and the ledger
// moving, and where that is written down.
//
// The card in the transcript is not the record of a proposal: the message is.
// An approve applied twice is two approval records rather than a no-op, so a
// page that forgot a line had been applied would invite the one press that does
// harm — and a page is reloaded, opened in a second tab, and left open for a
// day. So every state a line passes through is written onto the Partner's own
// message, through one route, and the card reads its lines from there.
//
// The version below is what makes two tabs safe. Comparing states cannot tell a
// fresh attempt from an abandoned one once a line has returned to `applying`:
// two tabs that both show a line in flight would both be allowed to write its
// outcome. So each entry carries a version, the caller sends the version it
// last rendered, and the writer admits the write only against that version. The
// loser is handed the entry as it stands and must show it.

// The six states one proposed action passes through.
//
// Waiting is the offer. Applying is written BEFORE the act is sent, so a reload
// during a run shows the line that was in flight as having been in flight
// rather than as fresh. Applied, refused and unresolved are what the act
// answered, in the act layer's own three distinctions. Dismissed is the human
// putting the card away.
const (
	ProposalWaiting    = "waiting"
	ProposalApplying   = "applying"
	ProposalApplied    = "applied"
	ProposalRefused    = "refused"
	ProposalUnresolved = "unresolved"
	ProposalDismissed  = "dismissed"
)

// ProposalStates is the six, in the order a line passes through them. The
// outcome route admits a state that is one of these and no other.
var ProposalStates = []string{
	ProposalWaiting, ProposalApplying, ProposalApplied,
	ProposalRefused, ProposalUnresolved, ProposalDismissed,
}

// ProposalState reports whether a word is one of the six.
func ProposalState(state string) bool {
	for _, known := range ProposalStates {
		if known == state {
			return true
		}
	}
	return false
}

// proposalTransitions is what a line may become, from what it is.
//
// Every pair changes the state, and that is the rule rather than a
// coincidence: the version's compare-and-set is what makes a write exclusive,
// and a pair that left the state where it was would let two writers each
// believe they had won. Try again on a line the page shows in flight is
// therefore one write, `applying` to `applying`, exclusive because the version
// moves under it.
//
// Reading the table: to the state named by the key, from any of the states
// listed. `applied`, `refused` and `unresolved` come only from `applying`,
// because they are what an act answered and nothing sends an act without
// recording `applying` first. `dismissed` comes from anything unsettled,
// including a line left in flight: a human putting a card away is allowed to
// put away a line nobody can settle any more.
var proposalTransitions = map[string][]string{
	ProposalApplying:   {ProposalWaiting, ProposalRefused, ProposalUnresolved, ProposalApplying},
	ProposalApplied:    {ProposalApplying},
	ProposalRefused:    {ProposalApplying},
	ProposalUnresolved: {ProposalApplying},
	ProposalDismissed:  {ProposalWaiting, ProposalRefused, ProposalUnresolved, ProposalApplying},
}

// ProposalMayBecome reports whether a line in one state may be written into
// another.
func ProposalMayBecome(from, to string) bool {
	for _, allowed := range proposalTransitions[to] {
		if allowed == from {
			return true
		}
	}
	return false
}

// ProposalRead is the goal as the Partner read it, kept with an action whose
// act binds what it finds at the tip.
//
// It is on an approve and an edit and on nothing else, because those are the two
// acts whose meaning depends on what the goal says: an approval authorises the
// goal as it stands when the transaction runs, and an edit replaces fields of
// it. The runner compares this against the canonical branch before it sends, so
// a card read yesterday cannot authorise work nobody read.
type ProposalRead struct {
	Intent   string   `json:"intent"`
	NextStep string   `json:"nextStep"`
	Tier     int      `json:"tier"`
	Labels   []string `json:"labels"`
}

// Proposal is one admitted or refused action, as the message persists it, the
// stream carries it and the card renders it.
type Proposal struct {
	// Index is this action's place in its answer, which is how the outcome
	// route names it. It is the index of every action the answer carried,
	// refused ones included, so a refusal cannot renumber the lines after it.
	Index int    `json:"index"`
	Verb  string `json:"verb"`
	Goal  string `json:"goal"`
	// Title is the subject as the pages say it, stamped by the service from the
	// tip. The card never titles a goal from the Partner's prose.
	Title  string            `json:"title"`
	Fields map[string]string `json:"fields,omitempty"`
	// Read is the goal as it stood when the action was admitted, on an approve
	// and an edit, and null everywhere else.
	Read *ProposalRead `json:"read"`
	Why  string        `json:"why,omitempty"`
	// Offered says whether the human was shown it as something to apply.
	//
	// A refused one travels rather than being dropped, for the reason a refused
	// deposit does: a human who asked for six goals to be put away and got five
	// cards has to be able to read why the sixth is not there (g1-s52 D3).
	Offered bool `json:"offered"`
	// Reason is why it was not offered, in the words a human reads, and "" for
	// one that was.
	Reason string `json:"reason,omitempty"`
	State  string `json:"state"`
	// Words are what the last state change said: the engine's own sentence on a
	// refusal, what was said on an unresolved answer, "" on a plain one.
	Words string `json:"words,omitempty"`
	// At is when this action was PROPOSED: the instant the service admitted it,
	// stamped once and never again.
	//
	// It used to be the instant of the last write, and that was the wrong fact
	// for the one place it is read. An answered row sorted to the end of its
	// group and read "today", so the inbox's ages were the ages of the human's
	// own presses rather than of the asking, and "n new" counted a proposal from
	// last week as new because somebody had just refused it (Sol's read of
	// g1-s60, deferred). Every list that dates a proposal dates the asking.
	At string `json:"at"`
	// UpdatedAt is when this entry was last written, and "" on one nothing has
	// written since it was admitted. It is what At used to carry, kept beside it
	// rather than in place of it, because both facts are true of a line that was
	// proposed on Monday and refused on Friday.
	UpdatedAt string `json:"updatedAt,omitempty"`
	// Version is how many times this entry has been written, counting the
	// service's own admission as the first. Every press sends the version of
	// the entry it last rendered, and the writer admits the write only against
	// it, so two tabs cannot both move one line.
	Version int `json:"version"`
}

// Settled reports whether nothing more will happen to this line by itself. It
// is what the runner asks of a line it lost a race on: a settled line is one to
// go past, and an unsettled one is one to stop at.
func (p Proposal) Settled() bool {
	switch p.State {
	case ProposalApplied, ProposalRefused, ProposalDismissed:
		return true
	default:
		return false
	}
}

// admitProposal decides whether one prepared action is offered to the human.
//
// This is the one place that can decide it, because this is the one place that
// holds the reading the turn was composed from. The tool server has no ledger
// reading — it validates shape and bounds and says so — and the host has no
// turn; whether the goals an action names are at the accepted tip is a fact
// about the ledger this turn was composed against, and admitting against a
// later reading would let a card be refused for a goal the Partner was told
// about.
//
// Admission checks EXISTENCE and nothing else. Whether the act is allowed in the
// goal's state is the engine's answer at the act, in its own words, exactly as
// it is for every button on every page: a page that refused a park because the
// goal looked claimed a minute ago would be a page guessing at the engine's
// rules.
//
// A goal an `open` admitted earlier in the same answer counts as being there,
// so "open X, then say Y waits for X" is one proposal a human applies in order.
// An `open` of an id the tip already carries is refused: the act would be.
func (s *Service) admitProposal(running *turn, prepared Action) {
	s.mu.Lock()
	observed := running.observed
	index := len(running.proposals)
	opened := map[string]bool{}
	for _, earlier := range running.proposals {
		if earlier.Offered && earlier.Verb == uitools.ProposeOpen {
			opened[earlier.Goal] = true
		}
	}
	s.mu.Unlock()

	admitted := Proposal{
		Index: index, Verb: prepared.Verb, Goal: prepared.Goal, Fields: prepared.Fields,
		Why: prepared.Why, State: ProposalWaiting, Version: 1,
		At: s.now().UTC().Format(time.RFC3339),
	}
	rows := rowsAt(observed)
	subject, atTheTip := rows[prepared.Goal]

	if prepared.Verb == uitools.ProposeOpen {
		if atTheTip {
			s.refuseProposal(running, admitted, "goal "+prepared.Goal+" already exists")
			return
		}
		// An open's title is the intent's first sentence, which is what the
		// pages call a goal that has one.
		admitted.Title = titleFrom(prepared.Fields[uitools.FieldIntent], prepared.Goal)
	} else {
		if !atTheTip && !opened[prepared.Goal] {
			s.refuseProposal(running, admitted, "the accepted tip carries no goal "+prepared.Goal)
			return
		}
		admitted.Title = titleFrom(subject.Intent, prepared.Goal)
		if atTheTip && needsTheReading(prepared.Verb) {
			admitted.Read = &ProposalRead{
				Intent: subject.Intent, NextStep: subject.NextStep,
				Tier: int(subject.Tier), Labels: append([]string{}, subject.Labels...),
			}
		}
	}
	// The goal at the other end of an edge is a goal too, and an edge naming one
	// nobody has is an edge the act would refuse. It is checked after the subject
	// so the refusal names whichever end is missing.
	for _, named := range edgesNamedBy(prepared.Fields) {
		if _, there := rows[named]; !there && !opened[named] {
			s.refuseProposal(running, admitted, "the accepted tip carries no goal "+named)
			return
		}
	}
	admitted.Offered = true
	s.record(running, Event{Kind: EventProposal, Proposal: &admitted})
}

// edgesNamedBy is every OTHER goal one prepared action's fields name.
//
// Three fields carry one: the blocker a block or an unblock waits on, and the two
// lists an open takes — the goals it will wait for, and the goals that will wait
// for it. All three are the same claim, that a goal at the other end of an edge
// exists, and all three were not checked: the blocker was, and an open's lists
// went through unread, so a card could offer to open a goal waiting for a goal
// nobody has and the refusal arrived after the human pressed (Sol's read of
// g1-s58, deferred).
//
// The lists arrive as the tool joined them, comma-separated, because that is how
// the frame carries a list of names and how the route body reads one. An empty
// name is not a name: a trailing comma says nothing about a goal.
//
// In the frame's own order, so a refusal names the first missing end a reader of
// the card would look for rather than whichever the map happened to yield.
func edgesNamedBy(fields map[string]string) []string {
	named := []string{}
	for _, field := range []string{uitools.FieldBlocker, uitools.FieldBlockedBy, uitools.FieldBlocks} {
		for _, id := range strings.Split(fields[field], ",") {
			if trimmed := strings.TrimSpace(id); trimmed != "" {
				named = append(named, trimmed)
			}
		}
	}
	return named
}

// refuseProposal records one action the human is not offered, with its reason.
func (s *Service) refuseProposal(running *turn, refused Proposal, reason string) {
	refused.Offered = false
	refused.Reason = reason
	if refused.Title == "" {
		refused.Title = refused.Goal
	}
	s.record(running, Event{Kind: EventProposal, Proposal: &refused})
}

// needsTheReading says which acts carry the goal as it was read.
//
// Two do. An approval binds the goal as it stands when the transaction runs, so
// the card shows the intent and the next step it was proposed against and the
// runner refuses a line whose goal has moved; an edit replaces fields of the
// goal, so the same comparison keeps it from overwriting somebody else's change.
// The other seven say what they do without depending on what the goal says.
func needsTheReading(verb string) bool {
	return verb == uitools.ProposeApprove || verb == uitools.ProposeEdit
}

// rowsAt is the goals the accepted tip carries, by id, from the reading the
// turn was composed against. A reading that could not be made carries none,
// which refuses every action with the reason that names the goal — the truthful
// answer for a server that cannot see the ledger.
func rowsAt(observed snapshot.Observation) map[string]backlog.Row {
	held := map[string]backlog.Row{}
	if observed.Tree == nil {
		return held
	}
	board := backlog.Project(observed.Tree, observed.Horizon, observed.Admission)
	for _, row := range append(append([]backlog.Row{}, board.Rows...), board.Closed...) {
		held[row.ID] = row
	}
	return held
}

// titleFrom is what a goal is called: the first sentence of its intent, or its
// id where there is none.
//
// It is the Decisions page's own rule (internal/ui/decisions/decisions.go
// titleOf), written here rather than imported: the rule is four lines and the
// import would be this package taking a dependency on a page in order to spell
// a title. What must not happen is the card titling a goal from the Partner's
// prose, and that is what this prevents — the words come from the ledger or
// from the intent the action itself carries.
func titleFrom(intent, id string) string {
	trimmed := strings.TrimSpace(intent)
	if cut := strings.IndexAny(trimmed, ".\n"); cut > 0 {
		trimmed = strings.TrimSpace(trimmed[:cut])
	}
	if trimmed == "" {
		return id
	}
	return trimmed
}

/* ---------------------------------------------- what the Partner is told -- */

// proposalsBlock is what the next question carries about the actions the last
// two answers proposed: one line each, from the states the message records.
//
// It is the messages' own account and not the Partner's memory of what it
// prepared. A Partner that proposed six pauses and was told nothing would
// answer the next question as though six goals were away; what actually
// happened is that four applied, one was refused in the engine's words and one
// is still waiting for the human, and every one of those is something the next
// answer has to build on.
//
// Two answers, because that is the window in which the human is still talking
// about them. Older proposals are in the transcript and, from step 2, in the
// Decisions inbox; a block that carried every proposal of a long conversation
// would be spending the context on a list.
func proposalsBlock(messages []Message) string {
	carried := []Message{}
	for at := len(messages) - 1; at >= 0 && len(carried) < 2; at-- {
		if messages[at].Role == RolePartner && len(messages[at].Proposals) > 0 {
			carried = append(carried, messages[at])
		}
	}
	if len(carried) == 0 {
		return ""
	}
	var built strings.Builder
	built.WriteString("\nWhat happened to the actions you proposed\n")
	for at := len(carried) - 1; at >= 0; at-- {
		built.WriteString("- " + proposalsLine(carried[at].Proposals) + "\n")
	}
	built.WriteString("- Nothing here was applied by you: every one of them was the human's own press, " +
		"or is still waiting for one.\n")
	return built.String()
}

// proposalsLine is one answer's proposals, counted by state, with the refused
// ones named: a count alone would tell the Partner that something was refused
// and not what to do about it.
func proposalsLine(proposals []Proposal) string {
	counted := map[string]int{}
	named := []string{}
	for _, proposal := range proposals {
		if !proposal.Offered {
			counted["not offered"]++
			named = append(named, proposal.Verb+" "+proposal.Goal+": "+proposal.Reason)
			continue
		}
		counted[proposal.State]++
		if proposal.State == ProposalRefused || proposal.State == ProposalUnresolved {
			named = append(named, proposal.Verb+" "+proposal.Goal+": "+proposal.Words)
		}
	}
	said := []string{}
	for _, state := range append(append([]string{}, ProposalStates...), "not offered") {
		if counted[state] > 0 {
			said = append(said, strconv.Itoa(counted[state])+" "+state)
		}
	}
	line := "Of the " + strconv.Itoa(len(proposals)) + " actions you proposed, " + strings.Join(said, ", ")
	if len(named) > 0 {
		line += " (" + strings.Join(named, "; ") + ")"
	}
	return line + "."
}

/* --------------------------------------------- where an outcome is written -- */

// ProposalConflict is what a write to one proposed action loses to: somebody
// else moved that line first.
//
// It carries the entry as it stands, because that is the whole remedy. A second
// tab, or a second press in the same tab, has to be able to show the line as it
// really is rather than as it rendered it — and a runner that lost the race must
// not send the act, because the writer that won may have sent it already.
type ProposalConflict struct {
	Held Proposal
}

func (c *ProposalConflict) Error() string {
	return "this action is at version " + strconv.Itoa(c.Held.Version) + " and says " + c.Held.State +
		"; read it again before pressing"
}

// RecordProposal writes one state onto one proposed action of one answer, and
// answers the entry as it now stands.
//
// It is the whole of the compare-and-set that makes two tabs safe, and it is
// here because here is where the mutex over this transcript is. The caller sends
// the version of the entry it last rendered; the write is admitted only when the
// persisted version is still that one AND the pair of states is one the line may
// pass through; and the version moves under it, so the loser of a race is told
// with the entry rather than being allowed to write over the winner.
//
// The message is rewritten in place through the writer the trim uses: one
// publication of the whole file, so a reader halfway through it reads the old
// file whole rather than a torn mixture, and nothing else in the transcript
// moves.
func (c *Conversation) RecordProposal(turn string, index, version int, state, words string, now time.Time) (Proposal, error) {
	if !ProposalState(state) {
		return Proposal{}, errors.New("an action's state is " + strings.Join(ProposalStates, ", ") + ", not " + state)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	at := -1
	for index, message := range c.messages {
		if message.Role == RolePartner && message.Turn == turn {
			at = index
		}
	}
	if at < 0 {
		return Proposal{}, errors.New("this conversation has no answer for turn " + turn)
	}
	proposals := c.messages[at].Proposals
	if index < 0 || index >= len(proposals) {
		return Proposal{}, errors.New("that answer proposed " + strconv.Itoa(len(proposals)) +
			" actions, so it has no action " + strconv.Itoa(index))
	}
	held := proposals[index]
	if held.Version != version || !ProposalMayBecome(held.State, state) {
		return Proposal{}, &ProposalConflict{Held: held}
	}
	// The slice is copied before it is written into: the messages the snapshot
	// handed a reader a moment ago share this backing array, and a state written
	// through it would change what they already showed.
	rewritten := append([]Proposal{}, proposals...)
	held.State = state
	held.Words = words
	// The write's own instant, beside the asking's rather than over it: the
	// entry's At is when the action was proposed, which is what every list that
	// dates a proposal reads.
	held.UpdatedAt = now.UTC().Format(time.RFC3339)
	held.Version = version + 1
	rewritten[index] = held
	messages := append([]Message{}, c.messages...)
	messages[at].Proposals = rewritten
	if err := writeTranscript(c.transcript, messages); err != nil {
		return Proposal{}, err
	}
	c.messages = messages
	return held, nil
}

// Proposed records what happened to one proposed action, for the route that is
// the only way into it.
//
// A turn that is still running is refused in words: until the answer's terminal
// beat there is no message to record an outcome on, and the card's own buttons
// are asleep for exactly that reason. It is refused here rather than in the
// route because whether a turn is running is this service's own fact.
func (s *Service) Proposed(human, turn string, index, version int, state, words string) (Proposal, error) {
	s.mu.Lock()
	running := s.current
	s.mu.Unlock()
	if running != nil && running.id == turn {
		return Proposal{}, errors.New("the Partner is still answering this turn, so its actions cannot be " +
			"applied yet; they wake when the answer ends")
	}
	conversation, err := s.conversation(human)
	if err != nil {
		return Proposal{}, err
	}
	held, err := conversation.RecordProposal(turn, index, version, state, words, s.now())
	if err != nil {
		return Proposal{}, err
	}
	// Every admitted write is a beat, on the same stream the answer's own
	// proposals arrived on, so a transcript open in another tab — or the drawer
	// beside the inbox this press came from — folds the change into its card
	// without reading the conversation again (g1-s60 D5). It is the first time a
	// human's act, rather than the Partner's turn, publishes here; it is the same
	// event the page already folds, and nothing on the stream carries authority.
	//
	// It is published after the write and never before: the beat says what the
	// record now holds, and a beat for a write that was refused would tell every
	// other page something that did not happen.
	s.publish(Event{
		Turn: turn, Kind: EventProposal, Proposal: &held,
		At: s.now().UTC().Format(time.RFC3339),
	})
	return held, nil
}

/* ------------------------------------------- what is still waiting on you -- */

// Unsettled is one proposed action still waiting on the human, with the answer
// it was proposed in.
//
// It is what the Decisions inbox composes its "Proposed by the Partner" group
// from (g1-s60 D1), and it is a reading of the transcript rather than a second
// store: the proposals live on the Partner's messages, where the outcome route
// writes them, so the card in the conversation and the row in the inbox cannot
// disagree about one action.
//
// The turn travels because the index alone names nothing: an action is one
// answer's nth, which is how the outcome route addresses it and how the inbox's
// own row is identified.
type Unsettled struct {
	Turn string `json:"turn"`
	Proposal
}

// Unsettled is every action across this human's transcript that is still
// waiting on them, newest answer first.
//
// Four states are, and every one of them is a choice a human has to make.
// Waiting is the offer. Refused and unresolved are answers to act on — Try
// again, or Dismiss — and they carry the words the engine said, which is the
// whole of what a recovery rests on: a reader that returned waiting lines alone
// would take the row away at exactly the moment it started to carry an
// explanation (Astra S60-03). Applying is a line that was in flight when a page
// went away, and nothing will settle it by itself. Applied and dismissed are
// settled, and the card keeps them as the record of what was done.
//
// An action the human was never offered is not here either. A refusal at
// admission is something to read on the card, beside the answer that explains
// it, and not an act anybody can apply.
//
// It reads the transcript as the snapshot does and holds nothing. Newest answer
// first, because the answers are the order the transcript has and the inbox
// composes its own from the dates; within one answer the lines keep the order
// they were proposed in, which is the order a human is meant to apply them.
func (s *Service) Unsettled(human string) ([]Unsettled, error) {
	conversation, err := s.conversation(human)
	if err != nil {
		return nil, err
	}
	messages := conversation.Messages(0)
	held := []Unsettled{}
	for at := len(messages) - 1; at >= 0; at-- {
		message := messages[at]
		if message.Role != RolePartner {
			continue
		}
		for _, proposal := range message.Proposals {
			if !proposal.Offered || proposal.State == ProposalApplied || proposal.State == ProposalDismissed {
				continue
			}
			held = append(held, Unsettled{Turn: message.Turn, Proposal: proposal})
		}
	}
	return held, nil
}
