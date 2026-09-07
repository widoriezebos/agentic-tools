# metasystem-stop-verb

- State: claimed
- Risk: severity=2 novelty=1 exposure=3 accumulation=1 basis="severity 2: without an orderly stop a human reaches for kill and leaves locks, leases and half-written records behind, which the custody family then has to clean up; novelty 1: the shutdown path exists in arm-supervision.sh, the verb wraps it; exposure 3: every operator on every machine; accumulation 1: nothing compounds, the gap is the same every day"
- Tier: 3
- Intent: Wido's word 2026-09-06: 'For a human it should be a simple command to stop the meta system. And not only simple but also intuitive.' Today there is none: metasystem delegate --cancel stops one job, steward restart restarts and never stops, and the only shutdown path is scripts/agents/arm-supervision.sh --shutdown, an internal script the fixtures call. DONE means: 'metasystem stop --repo .' (accepted from an agent-free terminal, a human act like arm and restart) stops everything the metasystem runs for that checkout in order - running delegate jobs cancelled through the cancel path, then the steward runner, repo watcher, narrator and supervision components through the existing shutdown path - and prints one line per thing it stopped and one line saying how to start again (steward arm). Idempotent: a second stop says nothing is running. 'metasystem status --repo .' prints the same list without stopping anything. A fleet form (--all for every enrolled checkout on this host) is welcome if cheap; per checkout is the requirement. No kill -9 unless the orderly path fails, and then it says so.
- Origin: main
- Next step: LIVE CAUSE OF THE RESURRECTION, reproduced by m1d 2026-09-07 09:32 CEST and recorded here as the goal's sharpest evidence: Wido shut supervision down and killed the steward runner, and 49 seconds later a new owner, watcher and reaper came up at generation 26. The cause is a live session's Stop hook: hooks.log shows its stop verdict (block=false, elapsed=5s) wrapping the arm. So the hook that judges whether stopping is safe is the thing that undoes the stop. This confirms rather than changes the design: the FIRST row of section 2's reader table is up in every mode, which is the path the hooks and the scheduler entry take, and under a closed fence it prints component=stopped outcome=standing, exits 0 and does nothing else - no re-arm, no announcement, no lease, no supervision, no runner. The reproduction used up --shutdown and a kill, not metasystem stop, so no fence was ever closed and the hook was free to re-arm; that is the unlanded world. The acceptance test m1d proposes, that a stop must survive the next Stop hook of a live session, is already one of slice 1's five scenarios: seat-survives fires a Stop hook after the stop and asserts it returns allow with up outcome=stopped and blocks nothing. Round 8 (in flight) strengthens it per Wido's durability requirement: the same scenario family now also drives the watcher component's steward repair and the arming path's ensure-runner after a stop and asserts by process-table diff that nothing was created and the marker still reads closed with its original actor. Budget raised by Wido to 2d/20/2400m/1/3 under R-83-m1b; slices 1b and 2 are goals metasystem-stop-escalation-proofs and metasystem-stop-fleet-form.
- OpenedAt: 2026-09-06T12:24:49Z
- Revision: 18
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
Integrity: sha256=b8d1dcc23504481064e20d8cddfcdc217592bea33f8f14c05f855f90e503940e
