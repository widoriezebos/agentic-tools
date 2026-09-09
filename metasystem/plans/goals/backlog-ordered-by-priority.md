# backlog-ordered-by-priority

- State: claimed
- Priority: 1
- Sequence: 2
- Risk: severity=2 novelty=2 exposure=3 accumulation=2 basis="severity 2: without an order the fleet works whatever a seat happens to read first, and the human's priorities reach the machines only by hand through pins and prompts; novelty 2: a new field on the goal record, a human verb, an ordered listing and a next verb, all on the ledger's existing grammar; exposure 3: every seat's choice of work on every machine; accumulation 2: every day without it the human re-sorts the backlog in conversation"
- Tier: 3
- Intent: Wido's word 2026-09-06: 'Backlog items need to be sorted in order of priority and then a sequence number. And this should be able to update after the fact so that we can change priority of the backlog.' Today nothing orders the ledger: a goal record has no rank, goal list sorts by nothing, and a seat with a free claim picks by reading the approved unclaimed records; the only order that exists is a pin, the order in a seat's kickoff prompt, and prose in a few records. DONE means: every goal record carries a priority (a small ordered scale, e.g. 1 highest) and a sequence number (its place within that priority, unique among open goals); the pair is set and changed after the fact by a human verb at the enrolled terminal (goal set-priority --by <human> --id <goal> --priority <n> [--sequence <n>], with re-sequencing of the others in that priority when a number is inserted) and recorded as a ledger event like set-pin; goal list prints open goals in that order, priority then sequence, with state and pin; goal next --machine <nick> returns the first approved, unclaimed goal in that order that is unpinned or pinned to that machine, and seats take work through it instead of reading; a goal with no priority yet sorts last, so approval of the grandfathered backlog needs no bulk edit; and the channel's status report shows the top of the order so the human sees what the fleet will do next.
- Origin: main
- Next step: DESIGN CLOSED at revision 2 (0260aef6, sha256 b1f93510884bfd408338a768f70ffb6ad6c1a49c1861d17421f8832b00cfde13): read 2 (bcc-crit2, Sol) returned zero findings after going to the page's riskiest part first; register at records/misc/backlog-ordered-by-priority-conclusion-vs-compaction-critique-r2.md. Fold-read cycles: 1; the land-or-fold call did not need to reach Wido. BUILD dispatched as chain root bolboc-build1 on the implementation lane (Sol), brief plans/backlog-ordered-by-priority-conclusion-vs-compaction-build-brief.md: section 3 order of work, question 1 canaries first and red on the untouched tree for the named reason, the three TestPriorityLifecycle premise pins green first, then the repair, then the Refused field and its three surfaces with their command and channel canaries red then green; the untouched list is a hard wall; no finding references in source comments. The chain closes under the DESIGN-BEARING law with a fresh-context read of the final work round; the orchestrator runs dispatch-fixtures.sh and goal-cli-fixtures.sh outside the sandbox and lands with a receipt-bound battery. BOC-01, BOC-03 and BOQ-02 close with this build. BOQ-01 stays blocked on the AGENTS.md ceiling (1398 of 1400 words) and is confirmed a checkout fact, not a frontier category; it is carried into this chain only if the ceiling can be met lawfully, otherwise it remains the one recorded deferral.
- OpenedAt: 2026-09-06T12:34:05Z
- Revision: 19
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
- 2026-09-09T09:15:49Z R007N5X6QWGHTNYGRC2FFTK877-m1b-c6925449 edit actor=m1b+main-1788680071-18713-e76d5d targets=backlog-ordered-by-priority
- 2026-09-09T09:30:00Z X6684GJRD607KT33P5F1235ZW2-m1b-c6925449 edit actor=m1b+main-1788680071-18713-e76d5d targets=backlog-ordered-by-priority
Integrity: sha256=6daf43abb7aeb082044dc39479036157f0e96eeb96c93fa5d3414a7b2ad5ce6b
