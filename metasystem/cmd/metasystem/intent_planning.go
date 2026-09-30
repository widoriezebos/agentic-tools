package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/report"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/textui"
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
			object: "goal", action: "open", primary: true, audience: "both", summary: "declare a new goal",
			usage: []string{"metasystem goal open G --intent TEXT --next TEXT --risk ANSWERS --basis TEXT"},
			details: []string{
				"The goal is queued for a person's approval. The four risk answers and their basis are the intake law's classification.",
				"--blocked-by parks the goal until those goals are done; --blocks parks the named goals on it.",
				"Execution follows a person's approval: goal open, then metasystem goal approve G, then metasystem goal claim G.",
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
			examples: []string{"metasystem goal open faster-proof --intent 'Proof runs in half the time.' --next 'Measure the slowest step.' --risk severity=1,novelty=1,exposure=1,accumulation=1 --basis 'local test tooling only'"},
			run:      runIntentOpen,
		},
		{
			object: "goal", action: "edit", audience: "both", summary: "change a goal's intent, next step, risk or labels in place",
			usage: []string{"metasystem goal edit G [--intent TEXT] [--next TEXT | --next-append TEXT]", "metasystem goal edit G --obligation STATE --owner NAME --recurrence VALUE ..."},
			details: []string{
				"Only the supplied fields change. --next-append adds to the next step the accepted ledger holds when the edit is published.",
				"Raising the risk of an approved goal takes --evidence; lowering it is a person's act.",
				"--obligation records a recurring responsibility; provide every required obligation field.",
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
			examples: []string{"metasystem goal edit verbs-match-intent --next 'Land slice 2.'", "metasystem goal edit verbs-match-intent --next-append 'Then review the help pages.'"},
			run:      runIntentEdit,
		},
		{
			object: "goal", action: "claim", audience: "agent", summary: "claim a goal for this session, or the next ready goal",
			usage: []string{"metasystem goal claim [G]", "metasystem goal claim G --take-over --reason TEXT"},
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
			examples: []string{"metasystem goal claim", "metasystem goal claim verbs-match-intent", "metasystem goal claim verbs-match-intent --take-over --reason 'm1b is gone for the day'"},
			run:      runIntentClaim,
		},
		{
			object: "goal", action: "release", audience: "agent", summary: "release a claim this session holds",
			usage:   []string{"metasystem goal release [G] --reason TEXT"},
			details: []string{"Without G the one goal this session holds is released; holding none or several names them instead.", "The reason is recorded on the goal's history line."},
			flags: withFlags([]intentFlag{
				intentTargetFlag,
				{name: "arc", advanced: true, usage: "release the arc's claim"},
				reasonFlag("because", "why the claim is released, recorded on the goal's history"), fileFlag("reason", "read the reason from FILE"),
			}, intentHumanActFlags),
			maxArgs:  1,
			examples: []string{"metasystem goal release --reason 'the design changes first'", "metasystem goal release verbs-match-intent --reason 'handing it to m1b'"},
			run:      runIntentRelease,
		},
		{
			object: "goal", action: "accept-risk", audience: "human", summary: "accept the risk of one severe or unproven review finding",
			usage: []string{"metasystem goal accept-risk G --finding F [--review R] --reason TEXT"},
			details: []string{
				"The review is inferred when the goal's work records exactly one examination; otherwise --review R names it (any job of the review, or human-carried for a carried finding).",
				"Records your risk decision on the goal and applies it to the review; a partial result says what still needs to complete.",
			},
			flags: withFlags([]intentFlag{
				intentTargetFlag,
				{name: "finding", value: "F", usage: "the finding id"},
				{name: "review", aliases: []string{"chain"}, value: "R", usage: "the review job (or its chain root)"},
				reasonFlag("why", "why the risk is accepted"),
				fileFlag("reason", "read the reason from FILE"),
			}, intentHumanActFlags, intentRelayFlags),
			maxArgs:  1,
			examples: []string{"metasystem goal accept-risk verbs-match-intent --finding S-1 --reason 'the exposure is local and reversible'"},
			run:      runIntentAcceptRisk,
		},
		{
			object: "goal", action: "pin", audience: "human", summary: "pin a goal to one machine, or clear its pin",
			usage:    []string{"metasystem goal pin G MACHINE", "metasystem goal pin G --clear"},
			flags:    withFlags([]intentFlag{intentTargetFlag, {name: "clear", usage: "remove the pin"}}, intentHumanActFlags),
			maxArgs:  2,
			examples: []string{"metasystem goal pin verbs-match-intent m1e", "metasystem goal pin verbs-match-intent --clear"},
			run:      runIntentPin,
		},
		{
			object: "goal", action: "prioritize", audience: "human", summary: "place an open goal in priority 1, 2 or 3",
			usage: []string{"metasystem goal prioritize G 1|2|3 [--sequence N]"},
			flags: withFlags([]intentFlag{
				intentTargetFlag,
				{name: "priority", value: "1|2|3", advanced: true, usage: "the priority, as an alternative to naming it after G"},
				{name: "sequence", value: "N", advanced: true, usage: "the one-based position within the priority"},
			}, intentHumanActFlags),
			maxArgs:  2,
			examples: []string{"metasystem goal prioritize verbs-match-intent 1", "metasystem goal prioritize verbs-match-intent 1 --sequence 2"},
			run:      runIntentPrioritize,
		},
		{
			object: "goal", action: "reopen", audience: "both", summary: "return a done or abandoned goal to the queue with a fresh next step",
			usage:    []string{"metasystem goal reopen G --next TEXT"},
			details:  []string{"Reopening an abandoned goal is a person's act. The fresh next step is recorded right after the reopen; a failure there is reported as partial."},
			flags:    withFlags([]intentFlag{intentTargetFlag, {name: "next", value: "TEXT", usage: "the fresh next step"}, fileFlag("next", "read the next step from FILE")}, intentHumanActFlags),
			maxArgs:  1,
			examples: []string{"metasystem goal reopen verbs-match-intent --next 'Cover the fleet commands.'"},
			run:      runIntentReopen,
		},
		{
			object: "goal", action: "abandon", audience: "human", summary: "record that a goal will never be worked, and why",
			usage: []string{"metasystem goal abandon G --reason TEXT [--successor G2]"},
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
			examples: []string{"metasystem goal abandon old-idea --reason 'superseded by the new design' --successor new-idea"},
			run:      runIntentAbandon,
		},
		{
			object: "goal", action: "block", audience: "both", summary: "record that a goal waits for another",
			usage:    []string{"metasystem goal block G --on G2"},
			details:  []string{"G parks until G2 is done, unless G2 is already done."},
			flags:    withFlags([]intentFlag{intentTargetFlag, {name: "on", aliases: []string{"blocker"}, value: "G2", usage: "the goal G waits for"}}, intentHumanActFlags),
			maxArgs:  1,
			examples: []string{"metasystem goal block slice-3 --on slice-2"},
			run:      runIntentBlock,
		},
		{
			object: "goal", action: "unblock", audience: "both", summary: "remove one blocker from a goal",
			usage:    []string{"metasystem goal unblock G --on G2"},
			details:  []string{"Removing a blocker that is not done is a person's act; the park lifts only when every remaining blocker is done."},
			flags:    withFlags([]intentFlag{intentTargetFlag, {name: "on", aliases: []string{"blocker"}, value: "G2", usage: "the goal G no longer waits for"}}, intentHumanActFlags, intentRelayFlags),
			maxArgs:  1,
			examples: []string{"metasystem goal unblock slice-3 --on slice-2"},
			run:      runIntentUnblock,
		},
		{
			object: "goal", action: "unapprove", audience: "human", summary: "withdraw a goal's execution approval",
			usage:    []string{"metasystem goal unapprove G --reason TEXT"},
			details:  []string{"A standing claim is parked with the approval."},
			flags:    withFlags([]intentFlag{intentTargetFlag, reasonFlag("because", "why the approval is withdrawn"), fileFlag("reason", "read the reason from FILE")}, intentHumanActFlags, intentRelayFlags),
			maxArgs:  1,
			examples: []string{"metasystem goal unapprove verbs-match-intent --reason 'the design changes first'"},
			run:      runIntentUnapprove,
		},
		{
			object: "grant", action: "add", audience: "human", summary: "record a power of attorney a seat acts under",
			usage: []string{"metasystem grant add --acts everything --for 24h", "metasystem grant add --tiers LIST --acts LIST --until DATE"},
			details: []string{
				"--acts everything: the main session holding this checkout's lease acts for you in every person's act, bar granting, enrolling and taking the helm, until the end.",
				"Its end is --for 8h, 24h, 7d or 1w (elapsed), or --until 18:00 (today, or tomorrow once passed), --until tomorrow or --until YYYY-MM-DD (the end of that day), at most one week.",
				"Only you at the enrolled terminal grant it. Rebuild every seat's engine before the first: older engines refuse a ledger that holds one.",
				"--acts from approve, budget and resume-parked: a seat then acts with --under GRANT on approve, budget and resume of a parked goal.",
				"There --until is the last day covered, YYYY-MM-DD, at most seven days out. The grant's id is printed.",
			},
			flags: withFlags([]intentFlag{
				{name: "tiers", value: "LIST", usage: "the tiers covered, for example 1 or 1,2"},
				{name: "acts", value: "LIST", usage: "everything, or the acts covered: approve, budget, resume-parked"},
				{name: "for", value: "DURATION", usage: "with everything: how long, 8h, 24h, 7d or 1w"},
				{name: "until", aliases: []string{"expires"}, value: "DATE", usage: "the end: HH:MM, tomorrow or YYYY-MM-DD"},
			}, intentHumanActFlags, intentRelayFlags),
			maxArgs:  0,
			examples: []string{"metasystem grant add --acts everything --for 24h", "metasystem grant add --acts everything --until 18:00", "metasystem grant add --tiers 1 --acts approve,budget --until 2026-10-01"},
			run:      runIntentGrant,
		},
		{
			object: "grant", action: "revoke", audience: "human", summary: "close a power of attorney early",
			usage:    []string{"metasystem grant revoke GRANT"},
			flags:    withFlags([]intentFlag{{name: "grant", value: "GRANT", advanced: true, usage: "the grant, as an alternative to naming it first"}}, intentHumanActFlags, intentRelayFlags),
			maxArgs:  1,
			examples: []string{"metasystem grant revoke 01ARZ3NDEKTSV4RRFFQ69G5FAV-mac-cli-m1"},
			run:      runIntentRevoke,
		},
		{
			object: "goal", action: "split", audience: "both", summary: "split a goal into independently claimable related goals",
			usage: []string{"metasystem goal split G --plan FILE"},
			details: []string{"The parent concludes as decomposed and its members become a group of related goals. The same goal rules apply",
				"to each member; splitting a person's goal is a person's act at the enrolled terminal. FILE, for goal big-goal:",
				"  # split big-goal",
				"  ## member first",
				"  - Intent: Build the reader.",
				"  - Next step: Write the reader's brief.",
				"  ## member second",
				"  - Intent: Build the writer.",
				"  - Next step: Write the writer's brief.",
				"  - BlockedBy: first",
				"  - Labels: io, writer",
				"BlockedBy and Labels are optional comma-separated lists."},
			flags:    withFlags([]intentFlag{intentTargetFlag, {name: "plan", aliases: []string{"members"}, value: "FILE", usage: "the member draft"}}, intentHumanActFlags),
			maxArgs:  1,
			examples: []string{"metasystem goal split big-goal --plan members.md"},
			run:      runIntentSplit,
		},
		{
			object: "goal", action: "group", audience: "both", summary: "put a goal into a group of related goals",
			usage:    []string{"metasystem goal group G GROUP"},
			details:  []string{"GROUP names the group of related goals, usually the goal they serve; a claim of the group covers its members."},
			flags:    withFlags([]intentFlag{intentTargetFlag, {name: "arc", value: "GROUP", advanced: true, usage: "the group, as an alternative to naming it after G"}}, intentHumanActFlags),
			maxArgs:  2,
			examples: []string{"metasystem goal group slice-2 verbs-match-intent"},
			run:      runIntentGroup,
		},
		{
			object: "goal", action: "ungroup", audience: "both", summary: "take a goal out of its group of related goals",
			usage:    []string{"metasystem goal ungroup G"},
			details:  []string{"A claim the goal held only through its group is released."},
			flags:    withFlags([]intentFlag{intentTargetFlag}, intentHumanActFlags),
			maxArgs:  1,
			examples: []string{"metasystem goal ungroup slice-2"},
			run:      runIntentUngroup,
		},
		{
			object: "goal", action: "allow", audience: "human", summary: "allow a goal something it may not do by default",
			usage: []string{"metasystem goal allow G PERMISSION --reason TEXT"},
			details: []string{
				"PERMISSION is " + strings.Join(goal.PermissionNames(), ", ") + ".",
				"stop-test-changes lets the goal's landing move or change a test assertion that says whether work must stop, under a declaration the Stop decision audit checks.",
				"Allowing is a person's act at the enrolled terminal; goal show lists what a goal is allowed.",
			},
			flags: withFlags([]intentFlag{
				intentTargetFlag,
				reasonFlag("why", "why the goal is allowed it"), fileFlag("reason", "read the reason from FILE"),
			}, intentHumanActFlags, intentRelayFlags),
			maxArgs:  2,
			examples: []string{"metasystem goal allow verbs-match-intent stop-test-changes --reason 'the Stop hook runs the engine directly'"},
			run:      runIntentAllow,
		},
		{
			object: "goal", action: "disallow", audience: "both", summary: "withdraw something a goal was allowed",
			usage:    []string{"metasystem goal disallow G PERMISSION [--reason TEXT]"},
			details:  []string{"PERMISSION is " + strings.Join(goal.PermissionNames(), ", ") + ". Anyone may withdraw a permission."},
			flags:    withFlags([]intentFlag{intentTargetFlag, reasonFlag("why", "why the permission is withdrawn"), fileFlag("reason", "read the reason from FILE")}, intentHumanActFlags),
			maxArgs:  2,
			examples: []string{"metasystem goal disallow verbs-match-intent stop-test-changes"},
			run:      runIntentDisallow,
		},
		{
			object: "goal", action: "notes", audience: "both", summary: "read, add or close a goal's non-breaking read findings",
			usage: []string{"metasystem goal notes G", "metasystem goal notes G --read LABEL --add TEXT...", "metasystem goal notes G --close ITEM --fixed COMMIT|--moved G2|--accepted REASON"},
			details: []string{
				"Notes are non-breaking read items. A finding stays a finding; closing a note never certifies one.",
				"--all lists closed items too.",
			},
			flags: withFlags([]intentFlag{
				intentTargetFlag,
				{name: "all", usage: "include closed items"},
				{name: "add", value: "TEXT", repeat: true, usage: "an item to record (repeatable)"},
				{name: "add-file", value: "FILE", advanced: true, usage: "record each line of FILE as its own note, not one note"},
				{name: "read", value: "LABEL", usage: "with --add: the read the items came from"},
				{name: "close", value: "ITEM", usage: "the item to close"},
				{name: "fixed", value: "COMMIT", usage: "with --close: the commit that fixed it"},
				{name: "moved", value: "G2", usage: "with --close: the open goal receiving it"},
				{name: "accepted", aliases: []string{"reason"}, value: "REASON", usage: "with --close: why it is not a defect"},
				fileFlag("accepted", "with --close: read why it is not a defect from FILE"),
			}, intentHumanActFlags),
			maxArgs:  1,
			examples: []string{"metasystem goal notes verbs-match-intent", "metasystem goal notes verbs-match-intent --read r2 --add 'help wraps at 80 columns'"},
			run:      runIntentNotes,
		},
		{
			object: "goal", action: "sync", audience: "both", summary: "bring the goal files and the published ledger into agreement",
			usage: []string{"metasystem goal sync", "metasystem goal sync --recover", "metasystem goal sync --refresh"},
			administrationUsage: []string{
				"metasystem goal sync --publish --goal G... --by NAME",
				"metasystem goal sync --accept-remote-history --by NAME",
				"metasystem goal sync --upgrade --source-digest SHA256 --by NAME [--amendments FILE] [--identity ULID] [--sync-mode remote|local]",
			},
			details: []string{
				"Without an option it previews and changes nothing: the goal files that differ from their published base.",
				"--recover completes goal changes that were interrupted across this installation; work that is still running is left alone.",
				"--refresh completes an interrupted refresh of the published view without reading any edit as new authority.",
				"--publish publishes the reviewed hand edits of exactly the goals named with --goal; when the edits it captures also",
				"touch another goal the whole publication is refused, naming it. Edits that need a person's proof ask for it.",
				"--accept-remote-history accepts the fetched current history of the same ledger locally after a rewind; nothing is pushed.",
				"--upgrade converts the legacy plans/goals.md under its reviewed SHA-256: without --source-digest it only shows the",
				"digest to review. --amendments FILE starts with 'MIGRATION_EPOCH: <RFC3339>' and 'REVIEWED_SOURCE_SHA256: <sha256>', then",
				"'### add-goal: ID' sections (intent, origin, next; optional blockedby, arc) or '### amend-goal: ID' sections (next,",
				"blockedby, arc, state); a parked amendment also gives parked-by, parked-at (timestamp or EPOCH) and parked-because.",
			},
			flags: []intentFlag{
				{name: "recover", usage: "complete interrupted goal changes"},
				{name: "refresh", usage: "complete an interrupted refresh"},
				{name: "publish", usage: "publish the reviewed hand edits of the goals --goal names"},
				{name: "goal", value: "G", repeat: true, usage: "with --publish: a goal whose edits publish (repeatable)"},
				{name: "accept-remote-history", usage: "accept the fetched history of the same ledger"},
				{name: "upgrade", usage: "convert the legacy goals file"},
				{name: "source-digest", value: "SHA256", usage: "--upgrade: the reviewed file's SHA-256"},
				{name: "amendments", value: "FILE", usage: "--upgrade: the amendment file"},
				{name: "identity", value: "ULID", advanced: true, usage: "--upgrade: the ledger identity (an existing one is kept on a rerun)"},
				{name: "sync-mode", value: "MODE", advanced: true, usage: "--upgrade: remote (default) or local"},
				{name: "by", value: "NAME", usage: "the person deciding"},
			},
			maxArgs:  0,
			examples: []string{"metasystem goal sync", "metasystem goal sync --recover", "metasystem goal sync --publish --goal verbs-match-intent --by Wido"},
			run:      runIntentGoalSync,
		},
		{
			object: "grant", action: "list", audience: "both", laidOut: true, summary: "the recorded powers of attorney, live and closed",
			usage:    []string{"metasystem grant list [--all]"},
			flags:    []intentFlag{{name: "all", usage: "include revoked and expired grants"}},
			maxArgs:  0,
			examples: []string{"metasystem grant list"},
			run:      runIntentGrantList,
		},
		{
			object: "incident", action: "list", audience: "both", laidOut: true, summary: "the broken-main incidents",
			usage:    []string{"metasystem incident list [--all]"},
			details:  []string{"An incident is a failure on main that someone must own."},
			flags:    []intentFlag{{name: "all", usage: "include closed incidents"}},
			maxArgs:  0,
			examples: []string{"metasystem incident list"},
			run:      runIntentIncidents,
		},
		{
			object: "incident", action: "claim", audience: "both", summary: "take on an incident with the goal whose work fixes it",
			usage:   []string{"metasystem incident claim I --goal G"},
			details: []string{"An agent's act; a person may assign it to another machine with --by NAME --to MACHINE."},
			flags: withFlags([]intentFlag{
				{name: "goal", value: "G", usage: "the goal fixing it"},
				{name: "branch", value: "NAME", advanced: true, usage: "the fix branch"},
				{name: "to", value: "MACHINE", advanced: true, usage: "with --by: the machine assigned the fix"},
			}, intentHumanActFlags),
			maxArgs:  1,
			examples: []string{"metasystem incident claim tr-01 --goal fix-trunk"},
			run:      func(inv *intentInvocation) int { return runIntentIncidentAct(inv, "own") },
		},
		{
			object: "incident", action: "close", audience: "human", summary: "close an incident with its reason",
			usage:    []string{"metasystem incident close I --reason TEXT"},
			details:  []string{"Closing an incident is a person's act."},
			flags:    withFlags([]intentFlag{reasonFlag("why", "why the incident is closed"), fileFlag("reason", "read the reason from FILE")}, intentHumanActFlags),
			maxArgs:  1,
			examples: []string{"metasystem incident close tr-01 --reason 'the flake is fixed at its source'"},
			run:      func(inv *intentInvocation) int { return runIntentIncidentAct(inv, "close") },
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
	// actorEitherStopping is actorEither for a stopping act (release): a
	// person at a terminal that is not enrolled names themself with --by and
	// acts under the terminal-grade proof the owner accepts for stopping
	// acts (rule H1: a human is never denied a verb).
	actorEitherStopping
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
			Summary: fmt.Sprintf("%s is done by the session holding the goal, not by a named person; nothing was done", verb),
			next:    withoutOption(inv.typedArgv(), "by"), nextReason: "from the session that holds the goal"}
	}
	stopping := actor == actorEitherStopping
	if stopping {
		actor = actorEither
	}
	if typed == "" && actor == actorEither && agent && !inv.input.has("lineage") {
		// A live general grant that admits this session makes a dual act
		// the granting person's, before the session's own shortcut (M6),
		// as pause and done already do; the owner proves it again.
		if by, ok := inv.attorneyActor(); ok {
			return append(args, "--by", by), nil, nil
		}
	}
	if typed == "" && (actor == actorAgent || actor == actorEither && agent) {
		if !agent {
			return nil, nil, &intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(target),
				Summary: "no session is named, so nothing was done",
				next:    append(inv.typedArgv(), "--lineage", "LINEAGE"), nextReason: "as the session that holds the work",
				Details: []string{"a session's launcher names it in METASYSTEM_OWNER_LINEAGE; metasystem session start prepares one"}}
		}
		return args, nil, nil
	}
	flags := &syncFlags{root: inv.stateRoot, fixtureHumanAuthority: inv.input.switched("fixture-human-authority"),
		temporaryWord: inv.input.text("temporary-human-word"), reviewBy: inv.input.text("review-by")}
	proof, err := proveGoalHumanAuthorityAt(verb, flags, inv.owners.prove, inv.owners.commandNow)
	mismatch := false
	switch {
	case err != nil:
	case flags.temporaryWord != "":
		// A recorded relayed word names the person it relays.
		if typed == "" {
			return nil, nil, &intentResult{Outcome: intentRefused, code: 1, Targets: inv.targets(target),
				Summary: "a relayed word needs the name of the person whose word it is, so nothing was done",
				next:    append(inv.typedArgv(), "--by", "NAME"), nextReason: "with the name of the person whose word it is"}
		}
		flags.by = typed
	default:
		if err = resolveGoalHuman(flags, proof); err == nil && typed != "" && typed != flags.by {
			mismatch = true
		}
	}
	if mismatch {
		return nil, nil, &intentResult{Outcome: intentRefused, code: 1, Targets: inv.targets(target),
			Summary: fmt.Sprintf("this terminal is enrolled for %s, not %s, so nothing was done", flags.by, typed),
			next:    withoutOption(inv.typedArgv(), "by"), nextReason: "the enrolled name is filled in"}
	}
	if err != nil && stopping && typed != "" && flags.temporaryWord == "" {
		// The owner proves the terminal itself when the request is built.
		return append(args, "--by", typed), nil, nil
	}
	if err != nil {
		if actor == actorEither && typed == "" {
			return nil, nil, inv.eitherRefusal(target, err, stopping)
		}
		return nil, nil, inv.personRefusal(target, err, typed)
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
			Summary: fmt.Sprintf("--%s and --%s-file say different things; nothing was done", name, name),
			next:    inv.typedArgvLess(name), nextReason: "keeps the text of --" + name + "-file"}
	}
	return text, nil
}

