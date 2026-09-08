# Backlog ordered by priority

Revision: 1. Date: 2026-09-08. Design authoring job: `backlogorder-design3`.

This page specifies the build; it implements nothing. Independent critique,
dispositions, the obligation matrix, certification, and receipts belong to the
orchestrator. All paths below are relative to the repository root. Proposed
symbols and tests are explicitly identified as new.

The contract is the Intent field of
`metasystem/plans/goals/backlog-ordered-by-priority.md`:

> DONE means: every goal record carries a priority (a small ordered scale, e.g. 1 highest) and a sequence number (its place within that priority, unique among open goals); the pair is set and changed after the fact by a human verb at the enrolled terminal (goal set-priority --by <human> --id <goal> --priority <n> [--sequence <n>], with re-sequencing of the others in that priority when a number is inserted) and recorded as a ledger event like set-pin; goal list prints open goals in that order, priority then sequence, with state and pin; goal next --machine <nick> returns the first approved, unclaimed goal in that order that is unpinned or pinned to that machine, and seats take work through it instead of reading; a goal with no priority yet sorts last, so approval of the grandfathered backlog needs no bulk edit; and the channel's status report shows the top of the order so the human sees what the fleet will do next.

The same record states: “Wido sets the first priorities himself once the verb
exists.” There is no bulk ranking, score, decay, automatic priority choice,
approval change, claim preemption, or new scheduler in this design.

## Grounding in the current tree

These are source observations, not runtime proof. Line numbers identify the
revision read for this design.

