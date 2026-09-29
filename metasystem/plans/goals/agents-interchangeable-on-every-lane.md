# agents-interchangeable-on-every-lane

- State: queued
- Risk: severity=2 novelty=2 exposure=2 accumulation=1 basis="A wrong default sends work to an agent nobody chose, but every run records its agent and the change reverts cleanly; the Devin adapter and auto resolution are new; the defaults ship to every adopted repository; it is a one-off change."
- Tier: 2
- Intent: Any agent on the host runs every lane: Devin runs the launch lanes, and the default agent is the first of claude, codex, devin found on PATH, with per-runtime models from configuration.
- Origin: human
- Next step: Land the built change with work land --message after its risk-selected tests pass.
- OpenedAt: 2026-09-29T13:02:16Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-29T13:02:16Z 0NG4QB97GEVGG45RMRZT618TP6-wr-m1-3164cf85 open actor=human:W targets=agents-interchangeable-on-every-lane
Integrity: sha256=bbb207bbee9cf9f77463f61ec11ffffa87f04c7a2ff8fb706443e46638f601cd
