# flake-fix-metasystem-cmd-metasystem-testintentexceptioncarryrunsinthisprocess

- State: approved
- Risk: severity=1 novelty=1 exposure=2 accumulation=1 basis="A test bed change only; nothing gains authority; a wrong fix shows as the same red in the lane and is returned."
- Tier: 1
- Intent: the test TestIntentExceptionCarryRunsInThisProcess in metasystem/cmd/metasystem failed once in the landing lane with a temp-dir cleanup race (TempDir RemoveAll: .git directory not empty) and was green on every repeat and on main; make its bed leave nothing writing after the test ends (git gc.auto=0 and maintenance.auto=false as other beds set, and no detached child outliving the test), so it holds under load without a wall clock (R-104)
- Origin: main
- Next step: reproduce under -count=20 -p 4 with the package in a workspace at origin/main; find what writes into the bed git directory after the test body (a detached land child or git background maintenance, finding 449); fix the bed, prove 20 of 20 green, hand in through the lane; m1e, after the-steward-sees U0
- OpenedAt: 2026-10-30T08:57:40Z
- Revision: 3
- Pinned: m1e
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-30T08:57:49Z revision=2 opid=F60JJ95ZA4J3FN2Z4CEYX9JVE2-m1e-afd9d95a authority=proven digest=c8367920c6bd7f7008352afdcfc2bf6dc12b28a5303ff28d71b23c8e5f9278d6 episode=2

History:
- 2026-10-30T08:57:40Z TJTNHCGNK2TJZCN6QZH9H7CJ4G-m1e-afd9d95a open actor=human:Wido targets=flake-fix-metasystem-cmd-metasystem-testintentexceptioncarryrunsinthisprocess
- 2026-10-30T08:57:49Z F60JJ95ZA4J3FN2Z4CEYX9JVE2-m1e-afd9d95a approve actor=human:Wido targets=flake-fix-metasystem-cmd-metasystem-testintentexceptioncarryrunsinthisprocess
- 2026-10-30T08:57:57Z 4QHQ1A4SHJ7F6705WXK077KNQ5-m1e-afd9d95a set-pin actor=human:Wido targets=flake-fix-metasystem-cmd-metasystem-testintentexceptioncarryrunsinthisprocess
Integrity: sha256=050d974b90efbbaf80762f510e8f565f2e8713134bd84091e0c9864cc58ea060
