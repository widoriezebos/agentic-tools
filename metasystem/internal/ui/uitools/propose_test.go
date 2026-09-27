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
// act takes the flag it sent has learned the rule, and the next call is right.
// So the tests below read the words, not a flag.
//
// The vocabulary is the public grammar's: the act is the goal object's public
// action and the flags are that action's own, under the descriptors' spellings
// (g1-s62 D1). What the tool maps them to — the route id in the frame's header
// and the route body's own field names in its lines — is asserted here too,
// because that mapping is the whole of what keeps a stored proposal working
// across a rename.

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
	return refusedBy(t, uitools.Readers{}, named, args)
}

func refusedBy(t *testing.T, readers uitools.Readers, named string, args uitools.Args) string {
	t.Helper()
	result := readers.Answer(uitools.OpPropose, args)
	testutil.Require(t, named+" is refused", result.Failed(), true)
	// A refused call is still readable by the model, in the form every refused
	// call of this server travels in.
	testutil.Expect(t, named+" is told in words",
		strings.Contains(result.Text(), "this call was refused — "+result.Problem), true)
	return result.Problem
}

// goalCatalogue is the engine's catalogue as this tool reads it: the goal
// object's public actions with their own usage lines. It carries the one action
// the refusal below quotes and one this interface has, so a build that looked
// the wrong row up would say the wrong form.
func goalCatalogue() []uitools.CommandFamily {
	return []uitools.CommandFamily{{
		Summary: "the public commands, object then action",
		Verbs: []uitools.Command{
			{Name: "goal open", Summary: "declare a new goal",
				Usage: []string{"metasystem goal open G --intent TEXT --next TEXT --risk ANSWERS --basis TEXT"}},
			{Name: "goal done", Summary: "conclude a goal",
				Usage: []string{"metasystem goal done G --reason TEXT"}},
		},
	}}
}

// The catalogue is the ten public actions and their public flags, and it is
// what the tool's own description tells the model.
func TestTheProposalCatalogueIsTheTenPublicActionsWithTheirFlags(t *testing.T) {
	t.Parallel()
	testutil.Expect(t, "ten acts", len(uitools.ProposedActs), 10)
	testutil.Expect(t, "named by the goal object's public actions", uitools.ProposeActions(), []string{
		"open", "approve", "unapprove", "prioritize",
		"block", "unblock", "pause", "resume", "edit", "abandon",
	})
	testutil.Expect(t, "each dispatching to the interface's own route", uitools.ProposeRoutes(), []string{
		"open-goal", "approve-goal", "withdraw-goal", "set-goal-priority",
		"block-goal", "unblock-goal", "park-goal", "unpark-goal", "edit-goal", "abandon-goal",
	})
	words := map[string]string{}
	flags := map[string][]string{}
	for _, act := range uitools.ProposedActs {
		flags[act.Action] = act.Fields()
		words[act.Action] = act.Word
	}
	testutil.Expect(t, "open takes the public flags of goal open", flags["open"],
		[]string{"intent", "next", "risk", "basis", "label", "blocked-by", "blocks"})
	testutil.Expect(t, "approve takes nothing but the goal", flags["approve"], []string{})
	testutil.Expect(t, "unapprove takes the reason", flags["unapprove"], []string{"reason"})
	testutil.Expect(t, "prioritize takes the band and the position", flags["prioritize"],
		[]string{"priority", "sequence"})
	testutil.Expect(t, "block names what the goal waits on", flags["block"], []string{"on"})
	testutil.Expect(t, "unblock names it too", flags["unblock"], []string{"on"})
	testutil.Expect(t, "pause takes the reason", flags["pause"], []string{"reason"})
	testutil.Expect(t, "resume takes nothing", flags["resume"], []string{})
	testutil.Expect(t, "edit takes the two texts and the label delta", flags["edit"],
		[]string{"intent", "next", "label", "unlabel"})
	testutil.Expect(t, "abandon takes the reason and the successor", flags["abandon"],
		[]string{"reason", "successor"})
	// Abandon is the row whose public flag and route body field differ: the
	// reason the Partner writes reaches the route as the `because` that body
	// already has, and the successor is spelt the same on both sides.
	abandon, known := uitools.ProposedActionOf("abandon")
	testutil.Require(t, "abandon is found by its public name", known, true)
	testutil.Expect(t, "and it dispatches to the route id", abandon.Route, "abandon-goal")
	testutil.Expect(t, "its arguments reach the route body as because and successor",
		abandon.Body(), []string{"because", "successor"})

	// One word for one act, everywhere a human reads it: the action's own name.
	testutil.Expect(t, "the ten words are the ten actions' names", words, map[string]string{
		"open": "Open", "approve": "Approve", "unapprove": "Unapprove", "prioritize": "Prioritize",
		"block": "Block", "unblock": "Unblock", "pause": "Pause", "resume": "Resume", "edit": "Edit",
		"abandon": "Abandon",
	})

	described := map[string]uitools.Tool{}
	for _, tool := range uitools.Catalogue() {
		described[tool.Name] = tool
	}
	tool := described[uitools.OpPropose]
	testutil.Expect(t, "the tool is offered", tool.Name, uitools.OpPropose)
	for _, act := range uitools.ProposedActs {
		testutil.Expect(t, "its description names goal "+act.Action,
			strings.Contains(tool.Description, "\n- goal "+act.Action), true)
	}
	testutil.Expect(t, "and sends the model to the command's own help",
		strings.Contains(tool.Description, "`metasystem goal ACTION --help`"), true)
	testutil.Expect(t, "and says words alone propose nothing",
		strings.Contains(tool.Description, uitools.WordsAloneProposeNothing), true)
	testutil.Expect(t, "and the permission rule admits it", uitools.Names(uitools.OpPropose), true)
	testutil.Expect(t, "the schema's enum is the public names",
		tool.InputSchema["properties"].(map[string]any)["verb"].(map[string]any)["enum"],
		uitools.ProposeActions())
}

