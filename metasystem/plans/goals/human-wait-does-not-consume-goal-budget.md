# human-wait-does-not-consume-goal-budget

- State: approved
- Priority: 1
- Sequence: 38
- Risk: severity=3 novelty=2 exposure=3 accumulation=1 basis="severity 3: budget enforcement can stop lawful work or permit unbounded time; novelty 2: a bounded change to existing ledger and elapsed projection; exposure 3: shared goal execution across seats; accumulation 1: each episode is evaluated independently, with no new aggregate accounting"
- Tier: 3
- Intent: Time a goal spends blocked on a recorded human act does not count toward its elapsed limit. Show waiting time separately and report waiting on the human since the recorded start rather than counting down to a breach. Define reliable evidence for human-wait start/end around existing next-step or plan records, goal asks and refused human verbs, without allowing arbitrary agent prose to create unrecorded free execution time.
- Origin: main
- Next step: Third independent successor of goal:breach-clock-and-budget-honesty, preserving the requirement added 2026-09-09 after its reviewed design. Specimen: breach-stop-wedges-seat claimed 10:07Z with 4h, next step WAITING ON WIDO from 11:47Z, custodian stopped it 16:08Z at 6h01m including the wait; the requested raise then required a resume. Needs its own bounded Codex Astra xhigh design for authoritative interval records, end/resume rules, subtraction and reporting through existing goal and ProjectBudget owners. Depends on a stable elapsed origin from goal:budget-raises-preserve-elapsed-origin. No implementation in the elapsed-origin slice.
- OpenedAt: 2026-09-11T05:37:49Z
- Revision: 7
- Labels: breach-clock-successor
- Pinned: m1c
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-13T07:45:09Z revision=3 opid=P9Y8R8HW09N69R50DW6NJDCVBT-m1e-c6925449 authority=proven digest=f3d1f7dc879d867ba326b5c9f93a6bb778cc783b54ccbbb854bd9cb71fe27d77

History:
- 2026-09-11T05:37:49Z 0JFW3QGPK4HM1ZGPW2XHP5411V-m1c-66a02980 open actor=m1c+main-1789023008-75159-b69143 targets=human-wait-does-not-consume-goal-budget
- 2026-09-13T07:45:05Z H6FRBQJBFKEXEKG6RTZAA2017F-m1e-c6925449 set-pin actor=human:Wido targets=human-wait-does-not-consume-goal-budget
- 2026-09-13T07:45:09Z P9Y8R8HW09N69R50DW6NJDCVBT-m1e-c6925449 approve actor=human:Wido targets=human-wait-does-not-consume-goal-budget
- 2026-09-13T07:45:14Z BQBYNH25Y7X8WT7YZ900C17NAK-m1e-c6925449 set-priority actor=human:Wido targets=human-wait-does-not-consume-goal-budget reason=priority-order subject=human-wait-does-not-consume-goal-budget from=unranked to=1:41 requested-sequence=41
- 2026-09-13T08:18:55Z B5G9KBS61Q28BAQ0ETN1G2V4AY-m1-c6925449 done actor=human:Wido targets=actionable-metrics,breach-clock-and-budget-honesty,chain-landing-carries-the-reviewed-diff,closing-read-and-receipt-in-parallel,dispatch-bases-chains-on-the-remote-tip,failed-job-attention,fixture-repo-copies-exclude-the-artifacts-store,fixture-stewards-outlive-their-suite,fixture-waits-name-their-producer,human-authority-surface-runs-its-cmd-tests,human-goal-verbs-forgiving,human-wait-does-not-consume-goal-budget,job-record-birth-token,lease-sweep-death-evidence,live-job-on-a-done-goal-unnoticed,machine-concurrency-governor,metasystem-stop-escalation-proofs,metasystem-stop-fleet-form,new-elapsed-budgets-use-explicit-hours,repo-root-paths-ride-agent-commits-unjudged,run-scoped-build-caches-have-a-janitor,skipped-test-fails-its-group-on-the-other-os,steward-catchup-livelock,steward-revives-a-done-goal,stop-batch-strands-a-resumable-goal,stop-refusal-fits-on-one-screen,suite-custody,token-spend-fence,watch-verb,watcher-repair-request-never-names-current,winddown-census-handoff-leak reason=priority-order from=1:41 to=1:40
- 2026-09-13T08:19:54Z J2760PA4M0G8M7F0NHSQVMX86Y-m1-c6925449 done actor=human:Wido targets=closing-read-and-receipt-in-parallel,dispatch-bases-chains-on-the-remote-tip,human-wait-does-not-consume-goal-budget,metasystem-stop-escalation-proofs,metasystem-stop-fleet-form,new-elapsed-budgets-use-explicit-hours,stop-refusal-fits-on-one-screen reason=priority-order from=1:40 to=1:39
- 2026-09-13T08:20:48Z 7EHQ87RTF0H4FCT7E5T5RAWMW3-m1-c6925449 done actor=human:Wido targets=actionable-metrics,chain-landing-carries-the-reviewed-diff,dispatch-bases-chains-on-the-remote-tip,fixture-repo-copies-exclude-the-artifacts-store,fixture-stewards-outlive-their-suite,fixture-waits-name-their-producer,human-authority-surface-runs-its-cmd-tests,human-wait-does-not-consume-goal-budget,lease-sweep-death-evidence,machine-concurrency-governor,metasystem-stop-escalation-proofs,metasystem-stop-fleet-form,new-elapsed-budgets-use-explicit-hours,repo-root-paths-ride-agent-commits-unjudged,run-scoped-build-caches-have-a-janitor,skipped-test-fails-its-group-on-the-other-os,steward-catchup-livelock,stop-refusal-fits-on-one-screen,suite-custody,token-spend-fence,watcher-repair-request-never-names-current,winddown-census-handoff-leak reason=priority-order from=1:39 to=1:38
Integrity: sha256=d9cf4d4e92135275616d7a5c5ad023e2b08e33344b39332f18ff5479d2033e07
