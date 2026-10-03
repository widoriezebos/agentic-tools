package partner

import (
	"context"
	"fmt"
	"os"
	"slices"
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

// Walks are the fixed requests a room offers under its conversation, by the
// sitting's purpose, in the order the room shows their presses (g1-s67 D3): a
// review keeps the five parts of the narrator's report (g1-s65 D6), and a
// sitting that shapes a record has four of its own.
var Walks = map[string][]string{
	PurposeReview:      {"asked", "built", "examined", "proven", "behaves"},
	PurposeShapeDesign: shapingWalks,
	PurposeShapeIntent: shapingWalks,
}

var shapingWalks = []string{"records", "today", "cases", "open"}

// walkNames are what each part is called where a human reads it.
var walkNames = map[string]string{
	"asked": "Asked", "built": "Built", "examined": "Examined", "proven": "Proven", "behaves": "Behaves",
	"records": "Records", "today": "Today", "cases": "Cases", "open": "Open",
}

// walkAbout is what each walk asks the Partner to walk the human through.
var walkAbout = map[string]string{
	"asked":    "what was asked: the goal's intent, the design and its decisions that govern it, and the rulings that touch it",
	"built":    "what was built: the change as a whole, file by file, and the decisions the code itself makes",
	"examined": "how it was examined: the reviews and critiques recorded against it, what each found and what was done about it",
	"proven":   "how it was proven: the tests and the proof recorded for it, what each one holds, and what they all assume",
	"behaves":  "how it behaves: what a person using it would see, and the evidence recorded of it running",
	"records": "what the records already hold that touches this subject: the standing rulings, the recorded " +
		"decisions, the open questions, and the intent and design records that name it",
	"today": "what the application does today where this subject touches it",
	"cases": "the cases at the edge of this subject: the awkward instances a rule here has to survive",
	"open":  "what this sitting has not settled yet, and what would settle each of it",
}

// shapingHow is what each shaping walk asks beyond its subject: where its
// answer is read from, and what it offers the human to record.
var shapingHow = map[string]string{
	"records": "Anchor every claim in the record and the section it is written in, and offer what you find " +
		"with the deposit tool as facts, decisions and open questions.",
	"today": "Answer from the code: read the checkout as it stands with your own reads — the files as they are " +
		"now, uncommitted edits included — and name each place as path:lines, the file and its lines, which " +
		"are the lines the desk reads. Put each file on the desk with the present tool as you come to it.",
	"cases": "Offer each case with the deposit tool as a case, with the clause it would become and the " +
		"consequence of leaving it open; the human decides it or leaves it open.",
	"open": "Offer each unsettled thing with the deposit tool as an open question, with the consequence of " +
		"leaving it open.",
}

// ReviewOpeningRequest is the review brief: the one question the interface asks
// in the human's name when a review starts (D3). It is fixed, and it is here so
// that what the interface says in a human's name is one text a reader can find.
func ReviewOpeningRequest(sitting Sitting) string {
	return "Open this review of the work recorded in " + sitting.Subject.ID + ".\n\n" + openingBrief
}

// WalkAgain is the opening asked again on the version the record names now
// (review-findings-read-as-decisions RF-03, RF-06): by Review the current
// version, after the record moved to the branch's current commit, and by Ask
// again, after a look that did not finish. It is asked through the walk route,
// as the interface's question, and a review is the one sitting that has it.
const WalkAgain = "again"

// WalkResume is the opening asked once more on the same version, after a look
// that stopped or failed (fix round 3, F-2): Ask the reviewer to look again.
// It is not a move to another version, so nothing raised before it is earlier.
const WalkResume = "resume"

// ReviewResumeRequest is the look at this version asked once more: what was
// already offered stands and is not offered again, and the look is finished
// with the opening's own brief.
func ReviewResumeRequest(sitting Sitting) string {
	return "Open this review once more, in " + sitting.Subject.ID + ": your look at the version its Reviewed line " +
		"names did not finish. What you already offered in this conversation stands; do not offer it again. " +
		"Finish the look.\n\n" + openingBrief
}

// earlierHeading is the record's section that holds what was raised about an
// earlier version, as the room moves it there when the version moves.
const earlierHeading = "Earlier findings"

// ReviewAgainRequest is the opening asked again: what was raised about the
// earlier version and the person's decision on each, named as earlier, and the
// opening's own brief on the version the record names now. Nothing carries
// over by itself: the reviewer says which still apply and raises those again.
func ReviewAgainRequest(sitting Sitting, earlier string) string {
	named := "Nothing was raised about an earlier version."
	if strings.TrimSpace(earlier) != "" {
		named = "What was raised about an earlier version, with the person's decision on each, as the record's " +
			earlierHeading + " section holds it:\n\n" + strings.TrimSpace(earlier) + "\n\n" +
			"Nothing of it carries over by itself: say which still apply to the version under review now, and offer " +
			"each that does again as a finding of this version, recommending the person's earlier decision where it " +
			"still fits. Say plainly which no longer apply."
	}
	return "Open this review again, in " + sitting.Subject.ID + ", on the version its Reviewed line names now; the " +
		"person asked you to examine that version afresh.\n\n" + named + "\n\n" + openingBrief
}

// earlierOf is the earlier findings a review record holds, its section's own
// lines, or "" where it holds none or cannot be read.
func (s *Service) earlierOf(record string) string {
	if s.facts.Document == nil {
		return ""
	}
	document, err := s.facts.Document(record)
	if err != nil {
		return ""
	}
	held := []string{}
	inside := false
	for _, line := range strings.Split(document.Source, "\n") {
		if heading, found := strings.CutPrefix(line, "## "); found {
			inside = strings.TrimSpace(heading) == earlierHeading
			continue
		}
		if inside && strings.HasPrefix(line, "#") {
			inside = false
		}
		if inside && strings.TrimSpace(line) != "" {
			held = append(held, line)
		}
	}
	return strings.Join(held, "\n")
}

// openingBrief is what every opening asks, the first and any asked again.
const openingBrief = "The human is here as an examiner, not as the one who holds the intent. You were not in the room that " +
	"shaped this work: read the review record's head for the goal and the commits it reviews, and read " +
	"the records and the code with your tools.\n\n" +
	"Begin with one plain paragraph, under no heading, that the person reads first: the goal in one " +
	"sentence, what this version adds, and how many files it touches, in their words and with no commit " +
	"id, file path or timestamp.\n\n" +
	"Then bring what the record holds in five parts, each under its own heading: Asked, Built, Examined, " +
	"Proven, Behaves. Anchor every claim — a file and its lines, a record and its section, a commit. " +
	"Read the change with the changes tool, which reads the reviewed tree and not this checkout.\n\n" +
	"Name what the examination did not try, what the tests assume, and what is not recorded at all. " +
	"Offer each case at the edge as a finding with the deposit tool. " + findingAsk + " " +
	"Put on the desk what you are explaining with the present tool.\n\n" +
	"Never give the verdict on this work: whether it lands is the person's to decide."

// WalkRequest is one walk's fixed request, in the words of the sitting's
// purpose.
func WalkRequest(part string, sitting Sitting) string {
	if sitting.Purpose != PurposeReview {
		return "Walk me through " + walkNames[part] + " for the sitting on " + sitting.Subject.ID + ": " +
			walkAbout[part] + ".\n\n" + shapingHow[part] + "\n\n" +
			"Weigh nothing: do not recommend, do not rank, and do not say which option you would pick. " +
			"Where nothing is recorded, say that nothing is recorded."
	}
	return "Walk me through " + walkNames[part] + " for the review in " + sitting.Subject.ID + ": " +
		walkAbout[part] + ".\n\n" +
		"Put each thing on the desk with the present tool as you come to it, in the order you explain it, " +
		"and anchor every claim. Name what is not recorded. If the record's head names earlier versions on " +
		"its Previously line, the version under review moved since: say first what changed. Offer a finding " +
		"the record's Findings section already carries only if it changed. " + findingAsk + " " +
		"Never give the verdict: whether it lands is the person's to decide."
}

// findingAsk is what a review asks of every finding it offers
// (review-findings-read-as-decisions §4): the plain layers a person decides it
// by, before the evidence, and one recommended decision with its reason.
const findingAsk = "Give each finding its severity (blocks, fix or note); a title: the problem in one plain " +
	"sentence, with no commit id, file path or timestamp; why it matters, in one or two plain sentences; and " +
	"recommend one decision per finding — must-fix, fix-later, not-a-problem or accept, a note only fix-later " +
	"or not-a-problem — with the reason in one sentence. The finding's own words, its anchor and its " +
	"consequence are its evidence, written as precisely as you like."

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
		"Weigh nothing and settle nothing: the verdict is the human's, recorded on the goal when they record the Outcome, and it is the word the landing gate waits for. The human " +
		"reads it, edits it and presses Record it.", nil
}

