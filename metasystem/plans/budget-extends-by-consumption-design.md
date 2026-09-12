# A budget extends once by consumption, and low-tier ceremony runs under power of attorney (goal budget-extends-by-consumption-and-breach-parks)

- Status: critiqued once (2026-09-12, nine material findings, all folded below); rule 2 builds now for tier 1; rule 1 is the design of record and is rewritten and read again when proof-attempts-settle-to-minutes-run lands (R-94-m1e)
- Goal: budget-extends-by-consumption-and-breach-parks (goal 8 of plans/delivery-efficiency-plan.md)
- Next step: build rule 2 (approve and set-budget under a tier-1 entry) behind the fixtures in section 4; put the unpark question of section 6 to Wido

## Decisions already recorded

Both rules are policy under R-36-m3 and both have Wido's word (2026-09-11
evening, memory/rulings.md): R-94-m1e for rule 1, with the once-per-goal
marker and the settlement prerequisite; R-95-m1e for rule 2, tier 1 first,
tier 2 only once tier-from-severity-and-novelty has landed, no entry longer
than seven days. The auto-park clause of the original intent is dropped
(fenced claims already leave the one-claim quota). This page designs the
mechanisms; it asks Wido nothing new.

## What is true today (read in the tree)

The budget is one five-member tuple on the goal file (`Budget:`), set by
approve and set-budget, and judged before every reservation by
`dispatch.EvaluateGoalRevisionAdmission` (internal/dispatch/admission.go)
over `ProjectBudget` (internal/dispatch/budget.go), which counts attempts
and reserved job minutes from the job records under artifacts/agents/jobs
and the proof attempts under artifacts/agents/proof-runs/attempts. Equality
at attemptLimit or reservedJobMinutesLimit closes admission with
`BUDGET_REFUSED: goal <id> revision=<n> admission closed: <fields>`; a
proposal that would cross reservedJobMinutesLimit is refused the same way
with `used+proposed`. Only the elapsed member has grace (R-31-m3, 50
percent). `job goal-admission --stop-lineage` returns exit 10 when the
holder's claim is closed, and the caller runs the breach-stop; the stop is a
verb under a StopCapability, and resume is a human act. Every attempt
reserves its cap (120 minutes for a diagnostic run today), so consumption
overstates what ran until proof-attempts-settle-to-minutes-run lands.

Human authority for approve, set-budget, unpark of a human park, resume and
accept-risk comes from three proofs: freshly observed enrolled-terminal
ancestry (`humanauthority.Prove`), a recorded temporary relayed word with a
review date (`--temporary-human-word --review-by`, refused once the fleet
has enrolled a terminal), or a verified channel answer. The approval record
carries `authority=proven|relayed|channel`; a relayed approval expires at
its review date (`GoalFile.ApprovalExpired`). Wido's standing grants of
2026-09-08 to 2026-09-10 (R-88-m1b, R-90-m1d, R-91-m1b) work only by running
each act inside the enrolled tmux pane, so the grant is a convention, not a
record: nothing on disk names its scope or its end, and every act reads as
`human:Wido` with nothing to say it was a seat acting under a grant. The
five-day audit counted 43 set-budget acts, 26 repeat raises within a day
and 11 overnight.

## 1. Rule 1: the budget extends once by consumption (design of record)

**Trigger.** Admission is about to refuse a proposal on attemptLimit or
reservedJobMinutesLimit (used plus proposed, admission's own test) and on no
other member. Elapsed, active-job and review-round breaches are never
extended: elapsed has its grace and the other two are not consumption.

**Advancement.** The goal advanced within the two hours before the
judgement. The critique showed that "a completed job record" is satisfied
by the very attempts that spent the box, and that a critique register close
writes no timestamped record at all, so the evidence set is narrowed to what
exists only on acceptance: (a) a reviewed round, the completed code-critic
root with its mirrored diff (internal/dispatch/review_reference.go), or a
landing or carry record for this goal; (c) a delivery receipt for this goal
that passed: a proof attempt with `GoalID`, `TestResult.Purpose` delivery,
`Terminal.Result` success and `EndedAt` inside the window. A goal that
consumed its box without one of these gets no extension.

**The extension.** A new verb, `goal extend-budget --id <goal>
--proposed-cap <minutes>`, is the seat's act (the claim holder's pair). It
refuses unless admission would refuse the goal on those two members for
that proposal (the same used-plus-proposed test the revision seam runs) and
the advancement test holds; it refuses when the goal already carries a
`BudgetExtension` record. It raises attemptLimit and reservedJobMinutesLimit
by the goal's tier box values, keeps the other three members, and writes on
the goal file:

