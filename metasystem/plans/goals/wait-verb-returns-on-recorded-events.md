# wait-verb-returns-on-recorded-events

- State: claimed
- Risk: severity=2 novelty=2 exposure=3 accumulation=2 basis="severity 2: a wait that never returns idles a seat, a wait that returns early spends a model turn; novelty 2: a wait verb over the existing job, run, attempt and ledger records is new, the records are not; exposure 3: every seat's waiting; accumulation 2: the waiter record, the four source readers, the adapter answer and the verbs"
- Tier: 2
- Intent: Member 1 of coordinator-wakes-on-events-not-polls (its design page plans/coordinator-wakes-on-events-not-polls-design.md, sections 2, 5 and 6). DONE means: the installed commands metasystem wait --job, --run, --attempt and --goal (landing and human-act, the answer wait keyed by question and taking its cursor from the question record) return on the recorded event or the bounded deadline with the typed exits of section 2; every adapter's wait-delivery operation answers blocking and the row records it, so no temporary fallback is needed; the version-2 waiter row and its by-id pointer exist; a restart recovers from the rows alone through --resume, with session start printing the WAITING commands where a start hook exists. This member owns the parent's DONE clauses 1 and 4 and clause 2's adapter operation. Fixtures: TestWaitJobTerminals, TestWaitRunTerminalsAndDeadline, TestWaitAdapterBlocking, TestWaitAttemptRequiresCommittedTerminal, TestWaitAttemptAfterDrain, TestWaitGoalLandingAndHumanAct, TestWaitGoalCursorHistory, TestWaitChannelAnswer, TestWaitRestartRecoveryReplay, TestWaitLockClockAndFetchBounds, TestWaitGoalFetchDeadline; bed legs wait-job-run, wait-proof, wait-ledger, wait-restart, wait-bounds.
- Origin: main
- Next step: 2026-09-13 04:2xZ m1b: Codex sol build round 1 running in a worktree as job wait-member1-build-1 (brief plans/wait-verb-returns-on-recorded-events-build-brief.md, DESIGN-BEARING, 90 min wall clock). On its return: the seat proves the round with job prove-round, an Opus code read at DESIGN-BEARING, fold rounds by follow-up, then the landing by human commit with the brief beside it.
- OpenedAt: 2026-09-13T02:05:04Z
- Revision: 6
- Budget: elapsedLimit=1d attemptLimit=14 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 1
- Approved: by=human:Wido at=2026-09-13T04:56:49Z revision=6 opid=WVYE8T88Y6TPQ75622FWQW4WC8-m1b-30a7e141 authority=proven digest=190a7ad169896d4190b7b05697035a9cd15747f4cf388623e6e1fdb627f52771
- Sliced: machine=m1b lineage=main-1789191336-90295-e4b24b revision=3 at=2026-09-13T02:06:30Z
- Claimed: machine=m1b lineage=main-1789191336-90295-e4b24b at=2026-09-13T04:56:49Z revision=6 accountingRevision=6 episodeAt=2026-09-13T02:05:12Z episodeRevision=3
- StopCapability: generation=6 revision=6 machine=m1b claimEpoch=2 fenceEpoch=0

History:
- 2026-09-13T02:05:04Z XEGWW0F70SRXPGCJ6280FM4W68-m1b-30a7e141 open actor=m1b+main-1789191336-90295-e4b24b targets=wait-verb-returns-on-recorded-events,coordinator-wakes-on-events-not-polls
- 2026-09-13T02:05:08Z 8WK1PQA7N99089FGP62NHTYBN7-m1b-30a7e141 approve actor=human:Wido targets=wait-verb-returns-on-recorded-events
- 2026-09-13T02:05:12Z M34T4BMJZ8PEPPW074BPBVW1PM-m1b-30a7e141 claim actor=m1b+main-1789191336-90295-e4b24b targets=wait-verb-returns-on-recorded-events
- 2026-09-13T02:06:30Z Q4N2HD7ENDBKVMB6VKJJ4ZQKQ5-m1b-30a7e141 slice-start actor=m1b+main-1789191336-90295-e4b24b targets=wait-verb-returns-on-recorded-events
- 2026-09-13T02:06:59Z 5VTJX2MFHT9AGJ4MF4NWHVVTP0-m1b-30a7e141 edit actor=m1b+main-1789191336-90295-e4b24b targets=wait-verb-returns-on-recorded-events
- 2026-09-13T04:56:49Z WVYE8T88Y6TPQ75622FWQW4WC8-m1b-30a7e141 set-budget actor=human:Wido targets=wait-verb-returns-on-recorded-events
Integrity: sha256=156b33dddf1c86257ac118c7c8a428bbaf233d8fc4a27496951f5cdf7ec916a4
