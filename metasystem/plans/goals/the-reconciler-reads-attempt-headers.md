# the-reconciler-reads-attempt-headers

- State: approved
- Priority: 1
- Sequence: 58
- Risk: severity=2 novelty=1 exposure=1 accumulation=1 basis="Lane test finding 335 (2026-10-15 17:31): the m1e seat's job reaper at 1.0 GB physical footprint from decoding 518 MB of September attempt records every minute; analysis H2; Astra rounds folded."
- Tier: 2
- Intent: The attempt reconciler lists, reads in full only what it acts on, never reads a settled attempt again (a settled sidecar after a confirmed settlement), and the counting readers stream heads that stop at the first payload key. Design metasystem/plans/designs/the-reconciler-reads-attempt-headers.md (Id 01M4ZB0000000000000000MEM2).
- Origin: human
- Next step: U1 (<=150 production lines) built by Codex in worktree -memB from a FINAL brief, read by Opus, gated alone, landed from the pane; the m1e seat rebuilt and restarted.
- OpenedAt: 2026-10-15T17:25:59Z
- Revision: 3
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-15T17:26:06Z revision=2 opid=7AE86DPTJ585CHVZG57NAXZ9A2-m1e-670f65d0 authority=proven digest=93ce2bd267f608f9e54b17a60b60dfdbd16c636e7c4a057cec06332e3dafb1a5 episode=2

History:
- 2026-10-15T17:25:59Z 7J2CRY7ZKS7BEMQKF9HD9CK606-m1e-670f65d0 open actor=human:Wido targets=the-reconciler-reads-attempt-headers
- 2026-10-15T17:26:06Z 7AE86DPTJ585CHVZG57NAXZ9A2-m1e-670f65d0 approve actor=human:Wido targets=a-proof-compiles-a-package-once,fixture-app-reads-its-whole-release,the-reconciler-reads-attempt-headers
- 2026-10-15T17:26:33Z VP8DA0947ZA06V4ZWF5K3QBX4A-m1e-670f65d0 set-priority actor=human:Wido targets=the-reconciler-reads-attempt-headers reason=priority-order subject=the-reconciler-reads-attempt-headers from=unranked to=1:58 requested-sequence=append
Integrity: sha256=3125d833c06182ed166ad745dcd2b89fd528fc4c22d472eecad75c48c260f988
