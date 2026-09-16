# The small-change lane (goal small-change-lane)

Revision: 1. Date: 2026-09-10. Design authoring job: `scl-design1`, on
claude with claude-fable-5-1 under R-89-m1b. Every line number below was
read at commit `b3d795fe` of branch `agent/scl-design1`. The revision record
at the end says what this revision decided and what it left to Wido.

This page specifies the build; it implements nothing. Independent critique,
dispositions, certification, the ledger and the receipt belong to the
orchestrator. All paths are relative to the repository root. Symbols marked
"new" do not exist in the tree yet.

The contract is the Intent of `metasystem/plans/goals/small-change-lane.md`:

> The 'change this little thing' case has a supported path: a dispatch lane
> cheap and fast enough that a certified one-line fix is not ceremony.

The record's older next step asked for certification folded into the build
return with the seat as oracle. That is superseded by Wido's order of
2026-09-01 (goal `critique-always`, approved 2026-09-06, edited by Wido
2026-09-07): anything built in the metasystem is a backlog item and every
backlog item is critiqued. The lane keeps an independent critique. What it
removes is everything else that makes a one-line fix cost what a feature
costs: the hand-written brief, the design round, the open-ended critique
loop, the per-round dispositions register, the seat's own turns around each
step, and the budget ceremony.

## What a small change costs today

`metasystem/plans/goals/review-round-limit-counts-per-chain.md` is the
specimen. Its record already decided everything (Intent, lines 8-10). Landing
it took a hand-written brief; three delegate rounds; a chain close; an engine
rebuild at main's exact tip plus `steward arm`; a schema-2 receipt of 36
groups in 54 minutes; and it still cannot land because one group refuses
every engine-changing candidate (goals
`gate-fence-fixtures-refuse-engine-changing-candidates` and
`gate-fence-returns-to-per-landing-proof`). The coordinator's own turns
around it were the larger cost.

Two things about the specimen are said plainly here, because the lane must
not pretend to have fixed them. First, as recorded it is not a small change
by this page's rule: fifteen files, and a risk record with accumulation 2,
which is `gateWidth: full` (`metasystem/internal/goal/file.go:108-113`). The
lane would have refused it at review and it would have taken the ordinary
ladder, as it did. What the lane removes for a change like it is the brief,
the extra rounds and the seat's turns; the receipt width is the risk
record's, not the lane's. Second, the gate-fence refusal blocks every
engine-changing candidate in every lane; the lane inherits that block and
waives nothing.

## Grounding in the current tree

Source observations, not runtime proof. Each row says what the design does
with the fact.

