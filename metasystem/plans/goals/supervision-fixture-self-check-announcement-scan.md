# supervision-fixture-self-check-announcement-scan

- State: queued
- Risk: severity=1 novelty=1 exposure=1 accumulation=1 basis="One scan in the supervision suite's self-check misreads two pidless bookkeeping files as main announcements; only the suite's own verdict is affected."
- Tier: 1
- Intent: scripts/agents/supervision-fixtures.sh, assert_fixture_supervision_isolation: the announcement scan treats every JSON file under a harness root's mains directory as a main announcement, but the protocol cursor and the reaped-after-claim stamp carry no pid, so the scan can misjudge them (critic finding SCC-61 of chain scp-build1-cc3); the operator-layout scenario's harness root is one level below its state root so the scan looks in a mains directory that does not exist (SCC-62); the ancestor walk under set -e can flake when an ancestor exits mid-walk (SCC-65). DONE means the scan reads only announcement files that carry a pid, the harness root is the state root, the walk tolerates an exiting ancestor, and the self-check fails a scenario that arms with the seat's pid.
- Origin: main
- Next step: One fixture round with the suite run seat-side; tier 1.
- OpenedAt: 2026-09-05T14:23:36Z
- Revision: 1
- Labels: robustness
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0

History:
- 2026-09-05T14:23:36Z X5D25EBMSK55W3G1Z4K4NBBHAR-m2-5fcf08ab open actor=human:Wido targets=supervision-fixture-self-check-announcement-scan
Integrity: sha256=0ba2e0afd8fa98ecffb18dc4548f9cd7771303aca70eead6807b14cd77fcd3a4
