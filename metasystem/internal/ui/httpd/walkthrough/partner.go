package main

import (
	"log"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner/fakeacp"
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
var fakeReads = []fakeacp.Read{
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
	service := partner.NewService(runtime, host,
		func(human string) (*partner.Conversation, error) {
			return partner.OpenConversation(checkout, human)
		},
		facts, time.Now)
	log.Printf("Project Partner: fake runtime, conversations under %s", checkout)
	return service
}
