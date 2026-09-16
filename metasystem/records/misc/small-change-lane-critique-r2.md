# small-change-lane design critique — round 2 of 3

## Evidence basis

I reviewed design revision 2, its brief and context pack, the goal record, all fourteen prior material findings, the binding rulings, and the current implementation. The current remote-tracking `origin/main` is `1f17cb99e`; its changes after this worktree's code revision are confined to unrelated plan and record files, so the cited implementation lines are materially identical on current `origin/main`. Evidence below is from reading source and Git state. Per the review constraint, I ran no Go test, metasystem test, fixture bed, or process-control command.

The mandatory moved-effects validator could not be run because this source checkout has no `metasystem/bin/metasystem` executable. Reading its implementation confirms that an absent `Moved effects` section is reported for the critic to adjudicate (`metasystem/cmd/metasystem/validate_verbs.go:418-472`). Finding `MOVED-EFFECTS-SCL-R2-M09` performs that adjudication.

## Prior material findings

| Prior finding | Closed by revision 2? | Evidence in this revision |
| --- | --- | --- |
| `SCL-R1-M01` — tier 3 admitted | **Closed.** | Section 2 makes `LANE_TIER` admit only effective tier 1 or 2 (`artifacts/reports/small-change-lane-design-r2.md:125-131`), and the revision record says tier 3 is out (`:685-689`). |
| `SCL-R1-M02` — no protected-surface refusal | **Closed.** | Section 2 makes `testpolicy.ProtectedPolicyChange` part of `LANE_SCOPE` (`artifacts/reports/small-change-lane-design-r2.md:193-198`). The separate engine-reader enumeration defect is new finding `SCL-R2-M02`. |
| `SCL-R1-M03` — incompatible tier-1 routes and boxes | **Closed.** | Sections 6 and 8 give both lane tiers one lane box while retaining the tier-1 direct-fix route outside the lane (`artifacts/reports/small-change-lane-design-r2.md:471-501`, `:538-564`), matching Wido's 2026-09-16 answer. |
| `SCL-R1-M04` — MECHANICAL critic cannot close | **Closed.** | Section 3 dispatches the critic as DESIGN-BEARING and grounds that choice in the present closure check (`artifacts/reports/small-change-lane-design-r2.md:297-325`; `metasystem/internal/dispatch/hazard.go:329-363`). |
| `SCL-R1-M05` — bounded-fold topology cannot close | **Not closed.** | Section 3 removes the in-lane fold and adds the missing terminal register advance (`artifacts/reports/small-change-lane-design-r2.md:327-346`), but the clean path still cannot execute: the new close duty also applies to the critic root (`SCL-R2-M05`), and `critique-closed` has no dispositions input (`SCL-R2-M07`). |
| `SCL-R1-M06` — impossible `LANE_DESIGN_GAP` register side effect | **Closed.** | The side effect is gone; any material lane critique produces overflow code `LANE_CRITIQUE` (`artifacts/reports/small-change-lane-design-r2.md:338-346`, `:420-423`). |
| `SCL-R1-M07` — plain edit invalidates approval | **Closed.** | Section 4 specifies a dedicated atomic `goal overflow` transaction, re-derived approval digest, claim rebind, and target-goal history entry (`artifacts/reports/small-change-lane-design-r2.md:385-432`). The separate post-overflow dispatch contradiction is new finding `SCL-R2-M06`. |
| `SCL-R1-M08` — stale proof-depth law | **Not closed.** | Sections 2 and 5 adopt Wido's binding changed-surface law, but the named `standard` selection cannot return the deep plan the admission rule proposes to inspect, and the page falsely says no surface raises risk (`SCL-R2-M03`). |
| `SCL-R1-M09` — brief lacks Working Mode and replayable test | **Closed.** | Section 2 requires structured `Test:` and `Paths:` lines, and section 3 renders exact Working Mode, Boundary, Ceiling, test command, and DONE sentence (`artifacts/reports/small-change-lane-design-r2.md:134-142`, `:253-295`). |
| `SCL-R1-M10` — unowned path hidden by fallback | **Closed.** | Section 2 adds `testpolicy.DirectOwners` before fallback and names its witness (`artifacts/reports/small-change-lane-design-r2.md:215-219`, `:247`). |
| `SCL-R1-M11` — R-90 builder row missing as prerequisite | **Closed.** | Section 11 makes the R-90 MECHANICAL xhigh row unit U0 and orders it first (`artifacts/reports/small-change-lane-design-r2.md:625-641`). |
| `SCL-R1-M12` — fixture commands do not run | **Not closed.** | Section 12 now describes the child-capability protocol, but its public verification command still uses nonexistent placeholder group names `section/<bed>`; see `SCL-R2-M10`. |
| `SCL-R1-M13` — no real DONE (4) measurement | **Not closed.** | Section 9 adds a real rollout row, but it measures elapsed wall time rather than seat time and requires a receipt to name the hash of the commit that contains that receipt; see `SCL-R2-M12`. |
| `SCL-R1-M14` — runtime independence asserted, not proved | **Not closed.** | Section 10 adds two runtime families, but Codex builds while Claude reviews, so neither runtime proves the other's lane operation, and the claimed identical brief bytes are impossible; see `SCL-R2-M11`. |

