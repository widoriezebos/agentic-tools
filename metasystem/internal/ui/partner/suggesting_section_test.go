package partner_test

import (
	"context"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner/fakeacp"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/uitools"
)

// Fold by section (g1-s66 D3): a section drafted anew is offered when it was
// asked for from that document's own page, where the section card stands, and
// is carried with its reason otherwise.

const foldDesign = "plans/designs/user-interface/g1-s66-the-loop-from-the-room.md"

func serviceFolding(t *testing.T, document, section, text string) *partner.Service {
	t.Helper()
	root := t.TempDir()
	host := partner.NewHostOn(
		partner.Runtime{Name: "fake", ReadOnly: "a fake server reads nothing"},
		root, fakeacp.Open(fakeacp.Script{
			Reads: []fakeacp.Read{{
				Name:  "mcp__metasystem__suggest",
				Title: "suggest(" + document + " · " + section + ")",
				Result: uitools.PreparedSectionLine + "\n" +
					uitools.SectionSuggestionHeader + document + uitools.SuggestionJoin + section + "\n" +
					uitools.SuggestionSeparator + "\n" + text + "\n",
			}},
			Chunks: []string{"Here is the section with the finding folded in."},
		}))
	t.Cleanup(host.Close)
	return partner.NewService(host.Runtime(), host,
		func(human string) (*partner.Conversation, error) { return partner.OpenConversation(root, human) },
		partner.Facts{}, func() time.Time { return time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC) })
}

func sectionSuggestionsOf(t *testing.T, service *partner.Service, page partner.Page) []partner.Suggestion {
	t.Helper()
	events, stop := service.Subscribe()
	defer stop()
	_, err := service.Submit(context.Background(), "Wido", "key-1", "fold S66-01 into section 4", page)
	testutil.Require(t, "admitted", err, nil)
	drain(t, events)
	read, err := service.Snapshot("Wido", 100)
	testutil.Require(t, "read back", err, nil)
	testutil.Require(t, "two messages", len(read.Messages), 2)
	return read.Messages[1].Suggestions
}

func TestASectionAskedForFromItsDocumentsPageIsOffered(t *testing.T) {
	t.Parallel()
	text := "## 4. Decisions\n\n- D1. The act carries the budget."
	service := serviceFolding(t, foldDesign, "4. Decisions", text)
	suggestions := sectionSuggestionsOf(t, service, partner.Page{Section: "Project", Path: "/project/" + foldDesign, Kind: "document", Subject: foldDesign})
	testutil.Require(t, "one suggestion", len(suggestions), 1)
	testutil.Expect(t, "for the document", suggestions[0].Document, foldDesign)
	testutil.Expect(t, "and the heading", suggestions[0].Section, "4. Decisions")
	testutil.Expect(t, "the words whole", suggestions[0].Text, text)
	testutil.Expect(t, "offered", suggestions[0].Offered, true)
	testutil.Expect(t, "belonging to no sheet's opening", suggestions[0].Opening, "")
}

func TestASectionAskedForFromAnotherPageIsCarriedWithItsReason(t *testing.T) {
	t.Parallel()
	service := serviceFolding(t, foldDesign, "4. Decisions", "## 4. Decisions\n\nwords")
	suggestions := sectionSuggestionsOf(t, service, partner.Page{Section: "Backlog", Path: "/backlog"})
	testutil.Require(t, "one suggestion", len(suggestions), 1)
	testutil.Expect(t, "not offered", suggestions[0].Offered, false)
	testutil.Expect(t, "with the page to ask from", suggestions[0].Reason,
		"a section is offered on its document's own page; ask from "+foldDesign)
}
