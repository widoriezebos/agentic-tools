# witness-fallback-only-on-environmental-refusal

- State: queued
- Risk: severity=1 novelty=1 exposure=1 accumulation=1 basis="severity 1: each item removes measured waste inside one test or fixture and changes no law; novelty 1: known shapes; exposure 1: one group's duration; accumulation 1: none"
- Tier: 1
- Intent: The witness gate falls back to the plain full gate only for environmental refusals (snapshot extraction, witness write, witness_refusal reasons); a test failure or a coverage-ratchet refusal inside the snapshot is terminal. Today scripts/agents/witness-gate.sh lines 157 to 159 re-run everything on any snapshot failure, which cost 21 minutes for a deterministic answer on 2026-08-30. DONE means a deterministic red inside the snapshot ends the gate red at once with the reason readable, and an environmental refusal still falls back.
- Origin: human
- Next step: Slice 4 item 2 of plans/suite-speed-plan.md. Classify the refusal reasons the snapshot gate can produce before changing the branch; a fixture proves both paths. Code critique only.
- OpenedAt: 2026-09-10T12:02:42Z
- Revision: 1
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0

History:
- 2026-09-10T12:02:42Z E906XJ9V80KJBQ3NZ6QCDD5HJ2-m1e-892cdaec open actor=m1e+main-1789030447-51011-5722fc targets=witness-fallback-only-on-environmental-refusal
Integrity: sha256=4b91eb18a937d75a5cfda0c5584358056a244daa6b0323631158ed246a1c25af
