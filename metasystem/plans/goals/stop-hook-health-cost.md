# stop-hook-health-cost

- State: approved
- Risk: severity=2 novelty=1 exposure=3 accumulation=1 basis="severity 2: an expired deadline refuses a turn end that was safe, which costs a seat a full turn and teaches it to distrust the refusal, but nothing unsafe is permitted; novelty 1: this is profiling and caching inside one existing verb, not new machinery; exposure 3: every seat on every machine pays this cost at every turn end; accumulation 1: first time this has been measured, though it is a plausible contributor to refusals already blamed on other causes"
- Tier: 3
- Intent: The Stop hook gets four seconds and one health call spends two of them: metasystem health --hook-preview costs a steady 2.0s on m1 (measured three times, 2.04/2.01/2.07), and the hook also pays turn-verdict at 0.83s plus arming, digest, watchdog, evidence and sha256 work inside the same budget. On 2026-09-05 that budget expired and the turn end was refused with 'Stop deadline expired before a safe turn verdict'. DONE means the hook's health input costs a small fraction of its budget, proven by a measurement in a fixture rather than by hand, with the refusal path unchanged when the budget really is exceeded
- Origin: main
- Next step: Measured on m1 2026-09-05 but not diagnosed: health --hook-preview is a steady 2.0s while goal fetch is 0.5s and proc probe is 0.006s, so the cost is inside health's own component set rather than the ledger read. Next: profile which health components dominate (the spend fence prices 647 usage records and the job scan reads 112 records are the first suspects), then cache or bound the expensive one inside the hook-preview path so the whole Stop hook fits its budget with headroom; fixture: a hook-preview under the fixture's record volume completes under one second. SIGHTING 2026-09-06 (m1, during a builder round plus a host suite run): the seat's own Stop hook expired its deadline twice, 07:19:14Z and 07:22:58Z, each a single 'stop response outcome=deadline-expired-block' line followed by an orphaned evidence-gc block - the worker was killed mid evidence-gc on a loaded machine. Load on the host is a normal condition when nodes build; the budget must hold under it.
- OpenedAt: 2026-09-05T10:09:49Z
- Revision: 7
- Pinned: m1d
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T06:53:37Z revision=2 opid=A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f authority=proven digest=27d89680efeefcda7f405f47ec025d549e7d1d2656474c2a74773d1c6fe39018
- Sliced: machine=m1d lineage=main-1788683763-71870-f7f607 revision=5 at=2026-09-06T10:52:15Z

History:
- 2026-09-05T10:09:49Z YM1KHGTB9X8C64HM9WQJ3KJNZ5-m1-a4f8999f open actor=m1+main-1788594343-3833-fb64b9 targets=stop-hook-health-cost
- 2026-09-06T06:53:37Z A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f approve actor=human:Wido targets=stop-hook-health-cost reason=sweep
- 2026-09-06T07:32:50Z 32TJK7X84HNNCD6KDECQSYW1DC-m1-a4f8999f edit actor=m1+main-1788594343-3833-fb64b9 targets=stop-hook-health-cost
- 2026-09-06T08:37:50Z Y8FP48VVV599P4B596DA0TA0S8-m1-a4f8999f set-pin actor=human:Wido targets=stop-hook-health-cost
- 2026-09-06T10:51:17Z JD3QXB9P6EBPWWZ706R398MCDJ-m1d-62183579 claim actor=m1d+main-1788683763-71870-f7f607 targets=stop-hook-health-cost
- 2026-09-06T10:52:15Z 3454EVH0XWXZ512RVC3QW1ENT9-m1d-62183579 slice-start actor=m1d+main-1788683763-71870-f7f607 targets=stop-hook-health-cost
- 2026-09-06T12:40:46Z M76NM2PW0DKWQ9AKNM0CEN247Z-m1d-62183579 release actor=m1d+main-1788683763-71870-f7f607 targets=stop-hook-health-cost
Integrity: sha256=6b9df6284df73ec6dccdec1abc115bbbc8e7de85e25f355267ec4d7b1d583bf1
