# stop-hook-never-forces-an-empty-turn

- State: queued
- Priority: 1
- Sequence: 4
- Risk: severity=2 novelty=2 exposure=3 accumulation=1 basis="severity 2: a wrong refusal rule either forces turns as today or lets an unsafe stop through; novelty 2: refusal causes are reclassified between seat and steward; exposure 3: every seat's every stop; accumulation 1: one hook"
- Tier: 3
- Intent: The Stop hook refused 331 times in five days, forcing 292 coordinator turns that cost 157 million prompt tokens, 90 of which only polled again and 73 were text-only; the recorded causes were 'the narrator digest could not be read' (184 across m1, m1b and m1d since 2026-09-10), 'stop deadline expired' (117 on m1) and 'supervision arming failed' (about 95) (claude-sessions.md section 4c and the seats' stop-refusal records). DONE means: (1) a refusal names a condition the seat can act on and the one command that clears it; (2) an infrastructure read failure the seat cannot change (narrator digest, hook-evidence state, arming state) never refuses the stop and is reported to the steward instead; (3) an infrastructure refusal is not repeated for an unchanged condition within one stop deadline; (4) the idle-with-backlog path is untouched: approved backlog with no job still refuses the stop and after three unchanged refusals hands the seat to steward continuation as today (records/goals/idle-with-backlog-alarm.md); proven by fixtures replaying the three recorded infrastructure causes and the approved-backlog-with-no-job case, and by a week of stop refusals under 10 per seat per day. Goal 4 of plans/delivery-efficiency-plan.md.
- Origin: human
- Next step: Read the Stop hook (scripts/agents and internal supervision, turnverdict.go) and classify the recorded refusal causes into seat-actionable, steward-owned and idle-path; write the design; critique; build; land.
- OpenedAt: 2026-09-11T15:45:01Z
- Revision: 8
- Pinned: m1e
- BudgetExceptions: 0

History:
- 2026-09-11T15:45:01Z T6Z7DYJ3CFGDRPYXNRNTTWXERM-m1e-5083721b open actor=m1e+main-1789141490-27414-b9c0d2 targets=stop-hook-never-forces-an-empty-turn
- 2026-09-11T15:47:04Z PNHSKRR4SQA1Q4SS26QWE5YQCW-m1-c6925449 set-pin actor=human:Wido targets=stop-hook-never-forces-an-empty-turn
- 2026-09-11T15:47:07Z VR9BVNVCK32QBWEFW8H5JVCSA9-m1-c6925449 set-priority actor=human:Wido targets=actionable-metrics,breach-clock-and-budget-honesty,burn-without-delivery-tripwire,carried-landing-debt-and-cap,delegate-job-liveness,failed-job-attention,fixture-stewards-outlive-their-suite,goal-abandoned-with-a-reason,human-carried-landing-carry,human-carried-landing-verb,human-goal-verbs-forgiving,job-record-birth-token,landing-receipt-survives-records-drift,lease-sweep-death-evidence,live-job-on-a-done-goal-unnoticed,machine-concurrency-governor,proof-harness-process-custody,severity-tiered-rigor,severity-tiered-rigor-p2,small-change-lane,steward-catchup-livelock,steward-revives-a-done-goal,stop-batch-strands-a-resumable-goal,stop-hook-never-forces-an-empty-turn,suite-custody,token-spend-fence,turn-verdict-hardening,watch-verb,watcher-repair-request-never-names-current,winddown-census-handoff-leak reason=priority-order subject=stop-hook-never-forces-an-empty-turn from=unranked to=1:4 requested-sequence=4
- 2026-09-11T15:49:10Z 2CS1CQV2DB5829AVW6VW20FGE7-m1-c6925449 approve actor=human:Wido targets=stop-hook-never-forces-an-empty-turn
- 2026-09-11T16:13:08Z 8YPN9R8X6K7CZSTKV5HDV265MX-m1-c6925449 unapprove actor=human:Wido targets=stop-hook-never-forces-an-empty-turn reason=Wido 2026-09-11: Codex critique round 1 of the delivery-efficiency plan accepted; intent amended before re-approval
- 2026-09-11T16:16:28Z 1YCC1Y3B6VEXMPQYS1MGSF8KY3-m1e-ab5421f5 edit actor=m1e+main-1789143377-71578-b9c0d2 targets=stop-hook-never-forces-an-empty-turn
- 2026-09-11T16:18:55Z CAT3YQTAVZKM2NQKKMAMQHAFX1-m1-c6925449 approve actor=human:Wido targets=stop-hook-never-forces-an-empty-turn
- 2026-09-11T20:30:31Z 117SRE9GY4WS8N9GCWPJ00SPZJ-m1-c6925449 unapprove actor=human:Wido targets=stop-hook-never-forces-an-empty-turn reason=Wido 2026-09-11: every program goal states runtime independence (claude, codex, devin, future adapters); intent amended before re-approval
Integrity: sha256=e63aceeed629840aa11fa4997121fcea05330c4ed078e46bc96cfc92d8f8571e
