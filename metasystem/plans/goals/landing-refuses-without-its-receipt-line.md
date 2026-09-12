# landing-refuses-without-its-receipt-line

- State: claimed
- Priority: 1
- Sequence: 5
- Risk: severity=1 novelty=1 exposure=3 accumulation=1 basis="severity 1: a refused landing is retried with the line; novelty 1: land.sh already validates the commit; exposure 3: every landing; accumulation 1: one check"
- Tier: 2
- Intent: 48 of 67 code landings between 6 and 11 September carry no task receipt anywhere and only 7 carry it in the same commit (git-records.md section 2 of the delivery deep dive), although development/project-rules-local.md requires the receipt in the same commit as the work, so the retro's evidence base is gone. DONE means: land.sh refuses a code landing whose commit does not append a RECEIPT line for its goal to memory/receipts.log, names the missing line and the one command that writes it; records-only and receipt-only commits are exempt; proven by a fixture landing with and without the line. Goal 5 of plans/delivery-efficiency-plan.md. Tier 2 by Wido's choice 2026-09-11: one check on the landing script. RUNTIME INDEPENDENCE (plan principle 0; architecture doctrine of 2026-08-16: correctness never depends on an accelerator): this holds for every runtime the roster can host today (claude, codex, devin) and for future adapters such as opencode; the mechanism lives in the Go engine, the ledger verbs and the adapter contract, never in one runtime's hook, harness, CLI or transcript format; a native facility may accelerate it through its adapter, and an agent on any runtime with no accelerator gets the same guarantee from the records and the verb alone; where the goal touches a runtime path, DONE is proven on at least two runtimes.
- Origin: human
- Next step: Built 2026-09-12: engine verb landing receipt-line (internal/landing/receiptline.go), land.sh step after staging in both flows, land-fixtures leg receipt-line, docs/project-rules.md. go test internal/landing + cmd/metasystem green, go-gate --fast green; section/land-fixtures through the engine and the one critic read in flight; then land via the human terminal and conclude.
- OpenedAt: 2026-09-11T15:50:31Z
- Revision: 10
- Pinned: m1b
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-11T20:33:10Z revision=7 opid=DWQ8CW24MN97QSVKMFZ36DGZK7-m1-c6925449 authority=proven digest=4450556aa366cf6847449506cbdbc53f41a644cb2dcf87c64b35f0de4ef861cd
- Claimed: machine=m1b lineage=main-1789191336-90295-e4b24b at=2026-09-12T06:07:26Z revision=9 accountingRevision=9 episodeAt=2026-09-12T06:07:26Z episodeRevision=9
- StopCapability: generation=9 revision=9 machine=m1b claimEpoch=2 fenceEpoch=0

History:
- 2026-09-11T15:50:31Z P8GF671DX0JP94MFS95CPM4H7G-m1-c6925449 open actor=human:Wido targets=landing-refuses-without-its-receipt-line reason=TierOverride: derived=3 set=2 why=Wido 2026-09-11: one check on the landing script; exposure alone does not make it tier 3, which goal 16 of the plan makes law
- 2026-09-11T15:50:38Z Y8KB1B921FG960ETKTTR5ATSR3-m1-c6925449 set-pin actor=human:Wido targets=landing-refuses-without-its-receipt-line
- 2026-09-11T15:50:51Z M4GZC1Q99QFRH9T13HSY774HAR-m1-c6925449 set-priority actor=human:Wido targets=actionable-metrics,breach-clock-and-budget-honesty,budget-extends-by-consumption-and-breach-parks,burn-without-delivery-tripwire,capped-round-continues-instead-of-restarting,carried-landing-debt-and-cap,coordinator-context-stays-under-budget,coordinator-wakes-on-events-not-polls,critique-closes-on-folded-proof,deep-battery-under-ten-minutes,delegate-job-liveness,delegate-rounds-reuse-a-warm-gate,delivery-receipt-stops-at-first-failure,failed-job-attention,fixture-stewards-outlive-their-suite,goal-abandoned-with-a-reason,human-carried-landing-carry,human-carried-landing-verb,human-goal-verbs-forgiving,job-record-birth-token,land-ready-work-lands-without-a-claim-slot,landing-receipt-survives-records-drift,landing-refuses-without-its-receipt-line,lease-sweep-death-evidence,live-job-on-a-done-goal-unnoticed,machine-concurrency-governor,proof-groups-detect-hangs-by-progress-not-the-clock,proof-harness-process-custody,retained-proof-reuse-crosses-claims-and-attempts,seat-opened-goals-name-their-blocker,severity-tiered-rigor,severity-tiered-rigor-p2,small-change-lane,steward-catchup-livelock,steward-revives-a-done-goal,stop-batch-strands-a-resumable-goal,suite-custody,token-spend-fence,turn-verdict-hardening,watch-verb,watcher-repair-request-never-names-current,winddown-census-handoff-leak reason=priority-order subject=landing-refuses-without-its-receipt-line from=unranked to=1:5 requested-sequence=5
- 2026-09-11T15:51:04Z B6187M4WKGC7TV7QBVEJ6VZWC2-m1-c6925449 approve actor=human:Wido targets=landing-refuses-without-its-receipt-line
- 2026-09-11T20:30:35Z BYJ0MTDXRFF6G7Z8M4J89XNVG1-m1-c6925449 unapprove actor=human:Wido targets=landing-refuses-without-its-receipt-line reason=Wido 2026-09-11: every program goal states runtime independence (claude, codex, devin, future adapters); intent amended before re-approval
- 2026-09-11T20:31:54Z DM1D3QCJDQYJBWHVRYDS2BYK6S-m1e-3ca84c28 edit actor=m1e+main-1789158700-39729-b9c0d2 targets=landing-refuses-without-its-receipt-line
- 2026-09-11T20:33:10Z DWQ8CW24MN97QSVKMFZ36DGZK7-m1-c6925449 approve actor=human:Wido targets=landing-refuses-without-its-receipt-line
- 2026-09-12T05:28:26Z 1M075XT14WZM0R275BBMJFVB85-m1e-c6925449 set-pin actor=human:Wido targets=landing-refuses-without-its-receipt-line
- 2026-09-12T06:07:26Z B2PGHP7FCFYRK1F5K0143KYYGN-m1b-30a7e141 claim actor=m1b+main-1789191336-90295-e4b24b targets=landing-refuses-without-its-receipt-line
- 2026-09-12T06:43:17Z QSB0620FAHAY4E16FQF7T0CK73-m1b-30a7e141 edit actor=m1b+main-1789191336-90295-e4b24b targets=landing-refuses-without-its-receipt-line
Integrity: sha256=07e786d09c51574c393e694ab304ae86f2dac47a6ad8c38764314397a08c85fd
