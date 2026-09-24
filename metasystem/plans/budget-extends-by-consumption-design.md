# A budget extends once by consumption, and low-tier ceremony runs under power of attorney (goal budget-extends-by-consumption-and-breach-parks)

- Kind: design
- Id: 01M3A3TATHVCADQTPBZ6PDPXHS
- Status: done
- Goals: budget-extends-by-consumption-and-breach-parks

- Status: revision 4 (2026-09-12 evening) after the second design read (Codex, gpt-5.6-sol; section 5); section 2b landed f3a9359b0 and rule 1 landed with it the same night (section 5 rounds 3 and 4, both on Opus). Rule 2 landed 4468c8211 for tier 1 (tier 2 joined with f55bbb6bf). Rule 1 is rewritten against both reads now that proof attempts settle to observed minutes (a633b7a08, 46e5678e4); the attorney unpark of R-105-m1e is section 2b. One invariant is Wido's (section 7); the build proceeds on the code's reading of it.
- Goal: budget-extends-by-consumption-and-breach-parks (goal 8 of plans/delivery-efficiency-plan.md)
- Next step: none
- Settled: landed (rule 2 4468c8211 and f55bbb6bf, section 2b f3a9359b0, rule 1 9a3102266) and concluded 72c7e435a; nothing open on this page. Wido's word on section 7 stands as the only follow-up, on his desk.

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

