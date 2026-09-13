# wait-verb-returns-on-recorded-events

- State: claimed
- Risk: severity=2 novelty=2 exposure=3 accumulation=2 basis="severity 2: a wait that never returns idles a seat, a wait that returns early spends a model turn; novelty 2: a wait verb over the existing job, run, attempt and ledger records is new, the records are not; exposure 3: every seat's waiting; accumulation 2: the waiter record, the four source readers, the adapter answer and the verbs"
- Tier: 2
- Intent: Member 1 of coordinator-wakes-on-events-not-polls (its design page plans/coordinator-wakes-on-events-not-polls-design.md, sections 2, 5 and 6). DONE means: the installed commands metasystem wait --job, --run, --attempt and --goal (landing and human-act, the answer wait keyed by question and taking its cursor from the question record) return on the recorded event or the bounded deadline with the typed exits of section 2; every adapter's wait-delivery operation answers blocking and the row records it, so no temporary fallback is needed; the version-2 waiter row and its by-id pointer exist; a restart recovers from the rows alone through --resume, with session start printing the WAITING commands where a start hook exists. This member owns the parent's DONE clauses 1 and 4 and clause 2's adapter operation. Fixtures: TestWaitJobTerminals, TestWaitRunTerminalsAndDeadline, TestWaitAdapterBlocking, TestWaitAttemptRequiresCommittedTerminal, TestWaitAttemptAfterDrain, TestWaitGoalLandingAndHumanAct, TestWaitGoalCursorHistory, TestWaitChannelAnswer, TestWaitRestartRecoveryReplay, TestWaitLockClockAndFetchBounds, TestWaitGoalFetchDeadline; bed legs wait-job-run, wait-proof, wait-ledger, wait-restart, wait-bounds.
- Origin: main
- Next step: 2026-09-13 08:3xZ m1b: WIDO'S LAND-OR-FOLD CALL, as promised at build fold-read cycle three. The build is complete and proved: Codex sol chain wait-member1-build-1 rounds 2-5 (round 1 gap-stopped on the brief), three harness Opus reads folded (10, 11, 8 material), round 5's proof green across the whole battery, the seat's fix of the last low finding (a matched event wins over a frontier change) tested. The rostered Opus code-critic at DESIGN-BEARING (job wait-member1-codecritic-1) then returned THREE material, all medium, open in its register: WVB-40 a resume with a new timeout after a stored result replays instead of renewing (the page's renewal rule); WVB-41 a landing wait never checks the landing destination against the ledger branch (in local mode a real landing never matches; the page asks for a named refusal at registration); WVB-42 the actionable check's report scan runs local Git without the wait's budget every ten seconds (0.3 s a cycle; the decision needs no Git). The seat's staged index (45 files, the six briefs, the page's row-2 correction) is ready in the m1b checkout; backup in the session scratchpad member1-wait-staged.diff. Options: FOLD (one Codex round on the three, ~1 h, then proof and a rostered read round 2 within the two-round box, then the landing) or LAND as is with the three as follow-up goals. Parked in Wido's name meanwhile; attempts used 9 of 14.
- OpenedAt: 2026-09-13T02:05:04Z
- Revision: 12
- Budget: elapsedLimit=1d attemptLimit=14 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 1
- Approved: by=human:Wido at=2026-09-13T04:56:49Z revision=6 opid=WVYE8T88Y6TPQ75622FWQW4WC8-m1b-30a7e141 authority=proven digest=190a7ad169896d4190b7b05697035a9cd15747f4cf388623e6e1fdb627f52771
- Sliced: machine=m1b lineage=main-1789191336-90295-e4b24b revision=3 at=2026-09-13T02:06:30Z
- Claimed: machine=m1b lineage=main-1789191336-90295-e4b24b at=2026-09-13T07:23:39Z revision=12 accountingRevision=12 episodeAt=2026-09-13T07:23:39Z episodeRevision=12
- StopCapability: generation=12 revision=12 machine=m1b claimEpoch=2 fenceEpoch=0

History:
- 2026-09-13T02:05:04Z XEGWW0F70SRXPGCJ6280FM4W68-m1b-30a7e141 open actor=m1b+main-1789191336-90295-e4b24b targets=wait-verb-returns-on-recorded-events,coordinator-wakes-on-events-not-polls
- 2026-09-13T02:05:08Z 8WK1PQA7N99089FGP62NHTYBN7-m1b-30a7e141 approve actor=human:Wido targets=wait-verb-returns-on-recorded-events
- 2026-09-13T02:05:12Z M34T4BMJZ8PEPPW074BPBVW1PM-m1b-30a7e141 claim actor=m1b+main-1789191336-90295-e4b24b targets=wait-verb-returns-on-recorded-events
- 2026-09-13T02:06:30Z Q4N2HD7ENDBKVMB6VKJJ4ZQKQ5-m1b-30a7e141 slice-start actor=m1b+main-1789191336-90295-e4b24b targets=wait-verb-returns-on-recorded-events
- 2026-09-13T02:06:59Z 5VTJX2MFHT9AGJ4MF4NWHVVTP0-m1b-30a7e141 edit actor=m1b+main-1789191336-90295-e4b24b targets=wait-verb-returns-on-recorded-events
- 2026-09-13T04:56:49Z WVYE8T88Y6TPQ75622FWQW4WC8-m1b-30a7e141 set-budget actor=human:Wido targets=wait-verb-returns-on-recorded-events
- 2026-09-13T05:11:26Z W4GEGG5KZWB3VJF5970JRFSN7K-m1b-30a7e141 edit actor=m1b+main-1789191336-90295-e4b24b targets=wait-verb-returns-on-recorded-events
- 2026-09-13T06:30:01Z 2WMCDABFY7CAH43GYRTR9DV3FY-m1b-30a7e141 edit actor=m1b+main-1789191336-90295-e4b24b targets=wait-verb-returns-on-recorded-events
- 2026-09-13T06:30:05Z CXFMB1NR2VXG4KHKZY99N952KB-m1b-30a7e141 release actor=m1b+main-1789191336-90295-e4b24b targets=wait-verb-returns-on-recorded-events
- 2026-09-13T06:32:05Z WQ0RZFV703J5FE6SMHJXW3SVGK-m1b-30a7e141 park actor=human:Wido targets=wait-verb-returns-on-recorded-events reason=built and proved; the rostered read holds three medium findings open (WVB-40, 41, 42); Wido decides fold or land (the record carries both options); parked so no seat hook or steward claims it meanwhile
- 2026-09-13T07:23:35Z J885FH1V4GH9QARK1PT67VR9DZ-m1b-30a7e141 unpark actor=human:Wido targets=wait-verb-returns-on-recorded-events
- 2026-09-13T07:23:39Z A9QCJW80X3NF5XY8X8Q9MGGHG9-m1b-30a7e141 claim actor=m1b+main-1789191336-90295-e4b24b targets=wait-verb-returns-on-recorded-events
Integrity: sha256=642c6274af1b8612f0db33a5b298b0810cfe8828d51539a7181154777c81b806
