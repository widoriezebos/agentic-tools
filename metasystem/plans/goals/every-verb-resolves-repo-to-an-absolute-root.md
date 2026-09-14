# every-verb-resolves-repo-to-an-absolute-root

- State: approved
- Priority: 1
- Sequence: 17
- Risk: severity=2 novelty=1 exposure=2 accumulation=2 basis="severity 2: false health alarms that send seats chasing phantom corruption; novelty 1: path hygiene; exposure 2: any human or seat running verbs by hand; accumulation 2: each false alarm costs a diagnosis"
- Tier: 2
- Intent: A relative --repo must not change what a verb reports. On 2026-09-14 metasystem health --repo . reported context-budget unknown ('state root must be absolute') and BUDGET_UNKNOWN for three goals with a false 'record identity contradicts its path', while the same call with an absolute path reported alive. DONE: every verb resolves --repo and --root to an absolute, symlink-resolved installation at the CLI boundary before any reader sees it, and a table test runs each verb with a relative path and asserts identical output to the absolute one.
- Origin: human
- Next step: Resolve the root once at the command boundary for all verbs, remove the per-reader absoluteness assumptions, and add the relative-versus-absolute table test.
- OpenedAt: 2026-09-14T16:16:45Z
- Revision: 4
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-14T18:54:54Z revision=2 opid=7D3A7CMBAMP3E25093TA8J2VW4-m1e-c6925449 authority=proven digest=daa71d2b476b7f59e14748cc85134391c3ef61356378c8c90599e832e5a1d699

History:
- 2026-09-14T16:16:45Z 2FC3WFPAYJENAPGDPHC3D3Z11Z-m1e-c6925449 open actor=human:Wido targets=every-verb-resolves-repo-to-an-absolute-root
- 2026-09-14T18:54:54Z 7D3A7CMBAMP3E25093TA8J2VW4-m1e-c6925449 approve actor=human:Wido targets=every-verb-resolves-repo-to-an-absolute-root
- 2026-09-14T18:56:07Z FMD69WYXD3PN7TMPZF3YFMEPTV-m1e-c6925449 set-priority actor=human:Wido targets=every-verb-resolves-repo-to-an-absolute-root reason=priority-order subject=every-verb-resolves-repo-to-an-absolute-root from=unranked to=1:48 requested-sequence=append
- 2026-09-14T20:22:52Z ASG6PCDND0PJ3KJWZ8A2DPTPZ2-m1e-c6925449 set-priority actor=human:Wido targets=actionable-metrics,chain-landing-carries-the-reviewed-diff,degraded-stop-forms-have-one-source,dispatch-bases-chains-on-the-remote-tip,every-verb-resolves-repo-to-an-absolute-root,failed-job-attention,fixture-repo-copies-exclude-the-artifacts-store,fixture-stewards-outlive-their-suite,fixture-waits-name-their-producer,human-authority-surface-runs-its-cmd-tests,human-goal-verbs-forgiving,human-wait-does-not-consume-goal-budget,job-record-birth-token,lease-sweep-death-evidence,live-job-on-a-done-goal-unnoticed,machine-concurrency-governor,member-size-gate,metasystem-stop-escalation-proofs,metasystem-stop-fleet-form,new-elapsed-budgets-use-explicit-hours,repo-root-paths-ride-agent-commits-unjudged,run-scoped-build-caches-have-a-janitor,skipped-test-fails-its-group-on-the-other-os,stop-batch-strands-a-resumable-goal,stop-decision-surface-is-a-gate,stop-response-carries-a-structured-report-reference,testing-surfaces-declare-their-mirror,tests-never-wait-on-wall-time,token-spend-fence,watch-verb,watcher-repair-request-never-names-current,winddown-census-handoff-leak reason=priority-order subject=every-verb-resolves-repo-to-an-absolute-root from=1:48 to=1:17 requested-sequence=17
Integrity: sha256=b92bd62d6cf9e948b4d2cef648749a16bf05e3d7ed1d1c3b349b932fc7f794a1
