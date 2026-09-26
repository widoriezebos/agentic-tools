# Critique r2 (final): Examine existing evidence before repeating tests

- Kind: design critique, round 2 of 2, failsafe final round of the same Fable review session as round 1
- Design: `metasystem/plans/designs/examine-evidence-before-retesting.md` (id 01M3E9ME5GK7J1Z6ZAM22WEY05, status draft, resolved again by `project design-of --goal examine-evidence-before-retesting`)
- Design SHA256 read: `a39d39ffa3b58895b5a99292d74e4b8582634e2dd8509f96b1d2b3f7ada466cb` (307 lines; round 1 read `71da354b…`)
- Dispositions read: `metasystem/plans/examine-evidence-before-retesting-dispositions.md` (57 lines)
- Model/session: Claude Fable 5.1 (`claude-fable-5-1`), the same session the dispositions record as `89620448-ca6f-48b9-93b0-e18bdcabb9e8`; not the design's author
- Date: 2026-09-26

## Verdict

**0 MATERIAL findings.** All three round-1 defects are dispositioned in the revised design with contracts that an implementer can build step 1 from without a first-use failure, unauthorized authority, data loss or silent false answer. One source fact in the revision is false and needs a one-line correction (ER-C4 below); it is not material because no implementer can build on a source that does not exist, so it forces a choice rather than a wrong build. Everything else is a fixture-expressible obligation or deferred.

Criteria applied verbatim: *Would an implementer working from this design build step 1 DIFFERENT, or WRONG, because of this finding? Does step 1 WORK, and is it SAFE, without this finding?*

## Disposition of the round-1 findings

### ER-C1 (closure deadlock or builder-closed examination): RESOLVED

- **Revised contract:** lines 147, 150-162 and 79. Completion is the authenticated `code-critic` job ending `completed` with a schema-valid decision return bound to the evidence subject; the review owner collects it; `ReturnBindsSubject` and the reference validator gain an evidence-kind branch that checks fresh critic identity, return digest and a complete decision map; no author-folded register, no `close --dispositions`; ordinary code/design closure untouched; `execute` entries are not dismissible findings.
- **Check against source:** the deadlock came from `ReadClosedClosure` demanding a folded register and `chainClosed`, which only `close --dispositions` writes. The revision no longer routes the evidence reference through that path, and it does not fabricate an ordinary closure, so it neither blocks nor lets the builder close. The critic-only exemption in `validateHazardCompletion` (hazard.go:248) means the examiner chain itself owes no further critique. Shape is correct.
- **Obligations (fixture-expressible, not material):** the evidence branch must require `dispatchMode` fresh, no `parentJob`, no `resumedSessionId` and a session absent from any construction chain, exactly as `validateIndependentCritiqueReference` does today; the decision map must be keyed by the subject's required group/component set with no missing and no extra entries; the "review-owner completion reference" is a pointer that verification re-derives from the job record and return digest, never a value it trusts. Test: a return with one component absent, or a completion record naming a job whose return digest differs, refuses.

### ER-C2 (contradiction granularity and the failed parent): RESOLVED

- **Revised contract:** lines 180-193 and 254-261. Contradiction is evaluated at the cited granularity within the parent's relevant execution context; the enclosing attempt's later failure does not contradict its completed component; a newer failed or incomplete observation of that component, ambiguous ordering, a live producer for the parent, or a newer opaque failed parent blocks; a newer detailed parent constrains only its named components and parent checks; recheck after the snapshot before acceptance; partial runs carry aggregate `partial`, never `pass`; the trusted parent publishes after reaper and cleanup via its named-selection entry.
- **Check against source:** both round-1 failure directions are closed: ER-4 is reachable (own parent not a veto) and legacy opacity still blocks (ER-2 has no silent pass). `parseSectionResult` (test_section.go) maps any unknown status to `invalid`, so an un-updated parser fails closed on `partial`. The parent runner's named selection already prints the diagnostic-selection line and reaps process groups (fixture-bed-scenarios.sh:243-250, 307).
- **Obligations:** `ValidateTestResult` and `RecomputeDelivery` accept an `examined-reuse` group only when its component map is complete and every `execute` decision has a joined fresh result from the current attempt. Test: a composed result citing nine components with the tenth neither cited nor freshly executed is insufficient; a `partial` section line is never a group pass.
- **Deferred hardening:** a newer failure of the same component at a third identity (an intermediate candidate) is in the examiner's frozen subject but is not a mechanical block. That is the design's declared judgment boundary, not a shape defect.

