# backlog-ordered-by-priority

- State: claimed
- Priority: 1
- Sequence: 2
- Risk: severity=2 novelty=2 exposure=3 accumulation=2 basis="severity 2: without an order the fleet works whatever a seat happens to read first, and the human's priorities reach the machines only by hand through pins and prompts; novelty 2: a new field on the goal record, a human verb, an ordered listing and a next verb, all on the ledger's existing grammar; exposure 3: every seat's choice of work on every machine; accumulation 2: every day without it the human re-sorts the backlog in conversation"
- Tier: 3
- Intent: Wido's word 2026-09-06: 'Backlog items need to be sorted in order of priority and then a sequence number. And this should be able to update after the fact so that we can change priority of the backlog.' Today nothing orders the ledger: a goal record has no rank, goal list sorts by nothing, and a seat with a free claim picks by reading the approved unclaimed records; the only order that exists is a pin, the order in a seat's kickoff prompt, and prose in a few records. DONE means: every goal record carries a priority (a small ordered scale, e.g. 1 highest) and a sequence number (its place within that priority, unique among open goals); the pair is set and changed after the fact by a human verb at the enrolled terminal (goal set-priority --by <human> --id <goal> --priority <n> [--sequence <n>], with re-sequencing of the others in that priority when a number is inserted) and recorded as a ledger event like set-pin; goal list prints open goals in that order, priority then sequence, with state and pin; goal next --machine <nick> returns the first approved, unclaimed goal in that order that is unpinned or pinned to that machine, and seats take work through it instead of reading; a goal with no priority yet sorts last, so approval of the grandfathered backlog needs no bulk edit; and the channel's status report shows the top of the order so the human sees what the fleet will do next.
- Origin: main
- Next step: CRITIQUE ROUND 1 of the conclusion-vs-compaction design RETURNED (bcc-crit1, Sol): three findings, one material, register at records/misc/backlog-ordered-by-priority-conclusion-vs-compaction-critique-r1.md with the coordinator's disposition. BCC-01, accepted after the coordinator confirmed it in split.go: a ranked survivor carrying a neighbour's compaction done line can later be archived by goal split, which appends the verb split and no done line, so the page's last-done-under-State-done rule returns the neighbour's timestamp; archive writers are done (doneRequest, reconcile) and split (split), and compaction lines carry the departing act's verb. BCC-02 and BCC-03 are wording, accepted. FOLD-READ CYCLE 1: revision 2 in place dispatched to the Fable design lane as bolboc-design4, brief plans/backlog-ordered-by-priority-conclusion-vs-compaction-design-fold-r1-brief.md, with a split-parent canary that must fail on the untouched tree and an enumerated proof of the archive-writer set. Then a fresh read whose reviews is revision 2; at cycle 2 the coordinator says aloud that this loop has no natural exit, and before cycle 3 the land-or-fold call goes to Wido. Design revision 1 remains landed at 8ca43596. Still open after the design closes: the build behind section 3 (BOC-01, BOC-03, BOQ-02 land with it) and BOQ-01, blocked on the AGENTS.md ceiling and confirmed a checkout fact.
- OpenedAt: 2026-09-06T12:34:05Z
- Revision: 17
- Pinned: m1b
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T12:34:28Z revision=2 opid=YRABK927PRM5N2ASNMKZVEYKWH-m1-7cd0bd60 authority=proven digest=57ae2782caf8a6b6be34f6216d0244fa1b0923b0cd41b028e58e3eaf293f436b
- Sliced: machine=m1b lineage=main-1788680071-18713-e76d5d revision=4 at=2026-09-08T06:11:48Z
- Claimed: machine=m1b lineage=main-1788680071-18713-e76d5d at=2026-09-09T06:30:14Z revision=14 accountingRevision=14
- StopCapability: generation=14 revision=14 machine=m1b claimEpoch=1 fenceEpoch=0