```text
- BudgetExtension: at=<iso> opid=<opid> attemptLimit=<from>-><to> reservedJobMinutesLimit=<from>-><to> evidence=<review|landing|receipt>:<record id>@<iso>
```

The record is the durable once-per-goal marker: not part of the claim
record, so release, reclaim, steal and set-budget leave it in place. The
extension does not rebind the claim revision or start a new accounting
episode; consumption is preserved. A human set-budget afterwards is
unchanged law; the marker stays, so the goal never extends itself twice.

**The second exhaustion.** The critique showed the premise wrong: today an
attempt or minutes exhaustion does not breach-stop; `job goal-admission`
and the revision seam exit 9 and dispatch.sh asks for a budget revision,
which is the raise loop this goal exists to cut. What the second exhaustion
does is therefore an open design question for the rewrite, not "as today".

**Wiring.** Both seams gain the verdict: `EvaluateGoalRevisionAdmission`
(the only place used-plus-proposed is judged, reached by `job
goal-revision-admission` from dispatch.sh's `require_goal_revision_admission`
and in-process from the proof run) and `EvaluateGoalAdmission`. Each prints
`BUDGET_EXTENSION_AVAILABLE goal=<id> evidence=<...>` with its own exit code
(11 is taken by stop-batch-reconcile's INDETERMINATE); the callers run
`goal extend-budget` once and judge again.

**Order.** Rule 1 is built after proof-attempts-settle-to-minutes-run
lands; this section is rewritten against the findings above and read once
more before it builds.

## 2. Rule 2: low-tier ceremony under a recorded power of attorney

**The entry.** The root record (`plans/goals/backlog.md`) gains a section,
one entry per line, the same shape as `Decomposed:` (a repeated field line
would be a duplicate key the root parser refuses):

```text
PowerOfAttorney:
- <id> by=human:<name> tiers=1 verbs=approve,set-budget since=<iso> expires=<YYYY-MM-DD> [revoked=<iso> revokedBy=human:<name>]
```

`goal grant --by <name> --tiers 1 --verbs approve,set-budget --expires
<date>` writes it; the proof is the human's own: enrolled-terminal
ancestry or the fixture grant (no channel question exists for a grant). A
temporary relayed word cannot grant (a relay cannot mint a delegation).
`--expires` is at most six days after the grant day, so an entry lives
seven days with the expiry day included; an entry a hand edit widens past
these bounds stays readable and is never honoured (`WithinBounds`); `--verbs` is a subset of {approve,
set-budget}; `--tiers` is exactly 1 in this build. Tier 2 joins when goal
tier-from-severity-and-novelty lands, by widening the accepted set in that
landing, not by a gate keyed to a goal id that prune can erase. Several
entries may be live at once; a renewal is a new entry, and `goal revoke
--by <name> --id <entry>` closes one early. Grant and revoke are root-record
operations with their own history lines, journaled and recoverable.

**The act.** `goal approve` and `goal set-budget` take `--under <entry-id>`
from a seat with no terminal and no `--by`. The command edge resolves the
entry from the accepted tree: unrevoked, today not after `expires`, the verb
in its scope, every target goal's tier in its scope. The act stays the
seat's act on the ledger: the history actor is the seat's own
`<machine>+<lineage>` and the line carries `authorityOutcome=POWER_OF_ATTORNEY
authorityRuling=<entry id>`; an approval binds with `authority=attorney` and
`by=<the seat actor>`, so no line reads `human:<name>` for an act the
human did not make, and the `--approved-ref` lookup, which reads
`human:` actors, never finds an attorney line. The approval stands after the
entry expires: the entry bounds when new acts may be made, not what was
lawfully made. Bounds the act keeps: approve under attorney takes the goal's
box or a tuple within it; neither approve nor set-budget under attorney
rewrites an approval a person made (proven, relayed or channel): they bind
only where no approval stands or where the standing one is an attorney act;
set-budget under attorney sets a tuple within the goal's tier box only (an
over-norm tuple needs the human's own proof and `--approved-ref` as today).
Not in scope, refused with the entry named: any goal of tier 2 or 3, done or
park of a human-opened goal, resume, steal, set-priority, accept-risk. The
act writes no authority proof file (`recordGoalApprovalProof` records
observed proofs only); the ledger line is its record.

**Ledger grammar this adds.** A root section `PowerOfAttorney:`, the
history outcome `POWER_OF_ATTORNEY` (requires `authorityRuling`, forbids a
review date and a word), and the approval authority `attorney` (by is the
seat actor and must equal the bound history line's actor). Each is a
parse-breaking change for an engine older than the landing, so the landing
is noted on plans/delivery-efficiency-seats.md for the fleet to rebuild.

**Where it cannot run.** A checkout declared the brain refuses every human
word carried by an agent (brainHumanWordClassification); an attorney act is
exactly that, so it is refused there too. Attorney entries are not relays,
so `refuseRelayedAfterFleetEnrollment` does not end them.

## 3. Alternatives not taken

- Admission writing the extension itself: judgement is read-only everywhere
  today and the breach-stop already follows judge-then-act; a second writer
  inside admission would need the goal-revision lock and the journal from a
  read path.
- Extending every member: elapsed has grace, active jobs and review rounds
  are not consumption, and the audit's repeat raises were attempts and
  minutes.
- A grant as a ruling row (R-68-m1's first form): rulings.md is prose the
  engine never reads; the root record is what every verb loads under the
  same lock, so the entry is checked by the machinery, not by the seat.
- Delegating done and park of human-opened goals: R-90 and R-91 both
  excluded concluding a human-opened goal; the intent keeps that.

## 4. Fixtures that prove it

Rule 2 (built now, tier 1):

- internal/goal: grant writes the entry and its history; grant refuses an
  expiry past seven days, a tier other than 1, a verb outside the set, and a
  relayed-word proof; two grants coexist and render and parse as a section;
  revoke closes one; approve and set-budget under a live entry confirm with
  the seat actor, `POWER_OF_ATTORNEY` and the entry id on the line and
  `authority=attorney` on the approval; each refuses under an expired or
  revoked entry, for a tier-2 goal, for an over-box set-budget, for a verb
  outside the entry, and approve refuses to rewrite a proven approval; an
  attorney approval does not expire with the entry; the goal file and the
  root record round-trip through render and parse.
- cmd/metasystem: `--under` is accepted by approve and set-budget only and
  combines with neither `--by` nor `--fixture-human-authority`; a missing or
  dead entry names the grant command.
- scripts/agents/goal-cli-fixtures.sh: a `power-of-attorney` scenario grants
  with the fixture human authority, approves and set-budgets a tier-1 goal
  from the seat with no terminal, is refused for a tier-2 goal, and is
  refused after `METASYSTEM_GOAL_NOW` passes the expiry.
- docs/orchestration.md and docs/backlog-mechanism.md carry the entry and
  the act; AGENTS.md is not touched (word budget).

Rule 1 (after settlement; fixtures rewritten with the section):

- internal/dispatch: the extension verdict on both seams, only for attempts
  or minutes breaches with acceptance evidence inside two hours and no
  marker; never for elapsed.
- internal/goal: extend-budget once, marker survives release, reclaim and
  set-budget, consumption preserved, a second exhaustion does what the
  rewrite decides.

## 6. Question for Wido (R-36-m3)

R-95-m1e names "unpark of a seat park" among the attorney acts. A seat's
own park needs no authority today, so that grant is empty; what a seat
cannot lift is a park a person recorded (`park`, `unapprove`, or a park in
the person's name), and the critique read a widening to those as a new
decision. The question: may a seat under a tier-1 entry lift a park that a
person recorded on a tier-1 goal? Evidence for yes: R-91 granted exactly
that for one goal on 2026-09-09 and the audit's overnight waits include such
parks. Evidence for no: a person's park is the one pause a seat has never
been able to end. Advice: yes, for parks recorded by `park` and `unapprove`
on tier-1 goals, never for a blocker park (R-93-m1e) and never for a
breach-stop. Until his word, unpark is outside the entry's verbs.

The critique also found that `goal unpark --by <name> --lineage <x>` from
an agent shell lifts a person's park with no proof at all (the proof gate
exists for approve and set-budget only); that is proposal P-2 in
memory/backlog-notes.md, a person's to open.

## 5. Critique record

Round 2 (implementation), code-critique subagent, 2026-09-12: three
material findings folded. The diff base had fallen eight commits behind
origin/main (rebased before landing); approve under attorney could rewrite a
channel or relayed approval (both verbs now refuse any approval a person
made); a hand-edited entry outside the bounds was honoured (`WithinBounds`
gates every use). Non-material: the seven-day life is now strict, the
grant proof is the terminal or the fixture, the verb labels name `--under`.

Round 1, design-critique subagent, 2026-09-12: nine material findings. F1
(unpark has no proof gate) and F2 (attorney unpark widens R-95) folded by
taking unpark out of the entry's verbs and putting the question in section
6; F3 (actor and grammar) folded: the seat stays the actor, new outcome and
authority tokens; F4 (root list form) folded: a section; F5 (tier-2 gate on
a goal id) folded: tier 2 leaves this build; F6 to F9 (exhaustion exits 9
and does not stop; the verb cannot re-check used-plus-proposed; a completed
job is not an advance; register closes leave no record) folded into the
rule 1 design of record, which is rewritten and read again before it
builds. Non-material notes N1 to N9 kept with the findings in the session
record.
