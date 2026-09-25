package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/report"
)

// The planning commands author, claim and steer goals: open and edit a goal,
// claim and release work, decide a risk, and the advanced acts of planning,
// authority and recovery. Each calls the existing goal owner, which keeps its
// own proof, locking and publication; the adapter only resolves the target
// and translates the public words into the owner's inputs.

func reasonFlag(owner, usage string) intentFlag {
	return intentFlag{name: "reason", aliases: []string{owner}, value: "TEXT", usage: usage}
}

func fileFlag(name, usage string) intentFlag {
	return intentFlag{name: name + "-file", value: "FILE", advanced: true, usage: usage}
}

var (
	intentHumanActFlags = []intentFlag{intentByFlag, intentLineageFlag, intentFixtureFlag}
	intentRelayFlags    = []intentFlag{intentTemporaryWordFlag, intentReviewByFlag}
	intentRiskFlags     = []intentFlag{
		{name: "risk", value: "ANSWERS", usage: "the four risk answers: severity=N,novelty=N,exposure=N,accumulation=N"},
		{name: "basis", value: "TEXT", usage: "the plain-English basis for the four answers (required with --risk)"},
		fileFlag("basis", "read the basis from FILE"),
		{name: "tier", value: "1|2|3", advanced: true, usage: "the recorded tier; a tier below the derived one is a person's act"},
	}
	intentLabelFlag = intentFlag{name: "label", value: "LABEL", repeat: true, advanced: true, usage: "a label token (repeatable)"}
	// The owner's five long budget limits, the advanced alternative to one
	// compact box. Given together they are one box; they never mix with one.
	intentLongBudgetFlags = []intentFlag{
		{name: "elapsed-limit", value: "DURATION", advanced: true, usage: "long form: the elapsed limit, for example 1d or 4h"},
		{name: "attempt-limit", value: "N", advanced: true, usage: "long form: the reservation-attempt limit"},
		{name: "reserved-job-minutes-limit", value: "N", advanced: true, usage: "long form: the reserved job-minute limit"},
		{name: "active-job-limit", value: "N", advanced: true, usage: "long form: the concurrent-job limit"},
		{name: "review-round-limit", value: "N", advanced: true, usage: "long form: the critic review-round limit"},
	}
	// set-obligation's fields, exposed on edit --obligation without a new schema.
	intentObligationFlags = []intentFlag{
		{name: "obligation", value: "STATE", advanced: true, usage: "bind a governed obligation: DRAFT, OBSERVE, LIMITED or ENFORCED"},
		{name: "owner", value: "NAME", advanced: true, usage: "obligation: the person accountable for it"},
		{name: "recurrence", value: "VALUE", advanced: true, usage: "obligation: single-experiment or standing-shared-process"},
		{name: "platform", value: "TOKEN", advanced: true, usage: "obligation: the authorized operating-system/architecture token"},
		{name: "toolchain-identity", value: "ID", advanced: true, usage: "obligation: the authorized toolchain identity"},
		{name: "surface-digest", value: "DIGEST", advanced: true, usage: "obligation: the authorized behavior-surface digest"},
		{name: "max-active-jobs", value: "N", advanced: true, usage: "obligation: the greatest active-job observation permitted"},
		{name: "timing-envelope-sec", value: "N", advanced: true, usage: "obligation: the maximum terminal duration in seconds"},
		{name: "effect", value: "EFFECT", repeat: true, advanced: true, usage: "obligation: a governing effect (repeatable)"},
		{name: "value-judgment", value: "yes|no|unknown", advanced: true, usage: "obligation: typed review assumption"},
		{name: "reversibility", value: "VALUE", advanced: true, usage: "obligation: reversible, compensable, irreversible or unknown"},
		{name: "severe-harm", value: "yes|no|unknown", advanced: true, usage: "obligation: typed review assumption"},
		{name: "unfamiliar-approach", value: "yes|no|unknown", advanced: true, usage: "obligation: typed review assumption"},
		{name: "test-discrimination", value: "strong|weak|unknown", advanced: true, usage: "obligation: typed review assumption"},
		{name: "correlated-assumption-risk", value: "yes|no|unknown", advanced: true, usage: "obligation: typed review assumption"},
		{name: "authority-scope-change", value: "yes|no|unknown", advanced: true, usage: "obligation: typed review assumption"},
		{name: "destructive-reach", value: "VALUE", advanced: true, usage: "obligation: none, reversible-local, destructive or unknown"},
	}
)

func withFlags(groups ...[]intentFlag) []intentFlag {
	var all []intentFlag
	for _, group := range groups {
		all = append(all, group...)
	}
	return all
}

