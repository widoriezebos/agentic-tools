# Goal scope bounds and the split mechanism — design (goal-scope-bounds, slice 1)

Status: REVISED (gsb-design-r2, 2026-08-31; round-1 findings GSB-R1-001..014
closed in section 9). Designer: Fable lane (R-25).
Authority: `plans/goals/goal-scope-bounds.md` (Wido 2026-08-30, human origin),
quoted verbatim in the slice-1 brief. Every file path below is relative to the
`metasystem/` module root. Every cited symbol was read in the current tree.

## 1. The gap and the two axes

The GOAL is the unit anybody claims (any machine, via the shared per-goal-file
ledger under `plans/goals/`, with a budget tuple, within dependency and pin
constraints). SLICES are the max-4-hour units inside a claimed goal, carved by
the claim holder, never picked independently (Ruling S; R-20, the 2026-08-29
mega-batch). Cross-machine parallelism on one feature is achieved by SPLITTING
into goals, never by sharing a goal.

Today three bounds exist and none is a planning-time scope floor:

- R-2 (`memory/rulings.md`): big tickets draft first — conduct, not machinery.
  Nothing mechanically refuses a bloated goal.
- The budget tuple (`internal/goal/budget.go`, `Budget` =
  elapsedLimit/attemptLimit/reservedJobMinutesLimit/activeJobLimit) — enforced
  by the breaker at spend time (`require_goal_admission` in
  `scripts/agents/dispatch.sh`), not at planning time.
- The slice law (`metasystem.budget.slice-norm-hours=4`, `metasystem.conf`
  line 9; `internal/dispatch/slice.go` `EvaluateSliceAdmission`; reaper
  capDeadline) — bounds the individual job reservation, not the goal.

This design adds the missing floor on the SCOPE axis only:

- **Scope (this goal):** planning parallelism. A per-goal norm on
  `reservedJobMinutesLimit` refuses over-norm budgets at set-budget and claim;
  `goal split` is the typed remedy; the result is an arc of small goals whose
  members are claimable per machine within recorded dependency edges.
- **Load (NOT this goal):** execution parallelism — per-machine slots so
  parallel passes never overload a host. That axis is owned entirely by the
  `machine-concurrency-governor` goal (`plans/goals/machine-concurrency-governor.md`).
  This design introduces no slot, host-load, or process-concurrency concept,
  and slices 2–3 must not touch that goal's seam. The only concurrency numbers
  here are the existing per-goal `activeJobLimit` (untouched) and the new
  per-goal job-minutes norm.

## 2. The norm

### 2.1 Key and layering rule

New configuration key, sibling of the slice norm:

```
metasystem.budget.goal-norm-job-minutes=1440
```

- Added to `metasystem.conf` immediately below line 9
  (`metasystem.budget.slice-norm-hours=4`). 1440 minutes = six 4-hour slices,
  the proposal in the goal's intent.
- Constants and resolver in `internal/config/budget.go`, following the
  `SliceNormHours` pattern exactly:
  - `GoalNormJobMinutesKey = "metasystem.budget.goal-norm-job-minutes"`
  - `DefaultGoalNormJobMinutes = uint64(1440)`
  - `func GoalNormJobMinutes(confPath string) (uint64, error)` calling
    `budgetLawValue(confPath, GoalNormJobMinutesKey, "1440")` then a
    `parseGoalNormJobMinutes` that requires a positive integer (same shape as
    `parseSliceNormHours`, `internal/config/budget.go:53`).
- Layering: committed root configuration ONLY — `.local` and environment
  sources are refused outside a fixture-authorized root
  (`fixtureBudgetLawRoot`, i.e. `metasystem.runtimes=fake`). This is inherited
  for free from `budgetLawValue` (`internal/config/budget.go:64`); no
  divergence from the SliceNormHours pattern.
- Validation: `internal/config/validate.go` gains, immediately after the
  SliceNormHoursKey block at lines 372–394, the identical three checks for the
  new key (committed value parses; `.local` source refused outside fixture
  roots; environment source refused outside fixture roots). Same wording,
  same `EnvName(...)` helper.

### 2.2 Check points

