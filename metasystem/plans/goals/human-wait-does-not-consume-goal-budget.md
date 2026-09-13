# human-wait-does-not-consume-goal-budget

- State: approved
- Priority: 1
- Sequence: 41
- Risk: severity=3 novelty=2 exposure=3 accumulation=1 basis="severity 3: budget enforcement can stop lawful work or permit unbounded time; novelty 2: a bounded change to existing ledger and elapsed projection; exposure 3: shared goal execution across seats; accumulation 1: each episode is evaluated independently, with no new aggregate accounting"
- Tier: 3
- Intent: Time a goal spends blocked on a recorded human act does not count toward its elapsed limit. Show waiting time separately and report waiting on the human since the recorded start rather than counting down to a breach. Define reliable evidence for human-wait start/end around existing next-step or plan records, goal asks and refused human verbs, without allowing arbitrary agent prose to create unrecorded free execution time.
- Origin: main
- Next step: Third independent successor of goal:breach-clock-and-budget-honesty, preserving the requirement added 2026-09-09 after its reviewed design. Specimen: breach-stop-wedges-seat claimed 10:07Z with 4h, next step WAITING ON WIDO from 11:47Z, custodian stopped it 16:08Z at 6h01m including the wait; the requested raise then required a resume. Needs its own bounded Codex Astra xhigh design for authoritative interval records, end/resume rules, subtraction and reporting through existing goal and ProjectBudget owners. Depends on a stable elapsed origin from goal:budget-raises-preserve-elapsed-origin. No implementation in the elapsed-origin slice.
- OpenedAt: 2026-09-11T05:37:49Z
- Revision: 4
- Labels: breach-clock-successor
- Pinned: m1c
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-13T07:45:09Z revision=3 opid=P9Y8R8HW09N69R50DW6NJDCVBT-m1e-c6925449 authority=proven digest=f3d1f7dc879d867ba326b5c9f93a6bb778cc783b54ccbbb854bd9cb71fe27d77

History:
- 2026-09-11T05:37:49Z 0JFW3QGPK4HM1ZGPW2XHP5411V-m1c-66a02980 open actor=m1c+main-1789023008-75159-b69143 targets=human-wait-does-not-consume-goal-budget
- 2026-09-13T07:45:05Z H6FRBQJBFKEXEKG6RTZAA2017F-m1e-c6925449 set-pin actor=human:Wido targets=human-wait-does-not-consume-goal-budget
- 2026-09-13T07:45:09Z P9Y8R8HW09N69R50DW6NJDCVBT-m1e-c6925449 approve actor=human:Wido targets=human-wait-does-not-consume-goal-budget
- 2026-09-13T07:45:14Z BQBYNH25Y7X8WT7YZ900C17NAK-m1e-c6925449 set-priority actor=human:Wido targets=human-wait-does-not-consume-goal-budget reason=priority-order subject=human-wait-does-not-consume-goal-budget from=unranked to=1:41 requested-sequence=41
Integrity: sha256=321db7e80648c09d779787d0eeb29f222580f53bf7ed22b6c6741b2f4b9f7c5a