// readTextFile reads one FILE option's text exactly, less its final line
// ends, resolved against the directory the command was started in.
func (inv *intentInvocation) readTextFile(option, path string) (string, *intentResult) {
	data, err := os.ReadFile(inv.inputPath(path))
	if err != nil {
		return "", &intentResult{Outcome: intentRefused, code: 2, Summary: fmt.Sprintf("--%s: %s; nothing was done", option, fileProblem("file", inv.inputPath(path), err)),
			next: inv.typedArgv(), nextReason: "once --" + option + " names a readable file", Details: []string{err.Error()}}
	}
	return strings.TrimRight(string(data), "\r\n"), nil
}

// fileProblem names a file that cannot be read by its path, in plain words:
// absent, or unreadable with the reason, never the system call's own text.
func fileProblem(what, path string, err error) string {
	if errors.Is(err, fs.ErrNotExist) {
		return "no " + what + " at " + path
	}
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		return what + " at " + path + " cannot be read: " + pathErr.Err.Error()
	}
	return what + " at " + path + " cannot be read: " + err.Error()
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
	longNames := make([]string, 0, len(intentLongBudgetFlags))
	for _, definition := range intentLongBudgetFlags {
		longNames = append(longNames, definition.name)
	}
	var given, missing, missingValues []string
	values := map[string]string{}
	for _, definition := range intentLongBudgetFlags {
		if inv.input.has(definition.name) {
			given = append(given, "--"+definition.name)
			values[definition.name] = inv.input.text(definition.name)
		} else {
			missing = append(missing, "--"+definition.name)
			missingValues = append(missingValues, "--"+definition.name, definition.value)
		}
	}
	switch {
	case len(given) == 0:
		return compact, nil
	case compact != "":
		return "", &intentResult{Outcome: intentRefused, code: 2,
			Summary: fmt.Sprintf("the budget is given twice, as %s and as %s; nothing was done", shellCommand([]string{compact}), strings.Join(given, " ")),
			next:    inv.typedArgvLess(longNames...), nextReason: "keeps the compact budget"}
	case len(missing) > 0:
		return "", &intentResult{Outcome: intentRefused, code: 2,
			Summary: fmt.Sprintf("the budget needs all five limits, and %s are missing; nothing was done", strings.Join(missing, ", ")),
			next:    append(inv.typedArgv(), missingValues...), nextReason: "with the missing limits filled in"}
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
	inv.budgetTargetFirst()
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
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "--pretty only formats --json output; nothing was read",
			next: inv.typedArgvWith("--json"), nextReason: "adds --json"})
	}
	if ready || tiers {
		filters := []string{"history"}
		if tiers {
			filters = append(filters, "label", "machine")
		}
		for _, name := range filters {
			if inv.input.has(name) && (name != "history" || inv.input.switched(name)) {
				return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: fmt.Sprintf("--%s does not apply to this view; nothing was read", name),
					next: inv.typedArgvLess(name), nextReason: "without --" + name})
			}
		}
	}
	if !ready && !tiers {
		if inv.input.has("machine") {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "--machine only picks whose ready goals --ready shows; nothing was read",
				next: inv.typedArgvWith("--ready"), nextReason: "adds --ready"})
		}
		return runIntentGoals(inv)
	}
	if ready && tiers || inv.input.switched("all") {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "--ready, --tiers and --all are different views; nothing was read",
			next: append(inv.typedArgvLess("tiers", "all"), "--ready"), nextReason: "or keep --tiers or --all alone instead"})
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
		return inv.render(intentResult{Outcome: intentConfirmed, text: lines, view: goalTiersView(probe, recorded, derived),
			Summary: fmt.Sprintf("%d open goal(s) with a risk record at %s", probe.Open, projection.Tip),
			Data: map[string]any{"tip": projection.Tip, "open": probe.Open, "recorded": probe.Recorded, "derived": probe.Derived,
				"tier3ShareRecorded": recorded, "tier3ShareDerived": derived, "lowerable": probe.Lowerable}})
	}
	labels := inv.input.values["label"]
	if err := goal.ValidateLabels(labels); err != nil {
		return inv.render(intentResult{Outcome: intentRefused, code: 2,
			Summary: "a label is a lowercase word of up to 32 letters, digits and dashes; nothing was read",
			next:    inv.typedArgvLess("label"), nextReason: "without --label, or with a label of that shape", Details: []string{err.Error()}})
	}
	machine := inv.input.text("machine")
	var err error
	if machine == "" {
		machine, err = inv.owners.dependencies.machine(inv.stateRoot)
	} else {
		err = goal.ValidateMachineNickname(machine)
	}
	if err != nil {
		return inv.render(intentResult{Outcome: intentRefused, code: 1, Summary: "this machine's name can't be told, so nothing was read",
			next: append(inv.typedArgvLess("machine"), "--machine", "NAME"), nextReason: "metasystem machine list names the machines",
			Details: []string{err.Error()}})
	}
	frontier, err := goal.Next(projection, machine, labels...)
	if err != nil {
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: "the ready goals can't be worked out, so nothing was read",
			next: inv.typedArgv(), nextReason: "try again; --verbose shows the cause", Details: []string{err.Error()}})
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
		result.next, result.nextReason = inv.publicArgv("goal", "claim", selection.GoalID), "claim it"
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
	for name, value := range map[string]string{"--intent": intent, "--next": next, "--risk": inv.input.text("risk"), "--basis": inv.input.text("basis")} {
		if strings.TrimSpace(value) == "" {
			missing = append(missing, name)
		}
	}
	slices.Sort(missing)
	if strings.TrimSpace(id) == "" {
		missing = append([]string{"a goal id"}, missing...)
	}
	if len(missing) > 0 {
		retry := inv.typedArgv()
		if strings.TrimSpace(id) == "" {
			retry = inv.typedArgvFor("GOAL")
		}
		placeholders := map[string]string{"--intent": "TEXT", "--next": "TEXT", "--risk": "severity=N,novelty=N,exposure=N,accumulation=N", "--basis": "TEXT"}
		for _, name := range missing {
			if value, ok := placeholders[name]; ok {
				retry = append(retry, name, value)
			}
		}
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(id),
			Summary: fmt.Sprintf("a new goal needs %s; nothing was done", strings.Join(missing, ", ")),
			next:    retry, nextReason: "the risk answers and their basis are your judgement of this goal, never a default",
			Data: map[string]any{"missing": missing}})
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
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(id),
				Summary: fmt.Sprintf("%s only go with --obligation; nothing was done", strings.Join(obligation, " ")),
				next:    inv.typedArgvWith("--obligation", "DRAFT"), nextReason: "or OBSERVE, LIMITED or ENFORCED, with every obligation field"})
		}
		if len(edits) > 0 {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(id),
				Summary: fmt.Sprintf("an obligation and an edit of %s are two separate commands; nothing was done", strings.Join(edits, " ")),
				next:    inv.typedArgvLess(editFields...), nextReason: "binds the obligation; then run the edit on its own"})
		}
		return inv.bindObligation(id)
	}
	if inv.input.has("approved-ref") {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(id),
			Summary: "--approved-ref only goes with --obligation; nothing was done",
			next:    inv.typedArgvLess("approved-ref"), nextReason: "the edit without it"})
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
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(id),
			Summary: "--next replaces the next step and --next-append adds to it; give one; nothing was done",
			next:    inv.typedArgvLess("next-append", "next-append-file"), nextReason: "keeps --next"})
	}
	if len(edits) == 0 {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(id),
			Summary: "nothing to change was named; nothing was done",
			next:    inv.typedArgvWith("--next", "TEXT"), nextReason: "or --intent, --next-append, --risk with --basis, --tier, --label, --unlabel"})
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
		return nil, "", errors.New("no session is named: the command has no --lineage and the shell names no session")
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
			Summary: "no goal is named and no session holding one is known, so nothing was done",
			next:    inv.typedArgvFor("GOAL"), nextReason: "names the goal", Details: []string{err.Error()}}), false
	case len(held) == 1:
		return held[0], 0, true
	case len(held) == 0:
		return "", inv.render(intentResult{Outcome: intentRefused, code: 1,
			Summary: fmt.Sprintf("no goal is named and this session holds none on %s; nothing was done", machine),
			next:    inv.typedArgvFor("GOAL"), nextReason: "names the goal"}), false
	}
	return "", inv.render(intentResult{Outcome: intentRefused, code: 2,
		Summary: fmt.Sprintf("this session holds %d goals (%s); nothing was done", len(held), strings.Join(held, " ")),
		next:    inv.typedArgvFor(held[0]), nextReason: "or name another of them", Data: map[string]any{"candidates": held}}), false
}

