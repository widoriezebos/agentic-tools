# the-lane-finishes-tests-its-page-names

- State: approved
- Risk: severity=1 novelty=1 exposure=1 accumulation=1 basis="Tests only: three test gaps named by Astra in its whole-branch read of the-lane-finishes-what-it-starts (2026-11-09 08:20), no production change; the landed code was read and proven"
- Tier: 1
- Intent: The three tests the page of the-lane-finishes-what-it-starts names and Astra found missing or too weak exist and reject their mutations: TestRebaseReviewStopKeepsTheReadAuthoritative (rebase, review, stop through the public commands; the original read stays authoritative); the admission-lock test fails when the examined-main check moves outside the lock; the scoped-clear fixture has the flake row (one counted pass, the flake stays open).
- Origin: human
- Next step: One unit, tests only, from the page of the-lane-finishes-what-it-starts (test rows at its lines 170-178) and designs/astra-finishes-impl-review.findings.md findings 2, 3 and 4; each test runs through the production composition with injected clocks; name the mutation that fails each.
- OpenedAt: 2026-11-09T11:55:57Z
- Revision: 2
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-11-09T11:56:09Z revision=2 opid=191HX9HSPQVD9SVWHVNR8N91KN-m1e-3844a5e3 authority=proven digest=a608c9169a6534c8ba07f79959561ecdc923111ed9e3f4180df79acf31129a62 episode=2

History:
- 2026-11-09T11:55:57Z BHS4PSZ4PJDMV5JJRWFG3V4DXS-m1e-3844a5e3 open actor=human:Wido targets=the-lane-finishes-tests-its-page-names
- 2026-11-09T11:56:09Z 191HX9HSPQVD9SVWHVNR8N91KN-m1e-3844a5e3 approve actor=human:Wido targets=the-lane-finishes-tests-its-page-names
Integrity: sha256=29876739b986ed004c41f6511a40f3d95260bd967e26e411dabb05af73669965
