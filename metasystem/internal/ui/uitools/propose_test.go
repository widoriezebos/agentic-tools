package uitools_test

import (
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/uitools"
)

// The third tool that reads nothing: one act on one goal, prepared for the
// human to press.
//
// Every assertion here is about a refusal being NAMED. A Partner that is told
// "that is not allowed" tries another spelling; a Partner that is told which
// act takes the field it sent has learned the rule, and the next call is right.
// So the tests below read the words, not a flag.

func prepared(t *testing.T, args uitools.Args) string {
	t.Helper()
	result := uitools.Readers{}.Answer(uitools.OpPropose, args)
	if result.Failed() {
		t.Fatalf("propose was refused: %s", result.Problem)
	}
	return result.Text()
}

// refusedPropose is one refused call, with the words it was refused in. The
// label names what was asked, because one test asks several times and a label
// is what a failure is read by.
func refusedPropose(t *testing.T, named string, args uitools.Args) string {
	t.Helper()
	result := uitools.Readers{}.Answer(uitools.OpPropose, args)
	testutil.Require(t, named+" is refused", result.Failed(), true)
	// A refused call is still readable by the model, in the form every refused
	// call of this server travels in.
	testutil.Expect(t, named+" is told in words",
		strings.Contains(result.Text(), "this call was refused — "+result.Problem), true)
	return result.Problem
}

// The catalogue is the ten acts and their fields, and it is what the tool's
// own description tells the model.
func TestTheProposalCatalogueIsTheTenActsWithTheirFields(t *testing.T) {
	t.Parallel()
	testutil.Expect(t, "ten acts", len(uitools.ProposedActs), 10)
	testutil.Expect(t, "dispatching to their route ids", uitools.ProposeRoutes(), []string{
		"open-goal", "approve-goal", "withdraw-goal", "set-goal-priority",
		"block-goal", "unblock-goal", "park-goal", "unpark-goal", "edit-goal", "abandon-goal",
	})
	// And named as the Partner names them: the route id for the nine that have
	// no public name here yet, and the action's own name for abandon.
	testutil.Expect(t, "named as the Partner names them", uitools.ProposeVerbs(), []string{
		"open-goal", "approve-goal", "withdraw-goal", "set-goal-priority",
		"block-goal", "unblock-goal", "park-goal", "unpark-goal", "edit-goal", "abandon",
	})
	fields := map[string][]string{}
	for _, act := range uitools.ProposedActs {
		fields[act.Route] = act.Fields()
		testutil.Expect(t, act.Route+" says the word the page's button uses", act.Button != "", true)
	}
	testutil.Expect(t, "open takes the route body's own fields", fields["open-goal"],
		[]string{"intent", "nextStep", "severity", "novelty", "exposure", "accumulation", "basis",
			"labels", "blockedBy", "blocks"})
	testutil.Expect(t, "approve takes nothing but the goal", fields["approve-goal"], []string{})
	testutil.Expect(t, "unapprove takes the reason", fields["withdraw-goal"], []string{"reason"})
	testutil.Expect(t, "priority takes the band and the position", fields["set-goal-priority"],
		[]string{"priority", "sequence"})
	testutil.Expect(t, "block takes the blocker", fields["block-goal"], []string{"blocker"})
	testutil.Expect(t, "unblock takes the blocker", fields["unblock-goal"], []string{"blocker"})
	testutil.Expect(t, "park takes the because", fields["park-goal"], []string{"because"})
	testutil.Expect(t, "unpark takes nothing", fields["unpark-goal"], []string{})
	testutil.Expect(t, "edit takes the three the route carries", fields["edit-goal"],
		[]string{"intent", "nextStep", "labels"})
	// Abandon takes the public flags of `metasystem goal abandon`, and its
	// reason reaches the route body as the because that body already has.
	testutil.Expect(t, "abandon takes the reason and the successor", fields["abandon-goal"],
		[]string{"reason", "successor"})
	abandon, known := uitools.ProposedActNamed("abandon")
	testutil.Require(t, "abandon is found by the name the Partner calls it", known, true)
	testutil.Expect(t, "and it dispatches to the route id", abandon.Route, "abandon-goal")
	testutil.Expect(t, "its reason is the route body's because", abandon.BodyField("reason"), "because")
	testutil.Expect(t, "and its successor is spelled the same on both sides",
		abandon.BodyField("successor"), "successor")

	described := map[string]uitools.Tool{}
	for _, tool := range uitools.Catalogue() {
		described[tool.Name] = tool
	}
	tool := described[uitools.OpPropose]
	testutil.Expect(t, "the tool is offered", tool.Name, uitools.OpPropose)
	for _, act := range uitools.ProposedActs {
		testutil.Expect(t, "its description names "+act.Name(),
			strings.Contains(tool.Description, act.Name()+" ("+act.Button+")"), true)
	}
	testutil.Expect(t, "and says words alone propose nothing",
		strings.Contains(tool.Description, uitools.WordsAloneProposeNothing), true)
	testutil.Expect(t, "and the permission rule admits it", uitools.Names(uitools.OpPropose), true)
}

