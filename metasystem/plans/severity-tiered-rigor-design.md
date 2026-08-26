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
