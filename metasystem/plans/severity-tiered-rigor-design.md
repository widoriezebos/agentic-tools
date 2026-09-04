# severity-tiered-rigor — design round 1 record (2026-08-26)

Sketch critiqued by codex; VERDICT needs-rework; 10 findings + minimal slice one (4h). Queued per Wido's ruling — shovel-ready for the claimant. Round budget: 2 remaining of 3.

SEE SESSION RECORD: round-1 verdict delivered 2026-08-26 ~13:20; re-run design round if lost.

## Round-1 findings (compressed; prescriptions binding for the build)
1. Critic-shopping: conformance passes on ONE clean chain (conformance.go:947,1045). Bind one authoritative chain per subject; multi-chain same-tree unions findings; class conflicts block and raise.
2. out-of-scope laundering: critiqueclosed.go:31,215 lets material findings close out-of-scope. SEVERE and UNPROVEN must reject out-of-scope; scope evidence may justify reclassification, never closure of a still-severe finding.
3. Class field: do NOT overload existing severity (design-critic.schema.json:43). New rigorClass: severe|bounded|unproven; UNPROVEN lands like SEVERE (fail closed — unknown frequency guessed into bounded is how severe passes); BOUNDED needs structured evidence: local, recoverable, no proof/authority/secrets/irreversible-data/external-side-effect boundary crossed.
4. Finding register: exhaustion reads only the latest return (dispatch/critique.go:78); later critics can omit/rename/downgrade. Canonical finding register on the chain root (stable id, critic, class, facts, status, evidence digest); downgrades are disputes for the original critic or human.
5. No uncapped class: keep finite caps for ALL classes — bounded shorter, severe/unproven the existing 3+3 (SKILL.md:36, critique.go:13); exhaustion blocks and raises, never authorizes more critique.
6. Enforcement home: job critique-exhaustion is the owner; route critique-round.sh through it (it bypasses today, critique-round.sh:9); landing separately consumes an exact-tree critique-exit certificate checked at conformance AND commit.sh.
7. Residual landing conflicts with existing exits (design early-exit = fixtures-folded only, SKILL.md:52; conformance refuses material findings :995). Separate design/code exits; code residual landing needs its own human-authorized exact-tree exception design — NOT in slice one.
8. Cheap unblock: gates must type alternatives invariant-preserving vs risk-accepting; only the former auto-defaults; risk acceptance requires human authority; debt-goal creation idempotent and BEFORE the unblock.
9. Near-miss register does not exist yet (queued; its law forbids blocking on recording). Slice one must not depend on it; recurrence invalidates BOUNDED to UNPROVEN (frequency evidence), promotes to SEVERE only on a severe invariant.
10. State home: chain class/finding state/round counters live on the job-chain root beside critiqueExhaustions; labels index only; appetite stays the outer human budget; weight stays battery-only.

## Minimal slice one (4h appetite, hard stop + raise)
rigorClass + structured facts + reopening trigger required on every material finding (missing/malformed → UNPROVEN); first all-bounded round recorded on the chain root with finding ids; max two further rounds; severe overrides the bounded deadline but keeps the existing second-exhaustion stop; job critique-exhaustion refuses at either boundary with critique-round.sh routed through it; zero-material code-landing rule preserved; bounded exhaustion's only outcome is a loud human raise. Residual landing, cheap unblocks, debt automation, near-miss promotion: LATER slices.

## Round 2 record (one-more-round; round 3 of 3 decides)
Slice-one scope as written is NOT buildable: prescriptions 3 and 6
need code-level grammar first. Five new findings bind round 3:
(1) dispatch.sh:1261-1308 strand/collide window — exhaustion decided
after pending-setup exists but before the cleanup trap; (2) empty
finding IDs are schema-valid and dropped by openMaterialIDs —
the exhaustion test then sees no open IDs (schema :45,
critique.go:103-118,218-223); (3) naive schema-required rigorClass
livelocks via protocol-error rounds that never count toward the cap
(dispatch.sh:1237-1246, critique.go:182-190); (4) the canonical
register needs a digest/version precondition or an atomic
register-update verb — RecordCAS compares only status
(critique.go:174-182, record.go:283-346); (5) critique-round.sh never
implemented its recorded stop-mechanism contract (only model/effort
options; no exhaustion consultation). Round 3 must pin: the
classification/default grammar at code level; malformed rounds
cap-binding; the driver migration choice; the exact-tree certificate;
atomic register mutation; the honest slice split (8 and 9 stay later).

