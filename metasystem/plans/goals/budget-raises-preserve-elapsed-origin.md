# budget-raises-preserve-elapsed-origin

- State: claimed
- Priority: 1
- Sequence: 8
- Risk: severity=3 novelty=2 exposure=3 accumulation=1 basis="severity 3: budget enforcement can stop lawful work or permit unbounded time; novelty 2: a bounded change to existing ledger and elapsed projection; exposure 3: shared goal execution across seats; accumulation 1: each episode is evaluated independently, with no new aggregate accounting"
- Tier: 3
- Intent: A human-approved goal budget increase preserves the current ownership episode elapsed origin instead of restarting the breach clock. Preserve obligation-discharge and supersession semantics, backward-readable legacy claims, existing human authority, and AccountingRevision reservation accounting. A fresh claim or explicit resume starts a new episode. Prove repeated raises cannot buy unrecorded elapsed time and do not rewind a consumed discharge origin.
- Origin: main
- Next step: First independent successor of goal:breach-clock-and-budget-honesty under Wido order to split large work before implementation. Use accepted Fix 1 in plans/breach-clock-and-budget-honesty-design.md and the current-code slice design by job breach-episode-slice-design-20260911. Reuse review decisions, not missing old source. Excludes duration grammar, human-wait intervals, machine quota, and attempt-accounting changes. Parent remains open until all successors are delivered. Evidence: /Users/wido/metasystem-evidence/agentic-tools/breach-clock-20260911.
- OpenedAt: 2026-09-11T05:37:41Z
- Revision: 6
- Labels: breach-clock-successor
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-11T05:45:42Z revision=2 opid=RXCHH0R93MBESKXPH2S9WTJA7C-m1c-66a02980 authority=proven digest=3fb96a169fb5b47c4de2fd150a1fad7178b3911b30664ac39fa8b5b0b4a8ee87
- Sliced: machine=m1c lineage=main-1789023008-75159-b69143 revision=4 at=2026-09-11T05:50:40Z
- AcceptedRisk: finding=BEO-R1-001 chain=budget-elapsed-origin-design-critic-20260911 by=Wido opid=8F2S0BM5Y8J6JWXPV73Y53ZC5N-m1c-66a02980
- Claimed: machine=m1c lineage=main-1789023008-75159-b69143 at=2026-09-11T05:48:36Z revision=4 accountingRevision=4
- StopCapability: generation=4 revision=4 machine=m1c claimEpoch=4 fenceEpoch=0

History:
- 2026-09-11T05:37:41Z QNCZWFZV355WCWBKX5R5W92GYP-m1c-66a02980 open actor=m1c+main-1789023008-75159-b69143 targets=budget-raises-preserve-elapsed-origin
- 2026-09-11T05:45:42Z RXCHH0R93MBESKXPH2S9WTJA7C-m1c-66a02980 approve actor=human:Wido targets=budget-raises-preserve-elapsed-origin
- 2026-09-11T05:45:46Z T2AE5EG8JMKA86VEPX77M7MHGY-m1c-66a02980 set-priority actor=human:Wido targets=actionable-metrics,breach-clock-and-budget-honesty,budget-raises-preserve-elapsed-origin,burn-without-delivery-tripwire,delegate-job-liveness,failed-job-attention,fixture-stewards-outlive-their-suite,governed-exhaustion-reprojection,human-goal-verbs-forgiving,job-record-birth-token,lease-sweep-death-evidence,live-job-on-a-done-goal-unnoticed,machine-concurrency-governor,proof-harness-process-custody,review-round-limit-counts-per-chain,severity-tiered-rigor,severity-tiered-rigor-p2,small-change-lane,steward-catchup-livelock,steward-revives-a-done-goal,stop-batch-strands-a-resumable-goal,suite-custody,token-spend-fence,turn-verdict-hardening,watch-verb,watcher-repair-request-never-names-current,winddown-census-handoff-leak reason=priority-order subject=budget-raises-preserve-elapsed-origin from=unranked to=1:8 requested-sequence=8
- 2026-09-11T05:48:36Z Q24KKTKGQ050CCGRX406R416WZ-m1c-66a02980 claim actor=m1c+main-1789023008-75159-b69143 targets=budget-raises-preserve-elapsed-origin
- 2026-09-11T05:50:40Z DDRN86HW5NF1AVJFXQ115J42GB-m1c-66a02980 slice-start actor=m1c+main-1789023008-75159-b69143 targets=budget-raises-preserve-elapsed-origin
- 2026-09-11T06:25:24Z 8F2S0BM5Y8J6JWXPV73Y53ZC5N-m1c-66a02980 accept-risk actor=human:Wido targets=budget-raises-preserve-elapsed-origin reason=Codex acting under Wido delegated approval of 2026-09-11. Native design critic round 2 returned zero material findings at source 6c32a388 and supplement SHA256 5ea46bfdb668635e97e9eacb477317b771f0c2c47cc4e9137c67d9a3040a030a, after accepting and correcting this finding. Its empty findings list did not emit the explicit material=false withdrawal that the register requires, so the old register row remains open. Approve ending design review and implementing the corrected contract. This accepts only the remaining design-to-implementation risk; it does not waive the specified regression, code critique, runtime canary, or required selected delivery proof. The claim validator must enforce strict EpisodeRevision < EpisodeObligationRevision < Claimed.Revision and prove invalid endpoint/out-of-range and valid interior cases.
Integrity: sha256=d3a6b143144311bfa82c53a1ef26730eb472de65f59c1ec8f5eeff924d556e7c
