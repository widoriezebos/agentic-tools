# Codex gpt-5.6-sol read of plans/critique-closes-on-folded-proof-design.md revision 2 (2026-09-13, plugin task task-mtyzghk0-f3ojfx, read-only)

# Adversarial design critique — round 2

**Verdict: not ready to build.** Revision 2 still has eight material findings. Several round-1 defects were narrowed, but none was completely closed: R1-F1/F2/F3/F4/F6/F7 are partial folds, F5 is largely renamed as `proof-gap`, and F8 remains open. Because these are contract-shape/invariant findings at round two, the design’s own R-97 rule requires human escalation, not slice 0 or an automatic third round.

## Findings, sorted by materiality

### 1. Critical — “Hunk” is not a complete or stable certification unit

**Attacks:** §§3–4, proof rows 1–3.

**Scenario:** Text hunks do not exist for binary changes or some mode-only changes. Whole-file deletion collapses unrelated content into one coarse hunk. One hunk can implement two findings but exact-once attribution forbids that. Whitespace-only production edits satisfy `correction`. Context changes alter a textual hunk digest despite the same logical edit. Worse, a correction entry’s `artifact` is the production file, while its added test hunk is in another file; `test` has no file field, yet every hunk must be claimed by an entry whose artifact equals the hunk file. Ordinary additions also contain context, so “old range is empty” generally holds only for a new file.

**Material:** Yes—changes `certification.json`, the diff parser, `finding_register.go`, and every gate.

**Amendment:** Replace hunks with canonical change units keyed by repository-relative endpoint, kind (`text`, `binary`, `mode`, `add`, `delete`), before/after mode and blob OIDs, and exact edit bytes. Permit one unit to discharge an explicit set of finding IDs while retaining unique unit ownership. Pin identity to one immutable tree pair; changed context or base means recomputation/refusal. Add fixtures for all named edge cases.

### 2. Critical — Passing evidence still does not prove the finding

**Attacks:** §3 “Kinds/Evidence”, §10 rows 1 and 3.

**Scenario:** `proof-gap` accepts `func TestX(t *testing.T) {}`. A correction can make a whitespace edit and cite an old passing test. A new test can be added beside a weakened test in another unit. The attempt-owner validator proves authentic execution and reuse ownership, not assertions or causal relevance. The design also calls the input a “schema-2 test result”; currently schema 2 is the committed receipt wrapping a schema-1 `proofrun.TestResult`, and legitimate reused groups may be owned by a terminal attempt whose aggregate result failed.

**Material:** Yes—changes the proof contract and the extracted proof validator.

**Amendment:** Bind each finding to a committed test receipt, exact group owner, test-file unit and observed native test identity. For corrections, require a counterfactual run where test changes are present but the production correction is absent to fail, followed by the final-tree pass. For proof gaps, require an explicit negative/mutation witness—or weaken the claim from “proof” to “test executed.”

### 3. Critical — The universal read subject cannot represent actual critics

**Attacks:** §5 and slice 0.

**Scenario:** A design critic has no implementation root or implementer `review.json`; it reads a design artifact at a commit. A live code follow-up is rebased before dispatch, while its inherited `reviews` still names the old implementation member. Conformance recomputes the boundary base from the then-current target. Consequently `implementerRoot/baseTree/reviewedTree/diffDigest` has no single defined source after a moved boundary or follow-up.

**Material:** Yes—changes `review.json`, critic returns, `dispatch.sh`, conformance and equality logic.

**Amendment:** Use role-discriminated subjects: design `{designPath, blob/content digest, reviewedCommit, declaredOutputsDigest}`; live code `{implementerRoot, reviewedMember, boundaryBaseProjectTree, reviewedProjectTree, immutableDiffDigest}`; commit `{commit, selectedParent, tree, diffDigest}`. Persist the engine-computed subject atomically before launch, after any rebase; never trust the critic to reconstruct it.

### 4. Critical — Closure has two competing authorities

**Attacks:** §§4–5 and bar (a).

**Scenario:** Existing consumers dereference `independentCritiqueJobRef`, which is stamped at fresh dispatch and validated by `reviews`, timing and the stamped member’s return. Revision 2 adds `closedOnRead` pointing to a possibly different follow-up round and says consumers accept “whichever round the stamp names.” Certification then invokes existing reconcile, which may reject the pre-fold critic it is intended to legitimize. Hazard, merge and landing can therefore select different reviews. Bar (a)’s “certification binds to such a read” is also ambiguous: the source read contained findings and was not clean.