// The fixed frame: the ROUTE, the goal, the route body's own fields under the
// body's own labels, the separator, and the explanation whole and to the end.
//
// The header is the route and not the public name, because the header is what
// the message persists: a public action renamed tomorrow must not orphan a
// proposal a human left waiting today.
func TestAProposalTravelsInTheFixedFrameUnderTheRoute(t *testing.T) {
	t.Parallel()
	text := prepared(t, uitools.Args{
		"verb": "pause", "goal": "g1-s42", "reason": "superseded by the seat inventory (g1-s42)",
		"explanation": "the five name the fleet inventory\n" + uitools.ProposalHeader + "not a header",
	})
	testutil.Expect(t, "it says what preparing is not",
		strings.HasPrefix(text, uitools.PreparedProposalLine+"\n"), true)
	testutil.Expect(t, "the header names the route the message persists",
		strings.Contains(text, uitools.ProposalHeader+"park-goal\n"), true)
	testutil.Expect(t, "the goal is named", strings.Contains(text, uitools.ProposalGoal+"g1-s42\n"), true)
	// One public word, two route bodies: pause's --reason is the park route's
	// own `because`, and the frame writes the route's spelling.
	testutil.Expect(t, "the reason is under the route body's own label",
		strings.Contains(text, uitools.ProposalBecause+"superseded by the seat inventory (g1-s42)\n"), true)
	testutil.Expect(t, "and not under the other route's",
		strings.Contains(text, uitools.ProposalReason), false)
	// Everything after the separator is the Partner's words, so a line of them
	// that reads like a header is carried as words.
	_, explanation, split := strings.Cut(text, uitools.ProposalSeparator+"\n")
	testutil.Require(t, "the separator ends the framing", split, true)
	testutil.Expect(t, "and the explanation is whole", strings.TrimSpace(explanation),
		"the five name the fleet inventory\n"+uitools.ProposalHeader+"not a header")

	// An unapprove's --reason is the withdraw route's own `reason`.
	withdrawing := prepared(t, uitools.Args{
		"verb": "unapprove", "goal": "g1-s42", "reason": "the budget assumed a July start",
		"explanation": "x",
	})
	testutil.Expect(t, "an unapprove's reason is the withdraw body's own",
		strings.Contains(withdrawing, uitools.ProposalReason+"the budget assumed a July start\n"), true)

	// And block's --on is the edge body's own `blocker`.
	blocking := prepared(t, uitools.Args{
		"verb": "block", "goal": "slice-3", "on": "slice-2", "explanation": "x",
	})
	testutil.Expect(t, "block's --on is the body's blocker",
		strings.Contains(blocking, uitools.ProposalBlocker+"slice-2\n"), true)
}

