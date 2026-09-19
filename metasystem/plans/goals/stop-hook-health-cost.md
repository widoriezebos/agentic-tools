# stop-hook-health-cost

- State: queued
- Risk: severity=2 novelty=1 exposure=3 accumulation=1 basis="severity 2: an expired deadline refuses a turn end that was safe, which costs a seat a full turn and teaches it to distrust the refusal, but nothing unsafe is permitted; novelty 1: this is profiling and caching inside one existing verb, not new machinery; exposure 3: every seat on every machine pays this cost at every turn end; accumulation 1: first time this has been measured, though it is a plausible contributor to refusals already blamed on other causes"
- Tier: 3
- Intent: The Stop hook gets four seconds and one health call spends two of them: metasystem health --hook-preview costs a steady 2.0s on m1 (measured three times, 2.04/2.01/2.07), and the hook also pays turn-verdict at 0.83s plus arming, digest, watchdog, evidence and sha256 work inside the same budget. On 2026-09-05 that budget expired and the turn end was refused with 'Stop deadline expired before a safe turn verdict'. DONE means the hook's health input costs a small fraction of its budget, proven by a measurement in a fixture rather than by hand, with the refusal path unchanged when the budget really is exceeded
- Origin: main
- Next step: parked 2026-09-19 in Wido's name (his 16:40 CEST grant, "yes you are allowed to open / approve / whatever in my name while working on this mssion"; m1e sync 14): lower priority than his machine-ready target. Resume only when Wido sets its priority and picks the goal. Existing work: on main 1271fa380 (09-06), the Stop hook health preview stops re-reading the world (metasystem/internal/spend/transcript.go, metasystem/internal/spend/cache.go, metasystem/internal/steward/health.go). Wido reopened the goal on 09-16 (df5c833f0) over a 20-42 s regression; no diagnosis of it is recorded. Left: about 100-400 lines, not verified. No origin goal/stop-hook-health-cost branch exists: nothing unlanded exists; the work below is all on main.
- OpenedAt: 2026-09-05T10:09:49Z
- Revision: 12
- Pinned: m1d
- BudgetExceptions: 0
- Sliced: machine=m1d lineage=main-1788683763-71870-f7f607 revision=5 at=2026-09-06T10:52:15Z

History:
- 2026-09-05T10:09:49Z YM1KHGTB9X8C64HM9WQJ3KJNZ5-m1-a4f8999f open actor=m1+main-1788594343-3833-fb64b9 targets=stop-hook-health-cost
- 2026-09-06T06:53:37Z A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f approve actor=human:Wido targets=stop-hook-health-cost reason=sweep
- 2026-09-06T07:32:50Z 32TJK7X84HNNCD6KDECQSYW1DC-m1-a4f8999f edit actor=m1+main-1788594343-3833-fb64b9 targets=stop-hook-health-cost
- 2026-09-06T08:37:50Z Y8FP48VVV599P4B596DA0TA0S8-m1-a4f8999f set-pin actor=human:Wido targets=stop-hook-health-cost
- 2026-09-06T10:51:17Z JD3QXB9P6EBPWWZ706R398MCDJ-m1d-62183579 claim actor=m1d+main-1788683763-71870-f7f607 targets=stop-hook-health-cost
- 2026-09-06T10:52:15Z 3454EVH0XWXZ512RVC3QW1ENT9-m1d-62183579 slice-start actor=m1d+main-1788683763-71870-f7f607 targets=stop-hook-health-cost
- 2026-09-06T12:40:46Z M76NM2PW0DKWQ9AKNM0CEN247Z-m1d-62183579 release actor=m1d+main-1788683763-71870-f7f607 targets=stop-hook-health-cost
- 2026-09-06T14:44:47Z G2Q924Z9YJYYTJ7AH49GK84KX8-m1d-62183579 claim actor=m1d+main-1788683763-71870-f7f607 targets=stop-hook-health-cost
- 2026-09-06T14:47:27Z YAMF6BQXFZMKWQK8DHV0PB57FY-m1d-62183579 edit actor=m1d+main-1788683763-71870-f7f607 targets=stop-hook-health-cost
- 2026-09-06T15:30:08Z 580XN2TMPTEWBH0W6V8KXQNSTD-m1d-62183579 done actor=m1d+main-1788683763-71870-f7f607 targets=stop-hook-health-cost
- 2026-09-16T09:17:21Z HZH6NQ9RJ49323Q7Q1E8W6EDKP-m1e-c6925449 reopen actor=human:Wido targets=stop-hook-health-cost
- 2026-09-19T15:48:30Z KT2340DTG2C5JW1V04KK9VHFBH-m1e-c6925449 edit actor=human:Wido targets=stop-hook-health-cost
Integrity: sha256=3ed51982ba2dc53867eef3c5e2d05cda4c9f8eee4596b8289fb46d3f1f4f084f
