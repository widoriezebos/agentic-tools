# Dispositions — patience-orphan-usage, critique round 3

Critic: design-critic-20260811t133402z-feda (codex gpt-5.6-sol, xhigh).
Verdict: 7 material. All 7 ACCEPTED; design amended in the same commit.
The round critiqued judgment, as briefed — the fact pass held (no
finding contradicted an anchored claim).

| # | Sev | Claim (short) | Disposition |
|---|-----|---------------|-------------|
| POU-R3-001 | critical | Succession neither necessary nor sufficient; closure exclusion recreates the park loss | ACCEPT — boundary is now the host's own recorded action (turn-log certified entries and accepted successor dispatches, F Q1.28-29); chain closure no longer excludes; a runner-closed, never-certified round keeps nagging by design. |
| POU-R3-002 | high | return.json existence exposes invalid returns as ready | ACCEPT — rows validate via return-complete; failed validation lists as `invalid`, not ready. |
| POU-R3-003 | critical | Terminal status does not silence the event writer | ACCEPT — derivation gates on proven group death via the shared custodian owner; not-yet-proven rows aggregate `pending-death-proof` and derive later. |
| POU-R3-004 | high | The runner failure exit skips aggregation | ACCEPT — one aggregation call added on the failure ramp before lease release (F Q2.21). |
| POU-R3-005 | high | Aggregation failure behavior unspecified at parks | ACCEPT — errors log to the flight recorder and never fail the park/conclusion/exit; accounting catches up at the next successful call. |
| POU-R3-006 | high | One row per open chain is not a safe prompt bound | ACCEPT — hard cap of 20 rows with an overflow summary row; per-chain and fence bounds stated. |
| POU-R3-007 | medium | Provenance schema left to the implementer | ACCEPT — fixed: sorted array of {jobId, round, provenance, source} with the four provenance values enumerated. |
