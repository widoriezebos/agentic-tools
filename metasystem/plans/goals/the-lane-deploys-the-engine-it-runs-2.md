# the-lane-deploys-the-engine-it-runs-2

- State: approved
- Risk: severity=3 novelty=2 exposure=3 accumulation=2 basis="The same design and the same answers as the-lane-deploys-the-engine-it-runs: a swap beside a running proof voids the proof or runs two engines (severity 3); the swap record joins existing keeper and cancellation forms (novelty 2); every automatic proof start passes it (exposure 3); one record per swap, completed and removed by the keeper (accumulation 2)."
- Tier: 3
- Intent: The lane deploys the engine it runs, part 2: the pending-swap record, distinct from the cancellation request, and its completion by the keeper before every automatic proof-start path, so an engine built during a running proof is swapped in at the proof's end and never beside it (U2b, design decision D3). Design: plans/designs/the-lane-deploys-the-engine-it-runs.md (accepted 2026-10-16; U2a landed 2026-10-16, U1.1 and U1.2a 2026-10-17, U1.2b 2026-10-19 under the first goal).
- Origin: main
- Next step: Brief U2b from the design page's D3 section and the U2 test list (arm during a running proof waits; the keeper completes the swap before moved-main recovery; mutations for both orders); Codex builds it in a registered workspace, Opus reads, sharded gate, hand landing.
- OpenedAt: 2026-10-19T21:21:32Z
- Revision: 2
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-19T21:21:41Z revision=2 opid=X1BGZS7CRR2Z4JMKJXJ701MTM2-m1e-f2f350ed authority=proven digest=6e5b7ceca67f9c674f4a50a4a0b3648afd6f8acb1e0736e31f56bde952c02307 episode=2

History:
- 2026-10-19T21:21:32Z J9F6RMBXEX513RR92RTG9QR6FW-m1e-f2f350ed open actor=human:Wido targets=the-lane-deploys-the-engine-it-runs-2
- 2026-10-19T21:21:41Z X1BGZS7CRR2Z4JMKJXJ701MTM2-m1e-f2f350ed approve actor=human:Wido targets=the-lane-deploys-the-engine-it-runs-2
Integrity: sha256=30f7f8fb881d99a6f5275758020fdfe656d5a29aa1cbcfb6b8062564976ef2d5
