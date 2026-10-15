# runs-advance-on-their-own

- State: done
- Risk: severity=3 novelty=2 exposure=3 accumulation=2 basis="Every unattended run depends on the driver (exposure 3); a wrong next step builds the wrong thing, caught by the read and the gate, recoverable (severity 3); the driver consumes records that goals 2 and 3 define (novelty 2); several units (accumulation 2)"
- Tier: 3
- Intent: Runs advance without a person: the driver consumes each step end at the unit boundary and starts the next step, merges main into the goal branch before each unit build, and starts work only within the measured host capacity
- Origin: main
- Next step: Design from plans/runs-advance-on-their-own-design-brief.md (units at most 250 production lines, at most 5), then build by hand with delegates
- Concluded: Narrowed on 2026-10-14 to the bounded landing-recovery extraction; landed by hand on 2026-10-15 03:10 at 0b48a1829 (broken machine rule): Codex build, one correction (unset and batch-idle again see a pending reservation), final Opus 5.5 read LAND with 0 material. Follow-ups for the open runs-advance work: a held reservation reports a failure and so skips the steward tick (report it as held); the production wiring of the unclaimed view has no test through newLandingAgentKeeper.
- OpenedAt: 2026-10-07T20:19:40Z
- FirstClaimAt: 2026-10-14T15:05:06Z
- Revision: 6
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-07T20:19:49Z revision=2 opid=C70G6QYMAARPGBH95GXJ0923SN-m1e-718ba0eb authority=proven digest=b3d93eec688f6818008d848a949f3cb036e08456a986274a226dcbd44c46328e episode=2
- Sliced: machine=m1e lineage=main-1790454088-93948-21671b revision=3 at=2026-10-14T16:49:42Z

History:
- 2026-10-07T20:19:40Z VQN7WX7T5069E3DYCE65RZTNKQ-m1e-718ba0eb open actor=human:Wido targets=runs-advance-on-their-own
- 2026-10-07T20:19:49Z C70G6QYMAARPGBH95GXJ0923SN-m1e-718ba0eb approve actor=human:Wido targets=runs-advance-on-their-own
- 2026-10-14T15:05:06Z PZ5706WTYQBX5A3EACMKWEN73V-m1e-f456f182 claim actor=m1e+main-1790454088-93948-21671b targets=runs-advance-on-their-own reason=areas unknown for goal runs-advance-on-their-own in design /Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/plans/designs/runs-advance-on-their-own.md; areas unknown for goal lane-first-run-fixes; areas unknown for goal lane-recovers-and-pauses; areas unknown for goal process-changes-cover-declarations-and-interventions; areas unknown for goal process-changes-cover-declarations-and-interventions
- 2026-10-14T16:49:42Z JCKY21E4TH8804P4WWK0JA67K5-m1e-f456f182 slice-start actor=m1e+main-1790454088-93948-21671b targets=runs-advance-on-their-own
- 2026-10-14T18:16:47Z KFQ7QMY5EVA6SZWGTCN8KBB75E-m1e-cd3db6d0 release actor=human:Wido targets=runs-advance-on-their-own displaced=m1e+main-1790454088-93948-21671b@2026-10-14T15:05:06Z reason=Whole-change read of the replacement 9f5600ed found five material defects (four regressions against main's tests, a polling work wait); the goal narrows to a bounded landing-recovery extraction built later (decision 2026-10-14 20:15, Astra-critiqued); design-round is read and landed first.
- 2026-10-15T01:10:35Z 1YCEV3ZEJX889SDHHMH4SP11D9-m1e-670f65d0 done actor=human:Wido targets=runs-advance-on-their-own
Integrity: sha256=5f40cdaf30bb4979c08e9b3de98715f0041ed6783aece8c2b15e8c8081b0c23d