func intentPlanningCommands() []intentCommand {
	return []intentCommand{
		{
			name: "open", audience: "both", summary: "declare a new goal",
			usage: []string{"metasystem open G --intent TEXT --next TEXT --risk ANSWERS --basis TEXT"},
			details: []string{
				"The goal is queued for a person's approval. The four risk answers and their basis are the intake law's classification.",
				"--blocked-by parks the goal until those goals are done; --blocks parks the named goals on it.",
				"Execution follows a person's approval: open, then metasystem approve G, then metasystem claim G.",
			},
			flags: withFlags([]intentFlag{
				intentTargetFlag,
				{name: "intent", value: "TEXT", usage: "the one-line intent"},
				{name: "next", value: "TEXT", usage: "the first next step"},
				fileFlag("intent", "read the intent from FILE"), fileFlag("next", "read the next step from FILE"),
			}, intentRiskFlags, []intentFlag{
				reasonFlag("why", "why a tier below the derived one is recorded"), fileFlag("reason", "read the reason from FILE"),
				{name: "origin", value: "human|main", advanced: true, usage: "the creation provenance (default main)"},
				{name: "blocked-by", value: "G", repeat: true, advanced: true, usage: "a goal this one waits for (repeatable)"},
				{name: "blocks", value: "G", repeat: true, advanced: true, usage: "a live goal that waits for this one (repeatable)"},
				intentLabelFlag,
			}, intentHumanActFlags, intentRelayFlags),
			maxArgs:  1,
			examples: []string{"metasystem open faster-proof --intent 'Proof runs in half the time.' --next 'Measure the slowest step.' --risk severity=1,novelty=1,exposure=1,accumulation=1 --basis 'local test tooling only'"},
			run:      runIntentOpen,
		},
		{
			name: "edit", audience: "both", summary: "change a goal's intent, next step, risk or labels in place",
			usage: []string{"metasystem edit G [--intent TEXT] [--next TEXT | --next-append TEXT]", "metasystem edit G --obligation STATE --owner NAME --recurrence VALUE ..."},
			details: []string{
				"Only the supplied fields change. --next-append adds to the next step the accepted ledger holds when the edit is published.",
				"Raising the risk of an approved goal takes --evidence; lowering it is a person's act.",
				"--obligation binds a governed obligation through the owner of set-obligation; every obligation field is required there.",
			},
			flags: withFlags([]intentFlag{
				intentTargetFlag,
				{name: "intent", value: "TEXT", usage: "the new one-line intent"},
				{name: "next", value: "TEXT", usage: "the new next step"},
				{name: "next-append", value: "TEXT", usage: "text added to the accepted next step"},
				fileFlag("intent", "read the intent from FILE"), fileFlag("next", "read the next step from FILE"), fileFlag("next-append", "read the appended text from FILE"),
			}, intentRiskFlags, []intentFlag{
				{name: "evidence", value: "REF", advanced: true, usage: "misclassification evidence when raising an approved goal's risk"},
				reasonFlag("why", "why the tier or risk changes"), fileFlag("reason", "read the reason from FILE"),
				intentLabelFlag,
				{name: "unlabel", value: "LABEL", repeat: true, advanced: true, usage: "a label token to remove (repeatable)"},
			}, intentObligationFlags, []intentFlag{intentApprovedRefFlag}, intentHumanActFlags, intentRelayFlags),
			maxArgs:  1,
			examples: []string{"metasystem edit verbs-match-intent --next 'Land slice 2.'", "metasystem edit verbs-match-intent --next-append 'Then review the help pages.'"},
			run:      runIntentEdit,
		},
		{
			name: "claim", audience: "agent", summary: "claim a goal for this session, or the next ready goal",
			usage: []string{"metasystem claim [G]", "metasystem claim G --take-over --reason TEXT"},
			details: []string{
				"Without G the machine's ready frontier chooses; a goal this machine already holds is continued, never switched.",
				"--take-over displaces another machine's claim; it is a person's act and never a fallback of an ordinary claim.",
			},
			flags: withFlags([]intentFlag{
				intentTargetFlag,
				{name: "arc", advanced: true, usage: "claim the goal's whole arc"},
				{name: "take-over", usage: "take over another machine's claim (a person's act)"},
				reasonFlag("because", "why the claim is taken over, recorded on the goal's history"), fileFlag("reason", "read the reason from FILE"),
				{name: "label", value: "LABEL", repeat: true, advanced: true, usage: "without G: only ready goals carrying every label"},
				{name: "budget", value: "BOX", advanced: true, usage: "the complete compact box for the claim"},
			}, intentLongBudgetFlags, intentHumanActFlags),
			maxArgs:  1,
			examples: []string{"metasystem claim", "metasystem claim verbs-match-intent", "metasystem claim verbs-match-intent --take-over --reason 'm1b is gone for the day'"},
			run:      runIntentClaim,
		},
		{
			name: "release", audience: "agent", summary: "release a claim this session holds",
			usage:   []string{"metasystem release [G] --reason TEXT"},
			details: []string{"Without G the one goal this session holds is released; holding none or several names them instead.", "The reason is recorded on the goal's history line."},
			flags: withFlags([]intentFlag{
				intentTargetFlag,
				{name: "arc", advanced: true, usage: "release the arc's claim"},
				reasonFlag("because", "why the claim is released, recorded on the goal's history"), fileFlag("reason", "read the reason from FILE"),
			}, intentHumanActFlags),
			maxArgs:  1,
			examples: []string{"metasystem release --reason 'the design changes first'", "metasystem release verbs-match-intent --reason 'handing it to m1b'"},
			run:      runIntentRelease,
		},
		{
			name: "ready", audience: "agent", summary: "mark the held goal built and waiting to land",
			usage:    []string{"metasystem ready [G]"},
			details:  []string{"The claim leaves the one-claim quota and its elapsed fence until it lands. It is the claim holder's own act."},
			flags:    []intentFlag{intentTargetFlag, intentLineageFlag},
			maxArgs:  1,
			examples: []string{"metasystem ready verbs-match-intent"},
			run:      runIntentReady,
		},
		{
			name: "decide", audience: "human", summary: "accept the risk of one severe or unproven review finding",
			usage: []string{"metasystem decide G --finding F --review R --reason TEXT"},
			details: []string{
				"R is any job of the review; its chain root is read from the recorded job. human-carried names a carried finding.",
				"The goal record lands first, then the accepted-risk register, the critique register and the authority proof; a later failure is reported as partial.",
			},
			flags: withFlags([]intentFlag{
				intentTargetFlag,
				{name: "finding", value: "F", usage: "the finding id"},
				{name: "review", aliases: []string{"chain"}, value: "R", usage: "the review job (or its chain root)"},
				reasonFlag("why", "why the risk is accepted"),
				fileFlag("reason", "read the reason from FILE"),
			}, intentHumanActFlags, intentRelayFlags),
			maxArgs:  1,
			examples: []string{"metasystem decide verbs-match-intent --finding S-1 --review job-7 --reason 'the exposure is local and reversible'"},
			run:      runIntentDecide,
		},
		{
			name: "pin", audience: "human", summary: "pin a goal to one machine, or clear its pin",
			usage:    []string{"metasystem pin G MACHINE", "metasystem pin G --clear"},
			flags:    withFlags([]intentFlag{intentTargetFlag, {name: "clear", usage: "remove the pin"}}, intentHumanActFlags),
			maxArgs:  2,
			examples: []string{"metasystem pin verbs-match-intent m1e", "metasystem pin verbs-match-intent --clear"},
			run:      runIntentPin,
		},
		{
			name: "prioritize", audience: "human", summary: "place an open goal in priority 1, 2 or 3",
			usage: []string{"metasystem prioritize G 1|2|3 [--sequence N]"},
			flags: withFlags([]intentFlag{
				intentTargetFlag,
				{name: "priority", value: "1|2|3", advanced: true, usage: "the priority, as an alternative to naming it after G"},
				{name: "sequence", value: "N", advanced: true, usage: "the one-based position within the priority"},
			}, intentHumanActFlags),
			maxArgs:  2,
			examples: []string{"metasystem prioritize verbs-match-intent 1", "metasystem prioritize verbs-match-intent 1 --sequence 2"},
			run:      runIntentPrioritize,
		},
		{
			name: "reopen", audience: "both", summary: "return a done or abandoned goal to the queue with a fresh next step",
			usage:    []string{"metasystem reopen G --next TEXT"},
			details:  []string{"Reopening an abandoned goal is a person's act. The fresh next step is recorded right after the reopen; a failure there is reported as partial."},
			flags:    withFlags([]intentFlag{intentTargetFlag, {name: "next", value: "TEXT", usage: "the fresh next step"}, fileFlag("next", "read the next step from FILE")}, intentHumanActFlags),
			maxArgs:  1,
			examples: []string{"metasystem reopen verbs-match-intent --next 'Cover the fleet commands.'"},
			run:      runIntentReopen,
		},
		{
			name: "abandon", audience: "human", summary: "record that a goal will never be worked, and why",
			usage: []string{"metasystem abandon G --reason TEXT [--successor G2]"},
			details: []string{
				"--successor names the live goal carrying the work; it is recorded after the abandonment is committed, and a refusal there is reported as partial.",
				"Live dependents are waived with --waive DEPENDENT=REASON or abandoned in the same act with --also DEPENDENT.",
			},
			flags: withFlags([]intentFlag{
				intentTargetFlag,
				reasonFlag("because", "why the goal will not be worked"),
				fileFlag("reason", "read the reason from FILE"),
				{name: "successor", value: "G2", usage: "the live goal carrying this one's work"},
				{name: "waive", value: "DEPENDENT=REASON", repeat: true, advanced: true, usage: "release a live dependent from this goal (repeatable)"},
				{name: "also", value: "G", repeat: true, advanced: true, usage: "abandon a live dependent in the same act (repeatable)"},
			}, intentHumanActFlags),
			maxArgs:  1,
			examples: []string{"metasystem abandon old-idea --reason 'superseded by the new design' --successor new-idea"},
			run:      runIntentAbandon,
		},
		{
			name: "block", audience: "both", summary: "record that a goal waits for another",
			usage:    []string{"metasystem block G --on G2"},
			details:  []string{"G parks until G2 is done, unless G2 is already done."},
			flags:    withFlags([]intentFlag{intentTargetFlag, {name: "on", aliases: []string{"blocker"}, value: "G2", usage: "the goal G waits for"}}, intentHumanActFlags),
			maxArgs:  1,
			examples: []string{"metasystem block slice-3 --on slice-2"},
			run:      runIntentBlock,
		},
		{
			name: "unblock", audience: "both", summary: "remove one blocker from a goal",
			usage:    []string{"metasystem unblock G --on G2"},
			details:  []string{"Removing a blocker that is not done is a person's act; the park lifts only when every remaining blocker is done."},
			flags:    withFlags([]intentFlag{intentTargetFlag, {name: "on", aliases: []string{"blocker"}, value: "G2", usage: "the goal G no longer waits for"}}, intentHumanActFlags, intentRelayFlags),
			maxArgs:  1,
			examples: []string{"metasystem unblock slice-3 --on slice-2"},
			run:      runIntentUnblock,
		},
		{
			name: "unapprove", audience: "human", summary: "withdraw a goal's execution approval",
			usage:    []string{"metasystem unapprove G --reason TEXT"},
			details:  []string{"A standing claim is parked with the approval."},
			flags:    withFlags([]intentFlag{intentTargetFlag, reasonFlag("because", "why the approval is withdrawn"), fileFlag("reason", "read the reason from FILE")}, intentHumanActFlags, intentRelayFlags),
			maxArgs:  1,
			examples: []string{"metasystem unapprove verbs-match-intent --reason 'the design changes first'"},
			run:      runIntentUnapprove,
		},
		{
			name: "grant", audience: "human", summary: "record a power of attorney a seat acts under",
			usage: []string{"metasystem grant --tiers LIST --acts LIST --until DATE"},
			details: []string{
				"--acts is from approve, budget and resume-parked; a seat then acts with --under GRANT on approve, budget and resume of a parked goal.",
				"--until is the last day covered, YYYY-MM-DD, at most seven days out. The grant's id is printed.",
			},
			flags: withFlags([]intentFlag{
				{name: "tiers", value: "LIST", usage: "the tiers covered, for example 1 or 1,2"},
				{name: "acts", value: "LIST", usage: "the acts covered: approve, budget, resume-parked"},
				{name: "until", aliases: []string{"expires"}, value: "DATE", usage: "the last day covered, YYYY-MM-DD"},
			}, intentHumanActFlags, intentRelayFlags),
			maxArgs:  0,
			examples: []string{"metasystem grant --tiers 1 --acts approve,budget --until 2026-10-01"},
			run:      runIntentGrant,
		},
		{
			name: "revoke", audience: "human", summary: "close a power of attorney early",
			usage:    []string{"metasystem revoke GRANT"},
			flags:    withFlags([]intentFlag{{name: "grant", value: "GRANT", advanced: true, usage: "the grant, as an alternative to naming it first"}}, intentHumanActFlags, intentRelayFlags),
			maxArgs:  1,
			examples: []string{"metasystem revoke 01ARZ3NDEKTSV4RRFFQ69G5FAV-mac-cli-m1"},
			run:      runIntentRevoke,
		},
		{
			name: "split", audience: "both", summary: "split a goal into independently claimable members",
			usage:    []string{"metasystem split G --plan FILE"},
			details:  []string{"FILE is the owner's member draft. The parent concludes as decomposed; its members form one arc."},
			flags:    withFlags([]intentFlag{intentTargetFlag, {name: "plan", aliases: []string{"members"}, value: "FILE", usage: "the member draft"}}, intentHumanActFlags),
			maxArgs:  1,
			examples: []string{"metasystem split big-goal --plan members.md"},
			run:      runIntentSplit,
		},
		{
			name: "group", audience: "both", summary: "move a goal into an arc",
			usage:    []string{"metasystem group G ARC"},
			flags:    withFlags([]intentFlag{intentTargetFlag, {name: "arc", value: "ARC", advanced: true, usage: "the arc, as an alternative to naming it after G"}}, intentHumanActFlags),
			maxArgs:  2,
			examples: []string{"metasystem group slice-2 verbs-match-intent"},
			run:      runIntentGroup,
		},
		{
			name: "ungroup", audience: "both", summary: "take a goal out of its arc",
			usage:    []string{"metasystem ungroup G"},
			details:  []string{"A claim riding the arc is released."},
			flags:    withFlags([]intentFlag{intentTargetFlag}, intentHumanActFlags),
			maxArgs:  1,
			examples: []string{"metasystem ungroup slice-2"},
			run:      runIntentUngroup,
		},
		{
			name: "resolve", audience: "both", summary: "discharge a review obligation with its test",
			usage: []string{"metasystem resolve G --review R --finding F --test NAME", "metasystem resolve G --review R --finding F --implementation-chain J --artifact PATH --result RUN --critic ROOT"},
			details: []string{
				"R is any job of the review; its chain root is read from the recorded job.",
				"The session holding G discharges under its own lineage; anyone else's discharge is a person's act.",
				"A fixture obligation is discharged by its implementation chain, artifact, governed test result and clean code-critic root.",
			},
			flags: withFlags([]intentFlag{
				intentTargetFlag,
				{name: "review", aliases: []string{"chain"}, value: "R", usage: "the review job (or its chain root)"},
				{name: "finding", value: "F", usage: "the finding id"},
				{name: "test", value: "NAME", usage: "the test that proves the finding resolved"},
				{name: "implementation-chain", value: "J", advanced: true, usage: "fixture obligation: the implementation chain carrying the fix"},
				{name: "artifact", value: "PATH", advanced: true, usage: "fixture obligation: the changed artifact"},
				{name: "result", value: "RUN", advanced: true, usage: "fixture obligation: the retained governed test result"},
				{name: "critic", value: "ROOT", advanced: true, usage: "fixture obligation: the clean code-critic root"},
			}, intentHumanActFlags),
			maxArgs:  1,
			examples: []string{"metasystem resolve verbs-match-intent --review job-7 --finding F-2 --test TestIntentReady"},
			run:      runIntentResolve,
		},
		{
			name: "notes", audience: "both", summary: "read, add or close a goal's non-breaking read findings",
			usage: []string{"metasystem notes G", "metasystem notes G --read LABEL --add TEXT...", "metasystem notes G --close ITEM --fixed COMMIT|--moved G2|--accepted REASON"},
			details: []string{
				"Notes are non-breaking read items. A finding stays a finding; closing a note never certifies one.",
				"--all lists closed items too.",
			},
			flags: withFlags([]intentFlag{
				intentTargetFlag,
				{name: "all", usage: "include closed items"},
				{name: "add", value: "TEXT", repeat: true, usage: "an item to record (repeatable)"},
				{name: "add-file", value: "FILE", advanced: true, usage: "record each line of FILE as its own note (the read-items --items-file format), not one note"},
				{name: "read", value: "LABEL", usage: "with --add: the read the items came from"},
				{name: "close", value: "ITEM", usage: "the item to close"},
				{name: "fixed", value: "COMMIT", usage: "with --close: the commit that fixed it"},
				{name: "moved", value: "G2", usage: "with --close: the open goal receiving it"},
				{name: "accepted", aliases: []string{"reason"}, value: "REASON", usage: "with --close: why it is not a defect"},
				fileFlag("accepted", "with --close: read why it is not a defect from FILE"),
			}, intentHumanActFlags),
			maxArgs:  1,
			examples: []string{"metasystem notes verbs-match-intent", "metasystem notes verbs-match-intent --read r2 --add 'help wraps at 80 columns'"},
			run:      runIntentNotes,
		},
		{
			name: "recover", audience: "both", summary: "recover the goal journal and this session's durable waits",
			usage: []string{"metasystem recover [G] [--session S]"},
			details: []string{
				"The journal recovery confirms, corrects, completes or closes every stranded goal entry of this installation; G, when named, is shown afterwards.",
				"The durable waits are the recorded wait continuations; --session checks they belong to the checkout holder's current session.",
				"Each owner's result is reported separately.",
			},
			flags:    []intentFlag{intentTargetFlag, {name: "session", value: "S", advanced: true, usage: "the runtime session that must hold the checkout"}},
			maxArgs:  1,
			examples: []string{"metasystem recover", "metasystem recover verbs-match-intent"},
			run:      runIntentRecover,
		},
		{
			name: "red", audience: "both", summary: "own or close a trunk-red incident",
			usage: []string{"metasystem red own ENTRY --goal G [--branch NAME]", "metasystem red close ENTRY --reason TEXT"},
			details: []string{
				"ENTRY is the retained incident id, as goals prints it. Owning is an agent's act; closing, and owning --by for another machine (--to), are a person's.",
			},
			flags: withFlags([]intentFlag{
				{name: "goal", value: "G", usage: "own: the goal fixing it"},
				{name: "branch", value: "NAME", advanced: true, usage: "own: the fix branch"},
				{name: "to", value: "MACHINE", advanced: true, usage: "own, with --by: the machine assigned the fix"},
				reasonFlag("why", "close: why the incident is closed"), fileFlag("reason", "read the reason from FILE"),
			}, intentHumanActFlags),
			maxArgs:  2,
			examples: []string{"metasystem red own tr-01 --goal fix-trunk", "metasystem red close tr-01 --reason 'the flake is fixed at its source'"},
			run:      runIntentRed,
		},
	}
}

