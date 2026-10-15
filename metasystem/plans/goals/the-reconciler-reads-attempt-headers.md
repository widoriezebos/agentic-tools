# the-reconciler-reads-attempt-headers

- State: approved
- Priority: 1
- Sequence: 57
- Risk: severity=2 novelty=1 exposure=1 accumulation=1 basis="Lane test finding 335 (2026-10-15 17:31): the m1e seat's job reaper at 1.0 GB physical footprint from decoding 518 MB of September attempt records every minute; analysis H2; Astra rounds folded."
- Tier: 2
- Intent: The attempt reconciler lists, reads in full only what it acts on, never reads a settled attempt again (a settled sidecar after a confirmed settlement), and the counting readers stream heads that stop at the first payload key. Design metasystem/plans/designs/the-reconciler-reads-attempt-headers.md (Id 01M4ZB0000000000000000MEM2).
- Origin: human
- Next step: U1 (<=150 production lines) built by Codex in worktree -memB from a FINAL brief, read by Opus, gated alone, landed from the pane; the m1e seat rebuilt and restarted.
- OpenedAt: 2026-10-15T17:25:59Z
- Revision: 4
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-15T17:26:06Z revision=2 opid=7AE86DPTJ585CHVZG57NAXZ9A2-m1e-670f65d0 authority=proven digest=93ce2bd267f608f9e54b17a60b60dfdbd16c636e7c4a057cec06332e3dafb1a5 episode=2

History:
- 2026-10-15T17:25:59Z 7J2CRY7ZKS7BEMQKF9HD9CK606-m1e-670f65d0 open actor=human:Wido targets=the-reconciler-reads-attempt-headers
- 2026-10-15T17:26:06Z 7AE86DPTJ585CHVZG57NAXZ9A2-m1e-670f65d0 approve actor=human:Wido targets=a-proof-compiles-a-package-once,fixture-app-reads-its-whole-release,the-reconciler-reads-attempt-headers
- 2026-10-15T17:26:33Z VP8DA0947ZA06V4ZWF5K3QBX4A-m1e-670f65d0 set-priority actor=human:Wido targets=the-reconciler-reads-attempt-headers reason=priority-order subject=the-reconciler-reads-attempt-headers from=unranked to=1:58 requested-sequence=append
- 2026-10-15T17:26:41Z PBN8F2SMFXX3YKV3CCXEFQRK61-m1e-670f65d0 done actor=human:Wido targets=a-proof-compiles-a-package-once,a-records-change-runs-its-affected-checks,asks-clear-on-their-effect,fixture-app-reads-its-whole-release,red-names-what-failed,steward-duties-run-on-every-tick,the-lane-says-what-it-does,the-reconciler-reads-attempt-headers reason=priority-order from=1:58 to=1:57
Integrity: sha256=143492a60908a152f47ad937d7b5b239a97fbb62c511a2894c2d2c12b917474c
