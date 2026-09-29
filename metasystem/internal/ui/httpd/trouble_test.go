package httpd

import (
	"net/http"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner/fakeacp"
)

// The turn route accepts a trouble beside the page (g1-s68 §6). The server
// composes the human's turn from the fixed sentence; what the page sent as
// text is not the question.

func TestATroubleIsAskedThroughTheTurnRoute(t *testing.T) {
	t.Parallel()
	served, service := servedPartner(t, fakeacp.Script{Chunks: []string{"What happened: ..."}})
	events, stop := service.Subscribe()
	defer stop()

	accepted := post(t, served, partnerTurnsPath, `{"key":"t1","text":"","about":{"section":"Backlog","path":"/backlog"},`+
		`"trouble":{"text":"work land is refused (REVIEW_STALE)","code":"REVIEW_STALE",`+
		`"where":{"section":"Backlog","path":"/backlog","subject":"g1","kind":"goal"},`+
		`"act":{"verb":"Land","object":"goal","target":"g1"},"at":"2026-09-28T19:41:00Z","tip":"7ed3baf"}}`, nil)
	testutil.Require(t, "accepted", accepted.Code, http.StatusAccepted)
	drain(t, events)

	snapshot := partnerSnapshot(t, get(t, served, partnerPath, nil))
	testutil.Require(t, "both messages", len(snapshot.Messages), 2)
	asked := snapshot.Messages[0]
	testutil.Expect(t, "the question is the fixed sentence", asked.Text, partner.TroubleRequest)
	testutil.Require(t, "and the trouble is kept for its chip", asked.Trouble != nil, true)
	testutil.Expect(t, "with its code", asked.Trouble.Code, "REVIEW_STALE")
	testutil.Expect(t, "and its act", *asked.Trouble.Act, partner.TroubleAct{Verb: "Land", Object: "goal", Target: "g1"})
}

func TestATroubleOverTheBoundIsRefusedAtTheRoute(t *testing.T) {
	t.Parallel()
	served, _ := servedPartner(t, fakeacp.Script{Chunks: []string{"never"}})
	long := strings.Repeat("a", partner.MaxTroubleText+1)
	refused := post(t, served, partnerTurnsPath, `{"key":"t2","about":{},"trouble":{"text":"`+long+`","where":{"section":"Backlog","path":"/backlog"},"at":"x"}}`, nil)
	testutil.Expect(t, "refused as a request", refused.Code, http.StatusBadRequest)
	testutil.Expect(t, "in words", strings.Contains(refused.Body.String(), "at most 2,000 characters"), true)
	snapshot := partnerSnapshot(t, get(t, served, partnerPath, nil))
	testutil.Expect(t, "nothing was asked", len(snapshot.Messages), 0)
}
