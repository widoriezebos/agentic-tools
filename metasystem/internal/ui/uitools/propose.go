package uitools

import (
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
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
// allowed" will guess again, and a Partner told "pause needs reason" will not.
//
// The vocabulary is THE PUBLIC GRAMMAR'S. An act here is one of the goal
// object's public actions, `metasystem goal ACTION`, and a field is one of that
// action's public flags under the descriptor's own spelling, so the Partner, a
// human at a terminal and the interface's own help say one word for one act
// (g1-s62 D1). What the tool then maps it to — the route id and the route
// body's own fields — is the interface's boundary and not a second vocabulary:
// the message persists the route id, so the public name can be renamed without
// touching a proposal anybody has stored.
//
// Ten of the goal object's actions are acts this interface has. The rest are
// refused with that action's own usage line from the engine's catalogue, so a
// Partner asked for one can say where it is done instead of offering a card the
// interface could never have had.

// The ten acts one proposal can be: the goal object's public actions, exactly
// as `metasystem goal ACTION --help` names them.
const (
	ActionOpen       = "open"
	ActionApprove    = "approve"
	ActionUnapprove  = "unapprove"
	ActionPrioritize = "prioritize"
	ActionBlock      = "block"
	ActionUnblock    = "unblock"
	ActionPause      = "pause"
	ActionResume     = "resume"
	ActionEdit       = "edit"
	ActionAbandon    = "abandon"
)

// The routes those ten dispatch to: the interface's own ids, exactly as the
// act table names them and the runner sends them.
//
// They are what the message persists, and that is why they are here beside the
// public names rather than instead of them: a public action renamed by the
// verbs seat is one line of this table, and a proposal a human left waiting
// yesterday still dispatches.
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
	ProposeAbandon  = "abandon-goal"
)

// The public flags, under the descriptors' own names and spellings. They are
// what the tool takes.
const (
	FlagIntent    = "intent"
	FlagNext      = "next"
	FlagRisk      = "risk"
	FlagBasis     = "basis"
	FlagLabel     = "label"
	FlagUnlabel   = "unlabel"
	FlagBlockedBy = "blocked-by"
	FlagBlocks    = "blocks"
	FlagReason    = "reason"
	FlagOn        = "on"
	FlagPriority  = "priority"
	FlagSequence  = "sequence"
	FlagSuccessor = "successor"
)

// The route bodies' own fields, under the bodies' own spellings. They are what
// the tool maps the public flags to and what the frame carries.
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
	// FieldSuccessor is the live goal carrying an abandoned goal's work. It is
	// the public flag's own name; the route body spells it the same way, and
	// the engine calls it Carried.
	FieldSuccessor = "successor"
	// FieldLabel and FieldUnlabel are the one pair that travels unresolved.
	// An edit's route body takes the WHOLE label list, and the whole list can
	// only be composed where the goal's own labels are read, which is at
	// admission and not in this process; so the delta travels under the public
	// flags' own spellings and `goal.ApplyLabelDelta` composes it there
	// (g1-s62 D1).
	FieldLabel   = "label"
	FieldUnlabel = "unlabel"
)

// ProposedAct is one row of the catalogue: the goal object's public action, the
// word one act is called by everywhere a human reads it, the route a proposal
// dispatches to, and the public flags that action's proposals may carry.
//
// Word is one word for one act. It is the action's own public name, so the
// card, the inbox row, the goal row's chip and the page's own button say the
// same thing a human at a terminal types (g1-s62 D3).
type ProposedAct struct {
	Action string
	Route  string
	Word   string
	// Needs are the flags the act cannot be published without.
	Needs []string
	// OneOf, where set, are flags of which at least one must be given. It is
	// the edit route's own rule: a body with all of them absent changes nothing
	// and the route refuses it rather than publishing a no-op.
	OneOf []string
	// Takes are the rest, each admitted and none required.
	Takes []string
}

// Fields is every public flag this act admits, needed and taken together.
func (a ProposedAct) Fields() []string {
	all := make([]string, 0, len(a.Needs)+len(a.Takes)+len(a.OneOf))
	all = append(all, a.Needs...)
	all = append(all, a.OneOf...)
	return append(all, a.Takes...)
}

// Travels is the route body's own fields this act's arguments travel as: what
// the tool maps its public flags to and what the frame writes.
func (a ProposedAct) Travels() []string {
	named := []string{}
	for _, flag := range a.Fields() {
		named = append(named, bodyFields(a.Action, flag)...)
	}
	return named
}

