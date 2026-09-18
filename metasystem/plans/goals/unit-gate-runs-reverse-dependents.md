# unit-gate-runs-reverse-dependents

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="Gate-only change on the landing lane and a new read-only verb; a wrong dependents computation shows as a missed or extra package in the gate's own tests"
- Tier: 2
- Intent: Every unit is gated as it stacks, by machinery. DONE when the batch lane's join gate and the verb metasystem gate unit --base <sha> run every reverse dependent of each changed package whole (with -tags batchtest for cmd/metasystem) and eject the unit on a red naming the failing tests.
- Origin: human
- Next step: Opened 09-18 by Wido (gate each unit as it stacks; machinery). U1 join gate + verb gate unit run every reverse dependent (dm-ugate) in flight on a2d38afad38379fcd9a4747f18adbe3035958309. Next: land in wave 3e; the integrator runs gate unit after every stacked unit from then on.
- OpenedAt: 2026-09-18T14:56:10Z
- Revision: 2
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-18T14:56:10Z 15T6NP85HS5ADAYTWNPMWR3G0T-m1e-c6925449 open actor=human:Wido targets=unit-gate-runs-reverse-dependents
- 2026-09-18T14:58:37Z SZ75KQX9ZXQAXQ9TNKMTGYPQG3-m1e-c6925449 edit actor=human:Wido targets=unit-gate-runs-reverse-dependents
Integrity: sha256=195584d29a7257865a24eeb0c873d5812e371573632950c7c34d59a19fc3653c