// Candidate is what the Behaves walk is told of the goal's candidate run
// (g1-s69 D3): where it answers, the commit it runs, and the tip the review's
// record names. A candidate with no address is not running.
type Candidate struct {
	Address  string
	Running  string
	Reviewed string
	// Evidence is what the review's Evidence path holds (g1-s71 D4), or nil
	// where nothing was read: the listing the Partner presents from.
	Evidence *Evidence
}

// Evidence is the listing the Behaves walk is handed: the path the record
// names, each entry as "path (kind, size)", bounded and saying the whole — or,
// where its walk stopped at its bound, that it is a part — or the words a read
// of it was refused with.
type Evidence struct {
	Path     string
	Entries  []string
	Supplied int
	Total    int
	Cut      bool
	Refusal  string
}

// EvidenceNote is what the Behaves walk's request says about the evidence: what
// is there, by the one path convention the present tool and the desk share, or
// why there is nothing to present. A cut is said first: a listing cut before
// it found a file never claims the path holds nothing.
func EvidenceNote(evidence Evidence) string {
	if strings.TrimSpace(evidence.Refusal) != "" {
		return "The evidence of this review could not be listed: " + evidence.Refusal + ". Say so, and present none."
	}
	if len(evidence.Entries) == 0 && evidence.Cut {
		return "The listing of the evidence path " + evidence.Path + " stopped at its bound before it found an image or " +
			"text: nothing was found within the listing's bounds, and the path may hold more. Say so, and present none."
	}
	if len(evidence.Entries) == 0 {
		return "The evidence path " + evidence.Path + " holds no image or text; say that nothing is recorded there."
	}
	said := "The evidence recorded under " + evidence.Path + " holds these files"
	switch {
	case evidence.Cut:
		said += fmt.Sprintf(", the first %d found listed; the listing stopped at its bound, so the path may hold more", evidence.Supplied)
	case evidence.Supplied < evidence.Total:
		said += fmt.Sprintf(", %d of %d listed", evidence.Supplied, evidence.Total)
	}
	said += ":\n- " + strings.Join(evidence.Entries, "\n- ") + "\n\n" +
		"Present a file with the present tool with kind evidence and its path relative to the evidence, exactly " +
		"as listed; present only what is listed."
	return said
}

