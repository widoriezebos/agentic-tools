# metasystem-stop-verb

- State: claimed
- Risk: severity=2 novelty=1 exposure=3 accumulation=1 basis="severity 2: without an orderly stop a human reaches for kill and leaves locks, leases and half-written records behind, which the custody family then has to clean up; novelty 1: the shutdown path exists in arm-supervision.sh, the verb wraps it; exposure 3: every operator on every machine; accumulation 1: nothing compounds, the gap is the same every day"
- Tier: 3
- Intent: Wido's word 2026-09-06: 'For a human it should be a simple command to stop the meta system. And not only simple but also intuitive.' Today there is none: metasystem delegate --cancel stops one job, steward restart restarts and never stops, and the only shutdown path is scripts/agents/arm-supervision.sh --shutdown, an internal script the fixtures call. DONE means: 'metasystem stop --repo .' (accepted from an agent-free terminal, a human act like arm and restart) stops everything the metasystem runs for that checkout in order - running delegate jobs cancelled through the cancel path, then the steward runner, repo watcher, narrator and supervision components through the existing shutdown path - and prints one line per thing it stopped and one line saying how to start again (steward arm). Idempotent: a second stop says nothing is running. 'metasystem status --repo .' prints the same list without stopping anything. A fleet form (--all for every enrolled checkout on this host) is welcome if cheap; per checkout is the requirement. No kill -9 unless the orderly path fails, and then it says so.
- Origin: main
- Next step: INTENT EDIT PENDING WIDO, and now TWO things belong in it, both his words. (1) durability, 2026-09-07: 'If I stop it, the supervisor obviously should die too, instead of starting it again. That kind of defeats the purpose.' (2) the arm verb, 2026-09-07, answering the seat's own admission that it had added scope without raising it: 'One word down, one word up is indeed what I want.' So metasystem arm is part of the ask, not scope creep: steward arm alone does not restore supervision, so after a full stop a human needs either two commands or one new one, and Wido chose one. The proposed replacement intent text is prepared in m1b's scratchpad (proposed-intent.txt) and he runs unapprove/edit/approve AFTER the landing, as agreed, because unapproving a claimed goal parks it. Build state: chain stopverb-build1 at round 20 (in flight), folding the third read's five findings; all slice-1 acceptance was green on the round-19 tree in m1b's environment. Remaining: verify r20, one read (the close law requires a fresh critic on the final round), fold only what changes behaviour or can wedge a checkout, land with the battery receipt, live proof, receipt, done.
- OpenedAt: 2026-09-06T12:24:49Z
- Revision: 23
- Budget: elapsedLimit=3d attemptLimit=40 reservedJobMinutesLimit=4800 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 3
- NormApproval: approvedRef=R-85-m1b minutes=4800 reviewRounds=3 goalRevision=21
- Approved: by=human:Wido at=2026-09-07T20:30:09Z revision=22 opid=RV03X9ZFZ49Q788Q6574D5Y269-m1b-927ecfdd authority=proven digest=a670f6144066d452d826af4266921f536eea42cb64274052a4406a4b3666e4f4
- Sliced: machine=m1 lineage=main-1788594343-3833-fb64b9 revision=4 at=2026-09-06T18:59:41Z
- Claimed: machine=m1b lineage=main-1788680071-18713-e76d5d at=2026-09-07T20:30:09Z revision=22 accountingRevision=22
- StopCapability: generation=22 revision=22 machine=m1b claimEpoch=1 fenceEpoch=0

