# steward-tick-load-flake

- State: claimed
- Intent: TestRunLoopTicksUntilTheStopFile flakes under load (wall-clock patience assumption) - m2's finding from the night of 2026-08-29, on the m1 steward seam
- Origin: main
- Next step: Appetite: 1h. Make the test patience load-tolerant the way the suite-custody work did elsewhere: condition-based waiting on the tick evidence, never wall-clock sleeps; prove by running the steward suite under artificial load
- OpenedAt: 2026-08-30T14:57:28Z
- Revision: 3
- Budget: elapsedLimit=3d attemptLimit=4 reservedJobMinutesLimit=240 activeJobLimit=1
- Claimed: machine=m0 lineage=main-1788178136-1684505-4ffe42 at=2026-08-31T13:09:26Z revision=3
- StopCapability: generation=3 revision=3 machine=m0 claimEpoch=1 fenceEpoch=0

History:
- 2026-08-30T14:57:28Z J8H2SS5TP3H03J240G5JGS3AMA-m1-bf243850 open actor=m1+coordinator targets=steward-tick-load-flake
- 2026-08-30T15:17:17Z AK0NK66HGBV8Q8RPQ4XB540QX6-m1-bf243850 set-budget actor=m1+coordinator targets=steward-tick-load-flake
- 2026-08-31T13:09:26Z ZCQ173VB6W6WMEDEQ68PKK4H25-m0-c5dbf036 claim actor=m0+main-1788178136-1684505-4ffe42 targets=steward-tick-load-flake
Integrity: sha256=4390b32787c97f3ac3f780b5f729b0e1970cf8230407b78f8e1ad920b7c7d402
