# The small-change lane (goal small-change-lane)

- Kind: design
- Id: 01M3A2YHDNX4ED00Y4PVBQTV29
- Status: accepted
- Goals: small-change-lane

Revision: 3, the final fold. Date: 2026-09-16. Author: a Claude Fable design
delegate for seat m1e (Wido, 2026-09-13: design is a Fable delegate's; the
seat drafts of 2026-09-13 are withdrawn and were mined for code facts only).
Critique: two Codex Sol rounds of the three (R-97-m1e). Round 2 said rework
with twelve material findings (SCL-R2-M01 to M12) and reopened five round-1
findings; this revision folds every one under R-117-m1e item 2 (a last round
that says rework closes with local corrections when every remaining material
finding folds locally; anything that needs a ruling goes on the question
list at the end). The seat checks each fold against the critique's own
reopening trigger (its "Rigor classification" table) before landing this
page; the table at the end names, per finding, where the fold is and how the
trigger is met. Every code fact below was re-read at `origin/main`
`7e99bb110` (fetched 2026-09-16 about 11:10 local); a fact this revision
could not re-verify is marked ASSUMPTION. Paths are relative to
`metasystem/` unless a line says otherwise.

Inputs: revision 2 (this delegate's lineage, 2026-09-16 09:50) and its
inputs (revision 1 `scl-design1`, the Codex Sol read of the seat's revision
2, Wido's three decisions of 2026-09-10); the Codex Sol critique of revision
2; the goal record `plans/goals/small-change-lane.md` at revision 119 with
Wido's four answers of 2026-09-16 and his reading of the whole ("the tiers
decide whether a change may use the lane, the box whether it fits, and the
clause plus the reader guard against a large change sliding through on the
small path"); rulings R-3 (as amended 2026-09-11), R-54-m1, R-60-m1,
R-90-m1, R-93-m1e, R-108-m1c, R-114-m1e to R-119-m1e; goal 19 of
`plans/delivery-efficiency-plan.md`.

This page specifies the build; it implements nothing. The contract is the
Intent of `plans/goals/small-change-lane.md` at revision 119: DONE (1) the
lane admits a change of tier 1 or 2 within recorded bounds (at most four
files and eighty changed lines, no protected surface); (2) its stages are
exactly build in a worktree, the focused tests named in the brief, one
independent read at the tier's depth, a receipt in standard mode, land; no
design page, no register, no conformance round; (3) a change that outgrows
the bounds mid-lane overflows to the ordinary pipeline with its work
carried, never silently; (4) a real one-line fix goes from claim to landed
in under 45 minutes of seat time and under 3 delegate rounds, recorded in
the receipt; and RUNTIME INDEPENDENCE: the mechanism lives in the engine,
the ledger verbs and the adapter contract, proven on two runtimes where a
runtime path is touched.

## 0. Dispositions of the round-1 read

One row per material finding of the Codex read of the seat's revision 2,
one per minor finding, one per 2026-09-10 decision, and the rows revision 2
added (NEW-MISSED). Rows this revision changes say "r3:". The round-2
findings have their own table at the end of the page.

| Finding | Disposition | Where |
| --- | --- | --- |
| M01 tier 3 admitted contrary to DONE (1) | FOLDED; confirmed by Wido (2) "tier 3 never enters the lane". `LANE_TIER` at open and edit. `DerivedTier` is `max(severity, novelty)` (`internal/goal/file.go:134-136`); exposure no longer forces tier 3. | section 2 |
| M02 no protected-surface refusal | FOLDED and SUPERSEDED-BY-WIDO (2): the rule-scope clause is wider than the predicate. `LANE_SCOPE` refuses the refusal register, a protected policy path (`internal/testpolicy/protection.go:37-49`) and the behavior-surface policy with every reader of it, whatever the size. r3: the readers are enumerated by one tested owner, not a hand list (round-2 M02). | section 2 |
| M03 tier-1 branch routed through the chain landing, wrong box, no `--blocks` | SUPERSEDED-BY-WIDO (2) and (3): one box for tiers 1 and 2; inside the lane every change has a non-author reader, so a tier-1 lane chain lands through the chain observation with its reader, and the tier-1 direct-fix class (`scripts/agents/landing-classes.json` row `tier-1`, `:18-19`) stays untouched OUTSIDE the lane. One lane budget box for both tiers (the tier-1 box has zero review rounds, `internal/config/budget.go:248`). `--blocks` on a seat's open (R-93-m1e, `internal/goal/verbs.go:729-731`). | sections 2, 6, 8 |
| M04 a MECHANICAL critic root is refused at close | FOLDED: the code-critic root is dispatched DESIGN-BEARING; closure compares the critic's builder rows with the required critique rows and says so (`internal/dispatch/hazard.go:351-358`). The work root stays MECHANICAL. | section 3 |
| M05 the in-lane fold cannot close | FOLDED and SUPERSEDED-BY-WIDO (3), DONE (2) literal: one read, no in-lane fold. r3: the clean path is executable end to end: register advance, the engine-rendered zero-material dispositions artifact, the join, the critic root closed first under the ordinary critic duties, then the work root (round-2 M05, M07). | section 3 |
| M06 the register cannot write `LANE_DESIGN_GAP` | FOLDED: no register side effect. A material finding on the one read is the lane's design gap, read off the critic's return, and overflows the goal with code `LANE_CRITIQUE`. | section 4 |
| M07 overflow by plain goal edit breaks the approval digest | SUPERSEDED-BY-WIDO (4) and FOLDED: `goal overflow`, a real verb, re-derives the approval digest and rebinds the claim revision the way `set-budget` does (`verbs.go:1246`); the work becomes a unit of an existing goal. r3: the implementer follow-up after overflow is admitted, keyed to the goal's current state (round-2 M06). | section 4 |
| M08 stale proof-depth law; `--mode auto` is not "standard mode" | SUPERSEDED-BY-WIDO (1) and FOLDED: the changed surfaces decide (`internal/testpolicy/select.go:156-173`, `risk.go:106-115`, R-3 as amended); the landing receipt is requested `standard` literally. r3: `LANE_DEPTH` derives the required mode from the contract's auto-mode selection, which never errors on insufficiency, and two surfaces do raise risk today (round-2 M03). | section 5 |
| M09 brief without Working Mode or a replayable test | FOLDED: `Working Mode: implement` and `Working Mode: review` (`scripts/agents/templates/brief.md:1`; `scripts/agents/dispatch.sh:1545` requires the header); `LANE_TEST` requires the `Test:` and `Paths:` segments in the Next step; the rendered brief carries them and `Boundary`, `Ceiling`, `Changed-line allocation` (`internal/dispatch/brief.go:151-201`, `brief.md:5`). | sections 2, 3 |
| M10 unowned paths invisible to selection | r3: FOLDED differently. An unowned path falls to the `residual` surface (`testing.json:4`, `select.go:132-135`), which declares `risk: severity 2` (`testing.json:27`), so `requiresDeep` is true (`risk.go:110-114`) and `LANE_DEPTH` refuses it. No `DirectOwners` and no `LANE_UNOWNED_PATH`: an extra eligibility gate is what round-2 M04 forbids. | sections 2, 5 |
| M11 R-90-m1's builder row is not a prerequisite | FOLDED: unit U0 flips the MECHANICAL builder row to maximal/xhigh in `hazard.go:37-41` and `scripts/agents/role-packets.json:4-11`; `internal/dispatch/build.go:486-488` refuses a differing builder effort, so nothing in the lane dispatches before U0. | section 12 |
| M12 fixture commands that do not run | FOLDED: every process fixture is a scenario of its bed (parent runs the engine-selected set; child protocol `--fixture-bed-child <scenario> <capability>`, `scripts/agents/fixture-bed-scenarios.sh:132-136`). r3: the exact public group ids and the exact catalog edits (round-2 M10). | section 13 |
| M13 no DONE (4) measurement | FOLDED. r3: seat-attended minutes from recorded stamps and the verb's injected clock, recorded before landing, no commit hash in the receipt (round-2 M12). | section 9 |
| M14 no two-runtime proof | FOLDED. r3: paired same-role evidence, hash equality only for the same template and input (round-2 M11). | section 10 |
| m01 stale deep-group count | FOLDED: stated as the rule, not a count. | section 5 |
| m02 no failsafe round declared | FOLDED: header. | header |
| m03 "no conformance round" imprecise | FOLDED: no delegated conformance-review round; `validate conformance --stage review` is the engine's computation of the round's diff (`internal/validate/conformance.go:362-434`) inside the seat's flow; the merge stage is the mission wall's and the lane never enters it (`conformance.go:451-520`, `internal/landing/observe.go:593-660` reads the review artifacts directly). | section 3 |
| 09-10 decision (1) proof depth by the four answers | SUPERSEDED by Wido (1) of 2026-09-16. | section 5 |
| 09-10 decision (2) critique follows the tier; tier-1 direct fix not retired; R-54-m1 stands | KEPT, reconciled by Wido (3): outside the lane, tier 1 is the direct fix with no critique; inside the lane every change has a non-author reader. | section 8 |
| 09-10 decision (3) four files, eighty lines, computed diff, overflow not failure, ten-landing review | KEPT, extended by Wido (2): one box for both tiers plus the rule-scope clause; the number is a recalibratable bound. | section 2 |
| NEW-MISSED N1: a fresh critic root's register starts at round 0 (`build.go:613-614`) and close-check refuses until the folded round equals the terminal round (`internal/dispatch/close.go:40-59`); `delegate close` runs `critique-register-close`, not the advance (`dispatch.sh:3018-3020`) | FOLDED: the seat runs `job critique-register-advance` (`cmd/metasystem/main.go:206`) before `validate critique-closed`. | section 3 |
| NEW-MISSED N2: `require_goal_tier_ladder` refuses every critic role and `--reviews` at tier 1 (`dispatch.sh:798-813`) | FOLDED: under `Lane: small` the code-critic role and `--reviews` are admitted at tier 1; design-critic stays refused. | section 3 |
| NEW-MISSED N3: revision 1 cited a tier-1 hazard refusal at `admission.go:211-214`; revision 2 could not find it | r3: FOUND at `origin/main`: `internal/dispatch/admission.go:232-234` refuses a non-MECHANICAL class under a tier-1 goal (`HAZARD_REFUSED`, register row `internal/refusal/register.go:77`). A DESIGN-BEARING critic root under a tier-1 lane goal would be refused there. FOLDED: under `Lane: small` that refusal admits the code-critic root's DESIGN-BEARING class (the lane's reader is the review the tier lacks); the implementer root stays MECHANICAL and is unaffected. The ASSUMPTION is withdrawn. | sections 2, 3 |
| NEW-MISSED N4: the brief carries `Boundary` and `Ceiling` and the review stage refuses paths outside the boundary | r3: the claim that the brief's `Boundary` bounds the diff is WITHDRAWN. The review stage checks changed paths only against the implementer returns' own `diffBoundary` declarations (`conformance.go:736-831`); `ParseBriefBounds` validates the header's syntax and authority (`brief.go:151-201`). The lane's path bound is its own check, `LANE_PATHS` (round-2 M01). | section 2 |
| NEW-MISSED N5: `delegate close` takes `--job <root>` (`dispatch.sh:2981`), not `--root` | FOLDED in the seat's commands. | section 7 |
| NEW-MISSED N6: "engine projection" in Wido (2) has no parenthesis | DECIDED by the seat ruling of 10:02 (kept below): the projection's declaration and readers, not every ENGINE member. r3: the readers are enumerated by one owned predicate with a test that fails when a reader appears unlisted (round-2 M02). | section 2, seat rulings |
| NEW-MISSED N7 (r3): the dispositions join requires a dispositions file even for zero findings (`cmd/metasystem/validate_verbs.go:74-95`; `internal/validate/critiqueclosed.go:157-205`) | FOLDED: `job lane-dispositions`, the engine-rendered artifact (round-2 M07). | section 3 |

## 1. Decisions

1. The lane is a record field, `- Lane: small`, on a goal of tier 1 or 2,
   bound into the approval digest. Tier 3 is refused (`LANE_TIER`).
2. Eligibility is exactly three predicates (Wido's reading of the whole,
   goal record revision 119): the TIER decides whether a change may use the
   lane; the BOX decides whether it fits: at most four changed files and
   eighty changed lines (additions plus deletions; no rename, copy, binary or
   mode-only change), every changed path a member of the goal's recorded
   `Paths:`, measured on the computed diff at the review stage; the
   RULE-SCOPE CLAUSE plus the reader guard against a large change sliding
   through: a diff touching the refusal register (`internal/refusal/`), a
   protected policy path (`testpolicy.ProtectedPolicyChange`) or the
   behavior-surface policy and any reader of it is outside the box whatever
   its size. Nothing else gates eligibility: not the accumulation answer
   (it sets the gate width, `file.go:138-143`, and widens the standard plan
   by the surfaces' cross-cutting obligations, `select.go:182`), not the
   number of owning surfaces, not the path class (the landing's class
   refusals, `ledger-path-not-goal-verb` and `record-not-owned` in
   `scripts/agents/landing-promotion.json:3`, judge every chain already).
   The numbers are a bound the ten-landing review recalibrates from the
   DONE (4) measurements, not law.
3. The stages are DONE (2)'s and nothing more: one build round on a brief
   the engine renders from the record; one non-author read on the
   code-critic lane (claude, Opus 5, R-108-m1c) at xhigh, dispatched
   DESIGN-BEARING; no fold inside the lane; a receipt requested in standard
   mode; the landing. Clean closes; material overflows. "No register" means
   no new refusal-register row is authored in-lane (Wido (3), read with (2));
   a change that needs one leaves the lane. The critic root's finding
   register is the engine's record of the read, not an entry authored by
   the change. `LANE_DEPTH` (decision 5) is DONE (2)'s "receipt in standard
   mode" made checkable, not a fourth eligibility predicate.
4. Overflow after claim is `goal overflow`, the seat's recorded
   transaction: `Lane: none`, the approval digest re-derived, the claim
   revision rebound, the tuple unchanged, and the goal recorded as a unit
   of the named existing goal. Never a plain `goal edit`. After a recorded
   overflow the chain's implementer follow-up is admitted; before it, it is
   refused.
5. Proof depth: the changed surfaces decide. `LANE_DEPTH` asks the shared
   testing contract, in its own auto mode, what the changed paths require;
   `deep` overflows. The landing receipt is then requested `standard`
   literally and is sufficient by construction. The tier's duties are the
   floor under it; a fix found by a proof names the attempt that revealed
   it and never claims a solo proof.
6. Outside the lane, tier 1 keeps its direct-fix class (R-54-m1). Inside
   the lane, a tier-1 change has the lane's reader and lands through the
   chain observation; the tier-1 hazard refusal at dispatch admits the
   reader's DESIGN-BEARING root under a lane goal. Section 8 says which path
   a tier-1 change takes.
7. The reviewed tree lands through the engine: `landing materialize`
   carries the chain's certified diff into the landing checkout; the RECEIPT
   line is appended before `land.sh`; the landing commit carries both. The
   receipt line names the reviewed tree and the seat's measured minutes,
   never its own commit hash, which the goal's Conclude records afterwards.
8. The lane's own build takes the ordinary ladder (Fable design, Codex Sol
   build per unit in a worktree, Opus read, critique by another model) and
   cannot ride the lane it builds. It changes no hook setting and no deny
   mode (R-118-m1e).

## 2. What "small" is, mechanically

A goal is in the lane when its record carries `- Lane: small`. Risk stays
the four answers and keeps deciding the tier and the width; the lane
decides the ladder's shape and the box. A stored record without the field
parses as `Lane: none` and renders the line on its next write (the fifth
budget member's migration shape).

`goal open --lane small` sets it; `goal edit --lane small|none` changes it
before approval by any actor; after approval the digest binds it
(`ApprovalDigest`, `file.go:232-241`, gains `lane=<small|none>` between the
tier and the tuple, and `ValidateApprovalRecord`, `file.go:805-813`,
recomputes it), so the lawful ways out after approval are unapprove, edit,
approve (`main.go:517`) or, after claim, `goal overflow` (section 4). An
edit that would leave the lane on an approved or claimed goal is refused
and names the verb.

Every lane refusal is a registered row (`internal/refusal/register.go`,
`Row{Code, Owner, Site, Shape: Agent, Override}`, `register.go:13-21`,
`Rows` at `:35`), so the fast gate's register audit sees it; the rows are
authored by the lane's own build on the ordinary ladder, never in-lane
(decision 3).

**Eligibility, the three predicates.** The tier (`LANE_TIER`, at open and
edit). The box (`LANE_FILES`, `LANE_LINES`, `LANE_SHAPE`, `LANE_PATHS`, at
review). The clause (`LANE_SCOPE`, at review). Every other `LANE_*` code on
this page is a stage-shape rule fixed by DONE (2) (what the brief needs,
what may dispatch, what the receipt is) or the overflow signal, not an
eligibility gate; revision 2's `LANE_WIDTH`, `LANE_SURFACES`,
`LANE_UNOWNED_PATH` and `LANE_RECORD_PATH` are removed (round-2 M04).

**At open and edit** (`OpenRisked`, `verbs.go:696`; `Edit`, `verbs.go:2515`;
flags in `cmd/metasystem/goalsync_mutations.go`, `--blocks` at `:625`,
`--tier` at `:648`):

- `LANE_TIER`: the effective tier (`f.Tier`, which `editRequest` keeps at
  or above `DerivedTier`, `verbs.go:2548`, `:2574-2578`) must be 1 or 2. A
  risk edit that raises a lane goal to tier 3 overflows it (section 4) when
  claimed and is refused before claim.
- `LANE_DONE_SENTENCE`: the Intent contains the literal `DONE means` with
  at least one sentence after it. That clause is the brief's only
  acceptance criterion and the critic's threat model.
- `LANE_TEST`: the Next step (one record line) carries two segments, each
  beginning at the text start or after ` || ` and ending at the next ` || `
  or the text end: `Test: <one shell command from the repository root, no
  pipe into another tool>` and `Paths: <path>[, <path>...]` naming every
  path the change may touch, tests included, at most four, each existing at
  HEAD or marked `new`. Parser `goal.LaneFields(next)` in
  `internal/goal/lane.go`. The brief copies both verbatim; the critic reruns
  the test; the landing note quotes it; the review stage compares the
  computed diff with `Paths:` (`LANE_PATHS`).
- A seat's open names `--blocks <its claimed goal>` (R-93-m1e,
  `verbs.go:729-731`, `:842-844`); the blocked goal parks and returns on
  `done` (`returnBlockerParks`, `verbs.go:897`). A human's open needs no
  blocker.

Witness: `internal/goal/lane_test.go`, `TestLaneOpenAdmission` with
subtests `tier-three`, `no-done-sentence`, `no-test-segment`,
`no-paths-segment`, `five-paths`, `absent-path`, `admits`;
`TestLaneChangesTheApprovalDigest` (the digest differs between `Lane: small`
and `Lane: none`, and an approval made at `small` fails validation after a
hand edit to `none`); `TestLaneEditAfterApprovalNamesTheVerbs`.

**At dispatch** (`dispatch.sh` after the goal binding at `:1635-1641`, which
gains a `lane` field from `job goal-binding` (`GoalBinding`,
`internal/dispatch/stop.go:32`, `ResolveGoalBinding`, `:54`); the engine
side beside `EvaluateGoalRevisionAdmissionForDispatch`,
`internal/dispatch/admission.go:189`):

- `LANE_HAZARD`: under a lane goal an implementer root is dispatched
  `--destructive-reach MECHANICAL` and a code-critic root `DESIGN-BEARING`;
  any other pairing is refused with "the change is not mechanical; leave
  the lane with goal overflow". Added to `AdmissionRefusalCodes`
  (`admission.go:15`) so it is citable as misclassification evidence.
- The tier-1 hazard refusal (`admission.go:232-234`: `binding.Tier == 1 &&
  hazard != HazardMechanical` is `HAZARD_REFUSED`) admits the code-critic
  role's DESIGN-BEARING class when the binding reads `Lane: small`; the
  lane's reader is the review the tier lacks, and the implementer root is
  MECHANICAL either way. Design-critic stays refused.
- `LANE_ROLE`: only implementer and code-critic dispatch under a lane goal.
- `LANE_ROUNDS`: an implementer follow-up on a root that carries
  `lane: "small"` is refused while the goal's record at the current
  accepted revision (`job goal-binding`) still reads `Lane: small`; it is
  admitted, as an ordinary follow-up, once the record reads `Lane: none`
  with an `Overflow:` line (section 4). The frozen root field says where
  the chain came from; the goal's current state says what may follow. The
  message names `goal overflow`.
- `LANE_BRIEF`: `--lane` and `--brief` together are refused; the brief is
  rendered (section 3).
- The root record gains `lane: "small"`, frozen beside `goalTier`
  (`build.go:551`) and inherited by follow-ups like `gateWidth`
  (`build.go:833-834`, `:874-880`), so an overflowed chain's later rounds
  still say where they came from.

Witness: `scripts/agents/dispatch-fixtures.sh` scenarios
`lane-dispatch-admission` and `lane-one-read` (section 13); the second
proves `LANE_ROUNDS` both ways on the same root: refused before `goal
overflow`, admitted after it.

**At review** (`validate conformance --stage review --job <root>`, which
computes `reviewedTree`, `diff.patch` and the changed paths,
`conformance.go:362-434`; new `LaneAdmission` in
`internal/dispatch/lane.go`, called by the review stage when the root
carries `lane: "small"`, its verdict written on the root as
`laneAdmission {round, verdict, code, files, lines, paths, affectedSurfaces,
requiredMode, engineBinding}` and re-evaluated on every later review, the
latest winning). When several eligibility checks fail, the refusal's one
`code` is the first failing check in the order this page lists them (tier,
box, clause, depth; inside the box `LANE_FILES`, `LANE_LINES`,
`LANE_SHAPE`, `LANE_PATHS`), and each witness below is chosen so that the
check it names is the first to fail, the depth witness failing that check
alone. Every path below is project-relative, as the review stage
speaks them:

- `LANE_FILES`: more than four changed paths. `LANE_LINES`: more than
  eighty changed lines, additions plus deletions. `LANE_SHAPE`: a rename,
  copy, binary or mode-only change. All three read one owner,
  `gittree.Workspace.DiffShape` (section 11), the count the tier-1 direct
  fix reads today.
- `LANE_PATHS`: a changed path that is not a member of the goal revision's
  `Paths:` set (the root's `goalRevision`, `build.go:550`). This is the
  lane's own check: the review stage's boundary is the union of the
  implementer returns' `diffBoundary` claims (`conformance.go:736-831`) and
  a return that truthfully declares an unauthorized path passes it; the
  brief's `Boundary` header is validated for syntax and authority only
  (`brief.go:151-201`). The receipt ledger is never in the reviewed diff
  (it is appended in the landing checkout, section 3), so it needs no
  exemption here.
- `LANE_SCOPE`, the rule-scope clause (Wido (2)): any changed path that is
  (a) under `internal/refusal/`; (b) a protected policy path,
  `testpolicy.ProtectedPolicyChange` (`protection.go:37-49`: `testing.json`,
  `internal/testpolicy/**`, `internal/proofrun/test_*`, the coverage
  ratchets, `validate-section-selector.sh`, `commit.sh`, `land.sh`); or
  (c) the behavior-surface policy or a reader of it,
  `behaviorsurface.ProjectionReader(path)`. The policy declares all three
  projections (`internal/behaviorsurface/policy.go:25-31`,
  `policy.v2.json`); its readers are every non-test Go file that imports
  the package and every script that calls the `behavior-surface` verb. At
  `7e99bb110` that set is: `internal/behaviorsurface/**`;
  `cmd/metasystem/behavior_surface.go`, `cmd/metasystem/proof_run.go`
  (`:1426`, `:1446`), `cmd/metasystem/rearm_on_landed.go` (`:143`, `:166`),
  `cmd/metasystem/test.go` (`:618`); `internal/dispatch/governed.go`
  (`:25-27`); `internal/gaterun/weight.go` (`:221`, `:248`);
  `internal/proofrun/coverage.go` (`:16`, `:241`);
  `internal/proofrun/manifest.go` (`:156-168`);
  `internal/steward/rearm_resolver.go` (`:112`, `:187-221`, `:330-354`);
  `scripts/agents/commit.sh` (`:469`), `scripts/agents/go-build.sh`
  (`:68`), `scripts/agents/go-gate.sh` (`:172`, `:314`, `:598`),
  `scripts/adopt-fixture-helpers.sh` (`:205-208`). Revision 2's list named
  `internal/proofrun/freeze.go` and `internal/landing/registers.go`, which
  do not read the policy, and missed the rest; the list on this page is
  not the owner either: `ProjectionReaders()` in
  `internal/behaviorsurface/readers.go` is, and its test walks the module
  for both kinds of reader and fails on any difference in either direction.
  The cost is stated plainly: a one-line fix inside `cmd/metasystem/test.go`
  or `proof_run.go` is outside the lane until the ten-landing review says
  otherwise. The clause is not "every member of the ENGINE projection"
  (`cmd/**`, `internal/**`, `scripts/agents/**`, `policy.v2.json:3-10`),
  which would close the lane to every engine one-liner (seat ruling 1).
- `LANE_DEPTH`: `testpolicy.Select(contract, {ChangedPaths, RequestedMode:
  auto, Purpose: delivery, GoalRisk: the goal's answers})` returns a plan
  whose `RequiredMode` is `deep` (`select.go:95-99`, `:167-173`; `Plan`,
  `:40-52`). Auto never returns the insufficiency error, which is raised
  only for a standard request against a deep requirement
  (`select.go:212-215`); the plan is the evidence and is recorded on the
  verdict as `requiredMode`. Today `deep` is owed for a protected path
  (already `LANE_SCOPE` (b)), an unowned path (the `residual` fallback
  declares `risk: severity 2`, `testing.json:4`, `:27`), or a surface with
  a declared raise: `coverage-policy` and `context-budget` both declare
  `risk: {severity: 2}` (`testing.json:14`, `:16`; the field is `risk`,
  `internal/testpolicy/contract.go:51-59`), so a change to
  `internal/proofrun/coverage.go` or `internal/usage/**` is refused here
  although neither is protected. Revision 2's "no surface declares a raise"
  was false.
- Engine binding is recorded, not refused: a changed path under
  `internal/`, `cmd/` or `scripts/agents/` sets `engineBinding: true`
  (the skew preflight's "engine or agent scripts", `dispatch.sh:284`), and
  the landing note carries the re-arm line (section 7).

Witness: `internal/dispatch/lane_test.go`: `TestLaneAdmissionBounds`
(`five-files`, `eighty-one-lines`, `rename`, `binary`, `mode-only` refuse;
`four-files-eighty-lines` admits); `TestLaneAdmissionPaths` (a computed
diff over `internal/dispatch/x.go` and `internal/dispatch/extra.go`, both
declared in the return's `diffBoundary`, under a goal whose `Paths:` names
only `x.go`, refuses `LANE_PATHS`; the same diff under `Paths: x.go,
extra.go` admits); `TestLaneAdmissionScope` (one path each under
`internal/refusal/`, `internal/testpolicy/`, `internal/behaviorsurface/`,
and `cmd/metasystem/rearm_on_landed.go` refuse `LANE_SCOPE`;
`internal/dispatch/x.go` admits with `engineBinding: true`);
`TestLaneAdmissionDepth` (under the tip's `testing.json`,
`internal/usage/retention.go` refuses `LANE_DEPTH` with `requiredMode:
deep` and a non-empty plan: it is owned by `context-budget`, which declares
`risk: {severity: 2}` (`testing.json:16`), is not a protected policy path,
and is not a behavior-surface reader, so depth is the only check it fails;
a path no surface owns, which falls to the `residual` surface
(`testing.json:4`, `:27`, also `risk: {severity: 2}`), refuses the same way;
`internal/dispatch/x.go` admits with `requiredMode: standard`).
`internal/behaviorsurface/readers_test.go`,
`TestProjectionReadersEnumerateEveryConsumer` (walks non-test `.go` files
for the import path and `scripts/**` for the verb; the enumerated set and
the walked set are equal; a fixture file added to a temporary copy of the
module fails it).

A refusal at review does not fail the chain and waits for no one (Wido's
09-10 decision (3)): the seat records the overflow (section 4) and the
chain continues on the ordinary ladder with its evidence intact.

## 3. The shape of the lane

**Build round.** One implementer round on the implementation lane (codex,
gpt-5.6-sol, R-108-m1c) at xhigh, which is the MECHANICAL builder row once
unit U0 lands R-90-m1 (`hazard.go:37-41`; `build.go:486-488` refuses any
other effort). Dispatch: `metasystem delegate --role implementer --goal <g>
--lane small --worktree --destructive-reach MECHANICAL`, no `--brief`. The
dispatcher calls the new engine verb `job lane-brief --repo <root> --goal
<g> --role implementer --out <file>` and passes the file where the caller's
brief goes today (`--brief`, `dispatch.sh:1441`), so it takes the same
header check (`brief_mode`, `dispatch.sh:858`, `:1545`: one filled Working
Mode; Boundary and Ceiling together), brief authority (`:862`: every cited
path exists in the delegate's tree), the testing requirement
(`build.go:1136-1140`), packet composition and record hashing.

Template `scripts/agents/templates/lane-brief.md`, filled only from the
record at the claimed revision:

- `Working Mode: implement`; Orchestrator Identity and Date as today;
  `Boundary: <JSON array of the Paths: members>`; `Ceiling: 80`;
  `Changed-line allocation: 80` (`brief.md:5`; `brief.go:151-201` parses
  the pair).
- Goal: the Intent verbatim. Workspace: the job worktree; only the
  `Paths:` members; never a record or ledger path.
- Inputs: the Next step verbatim; the `Test:` command as the named focused
  test the return must show as `ran`
  (`scripts/agents/schemas/implementer.schema.json:27-31`,
  `evidence[].level`).
- Constraints: the box in words (four files, eighty lines, only the
  `Paths:` members, no rename or binary, no refusal-register row, no
  protected path); the return's `diffBoundary` names exactly the paths it
  changed; one retained test that fails without the change (Wido,
  2026-09-14; goal 29's mutation proof in the return).
- Expected Return: the implementer schema's properties. Acceptance
  Criteria: the DONE sentence verbatim, then "and the named test proves it
  on the candidate tree". Gap Rule: the standard sentence.

If the record cannot fill the template the goal is not a lane goal:
`LANE_DONE_SENTENCE` or `LANE_TEST` at open, brief authority at dispatch.

Witness: `cmd/metasystem/lane_brief_test.go`, `TestLaneBriefRendersFromRecord`
(both templates render from a fixture record; the rendered bytes contain
exactly one Working Mode, the Boundary from `Paths:`, `Ceiling: 80`, the
`Test:` command and the DONE sentence; rendering the same template from the
same record twice gives identical bytes; a `Paths:` member that does not
exist makes `ReadBriefAdmission` (`brief.go:97`) refuse
`BRIEF_AUTHORITY_REFUSED`).

**The round's proof.** On the implementer's return the seat runs `validate
conformance --stage review --job <root>` (the lane verdict is written here)
and then `job prove-round --root . --job <root>`
(`cmd/metasystem/prove_round.go:62-99`): the engine proves the round's tree
in auto mode on the enrolled installation and records the attempt in the
round directory; the landing receipt reuses the passed groups by execution
identity (`proof.ReusedWhole`, `:104-112`). Neither is a delegate round.

**Read.** After the review stage admits the diff, one code-critic round:
`metasystem delegate --role code-critic --goal <g> --lane small --reviews
<root> --worktree --destructive-reach DESIGN-BEARING`, brief rendered by
`job lane-brief --role code-critic --reviews <root>` from
`templates/lane-review-brief.md`, which fills `review-brief.md`'s slots:
`Working Mode: review`; round budget one read; threat model "the computed
diff against the DONE sentence, nothing wider"; scope the diff's paths and
the `Test:` command. The critic answers three questions and nothing else:
conformance (inside the box, only what the DONE sentence needs, only the
`Paths:` members), acceptance (the DONE sentence holds on `reviewedTree`
with the named test as `ran` evidence, not the builder's word), defect (no
defect inside the diff, the code-critique skill's materiality question).
Design alternatives and pre-existing defects outside the diff close
`out-of-scope`. The reference from the work root to the critic is stamped
by the engine at the critic's dispatch and reconciled at close
(`StampClaimedReviewReference`, `internal/dispatch/review_reference.go:56`
onward; `dispatch.sh:3005`); the seat writes nothing.

Why DESIGN-BEARING for the critic root: closure requires the critic's
`configurationObligations` builder rows and its `reasoningEffort` to equal
the required critique rows, maximal/xhigh, and names DESIGN-BEARING as the
class whose builder rows are the critique's (`hazard.go:351-358`);
`EffectiveObligations` raises only the critique duty of a class
(`hazard.go:59-75`), so a MECHANICAL critic root can never close. Model and
effort recorded as R-28-m1 asks: claude, Opus 5 (R-108-m1c), xhigh, because
`hazard.go:359-362` also requires proof of maximal execution and a cheaper
critic cannot close at all; the lane saves scope and rounds, never critic
effort. At tier 1 this read is a lane duty, not a hazard duty
(`EffectiveObligations(MECHANICAL, 1)` requires none, `hazard.go:55`);
`require_goal_tier_ladder` (`dispatch.sh:798-813`) therefore admits the
code-critic role and `--reviews` when the goal reads `Lane: small`, the
tier-1 hazard refusal (`admission.go:232-234`) admits the DESIGN-BEARING
class for that role under the lane, and both keep refusing design-critic.

**Clean read closes.** Zero material findings. The sequence, each command
existing today except the second:

1. `metasystem job critique-register-advance --repo . --root-job <critic>
   --round-job <critic>` (`main.go:206`; `CritiqueRegisterAdvance`,
   `internal/dispatch/finding_register.go:70`): a fresh critic root starts
   with an empty register at round 0 (`build.go:613-614`), `close-check`
   refuses a critic chain whose folded round is not its terminal round
   (`close.go:40-59`), and `delegate close` runs only
   `critique-register-close` (`dispatch.sh:3018-3020`). The advance folds
   only material findings into the register (`finding_register.go:1122-1128`:
   a non-material finding is skipped, or withdraws an earlier material one),
   so a clean read leaves the register empty at round 1.
2. `metasystem job lane-dispositions --repo . --root-job <critic> --out
   artifacts/agents/<critic>/lane-dispositions.md` (new, unit U5a): reads
   the terminal round's `return.json`
   (`artifacts/agents/<critic>/rounds/<n>/return.json`, the canonical
   return, `docs/orchestration.md:197`); if any finding has `material:
   true` it writes nothing and exits 2 with `LANE_CRITIQUE` and the ids (the
   overflow signal); otherwise it writes the dispositions table the join
   requires (`| Finding id | Disposition | Reasoning and evidence |
   Amendment |`, `critiqueclosed.go:13`), one row `noted` per non-material
   finding with the evidence cell "non-material by the critic's own
   verdict; the lane has one read and no fold" and Amendment `none`, and a
   header with its separator and no rows when the array is empty. `noted`
   is the only disposition a non-material finding admits
   (`skills/code-critique/SKILL.md:58`), so the artifact is mechanical: the
   engine authors no judgement. Retained beside `rounds/`, outside the
   immutable round directory.
3. `metasystem validate critique-closed --findings
   artifacts/agents/<critic>/rounds/<n>/return.json --dispositions
   artifacts/agents/<critic>/lane-dispositions.md --repo . --root-job
   <critic>` (`validate_verbs.go:74-95`, both files required;
   `CritiqueClosed`, `critiqueclosed.go:28-52`: every finding id has a row,
   no material id is `noted`, no unknown id; with `--repo` the empty
   out-of-scope set is written to the register, `:57-80`).
4. `metasystem delegate close --job <critic>` (`dispatch.sh:2981`): the
   critic root closes under the ordinary critic duties (register closed,
   terminal round folded); the hazard duty exempts a critic-only chain
   (`hazard.go:243-250`), so no critic of the critic is ever required.
5. `metasystem delegate close --job <root>`: the work root closes under the
   lane close duty below, which reads the critic root closed in step 4.

**Material read overflows.** A material finding does not start a fold in
the lane: DONE (2) says one read, and a fresh closing critic would own a
new empty register and could not resolve the first root's finding
(`build.go:613`; the advance folds only its own root's rounds). Step 2
above exits `LANE_CRITIQUE`; the goal overflows with that code (section 4);
the ordinary ladder then folds on the work root (an implementer follow-up,
admitted after the overflow) and takes its closing read as a critic
follow-up on the same critic root, which inherits `reviews` (`build.go:874`)
and is advanced by the dispatcher before dispatch (`dispatch.sh:2657`).
Nothing new is built for that path.

**Close duty.** `CloseCheck` (`close.go:15`) gains one duty for a root whose
`role` is `implementer` and whose record carries `lane: "small"`, at any
tier: `laneAdmission.verdict` is `admitted` for the final work round, and
`independentCritiqueJobRef` names a closed, clean, fresh, distinct-session,
cross-family critic root that reviewed the final work round, validated by
`validateIndependentCritiqueReference` (`hazard.go:329`) with the
DESIGN-BEARING critique rows as `required`. The duty never applies to a
critic root: a critic root also carries `lane: "small"` (it is dispatched
`--lane small`) but has no `laneAdmission` (the verdict is the review
stage's, on the implementation root) and closing it under this duty would
demand a critic of the critic without end, the recursion the hazard duty
already refuses to enter for critic-only chains (`hazard.go:243-250`). A
work root whose goal reads `Lane: none` at a revision at or above the
root's `goalRevision` (overflowed) closes under the ordinary duties alone.

Witness: `internal/dispatch/decisions_test.go`,
`TestCloseCheckRequiresLaneReaderAndVerdict`, in this order: a critic root
carrying `lane: "small"` with an empty folded register closes with no
critique reference of its own; then the work root: no verdict refuses; a
verdict on an earlier round refuses; no critique reference refuses at tier
1 and at tier 2; the admitted verdict with the critic root closed in the
first step passes; an overflowed goal closes under the ordinary duties.

**Receipt and landing.** The implementer leaves its change uncommitted in
the job worktree (`build.go:1136-1140`, "leave every change in the worktree
and do not commit"); the review stage snapshotted it as `reviewedTree` and
`diff.patch` (`conformance.go:362-434`), and the landing binds the staged
candidate to exactly that output: `chainCertifiedOutput` reads
`artifacts/agents/<root>/rounds/<n>/review.json` and `diff.patch`
(`observe.go:593-660`) and `bindCertifiedChange` applies the patch to the
landing's HEAD tree and compares the certified paths' change digest with
the candidate's (`observe.go:866-900`); the receipt ledger rides as the one
carried path (`carriedReceiptLedger`, `observe.go:516-520`). No merge stage
is run: `--stage merge` needs the implementer branch's committed HEAD and
issues the mission wall's authorization (`conformance.go:451-520`); the
lane is not a mission and the landing never reads it. The sequence that
lands the reviewed tree with its receipt in one commit:

1. `metasystem landing materialize --root . --chain <root>` (new, unit
   U5c): reads the chain's certified output through the same seam as the
   observation, requires `chainClosed: true` and, for a lane root,
   `laneAdmission.verdict: admitted`; refuses when the index is not empty
   or any certified path is dirty in the working tree; computes `expected
   = Apply(HEAD tree, patch)` (`gittree.Workspace.Apply`,
   `internal/gittree/gittree.go:396`) and the certified paths
   (`ChangedPaths`, `:374`), then `PreflightMaterialize(HEAD tree,
   expected, paths)` and `MaterializePaths(expected, paths)` (`:616`,
   `:569`, the primitives the recertified merge uses,
   `internal/validate/recertification.go:915-918`); prints the paths, one
   per line, and `reviewedTree=<tree>`.
2. `metasystem goal lane-measure --id <g>` (new, unit U5c; section 9)
   prints the DONE (4) fields; the seat pastes them into the note.
3. `scripts/receipt.sh add --type implement --outcome shipped --goal <g>
   --built-by delegate --delegate codex:gpt-5.6-sol:<builder>
   --delegate claude:opus-5:<critic> --note "lane=small claim=<claim opid>
   measured-at=<stamp> seat-min=<n> rounds=<n> tree=<reviewedTree>
   revealed-by=<attempt|none>"` (`scripts/receipt.sh` is a shim to
   `metasystem receipt`, `receipt.sh:12`; `--note` at
   `cmd/metasystem/receipt_verbs.go:60`). The line is appended to
   `memory/receipts.log` in the landing checkout before anything is
   committed; it names the reviewed tree, never a commit.
4. `git add -- <paths from step 1> memory/receipts.log`; `tree=$(git
   write-tree)`.
5. `metasystem landing test-receipt --root . --tree $tree --mode standard
   --goal <g>` (`cmd/metasystem/landing_verbs.go:167` onward; `--mode
   auto|standard|deep`); the receipt reuses the round's passed groups by
   execution identity and runs the groups the ledger line's own surface
   adds (`goal-records`, `testing.json:25`: `section/static-contract-audits`,
   `section/return-schema-fixtures`).
6. `scripts/agents/land.sh -m <note> --staged-only --chain <root>
   --test-receipt artifacts/agents/landing/receipts/$tree.json --goal
   <g>`: `--staged-only` because pathspec mode requires an empty index
   (`land.sh:385-388`) and the receipt above needed the staged tree first;
   `land.sh` refuses a call with neither pathspecs nor `--staged-only`
   (`:176-179`), checks the staged RECEIPT line for the goal
   (`check_receipt_line`, `:490-513`, `landing receipt-line`,
   `internal/landing/receiptline.go:118-131`), observes the chain, commits,
   rebases, proves and pushes (`:1091-1093`, `:1163-1200`).

Section 5 says what the receipt proves. The landing observation
(`observeChain`, `observe.go:324`) gains: when the root's effective
obligations require a critique or the root carries `lane: "small"`, the
root must carry `independentCritiqueJobRef`, else `chain-not-critiqued`,
which replaces `chain-not-design-bearing` (`observe.go:394-396`, a
would-refuse that lands today because the code is not in
`scripts/agents/landing-promotion.json`) and is added to that file's
`refuseCodes` (`internal/landing/promotion.go:21`, `:42`). A lane root also
passes only with `laneAdmission.verdict: admitted` (`chain-lane-refused`,
promoted) and its provenance gains `lane=small`. Both tiers land here; the
goal binding is the root's (`observe.go:355-366`: the landing must name
the chain's own goal).

Witness: `scripts/agents/land-fixtures.sh` scenarios
`small-lane-worktree-to-landing` (end to end from an uncommitted worktree
change to one commit carrying the reviewed tree and the RECEIPT line),
`small-lane-clean-chain-lands`, `small-lane-uncritiqued-refused` (section
13).

"No conformance round" (DONE (2)) means no delegated conformance-review
round; the engine's review stage runs inside the seat's flow.

## 4. Overflow

Overflow is one recorded ledger transaction by the claim holder's pair or
a human, never a plain edit (Wido (4)): `goal overflow --id <g> --code
<LANE_*> --from <root job> --into <existing goal>`.

Effects, in one publish:

1. `Lane: none`; a new record line `- Overflow: at=<stamp> opid=<opid>
   code=<code> from=<root> into=<goal>`.
2. The approval is re-bound: `Approved` gets the digest of the record as
   it now reads, its `authority` set to `overflow=<opid>` and bound to this
   history event, the way a misclassification raise binds `raise=<opid>`
   to its edit event (`ValidateApprovalRecord`, `file.go:764-768`); the
   validator accepts `overflow=` alongside `raise=` and `overflow` beside
   `approve|resume|set-budget` as an approval-bearing verb (`:765`). The
   budget tuple is unchanged: the seat never widens its own box (a wider
   box is `set-budget`, over-norm with `--approved-ref`, or the one
   `extend-budget`, `main.go:521-522`).
3. The claim revision is rebound with `rebindClaimKeepEpisode`
   (`verbs.go:435`), exactly as `set-budget` does on a claimed goal
   (`verbs.go:1246`); a live chain survives that rebind today
   (registered-wait's member C landed 53c6d2ee7 after its set-budget under
   R-117-m1e item 6).
4. `--into <goal>` names a live goal; it gains one history line
   `overflow-unit from=<g> chain=<root> code=<code>` through the verb.
   From here the work is administered as that goal's unit: its design
   page and dispositions are that goal's, its RECEIPT line names both, and
   the lane goal's Conclude names the unit. The chain and the claim stay
   with the lane goal until it lands, because a chain lands only under the
   goal it was dispatched for (`observe.go:355-366`) and a fresh dispatch
   would spend the rounds the lane saved (seat ruling 2). For a
   seat-opened lane goal the natural `--into` is its `--blocks` target.
5. The chain continues on the ordinary ladder: the implementer follow-up
   (`LANE_ROUNDS` now admits it, section 2), the closing read as a critic
   follow-up, a receipt at the depth the surfaces say, `land.sh --chain
   --goal <g>` through the same materialize-then-receipt sequence.

Codes that overflow: `LANE_FILES`, `LANE_LINES`, `LANE_SHAPE`, `LANE_PATHS`,
`LANE_SCOPE`, `LANE_DEPTH`, `LANE_CRITIQUE`, and `LANE_TIER` when a risk edit
raises a claimed lane goal to tier 3.

Witness: `internal/goal/lane_test.go`,
`TestLaneOverflowRebindsAndKeepsTheBox` (`Lane: none`, the Overflow line,
the tuple unchanged, the claim revision rebound, `ValidateApprovalRecord`
passes on the re-derived digest and fails if the digest is left as it was;
the target goal's history line exists); `TestLaneOverflowRefusesForeignPair`
(another pair refused; a human admitted; an unknown or done `--into`
refused); `TestLaneEditCannotLeaveTheLaneAfterApproval` (`goal edit --lane
none` on an approved goal is refused and names `unapprove` or `overflow`);
`dispatch-fixtures.sh` scenario `lane-one-read` for the follow-up admitted
after the overflow.

## 5. What the receipt proves

The receipt is the shared testing contract's plan for the candidate,
requested in `standard` mode (DONE (2), literal). Depth follows the changed
surfaces, never the goal's answers and never the diff's size (R-3 as
amended 2026-09-11; `select.go:156-166` records the goal's answers as
cadence weight; `requiresDeep`, `risk.go:110-114`, is true for a protected
policy change, a surface raise to 2 or worse on any dimension, a
reversibility other than revert, delayed detection or unbounded recovery).
`LANE_DEPTH` asked the same contract in auto mode at review and refused
`deep`, so at landing the standard request equals the required mode by
construction and `Select` cannot raise its insufficiency error
(`select.go:212-215`). The plan always carries `always.canary` and
`always.standard` (`testing.json:104`: the three canaries;
`fast-static-build`, which is `go-gate.sh --fast` with the refusal-register
audit; the static and contract audits; `section/runtime-contract-audits`;
`test-environment-standard`); to that it adds the affected surfaces'
`standard` groups, their `critical` obligations' providers, and, when the
goal's accumulation is 2 or more, their `crossCutting` obligations
(`select.go:174-190`). So a lane receipt is the fast gate plus the owning
surfaces' standard groups, the executed mode equal to the requested mode.

The round's proof (`job prove-round`, section 3) ran the same plan in auto
mode on the round's tree; the landing receipt reuses every passed group
whose inputs are unchanged and runs the rest (the receipt-ledger line's own
groups). The tier is the floor (Wido (1)): the reader the tier requires at
tier 2 (`critique-always`, `hazard.go:55`) and the lane's reader at tier 1
are never lowered by the receipt's selection.

A fix found by a proof (Wido (1)): when a proof attempt of another goal's
landing revealed the defect, the lane goal's RECEIPT line and Conclude name
it, `revealed-by=<attempt id>`, and the lane's receipt is that fix's own
standard-mode run; the note never claims the fix was proven alone. Evidence
reuse is the engine's existing behaviour (`reusable-success`), nothing new
is built. Witness: the ten-landing review reads `revealed-by=` from
`memory/receipts.log` (section 9); this is a seat rule recorded in
`docs/orchestration.md` (unit U6), not engine law.

The lane never skips: the landing observation with the goal named; the
refusal register (every lane code is a row, audited by the fast gate); the
reader (no lane root closes or lands without the reference); the fast gate
on every receipt; the DONE sentence as the acceptance test in the brief,
the read and the landing note.

## 6. The budget

One box the norm reads for a lane goal at either tier:

    metasystem.budget.lane-small = 4h/4/480m/1/2

Elapsed 4h (R-44-m0b's small item; a cap, not the target of DONE (4)).
Attempts 4: the build, the read, and two for the overflow case's fold and
closing read; every terminal attempt counts (R-22-m1). Reserved minutes
480: four times `dispatch.cap-max` (120, `metasystem.conf`; R-58-m1, the
pool is a runaway guard). Active 1. Review rounds 2: the lane's one read and
the closing read after an overflow; inside R-42-m0's ceiling of three
(`metasystem.conf:15`). Why not the tier boxes: tier 1's is `1h/3/360m/1/0`
(`budget.go:248`, `metasystem.conf:16`), zero review rounds, so a tier-1
lane read would be over-norm; tier 2's `4h/6/720m/1/2` (`metasystem.conf:17`)
is wider than a lane needs.

Mechanics: `config.LaneBox(confPath)` beside `TierBox` (`budget.go:279`),
same grammar and refusals; a `boxFor(root, f)` helper returns the lane box
when `f.Lane == "small"` and the tier box otherwise, used wherever
`config.TierBox` is read for a goal today (`goalNormApproval`,
`internal/goal/norm.go:105-110`; `requireWithinGoalNorm`, `:151-155`;
`withinTierBox`, `verbs.go:1296-1301`; set-budget's box, `verbs.go:1092`,
`:1199`; the edit path, `:2662`); `openRequest` (`verbs.go:736`, `:746`)
assigns the lane box when opened `--lane small` with no tuple. `set-budget`
is never needed for a lane goal. Overflow keeps the tuple (section 4).

Witness: `internal/goal/approval_test.go`, `TestLaneBoxIsTheNorm` (a lane
goal at tier 1 opens with the lane box and its read is within norm;
`set-budget 4h/5/600m/1/2` is `GOAL_NORM_REFUSED` naming `lane-small`; a
`Lane: none` tier-1 goal is judged by `tier-1`).

## 7. The seat's part

In order, and nothing else:

1. `goal open --id <g> --lane small --blocks <claimed goal> --risk ...
   --basis ... --intent "... DONE means ..." --next "... || Paths: <p1>, <p2>
   || Test: <command>"`; `goal approve` in Wido's name (R-119-m1e).
2. `goal claim --id <g>`.
3. `metasystem delegate --role implementer --goal <g> --lane small
   --worktree --destructive-reach MECHANICAL --wait`.
4. `metasystem validate conformance --stage review --job <root>`; a lane
   refusal in the verdict: `goal overflow` and the ordinary ladder. Then
   `metasystem job prove-round --root . --job <root>`.
5. `metasystem delegate --role code-critic --goal <g> --lane small
   --reviews <root> --worktree --destructive-reach DESIGN-BEARING --wait`.
6. `metasystem job critique-register-advance --repo . --root-job <critic>
   --round-job <critic>`; `metasystem job lane-dispositions --repo .
   --root-job <critic> --out artifacts/agents/<critic>/lane-dispositions.md`
   (exit 2 `LANE_CRITIQUE`: `goal overflow --code LANE_CRITIQUE ...` and the
   ordinary ladder); `metasystem validate critique-closed --findings
   artifacts/agents/<critic>/rounds/1/return.json --dispositions
   artifacts/agents/<critic>/lane-dispositions.md --repo . --root-job
   <critic>`; `metasystem delegate close --job <critic>`; `metasystem
   delegate close --job <root>`.
7. `metasystem landing materialize --root . --chain <root>`; `metasystem
   goal lane-measure --id <g>`; `scripts/receipt.sh add --type implement
   --outcome shipped --goal <g> --built-by delegate --delegate
   codex:gpt-5.6-sol:<builder> --delegate claude:opus-5:<critic> --note
   "lane=small claim=<opid> measured-at=<stamp> seat-min=<n> rounds=<n>
   tree=<reviewedTree> revealed-by=<attempt|none>"`; `git add -- <paths>
   memory/receipts.log`; `tree=$(git write-tree)`; `metasystem landing
   test-receipt --root . --tree $tree --mode standard --goal <g>`;
   `scripts/agents/land.sh -m <note> --staged-only --chain <root>
   --test-receipt artifacts/agents/landing/receipts/$tree.json --goal <g>`;
   when the verdict carried `engineBinding`, re-arm:
   `scripts/agents/go-build.sh`, `metasystem up --repo <checkout>` (the
   re-arm rule after every engine landing, `docs/orchestration.md:321`).
8. `goal done --id <g> --conclude "<landing note>"`: the DONE sentence, the
   chain and critic ids, the receipt id and its group count, the landing
   commit (here and only here), the re-arm line when engine-binding, and
   the four DONE (4) facts copied from the RECEIPT line.

The seat writes no brief, no dispositions register in `plans/`, no
critique record in `records/misc/`. Twenty commands (two more when the
landing is engine-binding) and one paragraph; a seat that wants fewer
keystrokes wraps steps 6 and 7 in its own shell function, which changes
nothing the engine judges.

## 8. Tier 1: which path, and why

Outside the lane a tier-1 goal is R-54-m1's direct fix: a receipted build
with no reader, at most three files and forty lines, refused on the floor
rows (`internal/landing/tierone.go:94-96`, `:117`, `:120`;
`scripts/agents/path-classes.txt:70-85`: `internal/goal/`,
`internal/dispatch/`, `internal/landing/`, `internal/validate/`,
`internal/config/`, `dispatch.sh`, `commit.sh`, `land.sh`, the schemas, the
class manifests, `metasystem.conf`), landed with `land.sh --direct-fix tier-1
--root-job` (`land.sh:9`, `:208-212`). Unchanged by this build.

Inside the lane a tier-1 goal has the lane's reader, the box of section 2,
no floor rows, and the chain landing. The rules decide the path:

- A tier-1 change whose `Paths:` touch a floor row cannot take the direct
  fix at all (`tier1-floor-refused`); it takes the lane. That is most
  engine one-liners, and it is why the lane exists.
- A tier-1 change off the floor rows within three files and forty lines
  (a message text, a fixture leg, a doc line) takes the direct fix: no
  reader, one attempt fewer. The seat marks the lane only when a floor
  row or the wider box is needed.
- A tier-1 change over three files or forty lines, off the floor rows,
  takes the lane if within four and eighty, else the ordinary ladder at
  its tier.

No rulings row is needed: R-54-m1 stands as the outside-the-lane law, and
the lane's reader is DONE (2)'s and Wido (3)'s, inside it.

## 9. The DONE (4) measurement

A rollout row, not a fixture. What is measured, by whom, from what:

`metasystem goal lane-measure --id <g> [--root .]` (new, unit U5c;
`internal/goal/lanemeasure.go`) reads three recorded facts and one clock:
the goal's claim stamp (`ClaimRecord.At`, `file.go:280-283`); every job
record bound to the goal (`goalId`) with its `startedAt` and `endedAt`
(`build.go:633-634`; a record with `endedAt: null` refuses the measure,
"a delegate is still running"); and the verb's own `Now` seam
(`LaneMeasureOptions{Now func() time.Time}`, the shape `receipt.Options.Now`
has today, `internal/receipt/receipt.go:64`, `:81-85`), which the test
injects and production leaves nil. It prints one line:

    claim=<opid> claim-at=<stamp> measured-at=<now> seat-min=<n> delegate-min=<n> rounds=<n>

`seat-min` is the whole minutes from the claim stamp to `measured-at`
minus the minutes in which a delegate of this goal was running (the union
of the job records' `startedAt`..`endedAt` spans, overlaps merged);
`delegate-min` is that subtracted union; `rounds` is the count of terminal
job records bound to the goal (`job chain-members --terminal-only`,
`dispatch.sh:3013`, over every root bound to `<g>`). Both thresholds are
DONE (4)'s: 45 and 3.

What DONE (4) then shows, said plainly: `seat-min` is seat-ATTENDED time,
the minutes the goal waited on the seat rather than on a delegate. It
counts a seat that is idle or on another goal as attending, so it bounds
active seat time from above: a specimen under 45 by this measure is under
45 active seat minutes by any tighter measure, and a specimen over 45 is
re-run rather than argued down. No per-call activity ledger is built for
this goal (the seat's tool-call cursors exist, `internal/usage/calls.go`,
but joining them to a goal is a measurement design of its own and would
outweigh the lane). This is the honest narrowing; the ten-landing review
may tighten it.

Where recorded: in the same-commit RECEIPT line (R-117-m1e item 5), the
`note=` field of `memory/receipts.log`, written by `scripts/receipt.sh add`
in step 7 of section 7 BEFORE `land.sh`, with `claim=`, `measured-at=`,
`seat-min=`, `rounds=` and `tree=<reviewedTree>` as the candidate's
identity. The line carries no commit hash: a commit's hash covers the
receipt bytes, so the line cannot name the commit that contains it; the
landing commit goes into the goal's Conclude after the landing (section 7,
step 8). The test receipt (`internal/landing/receipt.go:39-56`) is not the
place: it binds trees, attempts and proof, has no note field, and is an
engine artifact the lane does not change.

Where read: the ten-landing review (`grep 'lane=small' memory/receipts.log`),
which recalibrates the bound, and the delivery-efficiency plan's weekly
measure (delegate rounds per landing, human acts per landing; plan section
5). The goal is not concluded before this row is recorded with both
thresholds met; a specimen over either threshold is recorded as such and a
second specimen is run.

The specimen: the first real lane goal is the specimen. Revision 2 named
`records/narrator-digest.log`, a record path the landing's class rules
would refuse; no candidate is named here. The first one-line fix a seat
needs after unit U5c lands becomes the specimen when it passes this list:
tier 1 or 2 after `DerivedTier`; `Paths:` of at most four members, none
under `internal/refusal/`, none protected, none a behavior-surface reader;
`LANE_DEPTH` standard under the tip's `testing.json` (so not under
`internal/proofrun/coverage.go`, `internal/usage/`, `internal/output/`,
`internal/runtimes/` or an unowned path); a `Test:` command that fails
without the change. Trunk-red one-liners of the kind landed three times on
2026-09-16 are the expected shape.

Witness: `internal/goal/lanemeasure_test.go`,
`TestLaneMeasureSubtractsDelegateSpans` (a fixture goal claimed at T; two
job records spanning T+2m..T+20m and T+25m..T+37m; `Now` injected at T+50m;
`seat-min=20`, `delegate-min=30`, `rounds=2`; overlapping spans T+2m..T+20m
and T+10m..T+30m count once; a record without `endedAt` refuses);
`TestReceiptLineCarriesNoCommitHash` is not a test but a rule of the note
grammar: `landed=` is refused by `lane-measure`'s printed grammar and by
the seat's step 8 placing it in the Conclude.

## 10. The two-runtime proof

The lane lives in the engine (`internal/goal`, `internal/dispatch`,
`internal/landing`, `internal/gittree`, `internal/behaviorsurface`,
`internal/config`), the ledger verbs (`goal open|edit|overflow|lane-measure`)
and the dispatcher (`dispatch.sh`, the templates), above the adapter
contract; no adapter (`scripts/agents/adapters/{claude,codex,devin,fake}.sh`,
`internal/adapter/`) learns the word `lane` (at `7e99bb110` neither
`dispatch.sh` nor any adapter contains the token). Every lane rule fires
before an adapter is chosen or after it has returned: `LANE_*` at open,
edit, dispatch admission and review; the close duty; the landing. What a
runtime can differ on is what it always could: delivering the rendered
brief and returning the role's `return.json`.

Proof on two runtimes, four legs:

1. Fake runtime, end to end: the dispatch and land bed scenarios of section
   13 run the whole lane under `adapters/fake.sh`.
2. Paired same-role evidence on the real runtimes: two real lane goals,
   both after unit U5c: specimen A builds on codex (gpt-5.6-sol) and is
   read on claude (Opus 5), the R-108-m1c lanes; specimen B builds on
   claude and is read on codex. For each role the two runtimes are
   compared on the engine-owned records, normalized by dropping job ids,
   stamps, session ids, runtime and model fields: the root record's `lane`,
   `laneAdmission.verdict` and `code`, `destructiveReach`,
   `configurationObligations`, `reasoningEffort`, `chainClosed`, the
   presence of `independentCritiqueJobRef` (implementer) or of the folded
   register at the terminal round (critic); the refusal codes each dispatch
   met on the way, which must be none on the clean path and exactly
   `LANE_BRIEF` on one deliberate `--brief` dispatch per runtime (the
   refusal fires before any adapter runs, which is the point); the
   critic's `verdictMaterialCount`; and the landing observation's verdict
   line. The four normalized records (implementer on codex, implementer on
   claude, critic on claude, critic on codex) are recorded side by side in
   `records/misc/small-change-lane-two-runtime-proof.md` with the job ids
   beside them; agreement per role is the proof. Specimen B needs a
   one-time exception to R-108-m1c's role lanes (question Q1); if Wido
   refuses it, leg 2 records specimen A alone and the DONE's runtime
   independence rests on legs 1 and 3, which the page then says.
3. Hash equality, scoped exactly: `job lane-brief` is a pure function of
   the template, the record at the claimed revision, the role and the
   `--reviews` target. Equality of rendered bytes is claimed only for the
   same template and the same inputs: the implementer brief of specimen A
   rendered twice (once at dispatch, once by the seat afterwards) hashes
   identically, and so does the critic brief; the implementer brief and
   the critic brief of one goal are different templates and are never
   compared. `TestLaneBriefRendersFromRecord` pins the determinism; the
   two-runtime record carries the four hashes.
4. Static witness: `section/runtime-contract-audits` (in `always.standard`,
   `testing.json:104`) gains one check: no file under
   `scripts/agents/adapters/` or `internal/adapter/` contains the token
   `lane`. It fails the day an adapter carries lane logic. ASSUMPTION: the
   section's script is found through `validate-section-selector.sh
   catalog`; the exact file is named by the builder of unit U6.

## 11. Moved effects

One owner moves. Paths in this table carry the `metasystem/` prefix so that
`metasystem validate moved-effects --file <this page> --root <repository
root>` resolves them (`cmd/metasystem/validate_verbs.go:418-472`).

| Effect | From | To | Code |
| --- | --- | --- | --- |
| the changed-line count and diff-shape refusal of a landing candidate: additions plus deletions over the named paths, with rename, copy, binary and mode-only changes refused | `internal/landing` (`tierOneDiffMetric`) | `internal/gittree` (`Workspace.DiffShape`) | `metasystem/internal/landing/tierone.go:168-215`, `metasystem/internal/gittree/numstat.go:11` |

`gittree.Workspace.DiffShape(fromTree, toTree string, paths []string)
(DiffShape, error)` with `DiffShape{Lines int, Renamed, Binary, ModeOnly
bool}` lives in `internal/gittree/numstat.go` beside `ChangedLines`, which
already parses `--numstat -z` (`numstat.go:11-24`). `landing.observeTierOne`
(`tierone.go:113-125`) and `dispatch.LaneAdmission` (`internal/dispatch/
lane.go`) both call it; `tierOneDiffMetric` is deleted, not wrapped, so one
count exists. Dependency direction: `landing` imports `gittree` today
(`tierone.go`, `observe.go`), `dispatch` imports `gittree` today
(`internal/dispatch/budget_extension.go`, `read_subject_compute.go`), and
`gittree` imports neither `landing` nor `dispatch`; no new edge and no
cycle. The tier-1 landing's behaviour is unchanged: the same refusal codes
(`tier1-diff-shape-refused`, `tier1-file-bound-refused`,
`tier1-line-bound-refused`) on the same inputs, proven by the existing
tier-one leg of the land bed and `internal/landing/observe_test.go`.

## 12. Units

Each unit is one chain on the ordinary ladder: Codex Sol builds in a
worktree with a brief that declares its Boundary and Ceiling (goal 28) and
proves every rule by mutation in its return (goal 29); Opus reads; the
critique is by another model; each lands under R-117-m1e item 5 (Opus read
with no NOT LAND, green selected groups, receipt in the same commit).
Allocation is additions plus deletions, tests included, at most 300.
"After" is the observable state.

| Unit | Files (tests) | Alloc | DONE | Witness | After |
| --- | --- | --- | --- | --- | --- |
| U0 R-90-m1's builder row | `internal/dispatch/hazard.go:37-41`, `scripts/agents/role-packets.json:4-11` (`hazard_closure_test.go`, `composition_test.go`) | 60 | MECHANICAL reads maximal/xhigh, critique false, live proof false, in both surfaces | the table-equality test fails on either surface alone changed; a MECHANICAL dispatch at medium is refused (`build.go:486-488`) | every MECHANICAL build runs at xhigh; the lane can dispatch |
| U1a the record | `internal/goal/file.go`, `lane.go` (new), `verbs.go`, `cmd/metasystem/goalsync_mutations.go`, `internal/refusal/register.go` (`internal/goal/lane_test.go`, `register_test.go`) | 260 | `Lane` parses, renders, migrates; `--lane` on open and edit; `LANE_TIER`, `LANE_DONE_SENTENCE`, `LANE_TEST` with `LaneFields`; the digest hashes the lane; rows registered | `TestLaneOpenAdmission`, `TestLaneChangesTheApprovalDigest`, `TestLaneEditAfterApprovalNamesTheVerbs`, the register audit | a goal can be opened in the lane and approved with the lane in its digest |
| U1b the box | `internal/config/budget.go`, `internal/goal/norm.go`, `verbs.go` (`boxFor`), `metasystem.conf` (`approval_test.go`, `internal/config/budget_test.go`) | 140 | `metasystem.budget.lane-small` read wherever the tier box is read | `TestLaneBoxIsTheNorm` | a tier-1 lane goal's read is within norm |
| U2 overflow | `internal/goal/verbs.go`, `file.go` (`Overflow` line, `overflow=` authority, the verb in the approval-bearing set), `cmd/metasystem/main.go` and `goalsync_mutations.go`, `register.go` (`lane_test.go`) | 280 | `goal overflow` as section 4 | `TestLaneOverflowRebindsAndKeepsTheBox`, `TestLaneOverflowRefusesForeignPair`, `TestLaneEditCannotLeaveTheLaneAfterApproval` | a claimed lane goal leaves the lane in one recorded transaction |
| U3a the rendered briefs | `cmd/metasystem/lanebrief.go` (new), `main.go`, `scripts/agents/templates/lane-brief.md`, `lane-review-brief.md` (`lane_brief_test.go`) | 280 | `job lane-brief` renders both templates from the record, deterministically | `TestLaneBriefRendersFromRecord` | a brief exists that no one wrote |
| U3b dispatch | `scripts/agents/dispatch.sh` (`--lane`, `LANE_HAZARD`, `LANE_ROLE`, `LANE_ROUNDS` keyed to the goal's state, `LANE_BRIEF`, the tier ladder under the lane, the root field), `internal/dispatch/admission.go:15`, `:232-234` (the tier-1 hazard refusal under the lane), `stop.go` (`lane` in the binding), `build.go`, `register.go` (`dispatch-fixtures.sh` scenarios `lane-dispatch-admission`, `lane-one-read`; `internal/proofrun/test_cost.go:25-27`) | 300 | section 2's dispatch rules | the scenarios: wrong class refused with no husk; design-critic refused; `--brief` with `--lane` refused; code-critic DESIGN-BEARING admitted at tier 1 under the lane; `rounds/1/prompt.md` carries the DONE sentence, the `Test:` command and one Working Mode; the root carries `lane`; implementer follow-up refused before overflow and admitted after | a lane chain dispatches on the rendered brief |
| U4a the scope owner | `internal/behaviorsurface/readers.go` (new) (`readers_test.go`) | 80 | `ProjectionReaders()` and `ProjectionReader(path)` enumerate the policy and every reader of it | `TestProjectionReadersEnumerateEveryConsumer` | the clause's (c) has one tested owner |
| U4b review admission | `internal/dispatch/lane.go` (new), `internal/validate/conformance.go` (the review-stage hook), `internal/gittree/numstat.go` (`DiffShape`), `internal/landing/tierone.go` (the call site), `register.go` (`lane_test.go`, `numstat_test.go`) | 300 | `LaneAdmission` with `LANE_FILES`, `LANE_LINES`, `LANE_SHAPE`, `LANE_PATHS`, `LANE_SCOPE`, `LANE_DEPTH`; the verdict on the root; the moved count | `TestLaneAdmissionBounds`, `Paths`, `Scope`, `Depth`; `TestDiffShape` (the tier-one cases moved with the code) | the box, the clause and the depth are enforced on the computed diff |
| U5a close | `internal/dispatch/close.go`, `cmd/metasystem/lanedispositions.go` (new), `main.go` (`decisions_test.go`, `lanedispositions_test.go`) | 200 | the lane close duty on the work root only; `job lane-dispositions` | `TestCloseCheckRequiresLaneReaderAndVerdict`; `TestLaneDispositionsRendersNotedRowsOrRefuses` (zero findings: header and separator only, the join passes; two non-material: two `noted` rows, the join passes; one material: exit 2 `LANE_CRITIQUE`, nothing written) | no lane chain closes without its reader and verdict, and a clean read closes with existing commands |
| U5b landing | `internal/landing/observe.go`, `promotion.go`, `scripts/agents/landing-promotion.json` (`land-fixtures.sh` scenarios `small-lane-clean-chain-lands`, `small-lane-uncritiqued-refused`; `observe_test.go`) | 240 | `chain-not-critiqued`, `chain-lane-refused`, `lane=small` provenance | the two scenarios | a lane chain lands with a pass verdict at either tier; an uncritiqued one is refused |
| U5c materialize and measure | `internal/landing/materialize.go` (new), `cmd/metasystem/landing_verbs.go`, `internal/goal/lanemeasure.go` (new), `main.go` (`materialize_test.go`, `lanemeasure_test.go`; `land-fixtures.sh` scenario `small-lane-worktree-to-landing`) | 260 | `landing materialize` carries the certified diff into the checkout; `goal lane-measure` prints the DONE (4) fields from recorded stamps and an injected clock | `TestMaterializeWritesExactlyTheCertifiedPaths` (dirty path refused; index not empty refused; the paths and `reviewedTree` printed); `TestLaneMeasureSubtractsDelegateSpans`; the end-to-end scenario | the reviewed tree and its receipt land in one commit; the measurement exists before the landing |
| U6 docs and the audit line | `docs/orchestration.md` (the working-modes table, the seat sequence of section 7, the tier-1 paths of section 8, the `revealed-by=` rule), `AGENTS.md` intake paragraph, `skills/code-critique/SKILL.md` (the lane's one-read note), `records/misc/severity-tiered-rigor-obligation-matrix.md`, the runtime-contract audit script (section 10) | 140 | the rules seats follow are written where seats read | `scripts/audit-metasystem.sh` (the 1400-word bundle cap), the audit line of section 10 | the lane is documented and its adapter boundary audited |

Total allocation 2540 lines over twelve units. Order: U0, U1a, U1b, U2,
U3a, U3b, U4a, U4b, U5a, U5b, U5c, U6; U4a may go beside U3a. The lane is
usable after U5c; the DONE (4) row and the two-runtime record follow. U4b
takes a deep receipt only if its diff touches a protected path; at this
tip none of its files is protected (`internal/gittree/**` is
`proof-and-landing`, `testing.json:7`, no raise), so its receipt is what
its surfaces say.

## 13. Fixtures and how they run

Go tests: `cd metasystem && go test ./internal/<pkg> -run '^<Test>$' -count=1
-timeout=2m` (`internal/goal` needs `-timeout 40m` under race and load).

Process fixtures are scenarios of their beds and run the way the beds run
today, as children `--fixture-bed-child <scenario> <capability>` with a
capability the parent mints (`scripts/agents/fixture-budget.sh:26-65`;
`fixture-bed-scenarios.sh:132-136`). The two beds and their exact public
group ids (`testing.json`):

- Dispatcher bed, group `section/dispatcher-adapter-and-mission-runner-fixtures`
  (`testing.json:99`). The parent accepts only `--comparison`
  (`dispatch-fixtures.sh:9-12`, `:40-43`) and takes its scenario list from
  the Go-owned catalog through `proof-run fixture-selection --family
  dispatcher --selection all` (`:46-47`; `fixtureScenarioCatalog`,
  `internal/proofrun/test_cost.go:25-27`; `FixtureScenarios`, `:33-46`).
  Catalog edit: `lane-dispatch-admission` and `lane-one-read` are appended
  to `fixtureScenarioCatalog["dispatcher"]`, and to nothing else (the
  `comparison` selection stays the two cost scenarios, `:20-23`). Selection
  check before any run: `bin/metasystem proof-run fixture-selection
  --family dispatcher --selection all | grep -c -x -e lane-dispatch-admission
  -e lane-one-read` prints `2`.
- Land bed, group `section/land-fixtures` (`testing.json:72`). The parent's
  scenario list is the literal array at `land-fixtures.sh:33-38`, with its
  leg count in the message (`"land fixtures passed (28 isolated legs)"`).
  Catalog edit: `small-lane-clean-chain-lands`,
  `small-lane-uncritiqued-refused` and `small-lane-worktree-to-landing` are
  appended to that array and the count becomes 31. Selection check: `grep
  -c -e small-lane-clean-chain-lands -e small-lane-uncritiqued-refused -e
  small-lane-worktree-to-landing scripts/agents/land-fixtures.sh` counts
  each name at least twice (the parent list and the scenario body).

The verification run is the engine's, from the enrolled checkout re-armed
at the tip: `bin/metasystem test run --root . --goal <claimed goal> --tree
<tree> --mode canary --purpose diagnostic --groups
section/dispatcher-adapter-and-mission-runner-fixtures,section/land-fixtures
--cap-min <min>` (`--groups` is the comma-separated diagnostic list,
`cmd/metasystem/test.go:155-160`); `land-fixtures.sh` also runs standalone
from a worktree with a built `bin/metasystem`.

| Scenario | Bed | Proves, and where it fails today |
| --- | --- | --- |
| `lane-dispatch-admission` | dispatcher | as unit U3b's witness: wrong class refused with no husk; design-critic refused; `--brief` with `--lane` refused; code-critic DESIGN-BEARING admitted at tier 1 under the lane (`admission.go:232-234` would refuse it today); the rendered prompt; the root's `lane` field. Fails today: `--lane` is an unknown option. |
| `lane-one-read` | dispatcher | a clean read: advance, `lane-dispositions` writes the table, `critique-closed` joins, both roots close in order (critic first); a material read: `lane-dispositions` exits 2 `LANE_CRITIQUE`, `goal overflow --code LANE_CRITIQUE` records, the goal reads `Lane: none`; on the same frozen lane root an implementer follow-up is refused `LANE_ROUNDS` before the overflow and admitted after it; a critic follow-up is admitted. Fails today: no lane. |
| `small-lane-clean-chain-lands` | land | a fake-runtime lane chain at tier 2 and one at tier 1, each with an admitted verdict, a clean closed critic and a schema-2 standard receipt, lands `pass bar=a` with `lane=small`. Fails today: `would-refuse code=chain-not-design-bearing` (`observe.go:394-396`), tier 1 has no reader path. |
| `small-lane-uncritiqued-refused` | land | the same chain without a critique reference is `chain-not-critiqued` and refused (promoted); with a `LANE_FILES` verdict, `chain-lane-refused`. Fails today: lands with a trailer. |
| `small-lane-worktree-to-landing` | land | end to end: a job worktree (`git worktree add`, the conformance bed's shape, `scripts/agents/conformance-fixtures.sh:115-136`) with an uncommitted one-line change and a job record naming `workspaceRoot` and `baseSha`; `validate conformance --stage review --job <root>` writes `review.json`, `diff.patch` and the admitted lane verdict; a closed critic root is fabricated and referenced the way the recertified-chain leg does it (`land-fixtures.sh:1882-1891`); `landing materialize` carries the diff into the landing checkout; `receipt.sh add` appends the RECEIPT line; `git add`, `write-tree`, `landing test-receipt --mode standard`, `land.sh --staged-only --chain --test-receipt --goal`; asserts one new commit whose project tree has the certified paths' entries equal to `reviewedTree`'s and whose diff adds the RECEIPT line naming the goal. Fails today: `landing materialize` is an unknown verb; the chain is `chain-not-design-bearing`. |

## Open questions (real gaps only)

1. Where `section/runtime-contract-audits` finds its script
   (`validate-section-selector.sh catalog`); the builder of U6 names the
   file (section 10, leg 4).

## Revision record

Revision 1 (2026-09-10, `scl-design1`): the lane as a record field bound
into the digest; three admission moments with registered codes; four files
and eighty lines; record paths refused; one build on a rendered brief; one
critique with one fold and one closing read; the box `4h/4/480m/1/2`; tier
1 folded into the lane with its direct-fix class retired; tier 3 admitted.

Revision 2 (2026-09-16 09:50): folded Wido's 09-10 decisions and his four
answers of 09-16 and every material and minor finding of the Codex read, as
section 0 says: tier 3 out; the rule-scope clause and `LANE_DEPTH`; one
read and no in-lane fold; the critic root DESIGN-BEARING; the design gap
read off the verdict; `goal overflow` with a re-derived digest and the
unit-of-an-existing-goal record; the changed surfaces decide depth; `Test:`
and `Paths:` in the record and the rendered brief; R-90-m1's row as unit
U0; the direct-fix class kept outside the lane; one lane box for both
tiers; the DONE (4) row and the two-runtime proof; fixtures on the beds'
real protocol.

Revision 3 (2026-09-16, this page, the final fold): folds the twelve
material findings of Codex Sol's round 2 and the five reopened round-1
findings, as the table below says. Changed from revision 2: eligibility is
exactly the tier, the box and the clause (`LANE_WIDTH`, `LANE_SURFACES`,
`LANE_UNOWNED_PATH`, `LANE_RECORD_PATH` and `DirectOwners` removed); the
box gains `LANE_PATHS` because the review stage's boundary is the returns'
own claim; the clause's (c) is one tested owner enumerating every reader of
the behavior-surface policy; `LANE_DEPTH` derives the required mode from
the contract's auto selection and names the two surfaces that raise risk
today; the tier-1 hazard refusal at `admission.go:232-234` is admitted for
the lane's critic; the close duty applies to the work root only and the
critic root closes first; `LANE_ROUNDS` is keyed to the goal's current
state and admits the follow-up after overflow; `job lane-dispositions`
gives the zero-material join its artifact; the landing sequence is
`landing materialize`, the RECEIPT line, the staged tree's standard receipt,
`land.sh --staged-only`, with no merge stage and no commit hash in the
receipt; `goal lane-measure` records seat-attended minutes from recorded
stamps and an injected clock before landing; the first real lane goal is
the specimen; the two-runtime proof pairs the same role across codex and
claude and scopes hash equality; the moved-effects inventory names
`gittree.Workspace.DiffShape`; the exact group ids and catalog edits; units
U4a (scope owner) and U5c (materialize and measure) added, `DirectOwners`
dropped.

Next: the seat checks the table below against the critique's reopening
triggers and lands this page; then units U0 to U6 built by Codex Sol in
worktrees, each read on Opus, landed under R-117-m1e item 5.

## Seat rulings (m1e, 2026-09-16 10:02 local)

1. CONFIRMED: "an engine projection" in Wido (2) means the projection's
   declaration and readers (section 2, `LANE_SCOPE` (c)), not every
   ENGINE-projection member; the other reading closes the lane to all engine
   code, which Wido's answer did not ask for. Operational note that is not a
   scope rule: a lane landing on any ENGINE member still invalidates the
   fleet's arming like any other such landing, so the lane's landing step
   names the re-arm (chain-common item 10) and its announcement.
2. CONFIRMED: "the work becomes a unit of an existing goal" is met by the
   `--into` record of section 4 while the chain and claim stay with the lane
   goal until it lands. What the rule protects is that overflowed work leaves
   the lane's lighter regime; section 4 does that (`Lane: none`, the ordinary
   ladder of step 5, the budget tuple unchanged), and re-homing the chain
   would only spend a fresh dispatch.

## Round-2 findings (Codex Sol, critique round 2 of 3) and how each fold meets its reopening trigger

Every finding was checked against the code at `origin/main` `7e99bb110`
before folding; none was factually wrong, so none is rejected. "Where"
names the section and the line of this page where the fold begins.

| Finding | Disposition (folded/rejected) | Where in r3 (section and line) | How the reopening trigger is met |
| --- | --- | --- | --- |
| SCL-R2-M01 the goal's `Paths:` boundary is not enforced | folded (verified: `conformance.go:736-831` checks changed paths against the returns' `diffBoundary` only; `brief.go:151-201` validates the header, never the diff) | section 2, `LANE_PATHS`, line L259; section 0 row N4 | trigger "a computed-diff witness refuses a path absent from the goal revision's canonical `Paths:` list": `LANE_PATHS` compares the review stage's changed paths with the goal revision's `Paths:` set; witness `TestLaneAdmissionPaths` (a return that truthfully declares the extra path, so the cumulative boundary passes, is refused `LANE_PATHS`); unit U4b |
| SCL-R2-M02 `LANE_SCOPE` omits four current ENGINE-projection readers | folded (verified: `freeze.go` and `registers.go` do not import the policy; the readers are the nine Go files and four scripts listed) | section 2, `LANE_SCOPE` (c), line L273 | trigger "every current production ENGINE-policy reader is enumerated by one tested scope owner": `behaviorsurface.ProjectionReaders()` is the one owner (unit U4a); `TestProjectionReadersEnumerateEveryConsumer` walks the module for import and verb readers and fails on any difference either way; the page's list is the current value, not the owner |
| SCL-R2-M03 the proof-depth test cannot observe deep selection as designed | folded (verified: `select.go:212-215` errors only for a standard request against a deep requirement; `testing.json:14`, `:16` declare `risk: {severity: 2}` on `coverage-policy` and `context-budget`; the field is `risk`, `contract.go:51-59`) | section 2, `LANE_DEPTH`, line L299; section 5 line L645 | trigger "an existing non-protected risk-raising surface deterministically yields `LANE_DEPTH` without relying on an empty error plan": the auto-mode plan's `RequiredMode` is the verdict; witness `TestLaneAdmissionDepth` on `internal/usage/retention.go` (owned by `context-budget`, not protected, not a behavior-surface reader; seat correction 1) with a non-empty plan and `requiredMode: deep`; unit U4b |
| SCL-R2-M04 two extra eligibility gates contradict the binding lane contract | folded (verified: Wido's answers and his reading of the whole name the tier, the box and the clause plus the reader, nothing about accumulation or owner counts; `file.go:138-143` makes accumulation the gate width) | section 1 decision 2, line L85; section 2 "Eligibility, the three predicates", line L163 | trigger "admission implements only the decided tier, box, and rule-scope predicates": `LANE_WIDTH`, `LANE_SURFACES`, `LANE_UNOWNED_PATH` and `LANE_RECORD_PATH` are removed; direct owners still derive the plan through `Select`; the depth rule overflows when the actual requirement is deeper than standard; the removed gates' effects are covered by `LANE_DEPTH` (the residual raise) and the landing's class refusals; no seat question, because Wido's recorded answers decide it |
| SCL-R2-M05 the new close duty makes the required critic root recursively unclosable | folded (verified: `hazard.go:243-250` exempts critic-only chains for exactly this reason) | section 3, "Close duty", line L491 | trigger "the critic root closes without another critic and its closed reference then closes the work root": the duty applies only to a root with `role: implementer`; the witness `TestCloseCheckRequiresLaneReaderAndVerdict` closes the critic root first and then the work root on that reference; unit U5a |
| SCL-R2-M06 overflow preserves the frozen lane marker while refusing the implementer fold it promises | folded (verified: `build.go:828-888` follow-ups inherit the parent's frozen facts) | section 2, `LANE_ROUNDS`, line L220; section 4 step 5 | trigger "the same frozen lane root refuses an implementer follow-up before overflow and admits one after validated overflow": `LANE_ROUNDS` reads the goal's current record through `job goal-binding`, refuses under `Lane: small`, admits under `Lane: none` with the `Overflow:` line; witness scenario `lane-one-read` does both on one root; unit U3b |
| SCL-R2-M07 the clean-read closure command has no required dispositions artifact | folded (verified: `validate_verbs.go:74-95` requires both files; `critiqueclosed.go:157-205` refuses a missing file or header; the advance folds material findings only, `finding_register.go:1122-1128`) | section 3, "Clean read closes", line L437 | trigger "the exact zero-findings join inputs and artifact owner are named and accepted by the existing command": findings `artifacts/agents/<critic>/rounds/<n>/return.json`; dispositions `artifacts/agents/<critic>/lane-dispositions.md` written by the engine's `job lane-dispositions` (owner: `cmd/metasystem/lanedispositions.go`, unit U5a), header and separator with one `noted` row per non-material finding, refusing on any material finding; the exact `validate critique-closed` invocation is given; witness `TestLaneDispositionsRendersNotedRowsOrRefuses` and scenario `lane-one-read` |
| SCL-R2-M08 the prescribed seat sequence cannot land the reviewed candidate or its receipt | folded (verified: `build.go:1136-1140` leaves the change uncommitted; `land.sh:176-179` refuses no pathspecs and no `--staged-only`; `:385-388` pathspec mode wants an empty index; `check_receipt_line` runs before commit, `:490-513`, `:1091-1093`; `observe.go:593-660`, `:866-900` bind the candidate to the review artifacts; the merge stage is the mission wall's, `conformance.go:451-520`) | section 3, "Receipt and landing", line L514; section 7 step 7, line L745 | trigger "one end-to-end witness carries the exact reviewed candidate and pre-created receipt into one successful landing commit": `landing materialize` (unit U5c) transfers the certified diff with the engine's own `Apply`, `PreflightMaterialize`, `MaterializePaths`; the RECEIPT line is appended before `land.sh`; the staged tree's standard receipt; `land.sh --staged-only --chain`; witness scenario `small-lane-worktree-to-landing` starts from an uncommitted job-worktree change and asserts one commit with the reviewed tree's entries and the RECEIPT line |
| MOVED-EFFECTS-SCL-R2-M09 a named count owner moves with no moved-effects inventory | folded (verified: `tierone.go:168-215` owns the count today; `gittree/numstat.go:11` already parses numstat; import directions checked) | section 11, line L926 | trigger "the moved-effects inventory names one metric owner and the validator passes": the section is headed `Moved effects` with the `| Effect | From | To | Code |` table, one row, destination `internal/gittree` (`Workspace.DiffShape`), both callers named, the paths in the Code cell carry the `metasystem/` prefix and exist at the tip so `validate moved-effects --root <repository root>` resolves them; the seat runs the validator on landing |
| SCL-R2-M10 the stated fixture verification groups do not exist | folded (verified: `testing.json:99` and `:72` carry the two ids; `test_cost.go:25-27` and `land-fixtures.sh:33-38` are the catalogs) | section 13, line L984 | trigger "exact public group IDs and scenario-catalog edits make every named witness selectable": `section/dispatcher-adapter-and-mission-runner-fixtures` and `section/land-fixtures`; the exact catalog edits (`fixtureScenarioCatalog["dispatcher"]` gains two names; the land parent array gains three and its count becomes 31); the selection checks (`proof-run fixture-selection ... | grep -c` prints 2; the land grep); the `test run --groups` command with both ids |
| SCL-R2-M11 the two-runtime proof exercises different operations and claims unequal briefs are identical | folded on the page; the run of specimen B needs question Q1 (verified: R-108-m1c fixes the implementation lane to codex and the read to claude) | section 10, line L871 | trigger "Codex and Claude each exercise equivalent lane operations and their normalized engine outcomes agree": leg 2 pairs the implementer on codex and on claude and the critic on claude and on codex, names the normalized fields and the refusal codes compared, and the record that carries them; leg 3 claims hash equality only for the same template and inputs; if Q1 is refused the page says leg 2 rests on specimen A and legs 1 and 3 |
| SCL-R2-M12 DONE (4) records wall elapsed time and asks a receipt to contain its own commit hash | folded (verified: `receipt.Options.Now` exists, `receipt.go:64`; job records carry `startedAt`/`endedAt`, `build.go:633-634`; `ClaimRecord.At`, `file.go:280-283`; `records/narrator-digest.log` is a record path) | section 9, line L797; section 7 steps 7 and 8 | trigger "active seat time is recorded from an injected clock before landing and the same-commit receipt contains no self-referential commit hash": `goal lane-measure` (unit U5c) computes seat-attended minutes from the claim stamp, the job spans and its injected `Now`, before `receipt.sh add`; the note grammar carries `tree=<reviewedTree>` and no `landed=`; the commit hash goes to the Conclude; the narrowing (seat-attended, an upper bound on active seat time) is stated; the specimen is the first real lane goal with an eligibility list, no ineligible file named |
| SCL-R1-M05 (reopened; mapped to R2-M05, M07) | folded | as R2-M05 and R2-M07 | the clean path is executable: advance, `lane-dispositions`, join, critic root closed, work root closed |
| SCL-R1-M08 (reopened; mapped to R2-M03) | folded | as R2-M03 | the standard receipt is sufficient by construction; the depth check reads the auto plan |
| SCL-R1-M12 (reopened; mapped to R2-M10) | folded | as R2-M10 | the exact ids and catalogs |
| SCL-R1-M13 (reopened; mapped to R2-M12) | folded | as R2-M12 | the measurement exists before landing, from recorded stamps and an injected clock |
| SCL-R1-M14 (reopened; mapped to R2-M11) | folded (specimen B pending Q1) | as R2-M11 | paired same-role evidence; scoped hash claim |
| Open-question recommendation 1 (Ceiling) | folded | section 0 row N4; section 2 `LANE_PATHS`, `LANE_LINES` | `LANE_LINES` is the authoritative eighty-line check; `LANE_PATHS` replaces the withdrawn brief-bound claim; the Ceiling question is closed, not open |
| Open-question recommendation 2 (fixture ownership) | folded | section 13 | the two catalogs and the two group ids exactly as recommended |

## Questions for the seat

Q1 (Wido's, through the seat; R-108-m1c is his word): may the two-runtime
proof's specimen B run its build on claude and its read on codex, once, for
that one real lane goal, so that each role is exercised on both runtimes
(section 10, leg 2)? Recommendation: yes, for specimen B only, recorded in
the two-runtime proof record with this page as the reason; without it the
paired same-role evidence the DONE's runtime independence asks for cannot
exist on real runtimes, and the page then rests leg 2 on specimen A alone.
Nothing on this page waits for the answer: units U0 to U6 build either way.

## Seat corrections (m1e, 2026-09-16)

The seat's fold check found SCL-R2-M03 only partly folded. This was the
final round under R-117-m1e item 2, so the seat closed it by correcting the
page itself, with no further design round:

1. The depth witness (section 2, `TestLaneAdmissionDepth`) used
   `internal/proofrun/coverage.go`. Section 2's `LANE_SCOPE` (c) also lists
   that file as a behavior-surface reader, so it could not yield
   `LANE_DEPTH` deterministically. The witness is now
   `internal/usage/retention.go`. At origin/main `9cafaae5f`: it is owned
   by `context-budget` (`testing.json:16`, `risk: {severity: 2}`); it is
   not in the reader list here, and no file under `internal/usage/`
   mentions `behaviorsurface`; it matches no pattern of
   `ProtectedPolicyChange`. The residual fallback (`testing.json:27`,
   `risk: {severity: 2}`) covers only paths no surface owns, which the
   witness's second case keeps; `internal/usage/**` is not residual.
2. Admission's single refusal `code` now has a stated order: the first
   failing check in the order this page lists them (tier, box, clause,
   depth). Witnesses are chosen so their named check fails first, and the
   depth witness fails that check alone. A witness that fails only one
   check is not possible in every case: `five-files` also exceeds a
   four-member `Paths:`, and a protected scope path is also deep.
3. The `finding_register.go` cite (section 3 "Clean read closes" and the
   SCL-R2-M07 row) moves from `:1073-1089` to `:1122-1128`, where the
   non-material skip or withdraw now sits at origin/main `9cafaae5f`.

Edits 1 and 2 add nine lines in section 2, so the round-2 findings table's
line numbers after L246 moved down by five or nine lines to match. No
other text changed.

With these, all twelve round-2 findings count as folded. SCL-R2-M11 still
depends on Q1, which is open with Wido; the build does not wait on it.
