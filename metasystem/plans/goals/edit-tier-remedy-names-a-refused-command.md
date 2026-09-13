# edit-tier-remedy-names-a-refused-command

- State: queued
- Priority: 3
- Sequence: 4
- Risk: severity=1 novelty=1 exposure=1 accumulation=1 basis="severity 1: a human loses one round trip; novelty 1: message text and one flag rule; exposure 1: only humans re-tiering an approved goal; accumulation 1: repeats on each re-tier"
- Tier: 1
- Intent: goal edit's refusal on an approved goal says unapprove it, edit --tier, then approve it, but goal edit --tier alone is refused with answer the four questions, and a tier above the derived one also needs --why; the human ran the printed remedy 2026-09-06 20:02Z, the tier edit was refused, and the goal was re-approved unchanged. Also unneeded here: the review-round limit is counted per critic chain, so a fresh critic chain never needs a higher tier. DONE means the remedy prints a command the CLI accepts, or edit --tier alone is admitted when the risk record already stands, with a fixture
- Origin: main
- Next step: decide between admitting edit --tier alone against the standing risk record or printing the full command with --risk --basis --why; fixture; grep the remedy text in internal/goal/verbs.go
- OpenedAt: 2026-09-06T20:05:11Z
- Revision: 3
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0

History:
- 2026-09-06T20:05:11Z 1VZ8G0MAHDCA3MPVD59YKGSZMN-m1b-c6925449 open actor=m1b+main-1788680071-18713-e76d5d targets=edit-tier-remedy-names-a-refused-command
- 2026-09-08T15:59:22Z GNNH301B3DN9BQJD3A641D76CC-m1-7cd0bd60 set-priority actor=human:Wido targets=edit-tier-remedy-names-a-refused-command reason=priority-order subject=edit-tier-remedy-names-a-refused-command from=unranked to=3:5 requested-sequence=5
- 2026-09-13T08:19:03Z CSQ5ADFKGC0RCY6RXRSVZ9E89W-m1-c6925449 done actor=human:Wido targets=answer-archive,app-doctrine,app-guardrail-custody,app-guardrail-program,brief-authority-preflight,claude-delegate-scratch-cleanup,claude-denywrite-list-is-a-snapshot,claude-network-allow-does-not-allow,conformance-runtime-state-litter,continuous-self-improvement,counselor,cross-repo-guardrails,defect-analysis-gate,delegate-sandbox-cannot-run-the-beds,design-prohibition-is-role-scoped,edit-tier-remedy-names-a-refused-command,effort-by-complexity-per-model,empty-brief-admitted,fable-effort-high,fixture-default-branch-assumption,fixture-enrollment-notes,follow-up-brief-cannot-cite-fresh-trunk-files,follow-up-rebase-drop-failure-names-hash,gap-rule-deletes-built-work,goal-by-flag-doubles-human-prefix,goal-done-flags-do-not-match-the-verb-table,goal-suite-sits-on-the-default-test-timeout,hook-demands-what-landing-forbids,host-health-role,human-carried-landing,incident-proposal-drafting,integration-branch,land-verb-pruning,landing-refusal-names-the-record-rule,ledger-authentication,malformed-return-field-loses-the-round,metasystem-way-patterns,missionrunner-terminate-flake,narrator-digests,norm-approval-reads-a-file-the-ledger-does-not-carry,paid-proof-authorization,precedent-index,python3-kit-port,reconciliation-guards,registry-design-names-selection-rows,repo-flag-resolves-one-root,resume-reclaims-on-handover,runtime-limit-is-not-a-protocol-error,same-process-succession,seam-declaration-convergence,seam-eventstream-error-surface,shutdown-claims-more-than-it-stops,standing-validation,steward-owned-execution,stop-message-truth,uncapped-delegate-fanout,verbs-match-intent,vm-epoch-identity-drift,watchdog-kill-observation-gap reason=priority-order from=3:5 to=3:4
Integrity: sha256=d87e7ae56f12ef7e543c03fc56dab6cae68600cf374c9a1f593f12d43775bbb1
