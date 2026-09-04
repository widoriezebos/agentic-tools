# dispatch-fixture-recollection-pass-budget

- State: queued
- Risk: severity=1 novelty=1 exposure=1 accumulation=2 basis="One fixture wait's attempt budget is too small under load; nothing outside the suite is affected, but every red in this scenario hides the legs behind it."
- Tier: 2
- Intent: scripts/agents/dispatch-fixtures.sh, scenario dispatch: the recollection wait landed in 4ab18db7 counts completed census passes with an attempt budget of two, and on m2 2026-09-05 22:37Z, with a code critic and a build running alongside, it failed with 'recollection did not conclude the delivered-then-lost critic after 2 completed recollection passes (scanSeq 151 -> 154)'. Two passes is not enough headroom when the steward's recollection competes for the machine. DONE means the budget is the number of passes the recollection needs plus two, the silence failsafe stays, and the leg passes with two other jobs running.
- Origin: main
- Next step: One number and its justification in the fixture: run dispatch-fixtures.sh seat-side under load, land through a chain. Not covered by R-76-m2 (opened 2026-09-05) nor R-78-m2; waits for Wido's word.
- OpenedAt: 2026-09-04T22:38:04Z
- Revision: 1
- Labels: robustness
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-04T22:38:04Z PKAXQZ5PAJ4ZFP1614C78RSEYR-m2-5fcf08ab open actor=human:Wido targets=dispatch-fixture-recollection-pass-budget
Integrity: sha256=8b67429cbce55aaa579369320e4731ac525955f1f572c60f3635a398ffb32025
