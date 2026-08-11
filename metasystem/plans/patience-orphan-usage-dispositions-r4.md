# Dispositions — patience-orphan-usage, critique round 4

Critic: design-critic-20260811t135031z-7fc2 (codex gpt-5.6-sol, xhigh).
Verdict: 6 material. All 6 ACCEPTED; design amended in the same commit
(whole-document regeneration discipline).

| # | Sev | Claim (short) | Disposition |
|---|-----|---------------|-------------|
| POU-R4-001 | critical | A return orphaned by the final successful cycle is never delivered | ACCEPT — terminal delivery: at completion and on the failure ramp the derived list lands as `- Landed unconsumed:` annotations in the final cycle's ledger block, where humans and graders read. |
| POU-R4-002 | critical | Named custodians do not establish group death | ACCEPT — the gate is pgid-ESRCH (EPERM blocks) AND all recorded custodians dead; vintage rule fixed: no pgid → custodians must all prove dead; neither recorded → unavailable. |
| POU-R4-003 | high | Overflow lacks order and a consistent limit | ACCEPT — sort by (chain root, round); 20 rows total: 19 data + 1 overflow summary when exceeded. |
| POU-R4-004 | high | The named flight-recorder event does not exist | ACCEPT — new additive registry kind `aggregation-failed` {mission, site, error}, named in the design. |
| POU-R4-005 | medium | Byte-identical conflicts with updatedAt | ACCEPT — content-equal writes are skipped; updatedAt changes exactly when content changes. |
| POU-R4-006 | medium | No field carries the parse failure | ACCEPT — `detail` added to the provenance entry schema. |