## Round 3 verdict: CONVERGED-BUILDABLE at an honest 31 hours
The deciding round pinned all six obligations and produced a complete
six-task build list — but the honest slice-one estimate is 31h, not
the original 4h token. Per the appetite law the resize is Wido's
call: the design is DONE and binding (task list in the round-3 record
below); the build starts only on his appetite. Tasks: (1-2)
classification grammar + cap-binding malformed rounds (schema v3,
register, exhaustion owner); (3) driver retirement + dispatch fixture
legs; (4) closure/disposition semantics 4h; (5) same-tree union +
exact-tree certificate 7h; (6) docs + integrated proof 3h.
Prescriptions 7-9 remain later slices.
. Evidence: [`close.go:10-39`](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/dispatch/close.go:10) and [`close.go:41-124`](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/dispatch/close.go:41).

VERDICT: CONVERGED-BUILDABLE

- Task 1 — v3 wire grammar and typed classifier, 5h. Add `internal/critique/model.go` with the types and normalization rules above; extend `internal/returnschema/returnschema.go`, `cmd/metasystem/schema.go`, `internal/validate/returncomplete.go`, `internal/adapter/return.go`, `internal/adapter/fake.go`, `scripts/agents/adapters/runtime-common.sh`, and the three critic role preambles. Keep checked-in v1 schemas unchanged; materialize v3 only for design-critic, code-critic, and warden. Fixture legs: zero-material/empty-rigor, lawful bounded/severe/unproven, missing rigor row, extra row, malformed facts, empty/whitespace/duplicate ID, recurrence-to-UNPROVEN, dangerous-blast-to-SEVERE, v1/v2 unchanged, and fake critic v3. Tests: `internal/critique/model_test.go`, `internal/returnschema/returnschema_test.go`, `internal/validate/returncomplete_direct_test.go`, `internal/adapter/fake_test.go`, and `scripts/agents/return-schema-fixtures.sh`.

- Task 2 — atomic canonical register and cap engine, 8h. Replace `CritiqueExhaustionAction`/`ExhaustionPatches` in `internal/dispatch/critique.go` with the locked, idempotent advance operation; initialize marked critic roots in `internal/dispatch/build.go`; protect the two owned fields in `internal/dispatch/record.go`; update the verb wiring in `cmd/metasystem/dispatch_verbs.go` and `cmd/metasystem/main.go`. Fixture legs: bounded starts at rounds 1/2/3; exactly two further rounds; mixed SEVERE/UNPROVEN overrides bounded deadline; round-3 first exhaustion; round-6 terminal exhaustion; protocol errors at off-cap, round 3, and round 6; synthetic-ID enumeration; omission/rename remains open; same-ID non-material resolution; same-root downgrade; cross-root class conflict; idempotent retry; concurrent advances; and generic `RecordCAS` rejection of register/exhaustion writes. Tests: `internal/dispatch/critique_chain_test.go`, `exhaustion_direct_test.go`, `decisions_test.go`, `record_test.go`, and `cmd/metasystem/dispatch_verbs_test.go`.

- Task 3 — dispatch cutover and standalone retirement, 4h. Reorder `follow_up` in `scripts/agents/dispatch.sh` so synchronization and critique advance occur before child reservation; add holder-only authority to the internal critique mutation; include warden; append the register’s open-ID carry list to every critic follow-up prompt; remove manifest patch helpers; delete `scripts/agents/critique-round.sh`. Fixture legs in `scripts/agents/dispatch-fixtures.sh`: an exhaustion refusal leaves no child record or payload; crash after atomic advance retries the same successor idempotently; corrected protocol-return follow-up carries the synthetic ID; warden follows identical caps; unauthorized internal invocation refuses; retired driver is absent and documented dispatch remains usable.

- Task 4 — closure and disposition semantics, 4h. Extend `internal/validate/critiqueclosed.go` to join the v3 rigor rows and reject `out-of-scope` for effective SEVERE or UNPROVEN while retaining the current scope-citation rule for BOUNDED. Extend `internal/dispatch/close.go` to require a fully ingested, clean register on marked critic roots. Fixture legs: bounded scope closure succeeds with evidence; severe/unproven scope closure fails; reclassified bounded succeeds only in a later explicit return; open/disputed register refuses close; clean register closes; legacy unmarked roots retain existing behavior. Tests: `internal/validate/critiqueclosed_test.go` and `internal/dispatch/decisions_test.go`.

