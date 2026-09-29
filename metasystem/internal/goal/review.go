package goal

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
)

// The review verdict on the ledger (g1-s69 D1, D2).
//
// A human's review of one goal ends in a record, and until now the record was a
// file in one checkout: the verdict it carried changed nothing, and the seat
// that holds the goal and lands it never heard of it. The ledger is the one
// thing every seat reads after its sync, so the verdict travels there: one
// history line on the goal, `review`, landed through the ledger's own commit,
// with the review record itself — and for a send-back the correction brief —
// published beside it in the same landing.
//
// Four rules bind the act, and each is a finding that was folded:
//
//   - The record is resolved in its home and must be about this goal: its
//     Goals line names it, its Outcome opens with the verdict, and the tip the
//     line records is the record's own Reviewed head, never an argument
//     (Astra S69-02).
//   - The Outcome is bound to the tip it was drafted for: its Reviewed at line
//     must name the head's tip, so an Outcome drafted before a retip cannot be
//     recorded against the new one (S69-07).
//   - The publication is create-only: a record already on the canonical tree at
//     that path with the same bytes is a replay, and different bytes there are
//     another review's words and are refused, decided again on every rebuild
//     over a moved tip (S69-03).
//   - A send-back leaves the Landing record alone: a landing claim stands
//     outside the one-claim quota, so clearing it could leave a holder with a
//     lawful second claim over quota (S69-05). The goal leaves the Review lane
//     by reading instead: a send-back line newer than the Landing is the
//     sent-back phase (SentBackOf).
//
// The holder answers a send-back with a line of its own, `send-back`: the
// attempt it started from the brief, or that its goal has several work items
// and the human must name one (S69-06).

// The two verdicts that act. "No verdict" performs nothing and has no word here.
const (
	VerdictClearToLand = "clear-to-land"
	VerdictSendBack    = "send-back"
)

// ReviewHome is where review records live, relative to the ledger's root: the
// review home of internal/project, beside the designs.
const ReviewHome = "plans/reviews/"

const (
	reviewVerb       = "review"
	sendBackVerb     = "send-back"
	reviewedPrefix   = "reviewed "
	sendBackPrefix   = "send-back "
	maxReviewedBytes = 1 << 20
	maxBriefBytes    = 256 << 10
)

// verdictWords are the Outcome's own words for each verdict, as the room's End
// sheet writes them on its first line.
var verdictWords = map[string]string{VerdictClearToLand: "clear to land", VerdictSendBack: "send back"}

// VerdictWords is the verdict as the Outcome's first line spells it, or "" for
// a word that is not one of the two.
func VerdictWords(verdict string) string { return verdictWords[verdict] }

// BriefPathFor is where the correction brief of one review record is published:
// beside it, under the same name. It carries no record head, so the project
// does not read it as a record of its own.
func BriefPathFor(record string) string {
	return strings.TrimSuffix(record, ".md") + ".brief.md"
}

var (
	reviewRecordPath = regexp.MustCompile(`^plans/reviews/[A-Za-z0-9][A-Za-z0-9._-]*\.md$`)
	reviewCommit     = regexp.MustCompile(`^[0-9a-f]{40,64}$`)
	workName         = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
)

// ReviewRecordHead is what the act reads out of a review record: the goals it
// names, the branch tip its Reviewed line records, its Outcome's verdict and
// Reviewed at lines, and how many of its findings the human answered fix.
type ReviewRecordHead struct {
	Goals      []string
	Tip        string
	Verdict    string
	ReviewedAt string
	Fixes      int
}

// findingAnswer is a finding's Answer line as the room's recorder writes it,
// nested under the finding's list item.
var findingAnswer = regexp.MustCompile(`^\s+[-*]\s+Answer:\s*(.*)$`)