// intentActor says who may perform an act.
type intentActor int

const (
	// actorEither is an agent session's act under its lineage, or a person's.
	actorEither intentActor = iota
	// actorHuman is only ever a person's act.
	actorHuman
	// actorAgent is only ever the holding session's own act.
	actorAgent
)

// actingAs is the owner's actor flags, and the observed proof when a person
// acts. An agent session acts under its own lineage. A person's act always
// proves the enrolled terminal first: a missing --by is filled with the
// enrolled name, and a typed --by must be that name. A typed name or lineage
// is never proof.
func (inv *intentInvocation) actingAs(verb, target string, actor intentActor) ([]string, *humanauthority.Proof, *intentResult) {
	args := inv.forward("lineage", "fixture-human-authority", "temporary-human-word", "review-by")
	typed := strings.TrimPrefix(inv.input.text("by"), "human:")
	agent := inv.input.has("lineage") || inv.owners.dependencies.ownerLineage != nil && inv.owners.dependencies.ownerLineage() != ""
	if typed != "" && actor == actorAgent {
		return nil, nil, &intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(target),
			Summary:  fmt.Sprintf("%s is the claim holder's own act and takes no --by; nothing was done", verb),
			Decision: "the session holding the claim runs it under its own lineage"}
	}
	if typed == "" && (actor == actorAgent || actor == actorEither && agent) {
		if !agent {
			return nil, nil, &intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(target),
				Summary:  "cannot tell which session acts: no --lineage and no METASYSTEM_OWNER_LINEAGE; nothing was done",
				Decision: "the session holding the work passes its own lineage with --lineage LINEAGE (a session is started with metasystem up)"}
		}
		return args, nil, nil
	}
	flags := &syncFlags{root: inv.stateRoot, fixtureHumanAuthority: inv.input.switched("fixture-human-authority"),
		temporaryWord: inv.input.text("temporary-human-word"), reviewBy: inv.input.text("review-by")}
	proof, err := proveGoalHumanAuthorityAt(verb, flags, inv.owners.prove, inv.owners.commandNow)
	switch {
	case err != nil:
	case flags.temporaryWord != "":
		// A recorded relayed word names the person it relays.
		if typed == "" {
			err = fmt.Errorf("a relayed word names its person with --by")
		}
		flags.by = typed
	default:
		if err = resolveGoalHuman(flags, proof); err == nil && typed != "" && typed != flags.by {
			err = fmt.Errorf("--by %s is not the person enrolled at this terminal", typed)
		}
	}
	if err != nil {
		summary := fmt.Sprintf("%s is a person's act and no enrolled person was proven here (%v); nothing was done", verb, err)
		if actor == actorEither && typed == "" {
			summary = fmt.Sprintf("cannot tell who acts: no agent lineage, and no enrolled person was proven here (%v); nothing was done", err)
		}
		return nil, nil, &intentResult{Outcome: intentRefused, code: 1, Targets: inv.targets(target), Summary: summary,
			Decision: "a person runs it at the enrolled terminal; an agent session passes --lineage LINEAGE"}
	}
	return append(args, "--by", flags.by), &proof, nil
}

// textValue is a TEXT option given inline or read from its --NAME-file,
// resolved against the directory the command was started in.
func (inv *intentInvocation) textValue(name string) (string, *intentResult) {
	value, path := inv.input.text(name), inv.input.text(name+"-file")
	if path == "" {
		return value, nil
	}
	text, problem := inv.readTextFile(name+"-file", path)
	if problem != nil {
		return "", problem
	}
	if inv.input.has(name) && value != text {
		return "", &intentResult{Outcome: intentRefused, code: 2,
			Summary:  fmt.Sprintf("--%s and --%s-file both give the %s; nothing was done", name, name, name),
			Decision: "give it once"}
	}
	return text, nil
}

// readTextFile reads one FILE option's text exactly, less its final line
// ends, resolved against the directory the command was started in.
func (inv *intentInvocation) readTextFile(option, path string) (string, *intentResult) {
	data, err := os.ReadFile(inv.inputPath(path))
	if err != nil {
		return "", &intentResult{Outcome: intentRefused, code: 2, Summary: fmt.Sprintf("cannot read --%s: %v; nothing was done", option, err)}
	}
	return strings.TrimRight(string(data), "\r\n"), nil
}

// resolveTextFiles reads every --NAME-file of this command whose NAME is a
// TEXT option, before any owner runs, so each owner receives the text as if
// it had been given inline. A repeatable option's files join its inline
// values in the order given. --add-file is left to the notes owner, which
// reads it as one note per line.
func (inv *intentInvocation) resolveTextFiles() *intentResult {
	for _, file := range inv.command.allFlags() {
		name, isFile := strings.CutSuffix(file.name, "-file")
		base, known := inv.command.lookupFlag(name)
		if !isFile || !known || base.value == "" || base.repeat != file.repeat || !inv.input.has(file.name) {
			continue
		}
		if !file.repeat {
			text, problem := inv.textValue(name)
			if problem != nil {
				return problem
			}
			inv.input.values[name] = []string{text}
			delete(inv.input.values, file.name)
			continue
		}
		var merged []string
		for _, given := range inv.input.sequence {
			value := given.value
			switch given.name {
			case name:
			case file.name:
				text, problem := inv.readTextFile(file.name, value)
				if problem != nil {
					return problem
				}
				value = text
			default:
				continue
			}
			if !containsIntentValue(merged, value) {
				merged = append(merged, value)
			}
		}
		inv.input.values[name] = merged
		delete(inv.input.values, file.name)
	}
	return nil
}

func (inv *intentInvocation) inputPath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(inv.cwd, path)
}

// forwardEach passes every value of a repeatable option through.
func (inv *intentInvocation) forwardEach(public, owner string) []string {
	var args []string
	for _, value := range inv.input.values[public] {
		args = append(args, "--"+owner, value)
	}
	return args
}

// namedGoal resolves the repository and the one named goal. When it returns
// false the refusal has been rendered and its exit code is returned.
func (inv *intentInvocation) namedGoal(fits func(*goal.GoalFile) bool) (string, int, bool) {
	id, problem := inv.singleTarget()
	if problem != nil {
		return "", inv.render(*problem), false
	}
	if problem := inv.selectRoot(); problem != nil {
		return "", inv.render(*problem), false
	}
	if id == "" {
		return "", inv.missingTarget(fits), false
	}
	return id, 0, true
}

func (inv *intentInvocation) refuse(target, summary, decision string) int {
	result := intentResult{Outcome: intentRefused, code: 2, Summary: summary, Decision: decision}
	if target != "" {
		result.Targets = inv.targets(target)
	}
	return inv.render(result)
}

// requestBuilder builds an owner's request with this invocation's clock and
// dependencies; the owner still decides who acts from the proof.
func (inv *intentInvocation) requestBuilder(proof *humanauthority.Proof, stopping bool) func(string, string, string, string) (goal.VerbRequest, error) {
	return func(verb, root, by, lineage string) (goal.VerbRequest, error) {
		if stopping {
			return syncStoppingReqWithProofWithDependencies(verb, root, by, lineage, proof, inv.owners.commandNow, inv.owners.dependencies)
		}
		return syncReqWithProofAtWithDependencies(verb, root, by, lineage, proof, inv.owners.commandNow, inv.owners.dependencies)
	}
}