- Task 5 — same-tree union and exact-tree certificate, 7h. Refactor `mergeCritique` in `internal/validate/conformance.go` and warden evaluation in `internal/validate/authorization.go` to evaluate every relevant exact-tree chain, union register findings, and refuse class conflicts instead of returning on the first clean chain. Add `internal/validate/critiqueexit.go` and CLI wiring in `cmd/metasystem/validate_verbs.go`/`main.go`; issue the exact certificate only after conformance and any mission authorization succeed. Add certificate consumption to the `--push` path in `scripts/agents/commit.sh`. Fixture legs: clean+material same-tree chains refuse; differing class refuses; stale-tree chains cannot satisfy current-tree review; unknown-subject protocol chain blocks; exact certificate succeeds; wrong base/tree, altered register, changed critic set, duplicate certificate, material exit kind, and missing certificate refuse; existing prose waiver receives only its existing-waiver certificate. Tests: `internal/validate/conformance_test.go`, `authorization_test.go`, new `critiqueexit_test.go`, `scripts/agents/conformance-fixtures.sh`, and `scripts/agents/static-reproof-fixtures.sh`.

- Task 6 — documentation and integrated proof, 3h. Update `docs/orchestration.md`, `skills/design-critique/SKILL.md`, `skills/code-critique/SKILL.md`, the warden instructions, and the implementation-bound obligation matrix in `plans/severity-tiered-rigor-design.md`; remove current-prescriptive references to the retired driver while leaving historical records intact. Run focused Go packages first, then `scripts/agents/return-schema-fixtures.sh`, `dispatch-fixtures.sh`, `conformance-fixtures.sh`, and `static-reproof-fixtures.sh`; finish with the repository design-obligation gate and normal full landing battery. No tests were run during this read-only decision pass; HEAD remained clean at `3cf7a83f6968a379f57fca4e9d80198a02b91524`.

Proposed receipt: `RECEIPT|type=design|outcome=converged-buildable|skills=design-critique+take-a-step-back|verify=read-only-code-grounding-at-3cf7a83|corrections=0|stop_loss=no|delegate=none|note=severity-tiered-rigor round 3 pinned a 31h slice-one build; prescriptions 7-9 remain later slices`

## Revision 2 (2026-09-03): the tier is the budget, the material stop closes the loop

Re-aimed by Wido's words in R-54-m1 and R-60-m1. Tasks 1 to 3 above
stand as landed (the rigor grammar, the canonical finding register,
the round engine, the dispatch cutover). This revision replaces tasks
4 to 6 with the build list below. It removes the fixed round caps and
puts the review depth where Wido put it: in the budget of the item,
decided by its tier at intake. Prescription 5 above ("keep finite caps
for all classes") is withdrawn by R-60-m1; the tier box is the only
bound.

### What is wrong today, in one paragraph

A goal carries no class. The budget norm is one number, 1440 reserved
minutes, whatever the item is (internal/goal/norm.go, key
metasystem.budget.goal-norm-job-minutes). The round engine in
internal/dispatch/critique.go stops a chain at rounds three and six by
constant, whatever the item is. A finding is material by a bare
boolean, so a critic can keep a loop alive with findings that change
nothing. And a one-line constant inside internal/ cannot land as a
direct fix because internal/ sits on the never-direct-fix list in
internal/landing/observe.go, so the smallest change takes the whole
ladder. The two specimens: the alert channel's thirteen rounds and the
Codex handshake's seven design revisions for one constant.

### The mechanism

**1. The tier lives on the goal.** A new goal field, `Tier`, with the
values 1, 2, 3, written by whoever opens the goal (`goal open --tier`)
and changeable by `goal edit --tier <n> --why "<text>"`; every change
is a history line. The three tiers are R-54-m1's, verbatim:

- Tier 1: a constant, a message text, a config value, a fixture, a
  dead-code removal. Build, the area's tests, land as a declared
  direct fix. No design round, no review.
- Tier 2: mechanical logic inside an existing owner. Build plus one
  code review. No design round.
- Tier 3: design-bearing (a new law, verb, schema, seam or role).
  Design, one design review, build, one code review.

Destructive reach is not a tier. It stays the job's hazard class
(internal/dispatch/hazard.go) and adds live proof on top of any tier.
A goal without a tier is not dispatchable: `delegate` refuses with
"classify the goal first: goal edit --tier". The landing of build part
one sweeps every open goal and writes its tier, each as an edit line by
the coordinator, so the fleet is never stuck on the day the rule lands.

**2. The tier box is the budget norm.** The norm stops being one
number. metasystem.conf carries one box per tier, in the four-number
notation of R-44-m0b (elapsed, attempts, reserved minutes, active
jobs), with these defaults:

    metasystem.budget.tier-1 = 1h/3/60m/1
    metasystem.budget.tier-2 = 4h/6/240m/1
    metasystem.budget.tier-3 = 8h/10/240m/1

`goal set-budget` inside the box of the goal's tier needs no approval.
Above the box it needs `--approved-ref` exactly as today (the strict
token in internal/goal/norm.go is unchanged). A goal opened without a
budget gets its tier's box. The old single-number norm key is deleted,
not kept as a fallback; a conf that still names it is refused at load
with the new keys spelled out.

Open point for Wido, with my recommendation. Today every dispatch
reserves the full 120-minute cap for the life of the goal revision
(goal dispatch-cap-necessity, R-58-m1), so a 240-minute pool holds
two dispatches and a tier-3 ladder has at least four. Until that goal
lands, the boxes above stall lawful ladders. I recommend the reserved
minutes in each box be written as attempts times the dispatch cap
(tier 1: 360m, tier 2: 720m, tier 3: 1200m) at the landing of this
part, and lowered to R-44's numbers when the reservation bug is gone.
The pool is a runaway guard; the tokens are the cost.

**3. The ladder depth follows the tier.** `delegate` reads the tier of
the goal it is dispatched under and refuses what the tier does not
have:

- Tier 1 refuses every critic role and every `--reviews`; its one
  implementer job is a MECHANICAL hazard job.
- Tier 2 refuses `--role design-critic`.
- Tier 3 accepts all roles.

The review budget replaces the round constants. In
internal/dispatch/critique.go the three constants
(firstSevereExhaustionRound 3, terminalExhaustionRound 6,
boundedFurtherRounds 2) go away. The boundary of a chain is its
tier's review budget: tier 2, two critic rounds per chain (the review
and the stamp after one correction); tier 3, three critic rounds per
chain, design chains and code chains alike. That is the "three rounds
then split" of R-60-m1, read as the default budget. A seat that needs
more raises the budget the same way it raises minutes: with a ruling
row and `--approved-ref`, which then also raises the round boundary
(`goal set-budget --review-rounds <n>`, the fifth number of the box,
default per tier as above).

