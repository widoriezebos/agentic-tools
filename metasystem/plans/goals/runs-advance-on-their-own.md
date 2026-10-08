# runs-advance-on-their-own

- State: approved
- Risk: severity=3 novelty=2 exposure=3 accumulation=2 basis="Every unattended run depends on the driver (exposure 3); a wrong next step builds the wrong thing, caught by the read and the gate, recoverable (severity 3); the driver consumes records that goals 2 and 3 define (novelty 2); several units (accumulation 2)"
- Tier: 3
- Intent: Runs advance without a person: the driver consumes each step end at the unit boundary and starts the next step, merges main into the goal branch before each unit build, and starts work only within the measured host capacity
- Origin: main
- Next step: Design from plans/runs-advance-on-their-own-design-brief.md (units at most 250 production lines, at most 5), then build by hand with delegates
- OpenedAt: 2026-10-07T20:19:40Z
- Revision: 2
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-07T20:19:49Z revision=2 opid=C70G6QYMAARPGBH95GXJ0923SN-m1e-718ba0eb authority=proven digest=b3d93eec688f6818008d848a949f3cb036e08456a986274a226dcbd44c46328e episode=2

History:
- 2026-10-07T20:19:40Z VQN7WX7T5069E3DYCE65RZTNKQ-m1e-718ba0eb open actor=human:Wido targets=runs-advance-on-their-own
- 2026-10-07T20:19:49Z C70G6QYMAARPGBH95GXJ0923SN-m1e-718ba0eb approve actor=human:Wido targets=runs-advance-on-their-own
Integrity: sha256=b9d89b3dc94a35cb3cecf20f48df177bb537baa0deb04ba0aef58fe65a5eccd4
