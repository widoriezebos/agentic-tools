# machinery-blocks-of-2026-10-02

- State: queued
- Priority: 2
- Sequence: 49
- Risk: severity=1 novelty=1 exposure=2 accumulation=2 basis="Each block stops an agent but the escape hatch asks the human (severity 1); known defects with traced causes (novelty 1); every seat (exposure 2); recurs on every run (accumulation 2)"
- Tier: 1
- Intent: Fix, by frequency once the unblock log shows them, the eight machinery blocks one seat hit on 2026-10-02 (each one a human question under blocked-agent-asks-the-human until fixed): (1) goal claim without METASYSTEM_OWNER_LINEAGE is read as a person and the refusal names the wrong cause (intent_planning.go:474-540); (2) critique chain rounds are filed under the previous round's job: collect says findings cannot be read, a zero-finding final round cannot close (design-critic-09e95ff70db9b256797eb586); (3) work review G --patch is refused by the seat's own commit guard, and the goal-free read failed without feedback; (4) with a lane registered, work land refuses any non-goal commit; (5) the Stop hook blocks every turn while the agent waits on in-process subagents and costs 15-16 s of a 15 s threshold; (6) 193 open alerts, mostly stale, and health roles with 'no lawful remedy'; (7) stale source copies under artifacts/ break go vet ./... and the static gate; (8) messages whose remedy is wrong ('the goals changed meanwhile; try again' after an approved-intent or not-open refusal). Evidence: agentic-tools-evidence/flaky-20261002/log.md.
- Origin: main
- Next step: Wait for the unblock log after switch-on; fix the most frequent block first, one at a time, by subtraction
- OpenedAt: 2026-10-02T12:29:50Z
- Revision: 3
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0

History:
- 2026-10-02T12:29:50Z 2R9JM2HYAZY2AZW272ZF1ZEV2X-m1e-9c612d71 open actor=human:Wido targets=machinery-blocks-of-2026-10-02
- 2026-10-02T12:29:57Z B8YKA2JDWBD1NZBQ3DB175XF2C-m1e-9c612d71 set-priority actor=human:Wido targets=machinery-blocks-of-2026-10-02 reason=priority-order subject=machinery-blocks-of-2026-10-02 from=unranked to=2:50 requested-sequence=append
- 2026-10-02T15:09:24Z SZGVJ23Q1Q1ZCKPKR7DBN6RPY6-ui-31a738e9 done actor=human:Wido targets=actionable-metrics,adoption-inventory-from-install-set,arming-binds-the-installed-engine-identity,commit-goal-binding,dependency-ratchet-refuses-new-wall-clock-and-layout-asserts,design-gate-at-dispatch,enrollment-proves-a-human-not-a-terminal,failed-job-attention,first-headless-run,fleet-pull,gate-governance-records,headless-continuous-delivery-proof,headless-fleet-coordination-proof,hook-enrollment-per-checkout,human-authority-surface-runs-its-cmd-tests,human-break-glass-commit-keeps-the-ledger-valid,human-wait-does-not-consume-goal-budget,idle-every-runtime-enforcement,land-verb-writes-the-receipt,landing-design-provenance,law-keys-read-committed-bytes,lease-sweep-death-evidence,live-job-on-a-done-goal-unnoticed,machine-concurrency-governor,machinery-blocks-of-2026-10-02,never-idle-ironclad,new-elapsed-budgets-use-explicit-hours,one-approval-gate,one-goal-act-one-ledger-commit,recovery-rehearsal,recovery-to-good-state,reviewers-check-the-rulings,role-context-composition,runtime-limit-is-not-a-protocol-error,steward-revivals-leave-the-receipt-stream,stop-batch-strands-a-resumable-goal,token-spend-fence,transport-remote-absent-refuses-every-landing,trunk-versus-candidate-is-a-verification-step,watcher-repair-request-never-names-current,winddown-census-handoff-leak reason=priority-order from=2:50 to=2:49
Integrity: sha256=a44718145f470044b04e9111fd5bc647e63c0ab8f203f68b929b0d62d52e8909
