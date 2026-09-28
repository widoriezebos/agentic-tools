package partner

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
)

// The review sitting (g1-s65 slice A), as the service owns it.
//
// A review is a sitting like any other: a record with a conversation of its
// own, a mark on that conversation, and piles the human's presses write into.
// What is its own is here: the fixed requests the interface asks in the human's
// name — the opening, the five walks and the close with its verdict — the room's
// working state on the mark, the desk's display suggestion, and the list of
// every sitting that still stands.

// DepositFinding is the review's own deposit (D8): a finding with its anchor
// and the consequence of leaving it unanswered. It is the tool's kind, spelled
// here for the admission that keeps it to a review.
const DepositFinding = "finding"

// DepositOutcome is the closing deposit, spelled here for the admission that
// writes a review's verdict on it (D10).
const DepositOutcome = "outcome"

// noFindingHere is what a finding offered outside a review is refused with.
const noFindingHere = "a finding is offered in a review sitting, and this sitting shapes a record; " +
	"say it as a fact or an open question instead"

// Walks are the five parts of the narrator's report, in the order the room
// shows their presses (D6).
var Walks = []string{"asked", "built", "examined", "proven", "behaves"}

// walkNames are what each part is called where a human reads it.
var walkNames = map[string]string{
	"asked": "Asked", "built": "Built", "examined": "Examined", "proven": "Proven", "behaves": "Behaves",
}

// walkAbout is what each walk asks the Partner to walk the human through.
var walkAbout = map[string]string{
	"asked":    "what was asked: the goal's intent, the design and its decisions that govern it, and the rulings that touch it",
	"built":    "what was built: the change as a whole, file by file, and the decisions the code itself makes",
	"examined": "how it was examined: the reviews and critiques recorded against it, what each found and what was done about it",
	"proven":   "how it was proven: the tests and the proof recorded for it, what each one holds, and what they all assume",
	"behaves":  "how it behaves: what a person using it would see, and the evidence recorded of it running",
}

// ReviewOpeningRequest is the review brief: the one question the interface asks
// in the human's name when a review starts (D3). It is fixed, and it is here so
// that what the interface says in a human's name is one text a reader can find.
func ReviewOpeningRequest(sitting Sitting) string {
	return "Open this review of the work recorded in " + sitting.Subject.ID + ".\n\n" +
		"The human is here as an examiner, not as the one who holds the intent. You were not in the room that " +
		"shaped this work: read the review record's head for the goal and the commits it reviews, and read " +
		"the records and the code with your tools.\n\n" +
		"Bring what the record holds in five parts, each under its own heading: Asked, Built, Examined, " +
		"Proven, Behaves. Anchor every claim — a file and its lines, a record and its section, a commit. " +
		"Read the change with the changes tool, which reads the reviewed tree and not this checkout.\n\n" +
		"Name what the examination did not try, what the tests assume, and what is not recorded at all. " +
		"Offer each case at the edge as a finding with the deposit tool, with its anchor and the consequence " +
		"of leaving it unanswered. Put on the desk what you are explaining with the present tool.\n\n" +
		"Never say whether to accept this work. The human decides; you make sure they can see it."
}

// WalkRequest is one walk's fixed request.
func WalkRequest(part string, sitting Sitting) string {
	return "Walk me through " + walkNames[part] + " for the review in " + sitting.Subject.ID + ": " +
		walkAbout[part] + ".\n\n" +
		"Put each thing on the desk with the present tool as you come to it, in the order you explain it, " +
		"and anchor every claim. Name what is not recorded. Never say whether to accept."
}

// The three verdicts the End sheet offers (D10), as the Outcome's first line
// spells them.
const (
	VerdictClear   = "clear to land"
	VerdictSend    = "send back"
	VerdictWithout = "no verdict"
)

// ReviewClosingRequest is the close of a review: the Outcome the human records
// with the verdict they chose as its first line.
func ReviewClosingRequest(sitting Sitting, verdict string) (string, error) {
	switch verdict {
	case VerdictClear, VerdictSend, VerdictWithout:
	default:
		return "", fmt.Errorf("a review ends %q, %q or with %q; %q is none of them",
			VerdictClear, VerdictSend, VerdictWithout, verdict)
	}
	return "Close this review in " + sitting.Subject.ID + ". Draft its Outcome and offer it with the deposit " +
		"tool as one deposit of kind outcome, and offer nothing else.\n\n" +
		"Its first line is exactly `Verdict: " + verdict + "`, which the human chose. Then name what was " +
		"examined — the files, the sections and the walks this review looked at — then every finding with " +
		"its answer as the record now carries it, then what was left open with its consequence. Draft it " +
		"from the record and this conversation and from nothing else.\n\n" +
		"Weigh nothing and settle nothing: the verdict is the human's, and it changes nothing yet. The human " +
		"reads it, edits it and presses Record it.", nil
}