// An open carries its route body's own `id` and no second naming of the same
// goal, and its four risk answers travel as the body's four numbers, parsed
// from the command's own one flag by the command's own owner.
func TestAnOpenCarriesTheBodysIdAndTheFourAnswers(t *testing.T) {
	t.Parallel()
	text := prepared(t, uitools.Args{
		"verb": "open", "goal": "refund-worker",
		"intent":      "Every refund lands within a day, with nobody touching the queue.",
		"next":        "Read the refund worker's retry loop.",
		"risk":        "severity=2,novelty=1,exposure=2,accumulation=1",
		"basis":       "payments, one team, one month of history",
		"label":       []any{"payments", "robustness", "payments"},
		"blocked-by":  []any{"bank-sandbox"},
		"explanation": "as discussed",
	})
	testutil.Expect(t, "the subject is the body's id",
		strings.Contains(text, uitools.ProposalID+"refund-worker\n"), true)
	testutil.Expect(t, "and is not said twice", strings.Contains(text, uitools.ProposalGoal), false)
	testutil.Expect(t, "the answers travel as the body's four numbers",
		strings.Contains(text, uitools.ProposalSeverity+"2\n") &&
			strings.Contains(text, uitools.ProposalNovelty+"1\n") &&
			strings.Contains(text, uitools.ProposalExposure+"2\n") &&
			strings.Contains(text, uitools.ProposalAccumulation+"1\n"), true)
	testutil.Expect(t, "and the one flag they came in does not travel",
		strings.Contains(text, "\nRisk: "), false)
	testutil.Expect(t, "a label named twice is one label, as the whole list",
		strings.Contains(text, uitools.ProposalLabels+"payments, robustness\n"), true)
	testutil.Expect(t, "and what it waits for is carried under the body's spelling",
		strings.Contains(text, uitools.ProposalBlockedBy+"bank-sandbox\n"), true)
	// The tier is derived on the card and is never the Partner's to name, so
	// no frame line carries one.
	testutil.Expect(t, "no tier travels", strings.Contains(text, "Tier"), false)

	// The four answers are the command's own form, and the command's own reader
	// refuses anything else in its own words.
	words := refusedPropose(t, "a risk in another form", uitools.Args{
		"verb": "open", "goal": "g", "intent": "i", "next": "n", "basis": "b",
		"risk": "severity=2,novelty=1", "explanation": "x",
	})
	testutil.Expect(t, "the command's own reader says what the form is",
		strings.Contains(words, "risk: --risk must be severity=<n>,novelty=<n>,exposure=<n>,accumulation=<n>"), true)

	out := refusedPropose(t, "an answer out of range", uitools.Args{
		"verb": "open", "goal": "g", "intent": "i", "next": "n", "basis": "b",
		"risk": "severity=4,novelty=1,exposure=1,accumulation=1", "explanation": "x",
	})
	testutil.Expect(t, "and that every answer is 1, 2 or 3",
		strings.Contains(out, "with every answer 1, 2, or 3"), true)
}

