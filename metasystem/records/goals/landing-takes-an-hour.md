# landing-takes-an-hour

- State: done
- Priority: 1
- Sequence: 49
- Risk: severity=3 novelty=2 exposure=3 accumulation=2 basis="Every landing and every unit check runs through it (exposure 3); a wrong gate hides a red until main (severity 3), recoverable by the full proof; reshapes existing gate and lane owners (novelty 2); five units (accumulation 2)"
- Tier: 3
- Intent: A goal lands in about an hour: the gate shards cmd and runs in parallel so a panic hides nothing, a proof holds the host, reds return per package as parallel fix units, the unit check includes the tests of what the unit changed, and the landing clock measures it (goal 2 took 6h44m; plan Learned 2026-10-08)
- Origin: main
- Next step: Design from plans/landing-takes-an-hour-design-brief.md (Fable, Astra critique), then build U1-U5; before switch-on
- Concluded: Landing takes an hour, integrated with the lane: U1 sharded parallel gate (a panic loses one shard), U4 unit check by impact through the Go adapter with reverse dependents, U5 landing clock with push completion reconciled, U3 reds per failed unit with one replay on main parent; integration fixes found by the landing read (an empty LANDING_ONLY is not a replay), layering (go list in the adapter) and the path flag. Landed by hand: full gate 16 min, one fix round.
- OpenedAt: 2026-10-08T15:31:35Z
- Revision: 4
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-08T15:31:42Z revision=2 opid=5ZBZMDBA591S3Z4GCB34HMXJ89-m1e-718ba0eb authority=proven digest=e85c4bc3e971cca3ae5a5a90daa52b624e6093dbc43182eb7e56634cfcdeaf31 episode=2

History:
- 2026-10-08T15:31:35Z 2251E4F2JBE59GGM9K2JH9K4FS-m1e-718ba0eb open actor=human:Wido targets=landing-takes-an-hour
- 2026-10-08T15:31:42Z 5ZBZMDBA591S3Z4GCB34HMXJ89-m1e-718ba0eb approve actor=human:Wido targets=landing-takes-an-hour
- 2026-10-08T15:31:50Z Q8E8FMXX256WEN04XCZ5YHMHJB-m1e-718ba0eb set-priority actor=human:Wido targets=landing-takes-an-hour reason=priority-order subject=landing-takes-an-hour from=unranked to=1:49 requested-sequence=append
- 2026-10-09T14:54:47Z EQQ0PSBQ7RRH1NFZ8003AC1Q7F-m1e-718ba0eb done actor=human:Wido targets=landing-takes-an-hour,landing-takes-an-hour-pipeline
Integrity: sha256=cfd318207fb7999707a9533177c765e0345a65f4e5681d2794aba90084e04a25
