# Dispositions — stop-loss-core, critique round 1

Critic: design-critic-20260811t073858z-cfc3 (codex gpt-5.6-sol, xhigh).
Verdict: 9 material. All 9 ACCEPTED. The amendment is structural: the fuse
became a pure replay of (sealed contract, ledger) — no cached state — which
resolves findings 05–09 at the root rather than individually.

| # | Sev | Claim (short) | Disposition |
|---|-----|---------------|-------------|
| 01 | critical | One regression funds arbitrarily many recovery decrements | ACCEPT — decay REMOVED; budgets + human reset cover reverts. |
| 02 | critical | Noise-gated multi-metric comparison not a total order; stored best ambiguous | ACCEPT — comparison is candidate-vs-stored-best only (fold, not order); raw lexicographic tuple, noise gates the first differing component; best recorded as `best=yes|no` on the line. |
| 03 | high | Recovery cycle had two classifications | ACCEPT — moot with decay removed; recovery cycles are plain `unresolved`. |
| 04 | high | Initial best undefined | ACCEPT — bests initialize from the sealed baseline measurement. |
| 05 | high | Relocation drops mission cycle-budget enforcement | ACCEPT — the runner's derived verdict enforces cycle-budget AND no-gain. |
| 06 | high | Asks not exactly-once; askId can't key idempotence | ACCEPT — idempotence is structural: a reset is its ledger line; replay applies it by position; duplicates are harmless and vocal. |
| 07 | high | Reset replay fails ledger-anchor reconciliation | ACCEPT — reconcile gains a specified tolerance: a trailing reset line without its unpark is replayable state, not divergence. |
| 08 | critical | Legacy ledger grammar can't derive bests/stagnant | ACCEPT — conservative legacy replay specified: baseline-seeded bests, classification words drive the count, unparseable folds as baseline. |
| 09 | critical | Migrated cache can lag the ledger after a crash | ACCEPT — no cache exists; the verdict is derived on every load. |