func runIntentClaim(inv *intentInvocation) int {
	if inv.input.switched("take-over") {
		return inv.takeOver()
	}
	if inv.input.has("reason") {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "--reason is only for taking over another machine's claim; nothing was done",
			next: inv.typedArgvLess("reason"), nextReason: "an ordinary claim; add --take-over instead to take a claim over"})
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
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "--arc needs the goal whose arc to claim; nothing was done",
				next: inv.typedArgvFor("GOAL"), nextReason: "names the goal"})
		}
		machine, err := inv.owners.dependencies.machine(inv.stateRoot)
		if err != nil {
			return inv.render(intentResult{Outcome: intentRefused, code: 1, Summary: "this machine's name can't be told, so no ready goal could be picked; nothing was done",
				next: inv.typedArgvFor("GOAL"), nextReason: "names the goal to claim", Details: []string{err.Error()}})
		}
		frontier, err := goal.Next(projection, machine, inv.input.values["label"]...)
		if err != nil {
			return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: "the ready goals can't be worked out, so nothing was claimed",
				next: inv.typedArgvFor("GOAL"), nextReason: "names the goal to claim", Details: []string{err.Error()}})
		}
		selection := goal.SelectNext(frontier)
		switch selection.Kind {
		case goal.NextSelectionContinue:
			// Held work is continued, never switched for the frontier's next goal.
			return inv.render(intentResult{Outcome: intentUnchanged, Targets: inv.targets(selection.GoalID),
				Summary: fmt.Sprintf("%s already holds %s; continue it (a claim never switches held work)", machine, selection.GoalID),
				next:    inv.publicArgv("goal", "show", selection.GoalID), nextReason: "the held goal and its next step",
				Data: map[string]any{"machine": machine, "selection": selection}})
		case goal.NextSelectionReady:
			id = selection.GoalID
		default:
			return inv.render(intentResult{Outcome: intentRefused, code: 1,
				Summary: fmt.Sprintf("no goal is ready for %s; nothing was claimed", machine),
				next:    inv.publicArgv("goal", "list"), nextReason: "a goal is ready once a person approves it",
				Data: map[string]any{"machine": machine, "frontier": frontier}})
		}
	} else if inv.input.has("label") {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(id), Summary: "--label picks among ready goals, and a goal is named; nothing was done",
			next: inv.typedArgvLess("label"), nextReason: "claims the named goal"})
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