// The two acts that carry nothing but the goal are prepared from the goal alone,
// and an edit's label delta travels unresolved for admission to compose.
func TestTheActsThatCarryNothingButTheGoal(t *testing.T) {
	t.Parallel()
	for action, route := range map[string]string{"approve": "approve-goal", "resume": "unpark-goal"} {
		text := prepared(t, uitools.Args{"verb": action, "goal": "g1-s14", "explanation": "as discussed"})
		testutil.Expect(t, action+" is prepared from the goal alone",
			strings.Contains(text, uitools.ProposalHeader+route+"\n"+uitools.ProposalGoal+"g1-s14\n"), true)
	}
	// An edit's labels are a DELTA, because the whole list can only be composed
	// where the goal's own labels are read. The two flags travel under their own
	// public spellings and admission composes them.
	text := prepared(t, uitools.Args{
		"verb": "edit", "goal": "g1-s18", "label": []any{"ui"}, "unlabel": []any{"memory"},
		"explanation": "the labels moved to the arc",
	})
	testutil.Expect(t, "an edit is prepared under its route",
		strings.Contains(text, uitools.ProposalHeader+"edit-goal\n"), true)
	testutil.Expect(t, "the labels to add travel under the public flag",
		strings.Contains(text, "\n"+uitools.ProposalLabel+"ui\n"), true)
	testutil.Expect(t, "and the labels to remove too",
		strings.Contains(text, "\n"+uitools.ProposalUnlabel+"memory\n"), true)
	testutil.Expect(t, "and no whole list is invented here",
		strings.Contains(text, uitools.ProposalLabels), false)
	// A field the caller gave empty is still a field given: an --unlabel list
	// sent empty says something about the labels and satisfies the edit's rule.
	emptied := prepared(t, uitools.Args{
		"verb": "edit", "goal": "g1-s18", "unlabel": []any{}, "explanation": "x",
	})
	testutil.Expect(t, "an emptied list is a field given",
		strings.Contains(emptied, "\n"+uitools.ProposalUnlabel+"\n"), true)
}

// An unknown act is refused with the nine this interface has; one of the goal
// object's other public actions is refused with that action's own usage line
// from the engine's catalogue, so the Partner can say where it is done.
func TestAGoalActionTheInterfaceHasNoActForIsRefusedWithItsUsageLine(t *testing.T) {
	t.Parallel()
	words := refusedPropose(t, "an unknown act", uitools.Args{"verb": "retire", "goal": "g1-s42", "explanation": "x"})
	testutil.Expect(t, "the ten are named", strings.Contains(words, "open, approve, unapprove"), true)
	testutil.Expect(t, "abandon among them", strings.Contains(words, "and abandon"), true)
	testutil.Expect(t, "and the unknown one is quoted", strings.Contains(words, `"retire"`), true)

	readers := uitools.Readers{Kit: uitools.Kit{Commands: goalCatalogue}}
	elsewhere := refusedBy(t, readers, "done, with the usage line",
		uitools.Args{"verb": "done", "goal": "g1-s42", "explanation": "x"})
	testutil.Expect(t, "done is named as an act the interface has not",
		strings.Contains(elsewhere, "done is not an act the interface has"), true)
	testutil.Expect(t, "and the terminal's own form comes from the catalogue",
		strings.Contains(elsewhere, "at a terminal: metasystem goal done G --reason TEXT"), true)
	testutil.Expect(t, "and the nine are named beside it",
		strings.Contains(elsewhere, "This interface's acts are open"), true)

	// Every one of the goal object's other actions is refused by its own name,
	// so none of them reaches the unknown-act refusal. Abandon left this list
	// when R-128-ui admitted it (g1-s64).
	for _, action := range []string{
		"done", "reopen", "budget", "claim", "release", "accept-risk", "pin",
		"split", "group", "ungroup", "notes", "sync",
	} {
		said := refusedBy(t, readers, action, uitools.Args{"verb": action, "goal": "g", "explanation": "x"})
		testutil.Expect(t, "the interface has no act for "+action,
			strings.Contains(said, action+" is not an act the interface has"), true)
		testutil.Expect(t, "and "+action+" is sent to a terminal",
			strings.Contains(said, "at a terminal: metasystem goal "+action), true)
	}
	// A build that cannot reach the catalogue says the bare command rather than
	// inventing a form.
	bare := refusedPropose(t, "done with no catalogue", uitools.Args{"verb": "done", "goal": "g", "explanation": "x"})
	testutil.Expect(t, "the bare command is said",
		strings.Contains(bare, "at a terminal: metasystem goal done."), true)

	none := refusedPropose(t, "no act at all", uitools.Args{"goal": "g1-s42", "explanation": "x"})
	testutil.Expect(t, "a call naming no act is told what the acts are",
		strings.Contains(none, "needs the act to propose"), true)
}