## New and surviving material findings

### SCL-R2-M01 — The goal's `Paths:` boundary is not enforced

Severity: **high**
Material: yes
Evidence level: read

Section 2 says the generated brief's `Boundary` is the path bound and that conformance already refuses a crossing (`artifacts/reports/small-change-lane-design-r2.md:176-192`). Section 3 therefore adds no lane rule comparing the reviewed candidate's changed paths with the goal's `Paths:` list (`:268-295`). That premise is false. `ParseBriefBounds` validates only the syntax and authority of the paths written in the brief (`metasystem/internal/dispatch/brief.go:151-201`). Conformance builds its cumulative boundary from each implementer return's self-declared `diffBoundary` (`metasystem/internal/validate/conformance.go:766-817`) and rejects only changed paths outside that declaration (`:819-831`). It never reads the lane brief's Boundary or the goal record's `Paths:` list.

An implementer can therefore change an undeclared fifth kind of file, declare it in its return, remain under the four-file/eighty-line counts, and pass the designed review gate. That builds a different authority boundary from the one Wido approved. Add a lane admission rule and refusal code that compare the computed changed-path set with the goal revision's canonical `Paths:` set, or make conformance consume the admitted brief bounds; add a witness where a return truthfully declares a changed path that the goal did not authorize.

### SCL-R2-M02 — `LANE_SCOPE` omits four current ENGINE-projection readers

Severity: **high**
Material: yes
Evidence level: read

The seat ruling fixes the scope as the ENGINE projection's declaration and readers, not every member (`artifacts/reports/small-change-lane-design-r2.md:703-711`). Section 2 implements that ruling with `internal/behaviorsurface/**`, `internal/proofrun/manifest.go`, `internal/proofrun/freeze.go`, and `internal/landing/registers.go` (`:193-211`). The stated reader list is not the current code's reader list.

Current production readers of `behaviorsurface.Engine` include:

- proof-run classification in `metasystem/cmd/metasystem/proof_run.go:1426-1446`;
- landed re-arm classification in `metasystem/cmd/metasystem/rearm_on_landed.go:143-166`;
- governed surface-digest capture in `metasystem/internal/dispatch/governed.go:25-27`;
- manifest membership in `metasystem/internal/proofrun/manifest.go:156-168`; and
- archived-engine digest resolution in `metasystem/internal/steward/rearm_resolver.go:187-221`.

By contrast, the named `internal/proofrun/freeze.go` and `internal/landing/registers.go` do not import or call the behavior-surface policy. Literal implementation would let changes to four projection readers take the small lane while excluding two unrelated files. Replace the hand-written list with the complete verified reader set, preferably through one owned predicate and a test that fails when a new production ENGINE reader is added without classification.

### SCL-R2-M03 — The proof-depth test cannot observe deep selection as designed

Severity: **high**
Material: yes
Evidence level: read

