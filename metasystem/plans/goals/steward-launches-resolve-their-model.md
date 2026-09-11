# steward-launches-resolve-their-model

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="severity 2: steward continuation has delivered nothing on any seat since the launches die; novelty 1: the roster resolution exists for delegates; exposure 2: steward launches on every seat; accumulation 1: one code path"
- Tier: 2
- Intent: Steward continuation launches on m1d and m1e have died within three seconds since 2026-09-09 13:57Z with the API error that the literal model name '<model>' is not supported: nine launches, first steward-7d27d52dc2060f32, still failing 2026-09-11 15:52Z (codex-sessions.md section 3 of the delivery deep dive). The roster placeholder is passed to Codex unresolved. DONE means: a steward launch resolves its model from the configured roster before spawning and refuses with a named configuration error when it cannot, never spawning with a placeholder; a fixture proves a launch with the metasystem.conf roster resolves; every seat's steward launch has completed at least once after the fix. Goal 2 of plans/delivery-efficiency-plan.md.
- Origin: human
- Next step: Find where the steward composes its delegate command (grep the '<model>' placeholder under scripts/agents and internal), resolve it through the same roster path as dispatch, add the refusal and the fixture, land.
- OpenedAt: 2026-09-11T15:44:54Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-11T15:44:54Z HF9CZF90NBZDKY1K5Z5G13AQZ9-m1e-5083721b open actor=m1e+main-1789141490-27414-b9c0d2 targets=steward-launches-resolve-their-model
Integrity: sha256=783a37dca177589d53e03b55b54973bdc4ed656fd804530bdf7e490fcb06de46