// CandidateNote is what the Behaves walk's request says about the candidate:
// where it runs and at which commit beside the reviewed tip, and when the two
// differ, that what runs is not the version under review.
func CandidateNote(candidate Candidate) string {
	if strings.TrimSpace(candidate.Address) == "" {
		return "No candidate of this goal is running now; say so, and walk what the record holds of it running."
	}
	said := "The candidate runs at " + candidate.Address + ", at commit " + orUnknown(candidate.Running) +
		"; the review is of " + orUnknown(candidate.Reviewed) + "."
	if candidate.Running == "" || candidate.Reviewed == "" || candidate.Running != candidate.Reviewed {
		said += " What runs is not the version under review: say so first, and do not present what it does as " +
			"evidence of the reviewed version."
	}
	return said
}

func orUnknown(commit string) string {
	if strings.TrimSpace(commit) == "" {
		return "a commit nobody recorded"
	}
	return commit
}

// Walk asks one of the walks of one sitting's room (g1-s65 D6, g1-s67 D3). It
// is the interface's question, marked as such, exactly as the opening turn is.
func (s *Service) Walk(ctx context.Context, human, record, part string, page Page) (string, error) {
	return s.WalkWith(ctx, human, record, part, page, nil)
}

// WalkWith is Walk with what the Behaves walk is told of the candidate run; the
// other walks are asked as they always were.
func (s *Service) WalkWith(ctx context.Context, human, record, part string, page Page, candidate *Candidate) (string, error) {
	conversation, err := s.conversationOf(human, record)
	if err != nil {
		return "", err
	}
	sitting := conversation.Sitting()
	if sitting == nil {
		return "", fmt.Errorf("no sitting is open on %s, so there is nothing to walk through", record)
	}
	if part == WalkAgain || part == WalkResume {
		if sitting.Purpose != PurposeReview {
			return "", fmt.Errorf("only a review's opening is asked again; this sitting shapes %s", record)
		}
		if part == WalkResume {
			return s.submit(ctx, human, record, "", ReviewResumeRequest(*sitting), page, true)
		}
		return s.submit(ctx, human, record, "", ReviewAgainRequest(*sitting, s.earlierOf(record)), page, true)
	}
	offered := Walks[sitting.Purpose]
	if !slices.Contains(offered, part) {
		return "", fmt.Errorf("a walk of this sitting is one of %s; %s is none of them", strings.Join(offered, ", "), part)
	}
	request := WalkRequest(part, *sitting)
	if part == "behaves" && sitting.Purpose == PurposeReview && candidate != nil {
		request += "\n\n" + CandidateNote(*candidate)
		if candidate.Evidence != nil {
			request += "\n\n" + EvidenceNote(*candidate.Evidence)
		}
	}
	return s.submit(ctx, human, record, "", request, page, true)
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
	conversation, err := s.openedOf(human, record)
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
		conversation, err := s.openedOf(human, record)
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

// Present is one thing the Partner puts on a room's desk while it explains
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
// sitting's: every sitting has a room with a desk (g1-s67 D4). Outside a sitting
// there is no desk to put it on, and the activity line says so rather than
// dropping it in silence.
func (s *Service) admitPresent(running *turn, prepared Present) {
	s.mu.Lock()
	conversation := running.conversation
	s.mu.Unlock()
	if conversation.Sitting() == nil {
		s.record(running, Event{Kind: EventActivity, Text: "The Partner offered something for a desk, " +
			"and only a sitting's room has one."})
		return
	}
	s.record(running, Event{Kind: EventPresent, Present: &prepared})
}