Every attempt reserved its full cap until a633b7a08: an ended proof attempt
is now charged the minutes it ran (`observedMinutes`, rounded up, at least
one; internal/dispatch/budget.go's proof-attempt loop) and a live one its
reservation, and 46e5678e4 keeps the deadline as a reservation figure only.
Consumption is therefore what ran, and rule 1 can read it.

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

## 1. Rule 1: the budget extends once by consumption (revision 3)

**What the rule is for.** Forty-three set-budget acts in five days,
twenty-six of them repeat raises within a day, eleven overnight (R-94-m1e):
a goal that is working consumes its box and a person is woken to raise it.
The rule lets a goal that is demonstrably advancing extend its attempts and
minutes once, by itself, and leaves the second exhaustion to a person.

**Trigger.** The revision seam (`EvaluateGoalRevisionAdmissionForDispatch`,
internal/dispatch/admission.go, reached by `job goal-revision-admission`
from the dispatcher and in-process by the proof run) is about to refuse a
proposal on `attemptLimit`, on `reservedJobMinutesLimit`, or on both at
once (at the limit, or used plus the proposed cap over it: the seam's own
test) and on no other member, with no live-stop reason (a corrupt-over-limit state is a
stop, never an extension). Elapsed, active-job and review-round breaches
are never extended: elapsed has its grace and its stop, the other two are
not consumption. The revision seam is the one authority for the offer: the
seat walk (`EvaluateGoalAdmission`) keeps refusing as today and offers
nothing, because it judges without a proposal and would spend the
once-per-goal marker on a refusal the exact seam might not make.

**Advancement.** The goal advanced within the two hours before the
judgement. The first read showed that a completed job record is satisfied
by the very attempts that spent the box, and that a critique register close
writes no record of its own; the second read tightened each selector to
what proves acceptance. The evidence is one of:

- a closed critique: a chain root for this goal carrying
  `independentCritiqueJobRef` to a code-critic chain root with
  `chainClosed=true`, its register folded, and its latest member
  `completed` with `endedAt` inside the window (the pointer alone is an
  index, internal/dispatch/review_reference.go; the closed chain is what
  internal/validate/recertification.go already demands);
- a landing: a RECEIPT line in memory/receipts.log read at the accepted
  tip, `goal=` this goal, `type=implement` and `outcome=shipped` (the
  shape only a code landing writes; a design, investigate, blocked or
  parked receipt is not a landing), corrections applied (a CORRECTION line
  that moves `goal=` away removes the evidence), whose epoch is inside the
  window; its identity is epoch plus the line's SHA-1, as corrections
  already address a line;
- a passed delivery receipt: a proof attempt for this goal with a valid
  `TestResult`, `Purpose` delivery, `Delivery.Sufficient=true`,
  `Terminal.Result` success, and `EndedAt` (equal to the terminal's time)
  inside the window (internal/proofrun/attempt.go).

The code-critic round and the proof attempt spend the same box they may
help unlock; R-94-m1e names exactly those as advancement, so that is
lawful. A goal that consumed its box without one of these gets no
extension. The window is two hours before the judgement, on the clock the
seam already uses (`now`).

**The extension.** A new verb, `goal extend-budget --id <goal> --revision
<n> --proposed-cap <minutes> --role <role> --dispatch-mode <mode>
--destructive-reach <class>`, is the seat's act (the claim holder's pair,
no human, no attorney). It replays the exact revision seam under the goal
revision lock with the same arguments the refused proposal carried
(cmd/metasystem owns this: the goal package cannot import dispatch), and
proceeds only when that seam refuses on attempts or minutes alone with no
live-stop reason and names the acceptance evidence; it refuses when the
seam admits (nothing to extend), when the refusal is on another member or
is a stop, when no evidence is inside the window, and when the goal
already carries a `BudgetExtension` record. It raises `attemptLimit` and
`reservedJobMinutesLimit` by the goal's tier box values (config.TierBox),
keeps `elapsedLimit`, `activeJobLimit` and `reviewRoundLimit`, and writes
on the goal file:

```text
- BudgetExtension: at=<iso> opid=<opid> attemptLimit=<from>-><to> reservedJobMinutesLimit=<from>-><to> evidence=<review|landing|receipt>:<record id>@<iso>
```

The record is the durable once-per-goal marker: it is not part of the
claim record, so release, re-claim, steal, approve, unapprove, set-budget,
done and reopen leave it in place, and it survives the accounting-revision
change a human set-budget makes; split keeps it on the archived parent and
copies it to no member (a member is a new goal with its own box). The
extension does not rebind the claim revision and starts no accounting
episode: the tuple changes, the consumption stays, and the approval record
stays. An extension is not an approval: the human's `ApprovalDigest` is
not rewritten (rewriting it would say the human approved the extension),
so the approval check becomes extension-aware: an approval whose `at`
precedes the marker's `at` is validated against the tuple the marker's
`from` values reconstruct; an approve or set-budget after the marker
validates against the current tuple as today. A human set-budget afterwards
is unchanged law, and the marker stays, so the goal never extends itself
twice. `goal edit` cannot write or remove the record; a hand edit that
adds, removes or alters the line refuses at reconcile; recovery replays
the verb from its journaled offer.

**Wiring.** `GoalRevisionAdmission` gains `Extension *BudgetExtensionOffer`
(the evidence kind, record id and time, and the from and to tuples), set
only when the refusal is on attempts or minutes alone with no live-stop
reason, no marker on the goal, and evidence inside the window. `job
goal-revision-admission --format json` prints the verdict with the offer;
the exit codes do not change (9 refuses, 10 stops). The refusal line
carries the offer in words for a reader (`; extension available:
<kind> <id> at <iso>`) and, after an extension, the marker (`; extended
once at <iso>; a further raise is a person's set-budget`). The
dispatcher's `require_goal_revision_admission` calls the json form,
decodes the offer, runs `goal extend-budget` once with the same arguments
and judges again; a second refusal refuses as today. The proof run
(cmd/metasystem/proof_run.go) reads the Go field before it reserves and
does the same in process. `require_goal_admission` is unchanged in what it judges; the dispatcher
now runs it after the revision seam for goal-bound work (before, the seat
walk ran first and would have refused the same exhaustion before the seam
could offer), and a goal-free operation keeps the early walk. The proof
run's native-delegate custody check also compares the delegate record's
machine and claim epoch with the binding, so the extension it applies is
the claim holder pair's own act. The order has a cost the page accepts: a
refusal by the seat walk after an extension (another claim on the machine
over its box, or a breach-stop) leaves the marker spent; the offer itself
is withheld when the raise would not admit the very proposal, so the
marker is never spent on a refusal the seam would repeat. The norm
coverage a later claim checks is extension-aware like the approval
digest (the original tuple is judged against the box), the seam refuses
a zero cap, an empty role and a dispatch mode outside fresh and follow-up
for every role, and a reader failure on the receipt ledger or the attempt
store refuses the seam instead of silently dropping an evidence kind.

**The second exhaustion.** Both reads showed R-94-m1e's premise wrong: an
attempts or minutes exhaustion does not breach-stop today; the seam exits
9 and the dispatcher asks for a budget revision. This is the one invariant
of the page that is Wido's (section 7). The build proceeds on the code's
reading: after the one extension a further exhaustion refuses as today and
the refusal line names the marker, so the seat's report and the channel
carry the fact and no seat retries the verb; no new stop is added. If Wido
means a breach-stop, it is one further slice (a stop reason, the
capability and cancellation flow, exit 10 and its fixtures) on top of this
build, not a change to it.

**Where consumption is read.** The verb and the seam read one projection
(`ProjectBudget`), so what the seat sees and what admission judges agree;
the projection counts ended proof attempts at their observed minutes and
live ones at their reservation (the settlement this rule waited for).

## 2. Rule 2: low-tier ceremony under a recorded power of attorney

**The entry.** The root record (`plans/goals/backlog.md`) gains a section,
one entry per line, the same shape as `Decomposed:` (a repeated field line
would be a duplicate key the root parser refuses):

```text
PowerOfAttorney:
- <id> by=human:<name> tiers=1 verbs=approve,set-budget since=<iso> expires=<YYYY-MM-DD> [revoked=<iso> revokedBy=human:<name>]
```

`goal grant --by <name> --tiers 1,2 --verbs approve,set-budget --expires
<date>` writes it; the proof is the human's own: enrolled-terminal
ancestry or the fixture grant (no channel question exists for a grant). A
temporary relayed word cannot grant (a relay cannot mint a delegation).
`--expires` is at most six days after the grant day, so an entry lives
seven days with the expiry day included; an entry a hand edit widens past
these bounds stays readable and is never honoured (`WithinBounds`);
`--verbs` is a subset of {approve, set-budget}; `--tiers` is 1, 2 or both:
tier 2 joined the delegable tiers with the landing of
tier-from-severity-and-novelty (2026-09-12), as R-95-m1e sequenced it, by
widening the accepted set in that landing rather than by a gate keyed to a
goal id that prune can erase. Several
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

## 2b. The attorney unpark (R-105-m1e)

Wido's word of 2026-09-12 evening answers section 6: under a tier-1 entry
(tier 2 once it joined, f55bbb6bf), a seat may lift a park a person
recorded by `park` or by `unapprove` on a goal the entry covers; never a
blocker park (R-93-m1e), never a breach-stop; and the park's own reason
must name a condition the seat can verify.

`goal unpark --id <goal> --under <entry> --verified "<what holds now>"` is
the seat's act under the entry. `unpark` joins `AttorneyVerbs`; a grant
names it like the others. The verb refuses when the entry does not name
`unpark` or is not live; when the goal's tier is not 1 (R-105-m1e grants
the unpark for tier-1 goals only, under an entry that covers tier 1 (a
tiers 1,2 entry lifts no tier-2 park, a tier-2 entry lifts nothing); tier 2
would be a new ruling); when the park carries a `blocker=` token (that
park lifts by itself when its blockers conclude); when the goal is not
parked (a breach-stopped goal is claimed, so the fence is never touched);
and when `--verified` is empty. The journaled intent carries `under` and
`verified`; recovery does not replay an attorney unpark (like grant and
revoke, the entry's liveness is judged at the act), it closes the entry by
name and the seat reruns the verb while the entry is live. It writes the `unpark` history line as the seat's own with
`authorityOutcome=POWER_OF_ATTORNEY authorityRuling=<entry>` and the
reason `verified: <what holds now>`, so a reader sees the park's reason
beside what the seat checked. What the seat verifies is the seat's word:
the verb cannot judge "the vendor shipped"; the history line makes the
claim auditable and the entry's expiry bounds the trust. `unapprove`'s
park (`approval revoked: …`) lifts the same way and returns the goal to
queued, because the approval it revoked is gone.

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

Rule 1 (revision 3):

- internal/dispatch: the offer on the revision seam only, for an attempts
  breach at the limit and for a minutes refusal at the limit or by
  used-plus-proposed (the existing 170+120 over 240 shape), never for
  elapsed, a review-round breach, a mixed breach or a live-stop reason,
  never when the marker is present; each of the three evidence kinds found
  and named (a closed critique chain, a shipped implement RECEIPT line at
  the tip with corrections applied, a sufficient delivery attempt) and
  each weaker shape refused (an open critique pointer, a design or parked
  receipt, a corrected-away goal, a terminal-success-but-insufficient
  attempt, evidence outside the window); the refusal line carries the
  offer and, after an extension, the marker; `--format json` carries the
  offer with exit 9.
- internal/goal: extend-budget raises the two members by the tier box,
  keeps the other three, writes the record and the history line, refuses
  with a marker, from another pair and from a person; the marker survives
  release, re-claim, steal, approve, unapprove, set-budget, done and
  reopen, stays on the split parent and reaches no member; the claim
  revision is untouched; the approval check accepts a pre-extension
  approval against the reconstructed tuple and a later approve or
  set-budget against the current one; a hand-written, hand-removed or
  hand-altered record refuses at reconcile; recovery replays the verb from
  its journaled offer; the file round-trips.
- cmd/metasystem: extend-budget replays the seam and refuses when the seam
  admits, refuses on another member or names a stop; the proof run's
  in-process path extends, re-judges and reserves.
- scripts/agents/dispatch-fixtures.sh: a goal exhausted on attempts with a
  shipped RECEIPT line inside the window is extended once by the
  dispatcher and the dispatch proceeds; the same goal exhausted again
  refuses naming the marker.
- scripts/agents/goal-cli-fixtures.sh: `extend-budget` on a fixture goal
  with a fixture receipt line; the second call refuses with the marker.

The attorney unpark (section 2b):

- internal/goal: unpark under a live entry naming `unpark` lifts a park
  recorded by `park` and one recorded by `unapprove` on a tier-1 goal,
  with the history line's outcome, ruling and verified reason; refuses a
  blocker park, a tier-2 goal under a tiers 1,2 entry, an entry without
  the verb, an empty `--verified`, and a goal that is not parked (an
  attorney unpark on a fenced claimed goal refuses and leaves the fence
  unchanged); recovery closes an interrupted attorney unpark by name.
- cmd/metasystem: `goal unpark --under` accepts `--verified` and combines
  with neither `--by` nor a proof.
- scripts/agents/goal-cli-fixtures.sh: the `power-of-attorney` scenario
  grants `unpark`, parks a tier-1 goal in Wido's name and the seat lifts it
  under the entry with a verified reason; a blocker park is refused.
- docs/backlog-mechanism.md carries the extension and the attorney unpark.

## 6. Question for Wido (R-36-m3), answered by R-105-m1e

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
breach-stop. Wido's answer is R-105-m1e (2026-09-12 evening): yes, as
advised; section 2b builds it.

The critique also found that `goal unpark --by <name> --lineage <x>` from
an agent shell lifts a person's park with no proof at all (the proof gate
exists for approve and set-budget only); that is proposal P-5 in
memory/backlog-notes.md, a person's to open.

