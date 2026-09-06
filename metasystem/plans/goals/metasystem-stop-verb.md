# metasystem-stop-verb

- State: claimed
- Risk: severity=2 novelty=1 exposure=3 accumulation=1 basis="severity 2: without an orderly stop a human reaches for kill and leaves locks, leases and half-written records behind, which the custody family then has to clean up; novelty 1: the shutdown path exists in arm-supervision.sh, the verb wraps it; exposure 3: every operator on every machine; accumulation 1: nothing compounds, the gap is the same every day"
- Tier: 3
- Intent: Wido's word 2026-09-06: 'For a human it should be a simple command to stop the meta system. And not only simple but also intuitive.' Today there is none: metasystem delegate --cancel stops one job, steward restart restarts and never stops, and the only shutdown path is scripts/agents/arm-supervision.sh --shutdown, an internal script the fixtures call. DONE means: 'metasystem stop --repo .' (accepted from an agent-free terminal, a human act like arm and restart) stops everything the metasystem runs for that checkout in order - running delegate jobs cancelled through the cancel path, then the steward runner, repo watcher, narrator and supervision components through the existing shutdown path - and prints one line per thing it stopped and one line saying how to start again (steward arm). Idempotent: a second stop says nothing is running. 'metasystem status --repo .' prints the same list without stopping anything. A fleet form (--all for every enrolled checkout on this host) is welcome if cheap; per checkout is the requirement. No kill -9 unless the orderly path fails, and then it says so.
- Origin: main
- Next step: Design ladder: revision 1 (426012505) -> critique r1 (11 material, records/misc/metasystem-stop-critique-r1.md) -> revision 2 (5d1688133, 763 lines) -> critique r2 (8 material, 3 critical, records/misc/metasystem-stop-critique-r2.md, all accepted with dispositions). NEXT: revision 3 folds the eight (design-mode follow-up on chain stopverb-design1-20260906, brief cites the r2 register); it is the LAST design round. Then Sol builds behind section 10 fixtures, Fable code review, land.sh --chain, goal done. Budget: four 120-minute jobs booked of 720; revision 3 + its critique fill the box; the build needs a raise from Wido (over-norm). 2026-09-06 20:3xZ: Wido said 'go headless' for first-headless-run; asked whether the stop verb should finish first; awaiting his word. If headless: park + release so another seat or the runner carries revision 3.
- OpenedAt: 2026-09-06T12:24:49Z
- Revision: 6
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T12:25:28Z revision=2 opid=ESANH6C2BR6VYW74A9Q5NQEWW2-m1-7cd0bd60 authority=proven digest=1c0acb89a08769fa2813f7e1b5f6d986a61a106fa9aa15c8cf9f56cb9ab4cc0a
- Sliced: machine=m1 lineage=main-1788594343-3833-fb64b9 revision=4 at=2026-09-06T18:59:41Z
- Claimed: machine=m1 lineage=main-1788594343-3833-fb64b9 at=2026-09-06T18:57:50Z revision=4 accountingRevision=4
- StopCapability: generation=4 revision=4 machine=m1 claimEpoch=5 fenceEpoch=0

History:
- 2026-09-06T12:24:49Z ZKBER0FQHEW28XV1PJ2X0K49PP-m1-a4f8999f open actor=m1+main-1788594343-3833-fb64b9 targets=metasystem-stop-verb
- 2026-09-06T12:25:28Z ESANH6C2BR6VYW74A9Q5NQEWW2-m1-7cd0bd60 approve actor=human:Wido targets=metasystem-stop-verb
- 2026-09-06T12:52:02Z AS7KGSCDCHR0A955HGEQ3AYGA8-m1-a4f8999f edit actor=m1+main-1788594343-3833-fb64b9 targets=metasystem-stop-verb
- 2026-09-06T18:57:50Z 2HP4EVXE54QNATDSQ06XEWMQT9-m1-a4f8999f claim actor=m1+main-1788594343-3833-fb64b9 targets=metasystem-stop-verb
- 2026-09-06T18:59:41Z NRRVQGJN9SAGYKNX9BQCZ7PWZV-m1-a4f8999f slice-start actor=m1+main-1788594343-3833-fb64b9 targets=metasystem-stop-verb
- 2026-09-06T20:18:20Z 531CDF4TX7JYEP5N28Q911A2FR-m1-a4f8999f edit actor=m1+main-1788594343-3833-fb64b9 targets=metasystem-stop-verb
Integrity: sha256=24351256070dae3b4e09df2553e5d67c0c21c2512bd8e86167343feb395d28b9
