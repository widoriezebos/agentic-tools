package main

import (
	"log"
	"path/filepath"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner/fakeacp"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/uitools"
)

// The walkthrough's Project Partner: the real host, the real conversation
// owner, the real routes, and a canned ACP server on the other end of a pipe.
//
// Everything above the wire is the shipped code. What is fake is only the
// agent: a server that streams one answer in pieces, reads one file on the way
// so the drawer shows an activity line, and asks once for something the
// permission point refuses so the refusal line is on screen too. The pauses
// are what make Stop and a reload mid-answer things a human can stand in front
// of and do.
//
// `fake` is not an admitted runtime name and cannot become one: partner.Admit
// refuses it, and this file never calls Admit. It builds the Runtime value
// itself, which is the seam's own escape hatch for a client that opens its own
// endpoint.

// The canned answer, in the pieces it streams in.
//
// It names goals this fixture's own ledger carries and a record its own
// checkout holds, because that is what makes the answer's links real: the page
// resolves a name against the conversation's index, and a name no ledger
// carries is text. So the walkthrough can be stood in front of to see a link
// ring the card it names.
var fakeAnswer = []string{
	"It is in **To Do** because nobody has authorised it yet.\n\n",
	"- `g1-s18` carries no approval, so no seat may claim it.\n",
	"- `g1-s12` is ahead of it in the same band.\n\n",
	"The board reads the accepted ledger, ",
	"so this is the tree at the tip the page shows ",
	"rather than whatever the working tree holds.\n\n",
	"### What I would look at next\n\n",
	"The design at plans/designs/reading.md is what governs this work. ",
	"Its next step is written down, so the only thing missing is a human's admission.\n",
}

// What the fake server reads on the way, in the tool server's own shape: one
// read whole and one that left rows behind, so the drawer's "Looked at 2
// things" and its outcomes can be stood in front of.
// The suggestion this fake prepares, for the one editor this slice offers it
// in: the edit sheet's Intent, as the tool's own fixed form carries it.
//
// It is narrowed to the prompts that carry the edit sheet, because a fake that
// prepared it for every question would have the service refuse it on every page
// that handed nothing over — which is a true refusal and a noisy fixture.
//
// It is narrowed to the sheet being OPEN rather than to its draft having been
// handed over, so that both halves of the interaction can be stood in front of
// on this fixture: with the draft attached the proposal appears under Intent,
// and with the chip's take-back pressed the same call is refused and the
// transcript shows the not-offered card with its reason.
const suggestedIntent = "Every refund lands within a day, with nobody touching the queue."

// The deposits this fake offers a sitting: a fact with its anchor, a decision
// with the reason it heard, and a case at the edge with the clause it would
// become and the consequence of leaving it open.
//
// They are narrowed to the sitting's own opening turn — the one question this
// interface asks on the human's behalf, whose fixed request names the deposit
// tool — so that the whole of the sitting can be stood in front of on this
// fixture: press Start a sitting, and the opening turn arrives marked as the
// interface's with a fact card, a decision card and a case card under it, the
// first two with Record it beside them and the case with Decide and Leave open.
// Asked from anywhere else the same calls are refused, which is a true
// refusal and shows the not-offered card with its reason.
const (
	depositedFact    = "The intent record says a session lasts twelve hours, and nothing recorded says what that limit protects."
	depositedAnchor  = "plans/intent/sessions.md:14"
	depositedChoice  = "The limit counts from last activity rather than from sign-in."
	depositedReason  = "a page nobody has touched for an hour is not a session in use"
	depositedCase    = "A person reads a long page for an hour without touching anything, and the tab is still open."
	depositedClause  = "Reading without input does not keep a session alive."
	depositedFollows = "Long readers are signed out mid-sentence until somebody rules on it."
)

// The closing deposit this fake offers when the human presses End the sitting:
// one outcome, carrying what the sitting settled and what it left open.
//
// It is narrowed to the closing request, so the outcome card appears on the one
// turn that asks for it and on no other — which is what the card means. The
// human then reads it, edits it, and presses Record it, and it becomes the
// record's Outcome section.
const depositedOutcome = `The session limit counts from last activity rather than from sign-in.

Constraints: the mobile client renews differently (internal/session/session.go:212) and is not changed here; nothing already signed in is signed out by the change itself.

Open questions: what the current twelve-hour limit protects — leaving it open means the new limit is chosen without knowing what the old one was for.

On the table: 1 fact, 1 decision, 1 open question.`

