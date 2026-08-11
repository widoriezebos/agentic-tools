# Dispositions — stop-loss-last-defense, critique round 2

Critic: design-critic-20260811t065301z-54bf (codex gpt-5.6-sol, xhigh).
Verdict: 14 material findings. All 14 ACCEPTED; design amended in the same
commit.

| # | Sev | Claim (short) | Disposition |
|---|-----|---------------|-------------|
| 0 | critical | Reset-on-improvement is blind to oscillation | ACCEPT — reset requires a NEW BEST beyond the sealed noise floor (ratchet); recovering lost ground never resets. |
| 1 | critical | Retiring the lifetime rule inside shared assert-stop-loss.sh weakens non-mission workflows | ACCEPT — retirement is mission-scoped behind an explicit switch; non-mission callers keep today's behavior and fixtures. |
| 2 | high | Credit budget lacks authoritative stream identity | ACCEPT — chain→stream binding is recorded by the runner at chain-root dispatch, never taken from a return. |
| 3 | high | Migration default rewrites sealed allowances | ACCEPT — contracts sealed without the key grant NO credit budget; the default applies only to contracts sealed after the key exists. A seal means what was signed. |
| 4 | high | Multi-closure cycle spend undefined | ACCEPT — every discovered closure mints (and spends) its credit; the cycle classifies once. |
| 5 | high | Improve+close in one cycle contradictory | ACCEPT — contract-improved takes precedence; credits still minted and spent. |
| 6 | high | Ledger-first is not crash-consistent | ACCEPT — append-then-apply with idempotent startup reconciliation keyed by askId; no duplicate lines, orphaned lines replay forward. |
| 7 | high | Honest host still punished when only the terminal result names the session | ACCEPT — observedSession comes from the earliest trusted harness artifact: handshake signal OR adapter terminal result. |
| 8 | high | Drain deadline not always finite | ACCEPT — fallback chain: capDeadline → createdAt + mission job cap + grace → treat as already due. Always finite. |
| 9 | high | drain-stalled park has no recovery authority | ACCEPT — recovery defined: human clears the named records via existing surfaces and answers `resume:`; the runner re-drains on resume. |
| 10 | critical | Per-stream scalar applied mark broken across chains | ACCEPT — applied marks are a SET of (chain root, round) pairs per stream; orphanhood is set membership, idempotent. |
| 11 | high | Single-writer usage dies under SIGKILL | ACCEPT — post-mortem derivation from the runtime's surviving on-disk event stream where one exists (codex JSONL), source recorded; honest `unavailable` otherwise. |
| 12 | high | Mission-state migration missing | ACCEPT — new state fields derived once by ledger/record replay inside a normal state write (new generation, hash chain intact); shape validation admits them as optional during migration. |
| 13 | high | Park-only orphan scan leaves the race open | ACCEPT — the scan runs at every turn conclusion AND at park; the applied/orphaned sets make it idempotent. |
