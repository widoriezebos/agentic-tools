# The small-change lane (goal small-change-lane)

Revision: 2. Date: 2026-09-16. Author: a Claude Fable design delegate for
seat m1e (Wido, 2026-09-13: design is a Fable delegate's; the seat drafts
of 2026-09-13 are withdrawn and were mined for code facts only). Failsafe
round: 2 (R-97-m1e). Every code fact below was re-read at `origin/main`
`0f657fd36` (fetched 2026-09-16 about 09:50 local); a fact this revision
could not re-verify is marked ASSUMPTION. Paths are relative to
`metasystem/`.

Inputs: revision 1 (`scl-design1`, Fable, 2026-09-10, read at `b3d795fe`),
the Codex Sol read of the seat's revision 2 (14 material, 3 minor findings),
Wido's three decisions of 2026-09-10 and his four answers of 2026-09-16
(verbatim on the goal record's Next step), rulings R-3 (as amended
2026-09-11), R-54-m1, R-60-m1, R-90-m1, R-93-m1e, R-114-m1e to R-119-m1e,
and goal 19 of `plans/delivery-efficiency-plan.md`.

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

## 0. Dispositions

One row per material finding of the Codex read, one per minor finding,
one per 2026-09-10 decision, and the rows this revision adds (NEW-MISSED).

| Finding | Disposition | Where |
| --- | --- | --- |
| M01 tier 3 admitted contrary to DONE (1) | FOLDED; confirmed by Wido (2) "tier 3 never enters the lane". `LANE_TIER` at open and edit. `DerivedTier` is `max(severity, novelty)` (`internal/goal/file.go:134-136`); exposure no longer forces tier 3. | section 2 |
| M02 no protected-surface refusal | FOLDED and SUPERSEDED-BY-WIDO (2): the rule-scope clause is wider than the predicate. `LANE_SCOPE` refuses the refusal register, a protected policy path (`internal/testpolicy/protection.go:37-49`) and the engine projection's own declaration, whatever the size. | section 2 |
| M03 tier-1 branch routed through the chain landing, wrong box, no `--blocks` | SUPERSEDED-BY-WIDO (2) and (3): one box for tiers 1 and 2; inside the lane every change has a non-author reader, so a tier-1 lane chain lands through the chain observation with its reader, and the tier-1 direct-fix class (`landing-classes.json` row `tier-1`, 3/40, `observeTierOne`) stays untouched OUTSIDE the lane. One lane budget box for both tiers (the tier-1 box has zero review rounds, `internal/config/budget.go:248`). `--blocks` on a seat's open (R-93-m1e, `verbs.go:729-731`). | sections 2, 6, 8 |
| M04 a MECHANICAL critic root is refused at close | FOLDED: the code-critic root is dispatched DESIGN-BEARING; closure compares the critic's builder rows with the required critique rows and says so (`internal/dispatch/hazard.go:351-358`). The work root stays MECHANICAL. | section 3 |
| M05 the in-lane fold cannot close | FOLDED and SUPERSEDED-BY-WIDO (3), DONE (2) literal: one read, no in-lane fold. Clean: advance the register for the terminal round, then close. Material: overflow. | sections 3, 4 |
| M06 the register cannot write `LANE_DESIGN_GAP` | FOLDED: no register side effect. A material finding of any kind on the one read is the lane's design gap, read off the critic's verdict (`validate critique-closed`), and overflows the goal with code `LANE_CRITIQUE`. | section 4 |
| M07 overflow by plain goal edit breaks the approval digest | SUPERSEDED-BY-WIDO (4) and FOLDED: `goal overflow`, a real verb, the seat's recorded transaction, re-derives the approval digest and rebinds the claim revision the way `set-budget` does (`verbs.go:1236-1248`); the work becomes a unit of an existing goal. | section 4 |
| M08 stale proof-depth law; `--mode auto` is not "standard mode" | SUPERSEDED-BY-WIDO (1) and FOLDED: the changed surfaces decide (`internal/testpolicy/select.go:156-173`, `risk.go:106-115`, R-3 as amended); the lane requests `standard` literally and refuses a change whose surfaces require deep (`LANE_DEPTH`); a fix found by a proof names the attempt that revealed it and never claims a solo proof. | section 5 |
| M09 brief without Working Mode or a replayable test | FOLDED: `Working Mode: implement` and `Working Mode: review` (`scripts/agents/templates/brief.md:1`; `dispatch.sh:1545` requires the header); `LANE_TEST` requires one `Test:` line in the Next step; the rendered brief carries it, and `Boundary`, `Ceiling` and `Changed-line allocation` (`internal/dispatch/brief.go:151-201`, `brief.md:5`). | sections 2, 3 |
| M10 unowned paths invisible to selection | FOLDED: `testpolicy.DirectOwners(paths)`, the prefix match of `select.go:121-135` before the fallback, refuses on an empty result (`LANE_UNOWNED_PATH`). | section 2 |
| M11 R-90-m1's builder row is not a prerequisite | FOLDED: unit U0 flips the MECHANICAL builder row to maximal/xhigh in `hazard.go:37-41` and `scripts/agents/role-packets.json:4-11`; `build.go:486-488` refuses a differing builder effort, so nothing in the lane dispatches before U0. | section 11 |
| M12 fixture commands that do not run | FOLDED: every process fixture is a scenario of its bed, driven the way the beds run today (parent runs the engine-selected set; child protocol `--fixture-bed-child <scenario> <capability>`, `scripts/agents/fixture-budget.sh:26-65`, `fixture-bed-scenarios.sh:132-136`); the diagnostic run is the engine's `test run --purpose diagnostic --groups section/<bed>`. No parent `--scenario` switch is built. | section 12 |
| M13 no DONE (4) measurement | FOLDED: section 9 names the specimen, the clock, the round count, the receipt fields and the reader. | section 9 |
| M14 no two-runtime proof | FOLDED: section 10. | section 10 |
| m01 stale deep-group count | FOLDED: stated as the rule, not a count (section 5). | section 5 |
| m02 no failsafe round declared | FOLDED: header. | header |
| m03 "no conformance round" imprecise | FOLDED: no delegated conformance-review round; `validate conformance --stage review` is the engine's computation of the round's diff (`internal/validate/conformance.go:362-434`) inside the seat's flow. | section 3 |
| 09-10 decision (1) proof depth by the four answers | SUPERSEDED by Wido (1) of 2026-09-16. | section 5 |
| 09-10 decision (2) critique follows the tier; tier-1 direct fix not retired; R-54-m1 stands | KEPT, reconciled by Wido (3): outside the lane, tier 1 is the direct fix with no critique; inside the lane every change has a non-author reader. | section 8 |
| 09-10 decision (3) four files, eighty lines, computed diff, overflow not failure, ten-landing review | KEPT, extended by Wido (2): one box for both tiers plus the rule-scope clause; the number is a recalibratable bound. | section 2 |
| NEW-MISSED N1: a fresh critic root's register starts at round 0 (`build.go:613-614`) and close-check refuses until the folded round equals the terminal round (`close.go:40-59`); `delegate close` runs `critique-register-close`, not the advance (`dispatch.sh:3018-3021`) | FOLDED: the seat runs `job critique-register-advance` (`cmd/metasystem/main.go:206`) before `validate critique-closed`; no new machinery. | section 3 |
| NEW-MISSED N2: `require_goal_tier_ladder` refuses every critic role and `--reviews` at tier 1 (`dispatch.sh:798-813`) | FOLDED: under `Lane: small` the code-critic role and `--reviews` are admitted at tier 1; design-critic stays refused. | section 3 |
| NEW-MISSED N3: revision 1 cited a tier-1 hazard refusal at `admission.go:211-214`; at `0f657fd36` lines 196-216 carry no such rule | ASSUMPTION: the build confirms no tier-keyed hazard refusal blocks a DESIGN-BEARING critic root under a tier-1 lane goal before unit U3b is briefed. | section 3 |
| NEW-MISSED N4: the brief now carries `Boundary` and `Ceiling` headers and a `Changed-line allocation` (goal 28, landed 2026-09-16), and the review stage refuses paths outside the boundary (`conformance.go:399`, `:733`) | FOLDED: the rendered lane brief fills them from the record; the lane's path bound rides that existing refusal, the line bound is the lane's own check. | sections 2, 3 |
| NEW-MISSED N5: `delegate close` takes `--job <root>` (`dispatch.sh:2981`), not `--root` | FOLDED in the seat's commands. | section 7 |
| NEW-MISSED N6: "engine projection" in Wido (2) has no parenthesis | DECIDED with a seat line: the projection's own declaration and digest readers, not every member of the ENGINE projection (which is `cmd/**`, `internal/**`, `scripts/agents/**`, `internal/behaviorsurface/policy.v2.json:3-10`, and would close the lane to every engine one-liner). | section 2, closing lines |

