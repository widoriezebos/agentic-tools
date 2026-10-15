# steward-duties-run-on-every-tick

- State: queued
- Risk: severity=2 novelty=1 exposure=3 accumulation=2 basis="finding 323 (03:12 10-15, runs-advance extraction read): when the keeper reports AgentFailed on a held reservation, landingStewardTick returns before the steward's duties (cmd/metasystem/landing_agent.go:490), so stale questions, incidents and flake fixes wait until a person runs work stop; and the production Unclaimed wiring (landing_agent.go:398) has no test — removing it passes every test"
- Tier: 2
- Intent: the lane's steward duties run on every tick whatever the keeper says about its agent, and the keeper's recovery wiring is proven in production shape
- Origin: human
- Next step: U1 brief (Astra to zero or stop rule), Codex build by hand after the lane fix, Opus read, land by hand, deploy
- OpenedAt: 2026-10-15T06:29:31Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-10-15T06:29:31Z 4VJBK9ZZGBB4NMSF8Z0MT79J3W-m1e-670f65d0 open actor=human:Wido targets=steward-duties-run-on-every-tick
Integrity: sha256=fbe62e4940428407bfe2088c12048c49dc6b5b8aa73fe2e880aacebb66794ba3
