# steward-tick-load-flake

- State: queued
- Intent: TestRunLoopTicksUntilTheStopFile flakes under load (wall-clock patience assumption) - m2's finding from the night of 2026-08-29, on the m1 steward seam
- Origin: main
- Next step: Appetite: 1h. Make the test patience load-tolerant the way the suite-custody work did elsewhere: condition-based waiting on the tick evidence, never wall-clock sleeps; prove by running the steward suite under artificial load
- OpenedAt: 2026-08-30T14:57:28Z
- Revision: 2
- Budget: elapsedLimit=3d attemptLimit=4 reservedJobMinutesLimit=240 activeJobLimit=1

History:
- 2026-08-30T14:57:28Z J8H2SS5TP3H03J240G5JGS3AMA-m1-bf243850 open actor=m1+coordinator targets=steward-tick-load-flake
- 2026-08-30T15:17:17Z AK0NK66HGBV8Q8RPQ4XB540QX6-m1-bf243850 set-budget actor=m1+coordinator targets=steward-tick-load-flake
Integrity: sha256=18dc5fd5fa040d5ce69332af4d9780fc2bc256bd15865a7fac9ae3a9fbc9a1de
