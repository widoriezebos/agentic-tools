package uitools

import (
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// The third operation that reads nothing.
//
// A human asks the Partner to do something the interface can do to a goal —
// pause six goals, take back an approval, open the goal they have just talked
// through — and the Partner cannot do it: it has no session, no cookie and no
// way to obtain one, and the act routes refuse a cookie-less request while a
// Partner is configured. What it can do is say, in a form the interface can
// render and the human can press, exactly which act it would make and with
// which arguments. That is this tool.
//
// It writes nothing and it applies nothing. It validates shape and bounds, the
// way deposit does, hands the action back in a fixed textual frame, and says
// that preparing applies nothing: the human sees the action on a card and
// decides whether to apply it. Every refusal below is named rather than
// classed — the field, the act, the form — because a Partner told "that is not
// allowed" will guess again, and a Partner told "park-goal takes because" will
// not.
//
// The vocabulary is the INTERFACE'S OWN and nothing else's. A verb here is one
// of the nine route ids the act table publishes, and a field is one of that
// route's own body fields under the body's own spelling. Nothing here reads a
// command catalogue: what the browser can do is what the browser's routes can
// do, and a proposal the routes cannot carry is a proposal that would be
// refused after the human pressed rather than before the card existed.

// The nine acts one proposal can be: the interface's own route ids, exactly as
// the act table names them and the runner dispatches on them.
//
// They are route ids rather than words a human reads, because the route id is
// what the message persists and the runner sends: a word on a button can be
// renamed by a presentation slice without touching a persisted proposal.
const (
	ProposeOpen     = "open-goal"
	ProposeApprove  = "approve-goal"
	ProposeWithdraw = "withdraw-goal"
	ProposePriority = "set-goal-priority"
	ProposeBlock    = "block-goal"
	ProposeUnblock  = "unblock-goal"
	ProposePark     = "park-goal"
	ProposeUnpark   = "unpark-goal"
	ProposeEdit     = "edit-goal"
)

// The fields, under the route bodies' own spellings.
const (
	FieldID           = "id"
	FieldIntent       = "intent"
	FieldNextStep     = "nextStep"
	FieldSeverity     = "severity"
	FieldNovelty      = "novelty"
	FieldExposure     = "exposure"
	FieldAccumulation = "accumulation"
	FieldBasis        = "basis"
	FieldLabels       = "labels"
	FieldBlockedBy    = "blockedBy"
	FieldBlocks       = "blocks"
	FieldReason       = "reason"
	FieldBecause      = "because"
	FieldPriority     = "priority"
	FieldSequence     = "sequence"
	FieldBlocker      = "blocker"
)

// ProposedAct is one row of the catalogue: the route a proposal dispatches to,
// the word the page that offers that act calls it by, and the fields its route
// body carries.
//
// Button is the page's own word, not a second name for the act: the card says
// what the button on the page says, so a human who has pressed Not now on the
// Decisions page reads Not now on the card rather than a synonym for it.
type ProposedAct struct {
	Route  string
	Button string
	// Needs are the fields the route body cannot do without.
	Needs []string
	// Takes are the rest of the body's fields, each admitted and none required.
	Takes []string
	// OneOf, where set, are fields of which at least one must be given. It is
	// the edit route's own rule: a body with all three absent changes nothing
	// and the route refuses it rather than publishing a no-op.
	OneOf []string
}

// Fields is every field this act admits, needed and taken together.
func (a ProposedAct) Fields() []string {
	all := make([]string, 0, len(a.Needs)+len(a.Takes)+len(a.OneOf))
	all = append(all, a.Needs...)
	all = append(all, a.OneOf...)
	return append(all, a.Takes...)
}

// ProposedActs is the whole catalogue, in the order the act table names them.
//
// It is one table, here, and a join test in internal/ui/httpd holds it against
// the act table itself: every route below is a row of Acts(), and every ledger
// act on a goal in Acts() is a row below. Neither list can grow a verb the
// other has not heard of.
var ProposedActs = []ProposedAct{
	{
		Route: ProposeOpen, Button: "Open goal",
		Needs: []string{FieldIntent, FieldNextStep,
			FieldSeverity, FieldNovelty, FieldExposure, FieldAccumulation, FieldBasis},
		Takes: []string{FieldLabels, FieldBlockedBy, FieldBlocks},
	},
	{Route: ProposeApprove, Button: "Approve"},
	{Route: ProposeWithdraw, Button: "Withdraw approval", Needs: []string{FieldReason}},
	{Route: ProposePriority, Button: "Set priority", Needs: []string{FieldPriority}, Takes: []string{FieldSequence}},
	{Route: ProposeBlock, Button: "Waits for", Needs: []string{FieldBlocker}},
	{Route: ProposeUnblock, Button: "No longer waits for", Needs: []string{FieldBlocker}},
	{Route: ProposePark, Button: "Not now", Needs: []string{FieldBecause}},
	{Route: ProposeUnpark, Button: "Return to queue"},
	{Route: ProposeEdit, Button: "Edit", OneOf: []string{FieldIntent, FieldNextStep, FieldLabels}},
}

// ProposedActOf is one row of the catalogue by its route id.
func ProposedActOf(route string) (ProposedAct, bool) {
	for _, act := range ProposedActs {
		if act.Route == route {
			return act, true
		}
	}
	return ProposedAct{}, false
}

// ProposeRoutes is the nine route ids, in the catalogue's order.
func ProposeRoutes() []string {
	named := make([]string, 0, len(ProposedActs))
	for _, act := range ProposedActs {
		named = append(named, act.Route)
	}
	return named
}

// The bounds. They are the sheets' own: an intent and a next step are one line
// each in the ledger, a reason is a sentence, and the explanation is the
// Partner's words for one card rather than a second answer.
const (
	// maxProposalLine is how much one of the ledger's one-line fields carries.
	maxProposalLine = 2000
	// maxProposalClause is how much a reason, a because or a basis carries.
	maxProposalClause = 500
	// maxExplanation is how much the Partner's own why for one action carries.
	maxExplanation = 2000
	// maxProposalList is how many ids or labels one list field carries. A
	// proposal naming forty blockers is a proposal a human cannot read on a
	// card, and the card is the whole point.
	maxProposalList = 25
)

// The fixed form a prepared proposal travels back in, written down once here
// and read once by the interface's host.
//
// It is textual because the result text is the channel this server has. The
// header names the route; the labelled lines after it carry the body's own
// fields under the body's own names; the separator ends the framing; and
// everything after the separator is the Partner's explanation, opaque to the
// end. An explanation may itself hold a line that reads like a header — a
// Partner explaining this very format would — and nothing after the separator
// is read as one.
const (
	ProposalHeader       = "Proposal: "
	ProposalGoal         = "Goal: "
	ProposalReason       = "Reason: "
	ProposalBecause      = "Because: "
	ProposalPriority     = "Priority: "
	ProposalSequence     = "Sequence: "
	ProposalBlocker      = "Blocker: "
	ProposalIntent       = "Intent: "
	ProposalNextStep     = "Next step: "
	ProposalLabels       = "Labels: "
	ProposalID           = "Id: "
	ProposalSeverity     = "Severity: "
	ProposalNovelty      = "Novelty: "
	ProposalExposure     = "Exposure: "
	ProposalAccumulation = "Accumulation: "
	ProposalBasis        = "Basis: "
	ProposalBlockedBy    = "Blocked by: "
	ProposalBlocks       = "Blocks: "
	ProposalSeparator    = "--- the explanation follows, whole and to the end ---"
)

// ProposalFrame is the frame's labelled lines, in the order the frame writes
// them, each with the body field it carries. It is a list rather than a map so
// that the frame's order is the frame's own and a reader can walk it.
var ProposalFrame = []struct {
	Label string
	Field string
}{
	{ProposalReason, FieldReason},
	{ProposalBecause, FieldBecause},
	{ProposalPriority, FieldPriority},
	{ProposalSequence, FieldSequence},
	{ProposalBlocker, FieldBlocker},
	{ProposalIntent, FieldIntent},
	{ProposalNextStep, FieldNextStep},
	{ProposalLabels, FieldLabels},
	{ProposalID, FieldID},
	{ProposalSeverity, FieldSeverity},
	{ProposalNovelty, FieldNovelty},
	{ProposalExposure, FieldExposure},
	{ProposalAccumulation, FieldAccumulation},
	{ProposalBasis, FieldBasis},
	{ProposalBlockedBy, FieldBlockedBy},
	{ProposalBlocks, FieldBlocks},
}

// PreparedProposalLine is what a prepared proposal answers the model with.
//
// It says "prepared" and then says what preparing is not, for the reason the
// suggestion's own line and the deposit's do: a Partner that read "paused" or
// "proposed to the human" would tell a human six goals are away when nothing
// has been written anywhere, and the whole of this design is that the human's
// press is the act.
const PreparedProposalLine = "prepared; preparing applies nothing: " +
	"the human sees the action on a card and decides whether to apply it"

// WordsAloneProposeNothing is the one sentence the tool's description and the
// Partner's instructions both say. It is written once because two places say
// it and a Partner that read two versions of it would have read two rules.
const WordsAloneProposeNothing = "Words alone propose nothing: an action the human can press exists " +
	"only because this tool was called for it."

// The fields a proposal never carries, each refused by name with where it can
// be done instead.
//
// They are named and not classed. A Partner refused "that field is not
// allowed" tries another spelling of it; a Partner refused "a proposal names no
// tier: the card derives it from the risk answers" has learned the rule. Three
// families are here: the authority and plumbing a browser act never carries,
// the two fields the card itself composes, and the forms the routes cannot
// carry at all.
var refusedProposalFields = []struct {
	field string
	words string
}{
	{"by", "a proposal never names the hand that acts: the act publishes under the human's own session"},
	{"origin", "a goal this interface opens is of origin human, and nothing else may be asked for"},
	{"lineage", "a lineage is the seat's plumbing and no part of an act a human presses"},
	{"temporaryHumanWord", "a temporary human word is a terminal's authority and is never proposed"},
	{"reviewBy", "a review date is the authority's own field and is set where the authority is given"},
	{"approvedRef", "an approved ref is the engine's plumbing and is never proposed"},
	{"fixtureHumanAuthority", "a fixture authority belongs to this kit's own tests"},
	{"under", "acting under another authority is not something this interface offers"},
	{"verified", "a verification is evidence and is recorded where the evidence is"},
	{"tier", "a proposal names no tier: the card derives it from the four risk answers and sends what it derived"},
	{"why", "the words you want the human to read go in explanation; a goal's own why is not proposed"},
	{"budget", "an approval carries the budget the card displays and no other; propose the approval and the card shows the tuple"},
	{"risk", "the four risk answers travel as severity, novelty, exposure and accumulation, each a number from 1 to 3"},
	{"evidence", "evidence is recorded where the work landed, not on an edit a human presses"},
	{"nextAppend", "this interface's edit replaces the next step; send the whole nextStep"},
}

// The goal acts this interface has no route for, refused by name.
//
// They are the verbs a human asks for that the browser genuinely cannot make —
// a goal abandoned, a goal done, a claim released — and a Partner that met a
// bare "unknown verb" for one of them would offer the human a card the
// interface could never have. Named, it says the act exists and that this
// interface is not where it is made, so the Partner can say where.
var actsNotInTheInterface = []string{
	"abandon", "done", "reopen", "budget", "claim", "release",
	"accept-risk", "pin", "grant", "revoke", "split", "group", "ungroup", "notes",
}

// propose prepares one action, or refuses the call in words.
//
// The order of the refusals is the order a reader would ask the questions in:
// which act is this, does this interface have it, which goal is it about, is
// every field one that act takes, is every field it needs given, and is every
// value within the bounds the sheets hold.
func propose(args Args) Result {
	verb := strings.ToLower(oneLine(args.Text("verb")))
	act, known := ProposedActOf(verb)
	if !known {
		return refusedCall(unknownVerb(verb))
	}
	explanation := strings.Trim(args.Text("explanation"), "\n")
	if utf8.RuneCountInString(explanation) > maxExplanation {
		return refusedCall("the explanation on one action carries at most " +
			strconv.Itoa(maxExplanation) + " characters; say the rest in your answer")
	}

	// The subject. Every act names one goal: the eight that take it in the
	// route's path, and the open whose route body carries it as `id`.
	subject := oneLine(args.Text("goal"))
	if subject == "" && verb == ProposeOpen {
		subject = oneLine(args.Text(FieldID))
	}
	if subject == "" {
		return refusedCall("this tool needs the goal the action is about, by its ledger id")
	}
	if verb == ProposeOpen {
		if given := oneLine(args.Text(FieldID)); given != "" && given != subject {
			return refusedCall("an open names one new goal: goal is " + subject + " and id is " + given)
		}
	} else if args.Given(FieldID) {
		return refusedCall("id is the new goal's own field on " + ProposeOpen +
			"; every other act names the goal it is about with goal")
	}

	for _, refused := range refusedProposalFields {
		if args.Given(refused.field) {
			return refusedCall(refused.field + " is not a field a proposal carries: " + refused.words)
		}
	}

	fields := map[string]string{}
	if verb == ProposeOpen {
		fields[FieldID] = subject
	}
	admitted := map[string]bool{FieldID: verb == ProposeOpen}
	for _, field := range act.Fields() {
		admitted[field] = true
	}
	// A field of another act is refused by naming the act that takes it, so a
	// Partner that sent a park's reason to an unapprove is told which word each
	// of them wants rather than being told that one of them is wrong.
	for _, field := range everyProposalField() {
		if admitted[field] || !args.Given(field) {
			continue
		}
		return refusedCall(field + " is not a field " + verb + " takes; " + takenBy(field) +
			" — " + verb + " takes " + listed(act.Fields()))
	}

	// A field the act cannot do without is asked for before any value is
	// judged, so an open with nothing in it is told which field to start with
	// rather than told that its first unanswered number is out of range.
	for _, needed := range act.Needs {
		if !args.Given(needed) {
			return refusedCall(verb + " needs " + needed + ": " + needs(verb, needed))
		}
	}
	// And every other key the caller sent, whatever it is. The two checks above
	// name a field this act does not take and a field no act takes; this one is
	// what catches a MISSPELLING, which is the way a field is lost in silence: a
	// Partner that sent `blocked_by` where the body says `blockedBy` was told it
	// had prepared the action, and the card carried no dependency at all.
	for _, key := range sortedKeys(args) {
		if commonProposalField(key) || admitted[key] || !args.Given(key) {
			continue
		}
		return refusedCall(quotedKey(key) + " is not a field this tool takes; " + verb +
			" takes " + listed(act.Fields()) + ", beside verb, goal and explanation")
	}

	for _, field := range act.Fields() {
		value, refusal := valueOf(args, field)
		if refusal != "" {
			return refusedCall(refusal)
		}
		if value == "" && !args.Given(field) {
			continue
		}
		fields[field] = value
	}
	for _, needed := range act.Needs {
		if strings.TrimSpace(fields[needed]) == "" {
			return refusedCall(verb + " needs " + needed + ": " + needs(verb, needed))
		}
	}
	if len(act.OneOf) > 0 && !anyGiven(fields, act.OneOf) {
		return refusedCall(verb + " changes at least one of " + listed(act.OneOf) +
			"; a proposal that changes none of them would publish nothing")
	}
	return Result{Prepared: PreparedProposalLine + "\n" + framed(act, subject, fields) + explanation + "\n"}
}

// framed is the action in the fixed form, the frame's own order.
//
// A field the caller SUPPLIED is written even when its value is empty, and that
// is the one rule here worth stating. A field nobody sent and a field sent empty
// are two different statements, and one act depends on the difference: an edit
// whose label list arrives empty CLEARS the labels, while an edit that says
// nothing about them leaves them alone. A frame that dropped the empty one would
// turn "clear the labels" into "change nothing", which the route then refuses as
// an edit that changes nothing at all. So the map's own keys decide what is
// written, and the map holds a field the caller gave whatever it gave.
func framed(act ProposedAct, subject string, fields map[string]string) string {
	var built strings.Builder
	built.WriteString(ProposalHeader + act.Route + "\n")
	// The open's subject is its route body's own `id`, and the other eight
	// carry the goal in the route's path. Each act's frame is exactly its
	// body's fields, so neither says one fact twice.
	if act.Route != ProposeOpen {
		built.WriteString(ProposalGoal + subject + "\n")
	}
	for _, line := range ProposalFrame {
		if said, given := fields[line.Field]; given {
			built.WriteString(line.Label + said + "\n")
		}
	}
	built.WriteString(ProposalSeparator + "\n")
	return built.String()
}

// valueOf is one field as the frame carries it, or the refusal its own bound
// gives. Every field of every act is read here, so the bounds are in one place
// and a field cannot be admitted by one act under one rule and by another
// under a different one.
func valueOf(args Args, field string) (string, string) {
	switch field {
	case FieldIntent, FieldNextStep:
		said := args.Text(field)
		// The ledger keeps a goal's intent and next step on one line each, and
		// the routes refuse a body that carries a break rather than writing a
		// broken record. It is refused here so the card never exists for it.
		if strings.ContainsAny(said, "\r\n") {
			return "", "a goal's " + field + " is one line in the ledger; fold the line breaks before proposing it"
		}
		said = strings.TrimSpace(said)
		if utf8.RuneCountInString(said) > maxProposalLine {
			return "", "the " + field + " carries at most " + strconv.Itoa(maxProposalLine) + " characters"
		}
		return said, ""
	case FieldReason, FieldBecause, FieldBasis:
		said := oneLine(args.Text(field))
		if utf8.RuneCountInString(said) > maxProposalClause {
			return "", "the " + field + " carries at most " + strconv.Itoa(maxProposalClause) +
				" characters; say the rest in your answer"
		}
		return said, ""
	case FieldSeverity, FieldNovelty, FieldExposure, FieldAccumulation:
		at := args.Number(field, 0)
		if at < 1 || at > 3 {
			return "", "the risk answer " + field + " is 1, 2 or 3, and a proposal answers all four"
		}
		return strconv.Itoa(at), ""
	case FieldPriority:
		at := args.Number(field, 0)
		if at < 1 || at > 3 {
			return "", "a priority is 1, 2 or 3"
		}
		return strconv.Itoa(at), ""
	case FieldSequence:
		if !args.Given(field) {
			return "", ""
		}
		at := args.Number(field, 0)
		if at < 1 {
			return "", "a sequence is a one-based position within the priority band"
		}
		return strconv.Itoa(at), ""
	case FieldBlocker:
		return oneLine(args.Text(field)), ""
	case FieldLabels, FieldBlockedBy, FieldBlocks:
		named := args.List(field)
		if len(named) > maxProposalList {
			return "", field + " carries at most " + strconv.Itoa(maxProposalList) +
				" names on one action, because the card is read before it is pressed"
		}
		return strings.Join(named, ", "), ""
	default:
		return oneLine(args.Text(field)), ""
	}
}

// needs is why a field is required, in the words the route or the engine gives.
func needs(verb, field string) string {
	switch field {
	case FieldIntent:
		return "a goal's intent says what done looks like, in one line"
	case FieldNextStep:
		return "a goal's next step states intent, constraints and freedoms, never a script of the how"
	case FieldBasis:
		return "the risk answers carry the basis they were judged on"
	case FieldReason:
		return "taking an approval back is recorded with the reason the human gave"
	case FieldBecause:
		return "a pause without a why is a stall in disguise"
	case FieldBlocker:
		return "an edge names the goal that waits and the goal it waits for"
	case FieldPriority:
		return "a priority band is 1, 2 or 3"
	default:
		return verb + " cannot be published without it"
	}
}

// unknownVerb is what a call naming something else is refused with: the nine
// this interface has, and, for a goal act it genuinely does not have, the fact
// that it exists somewhere else.
func unknownVerb(verb string) string {
	if verb == "" {
		return "this tool needs the act to propose, one of " + listed(ProposeRoutes())
	}
	bare := strings.TrimSuffix(strings.TrimSuffix(verb, "-goal"), "goal-")
	for _, elsewhere := range actsNotInTheInterface {
		if bare == elsewhere {
			return elsewhere + " is not an act this interface has, so there is nothing here to propose; " +
				"say in words that it is done at a terminal. This interface's acts are " + listed(ProposeRoutes())
		}
	}
	return `"` + verb + `" is not an act this interface has; it has ` + listed(ProposeRoutes())
}

// takenBy names the acts one field belongs to, so a misplaced field teaches
// where it goes.
func takenBy(field string) string {
	owners := []string{}
	for _, act := range ProposedActs {
		for _, taken := range act.Fields() {
			if taken == field {
				owners = append(owners, act.Route)
				break
			}
		}
	}
	if len(owners) == 0 {
		return field + " is no act's field"
	}
	return field + " is " + listed(owners) + "'s"
}

// everyProposalField is every field any act takes, so a field belonging to one
// act and sent to another is found however the catalogue grows.
func everyProposalField() []string {
	seen := map[string]bool{}
	all := []string{}
	for _, act := range ProposedActs {
		for _, field := range act.Fields() {
			if seen[field] {
				continue
			}
			seen[field] = true
			all = append(all, field)
		}
	}
	return all
}

// commonProposalField is one of the three every act takes: which act it is, the
// goal it is about, and the Partner's own words for the human.
func commonProposalField(key string) bool {
	return key == "verb" || key == "goal" || key == "explanation"
}

// sortedKeys is the caller's own keys in a fixed order, so a call with two
// unknown fields is refused with the same one twice rather than with whichever
// the map happened to yield first.
func sortedKeys(args Args) []string {
	named := make([]string, 0, len(args))
	for key := range args {
		named = append(named, key)
	}
	sort.Strings(named)
	return named
}

func quotedKey(key string) string {
	return `"` + oneLine(key) + `"`
}

func anyGiven(fields map[string]string, of []string) bool {
	for _, field := range of {
		if _, given := fields[field]; given {
			return true
		}
	}
	return false
}

// listed is a list a human reads: commas, and "and" before the last.
func listed(names []string) string {
	switch len(names) {
	case 0:
		return "nothing"
	case 1:
		return names[0]
	default:
		return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
	}
}

// proposeDescription is the tool's own description: the nine acts with the
// word each page's button uses, and the rule that words alone propose nothing.
//
// It is composed from the catalogue rather than written beside it, so a verb
// added to the table is a verb the model is told about.
func proposeDescription() string {
	var built strings.Builder
	built.WriteString("Propose one act on one goal for the human to apply. Call it once per action, " +
		"beside the answer you give in words, when the human asks you to do something this interface can do " +
		"to a goal. It writes nothing and applies nothing: the human sees the action on a card under your " +
		"answer, ticks the ones they want and presses Apply, and their press is the act. " +
		WordsAloneProposeNothing +
		" Any combination of acts may be proposed together, and a goal you propose to open may be named by a " +
		"later action of the same answer. The nine acts, by the word the interface's own button uses:")
	for _, act := range ProposedActs {
		built.WriteString("\n- " + act.Route + " (" + act.Button + ")")
		if fields := act.Fields(); len(fields) > 0 {
			built.WriteString(": " + listed(fields))
			if len(act.OneOf) > 0 {
				built.WriteString(" — at least one of " + listed(act.OneOf))
			}
		} else {
			built.WriteString(": the goal alone")
		}
	}
	return built.String()
}

// proposeSchema is what the tool takes: the act, the goal, the explanation and
// every field any act carries, each described once.
func proposeSchema() map[string]any {
	properties := map[string]any{
		"verb": map[string]any{
			"type": "string", "enum": ProposeRoutes(),
			"description": "Which of the nine acts this is, by its route id.",
		},
		"goal": map[string]any{
			"type":        "string",
			"description": "The goal the act is about, by its ledger id. On " + ProposeOpen + " it is the new goal's id.",
		},
		"explanation": map[string]any{
			"type": "string",
			"description": "Why you are proposing this, in your own words, for the human to read beside the action. " +
				"At most " + strconv.Itoa(maxExplanation) + " characters.",
		},
		FieldIntent: map[string]any{"type": "string",
			"description": "On " + ProposeOpen + " and " + ProposeEdit + ": what done looks like, in one line."},
		FieldNextStep: map[string]any{"type": "string",
			"description": "On " + ProposeOpen + " and " + ProposeEdit + ": intent, constraints and freedoms — never a script of the how, in one line."},
		FieldSeverity: riskProperty("How severe the harm would be if the change is wrong"),
		FieldNovelty:  riskProperty("How unfamiliar the approach is"),
		FieldExposure: riskProperty("How many users or systems it can affect"),
		FieldAccumulation: riskProperty(
			"How much change has accumulated since the last broad examination of the area"),
		FieldBasis: map[string]any{"type": "string",
			"description": "On " + ProposeOpen + ": the line the four risk answers were judged on."},
		FieldLabels: namesProperty("On " + ProposeOpen + ": the goal's labels. On " + ProposeEdit +
			": the whole label list as it should stand afterwards, which replaces the labels the goal has."),
		FieldBlockedBy: namesProperty("On " + ProposeOpen + ": the goals this one waits for."),
		FieldBlocks:    namesProperty("On " + ProposeOpen + ": the goals that will wait for this one."),
		FieldReason: map[string]any{"type": "string",
			"description": "On " + ProposeWithdraw + ": the reason the approval is taken back."},
		FieldBecause: map[string]any{"type": "string",
			"description": "On " + ProposePark + ": the reason the goal is paused."},
		FieldPriority: map[string]any{"type": "integer", "enum": []int{1, 2, 3},
			"description": "On " + ProposePriority + ": the band the goal goes in."},
		FieldSequence: map[string]any{"type": "integer",
			"description": "On " + ProposePriority + ": the one-based position in that band. Omit it to append."},
		FieldBlocker: map[string]any{"type": "string",
			"description": "On " + ProposeBlock + " and " + ProposeUnblock + ": the goal the one named in goal waits for."},
	}
	asked := schema(properties, []string{"verb", "goal", "explanation"})
	// Closed, unlike every other tool's. This is the one tool whose arguments
	// become an act a human presses, and a field the schema quietly tolerated is
	// a field the card would silently omit; a client that validates is told
	// before the call, and one that does not is told by the refusal above.
	asked["additionalProperties"] = false
	return asked
}

func riskProperty(what string) map[string]any {
	return map[string]any{"type": "integer", "enum": []int{1, 2, 3},
		"description": "On " + ProposeOpen + ": " + what + ". 1, 2 or 3, and all four are answered."}
}

func namesProperty(what string) map[string]any {
	return map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": what}
}