// syncOwner runs one goal verb that exists only on the synced ledger.
func (inv *intentInvocation) syncOwner(name string, args []string, proof *humanauthority.Proof, stopping bool, run func(goal.VerbRequest, *syncFlags) (goal.PublishResult, error), required ...string) func(syncRequestDependencies) int {
	return func(dependencies syncRequestDependencies) int {
		return runSyncOnlyWithDependencies(name, run, inv.requestBuilder(proof, stopping), dependencies, required...)(args)
	}
}

func (inv *intentInvocation) goalAct(id, act string, run func(syncRequestDependencies) int) intentResult {
	return inv.ownerCall(inv.targets(id), run, func() intentResult { return inv.afterGoalAct(id, act) })
}

func (inv *intentInvocation) mutation(name string, args []string) func(syncRequestDependencies) int {
	return func(dependencies syncRequestDependencies) int {
		code, _ := trySyncMutationWithCompletion(name, args, inv.owners.commandNow, dependencies, inv.owners.parkBranchCheck, inv.owners.completion)
		return code
	}
}

// longBudgetBox reads the five long limits as one compact box. They are an
// alternative to a compact box, never mixed with one, and incomplete limits
// are refused rather than filled in.
func (inv *intentInvocation) longBudgetBox(compact string) (string, *intentResult) {
	var given, missing []string
	values := map[string]string{}
	for _, definition := range intentLongBudgetFlags {
		if inv.input.has(definition.name) {
			given = append(given, "--"+definition.name)
			values[definition.name] = inv.input.text(definition.name)
		} else {
			missing = append(missing, "--"+definition.name)
		}
	}
	switch {
	case len(given) == 0:
		return compact, nil
	case compact != "":
		return "", &intentResult{Outcome: intentRefused, code: 2,
			Summary:  fmt.Sprintf("gives the box twice, as %s and as %s; nothing was done", shellCommand([]string{compact}), strings.Join(given, " ")),
			Decision: "give either the compact box or all five long limits"}
	case len(missing) > 0:
		return "", &intentResult{Outcome: intentRefused, code: 2,
			Summary:  fmt.Sprintf("the long limits are one box and %s are missing; nothing was done", strings.Join(missing, ", ")),
			Decision: "give all five long limits, or the complete compact box such as 1d/10/720m/1/3"}
	}
	return fmt.Sprintf("%s/%s/%sm/%s/%s", values["elapsed-limit"], values["attempt-limit"], values["reserved-job-minutes-limit"],
		values["active-job-limit"], values["review-round-limit"]), nil
}

// runIntentApproveWithLimits and runIntentBudgetWithLimits accept the five
// long limits as the box and then take the ordinary route.
func runIntentApproveWithLimits(inv *intentInvocation) int {
	box, problem := inv.longBudgetBox(inv.input.text("budget"))
	if problem != nil {
		return inv.render(*problem)
	}
	if box != "" {
		inv.input.values["budget"] = []string{box}
	}
	return runIntentApprove(inv)
}

func runIntentBudgetWithLimits(inv *intentInvocation) int {
	compact := inv.input.text("budget")
	if len(inv.input.args) > 1 {
		compact = inv.input.args[1]
	}
	box, problem := inv.longBudgetBox(compact)
	if problem != nil {
		return inv.render(*problem)
	}
	if box != "" && box != compact {
		inv.input.values["budget"] = []string{box}
	}
	return runIntentBudget(inv)
}

// runIntentGoalViews answers goals --ready and goals --tiers from the
// existing frontier and tier probe; plain goals is the backlog listing.
func runIntentGoalViews(inv *intentInvocation) int {
	ready, tiers := inv.input.switched("ready"), inv.input.switched("tiers")
	if inv.input.switched("pretty") && !inv.input.switched("json") {
		return inv.refuse("", "--pretty formats JSON; add --json; nothing was read", "metasystem goals --json --pretty")
	}
	if ready || tiers {
		filters := []string{"history"}
		if tiers {
			filters = append(filters, "label", "machine")
		}
		for _, name := range filters {
			if inv.input.has(name) && (name != "history" || inv.input.switched(name)) {
				return inv.refuse("", fmt.Sprintf("--%s does not apply to this view; nothing was read", name), "drop --"+name+" or list with metasystem goals")
			}
		}
	}
	if !ready && !tiers {
		if inv.input.has("machine") {
			return inv.refuse("", "--machine selects whose ready frontier goals --ready shows; nothing was read", "add --ready")
		}
		return runIntentGoals(inv)
	}
	if ready && tiers || inv.input.switched("all") {
		return inv.refuse("", "--ready, --tiers and --all are different views; give one; nothing was read", "for example metasystem goals --ready")
	}
	if problem := inv.selectRoot(); problem != nil {
		return inv.render(*problem)
	}
	projection, _, problem := inv.projection()
	if problem != nil {
		return inv.render(*problem)
	}
	if tiers {
		probe := goal.ProbeTiers(projection.Tree)
		recorded, derived := probe.Tier3Share()
		lines := []string{
			fmt.Sprintf("recorded tiers: 1=%d 2=%d 3=%d (tier 3: %d%%)", probe.Recorded[1], probe.Recorded[2], probe.Recorded[3], recorded),
			fmt.Sprintf("derived tiers:  1=%d 2=%d 3=%d (tier 3: %d%%)", probe.Derived[1], probe.Derived[2], probe.Derived[3], derived),
		}
		for _, lower := range probe.Lowerable {
			lines = append(lines, fmt.Sprintf("lowerable: %s %s recorded=%d derived=%d", lower.ID, lower.State, lower.Recorded, lower.Derived))
		}
		return inv.render(intentResult{Outcome: intentConfirmed, text: lines,
			Summary: fmt.Sprintf("%d open goal(s) with a risk record at %s", probe.Open, projection.Tip),
			Data: map[string]any{"tip": projection.Tip, "open": probe.Open, "recorded": probe.Recorded, "derived": probe.Derived,
				"tier3ShareRecorded": recorded, "tier3ShareDerived": derived, "lowerable": probe.Lowerable}})
	}
	labels := inv.input.values["label"]
	if err := goal.ValidateLabels(labels); err != nil {
		return inv.refuse("", err.Error(), "use label tokens the ledger accepts")
	}
	machine := inv.input.text("machine")
	var err error
	if machine == "" {
		machine, err = inv.owners.dependencies.machine(inv.stateRoot)
	} else {
		err = goal.ValidateMachineNickname(machine)
	}
	if err != nil {
		return inv.render(intentResult{Outcome: intentRefused, code: 1, Summary: "cannot tell whose frontier to read: " + err.Error(), Decision: "name the machine with --machine NAME"})
	}
	frontier, err := goal.Next(projection, machine, labels...)
	if err != nil {
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: "the ready frontier could not be read: " + err.Error()})
	}
	selection := goal.SelectNext(frontier)
	result := intentResult{Outcome: intentConfirmed,
		Data: map[string]any{"tip": projection.Tip, "machine": machine, "frontier": frontier, "selection": selection}}
	switch selection.Kind {
	case goal.NextSelectionContinue:
		result.Summary = "continue your claimed goal: " + selection.GoalID
		result.Targets = inv.targets(selection.GoalID)
	case goal.NextSelectionReady:
		result.Summary = "next ready goal: " + selection.GoalID
		result.Targets = inv.targets(selection.GoalID)
		result.next, result.nextReason = inv.publicArgv("claim", selection.GoalID), "claim it"
	default:
		result.Summary = "no ready goal for " + machine
	}
	return inv.render(result)
}

func runIntentOpen(inv *intentInvocation) int {
	id, problem := inv.singleTarget()
	if problem != nil {
		return inv.render(*problem)
	}
	intent, problem := inv.textValue("intent")
	if problem == nil {
		var next string
		next, problem = inv.textValue("next")
		if problem == nil {
			return inv.openGoal(id, intent, next)
		}
	}
	return inv.render(*problem)
}

func (inv *intentInvocation) openGoal(id, intent, next string) int {
	if problem := inv.selectRoot(); problem != nil {
		return inv.render(*problem)
	}
	var missing []string
	for name, value := range map[string]string{"G": id, "--intent": intent, "--next": next, "--risk": inv.input.text("risk"), "--basis": inv.input.text("basis")} {
		if strings.TrimSpace(value) == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		slices.Sort(missing)
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(id),
			Summary:  fmt.Sprintf("a new goal needs %s; nothing was done", strings.Join(missing, ", ")),
			Decision: "the four risk answers (severity, novelty, exposure, accumulation) and their basis are a judgement about this goal, not a default",
			Data:     map[string]any{"missing": missing}})
	}
	actor, _, problem := inv.actingAs("open", id, actorEither)
	if problem != nil {
		return inv.render(*problem)
	}
	args := append([]string{"--root", inv.stateRoot, "--id", id, "--intent", intent, "--next", next}, actor...)
	args = append(args, inv.forward("risk", "basis", "tier", "origin")...)
	if reason := inv.input.text("reason"); reason != "" {
		args = append(args, "--why", reason)
	}
	args = append(args, inv.forwardEach("blocked-by", "blocked-by")...)
	args = append(args, inv.forwardEach("blocks", "blocks")...)
	args = append(args, inv.forwardEach("label", "label")...)
	return inv.render(inv.goalAct(id, "open", inv.mutation("open", args)))
}

