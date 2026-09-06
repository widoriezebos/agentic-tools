# stop-hook-health-cost

- State: done
- Risk: severity=2 novelty=1 exposure=3 accumulation=1 basis="severity 2: an expired deadline refuses a turn end that was safe, which costs a seat a full turn and teaches it to distrust the refusal, but nothing unsafe is permitted; novelty 1: this is profiling and caching inside one existing verb, not new machinery; exposure 3: every seat on every machine pays this cost at every turn end; accumulation 1: first time this has been measured, though it is a plausible contributor to refusals already blamed on other causes"
- Tier: 3
- Intent: The Stop hook gets four seconds and one health call spends two of them: metasystem health --hook-preview costs a steady 2.0s on m1 (measured three times, 2.04/2.01/2.07), and the hook also pays turn-verdict at 0.83s plus arming, digest, watchdog, evidence and sha256 work inside the same budget. On 2026-09-05 that budget expired and the turn end was refused with 'Stop deadline expired before a safe turn verdict'. DONE means the hook's health input costs a small fraction of its budget, proven by a measurement in a fixture rather than by hand, with the refusal path unchanged when the budget really is exceeded
- Origin: main
- Next step: DIAGNOSED AND BUILT, IN REVIEW FOLD (m1d, 2026-09-06). The two seconds are the spend fence: at every turn end spend.Measure re-reads every job record and re-parses every Claude transcript whose directory name merely starts with the checkout's slug (44 directories on this host for the m1 checkout; two for a fresh one, hence 2.0 s versus 0.3 s), and the seat's own transcript grows every turn. Chain shhc-build1-20260906 round 1 (Sol): a duration per health role in the verdict JSON, transcript directories chosen by content (first cwd line), an incremental per-transcript cursor with byte-identical results, a cache for terminal job records, a measurement test (700 jobs, 40 transcripts: 543 ms cold, 48 ms warm). The review (Fable, shhc-cc1) found six material defects, two of them caches that could hide spend (a cache write failure shrank the number; a pending job outcome was cached forever), plus the seat's own outside-sandbox finding: a health read waits without bound on the steward tick's evidence lock, so a tick stopped or slow inside its durable write freezes every health call and every Stop hook behind it (the likely other cause of the deadline expiries; it hung the health fixture suite on every seat that ran it today). All seven accepted; dispositions plans/dispositions/stop-hook-health-cost-code-critique-r1.md; round 2 running (fold brief plans/stop-hook-health-cost-fold2-brief.md, which also re-wires the stop-hook-duration role that slice 2 of stop-hook-budget-is-ours landed under this chain). Then: outside-sandbox replay of both fixture suites, closing review, land, rebuild engines. Toolchain: GOTOOLCHAIN=go1.26.5 for gates and landings on this host until go-toolchain-pinned-in-go-mod lands.
- Concluded: The Stop hook's health preview now costs a small fraction of its budget: the spend fence reads only this checkout's transcripts, parses only new bytes, caches settled job outcomes, and a health read never waits without bound on the steward tick's lock (the hang that froze every health call and Stop hook behind a slow or stopped tick, and the health fixture suite on every seat today). Noted for the watch: a component whose writer holds its lock over 200 ms reads unknown for that tick and two in a row alert; a change in the delegate session set re-parses transcripts once.
- OpenedAt: 2026-09-05T10:09:49Z
- Revision: 10
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
- 2026-09-06T14:44:47Z G2Q924Z9YJYYTJ7AH49GK84KX8-m1d-62183579 claim actor=m1d+main-1788683763-71870-f7f607 targets=stop-hook-health-cost
- 2026-09-06T14:47:27Z YAMF6BQXFZMKWQK8DHV0PB57FY-m1d-62183579 edit actor=m1d+main-1788683763-71870-f7f607 targets=stop-hook-health-cost
- 2026-09-06T15:30:08Z 580XN2TMPTEWBH0W6V8KXQNSTD-m1d-62183579 done actor=m1d+main-1788683763-71870-f7f607 targets=stop-hook-health-cost
Integrity: sha256=95a15befc06f858290e1ed210112e21a012891fe27c7d3d91cb0f7bd2401074e
