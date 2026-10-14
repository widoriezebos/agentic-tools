# runs-advance-on-their-own

- State: claimed
- Risk: severity=3 novelty=2 exposure=3 accumulation=2 basis="Every unattended run depends on the driver (exposure 3); a wrong next step builds the wrong thing, caught by the read and the gate, recoverable (severity 3); the driver consumes records that goals 2 and 3 define (novelty 2); several units (accumulation 2)"
- Tier: 3
- Intent: Runs advance without a person: the driver consumes each step end at the unit boundary and starts the next step, merges main into the goal branch before each unit build, and starts work only within the measured host capacity
- Origin: main
- Next step: Design from plans/runs-advance-on-their-own-design-brief.md (units at most 250 production lines, at most 5), then build by hand with delegates
- OpenedAt: 2026-10-07T20:19:40Z
- FirstClaimAt: 2026-10-14T15:05:06Z
- Revision: 3
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-07T20:19:49Z revision=2 opid=C70G6QYMAARPGBH95GXJ0923SN-m1e-718ba0eb authority=proven digest=b3d93eec688f6818008d848a949f3cb036e08456a986274a226dcbd44c46328e episode=2
- Claimed: machine=m1e lineage=main-1790454088-93948-21671b at=2026-10-14T15:05:06Z areas="null" areas-source="01M4D2BENGCZB2AS0151WGP4QC@32a34f698f5cae9d05734d6347c6eeda864eef6e02a4a98f5aae0532186cc104" areas-known=false areas-warnings="[\"areas unknown for goal runs-advance-on-their-own in design /Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/plans/designs/runs-advance-on-their-own.md\",\"areas unknown for goal lane-first-run-fixes\",\"areas unknown for goal lane-recovers-and-pauses\",\"areas unknown for goal process-changes-cover-declarations-and-interventions\",\"areas unknown for goal process-changes-cover-declarations-and-interventions\"]" revision=3 accountingRevision=3 episodeAt=2026-10-14T15:05:06Z episodeRevision=3
- StopCapability: generation=3 revision=3 machine=m1e claimEpoch=9 fenceEpoch=0

History:
- 2026-10-07T20:19:40Z VQN7WX7T5069E3DYCE65RZTNKQ-m1e-718ba0eb open actor=human:Wido targets=runs-advance-on-their-own
- 2026-10-07T20:19:49Z C70G6QYMAARPGBH95GXJ0923SN-m1e-718ba0eb approve actor=human:Wido targets=runs-advance-on-their-own
- 2026-10-14T15:05:06Z PZ5706WTYQBX5A3EACMKWEN73V-m1e-f456f182 claim actor=m1e+main-1790454088-93948-21671b targets=runs-advance-on-their-own reason=areas unknown for goal runs-advance-on-their-own in design /Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/plans/designs/runs-advance-on-their-own.md; areas unknown for goal lane-first-run-fixes; areas unknown for goal lane-recovers-and-pauses; areas unknown for goal process-changes-cover-declarations-and-interventions; areas unknown for goal process-changes-cover-declarations-and-interventions
Integrity: sha256=72858e1d5235ebbd016b0deb30be1398e85ed5b857a5a33b752ca8838c1fa2d3