| Responsibility | Existing file and symbol | Observed behavior and design consequence |
| --- | --- | --- |
| Hazard classes and their duties | `metasystem/internal/dispatch/hazard.go:36-52`, `requiredConfigurationByHazard`; `:150-157`, the R-22-m1 Ruling O refusal codes; `:191-195`, the governing class is the strictest member; `:208-210`, a class without critique or live proof closes with no reference | MECHANICAL is `ordinary/medium`, no critique, no live proof. R-90-m1 (rulings.md:147) says the builder runs at xhigh; `critique-always` says the row requires critique. Neither has landed. The lane is a MECHANICAL lane and depends on both rows landing; section 2 says what the row must read. |
| Independent-critique closure | `hazard.go:286-327`, `validateIndependentCritiqueReference`: `:302-304` the critic's `reviews` must name the final work round; `:305-311` fresh context and distinct session; `:313-321` the critic's `reasoningEffort` must equal the class's critic effort and its model must prove maximal execution; `:322-325` the critic ended after the final work round | After a fold, a chain closes only with a critic that reviewed the fold's job. So "one critique" for the lane means one read, at most one fold, and one closing read (section 2). A class whose critic effort is `none` can never satisfy `:313-321`; section 2 fixes the row's values. |
| Reference stamping | `metasystem/internal/dispatch/review_reference.go:19-34`, `StampClaimedReviewReference`, writes `independentCritiqueJobRef` on the reviewed chain root at critic dispatch; `:42-106`, `ReconcileReviewReference` for evidence launched before stamping | The lane needs no new pointer machinery: the closing read is dispatched with `--reviews <final work job>` and stamps itself. |
| Chain close | `metasystem/scripts/agents/dispatch.sh:2642`, `close_chain`; `:2675-2700`: mirror every terminal member, `critique-register-close` for critic roots, `close-check`, then CAS `chainClosed: true`; `metasystem/internal/dispatch/close.go:15-31`, `CloseCheck` calls `validateHazardCompletion`; `:36-80` the folded register must hold no open, disputed or scope-closed severe entry; `:81-128` durability | `chainClosed` today means terminal, hazard duties met, register clean, evidence mirrored. Section 2 adds one lane duty to that meaning. |
| Goal admission at dispatch | `metasystem/internal/dispatch/admission.go:180-250`, `EvaluateGoalRevisionAdmission`; `:199-210` the risk gate in `mark` or `enforce` mode; `:211-214` a tier-1 goal refuses any hazard but MECHANICAL; `:15`, the admission refusal codes | The lane's dispatch-time checks live beside these. The lane adds one code to `:15`. |
| Tier ladder at dispatch | `dispatch.sh:740-755`, `require_goal_tier_ladder`: tier 1 refuses every critic role and `--reviews`; tier 2 refuses `design-critic` | Under `critique-always` a tier-1 chain must carry a critique but cannot dispatch one. Section 7 says how the lane resolves that. |
| Packet composition | `metasystem/internal/dispatch/composition.go:17`, the role-packet table; `:130-144`, `ComposeRolePacket` refuses an unknown hazard row; `:146-152` reads the caller's brief into the `task-direction` slot; `:181-191` appends the role's fixed sources; `:201-206` the generated runtime notice | The brief is a caller file today. The lane keeps the packet law and changes only who writes the file: the engine renders it from the goal record (section 2). |
| Brief handling in dispatch | `dispatch.sh:800`, `brief_mode` requires one Working Mode header; `:804-806`, `brief_authority` refuses cited paths that do not exist in the delegate's tree; `:1498-1508` goal binding gives revision, tier, width, machine, claim epoch; `:1589-1596` the shared testing requirement is appended to every implementer brief; `:1612-1618` the optional serving-goal section; `:1638-1642` composition | The rendered lane brief passes through the same checks and gets the same appended sections. A goal record that names a path that does not exist fails brief authority; that is a correct lane refusal (the record does not decide the change). |
| Serving-goal section | `metasystem/internal/dispatch/servinggoal.go:56-61`, `ServingGoalSection` projects `<id> — <intent>` as context, not instruction | The lane brief is the record's Intent as instruction, which this section deliberately is not. The lane therefore renders its own brief and does not reuse this section. |
| Testing requirement | `metasystem/internal/dispatch/build.go:1055-1060`, `TestingRequirement`; `:149-155` and `:418-420` a goal-bound root carries `gateWidth` from the binding, `area` by default | Unchanged. The lane's chain carries `area` by admission (section 1). |
| Selection law | `metasystem/internal/testpolicy/select.go:119-133` direct owners and unowned-path uncertainty; `:134-144` reverse consumers; `:149-157` required mode; `metasystem/internal/testpolicy/risk.go:106-109`, `requiresDeep`: severity, exposure, novelty or accumulation of 2 or more selects deep; `select.go:158-169` standard and deep groups of the affected surfaces, cross-cutting only under accumulation 2; `:187-190` an unowned path collapses the plan to the `unknown` set; `:200-202` standard delivery refuses when deep is required; `metasystem/internal/testpolicy/protection.go:37-49`, protected policy paths | The four risk answers, never the diff's shape, decide standard or deep (testing contract, section 5 step 2). The lane does not lower that. Section 3 states what "area width" therefore selects, and the one case where the brief's wish and the contract disagree. |
| Contract data | `metasystem/testing.json:5-21` surfaces; `:20` the `goal-records` surface owns `plans/**`, `records/**`, `memory/rulings.md`; `:86` `always.canary` (three groups) and `always.standard` (seven, including `fast-static-build`, which is `go-gate.sh --fast`); `:87` `unknown`; `:88` `cadence` | The fast gate is `always.canary` plus `always.standard`. "Owned by one or two surfaces" is decidable from `:5-21`. |
| Chain landing | `metasystem/internal/landing/observe.go:94-132`, `observe`; `:148-196`, `observeChain`: `:168-170` root must be an implementer; `:171-178` width from the root; `:179-183` schema-2 receipt required; `:190-193` a root whose hazard is not DESIGN-BEARING or DESTRUCTIVE-REACH is `chain-not-design-bearing`; `:194-196` `chain-open`; `:256-264` the certified output must match the candidate; `:291-301` held goal and path classes; `:321` pass | Today a MECHANICAL chain lands with a `would-refuse code=chain-not-design-bearing` trailer, because that code is not in `metasystem/scripts/agents/landing-promotion.json`'s `refuseCodes` (`metasystem/internal/landing/promotion.go:25-44`). The lane's chain must land with a pass verdict; section 3 amends `:190-193`. |
| Tier-1 direct fix | `metasystem/internal/landing/tierone.go:31-95`, `observeTierOne`: `:62` floor rows; `:77` more than three files; `:80` more than forty changed lines; `:85-91` receipt and full gate; `:96-122` root must be tier 1; `:124`, `tierOneDiffMetric` (added plus deleted, no rename, copy or binary); `metasystem/scripts/agents/landing-classes.json` row `tier-1` (3 files, 40 lines, authorized by R-54-m1); `metasystem/scripts/agents/path-classes.txt:67-83` floor rows | The tier-1 class is a receipted direct fix with no critic. Under `critique-always` it is unlawful. The lane reuses its diff metric and refuses nothing on the floor rows, because the lane has a critic (section 1). Its retirement is section 7. |
| Path classes | `path-classes.txt:28-38`: `memory/`, `plans/`, `records/` are `record`; `plans/goals/` and friends are `ledger` | "No plans/, records/ or memory/ change" is "no path of class record or ledger" (section 1). |
| Tier boxes and norm | `metasystem/internal/config/budget.go:22-24` the three keys; `:60-71` `tierBudgetKey`; `:220-254`, `TierBox`, five members, defaults at `:226`; `metasystem/internal/goal/norm.go:100-144`, `goalNormApproval`; `:146-161`, `requireWithinGoalNorm`; `metasystem/internal/goalbudget/budget.go:90`, `New`; `:109`, `Validate` | The norm reads one box per tier. Section 4 adds one box the norm reads by lane, ahead of the tier. |
| Goal record and verbs | `file.go:22-75`, `GoalFile`; `:28`, `Tier`; `:77-113`, `RiskRecord`, `DerivedTier`, `GateWidth`; `metasystem/internal/goal/verbs.go:479-507`, `OpenRisked` (a tier below the derivation is a human act); `:512-531`, `openRequest` assigns the box; `:1448`, `Edit`; `:1524`, an approved, claimed or parked goal refuses a tier change; `metasystem/cmd/metasystem/goalsync_mutations.go:223-230` flags; `:427-466` open; `:1395-1440` edit | The lane is a new record field with the same edit discipline as tier before approval and its own rule after claim (sections 1 and 4). |
| Configuration | `metasystem/metasystem.conf:15-20` budget keys; `:54-56` `dispatch.cap-min` and `cap-max` are 120; `:60` `landing.receipt-bound-min=40`; `:81-82` the design roster | Every dispatch reserves the full 120-minute cap (R-58-m1). The lane box's minutes member is attempts times 120 (section 4). |
| Critic rounds | `metasystem/internal/dispatch/finding_register.go:22-23`, the root fields; `:60`, `CritiqueRegisterAdvance`; `build.go:615-616` the box's round member frozen on the critic root; `metasystem/skills/code-critique/SKILL.md:60-95`, the round budget and exit; `:79-83` both layers, one focused follow-up, stop at the first clean round | The count is per critic root today; the seats run one root per read (the specimen goal's Intent). The lane's box is written for both today's accounting and the specimen goal's future per-goal accounting (section 4). |
| Roles and lanes | rulings R-25-m1 (rulings.md:51), R-28-m1 (:56), R-89-m1b (:146): design on claude, design critique on codex, implementation on codex gpt-5.6-sol, code critique on claude (Opus 5 today, by local roster) | The lane's builder is the implementer lane, its critic the code-critic lane. Section 2 records the model and effort decision as R-28-m1 requires. |
| Review-round rulings | R-42-m0 (:117), three rounds is the ceiling; R-60-m1 (:107), depth is a risk budget and the material stop ends a loop; R-54-m1 (:100), tiers and their ladders; R-44-m0b (:86) and R-45-m0b (:89), the standing tuple | The lane's round member is 2, inside the ceiling. R-54's tier-1 "no review" clause is superseded by `critique-always`; section 7 carries the ruling sweep. |
| Severity-tiered rigor | `metasystem/plans/severity-tiered-rigor-design.md:97-115` (the tier on the goal), `:117-141` (the box is the norm), `:207-215` and `:440-469` (tier-1 direct fix, floors, receipt) | The lane is the third leg beside that page and the staged-batch witness. Section 8 says where it joins and where it differs. |

