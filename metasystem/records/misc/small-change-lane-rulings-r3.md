# small-change-lane r3: fold check (R-117-m1e item 2), m1e checking delegate, 2026-09-16

Inputs: `scl/small-change-lane-design-r3.md` (end table L1093-1119, then only the cited lines),
`scl/small-change-lane-critique-r2.md` (findings L28-160, rigor classification L162-177).
The r3 table's line citations (L85, L163, L220, L254, L268, L294, L428, L482, L505, L636, L736,
L788, L862, L917, L975) all land on the text they name.

## Step 1: findings table

| Finding | What the critique asked | r3 lines that fold it | Status | Meets the reopening trigger's text |
| --- | --- | --- | --- | --- |
| SCL-R2-M01 | Lane rule and refusal code comparing the computed changed paths with the goal revision's `Paths:`; witness where a return truthfully declares an unauthorized path | L254-255 "`LANE_PATHS`: a changed path that is not a member of the goal revision's `Paths:` set"; L316-320 `TestLaneAdmissionPaths` | FOLDED | Yes: the computed-diff witness refuses `extra.go` declared in `diffBoundary` but absent from `Paths:` |
| SCL-R2-M02 | Complete verified reader set through one owned predicate, plus a test that fails on an unclassified new reader | L285-288 "`ProjectionReaders()` in `internal/behaviorsurface/readers.go` is, and its test walks the module for both kinds of reader and fails on any difference in either direction"; U4a L960 | FOLDED | Yes: one tested owner (U4a, `TestProjectionReadersEnumerateEveryConsumer`) |
| SCL-R2-M03 | Auto-mode selection to derive `requiredMode`, refuse when deep, literal standard only for the landing receipt; witness on an existing non-protected risk-raising surface | L294-296 "`testpolicy.Select(contract, {ChangedPaths, RequestedMode: auto, ...})` returns a plan whose `RequiredMode` is `deep`"; L324-326 witness on `internal/proofrun/coverage.go` | PARTIAL | Mechanism yes; the named witness is not deterministic. r3 L278 lists `internal/proofrun/coverage.go` as a `LANE_SCOPE` (c) reader. The verdict has one `code` (L244), the page names no precedence, and the bullet order puts SCOPE before DEPTH, so that path yields `LANE_SCOPE`, not `LANE_DEPTH`. The unowned-path sub-case does yield DEPTH, but through the `residual` fallback, not the owned surface the critique named. One-line repair: base the witness on an `internal/usage/` path (context-budget, severity 2, not protected, not in the reader list; r3's own L306), or state that DEPTH is judged and recorded on its own |
| SCL-R2-M04 | Remove `LANE_WIDTH` and `LANE_SURFACES`; direct owners only derive the plan; the depth rule overflows | L168-169 "revision 2's `LANE_WIDTH`, `LANE_SURFACES`, `LANE_UNOWNED_PATH` and `LANE_RECORD_PATH` are removed (round-2 M04)"; decision 2 L85-100 | FOLDED | Yes: grep finds the four codes only in removal statements (L62, L168-169, L1052-1053); eligibility is tier, box, clause |
| SCL-R2-M05 | Role or chain-shape guard so the duty applies only to the implementation root; witness closes the critic root first, then the work root on that reference | L482-483 "gains one duty for a root whose `role` is `implementer` and whose record carries `lane: "small"`"; witness L498-503 | FOLDED | Yes: the critic root closes with no critique reference, then the work root passes on it |
| SCL-R2-M06 | `LANE_ROUNDS` keyed to the current goal state: refuse under `Lane: small`, admit after a recorded overflow; positive post-overflow witness | L220-224 "refused while the goal's record ... still reads `Lane: small`; it is admitted, as an ordinary follow-up, once the record reads `Lane: none` with an `Overflow:` line" | FOLDED | Yes: `lane-one-read` (L1019) refuses before and admits after on the same frozen root |
| SCL-R2-M07 | Name the canonical critic `return.json` and an engine-owned dispositions artifact with the required header, or a zero-findings verb | L440-442 "`metasystem job lane-dispositions --repo . --root-job <critic> --out artifacts/agents/<critic>/lane-dispositions.md` (new, unit U5a)"; exact `critique-closed` call L457-460 | FOLDED | Yes: join inputs, owner (`lanedispositions.go`, U5a) and the header (`critiqueclosed.go:13`) are named; `lane-one-read` exercises the existing command |
| SCL-R2-M08 | An operation that carries the reviewed tree into the landing checkout, binds to the closed critic, stages the paths plus a pre-created receipt, and calls `land.sh` in a valid mode; one end-to-end witness | L522-523 "`metasystem landing materialize --root . --chain <root>` (new, unit U5c)"; L542-543 "appended to `memory/receipts.log` in the landing checkout before anything is committed" | FOLDED | Yes: `small-lane-worktree-to-landing` (L1022) runs from an uncommitted worktree change to one commit with the reviewed tree and the RECEIPT line. No merge stage is run (reasoned at L515-519); the binding to the closed critic goes through `chainCertifiedOutput`, which selects by the critic closure identity (spot-check 3) |
| MOVED-EFFECTS-SCL-R2-M09 | Moved-effects table naming the destination package and function and both callers; the validator passes | L926 table row `internal/landing` (`tierOneDiffMetric`) to `internal/gittree` (`Workspace.DiffShape`); L928-935 signature and both callers | FOLDED | Yes. I ran `metasystem/bin/metasystem validate moved-effects --file <r3> --root R0`: `moved-effects: inventory=present rows=1 problems=0`, exit 0 |
| SCL-R2-M10 | Exact group ids, exact catalog edits, and a command that checks selection before the beds run | L987-988 dispatcher group `section/dispatcher-adapter-and-mission-runner-fixtures` (`testing.json:99`); L1001-1003 land array gains three names, count 31 | FOLDED | Yes. The ids are at origin/main `testing.json:99` and `:72` as r3 says (the critique had the order swapped). Nit, not a trigger miss: the land check `grep -c -e A -e B -e C` prints one total line count, not "each name at least twice"; the dispatcher check is exact |
| SCL-R2-M11 | Paired same-role evidence on Codex and Claude with normalized records compared; hash equality only for the same template and inputs | L876-879 "specimen A builds on codex ... specimen B builds on claude and is read on codex"; L905-906 "Equality of rendered bytes is claimed only for the same template and the same inputs" | FOLDED (the trigger depends on Q1) | Yes if Q1 is granted. If it is refused, each role runs on only one real runtime and the trigger's "each exercise equivalent lane operations" is not met on real runtimes; the page says so (L893-895) |
| SCL-R2-M12 | Seat time measured with an injected clock before landing; a same-commit receipt with no self-referencing hash; an eligible specimen, or wait for one | L793-797 `LaneMeasureOptions{Now func() time.Time}`; L825-826 "The line carries no commit hash"; L839-841 no candidate named | FOLDED | Yes, with a stated narrowing: `seat-min` is seat-attended time (delegate spans subtracted, idle time still counted), declared as an upper bound on active seat time (L811-820). For a "≤45" threshold that bound is enough; the seat accepts or rejects the narrowing |

Reopened round-1 rows (R1-M05, M08, M12, M13, M14) and the two open-question recommendations map to the rows above and add no separate gap.

Spot-checks against origin/main `9cafaae5f` (r3 was written against `7e99bb110`, an ancestor):

1. `internal/dispatch/admission.go:232-234`: `if binding.Tier == 1 && hazard != HazardMechanical { verdict.PolicyRefusal = "HAZARD_REFUSED: ..."`. Matches r3.
2. `internal/dispatch/finding_register.go:1073-1089`: matches r3 at `7e99bb110` (`if !material { ... Resolution = "withdrawn" } continue`). At origin/main the file changed by 97 insertions and 13 deletions, so those lines now hold `artifactAbsentFromTree` and `foldCritiqueFindings`. The same non-material skip or withdraw logic now sits at `:1122-1128`. The meaning holds; the line number is stale at the tip.
3. `internal/landing/observe.go:593-660`: `chainCertifiedOutput` reads `rounds/<n>/review.json` and `diff.patch` and selects the output by the critic closure's tree and patch digest. Matches r3 (the file is unchanged since `7e99bb110`).

## Step 2: questions for the seat (1)

Q1, verbatim (r3 L1123-1130):

> Q1 (Wido's, through the seat; R-108-m1c is his word): may the two-runtime
> proof's specimen B run its build on claude and its read on codex, once, for
> that one real lane goal, so that each role is exercised on both runtimes
> (section 10, leg 2)? Recommendation: yes, for specimen B only, recorded in
> the two-runtime proof record with this page as the reason; without it the
> paired same-role evidence the DONE's runtime independence asks for cannot
> exist on real runtimes, and the page then rests leg 2 on specimen A alone.
> Nothing on this page waits for the answer: units U0 to U6 build either way.

r3's recommendation: yes, for specimen B only. No other seat question. The page's "Open questions" item 1 (where `section/runtime-contract-audits` finds its script) is left to the U6 builder, not the seat.

## Verdict: NOT ALL FOLDED

11 of 12 are FOLDED. One gap:

- SCL-R2-M03, PARTIAL: the witness named to meet the trigger (`internal/proofrun/coverage.go` refusing `LANE_DEPTH`) is also a `LANE_SCOPE` (c) reader on the same page. With one verdict `code` and no stated precedence, it cannot deterministically yield `LANE_DEPTH`. The repair is one line: base the witness on an `internal/usage/` file, or state that DEPTH is judged and recorded on its own.

Conditions the seat should note (not gaps): M11's trigger depends on Q1, and M12 meets its trigger as a stated upper bound.

## Step 3: landing

Not prepared, because Step 3 is gated on all 12 being FOLDED. What I found for when it runs: origin/main has no `plans/small-change-lane*-design.md` (only `plans/goals/small-change-lane.md`), so the target is `metasystem/plans/small-change-lane-design.md`. The pa-page precedent (`pa-page-land.diff`) also carried `records/misc/` companions: `<goal>-critique-r1.md`, `-critique-r2.md`, `-design-brief-r1.md`, `-design-r1.md`, `-design-r2.md` and `-rulings-r2.md`.

## Seat ruling and corrections (m1e, 2026-09-16, after this check)

The seat closed SCL-R2-M03 by correcting the page, with no new design round (R-117-m1e item 2, final round). The corrections are recorded in the page's own "Seat corrections (m1e, 2026-09-16)" section:

1. The depth witness is now `internal/usage/retention.go`. At origin/main `9cafaae5f` it is owned by `context-budget` (`testing.json:16`, `risk: {severity: 2}`), it is not in the page's reader list, no `internal/usage/` file mentions `behaviorsurface`, and it matches no `ProtectedPolicyChange` pattern. It is not a residual path: `testing.json:27` covers only unowned paths, which stay the witness's second case.
2. Admission's single refusal `code` is the first failing check in the page's order (tier, box, clause, depth). Witnesses are chosen so their named check fails first, and the depth witness fails that check alone.
3. The `finding_register.go` cite moves from `:1073-1089` to `:1122-1128`.

The findings table's line numbers after L246 moved down to match. The line numbers in this check's table above refer to r3 before these corrections. Afterwards, `validate moved-effects` on the corrected page printed `inventory=present rows=1 problems=0` (exit 0). Verdict after the ruling: ALL FOLDED. SCL-R2-M11 still depends on Q1, which is open with Wido and does not block the build or the landing.
