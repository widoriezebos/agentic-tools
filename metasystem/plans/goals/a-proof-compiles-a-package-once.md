# a-proof-compiles-a-package-once

- State: queued
- Risk: severity=2 novelty=2 exposure=1 accumulation=1 basis="Lane test finding 336 (2026-10-15 17:36): twelve concurrent Go compiles of 3.2-3.6 GB during one proof, swap 94 %, 61 GB in the compressor; analysis/memory-hogs-and-root-causes-2026-10-15.md H1; Astra rounds folded."
- Tier: 2
- Intent: A proof compiles a package once, then fans out: shards of one package start one at a time per package on the first run event or a package terminal, a producer's exit resolves its waiters, and in-test engine builds take one cross-process lock per build variant. Design metasystem/plans/designs/a-proof-compiles-a-package-once.md (Id 01M4ZA0000000000000000MEM1).
- Origin: human
- Next step: U1 the runner's per-package start chain (<=140 production lines), U2 the per-variant build lock (<=100), each built by Codex in worktree -memA from a FINAL brief, read by Opus, gated alone, landed from the pane, deployed to the landing seat.
- OpenedAt: 2026-10-15T17:25:52Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-10-15T17:25:52Z 0ZGS2HJTBTRCGGRQYJ4TPTEKVG-m1e-670f65d0 open actor=human:Wido targets=a-proof-compiles-a-package-once
Integrity: sha256=5eb08d50f503c2f6bc44de5fa3742641d5b52b6b31e9cfd022da10dbc2191cf8