// ReadReviewRecord reads a review record's head and Outcome. The head is the
// list before the first section; the Outcome is its section's first two lines
// of words, the verdict and the tip it was drafted for. A finding's answer is
// its Answer line's words before the dash, as the room reads it.
func ReadReviewRecord(content []byte) ReviewRecordHead {
	head := ReviewRecordHead{}
	lines := strings.Split(strings.ReplaceAll(string(content), "\r\n", "\n"), "\n")
	inHead := true
	outcomeLevel := 0
	inFindings := false
	var said []string
	for _, line := range lines {
		if level, title, heading := headingOf(line); heading {
			// The title is not a section: the head is the list before the
			// first section, as the room's own reader takes it.
			inHead = inHead && level == 1
			inFindings = title == "Findings"
			switch {
			case outcomeLevel > 0 && level <= outcomeLevel:
				outcomeLevel = -1
			case outcomeLevel == 0 && title == "Outcome":
				outcomeLevel = level
			}
			continue
		}
		if inHead {
			key, value, found := strings.Cut(strings.TrimPrefix(line, "- "), ":")
			if !strings.HasPrefix(line, "- ") || !found {
				continue
			}
			value = strings.TrimSpace(value)
			switch strings.ToLower(strings.TrimSpace(key)) {
			case "goals":
				head.Goals = strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' })
			case "reviewed":
				if strings.Contains(value, "(the tip of") {
					if fields := strings.Fields(value); len(fields) > 0 && reviewCommit.MatchString(fields[0]) {
						head.Tip = fields[0]
					}
				}
			}
			continue
		}
		if answer := findingAnswer.FindStringSubmatch(line); inFindings && answer != nil {
			if words, _, _ := strings.Cut(answer[1], " — "); strings.TrimSpace(words) == "fix" {
				head.Fixes++
			}
			continue
		}
		if outcomeLevel > 0 && strings.TrimSpace(line) != "" && len(said) < 2 {
			said = append(said, strings.TrimSpace(line))
		}
	}
	if len(said) > 0 {
		if words, found := strings.CutPrefix(said[0], "Verdict:"); found {
			head.Verdict = strings.TrimSpace(words)
		}
	}
	if len(said) > 1 {
		if tip, found := strings.CutPrefix(said[1], "Reviewed at:"); found {
			head.ReviewedAt = strings.TrimSpace(tip)
		}
	}
	return head
}

func headingOf(line string) (int, string, bool) {
	level := 0
	for level < len(line) && level < 6 && line[level] == '#' {
		level++
	}
	if level == 0 || level >= len(line) || line[level] != ' ' {
		return 0, "", false
	}
	return level, strings.TrimSpace(line[level:]), true
}

// ReviewAct is one verdict as the human performs it: the record in its home,
// relative to the ledger's root, its bytes as they stand, the verdict, and for
// a send-back the correction brief and, once the holder has asked, the work
// item it names.
type ReviewAct struct {
	Record  string
	Content []byte
	Verdict string
	Brief   []byte
	Work    string
}

// ReviewLine is one review history line, read back.
type ReviewLine struct {
	Opid    string
	At      string
	Verdict string
	Tip     string
	Record  string
	By      string
	Brief   string
	Work    string
}

// SendBackAnswer is the holder's answer to one send-back: the attempt it
// started from the brief, or the work items it could not choose between.
type SendBackAnswer struct {
	Opid       string
	At         string
	Review     string
	Attempt    int
	Work       string
	Candidates []string
}