History:
- 2026-09-06T12:34:05Z TDE1XRY1CH2KR2VBCXFHJHW82P-m1-a4f8999f open actor=m1+main-1788594343-3833-fb64b9 targets=backlog-ordered-by-priority
- 2026-09-06T12:34:28Z YRABK927PRM5N2ASNMKZVEYKWH-m1-7cd0bd60 approve actor=human:Wido targets=backlog-ordered-by-priority
- 2026-09-06T15:43:18Z DC782RYCANRSNPZP6ZSEBP01VZ-m1-7cd0bd60 set-pin actor=human:Wido targets=backlog-ordered-by-priority
- 2026-09-08T06:10:06Z 1TZYJK3Q3DMC2HM5MJX3R9B3MR-m1b-c6925449 claim actor=m1b+main-1788680071-18713-e76d5d targets=backlog-ordered-by-priority
- 2026-09-08T06:11:48Z KPV4VP6QD7Y2G8YVN84J6JFFWB-m1b-c6925449 slice-start actor=m1b+main-1788680071-18713-e76d5d targets=backlog-ordered-by-priority
- 2026-09-08T10:27:40Z M8PJ4TVAFMJ5D0KD7230R2W42G-m1b-c6925449 edit actor=m1b+main-1788680071-18713-e76d5d targets=backlog-ordered-by-priority
- 2026-09-08T10:27:44Z 8087QHM5PGKQANQ1M7AD9FAC88-m1b-c6925449 release actor=m1b+main-1788680071-18713-e76d5d targets=backlog-ordered-by-priority
- 2026-09-08T15:55:29Z XP1DYWFEBFN9Q13SWNTP8614SF-m1-7cd0bd60 set-priority actor=human:Wido targets=backlog-ordered-by-priority reason=priority-order subject=backlog-ordered-by-priority from=unranked to=1:3 requested-sequence=3
- 2026-09-08T17:01:09Z XR2031TXY1Q8MXG1K1BSRDRB26-m1b-c6925449 claim actor=m1b+main-1788680071-18713-e76d5d targets=backlog-ordered-by-priority
- 2026-09-08T17:01:54Z HRKKC2KXZR0SMT78JMK8X31NMF-m1b-c6925449 release actor=m1b+main-1788680071-18713-e76d5d targets=backlog-ordered-by-priority
- 2026-09-08T17:03:35Z YZH9GDDZJGK4DB0ENK5CGSXADW-m1b-c6925449 done actor=human:Wido targets=account-provenance,actionable-metrics,backlog-ordered-by-priority,breach-clock-and-budget-honesty,breach-stop-wedges-seat,burn-without-delivery-tripwire,delegate-job-liveness,dispatch-cap-necessity,failed-job-attention,fixture-stewards-outlive-their-suite,governed-exhaustion-reprojection,host-runtime-setup,human-goal-verbs-forgiving,job-record-birth-token,lease-sweep-death-evidence,live-job-on-a-done-goal-unnoticed,machine-concurrency-governor,proof-harness-process-custody,review-round-limit-counts-per-chain,severity-tiered-rigor,severity-tiered-rigor-p2,small-change-lane,steward-catchup-livelock,steward-revives-a-done-goal,suite-custody,token-spend-fence,turn-verdict-hardening,watch-verb,watcher-repair-request-never-names-current,winddown-census-handoff-leak reason=priority-order from=1:3 to=1:2
- 2026-09-08T17:04:33Z GE6Z5QPGTVCW2Z5AAG0X0VPC1F-m1b-c6925449 claim actor=m1b+main-1788680071-18713-e76d5d targets=backlog-ordered-by-priority
- 2026-09-09T05:05:49Z 8B8D6WNM4MT01NN7PXA2K3611H-m1b-43182c96 breach-stop actor=m1b+goal-stop-custodian targets=backlog-ordered-by-priority
- 2026-09-09T06:30:14Z HTYKC8PX25HNFPWETW4NAPY4SF-m1b-c6925449 resume actor=human:Wido targets=backlog-ordered-by-priority
- 2026-09-09T07:03:51Z QY30ZJ900KWDR18YEWJGFAHB41-m1b-c6925449 edit actor=m1b+main-1788680071-18713-e76d5d targets=backlog-ordered-by-priority
- 2026-09-09T08:40:41Z EACVCN0T2ZVRJ00ECF2PGM9XPA-m1b-c6925449 edit actor=m1b+main-1788680071-18713-e76d5d targets=backlog-ordered-by-priority
- 2026-09-09T08:58:16Z 87Y4ZMMFYFT6WJH35P54WSJ4VH-m1b-c6925449 edit actor=m1b+main-1788680071-18713-e76d5d targets=backlog-ordered-by-priority
Integrity: sha256=0a17df84a0b157022cd8bcfe2139cdf9849f1ad1a010b69c1c7d01a10ebf46fb