// acquireClaim claims one named goal for this session exactly as claim G
// does, and returns the owner's result.
func (inv *intentInvocation) acquireClaim(id string) intentResult {
	actor, proof, problem := inv.actingAs("claim", id, actorEither)
	if problem != nil {
		return *problem
	}
	args := append([]string{"--root", inv.stateRoot, "--id", id}, actor...)
	return inv.goalAct(id, "claim", inv.syncOwner("claim", args, proof, false, func(req goal.VerbRequest, f *syncFlags) (goal.PublishResult, error) {
		return goal.Claim(req, f.id)
	}, "id"))
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
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(id), Summary: "taking over another machine's claim needs a reason; nothing was done",
			next: inv.typedArgvWith("--reason", "TEXT"), nextReason: "TEXT says why"})
	}
	for _, name := range []string{"arc", "budget", "label"} {
		if inv.input.has(name) {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(id), Summary: "--" + name + " does not apply to a take-over; nothing was done",
				next: inv.typedArgvLess(name), nextReason: "without --" + name})
		}
	}
	for _, definition := range intentLongBudgetFlags {
		if inv.input.has(definition.name) {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(id), Summary: "--" + definition.name + " does not apply to a take-over; nothing was done",
				next: inv.typedArgvLess(definition.name), nextReason: "without --" + definition.name})
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
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(id), Summary: "a release needs a reason; nothing was done",
			next: inv.typedArgvWith("--reason", "TEXT"), nextReason: "TEXT says why the goal is released"})
	}
	actor, proof, problem := inv.actingAs("release", id, actorEitherStopping)
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