// reviewAct checks what the act asks before anything is published, and answers
// the record's head.
func (act ReviewAct) check(id string) (ReviewRecordHead, error) {
	words := VerdictWords(act.Verdict)
	if words == "" {
		return ReviewRecordHead{}, fmt.Errorf("goal review records clear-to-land or send-back; %q is neither, and a review that ends without a verdict records nothing on the goal", act.Verdict)
	}
	if !reviewRecordPath.MatchString(act.Record) || strings.HasSuffix(act.Record, ".brief.md") || path.Clean(act.Record) != act.Record {
		return ReviewRecordHead{}, fmt.Errorf("%s is not a review record in its home; a review record is %s<name>.md", act.Record, ReviewHome)
	}
	if len(act.Content) == 0 || len(act.Content) > maxReviewedBytes || !utf8.Valid(act.Content) {
		return ReviewRecordHead{}, fmt.Errorf("the review record %s is empty, not text, or larger than a record is", act.Record)
	}
	head := ReadReviewRecord(act.Content)
	if !contains(head.Goals, id) {
		named := strings.Join(head.Goals, ", ")
		if named == "" {
			named = "no goal"
		}
		return ReviewRecordHead{}, fmt.Errorf("the review record %s is a review of %s, not of %s; a verdict is recorded on the goal its record reviews", act.Record, named, id)
	}
	if head.Tip == "" {
		return ReviewRecordHead{}, fmt.Errorf("the review record %s names no branch tip on its Reviewed line; a verdict is recorded against the tip that was reviewed", act.Record)
	}
	if head.Verdict != words {
		return ReviewRecordHead{}, fmt.Errorf("the Outcome of %s does not open with \"Verdict: %s\"; record the Outcome End drafts for this verdict first", act.Record, words)
	}
	if head.ReviewedAt != head.Tip {
		return ReviewRecordHead{}, fmt.Errorf("the Outcome of %s was drafted for %s, and the record now reviews %s: the branch was retipped since; press End again", act.Record, orNone(head.ReviewedAt), short(head.Tip))
	}
	switch act.Verdict {
	case VerdictSendBack:
		if head.Fixes == 0 {
			return ReviewRecordHead{}, fmt.Errorf("the review record %s has no finding answered fix, and a send-back carries the findings answered fix as its correction brief; answer the findings the builder must fix, or end the review another way", act.Record)
		}
		if len(bytes.TrimSpace(act.Brief)) == 0 {
			return ReviewRecordHead{}, fmt.Errorf("a send-back carries its correction brief: the findings answered fix; mark at least one finding fix, or end the review another way")
		}
		if len(act.Brief) > maxBriefBytes || !utf8.Valid(act.Brief) {
			return ReviewRecordHead{}, fmt.Errorf("the correction brief is not text, or is larger than a brief is")
		}
		if act.Work != "" && !workName.MatchString(act.Work) {
			return ReviewRecordHead{}, fmt.Errorf("%q is not a work item's name", act.Work)
		}
	default:
		if len(act.Brief) > 0 || act.Work != "" {
			return ReviewRecordHead{}, fmt.Errorf("clear to land carries no correction brief and names no work; those are a send-back's")
		}
	}
	return head, nil
}

// Line is the review line this act records on goal id under the human by,
// or why the act is refused before anything is published.
func (act ReviewAct) Line(id, by string) (ReviewLine, error) {
	head, err := act.check(id)
	if err != nil {
		return ReviewLine{}, err
	}
	line := ReviewLine{Verdict: act.Verdict, Tip: head.Tip, Record: act.Record, By: by, Work: act.Work}
	if act.Verdict == VerdictSendBack {
		line.Brief = BriefPathFor(act.Record)
	}
	return line, nil
}

func orNone(tip string) string {
	if tip == "" {
		return "no tip"
	}
	return short(tip)
}

// Review records one human's verdict on one goal: the review line on its
// history, and the review record — with a send-back's brief — published beside
// it in the same ledger commit. It is a human act, from the enrolled terminal
// or a signed-in browser session, and the line's by is the human who performs
// it; it claims no authorship of the record.
func Review(r VerbRequest, id string, act ReviewAct, proof *humanauthority.Proof) (PublishResult, error) {
	if r.Actor.Human == "" {
		return PublishResult{}, fmt.Errorf("goal review is a human act and names its human (--by)")
	}
	if proof == nil || !(proof.ValidFor(r.Endpoint.Root) || proof.SessionValidFor(r.Endpoint.Root)) {
		return PublishResult{}, fmt.Errorf("goal review requires freshly observed enrolled-terminal human authority or a signed-in browser session")
	}
	if named := humanOfProof(r.Endpoint.Root, proof); named != "" && named != r.Actor.Human {
		return PublishResult{}, fmt.Errorf("the proof names %s and the act is attributed to %s; an act is recorded under the person who made it", named, r.Actor.Human)
	}
	head, err := act.check(id)
	if err != nil {
		return PublishResult{}, err
	}
	return Publish(r.Endpoint, reviewRequest(r, id, act, head, proof))
}

