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
		Reads:          fakeReads,
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