// runIntentQueueOnly is land G --queue-only: exactly the land-ready act. A
// refusal because this machine's landing slot is taken names the public
// landing of the goal that holds it, read from the goal ledger.
func runIntentQueueOnly(inv *intentInvocation, id string) int {
	if problem := inv.selectRoot(); problem != nil {
		return inv.render(*problem)
	}
	result := inv.landReady(id)
	if result.Outcome != intentRefused || len(result.next) > 0 {
		return inv.render(result)
	}
	projection, _, problem := inv.projection()
	if problem != nil {
		return inv.render(result)
	}
	mine := goalRecordClaim(projection, id)
	var waiting []string
	for _, other := range sortedLiveIDs(projection) {
		file := projection.Tree.Live[other]
		if other == id || file.State != goal.StateClaimed || file.Claimed == nil || file.Landing == nil || file.Claimed.HandedOver != (goal.HandedOver{}) {
			continue
		}
		if mine == "" || file.Claimed.Machine == mine {
			waiting = append(waiting, other)
		}
	}
	if len(waiting) == 1 {
		result.Summary = fmt.Sprintf("%s; this machine's one landing slot holds goal %s", strings.TrimSpace(result.Summary), waiting[0])
		result.next, result.nextReason = []string{"metasystem", "work", "land", waiting[0]}, "land the goal waiting in the slot first, then queue this one again"
		result.Decision = ""
	}
	return inv.render(result)
}

func (inv *intentInvocation) landReady(id string) intentResult {
	actor, proof, problem := inv.actingAs("ready", id, actorAgent)
	if problem != nil {
		return *problem
	}
	args := append([]string{"--root", inv.stateRoot, "--id", id}, actor...)
	return inv.goalAct(id, "ready", inv.syncOwner("land-ready", args, proof, false, func(req goal.VerbRequest, f *syncFlags) (goal.PublishResult, error) {
		return goal.LandReady(req, f.id)
	}, "id"))
}

// goalRecordClaim is the machine holding a live goal's claim, if any.
func goalRecordClaim(projection goal.Projection, id string) string {
	if file := projection.Tree.Live[id]; file != nil && file.Claimed != nil {
		return file.Claimed.Machine
	}
	return ""
}