func runIntentEdit(inv *intentInvocation) int {
	id, code, ok := inv.namedGoal(nil)
	if !ok {
		return code
	}
	var obligation []string
	for _, definition := range intentObligationFlags {
		if inv.input.has(definition.name) {
			obligation = append(obligation, "--"+definition.name)
		}
	}
	editFields := []string{"intent", "intent-file", "next", "next-file", "next-append", "next-append-file", "risk", "basis", "tier", "evidence", "reason", "label", "unlabel"}
	var edits []string
	for _, name := range editFields {
		if inv.input.has(name) {
			edits = append(edits, "--"+name)
		}
	}
	if len(obligation) > 0 {
		if !inv.input.has("obligation") {
			return inv.refuse(id, fmt.Sprintf("%s belong to an obligation, which --obligation STATE binds; nothing was done", strings.Join(obligation, " ")), "add --obligation STATE with every obligation field")
		}
		if len(edits) > 0 {
			return inv.refuse(id, fmt.Sprintf("binds an obligation and edits %s in one command; they are two acts; nothing was done", strings.Join(edits, " ")), "run the edit and the obligation as two commands")
		}
		return inv.bindObligation(id)
	}
	if inv.input.has("approved-ref") {
		return inv.refuse(id, "--approved-ref authorizes an obligation; an edit takes none; nothing was done", "add --obligation STATE with every obligation field, or drop --approved-ref")
	}
	intent, problem := inv.textValue("intent")
	if problem != nil {
		return inv.render(*problem)
	}
	next, problem := inv.textValue("next")
	if problem != nil {
		return inv.render(*problem)
	}
	appended, problem := inv.textValue("next-append")
	if problem != nil {
		return inv.render(*problem)
	}
	if next != "" && appended != "" {
		return inv.refuse(id, "--next replaces the next step and --next-append adds to it; give one; nothing was done", "")
	}
	if len(edits) == 0 {
		return inv.refuse(id, "names nothing to change; nothing was done", "give --intent, --next, --next-append, --risk with --basis, --tier, --label or --unlabel")
	}
	actor, proof, problem := inv.actingAs("edit", id, actorEither)
	if problem != nil {
		return inv.render(*problem)
	}
	args := append([]string{"--root", inv.stateRoot, "--id", id}, actor...)
	if intent != "" {
		args = append(args, "--intent", intent)
	}
	if next != "" {
		args = append(args, "--next", next)
	}
	args = append(args, inv.forward("risk", "basis", "tier", "evidence")...)
	if reason := inv.input.text("reason"); reason != "" {
		args = append(args, "--why", reason)
	}
	args = append(args, inv.forwardEach("label", "label")...)
	args = append(args, inv.forwardEach("unlabel", "unlabel")...)
	commandNow := inv.owners.commandNow
	return inv.render(inv.goalAct(id, "edit", inv.syncOwner("edit", args, proof, false, func(req goal.VerbRequest, f *syncFlags) (goal.PublishResult, error) {
		// The addition is applied to the next step at the tip this edit publishes.
		return goalEditEffectAppending(req, f, commandNow, appended)
	}, "id")))
}

// bindObligation is set-obligation's act with its own fields.
func (inv *intentInvocation) bindObligation(id string) int {
	actor, _, problem := inv.actingAs("set-obligation", id, actorHuman)
	if problem != nil {
		return inv.render(*problem)
	}
	args := append([]string{"--root", inv.stateRoot, "--id", id, "--state", inv.input.text("obligation")}, actor...)
	for _, definition := range intentObligationFlags[1:] {
		if definition.repeat {
			args = append(args, inv.forwardEach(definition.name, definition.name)...)
		} else {
			args = append(args, inv.forward(definition.name)...)
		}
	}
	args = append(args, inv.forward("approved-ref")...)
	return inv.render(inv.goalAct(id, "obligation", func(dependencies syncRequestDependencies) int {
		return runGoalSetObligationWithAuthorityFactsAtWithDependencies(args, inv.owners.prove, inv.owners.commandNow, dependencies)
	}))
}

// heldGoals are the live goals claimed by this machine under the acting
// session's lineage.
func (inv *intentInvocation) heldGoals(projection goal.Projection) ([]string, string, error) {
	lineage := inv.input.text("lineage")
	if lineage == "" && inv.owners.dependencies.ownerLineage != nil {
		lineage = inv.owners.dependencies.ownerLineage()
	}
	if lineage == "" {
		return nil, "", fmt.Errorf("no --lineage and no METASYSTEM_OWNER_LINEAGE")
	}
	machine, err := inv.owners.dependencies.machine(inv.stateRoot)
	if err != nil {
		return nil, "", err
	}
	var held []string
	for _, id := range goal.OrderedOpenGoalIDs(projection.Tree.Live) {
		file := projection.Tree.Live[id]
		if file.State == goal.StateClaimed && file.Claimed != nil && file.Claimed.Machine == machine && file.Claimed.Lineage == lineage {
			held = append(held, id)
		}
	}
	return held, machine, nil
}

// uniqueHeldGoal is the goal named, or else the one goal this session holds.
func (inv *intentInvocation) uniqueHeldGoal() (string, int, bool) {
	id, problem := inv.singleTarget()
	if problem != nil {
		return "", inv.render(*problem), false
	}
	if problem := inv.selectRoot(); problem != nil {
		return "", inv.render(*problem), false
	}
	if id != "" {
		return id, 0, true
	}
	projection, _, problem := inv.projection()
	if problem != nil {
		return "", inv.render(*problem), false
	}
	held, machine, err := inv.heldGoals(projection)
	switch {
	case err != nil:
		return "", inv.render(intentResult{Outcome: intentRefused, code: 2,
			Summary:  "needs a goal: without G it acts on the one goal this session holds, and the session is unknown: " + err.Error() + "; nothing was done",
			Decision: "name the goal, or pass the session's --lineage LINEAGE"}), false
	case len(held) == 1:
		return held[0], 0, true
	case len(held) == 0:
		return "", inv.render(intentResult{Outcome: intentRefused, code: 1,
			Summary: fmt.Sprintf("this session holds no claimed goal on %s; nothing was done", machine), Decision: "name the goal"}), false
	}
	return "", inv.render(intentResult{Outcome: intentRefused, code: 2,
		Summary:  fmt.Sprintf("this session holds %d goals (%s); nothing was done", len(held), strings.Join(held, " ")),
		Decision: "name the goal", Data: map[string]any{"candidates": held}}), false
}

func runIntentClaim(inv *intentInvocation) int {
	if inv.input.switched("take-over") {
		return inv.takeOver()
	}
	if inv.input.has("reason") {
		return inv.refuse("", "--reason explains a take-over; an ordinary claim takes none; nothing was done", "add --take-over to displace another machine's claim, or drop --reason")
	}
	id, problem := inv.singleTarget()
	if problem != nil {
		return inv.render(*problem)
	}
	box, problem := inv.longBudgetBox(inv.input.text("budget"))
	if problem != nil {
		return inv.render(*problem)
	}
	if problem := inv.selectRoot(); problem != nil {
		return inv.render(*problem)
	}
	projection, _, problem := inv.projection()
	if problem != nil {
		return inv.render(*problem)
	}
	if id == "" {
		if inv.input.switched("arc") {
			return inv.refuse("", "--arc claims a named goal's arc; nothing was done", "name the goal: metasystem claim G --arc")
		}
		machine, err := inv.owners.dependencies.machine(inv.stateRoot)
		if err != nil {
			return inv.render(intentResult{Outcome: intentRefused, code: 1, Summary: "cannot tell this machine: " + err.Error() + "; nothing was done", Decision: "name the goal"})
		}
		frontier, err := goal.Next(projection, machine, inv.input.values["label"]...)
		if err != nil {
			return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: "the ready frontier could not be read: " + err.Error()})
		}
		selection := goal.SelectNext(frontier)
		switch selection.Kind {
		case goal.NextSelectionContinue:
			// Held work is continued, never switched for the frontier's next goal.
			return inv.render(intentResult{Outcome: intentUnchanged, Targets: inv.targets(selection.GoalID),
				Summary: fmt.Sprintf("%s already holds %s; continue it (a claim never switches held work)", machine, selection.GoalID),
				next:    inv.publicArgv("show", selection.GoalID), nextReason: "the held goal and its next step",
				Data: map[string]any{"machine": machine, "selection": selection}})
		case goal.NextSelectionReady:
			id = selection.GoalID
		default:
			return inv.render(intentResult{Outcome: intentRefused, code: 1,
				Summary:  fmt.Sprintf("no ready goal for %s; nothing was claimed", machine),
				Decision: "a goal becomes ready when a person approves it", Data: map[string]any{"machine": machine, "frontier": frontier}})
		}
	} else if inv.input.has("label") {
		return inv.refuse(id, "--label chooses among ready goals; a named goal takes none; nothing was done", "drop --label, or omit G")
	}
	if file, _ := goalRecord(projection, id); file == nil {
		return unknownGoal(inv, id)
	}
	actor, proof, problem := inv.actingAs("claim", id, actorEither)
	if problem != nil {
		return inv.render(*problem)
	}
	args := append([]string{"--root", inv.stateRoot, "--id", id}, actor...)
	if box != "" {
		budget, problem := inv.completeBox(box, projection.Tree.Live[id])
		if problem != nil {
			return inv.render(*problem)
		}
		args = append(args, budgetLongFlags(budget)...)
	}
	if inv.input.switched("arc") {
		args = append(args, "--arc", id)
	}
	arc := inv.input.switched("arc")
	return inv.render(inv.goalAct(id, "claim", inv.syncOwner("claim", args, proof, false, func(req goal.VerbRequest, f *syncFlags) (goal.PublishResult, error) {
		budget, err := f.budgetTuple(false)
		if err != nil {
			return goal.PublishResult{}, err
		}
		var budgets []goal.Budget
		if budget != nil {
			budgets = append(budgets, *budget)
		}
		if arc {
			return goal.ClaimArc(req, f.id, budgets...)
		}
		return goal.Claim(req, f.id, budgets...)
	}, "id")))
}

// takeOver displaces another machine's claim through the steal owner; it is
// a person's explicit act with its reason.
func (inv *intentInvocation) takeOver() int {
	id, code, ok := inv.namedGoal(func(file *goal.GoalFile) bool { return file.State == goal.StateClaimed })
	if !ok {
		return code
	}
	reason := strings.TrimSpace(inv.input.text("reason"))
	if reason == "" {
		return inv.refuse(id, "taking over another machine's claim needs its reason; nothing was done", "say why with --reason TEXT")
	}
	for _, name := range []string{"arc", "budget", "label"} {
		if inv.input.has(name) {
			return inv.refuse(id, "--"+name+" does not apply to a take-over; nothing was done", "drop --"+name)
		}
	}
	for _, definition := range intentLongBudgetFlags {
		if inv.input.has(definition.name) {
			return inv.refuse(id, "--"+definition.name+" does not apply to a take-over; nothing was done", "drop --"+definition.name)
		}
	}
	actor, proof, problem := inv.actingAs("steal", id, actorHuman)
	if problem != nil {
		return inv.render(*problem)
	}
	args := append([]string{"--root", inv.stateRoot, "--id", id}, actor...)
	return inv.render(inv.goalAct(id, "take over", inv.syncOwner("steal", args, proof, false, func(req goal.VerbRequest, f *syncFlags) (goal.PublishResult, error) {
		return goal.StealWithReason(req, f.id, reason)
	}, "id")))
}