`LANE_DEPTH` says to call `testpolicy.Select` in requested `standard` delivery mode and inspect `plan.requiredMode` for deep (`artifacts/reports/small-change-lane-design-r2.md:220-227`). Current selection returns no plan in exactly that case: when required mode is deep and delivery requested standard, it returns `Plan{}` plus an error (`metasystem/internal/testpolicy/select.go:212-217`). The page also says no surface declares a raise because `testing.json` has no `riskRaise`; the schema field is named `risk` (`metasystem/internal/testpolicy/contract.go:51-59`), and current surfaces `coverage-policy` and `context-budget` both raise severity to 2 (`metasystem/testing.json:14-16`). `requiresDeep` treats that raise as deep (`metasystem/internal/testpolicy/risk.go:106-114`). Some non-protected, directly owned changes therefore take this branch today.

Literal implementation must either treat an expected insufficiency error as the verdict, lose the required-mode evidence, or accidentally admit a zero plan. Specify an auto-mode selection used only to derive `requiredMode`, followed by refusal when it is deep; reserve the literal standard request for the admitted landing receipt. Add a witness using an existing non-protected risk-raising surface.

### SCL-R2-M04 — Two extra eligibility gates contradict the binding lane contract

Severity: **high**
Material: yes
Evidence level: read

The binding goal answer says tiers decide whether the lane may be used, the `4 files/80 lines` box decides fit, and the refusal/protected/ENGINE clause plus reader prevent a large change sliding through (`metasystem/plans/goals/small-change-lane.md:8-10`). Revision 2 additionally refuses every goal whose accumulation is 2 or 3 (`LANE_WIDTH`) and every candidate with more than two direct owners (`LANE_SURFACES`) (`artifacts/reports/small-change-lane-design-r2.md:128-140`, `:215-219`). Neither extra gate is in Wido's four answers or the recorded DONE.

This is not an implementation detail. Accumulation does not raise the tier; it changes gate width (`metasystem/internal/goal/file.go:126-143`), and four authorized paths can lawfully have three direct owners. An implementer following this page would build a materially narrower lane than the decided one. Remove those admission gates. Continue to use direct owners to derive the selected proof, and let the changed-surface depth rule overflow a candidate when its actual proof requirement is deeper than standard.

### SCL-R2-M05 — The new close duty makes the required critic root recursively unclosable

Severity: **high**
Material: yes
Evidence level: read

The code-critic is dispatched with `--lane small` (`artifacts/reports/small-change-lane-design-r2.md:297-300`), and every lane root freezes `lane: "small"` (`:168-171`). The new `CloseCheck` duty then applies to “a root with `lane: "small"` at any tier” and requires an admitted lane verdict plus a distinct independent critic (`:348-355`). That includes the critic root the seat must close before it can close the work root (`:327-336`). The critic root has no `laneAdmission`—that verdict is written by conformance review of the implementation root—and requiring another independent critic would recurse forever.

Current hazard closure explicitly exempts critic-only chains because applying the builder's independent-review duty to them would require an infinite critic chain (`metasystem/internal/dispatch/hazard.go:243-250`). The lane duty needs the same role/chain-shape guard: apply it only to an implementation root. The witness must first close the critic root itself and then prove the implementation root consumes that closed reference.

### SCL-R2-M06 — Overflow preserves the frozen lane marker while refusing the implementer fold it promises

Severity: **high**
Material: yes
Evidence level: read

`LANE_ROUNDS` unconditionally refuses an implementer follow-up “on a lane root,” and the root's lane marker deliberately remains frozen and is inherited by follow-ups (`artifacts/reports/small-change-lane-design-r2.md:163-171`). Sections 3 and 4 nevertheless require a material finding or admission refusal to continue on that same work root with an ordinary-ladder implementer fold (`:338-346`, `:403-418`). Follow-ups already inherit immutable root facts such as goal tier, gate width, review subject, workspace, and base (`metasystem/internal/dispatch/build.go:828-888`). The design defines no post-overflow exception to the proposed frozen-lane refusal.

The fixture plan tests refusal before overflow and admission of a critic follow-up, but not admission of the required implementer follow-up after overflow (`artifacts/reports/small-change-lane-design-r2.md:662-667`). Define `LANE_ROUNDS` against a validated current goal state: refuse while the goal is still `Lane: small`, admit ordinary implementer follow-ups after a matching recorded overflow revision, and add that positive post-overflow witness.

### SCL-R2-M07 — The clean-read closure command has no required dispositions artifact

Severity: **medium**
Material: yes
Evidence level: read