// The fixed frame: the route, the goal, the body's own fields under the body's
// own labels, the separator, and the explanation whole and to the end.
func TestAProposalTravelsInTheFixedFrame(t *testing.T) {
	t.Parallel()
	text := prepared(t, uitools.Args{
		"verb": "park-goal", "goal": "g1-s42", "because": "superseded by the seat inventory (g1-s42)",
		"explanation": "the five name the fleet inventory\n" + uitools.ProposalHeader + "not a header",
	})
	testutil.Expect(t, "it says what preparing is not",
		strings.HasPrefix(text, uitools.PreparedProposalLine+"\n"), true)
	testutil.Expect(t, "the header names the route",
		strings.Contains(text, uitools.ProposalHeader+"park-goal\n"), true)
	testutil.Expect(t, "the goal is named", strings.Contains(text, uitools.ProposalGoal+"g1-s42\n"), true)
	testutil.Expect(t, "the field is under the body's own label",
		strings.Contains(text, uitools.ProposalBecause+"superseded by the seat inventory (g1-s42)\n"), true)
	// Everything after the separator is the Partner's words, so a line of them
	// that reads like a header is carried as words.
	_, explanation, split := strings.Cut(text, uitools.ProposalSeparator+"\n")
	testutil.Require(t, "the separator ends the framing", split, true)
	testutil.Expect(t, "and the explanation is whole", strings.TrimSpace(explanation),
		"the five name the fleet inventory\n"+uitools.ProposalHeader+"not a header")
}

// An open carries its route body's own `id` and no second naming of the same
// goal, and its four risk answers travel as numbers.
func TestAnOpenCarriesTheBodysIdAndTheFourAnswers(t *testing.T) {
	t.Parallel()
	text := prepared(t, uitools.Args{
		"verb": "open-goal", "goal": "refund-worker",
		"intent":   "Every refund lands within a day, with nobody touching the queue.",
		"nextStep": "Read the refund worker's retry loop.",
		"severity": float64(2), "novelty": float64(1), "exposure": float64(2), "accumulation": float64(1),
		"basis":       "payments, one team, one month of history",
		"labels":      []any{"payments", "robustness", "payments"},
		"blockedBy":   []any{"bank-sandbox"},
		"explanation": "as discussed",
	})
	testutil.Expect(t, "the subject is the body's id",
		strings.Contains(text, uitools.ProposalID+"refund-worker\n"), true)
	testutil.Expect(t, "and is not said twice", strings.Contains(text, uitools.ProposalGoal), false)
	testutil.Expect(t, "the answers travel as numbers",
		strings.Contains(text, uitools.ProposalSeverity+"2\n") &&
			strings.Contains(text, uitools.ProposalNovelty+"1\n") &&
			strings.Contains(text, uitools.ProposalExposure+"2\n") &&
			strings.Contains(text, uitools.ProposalAccumulation+"1\n"), true)
	testutil.Expect(t, "a label named twice is one label",
		strings.Contains(text, uitools.ProposalLabels+"payments, robustness\n"), true)
	testutil.Expect(t, "and what it waits for is carried",
		strings.Contains(text, uitools.ProposalBlockedBy+"bank-sandbox\n"), true)
	// The tier is derived on the card and is never the Partner's to name, so
	// no frame line carries one.
	testutil.Expect(t, "no tier travels", strings.Contains(text, "Tier"), false)
}