func runIntentRelease(inv *intentInvocation) int {
	id, code, ok := inv.uniqueHeldGoal()
	if !ok {
		return code
	}
	reason := strings.TrimSpace(inv.input.text("reason"))
	if reason == "" {
		return inv.refuse(id, "a release is recorded with its reason; nothing was done", "say why with --reason TEXT")
	}
	actor, proof, problem := inv.actingAs("release", id, actorEither)
	if problem != nil {
		return inv.render(*problem)
	}
	arc := inv.input.switched("arc")
	args := append([]string{"--root", inv.stateRoot, "--id", id}, actor...)
	return inv.render(inv.goalAct(id, "release", inv.syncOwner("release", args, proof, true, func(req goal.VerbRequest, f *syncFlags) (goal.PublishResult, error) {
		if arc {
			return goal.ReleaseArcWithReason(req, f.id, reason)
		}
		return goal.ReleaseWithReason(req, f.id, reason)
	}, "id")))
}

func runIntentReady(inv *intentInvocation) int {
	id, code, ok := inv.uniqueHeldGoal()
	if !ok {
		return code
	}
	actor, proof, problem := inv.actingAs("ready", id, actorAgent)
	if problem != nil {
		return inv.render(*problem)
	}
	args := append([]string{"--root", inv.stateRoot, "--id", id}, actor...)
	return inv.render(inv.goalAct(id, "ready", inv.syncOwner("land-ready", args, proof, false, func(req goal.VerbRequest, f *syncFlags) (goal.PublishResult, error) {
		return goal.LandReady(req, f.id)
	}, "id")))
}

// reviewRoot is the chain root of a named review job, read from its record.
func (inv *intentInvocation) reviewRoot(id, review string) (string, *intentResult) {
	if review == goal.HumanCarriedChain {
		return review, nil
	}
	root, err := dispatchcore.ChainRootOf(inv.stateRoot, review)
	if err != nil {
		return "", &intentResult{Outcome: intentRefused, code: 1, Targets: inv.targets(id),
			Summary:  fmt.Sprintf("review %s cannot be read: %v; nothing was done", shellCommand([]string{review}), err),
			Decision: "name a recorded review job of this goal"}
	}
	return root, nil
}

func runIntentDecide(inv *intentInvocation) int {
	id, code, ok := inv.namedGoal(nil)
	if !ok {
		return code
	}
	reason, problem := inv.textValue("reason")
	if problem != nil {
		return inv.render(*problem)
	}
	var missing []string
	for _, name := range []string{"finding", "review"} {
		if inv.input.text(name) == "" {
			missing = append(missing, "--"+name)
		}
	}
	if strings.TrimSpace(reason) == "" {
		missing = append(missing, "--reason")
	}
	if len(missing) > 0 {
		return inv.refuse(id, fmt.Sprintf("a risk decision needs %s; nothing was done", strings.Join(missing, ", ")), "name the finding, its review and why its risk is accepted")
	}
	chain, problem := inv.reviewRoot(id, inv.input.text("review"))
	if problem != nil {
		return inv.render(*problem)
	}
	actor, _, problem := inv.actingAs("accept-risk", id, actorHuman)
	if problem != nil {
		return inv.render(*problem)
	}
	args := append([]string{"--root", inv.stateRoot, "--id", id, "--finding", inv.input.text("finding"), "--chain", chain, "--why", reason}, actor...)
	result := inv.goalAct(id, "decide", func(dependencies syncRequestDependencies) int {
		return runGoalAcceptRiskWithFacts(args, inv.owners.prove, inv.owners.commandNow, dependencies, nil)
	})
	if data, ok := result.Data.(map[string]any); ok {
		data["chain"] = chain
	} else if result.Data == nil {
		result.Data = map[string]any{"chain": chain}
	}
	if result.Outcome == intentPartial {
		result.Summary = "the goal records the accepted risk, but a later record did not land: " + result.Summary
	}
	return inv.render(result)
}

func runIntentPin(inv *intentInvocation) int {
	id, code, ok := inv.namedGoal(nil)
	if !ok {
		return code
	}
	machine := ""
	if len(inv.input.args) > 1 {
		machine = inv.input.args[1]
	}
	clear := inv.input.switched("clear")
	switch {
	case clear && machine != "":
		return inv.refuse(id, fmt.Sprintf("names machine %s and --clear; nothing was done", shellCommand([]string{machine})), "pin to the machine, or clear the pin")
	case clear:
		machine = "-"
	case machine == "" || machine == "-":
		return inv.refuse(id, "needs the machine to pin the goal to; nothing was done", "metasystem pin "+id+" MACHINE, or metasystem pin "+id+" --clear")
	}
	actor, proof, problem := inv.actingAs("set-pin", id, actorHuman)
	if problem != nil {
		return inv.render(*problem)
	}
	args := append([]string{"--root", inv.stateRoot, "--id", id, "--pin", machine}, actor...)
	return inv.render(inv.goalAct(id, "pin", inv.syncOwner("set-pin", args, proof, false, func(req goal.VerbRequest, f *syncFlags) (goal.PublishResult, error) {
		return goal.SetPin(req, f.id, f.pin)
	}, "id", "pin")))
}

func runIntentPrioritize(inv *intentInvocation) int {
	id, code, ok := inv.namedGoal(nil)
	if !ok {
		return code
	}
	priority := inv.input.text("priority")
	if len(inv.input.args) > 1 {
		if priority != "" && priority != inv.input.args[1] {
			return inv.refuse(id, fmt.Sprintf("names two priorities, %s and --priority %s; nothing was done", inv.input.args[1], priority), "give the priority once")
		}
		priority = inv.input.args[1]
	}
	if priority != "1" && priority != "2" && priority != "3" {
		return inv.refuse(id, fmt.Sprintf("the priority is 1, 2 or 3, not %s; nothing was done", shellCommand([]string{priority})), "metasystem prioritize "+id+" 1|2|3")
	}
	actor, _, problem := inv.actingAs("set-priority", id, actorHuman)
	if problem != nil {
		return inv.render(*problem)
	}
	args := append([]string{"--root", inv.stateRoot, "--id", id, "--priority", priority}, actor...)
	args = append(args, inv.forward("sequence")...)
	return inv.render(inv.goalAct(id, "prioritize", func(dependencies syncRequestDependencies) int {
		return runGoalSetPriorityWithAuthorityAndInputs(args, inv.owners.prove, dependencies)
	}))
}

func runIntentReopen(inv *intentInvocation) int {
	id, code, ok := inv.namedGoal(func(*goal.GoalFile) bool { return false })
	if !ok {
		return code
	}
	next, problem := inv.textValue("next")
	if problem != nil {
		return inv.render(*problem)
	}
	if strings.TrimSpace(next) == "" {
		return inv.refuse(id, "a reopened goal needs its fresh next step; nothing was done", "say what happens next with --next TEXT")
	}
	projection, _, problem := inv.projection()
	if problem != nil {
		return inv.render(*problem)
	}
	file, where := goalRecord(projection, id)
	switch {
	case file == nil:
		return unknownGoal(inv, id)
	case where == "live":
		return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: inv.targets(id),
			Summary: fmt.Sprintf("%s is %s, not done or abandoned; nothing was done", id, file.State),
			next:    inv.publicArgv("edit", id, "--next", next), nextReason: "a live goal's next step is edited"})
	}
	actorKind := actorEither
	if where == "abandoned" {
		actorKind = actorHuman
	}
	actor, proof, problem := inv.actingAs("reopen", id, actorKind)
	if problem != nil {
		return inv.render(*problem)
	}
	reopened := inv.goalAct(id, "reopen", inv.mutation("reopen", append([]string{"--root", inv.stateRoot, "--id", id}, actor...)))
	if reopened.Outcome != intentConfirmed {
		return inv.render(reopened)
	}
	edited := inv.goalAct(id, "reopen", inv.syncOwner("set-next", append([]string{"--root", inv.stateRoot, "--id", id, "--next", next}, actor...), proof, false,
		func(req goal.VerbRequest, f *syncFlags) (goal.PublishResult, error) {
			return goal.Edit(req, f.id, goal.EditFields{NextStep: &f.next})
		}, "id"))
	if edited.Outcome == intentConfirmed {
		return inv.render(edited)
	}
	return inv.render(partialAfter(reopened, edited, "reopened "+id+", but its fresh next step was not recorded",
		inv.publicArgv("edit", id, "--next", next), "record the next step on the reopened goal"))
}

// partialAfter is a second act's failure after the first one committed.
func partialAfter(first, second intentResult, summary string, next []string, reason string) intentResult {
	result := second
	result.Outcome, result.code = intentPartial, max(second.code, 1)
	result.Summary = summary + ": " + second.Summary
	result.Data = map[string]any{"committed": first.Data, "refused": second.Data}
	result.next, result.nextReason, result.Decision = next, reason, ""
	return result
}

func runIntentAbandon(inv *intentInvocation) int {
	id, code, ok := inv.namedGoal(nil)
	if !ok {
		return code
	}
	reason, problem := inv.textValue("reason")
	if problem != nil {
		return inv.render(*problem)
	}
	if strings.TrimSpace(reason) == "" {
		return inv.refuse(id, "needs the reason the goal will never be worked; nothing was done", "say why with --reason TEXT")
	}
	successor := inv.input.text("successor")
	if successor == id {
		return inv.refuse(id, "a goal cannot carry itself; nothing was done", "name the live goal carrying the work")
	}
	actor, _, problem := inv.actingAs("abandon", id, actorHuman)
	if problem != nil {
		return inv.render(*problem)
	}
	args := append([]string{"--root", inv.stateRoot, "--id", id, "--because", reason}, actor...)
	args = append(args, inv.forwardEach("waive", "waive")...)
	args = append(args, inv.forwardEach("also", "also")...)
	abandoned := inv.goalAct(id, "abandon", func(dependencies syncRequestDependencies) int {
		return runGoalAbandonWithInputs(args, inv.owners.prove, inv.owners.commandNow, dependencies)
	})
	if abandoned.Outcome != intentConfirmed || successor == "" {
		return inv.render(abandoned)
	}
	carryArgs := append([]string{"--root", inv.stateRoot, "--id", id, "--to", successor}, actor...)
	carried := inv.ownerCall(inv.targets(id, successor), func(dependencies syncRequestDependencies) int {
		return runGoalCarryAbandonedWithInputs(carryArgs, inv.owners.prove, inv.owners.commandNow, dependencies)
	}, func() intentResult { return inv.afterGoalAct(id, "abandon") })
	if carried.Outcome == intentConfirmed {
		carried.Summary = fmt.Sprintf("abandoned %s; %s carries its work", id, successor)
		return inv.render(carried)
	}
	return inv.render(partialAfter(abandoned, carried, "abandoned "+id+", but "+successor+" was not recorded as its successor",
		inv.publicArgv("internal", "goal", "carry", "--id", id, "--to", successor), "name the successor of the abandoned goal"))
}

