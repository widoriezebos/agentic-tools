# a-landing-reads-its-paths-from-the-repository-top

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="Landing machinery: a wrong preimage check either refuses good landings (today) or, if loosened wrongly, admits work whose base moved; one helper in the landing path, no data migration, covered by a real-layout test."
- Tier: 2
- Intent: A hand or exception landing composed in a checkout whose installation is a subdirectory of its repository (metasystem/) admits a unit whose files main has not changed: the preimage check (internal/goal/branch/land.go treeEntry, and attest.go treeEntryAt) reads each path from the repository top, so it no longer refuses GOAL_UNIT_REREAD "build U no longer applies to main" for every unit that modifies a file (finding 638; it blocks the person-approved exception landing of the-deputy-acts-under-a-recorded-delegation).
- Origin: main
- Next step: One unit (--last): ls-tree --full-tree (or rev-parse TREE:PATH) in treeEntry and treeEntryAt; a test that composes and lands a goal of two units, one modifying an existing file, from an installation in a metasystem/ subdirectory of its repository, red on today's code and green after. Evidence: findings.md 638 (lane-test-20261009), a1's refusal panes/deputy-land-20261116-12.log. After it lands and deploys, the approved GOAL_NOT_FINISHED exception of the-deputy-acts-under-a-recorded-delegation is run again.
- OpenedAt: 2026-11-16T11:15:05Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-11-16T11:15:05Z 5B1JN3Y6GSVXA8TH7BMKK2MTAT-m1e-71c5cb39 open actor=m1e+steward-seat targets=a-landing-reads-its-paths-from-the-repository-top,the-deputy-acts-under-a-recorded-delegation
Integrity: sha256=96fcbef797c2fa056d2654322619dd61d60c2458298fb63f4deaf01d32a02bdb
