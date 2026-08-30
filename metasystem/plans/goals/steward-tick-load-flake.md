# steward-tick-load-flake

- State: queued
- Intent: TestRunLoopTicksUntilTheStopFile flakes under load (wall-clock patience assumption) - m2's finding from the night of 2026-08-29, on the m1 steward seam
- Origin: main
- Next step: Appetite: 1h. Make the test patience load-tolerant the way the suite-custody work did elsewhere: condition-based waiting on the tick evidence, never wall-clock sleeps; prove by running the steward suite under artificial load
- OpenedAt: 2026-08-30T14:57:28Z
- Revision: 1

History:
- 2026-08-30T14:57:28Z J8H2SS5TP3H03J240G5JGS3AMA-m1-bf243850 open actor=m1+coordinator targets=steward-tick-load-flake
Integrity: sha256=5bf11e288e3011b0ea3b83a1fc0429b2d47b4132b56cc94ecf4a5b1be603cec5
