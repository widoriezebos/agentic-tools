# Human Approval for Execution — design (goal human-approval-for-execution, revision 1)

Author m0 (Fable lane, job hae-design-r1c), 2026-09-03. Tier 3 under
R-54-m1: this design, one review, one fold, one closing review, then
build and one code review; R-60-m1's material stop criterion governs
the reviews. Wido's rule (verbatim on the goal record): "everybody can
get anything on the backlog, but only the human (I) can approve it for
execution. So the state ready for impl can only be set by me". The
goal's DONE line is adopted as the acceptance contract (§13).

## 0. Shape summary (one paragraph)

One new state, `approved`, sits between `queued` and `claimed` in the
synced per-goal ledger; `queued` keeps meaning "on the backlog, awaiting
the human's word" and a draft stays what it is today, a file outside the
ledger. Only the human moves a goal into `approved`, through a new verb
`goal approve` whose core gate is the same enrolled-terminal proof (or
the recorded relayed word, with its expiry and its conceded limit) that
`goal resume` and `goal set-obligation` already require; the approval
carries the complete budget tuple as one act, either a named standing box
(`--budget small|big`) or the four limits, and it runs the existing
over-norm strict-form gate once, at approval, so the claim never asks for
a token again. Every path that creates a claimed revision refuses a goal
that is not `approved` with a typed `APPROVAL_REQUIRED` message; the
frontier, the idle verdict, the steward's backlog judgment and `goal next`
count only approved goals as claimable. `goal unapprove` withdraws
approval (parking a running claim through the existing human park). The
existing backlog is grandfathered by one recorded human act, a sweep
whose rule is "every live goal that carried a valid budget on the day",
bound to the listing the human saw; the four machines rebuild first, the
sweep lands last.

## 1. What exists and is reused (traced)

1. The live ledger is the synced per-goal world, not the legacy file.
   This checkout has no `plans/goals.md` and carries 110 goal files under
   `plans/goals/` plus the root record; `converted()`
   (cmd/metasystem/goal.go:323-329) routes every verb to the sync engine.
   The four states are `queued|claimed|parked|done`, closed at
   internal/goal/file.go:189-202 and refused by name at :263-264; the
   file grammar is `ParseFile` (:206-372) and `RenderFile` (:693-807).
   The brief's seam citations for the state machine and the claim gate
   name the legacy ledger (internal/goal/goal.go:230-233, :275-281,
   :286-292; goalverbs.go:405-412): that world has no claim verb (its
   analog is `Promote`, goalverbs.go:351-370) and this design leaves it
   byte-identical (§16 records the mapping as a brief gap).
2. The claim path is `Claim`/`claimRequest` (internal/goal/verbs.go:404-482):
   `StateQueued` is the only admitted source (:452-453), the budget comes
   from the flags or the stored tuple (:463, `budgetForClaim` :163-174),
   and the over-norm gate runs at every claim (:467, `goalNormApproval`
   norm.go:98-132). Sibling claim-creating paths: `ClaimArc`
   (:1519-1607), `OpenClaim` (:1267-1340, agent-only, creates `claimed`
   directly), `Steal` (:1181-1265, human, reassigns a standing claim),
   the reopen-into-claimed-arc branch (:995-1019), `Resume`
   (stop.go:355-412, claimed only), and the recovery replay table
   (recover.go:236-346, which rebuilds claims through the same
   constructors).
3. Human authority today has two grades. The weak one is a name:
   `r.Actor.Human != ""` from `--by` (set-pin verbs.go:1885-1888, steal
   :1185, park/done/edit rows). The strong one is process ancestry:
   `humanauthority.Prove` (internal/humanauthority/authority.go:478-559)
   walks from the command's parent to the enrolled terminal and returns
   `AGENT_IN_AUTHORITY_CHAIN` on any adapter-signed ancestor (:533-538);
   `set-obligation` (verbs.go:553-557) and `resume` (stop.go:355-358)
   refuse without a proof that `AuthorizesSetObligation`/`AuthorizesResume`
   (authority.go:117-137). `lease.Classify` (internal/lease/classify.go:308-392)
   answers HUMAN for a terminal-holding caller with no recognised ancestor
   (:368-370); `syncReq` uses it only to derive the claim epoch
   (cmd/metasystem/goalsync_mutations.go:59-69).