The staged-batch witness has no page in the tree at this commit; the goal
record's one sentence and the cadence machinery (`testing.json:88`, the
testing contract's section 8) are all this page can cite for it.

## 1. What "small" is, mechanically

A goal is in the small lane when its record carries `- Lane: small`. The
lane is a property of the change and of the record's completeness, not of
the risk. Risk stays the four answers and keeps deciding the tier, the
receipt depth and the width; the lane decides the ladder's shape and the
box. A goal can be in the lane at any tier (section 7 says what that means
for tier 1 and tier 3).

`goal open --lane small` sets it; `goal edit --lane small|none` changes it
under the tier's edit discipline: any actor before approval, and after
approval only through unapprove, edit, approve (`verbs.go:1524`), because
the approval digest binds it (section 4). Leaving the lane after claim is a
human act; section 2 says when it happens.

The lane admission has three moments. Each check is a refusal with a
registered code (new rows in `metasystem/internal/refusal/register.go`,
shape `Agent`, override named), so the fast gate's refusal-register audit
sees every one of them.

**At open and edit** (`goal open`, `goal edit`, new checks in
`OpenRisked` and `Edit`):

- `LANE_WIDTH`: the risk record's accumulation must be 1, so the goal's
  width is `area` (`file.go:108-113`). Override: answer the four questions
  again or leave the lane.
- `LANE_DONE_SENTENCE`: the Intent must contain the literal `DONE means`
  and at least one sentence after it. That clause, from `DONE means` to the
  end of the Intent, is the lane's acceptance test and the critic's threat
  model. Override: `goal edit --intent` with the sentence.

**At dispatch** (`dispatch.sh` after the goal binding at `:1498-1508`; the
engine side in `EvaluateGoalRevisionAdmission`):

- `LANE_HAZARD`: a chain under a lane-small goal is dispatched with
  `--destructive-reach MECHANICAL`; any other class is refused with the
  message "the change is not mechanical; leave the lane with goal edit
  --lane none --by". Added to `AdmissionRefusalCodes` (`admission.go:15`)
  so it is citable as `refusal:LANE_HAZARD` misclassification evidence.
- `LANE_ROLE`: only the roles implementer and code-critic dispatch under a
  lane-small goal; `design-critic`, `warden` and `verifier` are refused
  (nothing in the lane needs them; a verifier is a live-proof duty, which
  is DESTRUCTIVE-REACH, already refused by `LANE_HAZARD`).
- `LANE_ROUNDS`: a second implementer follow-up on a lane root is refused
  (section 2).
- The root record gains `lane: "small"`, frozen at dispatch beside
  `goalTier` (`build.go:551`) and inherited by follow-ups like `gateWidth`
  (`build.go:788-793`).

**At review** (`validate conformance --stage review --job <root>`, which
already computes the exact `diff.patch` and `reviewedTree`; new
`LaneAdmission` in `metasystem/internal/dispatch/lane.go`, called from the
review stage when the root carries `lane: "small"`; the verdict is written
on the root as `laneAdmission` and re-evaluated on every review of a later
round, the latest verdict winning):

- `LANE_FILES`: more than four changed paths. `LANE_LINES`: more than eighty
  changed lines, added plus deleted, measured by `tierOneDiffMetric`
  (`tierone.go:124`) lifted into a shared helper. `LANE_SHAPE`: a rename,
  copy, binary or mode-only change, the same rule as tier 1. Why four and
  eighty: the tier-1 bound (three, forty) was written for a change with no
  critic and no test; the testing contract now requires a retained test
  proof on every landing, and a one-line fix with its test and one fixture
  leg is two to three files and forty to sixty lines. Twice the tier-1 line
  bound, plus one file, holds that shape and refuses a feature.
- `LANE_RECORD_PATH`: any changed path whose class under
  `path-classes.txt` is `record` or `ledger`. Records, briefs, dispositions
  and rulings do not ride a lane chain. The goal record itself changes only
  through goal verbs, as today.
- `LANE_UNOWNED_PATH`: the testing contract's selection over the changed
  paths reports uncertainty (`select.go:130-132`). `LANE_SURFACES`: more
  than two surfaces own the changed paths directly (the pattern matches at
  `select.go:121-129`, before the consumer walk). Consumers are not
  counted: a change to `internal/dispatch/**` is owned by one surface
  however many depend on it.
- Engine binding is recorded, not refused. When any changed path is under
  `internal/`, `cmd/` or `scripts/agents/`, the paths the skew preflight
  calls "engine or agent scripts" (`dispatch.sh:262`, refusal text at
  `:269`), the verdict carries `engineBinding: true`. What happens then is
  what happens on every ladder: after the push, the seat rebuilds the
  engine at the landed tip and runs `steward arm`, because the enrollment
  records the tip; and until the gate-fence goals land, the receipt's
  fixture beds refuse such a candidate. The lane changes neither fact; it
  makes the first a one-line step in the landing note (section 5) and
  names the second as an inherited block. A lane that excluded those
  paths would exclude nearly every one-line fix the metasystem has needed.