// The two acts that carry nothing but the goal are prepared from the goal
// alone, and a list field given empty is a statement an edit makes.
func TestTheActsThatCarryNothingButTheGoal(t *testing.T) {
	t.Parallel()
	for _, verb := range []string{"approve-goal", "unpark-goal"} {
		text := prepared(t, uitools.Args{"verb": verb, "goal": "g1-s14", "explanation": "as discussed"})
		testutil.Expect(t, verb+" is prepared from the goal alone",
			strings.Contains(text, uitools.ProposalHeader+verb+"\n"+uitools.ProposalGoal+"g1-s14\n"), true)
	}
	// An edit's labels emptied is the human clearing them, which the route
	// takes as an empty list; it is a field given, so the edit changes something.
	text := prepared(t, uitools.Args{
		"verb": "edit-goal", "goal": "g1-s18", "labels": []any{}, "explanation": "the labels moved to the arc",
	})
	testutil.Expect(t, "an emptied label list is a change",
		strings.Contains(text, uitools.ProposalHeader+"edit-goal\n"), true)
	// And it is IN the frame, empty: a field nobody sent and a field sent empty
	// are two different statements, and the second is how labels are cleared.
	testutil.Expect(t, "and the frame carries the emptied list",
		strings.Contains(text, "\n"+uitools.ProposalLabels+"\n"), true)
}

// An unknown act is refused with the nine this interface has; a goal verb this
// interface has no route for is refused by name, so the Partner can say where
// it is done.
func TestAnActThisInterfaceDoesNotHaveIsRefusedByName(t *testing.T) {
	t.Parallel()
	words := refusedPropose(t, "an unknown act", uitools.Args{"verb": "retire-goal", "goal": "g1-s42", "explanation": "x"})
	testutil.Expect(t, "the ten are named", strings.Contains(words, "open-goal"), true)
	testutil.Expect(t, "abandon among them", strings.Contains(words, "abandon"), true)
	testutil.Expect(t, "and the unknown one is quoted", strings.Contains(words, `"retire-goal"`), true)

	// Abandon left this list when the ruling admitted it; the acts still not
	// here are refused by name with where they are done.
	elsewhere := refusedPropose(t, "done", uitools.Args{"verb": "done", "goal": "g1-s42", "explanation": "x"})
	testutil.Expect(t, "done is named as an act this interface has not",
		strings.Contains(elsewhere, "done is not an act this interface has"), true)
	testutil.Expect(t, "and the human is sent to a terminal",
		strings.Contains(elsewhere, "done at a terminal"), true)

	none := refusedPropose(t, "no act at all", uitools.Args{"goal": "g1-s42", "explanation": "x"})
	testutil.Expect(t, "a call naming no act is told what the acts are",
		strings.Contains(none, "needs the act to propose"), true)
}

// A field of another act is refused by naming the act it belongs to, and the
// act's own fields beside it.
func TestAFieldAnotherActTakesIsRefusedByNamingThatAct(t *testing.T) {
	t.Parallel()
	words := refusedPropose(t, "a park's because on an unapprove", uitools.Args{
		"verb": "withdraw-goal", "goal": "g1-s14", "because": "not now", "reason": "the date passed",
		"explanation": "x",
	})
	testutil.Expect(t, "the field is named", strings.Contains(words, "because is not a field withdraw-goal takes"), true)
	testutil.Expect(t, "with the act it belongs to", strings.Contains(words, "because is park-goal's"), true)
	testutil.Expect(t, "and what this act does take", strings.Contains(words, "withdraw-goal takes reason"), true)

	// The body field `id` belongs to the open alone; every other act names its
	// goal with `goal`.
	onOther := refusedPropose(t, "an id on a park", uitools.Args{
		"verb": "park-goal", "goal": "g1-s14", "id": "g1-s14", "because": "not now", "explanation": "x",
	})
	testutil.Expect(t, "id is the open's own field",
		strings.Contains(onOther, "id is the new goal's own field on open-goal"), true)
}