4. The relayed form is `ProveOrTemporaryGoalAuthority` (authority.go:228-237):
   enrolled ancestry wins; else `--temporary-human-word` plus `--review-by`
   mint a `TEMPORARY_HUMAN_WORD` proof (:199-222) validated for three
   words minimum, a non-past date, and the R-32-m1 horizon
   (:159-180; governance/types.go:95-97, horizon 2026-09-06). The engine
   records the tuple on the history line (`recordTemporaryRelay`
   file.go:144-149), allows one relayed act per goal per ruling
   (`repeatedRelayedActError` :174-181), and concedes the limit in code:
   "the relay records the supplied words but cannot verify who supplied
   them" (authority.go:114-116). The command shape is `steward arm`'s
   (cmd/metasystem/steward_verbs.go:487-532) and set-obligation's
   (goalsync_mutations.go:554-650), which this design copies.
5. The over-norm strict form: `goalNormApproval` (norm.go:98-132) needs
   `--approved-ref` naming a rulings row or a `human:` history operation
   whose reason carries `goal=<id> minutes=<n> goalRevision=<r>`
   (`RecordedNormApproval` :53-91), at the goal's pre-touch revision
   (:122-124); it publishes `NormApproval` on the file (file.go:59-64),
   and `ValidateTree` proves minutes cover the budget (validate.go:206-209).
   Because the budget act itself bumps the revision, every claim after a
   set-budget needed a fresh token (rulings R-59-m1, R-62-m1 are the
   specimens). The norm is `goal.norm.job-minutes` (1440 per the brief).
6. Consumers of "claimable": `Next` (project.go:490-524, Ready = queued
   with blockers done, pin and labels honoured), `ReadClaimableBudgetedWork`
   (:276-331, Claimable = Ready with a valid budget, Queued = every
   queued goal), the turn verdict's idle block (turnverdict.go:246-265),
   the steward's backlog judgment (internal/steward/openwork.go:44-71),
   `goal next` (cmd/metasystem/goal.go:443-483), `goal list` sections
   (:384-386), and the queued-frontier digest (turnverdict.go:604-648).
7. Hand edits and replay: `mapOneChange` (reconcilemap.go:205-338)
   refuses generated fields and unknown state changes (:269-270);
   hand-created goals may not carry a budget (reconcilepub.go:101-106);
   `requestForEntry` (recover.go:209-347) replays journal entries through
   the verb constructors, and verbs needing a live proof re-run from
   their own entry point (:346).
8. Drafts: `plans/goals-drafts/` free-form files, promotion is `goal open`
   (docs/backlog-mechanism.md:58-65, R-2). Standing budget boxes:
   R-45-m0b, small 4h/10/240m/1 and big 8h/10/240m/1.
9. The backlog today (this worktree's tree): 99 queued (95 with a budget,
   4 without; 30 human-origin), 3 claimed (dispatch-cap-necessity on m1b,
   fleet-slack-channel on m0b, path-class-manifest on m1; all budgeted),
   7 parked (2 budgeted). The fleet is m0, m0b, m1, m1b (R-61-m1).

## 2. The state and the exact ledger-format change

**States, closed, five:** `queued | approved | claimed | parked | done`.
`StateApproved = "approved"` joins file.go:189-202; the parse message at
:264 names all five.

**One new record line** on the goal file, rendered after `Budget:` and
`NormApproval:` (RenderFile order) and parsed by the closed-key grammar
(`parseKVRecord`, file.go:598-629):

```
- Approved: by=human:<name> at=<RFC3339> revision=<n> authority=proven|relayed [reviewBy=<YYYY-MM-DD>]
```

`by` is the history actor form (as `Parked: by=`), `at` and `revision`
are the approval operation's stamp and the revision it landed as (as
`Claimed:`), `authority` says which proof class admitted it, `reviewBy`
is present exactly when `authority=relayed`. `Approved` is a generated
field: hand edits that change it refuse (reconcilemap.go:206-248 gains
the row); hand-created goals carrying it refuse (reconcilepub.go:101-106).