| Responsibility | Existing file and symbol | Observed behavior and design consequence |
| --- | --- | --- |
| Record grammar | `metasystem/internal/goal/file.go:23`, `GoalFile`; `:371`, `ParseFile`; `:694`, `parseFileField`; `:1104`, `RenderFile` | Closed Markdown fields, duplicate-field refusal, revision/history checks and trailing integrity digest. There is no priority or sequence field. Extend this grammar, not a separate index. |
| Event grammar | `metasystem/internal/goal/file.go:285`, `HistoryLine`; `:1266`, `ParseHistoryLine`; `:1429`, `RenderHistoryLine`; `metasystem/internal/goal/verbs.go:250`, `touch`; `:332`, `ackDisplacements` | Each touched goal receives one revision and an operation-identified History line. `reason=` consumes the remainder of a history line. Use that existing payload slot. Acknowledgments already merge into one root-file write. |
| Whole-ledger invariants | `metasystem/internal/goal/validate.go:27`, `TreeGoals`; `:145`, `ValidateTree`; `:475`, `ValidateCommit`; `:508`, `SortedGoalIds` | Live goals and archived goals are distinct; the root record is separate. Validation enforces dependencies, pins and one machine claim, with one arc counting once. ID sorting is alphabetical and also serves non-scheduling callers; do not redefine it globally. |
| Human pin act | `metasystem/internal/goal/verbs.go:2322`, `SetPin`; `:2356`, `setPinRequest`; `metasystem/cmd/metasystem/goalsync_mutations.go:1382`, `runGoalSetPin` | A human name, journal intent, atomic publish and History event already exist. This is the event/transaction model. It is not sufficient terminal proof: the generic request path can receive an explicit lineage. |
| Enrolled-terminal authority | `metasystem/cmd/metasystem/goalsync_mutations.go:58`, `syncReqClassified`; `:598`, `proveGoalHumanAuthority`; `:736`, `runGoalApproveWithAuthority`; `metasystem/internal/humanauthority/authority.go:143`, `Proof.ValidFor`; `:611`, `Prove` | Proof is an in-process observation bound to the checkout. A human string, passed lineage, or deserialized proof is not that observation. Reuse this boundary explicitly for the new act. |
| Publication and recovery | `metasystem/internal/goal/txn.go:467`, `PublishRequest`; `:508`, `Publish`; `:556`, `runTransaction`; `:347`, `PublishCAS`; `metasystem/internal/goal/recover.go:43`, `RecoverWithPolicy`; `:141`, `completeFromIntent`; `:209`, `requestForEntry` | A transaction builds all changed files from one captured tip, validates, and publishes by compare-and-swap. A competing tip causes a rebuild. The default retry deadline is 60 seconds. Unknown delivery stays journaled as pushed. Recovery can confirm a landed operation without repeating it; authority-sensitive unlanded operations can require a fresh terminal invocation. |
| Approval and sweep | `metasystem/internal/goal/file.go:181`, `ApprovalDigest`; `:574`, `ValidateApprovalRecord`; `:647`, `ValidateClaimRevision`; `metasystem/internal/goal/approval.go:584`, `sweepListing`; `:624`, `ApproveSweep` | Approval binds intent, risk/tier and budget, not scheduling metadata. The sweep can approve records with no rank and preserve standing claims. Keep those payloads and bindings unchanged. |
| Claim | `metasystem/internal/goal/verbs.go:588`, `Claim`; `:605`, `claimRequest`; `metasystem/internal/goal/validate.go:269`, quota block in `ValidateTree` | Claims require approved work, satisfied dependencies and a matching pin; publication validates the quota. Claims are keyed by machine and lineage. The human-approved budget is already present; a claim does not choose one. |
| Machine context | `metasystem/internal/goal/actor.go:21`, `ResolveMachine`; `metasystem/internal/goal/verbs.go:2341`, `validPinnedNickname` | Machine identity comes from enrolled Git configuration. A pin uses a nonempty word with no Unicode whitespace; its special `-` exclusion is only because set-pin uses that token to clear a pin. |
| Lifecycle and manual edits | `metasystem/internal/goal/verbs.go:1107`, `doneRequest`; `:1326`, `reopenRequest`; `metasystem/internal/goal/split.go:224`, mutation in `splitRequest`; `metasystem/internal/goal/reconcilemap.go:58`, `MapDeltas`; `:205`, `mapOneChange` | Done and split remove live records; reopen restores one. Split creates new member records. Reconcile already refuses a hand-edited pin and must likewise refuse direct rank edits. |
| Read projections | `metasystem/internal/goal/project.go:51`, `Project`; `:512`, `Next`; `metasystem/cmd/metasystem/goal.go:273`, `runGoalList`; `:337`, `listSynced`; `:455`, `nextSynced`; `:501`, `runGoalNext` | Reads use the accepted Git tree. Listing is alphabetical within state groups. Next already exists, resolves the local machine, and reports claimed/ready/blocked/awaiting work. Ready excludes expired approvals and unfinished blockers. Neither CLI reader currently declares `--fetch`, despite the projection banner advertising it; next has no `--machine`. |
| Seat consumers | `metasystem/internal/goal/project.go:278`, `ReadClaimableBudgetedWork`; `metasystem/internal/goal/turnverdict.go:410`, `enforceIdleBacklog`; `:473`, `idleBacklogNames`; `:503`, `idleBacklogDigest`; `:960`, `queuedFrontier`; `metasystem/internal/steward/ledgerattention.go:147`, `snapshotLedger`; `metasystem/internal/steward/revive.go:177`, `claimSeatIdleGoal`; `:208`, `decideForRevival` | The fresh work projection preserves Ready order. Idle continuation takes its first item, unless already holding a claim. A recorded continuation rechecks eligibility, not current rank. Some diagnostic queues sort by age or put pins first. These are existing consumers, not grounds to introduce another selection loop. |
| Channel status | `metasystem/internal/channel/report.go:37`, `ComposeStatusReport`; `:125`, `markedNextGoal`; `:229`, `Digest`; `:270`, `ShouldPost` | Next up currently means at most two held goals and is suppressed without a recent delivery. No ordered backlog head is shown. The report has a twelve-line ceiling. Execution approval separately targets one queued, locally pinned goal labeled `next`. |
| Command and seat guidance | `metasystem/cmd/metasystem/main.go:442`, goal verb table; `metasystem/AGENTS.md`, “The Goal Thread”; `metasystem/scripts/agents/roles/steward-continuation.md`, contract step 1; `metasystem/docs/backlog-mechanism.md`, pinning and sequencing sections | These are the existing help and work-taking instruction owners. Update them in the build to describe the new verb and ordered selection. No generic shell loop that parses next and then claims was found; none is required. |

## 1. Record fields and the order

Use three priorities: **1 highest, 2 middle, 3 lowest**. Priority is independent
of the existing risk-derived Tier. A sequence is a one-based position within a
priority. The open set is every member of `TreeGoals.Live`: queued, approved,
claimed and parked. The backlog root, drafts and done archive are excluded.
Pins, state and dependencies filter execution; they never change rank.