The seat flow says `metasystem validate critique-closed ...` but also says the seat writes no dispositions register (`artifacts/reports/small-change-lane-design-r2.md:517-533`). The present command requires both `--findings` and `--dispositions`, including when the findings array is empty (`metasystem/cmd/metasystem/validate_verbs.go:74-95`). Its parser refuses a missing file or a file without the exact dispositions table header (`metasystem/internal/validate/critiqueclosed.go:154-205`). Advancing the critic register is a different operation and does not supply that join input.

Name the canonical critic `return.json` path and an engine-owned, ephemeral or retained zero-row dispositions artifact with the required header, or design a specific zero-findings closure verb. “No new register entry” does not waive the existing join contract.

### SCL-R2-M08 — The prescribed seat sequence cannot land the reviewed candidate or its receipt

Severity: **critical**
Material: yes
Evidence level: read

The implementation brief tells the delegate to leave changes uncommitted in its job worktree (`metasystem/internal/dispatch/build.go:1133-1140`). The seat sequence runs only conformance review, then invokes `land.sh` with no materialization step, no pathspec, and no `--staged-only` (`artifacts/reports/small-change-lane-design-r2.md:503-525`). Current conformance has distinct review and merge stages (`metasystem/internal/validate/conformance.go:255-261`), while the described sequence invokes only review. Current `land.sh` refuses a call with neither pathspecs nor `--staged-only` (`metasystem/scripts/agents/land.sh:172-178`) and stages only the caller's explicit pathspecs (`:386-418`).

The receipt ordering is independently impossible: the page calls `scripts/receipt.sh add` only after `land.sh` (`artifacts/reports/small-change-lane-design-r2.md:521-530`), but `land.sh` checks that the staged candidate already appends the goal's RECEIPT line before it commits (`metasystem/scripts/agents/land.sh:487-513`, `:1091-1093`). This also violates binding R-117-m1e item 5's same-commit receipt requirement (`metasystem/memory/rulings.md:176-178`).

Specify the existing or new operation that transfers the exact reviewed tree into the landing checkout, binds merge conformance to the closed critic, stages the exact candidate paths plus the pre-created receipt line, and then calls `land.sh` in a valid mode. Add one end-to-end witness that begins with an uncommitted job-worktree change and observes the same reviewed tree and receipt line in one landing commit.

### MOVED-EFFECTS-SCL-R2-M09 — A named count owner moves with no moved-effects inventory or destination owner

Severity: **medium**
Material: yes
Evidence level: read; validator unavailable

Section 2 explicitly moves `tierOneDiffMetric` from landing into “a shared helper” (`artifacts/reports/small-change-lane-design-r2.md:184-186`), and unit U4b repeats that the metric is lifted without naming the new package or dependency direction (`:634`). Today landing owns the count in `metasystem/internal/landing/tierone.go:113-168`. The design has no section headed `Moved effects` and no required `| Effect | From | To | Code |` inventory.

An implementer must guess whether dispatch imports landing, landing imports dispatch, or a third package owns the metric; those choices produce different dependency graphs and leave two count implementations likely to drift. Add the mandatory moved-effects table, name the destination package/function, and state that both tier-one landing and lane admission call that one owner. Once an enrolled binary exists, the required `validate moved-effects` check must pass on the revised page.

### SCL-R2-M10 — The stated fixture verification groups do not exist

Severity: **medium**
Material: yes
Evidence level: read

Section 12 prescribes `--groups section/<bed>` while its bed column calls the beds `dispatch` and `land` (`artifacts/reports/small-change-lane-design-r2.md:643-667`). There is no `section/dispatch` group. The actual public group identifiers are `section/dispatcher-adapter-and-mission-runner-fixtures` and `section/land-fixtures` (`metasystem/testing.json:72`, `:99`; both are also present in the cadence list at `:106`). The dispatcher parent accepts only its Go-owned scenario catalog, currently defined at `metasystem/internal/proofrun/test_cost.go:20-46`; the land parent's explicit catalog is at `metasystem/scripts/agents/land-fixtures.sh:31-38`.

Literal substitution yields a non-runnable proof command, so the prior fixture finding remains open. Name the two exact group IDs, the exact catalog edits, and a command that verifies selection contains each new scenario before the beds are run.