## 1. Decisions

1. The lane is a record field, `- Lane: small`, on a goal of tier 1 or 2,
   bound into the approval digest. Tier 3 is refused (`LANE_TIER`).
2. One box for both tiers: at most four changed files and eighty changed
   lines (additions plus deletions, no rename, copy, binary or mode-only
   change), measured on the computed diff at the review stage; plus the
   rule-scope clause: a diff touching the refusal register
   (`internal/refusal/register.go`), a protected policy path
   (`testpolicy.ProtectedPolicyChange`) or the engine projection's
   declaration and readers is outside the box whatever its size. The
   numbers are a bound the ten-landing review recalibrates from the DONE
   (4) measurements, not law.
3. One build round on a brief the engine renders from the record; one
   non-author read on the code-critic lane (claude, Opus 5, R-108-m1c) at
   xhigh, dispatched DESIGN-BEARING; no fold inside the lane. Clean closes;
   material overflows. "No register" means no new refusal-register row is
   authored in-lane (Wido (3), read with (2)); a change that needs one
   leaves the lane. The critic root's finding register is the engine's
   record of the read, not an entry authored by the change.
4. Overflow after claim is `goal overflow`, the seat's recorded
   transaction: `Lane: none`, the approval digest re-derived, the claim
   revision rebound, the tuple unchanged, and the goal recorded as a unit
   of the named existing goal. Never a plain `goal edit`.
5. Proof depth: the changed surfaces decide; the lane requests a
   standard-mode receipt literally and refuses a change whose surfaces
   require deep (`LANE_DEPTH`); the tier's duties are the floor under it;
   a fix found by a proof names the attempt that revealed it and never
   claims a solo proof.
6. Outside the lane, tier 1 keeps its direct-fix class (R-54-m1). Inside
   the lane, a tier-1 change has the lane's reader and lands through the
   chain observation. Section 8 says which path a tier-1 change takes.
7. The lane's own build takes the ordinary ladder (Fable design, Codex Sol
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
tier and the tuple, and `ValidateApprovalRecord`, `file.go:800-813`,
recomputes it), so the lawful ways out after approval are unapprove, edit,
approve (`cmd/metasystem/main.go:520`) or, after claim, `goal overflow`
(section 4). An edit that would leave the lane on an approved or claimed
goal is refused and names the verb.

Every lane refusal is a registered row (`internal/refusal/register.go`,
`Row{Code, Owner, Site, Shape: Agent, Override}`, `register.go:13-21`), so
the fast gate's register audit sees it; the rows are authored by the lane's
own build on the ordinary ladder, never in-lane (decision 3).

**At open and edit** (`OpenRisked`, `verbs.go:696`; `Edit`, `verbs.go:2515`;
flags in `cmd/metasystem/goalsync_mutations.go`):

