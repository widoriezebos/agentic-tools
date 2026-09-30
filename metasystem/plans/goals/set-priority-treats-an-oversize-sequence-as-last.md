# set-priority-treats-an-oversize-sequence-as-last

- State: queued
- Priority: 3
- Sequence: 41
- Risk: severity=1 novelty=1 exposure=2 accumulation=1 basis="severity 1: a refused reorder, no wrong ledger state; novelty 1: clamp a bound that already has an append form; exposure 2: every human or seat reorder that names a position; accumulation 1: each refusal is visible and retried"
- Tier: 1
- Intent: What: The goal prioritize command accepts a position beyond the end of the queue and puts the goal last, reporting the position it actually used. Why: Asking for position 90 in a queue of 62 is refused today, although the only sensible meaning is "last". It broke a batch of Wido's reorderings on 09-15. Pros: Fewer pointless refusals when reordering the backlog. Cons: A typo in a position moves a goal to the end instead of being caught; reporting the used position keeps that visible.
- Origin: human
- Next step: Next: In internal/goal/order.go, turn a position past the end into "last" and report the position used; keep refusing positions below 1; update order_test and the two UI tests that expect the refusal (ui/act/act_test.go:257 and ui/httpd/walkthrough/main.go:1580, so tell the ui seat). Done when: goal prioritize with sequence 999 puts the goal last, says which position it got, and the queue stays numbered 1 to N without gaps.
- OpenedAt: 2026-09-15T06:06:03Z
- Revision: 6
- BudgetExceptions: 0

History:
- 2026-09-15T06:06:03Z 56ZG34WXN2DVPR59QM322CRG8B-m1e-c6925449 open actor=human:Wido targets=set-priority-treats-an-oversize-sequence-as-last
- 2026-09-15T06:06:09Z 4R1Q8B5DY6JN28KCAP8WQ3HW6V-m1e-c6925449 approve actor=human:Wido targets=set-priority-treats-an-oversize-sequence-as-last
- 2026-09-15T06:06:15Z STNM6WK6WE657ZSDC7B9N36KE8-m1e-c6925449 set-priority actor=human:Wido targets=set-priority-treats-an-oversize-sequence-as-last reason=priority-order subject=set-priority-treats-an-oversize-sequence-as-last from=unranked to=3:42 requested-sequence=append
- 2026-09-16T20:35:19Z 05KYTN17SVSY9CC3DCR8JXE1PV-m1e-c6925449 unapprove actor=human:Wido targets=set-priority-treats-an-oversize-sequence-as-last reason=Wido 2026-09-16 22:40 CEST: clean house on priority 1 first, then a few human convenience features; every priority 2+ goal is unapproved until then
- 2026-09-16T21:02:51Z HE021GFD8V1YPVBEXGSBFRHWV0-m1e-c6925449 set-priority actor=human:Wido targets=app-doctrine,app-guardrail-custody,app-guardrail-program,claude-delegate-scratch-cleanup,claude-denywrite-list-is-a-snapshot,continuous-self-improvement,counselor,cross-repo-guardrails,delegate-sandbox-cannot-run-the-beds,design-prohibition-is-role-scoped,effort-by-complexity-per-model,fixture-default-branch-assumption,goal-suite-sits-on-the-default-test-timeout,human-carried-landing,incident-proposal-drafting,integration-branch,land-verb-pruning,ledger-authentication,metasystem-way-patterns,missionrunner-terminate-flake,paid-proof-authorization,precedent-index,python3-kit-port,reconciliation-guards,registry-design-names-selection-rows,repo-flag-resolves-one-root,runtime-limit-is-not-a-protocol-error,seam-declaration-convergence,set-priority-treats-an-oversize-sequence-as-last,standing-validation,uncapped-delegate-fanout,watchdog-kill-observation-gap reason=priority-order subject=runtime-limit-is-not-a-protocol-error from=3:42 to=3:41 requested-sequence=54
- 2026-09-30T18:46:04Z NZJAXSP7D74BTZGZGQXE14GG3Z-m1e-b6a4eb0a edit actor=human:wido targets=set-priority-treats-an-oversize-sequence-as-last
Integrity: sha256=ebe4b18ec7acb2b319f2498019c2eb918675f1e34304a194cf311db01d5898c8
