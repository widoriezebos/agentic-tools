# Codex gpt-5.6-sol read of plans/critique-closes-on-folded-proof-design.md revision 1 (2026-09-13, plugin task task-mtyyr3vw-815uww, read-only)

# Adversarial critique — revision 1

**Verdict:** Revision 1 is not safe to implement as written. It has eight material gaps. The largest allow a certification to bind more change than its evidence proves, or let a “prior clean read” bypass review without preserving the exact reviewed subject.

## Findings, sorted by materiality

1. **Critical — The boundary is path-based, not change-based.**  
   **Attacks:** Decisions 1–2 and proofs 1–2 in the [design](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/plans/critique-closes-on-folded-proof-design.md:28).  
   **Scenario:** A finding names `internal/dispatch/finding_register.go`; the fold fixes its condition and also changes unrelated close policy in that file. `changedPaths ⊆ boundary` passes. A hand-edited certification can likewise widen `boundary` and update `correctionDiff`/`changedPaths`; recomputing the diff does not prove the boundary’s provenance. Directory artifacts are undefined—literal comparison refuses them, while prefix expansion authorizes a subtree. Rename syntax `old=>new` does not match either changed path.  
   **Material:** Yes—changes the certification schema and validators in [finding_register.go](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/internal/dispatch/finding_register.go:25) and every downstream gate.  
   **Amendment:** Derive the boundary from the exact certified finding set and an owner-written resolution manifest; never trust a stored boundary. Expand `NEW path` and renames to concrete endpoints, reject directory/prefix artifacts, and require every changed hunk—not merely every path—to be attributed to one certified correction. Unattributed hunks require critique.

2. **Critical — The evidence model cannot represent the existing proof contract.**  
   **Attacks:** `CERTIFY_NO_TEST`, `CERTIFY_EVIDENCE_TREE`, Decision 1.  
   **Scenario:** `finalTree` is the project subtree produced by conformance, while schema-2 `candidateTree` is the whole repository tree. Legitimate evidence therefore mismatches, or an implementation weakens one side’s meaning. A single `attemptId` cannot represent joined group results and reused attempts. A bespoke validator may also miss the attempt-owner checks already performed by landing. The register contains no test/test-artifact mapping, and the proposed CLI supplies none.  
   **Material:** Yes—changes [finding_register.go](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/internal/dispatch/finding_register.go:60) and requires reuse/extraction of the owner validation behind [landing/testing.go](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/internal/landing/testing.go:22).  
   **Amendment:** Record both `finalWholeTree` and its verified project-tree projection. Accept sorted `attemptIds` plus exact group execution/reuse identities, using the existing schema-2 owner validator. Add a strict resolution manifest mapping every finding to test identity, test artifact, evidence group, and correction.

3. **Critical — “Restoration” is neither a safe nor complete reviewed-subject equivalence.**  
   **Attacks:** Decision 3 and slice 2.  
   **Scenario:** Reversing a subset of previously approved hunks removes an approved fix and produces a tree the critic never read. Conversely, exact-hunk comparison refuses a legitimate return to the identical clean subject after context movement, rename representation, or an intermediate work round. Tree equality alone also conflates live project-worktree reviews with whole commit subjects having different parents/diffs. A zero-finding return is not necessarily a clean folded register. Current hazard validation additionally requires `reviews == final work job` and critic timing after that job, so a legitimate prior read still fails.  
   **Material:** Yes—changes [review_reference.go](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/internal/dispatch/review_reference.go:84), [hazard.go](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/internal/dispatch/hazard.go:328), conformance, and landing.  
   **Amendment:** Delete subset-restoration equivalence. Define typed subject identities including kind, implementation root, base/parent tree, reviewed tree, and diff digest; never equate live and commit subjects implicitly. Refuse only when the terminal subject exactly matches a prior clean subject and its folded register had no open/disputed findings. Persist that closure binding and make all gates consume it.

4. **High — Certification is incomplete as a state transition and does not preserve the two bars.**  
   **Attacks:** Decisions 1–2, hazard/landing rows, and the declared out-of-scope boundary.  
   **Scenario:** One bounded finding is certified while another is later auto-deferred; the register has no open findings and landing selects the certificate although not every material finding was proved. A crash between publishing the immutable record and changing the register wedges retries or leaves half-state. “Certified” is alternately described as a status and a resolution. Replacing `chainCertifiedOutput` with `finalTree` can also discard landing’s existing patch replay/post-image check after a base move.  
   **Material:** Yes—changes [finding_register.go](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/internal/dispatch/finding_register.go:453), [observe.go](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/internal/landing/observe.go:539), and bar (a) in [two-bars-for-changes-design.md](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/plans/two-bars-for-changes-design.md:206).  
   **Amendment:** Make `status=resolved, resolution=certified` explicit; require exact equality with all eligible unresolved findings; implement idempotent crash recovery under the register lock; mirror and revalidate the record. Amend bar (a) explicitly to require either an exact critic-read subject or a verified certificate. Use certificate `finalTree` only to select the terminal review patch, retaining `bindCertifiedChange`.

