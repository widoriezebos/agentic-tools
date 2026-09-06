# suite-outcomes-as-steward-incidents

- State: approved
- Intent: A red or vanished validation run is an incident the steward notices and acts on, not a log line only the operator's ad-hoc watcher sees (Wido 2026-08-24: an agent should have noticed and acted — none did)
- Origin: human
- Next step: Appetite: 2h. Suites register a run record with the steward at start and stamp the outcome at exit; the steward's tick treats a red outcome or a dead unstamped run as an incident: revive/notify per its existing verdict machinery. Acceptance: a battery killed mid-run or exiting red raises a steward incident visible in the stop message without any hand-armed watcher.
- OpenedAt: 2026-08-24T13:24:28Z
- Revision: 4
- Labels: custody
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=240 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T06:53:37Z revision=4 opid=A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f authority=proven digest=810968272fab2cfa905722de76984cf43bca3558254e1bc4cf146934c28b6f7e

History:
- 2026-08-24T13:24:28Z PXXTRB93CM8QY3NT8XBRXRYNY9-m2-bc1be9cb open actor=human:wido targets=suite-outcomes-as-steward-incidents
- 2026-08-26T05:40:26Z 2NVQ1RT427KX1VC5FK3ZD6V7AD-m2-bc1be9cb edit actor=m2+mac-coordinator targets=suite-outcomes-as-steward-incidents
- 2026-09-01T20:27:33Z 96FC0JCNZAFDKHE545G3K8XKMT-m0b-6638932d set-budget actor=m0b+main-1788250419-3170380-8a1fb3 targets=suite-outcomes-as-steward-incidents
- 2026-09-06T06:53:37Z A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f approve actor=human:Wido targets=suite-outcomes-as-steward-incidents reason=sweep
Integrity: sha256=f0253ae3dce6130c747e4f35f8f17d414f553b0801e56c002b8820b9a0e9de7d