func sortedLiveIDs(projection goal.Projection) []string {
	ids := make([]string, 0, len(projection.Tree.Live))
	for id := range projection.Tree.Live {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

// reviewRoot is the chain root of a named review job, read from its record.
func (inv *intentInvocation) reviewRoot(id, review string) (string, *intentResult) {
	if review == goal.HumanCarriedChain {
		return review, nil
	}
	root, err := dispatchcore.ChainRootOf(inv.stateRoot, review)
	if err != nil {
		return "", &intentResult{Outcome: intentRefused, code: 1, Targets: inv.targets(id),
			Summary: fmt.Sprintf("no review %s of this goal can be read; nothing was done", shellCommand([]string{review})),
			next:    inv.publicArgv("status", id), nextReason: "shows the goal's work and its reviews", Details: []string{err.Error()}}
	}
	return root, nil
}

// runIntentAcceptRisk is the accept-risk act: the review is inferred only
// when the goal's work records exactly one examination.
func runIntentAcceptRisk(inv *intentInvocation) int {
	if !inv.input.has("review") && len(inv.input.args) == 1 {
		if problem := inv.selectRoot(); problem != nil {
			return inv.render(*problem)
		}
		chain, problem := inv.uniqueExamination(inv.input.args[0], "accept-risk")
		if problem != nil {
			return inv.render(*problem)
		}
		inv.inferredChain = chain
		inv.input.values["review"] = []string{chain}
	}
	return runIntentDecide(inv)
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
		retry := inv.typedArgv()
		for _, name := range missing {
			retry = append(retry, name, map[string]string{"--finding": "FINDING", "--review": "REVIEW", "--reason": "TEXT"}[name])
		}
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(id),
			Summary: fmt.Sprintf("accepting a risk needs %s; nothing was done", strings.Join(missing, ", ")),
			next:    retry, nextReason: "the finding, its review and why its risk is accepted"})
	}
	chain := inv.inferredChain
	if chain == "" {
		var problem *intentResult
		if chain, problem = inv.reviewRoot(id, inv.input.text("review")); problem != nil {
			return inv.render(*problem)
		}
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
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(id), Summary: fmt.Sprintf("machine %s and --clear contradict each other; nothing was done", shellCommand([]string{machine})),
			next: inv.typedArgvLess("clear"), nextReason: "pins the goal to " + machine + "; or clear the pin with --clear alone"})
	case clear:
		machine = "-"
	case machine == "" || machine == "-":
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(id), Summary: "no machine is named to pin the goal to; nothing was done",
			next: inv.publicArgv("goal", "pin", id, "MACHINE"), nextReason: "metasystem machine list names the machines; --clear removes a pin"})
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
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(id), Summary: fmt.Sprintf("two priorities are named, %s and --priority %s; nothing was done", inv.input.args[1], priority),
				next: inv.typedArgvLess("priority"), nextReason: "keeps " + inv.input.args[1]})
		}
		priority = inv.input.args[1]
	}
	if priority != "1" && priority != "2" && priority != "3" {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(id), Summary: fmt.Sprintf("the priority is 1, 2 or 3, not %s; nothing was done", shellCommand([]string{priority})),
			next: inv.publicArgv("goal", "prioritize", id, "1"), nextReason: "or 2 or 3; 1 is the highest"})
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
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(id), Summary: "a reopened goal needs its next step; nothing was done",
			next: inv.typedArgvWith("--next", "TEXT"), nextReason: "TEXT says what happens next"})
	}
	projection, _, problem := inv.projection()
	if problem != nil {
		return inv.render(*problem)
	}
	file, where := goalRecord(projection, id)
	switch {
	case file == nil:
		return unknownGoal(inv, id)
	case where == "live" && file.NextStep == next:
		// Open with this next step is what a reopen asks for: a repeat,
		// success with no record (R-129-ui, U-idem).
		return inv.render(intentResult{Outcome: intentUnchanged, Targets: inv.targets(id),
			Summary: fmt.Sprintf("%s is already open (it is %s) with that next step", id, file.State)})
	case where == "live":
		return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: inv.targets(id),
			Summary: fmt.Sprintf("%s is %s, not done or abandoned; nothing was done", id, file.State),
			next:    inv.publicArgv("goal", "edit", id, "--next", next), nextReason: "a live goal's next step is edited"})
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
		inv.publicArgv("goal", "edit", id, "--next", next), "record the next step on the reopened goal"))
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
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(id), Summary: "abandoning a goal needs a reason; nothing was done",
			next: inv.typedArgvWith("--reason", "TEXT"), nextReason: "TEXT says why it will never be worked"})
	}
	successor := inv.input.text("successor")
	if successor == id {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(id), Summary: "a goal cannot be its own successor; nothing was done",
			next: append(inv.typedArgvLess("successor"), "--successor", "GOAL"), nextReason: "GOAL is the live goal that carries the work on"})
	}
	actor, _, problem := inv.actingAs("abandon", id, actorHuman)
	if problem != nil {
		return inv.render(*problem)
	}
	projection, _, problem := inv.projection()
	if problem != nil {
		return inv.render(*problem)
	}
	if file, where := goalRecord(projection, id); file != nil && where == "abandoned" && successor != "" {
		// Recovery of an abandonment that recorded no successor: the
		// original abandonment and its reason stand; the carry owner records
		// the successor under a person's fresh authority.
		if file.Abandoned != nil && file.Abandoned.Carried == successor {
			return inv.render(intentResult{Outcome: intentUnchanged, Targets: inv.targets(id, successor),
				Summary: fmt.Sprintf("%s is already abandoned and carried by %s; nothing was done", id, successor)})
		}
		carryArgs := append([]string{"--root", inv.stateRoot, "--id", id, "--to", successor}, actor...)
		carried := inv.ownerCall(inv.targets(id, successor), func(dependencies syncRequestDependencies) int {
			return runGoalCarryAbandonedWithInputs(carryArgs, inv.owners.prove, inv.owners.commandNow, dependencies)
		}, func() intentResult { return inv.afterGoalAct(id, "abandon") })
		if carried.Outcome == intentConfirmed {
			carried.Summary = fmt.Sprintf("%s was already abandoned; %s now carries its work", id, successor)
		}
		return inv.render(carried)
	}
	// A fresh abandonment names its successor in the same transaction, which
	// archives the goal and repoints its live dependents to the successor.
	args := append([]string{"--root", inv.stateRoot, "--id", id, "--because", reason}, actor...)
	if successor != "" {
		args = append(args, "--carried", successor)
	}
	args = append(args, inv.forwardEach("waive", "waive")...)
	args = append(args, inv.forwardEach("also", "also")...)
	abandoned := inv.goalAct(id, "abandon", func(dependencies syncRequestDependencies) int {
		return runGoalAbandonWithInputs(args, inv.owners.prove, inv.owners.commandNow, dependencies)
	})
	if abandoned.Outcome == intentConfirmed && successor != "" {
		abandoned.Summary = fmt.Sprintf("abandoned %s; %s carries its work", id, successor)
		summary := abandoned.Summary
		abandoned.view = doneView(summary)
	}
	return inv.render(abandoned)
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
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(id), Summary: "no other goal is named with --on; nothing was done",
			next: inv.typedArgvWith("--on", "GOAL"), nextReason: "GOAL is the other goal"})
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
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(id), Summary: "withdrawing an approval needs a reason; nothing was done",
			next: inv.typedArgvWith("--reason", "TEXT"), nextReason: "TEXT says why"})
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
	if isGeneralActs(inv.input.text("acts")) {
		return runIntentGrantEverything(inv)
	}
	if inv.input.has("for") {
		tomorrow := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "a grant of named acts ends on a date (--until), not after --for; nothing was done",
			next: append(withoutOption(inv.typedArgv(), "for"), "--until", tomorrow), nextReason: "--for is for --acts everything",
			Details: []string{grantEndExamples}})
	}
	var missing []string
	for _, name := range []string{"tiers", "acts", "until"} {
		if inv.input.text(name) == "" {
			missing = append(missing, "--"+name)
		}
	}
	if len(missing) > 0 {
		examples := map[string]string{"--tiers": "1", "--acts": "approve,budget", "--until": time.Now().AddDate(0, 0, 1).Format("2006-01-02")}
		retry := inv.typedArgv()
		for _, name := range missing {
			retry = append(retry, name, examples[name])
		}
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: fmt.Sprintf("a grant needs %s; nothing was done", strings.Join(missing, ", ")),
			next: retry, nextReason: "the filled-in values are examples: pick the tiers, acts and end date you mean"})
	}
	var verbs []string
	for _, act := range strings.Split(inv.input.text("acts"), ",") {
		act = strings.TrimSpace(act)
		if act == "" {
			continue
		}
		verb, known := grantActs[act]
		if !known {
			return inv.render(intentResult{Outcome: intentRefused, code: 2,
				Summary: fmt.Sprintf("%s isn't an act a grant covers; nothing was done", shellCommand([]string{act})),
				next:    append(withoutOption(inv.typedArgv(), "acts"), "--acts", "approve,budget,resume-parked"), nextReason: "keep the acts you mean"})
		}
		if !slices.Contains(verbs, verb) {
			verbs = append(verbs, verb)
		}
	}
	if problem := inv.selectRoot(); problem != nil {
		return inv.render(*problem)
	}
	actor, proof, problem := inv.actingAs("grant", "", actorHuman)
	if problem != nil {
		return inv.render(*problem)
	}
	if proof != nil && proof.Helm != nil && proof.Helm.Grant != "" {
		_ = humanauthority.RecordAttorneyRefusal(inv.stateRoot, *proof, "grant add", "a grant is added only by the person's own proof", proof.CheckedAt)
		refusal := inv.personRefusal("", proof.WalkRefusal(), "")
		refusal.code = 2
		refusal.Details = append(refusal.Details, "a grant is added only by the person, never under another grant ("+proof.Helm.Grant+")")
		return inv.render(*refusal)
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
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "two different grants are named; nothing was done",
				next: withoutOption(inv.typedArgv(), "grant"), nextReason: "name the grant once"})
		}
		entry = inv.input.args[0]
	}
	if entry == "" {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "needs the grant to close: metasystem grant revoke GRANT; nothing was done",
			next: inv.publicArgv("grant", "list"), nextReason: "lists the grants with their ids"})
	}
	if problem := inv.selectRoot(); problem != nil {
		return inv.render(*problem)
	}
	actor, _, problem := inv.actingAs("revoke", "", actorHuman)
	if problem != nil {
		return inv.render(*problem)
	}
	// A local revoke waits, bounded, for acts admitted under a grant to
	// finish; it publishes either way.
	var waited []string
	if checkout, err := canonicalCheckout(inv.stateRoot); err == nil {
		if locked, lockErr := inv.owners.attorney.withDefaults().exclusive(checkout, revokeLockWait); lockErr == nil && !locked {
			waited = append(waited, "an act admitted under a grant was still running after "+revokeLockWait.String()+"; the revoke is published anyway")
		}
	}
	args := append([]string{"--root", inv.stateRoot, "--id", entry}, actor...)
	result := inv.ownerCall([]intentTarget{{Kind: "grant", ID: entry}}, func(dependencies syncRequestDependencies) int {
		return runGoalRevokeWithInputs(args, inv.owners.prove, inv.owners.commandNow, dependencies)
	}, func() intentResult {
		return intentResult{Summary: "revoked " + entry, Data: map[string]any{"grant": entry}}
	})
	result.text = append(result.text, waited...)
	return inv.render(result)
}

