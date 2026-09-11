# seat-opened-goals-name-their-blocker

- State: approved
- Priority: 1
- Sequence: 6
- Risk: severity=1 novelty=1 exposure=2 accumulation=1 basis="severity 1: a refused open is re-issued with its blocker; novelty 1: dependency edges exist; exposure 2: seat opens only; accumulation 1: one verb"
- Tier: 2
- Intent: Seats opened 165 goals in five days against 62 concluded, growing the backlog by 117 (ledger.md section 5 of the delivery deep dive). DONE means: goal open from a seat (origin main) requires --blocks naming the claimed goal it unblocks, records the dependency edge, and refuses otherwise with the ruling; human-origin opens are unchanged; docs/orchestration.md carries the ruling that a seat opens only the defect that blocks its current goal and every other goal is opened by a person. The ruling is Wido's to confirm on the design page; the seat builds it as stated unless he names another. Goal 6 of plans/delivery-efficiency-plan.md.
- Origin: human
- Next step: Read OpenTiered in internal/goal/verbs.go and the open flags in cmd/metasystem/goalsync_mutations.go, add the flag, the edge and the refusal, the docs line, land.
- OpenedAt: 2026-09-11T15:45:05Z
- Revision: 4
- Pinned: m1e
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-11T15:49:14Z revision=4 opid=D0Z826ZYBY6WV1HRZCFH6B9Q4C-m1-c6925449 authority=proven digest=0b13e488aaec84e57a7552169768b56d4e9589891109c93a31f741ca406aee47

History:
- 2026-09-11T15:45:05Z 5W0FM0WGJ00Q1A7X06ESJFVC4X-m1e-5083721b open actor=m1e+main-1789141490-27414-b9c0d2 targets=seat-opened-goals-name-their-blocker
- 2026-09-11T15:47:14Z X1PRNWZEXP7ZK7V0Q8NE34S5BY-m1-c6925449 set-pin actor=human:Wido targets=seat-opened-goals-name-their-blocker
- 2026-09-11T15:47:18Z CW7PT0773AXANF6XTDSKH1KX37-m1-c6925449 set-priority actor=human:Wido targets=actionable-metrics,breach-clock-and-budget-honesty,burn-without-delivery-tripwire,carried-landing-debt-and-cap,delegate-job-liveness,failed-job-attention,fixture-stewards-outlive-their-suite,goal-abandoned-with-a-reason,human-carried-landing-verb,human-goal-verbs-forgiving,job-record-birth-token,landing-receipt-survives-records-drift,lease-sweep-death-evidence,live-job-on-a-done-goal-unnoticed,machine-concurrency-governor,proof-harness-process-custody,seat-opened-goals-name-their-blocker,severity-tiered-rigor,severity-tiered-rigor-p2,small-change-lane,steward-catchup-livelock,steward-revives-a-done-goal,stop-batch-strands-a-resumable-goal,suite-custody,token-spend-fence,turn-verdict-hardening,watch-verb,watcher-repair-request-never-names-current,winddown-census-handoff-leak reason=priority-order subject=seat-opened-goals-name-their-blocker from=unranked to=1:6 requested-sequence=6
- 2026-09-11T15:49:14Z D0Z826ZYBY6WV1HRZCFH6B9Q4C-m1-c6925449 approve actor=human:Wido targets=seat-opened-goals-name-their-blocker
Integrity: sha256=a8f25bac2d26f548251c5607e8a13be455869cdb6ff08b8e444f82e87d6a4f34
