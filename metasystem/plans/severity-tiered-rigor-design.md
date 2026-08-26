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
