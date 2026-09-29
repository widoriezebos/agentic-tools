# agents-interchangeable-on-every-lane

- State: claimed
- Risk: severity=2 novelty=2 exposure=2 accumulation=1 basis="A wrong default sends work to an agent nobody chose, but every run records its agent and the change reverts cleanly; the Devin adapter and auto resolution are new; the defaults ship to every adopted repository; it is a one-off change."
- Tier: 2
- Intent: Any agent on the host runs every lane: Devin runs the launch lanes, and the default agent is the first of claude, codex, devin found on PATH, with per-runtime models from configuration.
- Origin: human
- Next step: Land the built change with work land --message after its risk-selected tests pass.
- OpenedAt: 2026-09-29T13:02:16Z
- Revision: 3
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:W at=2026-09-29T13:02:25Z revision=2 opid=FT7QNPAPS0GDAD7W02Q2S35NB7-wr-m1-3164cf85 authority=proven digest=6d5b2577804091ca61b84cd72914f552d0a7da029e8a61238670f51492f2ee5c episode=2
- Claimed: machine=wr-m1 lineage=main-1790678069-98670-2b88ce at=2026-09-29T13:46:46Z revision=3 accountingRevision=3 episodeAt=2026-09-29T13:46:46Z episodeRevision=3
- StopCapability: generation=3 revision=3 machine=wr-m1 claimEpoch=1 fenceEpoch=0

History:
- 2026-09-29T13:02:16Z 0NG4QB97GEVGG45RMRZT618TP6-wr-m1-3164cf85 open actor=human:W targets=agents-interchangeable-on-every-lane
- 2026-09-29T13:02:25Z FT7QNPAPS0GDAD7W02Q2S35NB7-wr-m1-3164cf85 approve actor=human:W targets=agents-interchangeable-on-every-lane
- 2026-09-29T13:46:46Z KCAN8QGJ2BENXG6J60F9GNKHJA-wr-m1-f4eb764f claim actor=wr-m1+main-1790678069-98670-2b88ce targets=agents-interchangeable-on-every-lane
Integrity: sha256=88afa880a46a0e1902eba024e100834c0a91fbf98bf4595b39716a3d7824ab49