**4. The material stop is mechanical.** The critic return schema
(generated version three, internal/returnschema/returnschema.go)
gains one member on the rigor row: `artifact`, the repository path
the finding changes, with the metasystem/ prefix, or the literal
`NEW <path>` for a file that does not exist yet. The normalizer in
internal/critique/model.go applies R-60-m1: a finding marked material
whose artifact is empty, does not resolve in the reviewed tree, or is
not a NEW path, is normalized to not material, with a protocol note in
the register entry (`materialDemoted: no artifact`). It is not turned
into a synthetic finding and it does not count against the review
budget; it simply cannot keep the loop alive. The register advance
already resolves a finding the critic drops as material:false. So a
chain closes on the first round with zero material findings, and a
critic who wants to keep a chain open must name what changes.

**5. At the budget, the agreed parts build and the disputed points
become named obligations.** When a chain reaches its review budget
with open findings, the exhaustion verb stops refusing the next round
and instead offers one exit, `job critique-register-close --goal <id>`:

- every open or disputed finding of rigor class bounded becomes an
  obligation on the goal record (the existing `Obligation` field,
  one line per finding: id, artifact, the test that would prove it),
  and the register entry is marked `deferred:<goal>`;
- every open finding of class severe or unproven blocks the close; the
  verb prints the finding and its artifact and the coordinator asks the
  human (over the fleet conversation channel once it exists; by the
  ledger until then). The human's word, recorded on the goal, either
  raises the budget or accepts the risk, and only then does the close
  run with the finding recorded as `accepted-risk` in the counselor's
  register (records/counselor/accepted-risk-register.jsonl).

A closed chain with deferred findings lands normally: conformance's
zero-material rule reads the register, not the last critic return, and
deferred entries are not material. The goal cannot conclude while it
carries an obligation; `goal conclude` refuses and names the open ones.
Recurrence: when a later critic raises a finding with the same
artifact and facts digest as a deferred one, the normalizer classes it
unproven (the recurrence rule already in model.go), which blocks the
next close until the human decides. That is the near-miss promotion of
the original intent, without a second register.

**6. Tier 1 lands as a direct fix.** A new landing class in
scripts/agents/landing-classes.json, `tier-1`, declared with the goal
id. The landing floor in internal/landing/observe.go admits it when:
the goal's tier is 1; the diff touches at most three files and at
most forty changed lines; no path is of class ledger under the
path-class manifest; the area's tests ran, recorded as a gate stamp
in the landing message; and the never-direct-fix list is bypassed
for behavior paths only for this class. Outside those bounds the class
is refused with the bound named, and the item is a tier-2 item.

