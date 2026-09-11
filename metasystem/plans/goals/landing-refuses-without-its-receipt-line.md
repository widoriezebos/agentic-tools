# landing-refuses-without-its-receipt-line

- State: approved
- Priority: 1
- Sequence: 5
- Risk: severity=1 novelty=1 exposure=3 accumulation=1 basis="severity 1: a refused landing is retried with the line; novelty 1: land.sh already validates the commit; exposure 3: every landing; accumulation 1: one check"
- Tier: 2
- Intent: 48 of 67 code landings between 6 and 11 September carry no task receipt anywhere and only 7 carry it in the same commit (git-records.md section 2 of the delivery deep dive), although development/project-rules-local.md requires the receipt in the same commit as the work, so the retro's evidence base is gone. DONE means: land.sh refuses a code landing whose commit does not append a RECEIPT line for its goal to memory/receipts.log, names the missing line and the one command that writes it; records-only and receipt-only commits are exempt; proven by a fixture landing with and without the line. Goal 5 of plans/delivery-efficiency-plan.md. Tier 2 by Wido's choice 2026-09-11: one check on the landing script.
- Origin: human
- Next step: Read scripts/agents/land.sh and commit.sh and the memory/receipts.log format, add the check and its fixture, land.
- OpenedAt: 2026-09-11T15:50:31Z
- Revision: 4
- Pinned: m1e
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-11T15:51:04Z revision=4 opid=B6187M4WKGC7TV7QBVEJ6VZWC2-m1-c6925449 authority=proven digest=778d9ee0fb5638037e138f72044c1ffaa1f41d2c30cf5c8fee07acf3da6c8be5

History:
- 2026-09-11T15:50:31Z P8GF671DX0JP94MFS95CPM4H7G-m1-c6925449 open actor=human:Wido targets=landing-refuses-without-its-receipt-line reason=TierOverride: derived=3 set=2 why=Wido 2026-09-11: one check on the landing script; exposure alone does not make it tier 3, which goal 16 of the plan makes law
- 2026-09-11T15:50:38Z Y8KB1B921FG960ETKTTR5ATSR3-m1-c6925449 set-pin actor=human:Wido targets=landing-refuses-without-its-receipt-line
- 2026-09-11T15:50:51Z M4GZC1Q99QFRH9T13HSY774HAR-m1-c6925449 set-priority actor=human:Wido targets=actionable-metrics,breach-clock-and-budget-honesty,budget-extends-by-consumption-and-breach-parks,burn-without-delivery-tripwire,capped-round-continues-instead-of-restarting,carried-landing-debt-and-cap,coordinator-context-stays-under-budget,coordinator-wakes-on-events-not-polls,critique-closes-on-folded-proof,deep-battery-under-ten-minutes,delegate-job-liveness,delegate-rounds-reuse-a-warm-gate,delivery-receipt-stops-at-first-failure,failed-job-attention,fixture-stewards-outlive-their-suite,goal-abandoned-with-a-reason,human-carried-landing-carry,human-carried-landing-verb,human-goal-verbs-forgiving,job-record-birth-token,land-ready-work-lands-without-a-claim-slot,landing-receipt-survives-records-drift,landing-refuses-without-its-receipt-line,lease-sweep-death-evidence,live-job-on-a-done-goal-unnoticed,machine-concurrency-governor,proof-groups-detect-hangs-by-progress-not-the-clock,proof-harness-process-custody,retained-proof-reuse-crosses-claims-and-attempts,seat-opened-goals-name-their-blocker,severity-tiered-rigor,severity-tiered-rigor-p2,small-change-lane,steward-catchup-livelock,steward-revives-a-done-goal,stop-batch-strands-a-resumable-goal,suite-custody,token-spend-fence,turn-verdict-hardening,watch-verb,watcher-repair-request-never-names-current,winddown-census-handoff-leak reason=priority-order subject=landing-refuses-without-its-receipt-line from=unranked to=1:5 requested-sequence=5
- 2026-09-11T15:51:04Z B6187M4WKGC7TV7QBVEJ6VZWC2-m1-c6925449 approve actor=human:Wido targets=landing-refuses-without-its-receipt-line
Integrity: sha256=6b144a5bc688867498ed913f01fc15c08cde9a3c2b288c328786f0ebb2c77abb