func reviewRequest(r VerbRequest, id string, act ReviewAct, head ReviewRecordHead, proof *humanauthority.Proof) PublishRequest {
	line := ReviewLine{Verdict: act.Verdict, Tip: head.Tip, Record: act.Record, By: r.Actor.Human, Work: act.Work}
	published := []Change{{Path: act.Record, Content: act.Content}}
	if act.Verdict == VerdictSendBack {
		line.Brief = BriefPathFor(act.Record)
		published = append(published, Change{Path: line.Brief, Content: act.Brief})
	}
	reason := line.Reason()
	return PublishRequest{
		Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent:  Intent{Verb: reviewVerb, Targets: []string{id}, Args: intentArgs(r, map[string]string{"reason": reason})},
		Message: "goal review " + id + " " + act.Verdict,
		Mutate: func(tip string) ([]Change, error) {
			t, err := loadTreeFor(r.Endpoint, tip)
			if err != nil {
				return nil, err
			}
			f := t.Live[id]
			if f == nil {
				return nil, fmt.Errorf("goal %s is not live; a verdict is recorded on a goal waiting to land", id)
			}
			if opidLanded(f, r) {
				return nil, AlreadyApplied{}
			}
			if f.State != StateClaimed || f.Claimed == nil || f.Landing == nil {
				return nil, fmt.Errorf("goal %s is not waiting to land (it is %s); a verdict is recorded on a goal in the Review lane", id, f.State)
			}
			// Create-only, decided on this tip: the same bytes are a replay,
			// other bytes are another review's words and are never replaced.
			var changes []Change
			for _, change := range published {
				files, err := readCommitFiles(r.Endpoint, tip, change.Path)
				if err != nil {
					return nil, fmt.Errorf("cannot read %s on the canonical tree: %w", change.Path, err)
				}
				held, present := files[change.Path]
				switch {
				case !present:
					changes = append(changes, change)
				case !bytes.Equal(held, change.Content):
					return nil, fmt.Errorf("%s is already published with other words, another review's; recorded words are never overwritten, so start a new review of %s", change.Path, id)
				}
			}
			for _, recorded := range ReviewLinesOf(f) {
				if recorded.Verdict == line.Verdict && recorded.Tip == line.Tip && recorded.Record == line.Record &&
					recorded.Work == line.Work && recorded.Brief == line.Brief {
					return nil, AlreadyHolds{Reason: "goal " + id + " already carries this verdict: " + verdictWords[line.Verdict] + " at " + short(line.Tip) + " from " + line.Record}
				}
			}
			touch(f, r, reviewVerb, []string{id})
			event := &f.History[len(f.History)-1]
			event.Reason = reason
			recordSessionAuthority(event, proof)
			changes = append(changes, Change{Path: livePath(id), Content: RenderFile(f)})
			return ackDisplacements(t, r, changes), nil
		},
		Validate: func(commit string) error { return validateCommitFor(r.Endpoint, commit) },
	}
}

// Reason is the line's reason as the history carries it.
func (l ReviewLine) Reason() string {
	var b strings.Builder
	b.WriteString(reviewedPrefix + "verdict=" + l.Verdict + " tip=" + l.Tip + " record=" + l.Record + " by=" + l.By)
	if l.Brief != "" {
		b.WriteString(" brief=" + l.Brief)
	}
	if l.Work != "" {
		b.WriteString(" work=" + l.Work)
	}
	return b.String()
}