**7. Two rules from the old task four survive, folded here.** A finding
closed as out-of-scope stays closed only if its rigor class is bounded;
severe and unproven findings cannot be closed by scope, only
reclassified in a later explicit critic return (prescription 2). And
`CloseCheck` in internal/dispatch/close.go requires a fully folded
register: every terminal critic round advanced, no open severe or
unproven entry. The same-tree union and the exact-tree certificate
(old task five) stay a later slice; they guard critic-shopping, which
no specimen has shown yet.

### Build list

Three build parts and a docs part, each a chain under this goal; the
goal itself is tier 3, and its own ladder runs under the rules it
builds as soon as part one lands.

- Part one, the tier at intake: the goal field, the two verbs' flags,
  the conf boxes, the norm gate by tier, the dispatch refusals of
  point 3, and the sweep of the open goals. Files: internal/goal/file.go
  and norm.go, internal/goalbudget/budget.go, internal/config/budget.go,
  cmd/metasystem/goalsync_mutations.go, internal/dispatch/admission.go,
  scripts/agents/goal-cli-fixtures.sh. Fixtures: open with each tier;
  open without a tier refused; set-budget inside and above each box;
  the deleted norm key refused at load; delegate refusals per tier;
  the sweep is idempotent.
- Part two, the material stop and the review budget: the artifact
  member, the demotion rule, the tier-derived boundary, the fifth box
  number, the close verb with its two exits, obligations on the goal,
  conclude refusal, recurrence. Files: internal/returnschema/returnschema.go,
  internal/critique/model.go, internal/dispatch/critique.go and
  finding_register.go and close.go, internal/validate/conformance.go
  and critiqueclosed.go, cmd/metasystem/dispatch_verbs.go,
  scripts/agents/adapters/runtime-common.sh, the critic rows of
  scripts/agents/role-packets.json, scripts/agents/return-schema-fixtures.sh and
  dispatch-fixtures.sh. Fixtures: material without artifact demoted;
  artifact outside the tree demoted; NEW path accepted; chain closes at
  zero material; boundary at two and three rounds; raise by
  approved-ref; close with bounded findings writes obligations; close
  with a severe finding refuses; accepted risk recorded; conclude
  refuses with obligations; recurrence classes unproven.
- Part three, the tier-1 landing class: the class row, the floor rule
  with its bounds, the gate stamp. Files:
  scripts/agents/landing-classes.json, internal/landing/observe.go and
  observe_test.go, scripts/agents/land.sh, scripts/agents/land-fixtures.sh.
  Fixtures: a one-constant change under a tier-1 goal lands; four files
  refused; a ledger path refused; a tier-2 goal refused; missing gate
  stamp refused.
- Part four, docs: docs/orchestration.md, skills/design-critique/SKILL.md,
  skills/code-critique/SKILL.md, AGENTS.md's intake paragraph, and the
  obligation matrix of this file.

Budget of this goal: tier 3, big box, review rounds three per chain.

## Revision 3 (2026-09-03): the review folded

Chain str-design-cc1 returned fourteen material findings against
revision 2. Each is answered by number; an answer that amends a point
or build list of revision 2 supersedes it, and revision 2's prose
stays as the record. Code facts were read at HEAD 327de0b2.

### STR2-TIER-AUTHORITY-01: the tier is bound by approval, claim and root

Amends point 1. `Tier` is a `uint8` field on `GoalFile` (file.go),
rendered as its own `- Tier: <n>` line, distinct from `Ratified:
tier=`. `ApprovalDigest(intent, tier, budget)` hashes `tier=<n>`
between intent and tuple; `bindApproval` and `ValidateApprovalRecord` follow.
- `EditFields` gains `Tier *uint8`; `Edit` refuses a tier change on an
  approved, claimed or parked goal exactly as it refuses an intent
  change today (verbs.go:1193): unapprove, edit, approve. Before
  approval any actor may set it, so a downgrade after approval is
  always a human act, because `Unapprove` is one.
- The claim binds the revision, so the tier at that revision is the
  claimed tier. The three root writers in internal/dispatch/build.go
  write `goalTier` beside `goalRevision`, read through
  `ResolveGoalRevision` (servinggoal.go) extended to return it. Every
  refusal of point 3 reads the root's `goalTier`, never the live goal.

### STR2-GOAL-SWEEP-02: a human confirm over normal transactions