Extend `GoalFile` with new `Priority uint8` and `Sequence uint64` fields. Render
the pair immediately after State and before Risk/Tier:

```text
- State: approved
- Priority: 1
- Sequence: 2
```

Both fields absent means **unranked**; both Go values are zero. Render neither
line for an unranked record, including new `goal open` and migrated records.
Do not emit zero as a stored rank or manufacture a priority at approval.
Explicit values are decimal digits, with priority in 1..3 and sequence in
1..18446744073709551615; rendering uses ordinary decimal without leading zeros.
Refuse a lone field, an empty value, sign, fraction, overflow, zero or priority
outside the scale. Duplicate keys retain the existing parse refusal. The same
pair-shape checks apply to archived records.

`ValidateTree` additionally requires the ranked open goals of each priority to
occupy exactly 1..N, once each. This enforces both uniqueness and no holes; a
malformed tree is refused with the priority and offending goal paths. An
archived sequence is historical and does not occupy a live slot. Unranked
records neither occupy numbered slots nor fail this invariant.

**Ambiguity resolved:** “unique among open goals” could mean a globally unique
number. This design recommends uniqueness within each priority: the complete
`(priority, sequence)` pair is unique among ranked open goals. This follows
“its place within that priority” and permits priorities 1 and 2 each to have
position 1. Likewise, “every goal record carries” is interpreted together with
the explicit absent-priority exception: the schema supports the pair on every
goal, while missing pairs remain valid. No migration is required to use it.

Add a new `OrderedOpenGoalIDs` function in
`metasystem/internal/goal/order.go`. It returns ranked goals by ascending
priority then ascending sequence, followed by unranked goals by ascending goal
ID. ID is the deterministic fallback for the unranked backlog, preserving its
existing next-selection order. It must not silently break ties in invalid
ranked data: validation owns duplicate detection. Leave `SortedGoalIds` as the
alphabetical helper for serialization, histories and unrelated comparisons.

## 2. Human mutation, re-sequencing and history

Public act:

```text
goal set-priority --by <human> --id <goal> --priority <n> [--sequence <n>]
```

Add `runGoalSetPriority` in
`metasystem/cmd/metasystem/goalsync_mutations.go` and register it in
`metasystem/cmd/metasystem/main.go`. The verb also accepts the existing `--root`
and `--lineage` transport inputs; neither confers authority. Only the exact
fake-runtime fixture authority mechanism may bypass real process ancestry in
tests. Reject unrelated mutation flags and positional arguments rather than
accepting values that the verb ignores.

Use `proveGoalHumanAuthority` with no temporary-word or channel alternative,
then pass the observation through `syncReqWithProof` and into a new
`goal.SetPriority` entry point. The domain entry point requires a nonempty
human actor and `proof.ValidFor(r.Endpoint.Root)` before publishing, even if
the requested pair would be a no-op. `--by` plus `--lineage` alone must fail.
Keep the existing caller classification, brain restriction and guard
enrollment behavior. Do not broaden or repair `set-pin` in this change.

The new domain implementation and internal `setPriorityRequest` live in
`metasystem/internal/goal/order.go`. The latter constructs a `PublishRequest`
whose mutation operates on a freshly loaded whole tree on every attempt:

1. Require the target in the open set; rank changes are allowed while claimed
   or parked and require no release. Recognize the already-landed operation
   before calculating new positions.
2. Remove the target from its old ranked bucket, if any. The remaining members
   retain their relative order and are numbered 1..N.
3. Choose its destination position after that removal. An explicit sequence
   must be in 1..N+1 for the destination bucket. With sequence omitted, keep
   the old position when the priority is unchanged; otherwise append at N+1.
4. Insert the target at that position and number the destination 1..N+1.
   Within the same priority this is remove-then-insert, never two independent
   updates. A request for its existing pair is `NothingToDo`, with no new
   canonical commit or History event.
5. Emit changes only for goals whose final pair changed. All changed files,
   histories and integrity digests publish in one commit after whole-tree
   validation. There is no visible half-resequenced tree or separate order
   file to reconcile.

