package uitools

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

// The second operation that reads nothing.
//
// A sitting is a working conversation on one record, and the record is its
// memory: it deposits facts anchored where they can be checked, decisions
// recorded then and there with the human's own reasons, and open questions
// named with their consequences. One test covers all of them: end the sitting
// at any moment and a fresh worker with the same human must be able to go on
// from the record alone.
//
// Two more kinds are step 2's, and neither is an entry of the four piles. A
// CASE is a case at the edge, offered for the human to settle or to leave open:
// it lands on no pile by itself, and what lands is the decision or the open
// question the human makes of it (g1-s55 D1). An OUTCOME is the closing
// deposit, drafted when the human ends the sitting, and it is written as the
// record's own Outcome section rather than appended to a pile (D2).
//
// This is how the Partner offers one. It prepares a deposit; it does not write
// one. Nothing here reaches the record: the card appears on the transcript and
// on the room's table, the human edits it if they like, and Record it is their
// press. The paper's rule is what that is for — "the sitting proposes; only
// records rule" — and a tool that wrote would be the room taking an authority
// it does not have.
//
// This server cannot judge whether there is a sitting to deposit into: it runs
// in its own process, with the workspace's readers and no conversation, so it
// has no way to know. That is the whole of why it validates its arguments and
// says "prepared" rather than "offered": the turn-owning service holds the
// conversation and admits the deposit against the sitting's subject.

// The five kinds one deposit can be, and the one that waits.
//
// Proposals are the fourth pile a sitting maintains, and nothing offers one:
// the closing deposit was what would have made a proposal worth having, and it
// has a kind of its own now, because what it writes is the record's Outcome
// section and not a pile. A call that names a proposal is refused in words that
// say so, rather than being silently recorded as something else.
const (
	DepositFact     = "fact"
	DepositDecision = "decision"
	DepositQuestion = "question"
	DepositCase     = "case"
	DepositOutcome  = "outcome"
	DepositProposal = "proposal"
	// DepositFinding is the review's own (g1-s65 D8): a finding with the
	// anchor it sits at and the consequence of leaving it unanswered.
	DepositFinding = "finding"
)

// DepositKinds is what this build admits, in the order the tool's own
// description lists them.
var DepositKinds = []string{DepositFact, DepositDecision, DepositQuestion, DepositCase, DepositOutcome, DepositFinding}

// The bounds. A deposit is one entry of a record's section: a line or a
// paragraph, with one short clause beside it. Past these the call is refused in
// words rather than cut, because half an entry offered as a whole one is an
// entry a human cannot read as what the Partner meant.
const (
	// maxDeposit is how much one deposit says, in characters.
	maxDeposit = 2000
	// maxClause is how much the anchor, the reason, the consequence or the
	// clause carries. An anchor is a path with a line in it; a reason is a
	// sentence; a clause is the one line a case would become.
	maxClause = 500
	// maxOutcome is how much the closing deposit says. It is the one deposit
	// that is not an entry of a list: it is a whole section — the outcome as
	// decided, the constraints, the open questions with their consequences, and
	// what the table holds — so an entry's bound would refuse the very thing
	// the kind exists for.
	maxOutcome = 8000
)

// The fixed form a prepared deposit travels back in, written down once here and
// read once by the interface's host.
//
// It is textual because the result text is the channel this server has. The
// header names the kind; the labelled lines after it carry the one clause the
// kind takes; the separator ends the framing; and everything after the
// separator is the deposit's own words, opaque to the end. A deposit may itself
// hold a line that reads like a header — a fact about this interface's own
// result format, say — and nothing after the separator is read as one.
const (
	DepositHeader      = "Deposit: "
	DepositAnchor      = "Anchor: "
	DepositReason      = "Reason: "
	DepositConsequence = "Consequence: "
	// DepositClause carries what a case would become: the clause as the Partner
	// heard it, which the Decide sheet opens with (g1-s55 D1).
	DepositClause = "Clause: "
	// A finding's plain layers (review-findings-read-as-decisions §4): how much
	// it matters, the problem in a person's words, why it matters, and the one
	// decision the reviewer recommends, whose one sentence why rides as the
	// Reason line.
	DepositSeverity   = "Severity: "
	DepositTitle      = "Title: "
	DepositWhy        = "Why: "
	DepositRecommends = "Recommends: "
	DepositSeparator  = "--- the deposit follows, whole and to the end ---"
)

// A finding's severities and the decisions a reviewer recommends, in the words
// the tool takes them in and the card reads them back by.
const (
	SeverityBlocks = "blocks"
	SeverityFix    = "fix"
	SeverityNote   = "note"

	RecommendMustFix    = "must-fix"
	RecommendFixLater   = "fix-later"
	RecommendNotProblem = "not-a-problem"
	RecommendAccept     = "accept"
)