// A key no act takes at all is refused by name.
//
// It is the misspelling that matters, and it is how a field is lost in silence:
// a Partner that sent `blocked_by` where the body says `blockedBy` used to be
// told it had prepared the action, and the card carried no dependency — so a
// press opened a goal that waited for nothing.
func TestAKeyNoActTakesIsRefusedByName(t *testing.T) {
	t.Parallel()
	words := refusedPropose(t, "a misspelt blockedBy", uitools.Args{
		"verb": "open-goal", "goal": "refund-worker",
		"intent": "i", "nextStep": "n", "basis": "b",
		"severity": float64(1), "novelty": float64(1), "exposure": float64(1), "accumulation": float64(1),
		"blocked_by":  []any{"bank-sandbox"},
		"explanation": "x",
	})
	testutil.Expect(t, "the key is quoted back",
		strings.Contains(words, `"blocked_by" is not a field this tool takes`), true)
	testutil.Expect(t, "with what this act does take",
		strings.Contains(words, "open-goal takes intent, nextStep"), true)
	testutil.Expect(t, "and the three every act takes",
		strings.Contains(words, "beside verb, goal and explanation"), true)

	// The three common fields and the act's own are admitted, so nothing that
	// belongs is caught by this.
	prepared(t, uitools.Args{
		"verb": "park-goal", "goal": "g1-s44", "because": "away", "explanation": "x",
	})

	// And the schema says so too, for a client that validates before it calls.
	described := map[string]uitools.Tool{}
	for _, tool := range uitools.Catalogue() {
		described[tool.Name] = tool
	}
	testutil.Expect(t, "the schema is closed",
		described[uitools.OpPropose].InputSchema["additionalProperties"], false)
}

// Every authority and plumbing field, and every form the routes cannot carry,
// is refused by its own name with where it can be done instead.
func TestTheAuthorityAndPlumbingFieldsAreRefusedByName(t *testing.T) {
	t.Parallel()
	for field, says := range map[string]string{
		"by":                    "never names the hand that acts",
		"origin":                "origin human",
		"lineage":               "the seat's plumbing",
		"temporaryHumanWord":    "a terminal's authority",
		"reviewBy":              "the authority's own field",
		"approvedRef":           "the engine's plumbing",
		"fixtureHumanAuthority": "this kit's own tests",
		"under":                 "not something this interface offers",
		"verified":              "is evidence and is recorded",
		"tier":                  "the card derives it from the four risk answers",
		"why":                   "go in explanation",
		"budget":                "the budget the card displays",
		"risk":                  "travel as severity, novelty, exposure and accumulation",
		"evidence":              "where the work landed",
		"nextAppend":            "send the whole nextStep",
		// The two forms of abandon this interface does not carry, each refused
		// by name with the terminal form that does carry it.
		"waive": "metasystem goal abandon G --reason TEXT --waive DEPENDENT=REASON",
		"also":  "metasystem goal abandon G --reason TEXT --also DEPENDENT",
	} {
		words := refusedPropose(t, field, uitools.Args{
			"verb": "approve-goal", "goal": "g1-s14", "explanation": "x", field: "anything",
		})
		testutil.Expect(t, field+" is refused by name",
			strings.Contains(words, field+" is not a field a proposal carries"), true)
		testutil.Expect(t, "and says where it can be done, for "+field, strings.Contains(words, says), true)
	}
}

