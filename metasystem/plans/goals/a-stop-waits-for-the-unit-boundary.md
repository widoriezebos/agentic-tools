# a-stop-waits-for-the-unit-boundary

- State: approved
- Priority: 1
- Sequence: 59
- Risk: severity=3 novelty=2 exposure=3 accumulation=2 basis="A wrong stop loses one round and is recoverable (severity 3); the mechanism reuses FindBreachStops and the existing admission checks (novelty 2); every goal's budget passes through it (exposure 3); two units, no new store beyond the terminal reason and the overrun fields (accumulation 2)"
- Tier: 3
- Intent: No automatic stop cuts a unit mid-work: an elapsed-budget breach waits for the unit's durable handoff, the breached unit still gets its own read and allowed corrections, and the overrun is recorded on the goal
- Origin: main
- Next step: U1a (no automatic elapsed stop cuts a unit: BreachStopEligible in FindBreachStops, DEFERRED route, prober seams incl. CompleteRevival) then U1b (grace on the runner's authorization through the three admission checks, elapsedOverrun from the goal clock); design plans/designs/a-stop-waits-for-the-unit-boundary.md accepted at the stop 2026-10-19; brief with inventory, Astra, Codex by hand, Opus read, hand gate, landing
- OpenedAt: 2026-10-19T09:44:21Z
- Revision: 4
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-19T09:44:39Z revision=2 opid=YCVH4J2CMKGB5R48Y1MH2W8HR7-m1e-c5a1e456 authority=proven digest=c956d5dfc500bcc74a6c4df557f991865e8f3bb896701b055bdc7cf42922877d episode=2

History:
- 2026-10-19T09:44:21Z E9KH14VBNRFY6DDSVYRYM5MFRK-m1e-c5a1e456 open actor=human:Wido targets=a-stop-waits-for-the-unit-boundary
- 2026-10-19T09:44:39Z YCVH4J2CMKGB5R48Y1MH2W8HR7-m1e-c5a1e456 approve actor=human:Wido targets=a-seat-pauses-at-its-spend-line,a-stop-waits-for-the-unit-boundary
- 2026-10-19T09:44:49Z AEPMZFV33J2YP45QP1E926C2MS-m1e-c5a1e456 set-priority actor=human:Wido targets=a-stop-waits-for-the-unit-boundary reason=priority-order subject=a-stop-waits-for-the-unit-boundary from=unranked to=1:60 requested-sequence=append
- 2026-10-19T12:33:39Z F40E38EKT8XF89X65TNPJ5RYKR-m1e-f2f350ed done actor=human:Wido targets=a-persons-channel-carries-only-acts-and-deliveries,a-proof-unit-ends-on-time,a-proof-unit-ends-on-time-2,a-seat-pauses-at-its-spend-line,a-stop-waits-for-the-unit-boundary,tests-observe-only-what-they-inject reason=priority-order from=1:60 to=1:59
Integrity: sha256=6afc56a5883d4b4f0d9d6b413712e08352fe837bc800cad98c3bd622d7f220aa