Amends point 1's last sentence. The sweep is `goal classify-sweep`,
shaped like `ApproveSweep` (approval.go:349-440): `--preview` lists
every open goal without a tier with the tier and reason proposed in a
draft file the seat writes; `--confirm <listing-digest> --by <human>`
applies one `edit` transaction per goal under the human actor, which
verbs.go:1201 already admits on another pair's claim, and re-binds the
approval digest of each approved goal under that act. A changed
listing refuses with `SWEEP_LISTING_CHANGED`. The confirm also appends
the root-record change `TierLaw: since=<opid>` (`appendRootChange`,
approval.go:117); before that marker a tierless goal dispatches under
tier-3 rules and box, after it `delegate` refuses as revision 2 says.
`goal open` without `--tier` is refused from the landing. Fixtures:
preview digest; confirm; changed listing; tierless before and after.

### STR2-BUDGET-TUPLE-03: one five-member contract

Amends point 2 and the raise sentence of point 3. `goalbudget.Budget`
gains `ReviewRoundLimit int64`; `New` takes five values; `Validate`
requires it non-negative (tier 1 is 0); `renderBudgetRecord`,
`parseBudgetRecord`, `budgetIntentArgs` and `budgetFromIntentArgs`
carry `reviewRoundLimit`. A stored four-field record parses with the
member taken from the goal's tier box (the tier-3 box while it has no
tier) and renders as five on its next write. The conf notation is
`<elapsed>/<attempts>/<minutes>/<active>/<rounds>`, rounds 0, 2, 3.
- The box is assigned mechanically in exactly two places: `Open` with
  no tuple and `Approve` with no `--budget`; the digest binds tier and
  all five values, so the approving human sees the numbers.
- Every later change of any member is `SetBudgetApproved` with proof.
  Above the box in minutes or rounds it also needs `--approved-ref`,
  and the strict token in norm.go becomes `goal=<id> minutes=<m>
  reviewRounds=<n> goalRevision=<r>`; `StrictApprovalTriple` becomes a
  quadruple and `goalNormApproval` and `requireWithinGoalNorm` compare
  both members against the box. An unchanged minute token cannot raise
  rounds because the token names the rounds. goalsync_mutations.go:
  `--review-round-limit` joins the all-or-nothing tuple flags;
  `--budget small|big` becomes `--budget box`, the tier's.

### STR2-CONFIG-TOMBSTONE-04: the retired key is refused, not ignored

Amends point 2. internal/config/validate.go gains a `retiredKeys`
table mapping `metasystem.budget.goal-norm-job-minutes` to a message
naming the three tier keys; `Validate` adds a problem when it appears
in the committed file, the `.local` file or the environment. The load
boundary is the reader: the new `config.TierBox(confPath, tier)` in
internal/config/budget.go calls `refuseRetiredKeys` over the same three
sources before resolving, so `goalNormApproval` and `delegate` fail
closed on a stale conf even when nobody ran validate. Fixtures: one
leg per source in validate_test.go and budget_test.go.

### STR2-ROUND-ACCOUNTING-05: a frozen boundary and a consumed counter

Amends point 3. Critic roots gain `reviewRoundLimit`, copied by
build.go from the goal's tuple at the claimed revision, and
`criticRoundsConsumed`, starting at 0. `CritiqueRegisterAdvance`
(finding_register.go:37) increments the counter in its completed and
failed branches only; the cancelled branch consumes nothing.
`readCritiqueCapState` (critique.go:133) replaces the three constants,
`boundedCritiqueStart` and `recordFirstAllBoundedRound` with one rule:
exhausted when `criticRoundsConsumed >= reviewRoundLimit` and an entry
with status open or disputed exists; the severe-or-unproven scan runs
over those entries only. A raised budget reaches a chain only through
`job critique-budget-rebind --root-job`, which copies the approved
`ReviewRoundLimit` onto the root with the goal revision and opid.

### STR2-ARTIFACT-MEMBERSHIP-07: membership, not existence

Amends point 4. `artifact` has three canonical forms, pattern-bound in
returnschema.go and parsed by `ParseArtifactRef` in internal/critique/
model.go: `<path>`, `NEW <path>`, `<old>=><new>`. Membership is decided
at fold in finding_register.go against the root's subject set: for
code chains the changed-path set of the reviewed implementer round's
`diff.patch` (both rename sides), refusing the fold with "run
conformance --stage review first" when absent; for design chains
`declaredOutputs`, written on the root from a new required `--outputs
<file>` on design-critic dispatch, filled from the design's build list.
`<path>` must be in the set; `NEW <path>` must be in `declaredOutputs`
and absent from the reviewed tree; a rename needs either side in the
set. Anything else demotes (08). Fixtures: in-set accepted; unrelated
path and undeclared NEW demoted; rename by either side accepted;
design dispatch without outputs refused.

