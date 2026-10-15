# a-proof-compiles-a-package-once

- State: approved
- Priority: 1
- Sequence: 56
- Risk: severity=2 novelty=2 exposure=1 accumulation=1 basis="Lane test finding 336 (2026-10-15 17:36): twelve concurrent Go compiles of 3.2-3.6 GB during one proof, swap 94 %, 61 GB in the compressor; analysis/memory-hogs-and-root-causes-2026-10-15.md H1; Astra rounds folded."
- Tier: 2
- Intent: A proof compiles a package once, then fans out: shards of one package start one at a time per package on the first run event or a package terminal, a producer's exit resolves its waiters, and in-test engine builds take one cross-process lock per build variant. Design metasystem/plans/designs/a-proof-compiles-a-package-once.md (Id 01M4ZA0000000000000000MEM1).
- Origin: human
- Next step: U1 the runner's per-package start chain (<=140 production lines), U2 the per-variant build lock (<=100), each built by Codex in worktree -memA from a FINAL brief, read by Opus, gated alone, landed from the pane, deployed to the landing seat.
- OpenedAt: 2026-10-15T17:25:52Z
- Revision: 4
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-15T17:26:06Z revision=2 opid=7AE86DPTJ585CHVZG57NAXZ9A2-m1e-670f65d0 authority=proven digest=4418289d05fb4471761d4df9d17dd979d0fab8669c206832bf19498ad2c7a4bc episode=2

History:
- 2026-10-15T17:25:52Z 0ZGS2HJTBTRCGGRQYJ4TPTEKVG-m1e-670f65d0 open actor=human:Wido targets=a-proof-compiles-a-package-once
- 2026-10-15T17:26:06Z 7AE86DPTJ585CHVZG57NAXZ9A2-m1e-670f65d0 approve actor=human:Wido targets=a-proof-compiles-a-package-once,fixture-app-reads-its-whole-release,the-reconciler-reads-attempt-headers
- 2026-10-15T17:26:26Z YCWMJMV48TEREWMTBBWMGR1YHR-m1e-670f65d0 set-priority actor=human:Wido targets=a-proof-compiles-a-package-once reason=priority-order subject=a-proof-compiles-a-package-once from=unranked to=1:57 requested-sequence=append
- 2026-10-15T17:26:41Z PBN8F2SMFXX3YKV3CCXEFQRK61-m1e-670f65d0 done actor=human:Wido targets=a-proof-compiles-a-package-once,a-records-change-runs-its-affected-checks,asks-clear-on-their-effect,fixture-app-reads-its-whole-release,red-names-what-failed,steward-duties-run-on-every-tick,the-lane-says-what-it-does,the-reconciler-reads-attempt-headers reason=priority-order from=1:57 to=1:56
Integrity: sha256=13526458ed14cdfd618e8ad194f857cdace7f899cfdf2a4ac693fb453c483267
