# fleet-card-follows-the-simple-lane

- State: claimed
- Risk: severity=1 novelty=1 exposure=1 accumulation=1 basis="interface text and types for one card; read-only view"
- Tier: 1
- Intent: What: The landing lane card on the Fleet page shows the simple lane as it now is: its landing agent running, idle (a ready lane with nothing to land, no prompt), stopped by a person (who and why, and the command to resume it) or unready (the reason and the fix), with no restart count. Why: the simple lane (2026-10-01) replaced the old lane owner, so the card's states and restart count no longer matched what the lane reports. Pros: the card tells the truth about the lane; it is also the first change handed to the new lane from the ui seat. Cons: none of note; the card only reads.
- Origin: human
- Next step: source machine holds fleet-card-can-land-now; ejected fleet-card-follows-the-simple-lane through card: red: the landing agent's word: not a fault of this work: the lane proof cannot finish green. main dc57b2f40 + this member (8069e4d68, tree a4bc49fa6af1) merged cleanly and every one of the 28 selected test groups passed (TEST-RESULT sufficient=true, attempt proof-mupstdgw-636698ee9b4b7500), but the run ended unavailable: "could not prepare the passing result for delivery: testing group adapter-canary has no successful terminal outer attempt". Engine defect: for a lane-charged run, the PrepareSuccess in cmd/metasystem/test.go passes prepared.Installation (the temp execution worktree) to landing.PrepareTestingReceiptPayload, while the attempt record lives under prepared.ProofControlRoot() (the lane checkout), so ReadAttempt fails every time. Rejoin once that is fixed; no change to this member is needed.; Next: land branch goal/fleet-card-follows-the-simple-lane through the landing lane with work land. Done when: the card shows the lane's real state on the Fleet page after the landing.
- OpenedAt: 2026-10-01T09:44:44Z
- Revision: 10
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-01T09:44:48Z revision=2 opid=TSXV92DA1Z9GN8CMMQ4RXAZ14Q-ui-31a738e9 authority=proven digest=9f78b888b61aa7e2d8d673d604fb01672aee919e13d4816a008b5f8dc4427462 episode=2
- Claimed: machine=landing lineage=landing-lane at=2026-10-01T19:01:15Z revision=9 accountingRevision=9 episodeAt=2026-10-01T19:01:15Z episodeRevision=9
- HandedOver: fromMachine=ui fromLineage=main-1790664507-87928-61899b fromEpoch=2 batch=g9zrqwcx908fetwskjz83nf99a
- StopCapability: generation=9 revision=9 machine=landing claimEpoch=1 fenceEpoch=0

History:
- 2026-10-01T09:44:44Z 3RP95HKHQF8WMP5NVR3NE10583-ui-31a738e9 open actor=human:Wido targets=fleet-card-follows-the-simple-lane
- 2026-10-01T09:44:48Z TSXV92DA1Z9GN8CMMQ4RXAZ14Q-ui-31a738e9 approve actor=human:Wido targets=fleet-card-follows-the-simple-lane
- 2026-10-01T09:45:01Z P72V75XR81FXP2NM7VQRW6NM1Q-ui-d4023bb7 claim actor=ui+main-1790664507-87928-61899b targets=fleet-card-follows-the-simple-lane
- 2026-10-01T11:23:45Z 1D4S592VWVXB8D6CA5CZNTQ2AS-ui-43182c96 breach-stop actor=ui+goal-stop-custodian targets=fleet-card-follows-the-simple-lane
- 2026-10-01T14:48:21Z 6Z3GWZZXDEA0DXE6ME1BPW3JFR-ui-31a738e9 resume actor=human:Wido targets=fleet-card-follows-the-simple-lane
- 2026-10-01T14:53:06Z KJRCX6SZPCKQ7HZXT9GJT8APX9-ui-d4023bb7 handover actor=ui+main-1790664507-87928-61899b targets=fleet-card-follows-the-simple-lane
- 2026-10-01T17:48:59Z 92D2BM90H00V1MZJ8MPJV1NG9D-landing-106adb03 edit actor=landing+landing-lane targets=fleet-card-follows-the-simple-lane
- 2026-10-01T17:49:04Z BHKK8WA3FPWW1G127N20K5A2D7-landing-106adb03 release actor=landing+landing-lane targets=fleet-card-follows-the-simple-lane
- 2026-10-01T19:01:15Z P2QBAQ5AX7XJNW2272DRQFFSGA-ui-d4023bb7 claim actor=ui+main-1790664507-87928-61899b targets=fleet-card-follows-the-simple-lane
- 2026-10-01T19:59:59Z NFATKMEY7JEEMHA856514JAAYG-ui-d4023bb7 handover actor=ui+main-1790664507-87928-61899b targets=fleet-card-follows-the-simple-lane
Integrity: sha256=74c4b0ddfefb8957140502fa27bac80b182b3b165340c8849c7c05f340a4e4d1