### STR2-DEMOTION-TRANSITION-08: demotion is not withdrawal

Amends point 4. `foldCritiqueFindings` (finding_register.go:342) gains
a `demoted` outcome ahead of the `!material` branch: a material finding
failing 07 adds no entry, touches no existing entry, and is appended
with its reason to a per-round `demotions` list on the root. Only an
explicit `material:false` resolves an entry (fixture: re-emitted open
finding with an empty artifact stays open).

### STR2-CLOSE-STATE-MACHINE-09: the atomic transition table

Amends points 5 and 7. Status gains `deferred` and `accepted-risk`;
every non-open entry carries `resolution`: `withdrawn`, `out-of-scope`,
`deferred` or `accepted-risk`. `CritiqueRegisterClose` in
finding_register.go, called from close.go, evaluates once under the
register lock and takes one branch with one write:

| unresolved set U (open or disputed) | condition | outcome |
| --- | --- | --- |
| any entry severe or unproven | always | refuse; no write; print each with artifact |
| non-empty, all bounded | consumed < limit | refuse: budget not exhausted, dispatch the next round |
| non-empty, all bounded | consumed >= limit | all of U become `deferred` in one write, after 10's ledger step |
| empty | any | ordinary close; `CloseCheck` proceeds |

`CloseCheck` counts open and disputed as unresolved, deferred and
accepted-risk as resolved. critiqueclosed.go writes `out-of-scope` as
the resolution and refuses it for a severe or unproven entry, as do
CloseCheck and conformance however it got there.

### STR2-CLOSE-PERSISTENCE-10: the review-obligation model and commit order

Amends point 5. The governed `Obligation` field is not used. `GoalFile`
gains `ReviewObligations []ReviewObligation`, one `- ReviewObligation:
finding=<id> chain=<root> artifact=<ref> test=<text>
state=open|discharged` line each, parsed and rendered in file.go and
untouched by `clearClaimBinding`. verbs.go: `DeferFindings` (called by
the close verb as the pair) appends them; `DischargeReviewObligation
--finding --by` needs the human or the owning pair plus a test
citation; `Done` refuses while any is open and prints them.
`AcceptedRiskDecision` in verbs.go records `AcceptedRisk: finding=<id>
chain=<root> by=<human> opid=<opid>` under `humanauthority` proof as
`SetBudgetApproved` does, and `counselor.AppendAcceptedRisk` in new
internal/counselor/register.go emits one strict-schema line
(sources.go:48-77): id `ar-<root>-<findingId>`, kind `accepted-risk`,
class = rigor class, the finding's facts as specimen facts citing the
root record, a review link to the goal. Commit order, each step keyed
and skipped on rerun: (1) goal ledger transaction carrying root and
finding ids; (2) register append, skipped if the id exists; (3) root
write marking the entries with the goal opid. A crash leaves the chain
unclosed and the verb re-runnable; conformance refuses a root whose
deferred or accepted entries lack a matching goal line.

### STR2-CRITIC-UNION-11: the union lands in part two

Amends point 7's last sentence: the same-tree union moves into part
two; only the exact-tree certificate stays later. `mergeCritique`
(conformance.go:919) collects every code-critic root whose reviewed
tree equals the final tree and requires all to pass; one failing root
fails the merge, and the union of registers must hold no open or
disputed entry. The close verb is `job critique-register-close
--root-job <root>`; `--goal` is dropped. Fixtures: clean plus material
same-tree chains refuse, and differing class refuses.

### STR2-TIER1-PROTECTED-PATHS-12: a floor list and a hazard rule

Amends points 3 and 6. scripts/agents/path-classes.txt gains a line
kind `floor:<path> tier-1-refused`, read by `pathclass.Manifest` into a
`Floors` set; the four classes are unchanged. Rows: internal/goal/,
goalbudget/, humanauthority/, dispatch/, landing/, validate/, critique/,
returnschema/, config/; scripts/agents/dispatch.sh, commit.sh, land.sh,
schemas/, path-classes.txt, landing-classes.json; metasystem.conf.
`observeDirectFix` refuses `tier-1` when any changed path is under a
floor row, code `tier1-floor-refused`. Tier 1 does not force
MECHANICAL: `EvaluateGoalRevisionAdmission` (admission.go:124) refuses
a DESIGN-BEARING or DESTRUCTIVE-REACH job under a tier-1 root with
"the hazard needs review the tier does not have; goal edit --tier 2",
since both need independent critique in hazard.go. The hazard is
preserved and the tier moves, never the reverse.

