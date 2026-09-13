# Codex gpt-5.6-sol read of plans/critique-closes-on-folded-proof-design.md revision 3 (2026-09-13, plugin task task-mtzlg4r1-7q0ccy, read-only)

# Adversarial design critique — round 3

**Verdict: not ready to build.** Revision 3 has **eight material findings**. The narrowing legitimately removes the certification findings, and schema-v4 normalization is safely conservative, but the kept mechanisms are not yet closed. **Slice 0 is not behaviour-free; neither slice 0 nor slice 1 is ready to build.** Slices 2 and 3 also depend on omitted contracts.

## Findings, sorted by materiality

### 1. Critical — One closure object reintroduces critic shopping

**Section:** §3, proof 3, slice 1.

**Scenario:** Two code-critic chains launch concurrently on the same subject before either has folded. Critic A returns clean; critic B reports an invariant defect. A writes the closure, or whichever closes last overwrites it; fresh dispatch also overwrites `independentCritiqueJobRef`. Gates selecting one closure can ignore B. This regresses current `mergeCritique`, which examines all critic roots reviewing the implementation and unions same-tree classifications ([conformance.go](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/internal/validate/conformance.go:1016)).

**Material:** Yes—changes closure admission in `finding_register.go`, `review_reference.go`, hazard validation, conformance, and landing.

**Amendment:** Atomically reserve one critic chain per implementation-root/subject before launch, or require closure to join every already-dispatched chain on that subject. Write closure and stamp through one compare-and-set transaction; an existing different closure must refuse, never overwrite. Give every gate one shared closure validator.

### 2. Critical — Fixture obligations remain assertions, not proof

**Section:** §4, proof 4, slice 2; contradicts R-97-m1e and D81.

**Scenario:** Round 2 defers a mechanical finding naming fixture X. Today `goal discharge-review-obligation` merely matches finding plus chain, accepts any non-empty `--test`, and overwrites the stored test text ([verbs.go](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/internal/goal/verbs.go:1728)). The obligation can therefore be discharged before X runs and without joining the implementation’s mandatory code-critic closure. “Critique-always” independently requires critique, but does not prove that critique or test discharged this finding.

**Material:** Yes—changes `internal/goal/file.go`, `internal/goal/verbs.go`, and the proof boundary.

**Amendment:** Preserve the expected fixture immutably and add separate discharge evidence. Admission must verify the implementation chain, changed artifact, exact retained test result, and clean code-critic closure before discharging the obligation.

### 3. High — Persisting a pre-launch subject does not bind what the critic reads

**Section:** §3, proof 1.

**Scenario:** The engine records subject S, launches a critic against a mutable worktree, and that worktree is rebased or otherwise moved afterward. The critic reads R while its return references S. The checkout guard covers setup, not the critic’s lifetime; the subject therefore proves intended input, not observed input.

**Material:** Yes—changes `dispatch.sh`, subject construction, and review artifacts.

**Amendment:** Give the critic an immutable detached snapshot/materialization of the subject, or verify the readable workspace against S immediately before and after the critic. A returned reference must be engine-injected and validated, not model-asserted.

### 4. High — The prior-read rule fails two promised cases

**Section:** §§3, 4 and 7; contradicts R-60-m1.

**Scenario:** A no-change implementer follow-up has a different `reviewedMember`; because equality includes every field, it cannot equal the earlier subject despite §7 promising closure without another read. Separately, a clean first design read should stop immediately, but §3 writes closure only on an “implementation root”; no such root exists for a standalone design critique, and §4 defines only round-2 closure.

**Material:** Yes—changes subject equality and close routing in `finding_register.go` and `dispatch.sh`.

**Amendment:** Separate provenance from semantic equality: retain `reviewedMember` as provenance but exclude it from the equality key. Define role-specific closure ownership, including an explicit design-chain round-1-clean transition.

### 5. High — Slice 0 activates protocol behaviour

**Section:** §9.

**Scenario:** Slice 0 writes subjects into `review.json` and returns. Current critic returns are strict version 4, so they cannot carry that field; version 5 is declared dormant until slice 2. Activating version 5 earlier also requires its grain fields. Persisted artifact bytes and materialized-schema behaviour change either way.

**Material:** Yes—changes slice boundaries and return compatibility artifacts.

**Amendment:** Make slice 0 readers/types only: accept absent subjects, events and closure, but emit nothing. Activate engine-written subject artifacts in slice 1. Keep subjects out of model returns, or define a separate engine-owned envelope. Activate schema 5 and role/adapter text atomically in slice 2.

### 6. High — “Mirror learns the file” does not make refusal events durable

**Section:** §§3, 5 and slice 1.

**Scenario:** `REDUNDANT_READ` occurs before a successor record exists. Adding `reads-refused.jsonl` to `mirrorSources` is ineffective unless another job is subsequently mirrored; an already-closed implementation may never trigger that call. The event remains local and disappears with checkout cleanup. Mirroring synchronously under the global finding-register lock would also hold register advancement behind external evidence I/O.

**Material:** Yes—changes `mirror.go` and the refusal transaction.

**Amendment:** Append and fsync under the register lock, release it, then run a dedicated idempotent mirror transaction keyed by event ID. Define mirror-failure behaviour while preserving the read refusal.

### 7. Medium — Trajectory and tier enumeration are still incomplete

**Section:** §4, proof 4.

**Scenario:** `materialByRound` is undefined: raw material count, post-artifact-demotion count, or unresolved-register size can produce different “falling” verdicts. Goal-free design critics are also omitted, so `goalReviewRoundLimit` and rebind can remain at the global ceiling of three.

**Material:** Yes—changes `finding_register.go` and role-aware limit resolution in `build.go`.

**Amendment:** Define the trajectory as distinct findings admitted after schema normalization and artifact demotion for each folded round. Enumerate tier 1/2 refusal, tier 3 limit 2, and an explicit goal-free limit; apply the same rule in build, fallback accounting, and rebind.

### 8. Medium — Carried landings do not expose their original chain

**Section:** §5, proof 5, slices 0/3.

**Scenario:** Ordinary provenance carries `chain=<root>`, but carried provenance is `carried opid=... past=...`; neither it nor the verdict trailer identifies the original chain ([carried.go](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/internal/landing/carried.go:262)). The metrics loader cannot report carried landings “with their original chain” from the named trailers.

**Material:** Yes—changes the metric’s source contract and possibly landing trailers.

**Amendment:** Add an immutable original-chain field/trailer to carried records, or report carried attribution as unavailable. Do not infer it from `past`.

## Attacks that do not produce findings

- A version-4 mechanical-looking finding becoming invariant is the correct conservative normalization. Escalation is intentional because v4 contains no authenticated grain/fixture claim.
- A two-round cap is compatible with R-42-m0’s ceiling and legitimately narrows R-60-m1 under R-97-m1e.
- The global finding-register lock can serialize event append against register advance; the missing contract is what happens after append and outside that lock.

Evidence: read-only source trace; no tests run and no files modified. I applied the design-critique skill’s artifact-based materiality test.

`RECEIPT|type=design-critique|goal=critique-closes-on-folded-proof|outcome=8-material-findings|verify=read-only-source-trace|corrections=0|stop_loss=no|skills=design-critique|delegate=none|built_by=coordinator`