## 7. Wido's invariant (R-36-m3)

R-94-m1e says a second exhaustion "breach-stops as today"; the code has
never breach-stopped on attempts or minutes (a corrupt-over-limit state
stops; equality and a crossing proposal refuse with exit 9 and the
dispatcher asks for a raise). Both reads name this as Wido's to settle.
The build takes the code's reading (refuse, naming the marker); if Wido
means a stop, one further slice adds it. Put to him with this landing.

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

Revision 3 (2026-09-12 evening, m1c): rule 1 rewritten against F6 to F9
now that proof attempts settle to observed minutes; the verb takes no
proposal and extends an exhausted box; the availability rides the refusal
line and a verdict field, no new exit code; the second exhaustion refuses
as today naming the marker (put to Wido as the reading of R-94-m1e's
"breach-stops as today"); the attorney unpark of R-105-m1e added as
section 2b. Awaiting its second read.

Round 2 (design), Codex gpt-5.6-sol, 2026-09-12 evening, revision 3: ten
material and two minor findings; the loop closes under R-97-m1e with one
invariant to Wido (section 7). Folded: F1 the verb replays the exact
revision seam with the refused proposal's arguments and the seat walk
offers nothing; F3 the approval check is extension-aware and the human's
digest is not rewritten; F4 a `--format json` verdict carries the offer
for the dispatcher and the proof run reads the Go field; F5 a closed
critique chain, not a pointer; F6 a shipped implement RECEIPT line at the
tip with corrections applied; F7 a sufficient delivery attempt; F8 the
marker's lifecycle across approve, unapprove, set-budget, steal, reopen
and split; F9 the attorney unpark is tier 1 only; F10 the journal carries
`under` and `verified` and recovery closes the act by name; F11 the fenced
fixture reworded; F12 the stale sentence removed. F2 is section 7.