### SCL-R2-M11 — The two-runtime proof exercises different operations and claims unequal briefs are identical

Severity: **high**
Material: yes
Evidence level: read

Section 10 calls a Codex implementation round plus a Claude critic round proof that both runtimes receive identical rendered brief bytes (`artifacts/reports/small-change-lane-design-r2.md:594-613`). They deliberately do not: the implementer uses `lane-brief.md` with `Working Mode: implement`, while the critic uses `lane-review-brief.md` with `Working Mode: review` and different questions (`:268-286`, `:297-308`). More importantly, each runtime exercises only one role. That does not prove that both adapters preserve lane field handling, generated-brief semantics, refusals, and closure for the same operation, which is the still-binding runtime-independence part of DONE (`metasystem/plans/goals/small-change-lane.md:8`).

Keep the fake end-to-end bed, but add paired same-role evidence: render and dispatch an equivalent lane implementer through both Codex and Claude (and, if closure is claimed runtime-neutral, an equivalent critic through both), then compare normalized engine-owned records, admitted brief semantics, refusal codes, and closure outcomes. Hash equality may be claimed only for the same template and input record.

### SCL-R2-M12 — DONE (4) records wall elapsed time and asks a receipt to contain its own commit hash

Severity: **high**
Material: yes
Evidence level: read

Section 9 defines `seat-min` as the difference between the goal claim timestamp and the landing commit's author timestamp (`artifacts/reports/small-change-lane-design-r2.md:566-592`). That is wall elapsed time, not the required “seat time”; it includes delegate execution, queueing, and unrelated pauses. It also conflicts with Wido's binding clock rule: time-sensitive behavior and witnesses use injected clocks, never an ambient wall-time inference. The design names no owner for accumulating active seat minutes.

The proposed RECEIPT note additionally contains `landed=<commit>` (`artifacts/reports/small-change-lane-design-r2.md:526-530`) while R-117 requires that same receipt line to be in that commit. A Git commit hash incorporates the receipt bytes, so the line cannot contain the hash of the commit that contains the line. Adding the line after landing, as the page currently says, instead violates the same-commit rule and is already caught by `SCL-R2-M08`.

Define an injected-clock, active-seat-time record that is complete before landing; put claim coordinate, active minutes, delegate-round count, and candidate tree or landing operation identifier in the same-commit receipt. Record the resulting commit hash only in the post-landing goal conclusion. The named specimen candidate is also ineligible because `records/narrator-digest.log` is a record path rejected by `LANE_RECORD_PATH` (`artifacts/reports/small-change-lane-design-r2.md:212-214`, `:571-575`); name an eligible real specimen or explicitly wait for the first one rather than presenting this file as a candidate.

## Rigor classification