// A flag of another act is refused by naming the act it belongs to, and the
// act's own flags beside it.
func TestAFlagAnotherActTakesIsRefusedByNamingThatAct(t *testing.T) {
	t.Parallel()
	words := refusedPropose(t, "a priority on a pause", uitools.Args{
		"verb": "pause", "goal": "g1-s14", "priority": float64(1), "reason": "away",
		"explanation": "x",
	})
	testutil.Expect(t, "the flag is named", strings.Contains(words, "priority is not a flag pause takes"), true)
	testutil.Expect(t, "with the act it belongs to", strings.Contains(words, "priority is prioritize's"), true)
	testutil.Expect(t, "and what this act does take", strings.Contains(words, "pause takes reason"), true)

	// --unlabel is the edit's alone: an open has no labels to remove.
	onOpen := refusedPropose(t, "an unlabel on an open", uitools.Args{
		"verb": "open", "goal": "g", "intent": "i", "next": "n", "basis": "b",
		"risk": "severity=1,novelty=1,exposure=1,accumulation=1", "unlabel": []any{"ui"}, "explanation": "x",
	})
	testutil.Expect(t, "unlabel is the edit's own flag",
		strings.Contains(onOpen, "unlabel is not a flag open takes; unlabel is edit's"), true)

	// An abandon's successor sent to a pause is named as abandon's.
	successor := refusedPropose(t, "a successor on a pause", uitools.Args{
		"verb": "pause", "goal": "g", "reason": "not now", "successor": "new-idea", "explanation": "x",
	})
	testutil.Expect(t, "the successor is named as abandon's",
		strings.Contains(successor, "successor is abandon's"), true)

	// And one public word shared by three acts is admitted by all three: the
	// reason is unapprove's, pause's and abandon's.
	testutil.Expect(t, "the reason belongs to all three",
		strings.Contains(refusedPropose(t, "a reason on an approve", uitools.Args{
			"verb": "approve", "goal": "g", "reason": "x", "explanation": "x",
		}), "reason is unapprove, pause and abandon's"), true)
}

// Abandon travels in the frame the route reads: its public flags become the
// route body's own labels, so `reason` arrives as Because and `successor` as
// Successor.
//
// It is the row where the public flag and the body's spelling differ most
// visibly, and this is where that is held: a frame that wrote Reason would hand
// the runner a field the abandon route has no decoder for, and the card would
// carry no reason at all.
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

// A key no act takes at all is refused by name.
//
// It is the misspelling that matters, and it is how a field is lost in silence:
// a Partner that sent `blocked_by` where the descriptor says `blocked-by` used
// to be told it had prepared the action, and the card carried no dependency — so
// a press opened a goal that waited for nothing.
func TestAKeyNoActTakesIsRefusedByName(t *testing.T) {
	t.Parallel()
	words := refusedPropose(t, "a misspelt blocked-by", uitools.Args{
		"verb": "open", "goal": "refund-worker",
		"intent": "i", "next": "n", "basis": "b",
		"risk":        "severity=1,novelty=1,exposure=1,accumulation=1",
		"blocked_by":  []any{"bank-sandbox"},
		"explanation": "x",
	})
	testutil.Expect(t, "the key is quoted back",
		strings.Contains(words, `"blocked_by" is not a flag this tool takes`), true)
	testutil.Expect(t, "with what this act does take",
		strings.Contains(words, "open takes intent, next, risk"), true)
	testutil.Expect(t, "and the three every act takes",
		strings.Contains(words, "beside verb, goal and explanation"), true)

	// The route bodies' own spellings are not the tool's: a Partner that sent
	// the body's name is told it is not a flag either.
	body := refusedPropose(t, "the route body's own nextStep", uitools.Args{
		"verb": "edit", "goal": "g", "nextStep": "n", "explanation": "x",
	})
	testutil.Expect(t, "the body's spelling is not the public flag",
		strings.Contains(body, `"nextStep" is not a flag this tool takes`), true)

	// The three common fields and the act's own are admitted, so nothing that
	// belongs is caught by this.
	prepared(t, uitools.Args{
		"verb": "pause", "goal": "g1-s44", "reason": "away", "explanation": "x",
	})

	// And the schema says so too, for a client that validates before it calls.
	described := map[string]uitools.Tool{}
	for _, tool := range uitools.Catalogue() {
		described[tool.Name] = tool
	}
	testutil.Expect(t, "the schema is closed",
		described[uitools.OpPropose].InputSchema["additionalProperties"], false)
}