func runIntentBlock(inv *intentInvocation) int   { return inv.edge("block") }
func runIntentUnblock(inv *intentInvocation) int { return inv.edge("unblock") }

func (inv *intentInvocation) edge(verb string) int {
	id, code, ok := inv.namedGoal(nil)
	if !ok {
		return code
	}
	on := inv.input.text("on")
	if on == "" {
		return inv.refuse(id, "needs the other goal: --on G2; nothing was done", "metasystem "+verb+" "+id+" --on G2")
	}
	actor, _, problem := inv.actingAs(verb, id, actorEither)
	if problem != nil {
		return inv.render(*problem)
	}
	args := append([]string{"--root", inv.stateRoot, "--id", id, "--blocker", on}, actor...)
	return inv.render(inv.ownerCall(inv.targets(id, on), func(dependencies syncRequestDependencies) int {
		if verb == "block" {
			return runGoalBlockWithInputs(args, inv.owners.prove, inv.owners.commandNow, dependencies)
		}
		return runGoalUnblockWithInputs(args, inv.owners.prove, inv.owners.commandNow, dependencies)
	}, func() intentResult { return inv.afterGoalAct(id, verb) }))
}

func runIntentUnapprove(inv *intentInvocation) int {
	id, code, ok := inv.namedGoal(nil)
	if !ok {
		return code
	}
	reason, problem := inv.textValue("reason")
	if problem != nil {
		return inv.render(*problem)
	}
	if strings.TrimSpace(reason) == "" {
		return inv.refuse(id, "needs the reason the approval is withdrawn; nothing was done", "say why with --reason TEXT")
	}
	actor, _, problem := inv.actingAs("unapprove", id, actorHuman)
	if problem != nil {
		return inv.render(*problem)
	}
	args := append([]string{"--root", inv.stateRoot, "--id", id, "--because", reason}, actor...)
	return inv.render(inv.goalAct(id, "unapprove", func(dependencies syncRequestDependencies) int {
		return runGoalUnapproveWithInputs(args, inv.owners.prove, inv.owners.commandNow, dependencies)
	}))
}

// grantActs are the public act names and the owner verbs they cover.
var grantActs = map[string]string{"approve": "approve", "budget": "set-budget", "resume-parked": "unpark"}

func runIntentGrant(inv *intentInvocation) int {
	var missing []string
	for _, name := range []string{"tiers", "acts", "until"} {
		if inv.input.text(name) == "" {
			missing = append(missing, "--"+name)
		}
	}
	if len(missing) > 0 {
		return inv.refuse("", fmt.Sprintf("a power of attorney needs %s; nothing was done", strings.Join(missing, ", ")), "metasystem grant --tiers 1 --acts approve,budget --until YYYY-MM-DD")
	}
	var verbs []string
	for _, act := range strings.Split(inv.input.text("acts"), ",") {
		act = strings.TrimSpace(act)
		if act == "" {
			continue
		}
		verb, known := grantActs[act]
		if !known {
			return inv.refuse("", fmt.Sprintf("%s is not an act a power of attorney covers; they are approve, budget and resume-parked; nothing was done", shellCommand([]string{act})), "")
		}
		if !slices.Contains(verbs, verb) {
			verbs = append(verbs, verb)
		}
	}
	if problem := inv.selectRoot(); problem != nil {
		return inv.render(*problem)
	}
	actor, _, problem := inv.actingAs("grant", "", actorHuman)
	if problem != nil {
		return inv.render(*problem)
	}
	args := append([]string{"--root", inv.stateRoot, "--tiers", inv.input.text("tiers"), "--verbs", strings.Join(verbs, ","), "--expires", inv.input.text("until")}, actor...)
	report := &ownerReport{}
	dependencies := inv.owners.dependencies
	dependencies.report = report
	code := runGoalGrantWithInputs(args, inv.owners.prove, inv.owners.commandNow, dependencies)
	result := ownerResult(report, code, intentResult{Summary: "granted " + report.entry,
		Data: map[string]any{"grant": report.entry, "tiers": inv.input.text("tiers"), "acts": verbs, "until": inv.input.text("until")}})
	if report.entry != "" {
		result.Targets = []intentTarget{{Kind: "grant", ID: report.entry}}
	}
	if result.Outcome == intentConfirmed {
		result.text = append(result.text, "a seat acts under it with --under "+report.entry)
		if data, ok := result.Data.(map[string]any); ok && report.result != nil {
			data["owner"] = ownerPublication(*report.result)
		}
	}
	return inv.render(result)
}

func runIntentRevoke(inv *intentInvocation) int {
	entry := inv.input.text("grant")
	if len(inv.input.args) > 0 {
		if entry != "" && entry != inv.input.args[0] {
			return inv.refuse("", "names two grants; nothing was done", "name the grant once")
		}
		entry = inv.input.args[0]
	}
	if entry == "" {
		return inv.refuse("", "needs the grant to close: metasystem revoke GRANT; nothing was done", "the grant's id was printed when it was recorded")
	}
	if problem := inv.selectRoot(); problem != nil {
		return inv.render(*problem)
	}
	actor, _, problem := inv.actingAs("revoke", "", actorHuman)
	if problem != nil {
		return inv.render(*problem)
	}
	args := append([]string{"--root", inv.stateRoot, "--id", entry}, actor...)
	return inv.render(inv.ownerCall([]intentTarget{{Kind: "grant", ID: entry}}, func(dependencies syncRequestDependencies) int {
		return runGoalRevokeWithInputs(args, inv.owners.prove, inv.owners.commandNow, dependencies)
	}, func() intentResult {
		return intentResult{Summary: "revoked " + entry, Data: map[string]any{"grant": entry}}
	}))
}

func runIntentSplit(inv *intentInvocation) int {
	id, code, ok := inv.namedGoal(nil)
	if !ok {
		return code
	}
	plan := inv.input.text("plan")
	if plan == "" {
		return inv.refuse(id, "needs the member draft: --plan FILE; nothing was done", "write the members in the draft format goal split reads")
	}
	actor, _, problem := inv.actingAs("split", id, actorEither)
	if problem != nil {
		return inv.render(*problem)
	}
	args := append([]string{"--root", inv.stateRoot, "--id", id, "--members", inv.inputPath(plan)}, actor...)
	return inv.render(inv.goalAct(id, "split", func(dependencies syncRequestDependencies) int {
		return runGoalSplitWithInputs(args, inv.owners.commandNow, dependencies)
	}))
}

func runIntentGroup(inv *intentInvocation) int {
	id, code, ok := inv.namedGoal(nil)
	if !ok {
		return code
	}
	arc := inv.input.text("arc")
	if len(inv.input.args) > 1 {
		if arc != "" && arc != inv.input.args[1] {
			return inv.refuse(id, "names two arcs; nothing was done", "name the arc once")
		}
		arc = inv.input.args[1]
	}
	if arc == "" {
		return inv.refuse(id, "needs the arc: metasystem group G ARC; nothing was done", "")
	}
	actor, proof, problem := inv.actingAs("set-arc", id, actorEither)
	if problem != nil {
		return inv.render(*problem)
	}
	args := append([]string{"--root", inv.stateRoot, "--id", id, "--arc", arc}, actor...)
	return inv.render(inv.goalAct(id, "group", inv.syncOwner("set-arc", args, proof, false, func(req goal.VerbRequest, f *syncFlags) (goal.PublishResult, error) {
		return goal.SetArc(req, f.id, f.arc)
	}, "id", "arc")))
}

func runIntentUngroup(inv *intentInvocation) int {
	id, code, ok := inv.namedGoal(nil)
	if !ok {
		return code
	}
	actor, proof, problem := inv.actingAs("detach", id, actorEither)
	if problem != nil {
		return inv.render(*problem)
	}
	args := append([]string{"--root", inv.stateRoot, "--id", id}, actor...)
	return inv.render(inv.goalAct(id, "ungroup", inv.syncOwner("detach", args, proof, false, func(req goal.VerbRequest, f *syncFlags) (goal.PublishResult, error) {
		return goal.Detach(req, f.id)
	}, "id")))
}

func runIntentResolve(inv *intentInvocation) int {
	id, code, ok := inv.namedGoal(nil)
	if !ok {
		return code
	}
	var missing []string
	for _, name := range []string{"review", "finding"} {
		if inv.input.text(name) == "" {
			missing = append(missing, "--"+name)
		}
	}
	fixture := []string{"implementation-chain", "artifact", "result", "critic"}
	var fixtureGiven []string
	for _, name := range fixture {
		if inv.input.has(name) {
			fixtureGiven = append(fixtureGiven, "--"+name)
		}
	}
	switch {
	case inv.input.has("test") && len(fixtureGiven) > 0:
		return inv.refuse(id, fmt.Sprintf("gives both --test and the fixture proof %s; nothing was done", strings.Join(fixtureGiven, " ")), "a finding is discharged by its test, or a fixture obligation by its four proof fields")
	case !inv.input.has("test") && len(fixtureGiven) < len(fixture):
		missing = append(missing, "--test (or all of --implementation-chain, --artifact, --result, --critic)")
	}
	if len(missing) > 0 {
		return inv.refuse(id, fmt.Sprintf("discharging a review obligation needs %s; nothing was done", strings.Join(missing, ", ")), "")
	}
	chain, problem := inv.reviewRoot(id, inv.input.text("review"))
	if problem != nil {
		return inv.render(*problem)
	}
	actor, proof, problem := inv.actingAs("discharge-review-obligation", id, actorEither)
	if problem != nil {
		return inv.render(*problem)
	}
	if proof == nil {
		if chain == goal.HumanCarriedChain {
			return inv.refuse(id, "a human-carried finding is discharged by the person who carried it; nothing was done", "a person runs it at the enrolled terminal")
		}
		return inv.render(inv.goalAct(id, "resolve", func(dependencies syncRequestDependencies) int {
			return inv.dischargeAsOwningSession(id, chain, actor, dependencies)
		}))
	}
	args := append([]string{"--root", inv.stateRoot, "--id", id, "--finding", inv.input.text("finding"), "--chain", chain}, actor...)
	args = append(args, inv.forward("test", "implementation-chain", "artifact", "result", "critic")...)
	return inv.render(inv.goalAct(id, "resolve", func(dependencies syncRequestDependencies) int {
		return runGoalDischargeReviewObligationWithDependencies(args, inv.requestBuilder(proof, false), dischargeReviewObligation, dependencies)
	}))
}