**Material:** Yes—changes `review_reference.go`, `hazard.go`, conformance, landing and [bar (a)](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/plans/two-bars-for-changes-design.md:195).

**Amendment:** Create one canonical closure object on the implementation root: critic root, qualifying member/round, typed subject, and mechanism `clean|certified`. Keep `independentCritiqueJobRef` only as an index that must equal its critic root. One shared validator must serve every gate. State bar (a) as clean exact subject **or** bounded-findings certification, not certification to a clean read.

### 5. High — Crash recovery and register binding are not idempotent

**Attacks:** §3 “Coverage and transition”, §4.

**Scenario:** A retry cannot produce byte-identical content because `certifiedAt` is regenerated. `registerDigest` is defined over the pre-transition register, while merge compares it with a post-transition register “with entries excluded”; those are not a defined common projection. A global `artifacts/agents/finding-register.lock` exists today, followed by a critic-root record lock, but certification also writes the implementer payload and root stamp; the global lock does not serialize all those owners.

**Material:** Yes—changes the certificate writer, register transition and record locking.

**Amendment:** Define a deterministic immutable core digest excluding publication metadata; retries validate that core and reuse the original timestamp. Specify the exact register projection. Publish through a recovery journal with a fixed lock order—global register, critic root, implementer root—or prove equivalent staged recovery.

### 6. High — Round-two policy contradicts its cited rule and current schemas

**Attacks:** §6 and slice 3.

**Scenario:** R-97 and the current design-critique skill permit fixture exit only for a **falling** mechanical trajectory; §6 defers any round-two mechanical residue, while merely recording `materialByRound`. Current tier rules also reject design critics at tier 2, despite “every tier” implying otherwise. `goalReviewRoundLimit` and rebind are role-blind and can supply 3. Finally, active critic schema v4 is strict: “optional until slice 3” grain fields are impossible, while making them required rejects historical v4 returns.

**Material:** Yes—changes `build.go`, rebind, return schema, register close and the skill.

**Amendment:** Enforce round-two count lower than round one; otherwise escalate. Enumerate tier-1/tier-2 refusal, tier-3 `min(goalLimit,2)`, and goal-free behavior. Introduce schema v5 with a named validator/normalizer; missing grain on historical returns must conservatively become invariant, never mechanical.

R-97 legitimately narrows R-60, and a cap of two does not contradict R-42’s ceiling of three; the omitted falling condition is the actual contradiction.

### 7. High — Refusal and landing provenance are not durable or well scoped

**Attacks:** §§5, 7 and slice 4.

**Scenario:** `reads-refused.jsonl` is outside round directories, while current mirroring copies round evidence, records and selected landing artifacts—not this root file. No append lock, mirror owner, deduplication key or pruning rule is named. Metrics currently walk broadly qualifying main-branch commits; the design does not distinguish ordinary bar-(a) pass landings from manual commits or carried landings, whose provenance can embed the original chain despite bypassing its ordinary result.

**Material:** Yes—changes `dispatch/mirror.go`, archival cleanup, `metrics/data.go` and `compute.go`.

**Amendment:** Give events stable IDs and a locked writer; mirror them under the chain manifest and load local/mirrored copies with deduplication. Parse exact provenance and verdict trailers. Count ordinary `pass bar=a` landings; report carried landings separately. Define moved-chain checkout-segment lookup and unavailable-evidence coverage.

### 8. Medium — Slice 0 is behavioral, and later slices are not independently coherent

**Attacks:** §11.

**Scenario:** Admitting `resolved/certified` before its validator can let close logic consume forged state. Writing subjects changes persisted behavior. Strict-schema grain fields cannot be optional. If these stay dormant, slice 1 lacks usable subject/proof schemas; if activated, slice 0 is behavioral. Slice 3 consequently cannot safely interpret old critic returns.

**Material:** Yes—changes the implementation sequence and compatibility artifacts.

**Amendment:** Split dormant parsing from activation: add versioned readers and a behavior-preserving exported proof-owner wrapper first; slice 1 atomically activates certification plus all consumers; slice 2 activates subjects/closure/events; slice 3 activates v5 grain and cap; slice 4 consumes already-durable telemetry. Give every slice mixed-version fixtures.

Evidence: read-only source trace of all requested artifacts and cited rules; no files modified and no tests run.

`RECEIPT|type=design-critique|goal=critique-closes-on-folded-proof|outcome=8-material-findings|verify=read-only-source-trace|corrections=0|stop_loss=no|skills=design-critique|delegate=none|built_by=coordinator`
