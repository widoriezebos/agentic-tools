# Dispositions: kill-shell plan, round 11

Job: design-critic-20260812t004031z-0755 (codex gpt-5.6-sol, xhigh).
4 findings, 4 material, all accepted.

| id | disposition |
| --- | --- |
| KS-R11-001 | accepted (critical) — containment is stated as COHERENCE BY PAIRING, which is what adoption already does: exported scripts and the engine travel only together, through an adoption run; the program never ships them separately, so every adopted target holds a coherent shim-engine pair from its adoption date. The definition of done gains the clause: the program cannot close until the engine-delivery ruling is made OR the adopted payload explicitly remains the last coherent pair, recorded in the migration notes. |
| KS-R11-002 | accepted — the publication critical section REVALIDATES freshness under the lock: after claiming, the publisher re-derives the tracked-source state and aborts unless it still equals the stamp; a binary built from a state that moved is never published. |
| KS-R11-003 | accepted — a real latent defect in the shipped code, recorded as a Phase E implementation item: gate markers must be written temp-then-rename so pruners can never observe partial JSON and eat a live registration; today's direct write is the bug the ordered protocol would have tripped on. |
| KS-R11-004 | accepted — the publication critical section honors the owner-lock's ownership condition by construction: the claiming process carries the generated publication tag in its own argv for the whole section (the shipped dispatch pattern), so holder classification can always re-derive the owner. |