- The floor rows of `path-classes.txt:67-83` do not apply. They protect
  tier 1 from landing governance code with no critic; the lane has a
  critic. This is not a lowering: the lane is a rung above tier 1.
- `requiredMode` from the contract's plan for the changed paths and the
  goal's risk (`select.go:149-157`) is recorded on the verdict, for the
  landing note and for the open question in section 3. It refuses nothing.

A refusal at review does not fail the chain. It parks the goal by the seat
with the code in the reason, and the human decides: `goal edit --lane none
--by <human> --why <code>` re-boxes the goal to its tier's box (section 4)
and the chain continues on the ordinary ladder with its evidence intact
(section 2 says how it then closes). The seat never widens its own box by
making a bigger change.

## 2. The shape of the lane

**Build round.** One implementer round on the implementation lane (codex,
gpt-5.6-sol under R-25-m1) at xhigh (R-90-m1). The dispatch is
`metasystem delegate --role implementer --goal <id> --lane small --worktree
--destructive-reach MECHANICAL` with no `--brief`; `--lane` and `--brief`
together are refused. The dispatcher calls the new engine verb `job
lane-brief --root <root> --goal <id> --role implementer --out <file>` and
passes the rendered file where the caller's brief goes today, so it takes
the same Working Mode check, brief authority, testing requirement, return
path form and packet composition, and its bytes are hashed into the record
like any brief.

The template is new `metasystem/scripts/agents/templates/lane-brief.md`,
filled only from the goal record at the claimed revision:

- Working Mode: implement; Orchestrator Identity and Date as the dispatcher
  fills them today.
- Goal: the Intent, verbatim.
- Workspace: the job worktree and branch; may touch only paths owned by the
  testing contract; may not touch record or ledger paths.
- Inputs: the Next step, verbatim, and the paths it names (each must exist,
  or brief authority refuses the dispatch).
- Constraints: the lane bounds of section 1 in words (four files, eighty
  lines, no rename or binary, one or two owning surfaces); the retained
  test proof rule; wall clock is the job cap; token budget none.
- Expected Return: the implementer schema's properties, as
  `templates/brief.md` lists them.
- Acceptance Criteria: the DONE sentence, verbatim, as the single
  criterion, followed by "and the named test proves it on the candidate
  tree".
- Gap Rule: the standard sentence.

No Working Mode variation, no per-goal prose, no orchestrator judgment in
the brief. If the record cannot fill the template, the goal is not a lane
goal; that is the refusal `LANE_DONE_SENTENCE` at open, or brief authority
at dispatch, and both are cheaper than a delegate gap-stop.

**Critique.** After `validate conformance --stage review --job <root>`
admits the diff (section 1), one code-critic round on the code-critic lane:
`metasystem delegate --role code-critic --goal <id> --lane small --reviews
<root> --worktree --destructive-reach MECHANICAL`, brief rendered by `job
lane-brief --role code-critic --reviews <root>` from new
`metasystem/scripts/agents/templates/lane-review-brief.md`, which fills
`templates/review-brief.md`'s slots as follows: round budget one read and
one closing read; appetite the lane box; threat model "the computed diff
against the DONE sentence, nothing wider"; scope the diff's paths. The
critic is asked three things and nothing else:

1. Conformance: the diff stays inside the lane bounds and touches only what
   the DONE sentence needs; every path is owned by the contract.
2. Acceptance: the DONE sentence holds on `reviewedTree`, with the named
   test as the evidence (`ran`), not the implementer's word.
3. Defect: the change ships no defect within its own diff (the code-critique
   skill's materiality question).

Design alternatives, wider refactors and pre-existing defects outside the
diff are out of the threat model and close as `out-of-scope` under the
skill's rule. One finding class is material by definition and refuses the
lane: the record does not mechanically determine the change (the critic
cannot decide from the DONE sentence whether the diff is right, or the
right change needs a path the diff does not touch, which the rigor row
shows as a `NEW` artifact or an artifact outside the diff). That is a
design gap; its code is `LANE_DESIGN_GAP`, written on the root's
`laneAdmission` by the register fold, and the goal leaves the lane through
the human as in section 1.

Model and effort, recorded as R-28-m1 requires. The critic runs on the
code-critic lane's roster (claude; Opus 5 today by m1b's local roster under
R-89-m1b, Fable when Wido moves the lane back), at xhigh. Reasoning: the
closure check requires the critic's effort to equal the class's critic
effort and its model to prove maximal execution (`hazard.go:313-321`), so a
cheaper critic cannot close a chain at all; and the lane's saving is in
scope and rounds, not in the critic's effort, which Wido set to xhigh
everywhere on 2026-09-09. Consequence for `critique-always`: when it flips
the MECHANICAL row it must write `independentCritiqueEffortTier: maximal`
and `independentCritiqueReasoningEffort: xhigh` beside
`independentCritiqueRequired: true`, in both `hazard.go:37-41` and
`role-packets.json`; a row with `none` there is unsatisfiable at
`hazard.go:313-316`. The row this page assumes, after R-90-m1 and
`critique-always`, is:

    MECHANICAL: builder maximal/xhigh, critique required, critique maximal/xhigh, live proof false

**Fold.** Zero material findings after dispositions (`validate
critique-closed`) closes the critic root and the chain. Otherwise exactly one
fold: `metasystem delegate --follow-up <root> --lane small`, brief rendered
by `job lane-brief --role implementer --follow-up <root>` from
`templates/follow-up.md` with the open register entries (id, artifact,
claim, evidence) as the correction list, which is what conformance's
exhaustion discipline requires of a successor prompt (`conformance.go:985-995`).
The fold is reviewed again (section 1, `laneAdmission` re-evaluated), then
the closing read: a fresh code-critic root with `--reviews <fold job>`,
same template, threat model "the fold against the open findings and the DONE
sentence". A second fold is `LANE_ROUNDS`. A closing read with a material
finding parks the goal with `LANE_ROUNDS` and the human decides, as in
section 1.

