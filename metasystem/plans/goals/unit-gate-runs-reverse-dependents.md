# unit-gate-runs-reverse-dependents

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="Gate-only change on the landing lane and a new read-only verb; a wrong dependents computation shows as a missed or extra package in the gate's own tests"
- Tier: 2
- Intent: Every unit is gated as it stacks, by machinery. DONE when the batch lane's join gate and the verb metasystem gate unit --base <sha> run every reverse dependent of each changed package whole (with -tags batchtest for cmd/metasystem) and eject the unit on a red naming the failing tests.
- Origin: human
- Next step: U1 (dm-ugate) in flight as a Codex job on a2d38afad38379fcd9a4747f18adbe3035958309; land in wave 3e; the integrator runs gate unit after every stacked unit from then on.
- OpenedAt: 2026-09-18T14:56:10Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-18T14:56:10Z 15T6NP85HS5ADAYTWNPMWR3G0T-m1e-c6925449 open actor=human:Wido targets=unit-gate-runs-reverse-dependents
Integrity: sha256=08f4bd43cbe473abbf92b8dc284d7d5a98a7414a9a2dea31b73de5181d27d33d
