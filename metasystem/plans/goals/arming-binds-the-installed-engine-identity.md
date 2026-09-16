# arming-binds-the-installed-engine-identity

- State: queued
- Risk: severity=2 novelty=2 exposure=3 accumulation=1 basis="severity 2: a run can execute a binary the seat did not build without any record saying so; novelty 2: identity binding on top of projection comparison is new; exposure 3: every proof run; accumulation 1: the arming record and one check"
- Tier: 2
- Intent: The arming record and the drift check compare engine PROJECTIONS, so a stale engine (older tree) is caught but a binary built by another seat at the same tree projects identically and is invisible. On 2026-09-16 two seats each believed the other had installed a binary into their checkout and the records could not tell them; the answer came from file sizes, digests and build stamps read by hand. DONE: the arming record binds the installed binary identity (build stamp, digest, installing seat and time); a run against a binary whose identity differs from the arming record refuses or records loudly, naming both identities; a fixture installs a byte-different build of the same tree and shows the refusal; a same-seat reinstall of its own build is admitted silently; proven on two runtimes.
- Origin: human
- Next step: Read the arming record writer and the drift comparison (engine-policy-binding design r2 names them); add identity fields and the comparison as one unit with both witnesses; Opus read; land. Coordinate with engine-policy-binding-survives-drift-and-load so the seam is named once.
- OpenedAt: 2026-09-16T06:43:19Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-16T06:43:19Z XC5NW53TNHSRSHVJ081AZ5S6YE-m1e-c6925449 open actor=human:Wido targets=arming-binds-the-installed-engine-identity
Integrity: sha256=7d6fd0a696f061bc7729176613ccc0ebab092d765c59c312034236850f9167ae
