# a-proof-unit-ends-on-time

- State: done
- Priority: 1
- Sequence: 58
- Risk: severity=3 novelty=2 exposure=3 accumulation=2 basis="Lane test finding 374 (2026-10-17): the full proof of main hung 74 minutes on one deploy test whose wait had no producer check and no deadline under the lane's -timeout 0; the lane only reported drift; the steward ended it by hand. Findings 375, 379, 380 record the test's hold race, the unrecordable repeat of a dead unit and the proof folder's growth. Astra design rounds 1-5 (R-35-m3: no elapsed-time failure)."
- Tier: 3
- Intent: A proof unit never hangs the lane: a test wait fails when the thing it waits on is observed gone (never because time passed), and the lane names a unit that has finished no test for longer than its history, so a person can act.
- Origin: human
- Next step: U1 (D1): testenv AwaitFrom + ProcessGroupProducer adopted by the deploy bed's awaitHeld; brief FINAL by the stop rule; Codex build in -unitbound, Opus read, gate, hand landing. U2 (D2): the lane's quiet diagnostics after the-lane-says-what-it-does U1a.
- Concluded: U1 (1d5ba30607) landed by hand on 2026-10-19 after the sharded gate GATE OK 14:17 (tree b333b652, evidence gates/aproof-u1-20261019); U2-U5 continue in a-proof-unit-ends-on-time-2
- OpenedAt: 2026-10-17T05:48:03Z
- FirstClaimAt: 2026-10-19T11:09:34Z
- Revision: 5
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-17T05:48:39Z revision=2 opid=NNNDNP9783EA9GP7KXJDR4BVB9-m1e-c6925449 authority=proven digest=ca4b5dfd57a2bb664a76d04f72e37863ce619b80dff165a650e229ac00a27cd6 episode=2

History:
- 2026-10-17T05:48:03Z BSASQ4NEZB9R09BCZH7FR6Q3CN-m1e-c5a1e456 open actor=human:Wido targets=a-proof-unit-ends-on-time
- 2026-10-17T05:48:39Z NNNDNP9783EA9GP7KXJDR4BVB9-m1e-c6925449 approve actor=human:Wido targets=a-proof-unit-ends-on-time
- 2026-10-17T05:50:44Z CFZ65FDDHM2B4BG2BZAQTAMB09-m1e-c5a1e456 set-priority actor=human:Wido targets=a-proof-unit-ends-on-time reason=priority-order subject=a-proof-unit-ends-on-time from=unranked to=1:58 requested-sequence=append
- 2026-10-19T11:09:34Z R7AT362KYEFJ7W121SCJ9VXDHJ-m1e-f456f182 claim actor=human:Wido targets=a-proof-unit-ends-on-time authorityOutcome=HUMAN_AUTHORITY_PROVEN authorityGeneration=99 reason=areas unknown for goal a-proof-unit-ends-on-time in design /Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/plans/designs/a-proof-unit-ends-on-time.md; areas unknown for goal asks-clear-on-their-effect; areas unknown for goal lane-first-run-fixes; areas unknown for goal lane-recovers-and-pauses
- 2026-10-19T12:33:39Z F40E38EKT8XF89X65TNPJ5RYKR-m1e-f2f350ed done actor=human:Wido targets=a-persons-channel-carries-only-acts-and-deliveries,a-proof-unit-ends-on-time,a-proof-unit-ends-on-time-2,a-seat-pauses-at-its-spend-line,a-stop-waits-for-the-unit-boundary,tests-observe-only-what-they-inject displaced=m1e+main-1790454088-93948-21671b@2026-10-19T11:09:34Z
Integrity: sha256=7a1cfb0bb76f85cb76d1c6acb4131cc60a1b704b43c92cb2d4d636f7cf6c7176
