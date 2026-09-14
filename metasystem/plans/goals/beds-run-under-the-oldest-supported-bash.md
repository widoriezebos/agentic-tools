# beds-run-under-the-oldest-supported-bash

- State: queued
- Risk: severity=2 novelty=1 exposure=3 accumulation=2 basis="severity 2: a bed that cannot start hides every scenario in it; novelty 1: a known portability class; exposure 3: every bed on every Mac seat; accumulation 2: each new array risks it again"
- Tier: 2
- Intent: Fixture beds must run on the oldest shell a supported machine ships. On 2026-09-14 a new supervision bed expanded an empty array under set -u; macOS ships bash 3.2, which rejects that, so the whole bed died in the second it started while the build's sandbox, on a newer bash, saw nothing. The same class recurs whenever a bed adds an array. DONE: the gate parses and exercises every fixture bed's risky constructs under /bin/bash 3.2 semantics and refuses a bed that would fail there, naming the file and line.
- Origin: human
- Next step: Add a gate check that runs bash -n under the oldest supported bash and a static scan for empty-array expansion under set -u and other 3.2-incompatible constructs; prove it catches the 2026-09-14 line-45 defect.
- OpenedAt: 2026-09-14T16:16:20Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-14T16:16:20Z 0R2JYA4MM81F6BQDPSSP9KEJBH-m1e-c6925449 open actor=human:Wido targets=beds-run-under-the-oldest-supported-bash
Integrity: sha256=b2b3f8262501db2a6c4432c8f368e4687fd71cdafa4a9bd559e55eaf47f041a4
