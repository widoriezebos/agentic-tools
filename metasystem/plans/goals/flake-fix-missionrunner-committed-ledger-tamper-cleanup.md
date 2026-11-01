# flake-fix-missionrunner-committed-ledger-tamper-cleanup

- State: approved
- Risk: severity=1 novelty=1 exposure=1 accumulation=1 basis="finding 467; proof log 20261101T083747.857701000Z line 161038; the lane steward opens the same goal at tier 3 priority 1 from two sightings (internal/landing/plain/steward_flake.go:48-66)"
- Tier: 3
- Intent: the test TestGitAdapterCommittedLedgerTamperDoesNotReplaceAnchorTruth in metasystem/internal/missionrunner failed once in the landing lane (proof 20261101T083747, 09:49:30, load about 11) with TempDir RemoveAll cleanup: unlinkat .../bed/checkout/.git: directory not empty, and was green on the named replay of main; a git child of the bed was still writing .git when the test cleanup ran. Make the bed end its git children before the cleanup and hold under load without a wall clock (R-104: artificial clocks, never load-fragile tests). The lane called this red the goal's own (finding 467), so no sighting reached the register; this goal stands in for the one the lane steward would have opened at the second sighting.
- Origin: human
- Next step: fix the test bed in metasystem/internal/missionrunner/verification_git_adapter_test.go, run the test 20 times under -p 4 with its package, hand it in
- OpenedAt: 2026-11-01T10:03:05Z
- Revision: 2
- Budget: elapsedLimit=5d attemptLimit=24 reservedJobMinutesLimit=2400 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- NormApproval: approvedRef=N061JBB0PJMGH8ASQ6S2V06CJ3-m1e-89919481 minutes=2400 reviewRounds=20 goalRevision=1
- Approved: by=human:Wido at=2026-11-01T10:05:27Z revision=2 opid=N061JBB0PJMGH8ASQ6S2V06CJ3-m1e-89919481 authority=proven digest=9480ca1b2e65acc8c925e35f94bc1543d38b00933d33ab14d6b1bd55173918a3 episode=2

History:
- 2026-11-01T10:03:05Z 58F681F503WJ50PPT39DNWGPEY-m1e-89919481 open actor=human:Wido targets=flake-fix-missionrunner-committed-ledger-tamper-cleanup reason=TierOverride: derived=1 set=3 why=the lane steward opens its flake-fix goals at tier 3 (internal/landing/plain/steward_flake.go:63); this goal stands in for that one
- 2026-11-01T10:05:27Z N061JBB0PJMGH8ASQ6S2V06CJ3-m1e-89919481 approve actor=human:Wido targets=flake-fix-missionrunner-committed-ledger-tamper-cleanup
Integrity: sha256=20c40dc4235cf19a64e2a5432e3b19a03c4651381e0ce1a06deee301696f3358