// Body is what the route finally decodes: Travels, with an edit's label delta
// collapsed into the whole list admission composes from it.
func (a ProposedAct) Body() []string {
	named := []string{}
	for _, field := range a.Travels() {
		if field == FieldLabel || field == FieldUnlabel {
			field = FieldLabels
		}
		if !contained(named, field) {
			named = append(named, field)
		}
	}
	return named
}

// bodyFields is the route body field, or fields, one of an act's public flags
// becomes. Only `risk` becomes more than one: the command carries the four
// answers in one flag and the route body carries them one each.
func bodyFields(action, flag string) []string {
	switch flag {
	case FlagRisk:
		return []string{FieldSeverity, FieldNovelty, FieldExposure, FieldAccumulation}
	case FlagNext:
		return []string{FieldNextStep}
	case FlagOn:
		return []string{FieldBlocker}
	case FlagBlockedBy:
		return []string{FieldBlockedBy}
	case FlagReason:
		// One flag, three bodies. `metasystem goal pause --reason`,
		// `metasystem goal unapprove --reason` and `metasystem goal abandon
		// --reason` are one public word; the park route and the abandon route
		// each spell their own field `because`.
		if action == ActionPause || action == ActionAbandon {
			return []string{FieldBecause}
		}
		return []string{FieldReason}
	case FlagLabel:
		// An open's labels ARE the whole list: a goal being opened has none to
		// compose against. An edit's are a delta (FieldLabel above).
		if action == ActionOpen {
			return []string{FieldLabels}
		}
		return []string{FieldLabel}
	case FlagUnlabel:
		return []string{FieldUnlabel}
	default:
		// intent, basis, blocks, priority and sequence are spelt the same on
		// both sides.
		return []string{flag}
	}
}

// ProposedActs is the whole catalogue, in the order the act table names the
// routes.
//
// It is one table, here, and two join tests hold it from both sides: one in
// cmd/metasystem, which has the descriptor table, asserts every action is a
// current public goal action and every field one of its non-hidden flags
// (g1-s62 D2); one in internal/ui/httpd, which has the routes, asserts every
// route is a ledger act on a goal and every body field that route's own. So
// neither the public grammar nor the interface can grow an act the other has
// not heard of.
var ProposedActs = []ProposedAct{
	{
		Action: ActionOpen, Route: ProposeOpen, Word: "Open",
		Needs: []string{FlagIntent, FlagNext, FlagRisk, FlagBasis},
		Takes: []string{FlagLabel, FlagBlockedBy, FlagBlocks},
	},
	{Action: ActionApprove, Route: ProposeApprove, Word: "Approve"},
	{Action: ActionUnapprove, Route: ProposeWithdraw, Word: "Unapprove", Needs: []string{FlagReason}},
	{
		Action: ActionPrioritize, Route: ProposePriority, Word: "Prioritize",
		Needs: []string{FlagPriority}, Takes: []string{FlagSequence},
	},
	{Action: ActionBlock, Route: ProposeBlock, Word: "Block", Needs: []string{FlagOn}},
	{Action: ActionUnblock, Route: ProposeUnblock, Word: "Unblock", Needs: []string{FlagOn}},
	{Action: ActionPause, Route: ProposePark, Word: "Pause", Needs: []string{FlagReason}},
	{Action: ActionResume, Route: ProposeUnpark, Word: "Resume"},
	{
		Action: ActionEdit, Route: ProposeEdit, Word: "Edit",
		OneOf: []string{FlagIntent, FlagNext, FlagLabel, FlagUnlabel},
	},
	// Abandon, under R-128-ui (g1-s64): the act that says a goal will never be
	// worked. Its reason reaches the route body as `because`, which is the word
	// that body already has for a reason. Neither --waive nor --also is here:
	// they are refused by name below, with the terminal form, because releasing
	// or abandoning somebody else's dependent is a judgement made at a terminal.
	{
		Action: ActionAbandon, Route: ProposeAbandon, Word: "Abandon",
		Needs: []string{FlagReason}, Takes: []string{FlagSuccessor},
	},
}

