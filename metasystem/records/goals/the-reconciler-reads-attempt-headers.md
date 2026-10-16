# the-reconciler-reads-attempt-headers

- State: done
- Priority: 1
- Sequence: 55
- Risk: severity=2 novelty=1 exposure=1 accumulation=1 basis="Lane test finding 335 (2026-10-15 17:31): the m1e seat's job reaper at 1.0 GB physical footprint from decoding 518 MB of September attempt records every minute; analysis H2; Astra rounds folded."
- Tier: 2
- Intent: The attempt reconciler lists, reads in full only what it acts on, never reads a settled attempt again (a settled sidecar after a confirmed settlement), and the counting readers stream heads that stop at the first payload key. Design metasystem/plans/designs/the-reconciler-reads-attempt-headers.md (Id 01M4ZB0000000000000000MEM2).
- Origin: human
- Next step: U1 (<=150 production lines) built by Codex in worktree -memB from a FINAL brief, read by Opus, gated alone, landed from the pane; the m1e seat rebuilt and restarted.
- Concluded: U1 (53f96105b, reconciler lists and reads heads; reaper 1140 MB to 36 MB measured after restart) and U2 (e590dfc22, the disk pass keeps attempt references) landed on main by hand under the broken-machine rule; gate green alone 03:42-04:51; Opus read, Astra-critiqued briefs
- OpenedAt: 2026-10-15T17:25:59Z
- Revision: 7
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-15T17:26:06Z revision=2 opid=7AE86DPTJ585CHVZG57NAXZ9A2-m1e-670f65d0 authority=proven digest=93ce2bd267f608f9e54b17a60b60dfdbd16c636e7c4a057cec06332e3dafb1a5 episode=2

History:
- 2026-10-15T17:25:59Z 7J2CRY7ZKS7BEMQKF9HD9CK606-m1e-670f65d0 open actor=human:Wido targets=the-reconciler-reads-attempt-headers
- 2026-10-15T17:26:06Z 7AE86DPTJ585CHVZG57NAXZ9A2-m1e-670f65d0 approve actor=human:Wido targets=a-proof-compiles-a-package-once,fixture-app-reads-its-whole-release,the-reconciler-reads-attempt-headers
- 2026-10-15T17:26:33Z VP8DA0947ZA06V4ZWF5K3QBX4A-m1e-670f65d0 set-priority actor=human:Wido targets=the-reconciler-reads-attempt-headers reason=priority-order subject=the-reconciler-reads-attempt-headers from=unranked to=1:58 requested-sequence=append
- 2026-10-15T17:26:41Z PBN8F2SMFXX3YKV3CCXEFQRK61-m1e-670f65d0 done actor=human:Wido targets=a-proof-compiles-a-package-once,a-records-change-runs-its-affected-checks,asks-clear-on-their-effect,fixture-app-reads-its-whole-release,red-names-what-failed,steward-duties-run-on-every-tick,the-lane-says-what-it-does,the-reconciler-reads-attempt-headers reason=priority-order from=1:58 to=1:57
- 2026-10-15T17:26:55Z G6J7BMRPGVD16EJGRP954NEK85-m1e-670f65d0 done actor=human:Wido targets=a-proof-compiles-a-package-once,fixture-app-reads-its-whole-release,the-reconciler-reads-attempt-headers reason=priority-order from=1:57 to=1:56
- 2026-10-15T22:09:21Z 3HGQ2XDFPH8C0A0YHMKNF777RD-m1e-670f65d0 done actor=human:Wido targets=a-proof-compiles-a-package-once,the-reconciler-reads-attempt-headers reason=priority-order from=1:56 to=1:55
- 2026-10-16T03:31:27Z XE0X5XFEMFWTQNP48CVBV64DRX-m1e-670f65d0 done actor=human:Wido targets=a-closed-delegates-objects-live-in-the-repository,the-reconciler-reads-attempt-headers
Integrity: sha256=f84b1fb57c5d063b7321fe9e7aa4c25f1819383e4e8840690742aec05e1b2475
