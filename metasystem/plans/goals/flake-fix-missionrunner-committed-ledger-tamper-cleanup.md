# flake-fix-missionrunner-committed-ledger-tamper-cleanup

- State: queued
- Risk: severity=1 novelty=1 exposure=1 accumulation=1 basis="finding 467; proof log 20261101T083747.857701000Z line 161038; the lane steward opens the same goal at tier 3 priority 1 from two sightings (internal/landing/plain/steward_flake.go:48-66)"
- Tier: 3
- Intent: the test TestGitAdapterCommittedLedgerTamperDoesNotReplaceAnchorTruth in metasystem/internal/missionrunner failed once in the landing lane (proof 20261101T083747, 09:49:30, load about 11) with TempDir RemoveAll cleanup: unlinkat .../bed/checkout/.git: directory not empty, and was green on the named replay of main; a git child of the bed was still writing .git when the test cleanup ran. Make the bed end its git children before the cleanup and hold under load without a wall clock (R-104: artificial clocks, never load-fragile tests). The lane called this red the goal's own (finding 467), so no sighting reached the register; this goal stands in for the one the lane steward would have opened at the second sighting.
- Origin: human
- Next step: fix the test bed in metasystem/internal/missionrunner/verification_git_adapter_test.go, run the test 20 times under -p 4 with its package, hand it in
- OpenedAt: 2026-11-01T10:03:05Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-11-01T10:03:05Z 58F681F503WJ50PPT39DNWGPEY-m1e-89919481 open actor=human:Wido targets=flake-fix-missionrunner-committed-ledger-tamper-cleanup reason=TierOverride: derived=1 set=3 why=the lane steward opens its flake-fix goals at tier 3 (internal/landing/plain/steward_flake.go:63); this goal stands in for that one
Integrity: sha256=27fd522e8a6b82bee8a30563ac7a82b4f9a44e18e2ce806916d46c31e6c75286