The norm compares `Budget.ReservedJobMinutesLimit` against the resolved norm.
It inspects no other tuple field (elapsedLimit bounds a batch in wall time;
attemptLimit and activeJobLimit are not scope). The check runs at every path
that WRITES or BINDS a tuple to a goal, inside the verb's `Mutate` callback so
it re-runs on every transaction rebuild. The table below is exhaustive by
construction: it enumerates every `bindClaim` call site in the tree
(`internal/goal/verbs.go:459` claim, `:519` set-budget re-bind, `:972` reopen
adoption, `:1210` steal, `:1271` open-claim, `:1492` claim-arc, `:1978`
set-arc join; `internal/goal/stop.go:391` resume;
`internal/goal/reconcilepub.go:521` reconcile's set-arc replay) plus the one
tuple-writing path that does not bind (`setBudgetRequest` on unclaimed
goals). Slice 2's implementer re-runs
`grep -n bindClaim internal/goal/*.go` and stops on any site this table
misses:

| Path | File and function | When the check fires |
| --- | --- | --- |
| `goal set-budget` | `internal/goal/verbs.go` `setBudgetRequest` (both the tuple write and the `:519` re-bind) | on the proposed tuple, before `f.Budget = &budget` |
| `goal claim` | `verbs.go` `claimRequest` | on `budgetForClaim(f, supplied)`'s result (supplied or stored tuple), before the state transition |
| `goal claim --arc` | `verbs.go` `claimArcRequest` | per member, on each member's `budgetForClaim` result |
| `goal open --claim` | `verbs.go` `openClaimRequest` | on the supplied tuple; NO strict exception here — see 2.4b |
| `goal resume` | `goal.Resume` (stop.go) | on the fresh tuple the human supplies |
| `goal steal` | `verbs.go` `stealRequest` | per member that MOVES (claimed by the old pair), on its stored tuple, before its `bindClaim` — steal creates a new claimed revision over the retained tuple and is inside the exception grammar (2.4b) |
| arc rejoin auto-claims | `verbs.go` `reopenRequest` (claimed-arc adoption) and `setArcRequest` (claimed-destination join) | on the member's stored tuple; an over-norm tuple NEVER auto-claims — the mechanical rule is in 2.5 |
| reconcile's set-arc replay | `internal/goal/reconcilepub.go` set-arc branch (`:490-523`, the hand-join auto-claim) | on the member's stored tuple; an over-norm tuple refuses as a reconcile conflict — 2.5 |

A within-norm tuple passes silently. An over-norm tuple refuses unless one of
the two exception paths in 2.4 proves out.

Deliberately NOT a check point: `goal split` itself (members open without
budgets — the budget tuple is the human's word at claim, per the goal record),
and the read/projection paths (the norm is an admission bound, not a display
fact).

### 2.3 Typed refusal wording

One refusal string, emitted by a new helper in `internal/goal` (proposed file
`internal/goal/norm.go`, `func refuseGoalNorm(id string, reserved, norm uint64) error`):

```
GOAL_NORM_REFUSED: goal <id> reservedJobMinutesLimit <n>m exceeds the <norm>m goal norm (metasystem.budget.goal-norm-job-minutes); split it into an arc of members each within the norm (goal split --id <id> --members <draft.md>), or record the human word and pass --approved-ref (strict form: goal=<id> minutes=<n> goalRevision=<r>)
```

The `GOAL_NORM_REFUSED:` prefix is the machine-greppable type, matching the
existing `SLICE_CAP_REFUSED:` convention in `internal/dispatch/slice.go:104`.
The remedy names `goal split` by verb — the requirement's "typed remedy naming
split".

### 2.4 Exception paths — exactly two

**(a) Arc membership — each member clears the norm.** There is NO arc-based
bypass flag in the check. The exception is structural: the norm applies per
goal, and an arc is how over-norm intent lawfully exists — as members whose
individual tuples each clear the norm. The arc's aggregate job-minutes may
exceed the norm without any special code path. A member whose own tuple is
over-norm refuses exactly like any goal (failure mode 5.3); the remedy is a
further split of that member.

**(b) The human's strict proven approval.** `goal set-budget` and
`goal claim` (and `goal claim --arc`, `goal steal`, `goal resume`)
accept `--approved-ref <ref>`. `goal open --claim` deliberately does NOT:
the strict triple binds a positive accepted revision of an EXISTING goal,
and open --claim operates only when the target does not exist
(`internal/goal/verbs.go:1256-1263` rejects an existing target and builds
the new file at revision 0). There is no pre-touch revision for an approval
to name, so an over-norm tuple at open --claim refuses unconditionally, and
the refusal names the lawful three-step path: bare `goal open` (the goal
lands at revision 1), `goal set-budget --approved-ref` (the word names
`goalRevision=1`), then `goal claim`. Open --claim is agent-only anyway
(`verbs.go:1224-1226`), so no human convenience is lost. The proof
machinery mirrors
`internal/dispatch/slice.go` `recordedHumanApproval` +
`strictSliceApprovalTriple` (lines 109–199), with these exact deltas:

- Token form: `goal=<id> minutes=<n> goalRevision=<r>`. The middle token is
  `minutes=`, per the human's verbatim grammar in the goal intent — NOT the
  slice law's `capMin=`. This divergence is deliberate and load-bearing: a
  recorded slice-cap approval must never silently double as a goal-norm
  approval at the same numbers. The two approvals answer different questions
  (one job's cap vs one goal's total reservation ceiling) and must be
  separately provable. Exactly one `minutes=` triple for the goal id must
  match in the referenced text (the `len(matches) != 1` rule carries over).
- The two durable places a human can leave the word are unchanged: a
  `memory/rulings.md` register row (`| R-<n>... |`, matched by the existing
  `rulingApprovalRef` shape) or a `human:`-actor History line's `reason=`
  text on any goal file, referenced by opid.
- Revision binding: the triple's `goalRevision` must equal the target goal's
  `Revision` as read at the transaction tip, BEFORE `touch` bumps it. A
  competing write bumps the revision and the approval goes stale with the
  slice law's re-approval semantics; the refusal names both revisions
  (`GOAL_NORM_REFUSED: --approved-ref <ref> covers goal <id> revision <d>,
  not current revision <r>; re-approval is required`).
- Proven minutes bound: `approval.minutes >= tuple.ReservedJobMinutesLimit`,
  else refuse (`GOAL_NORM_REFUSED: reservedJobMinutesLimit <n>m exceeds
  --approved-ref <ref>'s proven <m>m`).
- Implementation seam: the prover cannot live in `internal/dispatch`
  (dispatch imports goal; goal importing dispatch is a cycle). Slice 2 adds
  `internal/goal/norm.go` with a generalized strict-triple scanner
  (`func StrictApprovalTriple(text, goalID, token string) (value, revision
  uint64, ok bool)`) and a rulings-register/tree-history search that takes the
  already-loaded `*TreeGoals` for history (the verb searches the tip it is
  mutating, `Live` and `Done` histories, human-actor lines only) plus
  `os.ReadFile(memory/rulings.md)` from the working tree.
  `internal/dispatch/slice.go` MAY be refactored to call the shared scanner
  with token `capMin=` (behavior-preserving), or left duplicated; slice 2's
  implementer keeps the slice law's observable behavior byte-identical either
  way.
- Plumbing: `--approved-ref` joins `syncFlags` in
  `cmd/metasystem/goalsync_mutations.go` (accepted by claim, set-budget,
  resume, steal; refused with "goal <verb> does not take
  --approved-ref" elsewhere — open --claim included, per the rule above —
  following the existing budget-flag gate at `parseSyncFlags` line 141).
  Engine signatures gain the argument: `SetBudget(r, id, budget,
  approvedRef)`, `Claim(r, id, approvedRef, budgets...)`, `ClaimArc`,
  `Steal`, `Resume` likewise. For steal, one ref must prove the triple of
  EVERY over-norm member that moves (the exactly-one rule is per goal id,
  so one rulings row block can carry several members' triples); any
  unproven over-norm mover refuses the whole steal by member name. The
  journal intent args carry `approvedRef` so recovery
  (`internal/goal/recover.go`, verb switch at ~line 206) replays the same
  admission.
- Durable proof (the slice-law pattern, completed): a proven admission is
  PUBLISHED, not just journaled. `GoalFile` (`internal/goal/file.go:21`)
  gains `NormApproval *GoalNormApprovalClaim` — the goal-side mirror of
  `SliceApprovalClaim` (`internal/dispatch/slice.go:35-40`) — with fields
  `ApprovedRef`, `Minutes`, `GoalRevision` (the revision the word named,
  pre-touch). Every admitting verb writes it in the same `Mutate` that
  binds the over-norm tuple, so it lands in the same accepted commit and
  every machine that pulls sees the coordinates beside the tuple; the
  clone-local journal (`internal/goal/journal.go:104-106` — an
  artifacts-directory path, never pushed) is thereby no longer the only
  record. A verb that writes a WITHIN-norm tuple clears the record.
  Render/parse joins the file grammar as one `NormApproval:` record line.
  One at-rest rule joins `ValidateTree` (config-free, so the validator
  stays pure over the tree): a goal carrying both `Budget` and
  `NormApproval` must satisfy `NormApproval.Minutes >=
  Budget.ReservedJobMinutesLimit`, else the tree is defective by name.
  At-rest presence-vs-norm checking is deliberately NOT added
  (`ValidateTree` reads no configuration today and this design keeps it
  that way); the bind-time check is the enforcement, the published record
  is the cross-machine evidence.

Approval flow worked example: set-budget of an over-norm tuple at revision 5
needs a word naming `goalRevision=5`; that write lands revision 6; a later
claim of the stored tuple needs a word naming `goalRevision=6` (or the claim
is made in the same breath as the approved set-budget by supplying the tuple
on claim directly). One rulings row can carry the current triple and be
updated by the human as revisions move — the register is the cheap durable
channel; the goal-history channel exists for one-shot words.

### 2.5 Over-norm tuples never auto-claim

The auto-claim conveniences (`reopenRequest`'s claimed-arc adoption,
`setArcRequest`'s claimed-destination join, and reconcile's set-arc replay)
carry no `--approved-ref` channel, and none is added: an approval names one
revision, an auto-claim is a side effect of a different verb, and threading
staleness semantics through side effects is how exceptions stop being
provable. The rule is instead mechanical and total: **an over-norm stored
tuple never binds through an auto-claim path.** Concretely:

- Slice 2 (arc uniformity still standing, so a queued landing inside a
  claimed arc would be an illegal tree): the reopen/set-arc auto-claim of an
  over-norm member REFUSES with `GOAL_NORM_REFUSED` naming the rejoin
  (`goal <id> rejoins a claimed arc with an over-norm tuple; set-budget it
  within the norm first, or rejoin after release`); reconcile's set-arc
  replay refuses the same case as a `conflict("budget", ...)` — the hand
  edit is unrepresentable, which is reconcile's ordinary answer.
- Slice 3 (uniformity repealed, 4.2): the re-keyed join rules land such a
  member QUEUED — a legal tree state by then — and the outcome detail says
  so; the over-norm tuple then binds only through the explicit
  `goal claim --approved-ref`, the plumbed path with real staleness
  semantics.

A lawfully approved over-norm goal is therefore never stranded: its rejoin
lands (queued), and its claim goes through the front door where the standing
`NormApproval` record and a fresh `--approved-ref` word are checkable. The
"existing auto-claim behavior remains" claim from round 1 is withdrawn —
it remains only for within-norm tuples.

## 3. The split verb

### 3.1 Grammar and surface

```
metasystem goal split --id <parent> --members <draft.md> [--by <human>] [--root ...] [--lineage ...]
```

- Mounts in the `goal` verb table in `cmd/metasystem/main.go` (the table at
  ~line 384, beside claim/set-budget):
  `{"split", "atomize one goal into an arc of small member goals; the parent concludes as decomposed", runGoalSplit}`.
- `runGoalSplit` is a synced-world-only verb (`runSyncOnly` shape in
  `cmd/metasystem/goalsync_mutations.go`; a legacy checkout refuses with the
  standard "works the synced backlog" line). New `members` string field on
  `syncFlags`; `--members` names a draft file the CLI reads and parses BEFORE
  building the request, so the engine verb receives structured definitions and
  the journal carries them (3.6).
- Engine verb: `func Split(r VerbRequest, parentID string, members
  []MemberDraft) (PublishResult, error)` plus `splitRequest(...)
  PublishRequest`, in a new `internal/goal/split.go`.

### 3.2 Member draft format

The draft is a Markdown file in the ledger's own field grammar (parsed with
the `parseKVRecord`/field conventions of `internal/goal/file.go`):

```
# split <parent-id>

## member <kebab-id>
- Intent: <one line, required>
- Next step: <one line, required>
- BlockedBy: <comma-separated ids, optional>
- Labels: <comma-separated label tokens, optional>
```

Draft validation (CLI-side parse + engine-side re-validation inside `Mutate`):

- The `# split <parent-id>` heading must name the `--id` parent exactly.
- ≥ 2 members (a one-member split is a rename and refuses by name).
- Member ids: `validId` kebab grammar (`internal/goal/goal.go:409`), unique
  within the draft, and colliding with no live or archived goal (checked on
  the transaction tip; ValidateCommit re-proves).
- `BlockedBy` entries must resolve to sibling members or goals existing on
  the tip; `ValidateCommit`'s referential-integrity and acyclicity rules
  (`internal/goal/validate.go:186–204`) re-enforce this on the built commit.
- `Labels` validated by `canonicalLabels` (`goal.go:440`).
- Closed field set: an unknown field refuses (the `parseKVRecord` posture —
  "a key the grammar does not know is a tree nobody reviewed").
- The draft may NOT set `Pinned`, `Origin`, `Budget`, `Arc`, or `State`:
  origin and arc are computed (3.4, 3.5), pin inheritance is fixed (3.4),
  budgets are the human's word at claim, and pinning is a human act
  (`SetPin`, `verbs.go:1793`) that a main-origin split must not mint.

### 3.3 Preconditions and ratification tier

Parent state rows (checked in `Mutate` on the tip, so a concurrent transition
re-decides them):

| Parent state | Rule |
| --- | --- |
| queued | splits under the ratification tier below |
| claimed by the actor's own pair | splits ONLY after the zero-spend preflight below proves no slicing has started on the claimed revision |
| claimed by a foreign pair | REFUSES by name: "goal <id> is claimed by <m>+<l>; whether its slicing has started is that machine's job-record truth — park or steal it first, then split". No `--by` override: the evidence a lawful override would need is not on this clone |
| parked | human act only (a pause stands until a human moves it — the standing parked-mutation rule). A parked goal holds no claim (`Park` runs `clearClaimBinding`, `verbs.go:822`) and dispatch admission refuses reservations without a claimed revision, so no NEW work can start; a straggler job from before the park is bounded by the reaper's capDeadline, and that residual is accepted here as the human's call — the park already displaced the claimant by name |
| archived (done) | refuses: "goal <id> is in the archive; there is nothing to split" |
| not found | refuses by name |

**Zero-spend preflight (the before-slicing gate).** The human's law says the
big goal evolves into an arc BEFORE slicing starts. For a parent claimed by
the caller's own pair, `runGoalSplit` proves it mechanically before
publishing: it acquires the goal-revision lock for the claimed revision
(`goalrevision.Acquire(root, id, revision, "goal-split")`, held across the
publish — the exact pattern `goal resume` uses at
`cmd/metasystem/goalsync_mutations.go:366`, and the same lock every job
reservation takes at `cmd/metasystem/run.go:78`, so no reservation can land
concurrently on this clone), then projects spending with
`dispatch.ProjectBudget(root, parent, now)` (`internal/dispatch/budget.go:237`;
the cmd layer already imports dispatch — resume does) and refuses unless the
projection is `BudgetKnown` with `Attempts == 0 && ActiveJobs == 0 &&
ReservedJobMinutes == 0`. Refusal: `goal <id> revision <r> has recorded work
(<a> attempts, <j> active jobs, <m> reserved minutes); split is a
before-slicing act — conclude the slice or take the parent to the human`.
This preflight is sound exactly because the claim holder's clone is where
that claim's reservations admit (the reservation path runs under this
clone's lease and lock); that is why the foreign-claim row refuses instead
of trusting an unprovable override. A previously-claimed-then-released
parent splits as queued: its historical spend is recorded history on a dead
claimed revision, not concurrent work, and the norm refusal at claim is
what pushes decomposition before work in the ordinary path. The engine
`Mutate` cannot re-run this preflight (goal must not import dispatch, and
job records are clone-local); the lock plus the clone-locality argument is
the design's stated coverage, and 5.1 names the recovery-window residual.

Ratification tier (requirement 5, verbatim law): members inherit the parent's
origin and authority envelope; a split never mints new authority and never
expands scope. Mechanically:

- **Human-origin parent: ratification is PROVEN, not asserted.** A plain
  `--by` string is not proof — the command layer copies it straight into
  `Actor.Human` (`cmd/metasystem/goalsync_mutations.go:55`), and a split
  ratified by an unproven string would be a route for minting goals that
  carry the parent's human Origin without any human having read the seams.
  So a human-origin split takes the same bar as the existing
  authority-bearing verbs: `runGoalSplit` calls `humanauthority.Prove(root,
  ppid, nil, now)` and `humanauthority.RecordProof` (the exact `goal
  resume` sequence, `goalsync_mutations.go:347-375`), and the engine verb
  carries it — `Split(r, parentID, members, proof *humanauthority.Proof)`,
  the signature precedent being `SetObligation(r, id, proposed, proof
  *humanauthority.Proof)` (`internal/goal/verbs.go:532`). The engine
  requires a non-nil proof (and `r.Actor.Human != ""` for the record's
  name) when the parent's Origin is human. Refusal: `goal <id> is
  human-origin; the split draft is ratified by its origin tier — re-run
  with --by <name> from the enrolled terminal after the human has read the
  draft`. This is ratification of the DRAFT, not fresh authority — the
  parent's standing word covers the members' combined intent; the proof
  records that Wido actually held the terminal. It also satisfies the
  existing human gates a parent conclusion would otherwise trip (done on a
  human-origin goal is a human act, `verbs.go:716`).
- **Main-origin parent:** the coordinator ratifies — the ordinary
  agent-holder caller runs it without `--by`; a human may also run it. No
  proof is demanded because no human-origin file is minted: members copy
  `Origin: main`.
- **What the machinery can and cannot enforce about scope.** Origin
  inheritance, arc binding, blocker transfer, label union, and pin
  inheritance are mechanical (3.4); whether the members' combined Intent
  text stays within the parent's is seam judgment, which the human's own
  law assigns to the ratification tier ("the split DRAFT is ratified by the
  parent's origin tier"). The design does not pretend the text comparison
  is mechanical; it makes the ratifier PROVEN and the decomposition fully
  traceable (every member's History line names the parent, the archived
  parent's conclusion names every member).
- R-2 stays the front door for genuinely new big authority: split neither
  widens nor needs it, because the members' combined intent stays within the
  parent's recorded intent. The verb enforces the inheritance rules below;
  the seam JUDGMENT (where to cut) stays with the drafter per R-25.

### 3.4 Member construction — origin inheritance

Each member `GoalFile` (schema: `internal/goal/file.go:21`):

- `Id`: from the draft. `State`: `StateQueued`. `Intent`, `NextStep`: from
  the draft.
- `Origin`: `parent.Origin`, verbatim — a split never mints new authority.
  (Origin is immutable everywhere else: refused in `EditFields`, refused in
  reconcile replay.)
- `OpenedAt`: `r.stamp()`. `Revision`: 0 then `touch(...)` → 1, exactly like
  `openRequest`.
- `Blocked`: the draft's `BlockedBy` PLUS the parent's own `Blocked` list
  (set-union, sorted) — the parent's inbound obligations transfer to every
  member so no recorded constraint is lost when the parent concludes (3.5).
- `Labels`: the draft's labels unioned with the parent's labels,
  canonicalized — classification flows down.
- `Pinned`: `parent.Pinned`, verbatim. Inheriting a standing pin is not new
  direction; changing it afterwards is `set-pin`, a human act.
- `Arc`: the arc named for the parent (3.5). `Budget`: nil (the human's word
  at claim). `Claimed`/`Obligation`/stop fields: nil.
- History: one line per member — `verb=split`,
  `targets=<parent,member1,member2,...>` (sorted, parent first), actor from
  `r.Actor.historyActor()`, via the shared `touch` helper.

### 3.5 Arc binding, dependency edges, parent conclusion — one transaction

The whole split is ONE `PublishRequest` whose `Mutate` returns one `[]Change`
set; `BuildCommit` (`internal/goal/txn.go:236`) makes it one commit on
`refs/metasystem/goals/accepted`, so atomicity is inherited from the
transaction model — there is no partial split on any accepted tree.

Changes in the one commit:

1. **Members:** `{Path: livePath(memberId), Content: RenderFile(member)}` for
   each member (`livePath`, `verbs.go:327`).
2. **Arc binding:** every member carries `Arc: <parent-id>` — the arc is
   named for the parent by using the parent's id as the arc name
   (self-documenting: `goal show` on any member points at the archived
   parent). The name is NOT assumed free: arc names are arbitrary strings
   with no registry (`internal/goal/file.go:358-359` accepts any value,
   `SetArc` checks only nonempty-and-different, `ValidateTree` groups
   matching strings with no uniqueness rule), so a hand-built arc may
   already use the parent's id. `Mutate` therefore refuses when ANY goal on
   the tip other than the parent — live OR archived, because a `reopen` of
   an archived member would rejoin its old arc — carries `Arc ==
   parentID`: `arc <parent-id> is already in use by <ids>; a split arc must
   be born empty — rename those members' arc or pick a different parent
   id`. The check re-runs on every transaction rebuild, so a racing set-arc
   loses or wins whole. If the parent itself carried `Arc != ""`, that
   membership is NOT inherited (arcs do not nest; the parent's old arc
   simply loses a member to the archive, exactly as if it had concluded
   normally).
3. **Dependency edges:** the draft's inter-member and external `BlockedBy`
   edges land on the member files (3.4). Additionally, every LIVE goal whose
   `Blocked` contains the parent id has that entry replaced by the full
   member id set in the same commit (conservative closure: what waited on
   the whole waits on all parts), with `touch(dependent, r, "split",
   targets)` bumping each rewritten dependent. Only queued and parked goals
   can hold such an edge (a claimed or done goal with a live blocker is
   already unrepresentable per `validate.go:209–237`); rewriting a parked
   dependent's edge here is lawful for any caller because it preserves a
   recorded constraint rather than lifting a human's pause — the design
   names this as the one agent-reachable write to a parked file, and the
   History line records the split opid.
4. **Parent conclusion (decomposed-with-pointer):**
   `{Path: livePath(parent), Delete: true}` and `{Path: donePath(parent),
   Content: RenderFile(parent')}` where `parent'` has `State: StateDone`,
   `Conclude: "decomposed into arc <parent-id>: goal:<m1>, goal:<m2>, ..."`,
   `Blocked: nil` (the obligations moved to the members in step 3 — a
   concluded parent retaining live blockers would trip the done-blocker rule,
   `validate.go:230`), claim binding cleared via `clearClaimBinding` when the
   parent was claimed (a breach-stopped parent therefore refuses split until
   `goal resume`, inheriting the standing `StopFence` refusal), `Parked:
   nil`, and the split History line appended. The `goal:<id>` pointer tokens
   are exactly the resolvable form `Done`'s residue linker already recognizes
   (`residueLinkRe`, `verbs.go:641`) — the conclusion self-verifies against
   the members that exist in the same commit, and the arc retro-debt hook
   (`retrodebt.Raise` in `Done`, `verbs.go:676`) does not fire because split
   is its own verb; the arc's retro debt raises when the LAST member
   concludes, which the existing `Done` path already handles.
5. **Root record:** a standing Goal-free declaration clears in the same
   commit (the `t.Root.Free != nil` block `openRequest` uses,
   `verbs.go:378`), and `ackDisplacements` wraps the change set as every
   verb does.

`Validate` on the built commit is `ValidateCommit` unchanged — the split tree
must pass every at-rest rule (member states are uniformly queued at creation,
so the arc rules hold trivially at split time; see 4 for what changes at
first claim).

Postcondition classification inside `Mutate` on a rebuilt tip:

- Parent archived and its History carries THIS opid → `AlreadyApplied{}`.
- Parent archived by another opid, or any member id exists with a different
  opid → `LostToCompetitor{Winner: lastOpid(...)}`.
- Parent live but transitioned (claimed by a foreign pair since the first
  read, parked, etc.) → the precondition table re-decides by name.

### 3.6 Journal intent — recovery completes the split

`Intent{Verb: "split", Targets: [parent, members...], Args: {"members":
<canonical serialization of the parsed draft>, ...intentArgs(r, ...)}}`. The
canonical serialization is the draft grammar itself (3.2) re-rendered from
the parsed structs — recovery must rebuild the operation WITHOUT the original
draft file, because `internal/goal/recover.go` completes a dead owner's entry
from stored intent alone.

Recovery is split by authority, because recovery deliberately rebuilds
`Actor` with machine and lineage only (`recover.go:304-305`) and the binding
test proves journaled human text must never become authority
(`recover_test.go:441-477`):

- **A ratified (human-authority) split does not recover; it closes
  rejected.** `completeFromIntent` gains, beside the existing `resume`
  guard (`recover.go:132-139`, the exact precedent), the rule: a `split`
  entry whose stored `Args["by"]` is nonempty terminalizes
  `OutcomeRejected` with `human ratification cannot be recovered from
  journal text; rerun goal split --by from the enrolled terminal`. This is
  safe and complete: a created-phase entry never pushed, so the parent
  stands untouched and the re-run is the whole remedy; a pushed entry is
  decided by the opid postcondition BEFORE completion is attempted
  (`recover.go:64-94`), so a split that actually landed confirms without
  rebuilding any authority.
- **A main-origin (machine) split recovers fully.** Slice 2 adds
  `case "split":` to the verb switch in `recover.go` (~line 206) that
  re-parses `Args["members"]` and calls `splitRequest`; the rebuilt
  `Mutate` re-checks the parent's origin on the tip, so even a mislabeled
  entry cannot complete a human-origin split with a machine actor.

The one-rule recovery machinery (opid postcondition, then owner liveness)
covers the rest with no new recovery logic. The round-1 claim that recovery
"rebuilds and completes" every split is withdrawn: it held only for
main-origin splits, and pretending otherwise required either weakening
human authority or an untestable fixture.

## 4. Claim-side dependency gating and the arc-invariant change (slice 3)

### 4.1 What already exists

The claim-side gate the requirement names already exists verbatim:
`claimRequest` refuses `goal <id> is blocked by <dep>, which is not done`
(`verbs.go:447–450`), `claimArcRequest` likewise per member, `Done` re-checks
it, and `ValidateTree` holds it at rest ("claimed while blocker not done",
`validate.go:218`). `goal claim --id <member>` (no `--arc`) claims exactly
one goal — per-member claiming is the existing plain-claim semantics
(`goalsync_mutations.go:467`). `goal next` already projects Ready vs Blocked
over the frontier. Slice 3 therefore does not invent the gate; it proves it
over split-created members and removes the one obstruction below.

### 4.2 The obstruction: arc uniformity

`ValidateTree` currently enforces (`validate.go:274–313`): every live member
of an arc shares one state, and a claimed arc has exactly one claimant pair.
Under that rule the FIRST independent member claim of a fresh split arc
(member 1 claimed, member 2 queued) is an illegal tree, and two machines on
two members is doubly illegal. The human's 2026-08-30 word is explicit and
newer: arcs of small goals are the parallel-execution shape, claimable per
machine; members claimable independently by any machine within recorded
dependencies. The old invariant guarded against splitting ONE claim's
ownership; split members are separate goals with separate claims, which is
precisely the sanctioned cross-machine shape (context point 1). The invariant
must change; hiding the members under a non-Arc grouping field to dodge it
would be silent narrowing of "bound to an arc".

Slice 3 changes, exactly:

1. **Delete** the arc-uniformity block (`validate.go:274–313`: the "arc
   mixes states" and "two claimant pairs" refusals). Arcs become planning
   groups with per-member state and per-member claims.
2. **Keep** the machine quota rule unchanged (`validate.go:239–272`): one
   claim per machine tree-wide, where multiple claims are legal only when all
   share one arc — under independent claiming this reads naturally as "a
   machine's claim capacity is one goal, or several members of one arc", and
   the code already expresses it without referencing uniformity.
3. **Cascade verbs stay as opt-in conveniences, with two corrections the
   mixed-arc world forces.** `ClaimArc` claims the queued members and loses
   to any foreign claim it meets; `ReleaseArc`/`ParkArc`/`UnparkArc` skip
   members not in the movable state. `Steal` is NOT already mixed-arc-safe
   as round 1 claimed: its mutation loop filters to the old pair
   (`verbs.go:1204-1207`), but its PREFLIGHT walks every arc member and
   checks every member's fence, pin, and budget (`verbs.go:1187-1198`), so
   an unrelated member held by another pair — pinned elsewhere,
   breach-stopped, or budgetless — would block a steal that would never
   move it. Slice 3 re-scopes the preflight to exactly the members that
   will move (`State == StateClaimed && ownPair(m.Claimed, oldPair)`):
   fence, pin, budget, and the new norm check (2.2) apply to movers only;
   non-movers neither block nor move. `ParkArc`'s one-displaced-pair
   collection point (`parkArcRequest`) currently records the FIRST foreign
   pair; slice 3 generalizes it to one `displaced=` marker per distinct
   foreign pair (the existing "one acknowledgment per displaced pair"
   doctrine, now with possibly several pairs).
4. **Reopen/set-arc "standing state" rules** (`reopenRequest` arc-rejoin,
   `setArcRequest` destination-join) are re-keyed from "the arc's one state"
   to mechanical per-case rules, since "the standing member" is ill-defined
   in a mixed arc: a reopened or joining member lands QUEUED unless (a) every
   live member of the arc is parked — then the existing human-only
   parked-join rule applies and it parks under the destination-record rule
   below — or (b) the caller's own pair holds a claimed member of the arc,
   every blocker of the joiner is done, and the joiner's stored tuple is
   within the norm (2.5), in which case the existing
   auto-claim-under-standing-claimant path applies to the CALLER's pair only
   (a stranger's or absent claim never auto-claims; the foreign-human
   injection row disappears with the one-claimant premise). This is the
   minimal re-keying that preserves each existing refusal's intent; the
   fixture matrix in 6.2 pins it.
5. **The all-parked destination record is defined, not implementer-chosen.**
   Members park separately after the repeal, so "the arc's record" is
   ill-defined: each live member carries its own `ParkRecord` (By, At,
   Because, Displaced — the ordinary Park verb writes one per goal,
   `verbs.go:817-821`). When rule 4(a) copies a record onto the joiner, it
   copies from the member whose `ParkRecord.At` is NEWEST, ties broken by
   the lexically smallest member id — the latest park is the latest human
   word about the arc, and the tiebreak makes the choice total. The copy
   carries By/At/Because verbatim and an empty Displaced (the joiner
   displaced nobody). The current first-encountered behavior
   (`verbs.go:1926-1932` and the parked-join arm) is an artifact of
   uniformity, where every record was interchangeable; it is replaced, not
   ratified.
6. **Reconcile's set-arc replay is re-keyed identically.** Reconcile does
   not call `setArcRequest`; it runs its own executable set-arc branch and
   picks the FIRST matching member as the standing state
   (`internal/goal/reconcilepub.go:437-529`; hand-edited Arc changes map
   there via `reconcilemap.go:305-311`). Left alone it would diverge from
   the verb the moment arcs mix. Slice 3 applies rules 4 and 5 to that
   branch verbatim — queued landing by default, all-parked human join under
   the newest-record rule, auto-claim only under the reconciling caller's
   own claimed pair with done blockers and a within-norm tuple — with
   refusals expressed as reconcile conflicts, reconcile's ordinary grammar.
7. **The frontier projection follows the repeal.** `Next` still applies the
   whole-arc pin rule: one member pinned to a foreign machine hides EVERY
   member from this machine's frontier (`internal/goal/project.go:93-121`,
   the `foreignPinnedArc` map). Under per-member claims that projection
   contradicts actual claimability. Slice 3 deletes the `foreignPinnedArc`
   computation and its suppression branch; the per-member pin filter
   (`project.go:117-119`) remains, so a foreign-pinned member hides only
   itself and its unpinned siblings surface as Ready when their blockers
   are done.
8. **Fixture proof** that the refusal names the unmet dependency: claim of a
   member with an undone sibling blocker exits nonzero with `is blocked by
   <sibling>, which is not done` on stderr, and succeeds after the sibling
   archives — from a second machine identity. Plus: a two-member arc with
   one member pinned to machine B shows the unpinned member Ready on
   machine A's `goal next`.

## 5. Failure modes

**5.1 Partial split.** Impossible on the accepted tree: one commit or
nothing (3.5). Crash windows are the journal's three phases: created-not-
pushed dies → owner-liveness rule abandons, completes from stored intent
(main-origin), or closes rejected toward a human re-run (ratified splits,
3.6); pushed-unknown → the opid postcondition decides (parent archived with
the opid = confirmed; competitor = lost). A split confirmed-late after a
competing claim of the parent is impossible in the same history — the CAS
push loses and `Mutate` re-runs, hitting the precondition table. One named
residual: between a dead owner's journal-create and recovery's completion,
the zero-spend preflight (3.3) does not re-run — accepted, because the dead
owner's clone was the only clone whose lease could admit reservations
against that claim, and its running jobs wind down under the reaper's
capDeadline; recovery completes the split against a parent whose claim was
already proven idle at journal time.

**5.2 Concurrent claim during split.** Machine B claims the parent between
A's fetch and push. A's push loses the CAS; `Mutate` re-runs on the new tip;
the parent is now claimed by a foreign pair → refusal by name toward
park/steal (3.3; no override). Symmetrically, a claim racing a landed
split finds the parent archived → `goal <id> is not live; nothing to claim`.
No window exists in which both the parent and its members are claimable.

**5.3 Member over norm.** A member's proposed tuple over the norm refuses
with the same `GOAL_NORM_REFUSED` remedy; the remedy is recursive — split
the member (it becomes a parent; its members join an arc named for IT, not
for the grandparent). The strict approval path remains available per member.

**5.4 Draft collisions and bad seams.** A member id colliding with any live
or archived goal refuses pre-publish and again at `ValidateCommit`; a
dependency cycle introduced by draft edges refuses at `ValidateCommit`
("blockedBy cycle"); both leave the parent untouched.

**5.5 Norm key misconfigured.** A malformed committed value refuses the
verb loudly (the `SliceNormHours` posture: never silently substitute the
default for the human's word); `metasystem config validate` names it.

**5.6 Approval staleness.** Any revision-bumping write between the human's
word and its use refuses with both revisions named (2.4b); re-approval is the
only path, exactly as the slice law rules.

## 6. Implementation plan for slices 2–3

### Slice 2 (Sol lane): norm + split verb

Files, in build order:

1. `internal/config/budget.go`: `GoalNormJobMinutesKey`,
   `DefaultGoalNormJobMinutes`, `GoalNormJobMinutes`,
   `parseGoalNormJobMinutes`. `internal/config/validate.go`: the three-source
   check block after line 394. `metasystem.conf`: the key at line 10.
   Tests mirror `internal/config/budget_test.go` (fixture-root override
   matrix) and `validate_test.go:232` (zero/non-numeric refusals).
2. `internal/goal/norm.go`: `refuseGoalNorm`, `StrictApprovalTriple(text,
   goalID, token)`, `RecordedNormApproval(repoRoot string, t *TreeGoals, ref,
   goalID string) (minutes, revision uint64, exists, proven bool, err error)`
   scanning `memory/rulings.md` rows and human-actor History lines on the
   loaded tree. Unit tests: triple parsing (exactly-one rule, zero refusals,
   `capMin=` triples invisible to the `minutes=` scanner and vice versa).
3. `internal/goal/file.go`: the `NormApproval *GoalNormApprovalClaim` field,
   its `NormApproval:` record-line render and parse (2.4b);
   `internal/goal/validate.go`: the config-free at-rest rule
   `NormApproval.Minutes >= Budget.ReservedJobMinutesLimit`.
4. `internal/goal/verbs.go` + `stop.go`: thread `approvedRef` through
   `SetBudget`/`setBudgetRequest`, `Claim`/`claimRequest`,
   `ClaimArc`/`claimArcRequest`, `Steal`/`stealRequest`, `Resume` (NOT
   `OpenClaim` — 2.4b); insert the norm check per the 2.2 table, including
   steal's per-mover check, the reopen/set-arc over-norm auto-claim
   refusals (2.5, slice-2 form), and write/clear of the `NormApproval`
   record at every admitting site. `internal/goal/reconcilepub.go`: the
   set-arc replay's over-norm auto-claim conflict (2.5). `recover.go` verb
   switch passes `in.Args["approvedRef"]` through.
5. `internal/goal/split.go`: `MemberDraft` struct, draft parser/serializer
   (round-trip property test), `Split(r, parentID, members, proof)`,
   `splitRequest` per section 3 (arc-name-in-use refusal included);
   `recover.go` gains `case "split":` plus the ratified-split rejection
   guard in `completeFromIntent` (3.6).
6. `cmd/metasystem/goalsync_mutations.go`: `members` and `approvedRef` fields
   on `syncFlags`, flag gating in `parseSyncFlags`, `runGoalSplit` with the
   zero-spend preflight under the goal-revision lock and the
   `humanauthority.Prove`/`RecordProof` sequence for human-origin parents
   (3.3); `cmd/metasystem/main.go`: the verb-table row.
7. `docs/backlog-mechanism.md`: the intake checklist (lines 66–84) is
   rewritten in the same slice — the "one DEPLOYABLE piece"/"small enough
   to claim as one deployable slice" bullets currently REQUIRE pre-intake
   shrinking, which now contradicts the human's law that a big goal is
   welcome at intake. Replacement wording: intent may be arc-sized at
   intake; the goal-norm refusal is the mechanical bound, and an over-norm
   goal evolves into an arc via `goal split` before slicing starts. This
   lands with the code so the governing document never contradicts the
   shipped machinery.

Fixture/test proof (Go tests against a fixture endpoint, the pattern of
`internal/goal/verbs_test.go` and `race_test.go`):

- Norm: set-budget over/at/under norm; claim binding a stored over-norm
  tuple refuses; steal of an over-norm member refuses without and passes
  with `--approved-ref`; open --claim of an over-norm tuple refuses naming
  the three-step path; approved-ref happy path via a rulings row and via a
  history-line reason writes the `NormApproval` record onto the published
  file; stale-revision refusal; the at-rest minutes rule; `.local`/env
  source refusal outside fixture roots.
- Split atomicity: one commit contains members + archived parent + rewritten
  inbound edges; a killed owner's created-phase MAIN-ORIGIN entry recovers
  to the full split, while a ratified (by-carrying) entry closes rejected
  with the parent untouched (recovery fixtures per `recover_test.go`
  patterns, the rejection mirroring
  `TestRecoveryNeverPromotesAHumanStringFromJournaledIntent`); the losing
  side of a parent-claim race classifies by name.
- Preconditions: split of an own-pair-claimed parent with a recorded
  reservation refuses naming the projection; a foreign-claimed parent
  refuses naming park/steal; an arc name already in use (live or archived
  bearer) refuses by name.
- Arc binding: every member's `Arc == parentID`; origin inheritance for both
  origins; human-origin split without proven enrolled-terminal authority
  refuses; pin and label inheritance; parent conclusion string carries
  resolvable `goal:` pointers; `ValidateCommit` green on the split tree.

Explicitly out of slice 2: any `ValidateTree` change (the split tree is
uniform at creation and needs none).

### Slice 3 (Sol lane): claim-side integration

1. `internal/goal/validate.go`: delete lines 274–313 (arc uniformity);
   quota block untouched.
2. `internal/goal/verbs.go`: `parkArcRequest` multi-pair displacement;
   `reopenRequest` and `setArcRequest` re-keyed per 4.2.4 with the
   newest-park destination-record rule (4.2.5); `stealRequest` preflight
   re-scoped to movers only (4.2.3).
3. `internal/goal/reconcilepub.go`: the set-arc branch re-keyed identically
   (4.2.6).
4. `internal/goal/project.go`: delete the `foreignPinnedArc` whole-arc
   suppression; per-member pin filter stays (4.2.7).
5. `docs/glossary.md`: the arc definition (lines 303–305, "a set of goals
   that claim and move as a whole") is rewritten in the SAME slice to "a
   planning group of goals sharing one arc name; members are claimed
   per machine within recorded dependency edges" — canon and tree change
   together.
6. Fixtures: two machine identities (the `race_test.go` two-actor pattern)
   claim two members of one split arc concurrently — both confirm;
   `ValidateCommit` green on the mixed tree; unmet-dependency claim refusal
   names the dependency verbatim; pinned member refuses the wrong machine
   while its unpinned sibling shows Ready on the other machine's frontier;
   quota fixture (machine holding a member refuses an unrelated second
   claim); cascade verbs over a mixed arc (`ClaimArc` sweeps the queued
   remainder; `Steal` moves only the stolen pair's members and is NOT
   blocked by an unrelated pinned/breach-stopped/budgetless member of
   another pair); all-parked join copies the newest park record
   deterministically; reconcile hand-join of a mixed arc matches the verb's
   outcome row for row.

Gate for both slices: the repository validation suite (run by the
orchestrator per KI-15), plus `go test ./internal/goal/... ./internal/config/...
./internal/dispatch/...`.

## 7. Non-goals

- The LOAD axis: per-machine slots, host saturation, suite timeouts —
  `machine-concurrency-governor` owns it entirely.
- The slice law: `slice-norm-hours`, `EvaluateSliceAdmission`'s observable
  behavior, reaper capDeadline, and the `capMin=` approval form all stay
  byte-identical.
- Elapsed/attempt/active-job bounds and the grace band: untouched.
- Automatic seam generation: the machinery triggers, atomizes, and enforces;
  the split CONTENT stays designed judgment (R-25), and no code proposes
  member boundaries.
- Budget assignment at split: members open budgetless by design.
- The legacy `plans/goals.md` world: split and the norm are synced-world
  only, like claim and set-budget today.
- Retro-debt, obligation, and stop-fence machinery: consumed as-is.

## 8. Conflicts for the human

1. **Arc uniformity: how far the repeal reaches is your call, and all three
   real alternatives are implementable.** Your 2026-08-30 word — members
   claimable independently per machine, bound to an arc — contradicts the
   standing at-rest invariant "an arc moves whole; one arc, one claimant"
   (`internal/goal/validate.go:274`), which encoded the older
   arc-as-one-claim model. Round 1 framed this as "repeal for all arcs, or
   invent a marker"; that framing was wrong, because the split transaction
   itself creates a durable structural discriminator with no new schema
   field: a split arc's name equals an ARCHIVED parent's id, and that
   archived file carries the split History line and the
   decomposed-with-pointer conclusion. `ValidateTree` already receives both
   the Live and Done maps (`validate.go:20-28`), so it can tell split arcs
   from hand-built ones today. The honest alternatives:
   - **(a) Repeal for all arcs** — one arc concept; pre-existing hand-built
     arcs also become per-member claimable. Simplest machinery; a global
     behavior change to arcs you built under the old model.
   - **(b) Repeal only for split-born arcs**, keyed off the archived-parent
     discriminator — old arcs keep whole-arc claiming. No schema change,
     but two arc behaviors decided by archive contents, and one real
     weakness: `goal prune` keeps only the newest N archive entries, so a
     pruned parent silently flips its arc back to whole-arc rules; (b)
     therefore also needs a prune guard for decomposed parents with live
     members.
   - **(c) A new schema marker** on members — explicit but a second arc
     concept in the file grammar.
   The design RECOMMENDS (a) for the single arc concept and because your
   word did not scope itself to split arcs, but (b) is not materially
   harder and is no longer presented as requiring an invented marker. Say
   which; slices build it.
2. **Approval token `minutes=` vs the shipped `capMin=`.** Your grammar
   "(goal= minutes= goalRevision=)" differs from the slice law's implemented
   `capMin=`. Designed as written, with the non-interchangeability rationale
   in 2.4b. If you prefer one shared token, name it.
3. **Ratification mechanism on human-origin splits.** "Split without a
   fresh human word" and "the draft is ratified by Wido" are reconciled by
   requiring `--by` PLUS the enrolled-terminal authority proof (3.3) as the
   ratification record — the same bar as `goal resume`, because a bare
   `--by` string is writable by any caller and would mint human-origin
   goals without you. Consequence you should know: ratified splits must be
   run from your enrolled terminal, and a died ratified split is re-run by
   you rather than auto-recovered (3.6). If ratification should be
   recordable some other way (for example a rulings row the agent cites by
   ref), this is the one place the design chose a mechanism you didn't
   specify.

## 9. Revision record — round 2 (gsb-design-r2, 2026-08-31)

Every round-1 finding, its disposition, and where the closure lives.
"Revised" means the finding was accepted and the design changed; no finding
was refuted. Evidence citations were re-read in the current tree during this
revision.

- **GSB-R1-001 — revised.** Human-origin split ratification is upgraded from
  an unproven `--by` string (which the command layer copies verbatim into
  `Actor.Human`, `cmd/metasystem/goalsync_mutations.go:55`) to the proven
  enrolled-terminal sequence `humanauthority.Prove` + `RecordProof`
  (`goalsync_mutations.go:347-375`), threaded into the engine as
  `Split(..., proof *humanauthority.Proof)` on the `SetObligation` precedent
  (`internal/goal/verbs.go:532`). Section 3.3 also states honestly which
  scope properties are mechanical (origin/arc/blocker/label/pin
  inheritance, full traceability) and which remain the ratifier's judgment
  per the human's own tier law. Closure: sections 3.3, 6.
- **GSB-R1-002 — revised.** Split of an own-pair-claimed parent now
  requires a zero-spend preflight: `dispatch.ProjectBudget`
  (`internal/dispatch/budget.go:237`) must show zero attempts, active jobs,
  and reserved minutes, under the same goal-revision lock reservations take
  (`cmd/metasystem/run.go:78`; resume pattern
  `goalsync_mutations.go:366`). The foreign-claim human override is REMOVED
  — the job-record evidence lives on the holder's clone, so the row refuses
  toward park/steal. The parked row's straggler-job residual and the
  recovery-window residual are named instead of hidden. Closure: sections
  3.3, 5.1, 6.
- **GSB-R1-003 — revised.** The admission table is rebuilt from the
  exhaustive `bindClaim` call-site inventory (9 sites) plus the non-binding
  tuple write, with a grep re-check instruction for the implementer. Steal
  gains the norm check per MOVER and joins the `--approved-ref` exception
  grammar; reconcile's set-arc replay (`internal/goal/reconcilepub.go:521`)
  gains the norm check with refusal as a reconcile conflict. Closure:
  sections 2.2, 2.4b, 2.5, 6.
- **GSB-R1-004 — revised.** `goal open --claim` is dropped from the
  exception grammar: the strict triple binds a positive pre-touch revision
  and `openClaimRequest` creates the file at revision 0
  (`internal/goal/verbs.go:1265-1270`), so there is no coordinate for
  approval to name. Over-norm at open --claim refuses unconditionally,
  naming the open → set-budget --approved-ref (revision 1) → claim path.
  The human's named check points (set-budget AND claim) keep their
  exception; nothing the human specified is weakened. Closure: sections
  2.2, 2.4b, 6.
- **GSB-R1-005 — revised.** The contradiction (checks on auto-claim paths,
  exception flag absent there) is resolved by a total rule: over-norm
  stored tuples NEVER auto-claim. Slice 2: reopen/set-arc rejoin refusals
  (uniformity still standing); slice 3: the re-keyed joins land such
  members queued, and the bind happens through `goal claim --approved-ref`.
  The round-1 sentence "existing auto-claim behavior remains" is withdrawn
  for over-norm tuples. Closure: sections 2.5, 4.2.4, 6.
- **GSB-R1-006 — revised.** Approval is now published: `GoalFile` gains
  `NormApproval *GoalNormApprovalClaim` (ref, minutes, named revision),
  the goal-side mirror of `SliceApprovalClaim`
  (`internal/dispatch/slice.go:35-40`), written in the same accepted commit
  that binds the over-norm tuple and cleared by a within-norm write, plus a
  config-free at-rest rule (approval minutes cover the stored tuple). The
  clone-local journal (`internal/goal/journal.go:104-106`) is no longer the
  only record. Closure: sections 2.4b, 6.
- **GSB-R1-007 — revised.** Split recovery is split by authority on the
  existing `resume` precedent (`internal/goal/recover.go:132-139`): a
  by-carrying split entry closes `OutcomeRejected` toward a human re-run
  (parent untouched — a created-phase entry never pushed; a pushed one is
  decided by the opid postcondition before completion is attempted), and
  only main-origin splits complete from stored intent. The fixture matrix
  now tests both outcomes; the round-1 "full recovery" claim is withdrawn.
  Closure: sections 3.6, 5.1, 6.
- **GSB-R1-008 — revised.** The "parent id is free as an arc name"
  assertion is withdrawn (arc names have no registry:
  `internal/goal/file.go:358-359`, `verbs.go:1866-1868`,
  `validate.go:279-313`). Split's `Mutate` refuses when any live OR
  archived goal other than the parent already carries `Arc == parentID`
  (archived included because reopen would rejoin the old arc), re-checked
  on every rebuild. Closure: sections 3.5.2, 6.
- **GSB-R1-009 — revised.** Slice 3 now includes the projection:
  `internal/goal/project.go`'s `foreignPinnedArc` whole-arc suppression
  (`project.go:93-121`) is deleted; the per-member pin filter stays; a
  fixture proves the unpinned sibling surfaces as Ready on the other
  machine. Closure: sections 4.2.7, 6.
- **GSB-R1-010 — revised.** The round-1 claim that steal is already
  mixed-arc-correct is withdrawn: its preflight walks every member
  (`internal/goal/verbs.go:1187-1198`) before the mover filter
  (`verbs.go:1204-1207`). Slice 3 re-scopes fence/pin/budget/norm checks to
  movers only, with a fixture where an unrelated pinned/breach-stopped/
  budgetless member of another pair does not block the steal. Closure:
  sections 4.2.3, 6.
- **GSB-R1-011 — revised.** Reconcile's executable set-arc branch
  (`internal/goal/reconcilepub.go:437-529`, reached from
  `reconcilemap.go:305-311`) is re-keyed identically to `setArcRequest` in
  slice 3 (queued default, newest-park record, caller's-own-pair auto-claim
  with done blockers and within-norm tuple), refusals as conflicts;
  `reconcilepub.go` joins the slice-3 file list, and a fixture pins
  verb/reconcile row-for-row agreement. Closure: sections 4.2.6, 6.
- **GSB-R1-012 — revised.** The all-parked destination record is now a
  mechanical rule: copy the member record with the newest `ParkRecord.At`,
  ties to the lexically smallest member id; By/At/Because verbatim,
  Displaced empty. The current first-encountered behavior
  (`internal/goal/verbs.go:1926-1932`) is named as a uniformity artifact
  and replaced, not ratified. Closure: sections 4.2.5, 6.
- **GSB-R1-013 — revised.** Conflict 1 is reframed with the real
  alternatives: the archived decomposed parent (name-equal to the arc,
  split history, member pointers) IS a durable structural discriminator
  available to `ValidateTree` (which receives Live and Done,
  `validate.go:20-28`), so scoping the repeal to split arcs needs no
  invented marker. The design still recommends the global repeal, but on
  its merits (one arc concept; the human's word was not scoped; option (b)
  needs a prune guard because `goal prune` can delete the discriminator),
  and the choice is presented as genuinely open. Closure: section 8.1.
- **GSB-R1-014 — revised.** The implementation plan gains the governing
  documents: slice 2 rewrites the intake checklist in
  `docs/backlog-mechanism.md` (the pre-intake slicing and one-slice bullets
  at lines 66–84 contradict "a big goal is welcome at intake"), and slice 3
  rewrites the arc definition in `docs/glossary.md` (lines 303–305,
  whole-claim arcs) in the same slice as the uniformity repeal, so canon
  and tree never disagree. Closure: sections 6 (slice 2 item 7, slice 3
  item 5).

## 10. Unresolved residue — Sol round 2 (goal:goal-scope-bounds)

The critique chain ran two verification rounds (14 -> 7 material
findings). Per the failsafe declared at loop start (one revision, one
verification, then land), the design lands with the seven findings
below recorded verbatim rather than spending the implementation
attempts on further convergence. Each is BINDING on the implementation
slices: closed or refuted with evidence, and where a finding
contradicts design text, THE FINDING TAKES PRECEDENCE - in particular
GSB-R1-001 (coordinator ratification must be enforced, not replaced by
lineage possession) and GSB-R1-002 (split-before-slicing is a hard rule
with no parked-parent bypass), which are requirement-grade. GSB-R1-013
(section 8 framing) is carried to the human directly alongside this
landing rather than through another revision.

### GSB-R1-001 (high)

The revised authorization design still replaces the required coordinator ratification for a main-origin split with possession of an ordinary agent lineage. It explicitly lets an ordinary agent-holder invoke the split without a proof, while the existing command accepts a supplied lineage and constructs an actor without proving that the coordinator ratified Fable's proposed member graph. An implementer following this design would authorize arbitrary eligible agents to make a judgment the human reserved for the coordinator.

Evidence: The supreme requirement is at metasystem/plans/goals/goal-scope-bounds.md:4-6. The substitution is specified at metasystem/plans/goal-scope-bounds-design.md:376-392, and the proof fixtures cover only human authority at metasystem/plans/goal-scope-bounds-design.md:752-755. The existing command accepts lineage and constructs the actor without coordinator ratification at metasystem/cmd/metasystem/goalsync_mutations.go:26-70.

### GSB-R1-002 (high)

The proposed zero-spend rule still does not enforce the hard requirement that splitting precede slicing. It expressly permits a parked parent with residual work from an earlier job and a previously claimed then released parent with historical spend. Its current-state projection cannot establish that slicing never began, and its command-side revision lock does not bind the transaction engine: SetBudget can change and rebind the claimed revision without that lock, while transaction mutation is retried against newer tips. Recovery also publishes without rerunning the preflight. The design therefore permits both known historical slicing and a concurrent race past the check.

Evidence: The hard ordering requirement is at metasystem/plans/goals/goal-scope-bounds.md:4-6. The parked residual exception, historical-claim allowance, acknowledged lack of engine enforcement, and recovery behavior are at metasystem/plans/goal-scope-bounds-design.md:313-349 and metasystem/plans/goal-scope-bounds-design.md:648-653. SetBudget changes the revision and rebinds the claim at metasystem/internal/goal/verbs.go:512-520. Mutations retry on captured tips and publish by compare-and-swap at metasystem/internal/goal/txn.go:613-690. Budget projection only scans the current claimed revision at metasystem/internal/dispatch/budget.go:234-376. The revision lock is checkout-local at metasystem/internal/goalrevision/lock.go:23-30 and metasystem/internal/goalrevision/lock.go:125-145, and the dispatcher uses that local lock at metasystem/scripts/agents/dispatch.sh:436-468.

### GSB-R1-013 (medium)

Section 8 still does not present its second human alternative honestly. It says a split-only discriminator can be preserved by guarding pruning only while decomposed parents have live members. The implemented pruner does not retain goals through arc membership, so it can delete the decomposed parent after all members become archived even though an archived member can later be reopened with its arc intact. At that point the proposed discriminator has disappeared. The human would be choosing an alternative whose stated preservation mechanism is insufficient.

Evidence: The alternative and its live-member-only guard are at metasystem/plans/goal-scope-bounds-design.md:828-840. Pruning retains completed goals through live dependency edges and recency, not arc membership, at metasystem/internal/goal/verbs.go:1290-1371. Reopen restores an archived goal while retaining its arc at metasystem/internal/goal/verbs.go:890-999.

### GSB-R1-014 (high)

The documentation sweep remains incomplete and would leave governing instructions that contradict the human's before-slicing rule. The design updates only the intake checklist and the glossary's arc definition, while the backlog mechanism and glossary still state generally that large work is divided by slicing. An implementer following the stated file plan would ship conflicting operational norms.

Evidence: The human requires goal splitting before slicing at metasystem/plans/goals/goal-scope-bounds.md:4-6. The limited documentation edits are at metasystem/plans/goal-scope-bounds-design.md:721-729 and metasystem/plans/goal-scope-bounds-design.md:772-776. The conflicting general slicing rule remains at metasystem/docs/backlog-mechanism.md:42-49, and the glossary still directs large work into slices at metasystem/docs/glossary.md:312-314.

### GSB-R2-001 (high)

A decomposed parent is not made permanently non-claimable. The design archives it and claims that no state can make both parent and members claimable, but the existing Reopen verb has no decomposed-parent or Split-history guard: it can restore an archived parent to queued status and clear its conclusion while its split members remain live. That recreates the original whole goal beside its atomized members and permits duplicate scope to be claimed.

Evidence: Parent archival and the no-dual-claimability assertion are at metasystem/plans/goal-scope-bounds-design.md:458-473 and metasystem/plans/goal-scope-bounds-design.md:655-660. The existing Reopen path restores archived goals to queued status, clears the conclusion, and contains no split-parent prohibition at metasystem/internal/goal/verbs.go:890-999. The required terminal decomposed-parent outcome is stated at metasystem/plans/goals/goal-scope-bounds.md:4-6.

### GSB-R2-002 (medium)

The mixed-arc cascade design does not cover ReleaseArc even though it promises that cascade verbs skip members that are not movable. The existing ReleaseArc skips unclaimed members but rejects the entire cascade when a sibling is legitimately claimed by another actor. The implementation plan names changes for ParkArc, UnparkArc, Steal, and reconciliation but no ReleaseArc change, so an implementer would leave a promised mixed-arc operation unusable.

Evidence: The skip-nonmovable contract is at metasystem/plans/goal-scope-bounds-design.md:568-584. The implementation work list omits ReleaseArc at metasystem/plans/goal-scope-bounds-design.md:762-769. The current ReleaseArc rejects a foreign-held claimed sibling rather than skipping it at metasystem/internal/goal/verbs.go:1514-1558.

### GSB-R2-003 (medium)

Splitting a parent that is already the final live member of an older arc bypasses the older arc's retroactive-debt transition. The design says removing that parent from its old arc behaves like a normal conclusion, but Split archives the parent directly and specifies no equivalent of Done's last-live-member debt raise. The old arc can therefore finish through Split without recording the debt that the implemented normal-conclusion path records.

Evidence: The claimed equivalence to normal conclusion and the direct parent archival are at metasystem/plans/goal-scope-bounds-design.md:442-473. The implemented Done verb checks whether the archived goal was the final live arc member and raises retroactive debt at metasystem/internal/goal/verbs.go:634-679. The proposed Split mutation contains no corresponding old-arc debt operation in metasystem/plans/goal-scope-bounds-design.md:418-482.

## 11. Addendum - mechanisms for the slice-2 gaps (gsb-design-r3)

The Sol-lane slice-2 implementer gap-stopped on four points the design
left to judgment. This section closes each with one chosen mechanism,
buildable without further decisions. It is append-only: where it
extends an earlier section it says so; the SS10 finding-over-text
precedence stands, and nothing here weakens a refusal path — every
mechanism only adds refusals or makes an existing one durable. Every
symbol and line cited below was re-read in the current tree during
this addendum.

### 11.1 Gap 1 (GSB-R1-001) — coordinator ratification is a holder-classified, draft-digest-bound token

The chosen mechanism is the ClaimEpoch pattern, extended with a draft
digest: the command layer mints a ratification token only under an
authenticated MAIN-holder (or human) lease classification, the engine
requires the token and independently re-verifies the digest against
the member definitions it is about to publish, and the accepted
commit records the whole triple. One-sentence reason for the choice:
the coordinator's ratification act IS invoking split over the draft
it just read, so an out-of-band ratification reference would add a
second durable channel that bottoms out in the same lease
authentication — the digest, not a reference, is what binds the act
to THESE member definitions.

- **Record shape.** New struct in `internal/goal/split.go`:
  `SplitRatification{Tier string, MainID string, ClaimEpoch int64,
  DraftSHA256 string}`. `Tier` is closed: `"human"` or `"main"`.
  `DraftSHA256` is the lowercase sha256 hex of the canonical draft
  serialization already defined in 3.6 (the re-rendered parsed
  structs — the same bytes the journal carries).
- **Command-layer minting (the who).** `runGoalSplit` classifies its
  caller with `lease.ClassifyVerb(root, ppid)` — the exact call
  `syncReq` already makes (`cmd/metasystem/goalsync_mutations.go:59`).
  For a MAIN-ORIGIN parent it refuses unless the classification is
  `Class == lease.ClassMain && Holder == true` (the checkout-lease
  holder: `Holder` is true only when the caller's `MainId` equals the
  lease's `HolderMainId`, `internal/lease/verbs.go:305` — the
  coordinator session by construction) or `Class == lease.ClassHuman`
  (a human may always run it, matching 3.3). Every other class —
  `DELEGATE`, `SUPERVISION`, `UNTRUSTED`, the full set at
  `internal/lease/classify.go:63-73` — refuses. Refusal wording:
  `SPLIT_RATIFY_REFUSED: goal <id> is main-origin; its split draft is
  ratified by the coordinator — re-run goal split from the MAIN
  checkout-lease holder session (a human may also run it with --by)`.
  On success the CLI fills the token: `Tier: "main"`, `MainID` and
  `ClaimEpoch` from the classification, `DraftSHA256` computed over
  the canonical serialization of the draft it parsed. For a
  HUMAN-ORIGIN parent the 3.3 `humanauthority.Prove`/`RecordProof`
  sequence is unchanged and the CLI fills `Tier: "human"` plus the
  same `DraftSHA256`.
- **Engine requirement (the what).** The signature becomes
  `Split(r, parentID, members, ratification SplitRatification, proof
  *humanauthority.Proof)`. Inside `Mutate`, re-run on every rebuilt
  tip: a main-origin parent requires `Tier == "main"`, nonempty
  `MainID`, and `ClaimEpoch >= 1`, exactly the bar `bindClaim` sets
  for the epoch it cannot itself authenticate
  (`internal/goal/verbs.go:110-113` — the engine requires the token;
  only the holder-classified command path mints it,
  `goalsync_mutations.go:59-69`). A human-origin parent requires
  `Tier == "human"`, non-nil proof, and nonempty `r.Actor.Human`
  (3.3 unchanged). BOTH tiers require the digest re-verification:
  the engine re-serializes the `members` it is about to publish and
  refuses on mismatch — `SPLIT_RATIFY_REFUSED: the ratified draft
  digest <first-8-hex> does not match the member definitions being
  published (<first-8-hex>); re-run goal split with the ratified
  draft`. This is what makes the ratification draft-specific rather
  than command-specific: no code path can publish member definitions
  other than the ones the ratifier's invocation hashed.
- **Published proof record.** The archived parent gains one record
  line, rendered/parsed with the `parseKVRecord` conventions of
  `internal/goal/file.go`:
  `Ratified: tier=<human|main> by=<name> mainId=<id> claimEpoch=<e> draftSha256=<64-hex>`
  — `by` present exactly when tier=human; `mainId` and `claimEpoch`
  present exactly when tier=main; `draftSha256` always. Grammar-level
  validation enforces that presence rule and the 64-hex shape. The
  line lands in the same accepted commit as the members, so every
  machine that pulls sees who ratified which exact draft bytes next
  to the decomposed-with-pointer conclusion.
- **Journal and recovery.** Intent args gain `ratifierTier`,
  `ratifierMainId`, `ratifierClaimEpoch`, and `draftSha256` beside
  the existing `by` and `members`. Recovery of a main-origin split
  replays them on the claimEpoch precedent — stored intent already
  lawfully carries the epoch back into the engine
  (`internal/goal/recover.go:189-195`) — and the rebuilt `Mutate`
  re-runs the digest check over the re-parsed `Args["members"]`, so
  a journal entry whose stored members were doctored after the
  ratifying invocation closes rejected instead of completing. The
  3.6 rule for by-carrying (human-ratified) entries is unchanged.
- **Fixture.** A main-origin split invoked under a non-holder
  classification refuses with `SPLIT_RATIFY_REFUSED` (fixture-side
  classification injection per the existing lease fixtures); under a
  holder classification it lands and the archived parent's
  `Ratified:` line carries tier=main and a digest equal to sha256 of
  the canonical draft; a recovery fixture with mismatching stored
  members and `draftSha256` terminalizes `OutcomeRejected` naming
  the digest.

### 11.2 Gap 2 (GSB-R1-002) — the ever-sliced fact is a `Sliced:` record on the goal file, written by dispatch before the first reservation exists

The chosen mechanism is a first-write-wins record published INTO the
shared goal ledger at first job-reservation admission, so split's own
`Mutate` — not a checkout-local lock — enforces the hard rule.
One-sentence reason: the fact must be visible to split's transaction
on every rebuilt tip, survive revision changes, machines, and
garbage collection, and the goal file is the only store already
inside the CAS transaction domain with all three properties (job
records are clone-local and reaped; `ProjectBudget` scans only the
current claimed revision, `internal/dispatch/budget.go:234-376`).

- **Record shape.** `GoalFile` (`internal/goal/file.go:21`) gains
  `Sliced *SlicedRecord{Machine string, Lineage string, Revision
  uint64, At string}`, rendered
  `Sliced: machine=<m> lineage=<l> revision=<r> at=<iso8601>` — the
  coordinates of the goal's FIRST admitted job reservation, ever.
  Written once; NO verb clears or rewrites it: release, set-budget,
  park, reopen, steal, and edit all carry it verbatim (edit's closed
  field set already refuses unknown deltas), and reconcile maps any
  hand edit of the line to a conflict — the unrepresentable-change
  posture. Grammar validation: positive `Revision`, RFC3339 `At`.
  Deliberately NO at-rest rule ties `Sliced` to state — a released
  or re-queued goal lawfully carries it; that is the point.
- **Writer and check point.** A new internal engine verb
  `MarkSliced(r VerbRequest, id string)` /
  `sliceStartRequest(r, id)` (History verb `slice-start`), with no
  CLI mount. Its `Mutate`: the goal must be live and claimed by
  `r.Actor`'s pair; `Sliced != nil` classifies `NothingToDo`
  (idempotent); otherwise it writes
  `{r.Actor.Machine, r.Actor.Lineage, f.Claimed.Revision, r.stamp()}`
  plus `touch`. The ONE caller is `dispatch.ClaimLaunch`
  (`internal/dispatch/claim.go:202`), the reservation-creating verb
  both dispatch legs funnel through
  (`scripts/agents/dispatch.sh:1477-1479` fresh, `:1939-1941`
  follow-up, invoking claim-launch at `:1489-1501`): when the launch
  carries a goal binding, after the admission verdicts pass and
  BEFORE the reservation record is written, it reads the accepted
  tree and, if `Sliced` is absent, publishes slice-start
  (`internal/dispatch` already imports `internal/goal` —
  `internal/dispatch/admission.go:9` — so no cycle). Fail closed: a
  slice-start that does not confirm (or classify `NothingToDo`)
  refuses the launch — `SLICE_START_UNRECORDED: goal <id>'s
  first-slicing fact could not land on the shared ledger; the
  reservation is refused` — no fact, no reservation, no job. Cost,
  stated: the first dispatch against each goal performs one ledger
  publish, and dispatching the first slice while the ledger remote
  is unreachable refuses like any other ledger mutation.
- **Split enforcement, engine-side.** Split's `Mutate` gains, ahead
  of the 3.3 precondition table and re-run on every rebuild:
  `Sliced != nil` refuses in EVERY parent state, parked included
  (the SS10 no-parked-parent-bypass requirement):
  `GOAL_SPLIT_REFUSED: goal <id> recorded its first slice
  (machine <m>, revision <r>, <at>); split is a before-slicing act
  and slicing has begun — conclude the goal (schedule residue per
  R-4) and open successor goals instead`. The finding's races close
  structurally: split and slice-start are CAS transactions on the
  same accepted branch, so exactly one of {split, first reservation}
  wins any interleaving — a split landing first archives the parent
  and slice-start's rebuilt `Mutate` refuses (so ClaimLaunch refuses
  the reservation); a slice-start landing first is seen by split's
  rebuilt `Mutate` (`internal/goal/txn.go:613`, mutate-per-capture)
  and refuses the split. SetBudget's revision re-bind and recovery's
  replay change nothing: the check reads the tree, not a lock, and
  recovery runs the same `Mutate`.
- **Reach of the rule, stated.** The refusal keys on `Sliced` ONLY.
  A goal that was claimed — even claimed, worked zero jobs, and
  released — still splits; a goal with `Sliced` never splits again,
  in any state, forever. The 3.3 zero-spend `ProjectBudget`
  preflight is RETAINED unchanged as a command-side defense for
  exactly one residual: goals whose slicing predates this record's
  existence (no backfill is possible or attempted). The `Sliced`
  check is the enforcement of record; 3.3's preflight is legacy
  defense, and 5.1's recovery-window residual paragraph is
  superseded for the sliced case — recovery re-runs the check.
- **Recovery.** `case "slice-start":` joins the verb switch
  (`internal/goal/recover.go:205`) rebuilding `sliceStartRequest`; a
  created-phase entry that dies abandons harmlessly because the
  fail-closed ordering means its reservation never launched.
- **Fixture.** First goal-bound ClaimLaunch publishes the record;
  a second launch publishes nothing (tree unchanged, `NothingToDo`);
  split of the sliced goal refuses naming machine, revision, and
  timestamp — also when the goal is parked or was released; split of
  a claimed-never-sliced goal still passes 3.3's preflight; a raced
  split/slice-start pair (the `BeforePush` fixture seam,
  `internal/goal/txn.go:484-487`) confirms exactly one winner with
  the loser classified by name.

### 11.3 Gap 3 (GSB-R2-001) — permanence is a root-record decomposition registry

Of the three named options, the chosen mechanism is the ROOT
DECOMPOSITION REGISTRY. One-sentence reason: the root record
(`plans/goals/backlog.md`) is the one ledger file no prune deletes
and it is already parsed into every `Mutate` and into `ValidateTree`
(`TreeGoals.Root`, `internal/goal/validate.go:21-28`), so one
appendix there guards reopen, open, open-claim, and the at-rest
tree without changing prune's retention contract (tombstone
retention couples permanence to `keep`-count semantics, and
GSB-R1-013 already proved that guard shape insufficient;
coordinated prune/open guards leave the fact spread across verb
behavior with no at-rest witness).

- **Record shape.** `RootRecord` (`internal/goal/root.go:17`) gains
  `Decomposed []DecomposedEntry{Id, Opid, At string}`, rendered as a
  new section parallel to `LegacyNotes:` (parse loop
  `root.go:62-88`, render `root.go:196-229`):

  ```
  Decomposed:
  - <parent-id> opid=<opid> at=<iso8601>
  ```

  Parse strictness: kebab `validId`, opid shape, RFC3339 stamp, and
  a duplicate parent id refuses the tree by name. Entries are
  append-only and render sorted by `At` then id. Growth is one line
  per split forever — accepted and stated.
- **The one writer.** Split's `Mutate` appends the entry
  `{parentID, r.opid(), r.stamp()}` and bumps `Root.Revision` with a
  root History line in the SAME `[]Change` set as the members and
  the archived parent — the exact root-write shape the Goal-free
  clear already uses (`internal/goal/verbs.go:378-386`). No verb
  removes an entry. Prune's mutation, which already rewrites the
  root for its own History line (`verbs.go:1363-1368`), carries the
  section through untouched.
- **The guards.** All inside `Mutate`, re-run per rebuild, via one
  helper `rootDecomposed(t.Root, id) bool`:
  - `reopenRequest`, after the archive lookup (`verbs.go:915-918`):
    a registered id refuses — `goal <id> was decomposed into arc
    <id>; a decomposed parent never returns — reopen or claim its
    member goals, or open a NEW goal under a new id`. This also
    closes the pre-prune window the finding proved
    (`verbs.go:890-999` has no split guard today).
  - `openRequest` (`verbs.go:362-370`) and `openClaimRequest`: a
    registered id refuses — `goal id <id> is retired: it names a
    decomposed parent (split opid <opid>); pick a different id`.
    This closes the prune-then-recreate hole: the identifier stays
    retired after `Prune` (`verbs.go:1290-1373`) deletes the
    archived parent, because the registry never prunes.
  - Split's own draft validation (3.2) additionally refuses member
    ids present in the registry.
- **At-rest rule** (config-free, joins `ValidateTree`): no LIVE
  goal's id may appear in `Root.Decomposed` — a violating tree is
  defective by name ("decomposed parent <id> is live again"). The
  archived parent itself remaining in `Done` is legal until pruned.
  Reconcile maps hand edits that add or remove registry entries, or
  that resurrect a registered id as a live file, to conflicts.
- **Amendment to 8.1(b), honest per GSB-R1-013.** If Wido selects
  the split-scoped repeal, its discriminator becomes membership in
  `Root.Decomposed` rather than presence of the archived parent —
  the registry is exactly the durable structural fact whose absence
  made option (b)'s prune guard insufficient. Option (b) no longer
  needs any prune guard; its remaining cost is only the two-behavior
  arc concept.
- **Fixture.** Split, then `prune --keep 0` (members hold no
  blocker edge to the parent, so the archived parent dies): `goal
  open --id <parent>` refuses naming the registry and the split
  opid; `goal reopen --id <parent>` before the prune refuses naming
  decomposition; a hand-built tree with a live goal named in
  `Decomposed:` fails `ValidateCommit`.

### 11.4 Gap 4 (GSB-R2-003) — the old-arc debt raises before the split's journal entry may terminalize confirmed

The chosen mechanism mirrors Done's last-live-member classification
but moves the raise INSIDE the confirmation ordering, so a
terminal-confirmed split entry PROVES the debt landed.
One-sentence reason: recovery can only hang an effect on a
non-terminal journal entry, so any raise that happens after
`MarkTerminal(OutcomeConfirmed)` (as Done's does today,
`internal/goal/verbs.go:659-679` running after
`internal/goal/txn.go:672`) is unrecoverable by construction — the
ordering, not the mirroring, is what the finding demands.

- **What raises.** The archived parent retains its `Arc` field
  verbatim (11.4 makes 3.5.4 explicit on this: only `Blocked`,
  `Parked`, and the claim binding clear — matching Done's archive
  shape). The debt condition and coordinates are Done's exactly
  (`verbs.go:667-676`): on the confirmed tip, if the archived
  parent's old `Arc` is nonempty and NO live goal carries it,
  `retrodebt.Raise(root, retrodebt.KindArc, archived.Arc+":"+opid,
  now)`. Split members carry `Arc == parentID`, not the old arc, so
  they never mask the check; a parent whose old arc still has live
  siblings raises nothing; a parent with `Arc == ""` raises nothing.
  `Raise` is idempotent by `kind:source`
  (`internal/retrodebt/debt.go:168-173`), so re-running the
  classification at every observation point below is free.
- **The transaction hook.** `PublishRequest`
  (`internal/goal/txn.go:467`) gains
  `AfterConfirmed func(tip string) error`, nil for every existing
  verb. In the CAS-landed leg it runs after the postcondition
  verifies and BEFORE `MarkTerminal(OutcomeConfirmed)`
  (`txn.go:666-672`); on error the entry STAYS pushed and the verb
  returns the error — precisely the existing stays-pushed posture of
  a failed confirming refetch (`txn.go:661-671`). The
  `AlreadyApplied` replay leg (`txn.go:722`) runs the hook before
  its confirm-mark under the same rule. `splitRequest` sets the hook
  to the classification above; `Split`'s wrapper needs no post-
  publish debt step at all.
- **Atomicity answer, stated.** Tree atomicity is untouched — the
  split commit stands whole on the accepted branch. A debt failure
  or a crash in the window leaves the journal entry PUSHED, which
  blocks this clone's further ledger mutations until classified
  (`txn.go:507-510`) — a disclosed, resumable stop, never a torn
  split and never a silently forgotten debt.
- **Recovery completes the effect.** For an entry whose
  `Intent.Verb == "split"`, a shared helper
  `raiseSplitOldArcDebt(e, tip, entry)` (parent =
  `entry.Intent.Targets[0]`; load the tip; run the classification)
  runs before the terminal mark at each recovery confirmation
  point: `ActionConfirm` and `ActionConfirmLate`
  (`internal/goal/recover.go:75-94`), and `completeFromIntent`'s
  path inherits the hook through the rebuilt `splitRequest`. The
  world may have moved by recovery time; the rule is honest about
  it: the classification runs against the tip the opid was found
  on, and if a goal has meanwhile rejoined the old arc, the arc is
  live again and its debt lawfully raises when ITS last member
  concludes — deferring to the arc's true conclusion is Done's own
  semantics, not a loss. The debt register is per-checkout state
  under `memory/` (`internal/retrodebt/debt.go:55-57`;
  `stateroot.Registers` resolves to `memory`), the same locality
  Done's raise already has.
- **Done itself is deliberately unchanged in slice 2.** Migrating
  Done onto the hook would strictly narrow the identical
  pre-existing crash window and is recorded as a follow-up backlog
  candidate, not smuggled into a split slice; this addendum may not
  widen slice 2's boundary beyond its gaps.
- **Fixture.** Splitting the final live member of arc `alpha`
  raises `arc-goal:alpha:<opid>` (mirroring the Done coverage at
  `internal/goal/verbs_test.go:146-153`); an injected one-shot
  `AfterConfirmed` failure leaves the entry pushed, and a
  subsequent `Recover` classifies it, raises the debt, then
  terminalizes — with exactly one debt entry existing afterwards
  (idempotence); a split whose parent has a live old-arc sibling
  raises nothing.

### Slice-2 build-order deltas

Section 6's slice-2 list is amended, not rewritten: item 3 adds the
`Sliced:` and `Ratified:` record grammar and the root `Decomposed:`
section with its at-rest rule; item 4 adds `MarkSliced` and the
open/reopen/open-claim registry guards; item 5 adds
`SplitRatification`, the digest check, the split-side `Sliced` and
registry refusals, the root-registry append, and the
`AfterConfirmed` hook with its `raiseSplitOldArcDebt` helper (plus
the `PublishRequest` field and its two txn call sites and two
recovery call sites); item 6 adds the holder classification gate in
`runGoalSplit` and the `internal/dispatch/claim.go` slice-start call.
The fixtures named per mechanism above join the slice-2 proof list.
