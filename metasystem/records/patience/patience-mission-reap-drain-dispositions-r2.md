# Dispositions — patience-mission-reap-drain, critique round 2

Critic: design-critic-20260811t105033z-4366 (codex gpt-5.6-sol, xhigh).
Verdict: 8 material. All 8 ACCEPTED; design amended in the same commit.
Two findings were genuine safety defects in round 1's own amendments
(PMRD-R2-003, -005), so the loop proceeds to a round 3 rather than
stopping on the diminishing-returns rule.

| # | Sev | Claim (short) | Disposition |
|---|-----|---------------|-------------|
| PMRD-R2-001 | high | Exactly-once relied on a single write that is really two | ACCEPT — booking is ledger-append then state-conclude; the crash between lands in the shipped ledger-ahead reconciliation after the entry strips the claim. |
| PMRD-R2-002 | high | faultDetail cannot reproduce ConcludeFaultedTurn | ACCEPT — the claim carries a pointer, not a copy: turn.json (satellite 1) is the single source of the fault facts. |
| PMRD-R2-003 | high | Answers during the park are NOT safe for a pending accepted conclusion | ACCEPT — only `resume:` acts immediately; other answers record but apply at the next turn boundary; the resumed conclusion re-validates transitions and faults rather than corrupts. |
| PMRD-R2-004 | high | The park/ask crash window had no recovery entry | ACCEPT — the ask is raised idempotently by both the park and the drain-resume entry. |
| PMRD-R2-005 | high | Expired handshake with no recorded pid mapped to process-lost without proof | ACCEPT — not reapable: no proof means no verdict; it survives to the deadline and the ask names it. |
| PMRD-R2-006 | medium | Deadline fallback used the wrong clock | ACCEPT — launched records use startedAt + their own capMin + grace; only setup husks use createdAt + setup grace. |
| PMRD-R2-007 | medium | The final survivor snapshot is not concurrency-safe | ACCEPT — the ask advises from the claim's snapshot; drain-resume re-proves against the live set. |
| PMRD-R2-008 | medium | The annotation count had no durable source | ACCEPT — the claim records the survivor ids; the annotation counts them. |