### ER-C3 (coverage identity): RESOLVED on its substantive terms

- **Revised contract:** lines 214-239. A versioned `CoverageCodeIdentity` (Go source and test trees with testdata and embedded assets, go.mod/go.sum, dependency and toolchain identities, flags, native and package inventories, ratchet, platform) built from the engine-projection and discovery owners, explicitly not `FullDigest`; a changed projection always reruns the full producer; an equal projection with an outside delta goes to the examiner, whose default on doubt is fresh coverage; values remain the old producer's observations.
- **Evaluation:** the disposition rejects treating the projection as sufficient because Go tests can read external files. That is a coherent and safer split than round 1's exact-only remedy: the deterministic guard is mechanical and necessary, the residual is bounded judgment with a rerun default, and no fresh percentage or lowered ratchet can appear because the ratchet digest sits inside the projection. The ER-4 residual is now achievable: no `go:embed` in the tree pulls `scripts/` into the engine (only the UI bundle, the partner skill, its vocabulary and the behaviour policy are embedded), so a scripts-only correction leaves the identity equal.
- **Obligation (fixture-expressible):** an `applicable` coverage decision must enumerate the examined outside-projection paths, and the verifier refuses when that set does not equal the recorded old/current delta, so a model omission becomes a refusal rather than an undisclosed gap. Test: delta of two paths, decision lists one, verification refuses naming the missing path.

## Required minimal correction, not material

### ER-C4: the named bound source for the examination does not exist

- **Design contract:** lines 128-130, "The bound comes from the existing code-review dispatch configuration and approved goal budget; both entry points resolve it identically."
- **Source fact:** no configured reader tool-call bound exists. `reviewBriefFacts` (intent_delivery.go:508-527) takes the count only from the caller's `--tool-calls`; `build` takes it from the brief line `Maximum reader tool calls: N` or `--read-tool-calls` (intent_work.go:45-47, 162); `metasystem.conf` has `role.code-critic.runtime`, `role.code-critic.model.<runtime>` and `dispatch.permissions.code-critic` and no tool-call key. The approved goal budget carries `ReviewRoundLimit` but no call count.
- **Why not material:** an implementer cannot build on an absent source; the shape (identical resolution by both verbs, no caller knob on `test`, refusal before a model starts) is right. Built literally, `review evidence` and `test` would refuse every examination with the existing "none is configured" remedy, which is visible, not silent.
- **Smallest correction:** name the source: a `role.code-critic.*` configuration value (or the hazard-class obligations) that the review owner writes into the brief's `Maximum reader tool calls` line for the evidence kind. Test: with the value absent both verbs refuse before dispatch naming it; with it present the generated brief carries the line and `test` passes no flag.

## Deferred (generality, not step 1)

- Line 127-128, reusing an "already planned" code review as the examiner: `ReadSubject` has one kind per return, so one job cannot bind a commit subject and an evidence subject under `ReturnBindsSubject` as written. Either define a compound return or drop the sentence; dispatching a separate examination works today.
- Whether examination runs before or after test-attempt admission (test.go:1899-1921): after admission a waiting examiner holds a live attempt that blocks other seats' reuse; before admission it holds nothing. Both work; say which.
- The intermediate-identity component failure noted under ER-C2.

## Actual reads and limits

Read in full: the revised design; the dispositions. Source checked this round: greps for tool-call bound sources across `cmd/metasystem`, `internal/dispatch`, `metasystem.conf` and the review-brief template; `go:embed` directives across the tree. Round-1 reads of `test_result.go`, `coverage.go`, `test_section.go`, `closure.go`, `hazard.go`, `intent_delivery.go`, `adopt-fixtures.sh` and `fixture-bed-scenarios.sh` were relied on without re-reading. Not read: `test_build.go` section adapter internals, `finding_register.go`, `landing/testing.go`, `testpolicy/select.go`, the engine-projection owner's path list. No tests run, no goal, receipt, configuration or design file modified, `metasystem.conf.local` not read. Tool calls used: 4 of 12. A clean critique is not human acceptance or implementation proof; the design remains draft.