// Walk asks one of the five walks in one review's conversation (D6). It is the
// interface's question, marked as such, exactly as the opening turn is.
func (s *Service) Walk(ctx context.Context, human, record, part string, page Page) (string, error) {
	if _, known := walkNames[part]; !known {
		return "", fmt.Errorf("a walk is one of %s; %s is none of them", strings.Join(Walks, ", "), part)
	}
	conversation, err := s.conversationOf(human, record)
	if err != nil {
		return "", err
	}
	sitting := conversation.Sitting()
	if sitting == nil || sitting.Purpose != PurposeReview {
		return "", fmt.Errorf("no review is open on %s, so there is nothing to walk through", record)
	}
	return s.submit(ctx, human, record, "", WalkRequest(part, *sitting), page, true)
}

// KeepRoom writes the room's working state onto one sitting's mark (D9).
func (s *Service) KeepRoom(human, record string, room Room) error {
	conversation, err := s.conversationOf(human, record)
	if err != nil {
		return err
	}
	return conversation.Keep(room, s.now())
}

// Unopened says whether one human's sitting on a record was begun and never
// opened: its mark was written and taken off again by a refused opening turn,
// and nothing was said in it (Sol SOL-A-05). Start reuses such a record rather
// than creating a second one. A record this human never sat on is not theirs,
// and is not unopened: nothing of theirs was ever written beside it.
func (s *Service) Unopened(human, record string) bool {
	own, err := s.conversationOf(human, "")
	if err != nil {
		return false
	}
	_, state := sittingFiles(own.directory, human, record)
	if _, err := os.Stat(state); err != nil {
		return false
	}
	conversation, err := s.conversationOf(human, record)
	if err != nil {
		return false
	}
	conversation.mu.Lock()
	defer conversation.mu.Unlock()
	return conversation.sitting == nil && len(conversation.messages) == 0
}

// Standing is every sitting that stands on one human's conversations, their own
// and every sitting's, by record (D16): what the Sittings tab and the Review
// lane's door read.
func (s *Service) Standing(human string) ([]Sitting, error) {
	own, err := s.conversationOf(human, "")
	if err != nil {
		return nil, err
	}
	found := []Sitting{}
	if sitting := own.Sitting(); sitting != nil {
		found = append(found, *sitting)
	}
	records, err := sittingsOn(own.directory, human)
	if err != nil {
		return nil, err
	}
	for _, record := range records {
		conversation, err := s.conversationOf(human, record)
		if err != nil {
			return nil, err
		}
		if sitting := conversation.Sitting(); sitting != nil {
			found = append(found, *sitting)
		}
	}
	sort.SliceStable(found, func(one, two int) bool { return found[one].Subject.ID < found[two].Subject.ID })
	return found, nil
}

// Present is one thing the Partner puts on the review's desk while it explains
// (D5): a file of the reviewed tree at a range, the change index, one file's
// diff, or a record's section. It is a display suggestion and nothing else — it
// navigates nowhere, and the human's Stop ends a walk's presenting.
type Present struct {
	// Kind is source, changes, diff or section.
	Kind    string `json:"kind"`
	Path    string `json:"path,omitempty"`
	From    int    `json:"from,omitempty"`
	To      int    `json:"to,omitempty"`
	Section string `json:"section,omitempty"`
}

// admitPresent offers one display suggestion to the page, where the turn is a
// review's. Anywhere else there is no desk to put it on, and the activity line
// says so rather than dropping it in silence.
func (s *Service) admitPresent(running *turn, prepared Present) {
	s.mu.Lock()
	conversation := running.conversation
	s.mu.Unlock()
	sitting := conversation.Sitting()
	if sitting == nil || sitting.Purpose != PurposeReview {
		s.record(running, Event{Kind: EventActivity, Text: "The Partner offered something for a desk, " +
			"and only a review room has one."})
		return
	}
	s.record(running, Event{Kind: EventPresent, Present: &prepared})
}
