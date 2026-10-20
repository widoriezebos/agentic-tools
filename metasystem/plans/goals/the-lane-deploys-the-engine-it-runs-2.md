# the-lane-deploys-the-engine-it-runs-2

- State: approved
- Priority: 1
- Sequence: 62
- Risk: severity=3 novelty=2 exposure=3 accumulation=2 basis="The same design and the same answers as the-lane-deploys-the-engine-it-runs: a swap beside a running proof voids the proof or runs two engines (severity 3); the swap record joins existing keeper and cancellation forms (novelty 2); every automatic proof start passes it (exposure 3); one record per swap, completed and removed by the keeper (accumulation 2)."
- Tier: 3
- Intent: The lane deploys the engine it runs, part 2: the pending-swap record, distinct from the cancellation request, and its completion by the keeper before every automatic proof-start path, so an engine built during a running proof is swapped in at the proof's end and never beside it (U2b, design decision D3). Design: plans/designs/the-lane-deploys-the-engine-it-runs.md (accepted 2026-10-16; U2a landed 2026-10-16, U1.1 and U1.2a 2026-10-17, U1.2b 2026-10-19 under the first goal).
- Origin: main
- Next step: Brief U2b from the design page's D3 section and the U2 test list (arm during a running proof waits; the keeper completes the swap before moved-main recovery; mutations for both orders); Codex builds it in a registered workspace, Opus reads, sharded gate, hand landing. Flaky: metasystem/cmd/metasystem TestLaneFixLapsedGoalBoxAdmitsCommittedReadRealDispatch; attempt 20261019T212050.742776000Z; log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261019T212050.742776000Z.log; load 5.310546875; seen 7 times. Flaky: metasystem/cmd/metasystem TestLaneFixLapsedGoalBoxAdmitsCommittedReadRealDispatch; attempt 20261020T054219.063899000Z; log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261020T054219.063899000Z.log; load 4.70703125; seen 8 times. Flaky: metasystem/cmd/metasystem TestLaneFixLapsedGoalBoxAdmitsCommittedReadRealDispatch; attempt 20261020T072937.156672000Z; log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261020T072937.156672000Z.log; load 4.85986328125; seen 9 times.
- OpenedAt: 2026-10-19T21:21:32Z
- Revision: 6
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-19T21:21:41Z revision=2 opid=X1BGZS7CRR2Z4JMKJXJ701MTM2-m1e-f2f350ed authority=proven digest=6e5b7ceca67f9c674f4a50a4a0b3648afd6f8acb1e0736e31f56bde952c02307 episode=2

History:
- 2026-10-19T21:21:32Z J9F6RMBXEX513RR92RTG9QR6FW-m1e-f2f350ed open actor=human:Wido targets=the-lane-deploys-the-engine-it-runs-2
- 2026-10-19T21:21:41Z X1BGZS7CRR2Z4JMKJXJ701MTM2-m1e-f2f350ed approve actor=human:Wido targets=the-lane-deploys-the-engine-it-runs-2
- 2026-10-19T21:21:50Z SE0G349B7E39WHTZV920PH51QK-m1e-f2f350ed set-priority actor=human:Wido targets=the-lane-deploys-the-engine-it-runs-2 reason=priority-order subject=the-lane-deploys-the-engine-it-runs-2 from=unranked to=1:62 requested-sequence=append
- 2026-10-19T22:02:56Z E2D0417F669BF16D953D2E2DE5-landing-41ad7d6c record-flake actor=landing+landing-agent targets=the-lane-deploys-the-engine-it-runs-2
- 2026-10-20T06:29:05Z D52AEA35C74678560A853A52D7-landing-41ad7d6c record-flake actor=landing+landing-agent targets=the-lane-deploys-the-engine-it-runs-2
- 2026-10-20T08:14:53Z 1E249C29FE4DB215C694F330E7-landing-41ad7d6c record-flake actor=landing+landing-agent targets=the-lane-deploys-the-engine-it-runs-2
Integrity: sha256=52d864db73ad8c41f03d934f960ca3117b882f41916e719bea001b4516de230a