// ProposedActionOf is one row of the catalogue by its public action name.
func ProposedActionOf(action string) (ProposedAct, bool) {
	for _, act := range ProposedActs {
		if act.Action == action {
			return act, true
		}
	}
	return ProposedAct{}, false
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

// ProposeActions is the ten public action names, in the catalogue's order.
func ProposeActions() []string {
	named := make([]string, 0, len(ProposedActs))
	for _, act := range ProposedActs {
		named = append(named, act.Action)
	}
	return named
}

// ProposeRoutes is the ten route ids, in the catalogue's order.
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
	// maxProposalClause is how much a reason or a basis carries.
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
// header names the route; the labelled lines after it carry the route body's
// own fields under the body's own names; the separator ends the framing; and
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
	ProposalSuccessor    = "Successor: "
	ProposalIntent       = "Intent: "
	ProposalNextStep     = "Next step: "
	ProposalLabels       = "Labels: "
	ProposalLabel        = "Label: "
	ProposalUnlabel      = "Unlabel: "
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
	{ProposalSuccessor, FieldSuccessor},
	{ProposalIntent, FieldIntent},
	{ProposalNextStep, FieldNextStep},
	{ProposalLabels, FieldLabels},
	{ProposalLabel, FieldLabel},
	{ProposalUnlabel, FieldUnlabel},
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

// The public flags a proposal never carries, each refused by its own name with
// where it can be done instead.
//
// They are named and not classed. A Partner refused "that flag is not allowed"
// tries another spelling of it; a Partner refused "a proposal names no tier:
// the card derives it from the risk answers" has learned the rule. Three
// families are here: the authority and plumbing a browser act never carries,
// the two fields the card itself composes, and the forms the interface's routes
// cannot carry at all — the last scoped to the act whose command offers them,
// so a flag that IS proposable elsewhere is refused where it is not and
// admitted where it is.
var refusedProposalFields = []struct {
	field string
	// on, where set, are the acts this refusal is about. A flag sent to any
	// other act meets the ordinary "not a field this act takes", which names
	// the act that does take it.
	on    []string
	words string
}{
	{field: "by", words: "a proposal never names the hand that acts: the act publishes under the human's own session"},
	{field: "id", words: "id is the flag that names a goal instead of naming it first; a proposal names its goal in goal"},
	{field: "origin", words: "a goal this interface opens is of origin human, and nothing else may be asked for"},
	{field: "lineage", words: "a lineage is the seat's plumbing and no part of an act a human presses"},
	{field: "temporary-human-word", words: "a temporary human word is a terminal's authority and is never proposed"},
	{field: "review-by", words: "a review date is the authority's own field and is set where the authority is given"},
	{field: "approved-ref", words: "an approved ref is the engine's plumbing and is never proposed"},
	{field: "fixture-human-authority", words: "a fixture authority belongs to this kit's own tests"},
	{field: "under", words: "acting under a recorded power of attorney is not something this interface offers"},
	{field: "verified", words: "what a seat verified is evidence and is recorded where the evidence is"},
	{field: "tier", words: "a proposal names no tier: the card derives it from the four risk answers and sends what it derived"},
	{field: "why", words: "the words you want the human to read go in explanation; why is the reason flag's other name"},
	{field: "budget", words: "an approval carries the budget the card displays and no other; propose the approval and the card shows the tuple"},
	{field: "evidence", words: "evidence is recorded where the work landed, not on an edit a human presses"},
	{field: "next-append", words: "this interface's edit replaces the next step; send the whole next"},
	{field: "waive", words: "a live dependent is released at a terminal: metasystem goal abandon G " +
		"--reason TEXT --waive DEPENDENT=REASON; from here, name the successor that carries the work, " +
		"or say in words that the dependents are waived at a terminal"},
	{field: "also", words: "a dependent is abandoned in the same act at a terminal: metasystem goal " +
		"abandon G --reason TEXT --also DEPENDENT; from here, propose one abandon per goal"},
	// The two the edit command carries and the edit route cannot: a goal's risk
	// is changed at a terminal, where the evidence for raising it is given.
	{field: FlagRisk, on: []string{ActionEdit},
		words: "this interface's edit changes the intent, the next step and the labels; a goal's risk is changed at a terminal"},
	{field: FlagBasis, on: []string{ActionEdit},
		words: "a basis is recorded with the risk answers it was judged on, and this interface's edit changes neither"},
	// Every -file form, named rather than classed: the text itself travels.
	{field: "intent-file", words: "a proposal carries the text itself; the -file forms read a file at a terminal"},
	{field: "next-file", words: "a proposal carries the text itself; the -file forms read a file at a terminal"},
	{field: "next-append-file", words: "a proposal carries the text itself; the -file forms read a file at a terminal"},
	{field: "basis-file", words: "a proposal carries the text itself; the -file forms read a file at a terminal"},
	{field: "reason-file", words: "a proposal carries the text itself; the -file forms read a file at a terminal"},
	{field: "verified-file", words: "a proposal carries the text itself; the -file forms read a file at a terminal"},
}

// RefusedProposalFlags is every public flag a proposal is refused for, by name.
//
// It is exported so the join test in cmd/metasystem can hold it against the
// descriptors from the other side: a name here that no proposable action's
// command carries is a refusal no Partner will ever meet, and a flag the verbs
// seat renames is a refusal that would quietly stop firing.
func RefusedProposalFlags() []string {
	named := make([]string, 0, len(refusedProposalFields))
	for _, refused := range refusedProposalFields {
		if !contained(named, refused.field) {
			named = append(named, refused.field)
		}
	}
	return named
}

// The goal object's other public actions: the ones this interface has no act
// for, refused by name with that action's own usage line.
//
// They are the actions a human asks for that the browser genuinely cannot make
// — a goal abandoned, a goal done, a claim released — and a Partner that met a
// bare "unknown action" for one of them would offer the human a card the
// interface could never have. Named, with the terminal's own form beside it, it
// says the act exists and where it is made, so the Partner can say where.
//
// The usage line is read from the engine's catalogue rather than written here,
// so a public form the verbs seat changes is a form this refusal follows by
// itself.
var actsNotInTheInterface = []string{
	"done", "reopen", "budget", "claim", "release", "accept-risk", "pin",
	"split", "group", "ungroup", "notes", "sync",
}

// ActsNotInTheInterface is those actions, for the join test that holds them
// against the goal object's own table: each must be a public action with a
// usage line, and every public action must be one of the ten, one of these, or
// a read.
func ActsNotInTheInterface() []string {
	return append([]string{}, actsNotInTheInterface...)
}

// propose prepares one action, or refuses the call in words.
//
// The order of the refusals is the order a reader would ask the questions in:
// which act is this, does this interface have it, which goal is it about, is
// every flag one that act takes, is every flag it needs given, and is every
// value within the bounds the sheets hold.
func (r Readers) propose(args Args) Result {
	action := strings.ToLower(oneLine(args.Text("verb")))
	act, known := ProposedActionOf(action)
	if !known {
		return refusedCall(r.unknownAction(action))
	}
	explanation := strings.Trim(args.Text("explanation"), "\n")
	if utf8.RuneCountInString(explanation) > maxExplanation {
		return refusedCall("the explanation on one action carries at most " +
			strconv.Itoa(maxExplanation) + " characters; say the rest in your answer")
	}

	// The subject. Every public form names the goal first, so the tool takes it
	// under one name whatever the route body then calls it.
	subject := oneLine(args.Text("goal"))
	if subject == "" {
		return refusedCall("this tool needs the goal the action is about, by its ledger id")
	}
	if refusal := boundedID("goal", subject); refusal != "" {
		return refusedCall(refusal)
	}

	for _, refused := range refusedProposalFields {
		if args.Given(refused.field) && scoped(refused.on, action) {
			return refusedCall(refused.field + " is not a flag a proposal carries: " + refused.words)
		}
	}

	admitted := map[string]bool{}
	for _, flag := range act.Fields() {
		admitted[flag] = true
	}
	// A flag of another act is refused by naming the act that takes it, so a
	// Partner that sent a priority to a pause is told which word each of them
	// wants rather than being told that one of them is wrong.
	for _, flag := range everyProposalFlag() {
		if admitted[flag] || !args.Given(flag) {
			continue
		}
		return refusedCall(flag + " is not a flag " + action + " takes; " + takenBy(flag) +
			" — " + action + " takes " + listed(act.Fields()))
	}

	// A flag the act cannot do without is asked for before any value is judged,
	// so an open with nothing in it is told which flag to start with rather
	// than told that its risk answers do not parse.
	for _, needed := range act.Needs {
		if !args.Given(needed) {
			return refusedCall(action + " needs " + needed + ": " + needs(action, needed))
		}
	}
	// And every other key the caller sent, whatever it is. The two checks above
	// name a flag this act does not take and a flag no act takes; this one is
	// what catches a MISSPELLING, which is the way a field is lost in silence: a
	// Partner that sent `blocked_by` where the descriptor says `blocked-by` was
	// told it had prepared the action, and the card carried no dependency at all.
	for _, key := range sortedKeys(args) {
		if commonProposalField(key) || admitted[key] || !args.Given(key) {
			continue
		}
		return refusedCall(quotedKey(key) + " is not a flag this tool takes; " + action +
			" takes " + listed(act.Fields()) + ", beside verb, goal and explanation")
	}

	// The body, under the route body's own field names: this is where the
	// public grammar becomes the interface's own.
	body := map[string]string{}
	if act.Action == ActionOpen {
		body[FieldID] = subject
	}
	for _, flag := range act.Fields() {
		said, refusal := valueOf(args, act, flag)
		if refusal != "" {
			return refusedCall(refusal)
		}
		for field, value := range said {
			body[field] = value
		}
	}
	for _, needed := range act.Needs {
		for _, field := range bodyFields(action, needed) {
			if strings.TrimSpace(body[field]) == "" {
				return refusedCall(action + " needs " + needed + ": " + needs(action, needed))
			}
		}
	}
	if len(act.OneOf) > 0 && !anyGiven(args, act.OneOf) {
		return refusedCall(action + " changes at least one of " + listed(act.OneOf) +
			"; a proposal that changes none of them would publish nothing")
	}
	return Result{Prepared: PreparedProposalLine + "\n" + framed(act, subject, body) + explanation + "\n"}
}

// scoped reports whether a refusal scoped to some acts is about this one. An
// unscoped refusal is about every act.
func scoped(on []string, action string) bool {
	if len(on) == 0 {
		return true
	}
	return contained(on, action)
}

// framed is the action in the fixed form, the frame's own order.
//
// A field the caller SUPPLIED is written even when its value is empty, and that
// is the one rule here worth stating. A field nobody sent and a field sent empty
// are two different statements, and one act depends on the difference: an edit
// whose whole label list arrives empty CLEARS the labels, while an edit that says
// nothing about them leaves them alone. A frame that dropped the empty one would
// turn "clear the labels" into "change nothing", which the route then refuses as
// an edit that changes nothing at all. So the map's own keys decide what is
// written, and the map holds a field the caller gave whatever it gave.
func framed(act ProposedAct, subject string, body map[string]string) string {
	var built strings.Builder
	built.WriteString(ProposalHeader + act.Route + "\n")
	// The open's subject is its route body's own `id`, and the other eight
	// carry the goal in the route's path. Each act's frame is exactly its
	// body's fields, so neither says one fact twice.
	if act.Route != ProposeOpen {
		built.WriteString(ProposalGoal + subject + "\n")
	}
	for _, line := range ProposalFrame {
		if said, given := body[line.Field]; given {
			built.WriteString(line.Label + said + "\n")
		}
	}
	built.WriteString(ProposalSeparator + "\n")
	return built.String()
}

// valueOf is one public flag as the frame carries it — the route body field or
// fields it becomes — or the refusal its own bound gives. A flag the caller did
// not send yields nothing.
//
// Every flag of every act is read here, so the bounds are in one place and a
// flag cannot be admitted by one act under one rule and by another under a
// different one.
func valueOf(args Args, act ProposedAct, flag string) (map[string]string, string) {
	if !args.Given(flag) {
		return nil, ""
	}
	into := func(value string) map[string]string {
		named := map[string]string{}
		for _, field := range bodyFields(act.Action, flag) {
			named[field] = value
		}
		return named
	}
	switch flag {
	case FlagIntent, FlagNext:
		said := args.Text(flag)
		// The ledger keeps a goal's intent and next step on one line each, and
		// the routes refuse a body that carries a break rather than writing a
		// broken record. It is refused here so the card never exists for it.
		if strings.ContainsAny(said, "\r\n") {
			return nil, "a goal's " + flag + " is one line in the ledger; fold the line breaks before proposing it"
		}
		said = strings.TrimSpace(said)
		if utf8.RuneCountInString(said) > maxProposalLine {
			return nil, "the " + flag + " carries at most " + strconv.Itoa(maxProposalLine) + " characters"
		}
		return into(said), ""
	case FlagReason, FlagBasis:
		said := oneLine(args.Text(flag))
		if utf8.RuneCountInString(said) > maxProposalClause {
			return nil, "the " + flag + " carries at most " + strconv.Itoa(maxProposalClause) +
				" characters; say the rest in your answer"
		}
		return into(said), ""
	case FlagRisk:
		// The command's own form, parsed by the command's own owner, so the
		// four answers a Partner writes are the four answers a human types and
		// one reader judges both (g1-s62 D1).
		record, err := goal.ParseRiskRecord(oneLine(args.Text(FlagRisk)), oneLine(args.Text(FlagBasis)))
		if err != nil {
			return nil, "risk: " + err.Error()
		}
		return map[string]string{
			FieldSeverity:     strconv.Itoa(int(record.Severity)),
			FieldNovelty:      strconv.Itoa(int(record.Novelty)),
			FieldExposure:     strconv.Itoa(int(record.Exposure)),
			FieldAccumulation: strconv.Itoa(int(record.Accumulation)),
		}, ""
	case FlagPriority:
		at := args.Number(flag, 0)
		if at < 1 || at > 3 {
			return nil, "a priority is 1, 2 or 3"
		}
		return into(strconv.Itoa(at)), ""
	case FlagSequence:
		at := args.Number(flag, 0)
		if at < 1 {
			return nil, "a sequence is a one-based position within the priority band"
		}
		return into(strconv.Itoa(at)), ""
	case FlagOn, FlagSuccessor:
		said := oneLine(args.Text(flag))
		if refusal := boundedID(flag, said); refusal != "" {
			return nil, refusal
		}
		return into(said), ""
	case FlagLabel, FlagUnlabel, FlagBlockedBy, FlagBlocks:
		named, refusal := args.List(flag)
		if refusal != "" {
			return nil, refusal
		}
		if refusal := boundedNames(flag, named, flag == FlagLabel || flag == FlagUnlabel); refusal != "" {
			return nil, refusal
		}
		return into(strings.Join(named, ", ")), ""
	default:
		return into(oneLine(args.Text(flag))), ""
	}
}

// boundedID is the refusal one goal id past the ledger's own bound is named
// with, or "" where it is within it.
//
// Every id this tool frames is one of the ledger's: the subject, the goal at the
// other end of an edge, an abandon's successor, and every name of an open's two
// lists. None of them was bounded at all, and an id is not free to be long —
// what this tool frames is written into the human's transcript whether the act is
// ever admitted or not, so one goal of three hundred thousand characters made
// that transcript unreadable by its own reader (Astra B-01).
func boundedID(field, id string) string {
	if len(id) <= goal.MaxIdBytes {
		return ""
	}
	return field + ": a goal id is at most " + strconv.Itoa(goal.MaxIdBytes) + " bytes in the ledger"
}

// boundedNames is the refusal one list field is named with: how many names it
// carries, and then each name as the token it is.
//
// The labels are judged by the ledger's own grammar through the owner that holds
// every other command boundary to it, exactly as the risk answers are parsed by
// the command's own reader: the number is the engine's and is never spelt twice.
func boundedNames(field string, named []string, labels bool) string {
	if len(named) > maxProposalList {
		return field + " carries at most " + strconv.Itoa(maxProposalList) +
			" names on one action, because the card is read before it is pressed"
	}
	if labels {
		if err := goal.ValidateLabels(named); err != nil {
			return field + ": " + err.Error()
		}
		return ""
	}
	for _, one := range named {
		if refusal := boundedID(field, one); refusal != "" {
			return refusal
		}
	}
	return ""
}

// BeyondTheProposalBounds is why a prepared FRAME is past the bounds a proposal
// is held to, naming the field and the bound, or "" where every field of it is
// within them.
//
// It is the other side of the same numbers the call is refused by, and it is not
// the same check: a frame reaches the interface as text a runtime reported, so
// nothing about it is this server's word. The interface asks this before it keeps
// any of a frame, because a frame is kept whether its act is admitted or refused,
// and one field of any length in the transcript cost the whole conversation its
// next open (Astra B-01). The bounds are here, beside the call's own, so that one
// number is never two.
func BeyondTheProposalBounds(subject, explanation string, fields map[string]string) string {
	if refusal := boundedID("goal", subject); refusal != "" {
		return refusal
	}
	if utf8.RuneCountInString(explanation) > maxExplanation {
		return "the explanation on one action carries at most " +
			strconv.Itoa(maxExplanation) + " characters"
	}
	for _, line := range ProposalFrame {
		said, given := fields[line.Field]
		if !given {
			continue
		}
		if refusal := boundedFrameField(line.Field, said); refusal != "" {
			return refusal
		}
	}
	return ""
}

// MostProposalsPerAnswer is how many actions one answer of the Partner's
// carries.
//
// There is a bound at all because an answer is written whole into the human's
// own transcript, and that transcript is read back whole on the next open: an
// answer of any length is a conversation that can no longer be reloaded, which
// is the one failure that takes every page reading it down with it. So a long
// list is worked in batches, and the batch is fifty (R-130-ui).
const MostProposalsPerAnswer = 50

// BeyondTheProposalCount is what one action past the batch is refused with, or
// "" where the answer has room for it.
//
// The number and the word for it are kept in one place, here beside the bounds:
// the sentence spells fifty out, because that is what the Partner reads, and a
// bound that moved without its own sentence would tell the Partner to do
// something other than what the interface will accept.
//
// The counting itself is not here. This server is one process for a whole
// Partner session and the wire carries no signal for where one answer ends, so
// the count is asked where an answer's own proposals are held — the interface's
// running turn — and this says what the bound is and how the refusal reads.
func BeyondTheProposalCount(alreadyCarried int) string {
	if alreadyCarried < MostProposalsPerAnswer {
		return ""
	}
	return "this answer already carries fifty proposals; say how many remain and " +
		"propose them in your next answer, after the human has applied these"
}

// boundedFrameField is one framing line's own bound. The ids and the labels are
// the ledger's tokens; an intent and a next step are the sheets' one-line
// fields; and everything else a frame carries — a reason, a basis, the four risk
// answers, a priority, a sequence — is a clause or a number, which the clause
// bound covers.
func boundedFrameField(field, said string) string {
	switch field {
	case FieldID, FieldBlocker, FieldSuccessor:
		return boundedID(field, said)
	case FieldBlockedBy, FieldBlocks:
		return boundedNames(field, framedNames(said), false)
	case FieldLabels, FieldLabel, FieldUnlabel:
		return boundedNames(field, framedNames(said), true)
	case FieldIntent, FieldNextStep:
		if utf8.RuneCountInString(said) > maxProposalLine {
			return "the " + field + " carries at most " + strconv.Itoa(maxProposalLine) + " characters"
		}
	default:
		if utf8.RuneCountInString(said) > maxProposalClause {
			return "the " + field + " carries at most " + strconv.Itoa(maxProposalClause) + " characters"
		}
	}
	return ""
}

// framedNames is one framing line's list as the frame joined it and the route
// body reads it: comma-separated, and an empty name is not a name.
func framedNames(said string) []string {
	named := []string{}
	for _, one := range strings.Split(said, ",") {
		if one = strings.TrimSpace(one); one != "" {
			named = append(named, one)
		}
	}
	return named
}

// needs is why a flag is required, in the words the command or the engine gives.
func needs(action, flag string) string {
	switch flag {
	case FlagIntent:
		return "a goal's intent says what done looks like, in one line"
	case FlagNext:
		return "a goal's next step states intent, constraints and freedoms, never a script of the how"
	case FlagRisk:
		return "the four answers are the intake law's classification: " +
			"severity=N,novelty=N,exposure=N,accumulation=N, each 1, 2 or 3"
	case FlagBasis:
		return "the risk answers carry the basis they were judged on"
	case FlagReason:
		switch action {
		case ActionPause:
			return "a pause without a why is a stall in disguise"
		case ActionAbandon:
			return "a goal that will never be worked is recorded with why, and the why is read for years"
		}
		return "taking an approval back is recorded with the reason the human gave"
	case FlagOn:
		return "an edge names the goal that waits and the goal it waits for"
	case FlagPriority:
		return "a priority band is 1, 2 or 3"
	default:
		return action + " cannot be published without it"
	}
}

// unknownAction is what a call naming something else is refused with: the ten
// this interface has, and, for one of the goal object's other public actions,
// that action's own usage line from the engine's catalogue.
func (r Readers) unknownAction(action string) string {
	if action == "" {
		return "this tool needs the act to propose, one of " + listed(ProposeActions())
	}
	if contained(actsNotInTheInterface, action) {
		return action + " is not an act the interface has; at a terminal: " + r.usageOf(action) +
			". This interface's acts are " + listed(ProposeActions())
	}
	return `"` + action + `" is not an act this interface has; it has ` + listed(ProposeActions())
}

// usageOf is one goal action's public form, in the engine's own catalogue's
// words, or the bare command where this build cannot reach the catalogue.
func (r Readers) usageOf(action string) string {
	if r.Kit.Commands != nil {
		for _, family := range r.Kit.Commands() {
			for _, command := range family.Verbs {
				if command.Name == "goal "+action && len(command.Usage) > 0 {
					return command.Usage[0]
				}
			}
		}
	}
	return "metasystem goal " + action
}

// takenBy names the acts one public flag belongs to, so a misplaced flag
// teaches where it goes.
func takenBy(flag string) string {
	owners := []string{}
	for _, act := range ProposedActs {
		if contained(act.Fields(), flag) {
			owners = append(owners, act.Action)
		}
	}
	if len(owners) == 0 {
		return flag + " is no act's flag"
	}
	return flag + " is " + listed(owners) + "'s"
}

// everyProposalFlag is every public flag any act takes, so a flag belonging to
// one act and sent to another is found however the catalogue grows.
func everyProposalFlag() []string {
	all := []string{}
	for _, act := range ProposedActs {
		for _, flag := range act.Fields() {
			if !contained(all, flag) {
				all = append(all, flag)
			}
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

func anyGiven(args Args, of []string) bool {
	for _, flag := range of {
		if args.Given(flag) {
			return true
		}
	}
	return false
}

func contained(named []string, wanted string) bool {
	for _, one := range named {
		if one == wanted {
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

// proposeDescription is the tool's own description: the ten acts under their
// public names with the flags each takes, and the rule that words alone propose
// nothing.
//
// It is composed from the catalogue rather than written beside it, so an act
// added to the table is an act the model is told about.
func proposeDescription() string {
	var built strings.Builder
	built.WriteString("Propose one act on one goal for the human to apply. Call it once per action, " +
		"beside the answer you give in words, when the human asks you to do something this interface can do " +
		"to a goal. It writes nothing and applies nothing: the human sees the action on a card under your " +
		"answer, ticks the ones they want and presses Apply, and their press is the act. " +
		WordsAloneProposeNothing +
		" Any combination of acts may be proposed together, and a goal you propose to open may be named by a " +
		"later action of the same answer. The act and its fields are the public command's own, as " +
		"`metasystem goal ACTION --help` shows them. The ten acts this interface has:")
	for _, act := range ProposedActs {
		built.WriteString("\n- goal " + act.Action)
		if fields := act.Fields(); len(fields) > 0 {
			built.WriteString(": " + listed(fields))
			// Said once. Where an act's whole field list IS the set it must
			// change one of, "at least one of" the same list again would be a
			// sentence the model reads twice and learns nothing from.
			switch {
			case len(act.OneOf) == len(fields):
				built.WriteString(" — at least one of them")
			case len(act.OneOf) > 0:
				built.WriteString(" — at least one of " + listed(act.OneOf))
			}
		} else {
			built.WriteString(": the goal alone")
		}
	}
	return built.String()
}

// proposeSchema is what the tool takes: the act, the goal, the explanation and
// every public flag any act carries, each described once.
func proposeSchema() map[string]any {
	properties := map[string]any{
		"verb": map[string]any{
			"type": "string", "enum": ProposeActions(),
			"description": "Which of the ten acts this is, by its public name under the goal object: metasystem goal ACTION.",
		},
		"goal": map[string]any{
			"type":        "string",
			"description": "The goal the act is about, by its ledger id. On " + ActionOpen + " it is the new goal's id.",
		},
		"explanation": map[string]any{
			"type": "string",
			"description": "Why you are proposing this, in your own words, for the human to read beside the action. " +
				"At most " + strconv.Itoa(maxExplanation) + " characters.",
		},
		FlagIntent: map[string]any{"type": "string",
			"description": "On " + ActionOpen + " and " + ActionEdit + ": what done looks like, in one line."},
		FlagNext: map[string]any{"type": "string",
			"description": "On " + ActionOpen + " and " + ActionEdit +
				": intent, constraints and freedoms — never a script of the how, in one line."},
		FlagRisk: map[string]any{"type": "string",
			"description": "On " + ActionOpen + ": the four risk answers in the command's own form, " +
				"severity=N,novelty=N,exposure=N,accumulation=N, each 1, 2 or 3."},
		FlagBasis: map[string]any{"type": "string",
			"description": "On " + ActionOpen + ": the line the four risk answers were judged on."},
		FlagLabel: namesProperty("On " + ActionOpen + ": the goal's labels. On " + ActionEdit +
			": the labels to add, which are composed onto the labels the goal already carries."),
		FlagUnlabel:   namesProperty("On " + ActionEdit + ": the labels to remove."),
		FlagBlockedBy: namesProperty("On " + ActionOpen + ": the goals this one waits for."),
		FlagBlocks:    namesProperty("On " + ActionOpen + ": the goals that will wait for this one."),
		FlagReason: map[string]any{"type": "string",
			"description": "On " + ActionUnapprove + ": why the approval is taken back. On " + ActionPause +
				": why the goal is paused. On " + ActionAbandon + ": why the goal will never be worked."},
		FlagSuccessor: map[string]any{"type": "string",
			"description": "On " + ActionAbandon + ": the live goal carrying this one's work. " +
				"Omit it where nothing carries it; a goal other goals wait for is then refused by the engine " +
				"until they are waived or abandoned at a terminal."},
		FlagOn: map[string]any{"type": "string",
			"description": "On " + ActionBlock + " and " + ActionUnblock + ": the goal the one named in goal waits for."},
		FlagPriority: map[string]any{"type": "integer", "enum": []int{1, 2, 3},
			"description": "On " + ActionPrioritize + ": the band the goal goes in."},
		FlagSequence: map[string]any{"type": "integer",
			"description": "On " + ActionPrioritize + ": the one-based position in that band. Omit it to append."},
	}
	asked := schema(properties, []string{"verb", "goal", "explanation"})
	// Closed, unlike every other tool's. This is the one tool whose arguments
	// become an act a human presses, and a field the schema quietly tolerated is
	// a field the card would silently omit; a client that validates is told
	// before the call, and one that does not is told by the refusal above.
	asked["additionalProperties"] = false
	return asked
}

func namesProperty(what string) map[string]any {
	return map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": what}
}