// Every authority and plumbing flag, and every form the routes cannot carry, is
// refused by its own name with where it can be done instead.
func TestTheAuthorityAndPlumbingFlagsAreRefusedByName(t *testing.T) {
	t.Parallel()
	for field, says := range map[string]string{
		"by":                      "never names the hand that acts",
		"id":                      "a proposal names its goal in goal",
		"origin":                  "origin human",
		"lineage":                 "the seat's plumbing",
		"temporary-human-word":    "a terminal's authority",
		"review-by":               "the authority's own field",
		"approved-ref":            "the engine's plumbing",
		"fixture-human-authority": "this kit's own tests",
		"under":                   "not something this interface offers",
		"verified":                "is evidence and is recorded",
		"tier":                    "the card derives it from the four risk answers",
		"why":                     "go in explanation",
		"budget":                  "the budget the card displays",
		"evidence":                "where the work landed",
		"next-append":             "send the whole next",
		// The two forms of abandon this interface does not carry, each refused
		// by name with the terminal form that does carry it.
		"waive":            "metasystem goal abandon G --reason TEXT --waive DEPENDENT=REASON",
		"also":             "metasystem goal abandon G --reason TEXT --also DEPENDENT",
		"intent-file":      "the -file forms read a file at a terminal",
		"next-file":        "the -file forms read a file at a terminal",
		"next-append-file": "the -file forms read a file at a terminal",
		"basis-file":       "the -file forms read a file at a terminal",
		"reason-file":      "the -file forms read a file at a terminal",
		"verified-file":    "the -file forms read a file at a terminal",
	} {
		words := refusedPropose(t, field, uitools.Args{
			"verb": "approve", "goal": "g1-s14", "explanation": "x", field: "anything",
		})
		testutil.Expect(t, field+" is refused by name",
			strings.Contains(words, field+" is not a flag a proposal carries"), true)
		testutil.Expect(t, "and says where it can be done, for "+field, strings.Contains(words, says), true)
	}

	// Two flags the edit command carries and the edit route cannot, refused
	// where the command offers them and admitted where they belong.
	for field, says := range map[string]string{
		"risk":  "a goal's risk is changed at a terminal",
		"basis": "recorded with the risk answers",
	} {
		words := refusedPropose(t, field+" on an edit", uitools.Args{
			"verb": "edit", "goal": "g", "intent": "i", "explanation": "x", field: "anything",
		})
		testutil.Expect(t, field+" is refused on an edit by name",
			strings.Contains(words, field+" is not a flag a proposal carries"), true)
		testutil.Expect(t, "and says where it is done, for "+field, strings.Contains(words, says), true)
	}
	// And both are the open's own.
	prepared(t, uitools.Args{
		"verb": "open", "goal": "g", "intent": "i", "next": "n", "basis": "b",
		"risk": "severity=1,novelty=1,exposure=1,accumulation=1", "explanation": "x",
	})
}