// A field the act needs and did not get is refused by that field's name, in the
// words the route or the engine gives for it.
func TestAMissingFieldIsRefusedByName(t *testing.T) {
	t.Parallel()
	for _, one := range []struct {
		args uitools.Args
		says string
	}{
		{uitools.Args{"verb": "park-goal", "goal": "g"}, "park-goal needs because: a pause without a why"},
		{uitools.Args{"verb": "withdraw-goal", "goal": "g"}, "withdraw-goal needs reason"},
		{uitools.Args{"verb": "block-goal", "goal": "g"}, "block-goal needs blocker"},
		{uitools.Args{"verb": "open-goal", "goal": "g"}, "open-goal needs intent"},
		{uitools.Args{"verb": "set-goal-priority", "goal": "g"}, "set-goal-priority needs priority"},
		{uitools.Args{"verb": "abandon", "goal": "g"},
			"abandon needs reason: a goal that will never be worked owes the reader why"},
	} {
		one.args["explanation"] = "x"
		words := refusedPropose(t, one.says, one.args)
		testutil.Expect(t, "it says which field, for "+one.says, strings.Contains(words, one.says), true)
	}
	// An edit that changes none of the three is the route's own refusal: it
	// would publish nothing.
	words := refusedPropose(t, "an edit that changes nothing",
		uitools.Args{"verb": "edit-goal", "goal": "g", "explanation": "x"})
	testutil.Expect(t, "an edit changes at least one field",
		strings.Contains(words, "edit-goal changes at least one of intent, nextStep and labels"), true)
}

// The bounds the sheets hold: the ledger's one-line fields, the clauses, the
// answers' range, and the list a card has to be readable with.
func TestTheBoundsAreTheSheetsOwn(t *testing.T) {
	t.Parallel()
	broken := refusedPropose(t, "a line break", uitools.Args{
		"verb": "edit-goal", "goal": "g", "intent": "one line\nand another", "explanation": "x",
	})
	testutil.Expect(t, "a line break in an intent is refused before the card exists",
		strings.Contains(broken, "one line in the ledger; fold the line breaks"), true)

	long := refusedPropose(t, "a long clause", uitools.Args{
		"verb": "park-goal", "goal": "g", "because": strings.Repeat("a", 501), "explanation": "x",
	})
	testutil.Expect(t, "a clause is bounded", strings.Contains(long, "carries at most 500 characters"), true)

	answer := refusedPropose(t, "a risk answer out of range", uitools.Args{
		"verb": "open-goal", "goal": "g", "intent": "i", "nextStep": "n", "basis": "b",
		"severity": float64(4), "novelty": float64(1), "exposure": float64(1), "accumulation": float64(1),
		"explanation": "x",
	})
	testutil.Expect(t, "a risk answer is 1, 2 or 3",
		strings.Contains(answer, "the risk answer severity is 1, 2 or 3"), true)

	names := make([]any, 26)
	for at := range names {
		names[at] = "goal-" + strings.Repeat("x", at+1)
	}
	list := refusedPropose(t, "too many names", uitools.Args{
		"verb": "open-goal", "goal": "g", "intent": "i", "nextStep": "n", "basis": "b",
		"severity": float64(1), "novelty": float64(1), "exposure": float64(1), "accumulation": float64(1),
		"blocks": names, "explanation": "x",
	})
	testutil.Expect(t, "a list is bounded so the card can be read",
		strings.Contains(list, "blocks carries at most 25 names"), true)

	said := refusedPropose(t, "a long explanation", uitools.Args{
		"verb": "unpark-goal", "goal": "g", "explanation": strings.Repeat("a", 2001),
	})
	testutil.Expect(t, "and the explanation is bounded",
		strings.Contains(said, "the explanation on one action carries at most 2000 characters"), true)
}