Example: priority 1 contains `a:1, b:2, c:3`. Moving c to sequence 2 produces
`a:1, c:2, b:3`. Moving c to priority 2 with no sequence removes its slot from
priority 1, restoring `a:1, b:2`, and appends it in priority 2. Inserting at
sequence N+1 appends; requesting N+2 refuses rather than silently clamping.
Omitting sequence on a same-priority request does not move it to the tail.
There is no clear-priority verb in this scope.

### Closure over lifecycle transitions

Dense positions require mechanical maintenance when a goal leaves or rejoins
the open set. A new internal order helper in `order.go` owns that calculation;
callers fold its changes into their existing transaction and event:

| Transition | Rank effect |
| --- | --- |
| Claim, release, approve, unapprove, park, unpark, resume, pin or arc membership edit | Preserve the pair. Parked and claimed goals still occupy their positions. |
| Done | Preserve the archived pair as history; compact the departed priority's remaining open goals in the same `doneRequest` commit. |
| Reopen | Preserve a previously assigned priority but append at its current tail; an unranked archive entry reopens unranked. Never reclaim an archived number that now belongs to another goal. Existing reopen approval/state rules still apply. |
| Split | Archive the parent's pair and compact its old bucket. New members start unranked: priority was assigned to the parent, not independently to those new intents. Existing member dependencies and pins remain governed by split. |
| Prune or deletion from the archive | No live ordering effect. |

Compaction is a consequence of an authorized lifecycle act, not a new human
priority decision. It only removes the hole; no survivor changes priority or
relative order. In split, a dependent whose blockers and sequence both change
must receive one merged file change and one revision/event, not duplicate
path writes or two touches. Apply the same single-touch rule to existing
acknowledgment piggybacks.

For all rank-only touches, retain `Approved`, its digest and revision,
`Claimed` including its timestamp, revision and accounting revision, Budget,
StopCapability, StopFence, Sliced and obligations exactly. Only the pair,
outer Revision, History and Integrity change. Re-sequencing another machine's
claimed work must not restart its clock, reset spend, invalidate approval or
mint fresh stop authority. Existing lifecycle transitions retain their own
authorized effects on those records.

### Event and failure contract

Journal intent uses verb `set-priority`, target `[subject-id]`, and string
arguments `by`, `priority`, and `sequence` only when explicitly provided. It
records the human's requested position, not a frozen list of displaced peers.
For each changed goal append the existing History grammar:

```text
- <at> <opid> set-priority actor=human:<name> targets=<sorted-affected-ids> reason=priority-order subject=<subject-id> from=<old-pair> to=<new-pair> requested-sequence=<request>
```

Here a pair is `1:2`, for example, or `unranked`; request is the explicit
decimal, `append`, or `keep`. Old/new pairs are specific to the containing
goal. Use the same timestamp and operation identifier for all effects of the
act. The History parser needs no new top-level key: the diagnostic payload
uses its existing `Reason` field. It is evidence, never authority or replay
input. A lifecycle compaction uses that lifecycle's verb and actor, with the
same before/after explanation appended to any existing reason, and includes
all affected IDs in its event targets. The original event coordinates and
existing displacement/acknowledgment semantics remain intact.

Publication order settles races. Two terminal-authorized insertions at
position 1 can both succeed: the first successful commit inserts first, and
the later successful commit rebuilds on it and inserts ahead of it. All ranks
remain unique. Competing edits of the same target are successive human acts;
the last published edit determines the final pair, and both events remain.
An intervening claim does not invalidate a rank edit. An intervening done or
split removes the target and refuses the edit. Re-evaluate the sequence range
on every retry: if a removal made the requested position impossible, refuse
with the new permissible range. Unrelated publication does not create a
spurious conflict. Retain the transaction engine's existing retry deadline.

Malformed input, missing human/proof, wrong terminal, missing/archived target,
sequence outside the current range, malformed accepted tree, sync/identity
refusal and unresolved prior push all refuse by name without changing the
canonical ledger. Distinguish the existing expired retry outcome from an
unknown push outcome. On the latter, report uncertainty and the operation
identifier; never retry as a fresh human act until recovery determines whether
it landed. Existing `printSyncResult` outcome/exit conventions apply.