// Severities and Recommendations are what a finding may say, in order.
var (
	Severities      = []string{SeverityBlocks, SeverityFix, SeverityNote}
	Recommendations = []string{RecommendMustFix, RecommendFixLater, RecommendNotProblem, RecommendAccept}
)

// Finding is a finding's plain layers, as the tool takes them and the
// interface's admission reads them back.
type Finding struct {
	Severity  string
	Title     string
	Why       string
	Recommend string
	Reason    string
}

// titleRefused is what a title carrying an id, a path or a time is refused with.
const titleRefused = "a finding's title is the problem in a person's words, with no commit id, file path or timestamp; " +
	"put those in the finding's own words, which the person reads as its evidence"

// FindingRefusal is why a finding cannot be offered to a person as a decision,
// in the words the Partner is told, or "" for a whole one. It is one rule, asked
// twice: by this tool, so the Partner hears it and offers the finding again,
// and by the interface's admission, so no card reaches a person without it.
func FindingRefusal(finding Finding) string {
	severity, recommend := strings.ToLower(oneLine(finding.Severity)), strings.ToLower(oneLine(finding.Recommend))
	switch {
	case severity == "":
		return "a finding says how much it matters as its severity: " + orList(Severities)
	case strings.TrimSpace(finding.Title) == "":
		return "a finding carries a title: the problem in one plain sentence"
	case strings.TrimSpace(finding.Why) == "":
		return "a finding carries why it matters: what happens if it is ignored"
	case recommend == "":
		return "a finding carries the one decision you recommend: " + orList(Recommendations)
	case strings.TrimSpace(finding.Reason) == "":
		return "a finding carries the reason for the decision you recommend, in one sentence"
	case !slices.Contains(Severities, severity):
		return "a finding's severity is " + orList(Severities) + "; " + strconv.Quote(severity) + " is none of them"
	case !slices.Contains(Recommendations, recommend):
		return "the decision you recommend is " + orList(Recommendations) + "; " + strconv.Quote(recommend) + " is none of them"
	case severity == SeverityNote && (recommend == RecommendMustFix || recommend == RecommendAccept):
		return "a note is fixed after landing or is not a problem; recommend " + RecommendFixLater + " or " + RecommendNotProblem
	case technical(finding.Title):
		return titleRefused
	}
	return ""
}

// The three things a title never carries: a commit id (seven or more hex
// characters with a digit and a letter among them), a file path (a file with
// its line, or a path through a directory to a file or with two directories),
// and a timestamp to the second or in ISO form.
var (
	hexWord  = regexp.MustCompile(`\b[0-9a-f]{7,64}\b`)
	filePath = regexp.MustCompile(`[\w.-]+\.[A-Za-z][A-Za-z0-9]{0,7}:\d+|[\w.-]+/[\w./-]*\.[A-Za-z][A-Za-z0-9]{0,7}\b|[\w.-]+/[\w.-]+/[\w./-]+`)
	moment   = regexp.MustCompile(`\b\d{1,2}:\d{2}:\d{2}|\b\d{4}-\d{2}-\d{2}T\d{1,2}:\d{2}`)
)

func technical(title string) bool {
	if filePath.MatchString(title) || moment.MatchString(title) {
		return true
	}
	for _, word := range hexWord.FindAllString(title, -1) {
		if strings.ContainsAny(word, "0123456789") && strings.ContainsAny(word, "abcdef") {
			return true
		}
	}
	return false
}

func orList(words []string) string {
	return strings.Join(words[:len(words)-1], ", ") + " or " + words[len(words)-1]
}

// DepositedLine is what a prepared deposit answers the model with.
//
// It says "prepared" and then says what preparing is not, for the reason the
// suggestion's own line does: a Partner that read "recorded" would tell a human
// their decision is in the record when nothing has been written anywhere, and
// the paper's own rule is that the room proposes and only records rule.
const DepositedLine = "prepared; preparing does not record it: the human presses Record it, " +
	"and it enters the record then and not before"