### STR2-TIER1-EVIDENCE-13: a receipt bound to the tree, a diff metric

Amends point 6. `metasystem landing test-receipt --root --tree <sha>
--command "<cmd>"` (landing_verbs.go) runs the command and writes
`artifacts/agents/landing/receipts/<tree>.json` with tree, command,
exit status and time. `ObserveParams` gains `TestReceipt`;
`observeDirectFix` for `tier-1` requires the receipt's tree to equal
the candidate tree and exit status zero. land.sh gains `--tests
"<cmd>"`, commit.sh passes `--test-receipt` to `landing observe`, and
the message stamp is dropped. Changed lines are added plus deleted from
`git diff --numstat base..candidate`; a `-` count (binary), an R or C
shape from `git diff-tree -M -C --name-status`, or a mode change with
zero lines refuses with `tier1-diff-shape-refused`; each is a fixture,
with a foreign-tree receipt and forty-one lines.

### STR2-RULING-CONFLICT-06 and STR2-RESERVATION-RECOMMENDATION-14: Wido's

**06, the fourth round.** (A) "three": R-42-m0 is the ceiling;
`Budget.Validate` refuses `ReviewRoundLimit > 3`, the raise of point 3
is withdrawn, and at exhaustion the close of 09 is the only exit.
(B) "raise": no ceiling, the token raise of 03 applies, and R-42-m0 is
amended to "three by default". Recommendation: A; R-42-m0 is the later
word and says it composes with R-60-m1 as a material-only extension,
never a raise, so A keeps both rows true.

**14, the reserved minutes.** (A) "ceiling": a new key
`dispatch.cap-max`, default 120, enforced by `ResolveCap` (cap.go:36)
against every source including the explicit argument; box minutes are
attempts times 120 (360, 720, 1200). (B) "computed": `config.TierBox`
sets the box minimum to attempts times the largest cap admitted for the
tier's roles and pairs by walking `cap.min.*` through `config.Keys`; an
explicit cap above it needs an approved budget. Recommendation: A; one
enforced ceiling is one rule, and B hides the number in the roster.

### Build list, revision 3 (replaces revision 2's parts one to three)

- Part one, the tier at intake: `Tier` field, digest and edit rules
  (01); classify-sweep and marker (02); five-member tuple, token
  quadruple, box assignment, `--review-round-limit` (03); tombstone
  (04); `goalTier` on roots, refusals by tier and the hazard-under-
  tier-1 refusal (12). Files: internal/goal/file.go, approval.go,
  verbs.go, norm.go, budget.go, internal/goalbudget/budget.go,
  internal/config/budget.go and validate.go, internal/dispatch/build.go,
  servinggoal.go, admission.go, cmd/metasystem/goalsync_mutations.go,
  scripts/agents/dispatch.sh, goal-cli-fixtures.sh.
- Part two, the material stop and the close: artifact forms and
  membership (07), demotion (08), counter, frozen limit and rebind verb
  (05), transition table and resolutions (09), review obligations,
  accepted-risk writer, commit order (10), union and `--root-job` close
  (11). Files: internal/returnschema/returnschema.go, internal/critique/
  model.go, internal/dispatch/critique.go, finding_register.go, close.go,
  build.go, internal/goal/file.go and verbs.go, internal/counselor/
  register.go, internal/validate/conformance.go and critiqueclosed.go,
  cmd/metasystem/dispatch_verbs.go, scripts/agents/dispatch.sh
  (`--outputs`), adapters/runtime-common.sh, role-packets.json,
  return-schema-fixtures.sh, dispatch-fixtures.sh, conformance-fixtures.sh.
- Part three, the tier-1 landing class: class row, floor rows and
  refusal (12), receipt verb, `TestReceipt`, diff metric (13). Files:
  scripts/agents/landing-classes.json, path-classes.txt, land.sh,
  commit.sh, land-fixtures.sh, internal/pathclass, internal/landing/
  observe.go and observe_test.go, cmd/metasystem/landing_verbs.go.
- Part four is unchanged. Decisions 06 and 14 gate only the `Validate`
  bound and the box minutes of part one; the rest builds either way.

Obligation matrix (part four, 2026-09-04): records/misc/severity-tiered-rigor-obligation-matrix.md, one row per mechanism point of revisions 2 to 4 with its proof or its pending status.
