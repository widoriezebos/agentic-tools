# metasystem-stop-verb

- State: claimed
- Risk: severity=2 novelty=1 exposure=3 accumulation=1 basis="severity 2: without an orderly stop a human reaches for kill and leaves locks, leases and half-written records behind, which the custody family then has to clean up; novelty 1: the shutdown path exists in arm-supervision.sh, the verb wraps it; exposure 3: every operator on every machine; accumulation 1: nothing compounds, the gap is the same every day"
- Tier: 3
- Intent: Wido's word 2026-09-06: 'For a human it should be a simple command to stop the meta system. And not only simple but also intuitive.' Today there is none: metasystem delegate --cancel stops one job, steward restart restarts and never stops, and the only shutdown path is scripts/agents/arm-supervision.sh --shutdown, an internal script the fixtures call. DONE means: 'metasystem stop --repo .' (accepted from an agent-free terminal, a human act like arm and restart) stops everything the metasystem runs for that checkout in order - running delegate jobs cancelled through the cancel path, then the steward runner, repo watcher, narrator and supervision components through the existing shutdown path - and prints one line per thing it stopped and one line saying how to start again (steward arm). Idempotent: a second stop says nothing is running. 'metasystem status --repo .' prints the same list without stopping anything. A fleet form (--all for every enrolled checkout on this host) is welcome if cheap; per checkout is the requirement. No kill -9 unless the orderly path fails, and then it says so.
- Origin: main
- Next step: BINDING REQUIREMENT ADDED BY WIDO 2026-09-07 morning, relayed by m1d and recorded here by the claimant (m1b) because only the claimant may edit a claimed goal's next step; the INTENT still needs his own unapprove/edit/approve at his enrolled terminal. His words, verbatim: 'If I stop it, the supervisor obviously should die too, instead of starting it again. That kind of defeats the purpose. ... This is totally broken at the moment and needs a fix.' Evidence m1d observed: the session stopped at 07:40 and supervision restarted the steward at 07:41:14. A STOP MUST BE DURABLE: after a stop, nothing that checkout runs comes back on its own (supervision owner, watcher, reaper, steward runner, narrator, and any boot or scheduler entry stay down, and no tick, hook, session boot or arm path re-arms them); the stop writes a durable marker naming who stopped it and when; every arm path reads that marker and refuses to resurrect, naming how to start again; status reports stopped-by-whom-and-when rather than looking idle; only a human arm clears it. THIS LANDS IN SLICE 1, not a later slice. Coordinator's reading (m1b, 2026-09-07 09:1xZ): revision 3 of plans/metasystem-stop-verb-design.md already specifies exactly this and the chain has built it -- section 2's fence record is never deleted once written and carries state/phase/generation/changedAt/by, and its reader table makes up (every mode), steward arm/restart/run, RepairEnrolledRunner, EnsureRunner, dispatch ClaimLaunch, run launch/register/adopt, proof-run launch, mission start/resume/run-loop, health and the turn verdict all refuse under a closed fence, with the scheduler entry and the hooks taking the up path; arm is gated to a human terminal. What Wido's words add is one gap: the printed stop and status lines carry the time but not WHO stopped it, and the fence record's by identity should be named there. That plus the durability assertions ride round 8 of chain stopverb-build1 (round 7 is in flight as of 09:05). Budget arithmetic after that: 8 of 10 attempts spent, so round 8 and its closing critic are the last two jobs and there is no spare; a stop on either parks the goal.
- OpenedAt: 2026-09-06T12:24:49Z
- Revision: 17
- Budget: elapsedLimit=2d attemptLimit=20 reservedJobMinutesLimit=2400 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 1
- NormApproval: approvedRef=R-83-m1b minutes=2400 reviewRounds=3 goalRevision=16
- Approved: by=human:Wido at=2026-09-07T07:29:27Z revision=17 opid=E1485VS3N1VW95FTYV0JSPQZ9X-m1b-927ecfdd authority=proven digest=ab1a6d382abeefed2e89c7dbdc01610fb08453e65d900c36612ce05df09d8bc9
- Sliced: machine=m1 lineage=main-1788594343-3833-fb64b9 revision=4 at=2026-09-06T18:59:41Z
- Claimed: machine=m1b lineage=main-1788680071-18713-e76d5d at=2026-09-07T07:29:27Z revision=17 accountingRevision=17
- StopCapability: generation=17 revision=17 machine=m1b claimEpoch=1 fenceEpoch=0

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
Integrity: sha256=57b9176a14aaa5db8c852f233255d5bf62424299746a802228a9af4587fddb2d