// dischargeAsOwningSession is the owning session's discharge: the request
// carries only the session's lineage, so the owner accepts it only from the
// pair holding the goal, and the discharge is attributed to that pair.
func (inv *intentInvocation) dischargeAsOwningSession(id, chain string, actor []string, dependencies syncRequestDependencies) int {
	lineage := ""
	for index := 0; index+1 < len(actor); index++ {
		if actor[index] == "--lineage" {
			lineage = actor[index+1]
		}
	}
	req, err := inv.requestBuilder(nil, false)("discharge-review-obligation", inv.stateRoot, "", lineage)
	if err != nil {
		dependencies.complain(err)
		return 1
	}
	if lineage == "" {
		lineage = req.Actor.Lineage
	}
	evidence := goal.DischargeEvidence{Root: inv.stateRoot, ImplementationChain: inv.input.text("implementation-chain"),
		Artifact: inv.input.text("artifact"), ResultRunID: inv.input.text("result"), CriticRoot: inv.input.text("critic")}
	attribution := req.Actor.Machine + "+" + lineage
	res, err := dischargeReviewObligation(req, id, inv.input.text("finding"), chain, attribution, inv.input.text("test"), evidence)
	return dependencies.publish(res, err)
}

func runIntentNotes(inv *intentInvocation) int {
	id, code, ok := inv.namedGoal(nil)
	if !ok {
		return code
	}
	adding := inv.input.has("add") || inv.input.has("add-file")
	closing := inv.input.has("close")
	var closure []string
	for _, name := range []string{"fixed", "moved", "accepted"} {
		if inv.input.has(name) {
			closure = append(closure, "--"+name)
		}
	}
	switch {
	case adding && closing:
		return inv.refuse(id, "adds and closes notes in one command; they are two acts; nothing was done", "run them as two commands")
	case adding:
		if inv.input.text("read") == "" {
			return inv.refuse(id, "added notes name the read they came from: --read LABEL; nothing was done", "")
		}
		if inv.input.has("add") && inv.input.has("add-file") {
			return inv.refuse(id, "--add and --add-file both give the items; nothing was done", "give them one way")
		}
		if len(closure) > 0 {
			return inv.refuse(id, strings.Join(closure, " ")+" close a note; nothing was done", "")
		}
		actor, proof, problem := inv.actingAs("read-items", id, actorEither)
		if problem != nil {
			return inv.render(*problem)
		}
		args := append([]string{"--root", inv.stateRoot, "--id", id, "--read", inv.input.text("read")}, actor...)
		args = append(args, inv.forwardEach("add", "item")...)
		if path := inv.input.text("add-file"); path != "" {
			args = append(args, "--items-file", inv.inputPath(path))
		}
		return inv.render(inv.goalAct(id, "notes", func(dependencies syncRequestDependencies) int {
			return runGoalReadItemsAddWithProof(args, proof, inv.owners.commandNow, dependencies)
		}))
	case closing:
		if len(closure) != 1 {
			return inv.refuse(id, "closing a note takes exactly one of --fixed COMMIT, --moved G2 or --accepted REASON; nothing was done", "")
		}
		if inv.input.has("read") {
			return inv.refuse(id, "--read labels added notes; nothing was done", "drop --read")
		}
		actor, proof, problem := inv.actingAs("read-items", id, actorEither)
		if problem != nil {
			return inv.render(*problem)
		}
		args := append([]string{"--root", inv.stateRoot, "--id", id, "--item", inv.input.text("close")}, actor...)
		args = append(args, inv.forward("fixed", "moved", "accepted")...)
		return inv.render(inv.goalAct(id, "notes", func(dependencies syncRequestDependencies) int {
			return runGoalReadItemsCloseWithProof(args, proof, inv.owners.commandNow, dependencies, nil)
		}))
	case len(closure) > 0 || inv.input.has("read"):
		return inv.refuse(id, "names how to add or close a note but not the note; nothing was done", "add --add TEXT or --close ITEM")
	}
	projection, _, problem := inv.projection()
	if problem != nil {
		return inv.render(*problem)
	}
	file, where := goalRecord(projection, id)
	if file == nil {
		return unknownGoal(inv, id)
	}
	items := []goal.ReadItem{}
	var lines []string
	for _, item := range file.ReadItems {
		if !inv.input.switched("all") && item.State != goal.ReadItemOpen {
			continue
		}
		items = append(items, item)
		line := fmt.Sprintf("%s [%s] %s: %s", item.ID, item.State, item.Read, item.Text)
		if item.ClosingReference != "" {
			line += " (" + item.ClosingReference + ")"
		}
		lines = append(lines, line)
	}
	return inv.render(intentResult{Outcome: intentConfirmed, Targets: inv.targets(id), text: lines,
		Summary: fmt.Sprintf("%d note(s) on %s", len(items), id), Data: map[string]any{"where": where, "tip": projection.Tip, "items": items}})
}

// runIntentRecover runs the two recovery owners and reports each result on
// its own: the goal journal's recovery rule, and the durable wait rows.
func runIntentRecover(inv *intentInvocation) int {
	id, problem := inv.singleTarget()
	if problem != nil {
		return inv.render(*problem)
	}
	if problem := inv.selectRoot(); problem != nil {
		return inv.render(*problem)
	}
	journal := map[string]any{}
	var lines []string
	failed := 0
	reports, err := recoverGoalJournal(inv.stateRoot, inv.owners.commandNow, inv.owners.dependencies)
	if err != nil {
		failed++
		journal["outcome"], journal["error"] = intentFailed, err.Error()
		lines = append(lines, "journal: not recovered: "+err.Error())
	} else {
		entries := []map[string]string{}
		for _, report := range reports {
			entries = append(entries, map[string]string{"opid": report.Opid, "action": string(report.Action), "detail": report.Detail})
			lines = append(lines, fmt.Sprintf("journal: %s: %s - %s", report.Opid, report.Action, report.Detail))
		}
		journal["outcome"], journal["entries"] = intentConfirmed, entries
		if len(reports) == 0 {
			journal["outcome"] = intentUnchanged
			lines = append(lines, "journal: clean; nothing to recover")
		}
	}
	waits := map[string]any{}
	session := inv.input.text("session")
	holderProblem := ""
	if session != "" {
		holder, err := lease.CurrentHolder(inv.stateRoot)
		switch {
		case err != nil:
			holderProblem = "the checkout holder cannot be read: " + err.Error()
		case holder.SessionId != session:
			holderProblem = "session " + session + " does not hold this checkout"
		}
	}
	if holderProblem != "" {
		failed++
		waits["outcome"], waits["error"] = intentRefused, holderProblem
		lines = append(lines, "waits: "+holderProblem)
	} else if rows, err := report.CurrentWaitingLines(inv.stateRoot); err != nil {
		failed++
		waits["outcome"], waits["error"] = intentFailed, err.Error()
		lines = append(lines, "waits: unreadable: "+err.Error())
	} else {
		waits["outcome"], waits["continuations"] = intentConfirmed, rows
		if len(rows) == 0 {
			waits["outcome"] = intentUnchanged
			lines = append(lines, "waits: none recorded")
		}
		for _, row := range rows {
			lines = append(lines, "waits: "+row)
		}
	}
	data := map[string]any{"journal": journal, "waits": waits}
	result := intentResult{Outcome: intentConfirmed, text: lines, Data: data, Summary: "recovery ran"}
	if id != "" {
		result.Targets = inv.targets(id)
		after := inv.afterGoalAct(id, "recover")
		data["goal"] = after.Data
		result.text = append(result.text, after.Summary)
	}
	switch failed {
	case 0:
	case 2:
		result.Outcome, result.code, result.Summary = intentFailed, 1, "neither recovery completed"
	default:
		result.Outcome, result.code, result.Summary = intentPartial, 1, "one recovery completed and the other did not"
	}
	return inv.render(result)
}

func runIntentRed(inv *intentInvocation) int {
	if len(inv.input.args) < 2 || inv.input.args[0] != "own" && inv.input.args[0] != "close" {
		return inv.refuse("", "needs own ENTRY --goal G or close ENTRY --reason TEXT; nothing was done", "the entry ids are listed by metasystem goals")
	}
	sub, entry := inv.input.args[0], inv.input.args[1]
	if problem := inv.selectRoot(); problem != nil {
		return inv.render(*problem)
	}
	target := []intentTarget{{Kind: "trunk-red", ID: entry}}
	args := []string{sub, "--root", inv.stateRoot, "--id", entry}
	var proof *humanauthority.Proof
	if sub == "own" {
		if inv.input.has("reason") {
			return inv.refuse("", "--reason closes an incident; owning takes none; nothing was done", "drop --reason")
		}
		if inv.input.text("goal") == "" {
			return inv.refuse("", "owning an incident names the goal fixing it: --goal G; nothing was done", "")
		}
		actorKind := actorEither
		if inv.input.has("to") {
			actorKind = actorHuman
		}
		actor, observed, problem := inv.actingAs("trunk-red own", entry, actorKind)
		if problem != nil {
			return inv.render(*problem)
		}
		proof = observed
		args = append(append(args, actor...), inv.forward("goal", "branch", "to")...)
	} else {
		for _, name := range []string{"goal", "branch", "to"} {
			if inv.input.has(name) {
				return inv.refuse("", "--"+name+" belongs to owning an incident; nothing was done", "drop --"+name)
			}
		}
		reason := strings.TrimSpace(inv.input.text("reason"))
		if reason == "" {
			return inv.refuse("", "closing an incident needs its reason; nothing was done", "say why with --reason TEXT")
		}
		actor, observed, problem := inv.actingAs("trunk-red close", entry, actorHuman)
		if problem != nil {
			return inv.render(*problem)
		}
		proof = observed
		args = append(append(args, actor...), "--why", reason)
	}
	return inv.render(inv.ownerCall(target, func(dependencies syncRequestDependencies) int {
		return runGoalTrunkRedWithDependencies(args, inv.requestBuilder(proof, false), nil, dependencies)
	}, func() intentResult {
		return intentResult{Summary: "trunk red " + entry + ": " + sub + " confirmed", Data: map[string]any{"entry": entry}}
	}))
}