- `LANE_TIER`: the effective tier (`f.Tier`, which `editRequest` keeps at
  or above `DerivedTier`, `verbs.go:2572-2590`) must be 1 or 2. A risk
  edit that raises a lane goal to tier 3 overflows it (section 4) when
  claimed and is refused before claim.
- `LANE_WIDTH`: accumulation must be 1, so the width is `area`
  (`file.go:138-143`).
- `LANE_DONE_SENTENCE`: the Intent contains the literal `DONE means` with
  at least one sentence after it. That clause is the brief's only
  acceptance criterion and the critic's threat model.
- `LANE_TEST`: the Next step carries one line `Test: <command>` (one shell
  command from the repository root, no pipe into another tool) and one
  line `Paths: <path>[, <path>...]` naming every path the change may touch,
  tests included, each existing at the claimed revision or marked `new`.
  The brief copies both verbatim; the critic reruns the test; the landing
  note quotes it.
- A seat's open names `--blocks <its claimed goal>` (R-93-m1e,
  `verbs.go:729-731`); the blocked goal parks and returns on `done`
  (`returnBlockerParks`, `verbs.go:897`). A human's open needs no blocker.

Witness: `internal/goal/lane_test.go`, `TestLaneOpenAdmission` with
subtests `tier-three`, `accumulation-two`, `no-done-sentence`, `no-test-line`,
`no-paths-line`, `admits`; `TestLaneChangesTheApprovalDigest` (the digest
differs between `Lane: small` and `Lane: none`, and an approval made at
`small` fails validation after a hand edit to `none`);
`TestLaneEditAfterApprovalNamesTheVerbs`.

**At dispatch** (`dispatch.sh` after the goal binding at `:1637-1638`; the
engine side beside `EvaluateGoalRevisionAdmissionForDispatch`,
`internal/dispatch/admission.go:189`):

- `LANE_HAZARD`: under a lane goal an implementer root is dispatched
  `--destructive-reach MECHANICAL` and a code-critic root `DESIGN-BEARING`;
  any other pairing is refused with "the change is not mechanical; leave
  the lane with goal overflow". Added to `AdmissionRefusalCodes`
  (`admission.go:15`) so it is citable as misclassification evidence.
- `LANE_ROLE`: only implementer and code-critic dispatch under a lane goal.
- `LANE_ROUNDS`: an implementer follow-up on a lane root is refused; there
  is nothing to follow up inside the lane (section 3).
- `LANE_BRIEF`: `--lane` and `--brief` together are refused; the brief is
  rendered (section 3).
- The root record gains `lane: "small"`, frozen beside `goalTier`
  (`build.go:551`) and inherited by follow-ups like `gateWidth`
  (`build.go:833-834`), so an overflowed chain's later rounds still say
  where they came from.

Witness: `dispatch-fixtures.sh` scenario `lane-dispatch-admission`
(section 12).

**At review** (`validate conformance --stage review --job <root>`, which
computes `reviewedTree`, `diff.patch` and the changed paths,
`conformance.go:362-399`; new `LaneAdmission` in
`internal/dispatch/lane.go`, called by the review stage when the root
carries `lane: "small"`, its verdict written on the root as
`laneAdmission {round, verdict, code, files, lines, owners, requiredMode,
engineBinding}` and re-evaluated on every later review, the latest winning):

- `LANE_FILES`: more than four changed paths. `LANE_LINES`: more than
  eighty changed lines, additions plus deletions, by `tierOneDiffMetric`
  (`internal/landing/tierone.go:168`) lifted into a shared helper.
  `LANE_SHAPE`: a rename, copy, binary or mode-only change. The path bound
  is also the brief's `Boundary` (section 3), which the review stage
  already refuses when crossed (`conformance.go:399`, `:733`); the line
  bound is the lane's own check, whatever the `Ceiling` header is enforced
  as (ASSUMPTION: whether the review stage enforces the ceiling was not
  established; the lane does not depend on it).
- `LANE_SCOPE`, the rule-scope clause (Wido (2)): any changed path that is
  (a) `internal/refusal/register.go` or any file of `internal/refusal/`;
  (b) a protected policy path, `testpolicy.ProtectedPolicyChange`
  (`protection.go:37-49`: `testing.json`, `internal/testpolicy/**`,
  `internal/proofrun/test_*`, the coverage ratchets,
  `validate-section-selector.sh`, `commit.sh`, `land.sh`); or (c) the
  engine projection's declaration and readers:
  `internal/behaviorsurface/**` (the policy and `policy.v2.json`, which
  define ENGINE, LANDING and PAYLOAD, `policy.go:25-31`, `:71`),
  `internal/proofrun/manifest.go` (`engineProjectionMember`, `:158-169`),
  `internal/proofrun/freeze.go` and `internal/landing/registers.go`
  (`TestReceiptProjection`, `ProjectWorkspaceTree`, `:85-91`). These three
  are the surfaces that judge every landing; the two chains Wido cites
  (5fed9c978, `register.go`; 87e33cbf2, `internal/proofrun/test_build.go`,
  protected) shipped through them on line counts alone. The clause is not
  "every member of the ENGINE projection": that set is `cmd/**`,
  `internal/**`, `scripts/agents/**` (`policy.v2.json:3-10`) and would
  exclude nearly every one-line fix the lane exists for. The seat line at
  the end asks Wido to confirm (c).
- `LANE_RECORD_PATH`: any changed path of class `record` or `ledger` under
  `scripts/agents/path-classes.txt:30-39` (`memory/`, `plans/`, `records/`;
  `plans/goals/` and friends). The goal record changes only through verbs.
