# unit-gate-runs-reverse-dependents

- State: approved
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="Gate-only change on the landing lane and a new read-only verb; a wrong dependents computation shows as a missed or extra package in the gate's own tests"
- Tier: 2
- Intent: Every unit is gated as it stacks, by machinery. DONE when the batch lane's join gate and the verb metasystem gate unit --base <sha> run every reverse dependent of each changed package whole (with -tags batchtest for cmd/metasystem) and eject the unit on a red naming the failing tests.
- Origin: human
- Next step: Opened 09-18 by Wido (gate each unit as it stacks; machinery). U1 join gate + verb gate unit run every reverse dependent (dm-ugate) in flight on a2d38afad38379fcd9a4747f18adbe3035958309. Next: land in wave 3e; the integrator runs gate unit after every stacked unit from then on.
- OpenedAt: 2026-09-18T14:56:10Z
- Revision: 3
- Budget: elapsedLimit=1d attemptLimit=20 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-18T14:59:39Z revision=3 opid=PA6032YQ1G064X0Z9Y1QAZWXZ9-m1e-c6925449 authority=proven digest=63ef3634356773f959dd428b7ed54f0b1e812050f057aa8ccb2166e1dbe3833a

History:
- 2026-09-18T14:56:10Z 15T6NP85HS5ADAYTWNPMWR3G0T-m1e-c6925449 open actor=human:Wido targets=unit-gate-runs-reverse-dependents
- 2026-09-18T14:58:37Z SZ75KQX9ZXQAXQ9TNKMTGYPQG3-m1e-c6925449 edit actor=human:Wido targets=unit-gate-runs-reverse-dependents
- 2026-09-18T14:59:39Z PA6032YQ1G064X0Z9Y1QAZWXZ9-m1e-c6925449 approve actor=human:Wido targets=unit-gate-runs-reverse-dependents
Integrity: sha256=252377f5b098c6265348c16381a862ea8b981e59511f155aafac0148f9339e55