| findingId | rigorClass | facts | reopeningTrigger |
| --- | --- | --- | --- |
| `SCL-R2-M01` | `severe` | `{"local":true,"recoverable":true,"proofBoundaryCrossed":true,"authorityBoundaryCrossed":true,"secretsBoundaryCrossed":false,"irreversibleDataBoundaryCrossed":false,"externalSideEffectBoundaryCrossed":false}` | Reopen until a computed-diff witness refuses a path absent from the goal revision's canonical `Paths:` list. |
| `SCL-R2-M02` | `severe` | `{"local":true,"recoverable":true,"proofBoundaryCrossed":true,"authorityBoundaryCrossed":false,"secretsBoundaryCrossed":false,"irreversibleDataBoundaryCrossed":false,"externalSideEffectBoundaryCrossed":false}` | Reopen until every current production ENGINE-policy reader is enumerated by one tested scope owner. |
| `SCL-R2-M03` | `severe` | `{"local":true,"recoverable":true,"proofBoundaryCrossed":true,"authorityBoundaryCrossed":false,"secretsBoundaryCrossed":false,"irreversibleDataBoundaryCrossed":false,"externalSideEffectBoundaryCrossed":false}` | Reopen until an existing non-protected risk-raising surface deterministically yields `LANE_DEPTH` without relying on an empty error plan. |
| `SCL-R2-M04` | `severe` | `{"local":true,"recoverable":true,"proofBoundaryCrossed":false,"authorityBoundaryCrossed":true,"secretsBoundaryCrossed":false,"irreversibleDataBoundaryCrossed":false,"externalSideEffectBoundaryCrossed":false}` | Reopen until admission implements only the decided tier, box, and rule-scope predicates. |
| `SCL-R2-M05` | `unproven` | `{"local":true,"recoverable":true,"proofBoundaryCrossed":false,"authorityBoundaryCrossed":false,"secretsBoundaryCrossed":false,"irreversibleDataBoundaryCrossed":false,"externalSideEffectBoundaryCrossed":false}` | Reopen until the critic root closes without another critic and its closed reference then closes the work root. |
| `SCL-R2-M06` | `unproven` | `{"local":true,"recoverable":true,"proofBoundaryCrossed":false,"authorityBoundaryCrossed":false,"secretsBoundaryCrossed":false,"irreversibleDataBoundaryCrossed":false,"externalSideEffectBoundaryCrossed":false}` | Reopen until the same frozen lane root refuses an implementer follow-up before overflow and admits one after validated overflow. |
| `SCL-R2-M07` | `unproven` | `{"local":true,"recoverable":true,"proofBoundaryCrossed":false,"authorityBoundaryCrossed":false,"secretsBoundaryCrossed":false,"irreversibleDataBoundaryCrossed":false,"externalSideEffectBoundaryCrossed":false}` | Reopen until the exact zero-findings join inputs and artifact owner are named and accepted by the existing command. |
| `SCL-R2-M08` | `severe` | `{"local":false,"recoverable":true,"proofBoundaryCrossed":true,"authorityBoundaryCrossed":false,"secretsBoundaryCrossed":false,"irreversibleDataBoundaryCrossed":false,"externalSideEffectBoundaryCrossed":false}` | Reopen until one end-to-end witness carries the exact reviewed candidate and pre-created receipt into one successful landing commit. |
| `MOVED-EFFECTS-SCL-R2-M09` | `unproven` | `{"local":true,"recoverable":true,"proofBoundaryCrossed":false,"authorityBoundaryCrossed":false,"secretsBoundaryCrossed":false,"irreversibleDataBoundaryCrossed":false,"externalSideEffectBoundaryCrossed":false}` | Reopen until the moved-effects inventory names one metric owner and the validator passes. |
| `SCL-R2-M10` | `severe` | `{"local":true,"recoverable":true,"proofBoundaryCrossed":true,"authorityBoundaryCrossed":false,"secretsBoundaryCrossed":false,"irreversibleDataBoundaryCrossed":false,"externalSideEffectBoundaryCrossed":false}` | Reopen until exact public group IDs and scenario-catalog edits make every named witness selectable. |
| `SCL-R2-M11` | `severe` | `{"local":true,"recoverable":true,"proofBoundaryCrossed":true,"authorityBoundaryCrossed":false,"secretsBoundaryCrossed":false,"irreversibleDataBoundaryCrossed":false,"externalSideEffectBoundaryCrossed":false}` | Reopen until Codex and Claude each exercise equivalent lane operations and their normalized engine outcomes agree. |
| `SCL-R2-M12` | `severe` | `{"local":false,"recoverable":true,"proofBoundaryCrossed":true,"authorityBoundaryCrossed":false,"secretsBoundaryCrossed":false,"irreversibleDataBoundaryCrossed":false,"externalSideEffectBoundaryCrossed":false}` | Reopen until active seat time is recorded from an injected clock before landing and the same-commit receipt contains no self-referential commit hash. |

## Open-question recommendations

1. **Ceiling:** current conformance does not enforce the brief's `Ceiling`; keep `LANE_LINES` as the authoritative eighty-line check and add the explicit goal-`Paths:` comparison required by `SCL-R2-M01` rather than claiming existing brief-bound enforcement.
2. **Fixture ownership:** add dispatcher scenarios to `fixtureScenarioCatalog["dispatcher"]` in `metasystem/internal/proofrun/test_cost.go:25-27`, add land scenarios to the parent list in `metasystem/scripts/agents/land-fixtures.sh:31-38`, and run the exact groups `section/dispatcher-adapter-and-mission-runner-fixtures` and `section/land-fixtures`.

VERDICT: rework