Recovery confirms a visible operation trailer without writing ranks again.
If the operation was not published, `completeFromIntent` must reject replay
of `set-priority` and direct a fresh invocation at the enrolled terminal;
`requestForEntry` must not reconstruct authority from `by` or a stored proof.
This uses the existing sensitive-act recovery pattern rather than copying
set-pin's name-only replay. A fresh request for an already-achieved pair is
an honest no-op. No new proof-storage protocol is needed: History and the
journal are the required event surfaces.

`MapDeltas` and `mapOneChange` must refuse added, removed or changed Priority
or Sequence fields in hand-created/edited records, including rank edits
combined with an otherwise lawful edit. The remedy is `goal set-priority` at
the terminal. `goal edit` and split drafts do not gain rank-setting fields.

## 3. Ordered listing

The synced `goal list` keeps its default JSON format and existing top-level
state arrays, done archive, tip and banners. Add an `open` array containing
all matching open goal objects in `OrderedOpenGoalIDs` order. This is the
authoritative cross-state list; JSON object-key order cannot express it.
Populate retained state arrays by filtering that same ordered traversal.
New Go fields appear as `Priority` and `Sequence`, following the current
GoalFile JSON naming convention; zero/zero means unranked in JSON.

`goal list --pretty` becomes one open-goal table in the same order, with
columns `PRIORITY SEQUENCE STATE PIN GOAL`. Show `-` for each missing rank
value and for an absent pin. Keep existing intent, claim-holder, park-reason
and relayed-approval explanations as details of the row, and keep the archived
count at the end. State grouping must not precede the priority comparison.
Apply repeated label filters after establishing order; do not renumber a
filtered subset. Empty `open` is `[]`, not null.

Add `--fetch` to the synced list and next readers, routing it to
`Project(fetchFirst=true)` instead of creating another fetch path. The default
remains the accepted offline view with existing staleness banners. Fetch
failure is an error, never an empty list. The pre-conversion single-file
ledger remains supported by its existing readers; the new rank mutation and
explicit machine/fetch options refuse there with the existing migrate-first
explanation. No migration of that separate legacy format is part of this work.

## 4. Next and taking work

Retain `goal.Next` as the one eligibility/frontier owner in
`metasystem/internal/goal/project.go`. Replace its alphabetical traversal with
`OrderedOpenGoalIDs`; retain its existing claimed, ready, blocked and awaiting
categories. Preserve label filtering, approval expiry, satisfied dependencies
and member-specific pin checks. Preserve its full Ready slice for the existing
backlog-health consumers even when a claim is held: health uses the presence
of waiting work as well as the held claim.

Add a new `SelectNext` in that file which consumes this frontier, returning
one of `continue`, `ready`, or `none`, with the held/candidate ID where
applicable. Any held claim takes precedence over Ready, including a held arc
whose multiple records count as the one existing claim. This prevents the
reader from directing a machine into a second claim; it does not tighten or
weaken the validator's arc quota. Within each category the order is the one
defined above. Labels never conceal a held claim.

`goal next --machine <nick>` supplies the read-only machine context explicitly;
without it use `ResolveMachine` as today. Validate an explicit nickname as
one nonempty word with no Unicode whitespace. The literal `-` is allowed
here: this read has no clear-pin operation and that name can already hold a
claim. Do not require it to be this checkout's machine or invent a fleet-name
registry; humans may inspect another machine's eligible work. An explicit
machine never changes the actor of a later `goal claim`.

Keep the existing one-line prose surface and zero exit status for represented
outcomes. Projection notices precede the line as today:

| Selection | Line |
| --- | --- |
| Held claim | `continue your claimed goal: <id>` |
| Free machine with an eligible goal | `next ready goal: <id>` |
| No eligible goal | `no claimable goal for machine <nick>` |

For `none`, append one explanation to that line using the existing frontier:
first blocked goal if any; otherwise count/first awaiting approval; otherwise
no matching eligible work, or empty backlog when the open set is empty.
There is no candidate ID to claim in a `none` result. Bad flags, invalid
machine, unreadable accepted state and failed fresh fetch are nonzero errors,
not `none`. Both explicit and implicit machine forms use the same selector.

“Approved, unclaimed” in the request does not repeal existing dependencies or
approval expiry. A high-ranked approved goal behind an unfinished dependency
is skipped for selection while remaining visible at its actual rank in list.
Pins filter, never promote: an eligible unpinned goal at 1:1 precedes a goal
pinned to this machine at 1:2.