// The actions this fake proposes, and the two prompts they are narrowed to.
//
// They are narrowed for the reason the deposits are: a fake that proposed six
// pauses for every question would have the service refuse them wherever the
// goals were not at the tip, which is a true refusal and a noisy fixture. Asked
// with either phrase below, the card appears under the answer with its lines and
// its Apply; asked anything else, nothing is proposed, which is also what the
// design says about words.
//
// The four of "put them away" are what the whole of the run can be stood in
// front of: the first pause applies, the second is refused by name in the
// engine's own sentence and the run goes ON to the third, and the withdrawal
// comes back with an answer that does not say what happened and stops the run
// there. So one press shows a line landing, a refusal passed, and an unresolved
// answer with Try again and Ask the Partner beside it.
//
// The two abandons are the act no page has a button for, so the card is the only
// place either of them can be stood in front of. The first is a goal nothing
// waits for, and it applies; the second names a goal two others wait for with no
// successor, and the engine refuses it in the sentence that says what a human
// does about it — which is the one refusal a slice about an irreversible act has
// to be able to show. They sit before the withdrawal, because the withdrawal
// stops the run.
//
// The fourth is an approve, and it is here for the Decisions inbox (g1-s60): an
// approve is the one line that carries a budget, so it is what the open row's
// read on opening and the bulk sheet's one read are stood in front of. Its goal
// is tier 3, which this fixture declares a budget law for, so the tuple the row
// and the sheet show is the project's own law rather than nothing at all.
//
// The one of "Create it" is the other shape the design draws: a goal talked
// through for ten minutes and then opened whole, with its intent, its first next
// step, the four risk answers the card derives a tier from, the basis, the labels
// and what it waits for.
const (
	proposedPauseApplies = "g1-s44"
	proposedPauseRefused = refusesPause
	proposedWithdraw     = unresolvedWithdraw
	proposedApprove      = "g1-s47"
	proposedOpen         = "refund-worker"
	// The goal nothing waits for, which the abandon lands on, and the goal
	// g1-s23 waits for, which the engine refuses until it is carried, waived or
	// abandoned at a terminal.
	proposedAbandon        = "g1-s46"
	proposedAbandonRefused = "g1-s24"
)

const (
	pausePhrase = "put them away"
	openPhrase  = "Create it"
)

// proposed is one canned propose call, in the tool's own fixed form.
//
// The route is what the frame carries and what the message persists; the title
// is the CALL, so it names the act the way the Partner asked for it, by the
// goal action's public name (g1-s62 D1).
func proposed(when, route, subject string, lines []string, explanation string) fakeacp.Read {
	frame := uitools.ProposalHeader + route + "\n"
	if route != uitools.ProposeOpen {
		frame += uitools.ProposalGoal + subject + "\n"
	}
	for _, line := range lines {
		frame += line + "\n"
	}
	asked := route
	if named, there := uitools.ProposedActOf(route); there {
		asked = named.Action
	}
	return fakeacp.Read{
		When:  when,
		Name:  "mcp__" + uitools.ServerName + "__" + uitools.OpPropose,
		Title: "propose(" + asked + " " + subject + ")",
		Result: uitools.PreparedProposalLine + "\n" + frame +
			uitools.ProposalSeparator + "\n" + explanation + "\n",
	}
}

// The lines of the two fixed requests the deposits are narrowed to. Each is one
// phrase of partner.OpeningRequest and partner.ClosingRequest, so a fixture that
// drifted from a request would stop offering them rather than offer them on
// every question.
const (
	openingPhrase = "Bring what the records already hold about it"
	closingPhrase = "Draft its closing deposit"
)