5. **High — Test-only certification can prove nothing about the reported defect.**  
   **Attacks:** Decision 8.  
   **Scenario:** Production remains unchanged while a new trivial passing test—or a weakened existing test—is cited for a substantive production finding. Every proposed mechanical condition passes.  
   **Material:** Yes—changes the certificate admission policy in `internal/dispatch/finding_register.go`.  
   **Amendment:** Permit test-only folds only for an explicit `proof-gap` resolution kind and additive test artifacts. Otherwise require the production artifact to change. Evidence must execute the exact immutable test inputs recorded by the certificate.

6. **High — Decision 4 lacks the machinery required by the existing budget law.**  
   **Attacks:** Decision 4 and slice 3.  
   **Scenario:** The current close operation defers every bounded finding at exhaustion; there is no mechanical-grain field, stored falling trajectory, fixture obligation, or evidence-bound obligation discharge. `critique-budget-rebind` can restore a tier-3 limit of three.  
   **Material:** Yes—changes [critique/model.go](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/internal/critique/model.go:10), [finding_register.go](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/internal/dispatch/finding_register.go:488), budget rebind, and the [design-critique skill](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/skills/design-critique/SKILL.md).  
   **Amendment:** Add an independent finding-grain field, canonical post-fold trajectory, one-to-one fixture/evidence obligations, mandatory code critique, and header evidence. Cap design-critic roots in the common budget owner and rebind path. Two rounds are compatible with R-42 and R-97; explicitly state that R-97 supersedes R-60 for this role and that tier eligibility remains unchanged.

7. **Medium — The three metrics are not all computable from existing records.**  
   **Attacks:** Decision 7.  
   **Scenario:** `rounds_per_landing` is recoverable from existing `Landing-Provenance: chain=` trailers plus job relationships, although the current loader ignores them. `redundant_reads` lacks normalized review subject/fold-state data. `reads_refused` is impossible: refusal occurs before a successor record and no event is persisted. Current `critique_rounds` is goal-level and counts follow-ups, not all rounds per landing.  
   **Material:** Yes—changes [metrics/data.go](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/internal/metrics/data.go:52), [compute.go](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/internal/metrics/compute.go:207), and dispatch telemetry.  
   **Amendment:** Persist a pre-reservation refusal event containing chain, typed subject, prior round, reason, and timestamp; load landing provenance, reviews, subject identity, and fold outcome explicitly.

8. **Medium — The slices do not land independently.**  
   **Attacks:** Section 10 and the goal’s [DONE/Absorbed contract](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/plans/goals/critique-closes-on-folded-proof.md:10), whose Absorbed clauses are themselves textually truncated.  
   **Scenario:** Slice 1 lacks proof-owner extraction, transactional durability, and bar-(a) amendment. Slice 2 lacks the cross-gate closure record. Slice 3 lacks grain/trajectory schema. Slice 4 expects telemetry earlier slices never emit.  
   **Material:** Yes—changes the implementation plan and acceptance artifact.  
   **Amendment:** First land backward-compatible schemas, subject identity, proof validation, and event records; then certification plus all four gates/bar (a); then prior-read closure plus all gates; then round-two policy; finally metrics. Repair the goal clauses before using them as acceptance criteria.

## Decisions sound without amendment

- Severe and unproven findings may never be certified.
- A certification is not counted as a critic read.
- Consumers must recompute tree differences and fail closed.
- Zero material findings stop critique; no fourth round is consistent with R-42.
- Refusing a redundant read before creating a successor record is operationally sound, provided the refusal event is persisted.
- Keeping the existing landing patch-replay/post-image binding is the correct mechanism for safe base movement.

Evidence: read-only source trace of every requested artifact; no files modified and no tests run.

`RECEIPT|type=design-critique|goal=critique-closes-on-folded-proof|outcome=material-findings|verify=read-only-source-trace|corrections=0|stop_loss=no|skills=design-critique|delegate=none|built_by=coordinator`
