# Dispositions — patience-turn-identity, critique round 2

Critic: design-critic-20260811t095647z-40f5 (codex gpt-5.6-sol, xhigh).
Verdict: 4 material. All 4 ACCEPTED; design amended in the same commit.
No structural findings — the loop stops by the diminishing-returns rule.

| # | Sev | Claim (short) | Disposition |
|---|-----|---------------|-------------|
| PTI-R2-001 | critical | Migration contradicted the annotation design and left fuse inputs undefined | ACCEPT — Migration rewritten to the annotation-line grammar; every cycle-block consumer named; annotations are audit trail, never fuse input (replay invariant). |
| PTI-R2-002 | high | Two incompatible no-witness application rules | ACCEPT — one application rule stated once: returns apply only when accepted; measurement effects always conclude; no third case. |
| PTI-R2-003 | high | Breaker transition undefined for measured rejected turns | ACCEPT — decoupled: witnessed violations increment consecutiveFailures exactly as today while the cycle books its measured classification; no-witness increments nothing. |
| PTI-R2-004 | high | Completion on a rejected return lacked an input/state contract | ACCEPT — conclude receives an empty verdict plus the measurement; streams and asks untouched; completion is the runner's transition, legal from any stream configuration. |