// A proposal always names the goal it is about.
func TestAProposalNamesItsGoal(t *testing.T) {
	t.Parallel()
	words := refusedPropose(t, "an act naming no goal",
		uitools.Args{"verb": "approve-goal", "explanation": "x"})
	testutil.Expect(t, "the goal is asked for by name",
		strings.Contains(words, "needs the goal the action is about, by its ledger id"), true)

	// An open may name it either way, and the two must agree.
	disagree := refusedPropose(t, "an open named twice", uitools.Args{
		"verb": "open-goal", "goal": "a", "id": "b", "intent": "i", "nextStep": "n", "basis": "b",
		"severity": float64(1), "novelty": float64(1), "exposure": float64(1), "accumulation": float64(1),
		"explanation": "x",
	})
	testutil.Expect(t, "an open names one new goal",
		strings.Contains(disagree, "an open names one new goal: goal is a and id is b"), true)
}

// Abandon travels in the frame the route reads: its public flags become the
// route body's own labels, so `reason` arrives as Because and `successor` as
// Successor.
//
// It is the one row where the name the Partner uses and the spelling the body
// decodes differ, and this is where that is held: a frame that wrote Reason
// would hand the runner a field the abandon route has no decoder for, and the
// card would carry no reason at all.
func TestAnAbandonTravelsUnderTheRouteBodysOwnLabels(t *testing.T) {
	t.Parallel()
	text := prepared(t, uitools.Args{
		"verb": "abandon", "goal": "old-idea",
		"reason": "superseded by the seat inventory", "successor": "new-idea",
		"explanation": "the inventory answers what this one was for",
	})

	testutil.Expect(t, "the header names the route the runner dispatches on",
		strings.Contains(text, uitools.ProposalHeader+"abandon-goal\n"), true)
	testutil.Expect(t, "the goal is named", strings.Contains(text, uitools.ProposalGoal+"old-idea\n"), true)
	testutil.Expect(t, "the reason is under the body's own label",
		strings.Contains(text, uitools.ProposalBecause+"superseded by the seat inventory\n"), true)
	testutil.Expect(t, "the successor is under its own",
		strings.Contains(text, uitools.ProposalSuccessor+"new-idea\n"), true)
	testutil.Expect(t, "and nothing writes the public flag's spelling",
		strings.Contains(text, uitools.ProposalReason+"superseded"), false)
	testutil.Expect(t, "the explanation follows whole",
		strings.HasSuffix(text, "the inventory answers what this one was for\n"), true)
}

// A successor is optional: an abandon that names none is prepared, and the
// frame says nothing about one rather than saying it is empty.
func TestAnAbandonWithNoSuccessorIsPrepared(t *testing.T) {
	t.Parallel()
	text := prepared(t, uitools.Args{
		"verb": "abandon", "goal": "old-idea", "reason": "nobody will work it",
		"explanation": "nothing carries this one's work",
	})

	testutil.Expect(t, "the reason is there",
		strings.Contains(text, uitools.ProposalBecause+"nobody will work it\n"), true)
	testutil.Expect(t, "and the successor line is absent",
		strings.Contains(text, uitools.ProposalSuccessor), false)
}

// A field of another act sent to an abandon is refused by naming the act it
// belongs to, and abandon's own fields are named beside it — including under the
// name the Partner calls abandon by.
func TestAFieldSentToAnAbandonIsRefusedByNamingItsOwnAct(t *testing.T) {
	t.Parallel()
	words := refusedPropose(t, "a park's because on an abandon", uitools.Args{
		"verb": "abandon", "goal": "g", "reason": "nobody will work it",
		"because": "nobody will work it", "explanation": "x",
	})

	testutil.Expect(t, "it names the act because belongs to",
		strings.Contains(words, "because is park-goal's"), true)
	testutil.Expect(t, "and says what abandon takes",
		strings.Contains(words, "abandon takes reason and successor"), true)

	elsewhere := refusedPropose(t, "an abandon's successor on a park", uitools.Args{
		"verb": "park-goal", "goal": "g", "because": "not now",
		"successor": "new-idea", "explanation": "x",
	})
	testutil.Expect(t, "the successor is named as abandon's",
		strings.Contains(elsewhere, "successor is abandon's"), true)
}