Why a closing read and not a critic follow-up: closure binds the critic to
the final work round by job id (`hazard.go:302-304`), a critic follow-up
inherits its root's `reviews` (`build.go:853`), and the seats already run
one root per read (the specimen goal's Intent). Nothing new is built for
it.

**Close.** `metasystem delegate close --root <root>` as today
(`dispatch.sh:2642`). `CloseCheck` gains one duty for a root with
`lane: "small"`: `laneAdmission.verdict` must be `admitted` for the final
work round, unless the goal record at a revision at or above the root's
`goalRevision` reads `Lane: none` (the human released it). So for the lane
`chainClosed: true` means: every member terminal; the critique reference
names a completed, fresh, xhigh, cross-family critic that reviewed the
final work round; the critic's register is folded and clean; the final diff
was admitted to the lane; evidence mirrored. A chain the human released
from the lane closes as an ordinary MECHANICAL chain and, when its critic
was clean and only the bound was exceeded, lands as one under the tier's
box; only the human's re-box was spent.

**Receipt and landing.** `landing test-receipt --root <root> --tree <tree>
--mode auto`, then `land.sh --chain <root> --test-receipt <receipt> --goal
<id>`. Section 3 says what the receipt selects.

## 3. What the receipt proves, and what the lane never skips

The receipt is the shared testing contract's plan for this candidate, in
`auto` mode, never a named battery and never the cadence set. With the lane
requiring accumulation 1, the plan can never be `full` and never adds
cross-cutting obligations (`select.go:166-168`). With all four answers at
1, the plan is exactly `always.canary`, `always.standard` (the fast gate:
`go-gate.sh --fast`, the static audits, the refusal-register audit) and the
owning surfaces' `standard` groups, which is what the goal record asked for.

With exposure or novelty at 2 or 3, the contract selects deep for the
affected surfaces (`risk.go:106-109`, `select.go:158-164`), and for
`dispatch-goal-mission` that is three fixture sections plus its consumers'
deep groups. The lane does not lower that. The brief for this page asked for
"the owning surfaces' standard groups plus the fast gate, never the full
battery"; the contract answers "standard when the four answers say so".
Those agree for a change whose honest exposure is 1 and disagree for a
one-line fix in `dispatch.sh`, whose honest exposure is 3. This page keeps
the contract's rule, because lowering it is a testing-contract decision
(`application-testing-contract-design.md`, section 5 step 2, "lines changed
never determine risk"), not a lane decision. It is the one open question
for Wido in the revision record, with a recommendation.

The lane never skips:

- the landing observation with the goal named (`land.sh --goal`, the held
  goal at `observe.go:291-295`) and the root's `goalId` and `goalRevision`
  binding, both as today;
- the refusal register: every lane code is a registered row, and the fast
  gate's audit runs in `always.standard` on every receipt;
- the critic: no lane chain closes without the reference of section 2;
- the receipt's fast gate: `always.canary` and `always.standard` are in
  every delivery plan (`select.go:191-192`), and a failed canary blocks
  every later stage;
- the DONE sentence as the acceptance test: it is the brief's only
  criterion, the critic's second question, and the landing note quotes it.

One amendment to the landing: `observeChain` at `observe.go:190-193`
refuses (as would-refuse) every root whose hazard is not DESIGN-BEARING or
DESTRUCTIVE-REACH. After `critique-always`, a MECHANICAL root carries a
critique reference too. The rule becomes: the root's class must require an
independent critique under `MinimumHazardConfiguration` and the root must
carry `independentCritiqueJobRef`; otherwise `chain-not-critiqued`, which
replaces `chain-not-design-bearing` and is added to
`landing-promotion.json`'s `refuseCodes`, since a chain that landed
uncritiqued is exactly what Wido forbade. A lane root additionally passes
only with `laneAdmission.verdict: admitted` (code `chain-lane-refused`, also
promoted); its provenance gains `lane=small`.

## 4. The budget

One new box the norm knows:

    metasystem.budget.lane-small = 4h/4/480m/1/2

Elapsed 4h: R-44-m0b's small-item elapsed, room for one build, one read, one
fold, one closing read, a receipt (`landing.receipt-bound-min=40`) and the
landing. Attempts 4: exactly those four dispatches; every terminal attempt
counts (R-22-m1). Reserved minutes 480: four times `dispatch.cap-max` (120,
`metasystem.conf:56`), because every dispatch reserves the full cap for the
life of the revision (R-58-m1); the pool is a runaway guard, not the spend.
Active 1. Review rounds 2: the read and the closing read. Under today's
per-root accounting each critic root freezes 2 and uses 1; under the
specimen goal's future per-goal accounting the two reads are the whole
allowance. Both are inside R-42-m0's ceiling of three.

Mechanics: `config.LaneBox(confPath, "small")` in `internal/config/budget.go`
beside `TierBox`, same five-member grammar, same `.local` and environment
refusals; `openRequest` (`verbs.go:512-531`) assigns the lane box when the
goal is opened with `--lane small` and no tuple; `goalNormApproval` and
`requireWithinGoalNorm` (`norm.go:100-161`) compare against the lane box
when the goal is in the lane, else the tier box. `set-budget` is never
needed for a lane goal: the box is the budget. Leaving the lane
(`goal edit --lane none --by <human>`) is one transaction that re-binds the
claim's revision and replaces the tuple with the tier's box, the same shape
as the risk raise after claim (severity-tiered-rigor, STR2-TIER-AUTHORITY-01);
it is a human act because it widens the budget. `job critique-budget-rebind
--root-job` then copies the new round member onto an open critic root, as
today.

`ApprovalDigest` (`file.go:181` per the severity-tiered-rigor page) hashes
`lane=<small|none>` between the tier and the tuple, so the approving human
sees the lane and its box together and a lane change after approval is the
unapprove, edit, approve path. A stored record without the field parses as
`Lane: none` and renders the line on its next write, the same migration
shape as the fifth budget member.

## 5. The seat's part

The coordinator does, in order, and nothing else:

1. `goal open --id <g> --lane small --risk ... --basis ... --intent "... DONE
   means ..." --next "<paths and the change>"`. The human approves.
2. `goal claim`, then `metasystem delegate --role implementer --goal <g>
   --lane small --worktree --destructive-reach MECHANICAL --wait`.
3. `validate conformance --stage review --job <root>`.
4. `metasystem delegate --role code-critic --goal <g> --lane small --reviews
   <root> --worktree --destructive-reach MECHANICAL --wait`; dispositions
   with `validate critique-closed`; at most one fold and one closing read
   as section 2.
5. `metasystem delegate close --root <critic>` and `--root <root>`.
6. `landing test-receipt --mode auto`, `land.sh --chain <root>
   --test-receipt <receipt> --goal <g>`; if the verdict carried
   `engineBinding`, rebuild and `steward arm`.
7. `goal done --id <g> --conclude "<landing note>"`.

The seat writes no brief, no per-round dispositions register in `plans/`,
no critique record in `records/misc/`. The critic root's finding register is
the record of the critique; `validate critique-closed` is the disposition.
The landing note is the goal's Conclude field, one paragraph: the DONE
sentence, the chain and critic ids, the receipt id and its selected group
count, the landing commit, and the rebuild-and-arm line when engine-binding.
It lives in the ledger through the verb, so no record path is touched by
hand. Process-to-payload for a one-line fix is then seven commands and one
paragraph.

## 6. Fixtures

Each fixture names where it fails on the untouched tree. Every command runs
from the repository root; Go tests carry a two-minute ceiling. Process
fixtures use the fake runtime and their own beds, as the existing scenarios
in `metasystem/scripts/agents/dispatch-fixtures.sh` and
`metasystem/scripts/agents/land-fixtures.sh` do.

| Fixture and owner | Smallest proving run | What it proves, and where it fails today |
| --- | --- | --- |
| Over the size bound: new `metasystem/internal/dispatch/lane_test.go`, `TestLaneAdmissionRefusesBounds` | `cd metasystem && go test ./internal/dispatch -run '^TestLaneAdmissionRefusesBounds$/^five-files$' -count=1 -timeout=2m` | A five-file diff refuses `LANE_FILES`; subtests `eighty-one-lines`, `rename`, `binary` refuse `LANE_LINES` and `LANE_SHAPE`; `four-files-eighty-lines` admits. Fails today: `LaneAdmission` does not exist, the package does not compile the test. |
| A plans/ change is refused: same file, `TestLaneAdmissionRefusesRecordPaths` | `cd metasystem && go test ./internal/dispatch -run '^TestLaneAdmissionRefusesRecordPaths$' -count=1 -timeout=2m` | A diff touching `plans/x.md` refuses `LANE_RECORD_PATH`; `memory/rulings.md` likewise; `docs/x.md` (class behavior, owned by `instructions`) admits. Fails today for the same reason. |
| Unowned and many-owner paths: same file, `TestLaneAdmissionSurfaces` | `cd metasystem && go test ./internal/dispatch -run '^TestLaneAdmissionSurfaces$' -count=1 -timeout=2m` | A path no surface owns refuses `LANE_UNOWNED_PATH`; three direct owners refuse `LANE_SURFACES`; one owner with four consumers admits and records `engineBinding: true` for an `internal/` path. Fails today for the same reason. |
| Open marks the lane and assigns the box: new `metasystem/internal/goal/lane_test.go`, `TestLaneOpenAssignsBoxAndDigest` | `cd metasystem && go test ./internal/goal -run '^TestLaneOpenAssignsBoxAndDigest$' -count=1 -timeout=2m` | Open with `--lane small` and accumulation 1 renders `- Lane: small` and the `lane-small` tuple; accumulation 2 refuses `LANE_WIDTH`; an Intent without `DONE means` refuses `LANE_DONE_SENTENCE`; the approval digest changes when the lane changes. Fails today: `OpenRisked` has no lane parameter and `GoalFile` has no `Lane` field. |
| The norm reads the lane box: extend `metasystem/internal/goal/approval_test.go` (the file that pins `GOAL_NORM_REFUSED` today), new `TestLaneBoxIsTheNorm` | `cd metasystem && go test ./internal/goal -run '^TestLaneBoxIsTheNorm$' -count=1 -timeout=2m` | A lane goal's `set-budget` to `4h/4/480m/1/2` needs no `--approved-ref`; `4h/5/600m/1/2` is `GOAL_NORM_REFUSED` naming the lane box; a tier-3 lane goal is judged by the lane box, not `tier-3`. Fails today: `config.LaneBox` does not exist. |
| Leaving the lane is a human act: same file, `TestLaneReleaseRebinds` | `cd metasystem && go test ./internal/goal -run '^TestLaneReleaseRebinds$' -count=1 -timeout=2m` | `goal edit --lane none` on a claimed goal without `--by` is refused; with a proven human it re-binds the claim revision, replaces the tuple with the tier box and writes one history line naming the code. Fails today: no `--lane`. |
| Dispatch refuses the wrong class and roles: extend `dispatch-fixtures.sh`, new scenario `lane-dispatch-admission` | `cd metasystem && METASYSTEM_DISPATCH_FIXTURE_SCENARIO=lane-dispatch-admission scripts/agents/dispatch-fixtures.sh` (the runner's scenario switch as it exists at `dispatch-fixtures.sh`; if none, the scenario runs inside the full script) | Under a lane goal, `--destructive-reach DESIGN-BEARING` refuses `LANE_HAZARD` with no husk left; `--role design-critic` refuses `LANE_ROLE`; `--brief` with `--lane` refuses; the rendered brief exists under `rounds/1/prompt.md` with the DONE sentence as the only acceptance line and the root carries `lane: "small"`. Fails today: `--lane` is an unknown option (`usage; exit 2` at `dispatch.sh:1335`). |
| Rendered brief passes authority: new `metasystem/cmd/metasystem/lane_brief_test.go`, `TestLaneBriefRendersFromRecord` | `cd metasystem && go test ./cmd/metasystem -run '^TestLaneBriefRendersFromRecord$' -count=1 -timeout=2m` | `job lane-brief` renders the implementer, code-critic and fold templates from a fixture record; a Next step naming a missing path makes `job brief-mode --authority-only` refuse. Fails today: the verb does not exist. |
| Clean critic lands on an area receipt: extend `land-fixtures.sh`, new scenario `small-lane-clean-chain-lands` | `cd metasystem && scripts/agents/land-fixtures.sh` (scenario selected as the file's existing scenarios are) | A fake-runtime lane chain with an admitted verdict, a clean closed critic and a schema-2 receipt whose plan is `always` plus one surface's standard groups lands with `pass bar=a` and `lane=small` in the provenance; the same chain without a critique reference is `chain-not-critiqued` and refused. Fails today: the observation is `would-refuse code=chain-not-design-bearing` (`observe.go:190-193`) and passes. |
| Material defect folds once, then closes or is refused: extend `dispatch-fixtures.sh`, new scenario `lane-fold-once` | as the scenario above | Critic round 1 registers one bounded material finding; one fold is admitted and its prompt enumerates the finding id; the closing read with zero material closes the chain; a second follow-up on the same root refuses `LANE_ROUNDS`; a closing read with a material finding parks the goal with `LANE_ROUNDS` in the reason. Fails today: no lane. |
| Design gap refuses the lane: same scenario, leg `design-gap` | as above | A critic finding whose artifact is `NEW <path>` writes `LANE_DESIGN_GAP` on the root; `close-check` refuses until the goal reads `Lane: none` at a later revision, after which the chain closes as an ordinary MECHANICAL chain. Fails today: no `laneAdmission` field, so close-check passes. |
| Close-check demands the verdict: extend `metasystem/internal/dispatch/decisions_test.go`, new `TestCloseCheckRequiresLaneVerdict` | `cd metasystem && go test ./internal/dispatch -run '^TestCloseCheckRequiresLaneVerdict$' -count=1 -timeout=2m` | A `lane: "small"` root with no verdict, or a verdict on an earlier round than the final work round, refuses; an admitted verdict on the final round passes. Fails today: no such field is read. |
| The hazard row after the flip: extend `metasystem/internal/dispatch/composition_test.go` (the file that pins the hazard table's equality today) | `cd metasystem && go test ./internal/dispatch -run 'Hazard' -count=1 -timeout=2m` | The MECHANICAL row reads maximal/xhigh, critique required, critique maximal/xhigh, live proof false, in both surfaces; a MECHANICAL chain without a distinct critic refuses closure. Fails today at `hazard.go:37-41`. This fixture is `critique-always`'s and R-90-m1's; the lane depends on it and does not own it. |
| Refusal codes registered: extend `metasystem/internal/refusal/register_test.go` | `cd metasystem && go test ./internal/refusal -count=1 -timeout=2m` | Every `LANE_*` code and `chain-not-critiqued`, `chain-lane-refused` have a row with owner, site and override. Fails today: the rows do not exist, and the fast gate's audit would flag an unregistered emitter. |

The gate the build hands to the orchestrator is the shared testing
contract's plan for the build's own candidate, in `auto` mode, through
`landing test-receipt`; this design requests no named battery. The build's
own goal is design-bearing and takes the ordinary ladder; it cannot ride the
lane it builds.

## 7. Rollout, and what the lane means for tier 1 and tier 3

**Goals already open.** No record carries `Lane`; every one parses as
`Lane: none` and nothing changes for them. The seat proposes lane marking
one goal at a time: before approval, `goal edit --lane small` by the seat;
for an approved goal, unapprove, edit, approve by Wido, or leave it on the
ordinary ladder. There is no bulk sweep: the lane is a judgment about the
record's completeness and the human's approval is the check on it. Three
candidates at this commit, named so the first specimens are not invented
later: R-90-m1's sweep of the MECHANICAL builder effort (two lines in
`hazard.go` and `role-packets.json` plus the test that pins them), which is
the lane's natural first specimen once `critique-always` has landed the
critique flip; the `critique-always` flip itself, which is two files and a
test but cannot ride the lane it enables and lands on the ordinary ladder;
and `cmd-tests-red-on-trunk-mac` if its diagnosis ends in a test fix rather
than a registration.

**Tier 1.** R-54-m1 defines tier 1 as a receipted direct fix with no
review. `critique-always`, approved and edited by Wido after that ruling,
requires a critique on every build chain regardless of class. When its flip
lands, a tier-1 chain must carry a critique (`hazard.go:208-215`) but tier 1
refuses every critic role (`dispatch.sh:743-746`): tier 1 becomes a rung no
chain can close on. The lane is the resolution: a tier-1 goal is a lane
goal by construction (`goal open --tier 1` sets `Lane: small` and refuses
`--lane none`), it lands as a lane chain with the lane's one critique, and
`require_goal_tier_ladder` admits the code-critic role under tier 1. The
tier-1 direct-fix landing class (`landing-classes.json` row `tier-1`,
`observeTierOne`, the `tier-1` declaration checks at `observe.go:122-127`,
the floor rows at `path-classes.txt:67-83` and `TierOneRefused`) is retired
by this build, not extended, because after `critique-always` there is no
lawful landing it can make. This is the point most likely to draw a critic's
finding; the argument is the dates: R-54-m1 on 2026-09-02 read
`critique-always` as not requiring every rung, and Wido's approval on
2026-09-06 and his own edit of the record on 2026-09-07 say every build
chain, regardless of class. The landing carries a rulings-register row
recording that R-54-m1's tier-1 clause is superseded on the review point
only; the tier-1 box and the "no design round" clause stand.

**Tier 3.** The risk derivation puts every exposure-3 change at tier 3
(`file.go:100-106`), and R-54-m1 gives tier 3 a design round. A tier-3
goal in the lane has no design round: the record's DONE sentence is the
design, the human approved the lane marking with the record, and the
critic's `LANE_DESIGN_GAP` is the mechanical check that the sentence really
decided the change. The tier and its receipt depth are untouched. A seat
that marks a design-bearing goal small will meet the critic's finding and
the human's re-box, which is the same cost as misclassification today and
is visible in the ledger.

**Order of landing.** `critique-always` first (its flip is a dependency of
sections 2 and 3, and it is approved with a one-hour appetite); then this
build; R-90-m1's builder-effort sweep may land before, with, or as the first
lane specimen after. If `critique-always` has not landed when this build
starts, the build's part 1 carries the MECHANICAL row as section 2 states it,
with that goal's three proofs as its tests, and the seat records on both
goals that the row landed under the lane; a row landed twice is a
conflict, so the seat checks before dispatch.

## 8. Where the lane joins the other two legs

The severity-tiered rigor page put the tier on the goal, made the tier box
the norm, made the material stop mechanical and gave the critic register its
close. The lane is built on all four: it is a fifth field beside the tier,
a fourth box the norm reads first, a register-closed critique, and a
material stop that names the design gap. It differs in one thing: the tier
answers "how much is at risk", the lane answers "does the record already
decide the change, and is the change small enough that one read proves
it". The staged-batch witness, as the goal record describes it, batches the
broad proof across landings at cadence; the lane lands on its own
per-candidate receipt and leaves the catch classes to the witness, exactly
as every landing does under the testing contract's section 8. The lane adds
nothing to cadence and takes nothing from it.

## Build list

Four parts, each a chain under this goal on the ordinary ladder.

- Part 1, the record and the box: `Lane` field, render and parse, the
  digest, `--lane` on open and edit with `LANE_WIDTH` and
  `LANE_DONE_SENTENCE`, the release transaction, `config.LaneBox`, the norm
  by lane, `goal open --tier 1` implying the lane. Files:
  `metasystem/internal/goal/file.go`, `verbs.go`, `norm.go`, `approval.go`,
  `metasystem/internal/config/budget.go`,
  `metasystem/cmd/metasystem/goalsync_mutations.go`,
  `metasystem/metasystem.conf`, `metasystem/internal/refusal/register.go`,
  tests as section 6. Carries the MECHANICAL row only under the condition in
  section 7.
- Part 2, dispatch and the brief: `--lane` on `delegate`, `LANE_HAZARD`,
  `LANE_ROLE`, `LANE_ROUNDS`, the root field, `job lane-brief` and the three
  templates, `require_goal_tier_ladder` admitting the code-critic under tier
  1, the admission code list. Files: `metasystem/scripts/agents/dispatch.sh`,
  `metasystem/cmd/metasystem/delegate.go`, `dispatch_verbs.go`, `main.go`,
  `metasystem/internal/dispatch/admission.go`, `build.go`, new `lane.go` and
  `lanebrief.go`, `metasystem/scripts/agents/templates/lane-brief.md`,
  `lane-review-brief.md`, `dispatch-fixtures.sh`.
- Part 3, review, close and landing: `LaneAdmission` at the review stage,
  `laneAdmission` on the root, `LANE_DESIGN_GAP` at the fold, the close
  duty, `chain-not-critiqued` and `chain-lane-refused` with their promotion
  rows, the tier-1 class retirement. Files:
  `metasystem/internal/validate/conformance.go`,
  `metasystem/internal/dispatch/close.go`, `finding_register.go`,
  `metasystem/internal/landing/observe.go`, `tierone.go` (deleted),
  `promotion.go`, `metasystem/internal/pathclass/pathclass.go`,
  `metasystem/scripts/agents/landing-classes.json`, `landing-promotion.json`,
  `path-classes.txt`, `land.sh`, `land-fixtures.sh`.
- Part 4, docs and rulings: `metasystem/docs/orchestration.md` (the
  collaboration loop's tier-1 sentence and the working-modes table),
  `metasystem/skills/code-critique/SKILL.md:64-66`, `metasystem/AGENTS.md`'s
  intake paragraph, the rulings row of section 7, and the obligation matrix
  row for this page in
  `metasystem/records/misc/severity-tiered-rigor-obligation-matrix.md`.

## Revision record

Revision 1 (2026-09-10, `scl-design1`). Decided: the lane is a record field
orthogonal to tier, bound into the approval digest; admission in three
moments with registered codes; four files and eighty lines; record and
ledger paths refused; engine binding recorded, never refused; one build
round on a rendered brief; one critique with at most one fold and one
closing read, on the code-critic lane at xhigh; `chainClosed` gains the
lane verdict; the receipt is the contract's `auto` plan; the box
`4h/4/480m/1/2`; the seat's seven commands and one paragraph; tier 1
becomes a lane goal and its direct-fix class is retired; tier 3 in the lane
has no design round.

Left to Wido, with recommendations:

1. Receipt depth for an exposure-3 one-liner. The contract selects the
   owning surfaces' deep groups whenever exposure or novelty is 2 or more,
   whatever the diff's size; the brief for this page wished for standard
   groups only. Recommendation: keep the contract's rule and let the
   gate-fence goals and group-evidence reuse (testing contract, section 6)
   bring the cost down; a size-based lowering is the "lines changed decide
   risk" rule the contract refused on purpose.
2. Retiring the tier-1 direct-fix class. Recommendation: retire it with
   this build, because `critique-always` leaves it no lawful landing, and
   record the supersession of R-54-m1's review clause as a ruling row.
3. The bounds. Four files and eighty lines are this page's numbers, with
   the reasoning in section 1; they are two constants in `lane.go` and a
   fixture each, so a different pair costs one lane change later.

What the code could not answer: the staged-batch witness has no page in the
tree, so section 8 relates the lane to it from the goal record's sentence
alone; and whether `dispatch-fixtures.sh` and `land-fixtures.sh` expose a
single-scenario switch was not established in this read, so the fixture
table names the scenario and lets the build pick the runner's real switch.