func runIntentSplit(inv *intentInvocation) int {
	id, code, ok := inv.namedGoal(nil)
	if !ok {
		return code
	}
	plan := inv.input.text("plan")
	if plan == "" {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(id), Summary: "a split needs the draft of its member goals; nothing was done",
			next: inv.typedArgvWith("--plan", "FILE"), nextReason: "FILE lists the member goals in the draft format"})
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
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(id), Summary: fmt.Sprintf("two arcs are named, %s and --arc %s; nothing was done", inv.input.args[1], arc),
				next: inv.typedArgvLess("arc"), nextReason: "keeps " + inv.input.args[1]})
		}
		arc = inv.input.args[1]
	}
	if arc == "" {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(id), Summary: "no arc is named to group the goal into; nothing was done",
			next: inv.publicArgv("goal", "group", id, "ARC"), nextReason: "ARC names the arc"})
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
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(id), Summary: fmt.Sprintf("--test and %s are two different ways to resolve it; give one; nothing was done", strings.Join(fixtureGiven, " ")),
			next: inv.typedArgvLess(fixture...), nextReason: "resolves the finding by its test; a fixture obligation takes the four fixture fields instead"})
	case !inv.input.has("test") && len(fixtureGiven) < len(fixture):
		missing = append(missing, "--test (or all of --implementation-chain, --artifact, --result, --critic)")
	}
	if len(missing) > 0 {
		retry := inv.typedArgv()
		for _, name := range missing {
			if flag, _, _ := strings.Cut(name, " "); strings.HasPrefix(flag, "--") {
				retry = append(retry, flag, strings.ToUpper(strings.TrimPrefix(flag, "--")))
			}
		}
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(id), Summary: fmt.Sprintf("resolving a review finding needs %s; nothing was done", strings.Join(missing, ", ")),
			next: retry, nextReason: "with the missing values filled in"})
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
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(id), Summary: "a finding a person carried is resolved by that person, not by a session; nothing was done",
				next: inv.typedArgvLess("lineage"), nextReason: "in a terminal you opened yourself"})
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
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(id), Summary: "adding and closing notes are two separate commands; nothing was done",
			next: inv.typedArgvLess("close", "fixed", "moved", "accepted"), nextReason: "adds the notes; then close with its own command"})
	case adding:
		if inv.input.text("read") == "" {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(id), Summary: "added notes need the read they came from; nothing was done",
				next: inv.typedArgvWith("--read", "LABEL"), nextReason: "LABEL names the read"})
		}
		if inv.input.has("add") && inv.input.has("add-file") {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(id), Summary: "--add and --add-file both give the notes; give them one way; nothing was done",
				next: inv.typedArgvLess("add-file"), nextReason: "keeps --add"})
		}
		if len(closure) > 0 {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(id), Summary: strings.Join(closure, " ") + " only go with closing a note; nothing was done",
				next: inv.typedArgvLess("fixed", "moved", "accepted"), nextReason: "adds the notes"})
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
			retry := inv.typedArgvLess("fixed", "moved", "accepted")
			if len(closure) == 0 {
				retry = append(retry, "--fixed", "COMMIT")
			} else {
				retry = append(retry, closure[0], inv.input.text(strings.TrimPrefix(closure[0], "--")))
			}
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(id), Summary: "closing a note takes one of --fixed, --moved or --accepted; nothing was done",
				next: retry, nextReason: "or --moved GOAL or --accepted REASON"})
		}
		if inv.input.has("read") {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(id), Summary: "--read only goes with adding notes; nothing was done",
				next: inv.typedArgvLess("read"), nextReason: "closes the note"})
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
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(id), Summary: "no note is named to add or close; nothing was done",
			next: inv.typedArgvWith("--close", "ITEM"), nextReason: "or --add TEXT to add a note"})
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

// waitContinuations reads this checkout's durable wait continuations,
// checked against the checkout holder's session when --session names one.
func (inv *intentInvocation) waitContinuations() (map[string]any, []string, bool) {
	var lines []string
	waitsFailed := false
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
		waitsFailed = true
		waits["outcome"], waits["error"] = intentRefused, holderProblem
		lines = append(lines, "waits: "+holderProblem)
	} else if rows, err := report.CurrentWaitingLines(inv.stateRoot); err != nil {
		waitsFailed = true
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
	return waits, lines, waitsFailed
}

// recoverWaits is work wait --list: the continuations and who may resume them.
func (inv *intentInvocation) recoverWaits() intentResult {
	waits, lines, failed := inv.waitContinuations()
	result := intentResult{Outcome: intentConfirmed, text: lines, Data: map[string]any{"waits": waits}, Summary: "this checkout's recorded wait continuations"}
	if failed {
		result.Outcome, result.code, result.Summary = intentRefused, 1, "the wait continuations cannot be recovered here"
	} else if waits["outcome"] == intentUnchanged {
		result.Outcome, result.Summary = intentUnchanged, "this checkout has no recorded wait continuations"
	}
	return result
}

// trackedDefectLabel says in plain words what a non-trunk-red entry is.
func trackedDefectLabel(class string) string {
	switch class {
	case goal.TrunkRedClassPendingFlake:
		return "pending flake, seen on a batch tip, not on main"
	case goal.TrunkRedClassKnownFlake:
		return "known flake, intermittent on main"
	case goal.TrunkRedClassHang:
		return "hang, a stalled test run"
	}
	return class + " entry"
}