// parseReviewReason reads one review line's reason, or answers why it is not
// one.
func parseReviewReason(reason string) (ReviewLine, error) {
	if !strings.HasPrefix(reason, reviewedPrefix) {
		return ReviewLine{}, fmt.Errorf("review history's reason opens with %q", strings.TrimSpace(reviewedPrefix))
	}
	fields, err := reasonFields(strings.TrimPrefix(reason, reviewedPrefix), "verdict", "tip", "record", "by", "brief", "work")
	if err != nil {
		return ReviewLine{}, fmt.Errorf("review history: %v", err)
	}
	line := ReviewLine{Verdict: fields["verdict"], Tip: fields["tip"], Record: fields["record"], By: fields["by"], Brief: fields["brief"], Work: fields["work"]}
	switch {
	case VerdictWords(line.Verdict) == "":
		return ReviewLine{}, fmt.Errorf("review history's verdict is clear-to-land or send-back")
	case !reviewCommit.MatchString(line.Tip):
		return ReviewLine{}, fmt.Errorf("review history's tip is a full commit id")
	case !reviewRecordPath.MatchString(line.Record):
		return ReviewLine{}, fmt.Errorf("review history's record is a review record in its home")
	case line.By == "":
		return ReviewLine{}, fmt.Errorf("review history names its human with by=")
	case line.Verdict == VerdictSendBack && line.Brief != BriefPathFor(line.Record):
		return ReviewLine{}, fmt.Errorf("a send-back names its brief beside its record")
	case line.Verdict == VerdictClearToLand && (line.Brief != "" || line.Work != ""):
		return ReviewLine{}, fmt.Errorf("clear-to-land history carries no brief and no work")
	case line.Work != "" && !workName.MatchString(line.Work):
		return ReviewLine{}, fmt.Errorf("review history's work is a work item's name")
	}
	return line, nil
}

// reasonFields reads space-separated key=value fields, each at most once and
// each one the caller names.
func reasonFields(said string, keys ...string) (map[string]string, error) {
	fields := map[string]string{}
	for _, field := range strings.Fields(said) {
		key, value, found := strings.Cut(field, "=")
		if !found || !contains(keys, key) || value == "" {
			return nil, fmt.Errorf("%q is not one of its fields", field)
		}
		if _, seen := fields[key]; seen {
			return nil, fmt.Errorf("%s= appears twice", key)
		}
		fields[key] = value
	}
	return fields, nil
}

// validReviewHistory is the grammar ParseHistoryLine holds the two new verbs
// to: a review is a human's line, and a send-back answer is the holder's.
func validReviewHistory(h HistoryLine) error {
	switch h.Verb {
	case reviewVerb:
		if !strings.HasPrefix(h.Actor, "human:") || len(h.Targets) != 1 {
			return fmt.Errorf("review history is a human's act on one goal")
		}
		_, err := parseReviewReason(h.Reason)
		return err
	case sendBackVerb:
		if strings.HasPrefix(h.Actor, "human:") || len(h.Targets) != 1 {
			return fmt.Errorf("send-back history is the holder's answer on one goal")
		}
		_, err := parseSendBackReason(h.Reason)
		return err
	}
	return nil
}

// ReviewLinesOf is every review line on a goal's history, oldest first.
func ReviewLinesOf(f *GoalFile) []ReviewLine {
	var lines []ReviewLine
	if f == nil {
		return lines
	}
	for _, h := range f.History {
		if h.Verb != reviewVerb {
			continue
		}
		line, err := parseReviewReason(h.Reason)
		if err != nil {
			continue
		}
		line.Opid, line.At = h.Opid, h.At
		lines = append(lines, line)
	}
	return lines
}

// landingIndex is where on the history the goal's standing Landing was
// written, or -1 where it stands on no line this history carries.
func landingIndex(f *GoalFile) int {
	if f == nil || f.Landing == nil {
		return -1
	}
	for index := len(f.History) - 1; index >= 0; index-- {
		if f.History[index].Opid == f.Landing.Opid {
			return index
		}
	}
	return -1
}

// Verdicts is where a goal's review stands on its history, as every seat reads
// it: the newest verdict recorded since its standing Landing, and for a
// send-back the holder's answer to it.
type Verdicts struct {
	// Latest is the newest review line since the Landing, or nil.
	Latest *ReviewLine
	// SentBack says the latest verdict is a send-back: the goal is out of
	// the Review lane, a phase of In progress, until a later land-ready.
	SentBack bool
	// Answer is the holder's answer to the latest send-back, or nil while
	// the holder has not answered it.
	Answer *SendBackAnswer
}

