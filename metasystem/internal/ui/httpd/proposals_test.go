package httpd

import (
	"reflect"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/uitools"
)

// The proposal catalogue and the act table are one list, read two ways.
//
// The catalogue is in the tool server, because that is where the Partner's
// arguments are validated; the act table is here, because this is where the
// routes are. Two inventories of what a human can do to a goal is the failure
// this test exists to prevent: a verb the Partner may propose and no route can
// carry is a card that is refused after the human presses it, and a route the
// Partner may not propose is an act the human can only reach by hand.
//
// It reads the act table itself rather than a projection of it: the table is
// what the interface publishes through interface(part="acts"), and a projection
// could agree with the catalogue while the table disagreed with both.

// goalActs is every row of the act table that is a ledger act on one goal.
//
// The hand is what identifies one: a ledger act needs a signed-in human, which
// is mayAct's rule and what ledgerHand says. The three Partner routes, the
// checkout's writes and the notepad's carry other hands and are not acts on a
// goal at all, so nothing here has to name them.
//
// The verdict and the candidate's run take the launch route's stronger hand,
// sessionHand, and are acts on one goal too (g1-s69 §6). Of those, stopping the
// candidate is the one the grammar does not carry: the design gives the
// Partner Run and not Stop (D3).
func goalActs() []string {
	named := []string{}
	for _, act := range Acts() {
		if (act.Requires == ledgerHand || act.Requires == sessionHand) && act.ID != routeAppStop {
			named = append(named, act.ID)
		}
	}
	return named
}

func TestEveryProposableActIsARouteThisInterfaceHas(t *testing.T) {
	t.Parallel()
	described := map[string]string{}
	for _, act := range Acts() {
		described[act.ID] = act.Requires
	}
	for _, act := range uitools.ProposedActs {
		hand, known := described[act.Route]
		testutil.Expect(t, act.Route+" is a route the act table names", known, true)
		// And it is a ledger act: a proposal the human applies publishes under
		// their own session, so a catalogue row whose route needed some other
		// hand would be a card promising an act this hand cannot make.
		testutil.Expect(t, act.Route+" needs a signed-in human", hand == ledgerHand || hand == sessionHand, true)
	}
}

func TestEveryLedgerActOnAGoalCanBeProposed(t *testing.T) {
	t.Parallel()
	for _, route := range goalActs() {
		_, known := uitools.ProposedActOf(route)
		testutil.Expect(t, route+" is in the proposal catalogue", known, true)
	}
	// And the two lists are the same length, so neither can carry a row the
	// other has not heard of.
	testutil.Expect(t, "the two lists are one list",
		len(uitools.ProposedActs), len(goalActs()))
}

// Every field a catalogue row's arguments become is a field of that route's own
// body, under the body's own spelling.
//
// The catalogue's own fields are the PUBLIC flags now (g1-s62 D1), which the
// cmd/metasystem join holds against the descriptor table; what this side holds
// is what the tool maps them to. Body() is that mapping's answer: the route
// body's fields, with an edit's label delta collapsed into the whole list
// admission composes from it. Where the two spellings differ — an abandon's
// public `--reason` is the abandon body's own `because` — it is the mapping
// that is joined here and not the public name, because the mapping is what the
// frame writes with.
//
// The bodies are this package's own structs, so the field names are read from
// their JSON tags: a route that renamed a field would fail here rather than
// leave the Partner proposing a field the decoder refuses. Each row below is
// the body the route decodes, which is the one place the spelling lives.
func TestEveryCatalogueFieldIsItsRoutesOwnBodyField(t *testing.T) {
	t.Parallel()
	// The bodies, by the route that decodes them. An act with no fields has no
	// body worth naming: approve's tuple is the card's and unpark's body is
	// empty, so both are listed with the empty set they take from the Partner.
	bodies := map[string][]string{
		routeOpen:     fieldsOf(openBody{}),
		routeApprove:  {},
		routeWithdraw: fieldsOf(withdrawBody{}),
		routePriority: fieldsOf(priorityBody{}),
		routeBlock:    fieldsOf(edgeBody{}),
		routeUnblock:  fieldsOf(edgeBody{}),
		routePark:     fieldsOf(parkBody{}),
		routeUnpark:   {},
		routeEditGoal: fieldsOf(editGoalBody{}),
		routeAbandon:  fieldsOf(abandonBody{}),
		routeReview:   fieldsOf(reviewBody{}),
		routeAppStart: {},
	}
	for _, act := range uitools.ProposedActs {
		carried, known := bodies[act.Route]
		testutil.Require(t, act.Route+" has a body in this test", known, true)
		for _, field := range act.Body() {
			testutil.Expect(t, act.Route+" carries "+field+" in its route body",
				contains(carried, field), true)
		}
	}
	// The open's own subject is its body's `id`, which is why the catalogue
	// leaves it out of the fields and the frame writes it as the subject.
	testutil.Expect(t, "the open body names the goal itself",
		contains(bodies[routeOpen], "id"), true)
	testutil.Expect(t, "and the catalogue does not ask for it twice",
		contains(mustAct(t, routeOpen).Fields(), "id"), false)
	// An edit's label delta is the one pair that travels unresolved, and the
	// whole list is what the route decodes.
	editing := mustAct(t, routeEditGoal)
	testutil.Expect(t, "an edit's delta travels under the public flags",
		contains(editing.Travels(), "label") && contains(editing.Travels(), "unlabel"), true)
	testutil.Expect(t, "and the route is given the whole list",
		contains(editing.Body(), "labels"), true)
}

// None of the fields a proposal carries is an authority a body could smuggle.
//
// The act bodies cannot carry one — the hand is mayAct's and a name in a body
// authorizes nothing — and this says so from the catalogue's side as well, so a
// field added to a body and to the catalogue together would still fail here.
// No field a proposal sends a route is an authority or a piece of plumbing.
//
// It is the body side of the same rule the cmd/metasystem join holds on the
// public side: what reaches a route is the act's own arguments and never the
// hand that acts, the tier it is recorded under or the box it is approved with.
func TestNoCatalogueFieldIsAnAuthority(t *testing.T) {
	t.Parallel()
	forbidden := []string{
		"by", "origin", "lineage", "temporaryHumanWord", "reviewBy", "approvedRef",
		"fixtureHumanAuthority", "under", "verified", "tier", "why", "budget", "risk", "evidence",
	}
	for _, act := range uitools.ProposedActs {
		for _, field := range act.Body() {
			testutil.Expect(t, act.Route+"'s "+field+" is not an authority",
				contains(forbidden, field), false)
		}
	}
}

func mustAct(t *testing.T, route string) uitools.ProposedAct {
	t.Helper()
	act, known := uitools.ProposedActOf(route)
	testutil.Require(t, route+" is in the catalogue", known, true)
	return act
}

func contains(named []string, wanted string) bool {
	for _, one := range named {
		if one == wanted {
			return true
		}
	}
	return false
}

// fieldsOf is one body's JSON field names, which is the spelling the route
// decodes and therefore the spelling the Partner must use.
func fieldsOf(body any) []string {
	named := []string{}
	for _, field := range structFields(body) {
		if field != "" && field != "-" {
			named = append(named, field)
		}
	}
	return named
}

func structFields(body any) []string {
	shape := reflect.TypeOf(body)
	named := make([]string, 0, shape.NumField())
	for at := range shape.NumField() {
		name, _, _ := strings.Cut(shape.Field(at).Tag.Get("json"), ",")
		named = append(named, name)
	}
	return named
}