// deposit prepares one deposit, or refuses the call in words.
func deposit(kind, text, anchor, reason, consequence, clauseSaid string, finding Finding) Result {
	kind = strings.ToLower(oneLine(kind))
	anchor, reason, consequence = oneLine(anchor), oneLine(reason), oneLine(consequence)
	clauseSaid = oneLine(clauseSaid)
	// Only the framing's own newlines are the framing's. What the model wrote
	// travels as it wrote it, apart from the blank lines a transport may have
	// left at either end of it.
	text = strings.Trim(text, "\n")
	switch {
	case kind == DepositProposal:
		return refusedCall("this interface records facts, decisions, open questions, cases at the edge and " +
			"the closing outcome in a sitting; a proposal is not one of them, so say it in words instead" +
			"; an act on a goal is proposed with " + OpPropose)
	case !depositKind(kind):
		return refusedCall("a deposit is a " + strings.Join(DepositKinds, ", a ") +
			"; " + quotedKind(kind) + " is none of them")
	case strings.TrimSpace(text) == "":
		return refusedCall("this tool needs the words of the " + kind + " to offer; a deposit is one entry of the record")
	case utf8.RuneCountInString(text) > bound(kind):
		return refusedCall("the " + kind + " carries at most " + strconv.Itoa(bound(kind)) +
			" characters and this one is " + strconv.Itoa(utf8.RuneCountInString(text)) +
			"; offer the entry itself, shorter")
	}
	if over := longestClause(anchor, reason, consequence, clauseSaid); over != "" {
		return refusedCall("the " + over + " on a deposit carries at most " + strconv.Itoa(maxClause) +
			" characters; say the rest in your answer")
	}
	if kind == DepositFinding {
		finding.Reason = reason
		finding.Severity, finding.Recommend = strings.ToLower(oneLine(finding.Severity)), strings.ToLower(oneLine(finding.Recommend))
		finding.Title, finding.Why = oneLine(finding.Title), oneLine(finding.Why)
		if over := longestClause(finding.Title, "", finding.Why, ""); over != "" {
			return refusedCall("the title and why it matters on a finding carry at most " + strconv.Itoa(maxClause) +
				" characters each; say the rest in the finding's own words")
		}
		if refusal := FindingRefusal(finding); refusal != "" {
			return refusedCall(refusal)
		}
	}
	// The clause each kind takes, and only that one. A reason on a fact and an
	// anchor on a decision are the Partner filling in a field the card has no
	// place for, so they are dropped here rather than carried into a card that
	// cannot show them.
	built := DepositHeader + kind + "\n"
	switch kind {
	case DepositFact:
		built += labelled(DepositAnchor, anchor)
	case DepositDecision:
		built += labelled(DepositReason, reason)
	case DepositQuestion:
		built += labelled(DepositConsequence, consequence)
	case DepositFinding:
		// A finding carries where it sits and what follows from leaving it,
		// as its evidence, and the plain layers a person reads it by: how much
		// it matters, the problem, why, and the decision the reviewer
		// recommends with its reason (review-findings-read-as-decisions §3).
		built += labelled(DepositAnchor, anchor) + labelled(DepositConsequence, consequence) +
			labelled(DepositSeverity, finding.Severity) + labelled(DepositTitle, finding.Title) +
			labelled(DepositWhy, finding.Why) + labelled(DepositRecommends, finding.Recommend) +
			labelled(DepositReason, reason)
	case DepositCase:
		// A case takes both of the clauses its two presses need: the clause it
		// would become, which the Decide sheet opens with, and the consequence
		// of leaving it open, which Leave open records with the question
		// (g1-s55 D1). It is the one kind with two, because it is the one kind
		// offered as a choice between two entries.
		built += labelled(DepositClause, clauseSaid) + labelled(DepositConsequence, consequence)
	}
	return Result{Prepared: DepositedLine + "\n" + built + DepositSeparator + "\n" + text + "\n"}
}

// labelled is one framing line, or nothing where the Partner gave no clause. A
// fact with no anchor and a decision with no reason are prepared all the same:
// the card refuses Record it until the human has supplied one, which is where
// that requirement belongs — they may know the anchor the Partner could not
// find (g1-s53 D4).
func labelled(label, said string) string {
	if strings.TrimSpace(said) == "" {
		return ""
	}
	return label + said + "\n"
}

func depositKind(kind string) bool {
	for _, known := range DepositKinds {
		if known == kind {
			return true
		}
	}
	return false
}

// bound is how much one kind of deposit says. Every kind is one entry of a
// list except the outcome, which is a whole section of the record.
func bound(kind string) int {
	if kind == DepositOutcome {
		return maxOutcome
	}
	return maxDeposit
}

func longestClause(anchor, reason, consequence, clauseSaid string) string {
	for _, one := range []struct {
		name string
		said string
	}{{"anchor", anchor}, {"reason", reason}, {"consequence", consequence}, {"clause", clauseSaid}} {
		if utf8.RuneCountInString(one.said) > maxClause {
			return one.name
		}
	}
	return ""
}

func quotedKind(kind string) string {
	if strings.TrimSpace(kind) == "" {
		return "a deposit with no kind at all"
	}
	return `"` + kind + `"`
}
