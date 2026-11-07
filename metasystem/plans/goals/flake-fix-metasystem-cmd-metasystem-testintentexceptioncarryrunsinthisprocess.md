# flake-fix-metasystem-cmd-metasystem-testintentexceptioncarryrunsinthisprocess

- State: approved
- Priority: 1
- Sequence: 128
- Risk: severity=1 novelty=1 exposure=2 accumulation=1 basis="A test bed change only; nothing gains authority; a wrong fix shows as the same red in the lane and is returned."
- Tier: 1
- Intent: the test TestIntentExceptionCarryRunsInThisProcess in metasystem/cmd/metasystem failed once in the landing lane with a temp-dir cleanup race (TempDir RemoveAll: .git directory not empty) and was green on every repeat and on main; make its bed leave nothing writing after the test ends (git gc.auto=0 and maintenance.auto=false as other beds set, and no detached child outliving the test), so it holds under load without a wall clock (R-104)
- Origin: main
- Next step: reproduce under -count=20 -p 4 with the package in a workspace at origin/main; find what writes into the bed git directory after the test body (a detached land child or git background maintenance, finding 449); fix the bed, prove 20 of 20 green, hand in through the lane; m1e, after the-steward-sees U0 Fixes analysis 2026-11-07 (Astra r2): diagnose first. Three incidents share the TempDir RemoveAll .git signature (TestSevenDayMainOneEpisode, TestTheHolderIsAskedOncePerBlockingOccurrence and its subtest). Trace each fixture effective git config (seatGit sets GIT_CONFIG_GLOBAL=/dev/null, seatpresence_test.go:332) and any detached child; repair at the owner of the writer. Root (Explore 2026-11-07 16:1x): detached automatic git maintenance (git 2.54 default, geometric) after the production fetch at internal/pattern/trunk.go:46 and after fixture commits writes commit-graphs and info into the bed .git after the test ends; GIT_CONFIG_GLOBAL=/dev/null does not disable it. Fix per bed, repo-local right after init: git config --local maintenance.auto false and gc.auto 0 in internal/pattern bed_test.go:41 (newBed), internal/steward seatpresence_tick_test.go:35 (gitRepoWithCurrentGoal) and seatpresence_test.go:322 (seatEnroll). Claims the three TempDir incidents.
- OpenedAt: 2026-10-30T08:57:40Z
- Revision: 7
- Pinned: m1f
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-30T08:57:49Z revision=2 opid=F60JJ95ZA4J3FN2Z4CEYX9JVE2-m1e-afd9d95a authority=proven digest=c8367920c6bd7f7008352afdcfc2bf6dc12b28a5303ff28d71b23c8e5f9278d6 episode=2

History:
- 2026-10-30T08:57:40Z TJTNHCGNK2TJZCN6QZH9H7CJ4G-m1e-afd9d95a open actor=human:Wido targets=flake-fix-metasystem-cmd-metasystem-testintentexceptioncarryrunsinthisprocess
- 2026-10-30T08:57:49Z F60JJ95ZA4J3FN2Z4CEYX9JVE2-m1e-afd9d95a approve actor=human:Wido targets=flake-fix-metasystem-cmd-metasystem-testintentexceptioncarryrunsinthisprocess
- 2026-10-30T08:57:57Z 4QHQ1A4SHJ7F6705WXK077KNQ5-m1e-afd9d95a set-pin actor=human:Wido targets=flake-fix-metasystem-cmd-metasystem-testintentexceptioncarryrunsinthisprocess
- 2026-11-07T09:56:21Z N9S4QNMX92Y6ZDGG65FW320P2X-m1e-1dae276b edit actor=human:Wido targets=flake-fix-metasystem-cmd-metasystem-testintentexceptioncarryrunsinthisprocess
- 2026-11-07T15:57:58Z RKVJM6XT5RR863279Z3PQDDQR5-m1e-9f79adc8 edit actor=human:Wido targets=flake-fix-metasystem-cmd-metasystem-testintentexceptioncarryrunsinthisprocess
- 2026-11-07T15:58:40Z SCBF1Y56N2EC81463HH14ND0J7-m1e-9f79adc8 set-pin actor=human:Wido targets=flake-fix-metasystem-cmd-metasystem-testintentexceptioncarryrunsinthisprocess
- 2026-11-07T15:58:54Z HJ4BEXTGFF97G3V3N2AATV2J0K-m1e-9f79adc8 set-priority actor=human:Wido targets=flake-fix-metasystem-cmd-metasystem-testintentexceptioncarryrunsinthisprocess reason=priority-order subject=flake-fix-metasystem-cmd-metasystem-testintentexceptioncarryrunsinthisprocess from=unranked to=1:128 requested-sequence=append
Integrity: sha256=e6a96f886af73f667214bc05a898c5dd7b69893beb6ecc57113e16adf6b2b872