History:
- 2026-09-06T12:24:49Z ZKBER0FQHEW28XV1PJ2X0K49PP-m1-a4f8999f open actor=m1+main-1788594343-3833-fb64b9 targets=metasystem-stop-verb
- 2026-09-06T12:25:28Z ESANH6C2BR6VYW74A9Q5NQEWW2-m1-7cd0bd60 approve actor=human:Wido targets=metasystem-stop-verb
- 2026-09-06T12:52:02Z AS7KGSCDCHR0A955HGEQ3AYGA8-m1-a4f8999f edit actor=m1+main-1788594343-3833-fb64b9 targets=metasystem-stop-verb
- 2026-09-06T18:57:50Z 2HP4EVXE54QNATDSQ06XEWMQT9-m1-a4f8999f claim actor=m1+main-1788594343-3833-fb64b9 targets=metasystem-stop-verb
- 2026-09-06T18:59:41Z NRRVQGJN9SAGYKNX9BQCZ7PWZV-m1-a4f8999f slice-start actor=m1+main-1788594343-3833-fb64b9 targets=metasystem-stop-verb
- 2026-09-06T20:18:20Z 531CDF4TX7JYEP5N28Q911A2FR-m1-a4f8999f edit actor=m1+main-1788594343-3833-fb64b9 targets=metasystem-stop-verb
- 2026-09-06T20:29:27Z 6Y4AQ1AAV7FDAJP4PAN8AK3775-m1-a4f8999f park actor=m1+main-1788594343-3833-fb64b9 targets=metasystem-stop-verb reason=Wido 2026-09-06 20:5xZ: go headless on m1 first. Design ladder is at revision 2 + critique r2 (8 material, register 703b8c619); next is revision 3 (last design round) then the build; any seat or the runner can carry it from the goal's next step.
- 2026-09-06T21:08:58Z VGPR7YCXCGD6J8TVPK8A51CA29-m1-a4f8999f unpark actor=m1+main-1788594343-3833-fb64b9 targets=metasystem-stop-verb
- 2026-09-06T21:50:17Z F5W1NABYA07P9C8R5W948W9DF4-m1b-c6925449 claim actor=m1b+main-1788680071-18713-e76d5d targets=metasystem-stop-verb
- 2026-09-06T21:59:14Z 5F4EYB9BNYGD3PBFKCAJYAC3JN-m1b-c6925449 edit actor=m1b+main-1788680071-18713-e76d5d targets=metasystem-stop-verb
- 2026-09-06T22:12:35Z H2FJBN30HBHVR5NW8F4DYCMCPM-m1b-c6925449 edit actor=m1b+main-1788680071-18713-e76d5d targets=metasystem-stop-verb
- 2026-09-06T22:12:38Z 63MGMTQ35VP5QWBVYZ1GN7AB0H-m1b-c6925449 release actor=m1b+main-1788680071-18713-e76d5d targets=metasystem-stop-verb
- 2026-09-06T23:58:57Z 1FRJG6G9W96P94Q4W54SE4E7SY-m1b-c6925449 claim actor=m1b+main-1788680071-18713-e76d5d targets=metasystem-stop-verb
- 2026-09-07T00:54:12Z EP7BZ2W9C3RTXTFD1SP5DMWW18-m1b-c6925449 release actor=m1b+main-1788680071-18713-e76d5d targets=metasystem-stop-verb
- 2026-09-07T00:54:32Z FHQ5X2KYPH2KPQVB7KZX0N5HGW-m1b-c6925449 claim actor=m1b+main-1788680071-18713-e76d5d targets=metasystem-stop-verb
- 2026-09-07T07:09:25Z 2MSYG6R14HMB0BJBCZS6PD5AW4-m1b-c6925449 edit actor=m1b+main-1788680071-18713-e76d5d targets=metasystem-stop-verb
- 2026-09-07T07:29:27Z E1485VS3N1VW95FTYV0JSPQZ9X-m1b-927ecfdd set-budget actor=human:Wido targets=metasystem-stop-verb displaced=m1b+main-1788680071-18713-e76d5d@2026-09-07T00:54:32Z
- 2026-09-07T07:50:18Z 0AV2DH69TN79CRMYKJE4SDZ4NC-m1b-c6925449 edit actor=m1b+main-1788680071-18713-e76d5d targets=metasystem-stop-verb
- 2026-09-07T09:07:28Z NB8J1X1WKA3AB6Z3AJH27PCS8K-m1b-c6925449 edit actor=m1b+main-1788680071-18713-e76d5d targets=metasystem-stop-verb
- 2026-09-07T17:22:36Z RY5SPBFK2NCY72KTWM17MTT1ZD-m1b-927ecfdd set-budget actor=human:Wido targets=metasystem-stop-verb displaced=m1b+main-1788680071-18713-e76d5d@2026-09-07T07:29:27Z
- 2026-09-07T20:05:46Z YYZJEWCTEBWG0W33VFD2GSHRW1-m1b-c6925449 edit actor=m1b+main-1788680071-18713-e76d5d targets=metasystem-stop-verb
- 2026-09-07T20:30:09Z RV03X9ZFZ49Q788Q6574D5Y269-m1b-927ecfdd set-budget actor=human:Wido targets=metasystem-stop-verb displaced=m1b+main-1788680071-18713-e76d5d@2026-09-07T17:22:36Z
- 2026-09-08T06:03:40Z BTQK44TAHQG04MNHEX3QW4SJ6W-m1b-c6925449 edit actor=m1b+main-1788680071-18713-e76d5d targets=metasystem-stop-verb
Integrity: sha256=df6a6d3089673dd668cd664f80662186e3f44e1bfb497af6935d0e20a736e39b
