# receipt-section-beds-do-not-export-the-engine

- State: claimed
- Priority: 1
- Sequence: 1
- Risk: severity=2 novelty=1 exposure=3 accumulation=1 basis="severity 2: every landing on every seat is refused until the two beds pass again; novelty 1: the change restores the bed environment of the day before; exposure 3: the receipt gate runs on every seat; accumulation 1: nothing is built on the variable"
- Tier: 3
- Intent: Since 016b83e2 the receipt's section beds run with METASYSTEM_BIN set to the candidate engine in the receipt worktree; every agent script copied into a fixture's scratch repository honours that variable, so up, steward arm and the pre-commit guard inside scratch repositories run the worktree engine instead of the copy enrolled there: supervision-and-census-fixtures fails every process scenario with ENROLLMENT_DRIFT and adoption-fixtures fails its new-plan guard leg, on every receipt whose enrolled engine carries 016b83e2 (proof-mtvn9elr on tip 53d89e2c). The candidate is already installed at the bed's own bin path where the default resolves. DONE: section beds carry no METASYSTEM_BIN (inherited or added; the go groups that consume the candidate engine keep theirs), the section test asserts the installed path and the absent variable, and a receipt taken on a tip at or after 016b83e2 passes both beds.
- Origin: main
- Next step: Small-change lane, high risk: one MECHANICAL implementer round (two files, a handful of lines), then one code-critic read because the receipt gate is fleet-wide; receipt; land with the chain. Opened by m1b 2026-09-10 17:45Z under the four-day authority; member of the-metasystem-validates-itself-with-itself (repairs member 1, landed as 016b83e2). Until it lands, no seat can take a sufficient receipt on a tip at or after 016b83e2.
- OpenedAt: 2026-09-10T15:39:22Z
- Revision: 5
- Labels: bootstrap
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=600 activeJobLimit=3 reviewRoundLimit=2
- BudgetExceptions: 1
- Approved: by=human:Wido at=2026-09-10T15:40:37Z revision=5 opid=AWTM4M23HHCEQY9YK69CB5WFDF-m1b-c6925449 authority=proven digest=70b788d9be914d903526cc702ea34a0bdbbee7e669300725057dc62659153c65
- Claimed: machine=m1b lineage=main-1788680071-18713-e76d5d at=2026-09-10T15:40:37Z revision=5 accountingRevision=5
- StopCapability: generation=5 revision=5 machine=m1b claimEpoch=1 fenceEpoch=0

History:
- 2026-09-10T15:39:22Z AVMNAVT6BFK2RJ1KMT8W35B8SV-m1b-c6925449 open actor=m1b+main-1788680071-18713-e76d5d targets=receipt-section-beds-do-not-export-the-engine
- 2026-09-10T15:40:06Z 5R666BKDY991ZHPBQJXA59ENQ8-m1b-c6925449 approve actor=human:Wido targets=receipt-section-beds-do-not-export-the-engine
- 2026-09-10T15:40:09Z 1WTD8HRVWQJCEY48YP2CBHA1VZ-m1b-c6925449 set-priority actor=human:Wido targets=actionable-metrics,breach-clock-and-budget-honesty,burn-without-delivery-tripwire,carried-landing-debt-and-cap,delegate-job-liveness,delivery-candidate-is-the-workspace-not-the-ledger,enrollment-binds-by-the-skew-rule-not-the-tip,failed-job-attention,fixture-stewards-outlive-their-suite,goal-abandoned-with-a-reason,governed-exhaustion-reprojection,human-carried-landing-carry,human-carried-landing-verb,human-goal-verbs-forgiving,job-record-birth-token,landing-receipt-survives-records-drift,landing-runs-standard-deep-runs-at-cadence,lease-sweep-death-evidence,live-job-on-a-done-goal-unnoticed,machine-concurrency-governor,proof-harness-process-custody,receipt-section-beds-do-not-export-the-engine,review-round-limit-counts-per-chain,severity-tiered-rigor,severity-tiered-rigor-p2,skew-preflight-knows-a-receipt-worktree,small-change-lane,steward-catchup-livelock,steward-revives-a-done-goal,stop-batch-strands-a-resumable-goal,suite-custody,testing-contract-owns-record-paths,token-spend-fence,turn-verdict-hardening,watch-verb,watcher-repair-request-never-names-current,winddown-census-handoff-leak reason=priority-order subject=receipt-section-beds-do-not-export-the-engine from=unranked to=1:1 requested-sequence=1
- 2026-09-10T15:40:33Z XYJ6FDWKWGHH37E8YBQSKB47Q7-m1b-c6925449 claim actor=m1b+main-1788680071-18713-e76d5d targets=receipt-section-beds-do-not-export-the-engine
- 2026-09-10T15:40:37Z AWTM4M23HHCEQY9YK69CB5WFDF-m1b-c6925449 set-budget actor=human:Wido targets=receipt-section-beds-do-not-export-the-engine
Integrity: sha256=0d8a96c598df7f9ef08b602674a81300f80ff7090c98e65ef42368fb973674c1
