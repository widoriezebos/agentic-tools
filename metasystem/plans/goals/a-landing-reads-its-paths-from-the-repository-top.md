# a-landing-reads-its-paths-from-the-repository-top

- State: approved
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="Landing machinery: a wrong preimage check either refuses good landings (today) or, if loosened wrongly, admits work whose base moved; one helper in the landing path, no data migration, covered by a real-layout test."
- Tier: 2
- Intent: A hand or exception landing composed in a checkout whose installation is a subdirectory of its repository (metasystem/) admits a unit whose files main has not changed: the preimage check (internal/goal/branch/land.go treeEntry, and attest.go treeEntryAt) reads each path from the repository top, so it no longer refuses GOAL_UNIT_REREAD "build U no longer applies to main" for every unit that modifies a file (finding 638; it blocks the person-approved exception landing of the-deputy-acts-under-a-recorded-delegation).
- Origin: main
- Next step: One unit (--last): ls-tree --full-tree (or rev-parse TREE:PATH) in treeEntry and treeEntryAt; a test that composes and lands a goal of two units, one modifying an existing file, from an installation in a metasystem/ subdirectory of its repository, red on today's code and green after. Evidence: findings.md 638 (lane-test-20261009), a1's refusal panes/deputy-land-20261116-12.log. After it lands and deploys, the approved GOAL_NOT_FINISHED exception of the-deputy-acts-under-a-recorded-delegation is run again.; ASKED W80R25E7CKET6N02JC51ZXGWGZ (other): The deputy's exception landing is refused by an engine path bug, not by a stale branch, so no rebase can cure it. Approve the one-unit fix goal a-landing-reads-its-paths-from-the-repository-top, built on m1e before a-runner-is-judged-by-its-progress?
- OpenedAt: 2026-11-16T11:15:05Z
- Revision: 4
- Pinned: m1e
- Budget: elapsedLimit=2d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- NormApproval: approvedRef=577PVWB45JA7SVSSD0QAKQDVAM-m1e-58d3d1e9 minutes=1200 reviewRounds=20 goalRevision=2
- Approved: by=human:Wido at=2026-11-16T12:14:17Z revision=3 opid=577PVWB45JA7SVSSD0QAKQDVAM-m1e-58d3d1e9 authority=proven digest=3e9250ca1083e3ed03ecab2b7fd08f29435f16e5cf54dcda956d0eaa601d05a2 episode=3

History:
- 2026-11-16T11:15:05Z 5B1JN3Y6GSVXA8TH7BMKK2MTAT-m1e-71c5cb39 open actor=m1e+steward-seat targets=a-landing-reads-its-paths-from-the-repository-top,the-deputy-acts-under-a-recorded-delegation
- 2026-11-16T11:15:52Z 4M715YFENKEYV58F19XN6GT2KB-m1e-71c5cb39 ask actor=m1e+steward-seat targets=a-landing-reads-its-paths-from-the-repository-top
- 2026-11-16T12:14:17Z 577PVWB45JA7SVSSD0QAKQDVAM-m1e-58d3d1e9 approve actor=human:Wido targets=a-landing-reads-its-paths-from-the-repository-top
- 2026-11-16T12:14:58Z 6CYXMYZAF3MN6CA3M21H88HSQG-m1e-58d3d1e9 set-pin actor=human:Wido targets=a-landing-reads-its-paths-from-the-repository-top
Integrity: sha256=e2bd08c62f66140960a08bd8fe8aa03c8ae710d8cdb9d8cb5cbed6706fbd765c
