# unit-gate-runs-reverse-dependents

- State: approved
- Priority: 1
- Sequence: 49
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="Gate-only change on the landing lane and a new read-only verb; a wrong dependents computation shows as a missed or extra package in the gate's own tests"
- Tier: 2
- Intent: Every unit is gated as it stacks, by machinery. DONE when the batch lane's join gate and the verb metasystem gate unit --base <sha> run every reverse dependent of each changed package whole (with -tags batchtest for cmd/metasystem) and eject the unit on a red naming the failing tests.
- Origin: human
- Next step: U1 (join gate and gate unit --base run every reverse dependent; a red ejects the unit) on main d2713932efcdc9a3adee0493bd8196f3996e1bb4; nothing remains; gate unit runs after each stacked unit.
- OpenedAt: 2026-09-18T14:56:10Z
- Revision: 5
- Budget: elapsedLimit=1d attemptLimit=20 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-18T14:59:39Z revision=3 opid=PA6032YQ1G064X0Z9Y1QAZWXZ9-m1e-c6925449 authority=proven digest=63ef3634356773f959dd428b7ed54f0b1e812050f057aa8ccb2166e1dbe3833a

History:
- 2026-09-18T14:56:10Z 15T6NP85HS5ADAYTWNPMWR3G0T-m1e-c6925449 open actor=human:Wido targets=unit-gate-runs-reverse-dependents
- 2026-09-18T14:58:37Z SZ75KQX9ZXQAXQ9TNKMTGYPQG3-m1e-c6925449 edit actor=human:Wido targets=unit-gate-runs-reverse-dependents
- 2026-09-18T14:59:39Z PA6032YQ1G064X0Z9Y1QAZWXZ9-m1e-c6925449 approve actor=human:Wido targets=unit-gate-runs-reverse-dependents
- 2026-09-18T15:01:01Z RTD7V1C31TB8MGA3AKEF18XZ96-m1e-c6925449 set-priority actor=human:Wido targets=unit-gate-runs-reverse-dependents reason=priority-order subject=unit-gate-runs-reverse-dependents from=unranked to=1:49 requested-sequence=49
- 2026-09-18T20:05:06Z GXZ1MMCGY1KHG1RTHGBSBV6KQC-m1e-c6925449 edit actor=human:Wido targets=unit-gate-runs-reverse-dependents
Integrity: sha256=55a6436e43b2e244582a4179b8fe2074d7f004d4c889879b3024193bfa7659a5
