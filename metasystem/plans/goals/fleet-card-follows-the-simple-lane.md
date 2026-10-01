# fleet-card-follows-the-simple-lane

- State: claimed
- Risk: severity=1 novelty=1 exposure=1 accumulation=1 basis="interface text and types for one card; read-only view"
- Tier: 1
- Intent: What: The landing lane card on the Fleet page shows the simple lane as it now is: its landing agent running, idle (a ready lane with nothing to land, no prompt), stopped by a person (who and why, and the command to resume it) or unready (the reason and the fix), with no restart count. Why: the simple lane (2026-10-01) replaced the old lane owner, so the card's states and restart count no longer matched what the lane reports. Pros: the card tells the truth about the lane; it is also the first change handed to the new lane from the ui seat. Cons: none of note; the card only reads.
- Origin: human
- Next step: Next: land branch goal/fleet-card-follows-the-simple-lane through the landing lane with work land. Done when: the card shows the lane's real state on the Fleet page after the landing.
- OpenedAt: 2026-10-01T09:44:44Z
- Revision: 3
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-01T09:44:48Z revision=2 opid=TSXV92DA1Z9GN8CMMQ4RXAZ14Q-ui-31a738e9 authority=proven digest=9f78b888b61aa7e2d8d673d604fb01672aee919e13d4816a008b5f8dc4427462 episode=2
- Claimed: machine=ui lineage=main-1790664507-87928-61899b at=2026-10-01T09:45:01Z revision=3 accountingRevision=3 episodeAt=2026-10-01T09:45:01Z episodeRevision=3
- StopCapability: generation=3 revision=3 machine=ui claimEpoch=2 fenceEpoch=0

History:
- 2026-10-01T09:44:44Z 3RP95HKHQF8WMP5NVR3NE10583-ui-31a738e9 open actor=human:Wido targets=fleet-card-follows-the-simple-lane
- 2026-10-01T09:44:48Z TSXV92DA1Z9GN8CMMQ4RXAZ14Q-ui-31a738e9 approve actor=human:Wido targets=fleet-card-follows-the-simple-lane
- 2026-10-01T09:45:01Z P72V75XR81FXP2NM7VQRW6NM1Q-ui-d4023bb7 claim actor=ui+main-1790664507-87928-61899b targets=fleet-card-follows-the-simple-lane
Integrity: sha256=d0a3ae285bcea07f49c99da178ed4f8b92c498e5f01147ab8e442c051664ae90