Seat guidance must require `goal next --machine <own-nick> --fetch` when free,
followed by the existing `goal claim --id <returned-id>` using its own
machine/lineage. It may read the selected record to understand the work;
scanning records, kickoff order or a pin-first preference must not select it.
On a held result, continue the held work. On a claim lost to another seat or
invalidated by a changed pin/approval/dependency, fetch and select again; never
fall back to a memorized second item. Stop a retry cycle on a transport,
authority or quota error and report that cause.

Next is a read, not a reservation. Its selection is true at the accepted tip
it read. A later priority edit affects later selections, not an already-held
claim or an already-recorded continuation handoff. Keep
`claimSeatIdleGoal`/`decideForRevival` rechecking the latter's eligibility and
the existing claim quota, without silently retargeting its recorded intent.
The same small read-to-claim race exists for an interactive seat: claim checks
current eligibility but does not require the candidate still to be first.
Requiring ordered selection and claim in one atomic operation would change
the claim contract and requires another design; it is not necessary for this
read-then-claim request.

`ReadClaimableBudgetedWork` and `enforceIdleBacklog` already preserve/use Ready
order; keep them on that owner. Make `idleBacklogNames` display that order
instead of pulling pinned items ahead. Have the synced `queuedFrontier` and
`snapshotLedger` waiting-queue projection filter the same ordered traversal
instead of sorting by age. These are diagnostics, not alternate eligibility
decisions. Keep `idleBacklogDigest` insensitive to order: it detects an idle
seat against the same available work, and changing priority must not reset
the existing refusal counter. No new worker loop or autonomous ranking actor
is added.

## 5. Channel status shows the head

Extend `ComposeStatusReport` using its one accepted projection and the shared
ordering/selection functions. Add two concise lines, independent of whether
any work was delivered in the report window:

```text
Backlog first: <feature name> — priority 1, sequence 2, approved, pin m2
Next for m1: <feature name> — priority 2, sequence 1, unpinned
```

The first is the first open goal globally, with its actual state and pin;
it may be queued, blocked, claimed or parked and is not an execution promise.
Use `priority unset, sequence unset` for an unranked goal. The second is
`SelectNext` for the report's machine: print `continue <feature name>` for
held work or `none claimable` for no candidate. For an empty open set the
first line is `Backlog first: empty`. If endpoint/projection reading fails,
use `Backlog order: unavailable — <cause>` and omit the machine-selection
line, rather than implying the queue is empty. Incorporate any stale or
single-machine projection notice into the backlog line so its limitations
are visible without defeating the line cap. Fold embedded whitespace in
causes/notices into spaces so each reserved entry stays one physical line.

Reserve space for these one or two lines inside the existing twelve-line
limit, after the optional brain line and status header are accounted for and
before allocating the remaining space to needs/deliveries/old Next up lines.
The existing undelivered summary keeps its reserved final slot. Fill the
remaining slots in their existing order, then emit the reserved backlog lines
before that final summary. Thus a busy needs/delivery report cannot hide the
order. Retain the existing Next up meaning and its delivery condition; the
new lines have their own explicit meanings.

Keep `markedNextGoal`, approval tokens, reply bindings and provider cadence
unchanged. Priority alone never requests or grants execution approval. If an
approval line does not fit, the existing check must still clear its returned
binding. `Digest`/`ShouldPost` already notice changed content at the normal
cadence; there is no extra immediate post or new channel event.

## 6. Fixtures and the smallest proving run

These fixtures are specified, **not written or run in this revision**. Every
command below runs from the repository root. New tests must use the existing
temporary-ledger helpers and observed fixture authority, never modify the
live backlog. Test names beginning `TestPriority`, `TestNextPriority`,
`TestGoalPriority` and `TestReportPriority` below are proposed names, not
claims that the current tree contains them. Existing fixture owners are
`oneClone`/`twoClones` in `metasystem/internal/goal/txn_test.go:52`, `seedLedger`
in `metasystem/internal/goal/verbs_test.go:29`, `testHumanAuthority` in
`metasystem/internal/goal/stop_test.go:31`, `publishGoalFixtures` in
`metasystem/internal/goal/approval_test.go:178`, and `reportLedger` in
`metasystem/internal/channel/channel_test.go:662`.