var fakeReads = []fakeacp.Read{
	{
		When:  "Open sheet: Edit goal",
		Name:  "mcp__" + uitools.ServerName + "__" + uitools.OpSuggest,
		Title: "suggest(Edit goal · Intent)",
		Result: uitools.PreparedLine + "\n" +
			uitools.SuggestionHeader + "Edit goal" + uitools.SuggestionJoin + "Intent\n" +
			uitools.SuggestionSeparator + "\n" + suggestedIntent + "\n",
	},
	{
		When:  openingPhrase,
		Name:  "mcp__" + uitools.ServerName + "__" + uitools.OpDeposit,
		Title: "deposit(fact)",
		Result: uitools.DepositedLine + "\n" +
			uitools.DepositHeader + uitools.DepositFact + "\n" +
			uitools.DepositAnchor + depositedAnchor + "\n" +
			uitools.DepositSeparator + "\n" + depositedFact + "\n",
	},
	{
		When:  openingPhrase,
		Name:  "mcp__" + uitools.ServerName + "__" + uitools.OpDeposit,
		Title: "deposit(decision)",
		Result: uitools.DepositedLine + "\n" +
			uitools.DepositHeader + uitools.DepositDecision + "\n" +
			uitools.DepositReason + depositedReason + "\n" +
			uitools.DepositSeparator + "\n" + depositedChoice + "\n",
	},
	{
		When:  openingPhrase,
		Name:  "mcp__" + uitools.ServerName + "__" + uitools.OpDeposit,
		Title: "deposit(case)",
		Result: uitools.DepositedLine + "\n" +
			uitools.DepositHeader + uitools.DepositCase + "\n" +
			uitools.DepositClause + depositedClause + "\n" +
			uitools.DepositConsequence + depositedFollows + "\n" +
			uitools.DepositSeparator + "\n" + depositedCase + "\n",
	},
	{
		When:  closingPhrase,
		Name:  "mcp__" + uitools.ServerName + "__" + uitools.OpDeposit,
		Title: "deposit(outcome)",
		Result: uitools.DepositedLine + "\n" +
			uitools.DepositHeader + uitools.DepositOutcome + "\n" +
			uitools.DepositSeparator + "\n" + depositedOutcome + "\n",
	},
	proposed(pausePhrase, uitools.ProposePark, proposedPauseApplies,
		[]string{uitools.ProposalBecause + "superseded by the seat inventory (g1-s42)"},
		"the three name the fleet inventory g1-s42 built, so the census answers what they were for"),
	proposed(pausePhrase, uitools.ProposePark, proposedPauseRefused,
		[]string{uitools.ProposalBecause + "superseded by the seat inventory (g1-s42)"},
		"the same inventory covers the phase this one publishes"),
	proposed(pausePhrase, uitools.ProposeAbandon, proposedAbandon,
		[]string{uitools.ProposalBecause + "the whole chain is read by the census now"},
		"nothing waits for this one, so retiring it costs nothing else"),
	proposed(pausePhrase, uitools.ProposeAbandon, proposedAbandonRefused,
		[]string{uitools.ProposalBecause + "the format was decided in the inventory"},
		"this one is the blocker g1-s23 waits for, so it needs a successor or a terminal"),
	proposed(pausePhrase, uitools.ProposeWithdraw, proposedWithdraw,
		[]string{uitools.ProposalReason + "the budget assumed a July start"},
		"this one was approved against a date that has passed"),
	// An approve takes no arguments of its own: the tuple is the card's, and the
	// inbox row's, from the backlog read each makes when it can show it.
	proposed(pausePhrase, uitools.ProposeApprove, proposedApprove, nil,
		"the fence work this one names has landed, so it can be admitted now"),
	proposed(openPhrase, uitools.ProposeOpen, proposedOpen, []string{
		uitools.ProposalIntent + "Every refund lands within a day, with nobody touching the queue.",
		uitools.ProposalNextStep + "Read the refund worker's retry loop and write the case where the bank answers late.",
		uitools.ProposalLabels + "payments, robustness",
		uitools.ProposalID + proposedOpen,
		uitools.ProposalSeverity + "2",
		uitools.ProposalNovelty + "1",
		uitools.ProposalExposure + "2",
		uitools.ProposalAccumulation + "1",
		uitools.ProposalBasis + "payments, one team, one month of history",
		uitools.ProposalBlockedBy + "g1-s24",
	}, "as discussed: the July incident and the two open asks it left behind"),
	{
		Title: "document(plans/designs/reading.md)",
		Result: "Source: plans/designs/reading.md as it stands, revision blob:7f31c0\n" +
			"Supplied: 412 of 412\n\n" +
			"- Kind: design\n- Status: draft\n\n# The reading pane\n\n" +
			"A document is read as a chapter of a book rather than as a file.\n",
	},
	{
		Title: "board(filters: waiting)",
		Result: "Source: the accepted tip 6984cde, observed 2026-09-23T17:54:00Z\n" +
			"Supplied: 2 of 5\n" +
			"More remains: call this tool again with cursor \"2\".\n\n" +
			"- waiting: waiting · Do waiting. · tier 2 · parked\n" +
			"- ready: running · Do running. · tier 3 · 2:6 · claimed · seat m1e\n",
	},
}

// fixtureConversations is where this fixture's transcripts go: a directory
// beside the fixture checkout, and NEVER the account's own.
//
// The server resolves the real one from the account's registry home, and a
// walkthrough that wrote there would put a fake Partner's words into a human's
// actual conversation — which is exactly the material g1-s53 D11 moved out of
// the checkout to keep private. So the fixture is handed a directory of its own
// and prints it, as it does for the notepad and the checkout it invents.
func fixtureConversations(checkout string) string {
	return filepath.Join(filepath.Dir(checkout), filepath.Base(checkout)+"-conversations")
}

func fakePartner(checkout string, facts partner.Facts) *partner.Service {
	runtime := partner.Runtime{
		Name:  "fake",
		Model: "fake-1",
		ReadOnly: "a canned server that reads nothing and writes nothing; " +
			"every request it makes is refused by the permission point",
		// The hand-off, as the walkthrough shows it: the fake server records
		// what session/new gave it and answers from a script, so nothing here
		// starts a process.
		Tools: &partner.ToolServer{
			Name: "metasystem", Command: "bin/metasystem",
			Args: []string{"ui", "tools", "--root", checkout},
		},
	}
	host := partner.NewHostOn(runtime, checkout, fakeacp.Open(fakeacp.Script{
		Models:         []string{"fake-1"},
		Reads:          append(append(append([]fakeacp.Read{}, fakeReads...), reviewReads...), shapingReads...),
		Answers:        append(append([]fakeacp.Answer{}, shapingAnswers...), reviewAnswers...),
		AskFor:         "mcp__metasystem__board",
		Permission:     "Write plans/goals/waiting.md",
		PermissionKind: "edit",
		Chunks:         fakeAnswer,
		Pause:          450 * time.Millisecond,
	}))
	conversations := fixtureConversations(checkout)
	service := partner.NewService(runtime, host,
		func(human string) (*partner.Conversation, error) {
			return partner.OpenConversation(conversations, human)
		},
		facts, time.Now)
	log.Printf("Project Partner: fake runtime, conversations under %s", conversations)
	return service
}
