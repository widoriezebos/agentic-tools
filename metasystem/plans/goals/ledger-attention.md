# ledger-attention

- State: claimed
- Intent: A machine notices when the shared ledger changes under it - the steward tick fetches and surfaces new claimable goals, pins addressed to this machine, and queue reorderings, so no human relays routine nudges between machines (Wido 2026-08-30: 'should we build a mechanism for you to be able to do that yourself?')
- Origin: human
- Next step: Appetite: 3h. Design half (1h, Fable lane): what the tick surfaces and where (the coordinator-facing narration), the staleness health condition (ledger moved but unexamined past a threshold), and the boundary - this is attention, never authority: surfacing a pin grants nothing, claims still go through goal verbs and budgets. Implementation half (2h, Sol lane): fetch on tick with the offline case fail-quiet-but-recorded, dedupe so a nudge fires once per change, health line in the digest. Agent-agnostic by construction - the ledger stays the only channel; this only reads it on a cadence. Sequenced after the watch verb (shared steward seam); m2 could claim it earlier since it is symmetric
- OpenedAt: 2026-08-30T13:38:36Z
- Revision: 4
- Budget: elapsedLimit=3d attemptLimit=6 reservedJobMinutesLimit=960 activeJobLimit=2
- Claimed: machine=m2 lineage=mac-coordinator at=2026-08-30T20:49:53Z revision=4
- StopCapability: generation=4 revision=4 machine=m2 claimEpoch=1 fenceEpoch=0

History:
- 2026-08-30T13:38:36Z GXV12W8X9D21QAQR97ECX52CCA-m1-bf243850 open actor=m1+coordinator targets=ledger-attention
- 2026-08-30T15:17:00Z KVRXBHRWRM6KM2SBR5J8RQPNZF-m1-bf243850 set-budget actor=m1+coordinator targets=ledger-attention
- 2026-08-30T16:52:28Z CZGRTJK4KQY57EXHNGCDF0A9TB-m2-bc1be9cb claim actor=m2+mac-coordinator targets=ledger-attention
- 2026-08-30T20:49:53Z MVHWBPCAX0JTBH9D720F2X3FXW-m2-bc1be9cb set-budget actor=human:wido targets=ledger-attention
Integrity: sha256=1efd5f37ace3ec0764148670093f54605b56a780a048e6f012719123ed6fe99d