Run the subtest for the changed behavior first. A failing canary stops
widening. Once it passes, widen to its owning test/package only to answer an
unresolved question; do not run every row for every edit. All new Go test
commands have a two-minute failure ceiling; exceeding it is a named test
failure to investigate, not permission to loop a battery.

| Fixture / owning test file in the build | Smallest proving run | Required independent observation |
| --- | --- | --- |
| Set-priority reorders and re-sequences: new `metasystem/internal/goal/order_test.go`, `TestPriorityReordersAndResequences` | `cd metasystem && go test ./internal/goal -run '^TestPriorityReordersAndResequences$/^insert$' -count=1 -timeout=2m` | Three ranked goals in a real temporary ledger; move the last to position 2, then read the published tree and assert the exact order, dense positions, affected revisions and human event. Separate selectable subtests `move-priority`, `append`, `same-priority-noop`, and `claimed-peer` prove those respective rules with the same command's final selector replaced by that name. |
| Removal and lifecycle closure: same new file, `TestPriorityLifecycle` | `cd metasystem && go test ./internal/goal -run '^TestPriorityLifecycle$/^done-reopen$' -count=1 -timeout=2m` | Conclude the middle of three, observe compaction and unchanged survivor order, then reopen it at the tail. Subtest `split` proves unranked children and one revision for a dependent whose sequence also changes. |
| Two seats race: same new file, `TestPriorityRace` | `cd metasystem && go test ./internal/goal -run '^TestPriorityRace$/^same-position$' -count=1 -timeout=2m` | Use `twoClones` and the existing `PublishRequest.BeforePush` seam to force competing insertions without sleeps. Read both commits and the converged tree: both events survive, the later insertion is first, no duplicate/hole is published. Subtests `same-target` and `range-changed` prove serial human edits and refusal when a concurrent removal invalidates the position. |
| Human act and recovery: new `metasystem/cmd/metasystem/goal_priority_test.go`, `TestGoalPriorityAuthority`; new domain `order_test.go`, `TestPriorityRecovery` | `cd metasystem && go test ./cmd/metasystem -run '^TestGoalPriorityAuthority$/^wrong-terminal$' -count=1 -timeout=2m` | A supplied human and lineage with unproved ancestry cannot publish; the same Go test's `proven` subtest drives the new handler successfully with the existing exact-root fixture proof. For recovery run `cd metasystem && go test ./internal/goal -run '^TestPriorityRecovery$/^unlanded$' -count=1 -timeout=2m`: a dead owner's stored human string cannot reorder. Its `landed` subtest confirms without a duplicate event. |
| Next returns the expected goal for pinned and unpinned machines: new `metasystem/internal/goal/order_test.go`, `TestNextPriority`; new command test file, `TestGoalPrioritySelection` | `cd metasystem && go test ./internal/goal -run '^TestNextPriority$/^pins$' -count=1 -timeout=2m` | Rank A at 1:1 pinned to m1, B at 1:2 unpinned, C at 1:3 pinned to m2. Free m1 selects A; free m2 and unpinned m3 select B. Selecting B ahead of C proves a local pin confers no extra rank. Subtests `held`, `blocked-expired`, `none` and `labels` cover quota/eligibility without changing claim semantics. `cd metasystem && go test ./cmd/metasystem -run '^TestGoalPrioritySelection$/^machine$' -count=1 -timeout=2m` drives explicit-machine CLI selection; its `fetch-failure` and `claim-race` subtests prove honest read failure and reselect-after-lost-claim. |
| A goal without priority sorts last: new domain `order_test.go`, `TestPriorityUnranked`; new command test file, `TestGoalPriorityListing` | `cd metasystem && go test ./internal/goal -run '^TestPriorityUnranked$/^sort-last$' -count=1 -timeout=2m` | Unranked alphabetical A must follow ranked Z even at priority 3. Multiple unranked records remain valid and ID-ordered. Subtests `grammar` and `manual-edit` refuse partial/zero/invalid pairs, duplicates/holes and rank edits through reconcile. `cd metasystem && go test ./cmd/metasystem -run '^TestGoalPriorityListing$/^cross-state$' -count=1 -timeout=2m` proves JSON `open` and pretty rows interleave states in the same rank order while exposing state and pin. |
| Sweep-approved backlog stays valid: extend `metasystem/internal/goal/approval_test.go`, existing `TestSweepBindsListedIntentAndPreservesClaimedWork` | `cd metasystem && go test ./internal/goal -run '^TestSweepBindsListedIntentAndPreservesClaimedWork$' -count=1 -timeout=2m` | Existing test already publishes a sweep over unranked queued and claimed records. Extend it to assign one priority afterward and validate the resulting tree: other records stay unranked; approval digests, approved payloads and held claim coordinates survive. No bulk fixture migration. |
| Channel shows the head without a delivery: extend `metasystem/internal/channel/channel_test.go`, new `TestReportPriority` | `cd metasystem && go test ./internal/channel -run '^TestReportPriority$/^no-delivery$' -count=1 -timeout=2m` | A ranked global head pinned elsewhere and a lower local candidate appear with distinct global/local meanings. Subtests `line-cap`, `unavailable`, `stale`, and `approval-binding` prove visibility under twelve-line pressure, honest read limitations and unchanged marked-goal approval binding. |
| Existing seat projections carry order: extend `metasystem/internal/goal/turnverdict_idle_test.go`, new `TestPriorityIdleProjection`; extend `metasystem/internal/steward/ledgerattention_test.go`, new `TestPriorityLedgerSnapshot` | `cd metasystem && go test ./internal/goal -run '^TestPriorityIdleProjection$' -count=1 -timeout=2m` | The idle continuation chooses the ranked Ready head; diagnostics do not promote a pin and reorder alone does not reset idle enforcement. `cd metasystem && go test ./internal/steward -run '^TestPriorityLedgerSnapshot$' -count=1 -timeout=2m` checks the waiting/ready projection order. |

