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

// The canned answer, in the pieces it streams in. It is about the fixture
// board, because that is what the walkthrough's pages show.
var fakeAnswer = []string{
	"Two goals are in **Ready for Work**.\n\n",
	"- `running` — claimed by `m1e`, so it is being worked now.\n",
	"- `waiting` — queued behind an approval that has not been given.\n\n",
	"The board reads the accepted ledger, ",
	"so this is the tree at the tip the page shows ",
	"rather than whatever the working tree holds.\n\n",
	"### What I would look at next\n\n",
	"`waiting` has been queued since it was opened. ",
	"Its next step is written down, so the only thing missing is a human's admission.\n",
}

func fakePartner(checkout string) *partner.Service {
	runtime := partner.Runtime{
		Name:  "fake",
		Model: "fake-1",
		ReadOnly: "a canned server that reads nothing and writes nothing; " +
			"every request it makes is refused by the permission point",
	}
	host := partner.NewHostOn(runtime, checkout, fakeacp.Open(fakeacp.Script{
		Models:         []string{"fake-1"},
		Activity:       "Read plans/goals/backlog.md",
		Permission:     "Write plans/goals/waiting.md",
		PermissionKind: "edit",
		Chunks:         fakeAnswer,
		Pause:          450 * time.Millisecond,
	}))
	service := partner.NewService(runtime, host,
		func(human string) (*partner.Conversation, error) {
			return partner.OpenConversation(checkout, human)
		},
		partner.Facts{}, time.Now)
	log.Printf("Project Partner: fake runtime, conversations under %s", checkout)
	return service
}
