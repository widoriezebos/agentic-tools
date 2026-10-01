# fleet-card-follows-the-simple-lane

- State: claimed
- Risk: severity=1 novelty=1 exposure=1 accumulation=1 basis="interface text and types for one card; read-only view"
- Tier: 1
- Intent: What: The landing lane card on the Fleet page shows the simple lane as it now is: its landing agent running, idle (a ready lane with nothing to land, no prompt), stopped by a person (who and why, and the command to resume it) or unready (the reason and the fix), with no restart count. Why: the simple lane (2026-10-01) replaced the old lane owner, so the card's states and restart count no longer matched what the lane reports. Pros: the card tells the truth about the lane; it is also the first change handed to the new lane from the ui seat. Cons: none of note; the card only reads.
- Origin: human
- Next step: Next: land branch goal/fleet-card-follows-the-simple-lane through the landing lane with work land. Done when: the card shows the lane's real state on the Fleet page after the landing.
- OpenedAt: 2026-10-01T09:44:44Z
- Revision: 6
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-01T09:44:48Z revision=2 opid=TSXV92DA1Z9GN8CMMQ4RXAZ14Q-ui-31a738e9 authority=proven digest=9f78b888b61aa7e2d8d673d604fb01672aee919e13d4816a008b5f8dc4427462 episode=2
- Claimed: machine=landing lineage=landing-lane at=2026-10-01T14:48:21Z revision=5 accountingRevision=5 episodeAt=2026-10-01T14:48:21Z episodeRevision=5
- HandedOver: fromMachine=ui fromLineage=main-1790664507-87928-61899b fromEpoch=2 batch=xy5b0kck87bjyryam7ja24x42z
- StopCapability: generation=5 revision=5 machine=landing claimEpoch=1 fenceEpoch=0

History:
- 2026-10-01T09:44:44Z 3RP95HKHQF8WMP5NVR3NE10583-ui-31a738e9 open actor=human:Wido targets=fleet-card-follows-the-simple-lane
- 2026-10-01T09:44:48Z TSXV92DA1Z9GN8CMMQ4RXAZ14Q-ui-31a738e9 approve actor=human:Wido targets=fleet-card-follows-the-simple-lane
- 2026-10-01T09:45:01Z P72V75XR81FXP2NM7VQRW6NM1Q-ui-d4023bb7 claim actor=ui+main-1790664507-87928-61899b targets=fleet-card-follows-the-simple-lane
- 2026-10-01T11:23:45Z 1D4S592VWVXB8D6CA5CZNTQ2AS-ui-43182c96 breach-stop actor=ui+goal-stop-custodian targets=fleet-card-follows-the-simple-lane
- 2026-10-01T14:48:21Z 6Z3GWZZXDEA0DXE6ME1BPW3JFR-ui-31a738e9 resume actor=human:Wido targets=fleet-card-follows-the-simple-lane
- 2026-10-01T14:53:06Z KJRCX6SZPCKQ7HZXT9GJT8APX9-ui-d4023bb7 handover actor=ui+main-1790664507-87928-61899b targets=fleet-card-follows-the-simple-lane
Integrity: sha256=6baa89a28230ca0d52b8048cedfeb4326f59eb3510afe1cd33f4fd8c6989ce00