Existing safety canaries are also available as single tests: `TestMachinePinning`
in `metasystem/internal/goal/verbs_test.go`, `TestQuotaIsOneClaimPerMachine` in
`metasystem/internal/goal/validate_test.go`, and
`TestNextFiltersCandidatesButNeverTheHeldClaim` plus
`TestNextTreatsArcMemberPinsIndependently` in
`metasystem/internal/goal/project_test.go`. Each runs with
`cd metasystem && go test ./internal/goal -run '^<test-name>$' -count=1 -timeout=2m`,
substituting the named test. These are guards to widen into after the relevant
new canary, not substitutes for priority proof.

The traced fixture entry point
`metasystem/scripts/agents/goal-cli-fixtures.sh:79` launches all scenarios. Its
private `--fixture-bed-child` route requires a capability minted by
`metasystem/scripts/agents/fixture-budget.sh`; an environment scenario name
is ignored. No public single-scenario switch was found. The design therefore
uses exact Go tests and does not require a new fixture-runner feature.

### Build boundary and gate handoff

The build changes the owners named above and their focused Go tests, plus
command help and the three named seat/backlog guidance files. It does not
edit existing goal records or the approval sweep payload/digest. Additive
field parsing must reach every active ledger reader before Wido first writes
a priority: an older binary will correctly reject unknown fields. Deploy the
reader-capable build across the fleet, retain the absent-field backlog, then
let Wido rank individual goals. No root schema switch, automatic enrollment,
bulk rewrite or downgrade data conversion is required. After ranked writes,
an old reader is not a valid rollback; recover through the ledger's existing
repair process rather than stripping fields/history by hand.

The requested gate is, from the repository root:

```sh
cd metasystem && scripts/agents/go-gate.sh --fast && scripts/agents/dispatch-fixtures.sh && scripts/agents/goal-cli-fixtures.sh
```

`metasystem/scripts/agents/go-gate.sh` explicitly says fast mode is not the
full landing gate. The orchestrator retains the final gate/receipt obligations
for this accumulation-risk goal; this design neither substitutes the command
above for those obligations nor requests a battery in each build round.
Per the supplied orchestration contract's delegate-sandbox limitation, the
orchestrator runs process-owning fixture gates outside the delegate sandbox.
No runtime gate is claimed for this design-only artifact.

This revision settles the scale, scoped uniqueness, absent-field exception,
lifecycle compaction, read-to-claim race and status meaning explicitly. The
missing public fixture selector and existing next/fetch differences are
accounted for, not assumed away. No second design round is needed for this
scope. A demand for globally unique sequence numbers, atomic selection-plus-
claim, or new human authority over another channel would reopen the design;
the builder must report such a gap rather than invent that behavior.