// VerdictsOf reads one goal's verdicts. A verdict older than the goal's
// standing Landing answered an earlier landing and says nothing now: a later
// land-ready puts the goal back in Review with no verdict on it.
func VerdictsOf(f *GoalFile) Verdicts {
	read := Verdicts{}
	if f == nil || f.Landing == nil {
		return read
	}
	since := landingIndex(f)
	if since < 0 {
		return read
	}
	for _, h := range f.History[since+1:] {
		switch h.Verb {
		case reviewVerb:
			line, err := parseReviewReason(h.Reason)
			if err != nil {
				continue
			}
			line.Opid, line.At = h.Opid, h.At
			read.Latest, read.Answer = &line, nil
			read.SentBack = line.Verdict == VerdictSendBack
		case sendBackVerb:
			answer, err := parseSendBackReason(h.Reason)
			if err != nil || read.Latest == nil || answer.Review != read.Latest.Opid {
				continue
			}
			answer.Opid, answer.At = h.Opid, h.At
			read.Answer = &answer
		}
	}
	return read
}

// SentBackOf is the send-back a holder has still to act on: the latest verdict
// is a send-back and the holder has not answered it — or answered that it
// needs to know which work, and the human has since named one on a newer
// review line, which is a send-back of its own.
func SentBackOf(f *GoalFile) (ReviewLine, bool) {
	read := VerdictsOf(f)
	if !read.SentBack || read.Answer != nil {
		return ReviewLine{}, false
	}
	return *read.Latest, true
}

func (a SendBackAnswer) reason() string {
	if len(a.Candidates) > 0 {
		return sendBackPrefix + "needs-work candidates=" + strings.Join(a.Candidates, ",") + " review=" + a.Review
	}
	said := sendBackPrefix + "attempt=" + strconv.Itoa(a.Attempt) + " review=" + a.Review
	if a.Work != "" {
		said += " work=" + a.Work
	}
	return said
}

func parseSendBackReason(reason string) (SendBackAnswer, error) {
	rest, found := strings.CutPrefix(reason, sendBackPrefix)
	if !found {
		return SendBackAnswer{}, fmt.Errorf("send-back history's reason opens with %q", strings.TrimSpace(sendBackPrefix))
	}
	if needs, asked := strings.CutPrefix(rest, "needs-work "); asked {
		fields, err := reasonFields(needs, "candidates", "review")
		if err != nil {
			return SendBackAnswer{}, fmt.Errorf("send-back history: %v", err)
		}
		candidates := strings.Split(fields["candidates"], ",")
		for _, one := range candidates {
			if !workName.MatchString(one) {
				return SendBackAnswer{}, fmt.Errorf("send-back history's candidates are work items' names")
			}
		}
		if len(candidates) < 2 || !validOpidShape(fields["review"]) {
			return SendBackAnswer{}, fmt.Errorf("a needs-work answer names two or more candidates and the review it answers")
		}
		return SendBackAnswer{Review: fields["review"], Candidates: candidates}, nil
	}
	fields, err := reasonFields(rest, "attempt", "review", "work")
	if err != nil {
		return SendBackAnswer{}, fmt.Errorf("send-back history: %v", err)
	}
	attempt, err := strconv.Atoi(fields["attempt"])
	if err != nil || attempt < 1 || !validOpidShape(fields["review"]) {
		return SendBackAnswer{}, fmt.Errorf("an attempt answer names the attempt it started and the review it answers")
	}
	if fields["work"] != "" && !workName.MatchString(fields["work"]) {
		return SendBackAnswer{}, fmt.Errorf("send-back history's work is a work item's name")
	}
	return SendBackAnswer{Review: fields["review"], Attempt: attempt, Work: fields["work"]}, nil
}

// AnswerSendBack records the holder's answer to the send-back it acted on: the
// attempt its correction started, or the candidates it could not choose
// between. It is the claim holder's own act, and it answers exactly the
// send-back it names, so a later pass finds the line and revises nothing twice.
func AnswerSendBack(r VerbRequest, id string, answer SendBackAnswer) (PublishResult, error) {
	if r.Actor.Human != "" {
		return PublishResult{}, fmt.Errorf("a send-back is answered by the seat that holds the goal; it takes no --by")
	}
	if !validOpidShape(answer.Review) {
		return PublishResult{}, fmt.Errorf("a send-back answer names the review line it answers")
	}
	answer.Opid, answer.At = "", ""
	if _, err := parseSendBackReason(answer.reason()); err != nil {
		return PublishResult{}, err
	}
	return Publish(r.Endpoint, answerSendBackRequest(r, id, answer))
}

