# metasystem-stop-verb

- State: claimed
- Risk: severity=2 novelty=1 exposure=3 accumulation=1 basis="severity 2: without an orderly stop a human reaches for kill and leaves locks, leases and half-written records behind, which the custody family then has to clean up; novelty 1: the shutdown path exists in arm-supervision.sh, the verb wraps it; exposure 3: every operator on every machine; accumulation 1: nothing compounds, the gap is the same every day"
- Tier: 3
- Intent: Wido's word 2026-09-06: 'For a human it should be a simple command to stop the meta system. And not only simple but also intuitive.' Today there is none: metasystem delegate --cancel stops one job, steward restart restarts and never stops, and the only shutdown path is scripts/agents/arm-supervision.sh --shutdown, an internal script the fixtures call. DONE means: 'metasystem stop --repo .' (accepted from an agent-free terminal, a human act like arm and restart) stops everything the metasystem runs for that checkout in order - running delegate jobs cancelled through the cancel path, then the steward runner, repo watcher, narrator and supervision components through the existing shutdown path - and prints one line per thing it stopped and one line saying how to start again (steward arm). Idempotent: a second stop says nothing is running. 'metasystem status --repo .' prints the same list without stopping anything. A fleet form (--all for every enrolled checkout on this host) is welcome if cheap; per checkout is the requirement. No kill -9 unless the orderly path fails, and then it says so.
- Origin: main
- Next step: SECOND RESURRECTION PATH, recorded 2026-09-07 by m1d and carried here: after that seat was stopped, its Stop hook re-armed the steward, and the revived steward logged 'steward revival: intent ... revives fixture-review-by-date-expired via job steward-...' - it judged a claimed goal stalled and moved to take it over while its holder was mid-landing. It got no further than the intent. The design already covers the path that matters: section 2's reader table names dispatch.ClaimLaunch as fenced, reached by delegate, by dispatch.sh directly AND by the steward's revival, so under a closed fence the revival cannot launch a job; and the runner itself is stopped and refuses to start (steward run reads the fence). The stop-fence scenario asserts the dispatch refusal today; if a round has room, assert the revival path by name too. STATUS 2026-09-07 08:0xZ: 10 of 20 attempts and 1200 of 2400 minutes spent. Green outside the sandbox on the round-8 tree: the fast Go gate, seat-survives (the seat's session and its Stop hook create nothing after a stop), status-is-live, seat-refused, and every pre-existing supervision scenario but rearm-launch-fails which is red on main. Round 9 (in flight) fixes the last three acceptance scenarios, all fixture bugs: a manifest path missing the metasystem/ prefix, a wait on a standing component that never exits, and a join passing a session pid outside its own ancestry. Then: the full package matrix and both battery beds outside the sandbox, the FIRST code critique of 62 files and about 6900 lines, close, land with the battery receipt, live stop/status/arm proof on this Mac, receipt, done. Main moved under the chain: 423e6ed6 landed the same dispatch-fixtures fixture-authority line this chain carries, so the next follow-up round rebases and resolves that hunk.
- OpenedAt: 2026-09-06T12:24:49Z
- Revision: 19
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
- 2026-09-07T07:50:18Z 0AV2DH69TN79CRMYKJE4SDZ4NC-m1b-c6925449 edit actor=m1b+main-1788680071-18713-e76d5d targets=metasystem-stop-verb
- 2026-09-07T09:07:28Z NB8J1X1WKA3AB6Z3AJH27PCS8K-m1b-c6925449 edit actor=m1b+main-1788680071-18713-e76d5d targets=metasystem-stop-verb
Integrity: sha256=37e3eaa637a9f2a5dcdaa4de3cebb031d10f04c6c32ab0e913856c675c438f9c
