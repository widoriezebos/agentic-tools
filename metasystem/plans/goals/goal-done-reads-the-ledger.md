# goal-done-reads-the-ledger

- State: queued
- Risk: severity=1 novelty=1 exposure=2 accumulation=1 basis="one check in the conclusion verb; a wrong lookup refuses a conclusion or accepts a dangling link, never touches code or main"
- Tier: 1
- Intent: goal done checks a conclusion's residue link (goal:<id>) against the accepted goal ledger through its owner, not against the local checkout's plans/goals folder (internal/goal/verbs.go:2420-2427 stats the local file), so a seat whose local main lags origin can conclude its goal without a person fast-forwarding the checkout. Evidence: the-impact-check-runs-what-changed, 2026-10-26 05:00-05:05, two refusals and one hand fast-forward (panes/done-impact-check-0503.sh, -0505.sh).
- Origin: main
- Next step: Steward cuts one brief (one unit, about 20 production lines): Done resolves each residue link through the ledger owner that goal open publishes to, with the local folder as a fallback only when the owner is unreachable and says so; tests: a link present on the ledger but absent locally resolves; a link absent on both is refused with today's words. One Astra round, machine route on ui after the-impact-check-selects-only-tests-this-host-can-run, lane landing.
- OpenedAt: 2026-10-26T09:12:23Z
- Revision: 1
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0

History:
- 2026-10-26T09:12:23Z A42B1D0DY42BXJS9P4RSHXK1J8-m1e-acb921fd open actor=human:Wido targets=goal-done-reads-the-ledger
Integrity: sha256=6cc65562e63cada970d3d97c9f75166a7bdaae0a43cd5b317134ada7b591ef36
