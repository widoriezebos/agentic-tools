package httpd

import (
	"net/http"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/act"
)

// The two directions of one relation, at the boundary: how the open request
// names them, and how the two edge routes name the goal that waits.

// A field that used to be one string and is now a list has to read both, or
// the first act a browser makes after the engine is replaced is refused for a
// field the human never touched.
func TestOpenRouteReadsBlocksAsAStringOrAList(t *testing.T) {
	t.Parallel()

	const head = `{"id":"ui-new","intent":"The board opens a goal.","nextStep":"Take it.",` +
		`"tier":0,"why":"","labels":[],"severity":1,"novelty":1,"exposure":1,"accumulation":1,` +
		`"basis":"one change behind an existing seam"`

	cases := []struct {
		name      string
		fields    string
		blocks    []string
		blockedBy []string
	}{
		{name: "the empty string is no goals", fields: `,"blocks":""}`},
		{name: "one string is one goal", fields: `,"blocks":"alpha"}`, blocks: []string{"alpha"}},
		{
			name: "a comma string is several goals, in the order they were named",
			// The order is the human's: the first goal named is the one an
			// open's park takes its marker from, so it must not be sorted.
			fields: `,"blocks":"beta, alpha ,,beta"}`, blocks: []string{"beta", "alpha"},
		},
		{name: "an array is several goals", fields: `,"blocks":["alpha","beta"]}`, blocks: []string{"alpha", "beta"}},
		{name: "an empty array is no goals", fields: `,"blocks":[]}`},
		{name: "null is no goals", fields: `,"blocks":null}`},
		{name: "an absent field is no goals", fields: "}"},
		{
			name:   "both directions travel together",
			fields: `,"blocks":["alpha"],"blockedBy":["gamma","delta"]}`,
			blocks: []string{"alpha"}, blockedBy: []string{"gamma", "delta"},
		},
		{
			name:      "blockedBy alone reads a comma string too",
			fields:    `,"blockedBy":"gamma,delta"}`,
			blockedBy: []string{"gamma", "delta"},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			rec := &acted{authorized: proven()}
			served := New(rec.acting(), loopback(), testBundle())

			response := post(t, served, "/api/backlog/goals", head+test.fields, nil)

			testutil.Require(t, "status", response.Code, http.StatusOK)
			testutil.Require(t, "one open reached the engine", len(rec.opened), 1)
			testutil.Expect(t, "the goals that will wait for this one", rec.opened[0].Blocks, test.blocks)
			testutil.Expect(t, "the goals this one will wait for", rec.opened[0].BlockedBy, test.blockedBy)
		})
	}
}

// A body that is neither a list of ids nor a line of them is the request's
// own fault, and is said so rather than being quietly read as nothing.
func TestOpenRouteRefusesABlocksFieldThatIsNeitherShape(t *testing.T) {
	t.Parallel()
	rec := &acted{authorized: proven()}
	served := New(rec.acting(), loopback(), testBundle())

	response := post(t, served, "/api/backlog/goals",
		`{"id":"ui-new","intent":"x","nextStep":"y","tier":0,"why":"","blocks":7,"labels":[],`+
			`"severity":1,"novelty":1,"exposure":1,"accumulation":1,"basis":"z"}`, nil)

	testutil.Require(t, "status", response.Code, http.StatusBadRequest)
	testutil.Expect(t, "the engine was not reached", rec.reached(), 0)
}

// The id in the path is the goal that WAITS, on both routes, whichever end of
// the relation the page acted from. The blocker travels in the body.
func TestTheTwoEdgeRoutesNameTheGoalThatWaitsInThePath(t *testing.T) {
	t.Parallel()
	rec := &acted{authorized: proven()}
	served := New(rec.acting(), loopback(), testBundle())

	blocked := post(t, served, "/api/backlog/goals/waiting/block", `{"blocker":"the-blocker"}`, nil)
	testutil.Require(t, "status of block", blocked.Code, http.StatusOK)
	unblocked := post(t, served, "/api/backlog/goals/waiting/unblock", `{"blocker":" the-blocker "}`, nil)
	testutil.Require(t, "status of unblock", unblocked.Code, http.StatusOK)

	testutil.Expect(t, "the edge that was written", rec.blocked, [][2]string{{"waiting", "the-blocker"}})
	testutil.Expect(t, "the edge that was removed", rec.unblocked, [][2]string{{"waiting", "the-blocker"}})
	payload := decodeBacklog(t, blocked.Result())
	testutil.Expect(t, "the answer is the backlog", payload["schemaVersion"] != nil, true)
}

// Authority is the act owner's and never the body's: a server nothing proves,
// that nobody has signed into, writes no edge and says how to fix that.
func TestTheTwoEdgeRoutesTakeTheSameAuthorityEveryActTakes(t *testing.T) {
	t.Parallel()
	for _, path := range []string{"/api/backlog/goals/waiting/block", "/api/backlog/goals/waiting/unblock"} {
		rec := &acted{authorized: agentStarted()}
		served := New(rec.acting(), loopback(), testBundle())

		response := post(t, served, path, `{"blocker":"the-blocker"}`, nil)

		testutil.Require(t, "status for "+path, response.Code, http.StatusForbidden)
		refusal := actRefusal(t, response)
		testutil.Expect(t, "the remedy is in the page for "+path, refusal.SignIn, true)
		testutil.Expect(t, "the engine was not reached for "+path, rec.reached(), 0)
	}
}

// An early unblock is the engine's own refusal, in the engine's own words,
// with the status that says the ledger would not have it in this state.
func TestAnEarlyUnblockComesBackAsTheEnginesRefusal(t *testing.T) {
	t.Parallel()
	rec := &acted{
		authorized: proven(),
		refusal: &act.Refusal{
			Kind: act.KindEngine, Code: "refused",
			Message: "goal waiting waits for the-blocker, which is not done; removing an unfinished blocker is an early lift and a human act",
		},
	}
	served := New(rec.acting(), loopback(), testBundle())

	response := post(t, served, "/api/backlog/goals/waiting/unblock", `{"blocker":"the-blocker"}`, nil)

	testutil.Require(t, "status", response.Code, http.StatusConflict)
	testutil.Expect(t, "the engine's own sentence", actRefusal(t, response).Error, rec.refusal.(*act.Refusal).Message)
}