// A flag the act needs and did not get is refused by that flag's name, in the
// words the command or the engine gives for it.
func TestAMissingFlagIsRefusedByName(t *testing.T) {
	t.Parallel()
	for _, one := range []struct {
		args uitools.Args
		says string
	}{
		{uitools.Args{"verb": "pause", "goal": "g"}, "pause needs reason: a pause without a why"},
		{uitools.Args{"verb": "unapprove", "goal": "g"}, "unapprove needs reason: taking an approval back"},
		{uitools.Args{"verb": "block", "goal": "g"}, "block needs on"},
		{uitools.Args{"verb": "unblock", "goal": "g"}, "unblock needs on"},
		{uitools.Args{"verb": "open", "goal": "g"}, "open needs intent"},
		{uitools.Args{"verb": "prioritize", "goal": "g"}, "prioritize needs priority"},
		{uitools.Args{"verb": "open", "goal": "g", "intent": "i", "next": "n"},
			"open needs risk: the four answers are the intake law's classification"},
		{uitools.Args{"verb": "abandon", "goal": "g"},
			"abandon needs reason: a goal that will never be worked is recorded with why"},
	} {
		one.args["explanation"] = "x"
		words := refusedPropose(t, one.says, one.args)
		testutil.Expect(t, "it says which flag, for "+one.says, strings.Contains(words, one.says), true)
	}
	// An edit that changes none of the four is the route's own refusal: it
	// would publish nothing.
	words := refusedPropose(t, "an edit that changes nothing",
		uitools.Args{"verb": "edit", "goal": "g", "explanation": "x"})
	testutil.Expect(t, "an edit changes at least one flag",
		strings.Contains(words, "edit changes at least one of intent, next, label and unlabel"), true)
}

// The bounds the sheets hold: the ledger's one-line fields, the clauses, the
// answers' range, and the list a card has to be readable with.
func TestTheBoundsAreTheSheetsOwn(t *testing.T) {
	t.Parallel()
	broken := refusedPropose(t, "a line break", uitools.Args{
		"verb": "edit", "goal": "g", "intent": "one line\nand another", "explanation": "x",
	})
	testutil.Expect(t, "a line break in an intent is refused before the card exists",
		strings.Contains(broken, "one line in the ledger; fold the line breaks"), true)

	long := refusedPropose(t, "a long clause", uitools.Args{
		"verb": "pause", "goal": "g", "reason": strings.Repeat("a", 501), "explanation": "x",
	})
	testutil.Expect(t, "a clause is bounded", strings.Contains(long, "carries at most 500 characters"), true)

	band := refusedPropose(t, "a priority out of range", uitools.Args{
		"verb": "prioritize", "goal": "g", "priority": float64(4), "explanation": "x",
	})
	testutil.Expect(t, "a priority is 1, 2 or 3", strings.Contains(band, "a priority is 1, 2 or 3"), true)

	place := refusedPropose(t, "a sequence below one", uitools.Args{
		"verb": "prioritize", "goal": "g", "priority": float64(1), "sequence": float64(0), "explanation": "x",
	})
	testutil.Expect(t, "a sequence is one-based",
		strings.Contains(place, "a one-based position within the priority band"), true)

	names := make([]any, 26)
	for at := range names {
		names[at] = "goal-" + strings.Repeat("x", at+1)
	}
	list := refusedPropose(t, "too many names", uitools.Args{
		"verb": "open", "goal": "g", "intent": "i", "next": "n", "basis": "b",
		"risk":   "severity=1,novelty=1,exposure=1,accumulation=1",
		"blocks": names, "explanation": "x",
	})
	testutil.Expect(t, "a list is bounded so the card can be read",
		strings.Contains(list, "blocks carries at most 25 names"), true)

	said := refusedPropose(t, "a long explanation", uitools.Args{
		"verb": "resume", "goal": "g", "explanation": strings.Repeat("a", 2001),
	})
	testutil.Expect(t, "and the explanation is bounded",
		strings.Contains(said, "the explanation on one action carries at most 2000 characters"), true)
}

// A proposal always names the goal it is about, under the one name every public
// form names it by.
func TestAProposalNamesItsGoal(t *testing.T) {
	t.Parallel()
	words := refusedPropose(t, "an act naming no goal",
		uitools.Args{"verb": "approve", "explanation": "x"})
	testutil.Expect(t, "the goal is asked for by name",
		strings.Contains(words, "needs the goal the action is about, by its ledger id"), true)
}