- `LANE_UNOWNED_PATH`: `testpolicy.DirectOwners(paths)` (new: the prefix
  match of `select.go:121-131` returned before the `Fallback` branch at
  `:132-135`) reports a path with no direct owner; the residual fallback
  (`testing.json:4`, `:27`) does not count. `LANE_SURFACES`: more than two
  direct owners over the whole diff. Consumers are not counted.
- `LANE_DEPTH`: the contract's plan for the changed paths, requested in
  `standard` mode for delivery (`testpolicy.Select`, `select.go:95`,
  `:167-173`, `:216`), reports `requiredMode: deep`. Today that is exactly a
  protected path or an unowned path (no surface declares a raise:
  `testing.json` has no `riskRaise`; `risk.go:106-115`), so the check
  coincides with `LANE_SCOPE` (b) and `LANE_UNOWNED_PATH`; it stays a
  separate rule so DONE (2)'s "standard mode" holds by construction when a
  surface declares a raise later.
- The floor rows of `path-classes.txt:70-85` do not apply: they protect
  the tier-1 direct fix, which has no reader; the lane has one.
- Engine binding is recorded, not refused: a changed path under
  `internal/`, `cmd/` or `scripts/agents/` sets `engineBinding: true`
  (the skew preflight's "engine or agent scripts", `dispatch.sh:284`), and
  the landing note carries the re-arm line (section 7).

Witness: `internal/dispatch/lane_test.go`: `TestLaneAdmissionBounds`
(`five-files`, `eighty-one-lines`, `rename`, `binary`, `mode-only` refuse;
`four-files-eighty-lines` admits); `TestLaneAdmissionScope` (one path each
under `internal/refusal/`, `internal/testpolicy/`,
`internal/behaviorsurface/` refuses `LANE_SCOPE`; `internal/dispatch/x.go`
admits with `engineBinding: true`); `TestLaneAdmissionRecordPaths`
(`plans/x.md`, `memory/rulings.md` refuse; `docs/x.md` admits);
`TestLaneAdmissionOwners` (a path no surface pattern matches refuses
`LANE_UNOWNED_PATH`; three direct owners refuse `LANE_SURFACES`; one owner
with four consumers admits); `TestLaneAdmissionDepth` (an in-memory
contract whose surface declares a raise to severity 2 refuses
`LANE_DEPTH`; the same paths under the tip's contract admit).
`internal/testpolicy/direct_owners_test.go`, `TestDirectOwners`.

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
brief goes today, so it takes the same header check (`brief_mode`,
`dispatch.sh:858`, `:1545`: one filled Working Mode; Boundary and Ceiling
together), brief authority (`:862`: every cited path exists in the
delegate's tree), the testing requirement (`build.go:1136-1140`), packet
composition and record hashing.

Template `scripts/agents/templates/lane-brief.md`, filled only from the
record at the claimed revision:

- `Working Mode: implement`; Orchestrator Identity and Date as today;
  `Boundary: <JSON array of the Paths: line>`; `Ceiling: 80`;
  `Changed-line allocation: 80` (`brief.md:5`; `brief.go:151-201` parses
  the pair).
- Goal: the Intent verbatim. Workspace: the job worktree; only the
  `Paths:` members; never a record or ledger path.
- Inputs: the Next step verbatim; the `Test:` line as the named focused
  test the return must show as `ran` (`implementer.schema.json`
  `evidence[].level`).
- Constraints: the box in words (four files, eighty lines, no rename or
  binary, one or two owning surfaces, no refusal-register row, no
  protected path); one retained test that fails without the change (Wido,
  2026-09-14; goal 29's mutation proof in the return).
- Expected Return: the implementer schema's properties. Acceptance
  Criteria: the DONE sentence verbatim, then "and the named test proves it
  on the candidate tree". Gap Rule: the standard sentence.

If the record cannot fill the template the goal is not a lane goal:
`LANE_DONE_SENTENCE` or `LANE_TEST` at open, brief authority at dispatch.

Witness: `cmd/metasystem/lane_brief_test.go`, `TestLaneBriefRendersFromRecord`
(both templates render from a fixture record; the rendered bytes contain
exactly one Working Mode, the Boundary from `Paths:`, `Ceiling: 80`, the
`Test:` line and the DONE sentence; a `Paths:` member that does not exist
makes `ReadBriefAdmission` (`brief.go:97`) refuse `BRIEF_AUTHORITY_REFUSED`).

**Read.** After the review stage admits the diff, one code-critic round:
`metasystem delegate --role code-critic --goal <g> --lane small --reviews
<root> --worktree --destructive-reach DESIGN-BEARING`, brief rendered by
`job lane-brief --role code-critic --reviews <root>` from
`templates/lane-review-brief.md`, which fills `review-brief.md`'s slots:
`Working Mode: review`; round budget one read; threat model "the computed
diff against the DONE sentence, nothing wider"; scope the diff's paths and
the `Test:` line. The critic answers three questions and nothing else:
conformance (inside the box, only what the DONE sentence needs, every path
owned), acceptance (the DONE sentence holds on `reviewedTree` with the
named test as `ran` evidence, not the builder's word), defect (no defect
inside the diff, the code-critique skill's materiality question). Design
alternatives and pre-existing defects outside the diff close
`out-of-scope`.

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
code-critic role and `--reviews` when the goal reads `Lane: small`, and
keeps refusing design-critic.

**Clean read closes.** Zero material findings after `validate
critique-closed` (`main.go:120`). Before it the seat advances the critic's
register for its terminal round, `metasystem job critique-register-advance
--repo <root> --root-job <critic> --round-job <critic>` (`main.go:206`;
`CritiqueRegisterAdvance`, `finding_register.go:70`), because a fresh critic
root starts with an empty register at round 0 (`build.go:613-614`),
`close-check` refuses a critic chain whose folded round is not its terminal
round (`close.go:40-59`), and `delegate close` runs only
`critique-register-close` (`dispatch.sh:3018-3021`). Then `metasystem
delegate close --job <critic>` and `--job <root>` (`dispatch.sh:2981`).

**Material read overflows.** A material finding does not start a fold in
the lane: DONE (2) says one read, and a fresh closing critic would own a
new empty register and could not resolve the first root's finding
(`build.go:613`; the advance folds only its own root's rounds). The goal
overflows with `LANE_CRITIQUE` (section 4); the ordinary ladder then folds
on the work root and takes its closing read as a critic follow-up on the
same critic root, which inherits `reviews` (`build.go:874`) and is advanced
by the dispatcher before dispatch (`dispatch.sh:2657`). Nothing new is
built for that path.

**Close duty.** `CloseCheck` (`close.go:15`) gains one duty for a root with
`lane: "small"` at any tier: `laneAdmission.verdict` is `admitted` for the
final work round, and `independentCritiqueJobRef` names a closed, clean,
fresh, distinct-session, cross-family critic root that reviewed the final
work round, validated by `validateIndependentCritiqueReference`
(`hazard.go:329`) with the DESIGN-BEARING critique rows as `required`. A
root whose goal reads `Lane: none` at a revision at or above the root's
`goalRevision` (overflowed) closes under the ordinary duties alone.

Witness: `internal/dispatch/decisions_test.go`,
`TestCloseCheckRequiresLaneReaderAndVerdict` (no verdict refuses; a verdict
on an earlier round refuses; no critique reference refuses at tier 1 and
at tier 2; an admitted verdict with a clean closed critic passes; an
overflowed goal closes under the ordinary duties).

**Receipt and landing.** `metasystem landing test-receipt --root <root>
--tree <tree> --mode standard`, then `scripts/agents/land.sh -m <note>
--chain <root> --test-receipt <receipt> --goal <g>` (`land.sh:9`, `:93`,
`:124`, `:186`). Section 5 says what the receipt proves. The landing
observation (`observeChain`, `observe.go:324`) gains: when the root's
effective obligations require a critique or the root carries
`lane: "small"`, the root must carry `independentCritiqueJobRef`, else
`chain-not-critiqued`, which replaces `chain-not-design-bearing`
(`observe.go:395-396`, a would-refuse that lands today because the code is
not in `scripts/agents/landing-promotion.json`) and is added to that file's
`refuseCodes` (`internal/landing/promotion.go:21`, `:42`). A lane root also
passes only with `laneAdmission.verdict: admitted` (`chain-lane-refused`,
promoted) and its provenance gains `lane=small`. Both tiers land here; the
goal binding is the root's (`observe.go:355-365`: the landing must name
the chain's own goal).

Witness: `land-fixtures.sh` scenario `small-lane-clean-chain-lands`
(section 12).

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
   validator accepts `overflow=` alongside `raise=`. The budget tuple is
   unchanged: the seat never widens its own box (a wider box is
   `set-budget`, over-norm with `--approved-ref`, or the one
   `extend-budget`, `main.go:522`).
3. The claim revision is rebound with `rebindClaimKeepEpisode`
   (`verbs.go:435-478`), exactly as `set-budget` does on a claimed goal
   (`verbs.go:1240-1246`); a live chain survives that rebind today
   (registered-wait's member C landed 53c6d2ee7 after its set-budget under
   R-117-m1e item 6).
4. `--into <goal>` names a live goal; it gains one history line
   `overflow-unit from=<g> chain=<root> code=<code>` through the verb.
   From here the work is administered as that goal's unit: its design
   page and dispositions are that goal's, its RECEIPT line names both, and
   the lane goal's Conclude names the unit. The chain and the claim stay
   with the lane goal until it lands, because a chain lands only under the
   goal it was dispatched for (`observe.go:355-365`) and a fresh dispatch
   would spend the rounds the lane saved. For a seat-opened lane goal the
   natural `--into` is its `--blocks` target.
5. The chain continues on the ordinary ladder: fold, closing read, deep
   receipt where the surfaces say so, `land.sh --chain --goal <g>`.

Codes that overflow: `LANE_FILES`, `LANE_LINES`, `LANE_SHAPE`, `LANE_SCOPE`,
`LANE_RECORD_PATH`, `LANE_UNOWNED_PATH`, `LANE_SURFACES`, `LANE_DEPTH`,
`LANE_CRITIQUE`, and `LANE_TIER` when a risk edit raises a claimed lane goal
to tier 3.

Witness: `internal/goal/lane_test.go`,
`TestLaneOverflowRebindsAndKeepsTheBox` (`Lane: none`, the Overflow line,
the tuple unchanged, the claim revision rebound, `ValidateApprovalRecord`
passes on the re-derived digest and fails if the digest is left as it was;
the target goal's history line exists); `TestLaneOverflowRefusesForeignPair`
(another pair refused; a human admitted; an unknown or done `--into`
refused); `TestLaneEditCannotLeaveTheLaneAfterApproval` (`goal edit --lane
none` on an approved goal is refused and names `unapprove` or `overflow`).

## 5. What the receipt proves

The receipt is the shared testing contract's plan for the candidate,
requested in `standard` mode (DONE (2), literal). Depth follows the changed
surfaces, never the goal's answers and never the diff's size (R-3 as
amended 2026-09-11; `select.go:156-166` records the goal's answers as
cadence weight; `requiresDeep`, `risk.go:106-115`, is true for a protected
policy change, an unowned path, a surface raise or an obligation). With
accumulation 1 the plan never adds cross-cutting groups (`select.go:182`)
and always carries `always.canary` and `always.standard` (`testing.json:104`:
the three canaries; `fast-static-build`, which is `go-gate.sh --fast` with
the refusal-register audit, the static and contract audits, and
`section/runtime-contract-audits`). The rule for what else runs: the direct
owners' `standard` groups, and their `deep` groups only when
`requiredMode` is deep, which the lane refuses (`LANE_DEPTH`). So a lane
receipt is the fast gate plus the owning surfaces' standard groups, the
executed mode equal to the requested mode.

The tier is the floor (Wido (1)): the reader the tier requires at tier 2
(`critique-always`, `hazard.go:55`) and the lane's reader at tier 1 are
never lowered by the receipt's selection.

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
480: four times `dispatch.cap-max` (120, `metasystem.conf:60`; R-58-m1, the
pool is a runaway guard). Active 1. Review rounds 2: the lane's one read and
the closing read after an overflow; inside R-42-m0's ceiling of three
(`metasystem.conf:15`). Why not the tier boxes: tier 1's is `1h/3/360m/1/0`
(`budget.go:248`, `metasystem.conf:16`), zero review rounds, so a tier-1
lane read would be over-norm; tier 2's `4h/6/720m/1/2` is wider than a
lane needs.

Mechanics: `config.LaneBox(confPath)` beside `TierBox` (`budget.go:279`),
same grammar and refusals; a `boxFor(root, f)` helper returns the lane box
when `f.Lane == "small"` and the tier box otherwise, used wherever
`config.TierBox` is read for a goal today (`goalNormApproval`,
`norm.go:105-110`; `requireWithinGoalNorm`, `:151-155`; `withinTierBox`,
`verbs.go:1296`; set-budget's box, `verbs.go:1192-1200`); `openRequest`
(`verbs.go:736`) assigns the lane box when opened `--lane small` with no
tuple. `set-budget` is never needed for a lane goal. Overflow keeps the
tuple (section 4).

Witness: `internal/goal/approval_test.go`, `TestLaneBoxIsTheNorm` (a lane
goal at tier 1 opens with the lane box and its read is within norm;
`set-budget 4h/5/600m/1/2` is `GOAL_NORM_REFUSED` naming `lane-small`; a
`Lane: none` tier-1 goal is judged by `tier-1`).

## 7. The seat's part

In order, and nothing else:

1. `goal open --id <g> --lane small --blocks <claimed goal> --risk ...
   --basis ... --intent "... DONE means ..." --next "... Paths: ... Test:
   ..."`; `goal approve` in Wido's name (R-119-m1e).
2. `goal claim --id <g>`.
3. `metasystem delegate --role implementer --goal <g> --lane small
   --worktree --destructive-reach MECHANICAL --wait`.
4. `metasystem validate conformance --stage review --job <root>`; a lane
   refusal in the verdict: `goal overflow` and the ordinary ladder.
5. `metasystem delegate --role code-critic --goal <g> --lane small
   --reviews <root> --worktree --destructive-reach DESIGN-BEARING --wait`.
6. `metasystem job critique-register-advance --repo . --root-job <critic>
   --round-job <critic>`; `metasystem validate critique-closed ...`; zero
   material: `metasystem delegate close --job <critic>`, then `--job
   <root>`; material: `goal overflow --code LANE_CRITIQUE ...`.
7. `metasystem landing test-receipt --root <root> --tree <tree> --mode
   standard`; `scripts/agents/land.sh -m <note> --chain <root>
   --test-receipt <receipt> --goal <g>`; when the verdict carried
   `engineBinding`, re-arm: `scripts/agents/go-build.sh`, `metasystem up
   --repo <checkout>` (the re-arm rule after every engine landing).
8. `scripts/receipt.sh add --type implement --outcome shipped --goal <g>
   --built-by delegate --delegate <builder id>,<critic id> --note
   "lane=small claim=<stamp> landed=<commit> seat-min=<n> rounds=<n>
   revealed-by=<attempt|none>"`; `goal done --id <g> --conclude "<landing
   note>"`.

The seat writes no brief, no dispositions register in `plans/`, no
critique record in `records/misc/`. The landing note is the Conclude
field: the DONE sentence, the chain and critic ids, the receipt id and its
group count, the landing commit, the re-arm line when engine-binding, and
the four DONE (4) facts. Eight commands and one paragraph.

## 8. Tier 1: which path, and why

Outside the lane a tier-1 goal is R-54-m1's direct fix: a receipted build
with no reader, at most three files and forty lines, refused on the floor
rows (`tierone.go:95`, `:117`, `:120`; `path-classes.txt:70-85`:
`internal/goal/`, `internal/dispatch/`, `internal/landing/`,
`internal/validate/`, `internal/config/`, `dispatch.sh`, `land.sh`, the
schemas, the class manifests, `metasystem.conf`), landed with `land.sh
--direct-fix tier-1 --root-job` (`land.sh:98`, `:208`). Unchanged by this
build.

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

A rollout row, not a fixture. The first real lane landing is the specimen:
R-90-m1's builder-row sweep is unit U0 of this build and lands first on
the ordinary ladder, so the first lane specimen is the next real one-line
fix a seat needs after unit U5b lands (candidates at this tip: the stray
tracked `records/narrator-digest.log` beside the repository root once
every seat has synced past 7c4e9cd5, one file, tier 1, off the floor rows
but over the direct fix's shape as a deletion; any trunk-red one-liner of
the kind landed three times on 2026-09-16).

Measured facts, all mechanical: `claim=` the `claim` history stamp on the
lane goal; `landed=` the landing commit and its author stamp; `seat-min=`
the minutes between them; `rounds=` the count of terminal job records
bound to the goal (`job chain-members --terminal-only` over every root
bound to `<g>`); both thresholds: 45 minutes and 3 rounds. Where recorded:
the RECEIPT line's `note=` in `memory/receipts.log` (`scripts/receipt.sh`
is a shim to `metasystem receipt`, and the log line already carries
`goal=`, `built_by=`, `delegate=` and `note=`) and the goal's Conclude. The
test receipt (`internal/landing/receipt.go:39-56`) is not the place: it
binds trees, attempts and proof, has no note field, and is an engine
artifact the lane does not change. Where read: the ten-landing review
(`grep 'lane=small' memory/receipts.log`), which recalibrates the bound,
and the delivery-efficiency plan's weekly measure (delegate rounds per
landing, human acts per landing; plan section 5). The goal is not
concluded before this row is recorded with both thresholds met; a specimen
over either threshold is recorded as such and a second specimen is run.

## 10. The two-runtime proof

The lane lives in the engine (`internal/goal`, `internal/dispatch`,
`internal/landing`, `internal/testpolicy`, `internal/config`), the ledger
verbs (`goal open|edit|overflow`) and the dispatcher (`dispatch.sh`, the
templates), above the adapter contract; no adapter (`scripts/agents/adapters/
{claude,codex,devin,fake}.sh`, `internal/adapter/`) learns the word `lane`.
Proof on two runtimes:

1. Fake runtime: the dispatch and land bed scenarios of section 12 run the
   whole lane under `adapters/fake.sh`.
2. Real runtimes: the DONE (4) specimen's build runs on codex (gpt-5.6-sol)
   and its read on claude (Opus 5); both job ids are in the RECEIPT line;
   the root records show `lane: "small"` and the rendered brief bytes hash
   identically under both adapters (`job lane-brief` is deterministic over
   the record; `TestLaneBriefRendersFromRecord` pins the bytes).
3. Static witness: `section/runtime-contract-audits` (in `always.standard`,
   `testing.json:104`) gains one check: no file under
   `scripts/agents/adapters/` or `internal/adapter/` contains the token
   `lane`. It fails the day an adapter carries lane logic. ASSUMPTION: the
   section's script is found through `validate-section-selector.sh
   catalog`; the exact file is named by the builder of unit U6.

## 11. Units

Each unit is one chain on the ordinary ladder: Codex Sol builds in a
worktree with a brief that declares its Boundary and Ceiling (goal 28) and
proves every rule by mutation in its return (goal 29); Opus reads; the
critique is by another model. Allocation is additions plus deletions,
tests included, at most 300. "After" is the observable state.

| Unit | Files (tests) | Alloc | DONE | Witness | After |
| --- | --- | --- | --- | --- | --- |
| U0 R-90-m1's builder row | `internal/dispatch/hazard.go:37-41`, `scripts/agents/role-packets.json:4-11` (`hazard_test.go`, `composition_test.go`) | 60 | MECHANICAL reads maximal/xhigh, critique false, live proof false, in both surfaces | the table-equality test fails on either surface alone changed; a MECHANICAL dispatch at medium is refused (`build.go:486-488`) | every MECHANICAL build runs at xhigh; the lane can dispatch |
| U1a the record | `internal/goal/file.go`, `verbs.go`, `cmd/metasystem/goalsync_mutations.go`, `internal/refusal/register.go` (`internal/goal/lane_test.go`, `register_test.go`) | 260 | `Lane` parses, renders, migrates; `--lane` on open and edit; `LANE_TIER`, `LANE_WIDTH`, `LANE_DONE_SENTENCE`, `LANE_TEST`; the digest hashes the lane; rows registered | `TestLaneOpenAdmission`, `TestLaneChangesTheApprovalDigest`, `TestLaneEditAfterApprovalNamesTheVerbs`, the register audit | a goal can be opened in the lane and approved with the lane in its digest |
| U1b the box | `internal/config/budget.go`, `internal/goal/norm.go`, `verbs.go` (`boxFor`), `metasystem.conf` (`approval_test.go`, `config` tests) | 140 | `metasystem.budget.lane-small` read wherever the tier box is read | `TestLaneBoxIsTheNorm` | a tier-1 lane goal's read is within norm |
| U2 overflow | `internal/goal/verbs.go`, `file.go` (`Overflow` line, `overflow=` authority), `cmd/metasystem/main.go` and `goalsync_mutations.go`, `register.go` (`lane_test.go`) | 280 | `goal overflow` as section 4 | `TestLaneOverflowRebindsAndKeepsTheBox`, `TestLaneOverflowRefusesForeignPair` | a claimed lane goal leaves the lane in one recorded transaction |
| U3a the rendered briefs | `cmd/metasystem/lanebrief.go` (new), `main.go`, `scripts/agents/templates/lane-brief.md`, `lane-review-brief.md` (`lane_brief_test.go`) | 280 | `job lane-brief` renders both templates from the record | `TestLaneBriefRendersFromRecord` | a brief exists that no one wrote |
| U3b dispatch | `scripts/agents/dispatch.sh` (`--lane`, `LANE_HAZARD`, `LANE_ROLE`, `LANE_ROUNDS`, `LANE_BRIEF`, the tier ladder under the lane, the root field), `internal/dispatch/admission.go:15`, `build.go`, `register.go` (`dispatch-fixtures.sh` scenario `lane-dispatch-admission`) | 300 | section 2's dispatch rules; N3 confirmed first | the scenario: wrong class refused with no husk; design-critic refused; `--brief` with `--lane` refused; code-critic admitted at tier 1 under the lane; `rounds/1/prompt.md` carries the DONE sentence, the `Test:` line and one Working Mode; the root carries `lane` | a lane chain dispatches on the rendered brief |
| U4a direct owners | `internal/testpolicy/select.go` (`DirectOwners`) (`direct_owners_test.go`) | 90 | the prefix match before the fallback is callable | `TestDirectOwners`; the receipt for this unit is deep (`internal/testpolicy/` is protected, `protection.go:41`) | unowned is decidable |
| U4b review admission | `internal/dispatch/lane.go` (new), `internal/validate/conformance.go` (the review-stage hook), `internal/landing/tierone.go` (the metric lifted), `register.go` (`lane_test.go`) | 300 | `LaneAdmission` with every section-2 review code; the verdict on the root | `TestLaneAdmissionBounds`, `Scope`, `RecordPaths`, `Owners`, `Depth` | the box is enforced on the computed diff |
| U5a close | `internal/dispatch/close.go` (`decisions_test.go`) | 140 | the lane close duty | `TestCloseCheckRequiresLaneReaderAndVerdict` | no lane chain closes without its reader and verdict |
| U5b landing | `internal/landing/observe.go`, `promotion.go`, `scripts/agents/landing-promotion.json` (`land-fixtures.sh` scenarios `small-lane-clean-chain-lands`, `small-lane-uncritiqued-refused`) | 280 | `chain-not-critiqued`, `chain-lane-refused`, `lane=small` provenance | the two scenarios | a lane chain lands with a pass verdict at either tier; an uncritiqued one is refused |
| U6 docs and the audit line | `docs/orchestration.md` (the working-modes table, the tier-1 paths of section 8, the `revealed-by=` rule), `AGENTS.md` intake paragraph, `skills/code-critique/SKILL.md` (the lane's one-read note), `records/misc/severity-tiered-rigor-obligation-matrix.md`, the runtime-contract audit script (section 10) | 140 | the rules seats follow are written where seats read | `scripts/audit-metasystem.sh` (the 1400-word bundle cap), the audit line of section 10 | the lane is documented and its adapter boundary audited |

Total allocation 2270 lines over eleven units. Order: U0, U1a, U1b, U2,
U3a, U3b, U4a, U4b, U5a, U5b, U6; U4a may go beside U3a. The lane is
usable after U5b; the DONE (4) row follows.

## 12. Fixtures and how they run

Go tests: `cd metasystem && go test ./internal/<pkg> -run '^<Test>$' -count=1
-timeout=2m` (`internal/goal` needs `-timeout 40m` under race and load).
Process fixtures are scenarios of their beds and run the way the beds run
today: the parent runs the engine-selected set (`dispatch-fixtures.sh:8-12`
accepts only `--comparison`; `:46-47` asks `proof-run fixture-selection
--family dispatcher`), each scenario as a child `--fixture-bed-child
<scenario> <capability>` with a capability the parent mints
(`fixture-budget.sh:26-65`; `fixture-bed-scenarios.sh:132-136`); the land
bed lists its scenarios at `land-fixtures.sh:33-38`. A new scenario is added
to the bed's list (land) or to the dispatcher family's owned set
(ASSUMPTION: declared where `proofrun.FixtureScenarios` reads it). The
verification run is the engine's: `bin/metasystem test run --root . --goal
<claimed goal> --tree <tree> --mode canary --groups section/<bed> --purpose
diagnostic --cap-min <min>` from the enrolled checkout, re-armed at the tip;
`land-fixtures.sh` also runs standalone from a worktree with a built
`bin/metasystem`.

| Scenario | Bed | Proves, and where it fails today |
| --- | --- | --- |
| `lane-dispatch-admission` | dispatch | as unit U3b's witness. Fails today: `--lane` is an unknown option. |
| `lane-one-read` | dispatch | a clean read: advance, `critique-closed` zero material, close both roots; a material read: `goal overflow --code LANE_CRITIQUE` records, the goal reads `Lane: none`, a critic follow-up is admitted, a lane implementer follow-up before overflow refused `LANE_ROUNDS`. Fails today: no lane. |
| `small-lane-clean-chain-lands` | land | a fake-runtime lane chain at tier 2 and one at tier 1, each with an admitted verdict, a clean closed critic and a schema-2 standard receipt, lands `pass bar=a` with `lane=small`. Fails today: `would-refuse code=chain-not-design-bearing` (`observe.go:395`), tier 1 has no reader path. |
| `small-lane-uncritiqued-refused` | land | the same chain without a critique reference is `chain-not-critiqued` and refused (promoted); with a `LANE_FILES` verdict, `chain-lane-refused`. Fails today: lands with a trailer. |

## Open questions (real gaps only)

1. Whether the review stage enforces the brief's `Ceiling` on the diff or
   only the `Boundary` (`conformance.go:399`, `:733`); the lane's line bound
   is its own check either way.
2. Where the dispatcher family's owned scenario set is declared for
   `proof-run fixture-selection`; the builder of U3b names it.

## Revision record

Revision 1 (2026-09-10, `scl-design1`): the lane as a record field bound
into the digest; three admission moments with registered codes; four files
and eighty lines; record paths refused; one build on a rendered brief; one
critique with one fold and one closing read; the box `4h/4/480m/1/2`; tier
1 folded into the lane with its direct-fix class retired; tier 3 admitted.

Revision 2 (2026-09-16, this page): folds Wido's 09-10 decisions and his
four answers of 09-16 and every material and minor finding of the Codex
read, as the dispositions table says. Changed from revision 1: tier 3 out
(`LANE_TIER`); the rule-scope clause (`LANE_SCOPE`) and `LANE_DEPTH`; one
read and no in-lane fold; the critic root DESIGN-BEARING; the design gap
read off the verdict (`LANE_CRITIQUE`); `goal overflow` with a re-derived
digest and the unit-of-an-existing-goal record; the changed surfaces decide
depth and the receipt is requested standard; `Test:` and `Paths:` lines,
Working Mode, Boundary and Ceiling in the rendered brief; `DirectOwners`;
R-90-m1's row as unit U0; the direct-fix class kept outside the lane and
the tier-1 path rule of section 8; one lane box for both tiers; the DONE
(4) row and the two-runtime proof; fixtures on the beds' real protocol;
`delegate close --job`; the register advance before `critique-closed`.

Next: one Codex Sol design read; then units U0 to U6 built by Codex Sol in
worktrees, each read on Opus, landed under R-117-m1e item 5 (Opus read with
no NOT LAND, green selected groups, receipt in the same commit).

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
