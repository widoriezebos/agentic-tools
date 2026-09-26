package uitools

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// The second operation that reads nothing.
//
// A sitting is a working conversation on one record, and the record is its
// memory: it deposits three kinds of working material while it runs — facts
// anchored where they can be checked, decisions recorded then and there with
// the human's own reasons, and open questions named with their consequences.
// One test covers all three: end the sitting at any moment and a fresh worker
// with the same human must be able to go on from the record alone.
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

// The three kinds one deposit can be, and the one that waits.
//
// Proposals are the fourth kind of working material a sitting maintains, and
// they are step 2: the closing deposit is what makes a proposal worth having,
// and the closing deposit is not here (g1-s53 §4). A call that names one is
// refused in words that say so, rather than being silently recorded as
// something else.
const (
	DepositFact     = "fact"
	DepositDecision = "decision"
	DepositQuestion = "question"
	DepositProposal = "proposal"
)

// DepositKinds is what this build admits, in the order the tool's own
// description lists them.
var DepositKinds = []string{DepositFact, DepositDecision, DepositQuestion}

// The bounds. A deposit is one entry of a record's section: a line or a
// paragraph, with one short clause beside it. Past these the call is refused in
// words rather than cut, because half an entry offered as a whole one is an
// entry a human cannot read as what the Partner meant.
const (
	// maxDeposit is how much one deposit says, in characters.
	maxDeposit = 2000
	// maxClause is how much the anchor, the reason or the consequence carries.
	// An anchor is a path with a line in it; a reason is a sentence.
	maxClause = 500
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
	DepositSeparator   = "--- the deposit follows, whole and to the end ---"
)

// DepositedLine is what a prepared deposit answers the model with.
//
// It says "prepared" and then says what preparing is not, for the reason the
// suggestion's own line does: a Partner that read "recorded" would tell a human
// their decision is in the record when nothing has been written anywhere, and
// the paper's own rule is that the room proposes and only records rule.
const DepositedLine = "prepared; preparing does not record it: the human presses Record it, " +
	"and it enters the record then and not before"

// deposit prepares one deposit, or refuses the call in words.
func deposit(kind, text, anchor, reason, consequence string) Result {
	kind = strings.ToLower(oneLine(kind))
	anchor, reason, consequence = oneLine(anchor), oneLine(reason), oneLine(consequence)
	// Only the framing's own newlines are the framing's. What the model wrote
	// travels as it wrote it, apart from the blank lines a transport may have
	// left at either end of it.
	text = strings.Trim(text, "\n")
	switch {
	case kind == DepositProposal:
		return refusedCall("this interface records facts, decisions and open questions in a sitting; " +
			"a proposal is not one of them yet, so say it in words instead")
	case !depositKind(kind):
		return refusedCall("a deposit is a " + strings.Join(DepositKinds, ", a ") +
			"; " + quotedKind(kind) + " is none of them")
	case strings.TrimSpace(text) == "":
		return refusedCall("this tool needs the words of the " + kind + " to offer; a deposit is one entry of the record")
	case utf8.RuneCountInString(text) > maxDeposit:
		return refusedCall("a deposit carries at most " + strconv.Itoa(maxDeposit) +
			" characters and this one is " + strconv.Itoa(utf8.RuneCountInString(text)) +
			"; offer the entry itself, shorter")
	}
	if over := longestClause(anchor, reason, consequence); over != "" {
		return refusedCall("the " + over + " on a deposit carries at most " + strconv.Itoa(maxClause) +
			" characters; say the rest in your answer")
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

func longestClause(anchor, reason, consequence string) string {
	for _, one := range []struct {
		name string
		said string
	}{{"anchor", anchor}, {"reason", reason}, {"consequence", consequence}} {
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