**File invariants** (ParseFile, beside :283-291): `State: approved` ⇒
`Approved` present and `Budget` present and valid; `State: queued` ⇒ no
`Approved` record; `authority=relayed` ⇒ `reviewBy` parses as a date;
`authority=proven` ⇒ no `reviewBy`. `Approved` may stand on `claimed`,
`parked`, and `done` files (it is the audit line of who admitted the
work); a `claimed` file WITHOUT it is tolerated at rest as a pre-gate
claim, exactly as a revisionless claim is tolerated (file.go:81-83), so
the cutover never freezes the tree (§10).

**Tree invariants** (ValidateTree): Goal-free exclusivity
(validate.go:285-294) and `declare-free` (verbs.go:1157-1162) treat
`approved` as they treat `queued`; the root-record and placement rules
are unchanged. Zero-current legality is a legacy-ledger rule
(goal.go:286-292) and does not apply to the synced world; its synced
twin is exactly this exclusivity rule, which still holds.

**Transitions**, the complete table (rows not listed keep today's rule):

| from → to | verb | who |
| --- | --- | --- |
| queued → approved | approve | human (proof, §5) |
| approved → queued | unapprove | human (proof) |
| approved → claimed | claim, claim --arc | agent (the only claim source) |
| claimed → parked (approval withdrawn) | unapprove | human (proof); composes park |
| approved → parked | park | as queued today (human-origin gate stands) |
| parked → approved | unpark | when the `Approved` record stands, else → queued |
| approved → done | done | as queued today |
| done → queued | reopen | clears `Approved`; a reopened goal needs a fresh word |
| claimed → claimed (new pair) | steal | human `--by` as today; `Approved` unchanged |
| claimed → claimed (fresh revision) | resume | as today; `Approved` unchanged |

Retired rows: `open --claim` (verbs.go:1267-1340) and the reopen
into-claimed-arc branch (:995-1019) create `claimed` without a human
word and are removed; their journal replays refuse by name
(recover.go:240-244 → "open --claim is retired; close this entry by
hand"). Split members (split.go:265) open `queued`; the human approves
the members he wants executed (§7).

## 3. The claim refusal: message and boundary

In `claimRequest` after the already-claimed check (verbs.go:443-451):

```
APPROVAL_REQUIRED: goal <id> is queued and not approved for execution; only the human approves it (goal approve --id <id> --by <name> --budget small|big …) — this claim is refused
APPROVAL_EXPIRED: goal <id> was approved by a relayed word reviewable by <date>, which has passed; a fresh approval is required (enrolled terminal preferred)
```

then the existing "only an approved goal claims" shape for `parked`/
`done`. The same two refusals run in `claimArcRequest` per member
(verbs.go:1572-1574): an unapproved queued member refuses the WHOLE
cascade naming the member (fail closed, not the silent skip parked
members get today), and in the steal preflight (:1226-1243) for a member
whose `Approved` record is missing (a pre-gate claim stolen after the
sweep is the one way to reach it; the human re-approves first).

**Budget at claim:** a claim on an approved goal takes the stored tuple
only. Budget flags on `goal claim` refuse (`the budget was bound by the
human's approval; claim carries no tuple`), and `--approved-ref` on
claim refuses as not applicable. `budgetForClaim` (:163-174) keeps its
"no structured budget" arm only for the tolerated pre-gate shapes.

**Boundary:** the refusal lives in the engine's mutation callback, so it
binds the CLI, the arc cascade, recovery replay, and any future caller
alike; `ValidateTree` adds the at-rest check "a claim landed at revision
r on a goal whose `Approved` record is absent and whose history has no
`approve` line before r" so a forged verb path cannot publish a
post-cutover claim without the record.

## 4. The human-only approval verb

```
metasystem goal approve --id <id> [--id <id> …] --by <name>
    (--budget small|big | --elapsed-limit … --attempt-limit … --reserved-job-minutes-limit … --active-job-limit …)
    [--approved-ref <ref>] [--temporary-human-word "<words>" --review-by <date>] [--lineage …]
metasystem goal unapprove --id <id> --by <name> --because "<text>" [relay flags]
metasystem goal approve --sweep [--confirm <listing-sha256>] --by <name> [relay flags]
```

**Budget as one act.** The approval names its budget or refuses:
`--budget small` binds R-45-m0b's 4h/10/240m/1, `--budget big` binds
8h/10/240m/1, the four limits bind exactly what the human typed
(`budgetTuple(true)`, goalsync_mutations.go:166-181, reused); box and
limits together refuse; no flags refuse ("approval names its budget;
the machinery supplies no default", docs/backlog-mechanism.md:12-13).
The verb writes `Budget` and `Approved` in one transaction, one revision,
one history line `approve actor=human:<name> targets=<ids>` per file,
with the relay tuple appended when relayed (§6). A budget the seat
stored earlier on a queued goal is replaced, never adopted. Repeated
`--id` approves several goals under one budget in one operation (the
split-member case, §7); each target runs the norm gate separately.

**Composition with the over-norm strict form (no duplication).** The
approval verb calls the existing `goalNormApproval` (norm.go:98-132)
exactly as set-budget does: an over-norm tuple needs `--approved-ref`
naming the strict token at the goal's pre-touch revision, and the
resulting `NormApproval` is published beside the budget. That is the ONE
strict-form check. Claim, arc claim and steal then use a new
`normApprovalForApproved(f)`: within the norm → nothing; over the norm →
the stored `NormApproval` must exist and cover `ReservedJobMinutesLimit`
(the same rule ValidateTree already enforces at validate.go:206-209),
else `GOAL_NORM_REFUSED` naming re-approval by the human. No fresh token
at claim, so the revision churn of R-59/R-62 ends: the approval act is
the human word the token expressed, and the token binds the revision the
human saw. Human `set-budget` and `resume` keep the fresh-token form
(they are separate human budget acts). Agent `set-budget` on any goal
that is not its own claimed goal refuses ("budgets on unclaimed work are
the human's approval act"), retiring the seat-budgets-queued-goals
practice that R-44/R-45 licensed; human `set-budget` on an approved goal
revises the bound tuple under the existing norm gate.

**The core human-classification gate.** The engine verb is
`Approve(r VerbRequest, ids []string, budget Budget, proof *humanauthority.Proof)`
and refuses unless `r.Actor.Human != "" && proof != nil && proof.AuthorizesApprove(root)`,
the exact shape of `SetObligation` (verbs.go:553-557). `AuthorizesApprove`
is `ValidFor || temporaryValidFor` (authority.go:117-119 pattern) and
`TemporaryApproveFor` reports the relayed class; `RecordApproveProof`
stores the proof under the operation id with action `goal approve`
(authority.go:561-607 pattern; `unapprove` and the sweep record theirs
under their own action names). The command layer mirrors
`runGoalSetObligationWithAuthority` (goalsync_mutations.go:560-650):
proof first (`ProveOrTemporaryGoalAuthority`, real wall clock), then
`syncReq`, then publish, then record. Consequences, exactly: a caller
whose ancestry crosses any adapter-signed process gets
`AGENT_IN_AUTHORITY_CHAIN` and the verb refuses; a caller from an
unenrolled terminal gets "no readable terminal enrollment" and refuses
(enrollment is `goal enroll-terminal`, main.go:406); `--by` alone
authorizes nothing. The `lease.Classify` HUMAN class is not the gate: it
proves a terminal, not the enrolled agent-free terminal, and the brief
asks for the stronger boundary the two existing human-only verbs already
use. Recovery never replays `approve`, `unapprove`, or the sweep from the
journal: they need a live observation and re-run from their entry point
(recover.go:346 arm).

**Approval binds what was approved.** The human approved an intent; an
agent `edit --intent` on an approved-but-unclaimed goal refuses ("the
human approved this intent; ask for a fresh approval or unapprove"),
while next-step, labels, blockers, arc and pin edits keep their queued
rows. After the claim the claimant's own intent edits follow today's rule
(verbs.go:1089-1098); the `Approved.revision` and the history order show
an auditor what was approved and what changed after.

**Unapprove.** On `approved`: → `queued`, clears `Approved`, `Budget`,
`NormApproval`; history `unapprove … reason=<because>`. On `claimed`: the
revoke the goal record asks for — the goal → `parked` with `Parked:
by=human:<name> … displaced=<pair>@<at> because=approval revoked: <text>`,
claim binding cleared through `clearClaimBinding` (verbs.go:128-137, so a
breach-stopped goal refuses and names `resume`), `Approved`, `Budget` and
`NormApproval` cleared. "At the next safe point" is what a human park
already means: dispatch admission needs a claimed revision, so no new job
reservation lands, running jobs run out, and the displaced pair hears the
displacement through the existing acknowledgment path (verbs.go:210-286).
`unpark` then yields `queued`, which needs a fresh word. On `parked`
with an `Approved` record: clears the record (the pause stands).

## 5. The relayed-word form

Same flags, same validator, same horizon as resume and set-obligation
(`ValidateTemporaryWordPair`, `validateTemporaryGoalAuthority`,
authority.go:143-180; horizon `governance.TemporaryGoalAuthorityHorizon`,
2026-09-06). Enrolled ancestry wins whenever provable; the relay is
evaluated only after ancestry fails.

**Durable record**, three places, all existing grammar: (1) the goal's
history line carries `authorityOutcome=TEMPORARY_HUMAN_WORD
authorityReviewBy=<date> authorityRuling=R-32-m1 temporaryHumanWord="<words>"`
(`recordTemporaryRelay`, file.go:144-149; `recordedRelayedAct` :183-186
gains `approve` and `unapprove`); (2) the `Approved` record carries
`authority=relayed reviewBy=<date>` so a reader of the file, the
frontier and the claim see the review date without scanning history;
(3) the proof file under `artifacts/agents/authority/proofs/<opid>.json`
with action `goal approve`. The command announces the temporary state on
stdout exactly as set-obligation does (goalsync_mutations.go:646-648).

**Expiry and review.** A relayed approval admits claims only until its
`reviewBy` date (UTC date comparison against the transaction's `r.Now`);
after it, claim refuses `APPROVAL_EXPIRED` (§3) and the frontier lists
the goal as awaiting (§8). Work already claimed before the date is not
stopped: expiry gates the START of work, which is the gate Wido asked for.
Re-approval is a fresh `approve` (terminal preferred); a second relayed
approve on the same goal under the same ruling refuses through
`repeatedRelayedActError` (file.go:174-181), as resume and set-obligation
do. The horizon constant makes every relayed approval expire by
2026-09-06 unless Wido renews the ruling; renewing is one constant under
his word and is not this design's.

**The honest limit, conceded.** An agent's relay cannot prove the human's
word (authority.go:114-116; enrollment law R-29-m1). The design records
the words, the date and the ruling, announces the temporary state, and
refuses after the date; it does not, and cannot, authenticate who typed
`--temporary-human-word`. Closing that forge belongs to goal
ledger-authentication (plans/goals/ledger-authentication.md), which this
design does not touch. The fleet conversation channel's authenticated
reply (plans/fleet-slack-channel-design.md §5, `AUTHENTICATED_CHANNEL_WORD`)
is the intended third proof class for approve from his phone; that
design's consumer list must name `goal approve --approved-ref <opid>`
when it folds, and this design reserves the seam: `AuthorizesApprove`
gains that branch there, not here.

## 6. Scope note on R-32-m1

R-32-m1 scoped the relayed form to exactly resume and set-obligation.
This design widens that set to `approve`/`unapprove` on the authority of
the goal's DONE line ("human-only verbs with the relayed-word form"),
approved for execution by R-61-m1, and of this brief's Input 5. The
review should confirm the widening is Wido's word; §16 lists it.

## 7. The draft question, settled: one new state

A draft is a file in `plans/goals-drafts/` (R-2; docs/backlog-mechanism.md
:58-65): free-form, no grammar, no budget, outside the ledger. It stays
that. Anyone, machines included, may open a goal from it or directly
("everybody can get anything on the backlog"); `goal open` is unchanged
and yields `queued`. R-2's "only his word opens the goal" for big tickets
is superseded by Wido's 2026-09-03 rule: the gate moved from opening to
execution, and drafting remains an optional shaping space. So the ledger
gains ONE state, `approved`; `queued` IS "awaiting approval"; there is no
`draft` or `pending-approval` ledger state. The simpler shape wins
(R-11): two new states would double every consumer switch in §1.6 for a
distinction the drafts directory already carries, and a ledger `draft`
would need its own open/promote verbs and a second human gate. Split
members open `queued` and are approved by the human with one repeated
`--id` act under one budget; a human-ratified split does not carry the
parent's approval to the members because members need their own
within-norm tuples, which is exactly the judgment approval exists to
record.

## 8. Consumers: frontier, idle verdict, `goal next`, list

`Next` (project.go:490-524): `StateApproved` takes today's queued arm
(labels, pin, blockers → Ready or Blocked), minus relayed approvals past
their review date, which join a new bucket `Awaiting` together with every
`StateQueued` goal. `ReadClaimableBudgetedWork` (:315-325): Claimable =
Ready (the budget check stays as a belt), Queued counts `Awaiting`. The
idle block (turnverdict.go:256-265) and the steward's judgment
(openwork.go:51-71) therefore see only approved goals as claimable work,
and the steward's "queued goals are visible but not claimable budgeted
work" line reads "N queued goals await the human's approval". `goal next`
(goal.go:470-481) prints, in order: `continue your claimed goal: X`;
`next ready goal: X` (approved); `all approved goals are blocked; the
first is X`; `no approved goal; N queued await the human's approval
(first: X)`; label and empty cases as today — the queued line never
suggests a claim. `goal list --pretty` gains the section `approved`
between `claimed` and `queued`, marking relayed approvals `(relayed,
review by <date>)`; the JSON gains the key. The queued-frontier digest
(turnverdict.go:614-638) covers `queued` and `approved` so the
once-per-change block keys on both.

## 9. The grandfather sweep: one rule, one recorded act

**The rule:** every live goal that carries a valid `Budget` at sweep time
is approved by the sweep; a goal without a budget is not. Rationale the
record states: a budget on a queued goal is the seat's act under the
standing tuple delegation R-44-m0b/R-45-m0b, which Wido pre-approved;
the sweep ratifies exactly that set once and ends the delegation
(§4, agent set-budget retired). A goal with no budget was never boxed
and gets its own approval. Claimed goals gain the `Approved` record with
state, claim, budget, revision binding and stop capability untouched
(a claim mid-flight is never broken: `Claimed.revision` keeps pointing
at the claim event, the sweep's history line is a later revision, and
`ValidateClaimRevision` file.go:378-394 still holds). Parked goals with
a budget gain the record and stay parked (unpark → approved).

**The act:** `goal approve --sweep` (no `--confirm`) is a dry run: it
prints the listing — one line per goal: id, state, origin, budget,
NormApproval if any — plus `listing-sha256=<digest over the sorted
"id state budget" lines>` and changes nothing. `goal approve --sweep
--confirm <digest> --by <name> [relay flags]` performs it in ONE
transaction (one opid, one commit, the multi-file shape `claim --arc`
already publishes): it refuses if the tree's listing digest differs from
`--confirm` (a budgeted goal appeared or moved since he looked, as split
binds `draftSha256`), it refuses if any `Approved` record already exists
on the tree (the sweep runs once), and it writes per file the history
line `approve actor=human:<name> targets=<id> reason=sweep` and the
`Approved` record, and one root-record line `approve actor=human:<name>
reason=sweep listing=<digest> approved=<n> skipped=<ids without budget>`
(root history is the existing declare-free/prune/ack channel,
root.go:17-30). On today's tree the listing is 95 queued + 3 claimed +
2 parked = 100 approved, 9 skipped (4 queued, 5 parked without budget).
It is not a per-goal question: Wido reads one listing and gives one
word. The proof classes and their limits are §4/§5's; a relayed sweep
stamps every record `authority=relayed reviewBy=<date>`, so all 100
expire together on that date unless he re-approves at the terminal —
stated on the dry run's last line.

## 10. Migration for the four-machine fleet

Old binaries cannot parse the new state or the `Approved` field (unknown
field/state refuse by name, file.go:263-264, :589-590), so a swept tree
is unreadable to a stale engine: every verb, `goal next`, the steward's
open-work judgment (WorkDegraded) and the turn verdict fail closed there.
That is safe and loud, and it fixes the order:

1. Land the build on main (engine, CLI, docs/backlog-mechanism.md law,
   fixtures). The tree is unchanged by the landing; old and new binaries
   read it.
2. Every machine (m0, m0b, m1, m1b) rebuilds and re-arms under the
   standing order R-37-m3, each landing message naming the commit. A
   rebuilt machine refuses every new claim (`APPROVAL_REQUIRED`) until
   step 4; its claimed goals continue (dispatch admission reads
   `claimed`), and `goal next` says the queue awaits approval. A machine
   not yet rebuilt keeps claiming under the old law during this window;
   the window is closed by step 4, and the seats' conduct rule on the goal
   record ("a seat claims only a goal Wido approved by word") covers it.
3. Wido enrolls his terminal where he types (`goal enroll-terminal`, per
   machine; none is enrolled as of R-29-m1) or uses the relay.
4. Wido runs the sweep (§9) on one machine; the ledger branch carries it
   to the others through the existing sync.
5. Pending journal entries of pre-cutover `claim`/`open-claim` replays
   refuse under the new rule and close by hand; the recovery report names
   them.

No format-version bump (rejected, §11); no data rewrite except the
sweep; rollback is `git revert` of the engine plus, if the sweep landed,
Wido's `unapprove` is unnecessary because an old engine ignores nothing —
it refuses, so a rollback after the sweep needs a forward fix, which the
closing review must weigh (§15).

## 11. Rejected alternatives

1. **Two new states (`draft`/`pending-approval` plus `approved`).**
   Rejected: `queued` already means awaiting, the drafts directory already
   holds drafts, and every consumer switch in §1.6 would double for a
   distinction with no consumer.
2. **No new state: approval as a record on a queued goal.** Rejected:
   claimability would again hide in a field, which is today's defect
   (budget presence made a queued goal claimable); the state is what every
   consumer switches on and what `goal list` shows.
3. **Approval as a strict-form history token consumed at claim
   (`--approved-ref` on claim).** Rejected: duplicates the norm machinery
   for a different question, keeps the claim agent-initiated with a
   copied token, and inherits the per-revision re-approval churn
   (R-59/R-62).
4. **Gate on `--by` only, like set-pin.** Rejected: a name is not a
   boundary; the brief asks for the core classification the two existing
   human-only verbs already enforce.
5. **A pre-sweep tolerance mode in the engine.** Rejected: any tolerance
   an agent can trigger is a bypass; the cutover order (§10) carries the
   window instead.
6. **Bumping the root `FormatVersion`.** Rejected: stale engines already
   refuse the new field and state by name; a version bump touches
   ParseRoot, migrate and adoption for the same fail-closed outcome.
7. **Revoke through breach-stop "next safe point" machinery.** Rejected:
   a human park already closes admission and lets running jobs finish;
   reusing it is one rule, not two.

## 12. Proof plan (tests by name; fixture)

internal/goal: `TestApprovedStateRoundTrips` (render/parse, both
authority forms, invariants refuse queued-with-record and
approved-without-budget); `TestClaimRefusesUnapprovedByName`
(`APPROVAL_REQUIRED` text, queued and arc member); `TestClaimTakesOnlyTheBoundBudget`
(flags and `--approved-ref` refused); `TestClaimOverNormUsesStoredNormApproval`
(no token at claim; missing cover refuses); `TestApproveBindsBudgetBoxesAndTuple`;
`TestApproveRunsNormGateOnce`; `TestApproveRefusesWithoutProof`
(agent-chain proof, name-only `--by`); `TestApproveRelayRecordsAndExpires`
(history tuple, `Approved` record, claim refused after `reviewBy`,
repeated relay refused); `TestUnapproveOnClaimedParksAndClears`
(displacement, admission closed, breach-stopped refuses);
`TestApprovalSurvivesParkAndClearsOnReopen`; `TestAgentIntentEditOnApprovedRefuses`;
`TestAgentSetBudgetOffOwnClaimRefuses`; `TestNextSeparatesApprovedFromAwaiting`
(Ready, Awaiting, expired relay); `TestClaimableWorkCountsOnlyApproved`
(steward and verdict inputs); `TestSweepDryRunListsAndDigests`;
`TestSweepConfirmsDigestApprovesBudgetedOnly` (100/9 shape on a fixture
tree; claimed goal's claim untouched; root line; second sweep refuses);
`TestSweepRefusesOnDigestDrift`; `TestHandEditCannotApprove` (reconcile
refuses queued→approved and a changed `Approved`); `TestRecoveryRefusesRetiredOpenClaim`;
`TestNoClaimWithoutApproval` (every claim-creating path enumerated:
claim, claim --arc, open --claim, reopen arc-join, steal, resume, replay —
each either passes through `approved` or refuses). cmd/metasystem:
`TestGoalApproveCommandProofOrder`, `TestGoalNextWordingForAwaiting`,
`TestGoalListApprovedSection`. Fixture
`scripts/agents/goal-cli-fixtures.sh` gains one scenario: open, claim
refused, approve by relay (announced), claim succeeds with the bound
budget, unapprove parks, sweep dry-run then confirm. The validation suite
runs outside the delegate sandbox (KI-15).

## 13. Build bound (one slice, tier 3)

The build is one implementer job over: internal/goal (file.go, verbs.go,
norm.go, project.go, validate.go, reconcilemap.go, reconcilepub.go,
recover.go, turnverdict.go), internal/humanauthority (three methods, one
record function), cmd/metasystem (goalsync_mutations.go, goal.go,
main.go), docs/backlog-mechanism.md and docs/glossary.md, the tests and
the fixture scenario. Out of scope, named: the fleet channel's proof
class (its design), forge-proofing (ledger-authentication), tier-derived
budget defaults (severity-tiered-rigor), the legacy ledger.
Acceptance is the goal's DONE line: state in file and projection;
approve/unapprove human-only with the relayed form; typed claim refusal;
idle verdict counts approved only; `goal next` separates the two; sweep
explicit and recorded; fixtures prove each refusal and the relayed
approval.

## 14. Decisions Wido may still change (recorded, not blocking)

D1 the state name `approved` (alternatives `ready`). D2 `--budget
small|big` as the named boxes; a tier-named box waits for the tiering
machinery. D3 relayed approvals expire at `reviewBy` and gate only the
start of work. D4 agent `set-budget` retired off its own claim. D5 the
sweep rule "budgeted on the day" with a digest-bound listing. D6 R-2's
opening restriction superseded by the execution gate.

## 15. Self-grade (R-24)

Confidence: high on the state, the claim boundary, the proof gate and the
sweep (every rule reuses a traced mechanism); medium on the fleet cutover
window (§10 step 2 depends on four rebuilds landing in one day). Reject
this design if any of these holds: a claim-creating path exists that
neither passes through `approved` nor refuses (the `TestNoClaimWithoutApproval`
enumeration is incomplete); the relayed form's widening beyond R-32-m1's
two verbs is not Wido's word (§6); or the fleet cannot rebuild all four
engines before the sweep, in which case the sweep must wait and the
design needs a stale-engine tolerance it deliberately rejected (§11.5).
The weakest part is §10's window between the first rebuilt machine and
the sweep, during which a rebuilt seat can only continue its claimed goal.

## 16. Brief gaps recorded (not filled silently)

1. Inputs 1–3 cite the legacy ledger (goal.go:275-292, goalverbs.go
   Claim and humanGate :405-412); the live claim path is verbs.go:404-482
   and the states are file.go:189-202; goalverbs.go has no Claim. The
   design targets the synced world and leaves the legacy ledger
   byte-identical; the zero-current rule's synced twin is the Goal-free
   exclusivity rule (§2). Confirm the mapping.
2. R-32-m1's "no other human-only act is opened" versus the goal's DONE
   line and Input 5 (relayed form for approve): resolved here by the later
   word (R-61-m1 approving the goal as written); confirm (§6).
3. R-2's "only his word opens the goal" versus Wido's 2026-09-03 rule
   ("everybody can get anything on the backlog"): resolved by the later
   rule (§7); a rulings-register note should record the supersession.
