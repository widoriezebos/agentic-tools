# wait-verb-returns-on-recorded-events

- State: claimed
- Risk: severity=2 novelty=2 exposure=3 accumulation=2 basis="severity 2: a wait that never returns idles a seat, a wait that returns early spends a model turn; novelty 2: a wait verb over the existing job, run, attempt and ledger records is new, the records are not; exposure 3: every seat's waiting; accumulation 2: the waiter record, the four source readers, the adapter answer and the verbs"
- Tier: 2
- Intent: Member 1 of coordinator-wakes-on-events-not-polls (its design page plans/coordinator-wakes-on-events-not-polls-design.md, sections 2, 5 and 6). DONE means: the installed commands metasystem wait --job, --run, --attempt and --goal (landing and human-act, the answer wait keyed by question and taking its cursor from the question record) return on the recorded event or the bounded deadline with the typed exits of section 2; every adapter's wait-delivery operation answers blocking and the row records it, so no temporary fallback is needed; the version-2 waiter row and its by-id pointer exist; a restart recovers from the rows alone through --resume, with session start printing the WAITING commands where a start hook exists. This member owns the parent's DONE clauses 1 and 4 and clause 2's adapter operation. Fixtures: TestWaitJobTerminals, TestWaitRunTerminalsAndDeadline, TestWaitAdapterBlocking, TestWaitAttemptRequiresCommittedTerminal, TestWaitAttemptAfterDrain, TestWaitGoalLandingAndHumanAct, TestWaitGoalCursorHistory, TestWaitChannelAnswer, TestWaitRestartRecoveryReplay, TestWaitLockClockAndFetchBounds, TestWaitGoalFetchDeadline; bed legs wait-job-run, wait-proof, wait-ledger, wait-restart, wait-bounds.
- Origin: main
- Next step: 2026-09-13 m1b: Codex round 8 folded the round 7 read (the ledger observer types its deadline and cancel as 65/130; replay tested through the real observer). Delivery proof GREEN on the staged candidate (proof-mtzq167v-809bcd6b4b2825aa, tree 851453ac on 349ac583). A harness Opus read of round 8's delta runs. Waiting on the human after it: land and fix forward, or grant a fifth rostered read (the granted rounds, 3 on R-109-m1e, are spent). Later-member design question (Fable): constant-cost replay of a saved goal act, and what a pruned goal's replay answers.
- OpenedAt: 2026-09-13T02:05:04Z
- Revision: 24
- Budget: elapsedLimit=1d attemptLimit=14 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 2
- NormApproval: approvedRef=R-109-m1e minutes=720 reviewRounds=3 goalRevision=16
- Approved: by=human:Wido at=2026-09-13T08:41:27Z revision=17 opid=0GJS0PQ05HBYNJ0612VQF81DGV-m1e-c6925449 authority=proven digest=42695f94fceb534f7c3ca2f51f1c9104fb6b84e09f04c0208bf1f519b384eeb8
- Sliced: machine=m1b lineage=main-1789191336-90295-e4b24b revision=3 at=2026-09-13T02:06:30Z
- Claimed: machine=m1b lineage=main-1789191336-90295-e4b24b at=2026-09-13T08:44:27Z revision=20 accountingRevision=17 episodeAt=2026-09-13T07:23:39Z episodeRevision=12 idleSeconds=1841
- StopCapability: generation=20 revision=20 machine=m1b claimEpoch=2 fenceEpoch=0

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
- 2026-09-13T07:32:51Z S5X77NNW5R9XY4WE3X6AYBVSGE-m1b-30a7e141 release actor=m1b+main-1789191336-90295-e4b24b targets=wait-verb-returns-on-recorded-events
- 2026-09-13T07:51:39Z BMQGJZTFC1RZMZF10WGE4354WB-m1b-30a7e141 claim actor=m1b+main-1789191336-90295-e4b24b targets=wait-verb-returns-on-recorded-events
- 2026-09-13T08:18:03Z G3DPNP55SMAQBZMXQDWGB5ABVH-m1b-30a7e141 release actor=m1b+main-1789191336-90295-e4b24b targets=wait-verb-returns-on-recorded-events
- 2026-09-13T08:29:53Z KAX17WQC75CWABGXH0QEHP6PJQ-m1b-30a7e141 claim actor=m1b+main-1789191336-90295-e4b24b targets=wait-verb-returns-on-recorded-events
- 2026-09-13T08:41:27Z 0GJS0PQ05HBYNJ0612VQF81DGV-m1e-c6925449 set-budget actor=human:Wido targets=wait-verb-returns-on-recorded-events displaced=m1b+main-1789191336-90295-e4b24b@2026-09-13T08:29:53Z
- 2026-09-13T08:43:33Z M15R4Y7X35N20R2FN4A23QSWJJ-m1b-30a7e141 edit actor=m1b+main-1789191336-90295-e4b24b targets=wait-verb-returns-on-recorded-events
- 2026-09-13T08:44:24Z PA3QHZQJWR22CVRX2PF3295FG7-m1b-30a7e141 release actor=m1b+main-1789191336-90295-e4b24b targets=wait-verb-returns-on-recorded-events
- 2026-09-13T08:44:27Z 0ZBV40GGADSMPC5QKPXRYTEF5S-m1b-30a7e141 claim actor=m1b+main-1789191336-90295-e4b24b targets=wait-verb-returns-on-recorded-events
- 2026-09-13T09:26:23Z 0CX7EGG3674MKQK0CDZ32Y43NM-m1b-30a7e141 edit actor=m1b+main-1789191336-90295-e4b24b targets=wait-verb-returns-on-recorded-events
- 2026-09-13T09:49:28Z WMW72GYTGHACTF5M3V3XKCEAN3-m1b-30a7e141 edit actor=m1b+main-1789191336-90295-e4b24b targets=wait-verb-returns-on-recorded-events
- 2026-09-13T10:01:09Z Q7XVA61JNRVE808BXM2FK98BAC-m1b-30a7e141 edit actor=m1b+main-1789191336-90295-e4b24b targets=wait-verb-returns-on-recorded-events
- 2026-09-13T11:29:49Z CBJ8EN0WCQMSJNQXD53KK6A6BQ-m1b-30a7e141 edit actor=m1b+main-1789191336-90295-e4b24b targets=wait-verb-returns-on-recorded-events
Integrity: sha256=6478b6ac577b19bbf4b3d005026b5159a35702e274336955e5c51229c6046078
