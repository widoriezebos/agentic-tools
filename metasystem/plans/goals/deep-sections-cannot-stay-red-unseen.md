# deep-sections-cannot-stay-red-unseen

- State: queued
- Risk: severity=3 novelty=2 exposure=3 accumulation=3 basis="severity 3: a red that only a deep section sees can sit on trunk for days and then fail an unrelated landing; novelty 2: needs a rule for when deep groups run outside a deep landing and where the result is seen; exposure 3: every deep-only section; accumulation 3: reds pile up unseen and each blocks the next deep landing"
- Tier: 2
- Intent: A red in a section that only a deep group runs must surface within a bounded time, not at the next unrelated deep landing. On 2026-09-14 m1c found witness-gate-fixtures red on trunk since 2026-09-09 (goal witness-bed-fakes-the-proof-engine-build). The section is only in proof-and-landing's deep group, standard landings never ran it, and a deep cadence run exists (internal/testpolicy/select.go:158-161, ruling R-3) yet did not surface it. DONE: the reason the cadence run missed it is recorded; every deep-only section runs against the trunk tip on a declared cadence or trigger; its result is recorded where seats and the next landing see it; a red names or opens a goal; and a fixture proves a planted deep-only red surfaces without a deep landing.
- Origin: human
- Next step: Find why the deep cadence run did not surface the witness-gate red between 2026-09-09 and 2026-09-14, then design where deep-only results are run and seen.
- OpenedAt: 2026-09-14T21:10:18Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-14T21:10:18Z A1BK3NVF3JMVA698PHBQ8C9Z2H-m1e-c6925449 open actor=human:Wido targets=deep-sections-cannot-stay-red-unseen reason=TierOverride: derived=3 set=2 why=Opened by m1c in Wido's name under R-110-m1e: the structural gap behind witness-bed-fakes-the-proof-engine-build.
Integrity: sha256=32a40c51f7312a07593da275ef3a088fb3b990399b73fab019197aaa0c23f507