Round 3 (implementation of section 2b), Opus 5 code-critique agent (the
R-108-m1c lane), 2026-09-12 evening: three material findings folded. The
unpark checked the goal's tier but not the entry's recorded tiers, so a
tier-2 entry lifted a tier-1 park (`attorneyCoversGoal` now runs beside the
tier-1 check, and section 2b says so); docs/orchestration.md still said
unpark waited on Wido's word; the CLI's refusal of `--by` with `--under`
was unproven for unpark (the bed proves it). Minor findings folded: the
grant help and the missing-entry refusal name `unpark`; a stray
`--verified` without `--under` refuses; the stale test name; a person's
park that gained a blocker edge afterwards is not lifted until the blocker
is done; the root record's history line carries the entry when the unpark
clears a free slot. Acknowledged, not built: an engine older than the
landing reads an entry naming `unpark` as outside the bounds and honours
none of its verbs (docs advise a separate entry while older engines run).
The grammar probes (reason text carrying `reason=`, authority tokens,
tabs, a 1000-character value) all round-tripped; the breach-stop fence is
unreachable from a parked goal; recovery closure and idempotency hold.

Round 4 (implementation of section 1, built by Codex Sol), Opus 5
code-critique agent, 2026-09-12 night: three material findings folded.
The offer was withheld when attempts and minutes breached together, the
commonest exhaustion (the gate is now "every breach is a consumption
member"; a both-members fixture); a refused `goal extend-budget` killed
the dispatch as an internal fault (the dispatcher now prints the refusal,
retakes the lock and judges again); the dispatcher's admission order had
changed while the page said it had not (recorded above with its cost; the
bed leg for a seat-walk refusal after an extension is not added, the
existing seam test proves the order's necessity and the cost is a decided
fact on this page). Minor findings folded: the offer is withheld when the
raise would not admit the proposal; evidence-reader errors refuse the seam;
the bed asserts the landing identity by epoch and SHA-1; an attempts breach
without evidence and a proposal beyond the raised box are proven to carry
no offer. Recorded, not built: the receipt fold is a second reading of the
ledger beside internal/metrics; the marker's validation cannot check the
tier box without config (reconcile and the verb's arithmetic guard it);
the verb's tier-0 fallback differs from the seam's TierLaw refusal; the
closed-critique reader's register and completion legs have positive proof
only; a refusal without an offer evaluates the seam twice. The seat
reverted one Codex change outside the page (an episode drop on caller
class in leaveOrDropEpisode) and updated the call-site test that pinned
the old admission order.
