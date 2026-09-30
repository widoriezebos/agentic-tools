# switch-on-trial

- State: claimed
- Risk: severity=1 novelty=1 exposure=1 accumulation=1 basis="Register-only change (append lines to receipts.log and the narrator digest); no code; reverts cleanly"
- Tier: 1
- Intent: Prove the landing lane end to end at switch-on: land m1e's uncommitted register lines as a goal-bound change
- Origin: human
- Next step: work land switch-on-trial --message with the staged receipts and narrator digest
- OpenedAt: 2026-09-30T08:57:48Z
- Revision: 4
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0
- Approved: by=human:wido at=2026-09-30T08:57:59Z revision=2 opid=NWWGSGH5CZT693973RT0H9YD2A-m1e-b6a4eb0a authority=proven digest=3719166eef0040b090622951c1fe0ea4b2315f94be69812443c6415955114360 episode=2
- Claimed: machine=m1e lineage=main-1790454088-93948-21671b at=2026-09-30T08:58:05Z revision=3 accountingRevision=3 episodeAt=2026-09-30T08:58:05Z episodeRevision=3
- StopCapability: generation=3 revision=3 machine=m1e claimEpoch=9 fenceEpoch=1
- StopFence: stopId=stop-switch-on-trial-r3-f1 revision=3 epoch=1 capabilityGeneration=3 closedAt=2026-09-30T10:36:56Z reason=ELAPSED_LIMIT

History:
- 2026-09-30T08:57:48Z R8BJDZ82WCT5276TRREY12TP9C-m1e-b6a4eb0a open actor=human:wido targets=switch-on-trial
- 2026-09-30T08:57:59Z NWWGSGH5CZT693973RT0H9YD2A-m1e-b6a4eb0a approve actor=human:wido targets=switch-on-trial
- 2026-09-30T08:58:05Z EEDJWQCK3SC1YPZ56F1EY4GKV9-m1e-f456f182 claim actor=m1e+main-1790454088-93948-21671b targets=switch-on-trial
- 2026-09-30T10:36:56Z G4K02BNFJKDX8TMBWVP56PBJ2E-m1e-43182c96 breach-stop actor=m1e+goal-stop-custodian targets=switch-on-trial
Integrity: sha256=6154495659e92bc6da674185f73d7abef36df760d9a4787db67a3595f0e5b392
