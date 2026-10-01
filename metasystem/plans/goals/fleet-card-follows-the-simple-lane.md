# fleet-card-follows-the-simple-lane

- State: queued
- Risk: severity=1 novelty=1 exposure=1 accumulation=1 basis="interface text and types for one card; read-only view"
- Tier: 1
- Intent: What: The landing lane card on the Fleet page shows the simple lane as it now is: its landing agent running, idle (a ready lane with nothing to land, no prompt), stopped by a person (who and why, and the command to resume it) or unready (the reason and the fix), with no restart count. Why: the simple lane (2026-10-01) replaced the old lane owner, so the card's states and restart count no longer matched what the lane reports. Pros: the card tells the truth about the lane; it is also the first change handed to the new lane from the ui seat. Cons: none of note; the card only reads.
- Origin: human
- Next step: Next: land branch goal/fleet-card-follows-the-simple-lane through the landing lane with work land. Done when: the card shows the lane's real state on the Fleet page after the landing.
- OpenedAt: 2026-10-01T09:44:44Z
- Revision: 1
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0

History:
- 2026-10-01T09:44:44Z 3RP95HKHQF8WMP5NVR3NE10583-ui-31a738e9 open actor=human:Wido targets=fleet-card-follows-the-simple-lane
Integrity: sha256=3519476052699893acee16e914a0f9cce7a35e4433d86bd4424eecbdef75e45a
