# a-continuation-job-may-write-the-stewards-state

- State: approved
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="a permission envelope of one launch role; too wide lets a continuation write where it should not, too narrow keeps every handoff blocked as today; both are caught by the unit's tests before main"
- Tier: 2
- Intent: A steward-continuation job may write the steward's own state: today its first step, session handoff --verify <nonce>, fails with "open artifacts/agents/steward/arbitration.flock: operation not permitted" because the continuation runs under the workspace sandbox, which allows writes only inside its own worktree; so every continuation's handshake now passes (since 2026-10-26 13:43) but its return is outcome=blocked at verify, the handoff never completes, and currentContinuation stays set with a terminal job (ui, 7d258d95d5fd7fa3, 16:53). The launch route's permission envelope for the steward-continuation role includes the steward's state directory (artifacts/agents/steward) for that step.
- Origin: main
- Next step: Steward cuts one brief (one unit, about 20 production lines): the steward-continuation role's launch permissions admit artifacts/agents/steward (the arbitration lock and highwater) and nothing else outside the worktree; tests: a continuation job under workspace permissions can take the arbitration lock and verify its handoff; a builder job still cannot write there. Evidence: ui's steward-7d258d95d5fd7fa3 return.json (outcome blocked) and its log. One Astra round, machine route on ui after the-impact-check-selects-only-tests-this-host-can-run, lane landing. a-round-survives-a-restart U1 D5 settles the ended continuation meanwhile.
- OpenedAt: 2026-10-26T16:08:13Z
- Revision: 2
- Budget: elapsedLimit=5d attemptLimit=12 reservedJobMinutesLimit=2400 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- NormApproval: approvedRef=X36JVFN9B996851H9H5ANJ15NV-m1e-acb921fd minutes=2400 reviewRounds=20 goalRevision=1
- Approved: by=human:Wido at=2026-10-26T16:10:40Z revision=2 opid=X36JVFN9B996851H9H5ANJ15NV-m1e-acb921fd authority=proven digest=b37a53cff313bba70316c475ef7b6d376a8289367c115f8d057e1882ba7ad787 episode=2

History:
- 2026-10-26T16:08:13Z 9YZJQ27TQ66R4JW25M18G4ARFY-m1e-acb921fd open actor=human:Wido targets=a-continuation-job-may-write-the-stewards-state
- 2026-10-26T16:10:40Z X36JVFN9B996851H9H5ANJ15NV-m1e-acb921fd approve actor=human:Wido targets=a-continuation-job-may-write-the-stewards-state
Integrity: sha256=2a73df58627efdd7fae4a22c7d230b74808ae7a9d0ee820ca14a00db156f05bb