// runIntentIncidents lists the trunk-red register, or claims or closes one
// entry through the same owner calls as red own and red close.
func runIntentIncidents(inv *intentInvocation) int {
	if problem := inv.selectRoot(); problem != nil {
		return inv.render(*problem)
	}
	projection, _, problem := inv.projection()
	if problem != nil {
		return inv.render(*problem)
	}
	lines, listed, tracked := []string{}, []goal.TrunkRedEntry{}, []goal.TrunkRedEntry{}
	for _, entry := range projection.Tree.TrunkRed {
		if entry.Closed != nil && !inv.input.switched("all") {
			continue
		}
		if entry.EntryClass() != goal.TrunkRedClassTrunkRed {
			tracked = append(tracked, entry)
			continue
		}
		listed = append(listed, entry)
		state := "open, unowned"
		switch {
		case entry.Closed != nil:
			state = "closed"
		case entry.FixGoal != "":
			state = "owned, fixed by goal " + entry.FixGoal
		}
		lines = append(lines, fmt.Sprintf("  %s  %s  %s (%s)", entry.ID, entry.Group, state, entry.Status))
	}
	summary := fmt.Sprintf("%d incident(s) on main", len(listed))
	if len(tracked) > 0 {
		lines = append(lines, "  tracked defects (not failures on main; they never hold a landing):")
		for _, entry := range tracked {
			state := "open"
			if entry.Closed != nil {
				state = "closed"
			}
			lines = append(lines, fmt.Sprintf("  %s  %s  %s, %s", entry.ID, entry.Group, trackedDefectLabel(entry.EntryClass()), state))
		}
		summary += fmt.Sprintf("; %d tracked flake or hang entr%s", len(tracked), map[bool]string{true: "y", false: "ies"}[len(tracked) == 1])
	}
	result := intentResult{Outcome: intentConfirmed, text: lines, Data: map[string]any{"incidents": listed, "trackedDefects": tracked},
		Summary: summary, view: incidentListView(listed, tracked, inv.input.switched("all"))}
	for _, entry := range listed {
		if entry.Closed == nil && entry.FixGoal == "" {
			result.next, result.nextReason = []string{"metasystem", "incident", "claim", entry.ID, "--goal", "G"}, "an unowned incident needs a goal that fixes it"
			break
		}
	}
	return inv.render(result)
}

// runIntentIncidentAct claims (the trunk-red register's own) or closes one
// entry through the register's owner.
func runIntentIncidentAct(inv *intentInvocation, sub string) int {
	if len(inv.input.args) != 1 {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "no incident is named; nothing was done",
			next: inv.publicArgv("incident", "list"), nextReason: "names the incidents; then repeat with one of them"})
	}
	entry := inv.input.args[0]
	if problem := inv.selectRoot(); problem != nil {
		return inv.render(*problem)
	}
	target := []intentTarget{{Kind: "trunk-red", ID: entry}}
	args := []string{sub, "--root", inv.stateRoot, "--id", entry}
	var proof *humanauthority.Proof
	if sub == "own" {
		if inv.input.has("reason") {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "--reason is for closing an incident, not owning it; nothing was done",
				next: inv.typedArgvLess("reason"), nextReason: "owns the incident"})
		}
		if inv.input.text("goal") == "" {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "owning an incident needs the goal that fixes it; nothing was done",
				next: inv.typedArgvWith("--goal", "GOAL"), nextReason: "GOAL is the goal that fixes it"})
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
				return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "--" + name + " is for owning an incident, not closing it; nothing was done",
					next: inv.typedArgvLess(name), nextReason: "closes the incident"})
			}
		}
		reason := strings.TrimSpace(inv.input.text("reason"))
		if reason == "" {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "closing an incident needs a reason; nothing was done",
				next: inv.typedArgvWith("--reason", "TEXT"), nextReason: "TEXT says why"})
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

// runIntentGrantList lists the ledger's recorded powers of attorney: the
// live ones, or with --all the revoked and expired ones too.
func runIntentGrantList(inv *intentInvocation) int {
	if problem := inv.selectRoot(); problem != nil {
		return inv.render(*problem)
	}
	projection, now, problem := inv.projection()
	if problem != nil {
		return inv.render(*problem)
	}
	var entries []goal.PowerOfAttorneyEntry
	if projection.Tree != nil && projection.Tree.Root != nil {
		entries = projection.Tree.Root.PowerOfAttorney
	}
	views := []map[string]any{}
	var shown []grantShown
	for _, entry := range entries {
		live, why := entry.LiveAt(now)
		if !live && !inv.input.switched("all") {
			continue
		}
		shown = append(shown, grantShown{entry: entry, live: live, why: why})
		if entry.General() {
			views = append(views, map[string]any{"grant": entry.ID, "by": entry.By, "acts": entry.Verbs, "for": entry.For, "checkout": entry.Checkout,
				"lineage": entry.Lineage, "since": entry.Since, "until": entry.Until, "revoked": entry.Revoked, "live": live})
			continue
		}
		views = append(views, map[string]any{"grant": entry.ID, "by": entry.By, "tiers": entry.Tiers, "acts": entry.Verbs, "since": entry.Since,
			"until": entry.Expires, "revoked": entry.Revoked, "live": live})
	}
	return inv.render(intentResult{Outcome: intentConfirmed, Data: map[string]any{"grants": views},
		Summary: fmt.Sprintf("%d power(s) of attorney", len(views)), view: grantListView(shown, inv.input.switched("all"))})
}

// grantShown is one grant grant list shows, with whether it is live and,
// when it is not, why.
type grantShown struct {
	entry goal.PowerOfAttorneyEntry
	live  bool
	why   string
}

// grantClosed says why a grant no longer holds, in local time: revoked
// when and by whom, ended when, or the ledger's own reason.
func grantClosed(entry goal.PowerOfAttorneyEntry, why string, env textui.Env) string {
	if revoked, err := time.Parse(time.RFC3339, entry.Revoked); err == nil {
		return "revoked " + env.Time(revoked) + " by " + strings.TrimPrefix(entry.RevokedBy, "human:")
	}
	if until, err := time.Parse(time.RFC3339, entry.Until); err == nil && entry.General() && !until.After(env.Now) {
		return "ended " + env.Time(until)
	}
	return "closed: " + why
}

// grantListView is grant list's page (output-style §6.6): how many grants
// are live, then one card per grant, its id whole on a row of its own.
func grantListView(grants []grantShown, all bool) func(*textui.Page) {
	return func(page *textui.Page) {
		env := page.Env()
		live := 0
		for _, grant := range grants {
			if grant.live {
				live++
			}
		}
		switch {
		case all && len(grants) > 0:
			page.Headline(textui.Count(len(grants), "grant", "grants"), textui.Number(int64(live))+" live", textui.Number(int64(len(grants)-live))+" closed")
		case len(grants) == 0 && all:
			page.Headline("No grants recorded")
		case len(grants) == 0:
			page.Headline("No live grants")
			page.Hint(textui.Hint{Argv: []string{"metasystem", "grant", "list", "--all"}, Reason: "also lists revoked and expired grants"})
		default:
			page.Headline(textui.Count(len(grants), "grant", "grants") + ", live")
		}
		section := page.Section("", "")
		for _, grant := range grants {
			entry := grant.entry
			by := strings.TrimPrefix(entry.By, "human:")
			state, end := textui.Live, ""
			if !grant.live {
				state, end = textui.Stopped, grantClosed(entry, grant.why, env)
			}
			if entry.General() {
				if grant.live {
					until, _ := time.Parse(time.RFC3339, entry.Until)
					end = env.Until(until)
				}
				card := section.Item(state, strings.Join([]string{goal.GeneralAct, "by " + by, end}, " · "))
				// A checkout is named from the home directory, never
				// relative to the repository it may itself be inside.
				outside := env
				outside.Repo = ""
				card.KV("for", textui.Plain("the main session of "+entry.For+" ("+outside.Path(entry.Checkout)+")"))
				card.KV("id", textui.Plain(entry.ID))
				continue
			}
			tiers := make([]string, 0, len(entry.Tiers))
			for _, tier := range entry.Tiers {
				tiers = append(tiers, fmt.Sprint(tier))
			}
			if grant.live {
				end = "until " + entry.Expires
			}
			card := section.Item(state, strings.Join([]string{strings.Join(entry.Verbs, ", "), "tiers " + strings.Join(tiers, ", "), "by " + by, end}, " · "))
			card.KV("id", textui.Plain(entry.ID))
		}
	}
}
