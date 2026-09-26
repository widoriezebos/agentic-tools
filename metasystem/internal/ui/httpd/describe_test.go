package httpd

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/manifest"
)

// Every write route this server dispatches is an act a human can be told
// about.
//
// The list below is written with the route constants themselves, so a route
// that is renamed fails to compile here and a route that is added without a
// sentence saying what it does and what hand it needs fails to pass. A human
// asking the Project Partner "what can I do from this page?" is answered from
// this table; a route missing from it would be an act nobody could find.
func TestEveryWriteRouteIsAnActWithAHand(t *testing.T) {
	t.Parallel()
	routes := []string{
		routeCreateRecord, routeRecordStatus, routeRecordGoals, routeAskQuestion,
		routeQuestionStatus, routeEditDocument, routePreviewSource,
		routeSignIn, routeSignOut,
		routeApprove, routeWithdraw, routePriority, routeOpen, routeBlock, routeUnblock,
		routePark, routeUnpark,
		routePartnerTurn, routePartnerStop, routePartnerSeeing,
		routePartnerSitting, routePartnerRise,
		routeAddSticky, routeEditSticky, routeRemoveSticky,
	}
	// The one read this table describes. A notepad's read belongs beside its
	// three acts because it is the answer all three give: what a human can do
	// from the panel is read theirs, write one, change one and remove one, and
	// a table that named three of the four would be answering "what can I do
	// here?" with three quarters of it.
	reads := []string{routeStickies}
	described := map[string]manifest.Act{}
	for _, act := range Acts() {
		described[act.ID] = act
	}
	testutil.Expect(t, "no act describes a route this server does not dispatch",
		len(described), len(routes)+len(reads))
	for _, route := range append(append([]string{}, routes...), reads...) {
		act, known := described[route]
		testutil.Expect(t, route+" is described", known, true)
		testutil.Expect(t, route+" says what it does", act.Does != "", true)
		testutil.Expect(t, route+" ends its sentence", strings.HasSuffix(act.Does, "."), true)
		testutil.Expect(t, route+" names the hand it needs", act.Requires != "", true)
		testutil.Expect(t, route+" is titled for a human", act.Title != "", true)
	}
}

// The ledger's acts carry mayAct's own rule: a signed-in human, and the boot
// proof only where no Partner runs. A description that promised less would
// send a human at a 403.
func TestTheLedgerActsNameTheHandMayActRequires(t *testing.T) {
	t.Parallel()
	for _, act := range Acts() {
		switch act.ID {
		case routeApprove, routeWithdraw, routePriority, routeOpen, routeBlock, routeUnblock,
			routePark, routeUnpark:
			testutil.Expect(t, act.ID+" needs a signed-in human",
				strings.Contains(act.Requires, "a signed-in human"), true)
			testutil.Expect(t, act.ID+" names the one exception",
				strings.Contains(act.Requires, "has named no Project Partner"), true)
		}
	}
}

// The route answers the join, per request, and says so rather than failing
// when this engine cannot describe itself.
func TestInterfacePayload(t *testing.T) {
	t.Parallel()
	calls := 0
	served := New(Info{Interface: func() (manifest.Manifest, error) {
		calls++
		return manifest.Manifest{
			SchemaVersion: manifest.SchemaVersion,
			Partner:       manifest.Partner{Reads: "the page", Refused: "writing"},
			Sections: []manifest.Section{{ID: "fleet", Title: "Fleet", Projected: false,
				Availability: "This build does not project Fleet."}},
			Acts: Acts(),
		}, nil
	}}, loopback(), testBundle())

	response := request(t, served, http.MethodGet, interfacePath, "127.0.0.1:7878", nil)
	testutil.Require(t, "status", response.Code, http.StatusOK)
	testutil.Expect(t, "content type", response.Header().Get("Content-Type"), "application/json")

	var payload manifest.Manifest
	testutil.Require(t, "decode the response", json.Unmarshal(response.Body.Bytes(), &payload), nil)
	testutil.Expect(t, "it carries the built half", payload.Sections[0].ID, "fleet")
	testutil.Expect(t, "with its availability", payload.Sections[0].Projected, false)
	testutil.Expect(t, "and the acts this server offers", len(payload.Acts), len(Acts()))
	testutil.Expect(t, "and what the Partner may not do", payload.Partner.Refused, "writing")
	testutil.Expect(t, "and it never claims the Partner writes", payload.Partner.Writes, false)

	request(t, served, http.MethodGet, interfacePath, "127.0.0.1:7878", nil)
	testutil.Expect(t, "it is composed per request", calls, 2)

	absent := New(Info{}, loopback(), testBundle())
	refused := request(t, absent, http.MethodGet, interfacePath, "127.0.0.1:7878", nil)
	testutil.Expect(t, "an engine that cannot describe itself says so",
		strings.Contains(refused.Body.String(), "cannot describe its own interface"), true)
}