// answerSendBackRequest is the answer's one mutation semantics, which the live
// act and recovery's replay both run.
func answerSendBackRequest(r VerbRequest, id string, answer SendBackAnswer) PublishRequest {
	reason := answer.reason()
	return PublishRequest{
		Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent:  Intent{Verb: sendBackVerb, Targets: []string{id}, Args: intentArgs(r, map[string]string{"reason": reason})},
		Message: "goal send-back " + id,
		Mutate: func(tip string) ([]Change, error) {
			t, err := loadTreeFor(r.Endpoint, tip)
			if err != nil {
				return nil, err
			}
			f := t.Live[id]
			if f == nil {
				return nil, fmt.Errorf("goal %s is not live; there is no send-back to answer", id)
			}
			if opidLanded(f, r) {
				return nil, AlreadyApplied{}
			}
			if f.State != StateClaimed || f.Claimed == nil || !ownPair(f.Claimed, r.Actor) {
				return nil, fmt.Errorf("goal %s is not this seat's claim; a send-back is answered by the seat that holds it", id)
			}
			read := VerdictsOf(f)
			if read.Latest == nil || read.Latest.Opid != answer.Review || !read.SentBack {
				return nil, fmt.Errorf("goal %s carries no standing send-back %s to answer", id, answer.Review)
			}
			if read.Answer != nil {
				return nil, AlreadyHolds{Reason: "goal " + id + "'s send-back " + answer.Review + " is already answered"}
			}
			touch(f, r, sendBackVerb, []string{id})
			f.History[len(f.History)-1].Reason = reason
			return ackDisplacements(t, r, []Change{{Path: livePath(id), Content: RenderFile(f)}}), nil
		},
		Validate: func(commit string) error { return validateCommitFor(r.Endpoint, commit) },
	}
}

// ReadPublished reads one file of the canonical tree at the accepted tip: the
// brief a send-back published, which the holder revises from.
func ReadPublished(e Endpoint, file string) ([]byte, error) {
	tip, present, err := e.repository().Accepted()
	if err != nil {
		return nil, err
	}
	if !present {
		return nil, fmt.Errorf("this clone has accepted no ledger tip yet; sync first")
	}
	files, err := readCommitFiles(e, tip, file)
	if err != nil {
		return nil, err
	}
	content, found := files[file]
	if !found {
		return nil, fmt.Errorf("%s is not published at %s", file, short(tip))
	}
	return content, nil
}

// ResolveReviewRecord resolves a review record in its home: the path, however
// it was given, must name a regular file beneath root's review home once every
// link is followed, and its bytes are read from there. It answers the record's
// path relative to root, which is the path the act publishes it at.
func ResolveReviewRecord(root, given string) (string, []byte, error) {
	if strings.TrimSpace(given) == "" {
		return "", nil, fmt.Errorf("goal review names its review record with --record PATH")
	}
	absolute := given
	if !filepath.IsAbs(absolute) {
		absolute = filepath.Join(root, absolute)
	}
	home, err := filepath.EvalSymlinks(filepath.Join(root, filepath.FromSlash(ReviewHome)))
	if err != nil {
		return "", nil, fmt.Errorf("this project has no review home at %s", ReviewHome)
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", nil, fmt.Errorf("no review record at %s", given)
	}
	name, err := filepath.Rel(home, resolved)
	if err != nil || name == "." || strings.HasPrefix(name, "..") || strings.ContainsRune(name, filepath.Separator) {
		return "", nil, fmt.Errorf("%s is not a review record in its home; a review record is %s<name>.md", given, ReviewHome)
	}
	info, err := os.Lstat(resolved)
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxReviewedBytes {
		return "", nil, fmt.Errorf("%s is not a review record this act can read", given)
	}
	content, err := os.ReadFile(resolved)
	if err != nil {
		return "", nil, fmt.Errorf("cannot read the review record %s: %w", given, err)
	}
	return ReviewHome + name, content, nil
}
